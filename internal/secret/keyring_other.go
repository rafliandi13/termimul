//go:build !windows

package secret

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

// On macOS and Linux a random 256-bit vault key lives in the OS credential store
// (macOS Keychain, or Secret Service such as GNOME Keyring / KWallet), and
// secrets are AES-GCM encrypted with it.
//
// Headless Linux machines often have no Secret Service. Then the key falls back
// to a 0600 file next to hosts.json, which protects against other users but not
// against anyone who can read this user's files.

const (
	keyringService = "termimus"
	keyringUser    = "vault-key"
)

var (
	vaultKeyOnce sync.Once
	vaultKey     []byte
	vaultKeyErr  error
)

func protect(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	key, err := loadVaultKey()
	if err != nil {
		return nil, err
	}
	return SealWithKey(key, plain)
}

func unprotect(sealed []byte) ([]byte, error) {
	if len(sealed) == 0 {
		return nil, nil
	}
	key, err := loadVaultKey()
	if err != nil {
		return nil, err
	}
	return OpenWithKey(key, sealed)
}

func loadVaultKey() ([]byte, error) {
	vaultKeyOnce.Do(func() { vaultKey, vaultKeyErr = getOrCreateVaultKey() })
	return vaultKey, vaultKeyErr
}

func getOrCreateVaultKey() ([]byte, error) {
	keyFile, err := vaultKeyFile()
	if err != nil {
		return nil, err
	}
	// A file key, once created, stays authoritative so secrets sealed with it remain readable.
	if data, err := os.ReadFile(keyFile); err == nil {
		return decodeVaultKey(string(data))
	}

	stored, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		return decodeVaultKey(stored)
	}
	keyringUsable := errors.Is(err, keyring.ErrNotFound)

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	if keyringUsable && keyring.Set(keyringService, keyringUser, encoded) == nil {
		return key, nil
	}
	if err := os.MkdirAll(filepath.Dir(keyFile), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyFile, []byte(encoded), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func decodeVaultKey(s string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(key) != 32 {
		return nil, errors.New("kunci vault rusak")
	}
	return key, nil
}

func vaultKeyFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "termimus", "vault.key"), nil
}
