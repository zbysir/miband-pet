package usage

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadClaude(t *testing.T) {
	f := filepath.Join(t.TempDir(), "claude-status.json")
	write(t, f, `{"model":{"id":"x"},"rate_limits":{"five_hour":{"used_percentage":23.5,"resets_at":2000000000},"seven_day":{"used_percentage":41,"resets_at":1000}}}`)
	s, err := ReadClaude(f, time.Unix(1500, 0))
	if err != nil {
		t.Fatal(err)
	}
	// seven_day 已经过了重置时间，不要
	if len(s.Windows) != 1 || s.Windows[0].Name != "5h" || s.Windows[0].Used != 23.5 || s.Windows[0].ResetsAt != 2000000000 {
		t.Fatalf("不对：%+v", s.Windows)
	}

	// API key 用户没有 rate_limits
	write(t, f, `{"model":{"id":"x"}}`)
	if s, _ := ReadClaude(f, time.Unix(1500, 0)); len(s.Windows) != 0 {
		t.Fatalf("没有额度时不该有窗口：%+v", s.Windows)
	}
}

func TestReadCodex(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "2026", "10", "20", "rollout-a.jsonl"), `{"payload":{"rate_limits":{"primary":{"used_percent":99,"window_minutes":10080,"resets_at":2000000000}}}}`+"\n")
	time.Sleep(20 * time.Millisecond)
	write(t, filepath.Join(dir, "2026", "10", "21", "rollout-b.jsonl"), strings.Join([]string{
		`{"timestamp":"2026-10-21T09:00:00Z","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":10,"window_minutes":300,"resets_at":2000000000}}}}`,
		`{"timestamp":"2026-10-21T09:47:18.099Z","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","primary":{"used_percent":20.0,"window_minutes":10080,"resets_at":1793082770},"secondary":null}}}`,
		`{"timestamp":"2026-10-21T09:48:00Z","payload":{"type":"agent_message"}}`,
	}, "\n")+"\n")
	s, err := ReadCodex(dir, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Windows) != 1 || s.Windows[0] != (Window{Name: "7d", Used: 20, ResetsAt: 1793082770}) {
		t.Fatalf("应该读最新文件的最后一条：%+v", s.Windows)
	}
	if s.Updated != time.Date(2026, 10, 21, 9, 47, 18, 0, time.UTC).Unix() {
		t.Fatalf("时间不对：%d", s.Updated)
	}
}

func TestStatuslineRoundTrip(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "claude", "settings.json")
	orig := `{
  "model": "opus",
  "statusLine": {"type": "command", "command": "echo 原来的状态栏", "padding": 0},
  "hooks": {"Stop": []}
}
`
	write(t, settings, orig)
	s := Statusline{SettingsPath: settings, Dir: filepath.Join(dir, "app support")}
	if s.Enabled() {
		t.Fatal("还没打开")
	}
	if err := s.Enable(); err != nil {
		t.Fatal(err)
	}
	if !s.Enabled() {
		t.Fatal("应该打开了")
	}
	raw, _ := os.ReadFile(settings)
	got := string(raw)
	if strings.Index(got, `"model"`) > strings.Index(got, `"statusLine"`) || strings.Index(got, `"statusLine"`) > strings.Index(got, `"hooks"`) {
		t.Fatalf("键的顺序变了：\n%s", got)
	}
	if !strings.Contains(got, `"padding": 0`) || !strings.Contains(got, "statusline.sh") {
		t.Fatalf("statusLine 不对：\n%s", got)
	}
	if bak, _ := os.ReadFile(settings + ".bak-miband-pet"); string(bak) != orig {
		t.Fatal("没有备份原文件")
	}

	// 跑一遍状态栏命令：数据存下来，原来的状态栏照常输出
	cmd := exec.Command("sh", "-c", s.command())
	cmd.Stdin = strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":63,"resets_at":2000000000}}}`)
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "原来的状态栏" {
		t.Fatalf("原来的状态栏没输出：%v %q", err, out)
	}
	c, err := ReadClaude(s.DataFile(), time.Unix(1500, 0))
	if err != nil || len(c.Windows) != 1 || c.Windows[0].Used != 63 {
		t.Fatalf("没存下状态栏数据：%v %+v", err, c)
	}

	// 关掉：原样还原
	if err := s.Disable(); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(settings)
	if s.Enabled() || !strings.Contains(string(raw), `"command": "echo 原来的状态栏"`) || strings.Contains(string(raw), "statusline.sh") {
		t.Fatalf("没还原：\n%s", raw)
	}
	if _, err := os.Stat(s.script()); !os.IsNotExist(err) {
		t.Fatal("脚本没删")
	}
}

func TestStatuslineWithoutOriginal(t *testing.T) {
	dir := t.TempDir()
	s := Statusline{SettingsPath: filepath.Join(dir, "settings.json"), Dir: dir}
	// 没有 settings.json：打开时新建，关掉时去掉 statusLine
	if err := s.Enable(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", s.command())
	cmd.Stdin = strings.NewReader(`{}`)
	if out, err := cmd.CombinedOutput(); err != nil || len(out) != 0 {
		t.Fatalf("没有原来的状态栏时应该什么都不输出：%v %q", err, out)
	}
	if err := s.Disable(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(s.SettingsPath)
	if strings.TrimSpace(string(raw)) != "{}" {
		t.Fatalf("应该还原成空对象：%q", raw)
	}

	// 用户之后自己换了状态栏：关掉时不动 settings.json
	write(t, s.SettingsPath, `{"statusLine": {"type": "command", "command": "my-line"}}`)
	if err := s.Disable(); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(s.SettingsPath)
	if !strings.Contains(string(raw), "my-line") {
		t.Fatal("不该改用户自己的状态栏")
	}
}
