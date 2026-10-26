package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGeneratedKeysCanLogIn(t *testing.T) {
	for _, kind := range []string{"ed25519", "rsa", "ecdsa"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := NewStore(t.TempDir())
			priv, err := generatePrivateKey(kind)
			if err != nil {
				t.Fatal(err)
			}
			k, err := s.AddKey("laptop", priv)
			if err != nil {
				t.Fatal(err)
			}
			pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(k.PublicKey))
			if err != nil {
				t.Fatalf("public key line %q: %v", k.PublicKey, err)
			}
			addr := startServer(t, pub)

			h, err := s.Save(HostInput{Host: Host{Address: "x", Username: "u", AuthType: "key", KeyID: k.ID}})
			if err != nil {
				t.Fatal(err)
			}
			host, sec, err := s.Get(h.ID)
			if err != nil {
				t.Fatal(err)
			}
			auth, err := authMethods(host, sec)
			if err != nil {
				t.Fatal(err)
			}
			if err := dial(addr, auth); err != nil {
				t.Fatalf("login failed: %v", err)
			}
		})
	}
}

func TestImportEncryptedKeyAndDuplicates(t *testing.T) {
	path, pub := writeKey(t, "rahasia")
	data, _ := os.ReadFile(path)
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ImportKeyText("k", string(data), ""); err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("expected passphrase error, got %v", err)
	}
	k, err := app.ImportKeyFile(path, "", "rahasia")
	if err != nil {
		t.Fatal(err)
	}
	if k.Label != "id_ed25519" || k.Fingerprint != ssh.FingerprintSHA256(pub) {
		t.Fatalf("unexpected key %+v", k)
	}
	if _, err := app.ImportKeyFile(path, "again", "rahasia"); err == nil {
		t.Fatal("duplicate key was accepted")
	}
}

func TestDeleteKeyInUse(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	priv, _ := generatePrivateKey("ed25519")
	k, _ := s.AddKey("k", priv)
	h, _ := s.Save(HostInput{Host: Host{Label: "web", Address: "x", Username: "u", AuthType: "key", KeyID: k.ID}})
	if err := s.DeleteKey(k.ID); err == nil || !strings.Contains(err.Error(), "web") {
		t.Fatalf("expected in-use error, got %v", err)
	}
	s.Delete(h.ID)
	if err := s.DeleteKey(k.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMigratesV1KeyPathHosts(t *testing.T) {
	dir := t.TempDir()
	keyPath, pub := writeKey(t, "")
	v1 := []hostRecord{
		{Host: Host{ID: "a", Label: "a", Address: "x", Username: "u", AuthType: "key", KeyPath: keyPath}},
		{Host: Host{ID: "b", Label: "b", Address: "x", Username: "u", AuthType: "key", KeyPath: keyPath}},
		{Host: Host{ID: "c", Label: "c", Address: "x", Username: "u", AuthType: "key", KeyPath: filepath.Join(dir, "missing")}},
	}
	raw, _ := json.Marshal(v1)
	os.WriteFile(filepath.Join(dir, "hosts.json"), raw, 0o600)

	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	keys := s.ListKeys()
	if len(keys) != 1 || keys[0].Fingerprint != ssh.FingerprintSHA256(pub) {
		t.Fatalf("expected one migrated key, got %+v", keys)
	}
	for _, h := range s.List() {
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
	if s2, err := NewStore(dir); err != nil || len(s2.List()) != 3 {
		t.Fatalf("reload failed: %v", err)
	}
}

func TestKnownHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
	_, pubA := writeKey(t, "")
	_, pubB := writeKey(t, "")
	line := func(host string, k ssh.PublicKey) string {
		return host + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))
	}
	os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"),
		[]byte("# comment\n"+line("10.0.0.1", pubA)+"\n"+line("[10.0.0.2]:2222", pubB)+"\nnot a valid line\n"), 0o600)

	app, _ := NewApp(t.TempDir())
	n, err := app.ImportKnownHosts()
	if err != nil || n != 2 {
		t.Fatalf("import: n=%d err=%v", n, err)
	}
	if n, _ := app.ImportKnownHosts(); n != 0 {
		t.Fatalf("second import added %d duplicates", n)
	}
	list, _ := app.ListKnownHosts()
	if len(list) != 2 || list[1].Hosts != "[10.0.0.2]:2222" {
		t.Fatalf("unexpected list %+v", list)
	}
	if err := app.DeleteKnownHost(list[0].ID); err != nil {
		t.Fatal(err)
	}
	list, _ = app.ListKnownHosts()
	if len(list) != 1 || list[0].Fingerprint != ssh.FingerprintSHA256(pubB) {
		t.Fatalf("delete removed the wrong entry: %+v", list)
	}
}
