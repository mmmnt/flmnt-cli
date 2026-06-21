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
			// User messages are a DEMOTED decision source (commits + plans are primary). Only clearly
			// directive messages count; questions, observations, corrections, and injected meta are not
			// decisions. This keeps the high-precision signal the brief surfaces.
			txt := r.UserText()
			if len(txt) < decisionMinLen || isMeta(txt) || strings.HasPrefix(strings.TrimSpace(txt), "/") {
				continue
			}
			if isCorrection(txt) {
				d.Candidates = append(d.Candidates, userCandidate(sid, sum.GitBranch, r, KindMistake, "User correction", txt))
				continue
			}
			if !isDirective(txt) {
				continue // a question or observation — not a decision
			}
			d.Candidates = append(d.Candidates, userCandidate(sid, sum.GitBranch, r, KindDecision, "Direction-setting message", txt))
		}
	}
	return d
}

// metaPrefixes mark injected/system content that is never a decision (compaction summaries,
// system reminders, interrupt notices, pasted images).
var metaPrefixes = []string{
	"this session is being continued",
	"caveat:",
	"<system-reminder>",
	"<task-notification>",
	"[system notification",
	"[request interrupted",
	"[image",
}

// metaContains marks injected/system content even when it isn't the leading text.
var metaContains = []string{"<system-reminder>", "<task-notification>", "[system notification"}

func isMeta(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	for _, p := range metaPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	for _, m := range metaContains {
		if strings.Contains(t, m) {
			return true
		}
	}
	return false
}

// isCorrection requires a correction marker as the message's PRIMARY intent (in the first 80 chars),
// not buried somewhere in a long body (which caused summaries to be miscategorized as mistakes).
func isCorrection(s string) bool {
	head := strings.ToLower(strings.TrimSpace(s))
	if len(head) > 80 {
		head = head[:80]
	}
	for _, m := range correctionMarkers {
		if strings.Contains(head, m) {
			return true
		}
	}
	return false
}

// questionStarts begin an inquiry, not a direction.
var questionStarts = []string{"why ", "what ", "how ", "when ", "who ", "where ", "does ", "do ",
	"can ", "could ", "should ", "is ", "are ", "will ", "would ", "did "}

// isDirective is true for substantive non-question messages (a direction, not an inquiry).
func isDirective(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	if strings.HasSuffix(t, "?") {
		return false
	}
	for _, q := range questionStarts {
		if strings.HasPrefix(t, q) {
			return false
		}
	}
	return true
}

func userCandidate(sid, branch string, r Record, kind Kind, title, txt string) Candidate {
	return Candidate{
		Kind:       kind,
		LocalID:    localID(sid, "user", r.UUID),
		Title:      title,
		Text:       truncate(txt, 1000),
		Timestamp:  r.Timestamp,
		Provenance: Provenance{SessionID: sid, Branch: branch, UUIDs: nz(r.UUID)},
	}
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
