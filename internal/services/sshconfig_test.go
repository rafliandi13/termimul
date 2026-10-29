package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"termimul/internal/sshtest"
	"termimul/internal/vault"
)

func TestImportSSHConfig(t *testing.T) {
	plainKey, _ := sshtest.WriteKey(t, "")
	lockedKey, _ := sshtest.WriteKey(t, "rahasia")
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	text := `
Host web
    HostName 10.0.0.5
    User deploy
    IdentityFile ` + plainKey + `
Host web-again
    HostName 10.0.0.5
    User deploy
    IdentityFile ` + plainKey + `
Host locked
    HostName 10.0.0.6
    User root
    IdentityFile ` + lockedKey + `
Host gone
    HostName 10.0.0.7
    User root
    IdentityFile ` + filepath.Join(dir, "missing") + `
Host saved
    HostName 10.0.0.8
    User admin
Host *
    Port 2222
`
	if err := os.WriteFile(config, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := vault.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveHost(vault.HostInput{Host: vault.Host{Address: "10.0.0.8", Port: 2222, Username: "admin"}, Password: "x"}); err != nil {
		t.Fatal(err)
	}

	preview, err := previewSSHConfig(store, config)
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]SSHConfigHost{}
	for _, h := range preview {
		status[h.Alias] = h
	}
	if len(preview) != 5 || status["web"].KeyStatus != "ok" || status["locked"].KeyStatus != "encrypted" ||
		status["gone"].KeyStatus != "missing" || !status["saved"].Duplicate || status["web"].Port != 2222 {
		t.Fatalf("unexpected preview %+v", preview)
	}

	all := SSHConfigImport{Aliases: []string{"web", "web-again", "locked", "gone", "saved"}}
	if _, err := importSSHConfig(store, config, all); err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("expected passphrase error, got %v", err)
	}
	all.Passphrases = map[string]string{lockedKey: "salah"}
	if _, err := importSSHConfig(store, config, all); err == nil {
		t.Fatal("wrong passphrase was accepted")
	}
	if n := len(store.Hosts()); n != 1 {
		t.Fatalf("failed import saved %d hosts", n-1)
	}

	all.Passphrases[lockedKey] = "rahasia"
	res, err := importSSHConfig(store, config, all)
	if err != nil {
		t.Fatal(err)
	}
	if res.HostsAdded != 3 || res.HostsSkipped != 2 || res.KeysAdded != 2 || len(res.NoKey) != 1 || res.NoKey[0] != "gone" {
		t.Fatalf("unexpected result %+v", res)
	}
	for _, h := range store.Hosts() {
		switch h.Label {
		case "web", "locked":
			if h.AuthType != "key" || h.KeyID == "" || h.Port != 2222 {
				t.Fatalf("%s: %+v", h.Label, h)
			}
		case "gone":
			if h.AuthType != "password" {
				t.Fatalf("gone: %+v", h)
			}
		}
	}

	// A second run finds everything already saved.
	res, err = importSSHConfig(store, config, all)
	if err != nil {
		t.Fatal(err)
	}
	if res.HostsAdded != 0 || res.HostsSkipped != 5 || res.KeysAdded != 0 {
		t.Fatalf("re-import: %+v", res)
	}
}
