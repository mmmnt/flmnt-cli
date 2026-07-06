package derive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

// eventEnvelope mirrors the core entry shape memoryImport accepts (full-fidelity: ids/timestamps
// preserved; payload.causal_refs drive CAUSED_BY edges).
type eventEnvelope struct {
	CorrelationID string         `json:"correlationId"`
	CausationID   string         `json:"causationId"`
	Timestamp     string         `json:"timestamp"`
	EntryType     string         `json:"entryType"`
	Payload       map[string]any `json:"payload"`
	TokenSize     int            `json:"tokenSize"`
}

type streamBatch struct {
	StreamSuffix string          `json:"streamSuffix"`
	Events       []eventEnvelope `json:"events"`
}

type importResult struct {
	StreamSuffix string `json:"streamSuffix"`
	Appended     int    `json:"appended"`
	Skipped      int    `json:"skipped"`
}

// Writer persists a derivation through the authenticated router GraphQL (memoryImport) — the same op
// `sync push` uses, so core stays private and the router enforces the caller's rights to the workspace.
// GQL is the authenticated client; correlationIds are deterministic (hash of the candidate's local id)
// so re-import dedups to a no-op and causal_refs resolve without any server round-trip.
type Writer struct {
	GQL       *apiclient.Client
	ProjectID string
	DryRun    bool
	Log       func(string)
}

// WriteResult summarizes a session write.
type WriteResult struct {
	Appended int
	Skipped  int
	ByKind   map[Kind]int
}

const importMutation = `mutation($projectId: ID!, $streams: [MemoryStreamImportInput!]!){ memoryImport(projectId: $projectId, streams: $streams){ imported { streamSuffix appended skipped } } }`

// deterministicID derives a stable UUID-shaped id from a seed, so re-runs produce identical
// correlationIds (idempotent import) and causal_refs resolve to the same targets.
func deterministicID(seed string) string {
	sum := sha256.Sum256([]byte("derive:" + seed))
	h := hex.EncodeToString(sum[:16])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

// WriteSession imports the derivation's candidates: decisions/commits/keyframe → the domain stream,
// mistakes → the mistake stream. Returns appended/skipped (skipped = already present = idempotent).
func (w *Writer) WriteSession(d SessionDerivation) (WriteResult, error) {
	res := WriteResult{ByKind: map[Kind]int{}}
	var domain, mistake []eventEnvelope
	for _, c := range d.Candidates {
		env := w.envelope(d, c)
		if c.Kind == KindMistake {
			mistake = append(mistake, env)
		} else {
			domain = append(domain, env)
		}
		res.ByKind[c.Kind]++
	}

	var batches []streamBatch
	if len(domain) > 0 {
		batches = append(batches, streamBatch{StreamSuffix: "domain", Events: domain})
	}
	if len(mistake) > 0 {
		batches = append(batches, streamBatch{StreamSuffix: "mistake", Events: mistake})
	}

	if w.DryRun {
		if w.Log != nil {
			raw, _ := json.MarshalIndent(map[string]any{"projectId": w.ProjectID, "streams": batches}, "", "  ")
			w.Log(string(raw))
		}
		res.Appended = len(domain) + len(mistake)
		return res, nil
	}

	imported, err := w.postImport(batches)
	if err != nil {
		return res, err
	}
	for _, im := range imported {
		res.Appended += im.Appended
		res.Skipped += im.Skipped
	}
	return res, nil
}

func (w *Writer) envelope(d SessionDerivation, c Candidate) eventEnvelope {
	id := deterministicID(c.LocalID)
	var refs []string
	for _, r := range c.CausalRefs {
		refs = append(refs, deterministicID(r))
	}
	content := c.Text
	if content == "" {
		content = c.Title
	}
	causation := id
	if len(refs) > 0 {
		causation = refs[0]
	}
	ts := c.Timestamp
	if ts == "" {
		ts = d.WindowTo
	}
	return eventEnvelope{
		CorrelationID: id,
		CausationID:   causation,
		Timestamp:     ts,
		EntryType:     entryType(c.Kind),
		Payload: map[string]any{
			"content":     content,
			"title":       c.Title,
			"confidence":  c.Confidence,
			"provenance":  c.Provenance,
			"causal_refs": refs,
			"derived":     true,
		},
		TokenSize: len(content) / 4,
	}
}

func entryType(k Kind) string {
	switch k {
	case KindKeyframe:
		// NOT "keyframe.written" — a derived per-session recap must not shadow the project's real
		// curated keyframe (which memoryKeyframe returns). The brief surfaces it as a fallback.
		return "session.recap"
	case KindMistake:
		return "decision.mistake"
	case KindCommit:
		return "commit.recorded"
	default:
		return "decision.made"
	}
}

// postImport runs memoryImport, serializing each stream's events into the JSON string the mutation
// expects.
func (w *Writer) postImport(batches []streamBatch) ([]importResult, error) {
	streams := make([]map[string]any, 0, len(batches))
	for _, b := range batches {
		ev, err := json.Marshal(b.Events)
		if err != nil {
			return nil, err
		}
		streams = append(streams, map[string]any{"streamSuffix": b.StreamSuffix, "events": string(ev)})
	}
	var out struct {
		MemoryImport struct {
			Imported []importResult `json:"imported"`
		} `json:"memoryImport"`
	}
	if err := w.GQL.Query(importMutation, map[string]any{"projectId": w.ProjectID, "streams": streams}, &out); err != nil {
		return nil, err
	}
	return out.MemoryImport.Imported, nil
}
