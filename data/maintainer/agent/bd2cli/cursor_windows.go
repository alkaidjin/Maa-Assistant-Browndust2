//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	procSetCursorPos         = user32.NewProc("SetCursorPos")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procSendMessageTimeoutW  = user32.NewProc("SendMessageTimeoutW")
	procGetWindowThreadProc  = user32.NewProc("GetWindowThreadProcessId")
)

// rawSendMessageTimeout 往窗口投一条空消息（WM_NULL），用来判断对方是否拒收。
// 返回 ok=false 且 errno=5 通常意味着窗口权限比我们高（UIPI 拦了）。
func rawSendMessageTimeout(hwnd uintptr) (bool, syscall.Errno) {
	var res uintptr
	// SMTO_ABORTIFHUNG = 0x0002
	r, _, e := procSendMessageTimeoutW.Call(hwnd, 0, 0, 0, 0x0002, 1000, uintptr(unsafe.Pointer(&res)))
	if r != 0 {
		return true, 0
	}
	if errno, ok := e.(syscall.Errno); ok {
		return false, errno
	}
	return false, 0
}

var (
	procKeybdEvent = user32.NewProc("keybd_event")
	procMouseEvent = user32.NewProc("mouse_event")
)

const (
	keyeventfKeyup      = 0x0002
	mouseeventfLeftDown = 0x0002
	mouseeventfLeftUp   = 0x0004
)

// rawKey 用系统级事件按一下键（不经过 MaaFramework）。
func rawKey(vk uint32) {
	procKeybdEvent.Call(uintptr(vk), 0, 0, 0)
	time.Sleep(60 * time.Millisecond)
	procKeybdEvent.Call(uintptr(vk), 0, uintptr(keyeventfKeyup), 0)
}

// rawClick 把鼠标移到 (x,y) 并做一次真实左键单击。
func rawClick(x, y int) (bool, syscall.Errno) {
	ok, errno := rawSetCursorPos(x, y)
	time.Sleep(150 * time.Millisecond)
	procMouseEvent.Call(uintptr(mouseeventfLeftDown), 0, 0, 0, 0)
	time.Sleep(70 * time.Millisecond)
	procMouseEvent.Call(uintptr(mouseeventfLeftUp), 0, 0, 0, 0)
	return ok, errno
}

type winPoint struct {
	X int32
	Y int32
}

// rawSetCursorPos 直接调 user32!SetCursorPos，用来判断"本进程有没有资格动鼠标"。
func rawSetCursorPos(x, y int) (bool, syscall.Errno) {
	r, _, e := procSetCursorPos.Call(uintptr(int32(x)), uintptr(int32(y)))
	if r != 0 {
		return true, 0
	}
	if errno, ok := e.(syscall.Errno); ok {
		return false, errno
	}
	return false, 0
}

// rawGetCursorPos 读当前鼠标坐标。
func rawGetCursorPos() (int, int, bool) {
	var p winPoint
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return int(p.X), int(p.Y), r != 0
}
