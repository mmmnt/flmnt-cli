package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/identity"
)

func runLogin(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	stdout, _, err = runLoginArgs(t, args...)
	return stdout, err
}

const testMarker = "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD"

// stubLocalLoginSeams gives the login a machine (marker + OS user), a recorder for the login file, and
// NO server anywhere — not in the environment, not in the login config — so a test that wants the
// token step names its server explicitly and one that does not never reaches the network.
func stubLocalLoginSeams(t *testing.T) *identity.Identity {
	t.Helper()
	stubLoginSeams(t)
	origMarker, origUser, origSave, origCfg := identityMarker, identityOSUser, identitySave, authHeaderLoadConfig
	identityMarker = func() (string, error) { return testMarker, nil }
	identityOSUser = func() (string, error) { return "mike", nil }
	saved := &identity.Identity{}
	identitySave = func(i identity.Identity) error { *saved = i; return nil }
	authHeaderLoadConfig = func() (auth.CLIConfig, error) { return auth.CLIConfig{}, nil }
	t.Setenv("QUORUM_SERVER_URL", "")
	t.Cleanup(func() {
		identityMarker, identityOSUser, identitySave, authHeaderLoadConfig = origMarker, origUser, origSave, origCfg
	})
	return saved
}

// deviceFlowNeverRuns fails the test if the device grant is attempted.
func deviceFlowNeverRuns(t *testing.T) {
	t.Helper()
	orig := loginRunDeviceFlow
	loginRunDeviceFlow = func(cfg auth.DeviceConfig, _ auth.DevicePrompter, _ func() time.Time, _ func(time.Duration)) (auth.TokenSet, error) {
		t.Errorf("the device grant must not run; cfg %+v", cfg)
		return auth.TokenSet{}, errors.New("must not run")
	}
	t.Cleanup(func() { loginRunDeviceFlow = orig })
}

/*
A local flmnt mints for a declared person. The declaration is a LOCAL LOGIN the CLI owns, with no

	input: the marker the host machine already carries and the OS user, digested to a sub that is
	persisted — the file is what makes the login explicit. Founder ruling 2026-10-19.
*/
func TestLoginLocalDerivesAndRecordsTheSub(t *testing.T) {
	saved := stubLocalLoginSeams(t)
	deviceFlowNeverRuns(t)

	stdout, err := runLogin(t, "--local")
	if err != nil {
		t.Fatal(err)
	}
	want := identity.Derive(testMarker, "mike").Sub
	if saved.Sub != want || saved.CreatedAt == "" {
		t.Fatalf("saved %+v, want sub %s", saved, want)
	}
	if !strings.Contains(stdout, want) {
		t.Fatalf("the derived sub must be printed; got %q", stdout)
	}
}

/* Nothing is ever invented: a machine with no marker is a refusal and nothing is written. */
func TestLoginLocalRefusesWithoutAMarker(t *testing.T) {
	stubLocalLoginSeams(t)
	deviceFlowNeverRuns(t)
	identityMarker = func() (string, error) { return "", errors.New("no machine marker could be read from this host") }
	saves := 0
	identitySave = func(identity.Identity) error { saves++; return nil }

	if _, err := runLogin(t, "--local"); err == nil || !strings.Contains(err.Error(), "marker") {
		t.Fatalf("without a marker: %v", err)
	}
	if saves != 0 {
		t.Fatal("a refused login must write nothing")
	}
}

func TestLogoutLocalRemovesTheLocalLogin(t *testing.T) {
	orig := identityRemove
	removed := false
	identityRemove = func() error { removed = true; return nil }
	t.Cleanup(func() { identityRemove = orig; _ = logoutCmd.Flags().Set("local", "false") })
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"logout", "--local"})

	if err := rootCmd.Execute(); err != nil || !removed {
		t.Fatalf("err %v removed %v", err, removed)
	}
}

/*
The CLI's own harness credential: after the login is recorded, and when the resolved server is a local

	flmnt that advertises a device grant, the CLI runs the device grant as `flmnt-cli` and — instead of
	sending the person to a browser — approves its own user code over the broker's loopback admin
	listener for the identity it just recorded (QRM-ADR-015). The token lands under the server URL as
	any login's does.
*/
func TestLoginLocalObtainsTheCLITokenByApprovingItself(t *testing.T) {
	saved := stubLocalLoginSeams(t)
	admin := &adminFixture{approveStatus: 200, approveBody: `{"status":"approved"}`}
	adminURL := admin.serve(t)
	loginDiscover = func(serverURL string) (oauthDiscovery, error) {
		if serverURL != "http://localhost:8000/mcp" {
			t.Errorf("discovery asked of %s", serverURL)
		}
		return oauthDiscovery{TokenEndpoint: "http://localhost:4010/token", DeviceAuthorizationEndpoint: "http://localhost:4010/device_authorization", AuthorizationEndpoint: "http://localhost:4010/authorize"}, nil
	}
	origRun, origStore := loginRunDeviceFlow, loginStoreToken
	t.Cleanup(func() { loginRunDeviceFlow, loginStoreToken = origRun, origStore })
	var deviceCfg auth.DeviceConfig
	loginRunDeviceFlow = func(cfg auth.DeviceConfig, prompt auth.DevicePrompter, _ func() time.Time, _ func(time.Duration)) (auth.TokenSet, error) {
		deviceCfg = cfg
		if err := prompt(auth.DeviceAuthResponse{UserCode: "LMNP-QRST", VerificationURI: "http://localhost:4010/device"}); err != nil {
			return auth.TokenSet{}, err
		}
		return auth.TokenSet{AccessToken: "local-access", RefreshToken: "local-refresh"}, nil
	}
	var storedURL string
	var stored auth.TokenSet
	loginStoreToken = func(u string, ts auth.TokenSet) error { storedURL = u; stored = ts; return nil }
	var cfgSaved auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { cfgSaved = c; return nil }

	stdout, err := runLogin(t, "--local", "--server-url", "http://localhost:8000/mcp", "--admin-url", adminURL)
	if err != nil {
		t.Fatal(err)
	}
	if deviceCfg.ClientID != "flmnt-cli" || deviceCfg.DeviceURL != "http://localhost:4010/device_authorization" || deviceCfg.TokenURL != "http://localhost:4010/token" {
		t.Fatalf("device grant cfg: %+v", deviceCfg)
	}
	if len(admin.approved) != 1 {
		t.Fatalf("the CLI must approve its own code once over the admin listener; posted %d", len(admin.approved))
	}
	got := admin.approved[0]
	if got["code"] != "LMNP-QRST" || got["sub"] != saved.Sub || got["client_id"] != saved.ClientID("flmnt-cli") {
		t.Fatalf("approved %v; want code LMNP-QRST, sub %s, client id %s", got, saved.Sub, saved.ClientID("flmnt-cli"))
	}
	if storedURL != "http://localhost:8000/mcp" || stored.AccessToken != "local-access" {
		t.Fatalf("stored %s %+v", storedURL, stored)
	}
	if cfgSaved.ServerURL != "http://localhost:8000/mcp" || cfgSaved.TokenURL != "http://localhost:4010/token" || cfgSaved.ClientID != "flmnt-cli" {
		t.Fatalf("config saved: %+v", cfgSaved)
	}
	if !strings.Contains(stdout, saved.Sub) || !strings.Contains(stdout, "Local token stored") {
		t.Fatalf("stdout: %q", stdout)
	}
	if strings.Contains(stdout, "visit") {
		t.Fatalf("nobody is sent to a browser: %q", stdout)
	}
}
