// Package sftpx implements the dual-pane file browser: each pane shows this
// computer or a server over SFTP, and files can be copied between any two
// panes, including directly from one server to another.
package sftpx

import (
	"errors"
	"fmt"
	"io"
	"os"
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

// Listing is one directory as shown in a pane.
type Listing struct {
	Path    string      `json:"path"`
	Parent  string      `json:"parent"` // "" at the filesystem root
	Entries []FileEntry `json:"entries"`
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

type pane struct {
	fs  FS
	key string // which filesystem: "local" or a host ID; equal keys mean the same files
	// One transfer at a time per pane keeps progress reporting simple.
	busy sync.Mutex
}

type Manager struct {
	mu    sync.Mutex
	panes map[string]*pane
}

func NewManager() *Manager {
	return &Manager{panes: map[string]*pane{}}
}

// Open shows a server in pane id and returns its home directory. key
// identifies the server (its host ID). The Manager owns client from now on.
func (m *Manager) Open(id, key string, client *ssh.Client) (string, error) {
	fs, err := newRemoteFS(client)
	if err != nil {
		client.Close()
		return "", fmt.Errorf("server tidak mendukung SFTP: %w", err)
	}
	m.set(id, key, fs)
	return fs.Home(), nil
}

// OpenLocal shows this computer in pane id and returns the home directory.
func (m *Manager) OpenLocal(id string) string {
	m.set(id, "local", LocalFS{})
	return LocalFS{}.Home()
}

func (m *Manager) set(id, key string, fs FS) {
	m.Close(id)
	m.mu.Lock()
	m.panes[id] = &pane{fs: fs, key: key}
	m.mu.Unlock()
}

func (m *Manager) Close(id string) {
	m.mu.Lock()
	p := m.panes[id]
	delete(m.panes, id)
	m.mu.Unlock()
	if p != nil {
		p.fs.Close()
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, p := range m.panes {
		p.fs.Close()
		delete(m.panes, id)
	}
}

func (m *Manager) get(id string) (*pane, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p := m.panes[id]; p != nil {
		return p, nil
	}
	return nil, errors.New("sesi SFTP sudah tertutup, hubungkan ulang")
}

// List returns folders first, then files, both sorted by name.
func (m *Manager) List(id, dir string) (Listing, error) {
	p, err := m.get(id)
	if err != nil {
		return Listing{}, err
	}
	infos, err := p.fs.ReadDir(dir)
	if err != nil {
		return Listing{}, friendlyError(err)
	}
	out := Listing{Path: dir, Entries: make([]FileEntry, 0, len(infos))}
	if parent := p.fs.Dir(dir); parent != dir {
		out.Parent = parent
	}
	for _, fi := range infos {
		e := FileEntry{
			Name:    fi.Name(),
			Path:    p.fs.Join(dir, fi.Name()),
			Size:    fi.Size(),
			Mode:    fi.Mode().String(),
			ModTime: fi.ModTime().Unix(),
			IsDir:   fi.IsDir(),
			IsLink:  fi.Mode()&os.ModeSymlink != 0,
		}
		// Follow symlinks so linked folders can be opened.
		if e.IsLink {
			if target, err := p.fs.Stat(e.Path); err == nil {
				e.IsDir = target.IsDir()
			}
		}
		out.Entries = append(out.Entries, e)
	}
	sort.Slice(out.Entries, func(i, j int) bool {
		a, b := out.Entries[i], out.Entries[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return out, nil
}

func (m *Manager) Mkdir(id, dir string) error {
	p, err := m.get(id)
	if err != nil {
		return err
	}
	return friendlyError(p.fs.Mkdir(dir))
}

func (m *Manager) Rename(id, from, to string) error {
	p, err := m.get(id)
	if err != nil {
		return err
	}
	return friendlyError(p.fs.Rename(from, to))
}

func (m *Manager) Remove(id, target string) error {
	p, err := m.get(id)
	if err != nil {
		return err
	}
	if target == "" || p.fs.Dir(target) == target {
		return errors.New("tidak boleh menghapus root")
	}
	// Unlink symlinks themselves; RemoveAll would follow one into its target folder.
	if fi, err := p.fs.Lstat(target); err == nil && (fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir()) {
		return friendlyError(p.fs.Remove(target))
	}
	return friendlyError(p.fs.RemoveAll(target))
}

// Copy copies paths (files or folders) from pane srcID into dstDir of pane
// dstID and returns how many files were transferred. Between two servers the
// data streams through this app without touching the local disk.
func (m *Manager) Copy(srcID string, paths []string, dstID, dstDir string, onProgress ProgressFunc) (int, error) {
	src, err := m.get(srcID)
	if err != nil {
		return 0, err
	}
	dst, err := m.get(dstID)
	if err != nil {
		return 0, err
	}
	unlock := lockPanes(srcID, src, dstID, dst)
	defer unlock()

	var jobs []job
	for _, p := range paths {
		target := dst.fs.Join(dstDir, src.fs.Base(p))
		// Two panes on the same server see the same files: copying a path onto
		// itself would truncate the source while reading it.
		if src.key == dst.key && (target == p || strings.HasPrefix(target, p+"/") || strings.HasPrefix(target, p+`\`)) {
			return 0, fmt.Errorf("%s tidak bisa disalin ke dirinya sendiri", src.fs.Base(p))
		}
		js, err := plan(src.fs, p, dst.fs, target)
		if err != nil {
			return 0, err
		}
		jobs = append(jobs, js...)
	}
	return run(src.fs, dst.fs, jobs, onProgress)
}

// Upload copies local files and folders into remoteDir of pane id.
func (m *Manager) Upload(id, remoteDir string, localPaths []string, onProgress ProgressFunc) (int, error) {
	p, err := m.get(id)
	if err != nil {
		return 0, err
	}
	p.busy.Lock()
	defer p.busy.Unlock()
	var jobs []job
	for _, lp := range localPaths {
		js, err := plan(LocalFS{}, lp, p.fs, p.fs.Join(remoteDir, LocalFS{}.Base(lp)))
		if err != nil {
			return 0, err
		}
		jobs = append(jobs, js...)
	}
	return run(LocalFS{}, p.fs, jobs, onProgress)
}

// Download copies a file to localPath, or a folder (recursively) into a new folder at localPath.
func (m *Manager) Download(id string, entry FileEntry, localPath string, onProgress ProgressFunc) error {
	p, err := m.get(id)
	if err != nil {
		return err
	}
	p.busy.Lock()
	defer p.busy.Unlock()
	jobs, err := plan(p.fs, entry.Path, LocalFS{}, localPath)
	if err != nil {
		return err
	}
	_, err = run(p.fs, LocalFS{}, jobs, onProgress)
	return err
}

// lockPanes takes both panes' transfer locks in a fixed order so two opposite
// copies can't deadlock.
func lockPanes(aID string, a *pane, bID string, b *pane) func() {
	if a == b {
		a.busy.Lock()
		return a.busy.Unlock
	}
	if aID > bID {
		a, b = b, a
	}
	a.busy.Lock()
	b.busy.Lock()
	return func() { b.busy.Unlock(); a.busy.Unlock() }
}

type job struct {
	src, dst string
	dir      bool
}

// plan walks src (without following symlinks) and lists the folders to create
// and files to copy, so progress can report "file i of n".
func plan(src FS, srcPath string, dst FS, dstPath string) ([]job, error) {
	fi, err := src.Lstat(srcPath)
	if err != nil {
		return nil, friendlyError(err)
	}
	switch {
	case fi.Mode().IsRegular():
		return []job{{src: srcPath, dst: dstPath}}, nil
	case !fi.IsDir():
		return nil, nil // symlinks and special files are skipped
	}
	jobs := []job{{src: srcPath, dst: dstPath, dir: true}}
	children, err := src.ReadDir(srcPath)
	if err != nil {
		return nil, friendlyError(err)
	}
	for _, c := range children {
		js, err := plan(src, src.Join(srcPath, c.Name()), dst, dst.Join(dstPath, c.Name()))
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, js...)
	}
	return jobs, nil
}

func run(src, dst FS, jobs []job, onProgress ProgressFunc) (int, error) {
	files := 0
	for _, j := range jobs {
		if !j.dir {
			files++
		}
	}
	done := 0
	for _, j := range jobs {
		if j.dir {
			if err := dst.MkdirAll(j.dst); err != nil {
				return done, friendlyError(err)
			}
			continue
		}
		p := Progress{Name: src.Base(j.src), Index: done + 1, Count: files}
		if err := copyFile(src, j.src, dst, j.dst, p, onProgress); err != nil {
			return done, fmt.Errorf("%s: %w", p.Name, err)
		}
		done++
	}
	return done, nil
}

func copyFile(src FS, srcPath string, dst FS, dstPath string, p Progress, onProgress ProgressFunc) error {
	in, err := src.Open(srcPath)
	if err != nil {
		return friendlyError(err)
	}
	defer in.Close()
	if fi, err := src.Stat(srcPath); err == nil {
		p.Total = fi.Size()
	}
	out, err := dst.Create(dstPath)
	if err != nil {
		return friendlyError(err)
	}
	report := throttle(p, onProgress)

	// Let an SFTP file drive the copy: its WriteTo / ReadFrom keep several
	// requests in flight, which is much faster than plain io.Copy over SFTP.
	var n int64
	if rf, ok := in.(*sftp.File); ok {
		w := &progressWriter{w: out, report: report}
		_, err = rf.WriteTo(w)
		n = w.n
	} else if wf, ok := out.(*sftp.File); ok {
		r := &progressReader{r: in, report: report}
		_, err = wf.ReadFrom(r)
		n = r.n
	} else {
		w := &progressWriter{w: out, report: report}
		_, err = io.Copy(w, in)
		n = w.n
	}
	report(n, true)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		dst.Remove(dstPath) // don't leave a truncated file behind
		return friendlyError(err)
	}
	return nil
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
