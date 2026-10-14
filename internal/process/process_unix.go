//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

// {dir} is substituted with the working directory as a single argument.
var termDefs = []termDef{
	{"konsole", []string{"--workdir", "{dir}", "-e"}},
	{"gnome-terminal", []string{"--working-directory={dir}", "--"}},
	{"kitty", []string{"--directory", "{dir}"}},
	{"alacritty", []string{"--working-directory", "{dir}", "-e"}},
	{"wezterm", []string{"start", "--cwd", "{dir}", "--"}},
	{"foot", []string{"--working-directory={dir}"}},
	{"xfce4-terminal", []string{"--working-directory={dir}", "-x"}},
	{"xterm", []string{"-e"}},
	{"x-terminal-emulator", []string{"-e"}},
}

func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

// Alive reports whether a process with this PID exists.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
