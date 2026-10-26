package backup

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"termimus/internal/knownhosts"
	"termimus/internal/sshtest"
	"termimus/internal/vault"
)

const pass = "export-pass-123"

// TestMoveToAnotherDevice exports from one data folder and imports into another,
// as when moving to a new laptop.
func TestMoveToAnotherDevice(t *testing.T) {
	src, _ := vault.Open(t.TempDir())
	priv, _ := vault.GeneratePrivateKey("ed25519")
	key, _ := src.AddKey("laptop", priv)
	src.SaveHost(vault.HostInput{Host: vault.Host{Label: "db", Group: "Prod", Address: "10.0.0.5", Username: "root"}, Password: "db-secret"})
	src.SaveHost(vault.HostInput{Host: vault.Host{Label: "web", Address: "10.0.0.6", Username: "ubuntu", AuthType: "key", KeyID: key.ID}})

	srcKH := knownhosts.New(filepath.Join(t.TempDir(), "known_hosts"))
	_, pub := sshtest.WriteKey(t, "")
	srcKH.Merge([]string{"10.0.0.5 " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))})

	snap, err := src.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	lines, _ := srcKH.Lines()
	file, err := Encode(Bundle{Vault: snap, KnownHosts: lines}, pass)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"db-secret", "10.0.0.5", "BEGIN OPENSSH", "root"} {
		if bytes.Contains(file, []byte(secret)) {
			t.Fatalf("export file leaks %q in plain text", secret)
		}
	}

	b, err := Decode(file, pass)
	if err != nil {
		t.Fatal(err)
	}
	dst, _ := vault.Open(t.TempDir())
	res, err := dst.Merge(b.Vault)
	if err != nil {
		t.Fatal(err)
	}
	if res.HostsAdded != 2 || res.KeysAdded != 1 {
		t.Fatalf("unexpected result %+v", res)
	}
	dstKH := knownhosts.New(filepath.Join(t.TempDir(), "known_hosts"))
	if n, _ := dstKH.Merge(b.KnownHosts); n != 1 {
		t.Fatalf("known hosts added: %d", n)
	}

	var web vault.Host
	for _, h := range dst.Hosts() {
		host, sec, err := dst.Host(h.ID)
		if err != nil {
			t.Fatal(err)
		}
		switch h.Label {
		case "db":
			if sec.Password != "db-secret" || host.Group != "Prod" {
				t.Errorf("db host not restored: %+v %q", host, sec.Password)
			}
		case "web":
			web = host
			if sec.Signer == nil {
				t.Error("web host lost its key")
			}
		}
	}
	if web.KeyID == "" || web.KeyID != dst.Keys()[0].ID {
		t.Fatalf("key reference not remapped: host %q, keys %+v", web.KeyID, dst.Keys())
	}

	// Importing the same file again adds nothing.
	again, err := dst.Merge(b.Vault)
	if err != nil || again.HostsAdded != 0 || again.HostsSkipped != 2 || again.KeysSkipped != 1 {
		t.Fatalf("second import: %+v, %v", again, err)
	}
}

func TestDecodeRejects(t *testing.T) {
	file, err := Encode(Bundle{}, pass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(file, "wrong-password"); err == nil || !strings.Contains(err.Error(), "password salah") {
		t.Fatalf("wrong password: %v", err)
	}

	var env envelope
	json.Unmarshal(file, &env)
	env.Data[len(env.Data)-1] ^= 1
	tampered, _ := json.Marshal(env)
	if _, err := Decode(tampered, pass); err == nil {
		t.Fatal("tampered file accepted")
	}

	json.Unmarshal(file, &env)
	env.KDF.Memory = 1 << 31
	greedy, _ := json.Marshal(env)
	if _, err := Decode(greedy, pass); err == nil || !strings.Contains(err.Error(), "tidak valid") {
		t.Fatalf("oversized KDF parameters: %v", err)
	}

	if _, err := Decode([]byte(`{"hosts":[]}`), pass); err == nil {
		t.Fatal("non-backup JSON accepted")
	}
	if _, err := Encode(Bundle{}, "short"); err == nil {
		t.Fatal("short password accepted")
	}
}
