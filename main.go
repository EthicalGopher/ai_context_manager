package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"aicw/internal/cli"
	"aicw/internal/ipc"
	"aicw/internal/mcp"
	"aicw/internal/service"
	"aicw/internal/storage"
	"aicw/internal/tools"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version is overridden at build time: -ldflags "-X main.Version=1.2.3".
var Version = "0.1.0-dev"

func dataDir() string {
	if d := os.Getenv("AICW_DATA_DIR"); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "aicw")
		}
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "aicw")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "aicw")
	}
	return filepath.Join(home, ".local", "share", "aicw")
}

// agentCommand handles `aicw mcp` and `aicw tool`, which are used by agent
// CLIs rather than people, never open a window, and never touch the UI.
func agentCommand(args []string) (handled bool) {
	if len(args) == 0 || (args[0] != "mcp" && args[0] != "tool") {
		return false
	}
	ws := os.Getenv("AICW_WORKSPACE")
	var rest []string
	for i := 1; i < len(args); i++ {
		if args[i] == "--workspace" && i+1 < len(args) {
			ws = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	fail := func(msg string) bool { fmt.Fprintln(os.Stderr, "aicw:", msg); os.Exit(1); return true }
	if ws == "" {
		return fail("no workspace: pass --workspace <path> or set AICW_WORKSPACE")
	}
	cwd, _ := os.Getwd()
	path, err := cli.ResolveRequest(ws, cwd, nil)
	if err != nil {
		return fail(err.Error())
	}
	st, err := storage.Open(filepath.Join(dataDir(), "aicw.db"))
	if err != nil {
		return fail("cannot open data store: " + err.Error())
	}
	defer st.Close()
	w, err := st.WorkspaceByPath(path)
	if err != nil || w == nil {
		return fail("workspace is not known to AICW yet; open it once with `aicw " + path + "`")
	}
	env := tools.Env{Store: st, DataDir: dataDir(), WorkspaceID: w.ID}
	if args[0] == "mcp" {
		if err := mcp.Serve(os.Stdin, os.Stdout, env, Version); err != nil {
			return fail(err.Error())
		}
		return true
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: aicw tool <name> [json-arguments]   (tools:")
		for _, t := range tools.All() {
			fmt.Fprintf(os.Stderr, "  %-20s %s\n", t.Name, t.Description)
		}
		os.Exit(2)
	}
	var argJSON json.RawMessage
	if len(rest) > 1 {
		argJSON = json.RawMessage(rest[1])
	}
	out, err := tools.Call(env, rest[0], argJSON)
	if err != nil {
		return fail(err.Error())
	}
	fmt.Println(out)
	return true
}

func main() {
	if agentCommand(os.Args[1:]) {
		return
	}
	cmd, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "aicw:", err)
		os.Exit(2)
	}
	switch cmd.Kind {
	case cli.KindHelp:
		fmt.Print(cli.Usage)
		return
	case cli.KindVersion:
		fmt.Println("aicw", Version)
		return
	case cli.KindDoctor:
		if cli.Doctor(os.Stdout, Version, dataDir()) > 0 {
			os.Exit(1)
		}
		return
	}

	// Validate the requested path before any window opens. Nothing is created.
	cwd, _ := os.Getwd()
	var resolved string
	if cmd.Path != "" {
		resolved, err = cli.ResolveRequest(cmd.Path, cwd, cli.AskTTY(os.Stdin, os.Stderr))
		if err != nil {
			fmt.Fprintln(os.Stderr, "aicw:", err)
			os.Exit(1)
		}
	}

	// If AICW is already running, hand the request over and exit cleanly.
	sock := filepath.Join(dataDir(), "aicw.sock")
	if err := ipc.Send(sock, ipc.Request{Args: os.Args[1:], Cwd: cwd}); err == nil {
		fmt.Println("aicw: opened in the running AI Context Workspace window")
		return
	}

	svc, err := service.New(dataDir(), Version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "aicw: cannot open data store:", err)
		os.Exit(1)
	}
	defer svc.Lifecycle().Close()

	if resolved != "" {
		if svc.Initial, err = svc.WS.Open(resolved, cmd.New); err != nil {
			svc.InitialError = err.Error()
		} else {
			svc.Initial.Page = cmd.Page
		}
	}

	if l, err := ipc.Listen(sock, func(r ipc.Request) { svc.Lifecycle().HandleExternalOpen(r.Args, r.Cwd) }); err == nil {
		defer l.Close()
		defer os.Remove(sock)
	}

	err = wails.Run(&options.App{
		Title:     "AI Context Workspace",
		Width:     1280,
		Height:    820,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: svc.MediaHandler(),
		},
		DragAndDrop:      &options.DragAndDrop{EnableFileDrop: true},
		BackgroundColour: &options.RGBA{R: 15, G: 17, B: 21, A: 1},
		OnStartup:        svc.Lifecycle().Startup,
		// A second `aicw <path>` hands its request to this instance instead of
		// opening a duplicate window.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "dev.aicw.workspace-manager",
			OnSecondInstanceLaunch: func(d options.SecondInstanceData) {
				svc.Lifecycle().HandleExternalOpen(d.Args, d.WorkingDirectory)
			},
		},
		Linux: &linux.Options{ProgramName: "aicw", WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand},
		Bind:  []interface{}{svc},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "aicw:", err)
		os.Exit(1)
	}
}
