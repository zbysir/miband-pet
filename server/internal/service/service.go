// Package service 是常驻服务：连着手环，把 herdr agent 状态推给手环上的快应用，agent 有变化时提醒。
// 命令行（miband app）和 macOS App 共用。
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"miband/internal/bt"
	"miband/internal/herdr"
	"miband/internal/xiaomi"
)

const (
	pollInterval      = 3 * time.Second
	reconnectInterval = 15 * time.Second
)

// 手环上接收 agent 列表的快应用：列表版和宠物版，装了哪个就推给哪个
const (
	PackageList = "dev.bysir.herdrband"
	PackagePet  = "dev.bysir.herdrpet"
)

var appPackages = []string{PackageList, PackagePet}

// Dial 用 macOS 经典蓝牙 RFCOMM 连手环。
func Dial(address string, onData func([]byte), onClosed func()) (xiaomi.Transport, error) {
	conn, _, err := bt.Open(address, onData, onClosed)
	if err != nil {
		return nil, err
	}
	return conn, nil
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
	LastError  string           `json:"lastError"`
	Packages   []string         `json:"packages"`   // 手环上装了的快应用
	Foreground string           `json:"foreground"` // 手环上正在前台的快应用
	Agents     []herdr.AppAgent `json:"agents"`
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
	dev  *xiaomi.Device
	apps *xiaomi.Apps
	// 状态变化时回调（界面用来刷新），不要在里面阻塞
	OnChange func()

	mu         sync.Mutex
	cfg        Config
	packages   []string
	foreground map[string]bool // 快应用打开时发 hello、离开时发 bye
	latest     *herdr.Snapshot
	notes      []herdr.Event
	lastErr    string
	herdrErr   string
	lastPush   time.Time
	events     []EventRecord

	dirty chan struct{} // 有新数据要推，或快应用打开了要一份全量
}

func New(cfg Config) (*Service, error) {
	dev, apps, err := NewDevice(cfg)
	if err != nil {
		return nil, err
	}
	s := &Service{dev: dev, apps: apps, cfg: cfg, foreground: map[string]bool{}, dirty: make(chan struct{}, 1)}
	apps.OnMessage = func(pkg string, payload any) {
		m, _ := payload.(map[string]any)
		switch m["t"] {
		case "hello":
			s.setForeground(pkg, true)
			s.markDirty()
		case "bye":
			s.setForeground(pkg, false)
		}
	}
	dev.OnDisconnect = func() {
		s.changed()
		s.markDirty() // 让 deliver 循环醒过来去重连
	}
	return s, nil
}

// Run 一直跑到 ctx 结束；结束时关掉蓝牙通道。
func (s *Service) Run(ctx context.Context) error {
	defer s.dev.Close()
	go s.pollHerdr(ctx)
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
		LastError:  s.lastErr,
		Packages:   append([]string{}, s.packages...),
		HerdrError: s.herdrErr,
		Events:     append([]EventRecord{}, s.events...),
		Settings: map[string]any{
			"notify": s.cfg.NotifyOn(), "notifyStart": s.cfg.NotifyStartOn(), "alert": s.cfg.AlertMode(), "address": s.cfg.Address,
		},
	}
	st.Foreground = s.foregroundLocked()
	if s.latest != nil {
		st.Agents = s.latest.Agents
	}
	if !s.lastPush.IsZero() {
		st.LastPush = s.lastPush.UnixMilli()
	}
	return st
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
			s.mu.Lock()
			var events []herdr.Event
			if s.cfg.NotifyOn() && prev != nil {
				events = herdr.Events(prev, cur, s.cfg.NotifyStartOn())
			}
			s.latest = snap
			s.herdrErr = ""
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
	for _, it := range items {
		installed[it.GetPackageName()] = true
	}
	var list []string
	for _, p := range appPackages {
		if installed[p] {
			list = append(list, p)
		}
	}
	s.mu.Lock()
	s.packages = list
	s.foreground = map[string]bool{} // 由快应用重新发 hello
	s.mu.Unlock()
	slog.Info("手环上的快应用", "packages", list)
	for _, p := range list {
		if err := s.apps.SetStatus(p, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureConnected(ctx context.Context) {
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
		slog.Warn("连接手环失败，稍后重试", "err", err, "retry", reconnectInterval)
		s.dev.Close()
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectInterval):
		}
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
		s.mu.Lock()
		var snap *herdr.Snapshot
		if s.latest != nil {
			cp := *s.latest
			snap = &cp
		}
		notes := s.notes
		s.notes = nil
		s.mu.Unlock()
		if snap == nil {
			continue
		}
		if err := s.push(snap, notes); err != nil {
			slog.Warn("推送失败，重连后重推", "err", err)
			s.mu.Lock()
			s.notes = append(notes, s.notes...)
			s.mu.Unlock()
			s.setErr(err)
			s.dev.Close()
			s.markDirty()
		}
	}
}

func (s *Service) push(snap *herdr.Snapshot, notes []herdr.Event) error {
	snap.TS = time.Now().UnixMilli()
	s.mu.Lock()
	packages := append([]string{}, s.packages...)
	fg := s.foregroundLocked()
	alert := s.cfg.AlertMode()
	s.mu.Unlock()

	for _, p := range packages {
		if err := s.apps.Send(p, snap); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.lastPush = time.Now()
	s.mu.Unlock()
	slog.Info("推送", "agents", len(snap.Agents))
	if len(notes) == 0 {
		s.changed()
		return nil
	}

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
	// 亮屏 = 把快应用打开到前台（手环会亮屏、不震动）
	wake := func(pkg string) error {
		if err := s.apps.Launch(pkg, ""); err != nil {
			return err
		}
		slog.Info("亮屏", "package", pkg, "events", len(notes))
		record("wake")
		return nil
	}
	switch alert {
	case "wake", "both":
		pkg := fg
		if pkg == "" && len(packages) > 0 {
			pkg = packages[len(packages)-1] // 优先宠物版（列表最后一个）
		}
		if pkg != "" {
			if err := wake(pkg); err != nil {
				return err
			}
			if alert == "wake" {
				return nil
			}
		}
	case "notify":
	default: // auto：快应用开着就只亮屏，没开就发通知
		if fg != "" {
			return wake(fg)
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
