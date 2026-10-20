package sync

import (
	"path/filepath"
	"reflect"
	"testing"
)

/*
A pull that keeps the local copy marks it (QRM-ADR-016 ruling 7); the next push replaces the cloud copy

	and clears the mark. Marks live beside the cursors, keyed the same way, and survive a reload.
*/
func TestMarksAreAddedOnceClearedWhenReplacedAndReloaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sync-marks.json")
	m, err := LoadMarks(path)
	if err != nil {
		t.Fatal(err)
	}
	m.Add("http://localhost:8000|quorum", "domain", []string{"e-2", "e-1", "e-2"})
	m.Add("http://localhost:8000|quorum", "plan", []string{"p-1"})
	if got := m.For("http://localhost:8000|quorum", "domain"); !reflect.DeepEqual(got, []string{"e-1", "e-2"}) {
		t.Fatalf("got %v", got)
	}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := LoadMarks(path)
	if err != nil {
		t.Fatal(err)
	}
	again.Clear("http://localhost:8000|quorum", "domain", []string{"e-1", "e-2"})
	if got := again.For("http://localhost:8000|quorum", "domain"); len(got) != 0 {
		t.Fatalf("cleared marks must be gone, got %v", got)
	}
	if got := again.For("http://localhost:8000|quorum", "plan"); !reflect.DeepEqual(got, []string{"p-1"}) {
		t.Fatalf("other streams untouched, got %v", got)
	}
	if got := again.For("elsewhere|ws", "domain"); got == nil || len(got) != 0 {
		t.Fatalf("an unknown key is an empty list, got %#v", got)
	}
}
