package services

import (
	"encoding/base64"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"termimus/internal/sftpx"
	"termimus/internal/terminal"
)

// TerminalService runs shell tabs. Output is pushed to the frontend as events:
// "ssh:data:<id>" (base64 bytes) and "ssh:closed:<id>" (true if the user logged out).
type TerminalService struct {
	*env
	sessions *terminal.Manager
}

// Connect opens a shell for hostID. The frontend picks sessionID and subscribes
// to the events before calling, so no output is missed.
func (s *TerminalService) Connect(sessionID, hostID string, cols, rows int) error {
	client, err := s.dialer.Dial(s.ctx, hostID)
	if err != nil {
		return err
	}
	return s.sessions.Start(sessionID, client, cols, rows, terminal.Handlers{
		// Base64 keeps multi-byte UTF-8 sequences split across reads intact;
		// xterm.js reassembles the bytes.
		OnData: func(b []byte) {
			runtime.EventsEmit(s.ctx, "ssh:data:"+sessionID, base64.StdEncoding.EncodeToString(b))
		},
		OnClose: func(loggedOut bool) {
			runtime.EventsEmit(s.ctx, "ssh:closed:"+sessionID, loggedOut)
		},
	})
}

func (s *TerminalService) Write(sessionID, data string) error {
	return s.sessions.Write(sessionID, []byte(data))
}

func (s *TerminalService) Resize(sessionID string, cols, rows int) error {
	return s.sessions.Resize(sessionID, cols, rows)
}

func (s *TerminalService) Disconnect(sessionID string) {
	s.sessions.Close(sessionID)
}

// SFTPService runs file-browser tabs. Transfer progress is pushed as
// "sftp:progress:<id>" events carrying an sftpx.Progress.
type SFTPService struct {
	*env
	sessions *sftpx.Manager
}

// Open connects and returns the remote home directory.
func (s *SFTPService) Open(sessionID, hostID string) (string, error) {
	client, err := s.dialer.Dial(s.ctx, hostID)
	if err != nil {
		return "", err
	}
	return s.sessions.Open(sessionID, client)
}

func (s *SFTPService) Close(sessionID string) {
	s.sessions.Close(sessionID)
}

func (s *SFTPService) List(sessionID, dir string) ([]sftpx.FileEntry, error) {
	return s.sessions.List(sessionID, dir)
}

func (s *SFTPService) MakeDir(sessionID, dir string) error {
	return s.sessions.Mkdir(sessionID, dir)
}

func (s *SFTPService) Rename(sessionID, from, to string) error {
	return s.sessions.Rename(sessionID, from, to)
}

func (s *SFTPService) Remove(sessionID, path string) error {
	return s.sessions.Remove(sessionID, path)
}

// UploadDialog asks for local files and uploads them into remoteDir.
func (s *SFTPService) UploadDialog(sessionID, remoteDir string) (int, error) {
	paths, err := runtime.OpenMultipleFilesDialog(s.ctx, runtime.OpenDialogOptions{Title: "Pilih file untuk di-upload"})
	if err != nil || len(paths) == 0 {
		return 0, err
	}
	return s.UploadPaths(sessionID, remoteDir, paths)
}

// UploadPaths uploads local files and folders, e.g. ones dropped from the file manager.
func (s *SFTPService) UploadPaths(sessionID, remoteDir string, localPaths []string) (int, error) {
	return s.sessions.Upload(sessionID, remoteDir, localPaths, s.progress(sessionID))
}

// Download asks where to save (a file name, or a parent folder for folders)
// and returns the saved path, or "" if the user cancelled.
func (s *SFTPService) Download(sessionID string, entry sftpx.FileEntry) (string, error) {
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
	return target, s.sessions.Download(sessionID, entry, target, s.progress(sessionID))
}

func (s *SFTPService) progress(sessionID string) sftpx.ProgressFunc {
	return func(p sftpx.Progress) {
		runtime.EventsEmit(s.ctx, "sftp:progress:"+sessionID, p)
	}
}
