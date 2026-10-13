// Package markdown implements safe, confirmed, backed-up file mutation plus
// diffing and generated-file headers.
package markdown

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aicw/internal/safety"
)

var (
	ErrNotConfirmed = errors.New("write requires explicit confirmation")
	ErrStale        = errors.New("file changed since the preview was generated")
)

// Preview describes what an Apply would do.
type Preview struct {
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	Changed  bool   `json:"changed"`
	Diff     string `json:"diff"`
	BaseHash string `json:"baseHash"` // pass back to Apply to detect concurrent edits
}

// AuditEntry records one mutation made by AICW.
type AuditEntry struct {
	Time    time.Time `json:"time"`
	Action  string    `json:"action"` // write | create | restore
	Path    string    `json:"path"`
	Backup  string    `json:"backup,omitempty"`
	NewHash string    `json:"newHash"`
}

// Writer mutates files under Root; backups and the audit log go to HistoryDir.
type Writer struct {
	Root       string
	HistoryDir string
	Now        func() time.Time
}

func (w *Writer) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

func hash(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:8])
}

// Preview computes a diff without touching disk.
func (w *Writer) Preview(rel, content string) (*Preview, error) {
	full, err := safety.SafeJoin(w.Root, rel)
	if err != nil {
		return nil, err
	}
	old, err := os.ReadFile(full)
	p := &Preview{Path: rel}
	switch {
	case err == nil:
		p.Exists = true
		p.BaseHash = hash(old)
	case os.IsNotExist(err):
	default:
		return nil, err
	}
	p.Changed = !p.Exists || string(old) != content
	p.Diff = Diff(string(old), content, rel)
	return p, nil
}

// Apply writes content to rel. confirmed must be true. If baseHash is
// non-empty it must match the current file (guards against stale previews).
// Existing files are backed up first.
func (w *Writer) Apply(rel, content string, confirmed bool, baseHash string) (*AuditEntry, error) {
	if !confirmed {
		return nil, ErrNotConfirmed
	}
	full, err := safety.SafeJoin(w.Root, rel)
	if err != nil {
		return nil, err
	}
	entry := &AuditEntry{Time: w.now(), Action: "create", Path: rel, NewHash: hash([]byte(content))}
	old, err := os.ReadFile(full)
	if err == nil {
		if baseHash != "" && baseHash != hash(old) {
			return nil, ErrStale
		}
		entry.Action = "write"
		if entry.Backup, err = w.backup(rel, old); err != nil {
			return nil, fmt.Errorf("backup failed, nothing written: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := atomicWrite(full, []byte(content)); err != nil {
		return nil, err
	}
	return entry, w.audit(entry)
}

func (w *Writer) backup(rel string, data []byte) (string, error) {
	dir := filepath.Join(w.HistoryDir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s__%s.bak", w.now().UTC().Format("20060102T150405.000000000Z"), strings.ReplaceAll(filepath.ToSlash(rel), "/", "%2F"))
	return name, os.WriteFile(filepath.Join(dir, name), data, 0o600)
}

// Backups lists backup names for rel, newest first.
func (w *Writer) Backups(rel string) []string {
	suffix := "__" + strings.ReplaceAll(filepath.ToSlash(rel), "/", "%2F") + ".bak"
	ents, _ := os.ReadDir(filepath.Join(w.HistoryDir, "backups"))
	var out []string
	for i := len(ents) - 1; i >= 0; i-- {
		if strings.HasSuffix(ents[i].Name(), suffix) {
			out = append(out, ents[i].Name())
		}
	}
	return out
}

// Restore puts a backup back (after backing up the current version).
func (w *Writer) Restore(rel, backup string, confirmed bool) (*AuditEntry, error) {
	if !confirmed {
		return nil, ErrNotConfirmed
	}
	if strings.ContainsAny(backup, "/\\") || strings.Contains(backup, "..") {
		return nil, safety.ErrEscape
	}
	data, err := os.ReadFile(filepath.Join(w.HistoryDir, "backups", backup))
	if err != nil {
		return nil, err
	}
	e, err := w.Apply(rel, string(data), true, "")
	if err != nil {
		return nil, err
	}
	e.Action = "restore"
	return e, nil
}

func (w *Writer) audit(e *AuditEntry) error {
	if err := os.MkdirAll(w.HistoryDir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(w.HistoryDir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	_, err = f.Write(append(b, '\n'))
	return err
}

// AuditLog returns all recorded mutations, oldest first.
func (w *Writer) AuditLog() []AuditEntry {
	b, err := os.ReadFile(filepath.Join(w.HistoryDir, "audit.jsonl"))
	if err != nil {
		return nil
	}
	var out []AuditEntry
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var e AuditEntry
		if json.Unmarshal([]byte(l), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

func atomicWrite(full string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), ".aicw-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(full); err == nil {
		mode = st.Mode().Perm()
	}
	os.Chmod(tmp.Name(), mode)
	return os.Rename(tmp.Name(), full)
}

// GeneratedHeader marks content as generated, with a timestamp.
func GeneratedHeader(t time.Time, what string) string {
	return fmt.Sprintf("<!-- Generated by AI Context Workspace (%s) at %s. Generated summaries may be stale: verify against the repository. -->\n\n", what, t.UTC().Format(time.RFC3339))
}

// Diff returns a unified-style line diff (LCS based). Empty if identical.
func Diff(a, b, name string) string {
	if a == b {
		return ""
	}
	al, bl := splitLines(a), splitLines(b)
	n, m := len(al), len(bl)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if al[i] == bl[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n+++ b/%s\n", name, name)
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && al[i] == bl[j]:
			sb.WriteString(" " + al[i] + "\n")
			i++
			j++
		case i < n && (j >= m || lcs[i+1][j] >= lcs[i][j+1]):
			sb.WriteString("-" + al[i] + "\n")
			i++
		default:
			sb.WriteString("+" + bl[j] + "\n")
			j++
		}
	}
	return sb.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// Move relocates a file inside the workspace (used for archiving). The
// destination must not exist. A backup copy is stored first, and the move is
// audited so it can be undone by moving back.
func (w *Writer) Move(from, to string, confirmed bool) (*AuditEntry, error) {
	if !confirmed {
		return nil, ErrNotConfirmed
	}
	src, err := safety.SafeJoin(w.Root, from)
	if err != nil {
		return nil, err
	}
	dst, err := safety.SafeJoin(w.Root, to)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(dst); err == nil {
		return nil, fmt.Errorf("destination already exists: %s", to)
	}
	entry := &AuditEntry{Time: w.now(), Action: "move", Path: from + " -> " + to, NewHash: hash(data)}
	if entry.Backup, err = w.backup(from, data); err != nil {
		return nil, fmt.Errorf("backup failed, nothing moved: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(src, dst); err != nil {
		return nil, err
	}
	return entry, w.audit(entry)
}
