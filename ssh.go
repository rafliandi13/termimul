package main

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type sshSession struct {
	client *ssh.Client
	sess   *ssh.Session
	stdin  io.WriteCloser
}

func (s *sshSession) close() {
	s.sess.Close()
	s.client.Close()
}

// Connect opens a shell for hostID. The frontend picks sessionID and subscribes
// to "ssh:data:<id>" / "ssh:closed:<id>" before calling, so no output is missed.
func (a *App) Connect(sessionID, hostID string, cols, rows int) error {
	host, sec, err := a.store.Get(hostID)
	if err != nil {
		return err
	}
	auth, err := authMethods(host, sec)
	if err != nil {
		return err
	}
	cfg := &ssh.ClientConfig{
		User:            host.Username,
		Auth:            auth,
		HostKeyCallback: a.verifyHostKey,
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(host.Address, strconv.Itoa(host.Port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return friendlyDialError(err)
	}

	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return err
	}
	s := &sshSession{client: client, sess: sess}
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := sess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		s.close()
		return err
	}
	if s.stdin, err = sess.StdinPipe(); err != nil {
		s.close()
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		s.close()
		return err
	}
	if err := sess.Shell(); err != nil {
		s.close()
		return err
	}

	a.mu.Lock()
	a.sessions[sessionID] = s
	a.mu.Unlock()

	go a.pump(sessionID, stdout)
	go keepAlive(client, a.ctx)
	go func() {
		err := sess.Wait()
		a.mu.Lock()
		delete(a.sessions, sessionID)
		a.mu.Unlock()
		client.Close()
		// The shell reporting an exit status (any code) means the user logged out;
		// anything else (no status, I/O error) means the connection dropped.
		var exitErr *ssh.ExitError
		loggedOut := err == nil || errors.As(err, &exitErr)
		runtime.EventsEmit(a.ctx, "ssh:closed:"+sessionID, loggedOut)
	}()
	return nil
}

// pump forwards raw terminal output. Base64 keeps multi-byte UTF-8 sequences
// that are split across reads intact; xterm.js reassembles the bytes.
func (a *App) pump(sessionID string, r io.Reader) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			runtime.EventsEmit(a.ctx, "ssh:data:"+sessionID, base64.StdEncoding.EncodeToString(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func (a *App) Write(sessionID, data string) error {
	s := a.session(sessionID)
	if s == nil {
		return nil
	}
	_, err := s.stdin.Write([]byte(data))
	return err
}

func (a *App) Resize(sessionID string, cols, rows int) error {
	s := a.session(sessionID)
	if s == nil || cols <= 0 || rows <= 0 {
		return nil
	}
	return s.sess.WindowChange(rows, cols)
}

func (a *App) Disconnect(sessionID string) {
	a.mu.Lock()
	s := a.sessions[sessionID]
	delete(a.sessions, sessionID)
	a.mu.Unlock()
	if s != nil {
		s.close()
	}
}

func (a *App) session(id string) *sshSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessions[id]
}

func authMethods(host Host, sec hostSecrets) ([]ssh.AuthMethod, error) {
	if host.AuthType == "key" {
		data, err := os.ReadFile(expandHome(host.KeyPath))
		if err != nil {
			return nil, fmt.Errorf("gagal membaca SSH key: %w", err)
		}
		var signer ssh.Signer
		if sec.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(sec.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(data)
		}
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return nil, errors.New("SSH key ini butuh passphrase; isi di pengaturan host")
		}
		if errors.Is(err, x509.IncorrectPasswordError) {
			return nil, errors.New("passphrase SSH key salah")
		}
		if err != nil {
			return nil, fmt.Errorf("SSH key tidak valid: %w", err)
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

// verifyHostKey is trust-on-first-use against our own known_hosts file:
// unknown hosts need explicit approval, changed keys are always rejected.
func (a *App) verifyHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	a.khMu.Lock()
	defer a.khMu.Unlock()

	f, err := os.OpenFile(a.knownHostsPath, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return err
	}
	f.Close()
	check, err := knownhosts.New(a.knownHostsPath)
	if err != nil {
		return err
	}
	err = check(hostname, remote, key)
	var keyErr *knownhosts.KeyError
	if err == nil || !errors.As(err, &keyErr) {
		return err
	}
	fingerprint := ssh.FingerprintSHA256(key)
	if len(keyErr.Want) > 0 {
		return fmt.Errorf("PERINGATAN: host key %s berubah (sekarang %s). Bisa jadi server di-reinstall, atau ada serangan man-in-the-middle. Koneksi dibatalkan", hostname, fingerprint)
	}

	answer, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:  runtime.QuestionDialog,
		Title: "Host baru",
		Message: fmt.Sprintf("Belum pernah terhubung ke %s.\n\nFingerprint %s:\n%s\n\nPercayai host ini dan lanjutkan?",
			hostname, key.Type(), fingerprint),
	})
	if err != nil {
		return err
	}
	if answer != "Yes" && answer != "Ya" {
		return errors.New("host key tidak dipercaya, koneksi dibatalkan")
	}
	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	kh, err := os.OpenFile(a.knownHostsPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer kh.Close()
	_, err = kh.WriteString(line + "\n")
	return err
}

func keepAlive(client *ssh.Client, ctx interface{ Done() <-chan struct{} }) {
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

func friendlyDialError(err error) error {
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

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}
