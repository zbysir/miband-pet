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
	cmd.Stdin = strings.NewReader(`{"session_id": "abc-123", "rate_limits":{"five_hour":{"used_percentage":63,"resets_at":2000000000}}}`)
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "原来的状态栏" {
		t.Fatalf("原来的状态栏没输出：%v %q", err, out)
	}
	if data, err := os.ReadFile(s.DataFile()); err != nil || !strings.Contains(string(data), `"used_percentage":63`) {
		t.Fatalf("没存下状态栏数据：%v %s", err, data)
	}
	if data, err := os.ReadFile(filepath.Join(s.SessionDir(), "abc-123.json")); err != nil || !strings.Contains(string(data), `"used_percentage":63`) {
		t.Fatalf("没按会话存一份：%v %s", err, data)
	}
	// 会话 id 里有别的字符：不按会话存（免得拼出别的路径）
	cmd = exec.Command("sh", "-c", s.command())
	cmd.Stdin = strings.NewReader(`{"session_id":"../../evil"}`)
	_, _ = cmd.CombinedOutput()
	if entries, _ := os.ReadDir(s.SessionDir()); len(entries) != 1 {
		t.Fatalf("会话目录里应该只有一个文件：%v", entries)
	}

	// 启动时清掉很久没刷新的会话文件
	stale := filepath.Join(s.SessionDir(), "old.json")
	write(t, stale, "{}")
	_ = os.Chtimes(stale, time.Now().Add(-8*24*time.Hour), time.Now().Add(-8*24*time.Hour))
	if err := s.Refresh(7 * 24 * time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("旧的会话文件没清掉")
	}
	if _, err := os.Stat(filepath.Join(s.SessionDir(), "abc-123.json")); err != nil {
		t.Fatal("新的会话文件不该清")
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
	if _, err := os.Stat(s.SessionDir()); !os.IsNotExist(err) {
		t.Fatal("会话目录没删")
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
