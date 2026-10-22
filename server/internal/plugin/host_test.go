package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"

	"miband/plugins"
)

// ---------- 在 goja 里模拟手环：用手环上的同一份运行时跑插件 ----------

// 和 render-app/src/pages/index/index.ux 里加载插件的方式一样
const harness = `
var createRuntime = (function () { var module = { exports: {} }; RUNTIME; return module.exports })()
function load(code, assets) {
  var rt = createRuntime({
    render: function (t) { __tree(JSON.stringify(t)) },
    error: function (m) { __err(m) },
    log: function () {},
    asset: function (p) { var h = assets[p]; if (!h) throw new Error('没有图片 ' + p); return 'internal://files/' + h + '.png' },
    vibrate: function () {},
    storageGet: function () {},
    storageSet: function () {},
    call: function (name, args) { __call(name, JSON.stringify(args)) }
  })
  var mod = { exports: {} }
  new Function('module', 'exports', 'require', 'console', '__h', '__Fragment', code)(mod, mod.exports, function () { return rt.api }, rt.api.console, rt.api.h, rt.api.Fragment)
  rt.setStore('screen', { width: 212, height: 303 })
  rt.mount(mod.exports.default)
  return rt
}
`

type band struct {
	t     *testing.T
	loop  *eventloop.EventLoop
	rt    goja.Value
	mu    sync.Mutex
	trees []map[string]any
	errs  []string
	calls []string // 插件调用 Mac 的记录："名字 参数JSON"
}

func newBand(t *testing.T, p *Plugin, code string) *band {
	t.Helper()
	runtimeJS, err := os.ReadFile("../../../render-app/src/common/runtime.js")
	if err != nil {
		t.Fatal(err)
	}
	b := &band{t: t, loop: eventloop.NewEventLoop()}
	b.loop.Start()
	t.Cleanup(func() { b.loop.Stop() })
	assets, _ := json.Marshal(p.Assets)
	done := make(chan error, 1)
	b.loop.RunOnLoop(func(vm *goja.Runtime) {
		_ = vm.Set("__tree", func(s string) {
			var m map[string]any
			_ = json.Unmarshal([]byte(s), &m)
			b.mu.Lock()
			b.trees = append(b.trees, m)
			b.mu.Unlock()
		})
		_ = vm.Set("__call", func(name, args string) {
			b.mu.Lock()
			b.calls = append(b.calls, name+" "+args)
			b.mu.Unlock()
		})
		_ = vm.Set("__err", func(s string) {
			b.mu.Lock()
			b.errs = append(b.errs, s)
			b.mu.Unlock()
		})
		if _, err := vm.RunString(strings.Replace(harness, "RUNTIME", string(runtimeJS), 1)); err != nil {
			done <- err
			return
		}
		load, _ := goja.AssertFunction(vm.Get("load"))
		var assetsObj any
		_ = json.Unmarshal(assets, &assetsObj)
		rt, err := load(goja.Undefined(), vm.ToValue(code), vm.ToValue(assetsObj))
		b.rt = rt
		done <- err
	})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	return b
}

// call 调运行时的方法（setStore / dispatch / swipe）
func (b *band) call(method string, args ...any) {
	b.loop.RunOnLoop(func(vm *goja.Runtime) {
		fn, _ := goja.AssertFunction(b.rt.ToObject(vm).Get(method))
		vals := make([]goja.Value, len(args))
		for i, a := range args {
			vals[i] = vm.ToValue(a)
		}
		if _, err := fn(b.rt, vals...); err != nil {
			b.t.Error(err)
		}
	})
}

func (b *band) setStore(name string, v any) {
	raw, _ := json.Marshal(v)
	var x any
	_ = json.Unmarshal(raw, &x)
	b.call("setStore", name, x)
}

// waitTree 等一棵满足 ok 的树
func (b *band) waitTree(ok func(tree map[string]any) bool) map[string]any {
	b.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		for i := len(b.trees) - 1; i >= 0; i-- {
			if b.trees[i] != nil && ok(b.trees[i]) {
				m := b.trees[i]
				b.mu.Unlock()
				return m
			}
		}
		errs := b.errs
		b.mu.Unlock()
		if len(errs) > 0 {
			b.t.Fatalf("插件出错：%v", errs)
		}
		time.Sleep(10 * time.Millisecond)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var last []string
	if n := len(b.trees); n > 0 {
		last = texts(b.trees[n-1])
	}
	b.t.Fatalf("等不到想要的树，最后一棵的文字：%v", last)
	return nil
}

// texts 收集树里所有文字
func texts(n map[string]any) []string {
	var out []string
	if x, ok := n["x"].(string); ok {
		out = append(out, x)
	}
	if c, ok := n["c"].([]any); ok {
		for _, x := range c {
			out = append(out, texts(x.(map[string]any))...)
		}
	}
	return out
}

func hasText(tree map[string]any, s string) bool {
	return strings.Contains(strings.Join(texts(tree), "|"), s)
}

func find(n map[string]any, pred func(map[string]any) bool) map[string]any {
	if pred(n) {
		return n
	}
	if c, ok := n["c"].([]any); ok {
		for _, x := range c {
			if r := find(x.(map[string]any), pred); r != nil {
				return r
			}
		}
	}
	return nil
}

// builtinBand 编译一个内置插件，放进模拟的手环里跑
func builtinBand(t *testing.T, name string) *band {
	t.Helper()
	sub, err := fs.Sub(plugins.FS, name)
	if err != nil {
		t.Fatal(err)
	}
	return sourceBand(t, sub)
}

func sourceBand(t *testing.T, fsys fs.FS) *band {
	t.Helper()
	blobs := map[string]blob{}
	p, err := compile(fsys, "", true, blobs)
	if err != nil || p.Err != "" {
		t.Fatalf("编译失败：%v %s", err, p.Err)
	}
	return newBand(t, p, string(blobs[p.Code].data))
}

func agentList(statuses ...string) []map[string]any {
	names := []string{"api", "web", "cli", "doc"}
	var out []map[string]any
	for i, s := range statuses {
		out = append(out, map[string]any{"id": names[i], "agent": "claude", "project": names[i], "title": "任务" + names[i], "status": s})
	}
	return out
}

// ---------- 插件 ----------

func TestAgentCat(t *testing.T) {
	b := builtinBand(t, "agent-cat")
	b.setStore("settings", map[string]any{"pet": "gray"})
	b.setStore("agents", agentList("working", "idle"))
	tree := b.waitTree(func(m map[string]any) bool { return hasText(m, "1 个在忙") })
	sprite := find(tree, func(n map[string]any) bool { return n["t"] == "a" })
	if sprite == nil {
		t.Fatal("没有动画节点")
	}
	if f := sprite["f"].([]any); len(f) != 6 || !strings.HasPrefix(f[0].(map[string]any)["src"].(string), "internal://files/") {
		t.Fatalf("动画帧不对：%v", f)
	}
	raw, _ := json.Marshal(tree)
	t.Logf("猫页面的树 %d 字节", len(raw))

	// 跑完一个 → 庆祝
	b.setStore("agents", agentList("done", "idle"))
	b.waitTree(func(m map[string]any) bool { return hasText(m, "搞定一个！") })

	// 卡住 → 举牌；点气泡在手环本地换下一个
	b.setStore("agents", agentList("blocked", "working"))
	tree = b.waitTree(func(m map[string]any) bool { return hasText(m, "1 个等你处理！") && hasText(m, "1/2") })
	bubble := find(tree, func(n map[string]any) bool {
		return n["e"] != nil && strings.Contains(n["s"].(string), "border-left-width")
	})
	if bubble == nil {
		t.Fatal("找不到气泡")
	}
	if !strings.Contains(bubble["s"].(string), "padding-left:10px") {
		t.Fatalf("气泡的 padding 没展开：%s", bubble["s"])
	}
	b.call("dispatch", bubble["i"], "click")
	b.waitTree(func(m map[string]any) bool { return hasText(m, "2/2") && hasText(m, "任务web") })
}

func TestAgentCatFishAndImpatient(t *testing.T) {
	b := builtinBand(t, "agent-cat")
	b.setStore("settings", map[string]any{"pet": "orange", "impatientMin": 5})
	b.setStore("stats", map[string]any{"fishLeft": 2})
	b.setStore("usage", []map[string]any{
		{"agent": "codex", "windows": []map[string]any{{"name": "7d", "used": 20, "resetsAt": 0}}},
		{"agent": "claude", "windows": []map[string]any{{"name": "5h", "used": 63.4, "resetsAt": 0}}},
	})
	// 卡了 7 分钟：着急
	since := time.Now().Add(-7 * time.Minute).UnixMilli()
	b.setStore("agents", []map[string]any{{"id": "p1", "agent": "claude", "project": "api", "title": "修登录", "status": "blocked", "since": since}})
	tree := b.waitTree(func(m map[string]any) bool { return hasText(m, "api 等了 7 分钟！") })
	if !hasText(tree, "Claude 5h 63%") {
		t.Fatalf("应该显示最紧的额度：%v", texts(tree))
	}
	sprite := find(tree, func(n map[string]any) bool { return n["t"] == "a" })
	if sprite["d"] != "300ms" {
		t.Fatalf("着急时举牌要更快：%v", sprite["d"])
	}

	// 点小鱼干喂猫：告诉 Mac，猫开心，数量先在手环上减掉
	fish := find(tree, func(n map[string]any) bool {
		return n["e"] != nil && find(n, func(c map[string]any) bool { return c["t"] == "m" }) != nil
	})
	if fish == nil {
		t.Fatal("找不到小鱼干按钮")
	}
	b.call("dispatch", fish["i"], "click")
	b.waitTree(func(m map[string]any) bool { return hasText(m, "好吃！还剩 1 条") })
	b.mu.Lock()
	calls := strings.Join(b.calls, ";")
	b.mu.Unlock()
	if calls != "feed []" {
		t.Fatalf("应该调用 Mac 的 feed：%q", calls)
	}
	// Mac 记完账推回新的数
	b.setStore("stats", map[string]any{"fishLeft": 1})
	b.waitTree(func(m map[string]any) bool {
		f := find(m, func(n map[string]any) bool {
			return n["e"] != nil && find(n, func(c map[string]any) bool { return c["t"] == "m" }) != nil
		})
		return f != nil && hasText(f, "1")
	})
}

func TestToday(t *testing.T) {
	b := builtinBand(t, "today")
	days := []map[string]any{}
	for i, d := range []int{3, 0, 5, 8, 1, 0, 12} {
		days = append(days, map[string]any{"date": fmt.Sprintf("10-%02d", 16+i), "weekday": "一二三四五六日"[i*3 : i*3+3], "done": d})
	}
	b.setStore("stats", map[string]any{"today": map[string]any{"done": 12, "fed": 4}, "fishLeft": 9, "fedTotal": 30, "days": days})
	b.setStore("usage", []map[string]any{{"agent": "claude", "windows": []map[string]any{{"name": "5h", "used": 85, "resetsAt": 0}}}})
	tree := b.waitTree(func(m map[string]any) bool { return hasText(m, "12 条小鱼干") })
	for _, want := range []string{"完成 12 个任务 · 喂了 4 条", "还剩 9 条 · 一共喂了 30 条", "Claude 5h", "85%"} {
		if !hasText(tree, want) {
			t.Errorf("没有 %q：%v", want, texts(tree))
		}
	}
	raw, _ := json.Marshal(tree)
	t.Logf("今日页面的树 %d 字节", len(raw))
}

func TestAgentsPaging(t *testing.T) {
	b := builtinBand(t, "agents")
	var list []map[string]any
	for i := range 10 {
		list = append(list, map[string]any{"id": string(rune('a' + i)), "agent": "claude", "project": "proj" + string(rune('a'+i)), "title": "t", "status": "idle"})
	}
	b.setStore("agents", list)
	b.waitTree(func(m map[string]any) bool { return hasText(m, "1/3") && hasText(m, "proja") })
	b.call("swipe", "up")
	tree := b.waitTree(func(m map[string]any) bool { return hasText(m, "2/3") })
	if !hasText(tree, "proje") || hasText(tree, "proja") {
		t.Fatalf("第二页内容不对：%v", texts(tree))
	}
}

func TestStyles(t *testing.T) {
	b := sourceBand(t, fstest.MapFS{
		"manifest.json": {Data: []byte(`{"name": "box"}`)},
		"index.jsx": {Data: []byte(`import { View, Row, Text } from 'miband'
export default function () {
  return <View style={{ padding: [7, 10] }}>
    <Text style={[{ paddingLeft: 3 }, { margin: 4 }, 'padding:1px 2px 3px']}>a</Text>
    <Text style={{ margin: [0, 6], marginLeft: 9 }}>b</Text>
    <Row gap={5}><Text>c</Text><Text>d</Text></Row>
  </View>
}`)},
	})
	tree := b.waitTree(func(m map[string]any) bool { return hasText(m, "d") })
	style := func(x string) string {
		return find(tree, func(n map[string]any) bool { return n["x"] == x })["s"].(string)
	}
	for _, c := range []struct{ got, want string }{
		{tree["s"].(string), "padding-top:7px;padding-right:10px;padding-bottom:7px;padding-left:10px"},
		{style("a"), "margin:4px;padding-top:1px;padding-right:2px;padding-bottom:3px;padding-left:2px"},
		{style("b"), "margin-left:9px;margin-top:0px;margin-right:6px;margin-bottom:0px"},
		{style("d"), "margin-left:5px"},
	} {
		if !strings.Contains(c.got, c.want) {
			t.Errorf("样式 %q 里没有 %q", c.got, c.want)
		}
	}
}

func TestBuildRejectsOtherImports(t *testing.T) {
	fsys := fstest.MapFS{"index.jsx": {Data: []byte(`import fs from 'fs'; export default () => null`)}}
	if _, err := build(fsys, "index.jsx"); err == nil || !strings.Contains(err.Error(), "只能 import") {
		t.Fatalf("应该拒绝 import 'fs'：%v", err)
	}
}

// ---------- Mac 和手环之间的协议 ----------

type fakeBand struct {
	mu   sync.Mutex
	msgs []map[string]any
}

func (f *fakeBand) send(v any) error {
	raw, _ := json.Marshal(v)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	f.mu.Lock()
	f.msgs = append(f.msgs, m)
	f.mu.Unlock()
	return nil
}

func (f *fakeBand) wait(t *testing.T, ok func(m map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		msgs := append([]map[string]any{}, f.msgs...)
		f.mu.Unlock()
		for _, m := range msgs {
			if ok(m) {
				return m
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("等不到想要的消息")
	return nil
}

func (f *fakeBand) all(typ string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []map[string]any
	for _, m := range f.msgs {
		if m["t"] == typ {
			out = append(out, m)
		}
	}
	return out
}

func startHost(t *testing.T, builtin fs.FS, userDir string) (*Host, *fakeBand) {
	t.Helper()
	f := &fakeBand{}
	h := NewHost()
	h.Send = f.send
	h.Load(builtin, userDir)
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	t.Cleanup(cancel)
	return h, f
}

func TestProtocol(t *testing.T) {
	h, f := startHost(t, plugins.FS, "")
	h.SetStore("agents", agentList("working"))
	time.Sleep(50 * time.Millisecond)
	if len(f.all("plugins")) != 0 {
		t.Fatal("渲染器不在前台时不该发")
	}
	h.HandleMessage(map[string]any{"t": "hello", "w": 212.0, "h": 303.0, "have": []any{}})
	idx := f.wait(t, func(m map[string]any) bool { return m["t"] == "plugins" })
	list := idx["list"].([]any)
	var names []string
	for _, p := range list {
		names = append(names, p.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "agent-cat,today,agents" {
		t.Fatalf("插件列表不对：%v", names)
	}
	cat := list[0].(map[string]any)
	if n := len(cat["assets"].(map[string]any)); n != 79 {
		t.Fatalf("猫应该有 78 张帧加一张小鱼干，实际 %d", n)
	}
	raw, _ := json.Marshal(idx)
	t.Logf("插件列表 %d 字节", len(raw))
	f.wait(t, func(m map[string]any) bool { return m["t"] == "store" && m["name"] == "agents" })

	// 手环要代码：分片发过来，拼起来是编译好的 JS
	code := cat["code"].(string)
	h.HandleMessage(map[string]any{"t": "need", "h": []any{code}})
	f.wait(t, func(m map[string]any) bool {
		return m["t"] == "blob" && m["h"] == code && m["k"].(float64) == m["n"].(float64)-1
	})
	var b64 strings.Builder
	for _, m := range f.all("blob") {
		if m["h"] == code {
			b64.WriteString(m["b"].(string))
			if m["x"] != "js" {
				t.Fatalf("代码的扩展名不对：%v", m["x"])
			}
		}
	}
	js, err := base64.StdEncoding.DecodeString(b64.String())
	if err != nil || !strings.Contains(string(js), "__h(") {
		t.Fatalf("代码不对：%v %.80s", err, js)
	}
	t.Logf("猫插件的代码 %d 字节", len(js))
	for _, m := range f.all("blob") {
		if raw, _ := json.Marshal(m); len(raw) > 15000 {
			t.Fatalf("单条消息太大：%d 字节", len(raw))
		}
	}

	// 数据没变不重发；变了才发
	n := len(f.all("store"))
	h.SetStore("agents", agentList("working"))
	h.SetStore("agents", agentList("done"))
	f.wait(t, func(m map[string]any) bool { return len(f.all("store")) == n+1 })

	h.ShowFirst()
	f.wait(t, func(m map[string]any) bool { return m["t"] == "show" && m["name"] == "agent-cat" })
}

func TestUserPluginErrorsAndReload(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) {
		p := filepath.Join(dir, "hello", name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("manifest.json", `{"name": "hello", "title": "你好"}`)
	write("index.jsx", `export default function () { return <Text>没闭合 }`)
	h, f := startHost(t, nil, dir)
	h.HandleMessage(map[string]any{"t": "hello", "have": []any{}})
	f.wait(t, func(m map[string]any) bool {
		if m["t"] != "plugins" {
			return false
		}
		e, _ := m["list"].([]any)[0].(map[string]any)["err"].(string)
		return strings.Contains(e, "编译失败")
	})
	if info := h.Plugins(); len(info) != 1 || !strings.Contains(info[0].Error, "编译失败") {
		t.Fatalf("界面上应该看到编译错误：%v", info)
	}

	// 改好后自动重新编译，发新的插件列表
	write("index.jsx", `import { Text } from 'miband'
export default function () { return <Text>好了</Text> }`)
	f.wait(t, func(m map[string]any) bool {
		if m["t"] != "plugins" {
			return false
		}
		p := m["list"].([]any)[0].(map[string]any)
		return p["err"] == nil && p["code"] != ""
	})

	// 手环上报的运行错误显示在界面上
	h.HandleMessage(map[string]any{"t": "log", "p": "hello", "level": "error", "msg": "渲染出错：boom"})
	if info := h.Plugins(); info[0].Error != "渲染出错：boom" {
		t.Fatalf("运行错误没记下来：%v", info)
	}
}
