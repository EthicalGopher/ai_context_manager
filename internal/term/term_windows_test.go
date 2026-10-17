//go:build windows

package term

import (
	"os"
	"testing"
	"time"
)

func TestWindowsConPTY(t *testing.T) {
	m := NewManager()
	b, mu, exit := collect(m)
	s, err := m.Start(Spec{Exe: "cmd.exe", Args: []string{"/c", "echo got:hello& exit 3"}, Dir: t.TempDir(), Env: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, mu, b, "got:hello")
	select {
	case c := <-exit:
		if c != 3 {
			t.Fatalf("exit code %d", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no exit event")
	}
	if m.Alive(s.ID) {
		t.Fatal("should be dead")
	}
}
