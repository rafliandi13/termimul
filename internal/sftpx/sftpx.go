// Package sftpx implements the SFTP file browser: listing, file operations,
// and recursive uploads/downloads with progress reporting.
package sftpx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"modTime"`
	IsDir   bool   `json:"isDir"`
	IsLink  bool   `json:"isLink"`
}

type Progress struct {
	Name  string `json:"name"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Index int    `json:"index"` // 1-based position in the batch
	Count int    `json:"count"`
}

// ProgressFunc is called at most every 100ms per file, and once when a file finishes.
type ProgressFunc func(Progress)

type session struct {
	ssh  *ssh.Client
	sftp *sftp.Client
	// One transfer at a time per tab keeps progress reporting simple.
	busy sync.Mutex
}

func (s *session) close() {
	s.sftp.Close()
	s.ssh.Close()
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager() *Manager {
	return &Manager{sessions: map[string]*session{}}
}

// Open starts SFTP on client under id and returns the remote home directory.
// The Manager owns client from now on.
func (m *Manager) Open(id string, client *ssh.Client) (string, error) {
	sc, err := sftp.NewClient(client, sftp.UseConcurrentReads(true), sftp.UseConcurrentWrites(true))
	if err != nil {
		client.Close()
		return "", fmt.Errorf("server tidak mendukung SFTP: %w", err)
	}
	home, err := sc.Getwd()
	if err != nil || home == "" {
		home = "/"
	}
	m.mu.Lock()
	m.sessions[id] = &session{ssh: client, sftp: sc}
	m.mu.Unlock()
	return home, nil
}

func (m *Manager) Close(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s != nil {
		s.close()
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		s.close()
		delete(m.sessions, id)
	}
}

func (m *Manager) get(id string) (*session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[id]; s != nil {
		return s, nil
	}
	return nil, errors.New("sesi SFTP sudah tertutup, hubungkan ulang")
}

// List returns folders first, then files, both sorted by name.
func (m *Manager) List(id, dir string) ([]FileEntry, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	infos, err := s.sftp.ReadDir(dir)
	if err != nil {
		return nil, friendlyError(err)
	}
	out := make([]FileEntry, 0, len(infos))
	for _, fi := range infos {
		e := FileEntry{
			Name:    fi.Name(),
			Path:    path.Join(dir, fi.Name()),
			Size:    fi.Size(),
			Mode:    fi.Mode().String(),
			ModTime: fi.ModTime().Unix(),
			IsDir:   fi.IsDir(),
			IsLink:  fi.Mode()&os.ModeSymlink != 0,
		}
		// Follow symlinks so linked folders can be opened.
		if e.IsLink {
			if target, err := s.sftp.Stat(e.Path); err == nil {
				e.IsDir = target.IsDir()
			}
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (m *Manager) Mkdir(id, dir string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return friendlyError(s.sftp.Mkdir(dir))
}

func (m *Manager) Rename(id, from, to string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return friendlyError(s.sftp.Rename(from, to))
}

func (m *Manager) Remove(id, p string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	if p == "/" || p == "" {
		return errors.New("tidak boleh menghapus root")
	}
	// Unlink symlinks themselves; RemoveAll would follow one into its target folder.
	if fi, err := s.sftp.Lstat(p); err == nil && (fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir()) {
		return friendlyError(s.sftp.Remove(p))
	}
	return friendlyError(s.sftp.RemoveAll(p))
}

// Upload copies local files and folders (recursively) into remoteDir and
// returns how many files were transferred.
func (m *Manager) Upload(id, remoteDir string, localPaths []string, onProgress ProgressFunc) (int, error) {
	s, err := m.get(id)
	if err != nil {
		return 0, err
	}
	s.busy.Lock()
	defer s.busy.Unlock()

	type job struct{ local, remote string }
	var jobs []job
	for _, lp := range localPaths {
		base := filepath.Base(lp)
		err := filepath.Walk(lp, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(lp, p)
			remote := path.Join(remoteDir, base, filepath.ToSlash(rel))
			if info.IsDir() {
				return friendlyError(s.sftp.MkdirAll(remote))
			}
			if info.Mode().IsRegular() {
				jobs = append(jobs, job{p, remote})
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}

	for i, j := range jobs {
		p := Progress{Name: filepath.Base(j.local), Index: i + 1, Count: len(jobs)}
		if err := uploadFile(s, j.local, j.remote, p, onProgress); err != nil {
			return i, fmt.Errorf("%s: %w", p.Name, err)
		}
	}
	return len(jobs), nil
}

func uploadFile(s *session, local, remote string, p Progress, onProgress ProgressFunc) error {
	src, err := os.Open(local)
	if err != nil {
		return err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	dst, err := s.sftp.Create(remote)
	if err != nil {
		return friendlyError(err)
	}
	defer dst.Close()
	p.Total = info.Size()
	r := &progressReader{r: src, report: throttle(p, onProgress)}
	_, err = dst.ReadFrom(r)
	r.report(r.n, true)
	return err
}

// Download copies a remote file to localPath, or a remote folder (recursively)
// into a new folder at localPath.
func (m *Manager) Download(id string, entry FileEntry, localPath string, onProgress ProgressFunc) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.busy.Lock()
	defer s.busy.Unlock()
	if entry.IsDir {
		return downloadDir(s, entry.Path, localPath, onProgress)
	}
	return downloadFile(s, entry.Path, localPath, Progress{Name: entry.Name, Index: 1, Count: 1}, onProgress)
}

func downloadDir(s *session, remoteDir, localDir string, onProgress ProgressFunc) error {
	type job struct{ remote, local string }
	var jobs []job
	walker := s.sftp.Walk(remoteDir)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return friendlyError(err)
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(walker.Path(), remoteDir), "/")
		local := filepath.Join(localDir, filepath.FromSlash(rel))
		if walker.Stat().IsDir() {
			if err := os.MkdirAll(local, 0o755); err != nil {
				return err
			}
		} else if walker.Stat().Mode().IsRegular() {
			jobs = append(jobs, job{walker.Path(), local})
		}
	}
	for i, j := range jobs {
		p := Progress{Name: path.Base(j.remote), Index: i + 1, Count: len(jobs)}
		if err := downloadFile(s, j.remote, j.local, p, onProgress); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
	}
	return nil
}

func downloadFile(s *session, remote, local string, p Progress, onProgress ProgressFunc) error {
	src, err := s.sftp.Open(remote)
	if err != nil {
		return friendlyError(err)
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return friendlyError(err)
	}
	dst, err := os.Create(local)
	if err != nil {
		return err
	}
	p.Total = info.Size()
	w := &progressWriter{w: dst, report: throttle(p, onProgress)}
	_, err = src.WriteTo(w)
	w.report(w.n, true)
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(local) // don't leave a truncated file behind
	}
	return err
}

func throttle(p Progress, onProgress ProgressFunc) func(done int64, force bool) {
	var last time.Time
	return func(done int64, force bool) {
		if !force && time.Since(last) < 100*time.Millisecond {
			return
		}
		last = time.Now()
		p.Done = done
		onProgress(p)
	}
}

type progressReader struct {
	r      io.Reader
	n      int64
	report func(int64, bool)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.n += int64(n)
	p.report(p.n, false)
	return n, err
}

type progressWriter struct {
	w      io.Writer
	n      int64
	report func(int64, bool)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.n += int64(n)
	p.report(p.n, false)
	return n, err
}

func friendlyError(err error) error {
	if err == nil {
		return nil
	}
	var se *sftp.StatusError
	if errors.As(err, &se) {
		switch se.FxCode() {
		case sftp.ErrSSHFxPermissionDenied:
			return errors.New("akses ditolak (permission denied)")
		case sftp.ErrSSHFxNoSuchFile:
			return errors.New("file atau folder tidak ditemukan")
		case sftp.ErrSSHFxFailure:
			return errors.New("operasi gagal di server (folder tidak kosong, sudah ada, atau disk penuh?)")
		}
	}
	if errors.Is(err, os.ErrPermission) {
		return errors.New("akses ditolak (permission denied)")
	}
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("file atau folder tidak ditemukan")
	}
	return err
}
