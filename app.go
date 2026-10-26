package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx   context.Context
	store *Store

	mu       sync.Mutex
	sessions map[string]*sshSession

	khMu           sync.Mutex
	knownHostsPath string
}

func NewApp(dataDir string) (*App, error) {
	store, err := NewStore(dataDir)
	if err != nil {
		return nil, err
	}
	return &App{
		store:          store,
		sessions:       map[string]*sshSession{},
		knownHostsPath: filepath.Join(dataDir, "known_hosts"),
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, s := range a.sessions {
		s.close()
		delete(a.sessions, id)
	}
}

func (a *App) ListHosts() []Host {
	return a.store.List()
}

func (a *App) SaveHost(in HostInput) (Host, error) {
	return a.store.Save(in)
}

func (a *App) DeleteHost(id string) error {
	return a.store.Delete(id)
}

func (a *App) PickKeyFile() (string, error) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ssh")
	if _, err := os.Stat(dir); err != nil {
		dir = home
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Pilih SSH private key",
		DefaultDirectory: dir,
	})
}
