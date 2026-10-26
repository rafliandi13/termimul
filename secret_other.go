//go:build !windows

package main

import "errors"

// TODO: macOS Keychain / libsecret. Until then, refuse rather than store plaintext.

var errNoSecretStore = errors.New("penyimpanan kredensial belum didukung di OS ini")

func protect(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	return nil, errNoSecretStore
}

func unprotect(sealed []byte) ([]byte, error) {
	if len(sealed) == 0 {
		return nil, nil
	}
	return nil, errNoSecretStore
}
