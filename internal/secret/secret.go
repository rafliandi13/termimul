// Package secret encrypts credentials at rest with the operating system's
// credential store: DPAPI on Windows, Keychain on macOS, Secret Service on Linux.
package secret

import (
	"encoding/base64"
	"errors"
)

// Seal encrypts plain and returns it base64-encoded. Empty input stays empty.
func Seal(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	enc, err := protect([]byte(plain))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

// Open reverses Seal.
func Open(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	plain, err := unprotect(raw)
	if err != nil {
		return "", errors.New("gagal membuka kredensial tersimpan (dibuat oleh akun atau perangkat lain?)")
	}
	return string(plain), nil
}
