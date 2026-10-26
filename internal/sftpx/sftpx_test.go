package sftpx

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"termimus/internal/sshtest"
)

func TestRoundTrip(t *testing.T) {
	m := NewManager()
	if _, err := m.Open("tab", sshtest.StartSFTPServer(t)); err != nil {
		t.Fatal(err)
	}
	defer m.Close("tab")
	var progress []Progress
	record := func(p Progress) { progress = append(progress, p) }

	// Local tree: project/readme.txt, project/src/main.go
	proj := filepath.Join(t.TempDir(), "project")
	os.MkdirAll(filepath.Join(proj, "src"), 0o755)
	os.WriteFile(filepath.Join(proj, "readme.txt"), []byte("hello"), 0o644)
	big := make([]byte, 3<<20)
	rand.Read(big)
	os.WriteFile(filepath.Join(proj, "src", "main.go"), big, 0o644)

	if err := m.Mkdir("tab", "/upload"); err != nil {
		t.Fatal(err)
	}
	n, err := m.Upload("tab", "/upload", []string{proj}, record)
	if err != nil || n != 2 {
		t.Fatalf("upload: n=%d err=%v", n, err)
	}
	if last := progress[len(progress)-1]; last.Done != last.Total || last.Index != 2 || last.Count != 2 {
		t.Fatalf("progress not reported to completion: %+v", last)
	}

	entries, err := m.List("tab", "/upload/project")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || !entries[0].IsDir || entries[0].Name != "src" || entries[1].Name != "readme.txt" {
		t.Fatalf("expected folders first: %+v", entries)
	}

	if err := m.Rename("tab", "/upload/project/readme.txt", "/upload/project/README.md"); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "copy")
	if err := m.Download("tab", FileEntry{Name: "project", Path: "/upload/project", IsDir: true}, out, record); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "README.md")); string(got) != "hello" {
		t.Fatalf("renamed file content %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "src", "main.go")); string(got) != string(big) {
		t.Fatal("large file corrupted in round trip")
	}

	single := filepath.Join(t.TempDir(), "single.md")
	if err := m.Download("tab", FileEntry{Name: "README.md", Path: "/upload/project/README.md"}, single, record); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(single); string(got) != "hello" {
		t.Fatalf("single file download %q", got)
	}

	if err := m.Remove("tab", "/upload/project"); err != nil {
		t.Fatal(err)
	}
	if entries, _ := m.List("tab", "/upload"); len(entries) != 0 {
		t.Fatalf("folder not removed: %+v", entries)
	}
	if err := m.Remove("tab", "/"); err == nil {
		t.Fatal("removing root must be refused")
	}
}

func TestClosedSession(t *testing.T) {
	m := NewManager()
	if _, err := m.List("nope", "/"); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}
