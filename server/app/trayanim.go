package main

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io/fs"
	"sync"

	"miband/internal/tray"
)

// 菜单栏的猫头：App 自己的一套猫（frontend/public/pet，和插件的分开），不跟着 agent 状态动（agent 状态是 Agent 猫插件在手环上显示的），
// 只按 App 里选的菜单栏猫换颜色。手环上的猫是插件自己的配置，两只可以不一样。

type trayAnim struct {
	mu    sync.Mutex
	skin  string
	cache map[string][]byte
	wake  chan struct{}
}

func newTrayAnim() *trayAnim {
	return &trayAnim{skin: "orange", cache: map[string][]byte{}, wake: make(chan struct{}, 1)}
}

func (t *trayAnim) update(st AppState) {
	t.mu.Lock()
	changed := false
	if p, ok := st.Settings["pet"].(string); ok && p != "" && p != t.skin {
		t.skin, changed = p, true
	}
	t.mu.Unlock()
	if changed {
		select {
		case t.wake <- struct{}{}:
		default:
		}
	}
}

// run 换了猫就换图标。
func (t *trayAnim) run(ctx context.Context) {
	last := ""
	for {
		t.mu.Lock()
		skin := t.skin
		t.mu.Unlock()
		if skin != last {
			if img := t.icon(skin, "sleep"); img != nil {
				tray.SetImage(img)
				last = skin
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.wake:
		}
	}
}

// icon 取这只猫在这个心情下第一帧的猫头（和最初的菜单栏图标同样的裁剪和大小），
// 输出 36px，即菜单栏 18pt 的 @2x。
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
	var out bytes.Buffer
	if png.Encode(&out, dst) != nil {
		return nil
	}
	t.cache[key] = out.Bytes()
	return t.cache[key]
}
