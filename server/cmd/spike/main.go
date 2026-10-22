// spike：渲染实验的 Mac 端。装上 spike/render-app 的 rpk，发界面树给手环，检查手环报回来的布局、图片、耗时。
//
//	go run ./cmd/spike [-config path] [-rpk path] [-tap 60s] [-out report.json]
//
// 要先退出 miband pet App（手环同一时间只接受一个连接）。
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"miband/internal/bt"
	"miband/internal/service"
	"miband/internal/xiaomi"
)

const pkg = "dev.bysir.spike"

func main() {
	bt.RunMain(run)
}

type msg = map[string]any

type spike struct {
	apps   *xiaomi.Apps
	inbox  chan msg
	seq    int
	report map[string]any
	maxMsg int // 实测能收到的最大消息字节数
	only   string
}

func (s *spike) want(name string) bool {
	return s.only == "" || strings.Contains(","+s.only+",", ","+name+",")
}

func run() int {
	home, _ := os.UserHomeDir()
	configPath := flag.String("config", filepath.Join(home, "Library/Application Support/miband-pet/config.json"), "配置文件")
	rpk := flag.String("rpk", "", "实验快应用的 rpk（默认 ../spike/render-app/dist 下最新的）")
	tap := flag.Duration("tap", 0, "最后留多久给手动点按测试（0 表示跳过）")
	out := flag.String("out", "spike-report.json", "报告写到哪")
	skipInstall := flag.Bool("no-install", false, "不重装 rpk")
	only := flag.String("only", "", "只跑这些测试，逗号分隔：sizes,layout,images,perf,ping（默认全跑）")
	evalTest := flag.Bool("evaltest", false, "只测手环能不能执行动态下发的 JS（语法支持、速度）")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if *rpk == "" {
		list, _ := filepath.Glob("../spike/render-app/dist/*.rpk")
		if len(list) == 0 {
			fatal(errors.New("找不到 rpk，先在 spike/render-app 里 npx aiot build"))
		}
		*rpk = list[len(list)-1]
	}
	cfg, err := service.LoadConfig(*configPath)
	if err != nil {
		fatal(err)
	}
	dev, apps, err := service.NewDevice(*cfg)
	if err != nil {
		fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			dev.Close()
			os.Exit(130)
		case <-done:
		}
	}()
	if err := dev.Connect(); err != nil {
		slog.Warn("连接失败，3 秒后重试", "err", err)
		time.Sleep(3 * time.Second)
		if err := dev.Connect(); err != nil {
			fatal(err)
		}
	}
	defer dev.Close()

	s := &spike{apps: apps, inbox: make(chan msg, 256), report: map[string]any{}, maxMsg: 16384, only: *only}
	apps.OnMessage = func(p string, payload any) {
		if m, ok := payload.(map[string]any); ok && p == pkg {
			s.inbox <- m
		}
	}
	if *evalTest {
		err = s.evalTest(*rpk, *skipInstall)
	} else {
		err = s.all(*rpk, *skipInstall, *tap)
	}
	if err != nil {
		fmt.Println("中止:", err)
	}
	raw, _ := json.MarshalIndent(s.report, "", "  ")
	_ = os.WriteFile(*out, raw, 0o644)
	fmt.Println("\n报告已写到", *out)
	return 0
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}

func (s *spike) all(rpk string, skipInstall bool, tap time.Duration) error {
	if !skipInstall {
		fmt.Println("安装", rpk)
		if _, err := s.apps.Install(rpk, func(i, n int) { fmt.Printf("\r传输 %d/%d", i, n) }); err != nil {
			return err
		}
		fmt.Println()
	}
	if err := s.apps.Launch(pkg, ""); err != nil {
		return err
	}
	if _, err := s.wait("hello", 30*time.Second); err != nil {
		return fmt.Errorf("等快应用打开: %w", err)
	}
	fmt.Println("快应用已连上")
	time.Sleep(500 * time.Millisecond)
	s.drain()

	if s.want("sizes") {
		s.testSizes()
	}
	if s.want("layout") {
		for _, mode := range []string{"flat", "rec"} {
			s.testLayout(mode)
			s.testUpdate(mode)
		}
	}
	if s.want("images") {
		s.testImages()
	}
	if s.want("perf") {
		for _, mode := range []string{"flat", "rec"} {
			s.testPerf(mode)
		}
	}
	if s.want("ping") {
		s.testPing()
	}
	if tap > 0 {
		s.testTap(tap)
	}
	return nil
}

// evalTest：渲染器里 new Function / eval 能不能用
func (s *spike) evalTest(rpk string, skipInstall bool) error {
	if !skipInstall {
		if _, err := s.apps.Install(rpk, nil); err != nil {
			return err
		}
	}
	if err := s.apps.Launch(pkg, ""); err != nil {
		return err
	}
	if _, err := s.wait("hello", 30*time.Second); err != nil {
		return fmt.Errorf("等渲染器打开: %w", err)
	}
	cases := []struct{ name, code string }{
		{"ES5", "var o = {n: a * b}; return JSON.stringify(o) + [1,2,3].map(function (x) { return x * a }).join('-')"},
		{"箭头/let/const/模板", "const f = (x) => x * a; let s = `v${f(b)}`; return s"},
		{"解构/展开/默认参数", "const {x, ...rest} = {x: 1, y: 2, z: 3}; const g = (p = 5) => p; return [x, Object.keys(rest).join(''), g(), ...[7, 8]].join(',')"},
		{"class/Map/Set/for-of", "class A { constructor(v) { this.v = v } get d() { return this.v * 2 } }; const m = new Map([[1, 'a']]); let n = 0; for (const [k] of m) n += k; return new A(b).d + n + new Set([1,1,2]).size"},
		{"可选链/空值合并（ES2020）", "const o = {p: null}; return (o.p?.q ?? 'dflt')"},
		{"async/await", "const f = async () => 1; return typeof f().then"},
		{"Object.is/entries/includes", "return Object.is(NaN, NaN) + ',' + Object.entries({a: 1}).length + ',' + [1, 2].includes(2) + ',' + 'abc'.padStart(5, '-')"},
		{"全局计时器", "return typeof setTimeout + ',' + typeof setInterval + ',' + typeof clearTimeout"},
		{"性能：10 万次循环 + 1000 个对象", "var t = Date.now(); var s = 0; for (var i = 0; i < 100000; i++) s += i % 7; var arr = []; for (var j = 0; j < 1000; j++) arr.push({i: j, s: 'x' + j}); JSON.stringify(arr); return (Date.now() - t) + 'ms'"},
	}
	for i, c := range cases {
		s.send(msg{"t": "js", "id": i, "code": c.code, "expr": "1 + 2 * 3"})
		m, err := s.wait("js_result", 10*time.Second)
		if err != nil {
			return err
		}
		if m["fnErr"] != nil {
			fmt.Printf("✗ %-28s %v\n", c.name, m["fnErr"])
		} else {
			fmt.Printf("✓ %-28s %v\n", c.name, m["fn"])
		}
		s.report[c.name] = m
	}
	return nil
}

func (s *spike) send(v msg) {
	if err := s.apps.Send(pkg, v); err != nil {
		slog.Warn("发送失败", "t", v["t"], "err", err)
	}
}

// wait 等某种类型的消息；中途收到的 err 打出来
func (s *spike) wait(t string, timeout time.Duration) (msg, error) {
	deadline := time.After(timeout)
	for {
		select {
		case m := <-s.inbox:
			if m["t"] == t {
				return m, nil
			}
			if m["t"] == "err" {
				fmt.Println("  手环报错:", m)
			}
			if m["t"] == "info" {
				s.report["screen"] = m
			}
		case <-deadline:
			return nil, fmt.Errorf("等 %s 超时", t)
		}
	}
}

func (s *spike) drain() {
	for {
		select {
		case m := <-s.inbox:
			if m["t"] == "info" {
				s.report["screen"] = m
			}
		default:
			return
		}
	}
}

// ---------- 界面树 ----------

func view(id, style string, children ...msg) msg {
	if children == nil {
		children = []msg{}
	}
	return msg{"type": "view", "id": id, "style": style, "children": children}
}

func text(id, style, s string) msg {
	return msg{"type": "text", "id": id, "style": style, "text": s}
}

// render 发一棵树并等手环报回布局
func (s *spike) render(label, mode string, tree msg, measure []string, expect int, sprites []string) (msg, error) {
	s.seq++
	s.drain()
	s.send(msg{"t": "tree", "label": label, "mode": mode, "tree": tree, "measure": measure, "expect": expect, "seq": s.seq, "sprites": sprites})
	for {
		m, err := s.wait("rects", 15*time.Second)
		if err != nil {
			return nil, err
		}
		if int(num(m["seq"])) == s.seq {
			return m, nil
		}
	}
}

type rect struct{ l, t, w, h float64 }

func (r rect) right() float64  { return r.l + r.w }
func (r rect) bottom() float64 { return r.t + r.h }

func rects(m msg) map[string]rect {
	out := map[string]rect{}
	rs, _ := m["rects"].(map[string]any)
	for id, v := range rs {
		o, _ := v.(map[string]any)
		if _, bad := o["err"]; bad {
			continue
		}
		out[id] = rect{num(o["l"]), num(o["t"]), num(o["w"]), num(o["h"])}
	}
	return out
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}

// ---------- 测试 ----------

type checker struct {
	name   string
	scale  float64
	passed []string
	failed []string
}

func (c *checker) eq(what string, got, want float64) {
	want *= c.scale
	if math.Abs(got-want) <= math.Max(1.5, c.scale) {
		c.passed = append(c.passed, what)
	} else {
		c.failed = append(c.failed, fmt.Sprintf("%s：实际 %.1f，期望 %.1f", what, got, want))
	}
}

func (c *checker) ok(what string, cond bool, detail string) {
	if cond {
		c.passed = append(c.passed, what)
	} else {
		c.failed = append(c.failed, what+"："+detail)
	}
}

func (c *checker) print() map[string]any {
	status := "通过"
	if len(c.failed) > 0 {
		status = "失败"
	}
	fmt.Printf("[%s] %s（%d 项通过）\n", status, c.name, len(c.passed))
	for _, f := range c.failed {
		fmt.Println("   ✗", f)
	}
	return map[string]any{"status": status, "passed": c.passed, "failed": c.failed}
}

var layoutIDs = []string{"r", "a1", "b1", "c1", "c2", "a2", "b3", "b4"}

func layoutTree(b1Style string) msg {
	return view("r", "flex-direction:column;padding:10px;background-color:#222222;width:212px",
		view("a1", "flex-direction:column;padding:6px;background-color:#334455",
			view("b1", b1Style,
				view("c1", "width:30px;height:20px;background-color:#ff5555"),
				view("c2", "width:30px;height:20px;background-color:#55ff55"),
			),
			text("t1", "font-size:16px;color:#ffffff", "左右布局"),
		),
		view("a2", "flex-direction:row;margin-top:8px",
			view("b3", "width:40px;height:20px;background-color:#5555ff"),
			view("b4", "width:40px;height:20px;margin-left:10px;background-color:#ffff55"),
		),
	)
}

// testLayout：整串 style 动态绑定 + 4 层嵌套，布局结果和 CSS 预期一致
func (s *spike) testLayout(mode string) {
	c := &checker{name: "布局 / " + mode}
	defer func() { s.report["layout_"+mode] = c.print() }()
	m, err := s.render("布局 "+mode, mode, layoutTree("flex-direction:row;justify-content:space-between;padding:4px;width:180px;background-color:#445566"), layoutIDs, len(layoutIDs), nil)
	if err != nil {
		c.ok("收到布局", false, err.Error())
		return
	}
	r := rects(m)
	s.report["raw_layout_"+mode] = m
	c.ok("每个节点都量到了", len(r) == len(layoutIDs), fmt.Sprintf("只量到 %d/%d，首次量到 %vms，组件挂载 %v，错误 %v", len(r), len(layoutIDs), m["firstOk"], m["mounted"], m["errs"]))
	if len(r) < len(layoutIDs) {
		return
	}
	c.scale = r["r"].w / 212
	c.ok("根节点宽度能换算", c.scale > 0, "根节点宽 0")
	if c.scale == 0 {
		return
	}
	c.eq("padding: a1 左边距 10", r["a1"].l-r["r"].l, 10)
	c.eq("padding: a1 上边距 10", r["a1"].t-r["r"].t, 10)
	c.eq("嵌套: b1 在 a1 里缩进 6", r["b1"].l-r["a1"].l, 6)
	c.eq("宽度: b1 = 180", r["b1"].w, 180)
	c.eq("尺寸: c1 宽 30", r["c1"].w, 30)
	c.eq("尺寸: c1 高 20", r["c1"].h, 20)
	c.eq("第 4 层: c1 在 b1 里缩进 4", r["c1"].l-r["b1"].l, 4)
	c.eq("space-between: c2 贴右边（留 4）", r["b1"].right()-r["c2"].right(), 4)
	c.eq("row: c1 c2 同一行", r["c2"].t-r["c1"].t, 0)
	c.eq("row: b3 b4 同一行", r["b4"].t-r["b3"].t, 0)
	c.eq("margin-left: b4 离 b3 10", r["b4"].l-r["b3"].right(), 10)
	c.eq("margin-top: a2 离 a1 8", r["a2"].t-r["a1"].bottom(), 8)
	s.report["rects_"+mode] = m["rects"]
	s.report["render_ms_layout_"+mode] = m["ms"]
}

// testUpdate：同一棵树换 style，手环要跟着重排
func (s *spike) testUpdate(mode string) {
	c := &checker{name: "更新 style / " + mode}
	defer func() { s.report["update_"+mode] = c.print() }()
	m, err := s.render("更新 "+mode, mode, layoutTree("flex-direction:column;padding:0px;width:180px;background-color:#665544"), layoutIDs, len(layoutIDs), nil)
	if err != nil {
		c.ok("收到布局", false, err.Error())
		return
	}
	r := rects(m)
	s.report["raw_update_"+mode] = m
	c.scale = r["r"].w / 212
	if c.scale == 0 {
		c.ok("量到根节点", false, fmt.Sprintf("组件挂载 %v，错误 %v", m["mounted"], m["errs"]))
		return
	}
	c.eq("改成 column 后 c2 在 c1 下面", r["c2"].t-r["c1"].bottom(), 0)
	c.eq("改成 column 后 c1 c2 左对齐", r["c2"].l-r["c1"].l, 0)
	c.eq("padding 改成 0", r["c1"].l-r["b1"].l, 0)
}

// testSizes：一条消息最多能多大
func (s *spike) testSizes() {
	results := []map[string]any{}
	// 64KB 会让手环不再确认数据帧，连接卡死，所以只测到 32KB
	for _, n := range []int{512, 2048, 8192, 16384, 32768} {
		s.drain()
		t0 := time.Now()
		s.send(msg{"t": "big", "n": n, "pad": strings.Repeat("x", n)})
		m, err := s.wait("big_ok", 20*time.Second)
		r := map[string]any{"bytes": n}
		if err != nil || int(num(m["len"])) != n {
			r["ok"] = false
			fmt.Printf("[消息大小] %6d 字节：失败 %v\n", n, err)
			results = append(results, r)
			break
		}
		ms := time.Since(t0).Milliseconds()
		r["ok"], r["ms"] = true, ms
		s.maxMsg = n
		fmt.Printf("[消息大小] %6d 字节：%d ms（约 %.1f KB/s）\n", n, ms, float64(n)/1024/(float64(ms)/1000))
		results = append(results, r)
	}
	s.report["sizes"] = results
}

func testPNG(w, h int, fg color.RGBA, flip bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			on := (x/8+y/8)%2 == 0
			if flip {
				on = !on
			}
			if on {
				img.Set(x, y, fg)
			} else {
				img.Set(x, y, color.RGBA{30, 30, 30, 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// pushImage 把图片分片推到手环，返回手环上的 uri
func (s *spike) pushImage(name string, data []byte) (string, msg, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	chunk := min(max(s.maxMsg/2, 1024), 8192)
	n := (len(b64) + chunk - 1) / chunk
	s.drain()
	t0 := time.Now()
	for i := range n {
		s.send(msg{"t": "imgc", "name": name, "i": i, "n": n, "b64": b64[i*chunk : min((i+1)*chunk, len(b64))]})
	}
	m, err := s.wait("img_saved", 30*time.Second)
	if err != nil {
		return "", nil, err
	}
	if m["err"] != nil {
		return "", m, fmt.Errorf("写文件失败 %v", m["err"])
	}
	m["total_ms"] = time.Since(t0).Milliseconds()
	m["chunks"] = n
	return m["uri"].(string), m, nil
}

// testImages：运行时推的图能不能显示（image 和 image-animator）
func (s *spike) testImages() {
	c := &checker{name: "运行时推送图片", scale: 1}
	defer func() { s.report["images"] = c.print() }()
	frames := []string{}
	pushed := []msg{}
	for i, col := range []color.RGBA{{255, 120, 40, 255}, {40, 200, 255, 255}} {
		data := testPNG(48, 48, col, i == 1)
		uri, m, err := s.pushImage(fmt.Sprintf("spike_%d.png", i), data)
		if err != nil {
			c.ok(fmt.Sprintf("推送第 %d 张图", i), false, err.Error())
			return
		}
		fmt.Printf("  推图 %d：%d 字节，%v 片，%v ms\n", i, len(data), m["chunks"], m["total_ms"])
		pushed = append(pushed, m)
		frames = append(frames, uri)
	}
	s.report["image_push"] = pushed
	c.passed = append(c.passed, "图片写进手环存储")

	for _, mode := range []string{"flat", "rec"} {
		tree := view("r", "flex-direction:column;padding:10px;width:212px",
			view("ir", "flex-direction:row;align-items:center",
				msg{"type": "image", "id": "img1", "src": frames[0], "style": "width:48px;height:48px"},
				msg{"type": "text", "id": "it", "style": "font-size:14px;color:#ffffff;margin-left:8px", "text": "运行时推的图 " + mode},
			),
		)
		measure := []string{"r", "ir"}
		var sprites []string
		if mode == "flat" {
			// image-animator 只在 flat 模式里有
			tree["children"] = append(tree["children"].([]msg), view("sr", "flex-direction:row;margin-top:8px",
				msg{"type": "sprite", "id": "sp1", "frames": []msg{{"src": frames[0]}, {"src": frames[1]}}, "duration": "600ms", "style": "width:48px;height:48px"},
				text("st", "font-size:14px;color:#ffffff;margin-left:8px", "两帧动画"),
			))
			measure = append(measure, "sp1")
			sprites = []string{"sp1"}
		}
		s.drain()
		s.seq++
		s.send(msg{"t": "tree", "label": "图片 " + mode, "mode": mode, "tree": tree, "measure": measure, "expect": len(measure), "seq": s.seq, "sprites": sprites})
		var imgEv, rectsMsg msg
		deadline := time.After(10 * time.Second)
	loop:
		for imgEv == nil || rectsMsg == nil {
			select {
			case m := <-s.inbox:
				switch m["t"] {
				case "img":
					imgEv = m
				case "rects":
					rectsMsg = m
				}
			case <-deadline:
				break loop
			}
		}
		c.ok("image complete 事件 / "+mode, imgEv != nil && imgEv["ok"] == true, fmt.Sprint("事件：", imgEv))
		if rectsMsg != nil && mode == "flat" {
			r := rects(rectsMsg)
			c.ok("image-animator 有尺寸", r["sp1"].w > 0, fmt.Sprint(rectsMsg["rects"]))
		}
		s.report["image_event_"+mode] = imgEv
		fmt.Printf("  %s：图片显示好了吗？动画在两种颜色间切换吗？看一眼手环（停 4 秒）\n", mode)
		time.Sleep(4 * time.Second)
	}
}

// testPerf：大一点的树渲染要多久
func (s *spike) testPerf(mode string) {
	results := []map[string]any{}
	for _, rows := range []int{10, 30, 60} {
		children := []msg{}
		for i := range rows {
			children = append(children, view(fmt.Sprintf("row%d", i), "flex-direction:row;justify-content:space-between;padding:2px 6px;margin-top:2px;background-color:#1a1a2a",
				text(fmt.Sprintf("tx%d", i), "font-size:14px;color:#dddddd", fmt.Sprintf("agent %d", i)),
				view(fmt.Sprintf("bar%d", i), fmt.Sprintf("width:%dpx;height:8px;background-color:#44aa66", 10+i%60)),
			))
		}
		tree := view("r", "flex-direction:column;width:212px", children...)
		last := fmt.Sprintf("bar%d", rows-1)
		expect := 1
		if mode == "rec" {
			expect = 1 + rows*2 // 递归模式下每个 view 都会报
		}
		t0 := time.Now()
		m, err := s.render(fmt.Sprintf("%d 行 %s", rows, mode), mode, tree, []string{last}, expect, nil)
		r := map[string]any{"rows": rows, "nodes": 1 + rows*3}
		if err != nil {
			r["err"] = err.Error()
			fmt.Printf("[性能 %s] %d 行：%v\n", mode, rows, err)
		} else {
			r["band_ms"], r["first_ok_ms"], r["mac_ms"], r["measured"], r["mounted"], r["errs"] = m["ms"], m["firstOk"], time.Since(t0).Milliseconds(), len(rects(m)), m["mounted"], m["errs"]
			fmt.Printf("[性能 %s] %d 行（%d 节点）：首次量到 %v ms，量完 %v ms，Mac 往返 %d ms，量到 %d/%d，组件挂载 %v，错误 %v\n", mode, rows, 1+rows*3, m["firstOk"], m["ms"], time.Since(t0).Milliseconds(), len(rects(m)), expect, m["mounted"], m["errs"])
		}
		results = append(results, r)
	}
	s.report["perf_"+mode] = results
}

func pingTree(i int) msg {
	return view("r", "flex-direction:column;padding:10px;width:212px",
		text("pt", "font-size:20px;color:#ffffff", fmt.Sprintf("第 %d 轮", i)),
		view("pb", fmt.Sprintf("width:%dpx;height:10px;margin-top:6px;background-color:#ff8844", 10+i*8)),
	)
}

// testPing：手环发 ping → Mac 回新树 → 手环画完，量往返时间
func (s *spike) testPing() {
	const n = 15
	s.drain()
	s.send(msg{"t": "ping_start", "n": n})
	rtts := []float64{}
	for i := range n {
		m, err := s.wait("ping", 10*time.Second)
		if err != nil {
			fmt.Println("[往返] 中断:", err)
			break
		}
		s.send(msg{"t": "pong", "i": m["i"], "ts": m["ts"], "tree": pingTree(i), "probe": "pb"})
		ok, err := s.wait("pong_ok", 10*time.Second)
		if err != nil {
			fmt.Println("[往返] 中断:", err)
			break
		}
		rtts = append(rtts, num(ok["rtt"]))
		if ok["how"] != "rect" {
			fmt.Println("  第", i, "轮没量到探针节点：", ok["how"])
		}
		if i+1 < n {
			s.send(msg{"t": "ping_next", "i": i + 1})
		}
	}
	if len(rtts) == 0 {
		return
	}
	sort.Float64s(rtts)
	p50, p90 := rtts[len(rtts)/2], rtts[len(rtts)*9/10]
	fmt.Printf("[往返] %d 轮：中位 %.0f ms，P90 %.0f ms，最慢 %.0f ms\n", len(rtts), p50, p90, rtts[len(rtts)-1])
	s.report["ping"] = map[string]any{"rounds": len(rtts), "p50": p50, "p90": p90, "max": rtts[len(rtts)-1], "all": rtts}
}

// testTap：手动点按，量「手指点下 → 画出新界面」
func (s *spike) testTap(d time.Duration) {
	count := 0
	tree := func() msg {
		return view("r", "flex-direction:column;padding:10px;width:212px",
			text("tt", "font-size:18px;color:#ffffff", fmt.Sprintf("点下面的方块（%d）", count)),
			view("btn", fmt.Sprintf("width:150px;height:60px;margin-top:10px;background-color:%s", []string{"#ff6644", "#4488ff"}[count%2])),
		)
	}
	s.render("点按测试", "flat", tree(), []string{"btn"}, 1, nil)
	fmt.Printf("[点按] 请在手环上点方块，%v 内有效\n", d)
	lat := []float64{}
	deadline := time.After(d)
	for {
		select {
		case m := <-s.inbox:
			if m["t"] == "ev" && m["id"] == "btn" {
				count++
				s.send(msg{"t": "pong", "i": count, "ts": m["ts"], "tree": tree(), "probe": "btn"})
			}
			if m["t"] == "pong_ok" {
				lat = append(lat, num(m["rtt"]))
				fmt.Printf("  点按 → 画完：%.0f ms\n", num(m["rtt"]))
			}
		case <-deadline:
			s.report["tap"] = lat
			return
		}
	}
}
