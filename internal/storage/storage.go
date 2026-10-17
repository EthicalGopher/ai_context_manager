// Package storage persists workspaces, sessions, settings and custom agent
// profiles in SQLite. No secrets are stored.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Session states.
const (
	StatusActive      = "active"
	StatusPaused      = "paused"
	StatusCompleted   = "completed"
	StatusInterrupted = "interrupted"
	StatusAttention   = "needs_attention"
	StatusExtGone     = "external_session_unavailable"
)

var validStatus = map[string]bool{StatusActive: true, StatusPaused: true, StatusCompleted: true, StatusInterrupted: true, StatusAttention: true, StatusExtGone: true}

type Workspace struct {
	ID         int64     `json:"id"`
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Agent      string    `json:"agent"`    // last selected agent
	Settings   string    `json:"settings"` // JSON: context selections, exclusions, task notes
	CreatedAt  time.Time `json:"createdAt"`
	LastOpened time.Time `json:"lastOpened"`
}

type Session struct {
	ID             int64     `json:"id"`
	WorkspaceID    int64     `json:"workspaceId"`
	Agent          string    `json:"agent"`
	ExternalID     string    `json:"externalId"`
	Status         string    `json:"status"`
	Title          string    `json:"title"`
	Handoff        string    `json:"handoff"`
	ContextVersion string    `json:"contextVersion"`
	ExitStatus     string    `json:"exitStatus"`
	PID            int       `json:"pid"`
	CreatedAt      time.Time `json:"createdAt"`
	LastActive     time.Time `json:"lastActive"`
}

type CustomAgent struct {
	ID   string `json:"id"`
	JSON string `json:"json"`
}

type Store struct {
	db   *sql.DB
	now  func() time.Time
	path string
}

// Open opens (creating if needed) the database file.
func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // serialise writers inside the process
	s := &Store{db: db, now: time.Now, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// SetClock overrides the clock (tests).
func (s *Store) SetClock(f func() time.Time) { s.now = f }

func (s *Store) migrate() error {
	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	steps := []string{`
CREATE TABLE workspaces(
  id INTEGER PRIMARY KEY, path TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
  agent TEXT NOT NULL DEFAULT '', settings TEXT NOT NULL DEFAULT '{}',
  created_at INTEGER NOT NULL, last_opened INTEGER NOT NULL);
CREATE TABLE sessions(
  id INTEGER PRIMARY KEY, workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  agent TEXT NOT NULL, external_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '', handoff TEXT NOT NULL DEFAULT '', context_version TEXT NOT NULL DEFAULT '',
  exit_status TEXT NOT NULL DEFAULT '', pid INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL, last_active INTEGER NOT NULL);
CREATE INDEX sessions_ws ON sessions(workspace_id, last_active);
CREATE TABLE settings(key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE custom_agents(id TEXT PRIMARY KEY, json TEXT NOT NULL);`, `
CREATE TABLE media(
  id INTEGER PRIMARY KEY, workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  name TEXT NOT NULL, stored TEXT NOT NULL, kind TEXT NOT NULL, mime TEXT NOT NULL, size INTEGER NOT NULL,
  note TEXT NOT NULL DEFAULT '', in_context INTEGER NOT NULL DEFAULT 0, added_at INTEGER NOT NULL);
CREATE INDEX media_ws ON media(workspace_id, added_at);`, `
CREATE TABLE conversations(
  id INTEGER PRIMARY KEY, workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  title TEXT NOT NULL, agent TEXT NOT NULL DEFAULT '', session_id INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE INDEX conv_ws ON conversations(workspace_id, updated_at);
CREATE TABLE messages(
  id INTEGER PRIMARY KEY, conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  role TEXT NOT NULL, source TEXT NOT NULL DEFAULT 'ui', body TEXT NOT NULL, created_at INTEGER NOT NULL);
CREATE INDEX msg_conv ON messages(conversation_id, id);
CREATE TABLE message_media(
  message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
  PRIMARY KEY(message_id, media_id));
CREATE TABLE notes(
  workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  name TEXT NOT NULL, body TEXT NOT NULL, updated_at INTEGER NOT NULL, updated_by TEXT NOT NULL DEFAULT 'user',
  PRIMARY KEY(workspace_id, name));
CREATE TABLE note_versions(
  id INTEGER PRIMARY KEY, workspace_id INTEGER NOT NULL, name TEXT NOT NULL, body TEXT NOT NULL,
  saved_at INTEGER NOT NULL, saved_by TEXT NOT NULL);
CREATE INDEX note_ver ON note_versions(workspace_id, name, id);
CREATE TABLE skills(
  id INTEGER PRIMARY KEY, workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', body TEXT NOT NULL,
  always_on INTEGER NOT NULL DEFAULT 1, updated_at INTEGER NOT NULL, updated_by TEXT NOT NULL DEFAULT 'user',
  UNIQUE(workspace_id, name));`, `
CREATE TABLE pages(
  id INTEGER PRIMARY KEY, workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  title TEXT NOT NULL COLLATE NOCASE, summary TEXT NOT NULL DEFAULT '', body TEXT NOT NULL DEFAULT '',
  sort INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL, updated_by TEXT NOT NULL DEFAULT 'user',
  UNIQUE(workspace_id, title));
CREATE TABLE page_versions(
  id INTEGER PRIMARY KEY, page_id INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
  title TEXT NOT NULL, summary TEXT NOT NULL, body TEXT NOT NULL, saved_at INTEGER NOT NULL, saved_by TEXT NOT NULL);
CREATE INDEX page_ver ON page_versions(page_id, id);`, `
ALTER TABLE messages ADD COLUMN ext_id TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX msg_ext ON messages(conversation_id, ext_id) WHERE ext_id <> '';`}
	if v > 0 && v < len(steps) && s.path != "" && s.path != ":memory:" {
		// Existing database about to change shape: keep a copy first.
		if err := copyFile(s.path, fmt.Sprintf("%s.bak-v%d", s.path, v)); err != nil {
			return fmt.Errorf("could not back up database before migration: %w", err)
		}
	}
	for i := v; i < len(steps); i++ {
		if _, err := s.db.Exec(steps[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA user_version=%d`, i+1)); err != nil {
			return err
		}
	}
	return nil
}

func ts(t time.Time) int64     { return t.UnixMilli() }
func fromTS(m int64) time.Time { return time.UnixMilli(m) }

// UpsertWorkspace returns the durable workspace record for a canonical path,
// creating it on first sight and updating last_opened otherwise. It never
// resets saved state.
func (s *Store) UpsertWorkspace(path, name string) (*Workspace, bool, error) {
	now := ts(s.now())
	res, err := s.db.Exec(`INSERT INTO workspaces(path,name,created_at,last_opened) VALUES(?,?,?,?)
		ON CONFLICT(path) DO NOTHING`, path, name, now, now)
	if err != nil {
		return nil, false, err
	}
	n, _ := res.RowsAffected()
	created := n > 0
	if !created {
		if _, err := s.db.Exec(`UPDATE workspaces SET last_opened=? WHERE path=?`, now, path); err != nil {
			return nil, false, err
		}
	}
	w, err := s.WorkspaceByPath(path)
	return w, created, err
}

func scanWS(r interface{ Scan(...any) error }) (*Workspace, error) {
	var w Workspace
	var c, l int64
	if err := r.Scan(&w.ID, &w.Path, &w.Name, &w.Agent, &w.Settings, &c, &l); err != nil {
		return nil, err
	}
	w.CreatedAt, w.LastOpened = fromTS(c), fromTS(l)
	return &w, nil
}

const wsCols = `id,path,name,agent,settings,created_at,last_opened`

func (s *Store) WorkspaceByPath(path string) (*Workspace, error) {
	w, err := scanWS(s.db.QueryRow(`SELECT `+wsCols+` FROM workspaces WHERE path=?`, path))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return w, err
}

func (s *Store) Workspace(id int64) (*Workspace, error) {
	return scanWS(s.db.QueryRow(`SELECT `+wsCols+` FROM workspaces WHERE id=?`, id))
}

func (s *Store) RecentWorkspaces(limit int) ([]Workspace, error) {
	rows, err := s.db.Query(`SELECT `+wsCols+` FROM workspaces ORDER BY last_opened DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Workspace{}
	for rows.Next() {
		w, err := scanWS(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}

func (s *Store) SaveWorkspaceState(id int64, agent, settingsJSON string) error {
	_, err := s.db.Exec(`UPDATE workspaces SET agent=?, settings=? WHERE id=?`, agent, settingsJSON, id)
	return err
}

func (s *Store) ForgetWorkspace(id int64) error {
	_, err := s.db.Exec(`DELETE FROM workspaces WHERE id=?`, id)
	return err
}

// ---- sessions ----

const sesCols = `id,workspace_id,agent,external_id,status,title,handoff,context_version,exit_status,pid,created_at,last_active`

func scanSes(r interface{ Scan(...any) error }) (*Session, error) {
	var x Session
	var c, l int64
	if err := r.Scan(&x.ID, &x.WorkspaceID, &x.Agent, &x.ExternalID, &x.Status, &x.Title, &x.Handoff, &x.ContextVersion, &x.ExitStatus, &x.PID, &c, &l); err != nil {
		return nil, err
	}
	x.CreatedAt, x.LastActive = fromTS(c), fromTS(l)
	return &x, nil
}

func (s *Store) CreateSession(x Session) (*Session, error) {
	if !validStatus[x.Status] {
		x.Status = StatusPaused
	}
	now := ts(s.now())
	res, err := s.db.Exec(`INSERT INTO sessions(workspace_id,agent,external_id,status,title,handoff,context_version,exit_status,pid,created_at,last_active)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`, x.WorkspaceID, x.Agent, x.ExternalID, x.Status, x.Title, x.Handoff, x.ContextVersion, x.ExitStatus, x.PID, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Session(id)
}

func (s *Store) Session(id int64) (*Session, error) {
	return scanSes(s.db.QueryRow(`SELECT `+sesCols+` FROM sessions WHERE id=?`, id))
}

// Sessions lists a workspace's sessions, newest first.
func (s *Store) Sessions(workspaceID int64) ([]Session, error) {
	rows, err := s.db.Query(`SELECT `+sesCols+` FROM sessions WHERE workspace_id=? ORDER BY last_active DESC, id DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		x, err := scanSes(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

// SessionPatch updates only the provided fields.
type SessionPatch struct {
	Status         *string
	ExternalID     *string
	Title          *string
	Handoff        *string
	ContextVersion *string
	ExitStatus     *string
	PID            *int
}

func (s *Store) UpdateSession(id int64, p SessionPatch) error {
	cur, err := s.Session(id)
	if err != nil {
		return err
	}
	if p.Status != nil {
		if !validStatus[*p.Status] {
			return fmt.Errorf("invalid session status %q", *p.Status)
		}
		cur.Status = *p.Status
	}
	if p.ExternalID != nil {
		cur.ExternalID = *p.ExternalID
	}
	if p.Title != nil {
		cur.Title = *p.Title
	}
	if p.Handoff != nil {
		cur.Handoff = *p.Handoff
	}
	if p.ContextVersion != nil {
		cur.ContextVersion = *p.ContextVersion
	}
	if p.ExitStatus != nil {
		cur.ExitStatus = *p.ExitStatus
	}
	if p.PID != nil {
		cur.PID = *p.PID
	}
	_, err = s.db.Exec(`UPDATE sessions SET status=?,external_id=?,title=?,handoff=?,context_version=?,exit_status=?,pid=?,last_active=? WHERE id=?`,
		cur.Status, cur.ExternalID, cur.Title, cur.Handoff, cur.ContextVersion, cur.ExitStatus, cur.PID, ts(s.now()), id)
	return err
}

func (s *Store) DeleteSession(id int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id=?`, id)
	return err
}

// ---- settings / custom agents ----

func (s *Store) Setting(key, def string) string {
	var v string
	if err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v); err != nil {
		return def
	}
	return v
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) PutCustomAgent(id, js string) error {
	_, err := s.db.Exec(`INSERT INTO custom_agents(id,json) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET json=excluded.json`, id, js)
	return err
}

func (s *Store) DeleteCustomAgent(id string) error {
	_, err := s.db.Exec(`DELETE FROM custom_agents WHERE id=?`, id)
	return err
}

func (s *Store) CustomAgents() ([]CustomAgent, error) {
	rows, err := s.db.Query(`SELECT id,json FROM custom_agents ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CustomAgent{}
	for rows.Next() {
		var c CustomAgent
		if err := rows.Scan(&c.ID, &c.JSON); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func copyFile(src, dst string) error {
	// Fold WAL contents into the main file so the copy is complete.
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}

// ---- media ----

type Media struct {
	ID          int64     `json:"id"`
	WorkspaceID int64     `json:"workspaceId"`
	Name        string    `json:"name"`
	Stored      string    `json:"stored"` // absolute path of AICW's private copy
	Kind        string    `json:"kind"`   // image | video | audio | pdf | file
	Mime        string    `json:"mime"`
	Size        int64     `json:"size"`
	Note        string    `json:"note"`
	InContext   bool      `json:"inContext"`
	AddedAt     time.Time `json:"addedAt"`
}

const mediaCols = `id,workspace_id,name,stored,kind,mime,size,note,in_context,added_at`

func scanMedia(r interface{ Scan(...any) error }) (*Media, error) {
	var m Media
	var ic, at int64
	if err := r.Scan(&m.ID, &m.WorkspaceID, &m.Name, &m.Stored, &m.Kind, &m.Mime, &m.Size, &m.Note, &ic, &at); err != nil {
		return nil, err
	}
	m.InContext, m.AddedAt = ic != 0, fromTS(at)
	return &m, nil
}

func (s *Store) AddMedia(m Media) (*Media, error) {
	res, err := s.db.Exec(`INSERT INTO media(workspace_id,name,stored,kind,mime,size,note,in_context,added_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		m.WorkspaceID, m.Name, m.Stored, m.Kind, m.Mime, m.Size, m.Note, b2i(m.InContext), ts(s.now()))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Media(id)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) Media(id int64) (*Media, error) {
	return scanMedia(s.db.QueryRow(`SELECT `+mediaCols+` FROM media WHERE id=?`, id))
}

func (s *Store) ListMedia(workspaceID int64) ([]Media, error) {
	rows, err := s.db.Query(`SELECT `+mediaCols+` FROM media WHERE workspace_id=? ORDER BY added_at DESC, id DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Media{}
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) UpdateMedia(id int64, note string, inContext bool) error {
	_, err := s.db.Exec(`UPDATE media SET note=?, in_context=? WHERE id=?`, note, b2i(inContext), id)
	return err
}

func (s *Store) DeleteMedia(id int64) error {
	_, err := s.db.Exec(`DELETE FROM media WHERE id=?`, id)
	return err
}

// SessionCount is per-workspace session totals for the sidebar.
type SessionCount struct{ Total, Resumable int }

func (s *Store) SessionCounts() (map[int64]SessionCount, error) {
	rows, err := s.db.Query(`SELECT workspace_id, COUNT(*), SUM(CASE WHEN status != 'completed' THEN 1 ELSE 0 END) FROM sessions GROUP BY workspace_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]SessionCount{}
	for rows.Next() {
		var id int64
		var c SessionCount
		if err := rows.Scan(&id, &c.Total, &c.Resumable); err != nil {
			return nil, err
		}
		out[id] = c
	}
	return out, rows.Err()
}
