// Package models finds the models an agent CLI can be pointed at: the list the
// CLI itself reports (kilo, agy) and Claude Code's aliases. Models AICW
// downloads itself are added by the service (see package localllm).
package models

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strings"
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

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// Discover lists models for an agent. exe is the agent's resolved executable.
func Discover(ctx context.Context, agent, exe string) Result {
	var r Result
	switch agent {
	case "claude":
		for _, a := range []struct{ id, label string }{{"sonnet", "Sonnet (latest)"}, {"opus", "Opus (latest)"}, {"haiku", "Haiku (latest)"}, {"fable", "Fable (latest)"}} {
			r.Models = append(r.Models, Model{ID: a.id, Label: a.label, Group: "Claude"})
		}
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
