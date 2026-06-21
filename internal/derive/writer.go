package derive

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// eventEnvelope mirrors the core entry shape /sync/import accepts (full-fidelity: ids/timestamps
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

// Writer persists a derivation to a flmnt endpoint via the authenticated /sync/import route — the
// same path `sync push` uses, so core stays private. The endpoint is resolved by the caller (prod
// by default; localhost for devs); AuthHeader is "Bearer <token>" for remote, "" for a local stack.
// correlationIds are deterministic (hash of the candidate's local id) so re-import dedups to a no-op
// and causal_refs resolve without any server round-trip.
type Writer struct {
	Endpoint   string
	ProjectID  string
	AuthHeader string
	DryRun     bool
	HTTP       *http.Client
	Log        func(string)
}

// WriteResult summarizes a session write.
type WriteResult struct {
	Appended int
	Skipped  int
	ByKind   map[Kind]int
}

func (w *Writer) client() *http.Client {
	if w.HTTP != nil {
		return w.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

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
	body := map[string]any{"project_id": w.ProjectID, "streams": batches}

	if w.DryRun {
		if w.Log != nil {
			raw, _ := json.MarshalIndent(body, "", "  ")
			w.Log(string(raw))
		}
		res.Appended = len(domain) + len(mistake)
		return res, nil
	}

	imported, err := w.postImport(body)
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
		// curated keyframe (which /keyframes/latest returns). The brief surfaces it as a fallback.
		return "session.recap"
	case KindMistake:
		return "decision.mistake"
	case KindCommit:
		return "commit.recorded"
	default:
		return "decision.made"
	}
}

func (w *Writer) postImport(body map[string]any) ([]importResult, error) {
	raw, _ := json.Marshal(body)
	url := syncBaseURL(w.Endpoint) + "/sync/import"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.AuthHeader != "" {
		req.Header.Set("Authorization", w.AuthHeader)
	}
	if w.ProjectID != "" {
		req.Header.Set("X-Workspace-Id", w.ProjectID) // the remote proxy requires workspace selection
	}
	resp, err := w.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("%s -> %d: %s", url, resp.StatusCode, string(b))
	}
	var out struct {
		Imported []importResult `json:"imported"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Imported, nil
}

// syncBaseURL normalizes a server/MCP URL to the base the /sync routes live on (strips a trailing
// /mcp or /sse), mirroring the CLI's sync transport.
func syncBaseURL(raw string) string {
	s := strings.TrimRight(raw, "/")
	for _, suf := range []string{"/mcp", "/sse"} {
		s = strings.TrimSuffix(s, suf)
	}
	return strings.TrimRight(s, "/")
}
