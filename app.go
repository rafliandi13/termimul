package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx   context.Context
	store *Store
	emit  func(event string, data any) // frontend events; swapped out in tests

	mu       sync.Mutex
	sessions map[string]*sshSession
	sftps    map[string]*sftpSession

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
		sftps:          map[string]*sftpSession{},
		knownHostsPath: filepath.Join(dataDir, "known_hosts"),
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.emit = func(event string, data any) { runtime.EventsEmit(ctx, event, data) }
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, s := range a.sessions {
		s.close()
		delete(a.sessions, id)
	}
	for id, s := range a.sftps {
		s.close()
		delete(a.sftps, id)
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

func (a *App) ListKeys() []Key {
	return a.store.ListKeys()
}

// GenerateKey creates a new key pair; kind is "ed25519", "rsa" or "ecdsa".
func (a *App) GenerateKey(label, kind string) (Key, error) {
	priv, err := generatePrivateKey(kind)
	if err != nil {
		return Key{}, err
	}
	return a.store.AddKey(label, priv)
}

func (a *App) ImportKeyFile(path, label, passphrase string) (Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Key{}, err
	}
	if label == "" {
		label = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return a.ImportKeyText(label, string(data), passphrase)
}

func (a *App) ImportKeyText(label, text, passphrase string) (Key, error) {
	priv, err := parsePrivateKey([]byte(strings.TrimSpace(text)+"\n"), passphrase)
	if err != nil {
		return Key{}, err
	}
	return a.store.AddKey(label, priv)
}

func (a *App) DeleteKey(id string) error {
	return a.store.DeleteKey(id)
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
