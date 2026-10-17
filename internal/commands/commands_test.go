package commands

import (
	"strings"
	"testing"
)

func TestExpand(t *testing.T) {
	p, c, ok := Expand("  /grill-me the checkout flow ")
	if !ok || c.Name != "grill-me" || !strings.HasPrefix(p, Marker+"grill-me]") || !strings.Contains(p, "about: the checkout flow") || strings.Contains(p, "{") {
		t.Fatalf("%v %q", ok, p)
	}
	if p, _, ok := Expand("/write-pages"); !ok || strings.Contains(p, "{focus}") || strings.Contains(p, "Focus on") {
		t.Fatalf("no-arg expansion left placeholders: %q", p)
	}
	for _, s := range []string{"hello", "/unknown x", "/", "see /grill-me", ""} {
		if _, _, ok := Expand(s); ok {
			t.Fatalf("%q must not expand", s)
		}
	}
	for _, c := range All() {
		if c.Name == "" || c.Description == "" || c.Prompt == "" {
			t.Fatalf("incomplete command %+v", c)
		}
		if p, _, _ := Expand("/" + c.Name + " x"); strings.Contains(p, "{") && !strings.Contains(p, "'<json>'") {
			t.Fatalf("/%s leaves a placeholder: %q", c.Name, p)
		}
	}
}
