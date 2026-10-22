package herdr

import (
	"context"
	"strings"
	"testing"
)

func TestEvents(t *testing.T) {
	prev := map[string]Agent{
		"a": {PaneID: "a", Agent: "claude", Status: "working", Cwd: "/x/miband", Name: "刷机"},
		"b": {PaneID: "b", Agent: "codex", Status: "working", Cwd: "/x/web"},
		"c": {PaneID: "c", Agent: "claude", Status: "idle", Cwd: "/x/c"},
		"d": {PaneID: "d", Agent: "claude", Status: "idle", Cwd: "/x/d"},
	}
	cur := map[string]Agent{
		"a": {PaneID: "a", Agent: "claude", Status: "done", Cwd: "/x/miband", Name: "刷机"},
		"b": {PaneID: "b", Agent: "codex", Status: "blocked", Cwd: "/x/web", TitleStrip: "登录"},
		"c": {PaneID: "c", Agent: "claude", Status: "working", Cwd: "/x/c"}, // 开始干活不提醒
		"d": {PaneID: "d", Agent: "claude", Status: "idle", Cwd: "/x/d"},
		"e": {PaneID: "e", Agent: "claude", Status: "blocked"}, // 新出现的不提醒
	}
	got := map[string]string{}
	for _, e := range Events(prev, cur, false) {
		got[e.Title] = e.Body
	}
	if len(got) != 2 || got["claude 完成"] != "miband · 刷机" || got["codex 等你处理"] != "web · 登录" {
		t.Fatalf("事件不对: %v", got)
	}

	got = map[string]string{}
	for _, e := range Events(prev, cur, true) {
		got[e.Title] = e.Body
	}
	if len(got) != 3 || got["claude 开始跑"] != "c · c" {
		t.Fatalf("带开始跑的事件不对: %v", got)
	}
}

func TestListLive(t *testing.T) {
	agents, err := List(context.Background())
	if err != nil {
		t.Skip("本机没有 herdr:", err)
	}
	s := NewSnapshot(agents)
	if len(s.Agents) != len(agents) || s.Key() == "" && len(agents) > 0 {
		t.Fatal("快照不对")
	}
	t.Logf("%d 个 agent", len(agents))
}

func TestNewSnapshotSorted(t *testing.T) {
	agents := map[string]Agent{}
	for _, id := range []string{"w1:p9", "w1:p1", "w7:pQ", "w1:p5", "w2:p3"} {
		agents[id] = Agent{PaneID: id, Status: "idle"}
	}
	// map 遍历顺序随机，多跑几次都得是同一个顺序
	for range 20 {
		s := NewSnapshot(agents)
		var ids []string
		for _, a := range s.Agents {
			ids = append(ids, a.ID)
		}
		if got := strings.Join(ids, ","); got != "w1:p1,w1:p5,w1:p9,w2:p3,w7:pQ" {
			t.Fatalf("顺序不对：%s", got)
		}
	}
}
