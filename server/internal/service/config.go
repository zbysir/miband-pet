package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config 是 config.json：{"auth_key": "0x…", "address": "AA:BB:CC:DD:EE:FF", "pet": "orange"}
type Config struct {
	AuthKey string `json:"auth_key"`
	Address string `json:"address"`
	// 菜单栏的猫：orange（橘猫，默认）/ gray（灰猫，中华田园猫）/ white（白猫，美短）。手环上的猫是 Agent 猫插件自己的配置
	Pet string `json:"pet,omitempty"`
	// 用户允许过的插件权限：插件名 → 当时的权限（plugin.Permissions.Key()），权限变了要重新允许
	PluginGrants map[string]string `json:"plugin_grants,omitempty"`
	// 用户在 App 里改过的插件配置：插件名 → 配置项 → 值（插件在 manifest.json 的 settings 里声明有哪些）
	PluginConfig map[string]map[string]any `json:"plugin_config,omitempty"`

	// 以下是旧版的提醒设置，现在是 Agent 猫插件的配置；读到了就搬过去（MigrateLegacy），不再写回
	Notify       *bool  `json:"notify,omitempty"`
	NotifyStart  *bool  `json:"notify_start,omitempty"`
	Alert        string `json:"alert,omitempty"`
	KeepAwake    bool   `json:"keep_awake,omitempty"`
	KeepAwakeSec int    `json:"keep_awake_sec,omitempty"`
	ImpatientMin *int   `json:"impatient_min,omitempty"`
}

// MigrateLegacy 把旧版的提醒设置搬进 Agent 猫插件的配置（插件里已经改过的不覆盖），返回有没有改动。
func (c *Config) MigrateLegacy() bool {
	old := map[string]any{}
	switch {
	case c.Notify != nil && !*c.Notify:
		old["alert"] = "off"
	case c.Alert == "wake":
		old["alert"] = "wake"
	case c.Alert != "": // notify / both / auto：都会震动提醒，归到发通知
		old["alert"] = "notify"
	}
	if c.NotifyStart != nil {
		old["notifyStart"] = *c.NotifyStart
	}
	if c.KeepAwake {
		old["keepAwake"] = true
	}
	// 数字用 float64，和从 config.json 读出来的一样（插件配置的校验只认这个）
	if c.KeepAwakeSec > 0 {
		old["keepAwakeSec"] = float64(c.KeepAwakeSec)
	}
	if c.ImpatientMin != nil {
		old["impatientMin"] = float64(*c.ImpatientMin)
	}
	c.Notify, c.NotifyStart, c.Alert, c.KeepAwake, c.KeepAwakeSec, c.ImpatientMin = nil, nil, "", false, 0, nil
	if len(old) == 0 {
		return false
	}
	if c.PluginConfig == nil {
		c.PluginConfig = map[string]map[string]any{}
	}
	cat := c.PluginConfig["agent-cat"]
	if cat == nil {
		cat = map[string]any{}
		c.PluginConfig["agent-cat"] = cat
	}
	for k, v := range old {
		if _, ok := cat[k]; !ok {
			cat[k] = v
		}
	}
	return true
}

var Pets = []string{"orange", "gray", "white"}

func (c Config) PetSkin() string {
	for _, p := range Pets {
		if c.Pet == p {
			return p
		}
	}
	return "orange"
}

func (c Config) Validate() error {
	if c.AuthKey == "" || c.Address == "" {
		return errors.New("缺手环密钥或地址（在 App 里扫码登录，或者运行 miband key）")
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
