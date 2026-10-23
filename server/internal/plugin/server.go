package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
)

// server.js 在 Mac 上跑（goja，每个插件一个运行时、一个事件循环），import ... from 'miband/server' 拿到下面 serverAPI 里的东西。
// 它和手环上的插件配合：手环上 call(name, ...args) 交给这里 on(name, fn) 注册的函数；
// 这里 publish(name, value)，手环上的插件 useStore(name) 就能读到（只有本插件读得到）。
// 插件之间不共用数据：getStore / onStore 只能读本插件推的和基座给的（BaseStores）。
const serverAPI = `
var __api = {
  on: function (name, fn) { __server.on(name, fn) },
  publish: function (name, value) { __server.publish(name, JSON.stringify(value === undefined ? null : value)) },
  getStore: function (name) { var s = __server.getStore(name); return s ? JSON.parse(s) : undefined },
  onStore: function (name, fn) { __server.onStore(name, function (s) { fn(JSON.parse(s)) }) },
  getConfig: function () { var s = __server.getStore('config'); return s ? JSON.parse(s) : {} },
  onConfig: function (fn) { __server.onStore('config', function (s) { fn(JSON.parse(s)) }) },
  wakeBand: function () { __server.wakeBand() },
  notifyMac: function (title, body) { __server.notifyMac(String(title), body == null ? '' : String(body)) },
  notifyBand: function (title, body) { __server.notifyBand(String(title), body == null ? '' : String(body)) },
  readFile: function (path, opts) { return __server.readFile(String(path), (opts && opts.tail) || 0) },
  readDir: function (path) { return JSON.parse(__server.readDir(String(path))) },
  stat: function (path) { var s = __server.stat(String(path)); return s ? JSON.parse(s) : null },
  exec: function (cmd, args) { return JSON.parse(__server.exec(String(cmd), JSON.stringify(args || []))) },
  storage: {
    get: function (key) { var s = __server.storageGet(String(key)); return s ? JSON.parse(s) : undefined },
    set: function (key, value) { __server.storageSet(String(key), JSON.stringify(value === undefined ? null : value)) }
  },
  home: __server.home
}
var console = {
  log: function () { __server.log('info', Array.prototype.map.call(arguments, String).join(' ')) },
  warn: function () { __server.log('warn', Array.prototype.map.call(arguments, String).join(' ')) },
  error: function () { __server.log('error', Array.prototype.map.call(arguments, String).join(' ')) }
}
`

type serverRuntime struct {
	h    *Host
	p    *Plugin
	loop *eventloop.EventLoop

	// 只在事件循环上访问
	vm        *goja.Runtime
	handlers  map[string]goja.Callable
	storeSubs map[string][]goja.Callable

	mu   sync.Mutex
	data map[string]json.RawMessage // storage
}

func startServer(h *Host, p *Plugin) *serverRuntime {
	m := &serverRuntime{h: h, p: p, loop: eventloop.NewEventLoop(), handlers: map[string]goja.Callable{}, storeSubs: map[string][]goja.Callable{}}
	m.loop.Start()
	m.run(func(vm *goja.Runtime) {
		m.vm = vm
		if err := m.setup(vm); err != nil {
			m.fail(err.Error())
		}
	})
	return m
}

func (m *serverRuntime) stop() {
	if m.vm != nil {
		m.vm.Interrupt("插件已停止")
	}
	m.loop.StopNoWait()
}

// run 在事件循环上执行；死循环之类的跑太久会被打断
func (m *serverRuntime) run(fn func(vm *goja.Runtime)) {
	m.loop.RunOnLoop(func(vm *goja.Runtime) {
		t := time.AfterFunc(30*time.Second, func() { vm.Interrupt("插件执行超过 30 秒，被打断") })
		defer t.Stop()
		defer func() {
			if r := recover(); r != nil {
				m.fail(fmt.Sprint(r))
			}
		}()
		fn(vm)
		vm.ClearInterrupt()
	})
}

func (m *serverRuntime) fail(msg string) {
	slog.Warn("插件 server.js 出错", "plugin", m.p.Name, "err", msg)
	m.h.mu.Lock()
	m.h.serverErr[m.p.Name] = "插件出错：" + msg
	m.h.mu.Unlock()
	m.h.changed()
}

// throw 让 Go 函数在 JS 里抛异常
func (m *serverRuntime) throw(err error) {
	panic(m.vm.NewGoError(err))
}

func (m *serverRuntime) setup(vm *goja.Runtime) error {
	api := vm.NewObject()
	set := func(name string, fn any) { _ = api.Set(name, fn) }
	perms := m.p.Permissions
	set("on", func(name string, fn goja.Callable) { m.handlers[name] = fn })
	set("publish", func(name, value string) {
		m.h.SetStore(m.p.Name+":"+name, json.RawMessage(value))
	})
	set("getStore", func(name string) string { return m.h.storeJSON(m.storeName(name)) })
	set("onStore", func(name string, fn goja.Callable) {
		key := m.storeName(name)
		m.storeSubs[key] = append(m.storeSubs[key], fn)
	})
	// 把手环点亮并显示本插件（把手环上的 wristkit 打开到前台，亮屏不震动）
	set("wakeBand", func() {
		if !perms.Notify {
			m.throw(errors.New("没有提醒的权限（manifest.json 里 permissions.notify）"))
		}
		if m.h.Wake == nil {
			m.throw(errors.New("手环没连上"))
		}
		if err := m.h.Wake(m.p.Name); err != nil {
			m.throw(err)
		}
	})
	set("notifyMac", func(title, body string) {
		if !perms.Notify {
			m.throw(errors.New("没有发通知的权限（manifest.json 里 permissions.notify）"))
		}
		script := fmt.Sprintf("display notification %s with title %s", appleScriptString(body), appleScriptString(title))
		if out, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
			m.throw(fmt.Errorf("发通知失败：%v %s", err, out))
		}
	})
	set("notifyBand", func(title, body string) {
		if !perms.Notify {
			m.throw(errors.New("没有发通知的权限（manifest.json 里 permissions.notify）"))
		}
		if m.h.NotifyBand == nil {
			m.throw(errors.New("手环没连上"))
		}
		if err := m.h.NotifyBand(title, body); err != nil {
			m.throw(err)
		}
	})
	// readFile(path, {tail: n})：只读最后 n 字节（大日志文件用）
	set("readFile", func(path string, tail int64) string {
		p, err := m.allowed(path)
		if err != nil {
			m.throw(err)
		}
		raw, err := readTail(p, tail)
		if err != nil {
			m.throw(err)
		}
		return string(raw)
	})
	set("readDir", func(path string) string {
		p, err := m.allowed(path)
		if err != nil {
			m.throw(err)
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			m.throw(err)
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		raw, _ := json.Marshal(names)
		return string(raw)
	})
	set("stat", func(path string) string {
		p, err := m.allowed(path)
		if err != nil {
			m.throw(err)
		}
		st, err := os.Stat(p)
		if err != nil {
			return ""
		}
		raw, _ := json.Marshal(map[string]any{"size": st.Size(), "mtime": st.ModTime().UnixMilli(), "isDir": st.IsDir()})
		return string(raw)
	})
	set("exec", func(cmd, argsJSON string) string {
		if !slices.Contains(perms.Exec, cmd) {
			m.throw(fmt.Errorf("没有运行 %s 的权限（manifest.json 里 permissions.exec）", cmd))
		}
		bin, err := lookCommand(cmd)
		if err != nil {
			m.throw(err)
		}
		var args []string
		_ = json.Unmarshal([]byte(argsJSON), &args)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, bin, args...)
		var stdout, stderr bytes.Buffer
		c.Stdout, c.Stderr = &stdout, &stderr
		err = c.Run()
		code := 0
		if err != nil {
			var ee *exec.ExitError
			if !errors.As(err, &ee) {
				m.throw(err)
			}
			code = ee.ExitCode()
		}
		raw, _ := json.Marshal(map[string]any{"code": code, "stdout": stdout.String(), "stderr": stderr.String()})
		return string(raw)
	})
	set("storageGet", func(key string) string { return m.storageGet(key) })
	set("storageSet", func(key, value string) { m.storageSet(key, value) })
	set("log", func(level, msg string) {
		switch level {
		case "warn", "error":
			slog.Warn("插件 server.js", "plugin", m.p.Name, "msg", msg)
		default:
			slog.Info("插件 server.js", "plugin", m.p.Name, "msg", msg)
		}
	})
	home, _ := os.UserHomeDir()
	set("home", home)
	set("name", m.p.Name)
	if err := vm.Set("__server", api); err != nil {
		return err
	}
	if _, err := vm.RunScript("miband/server", serverAPI); err != nil {
		return err
	}
	fn, err := vm.RunScript(m.p.Name+"/server.js", "(function (module, exports, require) {\n"+m.p.ServerCode+"\n})")
	if err != nil {
		return err
	}
	call, _ := goja.AssertFunction(fn)
	module := vm.NewObject()
	exports := vm.NewObject()
	_ = module.Set("exports", exports)
	require := func(name string) goja.Value {
		if name == "miband/server" {
			return vm.Get("__api")
		}
		panic(vm.NewTypeError("server.js 里只能 require('miband/server')，不支持 " + name))
	}
	_, err = call(goja.Undefined(), module, exports, vm.ToValue(require))
	return err
}

// call 执行 on(name) 注册的函数；没注册返回 false
func (m *serverRuntime) call(name string, args []any) bool {
	done := make(chan bool, 1)
	m.run(func(vm *goja.Runtime) {
		fn := m.handlers[name]
		if fn == nil {
			done <- false
			return
		}
		done <- true
		vals := make([]goja.Value, len(args))
		for i, a := range args {
			vals[i] = vm.ToValue(a)
		}
		if _, err := fn(goja.Undefined(), vals...); err != nil {
			m.fail(fmt.Sprintf("处理 %s 出错：%v", name, err))
		}
	})
	select {
	case ok := <-done:
		return ok
	case <-time.After(5 * time.Second):
		return true // 事件循环忙，当成已经交出去了
	}
}

// storeChanged：Mac 上的数据变了，通知 onStore 订阅了它的函数
func (m *serverRuntime) storeChanged(name, raw string) {
	m.run(func(vm *goja.Runtime) {
		for _, fn := range m.storeSubs[name] {
			if _, err := fn(goja.Undefined(), vm.ToValue(raw)); err != nil {
				m.fail(fmt.Sprintf("onStore(%s) 出错：%v", name, err))
			}
		}
	})
}

// allowed：path 在 permissions.read 允许的目录里才能读（跟着符号链接走到最终位置再判断）
func (m *serverRuntime) allowed(path string) (string, error) {
	p, err := filepath.Abs(expandHome(path))
	if err != nil {
		return "", err
	}
	real := resolve(p)
	for _, root := range m.p.Permissions.Read {
		r, err := filepath.Abs(expandHome(root))
		if err != nil {
			continue
		}
		r = resolve(r)
		if real == r || strings.HasPrefix(real, r+string(filepath.Separator)) {
			return p, nil
		}
	}
	return "", fmt.Errorf("没有读 %s 的权限（manifest.json 里 permissions.read）", path)
}

// resolve 解开符号链接；文件还不存在时解开它最近的存在的上级目录（不然和解开了的允许目录对不上，stat 一个还没生成的文件也会报没权限）
func resolve(p string) string {
	rest := ""
	for dir := p; ; dir = filepath.Dir(dir) {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest)
		}
		if filepath.Dir(dir) == dir {
			return p
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}

func readTail(path string, tail int64) ([]byte, error) {
	if tail <= 0 {
		return os.ReadFile(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := max(st.Size()-tail, 0)
	buf := make([]byte, st.Size()-start)
	n, err := f.ReadAt(buf, start)
	if errors.Is(err, io.EOF) && n == len(buf) {
		err = nil
	}
	return buf, err
}

// storeName：插件读数据时的名字。基座给的（BaseStores）直接读；其它的都是本插件 publish 的
func (m *serverRuntime) storeName(name string) string {
	if slices.Contains(BaseStores, name) {
		return name
	}
	return m.p.Name + ":" + name
}

// lookCommand 找命令：从访达打开的 App 拿到的 PATH 很短（没有 ~/.local/bin、Homebrew），
// 所以 PATH 里找不到时再看几个常见的安装位置
func lookCommand(name string) (string, error) {
	if strings.ContainsRune(name, '/') {
		return "", fmt.Errorf("命令只能写名字，不能带路径：%s", name)
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, dir := range []string{filepath.Join(home, ".local/bin"), "/opt/homebrew/bin", "/usr/local/bin", filepath.Join(home, "go/bin"), filepath.Join(home, ".cargo/bin")} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("没找到命令 %s", name)
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}

func appleScriptString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// ---------- storage：每个插件一个 JSON 文件 ----------

func (m *serverRuntime) dataPath() string {
	if m.h.DataDir == "" {
		return ""
	}
	return filepath.Join(m.h.DataDir, m.p.Name+".json")
}

func (m *serverRuntime) loadData() {
	if m.data != nil {
		return
	}
	m.data = map[string]json.RawMessage{}
	if f := m.dataPath(); f != "" {
		if raw, err := os.ReadFile(f); err == nil {
			_ = json.Unmarshal(raw, &m.data)
		}
	}
}

func (m *serverRuntime) storageGet(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadData()
	return string(m.data[key])
}

func (m *serverRuntime) storageSet(key, value string) {
	m.mu.Lock()
	m.loadData()
	m.data[key] = json.RawMessage(value)
	raw, _ := json.MarshalIndent(m.data, "", "  ")
	m.mu.Unlock()
	if f := m.dataPath(); f != "" {
		_ = os.MkdirAll(filepath.Dir(f), 0o700)
		if err := os.WriteFile(f, raw, 0o600); err != nil {
			slog.Warn("server.js 存数据失败", "plugin", m.p.Name, "err", err)
		}
	}
}
