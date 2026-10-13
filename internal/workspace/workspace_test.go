package workspace

import (
	"sync"
	"testing"

	"aicw/internal/storage"
)

func mgr(t *testing.T) *Manager {
	s, err := storage.Open(t.TempDir() + "/aicw.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return &Manager{Store: s}
}

func TestOpenRestoresExistingWorkspaceWithoutNewSession(t *testing.T) {
	m := mgr(t)
	r1, _ := m.Open("/p/a", false)
	if !r1.IsNew || r1.Action != ActionNew || len(r1.Sessions) != 0 {
		t.Fatalf("first open: %+v", r1)
	}
	m.Store.SaveWorkspaceState(r1.Workspace.ID, "claude", `{"task":"x"}`)
	sess, _ := m.StartSession(r1.Workspace.ID, "claude", "t")
	ext := "abc"
	m.Store.UpdateSession(sess.ID, storage.SessionPatch{ExternalID: &ext})

	r2, err := m.Open("/p/a", false)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Workspace.ID != r1.Workspace.ID || r2.IsNew {
		t.Fatal("workspace record must be reused")
	}
	if r2.Action != ActionResume || len(r2.Sessions) != 1 || r2.Resumable[0].ExternalID != "abc" {
		t.Fatalf("expected resume of saved session: %+v", r2)
	}
	if r2.Workspace.Agent != "claude" || r2.Workspace.Settings != `{"task":"x"}` {
		t.Fatalf("state not restored: %+v", r2.Workspace)
	}
	// Opening repeatedly never creates sessions.
	for i := 0; i < 3; i++ {
		m.Open("/p/a", false)
	}
	all, _ := m.Store.Sessions(r1.Workspace.ID)
	if len(all) != 1 {
		t.Fatalf("sessions = %d, want 1", len(all))
	}
}

func TestNewFlagKeepsOldSessions(t *testing.T) {
	m := mgr(t)
	r, _ := m.Open("/p/a", false)
	m.StartSession(r.Workspace.ID, "claude", "old")
	r2, _ := m.Open("/p/a", true)
	if r2.Action != ActionNew || len(r2.Sessions) != 1 {
		t.Fatalf("%+v", r2)
	}
}

func TestPickerWhenMultipleAndCompletedIgnored(t *testing.T) {
	m := mgr(t)
	r, _ := m.Open("/p/a", false)
	a, _ := m.StartSession(r.Workspace.ID, "claude", "1")
	m.StartSession(r.Workspace.ID, "kilo", "2")
	done := storage.StatusCompleted
	m.Store.UpdateSession(a.ID, storage.SessionPatch{Status: &done})
	r2, _ := m.Open("/p/a", false)
	if r2.Action != ActionResume || len(r2.Resumable) != 1 || r2.Resumable[0].Agent != "kilo" {
		t.Fatalf("%+v", r2)
	}
	m.StartSession(r.Workspace.ID, "codex", "3")
	r3, _ := m.Open("/p/a", false)
	if r3.Action != ActionChoose || len(r3.Resumable) != 2 {
		t.Fatalf("%+v", r3)
	}
}

func TestProjectsIsolated(t *testing.T) {
	m := mgr(t)
	a, _ := m.Open("/p/a", false)
	b, _ := m.Open("/p/b", false)
	m.StartSession(a.Workspace.ID, "claude", "a")
	rb, _ := m.Open("/p/b", false)
	if rb.Workspace.ID == a.Workspace.ID || len(rb.Sessions) != 0 || b.Workspace.Path != "/p/b" {
		t.Fatalf("projects mixed: %+v", rb)
	}
}

func TestConcurrentWrites(t *testing.T) {
	m := mgr(t)
	r, _ := m.Open("/p/a", false)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Open("/p/a", false)
			if _, err := m.StartSession(r.Workspace.ID, "claude", "c"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	all, _ := m.Store.Sessions(r.Workspace.ID)
	if len(all) != 30 {
		t.Fatalf("sessions = %d", len(all))
	}
}

func TestInvalidStatusRejected(t *testing.T) {
	m := mgr(t)
	r, _ := m.Open("/p/a", false)
	s, _ := m.StartSession(r.Workspace.ID, "claude", "x")
	bad := "bogus"
	if err := m.Store.UpdateSession(s.ID, storage.SessionPatch{Status: &bad}); err == nil {
		t.Fatal("invalid status accepted")
	}
}
