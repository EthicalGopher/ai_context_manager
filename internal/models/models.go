// Package models finds the models an agent CLI can be pointed at: the list the
// CLI itself reports (kilo, agy), Claude Code's aliases, and models served on
// this machine by a local server. AICW never runs a model; it only discovers
// names and, for local models, tells Claude Code where the server is.
package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Model struct {
	ID    string `json:"id"`    // stored and passed to the CLI (local models carry a "server:" prefix)
	Label string `json:"label"` // shown in the list
	Group string `json:"group"`
}

type Result struct {
	Models []Model  `json:"models"`
	Notes  []string `json:"notes"` // why a list is empty or short
}

// Local servers Claude Code can talk to (they speak the Anthropic Messages API).
type server struct {
	Key, Name, Base, Token string
	list                   func(ctx context.Context, base string) ([]string, error)
}

var servers = []server{
	{"ollama", "Ollama", "http://localhost:11434", "ollama", ollamaTags},
	{"lmstudio", "LM Studio", "http://localhost:1234", "lmstudio", openAIModels},
	{"llamacpp", "llama.cpp server", "http://localhost:8080", "llamacpp", openAIModels},
}

// LocalEnv reports whether id is a local model ("ollama:llama3.2:latest") and,
// if so, the name to pass to the CLI and the environment that points Claude
// Code at that server.
func LocalEnv(id string) (name string, env []string, ok bool) {
	for _, s := range servers {
		if rest, found := strings.CutPrefix(id, s.Key+":"); found && rest != "" {
			return rest, []string{"ANTHROPIC_BASE_URL=" + s.Base, "ANTHROPIC_AUTH_TOKEN=" + s.Token, "ANTHROPIC_API_KEY="}, true
		}
	}
	return id, nil, false
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// Discover lists models for an agent. exe is the agent's resolved executable.
func Discover(ctx context.Context, agent, exe string) Result {
	var r Result
	switch agent {
	case "claude":
		for _, a := range []struct{ id, label string }{{"sonnet", "Sonnet (latest)"}, {"opus", "Opus (latest)"}, {"haiku", "Haiku (latest)"}, {"fable", "Fable (latest)"}} {
			r.Models = append(r.Models, Model{ID: a.id, Label: a.label, Group: "Claude"})
		}
		r.add(localModels(ctx))
	case "kilo":
		out := run(ctx, exe, "models")
		for _, l := range lines(out) {
			if !strings.ContainsAny(l, " \t") && strings.Contains(l, "/") {
				r.Models = append(r.Models, Model{ID: l, Label: l, Group: groupOf(l)})
			}
		}
		if len(r.Models) == 0 {
			r.Notes = append(r.Notes, "Kilo reported no models. Run `kilo auth` to add a provider, then reopen this list.")
		}
	case "agy":
		for _, l := range lines(run(ctx, exe, "models")) {
			if id, label, ok := strings.Cut(l, "\t"); ok && id != "" && !strings.ContainsAny(id, " ") {
				r.Models = append(r.Models, Model{ID: id, Label: strings.TrimSpace(label), Group: "AGY"})
			}
		}
		if len(r.Models) == 0 {
			r.Notes = append(r.Notes, "AGY reported no models (is it signed in?). You can still type a model name.")
		}
	default:
		r.Notes = append(r.Notes, "This agent does not publish a model list. Type a model name.")
	}
	return r
}

func (r *Result) add(o Result) {
	r.Models, r.Notes = append(r.Models, o.Models...), append(r.Notes, o.Notes...)
}

// groupOf names the vendor of a "kilo/<vendor>/<model>" id; Kilo's moving
// "~vendor/…-latest" aliases get their own group.
func groupOf(id string) string {
	parts := strings.Split(id, "/")
	if len(parts) >= 3 {
		if strings.HasPrefix(parts[1], "~") {
			return "Latest aliases"
		}
		return parts[1]
	}
	return parts[0]
}

func lines(b []byte) []string {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader([]byte(ansi.ReplaceAllString(string(b), ""))))
	for sc.Scan() {
		if l := strings.TrimRight(sc.Text(), " \r"); strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func run(ctx context.Context, exe string, args ...string) []byte {
	if exe == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, exe, args...).Output()
	return out
}

// localModels asks each known local server what it is serving, in parallel.
func localModels(ctx context.Context) Result {
	type res struct {
		s     server
		names []string
		err   error
	}
	ch := make([]res, len(servers))
	var wg sync.WaitGroup
	for i, s := range servers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
			defer cancel()
			n, err := s.list(c, s.Base)
			ch[i] = res{s, n, err}
		}()
	}
	wg.Wait()
	var r Result
	for _, x := range ch {
		for _, n := range x.names {
			r.Models = append(r.Models, Model{ID: x.s.Key + ":" + n, Label: n, Group: "Local · " + x.s.Name})
		}
	}
	if len(r.Models) == 0 {
		if ollamaInstalled() {
			r.Notes = append(r.Notes, "Ollama has models on this machine but its server isn't running. Start it (`ollama serve`) and they will appear under Local.")
		} else {
			r.Notes = append(r.Notes, "No local model server found (Ollama, LM Studio or a llama.cpp server). Start one and local models will appear under Local.")
		}
	}
	return r
}

func ollamaInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	m, _ := filepath.Glob(filepath.Join(home, ".ollama", "models", "manifests", "*", "*", "*", "*"))
	return len(m) > 0
}

func getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func ollamaTags(ctx context.Context, base string) ([]string, error) {
	var t struct {
		Models []struct{ Name string } `json:"models"`
	}
	if err := getJSON(ctx, base+"/api/tags", &t); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range t.Models {
		out = append(out, m.Name)
	}
	return out, nil
}

func openAIModels(ctx context.Context, base string) ([]string, error) {
	var t struct {
		Data []struct{ ID string } `json:"data"`
	}
	if err := getJSON(ctx, base+"/v1/models", &t); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range t.Data {
		out = append(out, m.ID)
	}
	return out, nil
}
