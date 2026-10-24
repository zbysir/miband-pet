package ux

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

// 在 goja 里加载编好的页面（aiot-toolkit 的或我们的），用给定的数据画一遍，得到能比较的元素树：
// 每个元素的属性都求了值，if 按条件展开、for 按列表展开，事件都触发一遍、记下调了哪个方法和参数。
const harness = `
var global = this;
var $app_require$ = function (name) {
  return { instance: function () { return { send: function () {} }; }, getInfo: function () {}, vibrate: function () {} };
};
var aiot = {
  __ce__: function (tag, o, ch) { return { tag: tag, opts: o.__opts__ || {}, children: ch || [] }; },
  __ci__: function (o, fn) { return { ci: true, shown: o.__opts__.shown, fn: fn }; },
  __cf__: function (o, fn) { return { cf: true, exp: o.__opts__.exp, fn: fn }; }
};
TRANSLATE
function load() {
  var exp = {};
  __mod(global, global, global, exp, undefined);
  exp.entry(exp);
  return exp.default;
}
function render(page, data) {
  var calls = [];
  var vm = {};
  Object.keys(page).forEach(function (k) {
    if (typeof page[k] === 'function' && k !== 'template') {
      vm[k] = function () {
        var a = [].slice.call(arguments);
        calls.push([k].concat(a));
        return k + '(' + JSON.stringify(a) + ')';
      };
    }
  });
  Object.assign(vm, JSON.parse(JSON.stringify(page.data || {})), data);
  var ev = function (v) { return typeof v === 'function' ? v() : v; };
  function expand(n) {
    if (n.ci) return n.shown() ? flat(n.fn()) : [];
    if (n.cf) {
      var e = n.exp();
      var list = e && e.__list__ !== undefined ? e.__list__ : e;
      var tid = e && e.__tid__;
      var out = [];
      (list || []).forEach(function (item, i) {
        flat(n.fn(i, item)).forEach(function (x) { if (tid) x.tid = tid; out.push(x); });
      });
      return out;
    }
    var o = { tag: n.tag };
    Object.keys(n.opts).forEach(function (k) {
      var v = n.opts[k];
      if (k === 'events') {
        o.events = {};
        Object.keys(v).forEach(function (e) {
          calls = [];
          v[e]({ type: e, direction: 'up', value: 'v' });
          o.events[e] = calls;
        });
      } else if (k === 'dataset') {
        o.dataset = {};
        Object.keys(v).forEach(function (d) { o.dataset[d] = ev(v[d]); });
      } else o[k] = ev(v);
    });
    o.children = flat(n.children);
    return [o];
  }
  function flat(list) {
    var out = [];
    list.forEach(function (x) { out = out.concat(expand(x)); });
    return out;
  }
  return JSON.stringify(flat([page.template(vm)]));
}
`

type loaded struct {
	vm   *goja.Runtime
	page goja.Value
}

func loadPage(t *testing.T, js string) loaded {
	t.Helper()
	if !strings.HasPrefix(js, "export default function") {
		t.Fatalf("页面不是以 export default function 开头")
	}
	js = "var __mod = function" + strings.TrimPrefix(js, "export default function")
	vm := goja.New()
	if _, err := vm.RunString(strings.Replace(harness, "TRANSLATE", translateStyle, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(js); err != nil {
		t.Fatalf("加载页面：%v", err)
	}
	page, err := vm.RunString("load()")
	if err != nil {
		t.Fatalf("运行页面：%v", err)
	}
	return loaded{vm, page}
}

func (l loaded) render(t *testing.T, data map[string]any) any {
	t.Helper()
	fn, _ := goja.AssertFunction(l.vm.Get("render"))
	v, err := fn(nil, l.page, l.vm.ToValue(data))
	if err != nil {
		t.Fatalf("画页面：%v", err)
	}
	var out any
	if err := json.Unmarshal([]byte(v.String()), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (l loaded) get(t *testing.T, expr string) any {
	t.Helper()
	l.vm.Set("__p", l.page)
	v, err := l.vm.RunString("JSON.stringify(" + expr + ")")
	if err != nil {
		t.Fatal(err)
	}
	var out any
	json.Unmarshal([]byte(v.String()), &out)
	return out
}

// 两棵树不一样时，指出第一个不一样的地方
func diff(path string, a, b any) string {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			return path + "：类型不同"
		}
		for k := range x {
			if _, ok := y[k]; !ok {
				return path + "." + k + "：只在 aiot 里有，值 " + show(x[k])
			}
		}
		for k := range y {
			if _, ok := x[k]; !ok {
				return path + "." + k + "：只在我们这有，值 " + show(y[k])
			}
		}
		for k := range x {
			if d := diff(path+"."+k, x[k], y[k]); d != "" {
				return d
			}
		}
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return path + "：长度不同\n aiot " + show(a) + "\n 我们 " + show(b)
		}
		for i := range x {
			if d := diff(path+"["+itoa(i)+"]", x[i], y[i]); d != "" {
				return d
			}
		}
	default:
		if !reflect.DeepEqual(a, b) {
			return path + "：aiot " + show(a) + "，我们 " + show(b)
		}
	}
	return ""
}

func show(v any) string {
	b, _ := json.Marshal(v)
	if len(b) > 300 {
		return string(b[:300]) + "…"
	}
	return string(b)
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func compare(t *testing.T, name string, scenarios []map[string]any) {
	t.Helper()
	want, err := os.ReadFile("testdata/" + name + ".aiot.js")
	if err != nil {
		t.Fatal(err)
	}
	fsys := os.DirFS("testdata/" + name + ".assets")
	src, _ := os.ReadFile("testdata/" + name + ".ux")
	got, err := compilePageSource(fsys, "pages/index/index.ux", string(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	a := loadPage(t, string(want))
	b := loadPage(t, got.JS)
	for _, expr := range []string{"__p.style", "__p.data", "Object.keys(__p).filter(function (k) { return typeof __p[k] === 'function' && k !== 'template' }).sort()"} {
		if d := diff(expr, a.get(t, expr), b.get(t, expr)); d != "" {
			t.Errorf("%s：%s", name, d)
		}
	}
	for i, s := range scenarios {
		if d := diff("树", a.render(t, s), b.render(t, s)); d != "" {
			t.Errorf("%s 第 %d 组数据：%s", name, i, d)
		}
	}
}

func TestFeatures(t *testing.T) {
	base := map[string]any{
		"rootClass": "big", "h": 100, "title": "T", "a": 3, "b": 4, "n": 5, "mode": "b", "show": true, "color": "#123456",
		"w": 10, "sel": 1, "name": "pic", "active": true, "obj": map[string]any{"k": "v"}, "key": "k", "page": 1,
		"label": "L", "pct": 50, "visible": false,
		"list": []any{map[string]any{"id": "x", "name": "X", "subs": []any{1, 2}}, map[string]any{"id": "y", "name": "Y", "subs": []any{}}},
	}
	with := func(kv ...any) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	}
	compare(t, "features", []map[string]any{
		base,
		with("mode", "a", "show", false, "active", false),
		with("mode", "c", "list", []any{}),
		with("mode", "z", "rootClass", "", "visible", true),
	})
}

func TestNativeCat(t *testing.T) {
	bubble := map[string]any{"show": true, "project": "blog", "title": "修登录页", "count": "1/3", "color": "#ff9f1c"}
	bars := []any{
		map[string]any{"date": "d1", "weekday": "一", "label": "3", "labelColor": "#888", "h": 10, "color": "#111", "dayColor": "#222"},
		map[string]any{"date": "d2", "weekday": "二", "label": "", "labelColor": "#888", "h": 2, "color": "#111", "dayColor": "#fff"},
	}
	compare(t, "native_cat", []map[string]any{
		{},
		{"bubble": bubble, "bars": bars, "metric": "usd", "costText": "$3", "counts": map[string]any{"blocked": 2, "working": 1, "done": 0, "idle": 4},
			"quota": map[string]any{"show": true, "name": "Claude 5h", "w": 30, "color": "#f00", "text": "40%"},
			"quotas": []any{map[string]any{"label": "Claude 5h", "w": 20, "color": "#0f0", "text": "10%"}}},
	})
}
