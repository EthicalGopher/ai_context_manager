package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aicw/internal/storage"
)

func env(t *testing.T) (Env, *storage.Workspace) {
	dir := t.TempDir()
	s, err := storage.Open(filepath.Join(dir, "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	w, _, _ := s.UpsertWorkspace("/p", "p")
	return Env{Store: s, DataDir: dir, WorkspaceID: w.ID}, w
}

func call(t *testing.T, e Env, name, args string) (string, error) {
	return Call(e, name, json.RawMessage(args))
}

func TestNotesRoundTripAndLimits(t *testing.T) {
	e, _ := env(t)
	if _, err := call(t, e, "write_note", `{"name":"project-memory","body":"# Facts\n- uses Go"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, e, "append_note", `{"name":"project-memory","text":"- tests: go test ./..."}`); err != nil {
		t.Fatal(err)
	}
	out, _ := call(t, e, "read_note", `{"name":"project-memory"}`)
	if !strings.Contains(out, "uses Go") || !strings.Contains(out, "go test") || !strings.Contains(out, `"updatedBy": "agent"`) {
		t.Fatal(out)
	}
	for _, bad := range []string{`{"name":"../x","body":"a"}`, `{"name":"","body":"a"}`, `{"name":"ok","body":"a","extra":1}`} {
		if _, err := call(t, e, "write_note", bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	big, _ := json.Marshal(map[string]string{"name": "big", "body": strings.Repeat("x", MaxNote+1)})
	if _, err := call(t, e, "write_note", string(big)); err == nil {
		t.Fatal("oversize note accepted")
	}
	if _, err := call(t, e, "read_note", `{"name":"nope"}`); err == nil {
		t.Fatal("missing note should error")
	}
}

func TestReadContextIncludesAlwaysOnSkillsAndNotes(t *testing.T) {
	e, w := env(t)
	e.Store.PutSkill(w.ID, "always-read", "d", "ALWAYS BODY", true, "user")
	e.Store.PutSkill(w.ID, "optional", "opt desc", "OPTIONAL BODY", false, "user")
	e.Store.PutNote(w.ID, "memory", "NOTE BODY", "user")
	out, err := call(t, e, "read_context", `{}`)
	if err != nil || !strings.Contains(out, "ALWAYS BODY") || !strings.Contains(out, "NOTE BODY") || !strings.Contains(out, "opt desc") || strings.Contains(out, "OPTIONAL BODY") {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestAgentCanStartConversationAndPostButNotForgeUser(t *testing.T) {
	e, _ := env(t)
	out, err := call(t, e, "create_conversation", `{"title":"Plan release","first_message":"Hi, I created this."}`)
	if err != nil || !strings.Contains(out, `"id"`) {
		t.Fatal(err, out)
	}
	cs, _ := e.Store.Conversations(e.WorkspaceID)
	if len(cs) != 1 {
		t.Fatal("conversation not created")
	}
	img := filepath.Join(t.TempDir(), "shot.png")
	os.WriteFile(img, []byte("\x89PNG\r\n\x1a\n.."), 0o644)
	args, _ := json.Marshal(map[string]any{"conversation_id": cs[0].ID, "text": "see attached", "attachments": []string{img, "relative.png"}, "role": "user"})
	if _, err := call(t, e, "add_message", string(args)); err == nil {
		t.Fatal("unknown field 'role' must be rejected (no forging user messages)")
	}
	args, _ = json.Marshal(map[string]any{"conversation_id": cs[0].ID, "text": "see attached", "attachments": []string{img, "relative.png"}})
	out, err = call(t, e, "add_message", string(args))
	if err != nil || !strings.Contains(out, `"attached": 1`) || !strings.Contains(out, "must be absolute") {
		t.Fatal(err, out)
	}
	msgs, _ := e.Store.Messages(cs[0].ID, 0, 0)
	if len(msgs) != 2 || msgs[1].Role != storage.RoleAssistant || msgs[1].Source != storage.SourceAgent || len(msgs[1].Media) != 1 {
		t.Fatalf("%+v", msgs)
	}
	rd, _ := call(t, e, "read_conversation", `{"conversation_id":`+string(rune('0'+cs[0].ID))+`}`)
	if !strings.Contains(rd, "see attached") {
		t.Fatal(rd)
	}
}

func TestWorkspaceIsolation(t *testing.T) {
	e, _ := env(t)
	other, _, _ := e.Store.UpsertWorkspace("/q", "q")
	oc, _ := e.Store.CreateConversation(other.ID, "theirs", "")
	args, _ := json.Marshal(map[string]any{"conversation_id": oc.ID, "text": "intrude"})
	if _, err := call(t, e, "add_message", string(args)); err == nil {
		t.Fatal("cross-workspace write accepted")
	}
	if _, err := call(t, e, "read_conversation", `{"conversation_id":`+string(rune('0'+oc.ID))+`}`); err == nil {
		t.Fatal("cross-workspace read accepted")
	}
	e.WorkspaceID = 9999
	if _, err := call(t, e, "list_notes", `{}`); err == nil {
		t.Fatal("unknown workspace accepted")
	}
}

func TestAddMediaRefusesSecretsAndUnknownTool(t *testing.T) {
	e, _ := env(t)
	d := t.TempDir()
	envf := filepath.Join(d, ".env")
	os.WriteFile(envf, []byte("A=1"), 0o644)
	a, _ := json.Marshal(map[string]string{"path": envf})
	if _, err := call(t, e, "add_media", string(a)); err == nil {
		t.Fatal("secret file accepted")
	}
	if _, err := call(t, e, "rm_rf", `{}`); err == nil {
		t.Fatal("unknown tool accepted")
	}
	for _, tl := range All() {
		if tl.Name == "" || tl.Description == "" || tl.Schema["type"] != "object" {
			t.Errorf("bad tool def %+v", tl.Name)
		}
	}
}

func TestSkillsViaTool(t *testing.T) {
	e, _ := env(t)
	if _, err := call(t, e, "add_skill", `{"name":"keep-notes","description":"d","body":"Always update notes."}`); err != nil {
		t.Fatal(err)
	}
	l, _ := e.Store.Skills(e.WorkspaceID)
	if len(l) != 1 || !l[0].AlwaysOn || l[0].UpdatedBy != "agent" {
		t.Fatalf("%+v", l)
	}
}

func TestPagesWithEmbeddedLocalMedia(t *testing.T) {
	e, _ := env(t)
	img := filepath.Join(t.TempDir(), "pig.png")
	os.WriteFile(img, []byte("\x89PNG\r\n\x1a\n.."), 0o644)
	secret := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(secret, []byte("A=1"), 0o644)
	body := "# Meet Mr. Mak\n\n![Selected visual reference](" + img + ")\n\n![leak](" + secret + ")\n"
	args, _ := json.Marshal(map[string]string{"title": "Character", "summary": "Keep the pink nose.", "body": body})
	out, err := call(t, e, "write_page", string(args))
	if err != nil || !strings.Contains(out, ".env") {
		t.Fatalf("secret file must be reported, not stored: %v %s", err, out)
	}
	pg, _ := e.Store.PageByTitle(e.WorkspaceID, "character")
	ms, _ := e.Store.ListMedia(e.WorkspaceID)
	if pg == nil || len(ms) != 1 || !strings.Contains(pg.Body, "![Selected visual reference](media:") || !strings.Contains(pg.Body, secret) || pg.UpdatedBy != "agent" {
		t.Fatalf("%+v %+v", pg, ms)
	}
	a2, _ := json.Marshal(map[string]string{"title": "Character", "markdown": "## More\nText"})
	if _, err := call(t, e, "append_page", string(a2)); err != nil {
		t.Fatal(err)
	}
	rd, _ := call(t, e, "read_page", `{"title":"CHARACTER"}`)
	if !strings.Contains(rd, "## More") || !strings.Contains(rd, "media:") {
		t.Fatal(rd)
	}
	ls, _ := call(t, e, "list_pages", `{}`)
	if !strings.Contains(ls, "Keep the pink nose.") {
		t.Fatal(ls)
	}
	for _, bad := range []string{`{"title":"","body":"x"}`, `{"title":"t","body":"x","evil":1}`} {
		if _, err := call(t, e, "write_page", bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
