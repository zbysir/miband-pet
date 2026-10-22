// Package usage 读 Claude Code 和 Codex 的额度（5 小时窗口、每周窗口用了百分之几、什么时候重置）。
//
//   - Claude Code：从状态栏数据里读（Claude Code 每次刷新状态栏都会把 rate_limits 传给状态栏命令，
//     只有 Pro/Max 订阅才有）。要先在 App 里打开「显示 Claude Code 额度」，见 statusline.go。
//   - Codex：会话日志 ~/.codex/sessions/**/rollout-*.jsonl 里的 token_count 记录自带 rate_limits，直接读，不用配置。
//
// 读不到的就不返回，界面上也就不显示。
package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Window 是一个额度窗口。
type Window struct {
	Name     string  `json:"name"`     // 5h / 7d / 额度（Claude apps gateway 的花费上限）等
	Used     float64 `json:"used"`     // 用了百分之几，0~100（花费上限可能超过 100）
	ResetsAt int64   `json:"resetsAt"` // 什么时候重置（Unix 秒），0 表示不知道
}

// Source 是一种 agent 的额度。
type Source struct {
	Agent   string   `json:"agent"` // claude / codex
	Windows []Window `json:"windows"`
	Updated int64    `json:"updated"` // 数据是什么时候的（Unix 秒）
}

// Read 读所有能读到的额度。statusFile 是状态栏数据（见 Statusline），codexDir 是 Codex 的会话目录。
func Read(statusFile, codexDir string, now time.Time) []Source {
	var out []Source
	if s, err := ReadClaude(statusFile, now); err == nil && len(s.Windows) > 0 {
		out = append(out, *s)
	}
	if s, err := ReadCodex(codexDir, now); err == nil && len(s.Windows) > 0 {
		out = append(out, *s)
	}
	return out
}

// ---------- Claude Code ----------

type claudeWindow struct {
	Used     *float64 `json:"used_percentage"`
	ResetsAt int64    `json:"resets_at"`
}

// ReadClaude 读状态栏数据里的 rate_limits。
func ReadClaude(statusFile string, now time.Time) (*Source, error) {
	raw, err := os.ReadFile(statusFile)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(statusFile)
	if err != nil {
		return nil, err
	}
	var data struct {
		RateLimits map[string]*claudeWindow `json:"rate_limits"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("状态栏数据不是 JSON：%w", err)
	}
	src := &Source{Agent: "claude", Updated: st.ModTime().Unix()}
	for _, w := range []struct{ key, name string }{{"five_hour", "5h"}, {"seven_day", "7d"}, {"spend_limit", "额度"}} {
		cw := data.RateLimits[w.key]
		if cw == nil || cw.Used == nil {
			continue
		}
		// 窗口重置之后，旧数据就不对了（下次刷新状态栏前不知道新窗口用了多少）
		if cw.ResetsAt > 0 && now.Unix() >= cw.ResetsAt {
			continue
		}
		src.Windows = append(src.Windows, Window{Name: w.name, Used: *cw.Used, ResetsAt: cw.ResetsAt})
	}
	return src, nil
}

// ---------- Codex ----------

type codexWindow struct {
	Used          *float64 `json:"used_percent"`
	WindowMinutes int      `json:"window_minutes"`
	ResetsAt      int64    `json:"resets_at"`
}

type codexLine struct {
	Timestamp string `json:"timestamp"`
	Payload   struct {
		RateLimits *struct {
			Primary   *codexWindow `json:"primary"`
			Secondary *codexWindow `json:"secondary"`
		} `json:"rate_limits"`
	} `json:"payload"`
}

// ReadCodex 找最近写过的会话日志，读里面最后一条额度。
func ReadCodex(dir string, now time.Time) (*Source, error) {
	file, err := latestSession(dir)
	if err != nil {
		return nil, err
	}
	line, err := lastLineWith(file, []byte(`"rate_limits"`))
	if err != nil {
		return nil, err
	}
	var l codexLine
	if err := json.Unmarshal(line, &l); err != nil {
		return nil, err
	}
	rl := l.Payload.RateLimits
	if rl == nil {
		return nil, fmt.Errorf("%s 里没有额度", file)
	}
	src := &Source{Agent: "codex"}
	if t, err := time.Parse(time.RFC3339, l.Timestamp); err == nil {
		src.Updated = t.Unix()
	} else if st, err := os.Stat(file); err == nil {
		src.Updated = st.ModTime().Unix()
	}
	for _, w := range []*codexWindow{rl.Primary, rl.Secondary} {
		if w == nil || w.Used == nil || (w.ResetsAt > 0 && now.Unix() >= w.ResetsAt) {
			continue
		}
		src.Windows = append(src.Windows, Window{Name: windowName(w.WindowMinutes), Used: *w.Used, ResetsAt: w.ResetsAt})
	}
	return src, nil
}

func windowName(minutes int) string {
	switch {
	case minutes == 300:
		return "5h"
	case minutes == 10080:
		return "7d"
	case minutes > 0 && minutes%1440 == 0:
		return fmt.Sprintf("%dd", minutes/1440)
	case minutes > 0 && minutes%60 == 0:
		return fmt.Sprintf("%dh", minutes/60)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	}
	return "?"
}

// latestSession：会话日志按 年/月/日 分目录，只看最近的几天，取修改时间最新的文件
func latestSession(dir string) (string, error) {
	days, _ := filepath.Glob(filepath.Join(dir, "*", "*", "*"))
	if len(days) == 0 {
		return "", fmt.Errorf("%s 下没有会话日志", dir)
	}
	sort.Strings(days)
	if len(days) > 3 {
		days = days[len(days)-3:]
	}
	var best string
	var bestTime time.Time
	for _, d := range days {
		files, _ := filepath.Glob(filepath.Join(d, "*.jsonl"))
		for _, f := range files {
			if st, err := os.Stat(f); err == nil && st.ModTime().After(bestTime) {
				best, bestTime = f, st.ModTime()
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("%s 下没有会话日志", dir)
	}
	return best, nil
}

// lastLineWith 从文件末尾往前读（最多 2MB），返回最后一行包含 needle 的内容
func lastLineWith(file string, needle []byte) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const window = 2 << 20
	start := max(st.Size()-window, 0)
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	var last []byte
	for sc.Scan() {
		if bytes.Contains(sc.Bytes(), needle) {
			last = append(last[:0], sc.Bytes()...)
		}
	}
	if last == nil {
		return nil, fmt.Errorf("%s 里没有 %s", file, needle)
	}
	return last, nil
}
