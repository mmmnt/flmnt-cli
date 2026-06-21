package derive

import "testing"

func TestCorrelateChain(t *testing.T) {
	a := "Refactor the auth module to use the new token store and set cookies SameSite=strict to " +
		"stop the CSRF problems we keep seeing across staging deploys."
	b := "Also make sure the logout flow clears every cookie and revokes the refresh token server " +
		"side, because right now stale sessions linger far too long after sign out."

	// Causal chain: u1 (decision) -> assistant x1 -> u2 (decision).
	recs := []Record{
		{Type: "user", UUID: "u1", SessionID: "s", Message: userMsg(a)},
		{Type: "assistant", UUID: "x1", ParentUUID: "u1", SessionID: "s"},
		{Type: "user", UUID: "u2", ParentUUID: "x1", SessionID: "s", Message: userMsg(b)},
	}

	d := NominateSession("/r", recs)
	Correlate(&d, recs)

	byUUID := map[string]Candidate{}
	var keyframe string
	for _, c := range d.Candidates {
		if c.Kind == KindKeyframe {
			keyframe = c.LocalID
			continue
		}
		if len(c.Provenance.UUIDs) > 0 {
			byUUID[c.Provenance.UUIDs[0]] = c
		}
	}

	c1, c2 := byUUID["u1"], byUUID["u2"]
	if c1.LocalID == "" || c2.LocalID == "" {
		t.Fatalf("missing decision candidates (u1=%q u2=%q)", c1.LocalID, c2.LocalID)
	}
	if len(c1.CausalRefs) != 1 || c1.CausalRefs[0] != keyframe {
		t.Errorf("u1 should ref the keyframe %q, got %v", keyframe, c1.CausalRefs)
	}
	if len(c2.CausalRefs) != 1 || c2.CausalRefs[0] != c1.LocalID {
		t.Errorf("u2 should ref u1 (%s), got %v", c1.LocalID, c2.CausalRefs)
	}
}
