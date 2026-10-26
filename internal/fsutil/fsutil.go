// Package fsutil holds small filesystem helpers shared by the backend packages.
package fsutil

import (
	"os"
	"path/filepath"
	"strings"
)

// WriteFileAtomic writes to a temp file and renames it into place, so a crash
// mid-write can't leave a truncated file behind.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ExpandHome replaces a leading "~" with the user's home directory.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}
