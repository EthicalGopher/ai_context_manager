//go:build windows

package service

import (
	"os"
	"os/exec"
	"strconv"
)

// killTreeOnCancel kills cmd and all of its children when its context is cancelled.
func killTreeOnCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		pid := strconv.Itoa(cmd.Process.Pid)
		if err := exec.Command("taskkill", "/T", "/F", "/PID", pid).Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}

func defaultShell() string {
	for _, c := range []string{"pwsh.exe", "powershell.exe"} {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	if sh := os.Getenv("COMSPEC"); sh != "" {
		return sh
	}
	return "cmd.exe"
}
