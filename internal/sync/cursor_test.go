package sync

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathsLiveUnderDotFlmnt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cursors, err := DefaultCursorPath()
	if err != nil || cursors != filepath.Join(tmp, ".flmnt", "sync-cursors.json") {
		t.Fatalf("DefaultCursorPath = %q (%v)", cursors, err)
	}
	marks, err := DefaultMarkPath()
	if err != nil || marks != filepath.Join(tmp, ".flmnt", "sync-marks.json") {
		t.Fatalf("DefaultMarkPath = %q (%v)", marks, err)
	}
}
