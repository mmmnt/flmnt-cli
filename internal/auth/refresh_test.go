package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func makeJWTExp(exp int64) string {
	payload := fmt.Sprintf(`{"sub":"s","exp":%d}`, exp)
	return "h." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}

func TestRefreshAccessTokenReturnsNewAccessAndPreservesRefresh(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "id_token": "new-id"})
	}))
	defer srv.Close()

	ts, err := RefreshAccessToken(srv.URL, "client-1", "refresh-1")
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if ts.AccessToken != "new-access" {
		t.Fatalf("access: got %q", ts.AccessToken)
	}
	if ts.RefreshToken != "refresh-1" {
		t.Fatalf("refresh not preserved: got %q", ts.RefreshToken)
	}
	if gotForm.Get("grant_type") != "refresh_token" ||
		gotForm.Get("client_id") != "client-1" ||
		gotForm.Get("refresh_token") != "refresh-1" {
		t.Fatalf("unexpected form: %v", gotForm)
	}
}

func TestRefreshAccessTokenReturnsErrRefreshExpiredOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	if _, err := RefreshAccessToken(srv.URL, "c", "expired"); !errors.Is(err, ErrRefreshExpired) {
		t.Fatalf("expected ErrRefreshExpired, got %v", err)
	}
}

func TestEnsureFreshReturnsStoredWhenNotNearExpiry(t *testing.T) {
	now := time.Now()
	tok := TokenSet{AccessToken: makeJWTExp(now.Add(time.Hour).Unix()), RefreshToken: "r"}
	got, refreshed, err := EnsureFreshAccessToken(tok, "http://unused.invalid", "c", 2*time.Minute, now)
	if err != nil {
		t.Fatalf("EnsureFreshAccessToken: %v", err)
	}
	if refreshed {
		t.Fatal("a token far from expiry must not be refreshed")
	}
	if got.AccessToken != tok.AccessToken {
		t.Fatal("token should be unchanged")
	}
}

func TestEnsureFreshRefreshesWhenNearExpiry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fresh-access"})
	}))
	defer srv.Close()

	now := time.Now()
	tok := TokenSet{AccessToken: makeJWTExp(now.Add(30 * time.Second).Unix()), RefreshToken: "r"}
	got, refreshed, err := EnsureFreshAccessToken(tok, srv.URL, "c", 2*time.Minute, now)
	if err != nil {
		t.Fatalf("EnsureFreshAccessToken: %v", err)
	}
	if !refreshed {
		t.Fatal("a near-expiry token must be refreshed")
	}
	if got.AccessToken != "fresh-access" || got.RefreshToken != "r" {
		t.Fatalf("unexpected tokens: %+v", got)
	}
}

func TestEnsureFreshRefreshesWhenAccessTokenUndecodable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fresh-access"})
	}))
	defer srv.Close()

	tok := TokenSet{AccessToken: "garbage", RefreshToken: "r"}
	if _, refreshed, err := EnsureFreshAccessToken(tok, srv.URL, "c", time.Minute, time.Now()); err != nil || !refreshed {
		t.Fatalf("expected refresh for undecodable token (refreshed=%v err=%v)", refreshed, err)
	}
}

func TestEnsureFreshPropagatesRefreshError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer srv.Close()

	tok := TokenSet{AccessToken: "garbage", RefreshToken: "r"}
	if _, _, err := EnsureFreshAccessToken(tok, srv.URL, "c", time.Minute, time.Now()); !errors.Is(err, ErrRefreshExpired) {
		t.Fatalf("expected ErrRefreshExpired, got %v", err)
	}
}
