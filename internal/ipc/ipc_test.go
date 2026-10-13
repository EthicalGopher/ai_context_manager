package ipc

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSendReachesRunningInstance(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "a.sock")
	got := make(chan Request, 1)
	l, err := Listen(sock, func(r Request) { got <- r })
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	want := Request{Args: []string{"--new", "my project"}, Cwd: "/tmp/x y"}
	if err := Send(sock, want); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-got:
		if !reflect.DeepEqual(r, want) {
			t.Fatalf("%+v", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler not called")
	}
	if st, _ := os.Stat(sock); st.Mode().Perm() != 0o600 {
		t.Fatalf("socket perms %v", st.Mode().Perm())
	}
	if _, err := Listen(sock, func(Request) {}); err == nil {
		t.Fatal("second listener must be refused while one is live")
	}
}

func TestSendWithoutInstanceFailsAndStaleSocketReplaced(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "b.sock")
	if err := Send(sock, Request{}); err == nil {
		t.Fatal("expected error with no instance")
	}
	os.WriteFile(sock, nil, 0o600) // stale file
	l, err := Listen(sock, func(Request) {})
	if err != nil {
		t.Fatalf("stale socket should be replaced: %v", err)
	}
	l.Close()
}
