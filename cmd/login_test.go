package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

func TestPKCEFlowExchangesCode(t *testing.T) {
	// Mock Cognito token endpoint
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(auth.TokenSet{
			AccessToken:  "access-abc",
			RefreshToken: "refresh-xyz",
		})
	}))
	defer tokenSrv.Close()

	// Mock auth endpoint — immediately redirects to callback with a code
	var callbackURL string
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := "test-auth-code"
		http.Redirect(w, r, callbackURL+"?code="+code, http.StatusFound)
	}))
	defer authSrv.Close()

	var openedURL string
	var capturedCode string

	cfg := auth.PKCEConfig{
		AuthURL:     authSrv.URL,
		TokenURL:    tokenSrv.URL,
		ClientID:    "test-client",
		RedirectURI: "http://127.0.0.1:9877",
	}

	// Simulate browser: follow the redirect ourselves to hit the callback
	openBrowser := func(rawURL string) error {
		openedURL = rawURL
		u, _ := url.Parse(rawURL)
		redirect := u.Query().Get("redirect_uri")
		if redirect == "" {
			redirect = "http://127.0.0.1:9877"
		}
		go func() {
			http.Get(redirect + "?code=test-auth-code")
		}()
		_ = capturedCode
		return nil
	}
	callbackURL = cfg.RedirectURI

	tokens, err := auth.RunPKCEFlow(cfg, openBrowser)
	if err != nil {
		t.Fatalf("RunPKCEFlow: %v", err)
	}
	if tokens.AccessToken != "access-abc" {
		t.Errorf("expected access token 'access-abc', got %q", tokens.AccessToken)
	}
	if tokens.RefreshToken != "refresh-xyz" {
		t.Errorf("expected refresh token 'refresh-xyz', got %q", tokens.RefreshToken)
	}
	_ = openedURL
}
