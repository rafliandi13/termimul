// Package terminal runs interactive shell sessions over SSH, one per tab.
package terminal

import (
	"errors"
	"io"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Handlers receive a session's output and its end. loggedOut is true when the
// shell exited by itself (the user typed exit), false when the connection dropped.
type Handlers struct {
	OnData  func(data []byte)
	OnClose func(loggedOut bool)
}

type session struct {
	client *ssh.Client
	sess   *ssh.Session
	stdin  io.WriteCloser
}

func (s *session) close() {
	s.sess.Close()
	s.client.Close()
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager() *Manager {
	return &Manager{sessions: map[string]*session{}}
}

// Start opens a PTY shell on client under id. The Manager owns client from now on.
func (m *Manager) Start(id string, client *ssh.Client, cols, rows int, h Handlers) error {
	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return err
	}
	s := &session{client: client, sess: sess}
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := sess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		s.close()
		return err
	}
	if s.stdin, err = sess.StdinPipe(); err != nil {
		s.close()
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		s.close()
		return err
	}
	if err := sess.Shell(); err != nil {
		s.close()
		return err
	}

	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()

	go pump(stdout, h.OnData)
	go func() {
		err := sess.Wait()
		m.mu.Lock()
		delete(m.sessions, id)
		m.mu.Unlock()
		client.Close()
		// The shell reporting an exit status (any code) means the user logged out;
		// anything else (no status, I/O error) means the connection dropped.
		var exitErr *ssh.ExitError
		h.OnClose(err == nil || errors.As(err, &exitErr))
	}()
	return nil
}

func pump(r io.Reader, onData func([]byte)) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			onData(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// Write sends keystrokes; writes to a closed session are ignored.
func (m *Manager) Write(id string, data []byte) error {
	s := m.get(id)
	if s == nil {
		return nil
	}
	_, err := s.stdin.Write(data)
	return err
}

func (m *Manager) Resize(id string, cols, rows int) error {
	s := m.get(id)
	if s == nil || cols <= 0 || rows <= 0 {
		return nil
	}
	return s.sess.WindowChange(rows, cols)
}

func (m *Manager) Close(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s != nil {
		s.close()
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		s.close()
		delete(m.sessions, id)
	}
}

func (m *Manager) get(id string) *session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}
