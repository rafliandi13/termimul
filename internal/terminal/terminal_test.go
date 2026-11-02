package terminal

import (
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"termimul/internal/sshtest"
)

type pane struct {
	srv *sshtest.ShellServer

	mu     sync.Mutex
	output strings.Builder
}

func (p *pane) waitOutput(t *testing.T, want string) {
	t.Helper()
	for i := 0; i < 300; i++ {
		p.mu.Lock()
		got := p.output.String()
		p.mu.Unlock()
		if strings.Contains(got, want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("output never contained %q", want)
}

func start(t *testing.T, m *Manager, id string, cols, rows int) *pane {
	t.Helper()
	p := &pane{srv: sshtest.StartShellServer(t)}
	client, err := ssh.Dial("tcp", p.srv.Addr, &ssh.ClientConfig{User: "t", HostKeyCallback: ssh.InsecureIgnoreHostKey()})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Start(id, client, cols, rows, Handlers{
		OnData: func(b []byte) {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.output.Write(b)
		},
		OnClose: func(bool) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := <-p.srv.Sizes; got != [2]int{cols, rows} {
		t.Fatalf("pty size %v, want %v", got, [2]int{cols, rows})
	}
	return p
}

func TestPanesAreIndependent(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()
	a := start(t, m, "a", 80, 24)
	b := start(t, m, "b", 40, 12)

	// A resize reaches only the PTY of its own pane.
	if err := m.Resize("a", 100, 30); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-a.srv.Sizes:
		if got != [2]int{100, 30} {
			t.Fatalf("window-change %v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no window-change on pane a")
	}
	select {
	case got := <-b.srv.Sizes:
		t.Fatalf("pane b got a resize meant for a: %v", got)
	case <-time.After(50 * time.Millisecond):
	}

	if err := m.Write("a", []byte("to-a")); err != nil {
		t.Fatal(err)
	}
	if err := m.Write("b", []byte("to-b")); err != nil {
		t.Fatal(err)
	}
	a.waitOutput(t, "to-a")
	b.waitOutput(t, "to-b")
	if strings.Contains(a.output.String(), "to-b") {
		t.Fatal("input leaked between panes")
	}

	// Closing one pane ends its session and leaves the other running.
	m.Close("a")
	select {
	case <-a.srv.Closed:
	case <-time.After(3 * time.Second):
		t.Fatal("server session survived closing the pane")
	}
	if m.get("a") != nil {
		t.Fatal("closed session still tracked")
	}
	if err := m.Write("a", []byte("ignored")); err != nil {
		t.Fatalf("write to a closed pane: %v", err)
	}
	if err := m.Write("b", []byte("still-alive")); err != nil {
		t.Fatal(err)
	}
	b.waitOutput(t, "still-alive")
}
