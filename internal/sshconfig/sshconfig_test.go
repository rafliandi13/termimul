package sshconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func byAlias(entries []Entry) map[string]Entry {
	m := map[string]Entry{}
	for _, e := range entries {
		m[e.Alias] = e
	}
	return m
}

func TestParseSeveralHosts(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id_work")
	write(t, filepath.Join(dir, "config"), `
# comment
Host web
    HostName 10.0.0.5
    Port 2222
    User deploy
    IdentityFile "`+key+`"

Host db db-alias
  HostName=db.internal
  User = postgres # trailing comment

Host bare
`)
	entries, err := Parse(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Alias: "web", HostName: "10.0.0.5", Port: 2222, User: "deploy", IdentityFiles: []string{key}},
		{Alias: "db", HostName: "db.internal", Port: 22, User: "postgres"},
		{Alias: "db-alias", HostName: "db.internal", Port: 22, User: "postgres"},
		{Alias: "bare", HostName: "bare", Port: 22},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("got  %+v\nwant %+v", entries, want)
	}
}

func TestWildcardsAreSkippedButApply(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config"), `
Host app
    HostName app.example.com

Host *.example.com !secret.example.com
    User shared

Host secret.example.com
    Port 2200

Host *
    User fallback
    Port 2022

Match host app
    User ignored
`)
	entries, err := Parse(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected only concrete hosts, got %+v", entries)
	}
	m := byAlias(entries)
	// Host patterns match the alias, not HostName, so *.example.com doesn't apply to "app".
	if e := m["app"]; e.User != "fallback" || e.Port != 2022 || e.HostName != "app.example.com" {
		t.Fatalf("app: %+v", e)
	}
	if e := m["secret.example.com"]; e.User != "fallback" || e.Port != 2200 {
		t.Fatalf("negated pattern should not apply: %+v", e)
	}
}

func TestFirstValueWins(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config"), `
User global
Host a
    User first
Host a
    User second
    HostName %h.lan
`)
	entries, err := Parse(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].User != "global" || entries[0].HostName != "a.lan" {
		t.Fatalf("%+v", entries)
	}
}

func TestIncludeIsFollowed(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config"), `
Include conf.d/*.conf
Include missing-file
Include `+filepath.Join(dir, "abs.conf")+`

Host main
    HostName main.example.com
`)
	write(t, filepath.Join(dir, "conf.d", "a.conf"), "Host inc-a\n  HostName 192.168.1.10\n  Include nested\n")
	write(t, filepath.Join(dir, "conf.d", "b.conf"), "Host inc-b\n  User bob\n")
	write(t, filepath.Join(dir, "nested"), "Host nested\n  Port 2200\n")
	write(t, filepath.Join(dir, "abs.conf"), "Host abs\n")

	entries, err := Parse(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	var aliases []string
	for _, e := range entries {
		aliases = append(aliases, e.Alias)
	}
	want := []string{"inc-a", "nested", "inc-b", "abs", "main"}
	if !reflect.DeepEqual(aliases, want) {
		t.Fatalf("got %v, want %v", aliases, want)
	}
	m := byAlias(entries)
	if m["inc-a"].HostName != "192.168.1.10" || m["inc-b"].User != "bob" || m["nested"].Port != 2200 {
		t.Fatalf("%+v", entries)
	}
	if m["main"].HostName != "main.example.com" {
		t.Fatalf("options after an Include must stay with the outer block: %+v", m["main"])
	}
}

func TestIncludeInsideHostKeepsBlock(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config"), "Host a\n  Include extra\n  User after\nHost b\n")
	write(t, filepath.Join(dir, "extra"), "Port 2201\nHost c\n")

	entries, err := Parse(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	m := byAlias(entries)
	if m["a"].Port != 2201 || m["a"].User != "after" {
		t.Fatalf("a: %+v", m["a"])
	}
	if m["c"].User != "" || m["b"].Port != 22 {
		t.Fatalf("settings leaked: %+v", entries)
	}
}

func TestIncludeLoopIsBounded(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config"), "Include config\n")
	if _, err := Parse(filepath.Join(dir, "config")); err == nil {
		t.Fatal("expected an error for a recursive Include")
	}
}

func TestIdentityFileExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config"), `
Host srv
    HostName srv.example.com
    User alice
    IdentityFile ~/.ssh/%r@%h
    IdentityFile none
    IdentityFile keys/second
`)
	entries, err := Parse(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(home, ".ssh", "alice@srv.example.com"),
		filepath.Join(home, "keys", "second"),
	}
	if !reflect.DeepEqual(entries[0].IdentityFiles, want) {
		t.Fatalf("got %v, want %v", entries[0].IdentityFiles, want)
	}
}

func TestUnclosedQuote(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config"), "Host a\n  IdentityFile \"oops\n")
	if _, err := Parse(filepath.Join(dir, "config")); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestMissingConfig(t *testing.T) {
	if _, err := Parse(filepath.Join(t.TempDir(), "config")); !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}
