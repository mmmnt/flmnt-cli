// Package record is the per-entry write half of the CLI — the deterministic counterpart to the
// model-driven MCP write tools. Each method runs the matching memory mutation through the router
// GraphQL (recordMemoryMetric/recordMemoryPlan/recordMemorySupersession), so a hook/CI write is
// authenticated and RBAC-scoped by the router exactly like an agent's MCP tool call, and lands in the
// same stream feeding the same dashboard KPIs. All transport + auth lives in internal/apiclient.
package record

import (
	"net/url"
	"sort"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

// Client writes single entries to flmnt through the authenticated router GraphQL.
type Client struct {
	GQL *apiclient.Client
}

// MetricsStream / PlanStream / DomainStream resolve the canonical per-project stream ids.
func MetricsStream(project string) string { return project + "::metrics" }
func PlanStream(project string) string    { return project + "::plan" }
func DomainStream(project string) string  { return project + "::domain" }

// WorkspaceFromURL extracts the ?workspace=<id> param from an MCP URL — used as the project/workspace
// fallback when the login config carries the workspace in the server_url rather than a separate field.
func WorkspaceFromURL(raw string) string {
	if u, err := url.Parse(strings.TrimSpace(raw)); err == nil {
		return u.Query().Get("workspace")
	}
	return ""
}

const mutationMetric = `mutation($streamId: ID!, $metricName: String!, $value: Float!, $labels: [MemoryLabelInput!]){ recordMemoryMetric(streamId: $streamId, metricName: $metricName, value: $value, labels: $labels){ entryId } }`
const mutationPlan = `mutation($streamId: ID!, $content: String!){ recordMemoryPlan(streamId: $streamId, content: $content){ entryId } }`
const mutationSupersession = `mutation($streamId: ID!, $content: String!, $supersedes: ID!){ recordMemorySupersession(streamId: $streamId, content: $content, supersedes: $supersedes){ entryId } }`

// labelPairs converts a label map into the [{key,value}] MemoryLabelInput list, key-sorted so the
// GraphQL variables are deterministic.
func labelPairs(labels map[string]string) []map[string]string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]map[string]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, map[string]string{"key": k, "value": labels[k]})
	}
	return pairs
}

// Metric records an operational metric via the recordMemoryMetric mutation. Mirrors the record_metric MCP tool.
func (c Client) Metric(streamID, name string, value float64, labels map[string]string) error {
	return c.GQL.Query(mutationMetric, map[string]any{
		"streamId": streamID, "metricName": name, "value": value, "labels": labelPairs(labels),
	}, nil)
}

// Attestation records a ContextAttestation metric (value=1, labels {kind, note}) to the project's
// metrics stream. Mirrors the record_attestation MCP tool.
func (c Client) Attestation(project, kind, note string) error {
	return c.Metric(MetricsStream(project), "ContextAttestation", 1, map[string]string{"kind": kind, "note": note})
}

// Plan records a plan snapshot via the recordMemoryPlan mutation. Mirrors the record_plan MCP tool.
func (c Client) Plan(streamID, content string) error {
	return c.GQL.Query(mutationPlan, map[string]any{"streamId": streamID, "content": content}, nil)
}

// Supersession records a decision that REPLACES a prior entry (creating the SUPERSEDED_BY edge) via
// the recordMemorySupersession mutation. Mirrors the record_supersession MCP tool.
func (c Client) Supersession(streamID, content, supersedes string) error {
	return c.GQL.Query(mutationSupersession, map[string]any{"streamId": streamID, "content": content, "supersedes": supersedes}, nil)
}
