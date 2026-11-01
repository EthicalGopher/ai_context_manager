package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aicw/internal/storage"
)

func exportFixture(t *testing.T, size int64) (*Service, *storage.Page, *storage.Media) {
	s := newSvc(t)
	ws, err := s.OpenWorkspace(project(t), false)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.DataDir, "media")
	os.MkdirAll(dir, 0o755)
	file := filepath.Join(dir, "clip.mp4")
	os.WriteFile(file, []byte("VIDEODATA"), 0o644)
	m, err := s.Store.AddMedia(storage.Media{WorkspaceID: ws.Workspace.ID, Name: "clip.mp4", Stored: file, Kind: "video", Mime: "video/mp4", Size: size})
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("## Hello\n\n<script>alert(1)</script>\n\n![Demo](media:%d)\n\n[ext](https://evil.example)", m.ID)
	p, err := s.Store.PutPage(ws.Workspace.ID, "My: Page", "A summary", body, "user")
	if err != nil {
		t.Fatal(err)
	}
	return s, p, m
}

func TestExportPageSingleHTML(t *testing.T) {
	s, p, _ := exportFixture(t, 9)
	name, write, err := s.pageExport(p.ID)
	if err != nil || !strings.HasSuffix(name, ".html") {
		t.Fatalf("%q %v", name, err)
	}
	var b bytes.Buffer
	if err := write(&b); err != nil {
		t.Fatal(err)
	}
	h := b.String()
	for _, want := range []string{"<title>My: Page</title>", "A summary", "<h2>Hello</h2>", `data-kind="video" data-src="data:video/mp4;base64,`, "aicwMountViewer", ".mv-stage", "Zoom in"} {
		if !strings.Contains(h, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(h, "<script>alert(1)") {
		t.Error("raw HTML in a page must be escaped")
	}
	if strings.Contains(h, `href="https://evil.example"`) {
		t.Error("external links must not be live")
	}
}

func TestExportPageZipWhenMediaIsBig(t *testing.T) {
	s, p, m := exportFixture(t, 100<<20)
	name, write, err := s.pageExport(p.ID)
	if err != nil || !strings.HasSuffix(name, ".zip") {
		t.Fatalf("%q %v", name, err)
	}
	var b bytes.Buffer
	if err := write(&b); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range zr.File {
		got[f.Name] = true
	}
	if !got["index.html"] || !got[fmt.Sprintf("media/%d-clip.mp4", m.ID)] {
		t.Fatalf("zip contents %v", got)
	}
}

func TestExportPageIgnoresForeignMedia(t *testing.T) {
	s, p, _ := exportFixture(t, 9)
	other, _ := s.OpenWorkspace(project(t), false)
	secret := filepath.Join(s.DataDir, "media", "secret.png")
	os.WriteFile(secret, []byte("SECRET"), 0o644)
	f, _ := s.Store.AddMedia(storage.Media{WorkspaceID: other.Workspace.ID, Name: "s.png", Stored: secret, Kind: "image", Mime: "image/png", Size: 6})
	s.Store.PutPage(p.WorkspaceID, p.Title, p.Summary, fmt.Sprintf("![x](media:%d)", f.ID), "user")
	_, write, _ := s.pageExport(p.ID)
	var b bytes.Buffer
	write(&b)
	if strings.Contains(b.String(), "base64,") || !strings.Contains(b.String(), "missing media") {
		t.Fatal("media from another workspace leaked into the export")
	}
}
