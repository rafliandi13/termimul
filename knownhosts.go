package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

type KnownHost struct {
	ID          string `json:"id"`
	Hosts       string `json:"hosts"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
	Hashed      bool   `json:"hashed"`
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

func parseKnownHostLine(line string) (KnownHost, bool) {
	if strings.HasPrefix(line, "#") {
		return KnownHost{}, false
	}
	_, hosts, key, _, _, err := ssh.ParseKnownHosts([]byte(line))
	if err != nil || len(hosts) == 0 {
		return KnownHost{}, false
	}
	sum := sha256.Sum256([]byte(line))
	kh := KnownHost{
		ID:          hex.EncodeToString(sum[:8]),
		Hosts:       strings.Join(hosts, ", "),
		KeyType:     key.Type(),
		Fingerprint: ssh.FingerprintSHA256(key),
	}
	if strings.HasPrefix(hosts[0], "|1|") {
		kh.Hashed = true
		kh.Hosts = "(nama host di-hash)"
	}
	return kh, true
}

func (a *App) ListKnownHosts() ([]KnownHost, error) {
	a.khMu.Lock()
	defer a.khMu.Unlock()
	lines, err := readLines(a.knownHostsPath)
	if err != nil {
		return nil, err
	}
	out := []KnownHost{}
	for _, line := range lines {
		if kh, ok := parseKnownHostLine(line); ok {
			out = append(out, kh)
		}
	}
	return out, nil
}

func (a *App) DeleteKnownHost(id string) error {
	a.khMu.Lock()
	defer a.khMu.Unlock()
	lines, err := readLines(a.knownHostsPath)
	if err != nil {
		return err
	}
	kept := lines[:0]
	for _, line := range lines {
		if kh, ok := parseKnownHostLine(line); ok && kh.ID == id {
			continue
		}
		kept = append(kept, line)
	}
	return writeLines(a.knownHostsPath, kept)
}

// ImportKnownHosts merges the user's OpenSSH known_hosts into ours and
// returns how many new entries were added.
func (a *App) ImportKnownHosts() (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}
	src, err := readLines(filepath.Join(home, ".ssh", "known_hosts"))
	if err != nil {
		return 0, err
	}
	if len(src) == 0 {
		return 0, errors.New("file ~/.ssh/known_hosts tidak ditemukan atau kosong")
	}

	a.khMu.Lock()
	defer a.khMu.Unlock()
	lines, err := readLines(a.knownHostsPath)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for _, line := range lines {
		seen[line] = true
	}
	added := 0
	for _, line := range src {
		if _, ok := parseKnownHostLine(line); ok && !seen[line] {
			lines = append(lines, line)
			seen[line] = true
			added++
		}
	}
	if added == 0 {
		return 0, nil
	}
	return added, writeLines(a.knownHostsPath, lines)
}

func writeLines(path string, lines []string) error {
	var buf bytes.Buffer
	for _, l := range lines {
		buf.WriteString(l)
		buf.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
