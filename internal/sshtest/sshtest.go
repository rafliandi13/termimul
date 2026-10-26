// Package sshtest provides throwaway SSH/SFTP servers and keys for tests.
package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// WriteKey writes a new ed25519 private key (encrypted if passphrase != "")
// to a temp file and returns its path and public key.
func WriteKey(t *testing.T, passphrase string) (string, ssh.PublicKey) {
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

// HostKey returns a fresh server host key.
func HostKey(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// StartServer runs an SSH server on localhost that only accepts the public key
// allowed and rejects every channel. It returns the listen address.
func StartServer(t *testing.T, allowed ssh.PublicKey) string {
	t.Helper()
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(allowed.Marshal()) {
				return nil, nil
			}
			return nil, ssh.ErrNoAuth
		},
	}
	cfg.AddHostKey(HostKey(t))
	return serve(t, cfg, func(nc ssh.NewChannel) { nc.Reject(ssh.Prohibited, "test server") })
}

// Dial logs in to addr without verifying the host key.
func Dial(addr string, auth []ssh.AuthMethod) error {
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

// StartSFTPServer serves an in-memory filesystem over SFTP and returns a
// connected SSH client.
func StartSFTPServer(t *testing.T) *ssh.Client {
	t.Helper()
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(HostKey(t))
	addr := serve(t, cfg, func(nc ssh.NewChannel) {
		ch, reqs, err := nc.Accept()
		if err != nil {
			return
		}
		go func() {
			for req := range reqs {
				req.Reply(req.Type == "subsystem" && string(req.Payload[4:]) == "sftp", nil)
			}
		}()
		srv := sftp.NewRequestServer(ch, sftp.InMemHandler())
		srv.Serve()
		srv.Close()
	})
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{User: "t", HostKeyCallback: ssh.InsecureIgnoreHostKey()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func serve(t *testing.T, cfg *ssh.ServerConfig, handle func(ssh.NewChannel)) string {
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
				for nc := range chans {
					go handle(nc)
				}
			}()
		}
	}()
	return ln.Addr().String()
}
