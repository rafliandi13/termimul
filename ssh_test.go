package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// startServer runs a throwaway SSH server on localhost that only accepts pub.
func startServer(t *testing.T, pub ssh.PublicKey) string {
	t.Helper()
	_, hostPriv, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(hostPriv)
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(pub.Marshal()) {
				return nil, nil
			}
			return nil, ssh.ErrNoAuth
		},
	}
	cfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				sc, chans, reqs, err := ssh.NewServerConn(c, cfg)
				if err != nil {
					return
				}
				defer sc.Close()
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					ch.Reject(ssh.Prohibited, "test server")
				}
			}()
		}
	}()
	return ln.Addr().String()
}

func writeKey(t *testing.T, passphrase string) (string, ssh.PublicKey) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	var block *pem.Block
	var err error
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	sshPub, _ := ssh.NewPublicKey(pub)
	return path, sshPub
}

func dial(addr string, auth []ssh.AuthMethod) error {
	c, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "tester",
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err == nil {
		c.Close()
	}
	return err
}

func TestKeyAuth(t *testing.T) {
	cases := []struct {
		name, keyPass, givenPass string
		wantAuthErr              bool
	}{
		{"plain key", "", "", false},
		{"encrypted key with passphrase", "rahasia", "rahasia", false},
		{"encrypted key without passphrase", "rahasia", "", true},
		{"encrypted key wrong passphrase", "rahasia", "salah", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, pub := writeKey(t, tc.keyPass)
			addr := startServer(t, pub)
			auth, err := authMethods(Host{AuthType: "key", KeyPath: path}, hostSecrets{Passphrase: tc.givenPass})
			if tc.wantAuthErr {
				if err == nil {
					t.Fatal("expected key loading to fail")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := dial(addr, auth); err != nil {
				t.Fatalf("login failed: %v", err)
			}
		})
	}
}

func TestKeyAuthRejectsOtherKey(t *testing.T) {
	path, _ := writeKey(t, "")
	_, otherPub := writeKey(t, "")
	addr := startServer(t, otherPub)
	auth, err := authMethods(Host{AuthType: "key", KeyPath: path}, hostSecrets{})
	if err != nil {
		t.Fatal(err)
	}
	if err := dial(addr, auth); err == nil {
		t.Fatal("server accepted a key it doesn't know")
	}
}
