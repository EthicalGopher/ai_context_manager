// Package localllm manages a local model without any third-party model
// manager: it downloads llama.cpp's own llama-server and a GGUF model into
// AICW's data folder, starts the server on a loopback port on demand, and
// stops it again. llama-server speaks the Anthropic Messages API, so Claude
// Code can be pointed at it with ANTHROPIC_BASE_URL.
package localllm

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Entry is a model AICW knows how to download.
type Entry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	File string `json:"file"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
	Note string `json:"note"`
}

const hf = "https://huggingface.co/"

// Catalog lists small instruction-tuned models that support tool calls, at
// Q4_K_M quantisation. Sizes are the exact download sizes.
func Catalog() []Entry {
	return []Entry{
		{"qwen25-coder-3b", "Qwen2.5-Coder 3B Instruct", "qwen2.5-coder-3b-instruct-q4_k_m.gguf", hf + "Qwen/Qwen2.5-Coder-3B-Instruct-GGUF/resolve/main/qwen2.5-coder-3b-instruct-q4_k_m.gguf", 2104932800, "Light and fast; fine for chat, notes and pages. Needs about 4 GB of RAM."},
		{"qwen25-coder-7b", "Qwen2.5-Coder 7B Instruct", "qwen2.5-coder-7b-instruct-q4_k_m.gguf", hf + "Qwen/Qwen2.5-Coder-7B-Instruct-GGUF/resolve/main/qwen2.5-coder-7b-instruct-q4_k_m.gguf", 4683073536, "Better at code. Needs about 8 GB of RAM or VRAM."},
		{"qwen3-4b", "Qwen3 4B", "Qwen3-4B-Q4_K_M.gguf", hf + "Qwen/Qwen3-4B-GGUF/resolve/main/Qwen3-4B-Q4_K_M.gguf", 2497280256, "General purpose with reasoning. Needs about 5 GB."},
		{"qwen3-8b", "Qwen3 8B", "Qwen3-8B-Q4_K_M.gguf", hf + "Qwen/Qwen3-8B-GGUF/resolve/main/Qwen3-8B-Q4_K_M.gguf", 5027783488, "Strongest of these for agent work. Needs about 9 GB."},
		{"llama32-3b", "Llama 3.2 3B Instruct", "Llama-3.2-3B-Instruct-Q4_K_M.gguf", hf + "bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf", 2019377696, "Small general model. Needs about 4 GB."},
	}
}

type Local struct {
	File string `json:"file"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type Status struct {
	EngineInstalled bool     `json:"engineInstalled"`
	EngineVersion   string   `json:"engineVersion"`
	Models          []Local  `json:"models"`
	Catalog         []Entry  `json:"catalog"`
	Running         string   `json:"running"` // file being served, or ""
	Dir             string   `json:"dir"`
	Notes           []string `json:"notes"`
}

type Manager struct {
	Root string // <data>/llm

	mu   sync.Mutex
	cmd  *exec.Cmd
	file string
	base string
	done chan struct{}
	busy bool // an install or download is running
}

func New(dataDir string) *Manager { return &Manager{Root: filepath.Join(dataDir, "llm")} }

func (m *Manager) modelsDir() string { return filepath.Join(m.Root, "models") }
func (m *Manager) engineDir() string { return filepath.Join(m.Root, "engine") }

func exeName() string {
	if runtime.GOOS == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
}

// EnginePath returns the installed llama-server, or "".
func (m *Manager) EnginePath() string {
	var found string
	filepath.WalkDir(m.engineDir(), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == exeName() && found == "" {
			found = p
		}
		return nil
	})
	return found
}

func (m *Manager) Status() Status {
	st := Status{Catalog: Catalog(), Dir: m.Root, Models: []Local{}, Notes: []string{}}
	if p := m.EnginePath(); p != "" {
		st.EngineInstalled = true
		if b, err := os.ReadFile(filepath.Join(m.engineDir(), "VERSION")); err == nil {
			st.EngineVersion = strings.TrimSpace(string(b))
		}
	}
	ents, _ := os.ReadDir(m.modelsDir())
	names := map[string]string{}
	for _, c := range Catalog() {
		names[c.File] = c.Name
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".gguf") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		n := names[e.Name()]
		if n == "" {
			n = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		}
		st.Models = append(st.Models, Local{File: e.Name(), Name: n, Size: fi.Size()})
	}
	sort.Slice(st.Models, func(i, j int) bool { return st.Models[i].Name < st.Models[j].Name })
	m.mu.Lock()
	st.Running = m.file
	m.mu.Unlock()
	return st
}

func (m *Manager) begin() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busy {
		return errors.New("another download is already running")
	}
	m.busy = true
	return nil
}
func (m *Manager) end() { m.mu.Lock(); m.busy = false; m.mu.Unlock() }

// ---- engine ----

// assetSuffix picks the llama.cpp release build for this machine. GPU
// (Vulkan) builds are used where a Vulkan loader exists; they fall back to the
// CPU when no device is usable.
func assetSuffix() (string, error) {
	vulkan := func(paths ...string) bool {
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return true
			}
		}
		return false
	}
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		if vulkan("/usr/lib/libvulkan.so.1", "/usr/lib64/libvulkan.so.1", "/usr/lib/x86_64-linux-gnu/libvulkan.so.1") {
			return "bin-ubuntu-vulkan-x64.tar.gz", nil
		}
		return "bin-ubuntu-x64.tar.gz", nil
	case "linux/arm64":
		return "bin-ubuntu-arm64.tar.gz", nil
	case "windows/amd64":
		if vulkan(filepath.Join(os.Getenv("SystemRoot"), "System32", "vulkan-1.dll")) {
			return "bin-win-vulkan-x64.zip", nil
		}
		return "bin-win-cpu-x64.zip", nil
	case "windows/arm64":
		return "bin-win-cpu-arm64.zip", nil
	case "darwin/arm64":
		return "bin-macos-arm64.tar.gz", nil
	case "darwin/amd64":
		return "bin-macos-x64.tar.gz", nil
	}
	return "", fmt.Errorf("no prebuilt llama.cpp server for %s/%s", runtime.GOOS, runtime.GOARCH)
}

type release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name   string `json:"name"`
		URL    string `json:"browser_download_url"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

// InstallEngine downloads the newest llama.cpp release build for this machine.
func (m *Manager) InstallEngine(ctx context.Context, progress func(msg string, done, total int64)) error {
	if err := m.begin(); err != nil {
		return err
	}
	defer m.end()
	suffix, err := assetSuffix()
	if err != nil {
		return err
	}
	progress("Looking up the latest llama.cpp release…", 0, 0)
	var rels []release
	if err := getJSON(ctx, "https://api.github.com/repos/ggml-org/llama.cpp/releases?per_page=10", &rels); err != nil {
		return fmt.Errorf("could not reach GitHub: %w", err)
	}
	for _, r := range rels {
		for _, a := range r.Assets {
			if !strings.HasSuffix(a.Name, suffix) || strings.HasPrefix(a.Name, "cudart") {
				continue
			}
			tmp := filepath.Join(m.Root, "engine-download.tmp")
			os.MkdirAll(m.Root, 0o700)
			defer os.Remove(tmp)
			progress("Downloading "+a.Name+"…", 0, a.Size)
			sum, err := download(ctx, a.URL, tmp, 0, func(d int64) { progress("Downloading "+a.Name+"…", d, a.Size) })
			if err != nil {
				return err
			}
			if want, ok := strings.CutPrefix(a.Digest, "sha256:"); ok && want != sum {
				return errors.New("the downloaded engine failed its checksum; not installing it")
			}
			progress("Unpacking…", 0, 0)
			dir := m.engineDir()
			os.RemoveAll(dir + ".new")
			if err := unpack(tmp, a.Name, dir+".new"); err != nil {
				return err
			}
			os.WriteFile(filepath.Join(dir+".new", "VERSION"), []byte(r.Tag+" ("+a.Name+")"), 0o600)
			os.RemoveAll(dir)
			if err := os.Rename(dir+".new", dir); err != nil {
				return err
			}
			if m.EnginePath() == "" {
				return errors.New("the download did not contain llama-server")
			}
			progress("Engine installed ("+r.Tag+").", 0, 0)
			return nil
		}
	}
	return fmt.Errorf("no release asset ending in %s was found", suffix)
}

// ---- models ----

// Download fetches a catalogue model into the models folder, resuming a
// partial download. The final size must match.
func (m *Manager) Download(ctx context.Context, id string, progress func(msg string, done, total int64)) error {
	var e *Entry
	for _, c := range Catalog() {
		if c.ID == id {
			c := c
			e = &c
		}
	}
	if e == nil {
		return fmt.Errorf("unknown model %q", id)
	}
	if err := m.begin(); err != nil {
		return err
	}
	defer m.end()
	if err := os.MkdirAll(m.modelsDir(), 0o700); err != nil {
		return err
	}
	final := filepath.Join(m.modelsDir(), e.File)
	part := final + ".part"
	var have int64
	if fi, err := os.Stat(part); err == nil {
		have = fi.Size()
	}
	progress("Downloading "+e.Name+"…", have, e.Size)
	if _, err := download(ctx, e.URL, part, have, func(d int64) { progress("Downloading "+e.Name+"…", d, e.Size) }); err != nil {
		return err
	}
	if fi, err := os.Stat(part); err != nil || fi.Size() != e.Size {
		return fmt.Errorf("the download is incomplete; run it again to resume")
	}
	if err := os.Rename(part, final); err != nil {
		return err
	}
	progress(e.Name+" is ready.", e.Size, e.Size)
	return nil
}

// Delete removes a downloaded model file (stopping the server if it uses it).
func (m *Manager) Delete(file string) error {
	if file != filepath.Base(file) || !strings.HasSuffix(strings.ToLower(file), ".gguf") {
		return errors.New("not a model file")
	}
	m.mu.Lock()
	running := m.file == file
	m.mu.Unlock()
	if running {
		m.Stop()
	}
	return os.Remove(filepath.Join(m.modelsDir(), file))
}

// ---- server ----

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Ensure makes sure llama-server is serving file and returns its base URL.
func (m *Manager) Ensure(ctx context.Context, file string) (string, error) {
	if file != filepath.Base(file) || !strings.HasSuffix(strings.ToLower(file), ".gguf") {
		return "", errors.New("not a model file")
	}
	model := filepath.Join(m.modelsDir(), file)
	if _, err := os.Stat(model); err != nil {
		return "", fmt.Errorf("model %s is not downloaded (Settings → Local models)", file)
	}
	m.mu.Lock()
	if m.cmd != nil && m.file == file {
		select {
		case <-m.done:
		default:
			base := m.base
			m.mu.Unlock()
			return base, nil
		}
	}
	m.mu.Unlock()
	m.Stop()

	exe := m.EnginePath()
	if exe == "" {
		return "", errors.New("the local engine is not installed (Settings → Local models)")
	}
	port, err := freePort()
	if err != nil {
		return "", err
	}
	os.MkdirAll(m.Root, 0o700)
	logf, err := os.OpenFile(filepath.Join(m.Root, "server.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	// 32k tokens: Claude Code's own instructions and tool list take about 20k.
	cmd := exec.Command(exe, "-m", model, "--host", "127.0.0.1", "--port", fmt.Sprint(port), "--jinja", "-c", "32768", "-ngl", "99", "--alias", "local")
	cmd.Dir = filepath.Dir(exe)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Dir(exe)+string(os.PathListSeparator)+os.Getenv("LD_LIBRARY_PATH"))
	cmd.Stdout, cmd.Stderr = logf, logf
	if err := cmd.Start(); err != nil {
		logf.Close()
		return "", fmt.Errorf("could not start the local engine: %w", err)
	}
	done := make(chan struct{})
	go func() { cmd.Wait(); logf.Close(); close(done) }()
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(180 * time.Second)
	for {
		select {
		case <-done:
			return "", fmt.Errorf("the local engine stopped while loading %s:\n%s", file, m.logTail())
		case <-ctx.Done():
			cmd.Process.Kill()
			return "", ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
		if healthy(base) {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			return "", fmt.Errorf("the local engine did not become ready in 3 minutes:\n%s", m.logTail())
		}
	}
	m.mu.Lock()
	m.cmd, m.file, m.base, m.done = cmd, file, base, done
	m.mu.Unlock()
	return base, nil
}

func healthy(base string) bool {
	c := http.Client{Timeout: time.Second}
	resp, err := c.Get(base + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (m *Manager) logTail() string {
	b, _ := os.ReadFile(filepath.Join(m.Root, "server.log"))
	s := strings.TrimSpace(string(b))
	if len(s) > 1500 {
		s = "…" + s[len(s)-1500:]
	}
	return s
}

// Stop shuts the server down.
func (m *Manager) Stop() {
	m.mu.Lock()
	cmd, done := m.cmd, m.done
	m.cmd, m.file, m.base, m.done = nil, "", "", nil
	m.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Kill()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

// ---- helpers ----

func getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

type counter struct {
	w    io.Writer
	n    int64
	tick func(int64)
	last time.Time
}

func (c *counter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	if time.Since(c.last) > 300*time.Millisecond {
		c.last = time.Now()
		c.tick(c.n)
	}
	return n, err
}

// download streams url into path (appending from offset when the server
// honours Range) and returns the SHA-256 of the complete file.
func download(ctx context.Context, url, path string, offset int64, tick func(int64)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	flags := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK:
		flags |= os.O_TRUNC
		offset = 0
	default:
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return "", err
	}
	cw := &counter{w: f, n: offset, tick: tick}
	_, err = io.Copy(cw, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	tick(cw.n)
	rf, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer rf.Close()
	h := sha256.New()
	io.Copy(h, rf)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// unpack extracts a .tar.gz or .zip into dir, refusing paths that escape it.
func unpack(archive, name, dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dest := func(rel string) (string, error) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if p != dir && !strings.HasPrefix(p, dir+string(os.PathSeparator)) {
			return "", fmt.Errorf("unsafe path %q in archive", rel)
		}
		return p, nil
	}
	write := func(p string, r io.Reader, mode os.FileMode) error {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o600)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, r)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	}
	if strings.HasSuffix(name, ".zip") {
		zr, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer zr.Close()
		for _, zf := range zr.File {
			p, err := dest(zf.Name)
			if err != nil {
				return err
			}
			if zf.FileInfo().IsDir() {
				os.MkdirAll(p, 0o700)
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			err = write(p, rc, zf.Mode())
			rc.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	links := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		p, err := dest(h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(p, 0o700)
		case tar.TypeReg:
			if err := write(p, tr, os.FileMode(h.Mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			links[p] = h.Linkname // shared-library version links (libfoo.so -> libfoo.so.1)
		}
	}
	for p, target := range links {
		if filepath.IsAbs(target) || strings.Contains(target, "..") {
			continue
		}
		os.Remove(p)
		os.Symlink(target, p)
	}
	return nil
}
