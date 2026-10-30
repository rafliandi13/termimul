// Package vault persists hosts and the SSH keychain to hosts.json, with all
// secrets (passwords, private keys) encrypted through package secret.
package vault

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"termimul/internal/fsutil"
	"termimul/internal/secret"
)

// Host is what the frontend sees. Secrets never leave the backend; only
// HasPassword / HasPassphrase tell the UI whether one is stored.
type Host struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Group         string `json:"group"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	AuthType      string `json:"authType"` // "password" or "key"
	KeyID         string `json:"keyId"`    // keychain entry
	KeyPath       string `json:"keyPath"`  // legacy: key file on disk, only for hosts that couldn't be migrated
	HasPassword   bool   `json:"hasPassword"`
	HasPassphrase bool   `json:"hasPassphrase"`
}

// HostInput is sent by the host form. An empty Password keeps the stored
// value, so editing a host doesn't require retyping it.
type HostInput struct {
	Host
	Password   string `json:"password"`
	Passphrase string `json:"passphrase"`
}

// Secrets are the decrypted credentials for one host.
type Secrets struct {
	Password   string
	Passphrase string     // only for legacy KeyPath hosts
	Signer     ssh.Signer // set when the host uses a keychain key
}

type hostRecord struct {
	Host
	PasswordEnc   string `json:"passwordEnc,omitempty"`
	PassphraseEnc string `json:"passphraseEnc,omitempty"`
}

type file struct {
	Version  int          `json:"version"`
	Hosts    []hostRecord `json:"hosts"`
	Keys     []keyRecord  `json:"keys"`
	Tunnels  []Tunnel     `json:"tunnels"`
	Snippets []Snippet    `json:"snippets"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data file
}

// Open loads (or creates) hosts.json in dir, upgrading older formats.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "hosts.json"), data: file{Version: 2}}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	// Version 1 was a bare array of hosts.
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '[' {
		err = json.Unmarshal(trimmed, &s.data.Hosts)
	} else {
		err = json.Unmarshal(raw, &s.data)
	}
	if err != nil {
		return nil, err
	}
	if s.migrateKeyPaths() || s.data.Version < 2 {
		s.data.Version = 2
		if err := s.flush(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// migrateKeyPaths moves key files referenced by hosts into the keychain so hosts
// keep working on machines where that path doesn't exist. Hosts whose key can't
// be read are left on the legacy path.
func (s *Store) migrateKeyPaths() bool {
	changed := false
	for i := range s.data.Hosts {
		h := &s.data.Hosts[i]
		if h.AuthType != "key" || h.KeyID != "" || h.KeyPath == "" {
			continue
		}
		data, err := os.ReadFile(fsutil.ExpandHome(h.KeyPath))
		if err != nil {
			continue
		}
		passphrase, err := secret.Open(h.PassphraseEnc)
		if err != nil {
			continue
		}
		priv, err := ParsePrivateKey(data, passphrase)
		if err != nil {
			continue
		}
		k, err := s.addKeyLocked(strings.TrimSuffix(filepath.Base(h.KeyPath), filepath.Ext(h.KeyPath)), priv)
		var dup *DuplicateKeyError
		if errors.As(err, &dup) {
			k = dup.Existing
		} else if err != nil {
			continue
		}
		h.KeyID, h.KeyPath, h.PassphraseEnc = k.ID, "", ""
		changed = true
	}
	return changed
}

func (s *Store) Hosts() []Host {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Host, 0, len(s.data.Hosts))
	for _, r := range s.data.Hosts {
		out = append(out, r.public())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return strings.ToLower(out[i].Group) < strings.ToLower(out[j].Group)
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	return out
}

func (s *Store) SaveHost(in HostInput) (Host, error) {
	in.Label = strings.TrimSpace(in.Label)
	in.Group = strings.TrimSpace(in.Group)
	in.Address = strings.TrimSpace(in.Address)
	in.Username = strings.TrimSpace(in.Username)
	if in.Address == "" {
		return Host{}, errors.New("alamat host wajib diisi")
	}
	if in.Username == "" {
		return Host{}, errors.New("username wajib diisi")
	}
	if in.Port <= 0 || in.Port > 65535 {
		in.Port = 22
	}
	if in.AuthType != "key" {
		in.AuthType = "password"
	}
	if in.Label == "" {
		in.Label = in.Address
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if in.AuthType == "key" {
		if in.KeyID != "" && s.keyIndex(in.KeyID) < 0 {
			return Host{}, errors.New("key yang dipilih sudah tidak ada di keychain")
		}
		if in.KeyID == "" && in.KeyPath == "" {
			return Host{}, errors.New("pilih SSH key dari keychain")
		}
		if in.KeyID != "" {
			in.KeyPath = ""
		}
	}

	idx := -1
	if in.ID != "" {
		for i, r := range s.data.Hosts {
			if r.ID == in.ID {
				idx = i
				break
			}
		}
	}
	var rec hostRecord
	if idx >= 0 {
		rec = s.data.Hosts[idx]
	} else {
		in.ID = newID()
	}
	passwordEnc, passphraseEnc := rec.PasswordEnc, rec.PassphraseEnc
	rec.Host = in.Host

	if in.Password != "" {
		enc, err := secret.Seal(in.Password)
		if err != nil {
			return Host{}, err
		}
		passwordEnc = enc
	}
	if in.Passphrase != "" {
		enc, err := secret.Seal(in.Passphrase)
		if err != nil {
			return Host{}, err
		}
		passphraseEnc = enc
	}
	if in.KeyID != "" {
		passphraseEnc = "" // keychain keys are stored already decrypted
	}
	rec.PasswordEnc, rec.PassphraseEnc = passwordEnc, passphraseEnc

	if idx >= 0 {
		s.data.Hosts[idx] = rec
	} else {
		s.data.Hosts = append(s.data.Hosts, rec)
	}
	if err := s.flush(); err != nil {
		return Host{}, err
	}
	return rec.public(), nil
}

func (s *Store) DeleteHost(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.data.Hosts {
		if r.ID == id {
			s.data.Hosts = append(s.data.Hosts[:i], s.data.Hosts[i+1:]...)
			// Tunnels can't outlive the host they go through.
			kept := s.data.Tunnels[:0]
			for _, t := range s.data.Tunnels {
				if t.HostID != id {
					kept = append(kept, t)
				}
			}
			s.data.Tunnels = kept
			return s.flush()
		}
	}
	return nil
}

// Host returns a host together with its decrypted credentials.
func (s *Store) Host(id string) (Host, Secrets, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.data.Hosts {
		if r.ID != id {
			continue
		}
		var sec Secrets
		var err error
		if sec.Password, err = secret.Open(r.PasswordEnc); err != nil {
			return Host{}, sec, err
		}
		if sec.Passphrase, err = secret.Open(r.PassphraseEnc); err != nil {
			return Host{}, sec, err
		}
		if r.AuthType == "key" && r.KeyID != "" {
			if sec.Signer, err = s.signerLocked(r.KeyID); err != nil {
				return Host{}, sec, err
			}
		}
		return r.public(), sec, nil
	}
	return Host{}, Secrets{}, errors.New("host tidak ditemukan")
}

func (s *Store) Keys() []Key {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Key, 0, len(s.data.Keys))
	for _, k := range s.data.Keys {
		out = append(out, k.Key)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label) })
	return out
}

// DuplicateKeyError is returned when a key with the same fingerprint is already stored.
type DuplicateKeyError struct{ Existing Key }

func (e *DuplicateKeyError) Error() string {
	return fmt.Sprintf("key ini sudah ada di keychain sebagai \"%s\"", e.Existing.Label)
}

func (s *Store) AddKey(label string, priv any) (Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, err := s.addKeyLocked(label, priv)
	if err != nil {
		return Key{}, err
	}
	return k, s.flush()
}

func (s *Store) addKeyLocked(label string, priv any) (Key, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "key"
	}
	k, pemBytes, err := describeKey(priv, label)
	if err != nil {
		return Key{}, err
	}
	for _, existing := range s.data.Keys {
		if existing.Fingerprint == k.Fingerprint {
			return Key{}, &DuplicateKeyError{existing.Key}
		}
	}
	enc, err := secret.Seal(string(pemBytes))
	if err != nil {
		return Key{}, err
	}
	k.ID = newID()
	k.CreatedAt = time.Now().Unix()
	s.data.Keys = append(s.data.Keys, keyRecord{Key: k, PrivateEnc: enc})
	return k, nil
}

func (s *Store) DeleteKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var users []string
	for _, h := range s.data.Hosts {
		if h.KeyID == id {
			users = append(users, h.Label)
		}
	}
	if len(users) > 0 {
		return fmt.Errorf("key masih dipakai oleh: %s", strings.Join(users, ", "))
	}
	if i := s.keyIndex(id); i >= 0 {
		s.data.Keys = append(s.data.Keys[:i], s.data.Keys[i+1:]...)
		return s.flush()
	}
	return nil
}

func (s *Store) keyIndex(id string) int {
	for i, k := range s.data.Keys {
		if k.ID == id {
			return i
		}
	}
	return -1
}

func (s *Store) signerLocked(keyID string) (ssh.Signer, error) {
	i := s.keyIndex(keyID)
	if i < 0 {
		return nil, errors.New("key untuk host ini sudah dihapus dari keychain")
	}
	pemText, err := secret.Open(s.data.Keys[i].PrivateEnc)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey([]byte(pemText))
}

func (s *Store) flush() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path, data, 0o600)
}

func (r hostRecord) public() Host {
	h := r.Host
	h.HasPassword = r.PasswordEnc != ""
	h.HasPassphrase = r.PassphraseEnc != ""
	return h
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
