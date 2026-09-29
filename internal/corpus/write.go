package corpus

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Write puts every rendered document in dir, creating it when it does not exist. It returns the
// paths written, in the order the report holds them.
func Write(dir string, r Report) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	written := make([]string, 0, len(r.Files))
	for _, f := range r.Files {
		path := filepath.Join(dir, f.Name)
		if err := os.WriteFile(path, []byte(f.Markdown), 0o644); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// Stray names the markdown already in dir that this render did not produce. A generated corpus that
// leaves a hand-authored file beside it is the exact trap this command exists to remove: a reader
// grepping the directory finds the stale document first. Naming them is safe where deleting is not.
func Stray(dir string, r Report) ([]string, error) {
	generated := make(map[string]bool, len(r.Files))
	for _, f := range r.Files {
		generated[f.Name] = true
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var stray []string
	for _, item := range items {
		if item.IsDir() || !strings.HasSuffix(item.Name(), ".md") || generated[item.Name()] {
			continue
		}
		stray = append(stray, item.Name())
	}
	sort.Strings(stray)
	return stray, nil
}
