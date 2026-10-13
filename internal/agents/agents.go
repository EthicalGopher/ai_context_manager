// Package agents provides a provider-agnostic registry of AI coding CLIs.
// Each provider is a declarative Profile (argument templates, capabilities,
// verification status). Adding a provider never touches core code, and
// imported/custom profiles are validated data: they are never executed as
// scripts, and arguments are always passed as an argv array.
package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Handoff methods describe how prepared context reaches the agent.
const (
	HandoffPromptArg = "prompt-arg" // short initial prompt that points at the context file
	HandoffFile      = "file"       // context file written; user tells agent to read it
	HandoffClipboard = "clipboard"  // context copied to clipboard
	HandoffNone      = "none"
)

// Detection statuses shown on the compatibility screen.
const (
	StatusDetected    = "Detected"
	StatusNotFound    = "Not installed"
	StatusNeedsConfig = "Needs configuration"
	StatusUnknown     = "Unsupported/unknown"
)

type Capabilities struct {
	NewSession      bool `json:"newSession"`
	ResumeByID      bool `json:"resumeById"`      // can resume a specific conversation
	ContinueLast    bool `json:"continueLast"`    // can continue the most recent one
	AssignSessionID bool `json:"assignSessionId"` // caller may choose the ID up front
	InitialPrompt   bool `json:"initialPrompt"`
	ReadsInstrFiles bool `json:"readsInstructionFiles"`
	NonInteractive  bool `json:"nonInteractive"`
	PermissionModes bool `json:"permissionControls"`
	WorkingDirFlag  bool `json:"workingDirFlag"`
}

// Profile declares how to drive one CLI. Placeholders in arg templates:
// {prompt} {session} {cwd} {contextFile}. Each template element becomes one
// argv entry, so values are never re-parsed.
type Profile struct {
	ID              string       `json:"id"`
	DisplayName     string       `json:"displayName"`
	Executables     []string     `json:"executables"`
	VersionArgs     []string     `json:"versionArgs"`
	Docs            string       `json:"docs"`
	OS              []string     `json:"os,omitempty"` // empty = all
	Caps            Capabilities `json:"capabilities"`
	NewArgs         []string     `json:"newArgs"`
	PromptArgs      []string     `json:"promptArgs"` // appended when a prompt is passed
	ResumeArgs      []string     `json:"resumeArgs"`
	ContinueArgs    []string     `json:"continueArgs"`
	SessionIDArgs   []string     `json:"sessionIdArgs"` // appended on new sessions when AssignSessionID
	Handoff         string       `json:"handoff"`
	Verified        bool         `json:"verified"`        // flags checked against the real CLI
	VerifiedVersion string       `json:"verifiedVersion"` // version the flags were checked on
	Builtin         bool         `json:"builtin"`
	Notes           string       `json:"notes"`
	EnvNames        []string     `json:"envNames,omitempty"` // names only; values are never stored
	ExtraArgs       []string     `json:"extraArgs,omitempty"`
	Chat            *ChatSpec    `json:"chat,omitempty"` // non-interactive "one message in, one reply out" mode
}

// ChatSpec describes how to run one chat turn without a terminal UI. Arg
// templates use {prompt} and {session}. PromptVia "stdin" pipes the prompt
// instead of passing it as an argument (no quoting, length or leading-dash
// problems).
type ChatSpec struct {
	NewArgs         []string `json:"newArgs"`
	ResumeArgs      []string `json:"resumeArgs"`   // needs {session}
	ContinueArgs    []string `json:"continueArgs"` // fallback when the CLI picks its own session IDs
	SessionIDArgs   []string `json:"sessionIdArgs"`
	EditArgs        []string `json:"editArgs"`  // appended only when the user allows file edits
	PromptVia       string   `json:"promptVia"` // "arg" (default) or "stdin"
	AssignSessionID bool     `json:"assignSessionId"`
}

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,39}$`)
var placeholderRe = regexp.MustCompile(`\{[a-zA-Z]+\}`)
var knownPlaceholders = map[string]bool{"{prompt}": true, "{session}": true, "{cwd}": true, "{contextFile}": true}
var envNameRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// Validate checks a profile (typically user-supplied or imported).
func (p Profile) Validate() error {
	if !idRe.MatchString(p.ID) {
		return fmt.Errorf("invalid id %q (lowercase letters, digits, . _ -)", p.ID)
	}
	if strings.TrimSpace(p.DisplayName) == "" {
		return errors.New("displayName is required")
	}
	if len(p.Executables) == 0 {
		return errors.New("at least one executable candidate is required")
	}
	for _, e := range p.Executables {
		if strings.TrimSpace(e) == "" || strings.ContainsAny(e, "\n\x00") {
			return fmt.Errorf("invalid executable %q", e)
		}
	}
	switch p.Handoff {
	case HandoffPromptArg, HandoffFile, HandoffClipboard, HandoffNone:
	default:
		return fmt.Errorf("invalid handoff %q", p.Handoff)
	}
	for _, list := range [][]string{p.NewArgs, p.PromptArgs, p.ResumeArgs, p.ContinueArgs, p.SessionIDArgs, p.ExtraArgs, p.VersionArgs} {
		for _, a := range list {
			if strings.ContainsRune(a, 0) {
				return errors.New("argument contains NUL")
			}
			for _, ph := range placeholderRe.FindAllString(a, -1) {
				if !knownPlaceholders[ph] {
					return fmt.Errorf("unknown placeholder %s", ph)
				}
			}
		}
	}
	if p.Chat != nil {
		if p.Chat.PromptVia != "" && p.Chat.PromptVia != "arg" && p.Chat.PromptVia != "stdin" {
			return errors.New("chat.promptVia must be arg or stdin")
		}
		for _, list := range [][]string{p.Chat.NewArgs, p.Chat.ResumeArgs, p.Chat.ContinueArgs, p.Chat.SessionIDArgs, p.Chat.EditArgs} {
			for _, a := range list {
				if strings.ContainsRune(a, 0) {
					return errors.New("chat argument contains NUL")
				}
				for _, ph := range placeholderRe.FindAllString(a, -1) {
					if ph != "{prompt}" && ph != "{session}" {
						return fmt.Errorf("unknown chat placeholder %s", ph)
					}
				}
			}
		}
	}
	for _, n := range p.EnvNames {
		if !envNameRe.MatchString(n) {
			return fmt.Errorf("invalid environment variable name %q", n)
		}
	}
	if p.Handoff == HandoffPromptArg && len(p.PromptArgs) == 0 {
		return errors.New("handoff prompt-arg requires promptArgs")
	}
	return nil
}

// Status is the result of detection.
type Status struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Path     string `json:"path"`
	Version  string `json:"version"`
	Detail   string `json:"detail"`
	Verified bool   `json:"verified"`
}

// Registry holds built-in and custom profiles plus configured executable paths.
type Registry struct {
	profiles map[string]Profile
	// Paths maps provider ID to a user-configured executable path.
	Paths map[string]string
}

func NewRegistry() *Registry {
	r := &Registry{profiles: map[string]Profile{}, Paths: map[string]string{}}
	for _, p := range Builtins() {
		r.profiles[p.ID] = p
	}
	return r
}

// Add registers a custom profile. Built-in IDs cannot be replaced.
func (r *Registry) Add(p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if old, ok := r.profiles[p.ID]; ok && old.Builtin {
		return fmt.Errorf("%q is a built-in adapter and cannot be replaced", p.ID)
	}
	p.Builtin, p.Verified = false, false // imported definitions are never "verified"
	r.profiles[p.ID] = p
	return nil
}

func (r *Registry) Remove(id string) {
	if p, ok := r.profiles[id]; ok && !p.Builtin {
		delete(r.profiles, id)
	}
}

func (r *Registry) Get(id string) (Profile, bool) { p, ok := r.profiles[id]; return p, ok }

func (r *Registry) List() []Profile {
	out := make([]Profile, 0, len(r.profiles))
	for _, p := range r.profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Builtin != out[j].Builtin {
			return out[i].Builtin
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Detect finds the executable and (safely) its version.
func (r *Registry) Detect(ctx context.Context, id string) Status {
	p, ok := r.profiles[id]
	if !ok {
		return Status{ID: id, State: StatusUnknown, Detail: "no such adapter"}
	}
	st := Status{ID: id, Verified: p.Verified}
	if len(p.OS) > 0 && !contains(p.OS, runtime.GOOS) {
		st.State, st.Detail = StatusUnknown, "not supported on "+runtime.GOOS
		return st
	}
	exe := r.Paths[id]
	if exe != "" {
		if fi, err := os.Stat(exe); err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			st.State, st.Detail = StatusNeedsConfig, "configured path is not an executable file: "+exe
			return st
		}
	} else {
		for _, c := range p.Executables {
			if path, err := exec.LookPath(c); err == nil {
				exe = path
				break
			}
		}
	}
	if exe == "" {
		st.State, st.Detail = StatusNotFound, "none of "+strings.Join(p.Executables, ", ")+" found on PATH"
		return st
	}
	st.Path, st.State = exe, StatusDetected
	st.Version = version(ctx, exe, p.VersionArgs)
	return st
}

func version(ctx context.Context, exe string, args []string) string {
	if len(args) == 0 {
		args = []string{"--version"}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdin = nil
	out, _ := cmd.Output()
	for _, l := range strings.Split(string(out), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && regexp.MustCompile(`\d`).MatchString(l) {
			if len(l) > 80 {
				l = l[:80]
			}
			return l
		}
	}
	return ""
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// Mode selects how to start.
const (
	ModeNew      = "new"
	ModeResume   = "resume"
	ModeContinue = "continue"
)

type LaunchRequest struct {
	Mode        string
	Cwd         string
	SessionID   string // resume target, or ID to pre-assign for new sessions
	ContextFile string
	Prompt      string
	Exe         string   // resolved executable path
	Extra       []string // extra arguments placed before the prompt (already final, never templated)
}

// LaunchSpec is a fully built, shell-free command.
type LaunchSpec struct {
	Exe        string   `json:"exe"`
	Args       []string `json:"args"`
	Dir        string   `json:"dir"`
	Env        []string `json:"env"`   // names only, for display
	Notes      []string `json:"notes"` // honest limitations shown to the user
	Handoff    string   `json:"handoff"`
	ExternalID string   `json:"externalId"` // session ID the CLI will use, when known
	Display    string   `json:"display"`    // copyable approximation (not executed)
}

// BuildLaunch builds the argv for a profile. It never invokes a shell.
func BuildLaunch(p Profile, req LaunchRequest) (*LaunchSpec, error) {
	if req.Cwd == "" || !filepath.IsAbs(req.Cwd) {
		return nil, errors.New("working directory must be absolute")
	}
	if req.Exe == "" {
		return nil, errors.New("executable not resolved")
	}
	sub := func(tpl []string) []string {
		var out []string
		for _, a := range tpl {
			a = strings.ReplaceAll(a, "{prompt}", req.Prompt)
			a = strings.ReplaceAll(a, "{session}", req.SessionID)
			a = strings.ReplaceAll(a, "{cwd}", req.Cwd)
			a = strings.ReplaceAll(a, "{contextFile}", req.ContextFile)
			out = append(out, a)
		}
		return out
	}
	spec := &LaunchSpec{Exe: req.Exe, Dir: req.Cwd, Handoff: p.Handoff, Env: p.EnvNames}
	var args []string
	switch req.Mode {
	case ModeResume:
		switch {
		case p.Caps.ResumeByID && req.SessionID != "" && len(p.ResumeArgs) > 0:
			args = sub(p.ResumeArgs)
			spec.ExternalID = req.SessionID
		case p.Caps.ContinueLast && len(p.ContinueArgs) > 0:
			args = sub(p.ContinueArgs)
			spec.Notes = append(spec.Notes, p.DisplayName+" cannot resume a specific conversation here; this continues its most recent conversation in the directory, which may not be the one AICW recorded.")
		default:
			return nil, fmt.Errorf("%s has no verified resume mechanism; start a new session with the saved AICW handoff instead", p.DisplayName)
		}
	case ModeContinue:
		if !p.Caps.ContinueLast || len(p.ContinueArgs) == 0 {
			return nil, fmt.Errorf("%s cannot continue the last conversation", p.DisplayName)
		}
		args = sub(p.ContinueArgs)
	case ModeNew, "":
		if !p.Caps.NewSession {
			return nil, fmt.Errorf("%s cannot start a new session", p.DisplayName)
		}
		args = sub(p.NewArgs)
		if p.Caps.AssignSessionID && req.SessionID != "" && len(p.SessionIDArgs) > 0 {
			args = append(args, sub(p.SessionIDArgs)...)
			spec.ExternalID = req.SessionID
		} else {
			spec.Notes = append(spec.Notes, "This CLI does not let AICW choose its session ID; AICW can restore its own context but may not be able to resume the CLI's conversation.")
		}
	default:
		return nil, fmt.Errorf("unknown launch mode %q", req.Mode)
	}
	args = append(args, req.Extra...)
	switch {
	case req.ContextFile == "" && req.Prompt == "":
	case p.Handoff == HandoffPromptArg && p.Caps.InitialPrompt:
		args = append(args, sub(p.PromptArgs)...)
	case p.Handoff == HandoffClipboard:
		spec.Notes = append(spec.Notes, "Context will be copied to the clipboard; paste it into the agent.")
	default:
		spec.Handoff = HandoffFile
		spec.Notes = append(spec.Notes, "This CLI has no verified initial-prompt option. Context was written to "+req.ContextFile+"; ask the agent to read that file.")
	}
	args = append(args, p.ExtraArgs...)
	spec.Args = args
	spec.Display = shellQuote(append([]string{spec.Exe}, args...))
	spec.Notes = append(spec.Notes, p.DisplayName+" can read and modify files and run commands in "+req.Cwd+" according to its own permission settings, and sends context to its provider.")
	return spec, nil
}

func shellQuote(argv []string) string {
	q := make([]string, len(argv))
	for i, a := range argv {
		if a == "" || regexp.MustCompile(`[^A-Za-z0-9_@%+=:,./-]`).MatchString(a) {
			a = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
		q[i] = a
	}
	return strings.Join(q, " ")
}

type ChatRequest struct {
	Cwd       string
	Exe       string
	Prompt    string
	SessionID string // existing CLI session to continue ("" = first turn)
	NewID     string // ID to assign on a first turn when the CLI allows it
	Edits     bool
	Extra     []string // final arguments placed before the prompt
	FirstTurn bool
}

// ChatLaunch is a fully built non-interactive command.
type ChatLaunch struct {
	Exe        string
	Args       []string
	Stdin      string
	ExternalID string
	Notes      []string
}

// BuildChat builds the argv for one chat turn (no shell).
func BuildChat(p Profile, r ChatRequest) (*ChatLaunch, error) {
	c := p.Chat
	if c == nil {
		return nil, fmt.Errorf("%s has no verified non-interactive chat mode; use the terminal panel instead", p.DisplayName)
	}
	if !filepath.IsAbs(r.Cwd) || r.Exe == "" {
		return nil, errors.New("working directory must be absolute and the executable resolved")
	}
	prompt := r.Prompt
	if c.PromptVia != "stdin" && strings.HasPrefix(prompt, "-") {
		prompt = "Message: " + prompt // a leading dash would be read as an option
	}
	sub := func(tpl []string, session string) []string {
		var out []string
		for _, a := range tpl {
			a = strings.ReplaceAll(a, "{prompt}", prompt)
			out = append(out, strings.ReplaceAll(a, "{session}", session))
		}
		return out
	}
	out := &ChatLaunch{Exe: r.Exe}
	var args []string
	switch {
	case r.FirstTurn || (r.SessionID == "" && len(c.ContinueArgs) == 0):
		args = sub(c.NewArgs, "")
		if c.AssignSessionID && r.NewID != "" {
			args = append(args, sub(c.SessionIDArgs, r.NewID)...)
			out.ExternalID = r.NewID
		}
	case r.SessionID != "" && len(c.ResumeArgs) > 0:
		args = sub(c.ResumeArgs, r.SessionID)
		out.ExternalID = r.SessionID
	case len(c.ContinueArgs) > 0:
		args = sub(c.ContinueArgs, "")
		out.Notes = append(out.Notes, p.DisplayName+" continues its most recent conversation in this folder; if you also use it elsewhere in this project, replies may mix.")
	default:
		return nil, fmt.Errorf("%s cannot continue this conversation", p.DisplayName)
	}
	if r.Edits {
		args = append(args, c.EditArgs...)
	}
	args = append(args, r.Extra...)
	out.Args = args
	if c.PromptVia == "stdin" {
		out.Stdin = r.Prompt
	}
	return out, nil
}
