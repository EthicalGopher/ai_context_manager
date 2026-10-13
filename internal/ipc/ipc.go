// Package ipc lets a second `aicw` invocation hand its request to the running
// instance over a Unix socket (owner-only permissions), so the CLI can exit
// cleanly instead of starting a duplicate app.
package ipc

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"time"
)

type Request struct {
	Args []string `json:"args"`
	Cwd  string   `json:"cwd"`
}

// Send delivers req to a running instance. It returns an error if none answers.
func Send(sock string, req Request) error {
	c, err := net.DialTimeout("unix", sock, 1500*time.Millisecond)
	if err != nil {
		return err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	if err := json.NewEncoder(c).Encode(req); err != nil {
		return err
	}
	var ack struct{ OK bool }
	if err := json.NewDecoder(c).Decode(&ack); err != nil || !ack.OK {
		return errors.New("running instance did not acknowledge the request")
	}
	return nil
}

// Listen starts serving requests. A stale socket file (no one answering) is
// replaced; a live one is an error. Close the returned listener to stop.
func Listen(sock string, handle func(Request)) (net.Listener, error) {
	if c, err := net.DialTimeout("unix", sock, 500*time.Millisecond); err == nil {
		c.Close()
		return nil, errors.New("another instance is already listening")
	}
	os.Remove(sock)
	l, err := net.Listen("unix", sock)
	if err != nil {
		return nil, err
	}
	os.Chmod(sock, 0o600)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				c.SetDeadline(time.Now().Add(3 * time.Second))
				var req Request
				if json.NewDecoder(c).Decode(&req) != nil {
					return
				}
				json.NewEncoder(c).Encode(map[string]bool{"OK": true})
				handle(req)
			}()
		}
	}()
	return l, nil
}
