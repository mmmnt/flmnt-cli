// Package home resolves the CLI's per-user directory, ~/.flmnt, where every file the CLI keeps for a
// person lives.
package home

import (
	"os"
	"path/filepath"
)

// Dir is ~/.flmnt.
func Dir() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".flmnt"), nil
}
