// Package home resolves the CLI's per-user directory, ~/.flmnt, where every file the CLI keeps for a
// person lives. The directory was ~/.filament before the product was flmnt; Dir moves whatever is
// still there into ~/.flmnt, never over a file ~/.flmnt already holds, and removes it once empty.
package home

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const legacyName = ".filament"

// Dir is ~/.flmnt.
func Dir() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(h, ".flmnt")
	if err := moveInto(filepath.Join(h, legacyName), dir); err != nil {
		return "", err
	}
	return dir, nil
}

func moveInto(from, to string) error {
	entries, err := os.ReadDir(from)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o700); err != nil {
		return err
	}
	for _, e := range entries {
		src, dst := filepath.Join(from, e.Name()), filepath.Join(to, e.Name())
		info, err := os.Stat(dst)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := os.Rename(src, dst); err != nil {
				return err
			}
		case err != nil:
			return err
		case e.IsDir() && info.IsDir():
			if err := moveInto(src, dst); err != nil {
				return err
			}
		}
	}
	if left, err := os.ReadDir(from); err == nil && len(left) == 0 {
		return os.Remove(from)
	}
	return nil
}
