// Package service 是常驻服务：连着手环，跑插件（server/internal/plugin）并把界面推给手环上的渲染器。
// 读 agent 状态、提醒这些都是插件的事（见 server/plugins/agent-cat）。命令行（miband app）和 macOS App 共用。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"miband/internal/bt"
	"miband/internal/plugin"
	"miband/internal/usage"
	"miband/internal/xiaomi"
	"miband/plugins"
)

const (
	reconnectInterval    = 5 * time.Second // 断开后第一次重连的间隔，连续失败就翻倍
	maxReconnectInterval = time.Minute
)

// PackageRender 是手环上的通用渲染器（render-app），所有插件的界面都由它画。
const PackageRender = "dev.bysir.mibandpet"

// LegacyPackages 是改成插件之前的两个快应用（Agents 列表、Agent 猫），已经不再推送，界面上提示卸载。
var LegacyPackages = []string{"dev.bysir.herdrband", "dev.bysir.herdrpet"}

// PluginDir 是用户插件目录：~/Library/Application Support/miband-pet/plugins/<名字>/
func PluginDir() string {
	return filepath.Join(filepath.Dir(AppConfigPath()), "plugins")
}

// Dial 连手环：先走经典蓝牙 RFCOMM（日常连接）；连不上就扫描 BLE 再连。
// 手环在「连接新手机」的二维码界面时只能通过 BLE 连上（首次接管就靠这个）。
func Dial(address string, onData func([]byte), onClosed func()) (xiaomi.Transport, error) {
	t, _, err := dial(address, "", onData, onClosed)
	return t, err
}

// dial 按 prefer（上次成功的方式）优先尝试：经典蓝牙连不上时要等到超时（十几秒），
// 首次接管走通 BLE 之后就别每次都先等经典蓝牙超时了。
func dial(address, prefer string, onData func([]byte), onClosed func()) (xiaomi.Transport, string, error) {
	classic := func() (xiaomi.Transport, error) {
		conn, _, err := bt.Open(address, onData, onClosed)
		if err != nil {
			return nil, err // 不能返回 nil 的 *bt.Conn 包成非 nil 的接口
		}
		return conn, nil
	}
	ble := func() (xiaomi.Transport, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		conn, err := bt.OpenBLE(ctx, address, onData, onClosed)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	order := []string{"classic", "ble"}
	if prefer == "ble" {
		order = []string{"ble", "classic"}
	}
	var errs []string
	for _, kind := range order {
		open := classic
		if kind == "ble" {
			open = ble
		}
		t, err := open()
		if err == nil {
			return t, kind, nil
		}
		slog.Info("连接方式失败，换另一种", "transport", kind, "err", err)
		errs = append(errs, err.Error())
	}
	return nil, "", errors.New(strings.Join(errs, "；"))
}

// NewDevice 按配置创建手环会话（还没连接）。
func NewDevice(cfg Config) (*xiaomi.Device, *xiaomi.Apps, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	dev, err := xiaomi.NewDevice(cfg.AuthKey, cfg.Address, Dial)
	if err != nil {
		return nil, nil, err
	}
	return dev, xiaomi.NewApps(dev), nil
}

// State 是给界面看的快照。
type State struct {
	Connected  bool           `json:"connected"`
	Transport  string         `json:"transport"` // classic / ble
	LastError  string         `json:"lastError"`
	Packages   []string       `json:"packages"`   // 手环上装了的快应用
	Foreground string         `json:"foreground"` // 手环上正在前台的快应用
	Usage      []usage.Source `json:"usage"`
	Plugins    []plugin.Info  `json:"plugins"`
	PluginDir  string         `json:"pluginDir"`
	Legacy     []string       `json:"legacy"`   // 手环上还装着的旧版快应用
	LastPush   int64          `json:"lastPush"` // 毫秒
	Settings   map[string]any `json:"settings"`
}

type Service struct {
	dev        *xiaomi.Device
	apps       *xiaomi.Apps
	host       *plugin.Host
	statusline usage.Statusline // 接管 Claude Code 的状态栏拿额度（App 里打开）
	// 状态变化时回调（界面用来刷新），不要在里面阻塞
	OnChange func()
	// 插件数据（store）变化时回调（App 的插件预览用），不要在里面阻塞
	OnStore func()

	mu         sync.Mutex
	cfg        Config
	packages   []string        // 手环上装了的渲染器（装了就是 [PackageRender]）
	legacy     []string        // 手环上还装着的旧版快应用
	foreground map[string]bool // 快应用打开时发 hello、离开时发 bye
	lastErr    string
	transport  string

	dirty chan struct{} // 连接断了，叫醒重连循环
}

func New(cfg Config) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg.MigrateLegacy()
	dataDir := filepath.Dir(AppConfigPath())
	s := &Service{
		cfg: cfg, foreground: map[string]bool{}, dirty: make(chan struct{}, 1),
		statusline: usage.DefaultStatusline(dataDir),
	}
	dev, err := xiaomi.NewDevice(cfg.AuthKey, cfg.Address, func(address string, onData func([]byte), onClosed func()) (xiaomi.Transport, error) {
		s.mu.Lock()
		prefer := s.transport
		s.mu.Unlock()
		t, kind, err := dial(address, prefer, onData, onClosed)
		if err == nil {
			s.mu.Lock()
			s.transport = kind
			s.mu.Unlock()
		}
		return t, err
	})
	if err != nil {
		return nil, err
	}
	apps := xiaomi.NewApps(dev)
	s.dev, s.apps = dev, apps
	s.host = plugin.NewHost()
	s.host.Send = func(v any) error {
		if !s.dev.Connected() {
			return errNotConnected
		}
		return s.apps.Send(PackageRender, v)
	}
	s.host.OnChange = s.changed
	s.host.OnStore = func(string) {
		if s.OnStore != nil {
			s.OnStore()
		}
	}
	s.host.DataDir = filepath.Join(dataDir, "plugin-data")
	s.host.NotifyBand = func(title, body string) error {
		if !s.dev.Connected() {
			return errNotConnected
		}
		return s.dev.Notify(title, body)
	}
	s.host.Wake = s.wake
	s.host.SetGrants(copyGrants(cfg.PluginGrants))
	s.host.SetConfigs(copyConfigs(cfg.PluginConfig))
	s.publishBand()
	apps.OnMessage = func(pkg string, payload any) {
		m, _ := payload.(map[string]any)
		if pkg != PackageRender || m == nil {
			return
		}
		switch m["t"] {
		case "hello":
			s.setForeground(pkg, true)
		case "bye":
			s.setForeground(pkg, false)
		}
		s.host.HandleMessage(m)
	}
	dev.OnDisconnect = func() {
		s.host.Disconnected()
		s.publishBand()
		s.changed()
		s.markDirty() // 让 keepConnected 醒过来去重连
	}
	return s, nil
}

// Run 一直跑到 ctx 结束；结束时关掉蓝牙通道。
func (s *Service) Run(ctx context.Context) error {
	defer s.dev.Close()
	s.host.Load(plugins.FS, PluginDir())
	go s.host.Run(ctx)
	s.keepConnected(ctx)
	return ctx.Err()
}

// Close 立刻关掉蓝牙连接。退出时在主线程上调用：这时不能等 Run 结束，
// 因为 Run 里的蓝牙调用要派发到主线程，主线程在等就会死锁。
func (s *Service) Close() {
	s.dev.Close()
}

func (s *Service) changed() {
	if s.OnChange != nil {
		s.OnChange()
	}
}

func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := State{
		Connected: s.dev.Connected(),
		Transport: s.transport,
		LastError: s.lastErr,
		Packages:  append([]string{}, s.packages...),
		Legacy:    append([]string{}, s.legacy...),
		Usage:     s.usageLocked(),
		Plugins:   s.host.Plugins(),
		PluginDir: PluginDir(),
		Settings: map[string]any{
			"address": s.cfg.Address, "pet": s.cfg.PetSkin(), "claudeUsage": s.statusline.Enabled(),
		},
	}
	st.Foreground = s.foregroundLocked()
	if t := s.host.LastPush(); !t.IsZero() {
		st.LastPush = t.UnixMilli()
	}
	return st
}

// SetPet 换菜单栏的猫（手环上的猫是 Agent 猫插件自己的配置，两个分开）。
func (s *Service) SetPet(skin string) Config {
	s.mu.Lock()
	s.cfg.Pet = skin
	s.cfg.Pet = s.cfg.PetSkin() // 不认识的值回落到橘猫
	cfg := s.cfg
	s.mu.Unlock()
	s.changed()
	return cfg
}

// SetClaudeUsage 打开或关掉「显示 Claude Code 额度」（接管 Claude Code 的状态栏命令，见 usage.Statusline）。
func (s *Service) SetClaudeUsage(on bool) error {
	var err error
	if on {
		err = s.statusline.Enable()
	} else {
		err = s.statusline.Disable()
	}
	// 让读额度的插件（server.js 里 on('refresh')）马上重新读一次
	s.host.CallAll("refresh", nil)
	s.changed()
	return err
}

// usageLocked 汇总各插件 server.js 推的额度（数据名是 usage 的，即 "<插件名>:usage"），给 App 界面看
func (s *Service) usageLocked() []usage.Source {
	var out []usage.Source
	stores := s.host.Stores()
	names := make([]string, 0, len(stores))
	for k := range stores {
		if strings.HasSuffix(k, ":usage") {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	for _, k := range names {
		var list []usage.Source
		if json.Unmarshal(stores[k], &list) == nil {
			out = append(out, list...)
		}
	}
	return out
}

// SetPluginConfig 改插件的一个配置项（App 的设置表单）。
func (s *Service) SetPluginConfig(plugin, key string, value any) (Config, error) {
	if err := s.host.SetConfig(plugin, key, value); err != nil {
		s.mu.Lock()
		cfg := s.cfg
		s.mu.Unlock()
		return cfg, err
	}
	s.mu.Lock()
	if s.cfg.PluginConfig == nil {
		s.cfg.PluginConfig = map[string]map[string]any{}
	}
	if s.cfg.PluginConfig[plugin] == nil {
		s.cfg.PluginConfig[plugin] = map[string]any{}
	}
	s.cfg.PluginConfig[plugin][key] = value
	cfg := s.cfg
	s.mu.Unlock()
	s.changed()
	return cfg, nil
}

func copyConfigs(m map[string]map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(m))
	for k, v := range m {
		c := make(map[string]any, len(v))
		for kk, vv := range v {
			c[kk] = vv
		}
		out[k] = c
	}
	return out
}

// ---------- 插件预览（App 窗口里跑插件） ----------

// PluginBundle 返回插件编译好的界面代码和图片。
func (s *Service) PluginBundle(name string) (string, map[string][]byte, error) {
	return s.host.Bundle(name)
}

// PluginStores 返回所有插件数据。
func (s *Service) PluginStores() map[string]json.RawMessage {
	return s.host.Stores()
}

// PluginCall 处理预览里插件的 call(name, ...args)，和手环上发来的走同一条路。
func (s *Service) PluginCall(plugin, name string, args []any) error {
	return s.host.Call(plugin, name, args)
}

// AllowPlugin 允许插件的 server.js 用它在 manifest.json 里要的权限。key 是界面上显示时的权限，
// 和插件现在要的不一样（这期间插件改了）就不生效。
func (s *Service) AllowPlugin(name, key string) Config {
	s.mu.Lock()
	if s.cfg.PluginGrants == nil {
		s.cfg.PluginGrants = map[string]string{}
	}
	s.cfg.PluginGrants[name] = key
	cfg := s.cfg
	grants := copyGrants(s.cfg.PluginGrants)
	s.mu.Unlock()
	s.host.SetGrants(grants)
	s.changed()
	return cfg
}

// RevokePlugin 收回插件的权限，它的 server.js 停掉。
func (s *Service) RevokePlugin(name string) Config {
	s.mu.Lock()
	delete(s.cfg.PluginGrants, name)
	cfg := s.cfg
	grants := copyGrants(s.cfg.PluginGrants)
	s.mu.Unlock()
	s.host.SetGrants(grants)
	s.changed()
	return cfg
}

func copyGrants(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (s *Service) setForeground(pkg string, on bool) {
	s.mu.Lock()
	if on {
		s.foreground[pkg] = true
	} else {
		delete(s.foreground, pkg)
	}
	s.mu.Unlock()
	slog.Debug("快应用前台状态", "package", pkg, "foreground", on)
	s.publishBand()
	s.changed()
}

// publishBand 把手环的状态推给插件（基座数据 band，见 plugin.BaseStores）：
// connected 手环连着没有，foreground 手环上的 wristkit 开在前台没有（插件据此决定亮屏还是发通知）
func (s *Service) publishBand() {
	s.mu.Lock()
	fg := s.foregroundLocked() != ""
	s.mu.Unlock()
	s.host.SetStore("band", map[string]bool{"connected": s.dev.Connected(), "foreground": fg})
}

// foregroundLocked 返回手环上正在前台的快应用；没有则返回空串。调用方持有 mu。
func (s *Service) foregroundLocked() string {
	for _, p := range s.packages {
		if s.foreground[p] {
			return p
		}
	}
	return ""
}

func (s *Service) markDirty() {
	select {
	case s.dirty <- struct{}{}:
	default:
	}
}

func (s *Service) setErr(err error) {
	s.mu.Lock()
	if err != nil {
		s.lastErr = err.Error()
	} else {
		s.lastErr = ""
	}
	s.mu.Unlock()
	s.changed()
}

// refreshApps 重新读手环上装了哪些快应用，并告诉它们“手机端”在线。
func (s *Service) refreshApps() error {
	items, err := s.apps.List()
	if err != nil {
		return err
	}
	installed := map[string]bool{}
	var versions []string
	for _, it := range items {
		installed[it.GetPackageName()] = true
		versions = append(versions, fmt.Sprintf("%s@%d", it.GetPackageName(), it.GetVersionCode()))
	}
	var list, legacy []string
	if installed[PackageRender] {
		list = append(list, PackageRender)
	}
	for _, p := range LegacyPackages {
		if installed[p] {
			legacy = append(legacy, p)
		}
	}
	s.mu.Lock()
	s.packages, s.legacy = list, legacy
	s.foreground = map[string]bool{} // 由快应用重新发 hello
	s.mu.Unlock()
	s.publishBand()
	slog.Info("手环上的快应用", "packages", list, "versions", versions)
	for _, p := range list {
		if err := s.apps.SetStatus(p, true); err != nil {
			return err
		}
	}
	return nil
}

// waitAuthorized 等蓝牙授权。没授权时去碰 IOBluetooth 会阻塞到用户点完弹窗（还卡住主线程），所以先等。
func (s *Service) waitAuthorized(ctx context.Context) bool {
	for ctx.Err() == nil {
		switch bt.GetAuthorization() {
		case bt.AuthAllowed:
			return true
		case bt.AuthNotDetermined:
			bt.RequestAuthorization()
			s.setErr(errors.New("请在系统弹窗里允许 wristkit 使用蓝牙"))
		default:
			s.setErr(errors.New("没有蓝牙权限：到 系统设置 → 隐私与安全性 → 蓝牙 里允许 wristkit"))
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
	return false
}

func (s *Service) ensureConnected(ctx context.Context) {
	if !s.waitAuthorized(ctx) {
		return
	}
	// 手环长时间不在旁边（或连着手机）时别一直猛试：间隔从 5 秒翻倍到 1 分钟，连上后重置
	wait := reconnectInterval
	for !s.dev.Connected() && ctx.Err() == nil {
		err := s.dev.Connect()
		if err == nil {
			err = s.refreshApps()
		}
		s.setErr(err)
		if err == nil {
			return
		}
		slog.Warn("连接手环失败，稍后重试", "err", err, "retry", wait)
		s.dev.Close()
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, maxReconnectInterval)
	}
}

// keepConnected 一直保持连着手环：断了就重连。界面和数据由插件宿主自己推。
func (s *Service) keepConnected(ctx context.Context) {
	for ctx.Err() == nil {
		s.ensureConnected(ctx)
		select {
		case <-ctx.Done():
			return
		case <-s.dirty: // 断开了
		case <-time.After(time.Minute): // 定期回到 ensureConnected 检查连接
		}
	}
}

// ---------- 界面操作 ----------

var errNotConnected = errors.New("手环未连接")

func (s *Service) TestNotify() error {
	if !s.dev.Connected() {
		return errNotConnected
	}
	return s.dev.Notify("wristkit", "测试通知 "+time.Now().Format("15:04:05"))
}

// wake 把手环点亮并显示这个插件（插件的 wakeBand）：把渲染器打开到前台，手环会亮屏、不震动。
func (s *Service) wake(name string) error {
	if !s.dev.Connected() {
		return errNotConnected
	}
	s.mu.Lock()
	installed := len(s.packages) > 0
	s.mu.Unlock()
	if !installed {
		return errors.New("手环上还没装 wristkit")
	}
	s.host.Show(name)
	return s.apps.Launch(PackageRender, "")
}

func (s *Service) Launch(pkg string) error {
	if !s.dev.Connected() {
		return errNotConnected
	}
	return s.apps.Launch(pkg, "")
}

// ShowPlugin 让手环显示这个插件：手环上没开着渲染器就先打开它。
func (s *Service) ShowPlugin(name string) error {
	if !s.dev.Connected() {
		return errNotConnected
	}
	s.host.Show(name)
	s.mu.Lock()
	fg := s.foregroundLocked() != ""
	s.mu.Unlock()
	if fg {
		return nil
	}
	return s.apps.Launch(PackageRender, "")
}

// Remove 卸载手环上的快应用（用来清掉旧版）。
func (s *Service) Remove(pkg string) error {
	if !s.dev.Connected() {
		return errNotConnected
	}
	if err := s.apps.Remove(pkg); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond) // 等手环删完再刷新列表
	if err := s.refreshApps(); err != nil {
		return err
	}
	s.changed()
	return nil
}

// Install 装（或更新）快应用并打开。
func (s *Service) Install(path string, progress func(i, n int)) error {
	if !s.dev.Connected() {
		return errNotConnected
	}
	m, err := s.apps.Install(path, progress)
	if err != nil {
		return err
	}
	if err := s.refreshApps(); err != nil {
		return err
	}
	s.changed()
	return s.apps.Launch(m.Package, "")
}
