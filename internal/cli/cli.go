// Package cli parses the `aicw` command line and implements terminal-only
// commands (help, version, doctor). It never opens a window itself.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"

	"aicw/internal/agents"
	"aicw/internal/process"
	"aicw/internal/safety"
	"aicw/internal/storage"
)

const (
	KindOpen    = "open"
	KindHelp    = "help"
	KindVersion = "version"
	KindDoctor  = "doctor"
)

type Command struct {
	Kind string
	Path string // raw, unresolved; "" means no workspace requested
	New  bool
	Page string // optional UI page to show: one of Pages
}

// Pages are the valid values for --page.
var Pages = []string{"chat", "terminal", "pages", "overview", "workspaces", "fresh-start", "context", "media", "skills", "agents", "sessions", "settings"}

func validPage(p string) bool {
	for _, x := range Pages {
		if x == p {
			return true
		}
	}
	return false
}

const Usage = `AI Context Workspace (aicw)

Usage:
  aicw                  open the app (recent workspaces and resumable sessions)
  aicw .                open or resume the workspace in the current directory
  aicw <path>           open or resume the workspace at <path> (absolute, relative or ~/...)
  aicw --page <name> .  open a specific page (chat, terminal, skills, settings, overview, workspaces,
                        fresh-start, context, media, agents, sessions)
  aicw --new <path>     explicitly request a new Fresh Start session; old sessions are kept
  aicw tool <name> [json]   run an AICW tool (for agents without MCP; needs AICW_WORKSPACE or --workspace)
  aicw mcp --workspace <p>  MCP server over stdio exposing the same tools to agent CLIs
  aicw doctor           check installation, PATH, data directory, database and agent CLIs
  aicw --help           show this help
  aicw --version        show the version

Running "aicw ." resumes saved work by default. It never starts an AI session by itself.
`

// Parse interprets argv (without the program name).
func Parse(args []string) (Command, error) {
	cmd := Command{Kind: KindOpen}
	pathSet, rest := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !rest {
			switch a {
			case "--":
				rest = true
				continue
			case "-h", "--help", "help":
				return Command{Kind: KindHelp}, nil
			case "-v", "--version", "version":
				return Command{Kind: KindVersion}, nil
			case "--new":
				cmd.New = true
				continue
			case "--page":
				if i+1 >= len(args) {
					return cmd, fmt.Errorf("--page needs a value (%s)", strings.Join(Pages, ", "))
				}
				i++
				cmd.Page = args[i]
				if !validPage(cmd.Page) {
					return cmd, fmt.Errorf("unknown page %q (%s)", cmd.Page, strings.Join(Pages, ", "))
				}
				continue
			case "doctor":
				if !pathSet {
					return Command{Kind: KindDoctor}, nil
				}
			}
			if strings.HasPrefix(a, "-") && a != "-" {
				return cmd, fmt.Errorf("unknown option %q (try aicw --help)", a)
			}
		}
		if pathSet {
			return cmd, fmt.Errorf("only one path may be given (got %q and %q)", cmd.Path, a)
		}
		cmd.Path, pathSet = a, true
	}
	if cmd.New && !pathSet {
		return cmd, fmt.Errorf("--new needs a path (for example: aicw --new .)")
	}
	return cmd, nil
}

// ResolveRequest resolves a raw path against cwd. If a file is given and ask
// is non-nil, the user is asked whether to use the parent directory.
func ResolveRequest(raw, cwd string, ask func(parent string) bool) (string, error) {
	p, err := safety.ResolveDir(raw, cwd)
	if err == nil {
		return p, nil
	}
	if fe, ok := err.(*safety.IsFileError); ok && ask != nil && ask(fe.Parent) {
		return fe.Parent, nil
	}
	return "", err
}

// AskTTY prompts on a terminal; returns false when stdin is not interactive.
func AskTTY(in *os.File, out io.Writer) func(parent string) bool {
	return func(parent string) bool {
		if !isatty.IsTerminal(in.Fd()) {
			return false
		}
		fmt.Fprintf(out, "That is a file. Open its parent directory %s instead? [y/N] ", parent)
		line, _ := bufio.NewReader(in).ReadString('\n')
		l := strings.ToLower(strings.TrimSpace(line))
		return l == "y" || l == "yes"
	}
}

// Doctor prints a diagnostic report. It returns the number of failed checks.
func Doctor(w io.Writer, version, dataDir string) int {
	fails := 0
	check := func(ok bool, name, detail string) {
		mark := "ok  "
		if !ok {
			mark, fails = "FAIL", fails+1
		}
		fmt.Fprintf(w, "[%s] %s: %s\n", mark, name, detail)
	}
	info := func(name, detail string) { fmt.Fprintf(w, "[info] %s: %s\n", name, detail) }

	fmt.Fprintf(w, "aicw doctor (version %s)\n\n", version)
	exe, err := os.Executable()
	check(err == nil, "executable", exe)
	if p, err := exec.LookPath("aicw"); err != nil {
		check(false, "aicw on PATH", "not found; see docs/CLI.md for install/symlink instructions")
	} else {
		real1, _ := filepath.EvalSymlinks(p)
		real2, _ := filepath.EvalSymlinks(exe)
		check(real1 == real2, "aicw on PATH", p+mismatch(real1 != real2, " (resolves to a different binary than this one)"))
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		check(false, "data directory", dataDir+": "+err.Error())
	} else {
		f, err := os.CreateTemp(dataDir, ".doctor-*")
		if err == nil {
			f.Close()
			os.Remove(f.Name())
		}
		check(err == nil, "data directory writable", dataDir)
	}
	dbPath := filepath.Join(dataDir, "aicw.db")
	if s, err := storage.Open(dbPath); err != nil {
		check(false, "database", err.Error())
	} else {
		r, err := s.RecentWorkspaces(1)
		_ = r
		s.Close()
		check(err == nil, "database", dbPath)
	}
	_, gerr := exec.LookPath("git")
	check(gerr == nil, "git", mismatch(gerr != nil, "not found (Git status will be unavailable)")+mismatch(gerr == nil, "found"))

	if ts := process.ListTerminals(); len(ts) == 0 {
		info("terminal emulators", "none found; launches fall back to a copyable command")
	} else {
		names := []string{}
		for _, t := range ts {
			names = append(names, t.Name)
		}
		info("terminal emulators", strings.Join(names, ", "))
	}
	reg := agents.NewRegistry()
	fmt.Fprintln(w, "\nAgent CLIs:")
	for _, p := range reg.List() {
		st := reg.Detect(context.Background(), p.ID)
		v := ""
		if st.State == agents.StatusDetected {
			v = fmt.Sprintf(" %s (%s)", st.Path, st.Version)
		}
		ver := "unverified adapter"
		if p.Verified {
			ver = "adapter verified on " + p.VerifiedVersion
		}
		fmt.Fprintf(w, "  %-9s %-20s %s%s\n", p.ID, st.State, ver, v)
	}
	fmt.Fprintln(w)
	if fails == 0 {
		fmt.Fprintln(w, "All checks passed.")
	} else {
		fmt.Fprintf(w, "%d check(s) failed.\n", fails)
	}
	return fails
}

func mismatch(cond bool, s string) string {
	if cond {
		return s
	}
	return ""
}
