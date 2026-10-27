package sftpx

import (
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// FS is a filesystem a pane can browse: this computer or a server over SFTP.
// Copies work between any two, including server to server.
type FS interface {
	Home() string
	ReadDir(dir string) ([]os.FileInfo, error) // entries are not followed through symlinks
	Stat(p string) (os.FileInfo, error)
	Lstat(p string) (os.FileInfo, error)
	Mkdir(p string) error
	MkdirAll(p string) error
	Rename(from, to string) error
	Remove(p string) error
	RemoveAll(p string) error
	Open(p string) (io.ReadCloser, error)
	Create(p string) (io.WriteCloser, error)
	Join(dir, name string) string
	Base(p string) string
	Dir(p string) string
	Close() error
}

// LocalFS is this computer's filesystem.
type LocalFS struct{}

func (LocalFS) Home() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return string(filepath.Separator)
}

func (LocalFS) ReadDir(dir string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		if fi, err := e.Info(); err == nil {
			out = append(out, fi)
		}
	}
	return out, nil
}

func (LocalFS) Stat(p string) (os.FileInfo, error)      { return os.Stat(p) }
func (LocalFS) Lstat(p string) (os.FileInfo, error)     { return os.Lstat(p) }
func (LocalFS) Mkdir(p string) error                    { return os.Mkdir(p, 0o755) }
func (LocalFS) MkdirAll(p string) error                 { return os.MkdirAll(p, 0o755) }
func (LocalFS) Rename(from, to string) error            { return os.Rename(from, to) }
func (LocalFS) Remove(p string) error                   { return os.Remove(p) }
func (LocalFS) RemoveAll(p string) error                { return os.RemoveAll(p) }
func (LocalFS) Open(p string) (io.ReadCloser, error)    { return os.Open(p) }
func (LocalFS) Create(p string) (io.WriteCloser, error) { return os.Create(p) }
func (LocalFS) Join(dir, name string) string            { return filepath.Join(dir, name) }
func (LocalFS) Base(p string) string                    { return filepath.Base(p) }
func (LocalFS) Dir(p string) string                     { return filepath.Dir(p) }
func (LocalFS) Close() error                            { return nil }

// remoteFS is a server reached over SFTP; paths are POSIX.
type remoteFS struct {
	ssh  *ssh.Client
	sftp *sftp.Client
	home string
}

func newRemoteFS(client *ssh.Client) (*remoteFS, error) {
	sc, err := sftp.NewClient(client, sftp.UseConcurrentReads(true), sftp.UseConcurrentWrites(true))
	if err != nil {
		return nil, err
	}
	home, err := sc.Getwd()
	if err != nil || home == "" {
		home = "/"
	}
	return &remoteFS{ssh: client, sftp: sc, home: home}, nil
}

func (r *remoteFS) Home() string                              { return r.home }
func (r *remoteFS) ReadDir(dir string) ([]os.FileInfo, error) { return r.sftp.ReadDir(dir) }
func (r *remoteFS) Stat(p string) (os.FileInfo, error)        { return r.sftp.Stat(p) }
func (r *remoteFS) Lstat(p string) (os.FileInfo, error)       { return r.sftp.Lstat(p) }
func (r *remoteFS) Mkdir(p string) error                      { return r.sftp.Mkdir(p) }
func (r *remoteFS) MkdirAll(p string) error                   { return r.sftp.MkdirAll(p) }
func (r *remoteFS) Rename(from, to string) error              { return r.sftp.Rename(from, to) }
func (r *remoteFS) Remove(p string) error                     { return r.sftp.Remove(p) }
func (r *remoteFS) RemoveAll(p string) error                  { return r.sftp.RemoveAll(p) }
func (r *remoteFS) Open(p string) (io.ReadCloser, error)      { return r.sftp.Open(p) }
func (r *remoteFS) Create(p string) (io.WriteCloser, error)   { return r.sftp.Create(p) }
func (r *remoteFS) Join(dir, name string) string              { return path.Join(dir, name) }
func (r *remoteFS) Base(p string) string                      { return path.Base(p) }
func (r *remoteFS) Dir(p string) string                       { return path.Dir(p) }

func (r *remoteFS) Close() error {
	r.sftp.Close()
	return r.ssh.Close()
}
