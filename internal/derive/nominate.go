package derive

import (
	"fmt"
	"strings"
)

// correctionMarkers flag a user message that is correcting the agent (a mistake signal).
// Deliberately few: nothing downstream reviews these, so a marker that over-matches files a
// mistake against the founder that they never made. A missed one is still captured, as a prompt.
var correctionMarkers = []string{"revert", "undo", "rollback", "that's wrong", "thats wrong",
	"you're wrong", "incorrect", "not right", "not what i", "don't do that"}

// NominateSession applies deterministic rules to a parsed session, producing candidate
// keyframe/prompts/mistakes/commits. Pass B (git) adds commit correlation, reverts, and grounded
// provenance. Nothing is written here — Refine and Correlate run next, then the writer.
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

	// One keyframe per session (the recap unit). Refine fills its text with a templated recap.
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
				Confidence: "high", // a commit is a fact, not a judgement
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
			// User messages are CAPTURED, never judged. A prompt is what the human said — the data
			// source a decision is replayed from, which in a regulated setting is the evidence of how
			// an actor arrived at one. Only injected meta and slash-commands are dropped, because
			// neither is the human speaking; nothing here rules on whether what they said mattered.
			// Ruled cc6a9636: classification is deterministic or it does not happen, and no
			// deterministic rule for direction-versus-inquiry has been settled. A title that guessed
			// was worse than none — "Direction-setting message" was stamped on 16 of 72 real captures
			// that contained a question mark.
			txt := r.UserText()
			// Nothing said is nothing to capture. The byte floor was doing this job by accident: an
			// assistant turn and a tool result both reach here carrying no user text, and only the
			// 120-byte test kept them out of the record.
			if strings.TrimSpace(txt) == "" || isMeta(txt) || strings.HasPrefix(strings.TrimSpace(txt), "/") {
				continue
			}
			if isCorrection(txt) {
				d.Candidates = append(d.Candidates, userCandidate(sid, sum.GitBranch, r, KindMistake, "User correction", txt))
				continue
			}
			d.Candidates = append(d.Candidates, userCandidate(sid, sum.GitBranch, r, KindPrompt, "User message", txt))
		}
	}
	return d
}

// metaPrefixes mark injected/system content that is never the human directing (compaction summaries,
// system reminders, interrupt notices, pasted images).
var metaPrefixes = []string{
	"this session is being continued",
	"caveat:",
	"<system-reminder>",
	"<task-notification>",
	"<command-name>",
	"<command-message>",
	"<local-command-stdout>",
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
