package cmd_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/health"
)

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
