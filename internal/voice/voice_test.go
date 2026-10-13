package voice

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCleanAndSplit(t *testing.T) {
	if got := Clean("  [BLANK_AUDIO] hello   world \n (music)"); got != "Hello world" {
		t.Fatalf("%q", got)
	}
	if Clean("[BLANK_AUDIO]") != "" {
		t.Fatal("noise only should be empty")
	}
	want := []string{"whisper-cli", "-m", "/my models/m.bin", "-f", "{audio}"}
	if got := splitArgs(`whisper-cli -m "/my models/m.bin" -f {audio}`); !reflect.DeepEqual(got, want) {
		t.Fatalf("%q", got)
	}
}

func fake(t *testing.T, name, body string) string {
	p := filepath.Join(t.TempDir(), name)
	os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755)
	return p
}

func TestRecordStopProducesFileAndCancelDeletes(t *testing.T) {
	rec := fake(t, "rec", `f="$1"; printf 'RIFFxxxxWAVE%0200d' 0 > "$f"; trap 'exit 0' INT; while true; do sleep 0.05; done`)
	r := &Recorder{Argv: []string{rec, "{file}"}}
	dir := t.TempDir()
	if err := r.Start(dir); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // let the fake recorder install its trap
	if !r.Recording() || r.Start(dir) == nil {
		t.Fatal("double start must fail")
	}
	f, err := r.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(f); st == nil || st.Size() < 100 {
		t.Fatal("no audio file")
	}
	if _, err := r.Stop(); err == nil {
		t.Fatal("stop when idle must fail")
	}
	r.Start(dir)
	time.Sleep(300 * time.Millisecond)
	r.Cancel()
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("cancel must delete its file, dir has %d", len(ents))
	}
}

func TestRecorderFailureIsExplained(t *testing.T) {
	rec := fake(t, "rec", `echo "no microphone" >&2; exit 1`)
	r := &Recorder{Argv: []string{rec, "{file}"}}
	r.Start(t.TempDir())
	time.Sleep(300 * time.Millisecond)
	if _, err := r.Stop(); err == nil || !strings.Contains(err.Error(), "no microphone") {
		t.Fatalf("%v", err)
	}
}

func TestDetectCustomAndTranscribe(t *testing.T) {
	stt := fake(t, "stt", `echo "[BLANK_AUDIO] turn the heading blue"`)
	e, ok := Detect(t.TempDir(), Settings{Command: stt + " {audio}"})
	if !ok || !strings.Contains(e.Name, "custom") {
		t.Fatalf("%+v %v", e, ok)
	}
	audio := filepath.Join(t.TempDir(), "a.wav")
	os.WriteFile(audio, make([]byte, 300), 0o600)
	text, err := Transcribe(context.Background(), e, audio)
	if err != nil || text != "Turn the heading blue" {
		t.Fatalf("%q %v", text, err)
	}
	if e, ok := Detect(t.TempDir(), Settings{Command: "/no/such/tool {audio}"}); ok || e.Name == "" {
		t.Fatal("missing custom command must be reported, not silently replaced")
	}
	bad := fake(t, "bad", `echo boom >&2; exit 2`)
	if _, err := Transcribe(context.Background(), Engine{Argv: []string{bad, "{audio}"}}, audio); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("%v", err)
	}
	if _, err := Transcribe(context.Background(), Engine{}, audio); err == nil {
		t.Fatal("no engine must error")
	}
}

func TestWhisperCLIOutputFile(t *testing.T) {
	// The openai-whisper CLI writes <name>.txt into --output_dir.
	stt := fake(t, "stt", `out=""; audio="$1"; shift; while [ $# -gt 0 ]; do [ "$1" = "--output_dir" ] && out="$2"; shift; done; b=$(basename "$audio" .wav); echo "hello from file" > "$out/$b.txt"`)
	audio := filepath.Join(t.TempDir(), "x.wav")
	os.WriteFile(audio, make([]byte, 300), 0o600)
	text, err := Transcribe(context.Background(), Engine{Argv: []string{stt, "{audio}", "--output_dir", "{outdir}"}, Temp: true}, audio)
	if err != nil || text != "Hello from file" {
		t.Fatalf("%q %v", text, err)
	}
}
