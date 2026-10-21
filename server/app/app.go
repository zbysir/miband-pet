package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"miband/internal/service"
	"miband/internal/xiaomi"
)

// 打包进 App 的手环快应用（make app 时从 band-app/pet-app 的 dist 复制进来）
//
//go:embed all:rpk
var rpkFS embed.FS

// App 是绑定给前端的对象：前端通过 window.go.main.App.* 调用这些导出方法。
type App struct {
	ctx     context.Context
	cfgPath string

	mu         sync.Mutex
	cfg        *service.Config
	svc        *service.Service
	stop       context.CancelFunc
	done       chan struct{}
	setupErr   string
	installing string
	progress   float64

	emitMu  sync.Mutex
	emitDue bool
}

func NewApp() *App {
	return &App{cfgPath: service.AppConfigPath()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 被 kill / launchd 停止时也走正常退出流程（OnShutdown 里关蓝牙通道）
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		runtime.Quit(ctx)
	}()
	a.bootstrapConfig()
	a.restartService()
}

func (a *App) shutdown(context.Context) {
	a.stopService() // 一定要关掉蓝牙通道，否则手环那边会卡住
}

// bootstrapConfig：App 自己的配置不存在时，试着从 MIBAND_CONFIG 或仓库根目录的 config.json 复制一份。
func (a *App) bootstrapConfig() {
	if _, err := os.Stat(a.cfgPath); err == nil {
		return
	}
	candidates := []string{os.Getenv("MIBAND_CONFIG")}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "config.json"), filepath.Join(wd, "..", "..", "config.json"))
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		cfg, err := service.LoadConfig(p)
		if err != nil || cfg.Validate() != nil {
			continue
		}
		if err := cfg.Save(a.cfgPath); err == nil {
			slog.Info("已导入配置", "from", p, "to", a.cfgPath)
			return
		}
	}
}

func (a *App) stopService() {
	a.mu.Lock()
	stop, done := a.stop, a.done
	a.svc, a.stop, a.done = nil, nil, nil
	a.mu.Unlock()
	if stop != nil {
		stop()
		<-done
	}
}

func (a *App) restartService() {
	a.stopService()
	cfg, err := service.LoadConfig(a.cfgPath)
	if err == nil {
		err = cfg.Validate()
	}
	var svc *service.Service
	if err == nil {
		svc, err = service.New(*cfg)
	}
	a.mu.Lock()
	a.cfg = cfg
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			a.setupErr = ""
		} else {
			a.setupErr = err.Error()
		}
		a.mu.Unlock()
		a.emitState()
		return
	}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	svc.OnChange = a.emitState
	a.svc, a.stop, a.done, a.setupErr = svc, stop, done, ""
	a.mu.Unlock()
	go func() {
		defer close(done)
		svc.Run(ctx)
	}()
	a.emitState()
}

// emitState 通知前端重新拉状态；合并短时间内的多次变化。
func (a *App) emitState() {
	a.emitMu.Lock()
	if a.emitDue {
		a.emitMu.Unlock()
		return
	}
	a.emitDue = true
	a.emitMu.Unlock()
	time.AfterFunc(150*time.Millisecond, func() {
		a.emitMu.Lock()
		a.emitDue = false
		a.emitMu.Unlock()
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "state")
		}
	})
}

func (a *App) service() (*service.Service, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.svc == nil {
		return nil, errors.New("还没配置好手环")
	}
	return a.svc, nil
}

// ---------- 绑定给前端的方法 ----------

type AppState struct {
	service.State
	Configured      bool    `json:"configured"`
	ConfigPath      string  `json:"configPath"`
	SetupError      string  `json:"setupError"`
	Installing      string  `json:"installing"`
	InstallProgress float64 `json:"installProgress"`
}

// GetState 返回 AppState 的 JSON 对象（前端的类型定义在 frontend/src/types.ts，
// 不让 Wails 生成 Go 内部包的类型）。
func (a *App) GetState() map[string]any {
	raw, _ := json.Marshal(a.state())
	var m map[string]any
	json.Unmarshal(raw, &m)
	return m
}

func (a *App) state() AppState {
	a.mu.Lock()
	svc := a.svc
	st := AppState{ConfigPath: a.cfgPath, SetupError: a.setupErr, Installing: a.installing, InstallProgress: a.progress}
	if a.cfg != nil {
		st.Configured = a.cfg.Validate() == nil
	}
	a.mu.Unlock()
	if svc != nil {
		st.State = svc.State()
	}
	return st
}

// SaveSetup 保存 auth key 和手环地址，然后重连。
func (a *App) SaveSetup(authKey, address string) error {
	authKey, address = strings.TrimSpace(authKey), strings.ToUpper(strings.TrimSpace(address))
	if !strings.HasPrefix(authKey, "0x") {
		authKey = "0x" + authKey
	}
	cfg := service.Config{}
	if old, err := service.LoadConfig(a.cfgPath); err == nil {
		cfg = *old
	}
	cfg.AuthKey, cfg.Address = authKey, address
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, err := xiaomi.NewDevice(cfg.AuthKey, cfg.Address, nil); err != nil {
		return err
	}
	if err := cfg.Save(a.cfgPath); err != nil {
		return err
	}
	a.restartService()
	return nil
}

func (a *App) UpdateSettings(notify, notifyStart bool, alert string) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	cfg := svc.UpdateSettings(notify, notifyStart, alert)
	a.mu.Lock()
	a.cfg = &cfg
	a.mu.Unlock()
	return cfg.Save(a.cfgPath)
}

func (a *App) TestNotify() error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	return svc.TestNotify()
}

func (a *App) Launch(pkg string) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	return svc.Launch(pkg)
}

type BundledApp struct {
	Package     string `json:"package"`
	Name        string `json:"name"`
	VersionCode uint32 `json:"versionCode"`
	File        string `json:"file"`
}

// BundledApps 列出打包进 App 的手环快应用。
func (a *App) BundledApps() []BundledApp {
	var out []BundledApp
	entries, _ := fs.ReadDir(rpkFS, "rpk")
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".rpk") {
			continue
		}
		path, err := a.extractRpk(e.Name())
		if err != nil {
			continue
		}
		m, err := xiaomi.ReadManifest(path)
		if err != nil {
			continue
		}
		out = append(out, BundledApp{Package: m.Package, Name: m.Name, VersionCode: m.VersionCode, File: e.Name()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Package > out[j].Package }) // 宠物版在前
	return out
}

func (a *App) extractRpk(name string) (string, error) {
	data, err := rpkFS.ReadFile("rpk/" + name)
	if err != nil {
		return "", err
	}
	path := filepath.Join(os.TempDir(), "miband-pet-"+name)
	return path, os.WriteFile(path, data, 0o644)
}

// Install 把打包进来的快应用装（或更新）到手环上并打开。
func (a *App) Install(pkg string) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	var file string
	for _, b := range a.BundledApps() {
		if b.Package == pkg {
			file = b.File
		}
	}
	if file == "" {
		return fmt.Errorf("App 里没有打包 %s", pkg)
	}
	path, err := a.extractRpk(file)
	if err != nil {
		return err
	}
	a.mu.Lock()
	if a.installing != "" {
		a.mu.Unlock()
		return errors.New("正在安装另一个应用")
	}
	a.installing, a.progress = pkg, 0
	a.mu.Unlock()
	a.emitState()
	defer func() {
		a.mu.Lock()
		a.installing, a.progress = "", 0
		a.mu.Unlock()
		a.emitState()
	}()
	return svc.Install(path, func(i, n int) {
		a.mu.Lock()
		a.progress = float64(i) / float64(n)
		a.mu.Unlock()
		a.emitState()
	})
}
