package home

import (
	"path/filepath"
	"testing"
)

func TestDirIsDotFlmntUnderTheHomeDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if dir != filepath.Join(tmp, ".flmnt") {
		t.Fatalf("Dir = %q, want %q", dir, filepath.Join(tmp, ".flmnt"))
	}
}
