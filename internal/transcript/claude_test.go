package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadClaudeKeepsOnlyTerminalText(t *testing.T) {
	lines := []string{
		`{"type":"user","uuid":"u1","entrypoint":"cli","timestamp":"2026-01-01T10:00:00Z","message":{"role":"user","content":"hello there"}}`,
		`{"type":"assistant","uuid":"a1","entrypoint":"cli","message":{"content":[{"type":"thinking","thinking":"x"},{"type":"text","text":"Hi! "}]}}`,
		`{"type":"assistant","uuid":"a2","entrypoint":"cli","message":{"content":[{"type":"tool_use","name":"Bash"}]}}`,
		`{"type":"user","uuid":"u2","entrypoint":"cli","message":{"content":[{"type":"tool_result","content":"ok"}]}}`,
		`{"type":"user","uuid":"u3","entrypoint":"cli","message":{"content":"<command-name>/effort</command-name>"}}`,
		`{"type":"user","uuid":"u4","entrypoint":"cli","isMeta":true,"message":{"content":"meta"}}`,
		`{"type":"user","uuid":"u5","entrypoint":"sdk-cli","message":{"content":"from chat reply"}}`,
		`{"type":"user","uuid":"u6","entrypoint":"cli","message":{"content":[{"type":"image"},{"type":"text","text":"look at this"}]}}`,
	}
	p := filepath.Join(t.TempDir(), "s.jsonl")
	os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n{\"type\":\"user\",\"uu"), 0o600) // trailing partial line
	es, off, err := ReadClaude(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, e := range es {
		got = append(got, e.ID+":"+e.Role+":"+e.Text)
	}
	want := []string{"u1:user:hello there", "a1:assistant:Hi!", "u6:user:look at this"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", got)
	}
	// Appending the rest of the partial line is picked up from the saved offset.
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`id":"u7","entrypoint":"cli","message":{"content":"later"}}` + "\n")
	f.Close()
	es, _, _ = ReadClaude(p, off)
	if len(es) != 1 || es[0].ID != "u7" {
		t.Fatalf("%+v", es)
	}
}

func TestFindClaudeRejectsOddIDs(t *testing.T) {
	if FindClaude(t.TempDir(), "../x") != "" || FindClaude(t.TempDir(), "") != "" {
		t.Fatal("must refuse")
	}
}
