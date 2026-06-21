package derive

// Correlate fills CausalRefs on each candidate by collapsing the parentUuid chain to the
// nearest prior candidate — so edges connect decisions/mistakes/commits, not raw turns.
// Candidates with no candidate-ancestor attach to the session keyframe, yielding a connected
// DAG rooted at the recap.
func Correlate(d *SessionDerivation, recs []Record) {
	recByUUID := make(map[string]Record, len(recs))
	for _, r := range recs {
		if r.UUID != "" {
			recByUUID[r.UUID] = r
		}
	}

	candByUUID := map[string]string{} // source uuid -> candidate local id
	keyframeID := ""
	for _, c := range d.Candidates {
		if c.Kind == KindKeyframe {
			keyframeID = c.LocalID
			continue
		}
		if len(c.Provenance.UUIDs) > 0 {
			candByUUID[c.Provenance.UUIDs[0]] = c.LocalID
		}
	}

	for i := range d.Candidates {
		c := &d.Candidates[i]
		if c.Kind == KindKeyframe || len(c.CausalRefs) > 0 {
			continue
		}
		ref := ""
		if len(c.Provenance.UUIDs) > 0 {
			ref = nearestCandidateAncestor(c.Provenance.UUIDs[0], c.LocalID, recByUUID, candByUUID)
		}
		if ref == "" {
			ref = keyframeID
		}
		if ref != "" {
			c.CausalRefs = []string{ref}
		}
	}
}

// nearestCandidateAncestor walks the parentUuid chain from startUUID and returns the local id
// of the first ancestor record that is itself a candidate source (excluding selfID).
func nearestCandidateAncestor(startUUID, selfID string, recByUUID map[string]Record, candByUUID map[string]string) string {
	seen := map[string]bool{}
	cur := recByUUID[startUUID].ParentUUID
	for cur != "" && !seen[cur] {
		seen[cur] = true
		if id, ok := candByUUID[cur]; ok && id != selfID {
			return id
		}
		cur = recByUUID[cur].ParentUUID
	}
	return ""
}
