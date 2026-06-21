package derive

import (
	"fmt"
	"strings"
)

// decisionMinLen filters trivial user messages ("yes", "proceed") from decision nomination.
// Nomination is high-recall by design; the Phase-2 LLM pass judges signal and drops the rest.
const decisionMinLen = 120

// correctionMarkers flag a user message that is correcting the agent (a mistake signal).
// Kept conservative — false positives are fine (the LLM judges), false routing of decisions is not.
var correctionMarkers = []string{"revert", "undo", "rollback", "that's wrong", "thats wrong",
	"you're wrong", "incorrect", "not right", "not what i", "don't do that"}

// NominateSession applies deterministic rules to a parsed session, producing candidate
// keyframe/decisions/mistakes/commits. Phase-2 (LLM) judges + writes prose; Pass B (git)
// adds commit correlation, reverts, and grounded provenance. Nothing is written here.
func NominateSession(repo string, recs []Record) SessionDerivation {
	sum := Summarize("", recs)
	sid := sum.SessionID
	d := SessionDerivation{
		SessionID:  sid,
		Repo:       repo,
		Branch:     sum.GitBranch,
		WindowFrom: sum.FirstTs,
		WindowTo:   sum.LastTs,
	}

	// One keyframe per session (the recap unit). Prose is written by the LLM in Phase 2.
	d.Candidates = append(d.Candidates, Candidate{
		Kind:       KindKeyframe,
		LocalID:    localID(sid, "keyframe"),
		Title:      fmt.Sprintf("Session %s recap", shortID(sid)),
		Provenance: Provenance{SessionID: sid, Branch: sum.GitBranch},
	})

	for _, r := range recs {
		switch {
		case r.Type == "pr-link" && r.PRURL != "":
			d.Candidates = append(d.Candidates, Candidate{
				Kind:       KindCommit,
				LocalID:    localID(sid, "pr", r.PRURL),
				Title:      fmt.Sprintf("PR #%d", r.PRNumber),
				Timestamp:  r.Timestamp,
				Confidence: "high", // factual artifact, no LLM judgment needed
				Provenance: Provenance{SessionID: sid, Branch: sum.GitBranch, PRURL: r.PRURL, UUIDs: nz(r.UUID)},
			})
		case r.IsToolError():
			d.Candidates = append(d.Candidates, Candidate{
				Kind:       KindMistake,
				LocalID:    localID(sid, "err", r.UUID),
				Title:      "Tool error",
				Timestamp:  r.Timestamp,
				Provenance: Provenance{SessionID: sid, Branch: sum.GitBranch, UUIDs: nz(r.UUID), Files: r.EditedFiles()},
			})
		default:
			txt := r.UserText()
			if len(txt) < decisionMinLen || strings.HasPrefix(strings.TrimSpace(txt), "/") {
				continue
			}
			kind, title := KindDecision, "Direction-setting message"
			if hasCorrectionMarker(txt) {
				kind, title = KindMistake, "User correction"
			}
			d.Candidates = append(d.Candidates, Candidate{
				Kind:       kind,
				LocalID:    localID(sid, "user", r.UUID),
				Title:      title,
				Text:       truncate(txt, 1000),
				Timestamp:  r.Timestamp,
				Provenance: Provenance{SessionID: sid, Branch: sum.GitBranch, UUIDs: nz(r.UUID)},
			})
		}
	}
	return d
}

func hasCorrectionMarker(s string) bool {
	low := strings.ToLower(s)
	for _, m := range correctionMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

func localID(parts ...string) string { return strings.Join(parts, ":") }

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func nz(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
