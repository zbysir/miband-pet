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
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
)

// mac.js 在 Mac 上跑（goja，每个插件一个运行时、一个事件循环），import ... from 'miband/mac' 拿到下面 macAPI 里的东西。
// 它和手环上的插件配合：手环上 call(name, ...args) 交给这里 on(name, fn) 注册的函数；
// 这里 publish(name, value)，手环上的插件 useStore(name) 就能读到（只有本插件读得到）。
const macAPI = `
var __api = {
  on: function (name, fn) { __mac.on(name, fn) },
  publish: function (name, value) { __mac.publish(name, JSON.stringify(value === undefined ? null : value)) },
  getStore: function (name) { var s = __mac.getStore(name); return s ? JSON.parse(s) : undefined },
  onStore: function (name, fn) { __mac.onStore(name, function (s) { fn(JSON.parse(s)) }) },
  getConfig: function () { var s = __mac.getStore(__mac.name + ':config'); return s ? JSON.parse(s) : {} },
  onConfig: function (fn) { __mac.onStore(__mac.name + ':config', function (s) { fn(JSON.parse(s)) }) },
  notifyMac: function (title, body) { __mac.notifyMac(String(title), body == null ? '' : String(body)) },
  notifyBand: function (title, body) { __mac.notifyBand(String(title), body == null ? '' : String(body)) },
  readFile: function (path, opts) { return __mac.readFile(String(path), (opts && opts.tail) || 0) },
  readDir: function (path) { return JSON.parse(__mac.readDir(String(path))) },
  stat: function (path) { var s = __mac.stat(String(path)); return s ? JSON.parse(s) : null },
  exec: function (cmd, args) { return JSON.parse(__mac.exec(String(cmd), JSON.stringify(args || []))) },
  storage: {
    get: function (key) { var s = __mac.storageGet(String(key)); return s ? JSON.parse(s) : undefined },
    set: function (key, value) { __mac.storageSet(String(key), JSON.stringify(value === undefined ? null : value)) }
  },
  home: __mac.home
}
var console = {
  log: function () { __mac.log('info', Array.prototype.map.call(arguments, String).join(' ')) },
  warn: function () { __mac.log('warn', Array.prototype.map.call(arguments, String).join(' ')) },
  error: function () { __mac.log('error', Array.prototype.map.call(arguments, String).join(' ')) }
}
`

type macRuntime struct {
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

func startMac(h *Host, p *Plugin) *macRuntime {
	m := &macRuntime{h: h, p: p, loop: eventloop.NewEventLoop(), handlers: map[string]goja.Callable{}, storeSubs: map[string][]goja.Callable{}}
	m.loop.Start()
	m.run(func(vm *goja.Runtime) {
		m.vm = vm
		if err := m.setup(vm); err != nil {
			m.fail(err.Error())
		}
	})
	return m
}

func (m *macRuntime) stop() {
	if m.vm != nil {
		m.vm.Interrupt("插件已停止")
	}
	m.loop.StopNoWait()
}

// run 在事件循环上执行；死循环之类的跑太久会被打断
func (m *macRuntime) run(fn func(vm *goja.Runtime)) {
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

func (m *macRuntime) fail(msg string) {
	slog.Warn("插件 mac.js 出错", "plugin", m.p.Name, "err", msg)
	m.h.mu.Lock()
	m.h.macErr[m.p.Name] = "插件出错：" + msg
	m.h.mu.Unlock()
	m.h.changed()
}

// throw 让 Go 函数在 JS 里抛异常
func (m *macRuntime) throw(err error) {
	panic(m.vm.NewGoError(err))
}

func (m *macRuntime) setup(vm *goja.Runtime) error {
	mac := vm.NewObject()
	set := func(name string, fn any) { _ = mac.Set(name, fn) }
	perms := m.p.Permissions
	set("on", func(name string, fn goja.Callable) { m.handlers[name] = fn })
	set("publish", func(name, value string) {
		m.h.SetStore(m.p.Name+":"+name, json.RawMessage(value))
	})
	set("getStore", func(name string) string { return m.h.storeJSON(name) })
	set("onStore", func(name string, fn goja.Callable) { m.storeSubs[name] = append(m.storeSubs[name], fn) })
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
		if !perms.Exec {
			m.throw(errors.New("没有执行命令的权限（manifest.json 里 permissions.exec）"))
		}
		var args []string
		_ = json.Unmarshal([]byte(argsJSON), &args)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, cmd, args...)
		var stdout, stderr bytes.Buffer
		c.Stdout, c.Stderr = &stdout, &stderr
		err := c.Run()
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
			slog.Warn("插件 mac.js", "plugin", m.p.Name, "msg", msg)
		default:
			slog.Info("插件 mac.js", "plugin", m.p.Name, "msg", msg)
		}
	})
	home, _ := os.UserHomeDir()
	set("home", home)
	set("name", m.p.Name)
	if err := vm.Set("__mac", mac); err != nil {
		return err
	}
	if _, err := vm.RunScript("miband/mac", macAPI); err != nil {
		return err
	}
	fn, err := vm.RunScript(m.p.Name+"/mac.js", "(function (module, exports, require) {\n"+m.p.MacCode+"\n})")
	if err != nil {
		return err
	}
	call, _ := goja.AssertFunction(fn)
	module := vm.NewObject()
	exports := vm.NewObject()
	_ = module.Set("exports", exports)
	require := func(name string) goja.Value {
		if name == "miband/mac" {
			return vm.Get("__api")
		}
		panic(vm.NewTypeError("mac.js 里只能 require('miband/mac')，不支持 " + name))
	}
	_, err = call(goja.Undefined(), module, exports, vm.ToValue(require))
	return err
}

// call 执行 on(name) 注册的函数；没注册返回 false
func (m *macRuntime) call(name string, args []any) bool {
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
func (m *macRuntime) storeChanged(name, raw string) {
	m.run(func(vm *goja.Runtime) {
		for _, fn := range m.storeSubs[name] {
			if _, err := fn(goja.Undefined(), vm.ToValue(raw)); err != nil {
				m.fail(fmt.Sprintf("onStore(%s) 出错：%v", name, err))
			}
		}
	})
}

// allowed：path 在 permissions.read 允许的目录里才能读（跟着符号链接走到最终位置再判断）
func (m *macRuntime) allowed(path string) (string, error) {
	p, err := filepath.Abs(expandHome(path))
	if err != nil {
		return "", err
	}
	real := p
	if r, err := filepath.EvalSymlinks(p); err == nil {
		real = r
	}
	for _, root := range m.p.Permissions.Read {
		r, err := filepath.Abs(expandHome(root))
		if err != nil {
			continue
		}
		if rr, err := filepath.EvalSymlinks(r); err == nil {
			r = rr
		}
		if real == r || strings.HasPrefix(real, r+string(filepath.Separator)) {
			return p, nil
		}
	}
	return "", fmt.Errorf("没有读 %s 的权限（manifest.json 里 permissions.read）", path)
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

func (m *macRuntime) dataPath() string {
	if m.h.DataDir == "" {
		return ""
	}
	return filepath.Join(m.h.DataDir, m.p.Name+".json")
}

func (m *macRuntime) loadData() {
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

func (m *macRuntime) storageGet(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadData()
	return string(m.data[key])
}

func (m *macRuntime) storageSet(key, value string) {
	m.mu.Lock()
	m.loadData()
	m.data[key] = json.RawMessage(value)
	raw, _ := json.MarshalIndent(m.data, "", "  ")
	m.mu.Unlock()
	if f := m.dataPath(); f != "" {
		_ = os.MkdirAll(filepath.Dir(f), 0o700)
		if err := os.WriteFile(f, raw, 0o600); err != nil {
			slog.Warn("mac.js 存数据失败", "plugin", m.p.Name, "err", err)
		}
	}
}
