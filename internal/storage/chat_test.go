package storage

import (
	"path/filepath"
	"testing"
)

func open(t *testing.T) (*Store, *Workspace) {
	s, err := Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	w, _, _ := s.UpsertWorkspace("/p", "p")
	return s, w
}

func TestConversationsAndMessagesWithMedia(t *testing.T) {
	s, w := open(t)
	other, _, _ := s.UpsertWorkspace("/q", "q")
	m1, _ := s.AddMedia(Media{WorkspaceID: w.ID, Name: "a.png", Stored: "/x/a", Kind: "image", Mime: "image/png", Size: 1})
	m2, _ := s.AddMedia(Media{WorkspaceID: other.ID, Name: "b.png", Stored: "/x/b", Kind: "image", Mime: "image/png", Size: 1})
	c, err := s.CreateConversation(w.ID, "  ", "claude")
	if err != nil || c.Title != "New chat" {
		t.Fatalf("%+v %v", c, err)
	}
	msg, err := s.AddMessage(c.ID, RoleUser, SourceUI, "look", []int64{m1.ID, m2.ID, 999})
	if err != nil || len(msg.Media) != 1 || msg.Media[0].ID != m1.ID {
		t.Fatalf("foreign/missing media must be dropped: %+v %v", msg, err)
	}
	s.AddMessage(c.ID, RoleAssistant, SourceAgent, "ok", nil)
	all, _ := s.Messages(c.ID, 0, 0)
	after, _ := s.Messages(c.ID, all[0].ID, 0)
	if len(all) != 2 || len(after) != 1 || after[0].Body != "ok" {
		t.Fatalf("%+v %+v", all, after)
	}
	list, _ := s.Conversations(w.ID)
	if len(list) != 1 || list[0].Messages != 2 {
		t.Fatalf("%+v", list)
	}
	if o, _ := s.Conversations(other.ID); len(o) != 0 {
		t.Fatal("conversations leaked across workspaces")
	}
	if _, err := s.AddMessage(12345, RoleUser, SourceUI, "x", nil); err == nil {
		t.Fatal("unknown conversation accepted")
	}
	title := "Renamed"
	s.UpdateConversation(c.ID, &title, nil, nil)
	if c2, _ := s.Conversation(c.ID); c2.Title != "Renamed" || c2.Agent != "claude" {
		t.Fatalf("%+v", c2)
	}
	s.DeleteConversation(c.ID)
	if m, _ := s.Messages(c.ID, 0, 0); len(m) != 0 {
		t.Fatal("messages must cascade")
	}
}

func TestNotesKeepVersions(t *testing.T) {
	s, w := open(t)
	s.PutNote(w.ID, "memory", "v1", "user")
	s.PutNote(w.ID, "memory", "v2", "agent")
	n, _ := s.Note(w.ID, "memory")
	v, _ := s.NoteVersions(w.ID, "memory")
	if n.Body != "v2" || n.UpdatedBy != "agent" || len(v) != 1 || v[0].Body != "v1" || v[0].SavedBy != "user" {
		t.Fatalf("%+v %+v", n, v)
	}
	if x, _ := s.Note(w.ID, "none"); x != nil {
		t.Fatal("missing note should be nil")
	}
}

func TestSkillsUpsert(t *testing.T) {
	s, w := open(t)
	s.PutSkill(w.ID, "read-memory", "d", "body1", true, "user")
	s.PutSkill(w.ID, "read-memory", "d2", "body2", false, "agent")
	l, _ := s.Skills(w.ID)
	if len(l) != 1 || l[0].Body != "body2" || l[0].AlwaysOn || l[0].UpdatedBy != "agent" {
		t.Fatalf("%+v", l)
	}
}
