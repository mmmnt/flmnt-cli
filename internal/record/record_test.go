package record

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

type captured struct {
	method string
	auth   string
	query  string
	vars   map[string]any
}

// server returns an httptest server that records the GraphQL request and replies 200 {"data":{}}.
func server(t *testing.T, sink *captured) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sink.method = r.Method
		sink.auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(b, &req)
		sink.query = req.Query
		sink.vars = req.Variables
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":{"recordMemoryMetric":{"entryId":"e1"}}}`))
	}))
}

func newClient(url string) Client {
	return Client{GQL: apiclient.New(url, "tok")}
}

func TestMetricRunsRecordMemoryMetricMutationAuthenticated(t *testing.T) {
	var c captured
	srv := server(t, &c)
	defer srv.Close()
	if err := newClient(srv.URL).Metric("proj::metrics", "ci.run", 1, map[string]string{"category": "test"}); err != nil {
		t.Fatalf("Metric: %v", err)
	}
	if c.method != "POST" || !strings.Contains(c.query, "recordMemoryMetric") {
		t.Fatalf("query: %s", c.query)
	}
	if c.auth != "Bearer tok" {
		t.Fatalf("auth header = %q, want Bearer tok", c.auth)
	}
	if c.vars["streamId"] != "proj::metrics" || c.vars["metricName"] != "ci.run" || c.vars["value"].(float64) != 1 {
		t.Fatalf("vars: %#v", c.vars)
	}
	labels, _ := c.vars["labels"].([]any)
	if len(labels) != 1 {
		t.Fatalf("labels: %#v", c.vars["labels"])
	}
	if pair, _ := labels[0].(map[string]any); pair["key"] != "category" || pair["value"] != "test" {
		t.Fatalf("label pair: %#v", labels[0])
	}
}

func TestAttestationEmitsContextAttestationToMetricsStream(t *testing.T) {
	var c captured
	srv := server(t, &c)
	defer srv.Close()
	if err := newClient(srv.URL).Attestation("proj", "verified", "matched code"); err != nil {
		t.Fatalf("Attestation: %v", err)
	}
	if !strings.Contains(c.query, "recordMemoryMetric") || c.vars["streamId"] != "proj::metrics" {
		t.Fatalf("query=%s vars=%#v", c.query, c.vars)
	}
	if c.vars["metricName"] != "ContextAttestation" || c.vars["value"].(float64) != 1 {
		t.Fatalf("vars: %#v", c.vars)
	}
	labels, _ := c.vars["labels"].([]any)
	got := map[string]string{}
	for _, l := range labels {
		if pair, ok := l.(map[string]any); ok {
			got[pair["key"].(string)] = pair["value"].(string)
		}
	}
	if got["kind"] != "verified" || got["note"] != "matched code" {
		t.Fatalf("labels: %#v", labels)
	}
}

func TestPlanRunsRecordMemoryPlanMutation(t *testing.T) {
	var c captured
	srv := server(t, &c)
	defer srv.Close()
	if err := newClient(srv.URL).Plan("proj::plan", "step 1; step 2"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !strings.Contains(c.query, "recordMemoryPlan") || c.vars["streamId"] != "proj::plan" || c.vars["content"] != "step 1; step 2" {
		t.Fatalf("query=%s vars=%#v", c.query, c.vars)
	}
}

func TestSupersessionRunsRecordMemorySupersessionMutation(t *testing.T) {
	var c captured
	srv := server(t, &c)
	defer srv.Close()
	if err := newClient(srv.URL).Supersession("proj::domain", "use Postgres", "e123"); err != nil {
		t.Fatalf("Supersession: %v", err)
	}
	if !strings.Contains(c.query, "recordMemorySupersession") {
		t.Fatalf("query: %s", c.query)
	}
	if c.vars["streamId"] != "proj::domain" || c.vars["content"] != "use Postgres" || c.vars["supersedes"] != "e123" {
		t.Fatalf("vars: %#v", c.vars)
	}
}

func TestLabelPairsAreKeySorted(t *testing.T) {
	pairs := labelPairs(map[string]string{"b": "2", "a": "1"})
	if len(pairs) != 2 || pairs[0]["key"] != "a" || pairs[1]["key"] != "b" {
		t.Fatalf("pairs not key-sorted: %#v", pairs)
	}
}

func TestGraphqlErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"errors":[{"message":"not permitted"}]}`))
	}))
	defer srv.Close()
	if err := newClient(srv.URL).Metric("proj::metrics", "x", 1, nil); err == nil {
		t.Fatal("expected error when the router returns a GraphQL error")
	}
}

func TestWorkspaceFromURLExtractsWorkspaceParam(t *testing.T) {
	if ws := WorkspaceFromURL("https://host.example/mcp?workspace=abc123"); ws != "abc123" {
		t.Fatalf("WorkspaceFromURL = %q, want abc123", ws)
	}
}

func TestStreamHelpers(t *testing.T) {
	if MetricsStream("p") != "p::metrics" || PlanStream("p") != "p::plan" || DomainStream("p") != "p::domain" {
		t.Fatal("stream helpers")
	}
}
