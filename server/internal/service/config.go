package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config 是 config.json：{"auth_key": "0x…", "address": "AA:BB:CC:DD:EE:FF", "notify": true, "notify_start": true, "alert": "auto"}
type Config struct {
	AuthKey string `json:"auth_key"`
	Address string `json:"address"`
	// 是否提醒（默认开）；关掉后快应用里的数据照常更新
	Notify *bool `json:"notify,omitempty"`
	// agent 开始跑时也提醒（默认开）
	NotifyStart *bool `json:"notify_start,omitempty"`
	// 提醒方式：auto（默认：手环上快应用在前台就只亮屏，否则发通知）/ wake（总是只亮屏）/ notify（总是发通知，会震动）/ both
	Alert string `json:"alert,omitempty"`
}

func (c Config) NotifyOn() bool      { return c.Notify == nil || *c.Notify }
func (c Config) NotifyStartOn() bool { return c.NotifyStart == nil || *c.NotifyStart }

func (c Config) AlertMode() string {
	switch c.Alert {
	case "wake", "notify", "both":
		return c.Alert
	}
	return "auto"
}

func (c Config) Validate() error {
	if c.AuthKey == "" || c.Address == "" {
		return errors.New("缺 auth_key 或 address（先跑 get_key.py 拿 key）")
	}
	return nil
}

// AppConfigPath 是 macOS App 用的配置位置：~/Library/Application Support/miband-pet/config.json
func AppConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "miband-pet", "config.json")
}

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("%s 解析失败: %w", path, err)
	}
	return cfg, nil
}

// Save 写配置；文件里有 auth key，只给当前用户读写。
func (c Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}
