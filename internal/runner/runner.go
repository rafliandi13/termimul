// Package runner executes one command on several hosts at once over SSH
// (non-interactive exec, no PTY) and streams what happens on each host.
package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"
)

// DialFunc connects to a host. It must give up once ctx ends.
type DialFunc func(ctx context.Context, hostID string) (*ssh.Client, error)

type Options struct {
	Concurrency int           // hosts running at the same time; below 1 means DefaultConcurrency
	Timeout     time.Duration // per host, connecting included; 0 means no limit
}

const DefaultConcurrency = 4

// MaxOutput caps what is streamed per host, stdout and stderr together, so a
// runaway command can't flood the UI. The command keeps running past it.
const MaxOutput = 1 << 20

// Event reports progress on one host. Kind is "start" when the host leaves the
// queue, "output" for a chunk of stdout or stderr, and "done" exactly once, last.
type Event struct {
	HostID   string `json:"hostId"`
	Kind     string `json:"kind"`
	Stream   string `json:"stream,omitempty"` // output: "stdout" or "stderr"
	Data     string `json:"data,omitempty"`   // output
	ExitCode int    `json:"exitCode"`         // done: -1 when the command didn't run to completion
	Error    string `json:"error,omitempty"`  // done: why it didn't; a non-zero exit is not an error
	Duration int64  `json:"durationMs"`       // done: from leaving the queue
}

// Run executes command on every host and returns once all of them are done.
// A host that fails doesn't affect the others; cancelling ctx stops queued and
// running hosts alike. emit is never called concurrently.
func Run(ctx context.Context, dial DialFunc, hostIDs []string, command string, opt Options, emit func(Event)) {
	if opt.Concurrency < 1 {
		opt.Concurrency = DefaultConcurrency
	}
	var mu sync.Mutex
	send := func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		emit(e)
	}

	sem := make(chan struct{}, opt.Concurrency)
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for _, id := range hostIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
			}
			if ctx.Err() != nil {
				send(Event{HostID: id, Kind: "done", ExitCode: -1, Error: "dibatalkan"})
				return
			}
			send(Event{HostID: id, Kind: "start"})
			h := &hostRun{id: id, send: send}
			send(h.run(ctx, dial, command, opt.Timeout))
		}()
	}
	wg.Wait()
}

type hostRun struct {
	id   string
	send func(Event)

	mu      sync.Mutex
	written int
	capped  bool // MaxOutput reached and the notice sent
}

func (h *hostRun) run(parent context.Context, dial DialFunc, command string, timeout time.Duration) Event {
	start := time.Now()
	ctx, cancel := parent, context.CancelFunc(func() {})
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, timeout)
	}
	defer cancel()

	done := Event{HostID: h.id, Kind: "done", ExitCode: -1}
	finish := func(err error) Event {
		done.Duration = time.Since(start).Milliseconds()
		switch {
		case parent.Err() != nil:
			done.Error = "dibatalkan"
		case ctx.Err() != nil:
			done.Error = fmt.Sprintf("timeout setelah %s", timeout)
		case err != nil:
			done.Error = err.Error()
		}
		return done
	}

	client, err := dial(ctx, h.id)
	if err != nil {
		return finish(err)
	}
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		return finish(fmt.Errorf("gagal membuka sesi: %w", err))
	}
	defer sess.Close()
	stdout, stderr := &streamWriter{h: h, stream: "stdout"}, &streamWriter{h: h, stream: "stderr"}
	sess.Stdout, sess.Stderr = stdout, stderr
	if err := sess.Start(command); err != nil {
		return finish(fmt.Errorf("gagal menjalankan perintah: %w", err))
	}

	waited := make(chan error, 1)
	go func() { waited <- sess.Wait() }()
	select {
	case err = <-waited:
	case <-ctx.Done():
		sess.Signal(ssh.SIGKILL)
		client.Close()
		<-waited
		stdout.flush()
		stderr.flush()
		return finish(nil)
	}
	stdout.flush()
	stderr.flush()

	// The command finished on its own, so a deadline passing meanwhile doesn't matter.
	var exit *ssh.ExitError
	var missing *ssh.ExitMissingError
	switch {
	case err == nil:
		done.ExitCode = 0
	case errors.As(err, &exit):
		done.ExitCode = exit.ExitStatus()
	case errors.As(err, &missing):
		done.Error = "koneksi terputus sebelum perintah selesai"
	default:
		done.Error = err.Error()
	}
	done.Duration = time.Since(start).Milliseconds()
	return done
}

// output streams a chunk, enforcing MaxOutput across both streams.
func (h *hostRun) output(stream string, p []byte) {
	if len(p) == 0 {
		return
	}
	h.mu.Lock()
	if h.capped {
		h.mu.Unlock()
		return
	}
	left := MaxOutput - h.written
	if len(p) > left {
		p, h.capped = p[:left], true
	}
	h.written += len(p)
	truncated := h.capped
	h.mu.Unlock()

	if len(p) > 0 {
		h.send(Event{HostID: h.id, Kind: "output", Stream: stream, Data: string(p)})
	}
	if truncated {
		h.send(Event{HostID: h.id, Kind: "output", Stream: "stderr", Data: fmt.Sprintf("\n[output dipotong setelah %d KB]\n", MaxOutput/1024)})
	}
}

// streamWriter holds back a UTF-8 sequence split across writes, so every
// chunk sent to the UI is valid text.
type streamWriter struct {
	h       *hostRun
	stream  string
	pending []byte
}

func (w *streamWriter) Write(p []byte) (int, error) {
	buf := append(w.pending, p...)
	cut := len(buf)
	for i := 1; i <= utf8.UTFMax && i <= len(buf); i++ {
		if utf8.RuneStart(buf[len(buf)-i]) {
			if !utf8.FullRune(buf[len(buf)-i:]) {
				cut = len(buf) - i
			}
			break
		}
	}
	w.pending = append([]byte(nil), buf[cut:]...)
	w.h.output(w.stream, buf[:cut])
	return len(p), nil
}

func (w *streamWriter) flush() {
	if len(w.pending) > 0 {
		w.h.output(w.stream, w.pending)
		w.pending = nil
	}
}
