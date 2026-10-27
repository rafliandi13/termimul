// Package tunnel runs port-forwarding rules (local, remote and dynamic/SOCKS5)
// over SSH connections.
package tunnel

import (
	"errors"
	"io"
	"net"
	"strconv"
	"sync"

	"golang.org/x/crypto/ssh"

	"termimul/internal/vault"
)

type State string

const (
	Stopped  State = "stopped"
	Starting State = "starting"
	Running  State = "running"
	Failed   State = "error"
)

type Status struct {
	ID          string `json:"id"`
	State       State  `json:"state"`
	Error       string `json:"error"`
	Addr        string `json:"addr"` // where it actually listens (useful when BindPort was 0)
	Connections int    `json:"connections"`
}

type running struct {
	client   *ssh.Client
	listener net.Listener
	mu       sync.Mutex
	conns    map[io.Closer]struct{}
	stopped  bool
}

type Manager struct {
	mu       sync.Mutex
	running  map[string]*running
	statuses map[string]Status
	onChange func(Status)
}

// NewManager reports every status change to onChange (may be nil).
func NewManager(onChange func(Status)) *Manager {
	if onChange == nil {
		onChange = func(Status) {}
	}
	return &Manager{running: map[string]*running{}, statuses: map[string]Status{}, onChange: onChange}
}

func (m *Manager) Status(id string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.statuses[id]; ok {
		return s
	}
	return Status{ID: id, State: Stopped}
}

// SetStarting marks a tunnel as connecting, before its SSH client exists.
func (m *Manager) SetStarting(id string) {
	m.setStatus(Status{ID: id, State: Starting})
}

// Fail records an error for a tunnel that couldn't start.
func (m *Manager) Fail(id string, err error) {
	m.setStatus(Status{ID: id, State: Failed, Error: err.Error()})
}

// Start begins forwarding t over client. The Manager owns client from now on
// and closes it when the tunnel stops.
func (m *Manager) Start(t vault.Tunnel, client *ssh.Client) error {
	m.Stop(t.ID)
	bind := net.JoinHostPort(t.BindAddress, strconv.Itoa(t.BindPort))
	target := net.JoinHostPort(t.TargetHost, strconv.Itoa(t.TargetPort))

	var ln net.Listener
	var err error
	var handle func(net.Conn)
	switch t.Type {
	case "local":
		ln, err = net.Listen("tcp", bind)
		handle = func(c net.Conn) { m.forward(t.ID, c, func() (net.Conn, error) { return client.Dial("tcp", target) }) }
	case "remote":
		ln, err = client.Listen("tcp", bind)
		handle = func(c net.Conn) { m.forward(t.ID, c, func() (net.Conn, error) { return net.Dial("tcp", target) }) }
	case "dynamic":
		ln, err = net.Listen("tcp", bind)
		handle = func(c net.Conn) { m.socks(t.ID, c, client) }
	default:
		err = errors.New("jenis tunnel tidak dikenal")
	}
	if err != nil {
		client.Close()
		err = friendlyListenError(t, err)
		m.Fail(t.ID, err)
		return err
	}

	r := &running{client: client, listener: ln, conns: map[io.Closer]struct{}{}}
	m.mu.Lock()
	m.running[t.ID] = r
	m.mu.Unlock()
	m.setStatus(Status{ID: t.ID, State: Running, Addr: ln.Addr().String()})

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go handle(c)
		}
	}()
	// The SSH connection dropping ends the tunnel.
	go func() {
		client.Wait()
		if m.stop(t.ID, r) {
			m.setStatus(Status{ID: t.ID, State: Failed, Error: "koneksi SSH terputus"})
		}
	}()
	return nil
}

func (m *Manager) Stop(id string) {
	m.mu.Lock()
	r := m.running[id]
	m.mu.Unlock()
	if r != nil && m.stop(id, r) {
		m.setStatus(Status{ID: id, State: Stopped})
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.running))
	for id := range m.running {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Stop(id)
	}
}

// Forget stops a tunnel and drops its status (after the rule is deleted).
func (m *Manager) Forget(id string) {
	m.Stop(id)
	m.mu.Lock()
	delete(m.statuses, id)
	m.mu.Unlock()
}

// stop tears r down once; it reports whether this call did the work.
func (m *Manager) stop(id string, r *running) bool {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return false
	}
	r.stopped = true
	conns := r.conns
	r.conns = nil
	r.mu.Unlock()

	r.listener.Close()
	for c := range conns {
		c.Close()
	}
	r.client.Close()
	m.mu.Lock()
	if m.running[id] == r {
		delete(m.running, id)
	}
	m.mu.Unlock()
	return true
}

func (m *Manager) setStatus(s Status) {
	m.mu.Lock()
	m.statuses[s.ID] = s
	m.mu.Unlock()
	m.onChange(s)
}

// track registers open connections so Stop can close them and the UI can count them.
func (m *Manager) track(id string, c io.Closer, open bool) {
	m.mu.Lock()
	r := m.running[id]
	m.mu.Unlock()
	if r == nil {
		c.Close()
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		c.Close()
		return
	}
	if open {
		r.conns[c] = struct{}{}
	} else {
		delete(r.conns, c)
	}
	n := len(r.conns)
	addr := r.listener.Addr().String()
	r.mu.Unlock()
	m.setStatus(Status{ID: id, State: Running, Addr: addr, Connections: n})
}

func (m *Manager) forward(id string, in net.Conn, dial func() (net.Conn, error)) {
	out, err := dial()
	if err != nil {
		in.Close()
		return
	}
	m.pipe(id, in, out)
}

// pipe copies both ways until either side closes, then closes both.
func (m *Manager) pipe(id string, a, b net.Conn) {
	m.track(id, a, true)
	m.track(id, b, true)
	done := make(chan struct{}, 2)
	go func() { io.Copy(a, b); done <- struct{}{} }()
	go func() { io.Copy(b, a); done <- struct{}{} }()
	<-done
	a.Close()
	b.Close()
	<-done
	m.track(id, a, false)
	m.track(id, b, false)
}

func friendlyListenError(t vault.Tunnel, err error) error {
	where := "di komputer ini"
	if t.Type == "remote" {
		where = "di server"
	}
	return errors.New("tidak bisa membuka port " + strconv.Itoa(t.BindPort) + " " + where + ": " + err.Error())
}
