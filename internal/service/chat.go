package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aicw/internal/agents"
	"aicw/internal/commands"
	"aicw/internal/contextbuilder"
	"aicw/internal/markdown"
	"aicw/internal/media"
	"aicw/internal/rules"
	"aicw/internal/storage"
	"aicw/internal/term"
	"aicw/internal/transcript"
	"aicw/internal/voice"
)

// ---- conversations and messages ----

func (s *Service) Conversations(wsID int64) ([]storage.Conversation, error) {
	return s.Store.Conversations(wsID)
}

func (s *Service) NewConversation(wsID int64, title, agent string) (*storage.Conversation, error) {
	if _, err := s.wsPath(wsID); err != nil {
		return nil, err
	}
	if agent == "" {
		agent = s.GetSettings().DefaultAgent
	}
	return s.Store.CreateConversation(wsID, title, agent)
}

func (s *Service) RenameConversation(id int64, title string) error {
	if strings.TrimSpace(title) == "" || len(title) > 200 {
		return errors.New("title must be 1-200 characters")
	}
	return s.Store.UpdateConversation(id, &title, nil, nil)
}

func (s *Service) SetConversationAgent(id int64, agent string) error {
	if _, ok := s.Reg.Get(agent); !ok && agent != "shell" {
		return fmt.Errorf("unknown agent %q", agent)
	}
	return s.Store.UpdateConversation(id, nil, &agent, nil)
}

func (s *Service) DeleteConversation(id int64, confirmed bool) error {
	if !confirmed {
		return markdown.ErrNotConfirmed
	}
	if tid := s.termFor(id); tid != "" {
		s.term.Close(tid)
	}
	return s.Store.DeleteConversation(id)
}

func (s *Service) Messages(convID, afterID int64) ([]storage.Message, error) {
	s.mirrorTerminal(convID)
	return s.Store.Messages(convID, afterID, 0)
}

// mirrorTerminal copies what was typed and answered in the conversation's
// terminal (read from the agent's own session transcript) into the chat, so
// the chat view and the terminal show one conversation and chat replies can
// continue it. It is idempotent and silent on any problem.
func (s *Service) mirrorTerminal(convID int64) {
	conv, err := s.Store.Conversation(convID)
	if err != nil || conv.SessionID == 0 || (conv.Agent != "claude" && conv.Agent != "") {
		return
	}
	sess, err := s.Store.Session(conv.SessionID)
	if err != nil || sess.Agent != "claude" || sess.ExternalID == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := transcript.FindClaude(home, sess.ExternalID)
	if path == "" {
		return
	}
	s.tmu.Lock()
	if s.mirrorBusy[convID] {
		s.tmu.Unlock()
		return
	}
	s.mirrorBusy[convID] = true
	off := s.mirrorOff[convID]
	s.tmu.Unlock()
	defer func() { s.tmu.Lock(); delete(s.mirrorBusy, convID); s.tmu.Unlock() }()

	entries, next, err := transcript.ReadClaude(path, off)
	if err != nil {
		return
	}
	sent, sentAt, _ := s.Store.SentTexts(convID, 50)
	ctxPrompt, _, _ := strings.Cut(agents.ContextPrompt, "{contextFile}")
	for _, e := range entries {
		role, source := storage.RoleAssistant, storage.SourceTerminal
		if e.Role == "user" {
			role = storage.RoleUser
			if strings.HasPrefix(e.Text, ctxPrompt) || strings.HasPrefix(e.Text, commands.Marker) || sentFromChat(e, sent, sentAt) {
				continue // AICW's own opening prompt, or a message already shown from the composer
			}
		}
		s.Store.AddMirrored(convID, role, source, e.Text, e.ID, e.At)
	}
	s.tmu.Lock()
	s.mirrorOff[convID] = next
	s.tmu.Unlock()
}

// sentFromChat reports whether a terminal user entry is the chat composer's
// own message (typed into the terminal as a paste) rather than keyboard input.
func sentFromChat(e transcript.Entry, bodies []string, times []time.Time) bool {
	for i, b := range bodies {
		b = strings.TrimSpace(b)
		if b == "" || (e.Text != b && !strings.HasPrefix(e.Text, b+"\n[attached ")) {
			continue
		}
		if d := e.At.Sub(times[i]); e.At.IsZero() || (d > -2*time.Minute && d < 2*time.Minute) {
			return true
		}
	}
	return false
}

type SendOptions struct {
	ConversationID int64   `json:"conversationId"`
	Text           string  `json:"text"`
	MediaIDs       []int64 `json:"mediaIds"`
	// Mode chooses where the message goes: "terminal" (type it into the running
	// agent terminal), "chat" (run the agent non-interactively and post its
	// reply), "auto" (terminal if one is running, else chat), or "save" (just
	// store it).
	Mode string `json:"mode"`
}

type SendResult struct {
	Message *storage.Message `json:"message"`
	Sent    bool             `json:"sent"`    // delivered to a running agent terminal
	Running bool             `json:"running"` // an agent reply is being produced
	Note    string           `json:"note"`
}

// SendMessage records the user's message (with attachments) and, if an agent
// is running in this conversation's terminal, types it there as a paste.
func (s *Service) SendMessage(o SendOptions) (*SendResult, error) {
	if len(o.Text) > 64*1024 {
		return nil, errors.New("message is too long (64 KB max)")
	}
	if strings.TrimSpace(o.Text) == "" && len(o.MediaIDs) == 0 {
		return nil, errors.New("write a message or attach a file")
	}
	conv, err := s.Store.Conversation(o.ConversationID)
	if err != nil {
		return nil, errors.New("unknown conversation")
	}
	msg, err := s.Store.AddMessage(conv.ID, storage.RoleUser, storage.SourceUI, o.Text, o.MediaIDs)
	if err != nil {
		return nil, err
	}
	if conv.Title == "New chat" && strings.TrimSpace(o.Text) != "" {
		t := strings.Join(strings.Fields(o.Text), " ")
		if len(t) > 48 {
			t = t[:48] + "…"
		}
		s.Store.UpdateConversation(conv.ID, &t, nil, nil)
	}
	res := &SendResult{Message: msg}
	// A fixed command (/grill-me, /write-pages, ...) is shown as typed but the
	// agent receives its full instructions.
	deliver := o.Text
	if p, _, ok := commands.Expand(o.Text); ok {
		deliver = p
	}
	if o.Mode == "save" || o.Mode == "" {
		return res, nil
	}
	if o.Mode != "terminal" && o.Mode != "chat" && o.Mode != "auto" {
		return nil, fmt.Errorf("unknown send mode %q", o.Mode)
	}
	tid := s.termFor(conv.ID)
	if o.Mode == "chat" {
		tid = ""
	}
	if tid == "" && o.Mode == "terminal" {
		res.Note = "Saved. No agent is running in the terminal yet, so nothing was delivered. Start it from the terminal panel."
		return res, nil
	}
	if tid == "" {
		var b strings.Builder
		b.WriteString(deliver)
		for _, m := range msg.Media {
			fmt.Fprintf(&b, "\n[attached %s: %s]", m.Kind, m.Stored)
		}
		res.Note, res.Running = s.enqueueChat(conv.ID, strings.TrimSpace(b.String()))
		return res, nil
	}
	var b strings.Builder
	b.WriteString(deliver)
	for _, m := range msg.Media {
		fmt.Fprintf(&b, "\n[attached %s: %s]", m.Kind, m.Stored)
	}
	if err := s.term.Write(tid, []byte("\x1b[200~"+strings.TrimSpace(b.String())+"\x1b[201~")); err != nil {
		res.Note = "Saved, but delivery failed: " + err.Error()
		return res, nil
	}
	time.AfterFunc(80*time.Millisecond, func() { s.term.Write(tid, []byte("\r")) })
	res.Sent = true
	return res, nil
}

// ImportMediaData stores pasted data (for example a clipboard image).
func (s *Service) ImportMediaData(wsID int64, name, b64 string) (*storage.Media, error) {
	if _, err := s.Store.Workspace(wsID); err != nil {
		return nil, errors.New("unknown workspace")
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, errors.New("invalid data")
	}
	if int64(len(data)) > 256<<20 {
		return nil, errors.New("pasted data is larger than 256 MB")
	}
	tmp, err := os.MkdirTemp("", "aicw-paste-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	p := filepath.Join(tmp, media.SafeName(name))
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return nil, err
	}
	im, err := media.Import(p, s.mediaDir(wsID), media.Unique())
	if err != nil {
		return nil, err
	}
	return s.Store.AddMedia(storage.Media{WorkspaceID: wsID, Name: im.Name, Stored: im.Stored, Kind: im.Kind, Mime: im.Mime, Size: im.Size})
}

// ---- integrated terminal ----

type TermStartOptions struct {
	WorkspaceID    int64  `json:"workspaceId"`
	ConversationID int64  `json:"conversationId"`
	Agent          string `json:"agent"` // agent id, or "shell"
	Mode           string `json:"mode"`  // new | resume
	Cols           int    `json:"cols"`
	Rows           int    `json:"rows"`
}

type TermInfo struct {
	ID             string   `json:"id"`
	ConversationID int64    `json:"conversationId"`
	Agent          string   `json:"agent"`
	Running        bool     `json:"running"`
	Command        string   `json:"command"`
	Notes          []string `json:"notes"`
	SessionID      int64    `json:"sessionId"`
	Error          string   `json:"error"`
	Fallback       string   `json:"fallback"`
}

func (s *Service) termFor(convID int64) string {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if id, ok := s.convTerm[convID]; ok && s.term.Alive(id) {
		return id
	}
	return ""
}

// TermStatus reports the running terminal of a conversation, if any.
func (s *Service) TermStatus(convID int64) *TermInfo {
	id := s.termFor(convID)
	if id == "" {
		return nil
	}
	s.tmu.Lock()
	defer s.tmu.Unlock()
	return &TermInfo{ID: id, ConversationID: convID, Running: true, SessionID: s.termSession[id]}
}

func (s *Service) onTermExit(id string, code int) {
	s.tmu.Lock()
	conv, sid := s.termConv[id], s.termSession[id]
	delete(s.convTerm, conv)
	delete(s.termConv, id)
	delete(s.termSession, id)
	s.tmu.Unlock()
	if sid != 0 {
		paused, exit := storage.StatusPaused, fmt.Sprint(code)
		zero := 0
		s.Store.UpdateSession(sid, storage.SessionPatch{Status: &paused, ExitStatus: &exit, PID: &zero})
	}
	s.emit("term:exit", map[string]any{"id": id, "code": code, "conversationId": conv})
}

func (s *Service) skillsFor(wsID int64) []storage.Skill {
	k, _ := s.Store.Skills(wsID)
	return k
}

// TermStart starts an agent CLI (or a plain shell) inside a PTY. It only runs
// because the user asked; a second start for the same conversation returns
// the running terminal instead of launching a duplicate.
func (s *Service) TermStart(o TermStartOptions) (*TermInfo, error) {
	w, err := s.wsPath(o.WorkspaceID)
	if err != nil {
		return nil, err
	}
	conv, err := s.Store.Conversation(o.ConversationID)
	if err != nil || conv.WorkspaceID != w.ID {
		return nil, errors.New("conversation does not belong to this workspace")
	}
	if id := s.termFor(conv.ID); id != "" {
		return &TermInfo{ID: id, ConversationID: conv.ID, Running: true, SessionID: s.termSession[id], Agent: conv.Agent}, nil
	}
	agent := o.Agent
	if agent == "" {
		agent = conv.Agent
	}
	if agent == "" {
		agent = s.GetSettings().DefaultAgent
	}
	info := &TermInfo{ConversationID: conv.ID, Agent: agent}

	var exe string
	var args []string
	var externalID string
	var sess *storage.Session

	selfExe, _ := os.Executable()
	env := append(os.Environ(),
		"TERM=xterm-256color", "COLORTERM=truecolor",
		"AICW_WORKSPACE="+w.Path, fmt.Sprintf("AICW_CONVERSATION=%d", conv.ID),
		"PATH="+filepath.Dir(selfExe)+string(os.PathListSeparator)+os.Getenv("PATH"))

	if agent == "shell" {
		exe = defaultShell()
		info.Command = exe
	} else {
		prof, ok := s.Reg.Get(agent)
		if !ok {
			return nil, fmt.Errorf("unknown agent %q", agent)
		}
		st := s.Reg.Detect(context.Background(), agent)
		if st.State != agents.StatusDetected {
			info.Error = fmt.Sprintf("%s is %s: %s", prof.DisplayName, strings.ToLower(st.State), st.Detail)
			info.Fallback = "Install it or set its path in Settings → AI agents. You can still open a plain shell terminal."
			return info, nil
		}
		req := agents.LaunchRequest{Mode: agents.ModeNew, Cwd: w.Path, Exe: st.Path}
		if o.Mode == "resume" && conv.SessionID != 0 {
			if sess, err = s.Store.Session(conv.SessionID); err == nil {
				req.Mode, req.SessionID = agents.ModeResume, sess.ExternalID
			}
		}
		if req.Mode == agents.ModeNew {
			sess = nil
			pkg, err := s.BuildContext(ContextRequest{WorkspaceID: w.ID})
			if err != nil {
				return nil, err
			}
			req.ContextFile = filepath.Join(s.contextDir(w.ID), time.Now().Format("20060102-150405")+"-context.md")
			if err := os.MkdirAll(filepath.Dir(req.ContextFile), 0o700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(req.ContextFile, []byte(pkg.Markdown), 0o600); err != nil {
				return nil, err
			}
			req.SessionID = newUUID()
			req.Prompt = strings.ReplaceAll(agents.ContextPrompt, "{contextFile}", req.ContextFile)
		}
		if agent == "claude" { // verified flags: --mcp-config, --append-system-prompt
			cfg, err := s.writeMCPConfig(w.ID, w.Path, selfExe)
			if err != nil {
				return nil, err
			}
			req.Extra = []string{"--mcp-config", cfg, "--append-system-prompt", rules.Prompt(s.skillsFor(w.ID))}
		}
		spec, err := agents.BuildLaunch(prof, req)
		if err != nil {
			info.Error = err.Error()
			info.Fallback = "Start a new chat session instead; AICW includes saved notes in the context."
			return info, nil
		}
		exe, args, externalID = spec.Exe, spec.Args, spec.ExternalID
		info.Command, info.Notes = spec.Display, spec.Notes
	}

	t, err := s.term.Start(term.Spec{Exe: exe, Args: args, Dir: w.Path, Env: env, Cols: uint16(o.Cols), Rows: uint16(o.Rows)})
	if err != nil {
		info.Error = err.Error()
		return info, nil
	}
	info.ID, info.Running = t.ID, true
	active := storage.StatusActive
	if agent != "shell" {
		if sess == nil {
			sess, err = s.Store.CreateSession(storage.Session{WorkspaceID: w.ID, Agent: agent, ExternalID: externalID, Status: active, Title: conv.Title, PID: t.PID, ContextVersion: fmt.Sprint(contextbuilder.FormatVersion)})
		} else {
			err = s.Store.UpdateSession(sess.ID, storage.SessionPatch{Status: &active, PID: &t.PID})
		}
		if err == nil {
			info.SessionID = sess.ID
			s.Store.UpdateConversation(conv.ID, nil, &agent, &sess.ID)
		}
	}
	s.tmu.Lock()
	s.convTerm[conv.ID], s.termConv[t.ID] = t.ID, conv.ID
	if sess != nil {
		s.termSession[t.ID] = sess.ID
	}
	s.tmu.Unlock()
	return info, nil
}

func (s *Service) writeMCPConfig(wsID int64, wsPath, exe string) (string, error) {
	dir := filepath.Join(s.DataDir, "mcp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	cfg := map[string]any{"mcpServers": map[string]any{"aicw": map[string]any{"command": exe, "args": []string{"mcp", "--workspace", wsPath}, "env": map[string]string{"AICW_DATA_DIR": s.DataDir}}}}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	p := filepath.Join(dir, fmt.Sprintf("%d.json", wsID))
	return p, os.WriteFile(p, b, 0o600)
}

func (s *Service) TermWrite(id, data string) error { return s.term.Write(id, []byte(data)) }
func (s *Service) TermResize(id string, cols, rows int) error {
	return s.term.Resize(id, uint16(cols), uint16(rows))
}
func (s *Service) TermBuffer(id string) string {
	return base64.StdEncoding.EncodeToString(s.term.Buffer(id))
}
func (s *Service) TermClose(id string) error { return s.term.Close(id) }

// ---- skills (always-on instructions) and notes ----

func (s *Service) ListAgentSkills(wsID int64) ([]storage.Skill, error) { return s.Store.Skills(wsID) }

func (s *Service) StarterSkills() []storage.Skill { return rules.Starters() }

func (s *Service) SaveAgentSkill(wsID int64, name, desc, body string, alwaysOn bool) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 || strings.TrimSpace(body) == "" {
		return errors.New("a name (up to 64 characters) and instructions are required")
	}
	if len(body) > 256*1024 {
		return errors.New("skill is too large")
	}
	return s.Store.PutSkill(wsID, name, strings.TrimSpace(desc), body, alwaysOn, "user")
}

func (s *Service) DeleteAgentSkill(wsID int64, name string, confirmed bool) error {
	if !confirmed {
		return markdown.ErrNotConfirmed
	}
	return s.Store.DeleteSkill(wsID, name)
}

func (s *Service) ListNotes(wsID int64) ([]storage.Note, error) { return s.Store.Notes(wsID) }

func (s *Service) SaveNote(wsID int64, name, body string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 || len(body) > 256*1024 {
		return errors.New("note needs a name (up to 64 characters) and at most 256 KB of text")
	}
	return s.Store.PutNote(wsID, name, body, "user")
}

func (s *Service) NoteVersions(wsID int64, name string) ([]storage.NoteVersion, error) {
	return s.Store.NoteVersions(wsID, name)
}

func (s *Service) DeleteNote(wsID int64, name string, confirmed bool) error {
	if !confirmed {
		return markdown.ErrNotConfirmed
	}
	return s.Store.DeleteNote(wsID, name)
}

var instructionFiles = map[string]bool{"AGENTS.md": true, "CLAUDE.md": true, "GEMINI.md": true}

type InstructionStatus struct {
	File     string `json:"file"`
	Exists   bool   `json:"exists"`
	HasBlock bool   `json:"hasBlock"`
	Current  bool   `json:"current"` // block matches the skills as they are now
}

func (s *Service) InstructionsStatus(wsID int64) ([]InstructionStatus, error) {
	w, err := s.wsPath(wsID)
	if err != nil {
		return nil, err
	}
	block := rules.Block(s.skillsFor(wsID))
	var out []InstructionStatus
	for _, f := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"} {
		st := InstructionStatus{File: f}
		if b, err := os.ReadFile(filepath.Join(w.Path, f)); err == nil {
			st.Exists = true
			cur := rules.Extract(string(b))
			st.HasBlock, st.Current = cur != "", cur == strings.TrimRight(block, "\n")
		}
		out = append(out, st)
	}
	return out, nil
}

func (s *Service) instructionContent(wsID int64, file string) (string, error) {
	if !instructionFiles[file] {
		return "", errors.New("unsupported instruction file")
	}
	w, err := s.wsPath(wsID)
	if err != nil {
		return "", err
	}
	cur, _ := os.ReadFile(filepath.Join(w.Path, file))
	return rules.Apply(string(cur), rules.Block(s.skillsFor(wsID))), nil
}

// PreviewInstructions shows the diff for installing the always-on block into
// AGENTS.md / CLAUDE.md / GEMINI.md. Only the managed block changes.
func (s *Service) PreviewInstructions(wsID int64, file string) (*markdown.Preview, error) {
	content, err := s.instructionContent(wsID, file)
	if err != nil {
		return nil, err
	}
	return s.PreviewWrite(wsID, file, content)
}

func (s *Service) ApplyInstructions(wsID int64, file, baseHash string, confirmed bool) (*markdown.AuditEntry, error) {
	content, err := s.instructionContent(wsID, file)
	if err != nil {
		return nil, err
	}
	return s.ApplyWrite(wsID, file, content, baseHash, confirmed)
}

// ---- voice ----

type VoiceStatus struct {
	Recorder  string `json:"recorder"`
	Engine    string `json:"engine"`
	Ready     bool   `json:"ready"`
	Recording bool   `json:"recording"`
	Hint      string `json:"hint"`
}

func (s *Service) voiceSettings() voice.Settings {
	st := s.GetSettings()
	return voice.Settings{Command: st.VoiceCommand, Model: st.VoiceModel, Language: st.VoiceLanguage}
}

func (s *Service) VoiceStatus() VoiceStatus {
	v := VoiceStatus{Recorder: voice.RecorderTool(), Recording: s.rec.Recording()}
	e, ok := voice.Detect(s.DataDir, s.voiceSettings())
	v.Engine, v.Ready = e.Name, ok && v.Recorder != ""
	switch {
	case v.Recorder == "":
		v.Hint = "No audio recorder found. Install pipewire (pw-record), pulseaudio-utils (parecord) or alsa-utils (arecord)."
	case !ok:
		v.Hint = "No speech-to-text engine yet. Use “Set up offline voice” (installs faster-whisper in AICW's own folder) or set a command in Settings."
	}
	return v
}

func (s *Service) VoiceStart() error {
	return s.rec.Start(filepath.Join(s.DataDir, "voice", "tmp"))
}

func (s *Service) VoiceCancel() { s.rec.Cancel() }

// VoiceStop ends recording and returns the transcript. The audio file is
// deleted afterwards and never leaves the machine.
func (s *Service) VoiceStop() (string, error) {
	file, err := s.rec.Stop()
	if err != nil {
		return "", err
	}
	defer os.Remove(file)
	e, ok := voice.Detect(s.DataDir, s.voiceSettings())
	if !ok {
		return "", errors.New("no speech-to-text engine is set up yet (see Settings → Voice)")
	}
	text, err := voice.Transcribe(context.Background(), e, file)
	if err != nil {
		return "", err
	}
	if text == "" {
		return "", errors.New("no speech was recognised")
	}
	return text, nil
}

// VoiceSetup installs the offline engine; progress is emitted as events.
func (s *Service) VoiceSetup(confirmed bool) error {
	if !confirmed {
		return markdown.ErrNotConfirmed
	}
	go func() {
		err := voice.Setup(context.Background(), s.DataDir, func(l string) { s.emit("voice:setup", l) })
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		s.emit("voice:setup:done", msg)
	}()
	return nil
}

// ---- chat turns: run the agent non-interactively and post its reply ----

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]|\x1b\\][^\x07]*\x07")

type ChatState struct {
	Running bool `json:"running"`
	Queued  int  `json:"queued"`
}

func (s *Service) ChatStatus(convID int64) ChatState {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	_, running := s.chatCancel[convID]
	return ChatState{Running: running, Queued: len(s.chatQueue[convID])}
}

// StopChat cancels the running reply and drops anything queued behind it.
func (s *Service) StopChat(convID int64) {
	s.tmu.Lock()
	cancel := s.chatCancel[convID]
	s.chatQueue[convID] = nil
	s.tmu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) SetChatEdits(on bool) error {
	st := s.GetSettings()
	st.ChatEdits = on
	b, _ := json.Marshal(st)
	return s.Store.SetSetting("settings", string(b))
}

func (s *Service) postSystem(convID int64, text string) {
	s.Store.AddMessage(convID, storage.RoleSystem, storage.SourceUI, text, nil)
}

// enqueueChat starts (or queues) a reply for the conversation's agent.
func (s *Service) enqueueChat(convID int64, prompt string) (note string, running bool) {
	conv, err := s.Store.Conversation(convID)
	if err != nil {
		return "Conversation not found.", false
	}
	agent := conv.Agent
	if agent == "" {
		agent = s.GetSettings().DefaultAgent
	}
	prof, ok := s.Reg.Get(agent)
	if agent == "shell" || !ok || prof.Chat == nil {
		return "Saved. " + agentLabel(prof, agent) + " has no chat mode; start it in the terminal panel to talk to it, or pick another agent.", false
	}
	if st := s.Reg.Detect(context.Background(), agent); st.State != agents.StatusDetected {
		s.postSystem(convID, fmt.Sprintf("%s is %s (%s). Install it or set its path in Settings, or choose another agent.", prof.DisplayName, strings.ToLower(st.State), st.Detail))
		return "", false
	}
	s.tmu.Lock()
	if _, busy := s.chatCancel[convID]; busy {
		s.chatQueue[convID] = append(s.chatQueue[convID], prompt)
		n := len(s.chatQueue[convID])
		s.tmu.Unlock()
		return fmt.Sprintf("The agent is still working; your message is queued (%d waiting).", n), true
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.chatCancel[convID] = cancel
	s.tmu.Unlock()
	go s.chatLoop(ctx, convID, prompt)
	return "", true
}

func agentLabel(p agents.Profile, id string) string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return id
}

func (s *Service) chatLoop(ctx context.Context, convID int64, first string) {
	prompt := first
	for {
		s.runChatTurn(ctx, convID, prompt)
		s.tmu.Lock()
		q := s.chatQueue[convID]
		if len(q) == 0 || ctx.Err() != nil {
			delete(s.chatCancel, convID)
			delete(s.chatQueue, convID)
			s.tmu.Unlock()
			s.emit("chat:turn", map[string]any{"conversationId": convID, "state": "idle"})
			return
		}
		prompt, s.chatQueue[convID] = q[0], q[1:]
		s.tmu.Unlock()
	}
}

func (s *Service) runChatTurn(ctx context.Context, convID int64, prompt string) {
	conv, err := s.Store.Conversation(convID)
	if err != nil {
		return
	}
	w, err := s.wsPath(conv.WorkspaceID)
	if err != nil {
		s.postSystem(convID, err.Error())
		return
	}
	agent := conv.Agent
	if agent == "" {
		agent = s.GetSettings().DefaultAgent
	}
	prof, _ := s.Reg.Get(agent)
	st := s.Reg.Detect(ctx, agent)
	selfExe, _ := os.Executable()

	var sess *storage.Session
	if conv.SessionID != 0 {
		sess, _ = s.Store.Session(conv.SessionID)
	}
	first := sess == nil
	req := agents.ChatRequest{Cwd: w.Path, Exe: st.Path, Prompt: prompt, FirstTurn: first, Edits: s.GetSettings().ChatEdits}
	if first {
		req.NewID = newUUID()
		if agent != "claude" { // others get the context package path; Claude gets rules + MCP tools instead
			if pkg, err := s.BuildContext(ContextRequest{WorkspaceID: w.ID}); err == nil {
				f := filepath.Join(s.contextDir(w.ID), time.Now().Format("20060102-150405")+"-context.md")
				if os.MkdirAll(filepath.Dir(f), 0o700) == nil && os.WriteFile(f, []byte(pkg.Markdown), 0o600) == nil {
					req.Prompt = "First read the project context package at " + f + " (treat it as a starting point and verify claims). Then answer:\n\n" + prompt
				}
			}
		}
	} else {
		req.SessionID = sess.ExternalID
	}
	if agent == "claude" {
		cfg, err := s.writeMCPConfig(w.ID, w.Path, selfExe)
		if err != nil {
			s.postSystem(convID, "Could not prepare the AICW tools: "+err.Error())
			return
		}
		// Headless runs can't ask for permission, so AICW's own tools (notes,
		// pages, skills) are pre-approved; file edits stay governed by the toggle.
		req.Extra = []string{"--mcp-config", cfg, "--allowedTools", "mcp__aicw", "--append-system-prompt", rules.Prompt(s.skillsFor(w.ID))}
	}
	launch, err := agents.BuildChat(prof, req)
	if err != nil {
		s.postSystem(convID, err.Error())
		return
	}

	active, paused := storage.StatusActive, storage.StatusPaused
	if first {
		sess, err = s.Store.CreateSession(storage.Session{WorkspaceID: w.ID, Agent: agent, ExternalID: launch.ExternalID, Status: active, Title: conv.Title})
		if err != nil {
			s.postSystem(convID, err.Error())
			return
		}
		s.Store.UpdateConversation(convID, nil, &agent, &sess.ID)
	} else {
		s.Store.UpdateSession(sess.ID, storage.SessionPatch{Status: &active})
	}
	defer func() { s.Store.UpdateSession(sess.ID, storage.SessionPatch{Status: &paused}) }()
	s.emit("chat:turn", map[string]any{"conversationId": convID, "state": "running"})

	runCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(runCtx, launch.Exe, launch.Args...)
	cmd.Dir = w.Path
	cmd.Env = append(os.Environ(), "TERM=dumb", "NO_COLOR=1", "AICW_WORKSPACE="+w.Path, fmt.Sprintf("AICW_CONVERSATION=%d", convID),
		"PATH="+filepath.Dir(selfExe)+string(os.PathListSeparator)+os.Getenv("PATH"))
	killTreeOnCancel(cmd)
	if launch.Stdin != "" {
		cmd.Stdin = strings.NewReader(launch.Stdin)
	}
	var stdout, stderr limitedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	switch {
	case ctx.Err() != nil:
		s.postSystem(convID, "Stopped.")
	case runErr != nil:
		msg := strings.TrimSpace(ansiRe.ReplaceAllString(stderr.String(), ""))
		if len(msg) > 1200 {
			msg = "…" + msg[len(msg)-1200:]
		}
		if runCtx.Err() == context.DeadlineExceeded {
			msg = "timed out after 20 minutes"
		}
		s.postSystem(convID, fmt.Sprintf("%s failed (%v).\n\n%s", prof.DisplayName, runErr, msg))
	default:
		out := strings.TrimSpace(ansiRe.ReplaceAllString(stdout.String(), ""))
		if out == "" {
			s.postSystem(convID, prof.DisplayName+" finished but returned no text.")
		} else {
			if stdout.truncated {
				out += "\n\n_(output truncated)_"
			}
			s.Store.AddMessage(convID, storage.RoleAssistant, storage.SourceAgent, out, nil)
		}
	}
	for _, n := range launch.Notes {
		if first || runErr != nil {
			s.postSystem(convID, n)
		}
	}
}

// limitedBuffer keeps at most 512 KB of output.
type limitedBuffer struct {
	b         []byte
	truncated bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	const max = 512 * 1024
	if room := max - len(l.b); room > 0 {
		if len(p) > room {
			l.b, l.truncated = append(l.b, p[:room]...), true
		} else {
			l.b = append(l.b, p...)
		}
	} else if len(p) > 0 {
		l.truncated = true
	}
	return len(p), nil
}
func (l *limitedBuffer) String() string { return string(l.b) }

// VoiceResult is a recorded voice message: the audio kept as media plus a
// transcript when a speech engine is available.
type VoiceResult struct {
	Media      *storage.Media `json:"media"`
	Transcript string         `json:"transcript"`
	Warning    string         `json:"warning"`
}

// VoiceStopAttach stops recording and keeps the audio as a media item (so it
// can be sent as an audio attachment). It also tries to transcribe it; if that
// is not possible the audio is still returned with a warning.
func (s *Service) VoiceStopAttach(wsID int64) (*VoiceResult, error) {
	if _, err := s.Store.Workspace(wsID); err != nil {
		return nil, errors.New("unknown workspace")
	}
	file, err := s.rec.Stop()
	if err != nil {
		return nil, err
	}
	defer os.Remove(file)
	res := &VoiceResult{}
	if e, ok := voice.Detect(s.DataDir, s.voiceSettings()); ok {
		if t, err := voice.Transcribe(context.Background(), e, file); err != nil {
			res.Warning = "Could not transcribe: " + err.Error()
		} else {
			res.Transcript = t
		}
	} else {
		res.Warning = "No speech-to-text engine is set up, so only the audio is attached (set it up in Settings → Voice)."
	}
	named := filepath.Join(filepath.Dir(file), "voice-"+time.Now().Format("20060102-150405")+".wav")
	if err := os.Rename(file, named); err == nil {
		file = named
	}
	defer os.Remove(named)
	im, err := media.Import(file, s.mediaDir(wsID), media.Unique())
	if err != nil {
		return nil, err
	}
	m, err := s.Store.AddMedia(storage.Media{WorkspaceID: wsID, Name: im.Name, Stored: im.Stored, Kind: im.Kind, Mime: im.Mime, Size: im.Size, Note: res.Transcript})
	if err != nil {
		os.Remove(im.Stored)
		return nil, err
	}
	res.Media = m
	return res, nil
}

// SlashCommands lists the fixed composer commands for the "/" menu.
func (s *Service) SlashCommands() []commands.Command { return commands.All() }
