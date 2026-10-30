package vault

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"termimul/internal/sshtest"
)

func TestImportEncryptedKeyAndDuplicates(t *testing.T) {
	path, pub := sshtest.WriteKey(t, "rahasia")
	data, _ := os.ReadFile(path)
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePrivateKey(data, ""); err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("expected passphrase error, got %v", err)
	}
	priv, err := ParsePrivateKey(data, "rahasia")
	if err != nil {
		t.Fatal(err)
	}
	k, err := store.AddKey("id_ed25519", priv)
	if err != nil {
		t.Fatal(err)
	}
	if k.Type != "ED25519" || k.Fingerprint != ssh.FingerprintSHA256(pub) {
		t.Fatalf("unexpected key %+v", k)
	}
	var dup *DuplicateKeyError
	if _, err := store.AddKey("again", priv); !errors.As(err, &dup) || dup.Existing.ID != k.ID {
		t.Fatalf("duplicate key was accepted: %v", err)
	}
}

func TestDeleteKeyInUse(t *testing.T) {
	store, _ := Open(t.TempDir())
	priv, _ := GeneratePrivateKey("ed25519")
	k, _ := store.AddKey("k", priv)
	h, _ := store.SaveHost(HostInput{Host: Host{Label: "web", Address: "x", Username: "u", AuthType: "key", KeyID: k.ID}})
	if err := store.DeleteKey(k.ID); err == nil || !strings.Contains(err.Error(), "web") {
		t.Fatalf("expected in-use error, got %v", err)
	}
	store.DeleteHost(h.ID)
	if err := store.DeleteKey(k.ID); err != nil {
		t.Fatal(err)
	}
}

func TestPasswordIsEncryptedAndKeptOnEdit(t *testing.T) {
	dir := t.TempDir()
	store, _ := Open(dir)
	h, err := store.SaveHost(HostInput{Host: Host{Address: "10.0.0.1", Username: "root"}, Password: "s3cret-pass"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "hosts.json"))
	if strings.Contains(string(raw), "s3cret-pass") {
		t.Fatal("password stored in plain text")
	}
	// Editing without retyping the password keeps it.
	if _, err := store.SaveHost(HostInput{Host: Host{ID: h.ID, Label: "renamed", Address: "10.0.0.1", Username: "root"}}); err != nil {
		t.Fatal(err)
	}
	reopened, _ := Open(dir)
	got, sec, err := reopened.Host(h.ID)
	if err != nil || sec.Password != "s3cret-pass" || got.Label != "renamed" || !got.HasPassword {
		t.Fatalf("got %+v, password %q, err %v", got, sec.Password, err)
	}
}

func TestMigratesV1KeyPathHosts(t *testing.T) {
	dir := t.TempDir()
	keyPath, pub := sshtest.WriteKey(t, "")
	v1 := []hostRecord{
		{Host: Host{ID: "a", Label: "a", Address: "x", Username: "u", AuthType: "key", KeyPath: keyPath}},
		{Host: Host{ID: "b", Label: "b", Address: "x", Username: "u", AuthType: "key", KeyPath: keyPath}},
		{Host: Host{ID: "c", Label: "c", Address: "x", Username: "u", AuthType: "key", KeyPath: filepath.Join(dir, "missing")}},
	}
	raw, _ := json.Marshal(v1)
	os.WriteFile(filepath.Join(dir, "hosts.json"), raw, 0o600)

	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	keys := store.Keys()
	if len(keys) != 1 || keys[0].Fingerprint != ssh.FingerprintSHA256(pub) {
		t.Fatalf("expected one migrated key, got %+v", keys)
	}
	for _, h := range store.Hosts() {
		switch h.ID {
		case "a", "b":
			if h.KeyID != keys[0].ID || h.KeyPath != "" {
				t.Errorf("host %s not migrated: %+v", h.ID, h)
			}
		case "c":
			if h.KeyID != "" || h.KeyPath == "" {
				t.Errorf("unreadable key should stay on legacy path: %+v", h)
			}
		}
	}
	// Reloading must read the new format.
	if s2, err := Open(dir); err != nil || len(s2.Hosts()) != 3 {
		t.Fatalf("reload failed: %v", err)
	}
}

func TestSnippetsPersistAndMerge(t *testing.T) {
	dir := t.TempDir()
	store, _ := Open(dir)
	if _, err := store.SaveSnippet(Snippet{Name: "x"}); err == nil {
		t.Fatal("snippet without a command was accepted")
	}
	sn, err := store.SaveSnippet(Snippet{Command: "  df -h\nfree -m "})
	if err != nil || sn.ID == "" || sn.Name != "df -h" || sn.Command != "df -h\nfree -m" {
		t.Fatalf("got %+v, %v", sn, err)
	}
	sn.Name = "disk"
	if _, err := store.SaveSnippet(sn); err != nil {
		t.Fatal(err)
	}
	reopened, _ := Open(dir)
	if got := reopened.Snippets(); len(got) != 1 || got[0].Name != "disk" {
		t.Fatalf("after reload: %+v", got)
	}

	snap, _ := reopened.Snapshot()
	other, _ := Open(t.TempDir())
	res, err := other.Merge(snap)
	if err != nil || res.SnippetsAdded != 1 {
		t.Fatalf("merge: %+v, %v", res, err)
	}
	if res, _ := other.Merge(snap); res.SnippetsAdded != 0 {
		t.Fatal("snippet merged twice")
	}

	if err := reopened.DeleteSnippet(sn.ID); err != nil || len(reopened.Snippets()) != 0 {
		t.Fatalf("delete: %v", err)
	}
}
