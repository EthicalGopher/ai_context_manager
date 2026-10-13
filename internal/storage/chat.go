package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Roles and sources for chat messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	SourceUI      = "ui"
	SourceAgent   = "agent"
)

type Conversation struct {
	ID          int64     `json:"id"`
	WorkspaceID int64     `json:"workspaceId"`
	Title       string    `json:"title"`
	Agent       string    `json:"agent"`
	SessionID   int64     `json:"sessionId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Messages    int       `json:"messages"`
}

type Message struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversationId"`
	Role           string    `json:"role"`
	Source         string    `json:"source"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"createdAt"`
	Media          []Media   `json:"media"`
}

const convCols = `c.id,c.workspace_id,c.title,c.agent,c.session_id,c.created_at,c.updated_at,(SELECT COUNT(*) FROM messages m WHERE m.conversation_id=c.id)`

func scanConv(r interface{ Scan(...any) error }) (*Conversation, error) {
	var c Conversation
	var ca, ua int64
	if err := r.Scan(&c.ID, &c.WorkspaceID, &c.Title, &c.Agent, &c.SessionID, &ca, &ua, &c.Messages); err != nil {
		return nil, err
	}
	c.CreatedAt, c.UpdatedAt = fromTS(ca), fromTS(ua)
	return &c, nil
}

func (s *Store) CreateConversation(wsID int64, title, agent string) (*Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New chat"
	}
	now := ts(s.now())
	res, err := s.db.Exec(`INSERT INTO conversations(workspace_id,title,agent,created_at,updated_at) VALUES(?,?,?,?,?)`, wsID, title, agent, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Conversation(id)
}

func (s *Store) Conversation(id int64) (*Conversation, error) {
	return scanConv(s.db.QueryRow(`SELECT `+convCols+` FROM conversations c WHERE c.id=?`, id))
}

func (s *Store) Conversations(wsID int64) ([]Conversation, error) {
	rows, err := s.db.Query(`SELECT `+convCols+` FROM conversations c WHERE c.workspace_id=? ORDER BY c.updated_at DESC, c.id DESC`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Conversation{}
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Store) UpdateConversation(id int64, title, agent *string, sessionID *int64) error {
	c, err := s.Conversation(id)
	if err != nil {
		return err
	}
	if title != nil && strings.TrimSpace(*title) != "" {
		c.Title = strings.TrimSpace(*title)
	}
	if agent != nil {
		c.Agent = *agent
	}
	if sessionID != nil {
		c.SessionID = *sessionID
	}
	_, err = s.db.Exec(`UPDATE conversations SET title=?, agent=?, session_id=? WHERE id=?`, c.Title, c.Agent, c.SessionID, id)
	return err
}

func (s *Store) DeleteConversation(id int64) error {
	_, err := s.db.Exec(`DELETE FROM conversations WHERE id=?`, id)
	return err
}

// AddMessage appends a message and attaches media that belongs to the same
// workspace as the conversation.
func (s *Store) AddMessage(convID int64, role, source, body string, mediaIDs []int64) (*Message, error) {
	c, err := s.Conversation(convID)
	if err != nil {
		return nil, errors.New("unknown conversation")
	}
	now := ts(s.now())
	res, err := s.db.Exec(`INSERT INTO messages(conversation_id,role,source,body,created_at) VALUES(?,?,?,?,?)`, convID, role, source, body, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	for _, mid := range mediaIDs {
		m, err := s.Media(mid)
		if err != nil || m.WorkspaceID != c.WorkspaceID {
			continue // silently ignore foreign/missing media
		}
		s.db.Exec(`INSERT OR IGNORE INTO message_media(message_id,media_id) VALUES(?,?)`, id, mid)
	}
	s.db.Exec(`UPDATE conversations SET updated_at=? WHERE id=?`, now, convID)
	return s.Message(id)
}

func (s *Store) Message(id int64) (*Message, error) {
	var m Message
	var ca int64
	err := s.db.QueryRow(`SELECT id,conversation_id,role,source,body,created_at FROM messages WHERE id=?`, id).Scan(&m.ID, &m.ConversationID, &m.Role, &m.Source, &m.Body, &ca)
	if err != nil {
		return nil, err
	}
	m.CreatedAt = fromTS(ca)
	m.Media, err = s.messageMedia(m.ID)
	return &m, err
}

func (s *Store) messageMedia(msgID int64) ([]Media, error) {
	rows, err := s.db.Query(`SELECT `+prefixed("m.", mediaCols)+` FROM media m JOIN message_media mm ON mm.media_id=m.id WHERE mm.message_id=? ORDER BY m.id`, msgID)
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

func prefixed(p, cols string) string {
	parts := strings.Split(cols, ",")
	for i := range parts {
		parts[i] = p + parts[i]
	}
	return strings.Join(parts, ",")
}

// Messages returns messages with id > afterID (0 = from the start), oldest
// first, at most limit (0 = 500).
func (s *Store) Messages(convID, afterID int64, limit int) ([]Message, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	rows, err := s.db.Query(`SELECT id FROM messages WHERE conversation_id=? AND id>? ORDER BY id ASC LIMIT ?`, convID, afterID, limit)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	out := []Message{}
	for _, id := range ids {
		m, err := s.Message(id)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, nil
}

// ---- notes (living context documents the agent may update) ----

type Note struct {
	Name      string    `json:"name"`
	Body      string    `json:"body"`
	UpdatedAt time.Time `json:"updatedAt"`
	UpdatedBy string    `json:"updatedBy"`
}

func (s *Store) Notes(wsID int64) ([]Note, error) {
	rows, err := s.db.Query(`SELECT name,body,updated_at,updated_by FROM notes WHERE workspace_id=? ORDER BY name`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Note{}
	for rows.Next() {
		var n Note
		var u int64
		if err := rows.Scan(&n.Name, &n.Body, &u, &n.UpdatedBy); err != nil {
			return nil, err
		}
		n.UpdatedAt = fromTS(u)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) Note(wsID int64, name string) (*Note, error) {
	var n Note
	var u int64
	err := s.db.QueryRow(`SELECT name,body,updated_at,updated_by FROM notes WHERE workspace_id=? AND name=?`, wsID, name).Scan(&n.Name, &n.Body, &u, &n.UpdatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	n.UpdatedAt = fromTS(u)
	return &n, err
}

// PutNote replaces a note, keeping the previous body as a version.
func (s *Store) PutNote(wsID int64, name, body, by string) error {
	now := ts(s.now())
	if old, _ := s.Note(wsID, name); old != nil {
		if _, err := s.db.Exec(`INSERT INTO note_versions(workspace_id,name,body,saved_at,saved_by) VALUES(?,?,?,?,?)`, wsID, name, old.Body, ts(old.UpdatedAt), old.UpdatedBy); err != nil {
			return err
		}
	}
	_, err := s.db.Exec(`INSERT INTO notes(workspace_id,name,body,updated_at,updated_by) VALUES(?,?,?,?,?)
		ON CONFLICT(workspace_id,name) DO UPDATE SET body=excluded.body, updated_at=excluded.updated_at, updated_by=excluded.updated_by`, wsID, name, body, now, by)
	return err
}

type NoteVersion struct {
	ID      int64     `json:"id"`
	Body    string    `json:"body"`
	SavedAt time.Time `json:"savedAt"`
	SavedBy string    `json:"savedBy"`
}

func (s *Store) NoteVersions(wsID int64, name string) ([]NoteVersion, error) {
	rows, err := s.db.Query(`SELECT id,body,saved_at,saved_by FROM note_versions WHERE workspace_id=? AND name=? ORDER BY id DESC LIMIT 50`, wsID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NoteVersion{}
	for rows.Next() {
		var v NoteVersion
		var t int64
		if err := rows.Scan(&v.ID, &v.Body, &t, &v.SavedBy); err != nil {
			return nil, err
		}
		v.SavedAt = fromTS(t)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) DeleteNote(wsID int64, name string) error {
	_, err := s.db.Exec(`DELETE FROM notes WHERE workspace_id=? AND name=?`, wsID, name)
	return err
}

// ---- always-on skills ----

type Skill struct {
	ID          int64     `json:"id"`
	WorkspaceID int64     `json:"workspaceId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Body        string    `json:"body"`
	AlwaysOn    bool      `json:"alwaysOn"`
	UpdatedAt   time.Time `json:"updatedAt"`
	UpdatedBy   string    `json:"updatedBy"`
}

func (s *Store) Skills(wsID int64) ([]Skill, error) {
	rows, err := s.db.Query(`SELECT id,workspace_id,name,description,body,always_on,updated_at,updated_by FROM skills WHERE workspace_id=? ORDER BY name`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Skill{}
	for rows.Next() {
		var k Skill
		var on, u int64
		if err := rows.Scan(&k.ID, &k.WorkspaceID, &k.Name, &k.Description, &k.Body, &on, &u, &k.UpdatedBy); err != nil {
			return nil, err
		}
		k.AlwaysOn, k.UpdatedAt = on != 0, fromTS(u)
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) PutSkill(wsID int64, name, desc, body string, alwaysOn bool, by string) error {
	_, err := s.db.Exec(`INSERT INTO skills(workspace_id,name,description,body,always_on,updated_at,updated_by) VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(workspace_id,name) DO UPDATE SET description=excluded.description, body=excluded.body, always_on=excluded.always_on, updated_at=excluded.updated_at, updated_by=excluded.updated_by`,
		wsID, name, desc, body, b2i(alwaysOn), ts(s.now()), by)
	return err
}

func (s *Store) DeleteSkill(wsID int64, name string) error {
	_, err := s.db.Exec(`DELETE FROM skills WHERE workspace_id=? AND name=?`, wsID, name)
	return err
}
