// Package knownhosts manages Termimul' own known_hosts file: trust-on-first-use
// verification, listing, deleting, and importing entries from OpenSSH.
package knownhosts

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"

	"termimul/internal/fsutil"
)

type Entry struct {
	ID          string `json:"id"`
	Hosts       string `json:"hosts"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
	Hashed      bool   `json:"hashed"`
}

// ConfirmFunc asks the user whether to trust a host seen for the first time.
type ConfirmFunc func(hostname string, key ssh.PublicKey) (bool, error)

type DB struct {
	mu   sync.Mutex
	path string
}

func New(path string) *DB {
	return &DB{path: path}
}

// HostKeyCallback verifies against the file: unknown hosts need confirm to
// return true (and are then saved), changed keys are always rejected.
func (db *DB) HostKeyCallback(confirm ConfirmFunc) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		db.mu.Lock()
		defer db.mu.Unlock()

		f, err := os.OpenFile(db.path, os.O_CREATE|os.O_RDONLY, 0o600)
		if err != nil {
			return err
		}
		f.Close()
		check, err := xknownhosts.New(db.path)
		if err != nil {
			return err
		}
		err = check(hostname, remote, key)
		var keyErr *xknownhosts.KeyError
		if err == nil || !errors.As(err, &keyErr) {
			return err
		}
		if len(keyErr.Want) > 0 {
			return fmt.Errorf("PERINGATAN: host key %s berubah (sekarang %s). Bisa jadi server di-reinstall, atau ada serangan man-in-the-middle. Koneksi dibatalkan",
				hostname, ssh.FingerprintSHA256(key))
		}

		ok, err := confirm(hostname, key)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("host key tidak dipercaya, koneksi dibatalkan")
		}
		kh, err := os.OpenFile(db.path, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer kh.Close()
		_, err = kh.WriteString(xknownhosts.Line([]string{xknownhosts.Normalize(hostname)}, key) + "\n")
		return err
	}
}

func (db *DB) List() ([]Entry, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	lines, err := readLines(db.path)
	if err != nil {
		return nil, err
	}
	out := []Entry{}
	for _, line := range lines {
		if e, ok := parseLine(line); ok {
			out = append(out, e)
		}
	}
	return out, nil
}

func (db *DB) Delete(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	lines, err := readLines(db.path)
	if err != nil {
		return err
	}
	kept := lines[:0]
	for _, line := range lines {
		if e, ok := parseLine(line); ok && e.ID == id {
			continue
		}
		kept = append(kept, line)
	}
	return writeLines(db.path, kept)
}

// ImportFrom merges entries from another known_hosts file (typically
// ~/.ssh/known_hosts) and returns how many new entries were added.
func (db *DB) ImportFrom(path string) (int, error) {
	src, err := readLines(path)
	if err != nil {
		return 0, err
	}
	if len(src) == 0 {
		return 0, errors.New("file ~/.ssh/known_hosts tidak ditemukan atau kosong")
	}
	return db.Merge(src)
}

// Lines returns the valid entries as raw known_hosts lines, for export.
func (db *DB) Lines() ([]string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	lines, err := readLines(db.path)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, line := range lines {
		if _, ok := parseLine(line); ok {
			out = append(out, line)
		}
	}
	return out, nil
}

// Merge appends valid lines that aren't present yet and returns how many were added.
func (db *DB) Merge(src []string) (int, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	lines, err := readLines(db.path)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for _, line := range lines {
		seen[line] = true
	}
	added := 0
	for _, line := range src {
		if _, ok := parseLine(line); ok && !seen[line] {
			lines = append(lines, line)
			seen[line] = true
			added++
		}
	}
	if added == 0 {
		return 0, nil
	}
	return added, writeLines(db.path, lines)
}

// readLines returns every non-blank line, keeping comments and lines we
// can't parse so rewriting the file never drops them.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, sc.Err()
}

func writeLines(path string, lines []string) error {
	var buf bytes.Buffer
	for _, l := range lines {
		buf.WriteString(l)
		buf.WriteByte('\n')
	}
	return fsutil.WriteFileAtomic(path, buf.Bytes(), 0o600)
}

func parseLine(line string) (Entry, bool) {
	if strings.HasPrefix(line, "#") {
		return Entry{}, false
	}
	_, hosts, key, _, _, err := ssh.ParseKnownHosts([]byte(line))
	if err != nil || len(hosts) == 0 {
		return Entry{}, false
	}
	sum := sha256.Sum256([]byte(line))
	e := Entry{
		ID:          hex.EncodeToString(sum[:8]),
		Hosts:       strings.Join(hosts, ", "),
		KeyType:     key.Type(),
		Fingerprint: ssh.FingerprintSHA256(key),
	}
	if strings.HasPrefix(hosts[0], "|1|") {
		e.Hashed = true
		e.Hosts = "(nama host di-hash)"
	}
	return e, true
}
