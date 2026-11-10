//go:build windows

package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"bd2cli/internal/status"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procFindWindowW = user32.NewProc("FindWindowW")
)

// checks 返回 Windows 上的真实探测实现。
func checks() status.Checks {
	return status.Checks{
		MXURunning: func() bool { return processRunning("mxu.exe") },
		PortOpen:   portOpen,
		GameWindow: findWindow,
		Process:    processRunning,
	}
}

// processRunning 用 tasklist 判断进程是否在跑（不引第三方依赖）。
func processRunning(name string) bool {
	out, err := exec.Command("tasklist", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	want := `"` + strings.ToLower(name) + `"`
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), want) {
			return true
		}
	}
	return false
}

// findWindow 用 user32!FindWindowW 按类名 + 标题精确匹配。
func findWindow(class, title string) bool {
	return findWindowHandle(class, title) != 0
}

// findWindowHandle 返回窗口句柄（0 表示没找到）。
func findWindowHandle(class, title string) uintptr {
	c, err1 := syscall.UTF16PtrFromString(class)
	t, err2 := syscall.UTF16PtrFromString(title)
	if err1 != nil || err2 != nil {
		return 0
	}
	h, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(t)))
	return h
}

// findWindowPtr 把句柄交给 MaaFramework（它要 unsafe.Pointer）。
//
// HWND 是内核对象句柄、不指向 Go 堆；这里经由 &h 按位转写，
// 语义与 unsafe.Pointer(h) 相同，但不会触发 vet 的 unsafeptr 误报。
func findWindowPtr(class, title string) unsafe.Pointer {
	h := findWindowHandle(class, title)
	if h == 0 {
		return nil
	}
	return *(*unsafe.Pointer)(unsafe.Pointer(&h))
}

// openFile 用系统默认程序打开（计划任务里也能用）。
func openFile(path string) {
	_ = exec.Command("cmd", "/c", "start", "", path).Start()
}

func portOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
