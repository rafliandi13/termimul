package knownhosts

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"termimus/internal/sshtest"
)

func line(host string, k ssh.PublicKey) string {
	return host + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))
}

func TestImportListDelete(t *testing.T) {
	dir := t.TempDir()
	_, pubA := sshtest.WriteKey(t, "")
	_, pubB := sshtest.WriteKey(t, "")
	src := filepath.Join(dir, "openssh_known_hosts")
	os.WriteFile(src, []byte("# comment\n"+line("10.0.0.1", pubA)+"\n"+line("[10.0.0.2]:2222", pubB)+"\nnot a valid line\n"), 0o600)

	db := New(filepath.Join(dir, "known_hosts"))
	n, err := db.ImportFrom(src)
	if err != nil || n != 2 {
		t.Fatalf("import: n=%d err=%v", n, err)
	}
	if n, _ := db.ImportFrom(src); n != 0 {
		t.Fatalf("second import added %d duplicates", n)
	}
	list, _ := db.List()
	if len(list) != 2 || list[1].Hosts != "[10.0.0.2]:2222" {
		t.Fatalf("unexpected list %+v", list)
	}
	if err := db.Delete(list[0].ID); err != nil {
		t.Fatal(err)
	}
	list, _ = db.List()
	if len(list) != 1 || list[0].Fingerprint != ssh.FingerprintSHA256(pubB) {
		t.Fatalf("delete removed the wrong entry: %+v", list)
	}
}

func TestTrustOnFirstUse(t *testing.T) {
	db := New(filepath.Join(t.TempDir(), "known_hosts"))
	remote := &net.TCPAddr{IP: net.IPv4(10, 0, 0, 9), Port: 22}
	key := sshtest.HostKey(t).PublicKey()

	asked := 0
	refuse := db.HostKeyCallback(func(string, ssh.PublicKey) (bool, error) { asked++; return false, nil })
	if err := refuse("10.0.0.9:22", remote, key); err == nil {
		t.Fatal("unknown host accepted without confirmation")
	}
	if list, _ := db.List(); len(list) != 0 {
		t.Fatal("refused host was saved")
	}

	accept := db.HostKeyCallback(func(string, ssh.PublicKey) (bool, error) { asked++; return true, nil })
	if err := accept("10.0.0.9:22", remote, key); err != nil {
		t.Fatal(err)
	}
	// Known now: no second prompt.
	if err := accept("10.0.0.9:22", remote, key); err != nil || asked != 2 {
		t.Fatalf("err=%v, prompts=%d", err, asked)
	}
	// A different key for the same host must be refused without asking.
	changed := sshtest.HostKey(t).PublicKey()
	if err := accept("10.0.0.9:22", remote, changed); err == nil || !strings.Contains(err.Error(), "berubah") || asked != 2 {
		t.Fatalf("changed key: err=%v, prompts=%d", err, asked)
	}
}
