// Package services is the layer the frontend talks to. Each service is bound
// to Wails, which generates a TypeScript module for it under frontend/wailsjs.
// Services only translate between the UI and the internal packages; dialogs and
// events live here so the packages below stay free of Wails.
package services

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"

	"termimul/internal/knownhosts"
	"termimul/internal/sftpx"
	"termimul/internal/sshx"
	"termimul/internal/terminal"
	"termimul/internal/vault"
)

// env is shared by all services. ctx is set once the window has started.
type env struct {
	ctx      context.Context
	store    *vault.Store
	hostKeys *knownhosts.DB
	dialer   *sshx.Dialer
}

type Services struct {
	env        *env
	Hosts      *HostService
	Keychain   *KeychainService
	KnownHosts *KnownHostService
	Terminal   *TerminalService
	SFTP       *SFTPService
	Backup     *BackupService
}

// New opens the data folder (hosts.json, known_hosts) and wires the services.
func New(dataDir string) (*Services, error) {
	store, err := vault.Open(dataDir)
	if err != nil {
		return nil, err
	}
	e := &env{
		ctx:      context.Background(),
		store:    store,
		hostKeys: knownhosts.New(filepath.Join(dataDir, "known_hosts")),
	}
	e.dialer = &sshx.Dialer{Store: store, HostKeys: e.hostKeys, Confirm: e.confirmHostKey}
	return &Services{
		env:        e,
		Hosts:      &HostService{e},
		Keychain:   &KeychainService{e},
		KnownHosts: &KnownHostService{e},
		Terminal:   &TerminalService{env: e, sessions: terminal.NewManager()},
		SFTP:       &SFTPService{env: e, sessions: sftpx.NewManager()},
		Backup:     &BackupService{e},
	}, nil
}

// Bind lists the services to expose to the frontend.
func (s *Services) Bind() []any {
	return []any{s.Hosts, s.Keychain, s.KnownHosts, s.Terminal, s.SFTP, s.Backup}
}

func (s *Services) Startup(ctx context.Context) {
	s.env.ctx = ctx
}

func (s *Services) Shutdown(context.Context) {
	s.Terminal.sessions.CloseAll()
	s.SFTP.sessions.CloseAll()
}

func (e *env) confirmHostKey(hostname string, key ssh.PublicKey) (bool, error) {
	answer, err := runtime.MessageDialog(e.ctx, runtime.MessageDialogOptions{
		Type:  runtime.QuestionDialog,
		Title: "Host baru",
		Message: fmt.Sprintf("Belum pernah terhubung ke %s.\n\nFingerprint %s:\n%s\n\nPercayai host ini dan lanjutkan?",
			hostname, key.Type(), ssh.FingerprintSHA256(key)),
	})
	if err != nil {
		return false, err
	}
	return answer == "Yes" || answer == "Ya", nil
}
