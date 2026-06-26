// Package record is the per-entry write half of the CLI — the deterministic counterpart to the
// model-driven MCP write tools. Each method POSTs to the same Core REST route the matching MCP tool
// uses (/streams/{id}/metrics, /plans, /supersessions, /entries), so a hook/CI write lands in the
// exact same stream and feeds the same dashboard KPIs as an agent's tool call. Auth + base-URL
// handling mirrors internal/brief (Bearer + X-Workspace-Id; trailing /mcp or /sse stripped).
package record

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client writes single entries to a flmnt endpoint. AuthHeader is "Bearer <token>" for remote, ""
// for a local stack. ProjectID doubles as the X-Workspace-Id the remote proxy requires.
type Client struct {
	Endpoint   string
	ProjectID  string
	AuthHeader string
	HTTP       *http.Client
}

func (c Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 8 * time.Second}
}

// baseURL strips a trailing /mcp or /sse so REST routes resolve, matching brief.baseURL/syncBaseURL.
func (c Client) baseURL() string {
	s := strings.TrimRight(c.Endpoint, "/")
	for _, suf := range []string{"/mcp", "/sse"} {
		s = strings.TrimSuffix(s, suf)
	}
	return strings.TrimRight(s, "/")
}

func (c Client) post(path string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL()+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.AuthHeader != "" {
		req.Header.Set("Authorization", c.AuthHeader)
	}
	if c.ProjectID != "" {
		req.Header.Set("X-Workspace-Id", c.ProjectID)
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s -> %d: %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// MetricsStream / PlanStream / DomainStream resolve the canonical per-project stream ids.
func MetricsStream(project string) string { return project + "::metrics" }
func PlanStream(project string) string    { return project + "::plan" }
func DomainStream(project string) string  { return project + "::domain" }

// Metric records an operational metric to a metrics stream — POST /streams/{id}/metrics
// {metric_name, value, labels}. Mirrors the record_metric MCP tool.
func (c Client) Metric(streamID, name string, value float64, labels map[string]string) error {
	if labels == nil {
		labels = map[string]string{}
	}
	return c.post("/streams/"+streamID+"/metrics",
		map[string]any{"metric_name": name, "value": value, "labels": labels}, nil)
}

// Attestation records a ContextAttestation metric (value=1, labels {kind, note}) to the project's
// metrics stream. Mirrors the record_attestation MCP tool.
func (c Client) Attestation(project, kind, note string) error {
	return c.Metric(MetricsStream(project), "ContextAttestation", 1, map[string]string{"kind": kind, "note": note})
}

// Plan records a plan snapshot — POST /streams/{id}/plans {content}. Mirrors the record_plan MCP tool.
func (c Client) Plan(streamID, content string) error {
	return c.post("/streams/"+streamID+"/plans", map[string]any{"content": content}, nil)
}

// Supersession records a new decision that REPLACES a prior entry, creating the typed SUPERSEDED_BY
// edge — POST /streams/{id}/supersessions {content, supersedes}. Mirrors the record_supersession tool.
func (c Client) Supersession(streamID, content, supersedes string) error {
	return c.post("/streams/"+streamID+"/supersessions",
		map[string]any{"content": content, "supersedes": supersedes}, nil)
}
