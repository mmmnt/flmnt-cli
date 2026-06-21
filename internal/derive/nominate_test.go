package derive

import (
	"encoding/json"
	"testing"
)

func userMsg(text string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": []map[string]any{{"type": "text", "text": text}},
	})
	return b
}

func iptr(i int) *int { return &i }

func TestNominateSession(t *testing.T) {
	long := "Please refactor the authentication module to use the new token store and set the " +
		"session cookies with SameSite=strict, because we keep hitting CSRF issues in staging."
	corr := "no, revert that — the middleware approach was correct; we should not have changed the " +
		"handler signature at all, please undo it and restore the prior behavior entirely."

	recs := []Record{
		{Type: "user", UUID: "u1", SessionID: "s9", GitBranch: "feat", Timestamp: "t0", Message: userMsg(long)},
		{Type: "user", UUID: "t1", SessionID: "s9", Timestamp: "t1", ToolResult: &ToolUseResult{ExitCode: iptr(1)}},
		{Type: "pr-link", UUID: "p1", SessionID: "s9", Timestamp: "t2", PRNumber: 12, PRURL: "https://x/pr/12"},
		{Type: "user", UUID: "u2", SessionID: "s9", Timestamp: "t3", Message: userMsg(corr)},
		{Type: "user", UUID: "u3", SessionID: "s9", Timestamp: "t4", IsMeta: true, Message: userMsg(long)},
		{Type: "user", UUID: "u4", SessionID: "s9", Timestamp: "t5", Message: userMsg("yes, proceed")}, // too short
	}

	d := NominateSession("/repo", recs)
	c := d.Counts()

	if d.SessionID != "s9" {
		t.Fatalf("SessionID=%q want s9", d.SessionID)
	}
	if c[KindKeyframe] != 1 {
		t.Errorf("keyframe=%d want 1", c[KindKeyframe])
	}
	if c[KindCommit] != 1 {
		t.Errorf("commit=%d want 1", c[KindCommit])
	}
	if c[KindDecision] != 1 {
		t.Errorf("decision=%d want 1 (one long non-meta direction message; short + meta excluded)", c[KindDecision])
	}
	if c[KindMistake] != 2 {
		t.Errorf("mistake=%d want 2 (tool error + user correction)", c[KindMistake])
	}

	// The commit candidate is factual → high confidence, carries the PR url in provenance.
	var sawPR bool
	for _, cand := range d.Candidates {
		if cand.Kind == KindCommit {
			sawPR = cand.Confidence == "high" && cand.Provenance.PRURL == "https://x/pr/12"
		}
	}
	if !sawPR {
		t.Error("commit candidate missing high confidence or PR provenance")
	}
}
