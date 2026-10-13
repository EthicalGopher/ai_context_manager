package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestNoBackupWhenAlreadyCurrentAndBackupIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aicw.db")
	// Simulate an older schema (version 0 with data is impossible, so fake v1 then
	// pretend a v2 migration exists by checking the backup logic directly).
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.UpsertWorkspace("/p", "p")
	s.Close()
	if _, err := os.Stat(path + ".bak-v1"); err == nil {
		t.Fatal("no backup expected when already at the latest version")
	}
	db, _ := sql.Open("sqlite", path)
	var v int
	db.QueryRow("PRAGMA user_version").Scan(&v)
	db.Close()
	if v != 4 {
		t.Fatalf("user_version = %d, want 4", v)
	}
	if err := copyFile(path, path+".bak-v1"); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(path + ".bak-v1"); st.Mode().Perm() != 0o600 {
		t.Fatal("backup must be owner-only")
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aicw.db")
	s, _ := Open(path)
	w, created, _ := s.UpsertWorkspace("/p", "p")
	if !created {
		t.Fatal("expected created")
	}
	s.CreateSession(Session{WorkspaceID: w.ID, Agent: "claude", Status: StatusActive, ExternalID: "x"})
	s.Close()
	s, _ = Open(path)
	defer s.Close()
	w2, created, _ := s.UpsertWorkspace("/p", "p")
	ses, _ := s.Sessions(w2.ID)
	if created || w2.ID != w.ID || len(ses) != 1 || ses[0].ExternalID != "x" {
		t.Fatalf("state lost: %+v %+v", w2, ses)
	}
}

func TestMigrationFromV1KeepsDataAndBacksUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aicw.db")
	db, _ := sql.Open("sqlite", path)
	db.SetMaxOpenConns(1)
	db.Exec(`CREATE TABLE workspaces(id INTEGER PRIMARY KEY, path TEXT NOT NULL UNIQUE, name TEXT NOT NULL, agent TEXT NOT NULL DEFAULT '', settings TEXT NOT NULL DEFAULT '{}', created_at INTEGER NOT NULL, last_opened INTEGER NOT NULL);
CREATE TABLE sessions(id INTEGER PRIMARY KEY, workspace_id INTEGER NOT NULL, agent TEXT NOT NULL, external_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, title TEXT NOT NULL DEFAULT '', handoff TEXT NOT NULL DEFAULT '', context_version TEXT NOT NULL DEFAULT '', exit_status TEXT NOT NULL DEFAULT '', pid INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, last_active INTEGER NOT NULL);
CREATE TABLE settings(key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE custom_agents(id TEXT PRIMARY KEY, json TEXT NOT NULL);
INSERT INTO workspaces(path,name,created_at,last_opened) VALUES('/old','old',1,1);
PRAGMA user_version=1;`)
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(path + ".bak-v1"); err != nil {
		t.Fatal("expected pre-migration backup")
	}
	w, _ := s.WorkspaceByPath("/old")
	if w == nil {
		t.Fatal("workspace lost in migration")
	}
	m, err := s.AddMedia(Media{WorkspaceID: w.ID, Name: "a.png", Stored: "/x", Kind: "image", Mime: "image/png", Size: 3})
	if err != nil || m.ID == 0 {
		t.Fatal(err)
	}
	if l, _ := s.ListMedia(w.ID); len(l) != 1 {
		t.Fatal("media not listed")
	}
}
