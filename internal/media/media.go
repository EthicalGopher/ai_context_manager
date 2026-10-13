// Package media stores user-chosen images, videos, audio, PDFs and other files
// as private copies in the app data directory. Originals are never moved or
// modified. Content is treated as opaque data and is never executed.
package media

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aicw/internal/safety"
)

// MaxBytes is the per-file import limit.
const MaxBytes int64 = 2 << 30

var kinds = map[string]string{
	".png": "image", ".jpg": "image", ".jpeg": "image", ".gif": "image", ".webp": "image", ".svg": "image", ".bmp": "image", ".avif": "image",
	".mp4": "video", ".webm": "video", ".mov": "video", ".mkv": "video", ".m4v": "video", ".ogv": "video",
	".mp3": "audio", ".wav": "audio", ".ogg": "audio", ".m4a": "audio", ".flac": "audio", ".opus": "audio",
	".pdf": "pdf",
}

// Kind classifies by extension: image, video, audio, pdf or file.
func Kind(name string) string {
	if k, ok := kinds[strings.ToLower(filepath.Ext(name))]; ok {
		return k
	}
	return "file"
}

// MimeFor returns a content type for a stored file.
func MimeFor(name, path string) string {
	if m := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); m != "" {
		return m
	}
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		return http.DetectContentType(buf[:n])
	}
	return "application/octet-stream"
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._ -]+`)

// SafeName makes a display file name safe for use inside the store.
func SafeName(name string) string {
	n := unsafeName.ReplaceAllString(filepath.Base(name), "_")
	n = strings.Trim(n, ". ")
	if n == "" {
		n = "file"
	}
	if len(n) > 120 {
		ext := filepath.Ext(n)
		n = n[:120-len(ext)] + ext
	}
	return n
}

type Imported struct {
	Name   string
	Stored string
	Kind   string
	Mime   string
	Size   int64
}

// Import copies src into dir. dir is created with owner-only permissions.
// Directories, symlinks to directories, oversize files and likely secret files
// (.env, keys, …) are refused.
func Import(src, dir string, uniq string) (*Imported, error) {
	real, err := filepath.EvalSymlinks(src)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(real)
	if err != nil {
		return nil, err
	}
	switch {
	case st.IsDir():
		return nil, errors.New("directories cannot be imported; choose files")
	case !st.Mode().IsRegular():
		return nil, errors.New("not a regular file")
	case st.Size() > MaxBytes:
		return nil, fmt.Errorf("file is larger than the %d GB limit", MaxBytes>>30)
	case safety.IsSecretFile(real):
		return nil, errors.New("looks like a secrets file; refusing to store it")
	}
	name := SafeName(src)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	dst := filepath.Join(dir, uniq+"-"+name)
	in, err := os.Open(real)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dst)
		return nil, err
	}
	return &Imported{Name: name, Stored: dst, Kind: Kind(name), Mime: MimeFor(name, dst), Size: n}, nil
}

// Unique returns a short random prefix for stored file names.
func Unique() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}
