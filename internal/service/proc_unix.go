//go:build !windows

package service

import (
	"os"
	"os/exec"
	"syscall"
)

// killTreeOnCancel runs cmd in its own process group and kills the whole
// group when its context is cancelled.
func killTreeOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

func defaultShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/bash"
}
