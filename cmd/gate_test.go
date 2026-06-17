package cmd_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/gate"
)

func TestGateOutputsInjectionOnMissingKeyframe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	out, err := gate.Run(gate.Config{
		CoreURL:   srv.URL,
		ProjectID: "test-proj",
		Threshold: 1800 * time.Second,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected injection text on 404, got empty")
	}
}

func TestGateOutputsInjectionOnStaleKeyframe(t *testing.T) {
	stale := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"created_at":"` + stale + `"}`))
	}))
	defer srv.Close()

	out, err := gate.Run(gate.Config{
		CoreURL:   srv.URL,
		ProjectID: "test-proj",
		Threshold: 1800 * time.Second,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected injection text on stale keyframe, got empty")
	}
}

func TestGateSilentOnRecentKeyframe(t *testing.T) {
	recent := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"created_at":"` + recent + `"}`))
	}))
	defer srv.Close()

	out, err := gate.Run(gate.Config{
		CoreURL:   srv.URL,
		ProjectID: "test-proj",
		Threshold: 1800 * time.Second,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output for recent keyframe, got: %s", out)
	}
}

func TestGateSilentOnCoreUnreachable(t *testing.T) {
	out, err := gate.Run(gate.Config{
		CoreURL:   "http://127.0.0.1:19996",
		ProjectID: "test-proj",
		Threshold: 1800 * time.Second,
	})

	if err != nil {
		t.Fatalf("expected silent exit, got error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output when Core unreachable, got: %s", out)
	}
}
