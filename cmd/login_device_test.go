package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/pflag"
)

func runLoginArgs(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	loginCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs(append([]string{"login"}, args...))
	err = rootCmd.Execute()
	return out.String(), errb.String(), err
}

// stubLoginSeams isolates login from the network and the real config file.
func stubLoginSeams(t *testing.T) {
	t.Helper()
	origDisc, origLoad, origSave := loginDiscover, loginLoadConfig, loginSaveConfig
	loginDiscover = func(string) (oauthDiscovery, error) { return oauthDiscovery{}, nil }
	loginLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }
	loginSaveConfig = func(auth.CLIConfig) error { return nil }
	t.Cleanup(func() { loginDiscover = origDisc; loginLoadConfig = origLoad; loginSaveConfig = origSave })
}

func TestLoginDeviceRequiresDeviceURL(t *testing.T) {
	stubLoginSeams(t)
	if _, _, err := runLoginArgs(t, "--server-url", "https://s", "--device", "--token-url", "https://t", "--client-id", "c"); err == nil {
		t.Fatal("expected error without --device-url")
	}
}

func TestLoginDeviceStoresToken(t *testing.T) {
	stubLoginSeams(t)
	origRun := loginRunDeviceFlow
	origStore := loginStoreToken
	defer func() { loginRunDeviceFlow = origRun; loginStoreToken = origStore }()

	loginRunDeviceFlow = func(cfg auth.DeviceConfig, prompt auth.DevicePrompter, _ func() time.Time, _ func(time.Duration)) (auth.TokenSet, error) {
		if cfg.DeviceURL != "https://d" || cfg.ClientID != "c" {
			t.Fatalf("device cfg: %+v", cfg)
		}
		_ = prompt(auth.DeviceAuthResponse{UserCode: "AAAA-BBBB", VerificationURI: "https://verify"})
		return auth.TokenSet{AccessToken: "dev-access"}, nil
	}
	var storedURL string
	var stored auth.TokenSet
	loginStoreToken = func(u string, ts auth.TokenSet) error { storedURL = u; stored = ts; return nil }

	stdout, _, err := runLoginArgs(t,
		"--server-url", "https://s", "--device",
		"--device-url", "https://d", "--token-url", "https://t", "--client-id", "c",
	)
	if err != nil {
		t.Fatalf("login --device: %v", err)
	}
	if storedURL != "https://s" || stored.AccessToken != "dev-access" {
		t.Fatalf("stored %s %+v", storedURL, stored)
	}
	if !strings.Contains(stdout, "AAAA-BBBB") || !strings.Contains(stdout, "https://verify") {
		t.Fatalf("prompt output: %q", stdout)
	}
}

func TestLoginPKCEDiscoversEndpointsAndSavesConfig(t *testing.T) {
	stubLoginSeams(t)
	origPKCE := loginRunPKCEFlow
	origStore := loginStoreToken
	defer func() { loginRunPKCEFlow = origPKCE; loginStoreToken = origStore }()

	loginDiscover = func(string) (oauthDiscovery, error) {
		return oauthDiscovery{AuthorizationEndpoint: "https://disc/authorize", TokenEndpoint: "https://disc/token"}, nil
	}
	var pkceCfg auth.PKCEConfig
	loginRunPKCEFlow = func(cfg auth.PKCEConfig, _ func(string) error) (auth.TokenSet, error) {
		pkceCfg = cfg
		return auth.TokenSet{AccessToken: "acc"}, nil
	}
	var stored auth.TokenSet
	var storedURL string
	loginStoreToken = func(u string, ts auth.TokenSet) error { storedURL = u; stored = ts; return nil }
	var saved auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { saved = c; return nil }

	if _, _, err := runLoginArgs(t, "--server-url", "https://s", "--client-id", "cli-1"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if pkceCfg.AuthURL != "https://disc/authorize" || pkceCfg.TokenURL != "https://disc/token" || pkceCfg.ClientID != "cli-1" {
		t.Fatalf("pkce cfg: %+v", pkceCfg)
	}
	if pkceCfg.RedirectURI != "http://127.0.0.1:9877/" {
		t.Fatalf("redirect_uri must match the registered callback (trailing slash): %q", pkceCfg.RedirectURI)
	}
	if storedURL != "https://s" || stored.AccessToken != "acc" {
		t.Fatalf("store: %s %+v", storedURL, stored)
	}
	if saved.ServerURL != "https://s" || saved.AuthURL != "https://disc/authorize" || saved.TokenURL != "https://disc/token" || saved.ClientID != "cli-1" {
		t.Fatalf("saved: %+v", saved)
	}
}
