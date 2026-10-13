// Package workspace implements resume-first workspace opening on top of
// storage. Opening a workspace NEVER creates, resets or deletes a session.
package workspace

import (
	"fmt"
	"path/filepath"

	"aicw/internal/storage"
)

// Actions returned to the UI.
const (
	ActionResume = "resume" // exactly one resumable session: offer to resume it
	ActionChoose = "choose" // several: show the session picker
	ActionNew    = "new"    // none resumable (or --new): offer/start the Fresh Start flow
)

type OpenResult struct {
	Workspace    *storage.Workspace `json:"workspace"`
	Sessions     []storage.Session  `json:"sessions"`  // all sessions, newest first
	Resumable    []storage.Session  `json:"resumable"` // subset that can be resumed
	Action       string             `json:"action"`
	IsNew        bool               `json:"isNew"`        // workspace record created just now
	RequestedNew bool               `json:"requestedNew"` // `aicw --new`
	Page         string             `json:"page"`         // aicw --page <name>, set by the caller
	Reason       string             `json:"reason"`
}

type Manager struct{ Store *storage.Store }

// Resumable reports whether a session may be resumed.
func Resumable(s storage.Session) bool { return s.Status != storage.StatusCompleted }

// Open handles `aicw <path>` for an already-canonical path. wantNew reflects
// `aicw --new`. It restores the existing record untouched.
func (m *Manager) Open(path string, wantNew bool) (*OpenResult, error) {
	w, created, err := m.Store.UpsertWorkspace(path, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	all, err := m.Store.Sessions(w.ID)
	if err != nil {
		return nil, err
	}
	res := &OpenResult{Workspace: w, Sessions: all, Resumable: []storage.Session{}, IsNew: created, RequestedNew: wantNew}
	for _, s := range all {
		if Resumable(s) {
			res.Resumable = append(res.Resumable, s)
		}
	}
	switch {
	case wantNew:
		res.Action, res.Reason = ActionNew, "new session explicitly requested; existing sessions are kept"
	case len(res.Resumable) == 1:
		res.Action, res.Reason = ActionResume, "one saved session found"
	case len(res.Resumable) > 1:
		res.Action, res.Reason = ActionChoose, fmt.Sprintf("%d saved sessions found; choose which to resume", len(res.Resumable))
	default:
		res.Action, res.Reason = ActionNew, "no resumable session; confirm to start a new one"
	}
	return res, nil
}

// StartSession records a new session. Callers invoke it only after explicit
// user action. Existing sessions are untouched.
func (m *Manager) StartSession(workspaceID int64, agent, title string) (*storage.Session, error) {
	if agent == "" {
		return nil, fmt.Errorf("agent is required")
	}
	return m.Store.CreateSession(storage.Session{WorkspaceID: workspaceID, Agent: agent, Status: storage.StatusPaused, Title: title})
}
