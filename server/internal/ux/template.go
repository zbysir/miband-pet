package ux

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// 模板：<template> 里的元素树转成一串 aiot.__ce__ / __ci__ / __cf__ 调用，和 aiot-toolkit 编出来的一样：
//
//	aiot.__ce__("div", {__vm__: _vm_, __opts__: {classList: [...], style: …, events: {click: …}}}, [子元素])
//	aiot.__ci__({__vm__: _vm_, __opts__: {shown: function () {…}}}, function () { return [元素] })        if / elif / else
//	aiot.__cf__({__vm__: _vm_, __opts__: {exp: …, key: "$idx", value: "$item"}}, function ($idx, $item) { return [元素] })   for
//
// 用到数据的值都包成函数（框架靠它们知道依赖了什么、数据变了重算哪个），不变的直接写值。

// 可以直接写文字的元素
var textTags = map[string]bool{"text": true, "span": true, "a": true, "arc-text": true, "richtext": true, "marquee": true}

type templateCompiler struct {
	dir string // 页面所在目录（相对 app 根目录），解析相对路径的图片用
}

func parseTemplate(src string) (*html.Node, error) {
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(src), ctx)
	if err != nil {
		return nil, err
	}
	var roots []*html.Node
	for _, n := range nodes {
		if n.Type == html.ElementNode {
			roots = append(roots, n)
		} else if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
			return nil, fmt.Errorf("<template> 里只能有一个根元素，不能直接写文字：%q", strings.TrimSpace(n.Data))
		}
	}
	if len(roots) != 1 {
		return nil, fmt.Errorf("<template> 里要有且只有一个根元素，现在有 %d 个", len(roots))
	}
	return roots[0], nil
}

func attr(n *html.Node, name string) (string, bool) {
	for _, a := range n.Attr {
		if strings.SplitN(a.Key, ".", 2)[0] == name {
			return a.Val, true
		}
	}
	return "", false
}

func isElement(n *html.Node) bool { return n.Type == html.ElementNode }

// 动态值：含 {{ }}
func isDynamic(v string) bool {
	i := strings.Index(v, "{{")
	return i >= 0 && strings.Contains(v[i:], "}}")
}

// 去掉外面的 {{ }}（if、for 的值可以不写 {{ }}）
func unwrap(v string) string {
	t := strings.TrimSpace(v)
	if strings.HasPrefix(t, "{{") && strings.HasSuffix(t, "}}") && strings.Count(t, "{{") == 1 {
		return strings.TrimSpace(t[2 : len(t)-2])
	}
	return v
}

// 文字和 {{表达式}} 混写的值 → JS 表达式（各段用 + 连起来）；dynamic 表示有没有表达式
func (c *templateCompiler) value(v string, locals map[string]bool) (code string, dynamic bool, err error) {
	var parts []string
	var codes int
	rest := v
	for rest != "" {
		i := strings.Index(rest, "{{")
		if i < 0 {
			parts = append(parts, strconv.Quote(rest))
			break
		}
		j := strings.Index(rest[i+2:], "}}")
		if j < 0 {
			parts = append(parts, strconv.Quote(rest))
			break
		}
		if i > 0 {
			parts = append(parts, strconv.Quote(rest[:i]))
		}
		expr := strings.TrimSpace(rest[i+2 : i+2+j])
		if expr == "" {
			return "", false, fmt.Errorf("{{}} 里是空的：%q", v)
		}
		js, err := rewriteExpr(expr, locals)
		if err != nil {
			return "", false, err
		}
		parts = append(parts, "\x00"+js)
		codes++
		rest = rest[i+2+j+2:]
	}
	for k, p := range parts {
		if strings.HasPrefix(p, "\x00") {
			p = p[1:]
			if len(parts) > 1 {
				p = "(" + p + ")"
			}
			parts[k] = p
		}
	}
	if len(parts) == 0 {
		return `""`, false, nil
	}
	return strings.Join(parts, " + "), codes > 0, nil
}

// 文字内容：首尾空白去掉，中间连续空白换成一个空格
func normalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func textContent(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		} else if isElement(c) && textTags[c.Data] {
			b.WriteString(textContent(c))
		}
	}
	return normalizeText(b.String())
}

func fn(body string) string { return "function () { return " + body + "; }" }

func copyLocals(m map[string]bool, add ...string) map[string]bool {
	out := make(map[string]bool, len(m)+len(add))
	for k := range m {
		out[k] = true
	}
	for _, k := range add {
		out[k] = true
	}
	return out
}

// node 编译一个元素（连同它的 for / if），返回放进父元素子列表里的若干项（用 ", " 连好的）
func (c *templateCompiler) node(n *html.Node, locals map[string]bool) (string, error) {
	inner := locals
	var forItem, forIndex, forList string
	forVal, hasFor := attr(n, "for")
	if hasFor {
		var err error
		forIndex, forItem, forList, err = parseFor(unwrap(forVal))
		if err != nil {
			return "", err
		}
		inner = copyLocals(locals, forIndex, forItem)
	}

	var body string
	if n.Data == "block" {
		kids, err := c.children(n, inner)
		if err != nil {
			return "", err
		}
		body = kids
	} else {
		el, err := c.element(n, inner)
		if err != nil {
			return "", err
		}
		body = el
	}

	if hasFor {
		list, err := rewriteExpr(forList, inner)
		if err != nil {
			return "", fmt.Errorf("for=%q：%w", forVal, err)
		}
		exp := fn(list)
		if tid, ok := attr(n, "tid"); ok {
			if isDynamic(tid) {
				return "", fmt.Errorf("tid 要写成固定的名字，不能用 {{}}：%q", tid)
			}
			exp = fn("{__list__: " + list + ", __tid__: " + strconv.Quote(tid) + "}")
		}
		body = fmt.Sprintf("aiot.__cf__({__vm__: _vm_, __opts__: {exp: %s, key: %s, value: %s}}, function (%s, %s) { return [%s]; })",
			exp, strconv.Quote(forIndex), strconv.Quote(forItem), forIndex, forItem, body)
	}

	cond, hasCond, err := c.condition(n, locals)
	if err != nil {
		return "", err
	}
	if hasCond {
		body = fmt.Sprintf("aiot.__ci__({__vm__: _vm_, __opts__: {shown: %s}}, function () { return [%s]; })", fn(cond), body)
	}
	return body, nil
}

// for 的写法：list、item in list、(index, item) in list
func parseFor(v string) (index, item, list string, err error) {
	index, item = "$idx", "$item"
	v = strings.TrimSpace(v)
	toks, err := tokenize(v)
	if err != nil {
		return "", "", "", err
	}
	// 找顶层的 in
	depth := 0
	pos := 0
	for _, t := range toks {
		if t.kind == tPunct {
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			}
		}
		if t.kind == tIdent && t.text == "in" && depth == 0 {
			left := strings.TrimSpace(v[:pos])
			list = strings.TrimSpace(v[pos+2:])
			if strings.HasPrefix(left, "(") && strings.HasSuffix(left, ")") {
				names := strings.Split(left[1:len(left)-1], ",")
				switch len(names) {
				case 1:
					index = strings.TrimSpace(names[0]) // 和 aiot-toolkit 一样：(x) in list 的 x 是下标
				case 2:
					index, item = strings.TrimSpace(names[0]), strings.TrimSpace(names[1])
				default:
					return "", "", "", fmt.Errorf("for 的写法不对：%q", v)
				}
			} else {
				item = left
			}
			for _, name := range []string{index, item} {
				if name == "" || !isIdentStart(name[0]) {
					return "", "", "", fmt.Errorf("for 的写法不对：%q（应该像 item in list、(index, item) in list）", v)
				}
			}
			return index, item, list, nil
		}
		pos += len(t.text)
	}
	return index, item, v, nil
}

// if / elif / else 的条件：前面同一串里的条件都不成立，且自己的成立
func (c *templateCompiler) condition(n *html.Node, locals map[string]bool) (string, bool, error) {
	ifVal, isIf := attr(n, "if")
	elifVal, isElif := attr(n, "elif")
	_, isElse := attr(n, "else")
	if !isIf && !isElif && !isElse {
		return "", false, nil
	}
	cond := func(v string) (string, error) {
		js, err := rewriteExpr(unwrap(v), locals)
		if err != nil {
			return "", fmt.Errorf("条件 %q：%w", v, err)
		}
		return js, nil
	}
	if isIf {
		js, err := cond(ifVal)
		return "(" + js + ")", true, err
	}
	var parts []string
	for p := n.PrevSibling; p != nil; p = p.PrevSibling {
		if p.Type == html.TextNode && strings.TrimSpace(p.Data) == "" || p.Type == html.CommentNode {
			continue
		}
		if !isElement(p) {
			return "", false, fmt.Errorf("elif / else 前面要紧挨着 if 或 elif 的元素")
		}
		if v, ok := attr(p, "if"); ok {
			js, err := cond(v)
			if err != nil {
				return "", false, err
			}
			parts = append(parts, "(!("+js+"))")
			break
		}
		v, ok := attr(p, "elif")
		if !ok {
			return "", false, fmt.Errorf("<%s> 的 elif / else 前面要紧挨着 if 或 elif 的元素", n.Data)
		}
		js, err := cond(v)
		if err != nil {
			return "", false, err
		}
		parts = append(parts, "(!("+js+"))")
		if p.PrevSibling == nil {
			return "", false, fmt.Errorf("<%s> 的 elif / else 前面没有 if", n.Data)
		}
	}
	if len(parts) == 0 {
		return "", false, fmt.Errorf("<%s> 的 elif / else 前面没有 if", n.Data)
	}
	if isElif {
		js, err := cond(elifVal)
		if err != nil {
			return "", false, err
		}
		parts = append(parts, "("+js+")")
	}
	return strings.Join(parts, " && "), true, nil
}

// children 编译子元素列表
func (c *templateCompiler) children(n *html.Node, locals map[string]bool) (string, error) {
	var parts []string
	hasEl := false
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		if isElement(k) {
			hasEl = true
		}
	}
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		switch {
		case isElement(k):
			s, err := c.node(k, locals)
			if err != nil {
				return "", err
			}
			if s != "" {
				parts = append(parts, s)
			}
		case k.Type == html.TextNode && textTags[n.Data] && hasEl && strings.TrimSpace(k.Data) != "":
			// 文字元素里文字和元素混写：每段文字包成一个 span
			code, dyn, err := c.value(normalizeText(k.Data), locals)
			if err != nil {
				return "", err
			}
			if dyn {
				code = fn(code)
			}
			parts = append(parts, `aiot.__ce__("span", {__vm__: _vm_, __opts__: {value: `+code+`}}, [])`)
		case k.Type == html.TextNode && strings.TrimSpace(k.Data) != "" && !textTags[n.Data]:
			return "", fmt.Errorf("<%s> 里不能直接写文字 %q，要放进 <text>", n.Data, strings.TrimSpace(k.Data))
		}
	}
	return strings.Join(parts, ",\n"), nil
}

// element 编译元素本身（不管 for / if）
func (c *templateCompiler) element(n *html.Node, locals map[string]bool) (string, error) {
	tag := n.Data
	if tag == "img" {
		tag = "image"
	}
	var opts []string
	var events, dataset []string
	for _, a := range n.Attr {
		name := strings.SplitN(a.Key, ".", 2)[0]
		switch name {
		case "if", "elif", "else", "for", "tid", "static":
			continue
		}
		val := a.Val
		switch {
		case strings.HasPrefix(name, "@") || strings.HasPrefix(name, "on"):
			ev := strings.TrimPrefix(strings.TrimPrefix(name, "@"), "on")
			h, err := eventHandler(val, copyLocals(locals, "evt"))
			if err != nil {
				return "", fmt.Errorf("<%s %s>：%w", tag, a.Key, err)
			}
			events = append(events, ev+": "+h)
		case name == "class":
			code, dyn, err := c.value(val, locals)
			if err != nil {
				return "", err
			}
			if dyn {
				opts = append(opts, "classList: function () { var v = ("+code+"); return typeof v === 'string' ? v.split(' ').map(function (s) { return s.trim(); }).filter(Boolean) : v; }")
			} else {
				var names []string
				for _, s := range strings.Fields(val) {
					names = append(names, strconv.Quote(s))
				}
				opts = append(opts, "classList: ["+strings.Join(names, ", ")+"]")
			}
		case name == "style":
			code, dyn, err := c.value(val, locals)
			if err != nil {
				return "", err
			}
			if dyn {
				opts = append(opts, "style: function () { return globalThis.$translateStyle$("+code+"); }")
			} else {
				block, err := parseDecls(val)
				if err != nil {
					return "", fmt.Errorf("<%s style=%q>：%w", tag, val, err)
				}
				opts = append(opts, "style: "+block.js())
			}
		default:
			if tag == "image" && name == "src" && !isDynamic(val) {
				val = c.resolve(val)
			}
			var code string
			if val == "" {
				code = "true"
			} else {
				js, dyn, err := c.value(val, locals)
				if err != nil {
					return "", err
				}
				code = js
				if dyn {
					code = fn(js)
				}
			}
			if strings.HasPrefix(name, "data-") {
				dataset = append(dataset, camel(strings.TrimPrefix(name, "data-"))+": "+code)
			} else {
				opts = append(opts, strconv.Quote(camel(name))+": "+code)
			}
		}
	}
	if len(dataset) > 0 {
		opts = append(opts, "dataset: {"+strings.Join(dataset, ", ")+"}")
	}
	if len(events) > 0 {
		opts = append(opts, "events: {"+strings.Join(events, ", ")+"}")
	}

	hasEl := false
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		if isElement(k) {
			hasEl = true
		}
	}
	if textTags[tag] && !hasEl {
		if t := textContent(n); t != "" {
			code, dyn, err := c.value(t, locals)
			if err != nil {
				return "", err
			}
			if dyn {
				code = fn(code)
			}
			opts = append(opts, "value: "+code)
		}
	}
	kids, err := c.children(n, locals)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("aiot.__ce__(%s, {__vm__: _vm_, __opts__: {%s}}, [%s])", strconv.Quote(tag), strings.Join(opts, ", "), kids), nil
}

// 图片的相对路径换成从 app 根目录算的绝对路径
func (c *templateCompiler) resolve(src string) string {
	if src == "" || strings.HasPrefix(src, "/") || strings.Contains(src, "://") || strings.HasPrefix(src, "data:") {
		return src
	}
	return "/" + path.Join(c.dir, src)
}

// 事件的写法：方法名、方法名(参数)、箭头函数、function(){}
func eventHandler(v string, locals map[string]bool) (string, error) {
	v = strings.TrimSpace(unwrap(v))
	if v == "" {
		return "", fmt.Errorf("事件没写处理方法")
	}
	toks, err := tokenize(v)
	if err != nil {
		return "", err
	}
	var sig []token
	for _, t := range toks {
		if t.kind != tSpace {
			sig = append(sig, t)
		}
	}
	if sig[0].kind == tIdent && sig[0].text == "function" {
		return rewriteExpr(v, locals)
	}
	for _, t := range sig {
		if t.kind == tPunct && t.text == "=>" {
			return rewriteExpr(v, locals)
		}
	}
	// 方法名(参数)：最后是 )，和它配对的 ( 前面是方法名
	if last := sig[len(sig)-1]; last.kind == tPunct && last.text == ")" {
		depth := 0
		open := -1
		for i := len(v) - 1; i >= 0; i-- {
			switch v[i] {
			case ')':
				depth++
			case '(':
				depth--
			}
			if depth == 0 {
				open = i
				break
			}
		}
		if open > 0 {
			callee, err := rewriteExpr(v[:open], locals)
			if err != nil {
				return "", err
			}
			args := strings.TrimSpace(v[open+1 : len(v)-1])
			if args != "" {
				a, err := rewriteExpr(args, locals)
				if err != nil {
					return "", err
				}
				args = a + ", evt"
			} else {
				args = "evt"
			}
			return "function (evt) { return " + callee + "(" + args + "); }", nil
		}
	}
	callee, err := rewriteExpr(v, locals)
	if err != nil {
		return "", err
	}
	return "function (evt) { return " + callee + "(evt); }", nil
}

// compileTemplate 把 <template> 的内容编译成模板函数
func compileTemplate(src, dir string) (string, error) {
	root, err := parseTemplate(src)
	if err != nil {
		return "", err
	}
	c := &templateCompiler{dir: dir}
	body, err := c.node(root, map[string]bool{})
	if err != nil {
		return "", err
	}
	return "function (vm) {\n  var _vm_ = vm || this;\n  return " + body + ";\n}", nil
}
