package derive

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCursorDoneAfterMark(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "s.jsonl")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Cursor{path: filepath.Join(dir, "cursor.json"), Entries: map[string]int64{}}

	if c.Done(f) {
		t.Fatal("a never-marked file should not be Done")
	}
	c.Mark(f)
	if !c.Done(f) {
		t.Fatal("after Mark, Done should be true")
	}

	// A modtime change (e.g. an active/growing session) invalidates the cursor → reprocess.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(f, future, future); err != nil {
		t.Fatal(err)
	}
	if c.Done(f) {
		t.Error("after modtime change, Done should be false")
	}
	if err := c.Save(); err != nil {
		t.Errorf("Save: %v", err)
	}
}
