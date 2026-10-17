package agents

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fakeExe(t *testing.T, name, out string) string {
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	os.WriteFile(p, []byte("#!/bin/sh\necho '"+out+"'\n"), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return p
}

func TestDetect(t *testing.T) {
	r := NewRegistry()
	p := fakeExe(t, "claude", "claude 9.9.9")
	st := r.Detect(context.Background(), "claude")
	if st.State != StatusDetected || st.Path != p || !strings.Contains(st.Version, "9.9.9") {
		t.Fatalf("%+v", st)
	}
	t.Setenv("PATH", t.TempDir())
	if st := r.Detect(context.Background(), "claude"); st.State != StatusNotFound {
		t.Fatalf("%+v", st)
	}
	r.Paths["claude"] = "/does/not/exist"
	if st := r.Detect(context.Background(), "claude"); st.State != StatusNeedsConfig {
		t.Fatalf("%+v", st)
	}
	if st := r.Detect(context.Background(), "nope"); st.State != StatusUnknown {
		t.Fatalf("%+v", st)
	}
}

func TestBuildLaunchClaudeNewAndResume(t *testing.T) {
	r := NewRegistry()
	p, _ := r.Get("claude")
	spec, err := BuildLaunch(p, LaunchRequest{Mode: ModeNew, Cwd: "/tmp/my project", SessionID: "uuid-1", ContextFile: "/tmp/ctx dir/context.md", Prompt: strings.ReplaceAll(ContextPrompt, "{contextFile}", "/tmp/ctx dir/context.md"), Exe: "/usr/bin/claude"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--session-id", "uuid-1", strings.ReplaceAll(ContextPrompt, "{contextFile}", "/tmp/ctx dir/context.md")}
	if !reflect.DeepEqual(spec.Args, want) || spec.ExternalID != "uuid-1" || spec.Dir != "/tmp/my project" {
		t.Fatalf("%+v", spec)
	}
	spec, err = BuildLaunch(p, LaunchRequest{Mode: ModeResume, Cwd: "/tmp/x", SessionID: "uuid-1", Exe: "/usr/bin/claude"})
	if err != nil || !reflect.DeepEqual(spec.Args, []string{"--resume", "uuid-1"}) {
		t.Fatalf("%+v %v", spec, err)
	}
}

func TestShellMetacharactersStayOneArg(t *testing.T) {
	r := NewRegistry()
	p, _ := r.Get("kilo")
	evil := `/tmp/x"; rm -rf ~; echo "$(id)/c.md`
	spec, err := BuildLaunch(p, LaunchRequest{Mode: ModeNew, Cwd: "/tmp", ContextFile: evil, Exe: "/usr/bin/kilo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Args) != 2 || spec.Args[0] != "--prompt" || !strings.Contains(spec.Args[1], evil) {
		t.Fatalf("path must stay inside a single verbatim argv element: %q", spec.Args)
	}
}

func TestFreebuffFallsBackToFileHandoff(t *testing.T) {
	r := NewRegistry()
	p, _ := r.Get("freebuff")
	spec, err := BuildLaunch(p, LaunchRequest{Mode: ModeNew, Cwd: "/tmp", ContextFile: "/tmp/c.md", Exe: "/usr/bin/freebuff"})
	if err != nil || spec.Handoff != HandoffFile || len(spec.Args) != 0 {
		t.Fatalf("%+v %v", spec, err)
	}
	if !strings.Contains(strings.Join(spec.Notes, " "), "/tmp/c.md") {
		t.Fatal("note should name the context file")
	}
}

func TestResumeWithoutMechanismIsRefused(t *testing.T) {
	r := NewRegistry()
	p, _ := r.Get("gemini")
	if _, err := BuildLaunch(p, LaunchRequest{Mode: ModeResume, Cwd: "/tmp", SessionID: "x", Exe: "/bin/gemini"}); err == nil {
		t.Fatal("gemini resume is unverified and must be refused")
	}
}

func TestCustomProfileValidation(t *testing.T) {
	r := NewRegistry()
	good := Profile{ID: "my-agent", DisplayName: "Mine", Executables: []string{"mine"}, Handoff: HandoffFile, Caps: Capabilities{NewSession: true}, Verified: true}
	if err := r.Add(good); err != nil {
		t.Fatal(err)
	}
	if p, _ := r.Get("my-agent"); p.Verified || p.Builtin {
		t.Fatal("imported profiles must never be marked verified/builtin")
	}
	bad := []Profile{
		{ID: "Bad ID", DisplayName: "x", Executables: []string{"x"}, Handoff: HandoffFile},
		{ID: "ok", DisplayName: "x", Executables: nil, Handoff: HandoffFile},
		{ID: "ok", DisplayName: "x", Executables: []string{"x"}, Handoff: "weird"},
		{ID: "ok", DisplayName: "x", Executables: []string{"x"}, Handoff: HandoffFile, NewArgs: []string{"{bogus}"}},
		{ID: "ok", DisplayName: "x", Executables: []string{"x"}, Handoff: HandoffFile, EnvNames: []string{"lower=secret"}},
		{ID: "ok", DisplayName: "x", Executables: []string{"x"}, Handoff: HandoffPromptArg},
		{ID: "claude", DisplayName: "x", Executables: []string{"x"}, Handoff: HandoffFile},
	}
	for i, b := range bad {
		if err := r.Add(b); err == nil {
			t.Errorf("bad profile %d accepted", i)
		}
	}
	r.Remove("claude")
	if _, ok := r.Get("claude"); !ok {
		t.Fatal("built-ins cannot be removed")
	}
}

func TestBuiltinsValid(t *testing.T) {
	for _, p := range Builtins() {
		if err := p.Validate(); err != nil {
			t.Errorf("%s: %v", p.ID, err)
		}
	}
}

func TestBuildChatClaudeFirstAndLaterTurns(t *testing.T) {
	r := NewRegistry()
	p, _ := r.Get("claude")
	l, err := BuildChat(p, ChatRequest{Cwd: "/p", Exe: "/usr/bin/claude", Prompt: "-hello; rm -rf ~", FirstTurn: true, NewID: "u-1", Extra: []string{"--mcp-config", "/c.json"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l.Args, []string{"-p", "--session-id", "u-1", "--mcp-config", "/c.json"}) || l.Stdin != "-hello; rm -rf ~" || l.ExternalID != "u-1" {
		t.Fatalf("%+v", l)
	}
	l, _ = BuildChat(p, ChatRequest{Cwd: "/p", Exe: "x", Prompt: "next", SessionID: "u-1", Edits: true})
	if !reflect.DeepEqual(l.Args, []string{"-p", "--resume", "u-1", "--permission-mode", "acceptEdits"}) {
		t.Fatalf("%q", l.Args)
	}
	l, _ = BuildChat(p, ChatRequest{Cwd: "/p", Exe: "x", Prompt: "next", SessionID: "u-1"})
	for _, a := range l.Args {
		if a == "acceptEdits" {
			t.Fatal("edits must be opt-in")
		}
	}
}

func TestBuildChatKiloContinueAndDashPrompt(t *testing.T) {
	r := NewRegistry()
	p, _ := r.Get("kilo")
	l, _ := BuildChat(p, ChatRequest{Cwd: "/p", Exe: "k", Prompt: "--evil", FirstTurn: true})
	if !reflect.DeepEqual(l.Args, []string{"run", "Message: --evil"}) {
		t.Fatalf("%q", l.Args)
	}
	l, _ = BuildChat(p, ChatRequest{Cwd: "/p", Exe: "k", Prompt: "again", Edits: true})
	if !reflect.DeepEqual(l.Args, []string{"run", "--continue", "again", "--auto"}) || len(l.Notes) == 0 {
		t.Fatalf("%+v", l)
	}
}

func TestBuildChatUnsupportedAndCustomValidation(t *testing.T) {
	r := NewRegistry()
	p, _ := r.Get("freebuff")
	if _, err := BuildChat(p, ChatRequest{Cwd: "/p", Exe: "x", Prompt: "hi", FirstTurn: true}); err == nil {
		t.Fatal("freebuff has no chat mode")
	}
	bad := Profile{ID: "ok", DisplayName: "x", Executables: []string{"x"}, Handoff: HandoffFile, Chat: &ChatSpec{NewArgs: []string{"{cwd}"}}}
	if bad.Validate() == nil {
		t.Fatal("unknown chat placeholder accepted")
	}
	bad.Chat = &ChatSpec{PromptVia: "pipe"}
	if bad.Validate() == nil {
		t.Fatal("bad promptVia accepted")
	}
}

func TestModelOptionIsPassedAndValidated(t *testing.T) {
	r := NewRegistry()
	p, _ := r.Get("claude")
	l, err := BuildChat(p, ChatRequest{Cwd: "/p", Exe: "c", Prompt: "hi", FirstTurn: true, NewID: "id", Model: "opus[1m]"})
	if err != nil || !reflect.DeepEqual(l.Args[len(l.Args)-2:], []string{"--model", "opus[1m]"}) {
		t.Fatalf("%v %q", err, l.Args)
	}
	spec, err := BuildLaunch(p, LaunchRequest{Mode: ModeNew, Cwd: "/p", Exe: "c", SessionID: "s", Model: "sonnet"})
	if err != nil || !strings.Contains(strings.Join(spec.Args, " "), "--model sonnet") {
		t.Fatalf("%v %q", err, spec.Args)
	}
	k, _ := r.Get("kilo")
	l, _ = BuildChat(k, ChatRequest{Cwd: "/p", Exe: "k", Prompt: "hi", FirstTurn: true, Model: "ollama/qwen2.5-coder:7b"})
	if !strings.Contains(strings.Join(l.Args, " "), "--model ollama/qwen2.5-coder:7b") {
		t.Fatalf("%q", l.Args)
	}
	for _, bad := range []string{"--evil", "a b", "x;y", "-m"} {
		if _, err := BuildChat(p, ChatRequest{Cwd: "/p", Exe: "c", Prompt: "hi", FirstTurn: true, Model: bad}); err == nil {
			t.Fatalf("%q must be refused", bad)
		}
	}
	f, _ := r.Get("freebuff")
	if _, err := BuildLaunch(f, LaunchRequest{Mode: ModeNew, Cwd: "/p", Exe: "f", Model: "x"}); err == nil {
		t.Fatal("an agent without a model option must say so")
	}
}
