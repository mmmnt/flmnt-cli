package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
)

type oauthDiscovery struct {
	AuthorizationEndpoint       string `json:"authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	ClientID                    string `json:"client_id"`
	GraphqlEndpoint             string `json:"graphql_endpoint"`
}

func discoverOAuth(serverURL string) (oauthDiscovery, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return oauthDiscovery{}, err
	}
	origin := u.Scheme + "://" + u.Host
	resp, err := httpx.Client.Get(origin + "/.well-known/oauth-authorization-server")
	if err != nil {
		return oauthDiscovery{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return oauthDiscovery{}, fmt.Errorf("discovery returned %d", resp.StatusCode)
	}
	var doc oauthDiscovery
	if err := json.Unmarshal(raw, &doc); err != nil {
		return oauthDiscovery{}, err
	}
	for _, ep := range []string{doc.AuthorizationEndpoint, doc.TokenEndpoint, doc.DeviceAuthorizationEndpoint, doc.GraphqlEndpoint} {
		if !secureEndpoint(ep) {
			return oauthDiscovery{}, fmt.Errorf("discovery advertised an insecure endpoint: %s", ep)
		}
	}
	return doc, nil
}

// secureEndpoint accepts an https URL, or an http URL only when the host is loopback (local dev). A
// tampered discovery document could otherwise point the token/refresh POST at a cleartext http host.
func secureEndpoint(raw string) bool {
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	host := u.Hostname()
	return u.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveLoginEndpoints(flagAuth, flagToken, flagClient, envClient string, cfg auth.CLIConfig, doc oauthDiscovery) (authURL, tokenURL, clientID string) {
	authURL = firstNonEmpty(flagAuth, cfg.AuthURL, doc.AuthorizationEndpoint)
	tokenURL = firstNonEmpty(flagToken, cfg.TokenURL, doc.TokenEndpoint)
	clientID = firstNonEmpty(flagClient, envClient, cfg.ClientID, doc.ClientID)
	return
}
