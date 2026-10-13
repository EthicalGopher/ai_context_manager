package markdown

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aicw/internal/safety"
)

func newWriter(t *testing.T) (*Writer, string) {
	root := t.TempDir()
	tick := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &Writer{Root: root, HistoryDir: t.TempDir(), Now: func() time.Time { tick = tick.Add(time.Second); return tick }}, root
}

func TestApplyRequiresConfirmationAndNeverTouchesFile(t *testing.T) {
	w, root := newWriter(t)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("orig\n"), 0o644)
	if _, err := w.Apply("AGENTS.md", "new\n", false, ""); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("got %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(b) != "orig\n" {
		t.Fatal("file modified without confirmation")
	}
}

func TestBackupRestoreAndAudit(t *testing.T) {
	w, root := newWriter(t)
	p := filepath.Join(root, "AGENTS.md")
	os.WriteFile(p, []byte("v1\n"), 0o644)
	e, err := w.Apply("AGENTS.md", "v2\n", true, "")
	if err != nil || e.Backup == "" {
		t.Fatalf("apply: %+v %v", e, err)
	}
	if b, _ := os.ReadFile(p); string(b) != "v2\n" {
		t.Fatal("not written")
	}
	bk := w.Backups("AGENTS.md")
	if len(bk) != 1 {
		t.Fatalf("backups = %v", bk)
	}
	if _, err := w.Restore("AGENTS.md", bk[0], true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "v1\n" {
		t.Fatalf("restore got %q", b)
	}
	if n := len(w.AuditLog()); n != 2 {
		t.Fatalf("audit entries = %d", n)
	}
	if len(w.Backups("AGENTS.md")) != 2 {
		t.Fatal("restore must back up the replaced version too")
	}
}

func TestStalePreviewAndTraversal(t *testing.T) {
	w, root := newWriter(t)
	os.WriteFile(filepath.Join(root, "A.md"), []byte("one\n"), 0o644)
	pv, _ := w.Preview("A.md", "two\n")
	os.WriteFile(filepath.Join(root, "A.md"), []byte("changed elsewhere\n"), 0o644)
	if _, err := w.Apply("A.md", "two\n", true, pv.BaseHash); !errors.Is(err, ErrStale) {
		t.Fatalf("want ErrStale, got %v", err)
	}
	if _, err := w.Apply("../evil.md", "x", true, ""); !errors.Is(err, safety.ErrEscape) {
		t.Fatalf("want escape, got %v", err)
	}
	if _, err := w.Restore("A.md", "../../etc/passwd", true); err == nil {
		t.Fatal("backup name traversal must fail")
	}
}

func TestCreateNewFileAndPreview(t *testing.T) {
	w, root := newWriter(t)
	pv, _ := w.Preview("docs/new.md", "hello\n")
	if pv.Exists || !pv.Changed || !strings.Contains(pv.Diff, "+hello") {
		t.Fatalf("%+v", pv)
	}
	if _, err := os.Stat(filepath.Join(root, "docs")); err == nil {
		t.Fatal("preview created files")
	}
	e, err := w.Apply("docs/new.md", "hello\n", true, "")
	if err != nil || e.Action != "create" || e.Backup != "" {
		t.Fatalf("%+v %v", e, err)
	}
}

func TestDiff(t *testing.T) {
	d := Diff("a\nb\nc\n", "a\nB\nc\n", "x.md")
	for _, want := range []string{"-b", "+B", " a", " c"} {
		if !strings.Contains(d, want) {
			t.Errorf("diff missing %q:\n%s", want, d)
		}
	}
	if Diff("same\n", "same\n", "x") != "" {
		t.Error("identical must be empty")
	}
}

func TestMoveRequiresConfirmationAndIsUndoable(t *testing.T) {
	w, root := newWriter(t)
	os.WriteFile(filepath.Join(root, "old.md"), []byte("notes\n"), 0o644)
	if _, err := w.Move("old.md", ".aicw/archive/old.md", false); !errors.Is(err, ErrNotConfirmed) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "old.md")); err != nil {
		t.Fatal("moved without confirmation")
	}
	if _, err := w.Move("old.md", ".aicw/archive/old.md", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "old.md")); err == nil {
		t.Fatal("source should be gone")
	}
	if _, err := w.Move(".aicw/archive/old.md", "old.md", true); err != nil {
		t.Fatal("undo failed:", err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "old.md")); string(b) != "notes\n" {
		t.Fatal("content lost")
	}
	os.WriteFile(filepath.Join(root, "x.md"), []byte("1"), 0o644)
	os.WriteFile(filepath.Join(root, "y.md"), []byte("2"), 0o644)
	if _, err := w.Move("x.md", "y.md", true); err == nil {
		t.Fatal("must not overwrite destination")
	}
	if _, err := w.Move("x.md", "../out.md", true); err == nil {
		t.Fatal("must not escape")
	}
}
