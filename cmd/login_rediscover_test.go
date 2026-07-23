package cmd

import (
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

func TestLoginPrefersFreshDiscoveryOverStaleConfig(t *testing.T) {
	stubLoginSeams(t)
	t.Setenv("QUORUM_CLIENT_ID", "")
	origPKCE, origStore := loginRunPKCEFlow, loginStoreToken
	defer func() { loginRunPKCEFlow = origPKCE; loginStoreToken = origStore }()

	loginLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{
			AuthURL:  "https://oauth.production.flmnt.ai/authorize",
			TokenURL: "https://oauth.production.flmnt.ai/token",
			ClientID: "stale-client",
		}, nil
	}
	loginDiscover = func(string) (oauthDiscovery, error) {
		return oauthDiscovery{
			AuthorizationEndpoint: "https://oauth.production.mmmnt.ai/authorize",
			TokenEndpoint:         "https://oauth.production.mmmnt.ai/token",
			ClientID:              "flmnt-cli",
		}, nil
	}
	var pkceCfg auth.PKCEConfig
	loginRunPKCEFlow = func(cfg auth.PKCEConfig, _ func(string) error) (auth.TokenSet, error) {
		pkceCfg = cfg
		return auth.TokenSet{AccessToken: "acc"}, nil
	}
	loginStoreToken = func(string, auth.TokenSet) error { return nil }
	var saved auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { saved = c; return nil }

	if _, _, err := runLoginArgs(t, "--server-url", "https://mcp.production.flmnt.ai/mcp"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if pkceCfg.AuthURL != "https://oauth.production.mmmnt.ai/authorize" || pkceCfg.TokenURL != "https://oauth.production.mmmnt.ai/token" {
		t.Fatalf("login must use the freshly discovered broker, not stale config: %+v", pkceCfg)
	}
	if saved.AuthURL != "https://oauth.production.mmmnt.ai/authorize" || saved.TokenURL != "https://oauth.production.mmmnt.ai/token" {
		t.Fatalf("login must persist the discovered broker over stale config: %+v", saved)
	}
}
