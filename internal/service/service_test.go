package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aicw/internal/markdown"
	"aicw/internal/storage"
)

func newSvc(t *testing.T) *Service {
	s, err := New(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Lifecycle().Close() })
	return s
}

func project(t *testing.T) string {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Agents\n"), 0o644)
	os.WriteFile(filepath.Join(root, "README.md"), []byte("# Demo\n\nHello.\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".env"), []byte("TOKEN=abcdefghijklmnopqrstuvwxyz"), 0o644)
	return root
}

// installFake puts a fake `claude` on PATH.
func installFake(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\necho 'claude 1.0.0'\n"), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestOpenWorkspaceResumesAndNeverCreatesSession(t *testing.T) {
	s := newSvc(t)
	root := project(t)
	r1, err := s.OpenWorkspace(root, false)
	if err != nil || r1.Action != "new" {
		t.Fatalf("%+v %v", r1, err)
	}
	sess, _ := s.WS.StartSession(r1.Workspace.ID, "claude", "task")
	ext := "uuid-123"
	s.Store.UpdateSession(sess.ID, storage.SessionPatch{ExternalID: &ext})
	r2, err := s.OpenWorkspace(root, false)
	if err != nil || r2.Action != "resume" || r2.Resumable[0].ExternalID != "uuid-123" || len(r2.Sessions) != 1 {
		t.Fatalf("expected resume: %+v %v", r2, err)
	}
	r3, _ := s.OpenWorkspace(root, true)
	if !r3.RequestedNew || len(r3.Sessions) != 1 {
		t.Fatalf("--new must keep old sessions: %+v", r3)
	}
	if _, err := s.OpenWorkspace(filepath.Join(root, "missing"), false); err == nil {
		t.Fatal("invalid path accepted")
	}
	if _, err := os.Stat(filepath.Join(root, ".aicw")); err == nil {
		t.Fatal(".aicw created implicitly")
	}
}

func TestLaunchDryRunHasNoSideEffects(t *testing.T) {
	installFake(t)
	s := newSvc(t)
	r, _ := s.OpenWorkspace(project(t), false)
	res, err := s.Launch(LaunchOptions{WorkspaceID: r.Workspace.ID, Agent: "claude", Mode: "new", DryRun: true})
	if err != nil || res.Error != "" || res.Spec == nil {
		t.Fatalf("%+v %v", res, err)
	}
	if res.Spec.Args[0] != "--session-id" || res.Spec.ExternalID == "" {
		t.Fatalf("claude should pre-assign a session id: %+v", res.Spec.Args)
	}
	if !strings.Contains(res.Disclosure, "sent to that tool's provider") {
		t.Fatal("missing data-flow disclosure")
	}
	if all, _ := s.Store.Sessions(r.Workspace.ID); len(all) != 0 {
		t.Fatal("dry run created a session")
	}
	if entries, _ := os.ReadDir(filepath.Join(s.DataDir, "contexts")); len(entries) != 0 {
		t.Fatal("dry run wrote a context file")
	}
}

func TestLaunchMissingCLIGivesGuidanceNotSimulation(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	s := newSvc(t)
	r, _ := s.OpenWorkspace(project(t), false)
	res, err := s.Launch(LaunchOptions{WorkspaceID: r.Workspace.ID, Agent: "claude", Mode: "new"})
	if err != nil || res.Launched || !strings.Contains(res.Error, "not installed") || res.Fallback == "" {
		t.Fatalf("%+v %v", res, err)
	}
	if all, _ := s.Store.Sessions(r.Workspace.ID); len(all) != 0 {
		t.Fatal("no session may be recorded when nothing launched")
	}
}

func TestResumeUsesSavedIDDuplicateGuardAndIsolation(t *testing.T) {
	installFake(t)
	s := newSvc(t)
	r, _ := s.OpenWorkspace(project(t), false)
	sess, _ := s.WS.StartSession(r.Workspace.ID, "claude", "x")
	pid := os.Getpid() // a live process
	active, ext := storage.StatusActive, "abc"
	s.Store.UpdateSession(sess.ID, storage.SessionPatch{Status: &active, PID: &pid, ExternalID: &ext})
	res, err := s.Launch(LaunchOptions{WorkspaceID: r.Workspace.ID, Mode: "resume", SessionID: sess.ID, DryRun: true})
	if err != nil || !strings.Contains(res.Error, "already") {
		t.Fatalf("duplicate guard failed: %+v %v", res, err)
	}
	res, _ = s.Launch(LaunchOptions{WorkspaceID: r.Workspace.ID, Mode: "resume", SessionID: sess.ID, DryRun: true, Force: true})
	if res.Spec == nil || res.Spec.Args[0] != "--resume" || res.Spec.Args[1] != "abc" {
		t.Fatalf("force resume: %+v", res)
	}
	other, _ := s.OpenWorkspace(project(t), false)
	if _, err := s.Launch(LaunchOptions{WorkspaceID: other.Workspace.ID, Mode: "resume", SessionID: sess.ID, DryRun: true}); err == nil {
		t.Fatal("session from another workspace must be rejected")
	}
}

func TestGuardedWritesAndPathValidation(t *testing.T) {
	s := newSvc(t)
	root := project(t)
	r, _ := s.OpenWorkspace(root, false)
	id := r.Workspace.ID
	if _, err := s.ApplyWrite(id, "AGENTS.md", "new\n", "", false); err != markdown.ErrNotConfirmed {
		t.Fatalf("unconfirmed write: %v", err)
	}
	pv, _ := s.PreviewWrite(id, "AGENTS.md", "new\n")
	if !pv.Changed || !strings.Contains(pv.Diff, "+new") {
		t.Fatalf("%+v", pv)
	}
	if _, err := s.ApplyWrite(id, "../../etc/x", "x", "", true); err == nil {
		t.Fatal("traversal accepted")
	}
	if _, err := s.ApplyWrite(id, "AGENTS.md", "new\n", pv.BaseHash, true); err != nil {
		t.Fatal(err)
	}
	if b, _ := s.Backups(id, "AGENTS.md"); len(b) != 1 {
		t.Fatalf("backup missing: %v", b)
	}
	if _, err := os.Stat(filepath.Join(root, ".aicw")); err == nil {
		t.Fatal("history must not be written into the project")
	}
}

func TestReadFileRefusesSecrets(t *testing.T) {
	s := newSvc(t)
	r, _ := s.OpenWorkspace(project(t), false)
	if _, err := s.ReadFile(r.Workspace.ID, ".env"); err == nil {
		t.Fatal(".env must not be readable via UI")
	}
	if txt, err := s.ReadFile(r.Workspace.ID, "README.md"); err != nil || !strings.Contains(txt, "Hello") {
		t.Fatalf("%q %v", txt, err)
	}
	if _, err := s.ReadFile(r.Workspace.ID, "../../etc/passwd"); err == nil {
		t.Fatal("traversal accepted")
	}
}

func TestBuildContextExcludesSecretsAndHonoursExclusions(t *testing.T) {
	s := newSvc(t)
	r, _ := s.OpenWorkspace(project(t), false)
	s.SaveWorkspaceState(r.Workspace.ID, WorkspaceState{Excluded: []string{"README.md"}}, "claude")
	pkg, err := s.BuildContext(ContextRequest{WorkspaceID: r.Workspace.ID, Task: "do it"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pkg.Markdown, "abcdefghijklmnopqrstuvwxyz") || strings.Contains(pkg.Markdown, "Hello.") {
		t.Fatal("excluded/secret content leaked")
	}
}

func TestCustomAgentImportIsValidatedData(t *testing.T) {
	s := newSvc(t)
	if err := s.AddCustomAgent(`{"id":"x","displayName":"X","executables":["x"],"handoff":"file","capabilities":{"newSession":true},"bogusField":1}`); err == nil {
		t.Fatal("unknown fields must be rejected")
	}
	if err := s.AddCustomAgent(`{"id":"x","displayName":"X","executables":["x"],"handoff":"file","capabilities":{"newSession":true}}`); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.Reg.Get("x"); p.Verified {
		t.Fatal("custom agents are never verified")
	}
	if err := s.RemoveCustomAgent("claude"); err == nil {
		t.Fatal("built-in removal accepted")
	}
	s2, err := New(s.DataDir, "t")
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Lifecycle().Close()
	if _, ok := s2.Reg.Get("x"); !ok {
		t.Fatal("custom agent not persisted")
	}
}

func TestSettingsValidation(t *testing.T) {
	s := newSvc(t)
	st := s.GetSettings()
	st.Budget = 10
	if s.SaveSettings(st) == nil {
		t.Fatal("tiny budget accepted")
	}
	st = s.GetSettings()
	st.ContextDir = "relative/dir"
	if s.SaveSettings(st) == nil {
		t.Fatal("relative context dir accepted")
	}
	st = s.GetSettings()
	st.Budget = 4000
	if err := s.SaveSettings(st); err != nil || s.GetSettings().Budget != 4000 {
		t.Fatal(err)
	}
}

func TestArchiveAndUndo(t *testing.T) {
	s := newSvc(t)
	root := project(t)
	os.MkdirAll(filepath.Join(root, "context"), 0o755)
	os.WriteFile(filepath.Join(root, "context", "notes.md"), []byte("old notes"), 0o644)
	r, _ := s.OpenWorkspace(root, false)
	plan, _ := s.ArchivePlan(r.Workspace.ID, []string{"context/notes.md"})
	if _, err := s.ApplyMoves(r.Workspace.ID, plan, false); !errors.Is(err, markdown.ErrNotConfirmed) {
		t.Fatal("unconfirmed move must fail")
	}
	if _, err := s.ApplyMoves(r.Workspace.ID, plan, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, plan[0].To)); err != nil {
		t.Fatal("archive destination missing")
	}
	undo := []MovePair{{From: plan[0].To, To: plan[0].From}}
	if _, err := s.ApplyMoves(r.Workspace.ID, undo, true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "context", "notes.md")); string(b) != "old notes" {
		t.Fatal("undo lost content")
	}
}

func TestMediaImportServeAndDelete(t *testing.T) {
	s := newSvc(t)
	r, _ := s.OpenWorkspace(project(t), false)
	id := r.Workspace.ID
	src := filepath.Join(t.TempDir(), "clip.mp4")
	os.WriteFile(src, []byte("0123456789"), 0o644)
	res, err := s.ImportMedia(id, []string{src, "relative.png", filepath.Join(filepath.Dir(src), ".env")})
	if err != nil || len(res.Added) != 1 || len(res.Errors) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	m := res.Added[0]
	if m.Kind != "video" || m.Size != 10 || !strings.HasPrefix(m.Stored, s.DataDir) {
		t.Fatalf("%+v", m)
	}
	// served with Range support
	h := s.MediaHandler()
	req := httptest.NewRequest("GET", "/media/"+fmt.Sprint(m.ID), nil)
	req.Header.Set("Range", "bytes=2-5")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 206 || rec.Body.String() != "2345" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("%d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	for _, bad := range []string{"/media/999", "/media/abc", "/media/../x", "/other"} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", bad, nil))
		if rec.Code != 404 {
			t.Errorf("%s -> %d", bad, rec.Code)
		}
	}
	// in-context media appears in the package by path only
	s.UpdateMedia(m.ID, "see frame 3", true)
	pkg, _ := s.BuildContext(ContextRequest{WorkspaceID: id})
	if !strings.Contains(pkg.Markdown, m.Stored) || !strings.Contains(pkg.Markdown, "see frame 3") || strings.Contains(pkg.Markdown, "0123456789") {
		t.Fatal("media section wrong")
	}
	if err := s.DeleteMedia(m.ID, false); !errors.Is(err, markdown.ErrNotConfirmed) {
		t.Fatal("unconfirmed delete")
	}
	if err := s.DeleteMedia(m.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(m.Stored); err == nil {
		t.Fatal("stored copy remains")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal("the user's original must never be deleted")
	}
}

func TestWorkspaceListCountsAndPins(t *testing.T) {
	s := newSvc(t)
	a, _ := s.OpenWorkspace(project(t), false)
	b, _ := s.OpenWorkspace(project(t), false)
	s.WS.StartSession(a.Workspace.ID, "claude", "1")
	done, _ := s.WS.StartSession(a.Workspace.ID, "kilo", "2")
	st := storage.StatusCompleted
	s.Store.UpdateSession(done.ID, storage.SessionPatch{Status: &st})
	on, err := s.TogglePin(b.Workspace.ID)
	if err != nil || !on {
		t.Fatal(on, err)
	}
	list, _ := s.WorkspaceList()
	if len(list) != 2 {
		t.Fatal(list)
	}
	for _, w := range list {
		switch w.Workspace.ID {
		case a.Workspace.ID:
			if w.Sessions != 2 || w.Resumable != 1 || w.Pinned {
				t.Fatalf("%+v", w)
			}
		case b.Workspace.ID:
			if w.Sessions != 0 || !w.Pinned {
				t.Fatalf("%+v", w)
			}
		}
	}
	if on, _ := s.TogglePin(b.Workspace.ID); on {
		t.Fatal("second toggle should unpin")
	}
	if s.GetSettings().Budget != 8000 {
		t.Fatal("pinning must not disturb other settings")
	}
}

func TestChatSendDeliversToRunningTerminalAndAgentsCanReply(t *testing.T) {
	s := newSvc(t)
	r, _ := s.OpenWorkspace(project(t), false)
	id := r.Workspace.ID
	if err := s.AddCustomAgent(`{"id":"catagent","displayName":"Cat","executables":["cat"],"handoff":"none","capabilities":{"newSession":true}}`); err != nil {
		t.Fatal(err)
	}
	conv, _ := s.NewConversation(id, "", "catagent")
	// No terminal yet: saved, but not delivered, and the user is told why.
	res, err := s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "first question", Mode: "auto"})
	if err != nil || res.Sent || res.Note == "" {
		t.Fatalf("%+v %v", res, err)
	}
	if c, _ := s.Store.Conversation(conv.ID); c.Title != "first question" {
		t.Fatalf("title should come from the first message: %q", c.Title)
	}
	info, err := s.TermStart(TermStartOptions{WorkspaceID: id, ConversationID: conv.ID})
	if err != nil || info.Error != "" || !info.Running {
		t.Fatalf("%+v %v", info, err)
	}
	if again, _ := s.TermStart(TermStartOptions{WorkspaceID: id, ConversationID: conv.ID}); again.ID != info.ID {
		t.Fatal("second start must return the running terminal, not launch another")
	}
	// Attach a file and send.
	src := filepath.Join(t.TempDir(), "pic.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n.."), 0o644)
	imp, _ := s.ImportMedia(id, []string{src})
	res, err = s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "please look", MediaIDs: []int64{imp.Added[0].ID}, Mode: "auto"})
	if err != nil || !res.Sent {
		t.Fatalf("%+v %v", res, err)
	}
	var out string
	for i := 0; i < 100; i++ {
		b, _ := base64.StdEncoding.DecodeString(s.TermBuffer(info.ID))
		out = string(b)
		if strings.Contains(out, "please look") && strings.Contains(out, "[attached image: "+imp.Added[0].Stored+"]") {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if !strings.Contains(out, "please look") || !strings.Contains(out, "[attached image: ") {
		t.Fatalf("terminal never received the message: %q", out)
	}
	// An agent posting through the tools shows up in the same conversation.
	s.Store.AddMessage(conv.ID, storage.RoleAssistant, storage.SourceAgent, "done!", nil)
	msgs, _ := s.Messages(conv.ID, 0)
	if len(msgs) != 3 || msgs[2].Role != "assistant" {
		t.Fatalf("%+v", msgs)
	}
	// Closing ends the process and pauses the session record.
	s.TermClose(info.ID)
	time.Sleep(150 * time.Millisecond)
	if s.TermStatus(conv.ID) != nil {
		t.Fatal("terminal should be gone")
	}
	ses, _ := s.Store.Sessions(id)
	if len(ses) != 1 || ses[0].Status != storage.StatusPaused {
		t.Fatalf("%+v", ses)
	}
	if _, err := s.SendMessage(SendOptions{ConversationID: conv.ID}); err == nil {
		t.Fatal("empty message accepted")
	}
}

func TestShellTerminalAndWrongWorkspaceRejected(t *testing.T) {
	s := newSvc(t)
	a, _ := s.OpenWorkspace(project(t), false)
	b, _ := s.OpenWorkspace(project(t), false)
	conv, _ := s.NewConversation(a.Workspace.ID, "x", "shell")
	if _, err := s.TermStart(TermStartOptions{WorkspaceID: b.Workspace.ID, ConversationID: conv.ID, Agent: "shell"}); err == nil {
		t.Fatal("conversation from another workspace accepted")
	}
	info, err := s.TermStart(TermStartOptions{WorkspaceID: a.Workspace.ID, ConversationID: conv.ID, Agent: "shell"})
	if err != nil || !info.Running {
		t.Fatalf("%+v %v", info, err)
	}
	s.TermWrite(info.ID, "echo ws=$AICW_WORKSPACE\n")
	var out string
	for i := 0; i < 100 && !strings.Contains(out, "ws="+a.Workspace.Path); i++ {
		bb, _ := base64.StdEncoding.DecodeString(s.TermBuffer(info.ID))
		out = string(bb)
		time.Sleep(30 * time.Millisecond)
	}
	if !strings.Contains(out, "ws="+a.Workspace.Path) {
		t.Fatalf("AICW_WORKSPACE not set in terminal: %q", out)
	}
	s.TermClose(info.ID)
	if ses, _ := s.Store.Sessions(a.Workspace.ID); len(ses) != 0 {
		t.Fatal("a plain shell must not create an agent session")
	}
}

func TestInstructionsApplyOnlyManagedBlockWithConfirmation(t *testing.T) {
	s := newSvc(t)
	root := project(t)
	r, _ := s.OpenWorkspace(root, false)
	id := r.Workspace.ID
	for _, k := range s.StarterSkills() {
		s.SaveAgentSkill(id, k.Name, k.Description, k.Body, k.AlwaysOn)
	}
	st, _ := s.InstructionsStatus(id)
	if !st[0].Exists || st[0].HasBlock || st[1].Exists {
		t.Fatalf("%+v", st)
	}
	pv, err := s.PreviewInstructions(id, "AGENTS.md")
	if err != nil || !strings.Contains(pv.Diff, "+<!-- AICW:BEGIN") || !strings.Contains(pv.Diff, "read-context-first") {
		t.Fatalf("%+v %v", pv, err)
	}
	if _, err := s.ApplyInstructions(id, "AGENTS.md", pv.BaseHash, false); !errors.Is(err, markdown.ErrNotConfirmed) {
		t.Fatal("must require confirmation")
	}
	if _, err := s.PreviewInstructions(id, "../../etc/passwd"); err == nil {
		t.Fatal("arbitrary file accepted")
	}
	if _, err := s.ApplyInstructions(id, "AGENTS.md", pv.BaseHash, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.HasPrefix(string(b), "# Agents\n") || !strings.Contains(string(b), "read_context") {
		t.Fatalf("%s", b)
	}
	if st, _ := s.InstructionsStatus(id); !st[0].Current {
		t.Fatal("status should be current after applying")
	}
	s.SaveAgentSkill(id, "extra", "d", "extra rule", true)
	if st, _ := s.InstructionsStatus(id); st[0].Current {
		t.Fatal("status should go stale after skills change")
	}
	if bk, _ := s.Backups(id, "AGENTS.md"); len(bk) != 1 {
		t.Fatal("previous AGENTS.md must be backed up")
	}
}

func TestNotesFlowIntoContextPackage(t *testing.T) {
	s := newSvc(t)
	r, _ := s.OpenWorkspace(project(t), false)
	s.SaveNote(r.Workspace.ID, "project-memory", "Uses sqlite. Deploy on Fridays never.")
	pkg, _ := s.BuildContext(ContextRequest{WorkspaceID: r.Workspace.ID})
	if !strings.Contains(pkg.Markdown, "Deploy on Fridays never") || !strings.Contains(pkg.Markdown, "Living notes") {
		t.Fatal("notes missing from package")
	}
}

func TestVoiceStatusAndStopWithoutEngine(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	s := newSvc(t)
	v := s.VoiceStatus()
	if v.Ready || v.Hint == "" {
		t.Fatalf("%+v", v)
	}
	if _, err := s.VoiceStop(); err == nil {
		t.Fatal("stop when not recording must error")
	}
	if err := s.VoiceSetup(false); !errors.Is(err, markdown.ErrNotConfirmed) {
		t.Fatal("setup needs confirmation")
	}
}

func fakeChatAgent(t *testing.T, s *Service, script string) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fakeai"), []byte("#!/bin/sh\n"+script+"\n"), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := s.AddCustomAgent(`{"id":"fakeai","displayName":"Fake AI","executables":["fakeai"],"handoff":"none","capabilities":{"newSession":true},
		"chat":{"newArgs":["--print","{prompt}"],"continueArgs":["--continue","--print","{prompt}"],"editArgs":["--edit"]}}`)
	if err != nil {
		t.Fatal(err)
	}
}

func waitIdle(t *testing.T, s *Service, conv int64) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if !s.ChatStatus(conv).Running {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("chat turn never finished")
}

func TestChatTurnPostsAgentReplyAndContinuesSession(t *testing.T) {
	s := newSvc(t)
	fakeChatAgent(t, s, `last=""; for a in "$@"; do last="$a"; done; echo "reply[$#]: ${last##*$'\n'}"; echo "cwd=$(pwd) ws=$AICW_WORKSPACE" >&2`)
	root := project(t)
	r, _ := s.OpenWorkspace(root, false)
	conv, _ := s.NewConversation(r.Workspace.ID, "", "fakeai")
	res, err := s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "hello agent", Mode: "auto"})
	if err != nil || !res.Running {
		t.Fatalf("%+v %v", res, err)
	}
	waitIdle(t, s, conv.ID)
	msgs, _ := s.Messages(conv.ID, 0)
	if len(msgs) != 2 || msgs[1].Role != "assistant" || !strings.Contains(msgs[1].Body, "hello agent") {
		t.Fatalf("expected user + assistant reply: %+v", msgs)
	}
	// Second message continues the same session (uses continueArgs) and edits stay off.
	s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "and again", Mode: "auto"})
	waitIdle(t, s, conv.ID)
	msgs, _ = s.Messages(conv.ID, 0)
	if len(msgs) != 5 { // user, reply, [continue note], user, reply  (note only if present)
		if len(msgs) != 4 {
			t.Fatalf("%d messages: %+v", len(msgs), msgs)
		}
	}
	last := msgs[len(msgs)-1]
	if last.Role != "assistant" || !strings.Contains(last.Body, "reply[3]") || strings.Contains(last.Body, "--edit") {
		t.Fatalf("second turn should use --continue --print <prompt> without --edit: %+v", last)
	}
	ses, _ := s.Store.Sessions(r.Workspace.ID)
	if len(ses) != 1 || ses[0].Status != storage.StatusPaused {
		t.Fatalf("one reusable session expected: %+v", ses)
	}
	// Edits allowed: flag is passed.
	s.SetChatEdits(true)
	s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "edit please", Mode: "auto"})
	waitIdle(t, s, conv.ID)
	msgs, _ = s.Messages(conv.ID, 0)
	if !strings.Contains(msgs[len(msgs)-1].Body, "reply[4]") {
		t.Fatalf("--edit not passed when allowed: %+v", msgs[len(msgs)-1])
	}
}

func TestChatTurnFailureAndMissingAgentAreVisible(t *testing.T) {
	s := newSvc(t)
	fakeChatAgent(t, s, `echo "model overloaded" >&2; exit 3`)
	r, _ := s.OpenWorkspace(project(t), false)
	conv, _ := s.NewConversation(r.Workspace.ID, "", "fakeai")
	s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "x", Mode: "auto"})
	waitIdle(t, s, conv.ID)
	msgs, _ := s.Messages(conv.ID, 0)
	if last := msgs[len(msgs)-1]; last.Role != "system" || !strings.Contains(last.Body, "model overloaded") || !strings.Contains(last.Body, "failed") {
		t.Fatalf("%+v", last)
	}
	// An agent without chat mode explains itself instead of pretending.
	c2, _ := s.NewConversation(r.Workspace.ID, "", "freebuff")
	res, _ := s.SendMessage(SendOptions{ConversationID: c2.ID, Text: "hi", Mode: "auto"})
	if res.Running || !strings.Contains(res.Note, "no chat mode") {
		t.Fatalf("%+v", res)
	}
	t.Setenv("PATH", t.TempDir())
	c3, _ := s.NewConversation(r.Workspace.ID, "", "claude")
	s.SendMessage(SendOptions{ConversationID: c3.ID, Text: "hi", Mode: "auto"})
	m3, _ := s.Messages(c3.ID, 0)
	if len(m3) != 2 || m3[1].Role != "system" || !strings.Contains(m3[1].Body, "not installed") {
		t.Fatalf("%+v", m3)
	}
}

func TestChatQueueAndStop(t *testing.T) {
	s := newSvc(t)
	fakeChatAgent(t, s, `last=""; for a in "$@"; do last="$a"; done; case "$last" in *SLOW*) sleep 30;; esac; echo "done: $last"`)
	r, _ := s.OpenWorkspace(project(t), false)
	conv, _ := s.NewConversation(r.Workspace.ID, "", "fakeai")
	s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "SLOW one", Mode: "auto"})
	res, _ := s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "second", Mode: "auto"})
	if !strings.Contains(res.Note, "queued") || s.ChatStatus(conv.ID).Queued != 1 {
		t.Fatalf("%+v %+v", res, s.ChatStatus(conv.ID))
	}
	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	s.StopChat(conv.ID)
	waitIdle(t, s, conv.ID)
	if time.Since(start) > 5*time.Second {
		t.Fatal("stop must kill the running process promptly")
	}
	msgs, _ := s.Messages(conv.ID, 0)
	var stopped, ranSecond bool
	for _, m := range msgs {
		stopped = stopped || (m.Role == "system" && m.Body == "Stopped.")
		ranSecond = ranSecond || strings.Contains(m.Body, "done: second")
	}
	if !stopped || ranSecond {
		t.Fatalf("stop should cancel the queue too: %+v", msgs)
	}
}

func TestPagesSaveMessageAndFileBrowser(t *testing.T) {
	s := newSvc(t)
	root := project(t)
	os.MkdirAll(filepath.Join(root, "docs", "deep"), 0o755)
	os.MkdirAll(filepath.Join(root, "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(root, "docs", "deep", "art.png"), []byte("\x89PNG\r\n\x1a\n.."), 0o644)
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(root, "escape"))
	r, _ := s.OpenWorkspace(root, false)
	id := r.Workspace.ID

	top, err := s.ListDir(id, "", false)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range top {
		names = append(names, e.Name)
	}
	if strings.Join(names, ",") != "docs,AGENTS.md,README.md" && !(names[0] == "docs" && !contains(names, "node_modules") && !contains(names, "escape") && !contains(names, ".git")) {
		t.Fatalf("dirs first, noise hidden, escaping symlink skipped: %v", names)
	}
	var env FileEntry
	all, _ := s.ListDir(id, "", true)
	for _, e := range all {
		if e.Name == ".env" {
			env = e
		}
	}
	if !env.Secret {
		t.Fatal(".env should be flagged secret")
	}
	if _, err := s.ListDir(id, "../..", false); err == nil {
		t.Fatal("traversal accepted")
	}
	if _, err := s.ListDir(id, "escape", true); err == nil {
		t.Fatal("symlink escape accepted")
	}
	m, err := s.ImportProjectFile(id, "docs/deep/art.png")
	if err != nil || m.Kind != "image" {
		t.Fatalf("%+v %v", m, err)
	}
	if _, err := s.ImportProjectFile(id, ".env"); err == nil {
		t.Fatal("secret import accepted")
	}

	conv, _ := s.NewConversation(id, "c", "claude")
	msg, _ := s.Store.AddMessage(conv.ID, storage.RoleAssistant, storage.SourceAgent, "Final pig design.", []int64{m.ID})
	pg, err := s.SaveMessageToPage(msg.ID, "Character")
	if err != nil || !strings.Contains(pg.Body, fmt.Sprintf("![art.png](media:%d)", m.ID)) || !strings.Contains(pg.Body, "Final pig design.") {
		t.Fatalf("%+v %v", pg, err)
	}
	pg2, _ := s.SaveMessageToPage(msg.ID, "character")
	if pg2.ID != pg.ID || strings.Count(pg2.Body, "Final pig design.") != 2 {
		t.Fatal("saving again appends to the same page")
	}
	if _, err := s.RenamePage(pg.ID, "  "); err == nil {
		t.Fatal("blank rename accepted")
	}
	s.SavePage(id, "Locations", "", "x")
	if _, err := s.RenamePage(pg.ID, "locations"); err == nil {
		t.Fatal("rename onto an existing title accepted")
	}
	if err := s.DeletePage(pg.ID, false); !errors.Is(err, markdown.ErrNotConfirmed) {
		t.Fatal("delete needs confirmation")
	}
	if v, _ := s.PageVersions(pg.ID); len(v) != 1 {
		t.Fatalf("versions: %+v", v)
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func TestSendModesAndVoiceMessageAndAddToPage(t *testing.T) {
	s := newSvc(t)
	fakeChatAgent(t, s, `echo "chat reply"`)
	r, _ := s.OpenWorkspace(project(t), false)
	id := r.Workspace.ID
	conv, _ := s.NewConversation(id, "", "fakeai")
	if res, _ := s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "just save", Mode: "save"}); res.Running || res.Sent {
		t.Fatalf("%+v", res)
	}
	if res, _ := s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "term only", Mode: "terminal"}); res.Running || !strings.Contains(res.Note, "No agent is running in the terminal") {
		t.Fatalf("terminal mode must not fall back to chat: %+v", res)
	}
	if _, err := s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "x", Mode: "bogus"}); err == nil {
		t.Fatal("unknown mode accepted")
	}
	s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "chat mode", Mode: "chat"})
	waitIdle(t, s, conv.ID)
	msgs, _ := s.Messages(conv.ID, 0)
	if last := msgs[len(msgs)-1]; last.Role != "assistant" || !strings.Contains(last.Body, "chat reply") {
		t.Fatalf("%+v", last)
	}

	// Voice message: audio is kept as media with its transcript.
	t.Setenv("PATH", t.TempDir()+string(os.PathListSeparator)+os.Getenv("PATH"))
	rec := filepath.Join(t.TempDir(), "rec")
	os.WriteFile(rec, []byte("#!/bin/sh\nprintf 'RIFFxxxxWAVE%0300d' 0 > \"$1\"; trap 'exit 0' INT; while true; do sleep 0.05; done\n"), 0o755)
	stt := filepath.Join(t.TempDir(), "stt")
	os.WriteFile(stt, []byte("#!/bin/sh\necho 'add a locations page'\n"), 0o755)
	s.rec.Argv = []string{rec, "{file}"}
	st := s.GetSettings()
	st.VoiceCommand = stt + " {audio}"
	s.SaveSettings(st)
	if err := s.VoiceStart(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	vr, err := s.VoiceStopAttach(id)
	if err != nil || vr.Media == nil || vr.Media.Kind != "audio" || vr.Transcript != "Add a locations page" || !strings.HasPrefix(vr.Media.Name, "voice-") {
		t.Fatalf("%+v %v", vr, err)
	}
	if _, err := os.Stat(vr.Media.Stored); err != nil {
		t.Fatal("audio must be kept")
	}

	// Add data to a page: text only, media only, and both.
	pg, _ := s.SavePage(id, "About", "", "")
	if _, err := s.AddToPage(pg.ID, "  ", nil); err == nil {
		t.Fatal("empty add accepted")
	}
	p2, err := s.AddToPage(pg.ID, "First paragraph.", nil)
	if err != nil || p2.Body != "First paragraph." {
		t.Fatalf("%+v %v", p2, err)
	}
	p3, _ := s.AddToPage(pg.ID, "Caption text", []int64{vr.Media.ID})
	if !strings.Contains(p3.Body, fmt.Sprintf("![%s](media:%d)", vr.Media.Name, vr.Media.ID)) || !strings.Contains(p3.Body, "> Add a locations page") || !strings.HasSuffix(p3.Body, "Caption text") {
		t.Fatalf("%q", p3.Body)
	}
	other, _ := s.OpenWorkspace(project(t), false)
	foreign, _ := s.Store.AddMedia(storage.Media{WorkspaceID: other.Workspace.ID, Name: "x.png", Stored: "/x", Kind: "image", Mime: "image/png", Size: 1})
	p4, _ := s.AddToPage(pg.ID, "only text", []int64{foreign.ID})
	if strings.Contains(p4.Body, fmt.Sprintf("media:%d", foreign.ID)) {
		t.Fatal("media from another workspace must not be embedded")
	}
	if v, _ := s.PageVersions(pg.ID); len(v) != 3 {
		t.Fatalf("each add keeps a version: %d", len(v))
	}
}

func TestTerminalConversationIsMirroredIntoChat(t *testing.T) {
	s := newSvc(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	r, _ := s.OpenWorkspace(project(t), false)
	conv, _ := s.NewConversation(r.Workspace.ID, "", "claude")
	sess, _ := s.WS.StartSession(r.Workspace.ID, "claude", "task")
	ext := "11111111-2222-3333-4444-555555555555"
	s.Store.UpdateSession(sess.ID, storage.SessionPatch{ExternalID: &ext})
	s.Store.UpdateConversation(conv.ID, nil, nil, &sess.ID)

	// A message sent from the chat composer is typed into the terminal and so
	// also lands in the transcript; it must not be shown twice.
	s.SendMessage(SendOptions{ConversationID: conv.ID, Text: "from the composer", Mode: "save"})
	now := time.Now().UTC().Format(time.RFC3339Nano)
	file := filepath.Join(home, ".claude", "projects", "-proj", ext+".jsonl")
	os.MkdirAll(filepath.Dir(file), 0o755)
	write := func(lines ...string) {
		f, _ := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		defer f.Close()
		for _, l := range lines {
			f.WriteString(l + "\n")
		}
	}
	write(
		`{"type":"user","uuid":"u1","entrypoint":"cli","timestamp":"`+now+`","message":{"content":"Read the project context package at /x/context.md before doing anything else. Then wait."}}`,
		`{"type":"user","uuid":"u2","entrypoint":"cli","timestamp":"`+now+`","message":{"content":"from the composer"}}`,
		`{"type":"user","uuid":"u3","entrypoint":"cli","timestamp":"`+now+`","message":{"content":"typed in the terminal"}}`,
		`{"type":"assistant","uuid":"a1","entrypoint":"cli","timestamp":"`+now+`","message":{"content":[{"type":"text","text":"terminal answer"}]}}`,
	)
	bodies := func() []string {
		ms, _ := s.Messages(conv.ID, 0)
		var out []string
		for _, m := range ms {
			out = append(out, m.Role+":"+m.Body)
		}
		return out
	}
	want := "user:from the composer|user:typed in the terminal|assistant:terminal answer"
	if got := strings.Join(bodies(), "|"); got != want {
		t.Fatalf("got %q", got)
	}
	// Idempotent, and later lines are picked up.
	write(`{"type":"assistant","uuid":"a2","entrypoint":"cli","timestamp":"` + now + `","message":{"content":[{"type":"text","text":"more"}]}}`)
	s.mirrorOff[conv.ID] = 0 // as after an app restart: re-reading must not duplicate
	if got := strings.Join(bodies(), "|"); got != want+"|assistant:more" {
		t.Fatalf("got %q", got)
	}
}
