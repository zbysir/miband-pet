// Package service 是常驻服务：连着手环，跑插件（server/internal/plugin）并把界面推给手环上的渲染器，
// 把 herdr agent 状态交给插件，agent 有变化时提醒。命令行（miband app）和 macOS App 共用。
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"miband/internal/bt"
	"miband/internal/herdr"
	"miband/internal/plugin"
	"miband/internal/usage"
	"miband/internal/xiaomi"
	"miband/plugins"
)

const (
	pollInterval         = 3 * time.Second
	usageInterval        = time.Minute
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
	Connected  bool             `json:"connected"`
	Transport  string           `json:"transport"` // classic / ble
	LastError  string           `json:"lastError"`
	Packages   []string         `json:"packages"`   // 手环上装了的快应用
	Foreground string           `json:"foreground"` // 手环上正在前台的快应用
	Agents     []herdr.AppAgent `json:"agents"`
	Stats      StatsView        `json:"stats"`
	Usage      []usage.Source   `json:"usage"`
	Plugins    []plugin.Info    `json:"plugins"`
	PluginDir  string           `json:"pluginDir"`
	Legacy     []string         `json:"legacy"` // 手环上还装着的旧版快应用
	HerdrError string           `json:"herdrError"`
	LastPush   int64            `json:"lastPush"` // 毫秒
	Events     []EventRecord    `json:"events"`   // 最近的提醒，新的在前
	Settings   map[string]any   `json:"settings"`
}

type EventRecord struct {
	Time   int64  `json:"time"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Action string `json:"action"` // wake / notify
}

type Service struct {
	dev        *xiaomi.Device
	apps       *xiaomi.Apps
	host       *plugin.Host
	stats      *Stats
	statusline usage.Statusline // 接管 Claude Code 的状态栏拿额度（App 里打开）
	// 状态变化时回调（界面用来刷新），不要在里面阻塞
	OnChange func()

	mu         sync.Mutex
	cfg        Config
	packages   []string        // 手环上装了的渲染器（装了就是 [PackageRender]）
	legacy     []string        // 手环上还装着的旧版快应用
	foreground map[string]bool // 快应用打开时发 hello、离开时发 bye
	latest     *herdr.Snapshot
	notes      []herdr.Event
	lastErr    string
	herdrErr   string
	events     []EventRecord
	transport  string
	lastWake   time.Time            // 上次把快应用打开到前台（亮屏）的时间
	since      map[string]sinceInfo // pane → 进入当前状态的时间
	reminded   map[string]int64     // pane → 已经为哪次卡住（since）再提醒过
	usage      []usage.Source

	dirty chan struct{} // 有新数据要推，或快应用打开了要一份全量
}

func New(cfg Config) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	dataDir := filepath.Dir(AppConfigPath())
	s := &Service{
		cfg: cfg, foreground: map[string]bool{}, dirty: make(chan struct{}, 1),
		since: map[string]sinceInfo{}, reminded: map[string]int64{},
		stats:      LoadStats(filepath.Join(dataDir, "stats.json")),
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
	s.host.Calls["feed"] = func([]any) (any, error) { return nil, s.Feed() }
	s.pushSettings()
	s.host.SetStore("stats", s.stats.View(time.Now()))
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
		s.changed()
		s.markDirty() // 让 deliver 循环醒过来去重连
	}
	return s, nil
}

// Run 一直跑到 ctx 结束；结束时关掉蓝牙通道。
func (s *Service) Run(ctx context.Context) error {
	defer s.dev.Close()
	s.host.Load(plugins.FS, PluginDir())
	go s.host.Run(ctx)
	go s.pollHerdr(ctx)
	go s.pollUsage(ctx)
	go s.keepAwake(ctx)
	s.deliver(ctx)
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
		Connected:  s.dev.Connected(),
		Transport:  s.transport,
		LastError:  s.lastErr,
		Packages:   append([]string{}, s.packages...),
		Legacy:     append([]string{}, s.legacy...),
		Stats:      s.stats.View(time.Now()),
		Usage:      s.usage,
		Plugins:    s.host.Plugins(),
		PluginDir:  PluginDir(),
		HerdrError: s.herdrErr,
		Events:     append([]EventRecord{}, s.events...),
		Settings: map[string]any{
			"notify": s.cfg.NotifyOn(), "notifyStart": s.cfg.NotifyStartOn(), "alert": s.cfg.AlertMode(), "address": s.cfg.Address,
			"pet":       s.cfg.PetSkin(),
			"keepAwake": s.cfg.KeepAwake, "keepAwakeSec": s.cfg.KeepAwakeSecs(),
			"impatientMin": s.cfg.ImpatientMinutes(), "claudeUsage": s.statusline.Enabled(),
		},
	}
	st.Foreground = s.foregroundLocked()
	if s.latest != nil {
		st.Agents = s.latest.Agents
	}
	if t := s.host.LastPush(); !t.IsZero() {
		st.LastPush = t.UnixMilli()
	}
	return st
}

// SetPet 换猫：Agent 猫插件马上换成新猫的帧（手环上没有的帧会推过去）。
func (s *Service) SetPet(skin string) Config {
	s.mu.Lock()
	s.cfg.Pet = skin
	s.cfg.Pet = s.cfg.PetSkin() // 不认识的值回落到橘猫
	cfg := s.cfg
	s.mu.Unlock()
	s.pushSettings()
	s.changed()
	return cfg
}

// pushSettings 把插件要用的设置推给手环
func (s *Service) pushSettings() {
	s.mu.Lock()
	v := map[string]any{"pet": s.cfg.PetSkin(), "impatientMin": s.cfg.ImpatientMinutes()}
	s.mu.Unlock()
	s.host.SetStore("settings", v)
}

// SetImpatient 改「卡住超过几分钟猫着急、再提醒一次」，0 表示关。
func (s *Service) SetImpatient(minutes int) Config {
	s.mu.Lock()
	s.cfg.ImpatientMin = &minutes
	cfg := s.cfg
	s.mu.Unlock()
	s.pushSettings()
	s.changed()
	return cfg
}

// Feed 喂一条小鱼干（手环上的 Agent 猫插件调用）。
func (s *Service) Feed() error {
	if err := s.stats.Feed(time.Now()); err != nil {
		return err
	}
	s.host.SetStore("stats", s.stats.View(time.Now()))
	s.changed()
	return nil
}

// SetClaudeUsage 打开或关掉「显示 Claude Code 额度」（接管 Claude Code 的状态栏命令，见 usage.Statusline）。
func (s *Service) SetClaudeUsage(on bool) error {
	var err error
	if on {
		err = s.statusline.Enable()
	} else {
		err = s.statusline.Disable()
	}
	s.readUsage()
	s.changed()
	return err
}

func (s *Service) pollUsage(ctx context.Context) {
	for {
		s.readUsage()
		select {
		case <-ctx.Done():
			return
		case <-time.After(usageInterval):
		}
	}
}

func (s *Service) readUsage() {
	home, _ := os.UserHomeDir()
	list := usage.Read(s.statusline.DataFile(), filepath.Join(home, ".codex", "sessions"), time.Now())
	s.mu.Lock()
	s.usage = list
	s.mu.Unlock()
	if list == nil {
		list = []usage.Source{}
	}
	s.host.SetStore("usage", list)
	s.changed()
}

// SetKeepAwake 改「有 agent 等你回答时保持亮屏」的设置。
func (s *Service) SetKeepAwake(on bool, seconds int) Config {
	s.mu.Lock()
	s.cfg.KeepAwake, s.cfg.KeepAwakeSec = on, seconds
	s.cfg.KeepAwakeSec = s.cfg.KeepAwakeSecs()
	cfg := s.cfg
	s.mu.Unlock()
	s.changed()
	return cfg
}

// UpdateSettings 改提醒设置（不含 key 和地址，那两个要重启服务）。
func (s *Service) UpdateSettings(notify, notifyStart bool, alert string) Config {
	s.mu.Lock()
	s.cfg.Notify, s.cfg.NotifyStart, s.cfg.Alert = &notify, &notifyStart, alert
	cfg := s.cfg
	s.mu.Unlock()
	s.changed()
	return cfg
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
	s.changed()
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

func (s *Service) pollHerdr(ctx context.Context) {
	var prev map[string]herdr.Agent
	lastKey := ""
	for {
		cur, err := herdr.List(ctx)
		if err != nil {
			slog.Warn("读取 herdr 失败", "err", err)
			s.mu.Lock()
			s.herdrErr = err.Error()
			s.mu.Unlock()
			s.changed()
		} else {
			snap := herdr.NewSnapshot(cur)
			now := time.Now()
			if prev != nil {
				if n := herdr.Finished(prev, cur); n > 0 {
					s.stats.AddDone(n, now)
					slog.Info("完成任务，得小鱼干", "count", n)
				}
			}
			s.host.SetStore("stats", s.stats.View(now)) // 过了零点「今天」也要换
			s.mu.Lock()
			var events []herdr.Event
			if s.cfg.NotifyOn() && prev != nil {
				events = herdr.Events(prev, cur, s.cfg.NotifyStartOn())
			}
			s.trackSince(snap, prev == nil, now)
			if s.cfg.NotifyOn() {
				events = append(events, s.impatientLocked(snap, now)...)
			}
			s.latest = snap
			s.herdrErr = ""
			s.host.SetStore("agents", snap.Agents)
			s.notes = append(s.notes, events...)
			s.mu.Unlock()
			for _, e := range events {
				slog.Info("事件", "title", e.Title, "body", e.Body)
			}
			prev = cur
			if key := snap.Key(); key != lastKey || len(events) > 0 {
				lastKey = key
				s.markDirty()
				s.changed()
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

type sinceInfo struct {
	status string
	since  int64 // 毫秒；0 表示 Mac 开始观察之前就是这个状态
}

// trackSince 记下每个 agent 进入当前状态的时间，写进快照（插件用来算卡了多久）。调用方持有 mu。
func (s *Service) trackSince(snap *herdr.Snapshot, first bool, now time.Time) {
	seen := map[string]bool{}
	for i := range snap.Agents {
		a := &snap.Agents[i]
		seen[a.ID] = true
		info, ok := s.since[a.ID]
		if !ok || info.status != a.Status {
			info = sinceInfo{status: a.Status}
			if !first {
				info.since = now.UnixMilli()
			}
			s.since[a.ID] = info
		}
		a.Since = info.since
	}
	for id := range s.since {
		if !seen[id] {
			delete(s.since, id)
			delete(s.reminded, id)
		}
	}
}

// impatientLocked：卡住超过设定的分钟数，再提醒一次（每次卡住只提醒一次）。调用方持有 mu。
func (s *Service) impatientLocked(snap *herdr.Snapshot, now time.Time) []herdr.Event {
	limit := s.cfg.ImpatientMinutes()
	if limit <= 0 {
		return nil
	}
	var out []herdr.Event
	for _, a := range snap.Agents {
		if a.Status != "blocked" || a.Since == 0 || s.reminded[a.ID] == a.Since {
			continue
		}
		mins := int(now.Sub(time.UnixMilli(a.Since)).Minutes())
		if mins < limit {
			continue
		}
		s.reminded[a.ID] = a.Since
		name := a.Agent
		if name == "" {
			name = "agent"
		}
		label := a.Title
		if a.Project != "" {
			label = a.Project + " · " + a.Title
		}
		out = append(out, herdr.Event{Title: fmt.Sprintf("%s 等你 %d 分钟了", name, mins), Body: label})
	}
	return out
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
			s.setErr(errors.New("请在系统弹窗里允许 miband pet 使用蓝牙"))
		default:
			s.setErr(errors.New("没有蓝牙权限：到 系统设置 → 隐私与安全性 → 蓝牙 里允许 miband pet"))
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
			s.markDirty()
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

func (s *Service) deliver(ctx context.Context) {
	for ctx.Err() == nil {
		s.ensureConnected(ctx)
		select {
		case <-ctx.Done():
			return
		case <-s.dirty:
		case <-time.After(time.Minute): // 定期回到 ensureConnected 检查连接
			continue
		}
		// 界面由插件宿主自己推；这里只管提醒
		s.mu.Lock()
		notes := s.notes
		s.notes = nil
		s.mu.Unlock()
		if len(notes) == 0 {
			continue
		}
		if err := s.alert(notes); err != nil {
			slog.Warn("提醒失败，重连后重试", "err", err)
			s.mu.Lock()
			s.notes = append(notes, s.notes...)
			s.mu.Unlock()
			s.setErr(err)
			s.dev.Close()
			s.markDirty()
		}
	}
}

// keepAwake：开了「保持亮屏」且有 agent 等你回答时，每隔设定的秒数把快应用打开到前台一次，
// 手环就不会熄屏。推送提醒时亮过屏也算，不会紧接着再亮一次。
func (s *Service) keepAwake(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		s.mu.Lock()
		on, every := s.cfg.KeepAwake, time.Duration(s.cfg.KeepAwakeSecs())*time.Second
		due := time.Since(s.lastWake) >= every
		blocked := false
		if s.latest != nil {
			for _, a := range s.latest.Agents {
				blocked = blocked || a.Status == "blocked"
			}
		}
		installed := len(s.packages) > 0
		s.mu.Unlock()
		if !on || !blocked || !due || !installed || !s.dev.Connected() {
			continue
		}
		if err := s.apps.Launch(PackageRender, ""); err != nil {
			slog.Warn("保持亮屏失败", "err", err)
			continue
		}
		s.mu.Lock()
		s.lastWake = time.Now()
		s.mu.Unlock()
		slog.Info("保持亮屏")
	}
}

// alert 按设置提醒：亮屏（把渲染器打开到前台、切到猫）或发通知。
func (s *Service) alert(notes []herdr.Event) error {
	s.mu.Lock()
	installed := len(s.packages) > 0
	fg := s.foregroundLocked()
	alert := s.cfg.AlertMode()
	s.mu.Unlock()

	// 同一轮里多个 agent 一起变化，合成一条提醒，免得连震好几下
	title, body := notes[0].Title, notes[0].Body
	if len(notes) > 1 {
		title = fmt.Sprintf("%d 个 agent 有变化", len(notes))
		lines := make([]string, len(notes))
		for i, n := range notes {
			lines[i] = n.Title + "：" + n.Body
		}
		body = strings.Join(lines, "\n")
	}
	record := func(action string) {
		s.mu.Lock()
		s.events = append([]EventRecord{{Time: time.Now().UnixMilli(), Title: title, Body: body, Action: action}}, s.events...)
		if len(s.events) > 30 {
			s.events = s.events[:30]
		}
		s.mu.Unlock()
		s.changed()
	}
	// 亮屏 = 把渲染器打开到前台并切到第一页（猫），手环会亮屏、不震动
	wake := func() error {
		s.host.ShowFirst()
		if err := s.apps.Launch(PackageRender, ""); err != nil {
			return err
		}
		s.mu.Lock()
		s.lastWake = time.Now()
		s.mu.Unlock()
		slog.Info("亮屏", "events", len(notes))
		record("wake")
		return nil
	}
	switch alert {
	case "wake", "both":
		if installed {
			if err := wake(); err != nil {
				return err
			}
			if alert == "wake" {
				return nil
			}
		}
	case "notify":
	default: // auto：渲染器开着就只亮屏，没开就发通知
		if fg != "" {
			return wake()
		}
	}
	if err := s.dev.Notify(title, body); err != nil {
		return err
	}
	record("notify")
	return nil
}

// ---------- 界面操作 ----------

var errNotConnected = errors.New("手环未连接")

func (s *Service) TestNotify() error {
	if !s.dev.Connected() {
		return errNotConnected
	}
	return s.dev.Notify("miband pet", "测试通知 "+time.Now().Format("15:04:05"))
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
	s.markDirty()
	s.changed()
	return s.apps.Launch(m.Package, "")
}
