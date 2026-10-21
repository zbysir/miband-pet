// miband：Mac 直连小米手环（经典蓝牙 SPP），把 herdr agent 状态推给手环上的快应用，并在有变化时发通知。
//
//	miband app                 常驻：推送 agent 列表给快应用；agent 跑完/卡住时发通知
//	miband notify 标题 [正文]   发一条通知
//	miband apps                列出手环上的第三方快应用
//	miband install <rpk>       安装快应用并打开
//	miband launch [包名]        打开快应用（默认宠物版）
//
// 配置 config.json：{"auth_key": "0x…", "address": "AA:BB:CC:DD:EE:FF", "notify": true, "notify_start": true, "alert": "wake"}
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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
var appPackages = []string{"dev.bysir.herdrband", "dev.bysir.herdrpet"}

type config struct {
	AuthKey string `json:"auth_key"`
	Address string `json:"address"`
	Notify  *bool  `json:"notify"`
	// agent 开始跑时也提醒（默认开）
	NotifyStart *bool `json:"notify_start"`
	// 提醒方式：wake（默认，把快应用打开到前台，手环亮屏不震动）/ notify（发通知，会震动）/ both
	Alert string `json:"alert"`
}

func main() {
	bt.RunMain(run)
}

func run() int {
	configPath := flag.String("config", "", "配置文件路径（默认找 ./config.json，再找可执行文件上一级目录）")
	verbose := flag.Bool("v", false, "打印调试日志")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法: miband [-config path] [-v] <app|notify|apps|install|launch> ...")
		flag.PrintDefaults()
	}
	flag.Parse()
	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return 2
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "app":
		err = runBridge(ctx, cfg)
	case "notify":
		if len(args) < 2 {
			fatal(errors.New("用法: miband notify 标题 [正文]"))
		}
		err = withDevice(ctx, cfg, func(dev *xiaomi.Device, _ *xiaomi.Apps) error {
			if err := dev.Notify(args[1], strings.Join(args[2:], " ")); err != nil {
				return err
			}
			time.Sleep(time.Second) // 给手环一点时间展示，再断开
			fmt.Println("已发送")
			return nil
		})
	case "apps":
		err = withDevice(ctx, cfg, func(_ *xiaomi.Device, apps *xiaomi.Apps) error {
			list, err := apps.List()
			for _, a := range list {
				fmt.Printf("%-32s v%-4d %s\n", a.GetPackageName(), a.GetVersionCode(), a.GetAppName())
			}
			return err
		})
	case "launch":
		pkg := "dev.bysir.herdrpet"
		if len(args) > 1 {
			pkg = args[1]
		}
		err = withDevice(ctx, cfg, func(_ *xiaomi.Device, apps *xiaomi.Apps) error {
			if err := apps.Launch(pkg, ""); err != nil {
				return err
			}
			time.Sleep(time.Second)
			fmt.Println("已打开", pkg)
			return nil
		})
	case "install":
		if len(args) < 2 {
			fatal(errors.New("用法: miband install <rpk>"))
		}
		err = withDevice(ctx, cfg, func(_ *xiaomi.Device, apps *xiaomi.Apps) error {
			m, err := apps.Install(args[1], func(i, n int) { fmt.Printf("\r传输 %d/%d", i, n) })
			fmt.Println()
			if err != nil {
				return err
			}
			fmt.Println("已安装:", m.Package)
			return apps.Launch(m.Package, "")
		})
	default:
		flag.Usage()
		return 2
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		fatal(err)
	}
	return 0
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}

func loadConfig(path string) (*config, error) {
	candidates := []string{path}
	if path == "" {
		candidates = []string{"config.json"}
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "config.json"))
		}
	}
	for _, p := range candidates {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		cfg := &config{}
		if err := json.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("%s 解析失败: %w", p, err)
		}
		if cfg.AuthKey == "" || cfg.Address == "" {
			return nil, fmt.Errorf("%s 里缺 auth_key 或 address（先跑 get_key.py）", p)
		}
		return cfg, nil
	}
	return nil, fmt.Errorf("找不到配置文件（试过 %s）", strings.Join(candidates, ", "))
}

func dial(address string, onData func([]byte), onClosed func()) (xiaomi.Transport, error) {
	conn, _, err := bt.Open(address, onData, onClosed)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func newDevice(cfg *config) (*xiaomi.Device, *xiaomi.Apps, error) {
	dev, err := xiaomi.NewDevice(cfg.AuthKey, cfg.Address, dial)
	if err != nil {
		return nil, nil, err
	}
	return dev, xiaomi.NewApps(dev), nil
}

// withDevice 连上手环执行 fn，结束（含 Ctrl-C）时一定关掉 RFCOMM 通道：
// 通道没关就退出，手环那边的会话会卡在半开状态，之后谁都连不上，只能重启手环。
func withDevice(ctx context.Context, cfg *config, fn func(*xiaomi.Device, *xiaomi.Apps) error) error {
	dev, apps, err := newDevice(cfg)
	if err != nil {
		return err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			dev.Close()
			os.Exit(130)
		case <-done:
		}
	}()
	// 刚断开上一个连接时手环偶尔不响应，重试一次
	if err := dev.Connect(); err != nil {
		slog.Warn("连接失败，3 秒后重试", "err", err)
		time.Sleep(3 * time.Second)
		if err := dev.Connect(); err != nil {
			return err
		}
	}
	defer dev.Close()
	return fn(dev, apps)
}

// ---------------- 常驻服务 ----------------

type bridge struct {
	dev         *xiaomi.Device
	apps        *xiaomi.Apps
	notify      bool
	notifyStart bool
	alert       string
	packages    []string

	dirty chan struct{} // 有新数据要推，或快应用打开了要一份全量
	state chan func(*bridgeState)
}

type bridgeState struct {
	latest *herdr.Snapshot
	notes  []herdr.Event
}

// runBridge：连着手环，herdr 状态一变就把全量列表推给快应用；快应用打开时发 hello 也推一次。
// 有 agent 跑完或卡住时再发一条通知让手环震动（亮不亮屏看手环上「通知亮屏」的设置）。
func runBridge(ctx context.Context, cfg *config) error {
	dev, apps, err := newDevice(cfg)
	if err != nil {
		return err
	}
	b := &bridge{
		dev: dev, apps: apps,
		notify:      cfg.Notify == nil || *cfg.Notify,
		notifyStart: cfg.NotifyStart == nil || *cfg.NotifyStart,
		alert:       cfg.Alert,
		dirty:       make(chan struct{}, 1),
		state:       make(chan func(*bridgeState)),
	}
	apps.OnMessage = func(pkg string, payload any) {
		if m, ok := payload.(map[string]any); ok && m["t"] == "hello" {
			b.markDirty()
		}
	}
	dev.OnDisconnect = b.markDirty // 让 deliver 循环醒过来去重连
	defer dev.Close()

	var st bridgeState
	go b.pollHerdr(ctx)
	go b.deliver(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case fn := <-b.state:
			fn(&st)
		}
	}
}

func (b *bridge) markDirty() {
	select {
	case b.dirty <- struct{}{}:
	default:
	}
}

// with 在主循环里串行访问共享状态
func (b *bridge) with(ctx context.Context, fn func(*bridgeState)) {
	done := make(chan struct{})
	select {
	case b.state <- func(s *bridgeState) { fn(s); close(done) }:
		<-done
	case <-ctx.Done():
	}
}

func (b *bridge) pollHerdr(ctx context.Context) {
	var prev map[string]herdr.Agent
	lastKey := ""
	for {
		cur, err := herdr.List(ctx)
		if err != nil {
			slog.Warn("读取 herdr 失败", "err", err)
		} else {
			snap := herdr.NewSnapshot(cur)
			var events []herdr.Event
			if b.notify && prev != nil {
				events = herdr.Events(prev, cur, b.notifyStart)
				for _, e := range events {
					slog.Info("事件", "title", e.Title, "body", e.Body)
				}
			}
			prev = cur
			key := snap.Key()
			b.with(ctx, func(s *bridgeState) {
				s.latest = snap
				s.notes = append(s.notes, events...)
			})
			if key != lastKey || len(events) > 0 {
				lastKey = key
				b.markDirty()
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

func (b *bridge) ensureConnected(ctx context.Context) {
	for !b.dev.Connected() {
		err := b.dev.Connect()
		if err == nil {
			var list []string
			installed := map[string]bool{}
			if items, lerr := b.apps.List(); lerr == nil {
				for _, it := range items {
					installed[it.GetPackageName()] = true
				}
			} else {
				err = lerr
			}
			for _, p := range appPackages {
				if installed[p] {
					list = append(list, p)
				}
			}
			b.packages = list
			slog.Info("手环上的快应用", "packages", list)
			for _, p := range list {
				if serr := b.apps.SetStatus(p, true); serr != nil && err == nil {
					err = serr
				}
			}
			if err == nil {
				b.markDirty()
				return
			}
		}
		slog.Warn("连接手环失败，稍后重试", "err", err, "retry", reconnectInterval)
		b.dev.Close()
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectInterval):
		}
	}
}

func (b *bridge) deliver(ctx context.Context) {
	for ctx.Err() == nil {
		b.ensureConnected(ctx)
		select {
		case <-ctx.Done():
			return
		case <-b.dirty:
		case <-time.After(time.Minute): // 定期回到 ensureConnected 检查连接
			continue
		}
		var snap *herdr.Snapshot
		var notes []herdr.Event
		b.with(ctx, func(s *bridgeState) {
			if s.latest != nil {
				cp := *s.latest
				snap = &cp
			}
			notes, s.notes = s.notes, nil
		})
		if snap == nil {
			continue
		}
		if err := b.push(snap, notes); err != nil {
			slog.Warn("推送失败，重连后重推", "err", err)
			b.with(ctx, func(s *bridgeState) { s.notes = append(notes, s.notes...) })
			b.dev.Close()
			b.markDirty()
		}
	}
}

func (b *bridge) push(snap *herdr.Snapshot, notes []herdr.Event) error {
	snap.TS = time.Now().UnixMilli()
	for _, p := range b.packages {
		if err := b.apps.Send(p, snap); err != nil {
			return err
		}
	}
	slog.Info("推送", "agents", len(snap.Agents))
	if len(notes) == 0 {
		return nil
	}
	if b.alert != "notify" && len(b.packages) > 0 {
		// 打开快应用到前台：手环会亮屏、不震动。优先宠物版（列表最后一个）
		pkg := b.packages[len(b.packages)-1]
		if err := b.apps.Launch(pkg, ""); err != nil {
			return err
		}
		slog.Info("亮屏", "package", pkg, "events", len(notes))
		if b.alert != "both" {
			return nil
		}
	}
	// 同一轮里多个 agent 一起变化，合成一条通知，免得连震好几下
	title, body := notes[0].Title, notes[0].Body
	if len(notes) > 1 {
		title = fmt.Sprintf("%d 个 agent 有变化", len(notes))
		lines := make([]string, len(notes))
		for i, n := range notes {
			lines[i] = n.Title + "：" + n.Body
		}
		body = strings.Join(lines, "\n")
	}
	return b.dev.Notify(title, body)
}
