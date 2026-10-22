// Package herdr 读取本机 herdr 管理的 agent 状态。
package herdr

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Agent struct {
	PaneID     string `json:"pane_id"`
	Agent      string `json:"agent"`
	Status     string `json:"agent_status"` // idle / working / blocked / done / unknown
	Cwd        string `json:"cwd"`
	Name       string `json:"name"`
	TitleStrip string `json:"terminal_title_stripped"`
}

// List 跑 herdr agent list，按 pane_id 索引。
func List(ctx context.Context) (map[string]Agent, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "herdr", "agent", "list").Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("herdr agent list 失败: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	var resp struct {
		Result struct {
			Agents []Agent `json:"agents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("解析 herdr 输出失败: %w", err)
	}
	m := make(map[string]Agent, len(resp.Result.Agents))
	for _, a := range resp.Result.Agents {
		m[a.PaneID] = a
	}
	return m, nil
}

func (a Agent) Project() string {
	return filepath.Base(strings.TrimRight(a.Cwd, "/"))
}

func (a Agent) Title() string {
	if a.Name != "" {
		return a.Name
	}
	return a.TitleStrip
}

// Label 用在通知正文里：项目 · 任务标题
func (a Agent) Label() string {
	title := a.Title()
	if title == "" {
		title = a.PaneID
	}
	if p := a.Project(); p != "" && p != "." {
		return p + " · " + title
	}
	return title
}

var statusText = map[string]string{"done": "完成", "blocked": "等你处理", "idle": "完成", "working": "开始跑"}

// Event 是值得提醒的状态变化。
type Event struct{ Title, Body string }

// Events 在跑完（working → done/idle）和卡住（→ blocked）时提醒；withStart 时开始跑（→ working）也提醒。
// 新出现的 agent、标题变化不提醒，避免刷屏。
func Events(prev, cur map[string]Agent, withStart bool) []Event {
	var out []Event
	for pane, a := range cur {
		old, ok := prev[pane]
		if !ok || old.Status == a.Status {
			continue
		}
		finished := old.Status == "working" && (a.Status == "done" || a.Status == "idle")
		started := withStart && a.Status == "working"
		if a.Status == "blocked" || finished || started {
			name := a.Agent
			if name == "" {
				name = "agent"
			}
			text := statusText[a.Status]
			if text == "" {
				text = a.Status
			}
			out = append(out, Event{Title: name + " " + text, Body: a.Label()})
		}
	}
	return out
}

// Finished 数一下有几个 agent 刚跑完（working → done/idle）。
func Finished(prev, cur map[string]Agent) int {
	n := 0
	for pane, a := range cur {
		if old, ok := prev[pane]; ok && old.Status == "working" && (a.Status == "done" || a.Status == "idle") {
			n++
		}
	}
	return n
}

// AppAgent 是推给手环插件的单个 agent；字段要精简，整条消息要能装进一个数据帧。
type AppAgent struct {
	ID      string `json:"id"`
	Agent   string `json:"agent"`
	Project string `json:"project"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Since   int64  `json:"since,omitempty"` // 进入当前状态的时间（毫秒），Mac 开始观察之前就是这个状态的为 0
}

type Snapshot struct {
	T      string     `json:"t"`
	TS     int64      `json:"ts"`
	Agents []AppAgent `json:"agents"`
}

func NewSnapshot(agents map[string]Agent) *Snapshot {
	s := &Snapshot{T: "agents", TS: time.Now().UnixMilli(), Agents: make([]AppAgent, 0, len(agents))}
	for _, a := range agents {
		s.Agents = append(s.Agents, AppAgent{ID: a.PaneID, Agent: a.Agent, Project: a.Project(), Title: a.Title(), Status: a.Status})
	}
	return s
}

// Key 用来判断快照有没有变化（不含时间戳）
func (s *Snapshot) Key() string {
	var b strings.Builder
	keys := make([]string, 0, len(s.Agents))
	for _, a := range s.Agents {
		keys = append(keys, a.ID+"\x00"+a.Status+"\x00"+a.Title)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('\n')
	}
	return b.String()
}
