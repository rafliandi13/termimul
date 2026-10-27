package tunnel

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"termimul/internal/sshtest"
	"termimul/internal/vault"
)

// roundTrip sends a line through conn and expects it echoed back.
func roundTrip(t *testing.T, conn net.Conn) {
	t.Helper()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("halo termimul\n")); err != nil {
		t.Fatal(err)
	}
	got, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || got != "halo termimul\n" {
		t.Fatalf("echo through tunnel: %q, %v", got, err)
	}
}

func split(t *testing.T, addr string) (string, int) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := strconv.Atoi(port)
	return host, p
}

func startTunnel(t *testing.T, m *Manager, tun vault.Tunnel) Status {
	t.Helper()
	if err := m.Start(tun, sshtest.StartForwardServer(t)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Stop(tun.ID) })
	s := m.Status(tun.ID)
	if s.State != Running || s.Addr == "" {
		t.Fatalf("not running: %+v", s)
	}
	return s
}

func TestLocalForward(t *testing.T) {
	echoHost, echoPort := split(t, sshtest.EchoServer(t))
	var changes []Status
	m := NewManager(func(s Status) { changes = append(changes, s) })
	s := startTunnel(t, m, vault.Tunnel{ID: "L", Type: "local", BindAddress: "127.0.0.1", TargetHost: echoHost, TargetPort: echoPort})

	conn, err := net.Dial("tcp", s.Addr)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, conn)
	if n := m.Status("L").Connections; n == 0 {
		t.Error("open connection not counted")
	}
	conn.Close()

	m.Stop("L")
	if st := m.Status("L"); st.State != Stopped {
		t.Fatalf("after stop: %+v", st)
	}
	if c, err := net.DialTimeout("tcp", s.Addr, time.Second); err == nil {
		c.Close()
		t.Fatal("port still open after stop")
	}
	if len(changes) == 0 {
		t.Error("status changes not reported")
	}
}

func TestRemoteForward(t *testing.T) {
	echoHost, echoPort := split(t, sshtest.EchoServer(t))
	m := NewManager(nil)
	// Port 0 lets the test server pick a free port; Addr reports it.
	s := startTunnel(t, m, vault.Tunnel{ID: "R", Type: "remote", BindAddress: "127.0.0.1", TargetHost: echoHost, TargetPort: echoPort})

	_, port := split(t, s.Addr)
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	roundTrip(t, conn)
}

func TestDynamicSOCKS(t *testing.T) {
	_, echoPort := split(t, sshtest.EchoServer(t))
	m := NewManager(nil)
	s := startTunnel(t, m, vault.Tunnel{ID: "D", Type: "dynamic", BindAddress: "127.0.0.1"})

	conn, err := net.Dial("tcp", s.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Greeting: version 5, one method, "no authentication".
	conn.Write([]byte{5, 1, 0})
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil || reply[1] != 0 {
		t.Fatalf("greeting reply %v, %v", reply, err)
	}
	// CONNECT by domain name, the way browsers usually ask.
	req := []byte{5, 1, 0, 3, byte(len("localhost"))}
	req = append(req, "localhost"...)
	req = binary.BigEndian.AppendUint16(req, uint16(echoPort))
	conn.Write(req)
	resp := make([]byte, 10)
	if _, err := io.ReadFull(conn, resp); err != nil || resp[1] != 0 {
		t.Fatalf("connect reply %v, %v", resp, err)
	}
	roundTrip(t, conn)
}

func TestSSHDropMarksFailed(t *testing.T) {
	m := NewManager(nil)
	client := sshtest.StartForwardServer(t)
	if err := m.Start(vault.Tunnel{ID: "X", Type: "dynamic", BindAddress: "127.0.0.1"}, client); err != nil {
		t.Fatal(err)
	}
	client.Close()
	deadline := time.Now().Add(3 * time.Second)
	for m.Status("X").State != Failed && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if s := m.Status("X"); s.State != Failed || s.Error == "" {
		t.Fatalf("expected failed status, got %+v", s)
	}
}

func TestPortInUse(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	_, port := split(t, busy.Addr().String())

	m := NewManager(nil)
	err = m.Start(vault.Tunnel{ID: "B", Type: "dynamic", BindAddress: "127.0.0.1", BindPort: port}, sshtest.StartForwardServer(t))
	if err == nil {
		t.Fatal("expected an error for a port that is already in use")
	}
	if s := m.Status("B"); s.State != Failed {
		t.Fatalf("status %+v", s)
	}
}
