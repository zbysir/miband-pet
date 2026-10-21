package bt

/*
#include <stdlib.h>
#include "ble_darwin.h"
*/
import "C"

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime/cgo"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// BLEConn 是一条 BLE 连接（fe95 服务的 5e/5f 特征）。和 Conn 一样实现字节流读写。
type BLEConn struct {
	OnData   func([]byte)
	OnClosed func()

	handle cgo.Handle
	found  chan string // 扫描到的外设标识
	opened chan [2]int // status, 单次写入上限
	chunk  int
	once   sync.Once
}

// 一个进程同时只开一条 BLE 连接（Objective-C 那边也是这么假设的）
var bleMu sync.Mutex

// OpenBLE 扫描并连接 MAC 为 address 的手环。手环在二维码界面（连接新手机）或平时没连着时会广播。
func OpenBLE(ctx context.Context, address string, onData func([]byte), onClosed func()) (*BLEConn, error) {
	mac, err := hex.DecodeString(strings.ReplaceAll(address, ":", ""))
	if err != nil || len(mac) != 6 {
		return nil, fmt.Errorf("手环地址格式不对: %s", address)
	}
	rev := make([]byte, 6)
	for i := range mac {
		rev[i] = mac[5-i]
	}
	suffix := strings.ToUpper(hex.EncodeToString(mac[4:]))

	bleMu.Lock()
	c := &BLEConn{OnData: onData, OnClosed: onClosed, found: make(chan string, 1), opened: make(chan [2]int, 1)}
	c.handle = cgo.NewHandle(c)
	cs := C.CString(suffix)
	defer C.free(unsafe.Pointer(cs))
	C.ble_scan_start((*C.char)(unsafe.Pointer(&rev[0])), C.int(len(rev)), cs, C.uintptr_t(c.handle))

	var id string
	select {
	case id = <-c.found:
	case <-ctx.Done():
		C.ble_scan_stop()
		c.release()
		return nil, errors.New("没有扫描到手环")
	}
	C.ble_scan_stop()

	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	C.ble_connect(cid, C.uintptr_t(c.handle))
	select {
	case r := <-c.opened:
		if r[0] != 0 {
			c.Close()
			return nil, fmt.Errorf("BLE 连接手环失败 (%d)", r[0])
		}
		c.chunk = r[1]
		return c, nil
	case <-time.After(20 * time.Second):
		c.Close()
		return nil, errors.New("BLE 连接手环超时")
	}
}

func (c *BLEConn) Write(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	C.ble_write(unsafe.Pointer(&p[0]), C.int(len(p)), C.int(c.chunk))
	return nil
}

func (c *BLEConn) Close() {
	C.ble_disconnect()
	c.release()
}

func (c *BLEConn) release() {
	c.once.Do(func() {
		c.handle.Delete()
		bleMu.Unlock()
	})
}

func bleConn(h C.uintptr_t) *BLEConn {
	defer func() { recover() }() // handle 可能已经释放（连接关掉后还有晚到的回调）
	c, _ := cgo.Handle(h).Value().(*BLEConn)
	return c
}

//export goBleFound
func goBleFound(h C.uintptr_t, id *C.char, rssi C.int) {
	if c := bleConn(h); c != nil {
		select {
		case c.found <- C.GoString(id):
		default:
		}
	}
}

//export goBleOpened
func goBleOpened(h C.uintptr_t, status, chunk C.int) {
	if c := bleConn(h); c != nil {
		select {
		case c.opened <- [2]int{int(status), int(chunk)}:
		default:
		}
	}
}

//export goBleData
func goBleData(h C.uintptr_t, data unsafe.Pointer, length C.int) {
	if c := bleConn(h); c != nil && c.OnData != nil {
		c.OnData(C.GoBytes(data, length))
	}
}

//export goBleClosed
func goBleClosed(h C.uintptr_t) {
	if c := bleConn(h); c != nil && c.OnClosed != nil {
		c.OnClosed()
	}
}
