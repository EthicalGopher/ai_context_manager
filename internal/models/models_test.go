package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func fakeCLI(t *testing.T, body string) string {
	p := filepath.Join(t.TempDir(), "cli")
	os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755)
	return p
}

func TestKiloAndAgyListsAreParsed(t *testing.T) {
	kilo := fakeCLI(t, `printf '\033[1m██ banner\033[0m\nkilo/anthropic/claude-opus-5\n\nkilo/x-ai/grok-latest\nnot a model\n'`)
	r := Discover(context.Background(), "kilo", kilo)
	if len(r.Models) != 2 || r.Models[0].ID != "kilo/anthropic/claude-opus-5" || r.Models[1].Group != "x-ai" {
		t.Fatalf("%+v", r)
	}
	agy := fakeCLI(t, `printf 'Fetching available models...\ngemini-3.8-flash-high\tGemini 3.8 Flash (High)\nclaude-sonnet-4-6\tClaude Sonnet 4.6 (Thinking)\n'`)
	r = Discover(context.Background(), "agy", agy)
	if len(r.Models) != 2 || r.Models[0].ID != "gemini-3.8-flash-high" || r.Models[0].Label != "Gemini 3.8 Flash (High)" {
		t.Fatalf("%+v", r)
	}
	if r := Discover(context.Background(), "kilo", fakeCLI(t, "true")); len(r.Models) != 0 || len(r.Notes) == 0 {
		t.Fatalf("an empty list must explain itself: %+v", r)
	}
	if r := Discover(context.Background(), "codex", ""); len(r.Models) != 0 || len(r.Notes) == 0 {
		t.Fatalf("%+v", r)
	}
}
