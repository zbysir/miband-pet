// Package tray 是 macOS 菜单栏图标：关掉窗口后 App 在菜单栏常驻。
package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"

import (
	_ "embed"
	"unsafe"
)

//go:embed icon@2x.png
var icon []byte

var onShow, onQuit func()

// Init 创建菜单栏图标；onShow 在点“打开主窗口”时调用，onQuit 在点“退出”时调用（都在主线程上）。
func Init(show, quit func()) {
	onShow, onQuit = show, quit
	C.tray_init(unsafe.Pointer(&icon[0]), C.int(len(icon)))
}

// SetInfo 设置菜单第一行的状态文字。
func SetInfo(text string) {
	s := C.CString(text)
	defer C.free(unsafe.Pointer(s))
	C.tray_set_info(s)
}

func SetDockVisible(visible bool) {
	v := 0
	if visible {
		v = 1
	}
	C.tray_set_dock_visible(C.int(v))
}

//export goTrayShow
func goTrayShow() {
	if onShow != nil {
		go onShow() // 不在主线程上做 Wails 调用，免得和它内部的主线程派发互相等
	}
}

//export goTrayQuit
func goTrayQuit() {
	if onQuit != nil {
		go onQuit()
	}
}
