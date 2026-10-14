//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// The release binary is a GUI-subsystem executable (no console window when
// launched from Explorer). When started from a terminal with no redirected
// output, attach to that terminal so `aicw doctor` and friends can print.
func init() {
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 {
		return // output is already redirected (pipe/file), as for `aicw mcp`
	}
	if r, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole").Call(^uintptr(0)); r == 0 {
		return
	}
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = out, out
	}
	if in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = in
	}
}
