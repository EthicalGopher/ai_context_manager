package localllm

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCatalogIsSane(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Catalog() {
		if seen[c.ID] || seen[c.File] || !strings.HasPrefix(c.URL, "https://huggingface.co/") || !strings.HasSuffix(c.File, ".gguf") || c.Size < 1e9 || c.File != filepath.Base(c.File) {
			t.Fatalf("bad entry %+v", c)
		}
		seen[c.ID], seen[c.File] = true, true
	}
}

func tgz(t *testing.T, files map[string]string, links map[string]string) string {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for n, b := range files {
		tw.WriteHeader(&tar.Header{Name: n, Mode: 0o755, Size: int64(len(b)), Typeflag: tar.TypeReg})
		tw.Write([]byte(b))
	}
	for n, target := range links {
		tw.WriteHeader(&tar.Header{Name: n, Linkname: target, Typeflag: tar.TypeSymlink})
	}
	tw.Close()
	gz.Close()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	os.WriteFile(p, buf.Bytes(), 0o600)
	return p
}

func TestUnpackFindsServerAndRefusesEscapes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "engine")
	a := tgz(t, map[string]string{"llama-b1/llama-server": "bin", "llama-b1/libx.so.1": "lib"}, map[string]string{"llama-b1/libx.so": "libx.so.1"})
	if err := unpack(a, "a.tar.gz", dir); err != nil {
		t.Fatal(err)
	}
	m := &Manager{Root: filepath.Dir(dir)}
	if m.EnginePath() == "" {
		t.Fatal("llama-server not found")
	}
	if l, err := os.Readlink(filepath.Join(dir, "llama-b1", "libx.so")); err != nil || l != "libx.so.1" {
		t.Fatalf("symlink %q %v", l, err)
	}
	bad := tgz(t, map[string]string{"../evil": "x"}, nil)
	if err := unpack(bad, "a.tar.gz", filepath.Join(t.TempDir(), "e")); err == nil {
		t.Fatal("path traversal must be refused")
	}
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	w, _ := zw.Create("../../evil.txt")
	w.Write([]byte("x"))
	zw.Close()
	zp := filepath.Join(t.TempDir(), "a.zip")
	os.WriteFile(zp, zb.Bytes(), 0o600)
	if err := unpack(zp, "a.zip", filepath.Join(t.TempDir(), "z")); err == nil {
		t.Fatal("zip traversal must be refused")
	}
}

func TestDownloadResumesAndVerifiesSize(t *testing.T) {
	body := strings.Repeat("0123456789", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "m", time.Time{}, strings.NewReader(body))
	}))
	defer srv.Close()
	p := filepath.Join(t.TempDir(), "m.part")
	os.WriteFile(p, []byte(body[:4000]), 0o600)
	var last int64
	if _, err := download(context.Background(), srv.URL, p, 4000, func(d int64) { last = d }); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != body || last != int64(len(body)) {
		t.Fatalf("resume produced %d bytes (reported %d)", len(got), last)
	}
}

func TestEnsureStartsServerOnceAndStopsIt(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("needs a shell")
	}
	m := New(t.TempDir())
	os.MkdirAll(m.modelsDir(), 0o700)
	os.WriteFile(filepath.Join(m.modelsDir(), "tiny.gguf"), []byte("x"), 0o600)
	os.MkdirAll(m.engineDir(), 0o700)
	// A stand-in llama-server: serves /health on the requested port.
	script := "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" = --port ] && port=$2; shift; done\nexec python3 -I -m http.server $port --bind 127.0.0.1 >/dev/null 2>&1\n"
	os.WriteFile(filepath.Join(m.engineDir(), exeName()), []byte(script), 0o755)
	os.MkdirAll(filepath.Join(m.engineDir()), 0o700)
	// python's http.server answers GET /health with 404, so provide a file named health.
	os.WriteFile(filepath.Join(m.engineDir(), "health"), []byte("ok"), 0o600)
	base, err := m.Ensure(context.Background(), "tiny.gguf")
	if err != nil {
		t.Fatal(err)
	}
	base2, _ := m.Ensure(context.Background(), "tiny.gguf")
	if base != base2 || m.Status().Running != "tiny.gguf" {
		t.Fatalf("the running server must be reused: %q %q", base, base2)
	}
	m.Stop()
	if m.Status().Running != "" {
		t.Fatal("stopped")
	}
	if _, err := m.Ensure(context.Background(), "../x.gguf"); err == nil {
		t.Fatal("path must be refused")
	}
	if _, err := m.Ensure(context.Background(), "missing.gguf"); err == nil {
		t.Fatal("missing model must be reported")
	}
}
