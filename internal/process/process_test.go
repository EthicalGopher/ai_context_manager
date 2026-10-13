package process

import (
	"os"
	"reflect"
	"testing"

	"aicw/internal/agents"
)

func TestCommandIsArgvAndKeepsSpaces(t *testing.T) {
	spec := agents.LaunchSpec{Exe: "/usr/bin/claude", Dir: "/home/u/my project", Args: []string{"--resume", "a b; rm -rf /"}}
	tm := Terminal{Name: "gnome-terminal", Path: "/usr/bin/gnome-terminal", pre: []string{"--working-directory={dir}", "--"}}
	cmd := tm.Command(spec)
	want := []string{"/usr/bin/gnome-terminal", "--working-directory=/home/u/my project", "--", "/usr/bin/claude", "--resume", "a b; rm -rf /"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("%q", cmd.Args)
	}
	tm = Terminal{Name: "konsole", Path: "/usr/bin/konsole", pre: []string{"--workdir", "{dir}", "-e"}}
	if got := tm.Command(spec).Args[2]; got != "/home/u/my project" {
		t.Fatalf("%q", got)
	}
}

func TestLaunchMissingDir(t *testing.T) {
	_, err := Launch(Terminal{Path: "/bin/true"}, agents.LaunchSpec{Dir: "/definitely/not/here", Exe: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAlive(t *testing.T) {
	if !Alive(os.Getpid()) || Alive(0) || Alive(-5) {
		t.Fatal("Alive wrong")
	}
}
