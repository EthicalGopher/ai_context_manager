package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		args []string
		want Command
		err  bool
	}{
		{nil, Command{Kind: KindOpen}, false},
		{[]string{"."}, Command{Kind: KindOpen, Path: "."}, false},
		{[]string{"--new", "."}, Command{Kind: KindOpen, Path: ".", New: true}, false},
		{[]string{"./my project", "--new"}, Command{Kind: KindOpen, Path: "./my project", New: true}, false},
		{[]string{"~/code/p"}, Command{Kind: KindOpen, Path: "~/code/p"}, false},
		{[]string{"--help"}, Command{Kind: KindHelp}, false},
		{[]string{"-v"}, Command{Kind: KindVersion}, false},
		{[]string{"doctor"}, Command{Kind: KindDoctor}, false},
		{[]string{"--", "--weird-dir"}, Command{Kind: KindOpen, Path: "--weird-dir"}, false},
		{[]string{"--page", "media", "."}, Command{Kind: KindOpen, Path: ".", Page: "media"}, false},
		{[]string{".", "--page", "settings"}, Command{Kind: KindOpen, Path: ".", Page: "settings"}, false},
		{[]string{"--page", "terminal", "."}, Command{Kind: KindOpen, Path: ".", Page: "terminal"}, false},
		{[]string{"--page", "nope", "."}, Command{}, true},
		{[]string{"--page"}, Command{}, true},
		{[]string{"--bogus"}, Command{}, true},
		{[]string{"a", "b"}, Command{}, true},
		{[]string{"--new"}, Command{}, true},
	}
	for _, c := range cases {
		got, err := Parse(c.args)
		if (err != nil) != c.err {
			t.Errorf("%v: err=%v", c.args, err)
			continue
		}
		if !c.err && got != c.want {
			t.Errorf("%v: got %+v want %+v", c.args, got, c.want)
		}
	}
}

func TestResolveRequest(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	os.MkdirAll(filepath.Join(root, "my project"), 0o755)
	os.WriteFile(filepath.Join(root, "f.txt"), nil, 0o644)
	if p, err := ResolveRequest("my project", root, nil); err != nil || p != filepath.Join(root, "my project") {
		t.Fatalf("%q %v", p, err)
	}
	if _, err := ResolveRequest("f.txt", root, nil); err == nil {
		t.Fatal("file must be rejected without confirmation")
	}
	if p, err := ResolveRequest("f.txt", root, func(string) bool { return true }); err != nil || p != root {
		t.Fatalf("%q %v", p, err)
	}
	if _, err := ResolveRequest("f.txt", root, func(string) bool { return false }); err == nil {
		t.Fatal("declined prompt must fail")
	}
	if _, err := ResolveRequest("missing", root, nil); err == nil {
		t.Fatal("missing path must fail")
	}
	if _, err := os.Stat(filepath.Join(root, "missing")); err == nil {
		t.Fatal("must not create directories")
	}
}

func TestDoctorRuns(t *testing.T) {
	var buf bytes.Buffer
	Doctor(&buf, "test", t.TempDir())
	out := buf.String()
	for _, want := range []string{"aicw doctor", "data directory writable", "database", "Agent CLIs", "claude", "freebuff"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
