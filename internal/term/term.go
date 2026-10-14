// Package term runs commands inside a real pseudo-terminal so agent CLIs can
// live inside the app. Processes are started only on an explicit user action
// and always get an argument array (never a shell string).
package term

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const scrollback = 512 * 1024

type Spec struct {
	Exe        string
	Args       []string
	Dir        string
	Env        []string
	Cols, Rows uint16
}

type Session struct {
	ID  string
	PID int
	pty ptyConn

	mu   sync.Mutex
	buf  []byte
	done chan struct{}
	code int
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	next     int
	OnData   func(id string, data []byte)
	OnExit   func(id string, code int)
}

func NewManager() *Manager { return &Manager{sessions: map[string]*Session{}} }

func (m *Manager) Start(sp Spec) (*Session, error) {
	if sp.Exe == "" || sp.Dir == "" {
		return nil, errors.New("executable and directory are required")
	}
	if st, err := os.Stat(sp.Dir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("working directory unavailable: %s", sp.Dir)
	}
	if sp.Cols == 0 {
		sp.Cols = 100
	}
	if sp.Rows == 0 {
		sp.Rows = 30
	}
	p, err := startPTY(sp)
	if err != nil {
		return nil, fmt.Errorf("failed to start %s: %w", sp.Exe, err)
	}
	m.mu.Lock()
	m.next++
	s := &Session{ID: fmt.Sprintf("t%d", m.next), PID: p.Pid(), pty: p, done: make(chan struct{})}
	m.sessions[s.ID] = s
	m.mu.Unlock()

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		chunk := make([]byte, 32*1024)
		for {
			n, err := p.Read(chunk)
			if n > 0 {
				d := append([]byte(nil), chunk[:n]...)
				s.mu.Lock()
				s.buf = append(s.buf, d...)
				if len(s.buf) > scrollback {
					s.buf = s.buf[len(s.buf)-scrollback:]
				}
				s.mu.Unlock()
				if m.OnData != nil {
					m.OnData(s.ID, d)
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		code := p.Wait()
		// Let the reader drain what the process wrote last. Windows ConPTY
		// never reports EOF until it is closed, so don't wait forever.
		select {
		case <-readDone:
		case <-time.After(300 * time.Millisecond):
		}
		s.mu.Lock()
		s.code = code
		s.mu.Unlock()
		p.Close()
		<-readDone
		close(s.done)
		if m.OnExit != nil {
			m.OnExit(s.ID, code)
		}
	}()
	return s, nil
}

func (m *Manager) get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, errors.New("no such terminal")
	}
	return s, nil
}

func (s *Session) Alive() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

func (m *Manager) Alive(id string) bool {
	s, err := m.get(id)
	return err == nil && s.Alive()
}

func (m *Manager) Write(id string, data []byte) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	if !s.Alive() {
		return errors.New("terminal process has exited")
	}
	_, err = s.pty.Write(data)
	return err
}

func (m *Manager) Resize(id string, cols, rows uint16) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	if !s.Alive() || cols == 0 || rows == 0 {
		return nil
	}
	return s.pty.Resize(cols, rows)
}

// Buffer returns recent output so a re-attached view can repaint.
func (m *Manager) Buffer(id string) []byte {
	s, err := m.get(id)
	if err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.buf...)
}

// Close stops the process (hang-up, then a forced kill after a grace period) and
// forgets the session once it has exited.
func (m *Manager) Close(id string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	if s.Alive() {
		s.pty.Terminate(false)
		select {
		case <-s.done:
		case <-time.After(2 * time.Second):
			s.pty.Terminate(true)
			<-s.done
		}
	}
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
	return nil
}

// CloseAll stops every terminal (app shutdown).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Close(id)
	}
}

// ptyConn is a started process attached to a pseudo-terminal. The Unix
// implementation uses a PTY; Windows uses ConPTY.
type ptyConn interface {
	io.ReadWriteCloser
	Resize(cols, rows uint16) error
	Pid() int
	// Wait blocks until the process exits and returns its exit code.
	Wait() int
	// Terminate asks the process (and its children) to stop; force kills it.
	Terminate(force bool)
}
