// Package harness runs the local flmnt node: the baseline compose file with the checkout's override
// files stacked on it, every store's data kept on the host under the node's volumes path.
package harness

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/home"
	"github.com/mmmnt/flmnt-cli/internal/setup"
)

// VolumesPath is where every store keeps its data: the checkout's volumes_path, a leading ~ read as
// the home directory, else ~/.flmnt/volumes. One node holds every workspace, so it is the machine's.
func VolumesPath(pc *setup.ProjectConfig) (string, error) {
	if p := pc.VolumesPath; p != "" {
		if p == "~" || strings.HasPrefix(p, "~/") {
			h, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(h, strings.TrimPrefix(p, "~")), nil
		}
		return p, nil
	}
	dir, err := home.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "volumes"), nil
}

// Baseline is flmnt's own harness compose file, the release's, cached under ~/.flmnt/harness.
func Baseline() (string, error) {
	dir, err := home.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "harness", "compose.yaml"), nil
}

// ComposeArgs is the docker argument list for one compose call: the baseline when it is there, then
// each of the checkout's compose files in order, then its env file, then args.
func ComposeArgs(projectDir string, pc *setup.ProjectConfig, args []string) ([]string, error) {
	out := []string{"compose"}
	baseline, err := Baseline()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(baseline); err == nil {
		out = append(out, "-f", baseline)
	}
	for _, f := range pc.ComposeFiles {
		out = append(out, "-f", inProject(projectDir, f))
	}
	if pc.EnvFile != "" {
		out = append(out, "--env-file", inProject(projectDir, pc.EnvFile))
	}
	return append(out, args...), nil
}

func inProject(projectDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(projectDir, p)
}
