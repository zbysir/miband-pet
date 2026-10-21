// Package bt 封装 macOS 经典蓝牙 RFCOMM（SPP）连接。实现在 rfcomm_darwin.m 里，走 IOBluetooth。
package bt

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework IOBluetooth
#include <stdlib.h>
#include "rfcomm_darwin.h"
*/
import "C"

import (
	"fmt"
	"os"
	"runtime"
	"runtime/cgo"
	"strings"
	"sync"
	"time"
	"unsafe"
)

func init() {
	// 让 main goroutine 留在主线程上：IOBluetooth 的回调只投递到主线程的 run loop
	runtime.LockOSThread()
}

// RunMain 必须在 main() 里调用：fn 在另一个 goroutine 里跑，主线程留给 macOS run loop。fn 返回后进程退出。
func RunMain(fn func() int) {
	go func() { os.Exit(fn()) }()
	C.bt_run_main_loop()
}

// Conn 是一条打开的 RFCOMM 连接。收到的数据和断开事件通过回调送出，回调在蓝牙线程上执行，不要在里面阻塞。
type Conn struct {
	OnData   func([]byte)
	OnClosed func()

	handle cgo.Handle
	once   sync.Once
	opened chan [2]int // 异步打开的结果：status, mtu
}

// 一个进程同时只开一条连接（Objective-C 那边也是这么假设的）
var mu sync.Mutex

// Open 连接手环的 SPP 服务。address 形如 AA:BB:CC:DD:EE:FF。
func Open(address string, onData func([]byte), onClosed func()) (*Conn, int, error) {
	mu.Lock()
	c := &Conn{OnData: onData, OnClosed: onClosed, opened: make(chan [2]int, 1)}
	c.handle = cgo.NewHandle(c)
	addr := C.CString(strings.ReplaceAll(strings.ToLower(address), ":", "-"))
	defer C.free(unsafe.Pointer(addr))

	var channelID, mtu C.int
	if ret := C.bt_open(addr, C.uintptr_t(c.handle), &channelID, &mtu); ret != 0 {
		c.handle.Delete()
		mu.Unlock()
		return nil, 0, describe("打开 RFCOMM", int(ret))
	}
	select {
	case r := <-c.opened:
		if r[0] != 0 {
			c.Close()
			return nil, 0, describe("打开 RFCOMM", r[0])
		}
		return c, r[1], nil
	case <-time.After(15 * time.Second):
		c.Close()
		return nil, 0, describe("打开 RFCOMM", -4)
	}
}

func (c *Conn) Write(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	if ret := C.bt_write(unsafe.Pointer(&p[0]), C.int(len(p))); ret != 0 {
		return describe("RFCOMM 写入", int(ret))
	}
	return nil
}

func (c *Conn) Close() {
	c.once.Do(func() {
		C.bt_close()
		c.handle.Delete()
		mu.Unlock()
	})
}

func describe(op string, ret int) error {
	switch ret {
	case -1:
		return fmt.Errorf("%s失败：找不到设备或连接未打开", op)
	case -2:
		return fmt.Errorf("%s失败：手环上没找到 SPP 服务", op)
	case -3:
		return fmt.Errorf("%s失败：通道为空", op)
	case -4:
		return fmt.Errorf("%s失败：15 秒内没等到通道打开", op)
	case 0xe00002d6 - (1 << 32):
		return fmt.Errorf("%s失败：超时（手环是不是连着手机，或者不在旁边？）", op)
	}
	return fmt.Errorf("%s失败 (IOReturn 0x%x)", op, uint32(ret))
}

//export goRfcommData
func goRfcommData(h C.uintptr_t, data unsafe.Pointer, length C.int) {
	c, ok := cgo.Handle(h).Value().(*Conn)
	if !ok || c.OnData == nil {
		return
	}
	c.OnData(C.GoBytes(data, length))
}

//export goRfcommOpened
func goRfcommOpened(h C.uintptr_t, status, mtu C.int) {
	if c, ok := cgo.Handle(h).Value().(*Conn); ok {
		select {
		case c.opened <- [2]int{int(status), int(mtu)}:
		default:
		}
	}
}

//export goRfcommClosed
func goRfcommClosed(h C.uintptr_t) {
	c, ok := cgo.Handle(h).Value().(*Conn)
	if ok && c.OnClosed != nil {
		c.OnClosed()
	}
}
