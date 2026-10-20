package cmd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/cmd"
	"github.com/mmmnt/flmnt-cli/internal/health"
)

func TestRenderHealthLabelsLocalScopeWhenAServiceIsDown(t *testing.T) {
	var buf bytes.Buffer

	cmd.RenderHealth(&buf, []health.Result{
		{Service: "core", OK: false, Message: "connection refused"},
		{Service: "proxy", OK: true, Message: "ok"},
	})

	if !strings.Contains(buf.String(), "local stack") {
		t.Errorf("expected output to scope the failure to the local stack, got:\n%s", buf.String())
	}
}

func TestHealthChecksOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	results := health.Check(health.Config{
		CoreURL:   srv.URL,
		EngineURL: srv.URL,
		ProxyURL:  srv.URL,
	})

	for _, r := range results {
		if !r.OK {
			t.Errorf("expected %s to be OK, got: %s", r.Service, r.Message)
		}
	}
}

func TestHealthChecksDown(t *testing.T) {
	results := health.Check(health.Config{
		CoreURL:   "http://127.0.0.1:19999",
		EngineURL: "http://127.0.0.1:19998",
		ProxyURL:  "http://127.0.0.1:19997",
	})

	for _, r := range results {
		if r.OK {
			t.Errorf("expected %s to be down, got OK", r.Service)
		}
		if !strings.Contains(r.Message, "connection refused") && !strings.Contains(r.Message, "unreachable") && r.Message == "" {
			t.Errorf("expected error message for %s, got empty", r.Service)
		}
	}
}

func TestHealthChecksUnhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	results := health.Check(health.Config{
		CoreURL:   srv.URL,
		EngineURL: srv.URL,
		ProxyURL:  srv.URL,
	})

	for _, r := range results {
		if r.OK {
			t.Errorf("expected %s to report unhealthy on 503, got OK", r.Service)
		}
	}
}

/*
health probed core:3000, engine:3001 and the proxy:9876 — the host-port compose of an earlier local

	stack — and nothing else, so with the harness up and healthy on :8000 it reported three services
	"down", and against the hosted MCP it never asked the one server the session actually uses.
*/
func TestHealthProbesTheConfiguredServerAndSkipsUndeclaredLocalServices(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	results := health.Check(health.Config{ServerURL: srv.URL + "/mcp"})

	if len(results) != 1 || results[0].Service != "mcp" || !results[0].OK {
		t.Fatalf("expected one healthy mcp probe, got %+v", results)
	}
	if len(paths) != 1 || paths[0] != "/health" {
		t.Fatalf("the MCP's /health at the server's origin must be probed, got %v", paths)
	}
}

func TestHealthNamesTheEnvironmentItResolved(t *testing.T) {
	if line := cmd.HealthEnvironmentLine("http://localhost:8000/mcp"); !strings.Contains(line, "local") || !strings.Contains(line, "localhost:8000") {
		t.Errorf("loopback server: %q", line)
	}
	if line := cmd.HealthEnvironmentLine("https://mcp.production.flmnt.ai/mcp"); !strings.Contains(line, "hosted") || strings.Contains(line, "local") {
		t.Errorf("hosted server: %q", line)
	}
	if line := cmd.HealthEnvironmentLine(""); !strings.Contains(line, "no server URL") {
		t.Errorf("unconfigured: %q", line)
	}
}

/* The "local stack" caveat belongs to core/engine/proxy; a down MCP is the session's server, not a local extra. */
func TestRenderHealthDoesNotScopeADownServerToTheLocalStack(t *testing.T) {
	var buf bytes.Buffer

	cmd.RenderHealth(&buf, []health.Result{{Service: "mcp", OK: false, Message: "connection refused"}})

	if strings.Contains(buf.String(), "local stack") {
		t.Errorf("got:\n%s", buf.String())
	}
}
