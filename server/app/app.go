package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"miband/internal/micloud"
	"miband/plugins"
	"miband/internal/service"
	"miband/internal/tray"
	"miband/internal/xiaomi"
)

// 打包进 App 的手环快应用：渲染器（make app 时从 render-app/dist 复制进来）
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

	quitting atomic.Bool // 从菜单栏“退出”时为 true；否则关窗口只是隐藏

	loginCancel context.CancelFunc // 进行中的扫码登录

	anim *trayAnim // 菜单栏的猫
}

func NewApp() *App {
	return &App{cfgPath: service.AppConfigPath(), anim: newTrayAnim()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 被 kill / launchd 停止时也走正常退出流程（OnShutdown 里关蓝牙通道）
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		a.quit()
	}()
	tray.Init(a.showWindow, a.quit)
	go a.anim.run(ctx)
	a.bootstrapConfig()
	a.restartService()
}

// beforeClose：关窗口时只隐藏（连 Dock 图标一起），App 留在菜单栏继续推送；从菜单栏退出才真的退出。
func (a *App) beforeClose(ctx context.Context) bool {
	if a.quitting.Load() {
		return false
	}
	runtime.WindowHide(ctx)
	tray.SetDockVisible(false)
	return true
}

func (a *App) showWindow() {
	tray.SetDockVisible(true)
	runtime.WindowShow(a.ctx)
}

func (a *App) quit() {
	a.quitting.Store(true)
	runtime.Quit(a.ctx)
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

// stopService 停掉服务并关蓝牙通道。可能在主线程上调用（OnShutdown），
// 所以先直接关连接，再最多等 2 秒，不能无限等：服务里的蓝牙调用要派发到主线程，会互相卡死。
func (a *App) stopService() {
	a.mu.Lock()
	svc, stop, done := a.svc, a.stop, a.done
	a.svc, a.stop, a.done = nil, nil, nil
	a.mu.Unlock()
	if stop == nil {
		return
	}
	stop()
	svc.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		slog.Warn("等服务退出超时")
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
		st := a.state()
		tray.SetInfo(trayText(st))
		a.anim.update(st)
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

// trayText 是菜单栏菜单第一行的状态文字。
func trayText(st AppState) string {
	switch {
	case !st.Configured:
		return "还没配置手环"
	case !st.Connected && st.LastError != "":
		return "连接手环失败，重试中"
	case !st.Connected:
		return "正在连接手环…"
	}
	var blocked, working int
	for _, a := range st.Agents {
		switch a.Status {
		case "blocked":
			blocked++
		case "working":
			working++
		}
	}
	parts := []string{"已连接"}
	if blocked > 0 {
		parts = append(parts, fmt.Sprintf("%d 个等你处理", blocked))
	}
	if working > 0 {
		parts = append(parts, fmt.Sprintf("%d 个在忙", working))
	}
	if blocked == 0 && working == 0 {
		parts = append(parts, "都在休息")
	}
	return strings.Join(parts, " · ")
}

// ---------- 小米账号：扫码登录拿 auth key ----------

// 登录态缓存：下次拉 key 不用再扫码（手环重新绑定后 key 会变）
func (a *App) sessionPath() string {
	return filepath.Join(filepath.Dir(a.cfgPath), "session.json")
}

// CachedDevices 用缓存的登录态拉账号下的设备；没有缓存或已过期时返回错误，前端改走扫码。
func (a *App) CachedDevices() ([]micloud.Device, error) {
	raw, err := os.ReadFile(a.sessionPath())
	if err != nil {
		return nil, errors.New("没有登录过")
	}
	var s micloud.Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return s.Devices(ctx)
}

type LoginResult struct {
	Devices []micloud.Device `json:"devices"`
	Error   string           `json:"error"`
}

// StartLogin 拿登录二维码（data URL 返回给前端显示），后台等扫码；结果通过 "login" 事件发给前端。
func (a *App) StartLogin() (string, error) {
	a.mu.Lock()
	if a.loginCancel != nil {
		a.loginCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	a.loginCancel = cancel
	a.mu.Unlock()

	login, err := micloud.NewQRLogin(ctx)
	if err != nil {
		cancel()
		return "", err
	}
	go func() {
		defer cancel()
		res := LoginResult{}
		session, err := login.Wait(ctx)
		if err == nil {
			raw, _ := json.Marshal(session)
			os.MkdirAll(filepath.Dir(a.sessionPath()), 0o700)
			os.WriteFile(a.sessionPath(), raw, 0o600)
			res.Devices, err = session.Devices(context.Background())
		}
		if err != nil {
			if ctx.Err() == context.Canceled {
				return // 被新的登录取代了
			}
			res.Error = err.Error()
		}
		runtime.EventsEmit(a.ctx, "login", res)
	}()
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(login.QRImage), nil
}

func (a *App) CancelLogin() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loginCancel != nil {
		a.loginCancel()
		a.loginCancel = nil
	}
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

// SetKeepAwake 改「有 agent 等你回答时保持亮屏」：on 开关，seconds 每隔几秒亮一次屏（5~60）。
func (a *App) SetKeepAwake(on bool, seconds int) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	cfg := svc.SetKeepAwake(on, seconds)
	a.mu.Lock()
	a.cfg = &cfg
	a.mu.Unlock()
	return cfg.Save(a.cfgPath)
}

// SetPet 换猫：Mac 界面和手环上的 Agent 猫插件都马上换。
func (a *App) SetPet(skin string) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	cfg := svc.SetPet(skin)
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

// Install 把打包进来的快应用（渲染器）装（或更新）到手环上并打开。
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

// ShowPlugin 让手环显示这个插件（手环上没开着就先打开）。
func (a *App) ShowPlugin(name string) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	return svc.ShowPlugin(name)
}

// Uninstall 卸载手环上的快应用（清掉改成插件之前的旧版 Agent 猫、Agents 列表）。
func (a *App) Uninstall(pkg string) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	return svc.Remove(pkg)
}

// OpenPluginDir 在访达里打开用户插件目录（没有就先建，放一份说明）。
func (a *App) OpenPluginDir() error {
	dir := service.PluginDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	readme := filepath.Join(dir, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		_ = os.WriteFile(readme, []byte(pluginReadme), 0o644)
	}
	return exec.Command("open", dir).Run()
}

const pluginReadme = `# miband pet 插件目录

每个子目录是一个插件（manifest.json + index.jsx），保存后自动编译、推到手环。
内置插件（Agent 猫、今日、Agents）打包在 App 里，不在这个目录；想改的话，在 App 的「插件」里点「复制出来改」，
会把源码复制到这里，同名插件会替换内置的那个。

最小的插件：

    hello/manifest.json   {"name": "hello", "title": "你好"}
    hello/index.jsx

        import { Col, Text, useState } from 'miband'

        export default function Hello() {
          const [n, setN] = useState(0)
          return (
            <Col padding={16}>
              <Text style={{ fontSize: 18 }} onClick={() => setN(n + 1)}>点了 {n} 下</Text>
            </Col>
          )
        }

完整说明：https://github.com/zbysir/miband-pet/blob/main/docs/plugins.md
`

// CopyBuiltin 把内置插件的源码复制到用户插件目录，方便改；同名的用户插件会替换内置插件。
func (a *App) CopyBuiltin(name string) error {
	sub, err := fs.Sub(plugins.FS, name)
	if err != nil {
		return err
	}
	if _, err := fs.Stat(sub, "manifest.json"); err != nil {
		return fmt.Errorf("没有内置插件 %s", name)
	}
	dst := filepath.Join(service.PluginDir(), name)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("插件目录里已经有 %s 了", name)
	}
	err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(sub, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return err
	}
	return exec.Command("open", dst).Run()
}

// SetImpatient 改「卡住超过几分钟猫着急、再提醒一次」，0 表示关。
func (a *App) SetImpatient(minutes int) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	cfg := svc.SetImpatient(minutes)
	a.mu.Lock()
	a.cfg = &cfg
	a.mu.Unlock()
	return cfg.Save(a.cfgPath)
}

// SetClaudeUsage 打开或关掉「显示 Claude Code 额度」：接管 Claude Code 的状态栏命令拿额度，关掉时原样还原。
func (a *App) SetClaudeUsage(on bool) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	return svc.SetClaudeUsage(on)
}
