package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// startSFTPServer serves an in-memory filesystem over SFTP and returns a connected client.
func startSFTPServer(t *testing.T) (*ssh.Client, *sftp.Client) {
	t.Helper()
	_, hostPriv, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(hostPriv)
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		_, chans, reqs, err := ssh.NewServerConn(c, cfg)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(reqs)
		for nc := range chans {
			ch, chReqs, _ := nc.Accept()
			go func() {
				for req := range chReqs {
					req.Reply(req.Type == "subsystem" && string(req.Payload[4:]) == "sftp", nil)
				}
			}()
			go func() {
				srv := sftp.NewRequestServer(ch, sftp.InMemHandler())
				srv.Serve()
				srv.Close()
			}()
		}
	}()

	client, err := ssh.Dial("tcp", ln.Addr().String(), &ssh.ClientConfig{User: "t", HostKeyCallback: ssh.InsecureIgnoreHostKey()})
	if err != nil {
		t.Fatal(err)
	}
	sc, err := sftp.NewClient(client)
	if err != nil {
		t.Fatal(err)
	}
	return client, sc
}

func TestSFTPRoundTrip(t *testing.T) {
	client, sc := startSFTPServer(t)
	app, _ := NewApp(t.TempDir())
	var progress []Progress
	app.emit = func(_ string, data any) { progress = append(progress, data.(Progress)) }
	app.sftps["tab"] = &sftpSession{ssh: client, sftp: sc}
	defer app.CloseSFTP("tab")

	// Local tree: project/readme.txt, project/src/main.go
	local := t.TempDir()
	proj := filepath.Join(local, "project")
	os.MkdirAll(filepath.Join(proj, "src"), 0o755)
	os.WriteFile(filepath.Join(proj, "readme.txt"), []byte("hello"), 0o644)
	big := make([]byte, 3<<20)
	rand.Read(big)
	os.WriteFile(filepath.Join(proj, "src", "main.go"), big, 0o644)

	if err := app.MakeDir("tab", "/upload"); err != nil {
		t.Fatal(err)
	}
	n, err := app.UploadPaths("tab", "/upload", []string{proj})
	if err != nil || n != 2 {
		t.Fatalf("upload: n=%d err=%v", n, err)
	}
	if len(progress) == 0 || progress[len(progress)-1].Done != progress[len(progress)-1].Total {
		t.Fatalf("progress not reported to completion: %+v", progress)
	}

	entries, err := app.ListDir("tab", "/upload/project")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || !entries[0].IsDir || entries[0].Name != "src" || entries[1].Name != "readme.txt" {
		t.Fatalf("expected folders first: %+v", entries)
	}

	if err := app.RenamePath("tab", "/upload/project/readme.txt", "/upload/project/README.md"); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "copy")
	s, _ := app.sftpSession("tab")
	if err := app.downloadDir(s, "tab", "/upload/project", out); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "README.md")); string(got) != "hello" {
		t.Fatalf("renamed file content %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "src", "main.go")); len(got) != len(big) || string(got) != string(big) {
		t.Fatal("large file corrupted in round trip")
	}

	if err := app.RemovePath("tab", "/upload/project"); err != nil {
		t.Fatal(err)
	}
	if entries, _ := app.ListDir("tab", "/upload"); len(entries) != 0 {
		t.Fatalf("folder not removed: %+v", entries)
	}
	if err := app.RemovePath("tab", "/"); err == nil {
		t.Fatal("removing root must be refused")
	}
}
