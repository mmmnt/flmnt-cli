package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRevokeRefreshTokenPostsFormToEndpoint(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		seen = string(raw)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := RevokeRefreshToken(srv.URL, "filament-cli", "refresh-1"); err != nil {
		t.Fatalf("RevokeRefreshToken: %v", err)
	}
	if !strings.Contains(seen, "token=refresh-1") || !strings.Contains(seen, "client_id=filament-cli") {
		t.Fatalf("unexpected body: %s", seen)
	}
}

func TestRevokeRefreshTokenNoOpOnEmpty(t *testing.T) {
	if err := RevokeRefreshToken("", "", ""); err != nil {
		t.Fatalf("expected no-op, got %v", err)
	}
}

func TestRevokeRefreshTokenReturnsErrorOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad token"))
	}))
	defer srv.Close()

	err := RevokeRefreshToken(srv.URL, "filament-cli", "refresh-1")
	if err == nil {
		t.Fatal("expected error")
	}
}
