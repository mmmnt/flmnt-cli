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
