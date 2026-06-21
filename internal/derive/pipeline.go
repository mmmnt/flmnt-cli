package derive

// Confidence levels on a candidate.
const (
	ConfHigh       = "high"       // factual artifact (git commit, PR, revert)
	ConfTranscript = "transcript" // transcript-only signal; the LLM judges significance in Phase 2
)

// DeriveSession runs the full deterministic skeleton for one session: nominate (transcript) →
// correlate causal refs → enrich with git commits in the session window → assign confidence.
// No LLM, no writes — this is the artifact-grounded candidate set the Phase-2 judge consumes.
func DeriveSession(repo string, recs []Record) SessionDerivation {
	d := NominateSession(repo, recs)
	Correlate(&d, recs)
	if commits, err := CommitsInWindow(repo, d.Branch, d.WindowFrom, d.WindowTo); err == nil {
		d.addGitCommits(commits)
	}
	assignConfidence(&d)
	return d
}

func (d *SessionDerivation) keyframeID() string {
	for _, c := range d.Candidates {
		if c.Kind == KindKeyframe {
			return c.LocalID
		}
	}
	return ""
}

// addGitCommits turns correlated git commits into factual candidates: commits → commit candidates,
// reverts → mistake candidates. All carry the sha + files as ground-truth provenance.
func (d *SessionDerivation) addGitCommits(commits []Commit) {
	kf := d.keyframeID()
	for _, c := range commits {
		kind, title := KindCommit, "Commit "+shortSHA(c.SHA)+": "+c.Subject
		if c.IsRevert() {
			kind, title = KindMistake, "Revert "+shortSHA(c.SHA)+": "+c.Subject
		}
		cand := Candidate{
			Kind:       kind,
			LocalID:    localID(d.SessionID, "git", c.SHA),
			Title:      truncate(title, 140),
			Text:       c.Body,
			Timestamp:  c.Time,
			Confidence: ConfHigh, // git is ground truth
			Provenance: Provenance{SessionID: d.SessionID, Branch: d.Branch, CommitSHAs: []string{c.SHA}, Files: c.Files},
		}
		if kf != "" {
			cand.CausalRefs = []string{kf}
		}
		d.Candidates = append(d.Candidates, cand)
	}
}

// assignConfidence defaults any unscored non-keyframe candidate to transcript-only confidence.
func assignConfidence(d *SessionDerivation) {
	for i := range d.Candidates {
		if d.Candidates[i].Confidence == "" && d.Candidates[i].Kind != KindKeyframe {
			d.Candidates[i].Confidence = ConfTranscript
		}
	}
}
