package derive

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Cursor records which session files have been derived+written, keyed by file modtime, so a
// backfill can skip unchanged sessions. Correctness does NOT depend on it — /sync/import is
// idempotent via deterministic ids — it is purely an efficiency optimization (avoid re-deriving
// the whole 462MB corpus every run). An active/growing session re-processes (its modtime changes).
type Cursor struct {
	path    string
	Entries map[string]int64 `json:"entries"` // session file path -> last-processed modtime (unix)
}

func cursorPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".filament")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "derive-cursor.json"), nil
}

// LoadCursor reads the cursor state (empty on any error — the cursor is best-effort).
func LoadCursor() *Cursor {
	c := &Cursor{Entries: map[string]int64{}}
	p, err := cursorPath()
	if err != nil {
		return c
	}
	c.path = p
	if b, rerr := os.ReadFile(p); rerr == nil {
		_ = json.Unmarshal(b, c)
	}
	if c.Entries == nil {
		c.Entries = map[string]int64{}
	}
	return c
}

// Done reports whether sessionFile has already been processed at its current modtime.
func (c *Cursor) Done(sessionFile string) bool {
	fi, err := os.Stat(sessionFile)
	if err != nil {
		return false
	}
	return c.Entries[sessionFile] == fi.ModTime().Unix()
}

// Mark records sessionFile as processed at its current modtime.
func (c *Cursor) Mark(sessionFile string) {
	if fi, err := os.Stat(sessionFile); err == nil {
		c.Entries[sessionFile] = fi.ModTime().Unix()
	}
}

// Save persists the cursor (best-effort).
func (c *Cursor) Save() error {
	if c.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, b, 0o644)
}
