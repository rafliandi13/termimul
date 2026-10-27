package vault

import (
	"errors"
	"strings"

	"termimul/internal/secret"
)

// PortableHost and PortableKey carry secrets in plain text. They only exist in
// memory while an encrypted backup is being written or read (package backup).
type PortableHost struct {
	Host
	Password   string `json:"password,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

type PortableKey struct {
	Key
	PrivateKey string `json:"privateKey"` // OpenSSH PEM
}

type Snapshot struct {
	Hosts []PortableHost `json:"hosts"`
	Keys  []PortableKey  `json:"keys"`
}

type MergeResult struct {
	HostsAdded   int `json:"hostsAdded"`
	HostsSkipped int `json:"hostsSkipped"`
	KeysAdded    int `json:"keysAdded"`
	KeysSkipped  int `json:"keysSkipped"`
}

// Snapshot returns every host and key with secrets decrypted, for export.
func (s *Store) Snapshot() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := Snapshot{Hosts: []PortableHost{}, Keys: []PortableKey{}}
	for _, k := range s.data.Keys {
		pemText, err := secret.Open(k.PrivateEnc)
		if err != nil {
			return Snapshot{}, err
		}
		snap.Keys = append(snap.Keys, PortableKey{Key: k.Key, PrivateKey: pemText})
	}
	for _, r := range s.data.Hosts {
		p := PortableHost{Host: r.public()}
		var err error
		if p.Password, err = secret.Open(r.PasswordEnc); err != nil {
			return Snapshot{}, err
		}
		if p.Passphrase, err = secret.Open(r.PassphraseEnc); err != nil {
			return Snapshot{}, err
		}
		snap.Hosts = append(snap.Hosts, p)
	}
	return snap, nil
}

// Merge adds hosts and keys from an imported snapshot without touching existing
// ones. A key is a duplicate if its fingerprint is already stored; a host is a
// duplicate if one with the same label, address, port and user exists. Either
// everything is saved or, on error, nothing is.
func (s *Store) Merge(snap Snapshot) (MergeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	before := s.data
	before.Hosts = append([]hostRecord(nil), s.data.Hosts...)
	before.Keys = append([]keyRecord(nil), s.data.Keys...)
	res, err := s.mergeLocked(snap)
	if err == nil {
		err = s.flush()
	}
	if err != nil {
		s.data = before
		return MergeResult{}, err
	}
	return res, nil
}

func (s *Store) mergeLocked(snap Snapshot) (MergeResult, error) {
	var res MergeResult
	keyIDs := map[string]string{} // id in the backup -> id in this store
	for _, pk := range snap.Keys {
		priv, err := ParsePrivateKey([]byte(pk.PrivateKey), "")
		if err != nil {
			return res, errors.New("key \"" + pk.Label + "\" di file backup rusak")
		}
		k, err := s.addKeyLocked(pk.Label, priv)
		var dup *DuplicateKeyError
		switch {
		case errors.As(err, &dup):
			keyIDs[pk.ID] = dup.Existing.ID
			res.KeysSkipped++
		case err != nil:
			return res, err
		default:
			keyIDs[pk.ID] = k.ID
			res.KeysAdded++
		}
	}

	for _, ph := range snap.Hosts {
		if s.hasHostLocked(ph.Host) {
			res.HostsSkipped++
			continue
		}
		rec := hostRecord{Host: ph.Host}
		rec.ID = newID()
		rec.HasPassword, rec.HasPassphrase = false, false
		if ph.KeyID != "" {
			rec.KeyID = keyIDs[ph.KeyID] // "" if the backup lost the key; the host form will ask for one
		}
		var err error
		if rec.PasswordEnc, err = secret.Seal(ph.Password); err != nil {
			return res, err
		}
		if rec.PassphraseEnc, err = secret.Seal(ph.Passphrase); err != nil {
			return res, err
		}
		s.data.Hosts = append(s.data.Hosts, rec)
		res.HostsAdded++
	}
	return res, nil
}

func (s *Store) hasHostLocked(h Host) bool {
	for _, r := range s.data.Hosts {
		if strings.EqualFold(r.Address, h.Address) && r.Port == h.Port && r.Username == h.Username && r.Label == h.Label {
			return true
		}
	}
	return false
}
