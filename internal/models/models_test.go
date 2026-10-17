package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestLocalServerModelsAndEnv(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Write([]byte(`{"models":[{"name":"llama3.2:latest"},{"name":"qwen3-coder:30b"}]}`))
			return
		}
		w.Write([]byte(`{"data":[{"id":"my-gguf"}]}`))
	}))
	defer srv.Close()
	old := servers
	defer func() { servers = old }()
	servers = []server{{"ollama", "Ollama", srv.URL, "ollama", ollamaTags}, {"lmstudio", "LM Studio", "http://127.0.0.1:1", "x", openAIModels}, {"llamacpp", "llama.cpp server", srv.URL, "llamacpp", openAIModels}}
	var ids []string
	for _, m := range Discover(context.Background(), "claude", "").Models {
		if strings.HasPrefix(m.Group, "Local") {
			ids = append(ids, m.ID)
		}
	}
	if !reflect.DeepEqual(ids, []string{"ollama:llama3.2:latest", "ollama:qwen3-coder:30b", "llamacpp:my-gguf"}) {
		t.Fatalf("%v", ids)
	}
	name, env, ok := LocalEnv("ollama:qwen3-coder:30b")
	if !ok || name != "qwen3-coder:30b" || env[0] != "ANTHROPIC_BASE_URL="+srv.URL || env[2] != "ANTHROPIC_API_KEY=" {
		t.Fatalf("%q %v %v", name, env, ok)
	}
	if n, e, ok := LocalEnv("opus"); ok || n != "opus" || e != nil {
		t.Fatal("hosted models must be left alone")
	}
	if _, _, ok := LocalEnv("ollama:"); ok {
		t.Fatal("empty local name")
	}
}
