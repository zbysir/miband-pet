// Package plugin 是 Mac 这边的插件宿主：编译插件（JSX → JS），把代码、图片和数据（agents、设置）推给手环上的
// 渲染器（render-app）。插件的界面在手环上跑：点击、useState 都在手环本地处理，不经过 Mac。
// 插件还可以带一个 mac.js 在 Mac 上跑（见 mac.go）：处理手环发来的 call、推数据给手环、读文件、发通知。
//
// 一个插件是一个目录：manifest.json + index.jsx（可以再 import 同目录的其它文件）+ png 图片 + 可选的 mac.js。
// 内置插件打包在 App 里（server/plugins），用户插件放在 ~/Library/Application Support/miband-pet/plugins/<名字>/，
// 同名的用户插件会替换内置插件。写插件的说明见 docs/plugins.md。
package plugin

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// Manifest 是插件目录下的 manifest.json。
type Manifest struct {
	Name  string `json:"name"`  // 唯一名字，同名的用户插件替换内置插件
	Title string `json:"title"` // 显示名
	Order int    `json:"order"` // 在手环上的页序，小的在前；默认 100
	Main  string `json:"main"`  // 入口文件，默认 index.jsx
	// mac.js 要用的能力，用户插件要在 App 里允许后 mac.js 才会跑
	Permissions Permissions `json:"permissions"`
	// 配置项：App 照着画设置表单，值推给手环（useConfig()）和 mac.js（getConfig()）
	Settings []Setting `json:"settings"`
}

// Setting 是插件的一个配置项。
type Setting struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`              // select / bool / number / text
	Options     []Option `json:"options,omitempty"` // select 的选项
	Default     any      `json:"default"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// check 看 v 是不是这个配置项能接受的值，返回规整后的值
func (s Setting) check(v any) (any, error) {
	switch s.Type {
	case "bool":
		if b, ok := v.(bool); ok {
			return b, nil
		}
	case "number":
		if f, ok := v.(float64); ok {
			return f, nil
		}
	case "text":
		if str, ok := v.(string); ok {
			return str, nil
		}
	case "select":
		if str, ok := v.(string); ok {
			for _, o := range s.Options {
				if o.Value == str {
					return str, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("配置项 %s 不接受 %v", s.Key, v)
}

// Permissions 是 mac.js 能做的敏感的事。
type Permissions struct {
	Notify bool     `json:"notify,omitempty"` // 提醒：发通知（Mac 和手环）、点亮手环
	Read   []string `json:"read,omitempty"`   // 能读哪些目录下的文件（可以用 ~）
	Exec   []string `json:"exec,omitempty"`   // 能执行哪些命令（命令名，如 "herdr"）
}

// Empty：什么都没要
func (p Permissions) Empty() bool { return !p.Notify && len(p.Exec) == 0 && len(p.Read) == 0 }

// Key 用来记「用户允许过哪些权限」：权限变了要重新允许
func (p Permissions) Key() string {
	raw, _ := json.Marshal(p)
	return string(raw)
}

// Describe 给界面看的权限说明
func (p Permissions) Describe() []string {
	var out []string
	if p.Notify {
		out = append(out, "发通知、点亮手环")
	}
	for _, r := range p.Read {
		out = append(out, "读 "+r)
	}
	for _, c := range p.Exec {
		out = append(out, "运行 "+c)
	}
	return out
}

// Plugin 是编译好的插件。
type Plugin struct {
	Manifest
	Builtin bool
	Dir     string // 用户插件的目录；内置插件为空

	Code    string            // 代码的哈希（手环上按哈希缓存）
	Assets  map[string]string // 插件目录里的 png：路径 → 哈希
	Err     string            // 编译出错
	MacCode string            // 编译好的 mac.js；没有就是空
}

func readManifest(fsys fs.FS) (Manifest, error) {
	m := Manifest{Order: 100}
	raw, err := fs.ReadFile(fsys, "manifest.json")
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("manifest.json: %w", err)
	}
	if m.Name == "" {
		return m, errors.New("manifest.json 里缺 name")
	}
	if m.Title == "" {
		m.Title = m.Name
	}
	if m.Main == "" {
		m.Main = "index.jsx"
	}
	for _, s := range m.Settings {
		switch {
		case s.Key == "":
			return m, errors.New("manifest.json 的 settings 里有一项缺 key")
		case s.Type != "select" && s.Type != "bool" && s.Type != "number" && s.Type != "text":
			return m, fmt.Errorf("配置项 %s 的 type 只能是 select / bool / number / text", s.Key)
		case s.Type == "select" && len(s.Options) == 0:
			return m, fmt.Errorf("配置项 %s 是 select，要有 options", s.Key)
		}
		if _, err := s.check(s.Default); err != nil {
			return m, fmt.Errorf("配置项 %s 的默认值不对", s.Key)
		}
	}
	return m, nil
}

func hashOf(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:8])
}

// compile 编译插件，代码和图片登记到 blobs（哈希 → 内容）。编译出错记在 Err 里，手环上会显示出错页。
func compile(fsys fs.FS, dir string, builtin bool, blobs map[string]blob) (*Plugin, error) {
	m, err := readManifest(fsys)
	if err != nil {
		return nil, err
	}
	p := &Plugin{Manifest: m, Builtin: builtin, Dir: dir, Assets: map[string]string{}}
	code, err := build(fsys, m.Main)
	if err != nil {
		p.Err = "编译失败：" + err.Error()
		return p, nil
	}
	p.Code = hashOf([]byte(code))
	blobs[p.Code] = blob{ext: "js", data: []byte(code)}
	if _, err := fs.Stat(fsys, "mac.js"); err == nil {
		if p.MacCode, err = buildFor(fsys, "mac.js", "miband/mac"); err != nil {
			p.Err = "插件编译失败：" + err.Error()
			return p, nil
		}
	}
	_ = fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".png") {
			return nil
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil
		}
		h := hashOf(data)
		p.Assets[name] = h
		blobs[h] = blob{ext: "png", data: data}
		return nil
	})
	return p, nil
}

// build 用 esbuild 把入口和它 import 的文件打成一个 CommonJS 模块；'miband' 由手环上的运行时提供，
// JSX 编译成 __h(...)，手环加载插件时把 __h、__Fragment 传进去。
func build(fsys fs.FS, main string) (string, error) {
	return buildFor(fsys, main, "miband")
}

// buildFor：lib 是运行时提供的模块名（手环上是 miband，Mac 上是 miband/mac）
func buildFor(fsys fs.FS, main, lib string) (string, error) {
	exts := []string{"", ".jsx", ".js", ".json", "/index.jsx", "/index.js"}
	fsPlugin := api.Plugin{Name: "fsys", Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: `.*`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			if args.Path == lib {
				return api.OnResolveResult{Path: lib, External: true}, nil
			}
			p := args.Path
			if args.Kind != api.ResolveEntryPoint {
				if !strings.HasPrefix(p, "./") && !strings.HasPrefix(p, "../") {
					return api.OnResolveResult{}, fmt.Errorf("只能 import '%s' 或插件目录里的文件（./xxx），不支持 %q", lib, p)
				}
				p = path.Join(path.Dir(args.Importer), p)
			}
			p = path.Clean(strings.TrimPrefix(p, "/"))
			for _, ext := range exts {
				if st, err := fs.Stat(fsys, p+ext); err == nil && !st.IsDir() {
					return api.OnResolveResult{Path: p + ext, Namespace: "fsys"}, nil
				}
			}
			return api.OnResolveResult{}, fmt.Errorf("找不到 %s", p)
		})
		b.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "fsys"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			raw, err := fs.ReadFile(fsys, args.Path)
			if err != nil {
				return api.OnLoadResult{}, err
			}
			s := string(raw)
			loader := api.LoaderJSX // .js 里也允许写 JSX
			if path.Ext(args.Path) == ".json" {
				loader = api.LoaderJSON
			}
			return api.OnLoadResult{Contents: &s, Loader: loader}, nil
		})
	}}
	res := api.Build(api.BuildOptions{
		EntryPoints: []string{main},
		Bundle:      true,
		Write:       false,
		Format:      api.FormatCommonJS,
		Platform:    api.PlatformNeutral,
		Target:      api.ES2020, // 手环的 JS 引擎实测支持到 ES2020（docs/render-spike.md）
		JSX:         api.JSXTransform,
		JSXFactory:  "__h",
		JSXFragment: "__Fragment",
		Plugins:     []api.Plugin{fsPlugin},
		LogLevel:    api.LogLevelSilent,
	})
	if len(res.Errors) > 0 {
		var msgs []string
		for _, e := range res.Errors {
			loc := ""
			if e.Location != nil {
				loc = fmt.Sprintf("%s:%d:%d: ", e.Location.File, e.Location.Line, e.Location.Column)
			}
			msgs = append(msgs, loc+e.Text)
		}
		return "", errors.New(strings.Join(msgs, "\n"))
	}
	if len(res.OutputFiles) == 0 {
		return "", errors.New("esbuild 没有输出")
	}
	return string(res.OutputFiles[0].Contents), nil
}
