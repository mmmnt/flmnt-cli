package harness

import (
	"os"
	"path/filepath"
	"strings"
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

func TestComposeArgsStackTheBaselineThenTheCheckoutsOverridesInOrder(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	baseline := filepath.Join(tmp, ".flmnt", "harness", "compose.yaml")
	if err := os.MkdirAll(filepath.Dir(baseline), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baseline, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pc := &setup.ProjectConfig{ComposeFiles: []string{"docker/a.yml", "docker/b.yml"}, EnvFile: ".env.local"}

	got, err := ComposeArgs("/repo", pc, []string{"up", "-d"})
	if err != nil {
		t.Fatalf("ComposeArgs: %v", err)
	}
	want := []string{"compose", "-f", baseline, "-f", "/repo/docker/a.yml", "-f", "/repo/docker/b.yml", "--env-file", "/repo/.env.local", "up", "-d"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("ComposeArgs =\n %v\nwant\n %v", got, want)
	}
}

func TestComposeArgsLeaveOutABaselineThatIsNotThere(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	got, err := ComposeArgs("/repo", &setup.ProjectConfig{ComposeFiles: []string{"compose.yml"}}, []string{"ps"})
	if err != nil || strings.Join(got, " ") != "compose -f /repo/compose.yml ps" {
		t.Fatalf("ComposeArgs = %v (%v)", got, err)
	}
}
