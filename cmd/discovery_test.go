package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

func TestDiscoverOAuthParsesDoc(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/oauth-authorization-server" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"authorization_endpoint":"https://a/authorize","token_endpoint":"https://a/token","client_id":"cli-xyz","graphql_endpoint":"https://api/graphql"}`))
	}))
	defer srv.Close()

	doc, err := discoverOAuth(srv.URL)
	if err != nil {
		t.Fatalf("discoverOAuth: %v", err)
	}
	if doc.AuthorizationEndpoint != "https://a/authorize" || doc.TokenEndpoint != "https://a/token" || doc.ClientID != "cli-xyz" || doc.GraphqlEndpoint != "https://api/graphql" {
		t.Fatalf("doc: %+v", doc)
	}
}

func TestDiscoverOAuthErrorsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := discoverOAuth(srv.URL); err == nil {
		t.Fatal("expected error on non-200")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "x", "y"); got != "x" {
		t.Fatalf("got %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveLoginEndpointsPrefersFlags(t *testing.T) {
	cfg := auth.CLIConfig{AuthURL: "https://cfg/auth", TokenURL: "https://cfg/token", ClientID: "cfg-client"}
	doc := oauthDiscovery{AuthorizationEndpoint: "https://disc/auth", TokenEndpoint: "https://disc/token", ClientID: "disc-client"}
	a, tk, c := resolveLoginEndpoints("https://flag/auth", "https://flag/token", "flag-client", "env-client", cfg, doc)
	if a != "https://flag/auth" || tk != "https://flag/token" || c != "flag-client" {
		t.Fatalf("flags: %s %s %s", a, tk, c)
	}
}

func TestResolveLoginEndpointsFallsBackToDiscovery(t *testing.T) {
	doc := oauthDiscovery{AuthorizationEndpoint: "https://disc/auth", TokenEndpoint: "https://disc/token", ClientID: "disc-client"}
	a, tk, c := resolveLoginEndpoints("", "", "", "", auth.CLIConfig{}, doc)
	if a != "https://disc/auth" || tk != "https://disc/token" || c != "disc-client" {
		t.Fatalf("discovery: %s %s %s", a, tk, c)
	}
}

func TestResolveLoginEndpointsEnvClientBeatsConfigAndDiscovery(t *testing.T) {
	cfg := auth.CLIConfig{ClientID: "cfg-client"}
	doc := oauthDiscovery{ClientID: "disc-client"}
	_, _, c := resolveLoginEndpoints("", "", "", "env-client", cfg, doc)
	if c != "env-client" {
		t.Fatalf("client: %s", c)
	}
}
