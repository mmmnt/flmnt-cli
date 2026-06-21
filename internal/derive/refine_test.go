package derive

import "testing"

func TestRefineDropsNoiseAndDups(t *testing.T) {
	d := SessionDerivation{
		SessionID: "s",
		Branch:    "main",
		Candidates: []Candidate{
			{Kind: KindKeyframe, LocalID: "s:keyframe"},
			{Kind: KindDecision, LocalID: "d1", Text: "[Image #7] paste the verification code into the box"},       // noise
			{Kind: KindDecision, LocalID: "d2", Text: "Use the new token store and set SameSite=strict cookies."},  // keep
			{Kind: KindDecision, LocalID: "d3", Text: "use the NEW token store  and set samesite=strict cookies."}, // dup of d2 (normalized)
			{Kind: KindCommit, LocalID: "c1", Provenance: Provenance{CommitSHAs: []string{"abc"}}},
			{Kind: KindCommit, LocalID: "c2", Provenance: Provenance{CommitSHAs: []string{"abc"}}}, // dup sha
			{Kind: KindCommit, LocalID: "p1", Provenance: Provenance{PRURL: "https://x/pr/1"}},
			{Kind: KindCommit, LocalID: "p2", Provenance: Provenance{PRURL: "https://x/pr/1"}}, // dup PR
		},
	}
	Refine(&d)
	c := d.Counts()

	if c[KindDecision] != 1 {
		t.Errorf("decisions=%d want 1 (noise dropped, dup collapsed)", c[KindDecision])
	}
	if c[KindCommit] != 2 {
		t.Errorf("commits=%d want 2 (sha dup + PR dup collapsed)", c[KindCommit])
	}
	if c[KindKeyframe] != 1 {
		t.Errorf("keyframe=%d want 1", c[KindKeyframe])
	}
	// keyframe recap is synthesized factually.
	for _, cand := range d.Candidates {
		if cand.Kind == KindKeyframe && cand.Text == "" {
			t.Error("keyframe recap not synthesized")
		}
	}
}
