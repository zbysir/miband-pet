// Package ux 把快应用的 .ux 文件编译成手环上能直接跑的 JS，代替小米的 aiot-toolkit（它要装 Node）。
//
// 编出来的格式照着 aiot-toolkit 2.0.5 编出来的样子（server/internal/ux/testdata/*.aiot.js 是它的结果，
// 测试在 goja 里把两边的页面都跑一遍，比较画出来的元素树、样式表和点击时调的方法）：
//
//   - 页面 pages/<名字>/index.js：模板函数（aiot.__ce__ 这些调用）、样式表、脚本，包在一个固定的外壳里。
//   - app.js：应用的脚本、manifest、页面动态样式要用的 $translateStyle$。
package ux

import (
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
)

// File 是 .ux 文件拆出来的三段
type File struct {
	Template string
	Script   string
	Style    string
}

var blockRe = regexp.MustCompile(`(?s)<(template|script|style)(\s[^>]*)?>(.*?)</(template|script|style)>`)

// Parse 拆出 <template>、<script>、<style>
func Parse(src string) (File, error) {
	var f File
	// <template> 里可能嵌套同名标签（没有），但 <script> 里可能出现 '</template>' 字样的概率很低；按最外层找
	src = stripHTMLComments(src)
	if i := strings.Index(src, "<template"); i >= 0 {
		j := strings.LastIndex(src, "</template>")
		if j < i {
			return f, fmt.Errorf("<template> 没有结束")
		}
		open := strings.IndexByte(src[i:], '>')
		f.Template = src[i+open+1 : j]
		src = src[:i] + src[j+len("</template>"):]
	}
	for _, m := range blockRe.FindAllStringSubmatch(src, -1) {
		switch m[1] {
		case "script":
			f.Script = m[3]
		case "style":
			f.Style = m[3]
		}
	}
	return f, nil
}

func stripHTMLComments(s string) string {
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+j+3:]
	}
}

func jsString(s string) string { return strconv.Quote(s) }

// Page 是编好的页面
type Page struct {
	JS       string
	Features []string // 用到的快应用接口，如 system.vibrator
}

// CompilePage 编译一个页面。file 是 .ux 在 fsys 里的路径（如 pages/index/index.ux），页面里 import 的文件也从 fsys 读
func CompilePage(fsys fs.FS, file string, opts Options) (Page, error) {
	raw, err := fs.ReadFile(fsys, file)
	if err != nil {
		return Page{}, err
	}
	return compilePageSource(fsys, file, string(raw), opts)
}

// compilePageSource 编译页面源码 src（当作 fsys 里的 file，import 的相对路径从它算）
func compilePageSource(fsys fs.FS, file, src string, opts Options) (Page, error) {
	f, err := Parse(src)
	if err != nil {
		return Page{}, fmt.Errorf("%s：%w", file, err)
	}
	if strings.TrimSpace(f.Template) == "" {
		return Page{}, fmt.Errorf("%s：没有 <template>", file)
	}
	dir := pathDir(file)
	tpl, err := compileTemplate(f.Template, dir)
	if err != nil {
		return Page{}, fmt.Errorf("%s 的 <template>：%w", file, err)
	}
	rules, err := parseStyleSheet(f.Style)
	if err != nil {
		return Page{}, fmt.Errorf("%s 的 <style>：%w", file, err)
	}
	script, features, err := compileScript(fsys, file, f.Script, true, opts)
	if err != nil {
		return Page{}, fmt.Errorf("%s 的 <script>：%w", file, err)
	}
	body := "var $app_style$ = " + styleSheetJS(rules) + ";\n" +
		"var $app_script$ = " + script + ";\n" +
		"var $app_template$ = " + tpl + ";\n" +
		"$app_exports$['entry'] = function ($app_exports$) {\n" +
		"  $app_script$({}, $app_exports$, $app_require$);\n" +
		"  $app_exports$.default.template = $app_template$;\n" +
		"  $app_exports$.default.style = $app_style$;\n" +
		"};\n"
	return Page{JS: wrap("createPageHandler", body), Features: features}, nil
}

// CompileApp 编译 app.js：script 是 app.ux 的脚本（可以为空），manifest 是最终的 manifest.json 内容
func CompileApp(fsys fs.FS, file, script string, manifest []byte, opts Options) (string, []string, error) {
	var features []string
	body := "var $app_style$ = [];\n"
	if strings.TrimSpace(script) != "" {
		s, fs, err := compileScript(fsys, file, script, false, opts)
		if err != nil {
			return "", nil, err
		}
		features = fs
		body += "var $app_script$ = " + s + ";\n$app_script$({}, $app_exports$, $app_require$);\n"
	} else {
		body += "$app_exports$.default = {};\n"
	}
	body += "$app_exports$.default.style = $app_style$;\n" +
		"$app_exports$.default.manifest = " + string(manifest) + ";\n" +
		translateStyle
	return wrap("createAppHandler", body), features, nil
}

func pathDir(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

// 外壳照抄 aiot-toolkit（aiotpack 的 WrapPlugin）：手环按这个签名加载页面和 app.js
func wrap(name, body string) string {
	return `export default function(global, globalThis, window, $app_exports$, $app_evaluate$) {
  var org_app_require = $app_require$;
  (function(global, globalThis, window, $app_exports$, $app_evaluate$) {
    var setTimeout = global.setTimeout;
    var setInterval = global.setInterval;
    var clearTimeout = global.clearTimeout;
    var clearInterval = global.clearInterval;
    var $app_require$ = global.$app_require$ || org_app_require;
    var ` + name + ` = function() {
      return (function () {
` + body + `
      })();
    };
    return ` + name + `();
  })(global, globalThis, window, $app_exports$, $app_evaluate$);
}
`
}

// 页面上动态的 style="…{{}}…" 由它转成对象，app.js 里定义（照抄 aiot-toolkit）
const translateStyle = `
var $translateStyle$ = function (value) {
  if (typeof value === 'string') {
    return Object.fromEntries(
      value.split(';').filter(function (item) { return Boolean(item && item.trim()); }).map(function (item) {
        var m = item.match(/([^:]+):(.*)/);
        if (m && m.length > 2) {
          return [m[1].trim().replace(/-([a-z])/g, function (_, c) { return c.toUpperCase(); }), m[2].trim()];
        }
        return [];
      })
    );
  }
  return value;
};
globalThis.$translateStyle$ = $translateStyle$;
`
