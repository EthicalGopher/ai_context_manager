// Package transcript reads the conversation an agent CLI keeps on disk, so
// what is typed and answered in the embedded terminal can also appear in the
// chat view. Only Claude Code's session files are understood.
package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Entry is one visible message from a terminal session.
type Entry struct {
	ID   string // stable per message, for de-duplication
	Role string // "user" or "assistant"
	Text string
	At   time.Time
}

// FindClaude locates ~/.claude/projects/*/<sessionID>.jsonl, or "".
func FindClaude(home, sessionID string) string {
	if sessionID == "" || strings.ContainsAny(sessionID, `/\*?[`) {
		return ""
	}
	m, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", sessionID+".jsonl"))
	if len(m) == 0 {
		return ""
	}
	return m[0]
}

type line struct {
	Type        string `json:"type"`
	UUID        string `json:"uuid"`
	Timestamp   string `json:"timestamp"`
	Entrypoint  string `json:"entrypoint"`
	IsSidechain bool   `json:"isSidechain"`
	IsMeta      bool   `json:"isMeta"`
	Message     struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// ReadClaude returns the interactive-terminal messages in the session file
// after byte offset. Non-interactive runs (the chat-reply mode, entrypoint
// other than "cli"), tool calls, thinking and slash-command noise are left
// out. The returned offset covers only complete lines, so a half-written
// line is picked up next time.
func ReadClaude(path string, offset int64) ([]Entry, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() < offset {
		offset = 0 // file was replaced
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}
	var out []Entry
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		b, err := r.ReadBytes('\n')
		if err != nil { // EOF: ignore a trailing partial line
			break
		}
		offset += int64(len(b))
		if e, ok := parseClaude(b); ok {
			out = append(out, e)
		}
	}
	return out, offset, nil
}

func parseClaude(b []byte) (Entry, bool) {
	var l line
	if json.Unmarshal(b, &l) != nil || l.UUID == "" || l.IsSidechain || l.IsMeta || l.Entrypoint != "cli" {
		return Entry{}, false
	}
	if l.Type != "user" && l.Type != "assistant" {
		return Entry{}, false
	}
	var text string
	var s string
	if json.Unmarshal(l.Message.Content, &s) == nil {
		text = s
	} else {
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(l.Message.Content, &blocks) != nil {
			return Entry{}, false
		}
		var parts []string
		for _, bl := range blocks {
			if bl.Type == "text" && strings.TrimSpace(bl.Text) != "" {
				parts = append(parts, bl.Text)
			}
		}
		text = strings.Join(parts, "\n\n")
	}
	text = strings.TrimSpace(text)
	if text == "" || (l.Type == "user" && strings.HasPrefix(text, "<")) { // <command-name>, <local-command-*>
		return Entry{}, false
	}
	at, _ := time.Parse(time.RFC3339Nano, l.Timestamp)
	return Entry{ID: l.UUID, Role: l.Type, Text: text, At: at}, true
}
