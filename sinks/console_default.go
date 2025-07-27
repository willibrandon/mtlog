//go:build !windows
// +build !windows

package sinks

// enableWindowsVTProcessing is a no-op on non-Windows platforms
func enableWindowsVTProcessing() {
	// VT processing is not needed on Unix-like systems
}