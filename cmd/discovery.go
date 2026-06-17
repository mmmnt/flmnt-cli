package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

type oauthDiscovery struct {
	AuthorizationEndpoint       string `json:"authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	ClientID                    string `json:"client_id"`
}

func discoverOAuth(serverURL string) (oauthDiscovery, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return oauthDiscovery{}, err
	}
	origin := u.Scheme + "://" + u.Host
	resp, err := http.Get(origin + "/.well-known/oauth-authorization-server")
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
	return doc, nil
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
