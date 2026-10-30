package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"termimul/internal/sshtest"
)

// fakeShell understands a few commands:
//
//	echo X   prints X
//	fail     prints to stderr, exits 3
//	sleep    waits until stopped
//	nap      sleeps 50ms
//	flood    prints more than MaxOutput
func fakeShell(running *atomic.Int32, peak *atomic.Int32) sshtest.ExecFunc {
	return func(cmd string, stdout, stderr io.Writer, stop <-chan struct{}) int {
		n := running.Add(1)
		defer running.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		switch {
		case strings.HasPrefix(cmd, "echo "):
			fmt.Fprintln(stdout, strings.TrimPrefix(cmd, "echo "))
			return 0
		case cmd == "fail":
			fmt.Fprintln(stderr, "boom")
			return 3
		case cmd == "sleep":
			<-stop
			return 137
		case cmd == "nap":
			time.Sleep(50 * time.Millisecond)
			return 0
		case cmd == "flood":
			chunk := strings.Repeat("x", 64*1024)
			for i := 0; i < MaxOutput/len(chunk)+4; i++ {
				io.WriteString(stdout, chunk)
			}
			return 0
		}
		return 127
	}
}

type harness struct {
	addr    string
	running atomic.Int32
	peak    atomic.Int32
}

func newHarness(t *testing.T) *harness {
	h := &harness{}
	h.addr = sshtest.StartExecServer(t, fakeShell(&h.running, &h.peak))
	return h
}

// dial treats hostIDs starting with "down" as unreachable.
func (h *harness) dial(t *testing.T) DialFunc {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := ln.Addr().String()
	ln.Close()
	return func(ctx context.Context, hostID string) (*ssh.Client, error) {
		addr := h.addr
		if strings.HasPrefix(hostID, "down") {
			addr = closed
		}
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		c, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{User: "t", HostKeyCallback: ssh.InsecureIgnoreHostKey()})
		if err != nil {
			return nil, err
		}
		return ssh.NewClient(c, chans, reqs), nil
	}
}

type result struct {
	events []Event
	out    map[string]string
	done   map[string]Event
}

func collect(t *testing.T, ctx context.Context, dial DialFunc, hosts []string, cmd string, opt Options) *result {
	t.Helper()
	r := &result{out: map[string]string{}, done: map[string]Event{}}
	var mu sync.Mutex
	Run(ctx, dial, hosts, cmd, opt, func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		if _, finished := r.done[e.HostID]; finished {
			t.Errorf("event after done: %+v", e)
		}
		r.events = append(r.events, e)
		switch e.Kind {
		case "output":
			r.out[e.HostID] += e.Stream + ":" + e.Data
		case "done":
			r.done[e.HostID] = e
		}
	})
	return r
}

func TestFailuresDontStopOthers(t *testing.T) {
	h := newHarness(t)
	dial := h.dial(t)
	r := collect(t, context.Background(), dial, []string{"a", "down-1", "b", "a"}, "echo hi", Options{})
	if len(r.done) != 3 {
		t.Fatalf("want 3 hosts done (duplicate ignored), got %+v", r.done)
	}
	for _, id := range []string{"a", "b"} {
		if d := r.done[id]; d.ExitCode != 0 || d.Error != "" || r.out[id] != "stdout:hi\n" {
			t.Fatalf("%s: %+v, output %q", id, d, r.out[id])
		}
	}
	if d := r.done["down-1"]; d.ExitCode != -1 || d.Error == "" {
		t.Fatalf("unreachable host: %+v", d)
	}

	r = collect(t, context.Background(), dial, []string{"a"}, "fail", Options{})
	if d := r.done["a"]; d.ExitCode != 3 || d.Error != "" || r.out["a"] != "stderr:boom\n" {
		t.Fatalf("non-zero exit: %+v, output %q", d, r.out["a"])
	}
}

func TestConcurrencyLimit(t *testing.T) {
	h := newHarness(t)
	hosts := []string{"h1", "h2", "h3", "h4", "h5", "h6"}
	r := collect(t, context.Background(), h.dial(t), hosts, "nap", Options{Concurrency: 2})
	if len(r.done) != len(hosts) {
		t.Fatalf("not all hosts finished: %+v", r.done)
	}
	if p := h.peak.Load(); p > 2 || p < 1 {
		t.Fatalf("peak concurrency %d, want at most 2", p)
	}
}

func TestTimeout(t *testing.T) {
	h := newHarness(t)
	start := time.Now()
	r := collect(t, context.Background(), h.dial(t), []string{"a"}, "sleep", Options{Timeout: 200 * time.Millisecond})
	d := r.done["a"]
	if d.ExitCode != -1 || !strings.Contains(d.Error, "timeout") {
		t.Fatalf("got %+v", d)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("timeout did not stop the command")
	}
}

func TestCancelStopsRunningAndQueued(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for h.running.Load() == 0 {
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
	}()
	start := time.Now()
	r := collect(t, ctx, h.dial(t), []string{"a", "b", "c"}, "sleep", Options{Concurrency: 1})
	if time.Since(start) > 5*time.Second {
		t.Fatal("cancel did not stop the batch")
	}
	for _, id := range []string{"a", "b", "c"} {
		if d := r.done[id]; d.Error != "dibatalkan" || d.ExitCode != -1 {
			t.Fatalf("%s: %+v", id, d)
		}
	}
	// Give the server a moment to notice the closed sessions.
	for i := 0; i < 100 && h.running.Load() > 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if n := h.running.Load(); n != 0 {
		t.Fatalf("%d commands still running on the server", n)
	}
}

func TestOutputIsCapped(t *testing.T) {
	h := newHarness(t)
	r := collect(t, context.Background(), h.dial(t), []string{"a"}, "flood", Options{})
	out := r.out["a"]
	if d := r.done["a"]; d.ExitCode != 0 {
		t.Fatalf("%+v", d)
	}
	if n := strings.Count(out, "x"); n != MaxOutput {
		t.Fatalf("streamed %d bytes, want %d", n, MaxOutput)
	}
	if !strings.Contains(out, "dipotong") {
		t.Fatal("missing truncation notice")
	}
}

func TestStreamWriterKeepsRunesWhole(t *testing.T) {
	var chunks []string
	h := &hostRun{id: "a", send: func(e Event) { chunks = append(chunks, e.Data) }}
	w := &streamWriter{h: h, stream: "stdout"}
	text := []byte("héllo ✓ done")
	for i := range text {
		w.Write(text[i : i+1])
	}
	w.flush()
	for _, c := range chunks {
		if !strings.Contains("héllo ✓ done", c) {
			t.Fatalf("chunk %q is not valid text", c)
		}
	}
	if got := strings.Join(chunks, ""); got != "héllo ✓ done" {
		t.Fatalf("got %q", got)
	}
}
