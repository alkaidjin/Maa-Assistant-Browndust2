//go:build !windows

package main

import (
	"syscall"
	"unsafe"

	"bd2cli/internal/status"
)

// checks 在非 Windows 上返回空探测，Build 会跳过依赖外部环境的字段。
func checks() status.Checks { return status.Checks{} }

// findWindowHandle 在非 Windows 上恒为 0。
func findWindowHandle(string, string) uintptr { return 0 }

// findWindowPtr 在非 Windows 上恒为 nil。
func findWindowPtr(string, string) unsafe.Pointer { return nil }

// rawKey / rawClick 在非 Windows 上不可用。
func rawKey(uint32) {}

func rawClick(int, int) (bool, syscall.Errno) { return false, 0 }

// hiddenWindowAttr 在非 Windows 上无意义。
func hiddenWindowAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{} }

// openFile 在非 Windows 上不做任何事。
func openFile(string) {}
