// Package quickapp 把一个快应用的源码目录（和 aiot-toolkit 的 src/ 一样：manifest.json、app.ux、pages/…/*.ux、图片）
// 编译打包成签好名的安装包（.rpk），不用 Node 和 aiot-toolkit。
package quickapp

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"miband/internal/rpk"
	"miband/internal/ux"
)

// 原样放进包里的文件（图片、字体、音频）；.ux 和脚本编译后才放
var assetExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true,
	".svg": true, ".ttf": true, ".otf": true, ".mp3": true, ".wav": true}

// 签名用的调试证书（和 render-app/sign/debug 是同一张）。手环只认同一个包名一直用同一张证书签的包，换了要先卸载
//
//go:embed sign/private.pem
var keyPEM []byte

//go:embed sign/certificate.pem
var certPEM []byte

// DebugSigner 返回用内置调试证书签名的 Signer
func DebugSigner() (*rpk.Signer, error) { return rpk.NewSigner(keyPEM, certPEM) }

// Build 编译 src（快应用源码目录）并签名，返回安装包和最终的 manifest
func Build(src fs.FS, s *rpk.Signer) ([]byte, map[string]any, error) {
	files, manifest, err := Compile(src)
	if err != nil {
		return nil, nil, err
	}
	data, err := rpk.Build(files, "", s)
	return data, manifest, err
}

// Compile 编译 src，返回要放进安装包的文件（还没签名）
func Compile(src fs.FS) ([]rpk.File, map[string]any, error) {
	raw, err := fs.ReadFile(src, "manifest.json")
	if err != nil {
		return nil, nil, fmt.Errorf("没有 manifest.json：%w", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, nil, fmt.Errorf("manifest.json 写得不对：%w", err)
	}
	router, _ := manifest["router"].(map[string]any)
	pages, _ := router["pages"].(map[string]any)
	if len(pages) == 0 {
		return nil, nil, fmt.Errorf("manifest.json 的 router.pages 是空的")
	}
	names := make([]string, 0, len(pages))
	for name := range pages {
		names = append(names, name)
	}
	sort.Strings(names)

	features := map[string]bool{}
	if list, ok := manifest["features"].([]any); ok {
		for _, f := range list {
			if m, ok := f.(map[string]any); ok {
				if n, ok := m["name"].(string); ok {
					features[n] = true
				}
			}
		}
	}
	var out []rpk.File
	for _, name := range names {
		conf, _ := pages[name].(map[string]any)
		comp, _ := conf["component"].(string)
		if comp == "" {
			comp = "index"
		}
		page, err := ux.CompilePage(src, path.Join(name, comp+".ux"), ux.Options{})
		if err != nil {
			return nil, nil, err
		}
		for _, f := range page.Features {
			features[f] = true
		}
		out = append(out, rpk.File{Name: path.Join(name, comp+".js"), Body: []byte(page.JS)})
	}

	var appScript string
	if raw, err := fs.ReadFile(src, "app.ux"); err == nil {
		f, err := ux.Parse(string(raw))
		if err != nil {
			return nil, nil, fmt.Errorf("app.ux：%w", err)
		}
		appScript = f.Script
	}
	assets, err := Assets(src)
	if err != nil {
		return nil, nil, err
	}
	files, err := Assemble(src, manifest, appScript, out, features, assets, ux.Options{})
	return files, manifest, err
}

// Assets 是 fsys 里要原样放进安装包的文件（图片、字体、音频），路径不变
func Assets(fsys fs.FS) ([]rpk.File, error) {
	var out []rpk.File
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !assetExts[strings.ToLower(path.Ext(p))] {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		out = append(out, rpk.File{Name: p, Body: b})
		return nil
	})
	return out, err
}

// Assemble 组装安装包里的全部文件：manifest、app.js、编好的页面、原样放的文件。
// features 是页面用到的快应用接口，app.ux 的脚本（appScript，可以为空）用到的会补进去，最后写进 manifest。
func Assemble(fsys fs.FS, manifest map[string]any, appScript string, pages []rpk.File, features map[string]bool, assets []rpk.File, opts ux.Options) ([]rpk.File, error) {
	// app.js 要带上最终的 manifest，所以先编一遍拿到它用的接口，补进 features 再编
	_, fs2, err := ux.CompileApp(fsys, "app.ux", appScript, []byte("{}"), opts)
	if err != nil {
		return nil, fmt.Errorf("app.ux：%w", err)
	}
	for _, f := range fs2 {
		features[f] = true
	}
	var fnames []string
	for f := range features {
		fnames = append(fnames, f)
	}
	sort.Strings(fnames)
	list := []any{}
	for _, f := range fnames {
		list = append(list, map[string]any{"name": f})
	}
	manifest["features"] = list
	mjson, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	appJS, _, err := ux.CompileApp(fsys, "app.ux", appScript, mjson, opts)
	if err != nil {
		return nil, fmt.Errorf("app.ux：%w", err)
	}

	// 和 aiot-toolkit 打的一样：manifest.json 多一段 packageInfo，最后放 META-INF/build.txt
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	full := map[string]any{}
	for k, v := range manifest {
		full[k] = v
	}
	full["packageInfo"] = map[string]any{"toolkit": "2.0.5", "timeStamp": stamp, "node": "v24.13.0",
		"platform": "darwin", "arch": "arm64", "component": true}
	fjson, err := json.MarshalIndent(full, "", "  ")
	if err != nil {
		return nil, err
	}
	out := []rpk.File{
		{Name: "manifest-watch.json", Body: mjson},
		{Name: "manifest.json", Body: fjson},
		{Name: "app.js", Body: []byte(appJS)},
	}
	out = append(out, pages...)
	out = append(out, assets...)
	out = append(out, rpk.File{Name: "META-INF/build.txt", Body: []byte("originType=undefined\ntoolkit=2.0.5\ntimeStamp=" + stamp +
		"\nnode=v24.13.0\nplatform=darwin\narch=arm64\ncomponent=true")})
	return out, nil
}
