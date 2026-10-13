package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Page is a rich project document (Markdown with embedded media references
// like ![caption](media:12)) written by the user or by an agent.
type Page struct {
	ID          int64     `json:"id"`
	WorkspaceID int64     `json:"workspaceId"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Body        string    `json:"body"`
	Sort        int       `json:"sort"`
	UpdatedAt   time.Time `json:"updatedAt"`
	UpdatedBy   string    `json:"updatedBy"`
}

const pageCols = `id,workspace_id,title,summary,body,sort,updated_at,updated_by`

func scanPage(r interface{ Scan(...any) error }) (*Page, error) {
	var p Page
	var u int64
	if err := r.Scan(&p.ID, &p.WorkspaceID, &p.Title, &p.Summary, &p.Body, &p.Sort, &u, &p.UpdatedBy); err != nil {
		return nil, err
	}
	p.UpdatedAt = fromTS(u)
	return &p, nil
}

func (s *Store) Pages(wsID int64) ([]Page, error) {
	rows, err := s.db.Query(`SELECT `+pageCols+` FROM pages WHERE workspace_id=? ORDER BY sort, id`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Page{}
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *Store) Page(id int64) (*Page, error) {
	return scanPage(s.db.QueryRow(`SELECT `+pageCols+` FROM pages WHERE id=?`, id))
}

func (s *Store) PageByTitle(wsID int64, title string) (*Page, error) {
	p, err := scanPage(s.db.QueryRow(`SELECT `+pageCols+` FROM pages WHERE workspace_id=? AND title=?`, wsID, strings.TrimSpace(title)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// PutPage creates a page or replaces the one with the same title
// (case-insensitive), snapshotting the previous content as a version.
func (s *Store) PutPage(wsID int64, title, summary, body, by string) (*Page, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("page title is required")
	}
	now := ts(s.now())
	if old, err := s.PageByTitle(wsID, title); err != nil {
		return nil, err
	} else if old != nil {
		if _, err := s.db.Exec(`INSERT INTO page_versions(page_id,title,summary,body,saved_at,saved_by) VALUES(?,?,?,?,?,?)`, old.ID, old.Title, old.Summary, old.Body, ts(old.UpdatedAt), old.UpdatedBy); err != nil {
			return nil, err
		}
		if _, err := s.db.Exec(`UPDATE pages SET title=?, summary=?, body=?, updated_at=?, updated_by=? WHERE id=?`, title, summary, body, now, by, old.ID); err != nil {
			return nil, err
		}
		return s.Page(old.ID)
	}
	var next int
	s.db.QueryRow(`SELECT COALESCE(MAX(sort),0)+1 FROM pages WHERE workspace_id=?`, wsID).Scan(&next)
	res, err := s.db.Exec(`INSERT INTO pages(workspace_id,title,summary,body,sort,updated_at,updated_by) VALUES(?,?,?,?,?,?,?)`, wsID, title, summary, body, next, now, by)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Page(id)
}

// AppendToPage adds Markdown to the end of a page, creating it if needed.
func (s *Store) AppendToPage(wsID int64, title, markdown, by string) (*Page, error) {
	cur, err := s.PageByTitle(wsID, title)
	if err != nil {
		return nil, err
	}
	if cur == nil {
		return s.PutPage(wsID, title, "", markdown, by)
	}
	if strings.TrimSpace(cur.Body) == "" {
		return s.PutPage(wsID, cur.Title, cur.Summary, markdown, by)
	}
	return s.PutPage(wsID, cur.Title, cur.Summary, strings.TrimRight(cur.Body, "\n")+"\n\n"+markdown, by)
}

// ReorderPages sets the display order to the given ID sequence.
func (s *Store) ReorderPages(wsID int64, ids []int64) error {
	for i, id := range ids {
		if _, err := s.db.Exec(`UPDATE pages SET sort=? WHERE id=? AND workspace_id=?`, i+1, id, wsID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeletePage(id int64) error {
	_, err := s.db.Exec(`DELETE FROM pages WHERE id=?`, id)
	return err
}

type PageVersion struct {
	ID      int64     `json:"id"`
	Title   string    `json:"title"`
	Summary string    `json:"summary"`
	Body    string    `json:"body"`
	SavedAt time.Time `json:"savedAt"`
	SavedBy string    `json:"savedBy"`
}

func (s *Store) PageVersions(pageID int64) ([]PageVersion, error) {
	rows, err := s.db.Query(`SELECT id,title,summary,body,saved_at,saved_by FROM page_versions WHERE page_id=? ORDER BY id DESC LIMIT 50`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PageVersion{}
	for rows.Next() {
		var v PageVersion
		var t int64
		if err := rows.Scan(&v.ID, &v.Title, &v.Summary, &v.Body, &t, &v.SavedBy); err != nil {
			return nil, err
		}
		v.SavedAt = fromTS(t)
		out = append(out, v)
	}
	return out, rows.Err()
}
