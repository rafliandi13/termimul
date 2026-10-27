package services

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"termimul/internal/knownhosts"
	"termimul/internal/vault"
)

type HostService struct{ *env }

func (s *HostService) List() []vault.Host {
	return s.store.Hosts()
}

func (s *HostService) Save(in vault.HostInput) (vault.Host, error) {
	return s.store.SaveHost(in)
}

func (s *HostService) Delete(id string) error {
	return s.store.DeleteHost(id)
}

type KeychainService struct{ *env }

func (s *KeychainService) List() []vault.Key {
	return s.store.Keys()
}

// Generate creates a new key pair; kind is "ed25519", "rsa" or "ecdsa".
func (s *KeychainService) Generate(label, kind string) (vault.Key, error) {
	priv, err := vault.GeneratePrivateKey(kind)
	if err != nil {
		return vault.Key{}, err
	}
	return s.store.AddKey(label, priv)
}

func (s *KeychainService) ImportFile(path, label, passphrase string) (vault.Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return vault.Key{}, err
	}
	if label == "" {
		label = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return s.ImportText(label, string(data), passphrase)
}

func (s *KeychainService) ImportText(label, text, passphrase string) (vault.Key, error) {
	priv, err := vault.ParsePrivateKey([]byte(strings.TrimSpace(text)+"\n"), passphrase)
	if err != nil {
		return vault.Key{}, err
	}
	return s.store.AddKey(label, priv)
}

func (s *KeychainService) Delete(id string) error {
	return s.store.DeleteKey(id)
}

// PickFile opens a file dialog in ~/.ssh (or home) to choose a private key.
func (s *KeychainService) PickFile() (string, error) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ssh")
	if _, err := os.Stat(dir); err != nil {
		dir = home
	}
	return runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title:            "Pilih SSH private key",
		DefaultDirectory: dir,
	})
}

type KnownHostService struct{ *env }

func (s *KnownHostService) List() ([]knownhosts.Entry, error) {
	return s.hostKeys.List()
}

func (s *KnownHostService) Delete(id string) error {
	return s.hostKeys.Delete(id)
}

// Import merges ~/.ssh/known_hosts and returns how many entries were added.
func (s *KnownHostService) Import() (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}
	return s.hostKeys.ImportFrom(filepath.Join(home, ".ssh", "known_hosts"))
}
