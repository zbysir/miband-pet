// Package plugins 是打包进 App 的内置插件，每个子目录一个（格式见 docs/plugins.md）。
package plugins

import "embed"

//go:embed all:agent-cat
var FS embed.FS
