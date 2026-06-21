package derive

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Refine applies LLM-free cleanup to a derivation: drop obvious non-decision noise, dedup by
// content-hash (and commit sha / PR url), and synthesize a templated keyframe recap. Relevance
// ranking is NOT decided here — that's the server's embedding read path's job (write clean, rank
// at read). Runs before Correlate so causal_refs only point at surviving candidates.
func Refine(d *SessionDerivation) {
	seen := map[string]bool{}
	kept := d.Candidates[:0]
	for _, c := range d.Candidates {
		if c.Kind == KindKeyframe {
			kept = append(kept, c)
			continue
		}
		if c.Kind == KindDecision && isNoise(c.Text) {
			continue
		}
		key := dedupKey(c)
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, c)
	}
	d.Candidates = kept
	synthesizeKeyframe(d)
}

// isNoise drops user messages that are not decisions: pasted screenshots and slash-commands.
func isNoise(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return true
	}
	if strings.HasPrefix(t, "[image") || strings.Contains(t, "[image #") {
		return true
	}
	if strings.HasPrefix(t, "/") {
		return true
	}
	return false
}

// dedupKey collapses repeats: artifacts by their id (sha/PR), text candidates by normalized hash.
func dedupKey(c Candidate) string {
	if len(c.Provenance.CommitSHAs) > 0 {
		return "sha:" + c.Provenance.CommitSHAs[0]
	}
	if c.Provenance.PRURL != "" {
		return "pr:" + c.Provenance.PRURL
	}
	norm := strings.Join(strings.Fields(strings.ToLower(c.Text)), " ")
	sum := sha256.Sum256([]byte(string(c.Kind) + "|" + norm))
	return "h:" + hex.EncodeToString(sum[:8])
}

// synthesizeKeyframe writes a factual, templated recap into the session's keyframe candidate —
// counts of decisions/commits/mistakes + the PRs touched. No LLM; the body is the source data.
func synthesizeKeyframe(d *SessionDerivation) {
	var dec, mis, com int
	var prs []string
	for _, c := range d.Candidates {
		switch c.Kind {
		case KindDecision:
			dec++
		case KindMistake:
			mis++
		case KindCommit:
			com++
			if c.Provenance.PRURL != "" {
				prs = append(prs, c.Provenance.PRURL)
			}
		}
	}
	recap := fmt.Sprintf("Session %s on %s: %d decisions, %d commits, %d mistakes.",
		shortID(d.SessionID), branchOrNone(d.Branch), dec, com, mis)
	if u := uniq(prs); len(u) > 0 {
		recap += " PRs: " + strings.Join(u, ", ")
	}
	for i := range d.Candidates {
		if d.Candidates[i].Kind == KindKeyframe {
			d.Candidates[i].Text = recap
		}
	}
}

func branchOrNone(s string) string {
	if s == "" {
		return "(no branch)"
	}
	return s
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
