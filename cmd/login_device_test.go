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

func TestLoginDeviceRequiresDeviceURL(t *testing.T) {
	if _, _, err := runLoginArgs(t, "--server-url", "https://s", "--device", "--token-url", "https://t", "--client-id", "c"); err == nil {
		t.Fatal("expected error without --device-url")
	}
}

func TestLoginDeviceStoresToken(t *testing.T) {
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
