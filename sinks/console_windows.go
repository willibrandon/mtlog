//go:build windows
// +build windows

package sinks

import (
	"os"
	"sync"
	"syscall"
	"unsafe"
)

const (
	enableVirtualTerminalProcessing = 0x0004
)

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	vtProcessingEnabled sync.Once
)

// enableWindowsVTProcessing enables VT100 processing on Windows 10+
func enableWindowsVTProcessing() {
	vtProcessingEnabled.Do(func() {
		// Try to enable for stdout
		enableForHandle(os.Stdout.Fd())
		// Try to enable for stderr
		enableForHandle(os.Stderr.Fd())
	})
}

func enableForHandle(handle uintptr) {
	var mode uint32
	// Get current console mode
	ret, _, _ := procGetConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	if ret == 0 {
		return
	}
	
	// Check if VT processing is already enabled
	if mode&enableVirtualTerminalProcessing != 0 {
		return // Already enabled, avoid flicker
	}
	
	// Enable virtual terminal processing
	mode |= enableVirtualTerminalProcessing
	procSetConsoleMode.Call(handle, uintptr(mode))
}

