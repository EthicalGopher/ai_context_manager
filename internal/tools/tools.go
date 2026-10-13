// Package tools is the set of actions an AI agent may take inside AICW:
// create conversations, post messages, attach media, and read/update living
// context notes and skills. The same dispatcher backs the MCP server and the
// `aicw tool` command. Agents can only add/update AICW's own data; they cannot
// spawn processes, type into terminals, or write project files through it.
package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"aicw/internal/media"
	"aicw/internal/storage"
)

const (
	MaxText = 64 * 1024
	MaxNote = 256 * 1024
	Actor   = "agent"
)

type Env struct {
	Store       *storage.Store
	DataDir     string
	WorkspaceID int64
}

type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Run         func(Env, json.RawMessage) (any, error)
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,63}$`)

func obj(props map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}
func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
func bln(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
func strs(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func All() []Tool {
	return []Tool{
		{"read_context", "Read the project's living context: the current notes and enabled skills. Call this at the start of every session before answering.", obj(map[string]any{}), readContext},
		{"list_notes", "List context note names.", obj(map[string]any{}), listNotes},
		{"read_note", "Read one context note.", obj(map[string]any{"name": str("note name")}, "name"), readNote},
		{"write_note", "Create or replace a context note (the previous version is kept). Use it to record decisions, status and learnings so the next session starts informed.", obj(map[string]any{"name": str("note name, e.g. project-memory"), "body": str("full Markdown body")}, "name", "body"), writeNote},
		{"append_note", "Append Markdown to a context note, creating it if needed.", obj(map[string]any{"name": str("note name"), "text": str("Markdown to append")}, "name", "text"), appendNote},
		{"list_skills", "List skills (name, description, always_on).", obj(map[string]any{}), listSkills},
		{"read_skill", "Read a skill's full instructions.", obj(map[string]any{"name": str("skill name")}, "name"), readSkill},
		{"add_skill", "Create or update a skill that future sessions must follow. always_on=true means it is injected into every session's instructions once the user applies them.", obj(map[string]any{"name": str("skill name"), "description": str("one line"), "body": str("instructions in Markdown"), "always_on": bln("inject into every session (default true)")}, "name", "body"), addSkill},
		{"list_pages", "List the project's pages (rich documents shown in the app's Pages view).", obj(map[string]any{}), listPages},
		{"read_page", "Read a page by title.", obj(map[string]any{"title": str("page title")}, "title"), readPage},
		{"write_page", "Create or replace a page. body is Markdown. Embed images/video/audio with ![caption](media:ID) after add_media, or simply ![caption](/absolute/path/to/file.png) and AICW stores the file for you. Use pages for designs, plans, research and anything the user should browse later.", obj(map[string]any{"title": str("page title"), "summary": str("one-sentence lede shown under the title"), "body": str("full Markdown body")}, "title", "body"), writePage},
		{"append_page", "Append Markdown (with optional embedded media) to a page, creating it if needed.", obj(map[string]any{"title": str("page title"), "markdown": str("Markdown to append")}, "title", "markdown"), appendPage},
		{"list_conversations", "List this workspace's conversations, newest first.", obj(map[string]any{}), listConversations},
		{"create_conversation", "Start a new conversation (a chat thread in the app) with an optional first assistant message.", obj(map[string]any{"title": str("conversation title"), "first_message": str("optional first message, posted as the assistant")}, "title"), createConversation},
		{"read_conversation", "Read recent messages of a conversation.", obj(map[string]any{"conversation_id": num("id"), "limit": num("max messages, default 50")}, "conversation_id"), readConversation},
		{"add_message", "Post a message to a conversation as the assistant. Optionally attach media by absolute file path (images, audio, video, PDFs).", obj(map[string]any{"conversation_id": num("id"), "text": str("Markdown message"), "attachments": strs("absolute file paths to attach")}, "conversation_id", "text"), addMessage},
		{"add_media", "Store a file (image, audio, video, PDF, other) in the workspace media library by absolute path. Returns its id.", obj(map[string]any{"path": str("absolute path"), "note": str("what it is"), "include_in_context": bln("mention it in the context package")}, "path"), addMedia},
		{"list_media", "List media in the workspace library.", obj(map[string]any{}), listMedia},
	}
}

func Find(name string) (Tool, bool) {
	for _, t := range All() {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// Call runs a tool and returns pretty JSON text.
func Call(env Env, name string, args json.RawMessage) (string, error) {
	t, ok := Find(name)
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if _, err := env.Store.Workspace(env.WorkspaceID); err != nil {
		return "", errors.New("workspace not found")
	}
	res, err := t.Run(env, args)
	if err != nil {
		return "", err
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return string(b), nil
}

func parse(args json.RawMessage, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(args)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

func checkName(n string) (string, error) {
	n = strings.TrimSpace(n)
	if !nameRe.MatchString(n) {
		return "", errors.New("name must be 1-64 characters: letters, digits, space, . _ -")
	}
	return n, nil
}

func checkText(s string, max int) error {
	if len(s) > max {
		return fmt.Errorf("text is too long (%d bytes, limit %d)", len(s), max)
	}
	return nil
}

func readContext(e Env, _ json.RawMessage) (any, error) {
	notes, err := e.Store.Notes(e.WorkspaceID)
	if err != nil {
		return nil, err
	}
	skills, err := e.Store.Skills(e.WorkspaceID)
	if err != nil {
		return nil, err
	}
	type sk struct{ Name, Description, Body string }
	var always, other []sk
	for _, k := range skills {
		if k.AlwaysOn {
			always = append(always, sk{k.Name, k.Description, k.Body})
		} else {
			other = append(other, sk{Name: k.Name, Description: k.Description})
		}
	}
	return map[string]any{
		"instructions":     "Follow the always_on_skills in this session. Keep notes current with write_note/append_note when you make progress or decisions.",
		"always_on_skills": always,
		"other_skills":     other,
		"notes":            notes,
	}, nil
}

func listNotes(e Env, _ json.RawMessage) (any, error) {
	ns, err := e.Store.Notes(e.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, n := range ns {
		out = append(out, map[string]any{"name": n.Name, "bytes": len(n.Body), "updated_at": n.UpdatedAt, "updated_by": n.UpdatedBy})
	}
	return out, nil
}

func readNote(e Env, a json.RawMessage) (any, error) {
	var p struct{ Name string }
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	n, err := e.Store.Note(e.WorkspaceID, strings.TrimSpace(p.Name))
	if err != nil {
		return nil, err
	}
	if n == nil {
		return nil, errors.New("note not found")
	}
	return n, nil
}

func writeNote(e Env, a json.RawMessage) (any, error) {
	var p struct{ Name, Body string }
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	name, err := checkName(p.Name)
	if err != nil {
		return nil, err
	}
	if err := checkText(p.Body, MaxNote); err != nil {
		return nil, err
	}
	if err := e.Store.PutNote(e.WorkspaceID, name, p.Body, Actor); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "name": name, "bytes": len(p.Body)}, nil
}

func appendNote(e Env, a json.RawMessage) (any, error) {
	var p struct{ Name, Text string }
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	name, err := checkName(p.Name)
	if err != nil {
		return nil, err
	}
	cur, _ := e.Store.Note(e.WorkspaceID, name)
	body := p.Text
	if cur != nil {
		body = strings.TrimRight(cur.Body, "\n") + "\n\n" + p.Text
	}
	if err := checkText(body, MaxNote); err != nil {
		return nil, err
	}
	if err := e.Store.PutNote(e.WorkspaceID, name, body, Actor); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "name": name, "bytes": len(body)}, nil
}

func listSkills(e Env, _ json.RawMessage) (any, error) {
	ks, err := e.Store.Skills(e.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, k := range ks {
		out = append(out, map[string]any{"name": k.Name, "description": k.Description, "always_on": k.AlwaysOn})
	}
	return out, nil
}

func readSkill(e Env, a json.RawMessage) (any, error) {
	var p struct{ Name string }
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	ks, err := e.Store.Skills(e.WorkspaceID)
	if err != nil {
		return nil, err
	}
	for _, k := range ks {
		if k.Name == strings.TrimSpace(p.Name) {
			return k, nil
		}
	}
	return nil, errors.New("skill not found")
}

func addSkill(e Env, a json.RawMessage) (any, error) {
	var p struct {
		Name, Description, Body string
		AlwaysOn                *bool `json:"always_on"`
	}
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	name, err := checkName(p.Name)
	if err != nil {
		return nil, err
	}
	if err := checkText(p.Body, MaxNote); err != nil {
		return nil, err
	}
	on := true
	if p.AlwaysOn != nil {
		on = *p.AlwaysOn
	}
	if err := e.Store.PutSkill(e.WorkspaceID, name, strings.TrimSpace(p.Description), p.Body, on, Actor); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "name": name, "note": "The user must re-apply skills to the project instructions in AICW for changes to reach new sessions' instruction files."}, nil
}

func listConversations(e Env, _ json.RawMessage) (any, error) {
	cs, err := e.Store.Conversations(e.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, c := range cs {
		out = append(out, map[string]any{"id": c.ID, "title": c.Title, "agent": c.Agent, "messages": c.Messages, "updated_at": c.UpdatedAt})
	}
	return out, nil
}

func ownConv(e Env, id int64) (*storage.Conversation, error) {
	c, err := e.Store.Conversation(id)
	if err != nil || c.WorkspaceID != e.WorkspaceID {
		return nil, errors.New("conversation not found in this workspace")
	}
	return c, nil
}

func createConversation(e Env, a json.RawMessage) (any, error) {
	var p struct {
		Title        string
		FirstMessage string `json:"first_message"`
	}
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	if len(p.Title) > 200 {
		return nil, errors.New("title too long")
	}
	if err := checkText(p.FirstMessage, MaxText); err != nil {
		return nil, err
	}
	c, err := e.Store.CreateConversation(e.WorkspaceID, p.Title, "")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.FirstMessage) != "" {
		if _, err := e.Store.AddMessage(c.ID, storage.RoleAssistant, storage.SourceAgent, p.FirstMessage, nil); err != nil {
			return nil, err
		}
	}
	return map[string]any{"id": c.ID, "title": c.Title}, nil
}

func readConversation(e Env, a json.RawMessage) (any, error) {
	var p struct {
		ConversationID int64 `json:"conversation_id"`
		Limit          int
	}
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	if _, err := ownConv(e, p.ConversationID); err != nil {
		return nil, err
	}
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	all, err := e.Store.Messages(p.ConversationID, 0, 2000)
	if err != nil {
		return nil, err
	}
	if len(all) > p.Limit {
		all = all[len(all)-p.Limit:]
	}
	out := []map[string]any{}
	for _, m := range all {
		files := []string{}
		for _, md := range m.Media {
			files = append(files, md.Stored)
		}
		out = append(out, map[string]any{"id": m.ID, "role": m.Role, "text": m.Body, "attachments": files, "at": m.CreatedAt})
	}
	return out, nil
}

func addMessage(e Env, a json.RawMessage) (any, error) {
	var p struct {
		ConversationID int64 `json:"conversation_id"`
		Text           string
		Attachments    []string
	}
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	if _, err := ownConv(e, p.ConversationID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Text) == "" && len(p.Attachments) == 0 {
		return nil, errors.New("text or attachments required")
	}
	if err := checkText(p.Text, MaxText); err != nil {
		return nil, err
	}
	if len(p.Attachments) > 10 {
		return nil, errors.New("at most 10 attachments per message")
	}
	var ids []int64
	var warnings []string
	for _, path := range p.Attachments {
		m, err := importMedia(e, path, "", false)
		if err != nil {
			warnings = append(warnings, filepath.Base(path)+": "+err.Error())
			continue
		}
		ids = append(ids, m.ID)
	}
	// Agents can only speak as the assistant; they cannot forge user messages.
	msg, err := e.Store.AddMessage(p.ConversationID, storage.RoleAssistant, storage.SourceAgent, p.Text, ids)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": msg.ID, "attached": len(ids), "warnings": warnings}, nil
}

func importMedia(e Env, path, note string, inCtx bool) (*storage.Media, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("path must be absolute")
	}
	dir := filepath.Join(e.DataDir, "media", fmt.Sprint(e.WorkspaceID))
	im, err := media.Import(path, dir, media.Unique())
	if err != nil {
		return nil, err
	}
	return e.Store.AddMedia(storage.Media{WorkspaceID: e.WorkspaceID, Name: im.Name, Stored: im.Stored, Kind: im.Kind, Mime: im.Mime, Size: im.Size, Note: note, InContext: inCtx})
}

func addMedia(e Env, a json.RawMessage) (any, error) {
	var p struct {
		Path, Note       string
		IncludeInContext bool `json:"include_in_context"`
	}
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	if len(p.Note) > 2000 {
		return nil, errors.New("note too long")
	}
	m, err := importMedia(e, p.Path, p.Note, p.IncludeInContext)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": m.ID, "name": m.Name, "kind": m.Kind, "stored_path": m.Stored}, nil
}

func listMedia(e Env, _ json.RawMessage) (any, error) {
	ms, err := e.Store.ListMedia(e.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, m := range ms {
		out = append(out, map[string]any{"id": m.ID, "name": m.Name, "kind": m.Kind, "size": m.Size, "note": m.Note, "path": m.Stored})
	}
	return out, nil
}

var localImg = regexp.MustCompile(`!\[([^\]]*)\]\((/[^)\s]+)\)`)

// storeLocalMedia turns ![cap](/abs/path) references into stored media refs.
func storeLocalMedia(e Env, md string) (string, []string) {
	var warnings []string
	n := 0
	out := localImg.ReplaceAllStringFunc(md, func(m string) string {
		sub := localImg.FindStringSubmatch(m)
		n++
		if n > 20 {
			warnings = append(warnings, "more than 20 local files; the rest were left as paths")
			return m
		}
		med, err := importMedia(e, sub[2], sub[1], false)
		if err != nil {
			warnings = append(warnings, filepath.Base(sub[2])+": "+err.Error())
			return m
		}
		return fmt.Sprintf("![%s](media:%d)", sub[1], med.ID)
	})
	return out, warnings
}

func listPages(e Env, _ json.RawMessage) (any, error) {
	ps, err := e.Store.Pages(e.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, p := range ps {
		out = append(out, map[string]any{"id": p.ID, "title": p.Title, "summary": p.Summary, "updated_by": p.UpdatedBy, "updated_at": p.UpdatedAt})
	}
	return out, nil
}

func readPage(e Env, a json.RawMessage) (any, error) {
	var p struct{ Title string }
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	pg, err := e.Store.PageByTitle(e.WorkspaceID, p.Title)
	if err != nil {
		return nil, err
	}
	if pg == nil {
		return nil, errors.New("page not found")
	}
	return pg, nil
}

func checkTitle(t string) (string, error) {
	t = strings.TrimSpace(t)
	if t == "" || len(t) > 120 {
		return "", errors.New("title must be 1-120 characters")
	}
	return t, nil
}

func writePage(e Env, a json.RawMessage) (any, error) {
	var p struct{ Title, Summary, Body string }
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	title, err := checkTitle(p.Title)
	if err != nil {
		return nil, err
	}
	if err := checkText(p.Body, MaxNote); err != nil {
		return nil, err
	}
	if len(p.Summary) > 500 {
		return nil, errors.New("summary is too long (500 characters max)")
	}
	body, warnings := storeLocalMedia(e, p.Body)
	pg, err := e.Store.PutPage(e.WorkspaceID, title, strings.TrimSpace(p.Summary), body, Actor)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "id": pg.ID, "title": pg.Title, "warnings": warnings}, nil
}

func appendPage(e Env, a json.RawMessage) (any, error) {
	var p struct{ Title, Markdown string }
	if err := parse(a, &p); err != nil {
		return nil, err
	}
	title, err := checkTitle(p.Title)
	if err != nil {
		return nil, err
	}
	if err := checkText(p.Markdown, MaxText); err != nil {
		return nil, err
	}
	md, warnings := storeLocalMedia(e, p.Markdown)
	pg, err := e.Store.AppendToPage(e.WorkspaceID, title, md, Actor)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "id": pg.ID, "title": pg.Title, "warnings": warnings}, nil
}
