package services

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"termimul/internal/sshconfig"
	"termimul/internal/vault"
)

// SSHConfigHost is one ~/.ssh/config entry as the import preview shows it.
type SSHConfigHost struct {
	Alias     string `json:"alias"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	KeyPath   string `json:"keyPath"`   // first IdentityFile that exists, else the first listed
	KeyStatus string `json:"keyStatus"` // "", "ok", "encrypted", "missing" or "invalid"
	KeyError  string `json:"keyError"`
	Duplicate bool   `json:"duplicate"` // same address, port and user as a saved host
}

type SSHConfigPreview struct {
	Path  string          `json:"path"`
	Hosts []SSHConfigHost `json:"hosts"`
}

// SSHConfigImport is what the user picked in the preview. Passphrases are
// keyed by KeyPath and only needed for encrypted keys.
type SSHConfigImport struct {
	Aliases     []string          `json:"aliases"`
	Passphrases map[string]string `json:"passphrases"`
}

type SSHConfigResult struct {
	HostsAdded   int      `json:"hostsAdded"`
	HostsSkipped int      `json:"hostsSkipped"`
	KeysAdded    int      `json:"keysAdded"`
	NoKey        []string `json:"noKey"` // imported with password auth because their key couldn't be read
}

func sshConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

// PreviewSSHConfig lists the hosts in ~/.ssh/config without importing anything.
func (s *HostService) PreviewSSHConfig() (SSHConfigPreview, error) {
	path, err := sshConfigPath()
	if err != nil {
		return SSHConfigPreview{}, err
	}
	hosts, err := previewSSHConfig(s.store, path)
	return SSHConfigPreview{Path: path, Hosts: hosts}, err
}

// ImportSSHConfig re-reads ~/.ssh/config and imports the chosen aliases.
func (s *HostService) ImportSSHConfig(req SSHConfigImport) (SSHConfigResult, error) {
	path, err := sshConfigPath()
	if err != nil {
		return SSHConfigResult{}, err
	}
	return importSSHConfig(s.store, path, req)
}

func previewSSHConfig(store *vault.Store, path string) ([]SSHConfigHost, error) {
	entries, err := sshconfig.Parse(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("file %s tidak ditemukan", path)
	}
	if err != nil {
		return nil, fmt.Errorf("gagal membaca SSH config: %w", err)
	}
	existing := hostKeys(store.Hosts())
	defaultUser := localUser()
	out := make([]SSHConfigHost, 0, len(entries))
	for _, e := range entries {
		h := SSHConfigHost{Alias: e.Alias, Address: e.HostName, Port: e.Port, Username: e.User}
		if h.Username == "" {
			h.Username = defaultUser
		}
		h.KeyPath, h.KeyStatus, h.KeyError = inspectIdentity(e.IdentityFiles)
		h.Duplicate = existing[hostKey(h.Address, h.Port, h.Username)]
		out = append(out, h)
	}
	return out, nil
}

func importSSHConfig(store *vault.Store, path string, req SSHConfigImport) (SSHConfigResult, error) {
	hosts, err := previewSSHConfig(store, path)
	if err != nil {
		return SSHConfigResult{}, err
	}
	byAlias := make(map[string]SSHConfigHost, len(hosts))
	for _, h := range hosts {
		byAlias[h.Alias] = h
	}

	// Decrypt every key first, so a wrong passphrase stops the import before
	// anything is saved.
	var picked []SSHConfigHost
	keys := map[string]any{}
	for _, alias := range req.Aliases {
		h, ok := byAlias[alias]
		if !ok {
			return SSHConfigResult{}, fmt.Errorf("host %q sudah tidak ada di SSH config", alias)
		}
		picked = append(picked, h)
		if h.KeyStatus != "ok" && h.KeyStatus != "encrypted" {
			continue
		}
		if _, done := keys[h.KeyPath]; done {
			continue
		}
		data, err := os.ReadFile(h.KeyPath)
		if err != nil {
			return SSHConfigResult{}, err
		}
		priv, err := vault.ParsePrivateKey(data, req.Passphrases[h.KeyPath])
		if err != nil {
			return SSHConfigResult{}, fmt.Errorf("%s: %w", filepath.Base(h.KeyPath), err)
		}
		keys[h.KeyPath] = priv
	}

	var res SSHConfigResult
	keyIDs := map[string]string{}
	existing := hostKeys(store.Hosts())
	for _, h := range picked {
		k := hostKey(h.Address, h.Port, h.Username)
		if existing[k] {
			res.HostsSkipped++
			continue
		}
		in := vault.HostInput{Host: vault.Host{Label: h.Alias, Address: h.Address, Port: h.Port, Username: h.Username, AuthType: "password"}}
		if priv, ok := keys[h.KeyPath]; ok {
			id, found := keyIDs[h.KeyPath]
			if !found {
				key, err := store.AddKey(strings.TrimSuffix(filepath.Base(h.KeyPath), filepath.Ext(h.KeyPath)), priv)
				var dup *vault.DuplicateKeyError
				switch {
				case errors.As(err, &dup):
					key = dup.Existing
				case err != nil:
					return res, err
				default:
					res.KeysAdded++
				}
				id = key.ID
				keyIDs[h.KeyPath] = id
			}
			in.AuthType, in.KeyID = "key", id
		} else if h.KeyPath != "" {
			res.NoKey = append(res.NoKey, h.Alias)
		}
		if _, err := store.SaveHost(in); err != nil {
			return res, fmt.Errorf("%s: %w", h.Alias, err)
		}
		existing[k] = true
		res.HostsAdded++
	}
	return res, nil
}

// inspectIdentity picks the key ssh would try first and reports whether it can
// be imported as is.
func inspectIdentity(paths []string) (path, status, msg string) {
	if len(paths) == 0 {
		return "", "", ""
	}
	path = paths[0]
	var data []byte
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil {
			path, data = p, b
			break
		}
	}
	if data == nil {
		return path, "missing", "file key tidak ditemukan"
	}
	if _, err := vault.ParsePrivateKey(data, ""); err != nil {
		if errors.Is(err, vault.ErrPassphraseNeeded) {
			return path, "encrypted", ""
		}
		return path, "invalid", err.Error()
	}
	return path, "ok", ""
}

func hostKey(address string, port int, username string) string {
	return fmt.Sprintf("%s@%s:%d", username, strings.ToLower(address), port)
}

func hostKeys(hosts []vault.Host) map[string]bool {
	m := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		m[hostKey(h.Address, h.Port, h.Username)] = true
	}
	return m
}

// localUser is the name ssh would log in with when the config sets no User.
func localUser() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	// Windows reports DOMAIN\name.
	name := u.Username
	if i := strings.LastIndexByte(name, '\\'); i >= 0 {
		name = name[i+1:]
	}
	return name
}
