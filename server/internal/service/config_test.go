package service

import (
	"reflect"
	"testing"
)

// 旧版的提醒设置搬进 Agent 猫插件的配置；用户在插件里已经改过的不覆盖
func TestMigrateLegacy(t *testing.T) {
	on, mins := false, 3
	c := Config{NotifyStart: &on, Alert: "both", KeepAwake: true, KeepAwakeSec: 30, ImpatientMin: &mins,
		PluginConfig: map[string]map[string]any{"agent-cat": {"keepAwakeSec": 20.0}}}
	if !c.MigrateLegacy() {
		t.Fatal("应该有改动")
	}
	want := map[string]any{"alert": "notify", "notifyStart": false, "keepAwake": true, "keepAwakeSec": 20.0, "impatientMin": 3.0}
	if got := c.PluginConfig["agent-cat"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("搬过去的不对：%v", got)
	}
	if c.Alert != "" || c.NotifyStart != nil || c.KeepAwake || c.ImpatientMin != nil {
		t.Fatalf("旧字段没清掉：%+v", c)
	}
	if c.MigrateLegacy() {
		t.Fatal("搬过一次就不该再有改动")
	}

	off := false
	c = Config{Notify: &off, Alert: "wake"}
	c.MigrateLegacy()
	if c.PluginConfig["agent-cat"]["alert"] != "off" {
		t.Fatalf("关了提醒的应该是 off：%v", c.PluginConfig)
	}
}
