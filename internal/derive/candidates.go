package derive

// Kind classifies a derived candidate.
type Kind string

const (
	KindKeyframe Kind = "keyframe"
	KindDecision Kind = "decision"
	KindMistake  Kind = "mistake"
	KindCommit   Kind = "commit"
)

// Provenance traces a candidate back to its evidence (transcript uuids + git artifacts).
type Provenance struct {
	SessionID  string   `json:"session_id"`
	Branch     string   `json:"branch,omitempty"`
	UUIDs      []string `json:"uuids,omitempty"`
	Files      []string `json:"files,omitempty"`
	CommitSHAs []string `json:"commit_shas,omitempty"`
	PRURL      string   `json:"pr_url,omitempty"`
}

// Candidate is one nominated unit of reasoning memory — pre-LLM-judgment, pre-write.
type Candidate struct {
	Kind       Kind       `json:"kind"`
	LocalID    string     `json:"local_id"`              // stable within a run; basis for causal_refs
	Title      string     `json:"title"`                 // rule-derived; the LLM refines in Phase 2
	Text       string     `json:"text,omitempty"`        // raw source text (user msg / commit body)
	Timestamp  string     `json:"timestamp,omitempty"`
	CausalRefs []string   `json:"causal_refs,omitempty"` // local_ids of predecessors (filled in correlate)
	Confidence string     `json:"confidence,omitempty"`  // filled in provenance phase
	Provenance Provenance `json:"provenance"`
}

// SessionDerivation is the candidate set produced from one main session.
type SessionDerivation struct {
	SessionID  string      `json:"session_id"`
	Repo       string      `json:"repo"`
	Branch     string      `json:"branch,omitempty"`
	WindowFrom string      `json:"window_from,omitempty"`
	WindowTo   string      `json:"window_to,omitempty"`
	Candidates []Candidate `json:"candidates"`
}

// Counts tallies candidates by kind.
func (d SessionDerivation) Counts() map[Kind]int {
	m := map[Kind]int{}
	for _, c := range d.Candidates {
		m[c.Kind]++
	}
	return m
}
