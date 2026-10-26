package vault

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Key is a keychain entry as the frontend sees it; the private half stays in the backend.
type Key struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Type        string `json:"type"` // e.g. "ED25519", "RSA 4096"
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"publicKey"` // authorized_keys line
	CreatedAt   int64  `json:"createdAt"`
}

type keyRecord struct {
	Key
	PrivateEnc string `json:"privateEnc"` // unencrypted OpenSSH PEM, sealed with secret.Seal
}

// GeneratePrivateKey creates a key; kind is "ed25519" (default), "rsa" (4096 bit) or "ecdsa" (P-256).
func GeneratePrivateKey(kind string) (any, error) {
	switch kind {
	case "ed25519", "":
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		return priv, err
	case "rsa":
		return rsa.GenerateKey(rand.Reader, 4096)
	case "ecdsa":
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	return nil, fmt.Errorf("jenis key tidak dikenal: %s", kind)
}

// ParsePrivateKey accepts any key format x/crypto/ssh understands (OpenSSH, PEM PKCS#1/#8, EC).
func ParsePrivateKey(data []byte, passphrase string) (any, error) {
	var raw any
	var err error
	if passphrase != "" {
		raw, err = ssh.ParseRawPrivateKeyWithPassphrase(data, []byte(passphrase))
	} else {
		raw, err = ssh.ParseRawPrivateKey(data)
	}
	var missing *ssh.PassphraseMissingError
	switch {
	case errors.As(err, &missing):
		return nil, errors.New("key ini terenkripsi, isi passphrase-nya")
	case errors.Is(err, x509.IncorrectPasswordError):
		return nil, errors.New("passphrase SSH key salah")
	case err != nil && strings.Contains(err.Error(), "PuTTY"):
		return nil, errors.New("format PuTTY (.ppk) belum didukung; ekspor dulu ke OpenSSH lewat PuTTYgen")
	case err != nil:
		return nil, fmt.Errorf("SSH key tidak valid: %w", err)
	}
	if p, ok := raw.(*ed25519.PrivateKey); ok {
		raw = *p
	}
	return raw, nil
}

// describeKey fills the public fields of a Key and returns the private key as OpenSSH PEM.
func describeKey(priv any, label string) (Key, []byte, error) {
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return Key{}, nil, err
	}
	block, err := ssh.MarshalPrivateKey(priv, label)
	if err != nil {
		return Key{}, nil, fmt.Errorf("jenis key ini belum didukung: %w", err)
	}
	pub := signer.PublicKey()
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	if label != "" {
		line += " " + strings.ReplaceAll(label, " ", "-")
	}
	return Key{
		Label:       label,
		Type:        keyTypeName(priv),
		Fingerprint: ssh.FingerprintSHA256(pub),
		PublicKey:   line,
	}, pem.EncodeToMemory(block), nil
}

func keyTypeName(priv any) string {
	switch k := priv.(type) {
	case ed25519.PrivateKey:
		return "ED25519"
	case *rsa.PrivateKey:
		return fmt.Sprintf("RSA %d", k.N.BitLen())
	case *ecdsa.PrivateKey:
		return fmt.Sprintf("ECDSA %d", k.Curve.Params().BitSize)
	}
	return "Unknown"
}
