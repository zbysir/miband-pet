package ux

import (
	"fmt"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// 模板表达式（{{ }} 里面、if、for、事件）改写成页面上能跑的 JS：用到的数据和方法前面加 _vm_.，
// 和 aiot-toolkit 一样。不加前缀的：locals（for 的循环变量、事件里的 evt）、箭头函数的参数、
// JS 的全局对象（Math、JSON…）、属性名（a.b 的 b）、对象字面量的键。
//
// 只做词法扫描，不建语法树：模板里的表达式都很短，够用了。

// 不加前缀的全局名字（aiot-toolkit 的规则是「在 Node 的 globalThis 上有的」，这里列常用的）
var globalNames = map[string]bool{
	"Math": true, "JSON": true, "Date": true, "String": true, "Number": true, "Boolean": true, "Array": true,
	"Object": true, "parseInt": true, "parseFloat": true, "isNaN": true, "isFinite": true, "console": true,
	"encodeURIComponent": true, "decodeURIComponent": true, "encodeURI": true, "decodeURI": true, "RegExp": true,
	"Error": true, "Symbol": true, "Map": true, "Set": true, "Promise": true, "globalThis": true, "Intl": true,
	"Infinity": true, "NaN": true, "undefined": true, "setTimeout": true, "clearTimeout": true,
	"setInterval": true, "clearInterval": true,
}

// 关键字和字面量
var keywords = map[string]bool{
	"true": true, "false": true, "null": true, "this": true, "typeof": true, "instanceof": true, "in": true,
	"new": true, "void": true, "delete": true, "of": true, "function": true, "return": true, "var": true,
	"let": true, "const": true, "if": true, "else": true, "async": true, "await": true,
}

type tokKind int

const (
	tSpace tokKind = iota
	tIdent
	tNumber
	tString
	tTemplate // 模板字符串，内容另行处理
	tPunct
)

type token struct {
	kind tokKind
	text string
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

var puncts = []string{"=>", "?.", "??", "===", "!==", "==", "!=", "<=", ">=", "&&", "||", "...", "**"}

func tokenize(s string) ([]token, error) {
	var out []token
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			j := i
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			out = append(out, token{tSpace, s[i:j]})
			i = j
		case isIdentStart(c):
			j := i
			for j < len(s) && isIdentPart(s[j]) {
				j++
			}
			out = append(out, token{tIdent, s[i:j]})
			i = j
		case c >= '0' && c <= '9' || (c == '.' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9'):
			j := i
			for j < len(s) && (isIdentPart(s[j]) || s[j] == '.') {
				j++
			}
			out = append(out, token{tNumber, s[i:j]})
			i = j
		case c == '\'' || c == '"':
			j := i + 1
			for j < len(s) && s[j] != c {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(s) {
				return nil, fmt.Errorf("字符串没有结束：%s", s[i:])
			}
			out = append(out, token{tString, s[i : j+1]})
			i = j + 1
		case c == '`':
			j := i + 1
			depth := 0
			for j < len(s) {
				if s[j] == '\\' {
					j += 2
					continue
				}
				if depth == 0 && s[j] == '`' {
					break
				}
				if s[j] == '$' && j+1 < len(s) && s[j+1] == '{' {
					depth++
					j += 2
					continue
				}
				if depth > 0 && s[j] == '}' {
					depth--
				}
				j++
			}
			if j >= len(s) {
				return nil, fmt.Errorf("模板字符串没有结束：%s", s[i:])
			}
			out = append(out, token{tTemplate, s[i : j+1]})
			i = j + 1
		default:
			p := string(c)
			for _, q := range puncts {
				if strings.HasPrefix(s[i:], q) {
					if len(q) > len(p) {
						p = q
					}
				}
			}
			// a?.5 这种是三元表达式，不是可选链
			if p == "?." && i+2 < len(s) && s[i+2] >= '0' && s[i+2] <= '9' {
				p = "?"
			}
			out = append(out, token{tPunct, p})
			i += len(p)
		}
	}
	return out, nil
}

// rewriteExpr 给表达式里的数据加上 _vm_. 前缀。locals 里的名字不加。
func rewriteExpr(expr string, locals map[string]bool) (string, error) {
	if err := checkExpr(expr); err != nil {
		return "", err
	}
	toks, err := tokenize(expr)
	if err != nil {
		return "", err
	}
	// 只看有意义的 token（去掉空白）的下标
	var sig []int
	for i, t := range toks {
		if t.kind != tSpace {
			sig = append(sig, i)
		}
	}
	at := func(k int) *token {
		if k < 0 || k >= len(sig) {
			return nil
		}
		return &toks[sig[k]]
	}
	is := func(k int, p string) bool {
		t := at(k)
		return t != nil && t.kind == tPunct && t.text == p
	}
	// 括号配对
	match := map[int]int{}
	var stack []int
	for k := range sig {
		t := at(k)
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			stack = append(stack, k)
		case ")", "]", "}":
			if len(stack) == 0 {
				return "", fmt.Errorf("括号不配对：%s", expr)
			}
			match[stack[len(stack)-1]] = k
			match[k] = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) > 0 {
		return "", fmt.Errorf("括号不配对：%s", expr)
	}
	scope := map[string]bool{}
	for k, v := range locals {
		scope[k] = v
	}
	// 箭头函数的参数：x => …、(a, b) => …
	params := map[int]bool{} // 是参数的 token（sig 下标）
	for k := range sig {
		if !is(k, "=>") {
			continue
		}
		if t := at(k - 1); t != nil && t.kind == tIdent {
			params[k-1] = true
			scope[t.text] = true
		} else if is(k-1, ")") {
			open := match[k-1]
			depth := 0
			for j := open + 1; j < k-1; j++ {
				t := at(j)
				if t.kind == tPunct && (t.text == "(" || t.text == "[" || t.text == "{") {
					depth++
				} else if t.kind == tPunct && (t.text == ")" || t.text == "]" || t.text == "}") {
					depth--
				}
				// 参数名：在参数列表顶层，前面是 ( 或 , 或 ...
				if t.kind == tIdent && depth == 0 && (j == open+1 || is(j-1, ",") || is(j-1, "...")) {
					params[j] = true
					scope[t.text] = true
				}
			}
		}
	}
	// 每个 { 是不是对象字面量（表达式里的 { 除了箭头函数体都是对象）
	objectBrace := map[int]bool{}
	for k := range sig {
		if is(k, "{") && !is(k-1, "=>") {
			objectBrace[k] = true
		}
	}
	// 当前位置最里层的括号
	enclosing := func(k int) int {
		depth := 0
		for j := k - 1; j >= 0; j-- {
			t := at(j)
			if t.kind != tPunct {
				continue
			}
			switch t.text {
			case ")", "]", "}":
				depth++
			case "(", "[", "{":
				if depth == 0 {
					return j
				}
				depth--
			}
		}
		return -1
	}

	repl := map[int]string{} // toks 下标 → 替换成的文字
	for k := range sig {
		t := at(k)
		switch t.kind {
		case tTemplate:
			s, err := rewriteTemplateLiteral(t.text, scope)
			if err != nil {
				return "", err
			}
			repl[sig[k]] = s
			continue
		case tIdent:
		default:
			continue
		}
		name := t.text
		if keywords[name] || params[k] {
			continue
		}
		if is(k-1, ".") || is(k-1, "?.") {
			continue
		}
		inObject := false
		if e := enclosing(k); e >= 0 && objectBrace[e] && (is(k-1, "{") || is(k-1, ",")) {
			inObject = true
		}
		if inObject && is(k+1, ":") {
			continue // 对象的键
		}
		if scope[name] || globalNames[name] {
			continue
		}
		if inObject && (is(k+1, ",") || is(k+1, "}")) {
			repl[sig[k]] = name + ": _vm_." + name // 简写 {x}
			continue
		}
		repl[sig[k]] = "_vm_." + name
	}
	var b strings.Builder
	for i, t := range toks {
		if r, ok := repl[i]; ok {
			b.WriteString(r)
		} else {
			b.WriteString(t.text)
		}
	}
	return b.String(), nil
}

// 模板字符串里 ${} 的内容也要改写
func rewriteTemplateLiteral(s string, scope map[string]bool) (string, error) {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteString(s[i : i+2])
			i += 2
			continue
		}
		if s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			depth := 1
			j := i + 2
			for j < len(s) && depth > 0 {
				switch s[j] {
				case '{':
					depth++
				case '}':
					depth--
				}
				j++
			}
			inner, err := rewriteExpr(s[i+2:j-1], scope)
			if err != nil {
				return "", err
			}
			b.WriteString("${" + inner + "}")
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String(), nil
}

// checkExpr 检查表达式的语法（交给 esbuild 解析一遍），写错了在编译时就报出来，不用等到手环上
func checkExpr(expr string) error {
	res := api.Transform("(\n"+expr+"\n)", api.TransformOptions{Loader: api.LoaderJS, LogLevel: api.LogLevelSilent})
	if len(res.Errors) > 0 {
		return fmt.Errorf("表达式 %q 写得不对：%s", strings.TrimSpace(expr), res.Errors[0].Text)
	}
	return nil
}
