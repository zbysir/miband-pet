package ux

import "testing"

func TestRewriteExpr(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a + b", "_vm_.a + _vm_.b"},
		{"a.b.c", "_vm_.a.b.c"},
		{"a[b]", "_vm_.a[_vm_.b]"},
		{"a?.b ?? c", "_vm_.a?.b ?? _vm_.c"},
		{"x ? 'y' : z", "_vm_.x ? 'y' : _vm_.z"},
		{"{k: v, w}", "{k: _vm_.v, w: _vm_.w}"},
		{"{k: a ? b : c}", "{k: _vm_.a ? _vm_.b : _vm_.c}"},
		{"list.map(x => x.n + m)", "_vm_.list.map(x => x.n + _vm_.m)"},
		{"(a, b) => a + c", "(a, b) => a + _vm_.c"},
		{"Math.max(a, 1)", "Math.max(_vm_.a, 1)"},
		{"`n=${n}个`", "`n=${_vm_.n}个`"},
		{"item.name + i", "item.name + i"},
		{"typeof a === 'string'", "typeof _vm_.a === 'string'"},
		{"a?.5:1", "_vm_.a?.5:1"},
		{"!show", "!_vm_.show"},
	}
	for _, c := range cases {
		got, err := rewriteExpr(c.in, map[string]bool{"item": true, "i": true})
		if err != nil {
			t.Errorf("%q：%v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q → %q，应该是 %q", c.in, got, c.want)
		}
	}
}

func TestExprSyntaxError(t *testing.T) {
	for _, bad := range []string{"a +", "a b", "(a", "'x"} {
		if _, err := rewriteExpr(bad, nil); err == nil {
			t.Errorf("%q 应该报错", bad)
		}
	}
}
