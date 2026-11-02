package services

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"termimul/internal/sftpx"
	"termimul/internal/terminal"
)

// TerminalService runs shell sessions, one per terminal pane. Output is pushed to the frontend as events:
// "ssh:data:<id>" (base64 bytes) and "ssh:closed:<id>" (true if the user logged out).
type TerminalService struct {
	*env
	sessions *terminal.Manager

	mu    sync.Mutex
	conns map[string]context.CancelFunc // per session: ends its dial and keep-alive
}

// Connect opens a shell for hostID. The frontend picks sessionID and subscribes
// to the events before calling, so no output is missed.
func (s *TerminalService) Connect(sessionID, hostID string, cols, rows int) error {
	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.conns[sessionID] = cancel
	s.mu.Unlock()

	client, err := s.dialer.Dial(ctx, hostID)
	if err != nil {
		s.release(sessionID)
		return err
	}
	err = s.sessions.Start(sessionID, client, cols, rows, terminal.Handlers{
		// Base64 keeps multi-byte UTF-8 sequences split across reads intact;
		// xterm.js reassembles the bytes.
		OnData: func(b []byte) {
			runtime.EventsEmit(s.ctx, "ssh:data:"+sessionID, base64.StdEncoding.EncodeToString(b))
		},
		OnClose: func(loggedOut bool) {
			s.release(sessionID)
			runtime.EventsEmit(s.ctx, "ssh:closed:"+sessionID, loggedOut)
		},
	})
	if err != nil {
		s.release(sessionID)
		return err
	}
	// Disconnected while connecting: it found no session to close, so close it here.
	if ctx.Err() != nil {
		s.sessions.Close(sessionID)
	}
	return nil
}

func (s *TerminalService) Write(sessionID, data string) error {
	return s.sessions.Write(sessionID, []byte(data))
}

func (s *TerminalService) Resize(sessionID string, cols, rows int) error {
	return s.sessions.Resize(sessionID, cols, rows)
}

func (s *TerminalService) Disconnect(sessionID string) {
	s.release(sessionID)
	s.sessions.Close(sessionID)
}

func (s *TerminalService) release(sessionID string) {
	s.mu.Lock()
	cancel := s.conns[sessionID]
	delete(s.conns, sessionID)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// LocalSource is the source name for "this computer" in a file-browser pane.
const LocalSource = "local"

// SFTPService runs the dual-pane file browser. Each pane has its own ID and
// shows either this computer (source "local") or a saved host. Transfer
// progress is pushed as "sftp:progress:<progressID>" events carrying an
// sftpx.Progress; the frontend uses the tab ID so both panes share one bar.
type SFTPService struct {
	*env
	sessions *sftpx.Manager
}

// Open shows source (LocalSource or a host ID) in pane paneID and returns the
// starting directory.
func (s *SFTPService) Open(paneID, source string) (string, error) {
	if source == LocalSource {
		return s.sessions.OpenLocal(paneID), nil
	}
	client, err := s.dialer.Dial(s.ctx, source)
	if err != nil {
		return "", err
	}
	return s.sessions.Open(paneID, source, client)
}

func (s *SFTPService) Close(paneID string) {
	s.sessions.Close(paneID)
}

func (s *SFTPService) List(paneID, dir string) (sftpx.Listing, error) {
	return s.sessions.List(paneID, dir)
}

func (s *SFTPService) MakeDir(paneID, dir string) error {
	return s.sessions.Mkdir(paneID, dir)
}

func (s *SFTPService) Rename(paneID, from, to string) error {
	return s.sessions.Rename(paneID, from, to)
}

func (s *SFTPService) Remove(paneID, path string) error {
	return s.sessions.Remove(paneID, path)
}

// Copy copies paths from one pane into dstDir of another, including directly
// between two servers. Returns the number of files copied.
func (s *SFTPService) Copy(progressID, srcPane string, paths []string, dstPane, dstDir string) (int, error) {
	return s.sessions.Copy(srcPane, paths, dstPane, dstDir, s.progress(progressID))
}

// UploadDialog asks for files on this computer and copies them into dir of the pane.
func (s *SFTPService) UploadDialog(progressID, paneID, dir string) (int, error) {
	paths, err := runtime.OpenMultipleFilesDialog(s.ctx, runtime.OpenDialogOptions{Title: "Pilih file untuk di-upload"})
	if err != nil || len(paths) == 0 {
		return 0, err
	}
	return s.UploadPaths(progressID, paneID, dir, paths)
}

// UploadPaths copies files and folders from this computer, e.g. ones dropped
// from the file manager, into dir of the pane.
func (s *SFTPService) UploadPaths(progressID, paneID, dir string, localPaths []string) (int, error) {
	return s.sessions.Upload(paneID, dir, localPaths, s.progress(progressID))
}

// Download asks where to save (a file name, or a parent folder for folders)
// and returns the saved path, or "" if the user cancelled.
func (s *SFTPService) Download(progressID, paneID string, entry sftpx.FileEntry) (string, error) {
	var target string
	if entry.IsDir {
		parent, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{Title: "Simpan folder ke…"})
		if err != nil || parent == "" {
			return "", err
		}
		target = filepath.Join(parent, entry.Name)
	} else {
		var err error
		target, err = runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{Title: "Simpan file", DefaultFilename: entry.Name})
		if err != nil || target == "" {
			return "", err
		}
	}
	return target, s.sessions.Download(paneID, entry, target, s.progress(progressID))
}

func (s *SFTPService) progress(progressID string) sftpx.ProgressFunc {
	return func(p sftpx.Progress) {
		runtime.EventsEmit(s.ctx, "sftp:progress:"+progressID, p)
	}
}
