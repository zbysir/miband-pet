package ux

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// 样式：<style> 和静态的 style="…" 转成快应用要的样子，和 aiot-toolkit 一样：
//
//	[[[[选择器类型, 名字]], {属性: 值}], …]   选择器类型 0 是 .class，1 是 #id，2 是标签
//
// 属性名转成驼峰，没单位的数字转成数字，颜色规范成 #rrggbb / rgba(…)，padding、margin、border 这些简写展开成四个方向。
// 只支持单个 class / id / 标签做选择器（快应用不支持后代选择器）。

type styleDecl struct {
	name  string // 驼峰
	value any    // string 或 float64
}

// 一组声明，保持写的顺序（后写的同名属性覆盖前面的值，位置不变）
type styleBlock []styleDecl

func (b *styleBlock) set(name string, v any) {
	for i := range *b {
		if (*b)[i].name == name {
			(*b)[i].value = v
			return
		}
	}
	*b = append(*b, styleDecl{name, v})
}

type styleRule struct {
	kind  int // 0 class，1 id，2 标签
	name  string
	block styleBlock
}

func camel(s string) string {
	parts := strings.Split(s, "-")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func stripComments(s string) string {
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+2+j+2:]
	}
}

// parseStyleSheet 解析 <style> 的内容
func parseStyleSheet(css string) ([]styleRule, error) {
	css = stripComments(css)
	var rules []styleRule
	for {
		css = strings.TrimSpace(css)
		if css == "" {
			return rules, nil
		}
		open := strings.IndexByte(css, '{')
		if open < 0 {
			return nil, fmt.Errorf("样式写得不对：%q 后面缺少 {", css)
		}
		close := strings.IndexByte(css[open:], '}')
		if close < 0 {
			return nil, fmt.Errorf("样式写得不对：%q 缺少 }", strings.TrimSpace(css[:open]))
		}
		sel := strings.TrimSpace(css[:open])
		body := css[open+1 : open+close]
		css = css[open+close+1:]
		if strings.HasPrefix(sel, "@") {
			return nil, fmt.Errorf("不支持 %s", sel)
		}
		block, err := parseDecls(body)
		if err != nil {
			return nil, fmt.Errorf("%s：%w", sel, err)
		}
		for _, s := range strings.Split(sel, ",") {
			s = strings.TrimSpace(s)
			r := styleRule{block: append(styleBlock(nil), block...)}
			switch {
			case strings.HasPrefix(s, "."):
				r.kind, r.name = 0, s[1:]
			case strings.HasPrefix(s, "#"):
				r.kind, r.name = 1, s[1:]
			default:
				r.kind, r.name = 2, s
			}
			if r.name == "" || strings.ContainsAny(r.name, " >+~.#:[*") {
				return nil, fmt.Errorf("不支持选择器 %q：只能用单个 .class、#id 或标签名", s)
			}
			rules = append(rules, r)
		}
	}
}

// parseDecls 解析 a: b; c: d
func parseDecls(body string) (styleBlock, error) {
	var block styleBlock
	depth := 0
	start := 0
	var items []string
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ';':
			if depth == 0 {
				items = append(items, body[start:i])
				start = i + 1
			}
		}
	}
	items = append(items, body[start:])
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		j := strings.IndexByte(it, ':')
		if j <= 0 {
			return nil, fmt.Errorf("样式 %q 写得不对", it)
		}
		name := camel(strings.TrimSpace(it[:j]))
		value := strings.TrimSpace(it[j+1:])
		value = strings.TrimSpace(strings.TrimSuffix(value, "!important"))
		if err := addDecl(&block, name, value); err != nil {
			return nil, err
		}
	}
	return block, nil
}

// 按空白拆开值，括号里的空白不拆（rgba(0, 0, 0, 0.5) 是一项）
func splitValue(v string) []string {
	var out []string
	depth := 0
	start := -1
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
		}
		space := c == ' ' || c == '\t' || c == '\n' || c == '\r'
		if space && depth == 0 {
			if start >= 0 {
				out = append(out, v[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, v[start:])
	}
	return out
}

var sides = []string{"Top", "Right", "Bottom", "Left"}

func fourSides(vals []string) []string {
	switch len(vals) {
	case 1:
		return []string{vals[0], vals[0], vals[0], vals[0]}
	case 2:
		return []string{vals[0], vals[1], vals[0], vals[1]}
	case 3:
		return []string{vals[0], vals[1], vals[2], vals[1]}
	default:
		return vals[:4]
	}
}

var borderStyles = map[string]bool{"none": true, "hidden": true, "dotted": true, "dashed": true, "solid": true,
	"double": true, "groove": true, "ridge": true, "inset": true, "outset": true}

func addDecl(b *styleBlock, name, value string) error {
	vals := splitValue(value)
	if len(vals) == 0 {
		return fmt.Errorf("%s 没有值", name)
	}
	switch name {
	case "padding", "margin":
		for i, v := range fourSides(vals) {
			b.set(name+sides[i], normalValue(v))
		}
	case "borderWidth", "borderColor":
		kind := strings.TrimPrefix(name, "border")
		for i, v := range fourSides(vals) {
			b.set("border"+sides[i]+kind, normalValue(v))
		}
	case "border", "borderTop", "borderRight", "borderBottom", "borderLeft":
		side := strings.TrimPrefix(name, "border")
		var width, style, color string
		for _, v := range vals {
			switch {
			case borderStyles[v]:
				style = v
			case isColor(v):
				color = v
			default:
				width = v
			}
		}
		targets := sides
		if side != "" {
			targets = []string{side}
		}
		if color != "" {
			for _, s := range targets {
				b.set("border"+s+"Color", normalValue(color))
			}
		}
		if style != "" {
			b.set("border"+side+"Style", style)
		}
		if width != "" {
			for _, s := range targets {
				b.set("border"+s+"Width", normalValue(width))
			}
		}
	case "lines":
		f, err := strconv.ParseFloat(vals[0], 64)
		if err != nil {
			return fmt.Errorf("lines 要是数字：%q", value)
		}
		b.set(name, f)
	default:
		if len(vals) == 1 {
			b.set(name, normalValue(vals[0]))
		} else {
			parts := make([]string, len(vals))
			for i, v := range vals {
				parts[i] = fmt.Sprint(normalValue(v))
			}
			b.set(name, strings.Join(parts, " "))
		}
	}
	return nil
}

// 单个值：没单位的数字 → 数字；颜色 → 规范写法；其它原样
func normalValue(v string) any {
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	if isColor(v) {
		return formatColor(v)
	}
	return v
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func isColor(v string) bool {
	if strings.HasPrefix(v, "#") {
		h := v[1:]
		return isHex(h) && (len(h) == 3 || len(h) == 4 || len(h) == 6 || len(h) == 8)
	}
	l := strings.ToLower(v)
	return strings.HasPrefix(l, "rgb(") || strings.HasPrefix(l, "rgba(") || colorNames[l]
}

// 颜色写法和 aiot-toolkit（tinycolor）一样：#abc → #aabbcc，rgba 的数字取整、透明度保留两位，颜色名原样
func formatColor(v string) string {
	if strings.HasPrefix(v, "#") {
		h := strings.ToLower(v[1:])
		switch len(h) {
		case 3:
			return "#" + string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		case 4:
			return "#" + string([]byte{h[0], h[0], h[1], h[1], h[2], h[2], h[3], h[3]})
		}
		return "#" + h
	}
	l := strings.ToLower(v)
	if strings.HasPrefix(l, "rgb") {
		i := strings.IndexByte(l, '(')
		j := strings.LastIndexByte(l, ')')
		if i < 0 || j < i {
			return v
		}
		parts := strings.Split(l[i+1:j], ",")
		if len(parts) != 3 && len(parts) != 4 {
			return v
		}
		nums := make([]float64, len(parts))
		for k, p := range parts {
			f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil {
				return v
			}
			nums[k] = f
		}
		r, g, b := math.Round(nums[0]), math.Round(nums[1]), math.Round(nums[2])
		a := 1.0
		if len(nums) == 4 {
			a = math.Round(nums[3]*100) / 100
		}
		if a == 1 {
			return fmt.Sprintf("rgb(%v, %v, %v)", r, g, b)
		}
		return fmt.Sprintf("rgba(%v, %v, %v, %v)", r, g, b, a)
	}
	return v
}

var colorNames = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue
	blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan
	darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon
	darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey
	dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite gold goldenrod gray green greenyellow grey
	honeydew hotpink indianred indigo ivory khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral
	lightcyan lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen lightskyblue
	lightslategray lightslategrey lightsteelblue lightyellow lime limegreen linen magenta maroon mediumaquamarine
	mediumblue mediumorchid mediumpurple mediumseagreen mediumslateblue mediumspringgreen mediumturquoise
	mediumvioletred midnightblue mintcream mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered
	orchid palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum powderblue purple
	rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue slateblue
	slategray slategrey snow springgreen steelblue tan teal thistle tomato transparent turquoise violet wheat white
	whitesmoke yellow yellowgreen`) {
		colorNames[n] = true
	}
}

// 样式值写成 JS
func jsValue(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case string:
		return strconv.Quote(x)
	}
	return "null"
}

func (b styleBlock) js() string {
	parts := make([]string, len(b))
	for i, d := range b {
		parts[i] = d.name + ": " + jsValue(d.value)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func styleSheetJS(rules []styleRule) string {
	parts := make([]string, len(rules))
	for i, r := range rules {
		parts[i] = fmt.Sprintf("[[[%d, %s]], %s]", r.kind, strconv.Quote(r.name), r.block.js())
	}
	return "[" + strings.Join(parts, ",\n") + "]"
}
