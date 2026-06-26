package record

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type captured struct {
	method string
	path   string
	auth   string
	wsID   string
	body   map[string]any
}

// server returns an httptest server that records the request and replies 200 {}.
func server(t *testing.T, sink *captured) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sink.method = r.Method
		sink.path = r.URL.Path
		sink.auth = r.Header.Get("Authorization")
		sink.wsID = r.Header.Get("X-Workspace-Id")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &sink.body)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
}

func newClient(url string) Client {
	return Client{Endpoint: url, ProjectID: "proj", AuthHeader: "Bearer tok"}
}

func TestMetricPostsToMetricsRouteWithHeaders(t *testing.T) {
	var c captured
	srv := server(t, &c)
	defer srv.Close()
	if err := newClient(srv.URL).Metric("proj::metrics", "ci.run", 1, map[string]string{"category": "test"}); err != nil {
		t.Fatalf("Metric: %v", err)
	}
	if c.method != "POST" || c.path != "/streams/proj::metrics/metrics" {
		t.Fatalf("got %s %s", c.method, c.path)
	}
	if c.auth != "Bearer tok" || c.wsID != "proj" {
		t.Fatalf("headers: auth=%q ws=%q", c.auth, c.wsID)
	}
	if c.body["metric_name"] != "ci.run" || c.body["value"].(float64) != 1 {
		t.Fatalf("body: %#v", c.body)
	}
	if labels, _ := c.body["labels"].(map[string]any); labels["category"] != "test" {
		t.Fatalf("labels: %#v", c.body["labels"])
	}
}

func TestAttestationEmitsContextAttestationToMetricsStream(t *testing.T) {
	var c captured
	srv := server(t, &c)
	defer srv.Close()
	if err := newClient(srv.URL).Attestation("proj", "verified", "matched code"); err != nil {
		t.Fatalf("Attestation: %v", err)
	}
	if c.path != "/streams/proj::metrics/metrics" {
		t.Fatalf("path: %s", c.path)
	}
	if c.body["metric_name"] != "ContextAttestation" || c.body["value"].(float64) != 1 {
		t.Fatalf("body: %#v", c.body)
	}
	if labels, _ := c.body["labels"].(map[string]any); labels["kind"] != "verified" || labels["note"] != "matched code" {
		t.Fatalf("labels: %#v", c.body["labels"])
	}
}

func TestPlanPostsContentToPlansRoute(t *testing.T) {
	var c captured
	srv := server(t, &c)
	defer srv.Close()
	if err := newClient(srv.URL).Plan("proj::plan", "step 1; step 2"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if c.path != "/streams/proj::plan/plans" || c.body["content"] != "step 1; step 2" {
		t.Fatalf("got %s body=%#v", c.path, c.body)
	}
}

func TestSupersessionPostsContentAndSupersedes(t *testing.T) {
	var c captured
	srv := server(t, &c)
	defer srv.Close()
	if err := newClient(srv.URL).Supersession("proj::domain", "use Postgres", "e123"); err != nil {
		t.Fatalf("Supersession: %v", err)
	}
	if c.path != "/streams/proj::domain/supersessions" {
		t.Fatalf("path: %s", c.path)
	}
	if c.body["content"] != "use Postgres" || c.body["supersedes"] != "e123" {
		t.Fatalf("body: %#v", c.body)
	}
}

func TestBaseURLStripsMcpSuffix(t *testing.T) {
	var c captured
	srv := server(t, &c)
	defer srv.Close()
	// Endpoint with a trailing /mcp must still hit /streams/... at the host root.
	cl := Client{Endpoint: srv.URL + "/mcp", ProjectID: "proj"}
	if err := cl.Plan("proj::plan", "x"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if c.path != "/streams/proj::plan/plans" {
		t.Fatalf("path should not include /mcp: %s", c.path)
	}
}

func TestNon2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer srv.Close()
	if err := newClient(srv.URL).Metric("proj::metrics", "x", 1, nil); err == nil {
		t.Fatal("expected error on 403")
	}
}

// StreamHelpers sanity-checks the canonical stream id construction.
func TestStreamHelpers(t *testing.T) {
	if MetricsStream("p") != "p::metrics" || PlanStream("p") != "p::plan" || DomainStream("p") != "p::domain" {
		t.Fatal("stream helpers")
	}
}
