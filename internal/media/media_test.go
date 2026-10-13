package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKindAndSafeName(t *testing.T) {
	for n, want := range map[string]string{"a.PNG": "image", "b.mp4": "video", "c.mp3": "audio", "d.pdf": "pdf", "e.zip": "file", "noext": "file"} {
		if Kind(n) != want {
			t.Errorf("%s -> %s", n, Kind(n))
		}
	}
	if got := SafeName("../../etc/pass wd?.png"); got != "pass wd_.png" {
		t.Errorf("%q", got)
	}
	if SafeName("...") != "file" {
		t.Error("empty name fallback")
	}
}

func TestImportCopiesAndRefuses(t *testing.T) {
	src, dir := t.TempDir(), filepath.Join(t.TempDir(), "store")
	png := filepath.Join(src, "shot one.png")
	os.WriteFile(png, []byte("\x89PNG\r\n\x1a\nxxxx"), 0o644)
	im, err := Import(png, dir, "u1")
	if err != nil || im.Kind != "image" || im.Mime != "image/png" || im.Size != 12 {
		t.Fatalf("%+v %v", im, err)
	}
	if _, err := os.Stat(png); err != nil {
		t.Fatal("original must remain")
	}
	if st, _ := os.Stat(im.Stored); st.Mode().Perm() != 0o600 {
		t.Fatal("stored copy must be private")
	}
	if st, _ := os.Stat(dir); st.Mode().Perm() != 0o700 {
		t.Fatal("store dir must be private")
	}
	if _, err := Import(src, dir, "u2"); err == nil {
		t.Fatal("directory accepted")
	}
	env := filepath.Join(src, ".env")
	os.WriteFile(env, []byte("A=1"), 0o644)
	if _, err := Import(env, dir, "u3"); err == nil {
		t.Fatal("secret file accepted")
	}
	if _, err := Import(filepath.Join(src, "missing.png"), dir, "u4"); err == nil {
		t.Fatal("missing file accepted")
	}
	if _, err := Import(png, dir, "u1"); err == nil {
		t.Fatal("must not overwrite an existing stored file")
	}
}
