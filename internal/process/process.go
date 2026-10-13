// Package process launches agent CLIs in an external terminal emulator using
// argument arrays only (never a shell string).
package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"aicw/internal/agents"
)

// Terminal describes how to run a command in a given emulator.
type Terminal struct {
	Name string   `json:"name"`
	Path string   `json:"path"`
	pre  []string // arguments placed before the command (may include {dir})
}

type termDef struct {
	name string
	pre  []string
}

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

// ListTerminals returns terminal emulators found on PATH.
func ListTerminals() []Terminal {
	var out []Terminal
	for _, d := range termDefs {
		if p, err := exec.LookPath(d.name); err == nil {
			out = append(out, Terminal{Name: d.name, Path: p, pre: d.pre})
		}
	}
	return out
}

// FindTerminal picks the preferred terminal by name, or the first available.
func FindTerminal(pref string) (Terminal, error) {
	if runtime.GOOS != "linux" {
		return Terminal{}, fmt.Errorf("automatic terminal launch is only implemented on Linux (this is %s); use the copyable command instead", runtime.GOOS)
	}
	ts := ListTerminals()
	if len(ts) == 0 {
		return Terminal{}, errors.New("no supported terminal emulator found on PATH; copy the command and run it manually")
	}
	if pref != "" {
		for _, t := range ts {
			if t.Name == pref {
				return t, nil
			}
		}
		return Terminal{}, fmt.Errorf("configured terminal %q was not found", pref)
	}
	return ts[0], nil
}

// Command builds (without starting) the terminal command for a spec.
func (t Terminal) Command(spec agents.LaunchSpec) *exec.Cmd {
	var args []string
	for _, a := range t.pre {
		if a == "{dir}" {
			a = spec.Dir
		} else if len(a) > 5 && a[len(a)-5:] == "{dir}" {
			a = a[:len(a)-5] + spec.Dir
		}
		args = append(args, a)
	}
	args = append(args, spec.Exe)
	args = append(args, spec.Args...)
	cmd := exec.Command(t.Path, args...)
	cmd.Dir = spec.Dir
	cmd.Env = os.Environ()
	return cmd
}

// Launch starts the agent in a terminal and returns the launcher PID. The
// process is detached so closing AICW does not kill the agent.
func Launch(t Terminal, spec agents.LaunchSpec) (int, error) {
	if st, err := os.Stat(spec.Dir); err != nil || !st.IsDir() {
		return 0, fmt.Errorf("working directory unavailable: %s", spec.Dir)
	}
	cmd := t.Command(spec)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start %s: %w", t.Name, err)
	}
	pid := cmd.Process.Pid
	go cmd.Wait() // reap; the terminal may fork and exit immediately
	return pid, nil
}

// Alive reports whether a process with this PID exists.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
