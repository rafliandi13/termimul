package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"termimul/internal/runner"
	"termimul/internal/vault"
)

// SnippetService stores command snippets and runs them on several hosts.
// Progress is pushed as "snippet:run" events carrying a RunEvent.
type SnippetService struct {
	*env

	mu   sync.Mutex
	runs map[string]context.CancelFunc
}

type RunRequest struct {
	Command     string   `json:"command"`
	HostIDs     []string `json:"hostIds"`
	Concurrency int      `json:"concurrency"`
	TimeoutSec  int      `json:"timeoutSec"` // 0 = no limit
}

// RunEvent is a runner.Event tagged with its run. Kind "finished" is sent
// once, after every host is done.
type RunEvent struct {
	RunID string `json:"runId"`
	runner.Event
}

func (s *SnippetService) List() []vault.Snippet {
	return s.store.Snippets()
}

func (s *SnippetService) Save(sn vault.Snippet) (vault.Snippet, error) {
	return s.store.SaveSnippet(sn)
}

func (s *SnippetService) Delete(id string) error {
	return s.store.DeleteSnippet(id)
}

// Run starts the command on the hosts and returns the run ID right away.
func (s *SnippetService) Run(req RunRequest) (string, error) {
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return "", errors.New("perintah wajib diisi")
	}
	if len(req.HostIDs) == 0 {
		return "", errors.New("pilih minimal satu host")
	}
	opt := runner.Options{Concurrency: req.Concurrency}
	if req.TimeoutSec > 0 {
		opt.Timeout = time.Duration(req.TimeoutSec) * time.Second
	}

	id := newRunID()
	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.runs[id] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.runs, id)
			s.mu.Unlock()
			cancel()
			runtime.EventsEmit(s.ctx, "snippet:run", RunEvent{RunID: id, Event: runner.Event{Kind: "finished"}})
		}()
		runner.Run(ctx, s.dialer.Dial, req.HostIDs, command, opt, func(e runner.Event) {
			runtime.EventsEmit(s.ctx, "snippet:run", RunEvent{RunID: id, Event: e})
		})
	}()
	return id, nil
}

// Cancel stops every queued and running host of a run.
func (s *SnippetService) Cancel(runID string) {
	s.mu.Lock()
	cancel := s.runs[runID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *SnippetService) cancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cancel := range s.runs {
		cancel()
	}
}

func newRunID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
