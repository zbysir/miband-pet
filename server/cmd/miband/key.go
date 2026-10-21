package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"miband/internal/micloud"
	"miband/internal/service"
)

// runKey：扫码登录小米账号（有缓存的登录态就跳过），拉手环的 auth key 和地址写进配置。
func runKey(configPath string) error {
	if configPath == "" {
		configPath = "config.json"
	}
	sessionPath := filepath.Join(filepath.Dir(configPath), ".xiaomi_session.json")
	ctx := context.Background()

	devices, err := devicesFromCache(ctx, sessionPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Println("缓存的登录态不能用了，重新扫码：", err)
		}
		login, err := micloud.NewQRLogin(ctx)
		if err != nil {
			return err
		}
		png := filepath.Join(os.TempDir(), "miband-login-qr.png")
		if err := os.WriteFile(png, login.QRImage, 0o600); err != nil {
			return err
		}
		exec.Command("open", png).Run()
		fmt.Println("二维码已打开，用登录了同一个小米账号的手机扫码确认（5 分钟内有效）")
		wctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		session, err := login.Wait(wctx)
		os.Remove(png)
		if err != nil {
			return err
		}
		fmt.Println("登录成功，userId =", session.UserID)
		raw, _ := json.Marshal(session)
		if err := os.WriteFile(sessionPath, raw, 0o600); err != nil {
			return err
		}
		if devices, err = session.Devices(ctx); err != nil {
			return err
		}
	}

	var band *micloud.Device
	for i, d := range devices {
		mark := "无 key"
		if d.AuthKey != "" {
			mark = "有 key"
		}
		fmt.Printf("- %s  %s  %s\n", d.Name, d.MAC, mark)
		if band == nil && d.AuthKey != "" && (strings.Contains(d.Name, "手环") || strings.Contains(strings.ToLower(d.Name), "band")) {
			band = &devices[i]
		}
	}
	if band == nil {
		return errors.New("账号下没找到带 key 的手环（先用小米运动健康绑定一次）")
	}

	cfg := service.Config{}
	if old, err := service.LoadConfig(configPath); err == nil {
		cfg = *old
	}
	cfg.AuthKey, cfg.Address = band.AuthKey, band.MAC
	if err := cfg.Save(configPath); err != nil {
		return err
	}
	fmt.Printf("已把 %s 的 key 和地址写进 %s\n", band.Name, configPath)
	return nil
}

func devicesFromCache(ctx context.Context, path string) ([]micloud.Device, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s micloud.Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return s.Devices(ctx)
}
