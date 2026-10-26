// Package backup reads and writes password-protected export files (.termimus).
//
// The bundle is JSON, encrypted with AES-256-GCM under a key derived from the
// export password with Argon2id. Nothing in the file is readable without the
// password, so it is safe to move between machines and operating systems.
package backup

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/argon2"

	"termimus/internal/secret"
	"termimus/internal/vault"
)

const (
	format  = "termimus-backup"
	version = 1

	// MinPasswordLength guards against trivially guessable export passwords.
	MinPasswordLength = 8

	// Argon2id cost for new exports (RFC 9106 second recommended option).
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
	// Upper bounds accepted on import, so a crafted file can't exhaust memory or CPU.
	maxArgonTime   = 10
	maxArgonMemory = 1024 * 1024 // KiB = 1 GiB
)

type Bundle struct {
	ExportedAt int64          `json:"exportedAt"`
	Vault      vault.Snapshot `json:"vault"`
	KnownHosts []string       `json:"knownHosts"`
}

type envelope struct {
	Format  string    `json:"format"`
	Version int       `json:"version"`
	KDF     kdfParams `json:"kdf"`
	Data    []byte    `json:"data"` // nonce || ciphertext, base64 in JSON
}

type kdfParams struct {
	Name    string `json:"name"`
	Salt    []byte `json:"salt"`
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
}

func Encode(b Bundle, password string) ([]byte, error) {
	if len([]rune(password)) < MinPasswordLength {
		return nil, fmt.Errorf("password export minimal %d karakter", MinPasswordLength)
	}
	if b.ExportedAt == 0 {
		b.ExportedAt = time.Now().Unix()
	}
	plain, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	kdf := kdfParams{Name: "argon2id", Salt: make([]byte, 16), Time: argonTime, Memory: argonMemory, Threads: argonThreads}
	if _, err := rand.Read(kdf.Salt); err != nil {
		return nil, err
	}
	sealed, err := secret.SealWithKey(deriveKey(password, kdf), plain)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(envelope{Format: format, Version: version, KDF: kdf, Data: sealed}, "", "  ")
}

func Decode(data []byte, password string) (Bundle, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil || env.Format != format {
		return Bundle{}, errors.New("bukan file backup Termimus")
	}
	if env.Version > version {
		return Bundle{}, errors.New("file backup dibuat oleh versi Termimus yang lebih baru; update aplikasi dulu")
	}
	k := env.KDF
	if k.Name != "argon2id" || len(k.Salt) < 16 || k.Time == 0 || k.Time > maxArgonTime ||
		k.Memory == 0 || k.Memory > maxArgonMemory || k.Threads == 0 {
		return Bundle{}, errors.New("parameter enkripsi file backup tidak valid")
	}
	plain, err := secret.OpenWithKey(deriveKey(password, k), env.Data)
	if err != nil {
		return Bundle{}, errors.New("password salah atau file backup rusak")
	}
	var b Bundle
	if err := json.Unmarshal(plain, &b); err != nil {
		return Bundle{}, errors.New("isi file backup rusak")
	}
	return b, nil
}

func deriveKey(password string, k kdfParams) []byte {
	return argon2.IDKey([]byte(password), k.Salt, k.Time, k.Memory, k.Threads, 32)
}
