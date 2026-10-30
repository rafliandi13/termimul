// Package sshx opens authenticated SSH connections to hosts stored in the vault.
package sshx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"termimul/internal/fsutil"
	"termimul/internal/knownhosts"
	"termimul/internal/vault"
)

type Dialer struct {
	Store    *vault.Store
	HostKeys *knownhosts.DB
	Confirm  knownhosts.ConfirmFunc // asks the user to trust a new host key
}

// Dial connects to a saved host and keeps the connection alive until ctx ends
// or the connection closes.
func (d *Dialer) Dial(ctx context.Context, hostID string) (*ssh.Client, error) {
	host, sec, err := d.Store.Host(hostID)
	if err != nil {
		return nil, err
	}
	auth, err := AuthMethods(host, sec)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            host.Username,
		Auth:            auth,
		HostKeyCallback: d.HostKeys.HostKeyCallback(d.Confirm),
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(host.Address, strconv.Itoa(host.Port))
	client, err := dialContext(ctx, addr, cfg)
	if err != nil {
		return nil, FriendlyError(err)
	}
	go keepAlive(ctx, client)
	return client, nil
}

// dialContext is ssh.Dial that gives up, mid-handshake too, once ctx ends.
func dialContext(ctx context.Context, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	conn, err := (&net.Dialer{Timeout: cfg.Timeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if !stop() {
		if err == nil {
			c.Close()
		}
		return nil, ctx.Err()
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func AuthMethods(host vault.Host, sec vault.Secrets) ([]ssh.AuthMethod, error) {
	if host.AuthType == "key" {
		if sec.Signer != nil {
			return []ssh.AuthMethod{ssh.PublicKeys(sec.Signer)}, nil
		}
		data, err := os.ReadFile(fsutil.ExpandHome(host.KeyPath))
		if err != nil {
			return nil, fmt.Errorf("gagal membaca SSH key: %w", err)
		}
		priv, err := vault.ParsePrivateKey(data, sec.Passphrase)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.NewSignerFromKey(priv)
		if err != nil {
			return nil, err
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	}

	if sec.Password == "" {
		return nil, errors.New("password belum disimpan untuk host ini")
	}
	// Many servers only offer keyboard-interactive, so answer it with the same password.
	interactive := ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range answers {
			answers[i] = sec.Password
		}
		return answers, nil
	})
	return []ssh.AuthMethod{ssh.Password(sec.Password), interactive}, nil
}

func keepAlive(ctx context.Context, client *ssh.Client) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
		}
	}
}

// FriendlyError turns common dial failures into messages a user can act on.
func FriendlyError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unable to authenticate"):
		return errors.New("autentikasi gagal: username, password, atau key salah")
	case strings.Contains(msg, "i/o timeout"):
		return errors.New("koneksi timeout: server tidak merespons")
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "actively refused"):
		return errors.New("koneksi ditolak: port SSH tertutup atau service SSH tidak jalan")
	case strings.Contains(msg, "no such host"):
		return errors.New("alamat host tidak ditemukan (cek DNS / penulisan)")
	}
	return err
}
