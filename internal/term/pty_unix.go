//go:build !windows

package term

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

type unixPTY struct {
	cmd *exec.Cmd
	f   *os.File
}

func startPTY(sp Spec) (ptyConn, error) {
	cmd := exec.Command(sp.Exe, sp.Args...)
	cmd.Dir = sp.Dir
	cmd.Env = sp.Env
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: sp.Cols, Rows: sp.Rows})
	if err != nil {
		return nil, err
	}
	return &unixPTY{cmd: cmd, f: f}, nil
}

func (p *unixPTY) Read(b []byte) (int, error)  { return p.f.Read(b) }
func (p *unixPTY) Write(b []byte) (int, error) { return p.f.Write(b) }
func (p *unixPTY) Close() error                { return p.f.Close() }
func (p *unixPTY) Pid() int                    { return p.cmd.Process.Pid }

func (p *unixPTY) Resize(cols, rows uint16) error {
	return pty.Setsize(p.f, &pty.Winsize{Cols: cols, Rows: rows})
}

func (p *unixPTY) Wait() int {
	if ee, ok := p.cmd.Wait().(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 0
}

func (p *unixPTY) Terminate(force bool) {
	sig := syscall.SIGHUP
	if force {
		sig = syscall.SIGKILL
	}
	pid := p.cmd.Process.Pid
	syscall.Kill(-pid, sig)
	p.cmd.Process.Signal(sig)
}
