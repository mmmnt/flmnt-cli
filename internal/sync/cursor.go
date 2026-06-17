package sync

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// CursorStore persists per-source, per-stream "afterRevision" cursors so repeat
// syncs move only new events. Cursors are keyed by source (MCP URL + workspace)
// then by stream suffix. They are an optimization; the server import dedups by
// correlationId regardless, so a missing or stale cursor is always safe.
type CursorStore struct {
	path    string
	cursors map[string]map[string]int
}

func cursorKey(mcpURL, workspace string) string {
	return syncBaseURL(mcpURL) + "|" + workspace
}

// DefaultCursorPath is ~/.filament/sync-cursors.json.
func DefaultCursorPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".filament", "sync-cursors.json"), nil
}

// LoadCursors reads the cursor file at path (an absent file is an empty store).
func LoadCursors(path string) (*CursorStore, error) {
	s := &CursorStore{path: path, cursors: map[string]map[string]int{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(raw, &s.cursors); err != nil {
		return nil, err
	}
	if s.cursors == nil {
		s.cursors = map[string]map[string]int{}
	}
	return s, nil
}

// For returns the suffix->revision map for a source key (never nil).
func (s *CursorStore) For(key string) map[string]int {
	out := map[string]int{}
	for k, v := range s.cursors[key] {
		out[k] = v
	}
	return out
}

// Set records a stream's synced revision for a source key.
func (s *CursorStore) Set(key, suffix string, revision int) {
	if s.cursors[key] == nil {
		s.cursors[key] = map[string]int{}
	}
	s.cursors[key][suffix] = revision
}

// Save writes the cursors back to disk (0700 dir, 0600 file).
func (s *CursorStore) Save() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.cursors, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}
