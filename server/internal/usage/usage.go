// Package usage 管 Claude Code 和 Codex 的额度（5 小时窗口、每周窗口用了百分之几、什么时候重置）。
//
// 读额度的是 Agent 猫插件的 server.js（server/plugins/agent-cat/server.js），这里只剩：
//   - Statusline：接管 Claude Code 的状态栏命令拿额度（App 里的开关，见 statusline.go）；
//   - Source / Window：server.js 推上来的数据的样子，App 界面显示用。
package usage

// Window 是一个额度窗口。
type Window struct {
	Name     string  `json:"name"`     // 5h / 7d / 额度（Claude apps gateway 的花费上限）等
	Used     float64 `json:"used"`     // 用了百分之几，0~100（花费上限可能超过 100）
	ResetsAt int64   `json:"resetsAt"` // 什么时候重置（Unix 秒），0 表示不知道
}

// Source 是一种 agent 的额度。
type Source struct {
	Agent   string   `json:"agent"` // claude / codex
	Windows []Window `json:"windows"`
	Updated int64    `json:"updated"` // 数据是什么时候的（Unix 秒）
}
