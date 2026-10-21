package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"sync"
	"time"

	"miband/internal/tray"
)

// 菜单栏的猫头：不动，只按状态换表情和右下角的小圆点，颜色跟着选的猫走。
// 心情规则和窗口里、手环上的猫一样：有卡住的 → 举牌；刚有 agent 从 working 变成 done/idle → 庆祝 8 秒；
// 有在跑的 → 干活；否则睡觉。

const celebrateFor = 8 * time.Second

// 右下角的状态圆点（和 App 里的状态色一致）；睡觉不加
var moodBadge = map[string]color.NRGBA{
	"alert":     {255, 159, 28, 255},
	"work":      {46, 160, 255, 255},
	"celebrate": {61, 220, 132, 255},
}

type trayAnim struct {
	mu             sync.Mutex
	skin           string
	blocked        bool
	working        bool
	prev           map[string]string
	celebrateUntil time.Time
	cache          map[string][]byte
}

func newTrayAnim() *trayAnim {
	return &trayAnim{skin: "orange", prev: map[string]string{}, cache: map[string][]byte{}}
}

func (t *trayAnim) update(st AppState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := st.Settings["pet"].(string); ok && p != "" {
		t.skin = p
	}
	t.blocked, t.working = false, false
	for _, a := range st.Agents {
		if t.prev[a.ID] == "working" && (a.Status == "done" || a.Status == "idle") {
			t.celebrateUntil = time.Now().Add(celebrateFor)
		}
		t.prev[a.ID] = a.Status
		t.blocked = t.blocked || a.Status == "blocked"
		t.working = t.working || a.Status == "working"
	}
}

func (t *trayAnim) mood() string {
	switch {
	case t.blocked:
		return "alert"
	case time.Now().Before(t.celebrateUntil):
		return "celebrate"
	case t.working:
		return "work"
	}
	return "sleep"
}

// run 每秒看一次该显示哪张（庆祝 8 秒后要自己换回去），变了才换图。
func (t *trayAnim) run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	last := ""
	for {
		t.mu.Lock()
		skin, mood := t.skin, t.mood()
		t.mu.Unlock()
		if key := skin + "/" + mood; key != last {
			if img := t.icon(skin, mood); img != nil {
				tray.SetImage(img)
				last = key
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// icon 取这只猫在这个心情下第一帧的猫头（和最初的菜单栏图标同样的裁剪和大小），
// 再在右下角画状态圆点。输出 36px，即菜单栏 18pt 的 @2x。
func (t *trayAnim) icon(skin, mood string) []byte {
	key := skin + "/" + mood
	if b, ok := t.cache[key]; ok {
		return b
	}
	raw, err := fs.ReadFile(assets, "frontend/dist/pet/"+skin+"/"+mood+"_0.png")
	if err != nil {
		return nil
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	// 160px 的帧里，猫头在 (32,36) 起 96×96 的区域，最近邻缩到 36×36。
	// 右上角（x≥116、y<60，即画布第 29 格往右、第 15 格往上）没有猫，是牌子和 Z，不裁进来
	const size, srcX, srcY, srcSize = 36, 32, 36, 96
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx, sy := srcX+x*srcSize/size, srcY+y*srcSize/size
			if sx < 116 || sy >= 60 {
				dst.Set(x, y, src.At(sx, sy))
			}
		}
	}
	if c, ok := moodBadge[mood]; ok {
		drawDot(dst, 30, 30, 6, c)
	}
	var out bytes.Buffer
	if png.Encode(&out, dst) != nil {
		return nil
	}
	t.cache[key] = out.Bytes()
	return t.cache[key]
}

// drawDot 画一个带深色描边的实心圆点，浅色、深色菜单栏上都看得清。
func drawDot(img *image.NRGBA, cx, cy, r int, c color.NRGBA) {
	ring := color.NRGBA{30, 32, 38, 255}
	for y := cy - r - 1; y <= cy+r+1; y++ {
		for x := cx - r - 1; x <= cx+r+1; x++ {
			d := (x-cx)*(x-cx) + (y-cy)*(y-cy)
			switch {
			case d <= r*r:
				img.Set(x, y, c)
			case d <= (r+1)*(r+1):
				img.Set(x, y, ring)
			}
		}
	}
}
