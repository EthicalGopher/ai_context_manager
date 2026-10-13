// Package history generates session handoff notes from verified inputs.
package history

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"aicw/internal/markdown"
)

type TestRun struct {
	Command string `json:"command"`
	Result  string `json:"result"` // as actually observed; AICW does not run tests
}

type HandoffInput struct {
	Task         string    `json:"task"`
	Agent        string    `json:"agent"`
	SessionID    string    `json:"sessionId"`
	Progress     string    `json:"progress"`
	FilesChanged []string  `json:"filesChanged"`
	Tests        []TestRun `json:"tests"`
	Issues       string    `json:"issues"`
	NextSteps    string    `json:"nextSteps"`
	Now          time.Time `json:"-"`
}

// Handoff renders the note. Anything the user did not supply is marked
// "not recorded" rather than invented.
func Handoff(in HandoffInput) string {
	if in.Now.IsZero() {
		in.Now = time.Now()
	}
	var b strings.Builder
	b.WriteString(markdown.GeneratedHeader(in.Now, "session handoff"))
	fmt.Fprintf(&b, "# Session Handoff: %s\n\n", or(in.Task, "untitled"))
	fmt.Fprintf(&b, "- Date: %s\n- Agent: %s\n", in.Now.UTC().Format("2006-01-02 15:04 UTC"), or(in.Agent, "unknown"))
	if in.SessionID != "" {
		fmt.Fprintf(&b, "- External session ID: `%s`\n", in.SessionID)
	}
	b.WriteString("\n> The next session must verify these notes against the repository before relying on them.\n\n")
	b.WriteString("## Verified progress\n\n" + or(in.Progress, "not recorded") + "\n\n")
	b.WriteString("## Files changed\n\n")
	if len(in.FilesChanged) == 0 {
		b.WriteString("not recorded\n")
	}
	for _, f := range in.FilesChanged {
		fmt.Fprintf(&b, "- `%s`\n", f)
	}
	b.WriteString("\n## Tests actually run\n\n")
	if len(in.Tests) == 0 {
		b.WriteString("No tests recorded. Do not assume tests pass.\n")
	}
	for _, t := range in.Tests {
		fmt.Fprintf(&b, "- `%s` → %s\n", t.Command, or(t.Result, "result not recorded"))
	}
	b.WriteString("\n## Unresolved issues\n\n" + or(in.Issues, "none recorded") + "\n\n")
	b.WriteString("## Next steps\n\n" + or(in.NextSteps, "not recorded") + "\n")
	return b.String()
}

func or(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return strings.TrimSpace(s)
}

// GitChangedFiles lists changed paths via `git status --porcelain` (read-only).
func GitChangedFiles(root string) []string {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if len(l) > 3 {
			files = append(files, strings.Trim(l[3:], `"`))
		}
	}
	return files
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// FileName returns YYYY-MM-DD-slug.md.
func FileName(t time.Time, title string) string {
	s := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if s == "" {
		s = "session"
	}
	if len(s) > 48 {
		s = strings.Trim(s[:48], "-")
	}
	return t.Format("2006-01-02") + "-" + s + ".md"
}
