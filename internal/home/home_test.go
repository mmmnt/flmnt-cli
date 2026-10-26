package home

import (
	"errors"
	"io/fs"
	"os"
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

func TestDirMovesEverythingUnderTheFormerDirectoryIntoDotFlmnt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	legacy := filepath.Join(tmp, ".filament")
	if err := os.MkdirAll(filepath.Join(legacy, "keyring"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"config.json": "c", filepath.Join("keyring", "item"): "k"} {
		if err := os.WriteFile(filepath.Join(legacy, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	for name, want := range map[string]string{"config.json": "c", filepath.Join("keyring", "item"): "k"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q (%v), want %q", name, got, err, want)
		}
	}
	if _, err := os.Stat(legacy); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("~/.filament still present: %v", err)
	}
}

func TestDirNeverOverwritesAFileDotFlmntAlreadyHolds(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	legacy, current := filepath.Join(tmp, ".filament"), filepath.Join(tmp, ".flmnt")
	for dir, body := range map[string]string{legacy: "old", current: "new"} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Dir(); err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(current, "config.json")); string(got) != "new" {
		t.Fatalf("~/.flmnt/config.json = %q, want new", got)
	}
	if got, _ := os.ReadFile(filepath.Join(legacy, "config.json")); string(got) != "old" {
		t.Fatalf("~/.filament/config.json = %q, want it left in place", got)
	}
}
