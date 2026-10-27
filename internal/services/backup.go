package services

import (
	"fmt"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"termimul/internal/backup"
	"termimul/internal/vault"
)

// BackupService exports and imports hosts, keychain and known hosts as a
// password-protected .termimul file.
type BackupService struct{ *env }

type ImportResult struct {
	vault.MergeResult
	KnownHostsAdded int `json:"knownHostsAdded"`
}

// *.termimus is accepted for files exported before the rename to Termimul.
var backupFilter = []runtime.FileFilter{{DisplayName: "Termimul backup (*.termimul)", Pattern: "*.termimul;*.termimus"}}

// Export asks where to save and writes the encrypted file. Returns the saved
// path, or "" if the user cancelled.
func (s *BackupService) Export(password string) (string, error) {
	if len([]rune(password)) < backup.MinPasswordLength {
		return "", fmt.Errorf("password export minimal %d karakter", backup.MinPasswordLength)
	}
	path, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		Title:           "Simpan backup Termimul",
		DefaultFilename: "termimul-" + time.Now().Format("2006-01-02") + ".termimul",
		Filters:         backupFilter,
	})
	if err != nil || path == "" {
		return "", err
	}
	snap, err := s.store.Snapshot()
	if err != nil {
		return "", err
	}
	lines, err := s.hostKeys.Lines()
	if err != nil {
		return "", err
	}
	data, err := backup.Encode(backup.Bundle{Vault: snap, KnownHosts: lines}, password)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o600)
}

// PickFile asks for a backup file to import.
func (s *BackupService) PickFile() (string, error) {
	return runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title:   "Pilih file backup Termimul",
		Filters: backupFilter,
	})
}

// Import decrypts path with password and merges it into the current data.
func (s *BackupService) Import(path, password string) (ImportResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ImportResult{}, err
	}
	b, err := backup.Decode(data, password)
	if err != nil {
		return ImportResult{}, err
	}
	merged, err := s.store.Merge(b.Vault)
	if err != nil {
		return ImportResult{}, err
	}
	added, err := s.hostKeys.Merge(b.KnownHosts)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{MergeResult: merged, KnownHostsAdded: added}, nil
}
