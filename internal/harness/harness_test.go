package harness

import (
	"path/filepath"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/setup"
)

func TestVolumesPathDefaultsToDotFlmntVolumes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := VolumesPath(&setup.ProjectConfig{})
	if err != nil || got != filepath.Join(tmp, ".flmnt", "volumes") {
		t.Fatalf("VolumesPath = %q (%v)", got, err)
	}
}

func TestVolumesPathReadsTheDeclarationWithALeadingTildeAsHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := VolumesPath(&setup.ProjectConfig{VolumesPath: "~/data/flmnt"})
	if err != nil || got != filepath.Join(tmp, "data", "flmnt") {
		t.Fatalf("VolumesPath = %q (%v)", got, err)
	}
}
