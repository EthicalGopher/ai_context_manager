package service

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aicw/internal/storage"
)

func TestWritePageHTMLEmbedsMedia(t *testing.T) {
	s := newSvc(t)
	ws, _ := s.OpenWorkspace(project(t), false)
	dir := filepath.Join(s.DataDir, "media")
	os.MkdirAll(dir, 0o755)
	stored := filepath.Join(dir, "a.mp4")
	os.WriteFile(stored, []byte("VIDEO-BYTES"), 0o644)
	m, err := s.Store.AddMedia(storage.Media{WorkspaceID: ws.Workspace.ID, Name: "a.mp4", Stored: stored, Kind: "video", Mime: "video/mp4", Size: 11})
	if err != nil {
		t.Fatal(err)
	}
	other, _ := s.OpenWorkspace(project(t), false)
	foreign, _ := s.Store.AddMedia(storage.Media{WorkspaceID: other.Workspace.ID, Name: "b.mp4", Stored: stored, Kind: "video", Mime: "video/mp4", Size: 11})
	p, _ := s.Store.PutPage(ws.Workspace.ID, "Demo <b>", "", "x", "user")

	body := `<div class="mv" data-src="/media/` + itoa(m.ID) + `"></div><img src="/media/` + itoa(foreign.ID) + `">`
	var out bytes.Buffer
	if err := s.writePageHTML(&out, p, PageExport{Body: body, CSS: "body{}", JS: "var a=1;</script>"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, `data-src="data:video/mp4;base64,`+base64.StdEncoding.EncodeToString([]byte("VIDEO-BYTES"))+`"`) {
		t.Fatalf("media not embedded: %s", got)
	}
	if !strings.Contains(got, `<img src="">`) || strings.Contains(got, "/media/") {
		t.Fatalf("media from another workspace must not be embedded: %s", got)
	}
	if !strings.Contains(got, "<title>Demo &lt;b&gt;</title>") || strings.Count(got, "</script>") != 1 {
		t.Fatalf("title or script not escaped: %s", got)
	}
}

func TestWritePageHTMLRefusesHugeMedia(t *testing.T) {
	s := newSvc(t)
	ws, _ := s.OpenWorkspace(project(t), false)
	dir := filepath.Join(s.DataDir, "media")
	os.MkdirAll(dir, 0o755)
	stored := filepath.Join(dir, "big.mp4")
	os.WriteFile(stored, []byte("x"), 0o644)
	m, _ := s.Store.AddMedia(storage.Media{WorkspaceID: ws.Workspace.ID, Name: "big.mp4", Stored: stored, Kind: "video", Mime: "video/mp4", Size: maxExportMedia + 1})
	p, _ := s.Store.PutPage(ws.Workspace.ID, "Big", "", "x", "user")
	err := s.writePageHTML(&bytes.Buffer{}, p, PageExport{Body: `<div data-src="/media/` + itoa(m.ID) + `"></div>`})
	if err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("expected a size error, got %v", err)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
