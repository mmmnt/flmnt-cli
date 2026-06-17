package cmd_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/proxy"
)

func TestProxyInjectsAuthHeader(t *testing.T) {
	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	p, err := proxy.New(proxy.Config{
		TargetURL:    backend.URL,
		TokenFetcher: func() (string, error) { return "test-token-123", nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	p.ServeHTTP(rr, req)

	if gotAuth != "Bearer test-token-123" {
		t.Errorf("expected 'Bearer test-token-123', got %q", gotAuth)
	}
}

func TestProxyHealthEndpointAuthenticated(t *testing.T) {
	p, err := proxy.New(proxy.Config{
		TargetURL:    "http://127.0.0.1:19995",
		TokenFetcher: func() (string, error) { return "tok", nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	p.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 from /health, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !contains(body, "authenticated") {
		t.Errorf("expected 'authenticated' in health body, got: %s", body)
	}
}

func TestProxyHealthEndpointUnauthenticated(t *testing.T) {
	p, err := proxy.New(proxy.Config{
		TargetURL:    "http://127.0.0.1:19995",
		TokenFetcher: func() (string, error) { return "", nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	p.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 from /health, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !contains(body, `"authenticated":false`) {
		t.Errorf("expected authenticated:false in health body, got: %s", body)
	}
}

func TestProxyNewReturnsErrorOnInvalidURL(t *testing.T) {
	_, err := proxy.New(proxy.Config{
		TargetURL:    "://not-a-url",
		TokenFetcher: func() (string, error) { return "", nil },
	})
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestProxyForwardsRequestPathWithoutDoubling(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	p, err := proxy.New(proxy.Config{
		TargetURL:    backend.URL,
		TokenFetcher: func() (string, error) { return "tok", nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	p.ServeHTTP(rr, req)

	if gotPath != "/mcp" {
		t.Errorf("expected /mcp at backend, got %q — TargetURL must not include /mcp path", gotPath)
	}
}
