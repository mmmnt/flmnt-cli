package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCreatesTheDirectoryAndEveryDocument(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "training")

	if _, err := Write(dir, Report{Files: []File{{Name: "the-methodology.md", Markdown: "# THE METHODOLOGY\n"}}}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "the-methodology.md"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "# THE METHODOLOGY\n" {
		t.Errorf("want the rendered markdown on disk, got %q", got)
	}
}

func TestStrayNamesMarkdownInTheDirectoryTheRenderDidNotProduce(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"the-methodology.md", "architecture.md", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Stray(dir, Report{Files: []File{{Name: "the-methodology.md"}}})
	if err != nil {
		t.Fatalf("Stray: %v", err)
	}
	if len(got) != 1 || got[0] != "architecture.md" {
		t.Errorf("want only the ungenerated markdown named, got %v", got)
	}
}
