package rules

import (
	"strings"
	"testing"

	"aicw/internal/storage"
)

var sk = []storage.Skill{
	{Name: "read-first", Description: "d1", Body: "READ FIRST BODY", AlwaysOn: true},
	{Name: "optional", Description: "opt", Body: "OPTIONAL BODY", AlwaysOn: false},
}

func TestBodyForcesReadAndUpdateAndInlinesOnlyAlwaysOn(t *testing.T) {
	b := Body(sk)
	for _, want := range []string{"Read context first", "read_context", "Keep context current", "write_note", "READ FIRST BODY", "read_skill", "add_message", "aicw tool"} {
		if !strings.Contains(b, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(b, "OPTIONAL BODY") || !strings.Contains(b, "**optional**: opt") {
		t.Error("non-always-on skills should be listed, not inlined")
	}
}

func TestApplyReplacesOnlyManagedBlockAndIsIdempotent(t *testing.T) {
	orig := "# My rules\n\nUse tabs.\n"
	once := Apply(orig, Block(sk))
	if !strings.HasPrefix(once, orig) || !strings.Contains(once, Begin) {
		t.Fatal(once)
	}
	if twice := Apply(once, Block(sk)); twice != once {
		t.Fatalf("not idempotent:\n%s\n---\n%s", once, twice)
	}
	changed := Apply(once, Block([]storage.Skill{{Name: "new", Body: "NEW BODY", AlwaysOn: true}}))
	if !strings.Contains(changed, "NEW BODY") || strings.Contains(changed, "READ FIRST BODY") || !strings.HasPrefix(changed, orig) {
		t.Fatal(changed)
	}
	withTail := Apply(once+"\n## After\nkeep me\n", Block(sk))
	if !strings.Contains(withTail, "keep me") {
		t.Fatal("content after the block must be preserved")
	}
	if Apply("", Block(sk)) != Block(sk) {
		t.Fatal("empty file")
	}
	if Extract(once) == "" || Extract(orig) != "" {
		t.Fatal("extract")
	}
}

func TestPromptIsCapped(t *testing.T) {
	big := []storage.Skill{{Name: "big", Body: strings.Repeat("x", 50000), AlwaysOn: true}}
	if p := Prompt(big); len(p) > MaxPrompt+200 || !strings.Contains(p, "truncated") {
		t.Fatalf("len=%d", len(p))
	}
}

func TestStartersValid(t *testing.T) {
	if len(Starters()) < 2 {
		t.Fatal("expected starters")
	}
	for _, s := range Starters() {
		if s.Name == "" || s.Body == "" {
			t.Fatal(s)
		}
	}
}
