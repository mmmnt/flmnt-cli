package sync

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// MarkStore persists the records a person chose as LOCAL TRUTH on a pull (QRM-ADR-016 ruling 7): the
// correlation ids the server reported under `kept`, keyed like cursors by the local side (MCP URL +
// workspace) then by stream suffix. The next push names them in `replace`, and a replaced mark is
// cleared. Kept beside the cursors in its own file, so the cursor file's shape never changes.
type MarkStore struct {
	path  string
	marks map[string]map[string][]string
}

// DefaultMarkPath is ~/.filament/sync-marks.json.
func DefaultMarkPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".filament", "sync-marks.json"), nil
}

// LoadMarks reads the mark file at path (an absent file is an empty store).
func LoadMarks(path string) (*MarkStore, error) {
	s := &MarkStore{path: path, marks: map[string]map[string][]string{}}
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
	if err := json.Unmarshal(raw, &s.marks); err != nil {
		return nil, err
	}
	if s.marks == nil {
		s.marks = map[string]map[string][]string{}
	}
	return s, nil
}

// For returns the marked correlation ids of one stream under a source key, sorted; never nil.
func (s *MarkStore) For(key, suffix string) []string {
	out := make([]string, 0, len(s.marks[key][suffix]))
	out = append(out, s.marks[key][suffix]...)
	sort.Strings(out)
	return out
}

// Add records the ids kept on a pull; an id already marked stays marked once.
func (s *MarkStore) Add(key, suffix string, ids []string) {
	if len(ids) == 0 {
		return
	}
	if s.marks[key] == nil {
		s.marks[key] = map[string][]string{}
	}
	have := map[string]bool{}
	for _, id := range s.marks[key][suffix] {
		have[id] = true
	}
	for _, id := range ids {
		if !have[id] {
			s.marks[key][suffix] = append(s.marks[key][suffix], id)
			have[id] = true
		}
	}
}

// Clear forgets the ids a push replaced.
func (s *MarkStore) Clear(key, suffix string, ids []string) {
	gone := map[string]bool{}
	for _, id := range ids {
		gone[id] = true
	}
	kept := s.marks[key][suffix][:0]
	for _, id := range s.marks[key][suffix] {
		if !gone[id] {
			kept = append(kept, id)
		}
	}
	if len(kept) == 0 {
		delete(s.marks[key], suffix)
		if len(s.marks[key]) == 0 {
			delete(s.marks, key)
		}
		return
	}
	s.marks[key][suffix] = kept
}

// Save writes the marks back to disk (0700 dir, 0600 file).
func (s *MarkStore) Save() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.marks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}
