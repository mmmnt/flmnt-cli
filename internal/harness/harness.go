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
