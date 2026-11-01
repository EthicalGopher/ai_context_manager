package service

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"aicw/internal/markdown"
	"aicw/internal/media"
	"aicw/internal/safety"
	"aicw/internal/storage"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (s *Service) ListPages(wsID int64) ([]storage.Page, error) { return s.Store.Pages(wsID) }

func (s *Service) GetPage(id int64) (*storage.Page, error) { return s.Store.Page(id) }

func (s *Service) SavePage(wsID int64, title, summary, body string) (*storage.Page, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 120 {
		return nil, errors.New("title must be 1-120 characters")
	}
	if len(body) > 256*1024 || len(summary) > 500 {
		return nil, errors.New("page is too large (256 KB body, 500 characters summary)")
	}
	if _, err := s.Store.Workspace(wsID); err != nil {
		return nil, errors.New("unknown workspace")
	}
	return s.Store.PutPage(wsID, title, strings.TrimSpace(summary), body, "user")
}

// RenamePage changes a page's title keeping its content and history.
func (s *Service) RenamePage(id int64, title string) (*storage.Page, error) {
	p, err := s.Store.Page(id)
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 120 {
		return nil, errors.New("title must be 1-120 characters")
	}
	if other, _ := s.Store.PageByTitle(p.WorkspaceID, title); other != nil && other.ID != p.ID {
		return nil, errors.New("another page already has that title")
	}
	return s.Store.PutPage(p.WorkspaceID, title, p.Summary, p.Body, "user")
}

func (s *Service) DeletePage(id int64, confirmed bool) error {
	if !confirmed {
		return markdown.ErrNotConfirmed
	}
	return s.Store.DeletePage(id)
}

func (s *Service) ReorderPages(wsID int64, ids []int64) error { return s.Store.ReorderPages(wsID, ids) }

func (s *Service) PageVersions(id int64) ([]storage.PageVersion, error) {
	return s.Store.PageVersions(id)
}

// SaveMessageToPage appends a chat message (text and attachments) to a page,
// creating the page if needed, so conversations become browsable documents.
func (s *Service) SaveMessageToPage(messageID int64, pageTitle string) (*storage.Page, error) {
	m, err := s.Store.Message(messageID)
	if err != nil {
		return nil, errors.New("message not found")
	}
	conv, err := s.Store.Conversation(m.ConversationID)
	if err != nil {
		return nil, err
	}
	pageTitle = strings.TrimSpace(pageTitle)
	if pageTitle == "" || len(pageTitle) > 120 {
		return nil, errors.New("choose a page title (up to 120 characters)")
	}
	md := pageMarkdown(m.Media, m.Body)
	if md == "" {
		return nil, errors.New("this message is empty")
	}
	return s.Store.AppendToPage(conv.WorkspaceID, pageTitle, md, "user")
}

// ---- project file browser (read-only) ----

type FileEntry struct {
	Name   string `json:"name"`
	Rel    string `json:"rel"`
	Dir    bool   `json:"dir"`
	Size   int64  `json:"size"`
	Secret bool   `json:"secret"`
}

var hiddenDirs = map[string]bool{".git": true, "node_modules": true, ".venv": true, "__pycache__": true, ".cache": true}

// ListDir lists one directory of the project. Paths are validated against the
// workspace root; symlinks that leave it are not followed.
func (s *Service) ListDir(wsID int64, rel string, showAll bool) ([]FileEntry, error) {
	w, err := s.wsPath(wsID)
	if err != nil {
		return nil, err
	}
	dir, err := safety.SafeJoin(w.Path, filepath.Clean("./"+rel))
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []FileEntry{}
	for _, e := range ents {
		name := e.Name()
		if !showAll && (hiddenDirs[name] || strings.HasPrefix(name, ".") && name != ".claude" && name != ".github" && name != ".agents") {
			continue
		}
		full := filepath.Join(dir, name)
		if e.Type()&os.ModeSymlink != 0 {
			if real, err := filepath.EvalSymlinks(full); err != nil || !safety.Contains(w.Path, real) {
				continue
			}
		}
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		r, _ := filepath.Rel(w.Path, full)
		out = append(out, FileEntry{Name: name, Rel: filepath.ToSlash(r), Dir: info.IsDir(), Size: info.Size(), Secret: !info.IsDir() && safety.IsSecretFile(name)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// ImportProjectFile copies a file from the project into the media library so
// it can be embedded in pages and chats.
func (s *Service) ImportProjectFile(wsID int64, rel string) (*storage.Media, error) {
	w, err := s.wsPath(wsID)
	if err != nil {
		return nil, err
	}
	full, err := safety.SafeJoin(w.Path, filepath.Clean("./"+rel))
	if err != nil {
		return nil, err
	}
	im, err := media.Import(full, s.mediaDir(wsID), media.Unique())
	if err != nil {
		return nil, err
	}
	return s.Store.AddMedia(storage.Media{WorkspaceID: wsID, Name: im.Name, Stored: im.Stored, Kind: im.Kind, Mime: im.Mime, Size: im.Size})
}

// pageMarkdown renders attachments (as embeds) followed by text.
func pageMarkdown(ms []storage.Media, text string) string {
	var b strings.Builder
	for _, md := range ms {
		name := strings.NewReplacer("[", "(", "]", ")").Replace(md.Name)
		switch md.Kind {
		case "image", "video", "audio":
			fmt.Fprintf(&b, "![%s](media:%d)\n\n", name, md.ID)
		default:
			fmt.Fprintf(&b, "[%s](media:%d)\n\n", name, md.ID)
		}
		if md.Kind == "audio" && strings.TrimSpace(md.Note) != "" {
			fmt.Fprintf(&b, "> %s\n\n", strings.Join(strings.Fields(md.Note), " "))
		}
	}
	if t := strings.TrimSpace(text); t != "" {
		b.WriteString(t + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// AddToPage appends what the user composed (text and/or attachments) to a page.
func (s *Service) AddToPage(pageID int64, text string, mediaIDs []int64) (*storage.Page, error) {
	p, err := s.Store.Page(pageID)
	if err != nil {
		return nil, errors.New("page not found")
	}
	if len(text) > 64*1024 {
		return nil, errors.New("text is too long (64 KB max)")
	}
	var ms []storage.Media
	for _, id := range mediaIDs {
		if m, err := s.Store.Media(id); err == nil && m.WorkspaceID == p.WorkspaceID {
			ms = append(ms, *m)
		}
	}
	md := pageMarkdown(ms, text)
	if md == "" {
		return nil, errors.New("write something or attach a file")
	}
	return s.Store.AppendToPage(p.WorkspaceID, p.Title, md, "user")
}

// PageExport is what the front end renders for a shareable HTML copy of a
// page: the body markup (media referenced as "/media/<id>"), the stylesheet and
// the viewer script. The backend embeds the media and writes one file.
type PageExport struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	CSS   string `json:"css"`
	JS    string `json:"js"`
}

// maxExportMedia caps the raw media embedded in one shareable HTML file.
const maxExportMedia = 200 << 20

var exportMediaRef = regexp.MustCompile(`="/media/(\d+)"`)

// ExportPageHTML asks where to save and writes the page as one self-contained
// .html file (media embedded) that can be sent to anyone.
func (s *Service) ExportPageHTML(id int64, x PageExport) (string, error) {
	p, err := s.Store.Page(id)
	if err != nil {
		return "", err
	}
	if s.ctx == nil {
		return "", errors.New("no window")
	}
	name := strings.TrimSuffix(media.SafeName(p.Title), ".") + ".html"
	dest, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{Title: "Download page as HTML", DefaultFilename: name})
	if err != nil || dest == "" {
		return "", err
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	if err := s.writePageHTML(f, p, x); err != nil {
		f.Close()
		os.Remove(dest)
		return "", err
	}
	return dest, f.Close()
}

func (s *Service) writePageHTML(w io.Writer, p *storage.Page, x PageExport) error {
	if len(x.Body) > 8<<20 || len(x.CSS) > 256<<10 || len(x.JS) > 256<<10 {
		return errors.New("page export is too large")
	}
	refs := exportMediaRef.FindAllStringSubmatchIndex(x.Body, -1)
	files := make([]*storage.Media, len(refs))
	var total int64
	for i, r := range refs {
		mid, _ := strconv.ParseInt(x.Body[r[2]:r[3]], 10, 64)
		m, err := s.Store.Media(mid)
		if err != nil || m.WorkspaceID != p.WorkspaceID || !safety.Contains(filepath.Join(s.DataDir, "media"), m.Stored) {
			continue // deleted or foreign media: leave an empty source
		}
		files[i] = m
		total += m.Size
	}
	if total > maxExportMedia {
		return fmt.Errorf("the media on this page is %d MB; a single HTML file can embed at most %d MB. Remove or shorten some videos and try again", total>>20, maxExportMedia>>20)
	}
	bw := bufio.NewWriterSize(w, 1<<20)
	fmt.Fprintf(bw, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">`+
		`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data: blob:; media-src data: blob:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src data: blob:">`+
		`<title>%s</title><style>%s</style></head><body>`, html.EscapeString(p.Title), strings.ReplaceAll(x.CSS, "</style", `<\/style`))
	last := 0
	for i, r := range refs {
		bw.WriteString(x.Body[last:r[0]])
		last = r[1]
		m := files[i]
		if m == nil {
			bw.WriteString(`=""`)
			continue
		}
		f, err := os.Open(m.Stored)
		if err != nil {
			bw.WriteString(`=""`)
			continue
		}
		fmt.Fprintf(bw, `="data:%s;base64,`, html.EscapeString(m.Mime))
		enc := base64.NewEncoder(base64.StdEncoding, bw)
		_, cerr := io.Copy(enc, f)
		f.Close()
		if cerr != nil {
			return cerr
		}
		enc.Close()
		bw.WriteString(`"`)
	}
	bw.WriteString(x.Body[last:])
	fmt.Fprintf(bw, `<script>%s</script></body></html>`, strings.ReplaceAll(x.JS, "</script", `<\/script`))
	return bw.Flush()
}
