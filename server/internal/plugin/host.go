package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// BaseStores 是基座给插件的数据，所有插件都能读；其它数据只有推它的插件自己能读（插件之间不共用数据）。
//   - screen：手环屏幕大小（手环上的页面给的）
//   - band：手环的状态 {connected, foreground}（foreground 是手环上 wristkit 是否开在前台）
var BaseStores = []string{"screen", "band"}

// 单条消息不能太大：64KB 会让手环不再确认数据帧，连接卡死（docs/render-spike.md）。
// 代码和图片按 base64 分片发，每片不超过这么多字符。
const chunkChars = 6000

type blob struct {
	ext  string // js / png
	data []byte
}

// Host 管理所有插件，把代码、图片、数据推给手环上的渲染器。
type Host struct {
	// Send 发一条消息给手环上的渲染器。由使用方设置，会阻塞到发完。
	Send func(v any) error
	// OnChange 在插件列表、出错状态、当前页变化时调用（给界面刷新用），不要阻塞。
	OnChange func()
	// Calls 是手环上的插件能让 Mac 做的事（插件里 call(name, ...args)），按名字注册。结果不回传：
	// 要让插件看到变化，就更新对应的 store。插件自己的 server.js 用 on(name) 注册的优先。
	Calls map[string]func(args []any) (any, error)
	// DataDir 存 server.js 的 storage 数据；为空则不持久化。
	DataDir string
	// NotifyBand 给手环发通知（server.js 的 notifyBand）。
	NotifyBand func(title, body string) error
	// Wake 把手环点亮并显示这个插件（server.js 的 wakeBand）。
	Wake func(plugin string) error
	// OnStore 在任何数据变化时调用（App 的预览用），不要阻塞。
	OnStore func(name string)

	mu         sync.Mutex
	builtin    fs.FS
	userDir    string
	userSig    string
	plugins    []*Plugin
	blobs      map[string]blob
	stores     map[string]json.RawMessage
	runErr     map[string]string // 手环上报的运行错误：插件名 → 错误
	serverErr  map[string]string // server.js 的错误
	servers    map[string]*serverRuntime
	grants     map[string]string         // 用户允许过的权限：插件名 → Permissions.Key()（内置插件不用问）
	configs    map[string]map[string]any // 用户改过的配置：插件名 → 配置项 → 值
	foreground bool                      // 渲染器在手环前台（收到 hello，还没 bye）
	active     string                    // 手环上当前显示的插件
	lastPush   time.Time

	// 待发的消息，由 Run 里的发送循环发出去
	outIndex  bool
	outStores map[string]bool
	outNeeds  []string
	outShow   string // 要手环显示的插件；空表示没有
	kick      chan struct{}
}

func NewHost() *Host {
	return &Host{
		Calls:     map[string]func([]any) (any, error){},
		blobs:     map[string]blob{},
		stores:    map[string]json.RawMessage{},
		runErr:    map[string]string{},
		serverErr: map[string]string{},
		servers:   map[string]*serverRuntime{},
		outStores: map[string]bool{},
		kick:      make(chan struct{}, 1),
	}
}

// Load 加载内置插件（builtin 根目录下每个子目录一个插件）和用户插件目录；同名的用户插件替换内置插件。
func (h *Host) Load(builtin fs.FS, userDir string) {
	h.mu.Lock()
	h.builtin, h.userDir = builtin, userDir
	h.mu.Unlock()
	h.reload()
}

// Run 负责发送和用户插件的热更新，一直跑到 ctx 结束。
func (h *Host) Run(ctx context.Context) {
	watch := time.NewTicker(2 * time.Second)
	defer watch.Stop()
	defer h.stopServers()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.kick:
			h.flush()
		case <-watch.C:
			if h.userChanged() {
				slog.Info("用户插件有改动，重新加载")
				h.reload()
			}
		}
	}
}

func (h *Host) reload() {
	h.mu.Lock()
	builtin, userDir := h.builtin, h.userDir
	h.mu.Unlock()

	blobs := map[string]blob{}
	byName := map[string]*Plugin{}
	add := func(fsys fs.FS, dir string, isBuiltin bool) {
		p, err := compile(fsys, dir, isBuiltin, blobs)
		if err != nil {
			slog.Warn("跳过插件", "dir", dir, "err", err)
			return
		}
		if p.Err != "" {
			slog.Warn("插件编译失败", "plugin", p.Name, "err", p.Err)
		}
		byName[p.Name] = p
	}
	if builtin != nil {
		entries, _ := fs.ReadDir(builtin, ".")
		for _, e := range entries {
			if sub, err := fs.Sub(builtin, e.Name()); err == nil && e.IsDir() {
				add(sub, e.Name(), true)
			}
		}
	}
	sig := ""
	if userDir != "" {
		entries, _ := os.ReadDir(userDir)
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				dir := filepath.Join(userDir, e.Name())
				add(os.DirFS(dir), dir, false)
			}
		}
		sig = dirSignature(userDir)
	}
	list := make([]*Plugin, 0, len(byName))
	for _, p := range byName {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Order != list[j].Order {
			return list[i].Order < list[j].Order
		}
		return list[i].Name < list[j].Name
	})
	h.stopServers()
	h.mu.Lock()
	h.plugins, h.blobs, h.userSig = list, blobs, sig
	h.runErr, h.serverErr = map[string]string{}, map[string]string{}
	h.outIndex = true
	h.mu.Unlock()
	h.publishConfigs()
	h.startServers()
	slog.Info("插件已加载", "count", len(list))
	h.changed()
	h.markDirty()
}

// dirSignature：目录下所有文件的路径、大小、修改时间，用来发现用户插件的改动
func dirSignature(dir string) string {
	var b strings.Builder
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			fmt.Fprintf(&b, "%s %d %d\n", p, info.Size(), info.ModTime().UnixNano())
		}
		return nil
	})
	return b.String()
}

func (h *Host) userChanged() bool {
	h.mu.Lock()
	dir, sig := h.userDir, h.userSig
	h.mu.Unlock()
	return dir != "" && dirSignature(dir) != sig
}

// startServers 跑各插件的 server.js：内置插件和没要权限的直接跑，要了权限的用户插件等用户允许。
func (h *Host) startServers() {
	h.mu.Lock()
	list := h.plugins
	h.mu.Unlock()
	for _, p := range list {
		if p.ServerCode == "" || p.Err != "" || !h.allowed(p) {
			continue
		}
		m := startServer(h, p)
		h.mu.Lock()
		h.servers[p.Name] = m
		h.mu.Unlock()
	}
}

func (h *Host) stopServers() {
	h.mu.Lock()
	servers := h.servers
	h.servers = map[string]*serverRuntime{}
	h.mu.Unlock()
	for _, m := range servers {
		m.stop()
	}
}

func (h *Host) allowed(p *Plugin) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.allowedLocked(p)
}

func (h *Host) allowedLocked(p *Plugin) bool {
	return p.Builtin || p.Permissions.Empty() || h.grants[p.Name] == p.Permissions.Key()
}

// ---------- 配置 ----------

// SetConfigs 设置用户改过的配置（插件名 → 配置项 → 值），在 Load 之前调用。
func (h *Host) SetConfigs(configs map[string]map[string]any) {
	h.mu.Lock()
	h.configs = configs
	h.mu.Unlock()
	h.publishConfigs()
}

// SetConfig 改一个配置项（App 的设置表单），值不合法时返回错误。改完推给手环和 server.js。
func (h *Host) SetConfig(plugin, key string, value any) error {
	h.mu.Lock()
	var setting *Setting
	for _, p := range h.plugins {
		if p.Name != plugin {
			continue
		}
		for i := range p.Settings {
			if p.Settings[i].Key == key {
				setting = &p.Settings[i]
			}
		}
	}
	if setting == nil {
		h.mu.Unlock()
		return fmt.Errorf("插件 %s 没有配置项 %s", plugin, key)
	}
	v, err := setting.check(value)
	if err != nil {
		h.mu.Unlock()
		return err
	}
	if h.configs == nil {
		h.configs = map[string]map[string]any{}
	}
	if h.configs[plugin] == nil {
		h.configs[plugin] = map[string]any{}
	}
	h.configs[plugin][key] = v
	h.mu.Unlock()
	h.publishConfigs()
	h.changed()
	return nil
}

// configOf 是插件现在的配置：默认值 + 用户改过的（不合法的、manifest 里已经没有的项忽略）。调用方持有 mu。
func (h *Host) configOf(p *Plugin) map[string]any {
	out := map[string]any{}
	for _, s := range p.Settings {
		out[s.Key] = s.Default
		if v, ok := h.configs[p.Name][s.Key]; ok {
			if v, err := s.check(v); err == nil {
				out[s.Key] = v
			}
		}
	}
	return out
}

// publishConfigs 把每个有配置项的插件的配置推成 "<插件名>:config"
func (h *Host) publishConfigs() {
	h.mu.Lock()
	cfgs := map[string]map[string]any{}
	for _, p := range h.plugins {
		if len(p.Settings) > 0 {
			cfgs[p.Name] = h.configOf(p)
		}
	}
	h.mu.Unlock()
	for name, c := range cfgs {
		h.SetStore(name+":config", c)
	}
}

// CallAll 让所有在 server.js 里 on(name) 注册了的插件都执行一遍（比如 App 里开了 Claude 额度后让插件马上刷新）。
func (h *Host) CallAll(name string, args []any) {
	h.mu.Lock()
	servers := make([]*serverRuntime, 0, len(h.servers))
	for _, m := range h.servers {
		servers = append(servers, m)
	}
	h.mu.Unlock()
	for _, m := range servers {
		m.call(name, args)
	}
}

// SetGrants 设置用户允许过的权限（插件名 → Permissions.Key()），变了就重新跑 server.js。
func (h *Host) SetGrants(grants map[string]string) {
	h.mu.Lock()
	h.grants = grants
	running := h.servers != nil && len(h.plugins) > 0
	h.mu.Unlock()
	if running {
		h.regrant()
	}
}

// regrant：权限变了，重新跑 server.js
func (h *Host) regrant() {
	h.stopServers()
	h.mu.Lock()
	h.serverErr = map[string]string{}
	h.mu.Unlock()
	h.startServers()
	h.changed()
}

func (h *Host) changed() {
	if h.OnChange != nil {
		h.OnChange()
	}
}

func (h *Host) markDirty() {
	select {
	case h.kick <- struct{}{}:
	default:
	}
}

// ---------- 数据和消息 ----------

// SetStore 推一份数据（agents、settings 等）给手环上的插件，插件里用 useStore(name) 读。没变就不发。
func (h *Host) SetStore(name string, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		slog.Warn("store 编码失败", "name", name, "err", err)
		return
	}
	h.mu.Lock()
	if string(h.stores[name]) == string(raw) {
		h.mu.Unlock()
		return
	}
	h.stores[name] = raw
	h.outStores[name] = true
	servers := make([]*serverRuntime, 0, len(h.servers))
	for _, m := range h.servers {
		servers = append(servers, m)
	}
	h.mu.Unlock()
	h.markDirty()
	for _, m := range servers {
		m.storeChanged(name, string(raw))
	}
	if h.OnStore != nil {
		h.OnStore(name)
	}
}

func (h *Host) storeJSON(name string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return string(h.stores[name])
}

// Stores 返回所有数据（App 的预览用）
func (h *Host) Stores() map[string]json.RawMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]json.RawMessage, len(h.stores))
	for k, v := range h.stores {
		out[k] = v
	}
	return out
}

// Call 处理插件的 call(name, ...args)：先找插件自己 server.js 里 on(name) 注册的，再找 Calls。
// 手环发来的和 App 预览里点的都走这里。
func (h *Host) Call(plugin, name string, args []any) error {
	h.mu.Lock()
	m := h.servers[plugin]
	fn := h.Calls[name]
	h.mu.Unlock()
	if m != nil && m.call(name, args) {
		return nil
	}
	if fn == nil {
		return fmt.Errorf("插件 %s 调用了没有的功能 %s", plugin, name)
	}
	_, err := fn(args)
	return err
}

// Bundle 返回插件编译好的界面代码和图片（App 的预览用）
func (h *Host) Bundle(name string) (code string, assets map[string][]byte, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, p := range h.plugins {
		if p.Name != name {
			continue
		}
		if p.Err != "" {
			return "", nil, errors.New(p.Err)
		}
		assets = map[string][]byte{}
		for path, hash := range p.Assets {
			assets[path] = h.blobs[hash].data
		}
		return string(h.blobs[p.Code].data), assets, nil
	}
	return "", nil, fmt.Errorf("没有插件 %s", name)
}

// HandleMessage 处理手环渲染器发来的消息。
func (h *Host) HandleMessage(m map[string]any) {
	switch m["t"] {
	case "hello":
		h.mu.Lock()
		h.foreground = true
		h.outIndex = true
		for name := range h.stores {
			h.outStores[name] = true
		}
		h.mu.Unlock()
		h.markDirty()
	case "bye":
		h.mu.Lock()
		h.foreground = false
		h.mu.Unlock()
	case "need":
		list, _ := m["h"].([]any)
		h.mu.Lock()
		for _, x := range list {
			if s, ok := x.(string); ok {
				h.outNeeds = append(h.outNeeds, s)
			}
		}
		h.mu.Unlock()
		h.markDirty()
	case "call":
		plugin, _ := m["p"].(string)
		name, _ := m["name"].(string)
		args, _ := m["args"].([]any)
		go func() {
			if err := h.Call(plugin, name, args); err != nil {
				slog.Info("插件调用失败", "plugin", plugin, "name", name, "err", err)
			}
		}()
	case "nav":
		name, _ := m["name"].(string)
		h.mu.Lock()
		h.active = name
		h.mu.Unlock()
		h.changed()
	case "log":
		name, _ := m["p"].(string)
		level, _ := m["level"].(string)
		msg, _ := m["msg"].(string)
		switch level {
		case "error":
			slog.Warn("插件出错", "plugin", name, "err", msg)
			h.mu.Lock()
			h.runErr[name] = msg
			h.mu.Unlock()
			h.changed()
		case "warn":
			slog.Warn("插件", "plugin", name, "msg", msg)
		default:
			slog.Info("插件", "plugin", name, "msg", msg)
		}
	}
}

// Disconnected：蓝牙断了，渲染器肯定不在前台了。
func (h *Host) Disconnected() {
	h.mu.Lock()
	h.foreground = false
	h.mu.Unlock()
}

// Show 让手环显示这个插件（App 里切换、插件 wakeBand）。渲染器不在前台时，等它打开（hello）后再切。
func (h *Host) Show(name string) {
	h.mu.Lock()
	h.outShow = name
	h.mu.Unlock()
	h.markDirty()
}

// ---------- 发送 ----------

type indexEntry struct {
	Name   string            `json:"name"`
	Title  string            `json:"title"`
	Code   string            `json:"code"`
	Assets map[string]string `json:"assets"`
	Err    string            `json:"err,omitempty"`
}

// flush 把待发的消息发出去：插件列表 → 数据 → 手环要的代码和图片。渲染器不在前台时先攒着。
func (h *Host) flush() {
	h.mu.Lock()
	if !h.foreground || h.Send == nil {
		h.mu.Unlock()
		return
	}
	var msgs []any
	if h.outIndex {
		list := make([]indexEntry, len(h.plugins))
		for i, p := range h.plugins {
			list[i] = indexEntry{Name: p.Name, Title: p.Title, Code: p.Code, Assets: p.Assets, Err: p.Err}
		}
		msgs = append(msgs, map[string]any{"t": "plugins", "list": list})
	}
	names := make([]string, 0, len(h.outStores))
	for name := range h.outStores {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		msgs = append(msgs, map[string]any{"t": "store", "name": name, "v": h.stores[name]})
	}
	if h.outShow != "" {
		msgs = append(msgs, map[string]any{"t": "show", "name": h.outShow})
	}
	sent := map[string]bool{}
	for _, hash := range h.outNeeds {
		b, ok := h.blobs[hash]
		if !ok || sent[hash] {
			continue
		}
		sent[hash] = true
		b64 := base64.StdEncoding.EncodeToString(b.data)
		n := (len(b64) + chunkChars - 1) / chunkChars
		for k := range n {
			msgs = append(msgs, map[string]any{"t": "blob", "h": hash, "x": b.ext, "k": k, "n": n, "b": b64[k*chunkChars : min((k+1)*chunkChars, len(b64))]})
		}
	}
	h.outIndex, h.outStores, h.outNeeds, h.outShow = false, map[string]bool{}, nil, ""
	h.mu.Unlock()

	for i, m := range msgs {
		if err := h.Send(m); err != nil {
			slog.Warn("发给手环失败", "err", err, "left", len(msgs)-i)
			// 没发出去的列表和数据下次 hello 时会重发；代码和图片手环会再要
			return
		}
	}
	if len(msgs) > 0 {
		h.mu.Lock()
		h.lastPush = time.Now()
		h.mu.Unlock()
		slog.Debug("已发给手环", "messages", len(msgs), "blobs", len(sent))
	}
}

// ---------- 给界面看的状态 ----------

type Info struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Builtin     bool           `json:"builtin"`
	Dir         string         `json:"dir"`
	Error       string         `json:"error"`
	Active      bool           `json:"active"` // 手环上当前显示的是这一页
	Code        string         `json:"code"`   // 代码的哈希，变了预览要重新加载
	HasServer   bool           `json:"hasServer"`
	Permissions []string       `json:"permissions"` // server.js 要的权限（给人看的）
	NeedGrant   bool           `json:"needGrant"`   // 要了权限还没被允许，server.js 没跑
	PermKey     string         `json:"permKey"`
	Settings    []Setting      `json:"settings"` // 配置项
	Config      map[string]any `json:"config"`   // 现在的配置
}

func (h *Host) Plugins() []Info {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Info, len(h.plugins))
	for i, p := range h.plugins {
		err := p.Err
		if err == "" {
			err = h.runErr[p.Name]
		}
		if err == "" {
			err = h.serverErr[p.Name]
		}
		hasServer := p.ServerCode != ""
		out[i] = Info{
			Name: p.Name, Title: p.Title, Builtin: p.Builtin, Dir: p.Dir, Error: err, Active: p.Name == h.active, Code: p.Code,
			HasServer: hasServer, Permissions: p.Permissions.Describe(), PermKey: p.Permissions.Key(),
			NeedGrant: hasServer && !h.allowedLocked(p),
			Settings:  p.Settings, Config: h.configOf(p),
		}
	}
	return out
}

// LastPush 最近一次给手环发东西的时间。
func (h *Host) LastPush() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastPush
}
