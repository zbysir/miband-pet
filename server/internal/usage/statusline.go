package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Statusline 接管 Claude Code 的状态栏命令，用来拿额度：Claude Code 只把 rate_limits 传给状态栏命令。
//
// 打开（Enable）时把 ~/.claude/settings.json 里的 statusLine.command 换成 Dir/statusline.sh：
// 它先把状态栏数据存到 Dir/claude-status.json，再原样交给原来的状态栏命令，状态栏显示不变。
// 原来的 statusLine 存在 Dir/statusline-orig.json，关掉（Disable）时原样还原。
// 改 settings.json 之前备份到 settings.json.bak-miband-pet，只改 statusLine 这一项，其它键的顺序不变。
type Statusline struct {
	SettingsPath string // ~/.claude/settings.json
	Dir          string // App 的数据目录
}

func DefaultStatusline(dir string) Statusline {
	home, _ := os.UserHomeDir()
	return Statusline{SettingsPath: filepath.Join(home, ".claude", "settings.json"), Dir: dir}
}

func (s Statusline) script() string   { return filepath.Join(s.Dir, "statusline.sh") }
func (s Statusline) origFile() string { return filepath.Join(s.Dir, "statusline-orig.json") }
func (s Statusline) origCmd() string  { return filepath.Join(s.Dir, "statusline-orig.cmd") }

// DataFile 是状态栏数据存的位置（Read 从这里读 Claude 的额度）。
func (s Statusline) DataFile() string { return filepath.Join(s.Dir, "claude-status.json") }

func (s Statusline) command() string { return fmt.Sprintf("sh %q", s.script()) }

const wrapper = `#!/bin/sh
# miband pet 生成：把 Claude Code 传给状态栏的数据存一份（App 从里面读额度），再交给原来的状态栏命令。
# 在 miband pet 里关掉「显示 Claude Code 额度」会还原原来的设置。
dir=$(dirname "$0")
input=$(cat)
printf '%s' "$input" > "$dir/claude-status.json.$$" && mv -f "$dir/claude-status.json.$$" "$dir/claude-status.json"
if [ -s "$dir/statusline-orig.cmd" ]; then
  printf '%s' "$input" | sh -c "$(cat "$dir/statusline-orig.cmd")"
fi
`

// Enabled：settings.json 里的状态栏命令是不是我们的。用户之后自己改了状态栏就算关了。
func (s Statusline) Enabled() bool {
	sl, _, err := s.readStatusLine()
	if err != nil || sl == nil {
		return false
	}
	var obj map[string]any
	if json.Unmarshal(sl, &obj) != nil {
		return false
	}
	return obj["command"] == s.command()
}

// readStatusLine 返回 settings.json 里的 statusLine（没有为 nil）和整个文件内容（文件不存在为 nil）
func (s Statusline) readStatusLine() (json.RawMessage, []byte, error) {
	raw, err := os.ReadFile(s.SettingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	keys, vals, err := topLevel(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("%s 不是合法的 JSON：%w", s.SettingsPath, err)
	}
	for i, k := range keys {
		if k == "statusLine" {
			return vals[i], raw, nil
		}
	}
	return nil, raw, nil
}

func (s Statusline) Enable() error {
	if s.Enabled() {
		return os.WriteFile(s.script(), []byte(wrapper), 0o755) // 脚本可能是旧版本，更新一下
	}
	orig, raw, err := s.readStatusLine()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	// 记下原来的 statusLine，还原时用；原来的命令给脚本调用
	origCmd := ""
	next := map[string]any{"type": "command"}
	if orig != nil {
		var obj map[string]any
		if err := json.Unmarshal(orig, &obj); err == nil {
			next = obj
			if c, ok := obj["command"].(string); ok && obj["type"] == "command" {
				origCmd = c
			}
		}
	}
	origJSON := []byte("null")
	if orig != nil {
		origJSON = orig
	}
	if err := os.WriteFile(s.origFile(), origJSON, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(s.origCmd(), []byte(origCmd), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(s.script(), []byte(wrapper), 0o755); err != nil {
		return err
	}
	next["type"] = "command"
	next["command"] = s.command()
	val, _ := json.Marshal(next)
	return s.writeStatusLine(raw, val)
}

// Disable 还原原来的 statusLine。如果状态栏已经不是我们的（用户自己改过），就不动 settings.json。
func (s Statusline) Disable() error {
	if s.Enabled() {
		orig, err := os.ReadFile(s.origFile())
		if err != nil {
			return fmt.Errorf("找不到原来的状态栏设置（%s），请手动改 %s：%w", s.origFile(), s.SettingsPath, err)
		}
		_, raw, err := s.readStatusLine()
		if err != nil {
			return err
		}
		var val json.RawMessage
		if t := bytes.TrimSpace(orig); !bytes.Equal(t, []byte("null")) {
			val = t
		}
		if err := s.writeStatusLine(raw, val); err != nil {
			return err
		}
	}
	for _, f := range []string{s.script(), s.origFile(), s.origCmd(), s.DataFile()} {
		_ = os.Remove(f)
	}
	return nil
}

// writeStatusLine 把 statusLine 换成 val（nil 表示删掉），其它键原样保留、顺序不变；写之前备份。
func (s Statusline) writeStatusLine(raw []byte, val json.RawMessage) error {
	var keys []string
	var vals []json.RawMessage
	if raw != nil {
		var err error
		if keys, vals, err = topLevel(raw); err != nil {
			return err
		}
		if err := os.WriteFile(s.SettingsPath+".bak-miband-pet", raw, 0o600); err != nil {
			return err
		}
	}
	found := false
	for i := 0; i < len(keys); i++ {
		if keys[i] != "statusLine" {
			continue
		}
		found = true
		if val == nil {
			keys = append(keys[:i], keys[i+1:]...)
			vals = append(vals[:i], vals[i+1:]...)
			i--
		} else {
			vals[i] = val
		}
	}
	if !found && val != nil {
		keys, vals = append(keys, "statusLine"), append(vals, val)
	}
	var b bytes.Buffer
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		kj, _ := json.Marshal(k)
		b.WriteString("\n  ")
		b.Write(kj)
		b.WriteString(": ")
		if err := json.Indent(&b, vals[i], "  ", "  "); err != nil {
			return err
		}
	}
	if len(keys) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	if err := os.MkdirAll(filepath.Dir(s.SettingsPath), 0o700); err != nil {
		return err
	}
	tmp := s.SettingsPath + ".tmp-miband-pet"
	if err := os.WriteFile(tmp, b.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.SettingsPath)
}

// topLevel 按原来的顺序拆出 JSON 对象的顶层键和值
func topLevel(raw []byte) ([]string, []json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, errors.New("顶层不是对象")
	}
	var keys []string
	var vals []json.RawMessage
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		k, ok := tok.(string)
		if !ok {
			return nil, nil, errors.New("键不是字符串")
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, nil, err
		}
		keys, vals = append(keys, k), append(vals, v)
	}
	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}
	if dec.More() || strings.TrimSpace(string(raw[dec.InputOffset():])) != "" {
		return nil, nil, errors.New("对象后面还有内容")
	}
	return keys, vals, nil
}
