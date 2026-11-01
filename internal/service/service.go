// Package service is the UI-independent application layer bound to the
// frontend. Every workspace-scoped method takes a workspace ID and looks the
// path up in storage; paths supplied by the UI are never trusted.
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"aicw/internal/agents"
	"aicw/internal/cli"
	"aicw/internal/contextbuilder"
	"aicw/internal/history"
	"aicw/internal/localllm"
	"aicw/internal/markdown"
	"aicw/internal/media"
	"aicw/internal/process"
	"aicw/internal/safety"
	"aicw/internal/scanner"
	"aicw/internal/skills"
	"aicw/internal/storage"
	"aicw/internal/term"
	"aicw/internal/voice"
	"aicw/internal/workspace"
)

// Settings are global preferences. No secrets are stored here.
type Settings struct {
	DefaultAgent  string            `json:"defaultAgent"`
	DefaultMode   string            `json:"defaultMode"`
	Budget        int               `json:"budget"`
	IgnoreDirs    []string          `json:"ignoreDirs"`
	IgnoreGlobs   []string          `json:"ignoreGlobs"`
	MaxFileKB     int               `json:"maxFileKB"`
	ContextDir    string            `json:"contextDir"` // "" = app data directory
	ConfirmWrites bool              `json:"confirmWrites"`
	Theme         string            `json:"theme"`
	Terminal      string            `json:"terminal"`
	AgentPaths    map[string]string `json:"agentPaths"`
	AgentModels   map[string]string `json:"agentModels"`  // agent id -> model name passed to its CLI ("" = the CLI's own default)
	AutoTerminal  bool              `json:"autoTerminal"` // open and start the agent terminal when a chat opens
	ChatEdits     bool              `json:"chatEdits"`    // let chat replies edit files (off by default)
	VoiceCommand  string            `json:"voiceCommand"`
	VoiceModel    string            `json:"voiceModel"`
	VoiceLanguage string            `json:"voiceLanguage"`
	Pinned        []string          `json:"pinned"` // workspace paths shown under Favourites
}

func DefaultSettings() Settings {
	return Settings{DefaultAgent: "claude", DefaultMode: "clean", Budget: 8000, IgnoreDirs: scanner.DefaultOptions().IgnoreDirs, IgnoreGlobs: []string{}, Pinned: []string{}, AutoTerminal: true, MaxFileKB: 512, ConfirmWrites: true, Theme: "dark", AgentPaths: map[string]string{}, AgentModels: map[string]string{}}
}

// WorkspaceState is per-workspace saved UI/context state.
type WorkspaceState struct {
	Task        string   `json:"task"`
	KnownIssues string   `json:"knownIssues"`
	NextSteps   string   `json:"nextSteps"`
	Excluded    []string `json:"excluded"`
	Mode        string   `json:"mode"`
}

type Service struct {
	ctx     context.Context
	Store   *storage.Store
	WS      *workspace.Manager
	Reg     *agents.Registry
	DataDir string
	Version string
	Initial *workspace.OpenResult
	// InitialError is shown by the UI if the startup request could not be opened.
	InitialError string
	mu           sync.Mutex

	term *term.Manager
	rec  *voice.Recorder
	tmu  sync.Mutex
	// terminal bookkeeping (guarded by tmu)
	convTerm    map[int64]string // conversation -> terminal id
	termConv    map[string]int64
	termSession map[string]int64
	// chat turns (guarded by tmu)
	chatCancel map[int64]context.CancelFunc
	chatQueue  map[int64][]string
	mirrorOff  map[int64]int64 // conversation -> bytes of its session transcript already mirrored
	mirrorBusy map[int64]bool
	LLM        *localllm.Manager
}

func New(dataDir, version string) (*Service, error) {
	st, err := storage.Open(filepath.Join(dataDir, "aicw.db"))
	if err != nil {
		return nil, err
	}
	s := &Service{Store: st, WS: &workspace.Manager{Store: st}, Reg: agents.NewRegistry(), DataDir: dataDir, Version: version,
		term: term.NewManager(), rec: &voice.Recorder{}, convTerm: map[int64]string{}, termConv: map[string]int64{}, termSession: map[string]int64{}, chatCancel: map[int64]context.CancelFunc{}, chatQueue: map[int64][]string{}, mirrorOff: map[int64]int64{}, mirrorBusy: map[int64]bool{}, LLM: localllm.New(dataDir)}
	s.term.OnData = func(id string, d []byte) {
		s.emit("term:data", map[string]string{"id": id, "data": base64.StdEncoding.EncodeToString(d)})
	}
	s.term.OnExit = s.onTermExit
	s.applySettings()
	if cas, err := st.CustomAgents(); err == nil {
		for _, c := range cas {
			var p agents.Profile
			if json.Unmarshal([]byte(c.JSON), &p) == nil {
				s.Reg.Add(p)
			}
		}
	}
	return s, nil
}

// Lifecycle holds methods the host needs but the frontend must never call.
// It is deliberately not bound to the UI.
type Lifecycle struct{ s *Service }

func (s *Service) Lifecycle() *Lifecycle { return &Lifecycle{s} }

func (l *Lifecycle) Startup(ctx context.Context)                  { l.s.ctx = ctx }
func (l *Lifecycle) Close()                                       { l.s.term.CloseAll(); l.s.LLM.Stop(); l.s.Store.Close() }
func (l *Lifecycle) HandleExternalOpen(args []string, cwd string) { l.s.handleExternalOpen(args, cwd) }

func (s *Service) applySettings() {
	st := s.GetSettings()
	s.Reg.Paths = map[string]string{}
	for k, v := range st.AgentPaths {
		if v != "" {
			s.Reg.Paths[k] = v
		}
	}
}

// ---- bootstrap / settings ----

type BootInfo struct {
	Version      string                `json:"version"`
	DataDir      string                `json:"dataDir"`
	Initial      *workspace.OpenResult `json:"initial"`
	InitialError string                `json:"initialError"`
	Settings     Settings              `json:"settings"`
}

func (s *Service) Bootstrap() BootInfo {
	return BootInfo{Version: s.Version, DataDir: s.DataDir, Initial: s.Initial, InitialError: s.InitialError, Settings: s.GetSettings()}
}

func (s *Service) GetSettings() Settings {
	d := DefaultSettings()
	if raw := s.Store.Setting("settings", ""); raw != "" {
		json.Unmarshal([]byte(raw), &d)
	}
	if d.AgentPaths == nil {
		d.AgentPaths = map[string]string{}
	}
	if d.AgentModels == nil {
		d.AgentModels = map[string]string{}
	}
	if d.Pinned == nil {
		d.Pinned = []string{}
	}
	return d
}

func (s *Service) SaveSettings(st Settings) error {
	if st.Budget < 500 || st.Budget > 200000 {
		return errors.New("context budget must be between 500 and 200000 tokens")
	}
	if st.MaxFileKB < 1 || st.MaxFileKB > 10240 {
		return errors.New("file size limit must be between 1 and 10240 KB")
	}
	if st.ContextDir != "" && !filepath.IsAbs(st.ContextDir) {
		return errors.New("context directory must be an absolute path")
	}
	for k, v := range st.AgentPaths {
		if v == "" {
			delete(st.AgentPaths, k)
			continue
		}
		if _, ok := s.Reg.Get(k); !ok {
			return fmt.Errorf("unknown agent %q", k)
		}
	}
	for k, v := range st.AgentModels {
		v = strings.TrimSpace(v)
		if v == "" {
			delete(st.AgentModels, k)
			continue
		}
		p, ok := s.Reg.Get(k)
		if !ok {
			return fmt.Errorf("unknown agent %q", k)
		}
		if len(p.ModelArgs) == 0 {
			return fmt.Errorf("%s has no known model option", p.DisplayName)
		}
		if err := agents.ValidModel(v); err != nil {
			return err
		}
		st.AgentModels[k] = v
	}
	b, _ := json.Marshal(st)
	if err := s.Store.SetSetting("settings", string(b)); err != nil {
		return err
	}
	s.applySettings()
	return nil
}

// ---- workspaces ----

// OpenWorkspace opens or resumes the workspace at an already-resolved path or a
// raw path from the UI (resolved here, relative to the home directory).
func (s *Service) OpenWorkspace(path string, wantNew bool) (*workspace.OpenResult, error) {
	home, _ := os.UserHomeDir()
	p, err := safety.ResolveDir(path, home)
	if err != nil {
		return nil, err
	}
	return s.WS.Open(p, wantNew)
}

// handleExternalOpen processes a request forwarded from a second `aicw`
// invocation. It focuses the window and emits an event to the UI.
func (s *Service) handleExternalOpen(args []string, cwd string) {
	cmd, err := cli.Parse(args)
	if s.ctx != nil {
		runtime.WindowUnminimise(s.ctx)
		runtime.WindowShow(s.ctx)
	}
	if err != nil {
		s.emit("app:error", err.Error())
		return
	}
	if cmd.Kind != cli.KindOpen || cmd.Path == "" {
		return
	}
	p, err := cli.ResolveRequest(cmd.Path, cwd, nil)
	if err != nil {
		s.emit("app:error", err.Error())
		return
	}
	res, err := s.WS.Open(p, cmd.New)
	if err != nil {
		s.emit("app:error", err.Error())
		return
	}
	res.Page = cmd.Page
	s.emit("workspace:open", res)
}

func (s *Service) emit(name string, data any) {
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, name, data)
	}
}

func (s *Service) RecentWorkspaces() ([]storage.Workspace, error) {
	return s.Store.RecentWorkspaces(20)
}

func (s *Service) ForgetWorkspace(id int64, confirmed bool) error {
	if !confirmed {
		return markdown.ErrNotConfirmed
	}
	return s.Store.ForgetWorkspace(id) // removes AICW's record only; project files are untouched
}

func (s *Service) PickDirectory() (string, error) {
	if s.ctx == nil {
		return "", errors.New("no window")
	}
	return runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{Title: "Select a project directory"})
}

func (s *Service) wsPath(id int64) (*storage.Workspace, error) {
	w, err := s.Store.Workspace(id)
	if err != nil {
		return nil, fmt.Errorf("unknown workspace: %w", err)
	}
	if st, err := os.Stat(w.Path); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("workspace directory is missing: %s", w.Path)
	}
	return w, nil
}

func (s *Service) GetWorkspaceState(id int64) (WorkspaceState, error) {
	w, err := s.Store.Workspace(id)
	if err != nil {
		return WorkspaceState{}, err
	}
	st := WorkspaceState{Excluded: []string{}, Mode: "clean"}
	json.Unmarshal([]byte(w.Settings), &st)
	if st.Excluded == nil {
		st.Excluded = []string{}
	}
	return st, nil
}

func (s *Service) SaveWorkspaceState(id int64, st WorkspaceState, agent string) error {
	b, _ := json.Marshal(st)
	return s.Store.SaveWorkspaceState(id, agent, string(b))
}

func (s *Service) scanOpts() scanner.Options {
	st := s.GetSettings()
	o := scanner.DefaultOptions()
	o.IgnoreDirs, o.IgnoreGlobs, o.MaxFileBytes = st.IgnoreDirs, st.IgnoreGlobs, int64(st.MaxFileKB)*1024
	return o
}

func (s *Service) Scan(id int64) (*scanner.Result, error) {
	w, err := s.wsPath(id)
	if err != nil {
		return nil, err
	}
	return scanner.Scan(w.Path, s.scanOpts())
}

// ReadFile returns text of a scanned file for preview. Secret files are refused.
func (s *Service) ReadFile(id int64, rel string) (string, error) {
	w, err := s.wsPath(id)
	if err != nil {
		return "", err
	}
	full, err := safety.SafeJoin(w.Path, rel)
	if err != nil {
		return "", err
	}
	if safety.IsSecretFile(rel) {
		return "", errors.New("refusing to display a likely secret file")
	}
	fi, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if max := int64(s.GetSettings().MaxFileKB) * 1024; fi.Size() > max {
		return "", fmt.Errorf("file is larger than the %d KB limit", max/1024)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if safety.LooksSecret(string(b)) {
		return "", errors.New("file appears to contain credentials; not displayed")
	}
	return string(b), nil
}

// ---- context ----

type ContextRequest struct {
	WorkspaceID int64  `json:"workspaceId"`
	Budget      int    `json:"budget"` // 0 = setting
	Task        string `json:"task"`
	KnownIssues string `json:"knownIssues"`
	NextSteps   string `json:"nextSteps"`
}

func (s *Service) BuildContext(r ContextRequest) (*contextbuilder.ContextPackage, error) {
	w, err := s.wsPath(r.WorkspaceID)
	if err != nil {
		return nil, err
	}
	scan, err := scanner.Scan(w.Path, s.scanOpts())
	if err != nil {
		return nil, err
	}
	state, _ := s.GetWorkspaceState(r.WorkspaceID)
	budget := r.Budget
	if budget == 0 {
		budget = s.GetSettings().Budget
	}
	handoff := ""
	if ses, _ := s.Store.Sessions(w.ID); len(ses) > 0 {
		for _, x := range ses {
			if x.Handoff != "" {
				handoff = x.Handoff
				break
			}
		}
	}
	var refs []contextbuilder.MediaRef
	if items, err := s.Store.ListMedia(w.ID); err == nil {
		for _, m := range items {
			if m.InContext {
				refs = append(refs, contextbuilder.MediaRef{Name: m.Name, Path: m.Stored, Kind: m.Kind, Note: m.Note})
			}
		}
	}
	var notes []contextbuilder.NoteRef
	if ns, err := s.Store.Notes(w.ID); err == nil {
		for _, n := range ns {
			notes = append(notes, contextbuilder.NoteRef{Name: n.Name, Body: n.Body, By: n.UpdatedBy})
		}
	}
	return contextbuilder.Build(w.Path, scan, contextbuilder.Options{Media: refs, Notes: notes, Budget: budget, Task: r.Task, KnownIssues: r.KnownIssues, NextSteps: r.NextSteps, Handoff: handoff, Exclude: state.Excluded})
}

func (s *Service) CopyToClipboard(text string) error {
	if s.ctx == nil {
		return errors.New("no window")
	}
	return runtime.ClipboardSetText(s.ctx, text)
}

// ExportContext asks for a destination and writes the package as one .md file.
func (s *Service) ExportContext(r ContextRequest) (string, error) {
	pkg, err := s.BuildContext(r)
	if err != nil {
		return "", err
	}
	if s.ctx == nil {
		return "", errors.New("no window")
	}
	dest, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{Title: "Export context package", DefaultFilename: "context-package.md"})
	if err != nil || dest == "" {
		return "", err
	}
	return dest, os.WriteFile(dest, []byte(pkg.Markdown), 0o600)
}

// ---- guarded file writes ----

func (s *Service) writer(id int64) (*markdown.Writer, error) {
	w, err := s.wsPath(id)
	if err != nil {
		return nil, err
	}
	return &markdown.Writer{Root: w.Path, HistoryDir: filepath.Join(s.DataDir, "history", fmt.Sprint(id))}, nil
}

func (s *Service) PreviewWrite(id int64, rel, content string) (*markdown.Preview, error) {
	w, err := s.writer(id)
	if err != nil {
		return nil, err
	}
	return w.Preview(rel, content)
}

func (s *Service) ApplyWrite(id int64, rel, content, baseHash string, confirmed bool) (*markdown.AuditEntry, error) {
	w, err := s.writer(id)
	if err != nil {
		return nil, err
	}
	return w.Apply(rel, content, confirmed, baseHash)
}

func (s *Service) Backups(id int64, rel string) ([]string, error) {
	w, err := s.writer(id)
	if err != nil {
		return nil, err
	}
	return w.Backups(rel), nil
}

func (s *Service) RestoreBackup(id int64, rel, backup string, confirmed bool) (*markdown.AuditEntry, error) {
	w, err := s.writer(id)
	if err != nil {
		return nil, err
	}
	return w.Restore(rel, backup, confirmed)
}

func (s *Service) AuditLog(id int64) ([]markdown.AuditEntry, error) {
	w, err := s.writer(id)
	if err != nil {
		return nil, err
	}
	return w.AuditLog(), nil
}

// InstructionDraft proposes AGENTS.md content (not written).
func (s *Service) InstructionDraft(id int64) (string, error) {
	w, err := s.wsPath(id)
	if err != nil {
		return "", err
	}
	scan, err := scanner.Scan(w.Path, s.scanOpts())
	if err != nil {
		return "", err
	}
	return contextbuilder.InstructionDraft(w.Path, scan, time.Now()), nil
}

type MovePair struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ArchivePlan lists where each selected file would go. Nothing is moved.
func (s *Service) ArchivePlan(id int64, rels []string) ([]MovePair, error) {
	if _, err := s.wsPath(id); err != nil {
		return nil, err
	}
	day := time.Now().Format("2006-01-02")
	var out []MovePair
	for _, r := range rels {
		out = append(out, MovePair{From: r, To: ".aicw/archive/" + day + "/" + filepath.ToSlash(r)})
	}
	return out, nil
}

// ApplyMoves performs confirmed moves (archive, or undo by swapping From/To).
func (s *Service) ApplyMoves(id int64, moves []MovePair, confirmed bool) ([]markdown.AuditEntry, error) {
	w, err := s.writer(id)
	if err != nil {
		return nil, err
	}
	var done []markdown.AuditEntry
	for _, m := range moves {
		e, err := w.Move(m.From, m.To, confirmed)
		if err != nil {
			return done, fmt.Errorf("%s: %w", m.From, err)
		}
		done = append(done, *e)
	}
	return done, nil
}

// ---- agents ----

type AgentInfo struct {
	Profile agents.Profile `json:"profile"`
	Status  agents.Status  `json:"status"`
}

func (s *Service) ListAgents() []AgentInfo {
	var out []AgentInfo
	for _, p := range s.Reg.List() {
		out = append(out, AgentInfo{Profile: p, Status: s.Reg.Detect(context.Background(), p.ID)})
	}
	return out
}

func (s *Service) ListTerminals() []string {
	var out []string
	for _, t := range process.ListTerminals() {
		out = append(out, t.Name)
	}
	return out
}

func (s *Service) AddCustomAgent(jsonText string) error {
	var p agents.Profile
	dec := json.NewDecoder(strings.NewReader(jsonText))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return fmt.Errorf("invalid agent definition: %w", err)
	}
	if err := s.Reg.Add(p); err != nil {
		return err
	}
	cp, _ := s.Reg.Get(p.ID)
	b, _ := json.Marshal(cp)
	return s.Store.PutCustomAgent(p.ID, string(b))
}

func (s *Service) RemoveCustomAgent(id string) error {
	if p, ok := s.Reg.Get(id); !ok || p.Builtin {
		return errors.New("only custom agents can be removed")
	}
	s.Reg.Remove(id)
	return s.Store.DeleteCustomAgent(id)
}

// ---- sessions ----

func (s *Service) Sessions(wsID int64) ([]storage.Session, error) { return s.Store.Sessions(wsID) }

func (s *Service) SetSessionStatus(sessionID int64, status string) error {
	return s.Store.UpdateSession(sessionID, storage.SessionPatch{Status: &status})
}

func (s *Service) DeleteSession(sessionID int64, confirmed bool) error {
	if !confirmed {
		return markdown.ErrNotConfirmed
	}
	return s.Store.DeleteSession(sessionID)
}

type LaunchOptions struct {
	WorkspaceID int64  `json:"workspaceId"`
	Agent       string `json:"agent"`
	Mode        string `json:"mode"` // new | resume
	SessionID   int64  `json:"sessionId"`
	Title       string `json:"title"`
	Task        string `json:"task"`
	KnownIssues string `json:"knownIssues"`
	NextSteps   string `json:"nextSteps"`
	DryRun      bool   `json:"dryRun"`
	Force       bool   `json:"force"` // launch even if the session looks already running
}

type LaunchResult struct {
	Launched    bool               `json:"launched"`
	Spec        *agents.LaunchSpec `json:"spec"`
	SessionID   int64              `json:"sessionId"`
	ContextFile string             `json:"contextFile"`
	Tokens      int                `json:"tokens"`
	Disclosure  string             `json:"disclosure"`
	Error       string             `json:"error"`
	Fallback    string             `json:"fallback"` // guidance when launching is impossible
}

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6], b[8] = (b[6]&0x0f)|0x40, (b[8]&0x3f)|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (s *Service) contextDir(wsID int64) string {
	base := s.GetSettings().ContextDir
	if base == "" {
		base = filepath.Join(s.DataDir, "contexts")
	}
	return filepath.Join(base, fmt.Sprint(wsID))
}

// Launch implements steps 5-6 of Fresh Start and resume. It starts a real CLI
// or returns an explanatory error; it never simulates an AI response. With
// DryRun it only reports what would run.
func (s *Service) Launch(o LaunchOptions) (*LaunchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.wsPath(o.WorkspaceID)
	if err != nil {
		return nil, err
	}
	res := &LaunchResult{}
	var sess *storage.Session
	mode := agents.ModeNew
	if o.Mode == "resume" {
		mode = agents.ModeResume
		if sess, err = s.Store.Session(o.SessionID); err != nil || sess.WorkspaceID != w.ID {
			return nil, errors.New("session does not belong to this workspace")
		}
		o.Agent = sess.Agent
		res.SessionID = sess.ID
		if sess.Status == storage.StatusActive && process.Alive(sess.PID) && !o.Force {
			res.Error = fmt.Sprintf("This session appears to be running already (launcher PID %d). Switch to its terminal, or launch again with force.", sess.PID)
			return res, nil
		}
	}
	if o.Agent == "" {
		o.Agent = s.GetSettings().DefaultAgent
	}
	prof, ok := s.Reg.Get(o.Agent)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", o.Agent)
	}
	st := s.Reg.Detect(context.Background(), o.Agent)
	res.Disclosure = fmt.Sprintf("Launching %s will let it read and edit files and run commands in %s (per its own permission settings). The context package and anything the agent reads is sent to that tool's provider.", prof.DisplayName, w.Path)

	model := s.GetSettings().AgentModels[o.Agent]
	if strings.HasPrefix(model, localPrefix) {
		res.Error = "A local model runs in the in-app Terminal panel or in chat replies, not in an external terminal. Use the Terminal panel, or pick a hosted model."
		return res, nil
	}
	req := agents.LaunchRequest{Mode: mode, Cwd: w.Path, Exe: st.Path, Model: model}
	if mode == agents.ModeResume {
		req.SessionID = sess.ExternalID
	} else {
		pkg, err := s.BuildContext(ContextRequest{WorkspaceID: w.ID, Task: o.Task, KnownIssues: o.KnownIssues, NextSteps: o.NextSteps})
		if err != nil {
			return nil, err
		}
		res.Tokens = pkg.Tokens
		req.ContextFile = filepath.Join(s.contextDir(w.ID), time.Now().Format("20060102-150405")+"-context.md")
		req.SessionID = newUUID()
		req.Prompt = strings.ReplaceAll(agents.ContextPrompt, "{contextFile}", req.ContextFile)
		if !o.DryRun && st.State == agents.StatusDetected {
			if err := os.MkdirAll(filepath.Dir(req.ContextFile), 0o700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(req.ContextFile, []byte(pkg.Markdown), 0o600); err != nil {
				return nil, err
			}
			res.ContextFile = req.ContextFile
		}
	}
	if st.State != agents.StatusDetected {
		res.Error = fmt.Sprintf("%s is %s: %s", prof.DisplayName, strings.ToLower(st.State), st.Detail)
		res.Fallback = "Install the CLI or set its executable path in AI Agents, or export/copy the context package from the Context page and paste it into your tool."
		return res, nil
	}
	spec, err := agents.BuildLaunch(prof, req)
	if err != nil {
		res.Error = err.Error()
		res.Fallback = "Start a new session instead; AICW will include the saved handoff summary in the context package."
		return res, nil
	}
	res.Spec = spec
	if o.DryRun {
		return res, nil
	}
	term, err := process.FindTerminal(s.GetSettings().Terminal)
	if err != nil {
		res.Error = err.Error()
		res.Fallback = "Run this in a terminal inside the project: " + spec.Display
		return res, nil
	}
	pid, err := process.Launch(term, *spec)
	if err != nil {
		res.Error = err.Error()
		res.Fallback = "Run this in a terminal inside the project: " + spec.Display
		return res, nil
	}
	active := storage.StatusActive
	if sess == nil {
		if sess, err = s.Store.CreateSession(storage.Session{WorkspaceID: w.ID, Agent: o.Agent, ExternalID: spec.ExternalID, Status: active, Title: o.Title, PID: pid, ContextVersion: fmt.Sprint(contextbuilder.FormatVersion)}); err != nil {
			return nil, err
		}
	} else {
		err = s.Store.UpdateSession(sess.ID, storage.SessionPatch{Status: &active, PID: &pid})
	}
	res.SessionID, res.Launched = sess.ID, true
	s.SaveWorkspaceState(w.ID, mustState(s, w.ID, o), o.Agent)
	return res, err
}

func mustState(s *Service, id int64, o LaunchOptions) WorkspaceState {
	st, _ := s.GetWorkspaceState(id)
	st.Task, st.KnownIssues, st.NextSteps = o.Task, o.KnownIssues, o.NextSteps
	return st
}

// ---- skills ----

func (s *Service) Skills(id int64) (*skills.Inventory, error) {
	w, err := s.wsPath(id)
	if err != nil {
		return nil, err
	}
	home, _ := os.UserHomeDir()
	return skills.Discover(w.Path, home)
}

type SkillsOverview struct {
	Inventory       *skills.Inventory       `json:"inventory"`
	Templates       []skills.Template       `json:"templates"`
	Compat          []skills.Compat         `json:"compat"`
	Recommendations []skills.Recommendation `json:"recommendations"`
}

func (s *Service) SkillsOverview(id int64, agent string) (*SkillsOverview, error) {
	inv, err := s.Skills(id)
	if err != nil {
		return nil, err
	}
	return &SkillsOverview{Inventory: inv, Templates: skills.Templates(), Compat: skills.CompatTable(), Recommendations: skills.Recommend(inv, agent)}, nil
}

type SkillPreview struct {
	RelPath string            `json:"relPath"`
	Content string            `json:"content"`
	Preview *markdown.Preview `json:"preview"`
}

func (s *Service) PreviewSkillInstall(id int64, templateID, agent string) ([]SkillPreview, error) {
	t, ok := skills.FindTemplate(templateID)
	if !ok {
		return nil, errors.New("unknown skill template")
	}
	w, err := s.writer(id)
	if err != nil {
		return nil, err
	}
	var out []SkillPreview
	for _, f := range skills.Render(t, agent) {
		pv, err := w.Preview(f.RelPath, f.Content)
		if err != nil {
			return nil, err
		}
		out = append(out, SkillPreview{RelPath: f.RelPath, Content: f.Content, Preview: pv})
	}
	return out, nil
}

func (s *Service) ApplySkillInstall(id int64, templateID, agent string, confirmed bool) ([]markdown.AuditEntry, error) {
	t, ok := skills.FindTemplate(templateID)
	if !ok {
		return nil, errors.New("unknown skill template")
	}
	w, err := s.writer(id)
	if err != nil {
		return nil, err
	}
	var out []markdown.AuditEntry
	for _, f := range skills.Render(t, agent) {
		e, err := w.Apply(f.RelPath, f.Content, confirmed, "")
		if err != nil {
			return out, err
		}
		out = append(out, *e)
	}
	return out, nil
}

func (s *Service) PreviewSkillImport(src string) (*skills.ImportPreview, error) {
	home, _ := os.UserHomeDir()
	p, err := safety.ResolveDir(src, home)
	if err != nil {
		return nil, err
	}
	return skills.PreviewImport(p)
}

func (s *Service) PickSkillDirectory() (string, error) {
	if s.ctx == nil {
		return "", errors.New("no window")
	}
	return runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{Title: "Select a skill directory (containing SKILL.md)"})
}

// ApplySkillImport copies a previewed skill into the project's skill folder
// for the chosen agent. Each file goes through the backup-and-audit writer.
func (s *Service) ApplySkillImport(id int64, src, agent string, confirmed bool) ([]markdown.AuditEntry, error) {
	pv, err := s.PreviewSkillImport(src)
	if err != nil {
		return nil, err
	}
	w, err := s.writer(id)
	if err != nil {
		return nil, err
	}
	dir := ".agents/skills"
	if agent == "claude" {
		dir = ".claude/skills"
	}
	name := filepath.Base(pv.Source)
	var out []markdown.AuditEntry
	for _, f := range pv.Files {
		text, err := skills.ReadImportFile(pv.Source, f.Rel)
		if err != nil {
			return out, err
		}
		e, err := w.Apply(dir+"/"+name+"/"+f.Rel, text, confirmed, "")
		if err != nil {
			return out, err
		}
		out = append(out, *e)
	}
	return out, nil
}

// ---- handoff ----

func (s *Service) SuggestChangedFiles(id int64) []string {
	if w, err := s.wsPath(id); err == nil {
		return history.GitChangedFiles(w.Path)
	}
	return nil
}

func (s *Service) GenerateHandoff(in history.HandoffInput) string {
	in.Now = time.Now()
	return history.Handoff(in)
}

// SaveHandoff stores the summary on the session and, if relPath is non-empty
// and confirmed, writes the note into the project via the guarded writer.
func (s *Service) SaveHandoff(wsID, sessionID int64, content, relPath string, confirmed bool) error {
	if sessionID != 0 {
		if err := s.Store.UpdateSession(sessionID, storage.SessionPatch{Handoff: &content}); err != nil {
			return err
		}
	}
	if relPath == "" {
		return nil
	}
	w, err := s.writer(wsID)
	if err != nil {
		return err
	}
	_, err = w.Apply(relPath, content, confirmed, "")
	return err
}

func (s *Service) HandoffFileName(title string) string {
	return ".aicw/sessions/" + history.FileName(time.Now(), title)
}

func (s *Service) Doctor() string {
	var sb strings.Builder
	cli.Doctor(&sb, s.Version, s.DataDir)
	return sb.String()
}

// WorkspaceWarnings returns non-blocking cautions for step 1 of Fresh Start.
func (s *Service) WorkspaceWarnings(id int64) ([]string, error) {
	w, err := s.wsPath(id)
	if err != nil {
		return nil, err
	}
	out := []string{}
	if home, err := os.UserHomeDir(); err == nil && !safety.Contains(home, w.Path) {
		out = append(out, "This directory is outside your home directory.")
	}
	if st, err := os.Stat(w.Path); err == nil {
		if st.Mode().Perm()&0o002 != 0 {
			out = append(out, "This directory is world-writable.")
		}
		if f, err := os.CreateTemp(w.Path, ".aicw-probe-*"); err != nil {
			out = append(out, "This directory is not writable by you; AICW can inspect it but not write files into it.")
		} else {
			f.Close()
			os.Remove(f.Name())
		}
	}
	return out, nil
}

// ---- media (images, video, audio, PDFs, other files) ----

func (s *Service) mediaDir(wsID int64) string {
	return filepath.Join(s.DataDir, "media", fmt.Sprint(wsID))
}

func (s *Service) ListMedia(wsID int64) ([]storage.Media, error) { return s.Store.ListMedia(wsID) }

// PickMedia opens a multi-file dialog and returns the chosen paths.
func (s *Service) PickMedia() ([]string, error) {
	if s.ctx == nil {
		return nil, errors.New("no window")
	}
	return runtime.OpenMultipleFilesDialog(s.ctx, runtime.OpenDialogOptions{Title: "Add images, videos, audio, PDFs or other files"})
}

type ImportResult struct {
	Added  []storage.Media `json:"added"`
	Errors []string        `json:"errors"`
}

// ImportMedia copies the given files into AICW's private store. Originals are
// untouched. Per-file failures are reported without aborting the batch.
func (s *Service) ImportMedia(wsID int64, paths []string) (*ImportResult, error) {
	if _, err := s.Store.Workspace(wsID); err != nil {
		return nil, errors.New("unknown workspace")
	}
	res := &ImportResult{Added: []storage.Media{}, Errors: []string{}}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			res.Errors = append(res.Errors, p+": path must be absolute")
			continue
		}
		im, err := media.Import(p, s.mediaDir(wsID), strings.ReplaceAll(newUUID(), "-", "")[:12])
		if err != nil {
			res.Errors = append(res.Errors, filepath.Base(p)+": "+err.Error())
			continue
		}
		m, err := s.Store.AddMedia(storage.Media{WorkspaceID: wsID, Name: im.Name, Stored: im.Stored, Kind: im.Kind, Mime: im.Mime, Size: im.Size})
		if err != nil {
			os.Remove(im.Stored)
			res.Errors = append(res.Errors, filepath.Base(p)+": "+err.Error())
			continue
		}
		res.Added = append(res.Added, *m)
	}
	return res, nil
}

func (s *Service) UpdateMedia(id int64, note string, inContext bool) error {
	if len(note) > 2000 {
		return errors.New("note is too long (2000 characters max)")
	}
	return s.Store.UpdateMedia(id, note, inContext)
}

func (s *Service) MediaPath(id int64) (string, error) {
	m, err := s.Store.Media(id)
	if err != nil {
		return "", err
	}
	return m.Stored, nil
}

// DeleteMedia removes AICW's stored copy and record. The original file the
// user imported from is never touched.
func (s *Service) DeleteMedia(id int64, confirmed bool) error {
	if !confirmed {
		return markdown.ErrNotConfirmed
	}
	m, err := s.Store.Media(id)
	if err != nil {
		return err
	}
	if !safety.Contains(filepath.Join(s.DataDir, "media"), m.Stored) {
		return errors.New("refusing to delete a file outside the media store")
	}
	if err := os.Remove(m.Stored); err != nil && !os.IsNotExist(err) {
		return err
	}
	return s.Store.DeleteMedia(id)
}

// MediaHandler serves stored files at /media/<id> to the webview, with Range
// support so video and audio can seek. Only IDs present in the database are
// served, and files are sandboxed with a restrictive CSP.
func (s *Service) MediaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/media/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if !strings.HasPrefix(r.URL.Path, "/media/") || err != nil {
			http.NotFound(w, r)
			return
		}
		m, err := s.Store.Media(id)
		if err != nil || !safety.Contains(filepath.Join(s.DataDir, "media"), m.Stored) {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(m.Stored)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, _ := f.Stat()
		w.Header().Set("Content-Type", m.Mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=86400") // stored files never change under the same id; lets video re-seek without re-reading
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; media-src 'self'; style-src 'unsafe-inline'; sandbox")
		disp := "inline"
		if m.Kind == "file" {
			disp = "attachment"
		}
		w.Header().Set("Content-Disposition", disp+`; filename="`+media.SafeName(m.Name)+`"`)
		http.ServeContent(w, r, m.Name, st.ModTime(), f)
	})
}

// WorkspaceSummary feeds the sidebar workspace browser.
type WorkspaceSummary struct {
	Workspace storage.Workspace `json:"workspace"`
	Sessions  int               `json:"sessions"`
	Resumable int               `json:"resumable"`
	Pinned    bool              `json:"pinned"`
}

func (s *Service) WorkspaceList() ([]WorkspaceSummary, error) {
	ws, err := s.Store.RecentWorkspaces(50)
	if err != nil {
		return nil, err
	}
	counts, err := s.Store.SessionCounts()
	if err != nil {
		return nil, err
	}
	pinned := map[string]bool{}
	for _, p := range s.GetSettings().Pinned {
		pinned[p] = true
	}
	out := make([]WorkspaceSummary, 0, len(ws))
	for _, w := range ws {
		c := counts[w.ID]
		out = append(out, WorkspaceSummary{Workspace: w, Sessions: c.Total, Resumable: c.Resumable, Pinned: pinned[w.Path]})
	}
	return out, nil
}

// TogglePin adds or removes a known workspace from Favourites.
func (s *Service) TogglePin(id int64) (bool, error) {
	w, err := s.Store.Workspace(id)
	if err != nil {
		return false, err
	}
	st := s.GetSettings()
	var next []string
	found := false
	for _, p := range st.Pinned {
		if p == w.Path {
			found = true
			continue
		}
		next = append(next, p)
	}
	if !found {
		next = append(next, w.Path)
	}
	st.Pinned = next
	b, _ := json.Marshal(st)
	return !found, s.Store.SetSetting("settings", string(b))
}
