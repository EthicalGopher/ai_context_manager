package term

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func collect(m *Manager) (*bytes.Buffer, *sync.Mutex, chan int) {
	var b bytes.Buffer
	var mu sync.Mutex
	exit := make(chan int, 1)
	m.OnData = func(_ string, d []byte) { mu.Lock(); b.Write(d); mu.Unlock() }
	m.OnExit = func(_ string, c int) { exit <- c }
	return &b, &mu, exit
}

func waitFor(t *testing.T, mu *sync.Mutex, b *bytes.Buffer, want string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		mu.Lock()
		ok := strings.Contains(b.String(), want)
		mu.Unlock()
		if ok {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	t.Fatalf("never saw %q; output: %q", want, b.String())
}

func TestInteractiveEchoAndExitCode(t *testing.T) {
	m := NewManager()
	b, mu, exit := collect(m)
	s, err := m.Start(Spec{Exe: "/bin/sh", Args: []string{"-c", `read x; echo got:$x; exit 3`}, Dir: t.TempDir(), Env: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Write(s.ID, []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, mu, b, "got:hello")
	select {
	case c := <-exit:
		if c != 3 {
			t.Fatalf("exit code %d", c)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no exit event")
	}
	if m.Alive(s.ID) {
		t.Fatal("should be dead")
	}
	if err := m.Write(s.ID, []byte("x")); err == nil {
		t.Fatal("write to dead terminal must fail")
	}
	if !strings.Contains(string(m.Buffer(s.ID)), "got:hello") {
		t.Fatal("scrollback missing")
	}
}

func TestArgvNoShellInterpolationAndEnv(t *testing.T) {
	m := NewManager()
	b, mu, _ := collect(m)
	dir := t.TempDir()
	m.Start(Spec{Exe: "/bin/sh", Args: []string{"-c", `printf '%s|%s|%s' "$0" "$AICW_WORKSPACE" "$(pwd)"`, `a b; echo pwned`}, Dir: dir, Env: append(os.Environ(), "AICW_WORKSPACE=/ws")})
	waitFor(t, mu, b, "a b; echo pwned|/ws|")
	mu.Lock()
	defer mu.Unlock()
	if strings.Count(b.String(), "pwned") != 1 {
		t.Fatalf("argument was interpreted by a shell: %q", b.String())
	}
}

func TestResizeAndCloseKillsProcess(t *testing.T) {
	m := NewManager()
	collect(m)
	s, _ := m.Start(Spec{Exe: "/bin/sh", Args: []string{"-c", "sleep 60"}, Dir: t.TempDir(), Env: os.Environ()})
	if err := m.Resize(s.ID, 120, 40); err != nil {
		t.Fatal(err)
	}
	if !m.Alive(s.ID) {
		t.Fatal("should be alive")
	}
	start := time.Now()
	if err := m.Close(s.ID); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 4*time.Second || m.Alive(s.ID) {
		t.Fatal("process not stopped promptly")
	}
	if err := m.Write(s.ID, nil); err == nil {
		t.Fatal("closed session must be forgotten")
	}
}

func TestStartErrors(t *testing.T) {
	m := NewManager()
	if _, err := m.Start(Spec{Exe: "/nonexistent-binary", Dir: t.TempDir()}); err == nil {
		t.Fatal("missing executable accepted")
	}
	if _, err := m.Start(Spec{Exe: "/bin/sh", Dir: "/definitely/not/here"}); err == nil {
		t.Fatal("missing dir accepted")
	}
	if _, err := m.Start(Spec{}); err == nil {
		t.Fatal("empty spec accepted")
	}
}
