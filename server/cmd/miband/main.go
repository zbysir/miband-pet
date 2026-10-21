// miband：Mac 直连小米手环（经典蓝牙 SPP），把 herdr agent 状态推给手环上的快应用，并在有变化时提醒。
//
//	miband app                 常驻：推送 agent 列表给快应用；agent 开始跑/跑完/卡住时提醒
//	miband notify 标题 [正文]   发一条通知
//	miband apps                列出手环上的第三方快应用
//	miband install <rpk>       安装快应用并打开
//	miband launch [包名]        打开快应用（默认宠物版）
//
// 配置 config.json：{"auth_key": "0x…", "address": "AA:BB:CC:DD:EE:FF", "notify": true, "notify_start": true, "alert": "auto"}
package main

import (
	"context"
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
	"miband/internal/service"
	"miband/internal/xiaomi"
)

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
		var svc *service.Service
		if svc, err = service.New(*cfg); err == nil {
			err = svc.Run(ctx)
		}
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
		pkg := service.PackagePet
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

func loadConfig(path string) (*service.Config, error) {
	candidates := []string{path}
	if path == "" {
		candidates = []string{"config.json"}
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "config.json"))
		}
	}
	for _, p := range candidates {
		cfg, err := service.LoadConfig(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		return cfg, nil
	}
	return nil, fmt.Errorf("找不到配置文件（试过 %s）", strings.Join(candidates, ", "))
}

// withDevice 连上手环执行 fn，结束（含 Ctrl-C）时一定关掉 RFCOMM 通道：
// 通道没关就退出，手环那边的会话会卡在半开状态，之后谁都连不上，只能重启手环。
func withDevice(ctx context.Context, cfg *service.Config, fn func(*xiaomi.Device, *xiaomi.Apps) error) error {
	dev, apps, err := service.NewDevice(*cfg)
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
