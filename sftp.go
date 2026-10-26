package main

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
	"github.com/wailsapp/wails/v2/pkg/runtime"
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

type sftpSession struct {
	ssh  *ssh.Client
	sftp *sftp.Client
	// One transfer at a time per tab keeps progress reporting simple.
	busy sync.Mutex
}

func (s *sftpSession) close() {
	s.sftp.Close()
	s.ssh.Close()
}

// OpenSFTP connects a file-browser tab and returns the remote home directory.
func (a *App) OpenSFTP(sessionID, hostID string) (string, error) {
	client, err := a.dial(hostID)
	if err != nil {
		return "", err
	}
	sc, err := sftp.NewClient(client, sftp.UseConcurrentReads(true), sftp.UseConcurrentWrites(true))
	if err != nil {
		client.Close()
		return "", fmt.Errorf("server tidak mendukung SFTP: %w", err)
	}
	home, err := sc.Getwd()
	if err != nil || home == "" {
		home = "/"
	}
	a.mu.Lock()
	a.sftps[sessionID] = &sftpSession{ssh: client, sftp: sc}
	a.mu.Unlock()
	return home, nil
}

func (a *App) CloseSFTP(sessionID string) {
	a.mu.Lock()
	s := a.sftps[sessionID]
	delete(a.sftps, sessionID)
	a.mu.Unlock()
	if s != nil {
		s.close()
	}
}

func (a *App) sftpSession(id string) (*sftpSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s := a.sftps[id]; s != nil {
		return s, nil
	}
	return nil, errors.New("sesi SFTP sudah tertutup, hubungkan ulang")
}

// ListDir returns folders first, then files, both sorted by name.
func (a *App) ListDir(sessionID, dir string) ([]FileEntry, error) {
	s, err := a.sftpSession(sessionID)
	if err != nil {
		return nil, err
	}
	infos, err := s.sftp.ReadDir(dir)
	if err != nil {
		return nil, friendlySFTPError(err)
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

func (a *App) MakeDir(sessionID, dir string) error {
	s, err := a.sftpSession(sessionID)
	if err != nil {
		return err
	}
	return friendlySFTPError(s.sftp.Mkdir(dir))
}

func (a *App) RenamePath(sessionID, from, to string) error {
	s, err := a.sftpSession(sessionID)
	if err != nil {
		return err
	}
	return friendlySFTPError(s.sftp.Rename(from, to))
}

func (a *App) RemovePath(sessionID, p string) error {
	s, err := a.sftpSession(sessionID)
	if err != nil {
		return err
	}
	if p == "/" || p == "" {
		return errors.New("tidak boleh menghapus root")
	}
	// Unlink symlinks themselves; RemoveAll would follow one into its target folder.
	if fi, err := s.sftp.Lstat(p); err == nil && (fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir()) {
		return friendlySFTPError(s.sftp.Remove(p))
	}
	return friendlySFTPError(s.sftp.RemoveAll(p))
}

// UploadDialog asks for local files and uploads them into remoteDir.
func (a *App) UploadDialog(sessionID, remoteDir string) (int, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: "Pilih file untuk di-upload"})
	if err != nil || len(paths) == 0 {
		return 0, err
	}
	return a.UploadPaths(sessionID, remoteDir, paths)
}

// UploadPaths uploads local files and folders (recursively) into remoteDir.
func (a *App) UploadPaths(sessionID, remoteDir string, localPaths []string) (int, error) {
	s, err := a.sftpSession(sessionID)
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
				if err := s.sftp.MkdirAll(remote); err != nil {
					return friendlySFTPError(err)
				}
				return nil
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
		if err := a.uploadFile(s, sessionID, j.local, j.remote, i+1, len(jobs)); err != nil {
			return i, fmt.Errorf("%s: %w", filepath.Base(j.local), err)
		}
	}
	return len(jobs), nil
}

func (a *App) uploadFile(s *sftpSession, sessionID, local, remote string, index, count int) error {
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
		return friendlySFTPError(err)
	}
	defer dst.Close()
	r := &progressReader{r: src, report: a.reporter(sessionID, Progress{Name: filepath.Base(local), Total: info.Size(), Index: index, Count: count})}
	_, err = dst.ReadFrom(r)
	r.report(r.n, true)
	return err
}

// Download saves a remote file (via a save dialog) or a whole folder (via a folder picker).
func (a *App) Download(sessionID string, entry FileEntry) (string, error) {
	s, err := a.sftpSession(sessionID)
	if err != nil {
		return "", err
	}
	if entry.IsDir {
		parent, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Simpan folder ke…"})
		if err != nil || parent == "" {
			return "", err
		}
		s.busy.Lock()
		defer s.busy.Unlock()
		return filepath.Join(parent, entry.Name), a.downloadDir(s, sessionID, entry.Path, filepath.Join(parent, entry.Name))
	}

	target, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "Simpan file", DefaultFilename: entry.Name})
	if err != nil || target == "" {
		return "", err
	}
	s.busy.Lock()
	defer s.busy.Unlock()
	return target, a.downloadFile(s, sessionID, entry.Path, target, 1, 1)
}

func (a *App) downloadDir(s *sftpSession, sessionID, remoteDir, localDir string) error {
	type job struct{ remote, local string }
	var jobs []job
	walker := s.sftp.Walk(remoteDir)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return friendlySFTPError(err)
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
		if err := a.downloadFile(s, sessionID, j.remote, j.local, i+1, len(jobs)); err != nil {
			return fmt.Errorf("%s: %w", path.Base(j.remote), err)
		}
	}
	return nil
}

func (a *App) downloadFile(s *sftpSession, sessionID, remote, local string, index, count int) error {
	src, err := s.sftp.Open(remote)
	if err != nil {
		return friendlySFTPError(err)
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return friendlySFTPError(err)
	}
	dst, err := os.Create(local)
	if err != nil {
		return err
	}
	w := &progressWriter{w: dst, report: a.reporter(sessionID, Progress{Name: path.Base(remote), Total: info.Size(), Index: index, Count: count})}
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

// reporter emits "sftp:progress:<id>" at most every 100ms, plus once when forced.
func (a *App) reporter(sessionID string, p Progress) func(done int64, force bool) {
	var last time.Time
	return func(done int64, force bool) {
		if !force && time.Since(last) < 100*time.Millisecond {
			return
		}
		last = time.Now()
		p.Done = done
		a.emit("sftp:progress:"+sessionID, p)
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

func friendlySFTPError(err error) error {
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
