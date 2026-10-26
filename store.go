package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	KeyPath       string `json:"keyPath"`
	HasPassword   bool   `json:"hasPassword"`
	HasPassphrase bool   `json:"hasPassphrase"`
}

// HostInput is sent by the host form. An empty Password or Passphrase keeps
// the stored value, so editing a host doesn't require retyping it.
type HostInput struct {
	Host
	Password   string `json:"password"`
	Passphrase string `json:"passphrase"`
}

type hostRecord struct {
	Host
	PasswordEnc   string `json:"passwordEnc,omitempty"`
	PassphraseEnc string `json:"passphraseEnc,omitempty"`
}

type hostSecrets struct {
	Password   string
	Passphrase string
}

type Store struct {
	mu      sync.Mutex
	path    string
	records []hostRecord
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "hosts.json")}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.records); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) List() []Host {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Host, 0, len(s.records))
	for _, r := range s.records {
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

func (s *Store) Save(in HostInput) (Host, error) {
	in.Label = strings.TrimSpace(in.Label)
	in.Group = strings.TrimSpace(in.Group)
	in.Address = strings.TrimSpace(in.Address)
	in.Username = strings.TrimSpace(in.Username)
	in.KeyPath = strings.TrimSpace(in.KeyPath)
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
	if in.AuthType == "key" && in.KeyPath == "" {
		return Host{}, errors.New("pilih file SSH key")
	}
	if in.Label == "" {
		in.Label = in.Address
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	if in.ID != "" {
		for i, r := range s.records {
			if r.ID == in.ID {
				idx = i
				break
			}
		}
	}
	var rec hostRecord
	if idx >= 0 {
		rec = s.records[idx]
	} else {
		in.ID = newID()
	}
	passwordEnc, passphraseEnc := rec.PasswordEnc, rec.PassphraseEnc
	rec.Host = in.Host

	if in.Password != "" {
		enc, err := sealSecret(in.Password)
		if err != nil {
			return Host{}, err
		}
		passwordEnc = enc
	}
	if in.Passphrase != "" {
		enc, err := sealSecret(in.Passphrase)
		if err != nil {
			return Host{}, err
		}
		passphraseEnc = enc
	}
	rec.PasswordEnc, rec.PassphraseEnc = passwordEnc, passphraseEnc

	if idx >= 0 {
		s.records[idx] = rec
	} else {
		s.records = append(s.records, rec)
	}
	if err := s.flush(); err != nil {
		return Host{}, err
	}
	return rec.public(), nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.records {
		if r.ID == id {
			s.records = append(s.records[:i], s.records[i+1:]...)
			return s.flush()
		}
	}
	return nil
}

func (s *Store) Get(id string) (Host, hostSecrets, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.records {
		if r.ID != id {
			continue
		}
		var sec hostSecrets
		var err error
		if sec.Password, err = openSecret(r.PasswordEnc); err != nil {
			return Host{}, sec, err
		}
		if sec.Passphrase, err = openSecret(r.PassphraseEnc); err != nil {
			return Host{}, sec, err
		}
		return r.public(), sec, nil
	}
	return Host{}, hostSecrets{}, errors.New("host tidak ditemukan")
}

// flush writes to a temp file first so a crash mid-write can't corrupt the host list.
func (s *Store) flush() error {
	data, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (r hostRecord) public() Host {
	h := r.Host
	h.HasPassword = r.PasswordEnc != ""
	h.HasPassphrase = r.PassphraseEnc != ""
	return h
}

func sealSecret(plain string) (string, error) {
	enc, err := protect([]byte(plain))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

func openSecret(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	plain, err := unprotect(raw)
	if err != nil {
		return "", errors.New("gagal membuka kredensial tersimpan (dibuat oleh akun Windows lain?)")
	}
	return string(plain), nil
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
