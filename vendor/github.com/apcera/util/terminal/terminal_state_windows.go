// Copyright 2014 Apcera Inc. All rights reserved.

// +build windows

package terminal

import (
	"os"
	"syscall"
	"unsafe"
)

// Windows terminal code adapted from golang.org/x/crypto/ssh/terminal
var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

type WindowsTerminalState struct {
	mode uint32
}

func Isatty(fd uintptr) bool {
	var st uint32
	r, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, fd, uintptr(unsafe.Pointer(&st)), 0)
	return r != 0 && e == 0
}

func getOSTerminalState() (*WindowsTerminalState, error) {
	fd := uintptr(os.Stdout.Fd())
	var st uint32
	_, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, fd, uintptr(unsafe.Pointer(&st)), 0)
	if e != 0 {
		return nil, error(e)
	}
	return &WindowsTerminalState{st}, nil
}

func (wts *WindowsTerminalState) IsValid() bool {
	if wts == nil {
		return false
	}
	return true
}

func (wts *WindowsTerminalState) Restore() {
	if !wts.IsValid() {
		return
	}
	_, _, _ = syscall.Syscall(procSetConsoleMode.Addr(), 2, uintptr(os.Stdout.Fd()), uintptr(wts.mode), 0)
	return
}
