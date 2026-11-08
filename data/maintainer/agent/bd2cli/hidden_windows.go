//go:build windows

package main

import "syscall"

// hiddenWindowAttr 让外部命令不弹黑框。
func hiddenWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
