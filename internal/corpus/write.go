package corpus

import (
	"os"
	"path/filepath"
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
