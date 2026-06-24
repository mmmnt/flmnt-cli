package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// The broker (single AS) passes the client's original `state` through unchanged
// on the loopback redirect. RunPKCEFlow must send a non-empty `state` on the
// authorize request so the broker has a value to echo back.
func TestPKCEFlowSendsState(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TokenSet{AccessToken: "a", RefreshToken: "r"})
	}))
	defer tokenSrv.Close()

	cfg := PKCEConfig{
		AuthURL:     "https://oauth.example.flmnt.dev/authorize",
		TokenURL:    tokenSrv.URL,
		ClientID:    "flmnt-cli",
		RedirectURI: "http://127.0.0.1:9877/",
	}

	var sentState string
	openBrowser := func(rawURL string) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		sentState = u.Query().Get("state")
		// Echo the state back unchanged, as the broker does.
		go func() { http.Get(cfg.RedirectURI + "?code=c&state=" + url.QueryEscape(sentState)) }()
		return nil
	}

	if _, err := RunPKCEFlow(cfg, openBrowser); err != nil {
		t.Fatalf("RunPKCEFlow: %v", err)
	}
	if sentState == "" {
		t.Fatal("authorize request did not include a non-empty state parameter")
	}
}

// When the broker echoes back a state that does NOT match what the client sent,
// the loopback callback must reject it (CSRF protection) and fail fast rather
// than exchange the code.
func TestPKCEFlowRejectsMismatchedState(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TokenSet{AccessToken: "a", RefreshToken: "r"})
	}))
	defer tokenSrv.Close()

	cfg := PKCEConfig{
		AuthURL:     "https://oauth.example.flmnt.dev/authorize",
		TokenURL:    tokenSrv.URL,
		ClientID:    "flmnt-cli",
		RedirectURI: "http://127.0.0.1:9877/",
	}

	openBrowser := func(rawURL string) error {
		go func() { http.Get(cfg.RedirectURI + "?code=c&state=attacker-state") }()
		return nil
	}

	if _, err := RunPKCEFlow(cfg, openBrowser); err == nil {
		t.Fatal("expected RunPKCEFlow to reject mismatched state")
	}
}
