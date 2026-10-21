package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"sync"
	"time"

	"miband/internal/tray"
)

// 菜单栏的猫：和窗口里、手环上的猫同一套帧、同一套心情规则，颜色跟着选的猫走。

var moodFrames = map[string]struct {
	count int
	cycle time.Duration
}{
	"work":      {6, 900 * time.Millisecond},
	"alert":     {6, 700 * time.Millisecond},
	"celebrate": {6, 800 * time.Millisecond},
	"sleep":     {8, 2400 * time.Millisecond},
}

const celebrateFor = 8 * time.Second

type trayAnim struct {
	mu             sync.Mutex
	skin, mood     string
	since          time.Time // 当前心情从什么时候开始，帧从这里算起
	prev           map[string]string
	celebrateUntil time.Time
	cache          map[string][]byte
}

func newTrayAnim() *trayAnim {
	return &trayAnim{skin: "orange", mood: "sleep", since: time.Now(), prev: map[string]string{}, cache: map[string][]byte{}}
}

// update 根据最新状态算心情：有卡住的 → 举牌；刚有 agent 从 working 变成 done/idle → 庆祝 8 秒；有在跑的 → 干活；否则睡觉。
func (t *trayAnim) update(st AppState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := st.Settings["pet"].(string); ok && p != "" {
		t.skin = p
	}
	var blocked, working bool
	for _, a := range st.Agents {
		before := t.prev[a.ID]
		if before == "working" && (a.Status == "done" || a.Status == "idle") {
			t.celebrateUntil = time.Now().Add(celebrateFor)
		}
		t.prev[a.ID] = a.Status
		blocked = blocked || a.Status == "blocked"
		working = working || a.Status == "working"
	}
	t.setMood(blocked, working)
}

func (t *trayAnim) setMood(blocked, working bool) {
	mood := "sleep"
	switch {
	case blocked:
		mood = "alert"
	case time.Now().Before(t.celebrateUntil):
		mood = "celebrate"
	case working:
		mood = "work"
	}
	if mood != t.mood {
		t.mood, t.since = mood, time.Now()
	}
}

// run 按心情的节奏换帧；帧有变化才换图。
func (t *trayAnim) run(ctx context.Context) {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		t.mu.Lock()
		if t.mood == "celebrate" && time.Now().After(t.celebrateUntil) {
			// 庆祝时间到了：回到干活或睡觉（下一次状态更新会再精确算一遍）
			t.mood, t.since = "sleep", time.Now()
			for _, s := range t.prev {
				if s == "working" {
					t.mood = "work"
				}
			}
		}
		skin, mood := t.skin, t.mood
		mf := moodFrames[mood]
		i := int(time.Since(t.since)/(mf.cycle/time.Duration(mf.count))) % mf.count
		t.mu.Unlock()

		key := fmt.Sprintf("%s/%s_%d", skin, mood, i)
		if key == last {
			continue
		}
		if img := t.frame(mood, key); img != nil {
			tray.SetImage(img)
			last = key
		}
	}
}

// 裁剪框的上边：40 格高只能裁 36 格，举牌、彩纸在最上面，笔记本在最下面，按心情取舍
var cropTop = map[string]int{"alert": 0, "celebrate": 0, "work": 4, "sleep": 2}

// frame 取一帧菜单栏图标：App 内置的帧是 160px（40 格 × 4 倍），缩回 40 格原图，
// 再裁出 36×36（含举牌、Z、笔记本这些道具），正好是菜单栏 18pt 的 @2x，一格对一个像素。
func (t *trayAnim) frame(mood, key string) []byte {
	if b, ok := t.cache[key]; ok {
		return b
	}
	raw, err := fs.ReadFile(assets, "frontend/dist/pet/"+key+".png")
	if err != nil {
		return nil
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	const scale, x0, size = 4, 3, 36
	y0 := cropTop[mood]
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dst.Set(x, y, src.At((x0+x)*scale+1, (y0+y)*scale+1))
		}
	}
	var out bytes.Buffer
	if png.Encode(&out, dst) != nil {
		return nil
	}
	t.cache[key] = out.Bytes()
	return t.cache[key]
}
