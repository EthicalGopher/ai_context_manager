//go:build windows

package term

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"
)

type winPTY struct {
	c *conpty.ConPty
}

func startPTY(sp Spec) (ptyConn, error) {
	if !conpty.IsConPtyAvailable() {
		return nil, fmt.Errorf("ConPTY requires Windows 10 version 1809 or newer")
	}
	argv := append([]string{sp.Exe}, sp.Args...)
	if ext := strings.ToLower(filepath.Ext(sp.Exe)); ext == ".cmd" || ext == ".bat" {
		// npm-installed agent CLIs are .cmd shims, which need cmd.exe.
		argv = append([]string{"cmd.exe", "/c"}, argv...)
	}
	opts := []conpty.ConPtyOption{
		conpty.ConPtyDimensions(int(sp.Cols), int(sp.Rows)),
		conpty.ConPtyWorkDir(sp.Dir),
	}
	if sp.Env != nil {
		opts = append(opts, conpty.ConPtyEnv(sp.Env))
	}
	c, err := conpty.Start(windows.ComposeCommandLine(argv), opts...)
	if err != nil {
		return nil, err
	}
	return &winPTY{c: c}, nil
}

func (p *winPTY) Read(b []byte) (int, error)  { return p.c.Read(b) }
func (p *winPTY) Write(b []byte) (int, error) { return p.c.Write(b) }
func (p *winPTY) Close() error                { return p.c.Close() }
func (p *winPTY) Pid() int                    { return p.c.Pid() }

func (p *winPTY) Resize(cols, rows uint16) error { return p.c.Resize(int(cols), int(rows)) }

func (p *winPTY) Wait() int {
	code, err := p.c.Wait(context.Background())
	if err != nil {
		return -1
	}
	return int(code)
}

// Terminate has no hang-up signal on Windows: the polite attempt sends Ctrl+C
// through the console, the forced one kills the whole process tree.
func (p *winPTY) Terminate(force bool) {
	if !force {
		p.c.Write([]byte{0x03})
		return
	}
	exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.c.Pid())).Run()
}
