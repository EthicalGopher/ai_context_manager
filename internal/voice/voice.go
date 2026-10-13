// Package voice records microphone audio with the system's own tools and turns
// it into text with a local speech-to-text engine. Audio never leaves the
// machine and the temporary recording is deleted after transcription.
package voice

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
)

//go:embed transcribe.py
var transcribeScript []byte

// Settings select the engine. All fields are optional.
type Settings struct {
	Command  string `json:"command"`  // custom: argv template with {audio}, split on spaces (quotes honoured)
	Model    string `json:"model"`    // faster-whisper / whisper model name (default "base")
	Language string `json:"language"` // "auto" or a code like "en"
}

type Engine struct {
	Name string   `json:"name"`
	Argv []string `json:"-"`
	Temp bool     `json:"-"`
}

// Recorder wraps one recording process.
type Recorder struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	file string
	out  bytes.Buffer
	// Argv overrides the recorder command (tests). {file} is substituted.
	Argv []string
}

var recorders = [][]string{
	{"pw-record", "--rate", "16000", "--channels", "1", "--format", "s16", "{file}"},
	{"parecord", "--file-format=wav", "--format=s16le", "--rate=16000", "--channels=1", "{file}"},
	{"arecord", "-q", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "wav", "{file}"},
	{"ffmpeg", "-loglevel", "error", "-y", "-f", "pulse", "-i", "default", "-ar", "16000", "-ac", "1", "{file}"},
}

// RecorderTool returns the first available recorder's name, or "".
func RecorderTool() string {
	for _, r := range recorders {
		if _, err := exec.LookPath(r[0]); err == nil {
			return r[0]
		}
	}
	return ""
}

func (r *Recorder) argv(file string) ([]string, error) {
	src := r.Argv
	if src == nil {
		for _, c := range recorders {
			if _, err := exec.LookPath(c[0]); err == nil {
				src = c
				break
			}
		}
	}
	if src == nil {
		return nil, errors.New("no audio recorder found (install pipewire's pw-record, pulseaudio's parecord, or alsa-utils' arecord)")
	}
	out := make([]string, len(src))
	for i, a := range src {
		out[i] = strings.ReplaceAll(a, "{file}", file)
	}
	return out, nil
}

func (r *Recorder) Recording() bool { r.mu.Lock(); defer r.mu.Unlock(); return r.cmd != nil }

// Start begins recording into a new file inside dir.
func (r *Recorder) Start(dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil {
		return errors.New("already recording")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file := filepath.Join(dir, fmt.Sprintf("rec-%d.wav", time.Now().UnixNano()))
	argv, err := r.argv(file)
	if err != nil {
		return err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	r.out.Reset()
	cmd.Stderr = &r.out
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot start %s: %w", argv[0], err)
	}
	r.cmd, r.file = cmd, file
	return nil
}

// Stop ends the recording (SIGINT so the WAV header is finalised) and
// returns the audio file path.
func (r *Recorder) Stop() (string, error) {
	r.mu.Lock()
	cmd, file := r.cmd, r.file
	r.cmd = nil
	r.mu.Unlock()
	if cmd == nil {
		return "", errors.New("not recording")
	}
	cmd.Process.Signal(syscall.SIGINT)
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		cmd.Process.Kill()
		<-done
	}
	st, err := os.Stat(file)
	if err != nil || st.Size() < 100 {
		os.Remove(file)
		msg := strings.TrimSpace(r.out.String())
		if msg == "" {
			msg = "no audio was captured (is a microphone connected and unmuted?)"
		}
		return "", errors.New(msg)
	}
	return file, nil
}

// Cancel stops and deletes the recording.
func (r *Recorder) Cancel() {
	if f, err := r.Stop(); err == nil {
		os.Remove(f)
	}
}

func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	has := false
	for _, c := range s {
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteRune(c)
			}
		case c == '"' || c == '\'':
			quote, has = c, true
		case unicode.IsSpace(c):
			if has || cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(c)
		}
	}
	if has || cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// VenvPython is where the optional offline engine is installed.
func VenvPython(dataDir string) string {
	return filepath.Join(dataDir, "voice", "venv", "bin", "python")
}

func moduleAvailable(py, mod string) bool {
	return exec.Command(py, "-c", "import "+mod).Run() == nil
}

// Detect picks the speech-to-text engine: a user command, the app's own
// faster-whisper environment, system faster-whisper, or the `whisper` CLI.
func Detect(dataDir string, s Settings) (Engine, bool) {
	model := s.Model
	if model == "" {
		model = "base"
	}
	lang := s.Language
	if lang == "" {
		lang = "auto"
	}
	if strings.TrimSpace(s.Command) != "" {
		argv := splitArgs(s.Command)
		if len(argv) > 0 {
			if _, err := exec.LookPath(argv[0]); err == nil {
				return Engine{Name: "custom command: " + argv[0], Argv: argv}, true
			}
		}
		return Engine{Name: "custom command not found"}, false
	}
	script := filepath.Join(dataDir, "voice", "transcribe.py")
	for _, py := range []string{VenvPython(dataDir), "python3"} {
		if _, err := exec.LookPath(py); err != nil {
			continue
		}
		if moduleAvailable(py, "faster_whisper") {
			os.MkdirAll(filepath.Dir(script), 0o700)
			os.WriteFile(script, transcribeScript, 0o600)
			name := "faster-whisper (" + model + ", offline)"
			return Engine{Name: name, Argv: []string{py, script, "{audio}", model, lang}}, true
		}
	}
	if p, err := exec.LookPath("whisper"); err == nil {
		args := []string{p, "{audio}", "--model", model, "--output_format", "txt", "--output_dir", "{outdir}", "--fp16", "False"}
		if lang != "auto" {
			args = append(args, "--language", lang)
		}
		return Engine{Name: "whisper CLI (" + model + ")", Argv: args, Temp: true}, true
	}
	return Engine{}, false
}

var noise = regexp.MustCompile(`(?i)\[(blank_audio|music|silence|noise)\]|\((music|silence|noise)\)`)

// Clean tidies a raw transcript: removes engine noise tags, collapses
// whitespace and capitalises the first letter.
func Clean(t string) string {
	t = noise.ReplaceAllString(t, " ")
	t = strings.Join(strings.Fields(t), " ")
	if t == "" {
		return ""
	}
	r := []rune(t)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// Transcribe runs the engine on a WAV file and returns cleaned text.
func Transcribe(ctx context.Context, e Engine, audio string) (string, error) {
	if len(e.Argv) == 0 {
		return "", errors.New("no speech-to-text engine configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	outdir, err := os.MkdirTemp("", "aicw-stt-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(outdir)
	argv := make([]string, len(e.Argv))
	for i, a := range e.Argv {
		a = strings.ReplaceAll(a, "{audio}", audio)
		argv[i] = strings.ReplaceAll(a, "{outdir}", outdir)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 600 {
			msg = msg[len(msg)-600:]
		}
		return "", fmt.Errorf("transcription failed: %v: %s", err, msg)
	}
	text := stdout.String()
	if e.Temp {
		base := strings.TrimSuffix(filepath.Base(audio), filepath.Ext(audio)) + ".txt"
		if b, err := os.ReadFile(filepath.Join(outdir, base)); err == nil {
			text = string(b)
		}
	}
	return Clean(text), nil
}

// Setup creates an app-private virtual environment with faster-whisper.
// progress receives human-readable lines. It needs network access.
func Setup(ctx context.Context, dataDir string, progress func(string)) error {
	py, err := exec.LookPath("python3")
	if err != nil {
		return errors.New("python3 is required for offline voice setup")
	}
	venv := filepath.Join(dataDir, "voice", "venv")
	run := func(name string, args ...string) error {
		progress("$ " + name + " " + strings.Join(args, " "))
		cmd := exec.CommandContext(ctx, name, args...)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
			if l != "" {
				progress(l)
			}
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(venv), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(VenvPython(dataDir)); err != nil {
		if err := run(py, "-m", "venv", venv); err != nil {
			return fmt.Errorf("creating the virtual environment failed: %w", err)
		}
	}
	if err := run(VenvPython(dataDir), "-m", "pip", "install", "--disable-pip-version-check", "faster-whisper"); err != nil {
		return fmt.Errorf("installing faster-whisper failed: %w", err)
	}
	progress("faster-whisper installed. The speech model (about 150 MB for \"base\") downloads on first use.")
	return nil
}
