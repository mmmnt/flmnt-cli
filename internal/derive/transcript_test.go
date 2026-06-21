package derive

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleJSONL = `{"type":"user","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","sessionId":"s1","cwd":"/repo","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"do a thing"}]}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","timestamp":"2026-01-01T00:00:01Z","sessionId":"s1","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}
{"type":"user","uuid":"t1","parentUuid":"a1","timestamp":"2026-01-01T00:00:02Z","sessionId":"s1","isMeta":true,"toolUseResult":{"exitCode":1},"message":{"role":"user","content":[{"type":"tool_result","is_error":true}]}}
{"type":"attachment","uuid":"e1","timestamp":"2026-01-01T00:00:03Z","sessionId":"s1","attachment":{"type":"edited_text_file","filename":"/repo/x.go"}}
{"type":"pr-link","uuid":"p1","timestamp":"2026-01-01T00:00:04Z","sessionId":"s1","prNumber":7,"prUrl":"https://example/pr/7"}

{"this is":"a malformed line for the type field only, still valid json but unknown shape"}
`

func writeSample(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "s1.jsonl")
	if err := os.WriteFile(path, []byte(sampleJSONL), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	return path
}

func TestParseSessionTolerant(t *testing.T) {
	recs, err := ParseSession(writeSample(t))
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	// 5 typed records + 1 unknown-shape line (still valid JSON) = 6; blank line skipped.
	if len(recs) != 6 {
		t.Fatalf("want 6 records, got %d", len(recs))
	}
	if recs[1].ParsedMessage().StopReason != "tool_use" {
		t.Errorf("assistant stop_reason not parsed: %q", recs[1].ParsedMessage().StopReason)
	}
}

func TestSummarizeSignals(t *testing.T) {
	recs, _ := ParseSession(writeSample(t))
	s := Summarize("s1.jsonl", recs)

	if s.SessionID != "s1" {
		t.Errorf("SessionID = %q, want s1", s.SessionID)
	}
	if s.Cwd != "/repo" {
		t.Errorf("Cwd = %q, want /repo", s.Cwd)
	}
	if s.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want main", s.GitBranch)
	}
	if s.ToolErrors != 1 {
		t.Errorf("ToolErrors = %d, want 1", s.ToolErrors)
	}
	if s.FileEdits != 1 {
		t.Errorf("FileEdits = %d, want 1", s.FileEdits)
	}
	if s.PRLinks != 1 {
		t.Errorf("PRLinks = %d, want 1", s.PRLinks)
	}
	// Two "user" records, but one is isMeta (the tool_result) → only 1 real user message.
	if s.UserMsgs != 1 {
		t.Errorf("UserMsgs = %d, want 1 (isMeta excluded)", s.UserMsgs)
	}
	if s.FirstTs != "2026-01-01T00:00:00Z" || s.LastTs != "2026-01-01T00:00:04Z" {
		t.Errorf("time bounds wrong: first=%q last=%q", s.FirstTs, s.LastTs)
	}
}
