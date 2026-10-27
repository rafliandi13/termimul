package sftpx

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"termimul/internal/sshtest"
)

// localTree creates project/readme.txt and project/src/main.go (3 MiB) and
// returns the project path and the big file's content.
func localTree(t *testing.T) (string, []byte) {
	t.Helper()
	proj := filepath.Join(t.TempDir(), "project")
	os.MkdirAll(filepath.Join(proj, "src"), 0o755)
	os.WriteFile(filepath.Join(proj, "readme.txt"), []byte("hello"), 0o644)
	big := make([]byte, 3<<20)
	rand.Read(big)
	os.WriteFile(filepath.Join(proj, "src", "main.go"), big, 0o644)
	return proj, big
}

func names(l Listing) string {
	var n []string
	for _, e := range l.Entries {
		n = append(n, e.Name)
	}
	return strings.Join(n, ",")
}

func TestLocalToRemoteAndBack(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()
	if _, err := m.Open("srv", "host-a", sshtest.StartSFTPServer(t)); err != nil {
		t.Fatal(err)
	}
	m.OpenLocal("pc")
	var progress []Progress
	record := func(p Progress) { progress = append(progress, p) }

	proj, big := localTree(t)
	if err := m.Mkdir("srv", "/upload"); err != nil {
		t.Fatal(err)
	}
	n, err := m.Copy("pc", []string{proj}, "srv", "/upload", record)
	if err != nil || n != 2 {
		t.Fatalf("copy to server: n=%d err=%v", n, err)
	}
	if last := progress[len(progress)-1]; last.Done != last.Total || last.Index != 2 || last.Count != 2 {
		t.Fatalf("progress not reported to completion: %+v", last)
	}

	l, err := m.List("srv", "/upload/project")
	if err != nil {
		t.Fatal(err)
	}
	if names(l) != "src,readme.txt" || l.Parent != "/upload" {
		t.Fatalf("expected folders first: %s (parent %q)", names(l), l.Parent)
	}
	if err := m.Rename("srv", "/upload/project/readme.txt", "/upload/project/README.md"); err != nil {
		t.Fatal(err)
	}

	back := t.TempDir()
	if n, err := m.Copy("srv", []string{"/upload/project"}, "pc", back, record); err != nil || n != 2 {
		t.Fatalf("copy back: n=%d err=%v", n, err)
	}
	if got, _ := os.ReadFile(filepath.Join(back, "project", "README.md")); string(got) != "hello" {
		t.Fatalf("renamed file content %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(back, "project", "src", "main.go")); string(got) != string(big) {
		t.Fatal("large file corrupted in round trip")
	}

	if err := m.Remove("srv", "/upload/project"); err != nil {
		t.Fatal(err)
	}
	if l, _ := m.List("srv", "/upload"); len(l.Entries) != 0 {
		t.Fatalf("folder not removed: %s", names(l))
	}
	if err := m.Remove("srv", "/"); err == nil {
		t.Fatal("removing root must be refused")
	}
}

// TestServerToServer copies straight from one server to another.
func TestServerToServer(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()
	m.Open("a", "host-a", sshtest.StartSFTPServer(t))
	m.Open("b", "host-b", sshtest.StartSFTPServer(t))
	m.OpenLocal("pc")

	proj, big := localTree(t)
	m.Mkdir("a", "/data")
	if _, err := m.Copy("pc", []string{proj}, "a", "/data", func(Progress) {}); err != nil {
		t.Fatal(err)
	}

	m.Mkdir("b", "/backup")
	n, err := m.Copy("a", []string{"/data/project"}, "b", "/backup", func(Progress) {})
	if err != nil || n != 2 {
		t.Fatalf("server to server: n=%d err=%v", n, err)
	}
	out := t.TempDir()
	if _, err := m.Copy("b", []string{"/backup/project/src/main.go"}, "pc", out, func(Progress) {}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "main.go")); string(got) != string(big) {
		t.Fatal("file corrupted copying between servers")
	}
}

func TestRefusesCopyOntoItself(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()
	proj, _ := localTree(t)
	m.OpenLocal("left")
	m.OpenLocal("right") // both panes show this computer
	before, _ := os.ReadFile(filepath.Join(proj, "readme.txt"))

	if _, err := m.Copy("left", []string{filepath.Join(proj, "readme.txt")}, "right", proj, func(Progress) {}); err == nil {
		t.Fatal("copying a file onto itself must be refused")
	}
	if _, err := m.Copy("left", []string{proj}, "right", filepath.Join(proj, "src"), func(Progress) {}); err == nil {
		t.Fatal("copying a folder into itself must be refused")
	}
	if after, _ := os.ReadFile(filepath.Join(proj, "readme.txt")); string(after) != string(before) {
		t.Fatal("source file was modified")
	}

	// A different folder on the same machine is fine.
	dst := t.TempDir()
	if n, err := m.Copy("left", []string{proj}, "right", dst, func(Progress) {}); err != nil || n != 2 {
		t.Fatalf("local copy: n=%d err=%v", n, err)
	}
}

func TestUploadAndDownload(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()
	m.Open("srv", "host-a", sshtest.StartSFTPServer(t))
	proj, _ := localTree(t)
	if n, err := m.Upload("srv", "/", []string{proj}, func(Progress) {}); err != nil || n != 2 {
		t.Fatalf("upload: n=%d err=%v", n, err)
	}
	single := filepath.Join(t.TempDir(), "renamed.txt")
	if err := m.Download("srv", FileEntry{Name: "readme.txt", Path: "/project/readme.txt"}, single, func(Progress) {}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(single); string(got) != "hello" {
		t.Fatalf("download %q", got)
	}
}

func TestClosedSession(t *testing.T) {
	m := NewManager()
	if _, err := m.List("nope", "/"); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}
