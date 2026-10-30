package cmd

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/identity"
)

// runLogin runs `flmnt login`; the caller stubs the seams (stubLoginSeams) first, since a login wipes
// the keychain and the local login, and a test must never reach the real ones.
func runLogin(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	stdout, _, err = runLoginArgs(t, args...)
	return stdout, err
}

const testMarker = "ABB0B3CF-5122-5F8A-835B-A01323C6EDCD"

// stubLocalLoginSeams gives the login a machine (marker + OS user) with NO identity recorded yet and a
// recorder for the login file, on top of stubLoginSeams: discovery answers an empty document, so the
// token step is skipped unless a test stubs discovery itself, and nothing reaches the network, the
// real keychain or the real ~/.filament.
func stubLocalLoginSeams(t *testing.T) *identity.Identity {
	t.Helper()
	stubLoginSeams(t)
	origMarker, origUser, origSave, origLoad := identityMarker, identityOSUser, identitySave, identityLoad
	identityMarker = func() (string, error) { return testMarker, nil }
	identityOSUser = func() (string, error) { return "mike", nil }
	identityLoad = func() (identity.Identity, error) { return identity.Identity{}, identity.ErrNotFound }
	saved := &identity.Identity{}
	identitySave = func(i identity.Identity) error { *saved = i; return nil }
	t.Cleanup(func() {
		identityMarker, identityOSUser, identitySave, identityLoad = origMarker, origUser, origSave, origLoad
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
	if saved.Salt == "" || saved.CreatedAt == "" {
		t.Fatalf("a first login mints a salted identity (QRM-ADR-016 §5g); saved %+v", saved)
	}
	if want := identity.DeriveSalted(testMarker, "mike", saved.Salt).Sub; saved.Sub != want || saved.Sub == identity.Derive(testMarker, "mike").Sub {
		t.Fatalf("saved %+v, want the salted derivation %s", saved, want)
	}
	if !strings.Contains(stdout, saved.Sub) {
		t.Fatalf("the derived sub must be printed; got %q", stdout)
	}
}

/*
`login --local --new` re-mints on purpose — after a refused connection on a teammate's harness, or for a

	fresh start: a new salt, a new sub, and the old sub kept in the identity's history and named in
	the output, so no one is ever asked which UUID was theirs (QRM-ADR-016 §5g).
*/
func TestLoginLocalNewRemintsAndKeepsTheOldSubInTheHistory(t *testing.T) {
	saved := stubLocalLoginSeams(t)
	deviceFlowNeverRuns(t)
	held := identity.Derive(testMarker, "mike")
	held.CreatedAt = "2026-10-20T16:49:09Z"
	identityLoad = func() (identity.Identity, error) { return held, nil }

	stdout, err := runLogin(t, "--local", "--new")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Sub == held.Sub || saved.Salt == "" || len(saved.History) != 1 || saved.History[0].Sub != held.Sub || saved.History[0].ReplacedAt != "2026-10-20T16:00:00Z" {
		t.Fatalf("saved %+v", *saved)
	}
	if !strings.Contains(stdout, saved.Sub) || !strings.Contains(stdout, "formerly answered as "+held.Sub) {
		t.Fatalf("the output names the new sub and the one it replaces: %q", stdout)
	}

	again := stubLocalLoginSeams(t)
	identityLoad = func() (identity.Identity, error) { return held, nil }
	if _, err := runLogin(t, "--local"); err != nil || again.Sub != "" {
		t.Fatalf("without --new a held identity is reused, never re-minted: %v %+v", err, *again)
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

/*
The local login IS the login from then on. The CLI holds one login at a time (founder ruling
2026-10-20): the session config records the local kind and the loopback server it is for, so no
command needs --server-url afterwards, and every hosted token is cleared so nothing hosted — an
email from an old production token — can answer for this person again.
*/
func TestLoginLocalBecomesTheSessionAndClearsTheHostedLogin(t *testing.T) {
	stubLocalLoginSeams(t)
	deviceFlowNeverRuns(t)
	identityLoad = func() (identity.Identity, error) { return localMike, nil }
	var saved auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { saved = c; return nil }
	wiped := false
	loginDeleteAllTokens = func() ([]string, error) {
		wiped = true
		return []string{"https://mcp.production.flmnt.ai|access_token", "https://mcp.production.flmnt.ai|id_token"}, nil
	}

	stdout, err := runLogin(t, "--local")
	if err != nil {
		t.Fatal(err)
	}
	if !wiped {
		t.Error("a local login must clear every hosted token: one login at a time")
	}
	want := auth.CLIConfig{Login: auth.LoginLocal, ServerURL: defaultLocalURL, Sub: localMike.Sub, Identity: localMike.Sub, LoggedInAt: "2026-10-20T16:00:00Z"}
	if saved != want {
		t.Errorf("session recorded %+v, want %+v", saved, want)
	}
	if !strings.Contains(stdout, defaultLocalURL) || !strings.Contains(stdout, "mcp.production.flmnt.ai") {
		t.Errorf("the output must say which server the session is for and that the hosted login was cleared; got %q", stdout)
	}
}

/* The server a local login records is the loopback one named, and only a loopback one. */
func TestLoginLocalRecordsTheLoopbackServerNamedAndRefusesAHostedOne(t *testing.T) {
	stubLocalLoginSeams(t)
	deviceFlowNeverRuns(t)
	saves := 0
	identitySave = func(identity.Identity) error { saves++; return nil }
	var saved auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { saved = c; return nil }

	if _, err := runLogin(t, "--local", "--server-url", "http://127.0.0.1:9000/mcp"); err != nil {
		t.Fatal(err)
	}
	if saved.ServerURL != "http://127.0.0.1:9000/mcp" {
		t.Errorf("recorded %q", saved.ServerURL)
	}
	if _, err := runLogin(t, "--local", "--server-url", "https://mcp.production.flmnt.ai"); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("a hosted server is refused for a local login: %v", err)
	}
	if saves != 1 {
		t.Errorf("the refused login must write nothing; saves = %d", saves)
	}
}

/*
A hosted login replaces the SESSION the same way — every token goes before its own is stored, and the
config records the hosted kind and who the token says we are — and keeps the local IDENTITY: who this
machine is (the derived sub) is not a login, and a person who signs in to the cloud still needs it to
push and link their local records (QRM-ADR-016). Founder ruling 2026-10-20: identity and session are
separate; whoami answers by the session's kind, not by which credential survived.
*/
func TestLoginHostedBecomesTheSessionAndKeepsTheLocalIdentity(t *testing.T) {
	stubLoginSeams(t)
	stubIdentityRing(t)
	if err := identity.Save(localMike); err != nil {
		t.Fatal(err)
	}
	origPKCE, origStore := loginRunPKCEFlow, loginStoreToken
	t.Cleanup(func() { loginRunPKCEFlow, loginStoreToken = origPKCE, origStore })
	loginRunPKCEFlow = func(auth.PKCEConfig, func(string) error) (auth.TokenSet, error) {
		return auth.TokenSet{AccessToken: "acc", RefreshToken: "ref", IDToken: idToken(t, `{"sub":"u-123","email":"mike@flmnt.ai"}`)}, nil
	}
	order := []string{}
	loginStoreToken = func(string, auth.TokenSet) error { order = append(order, "store"); return nil }
	loginDeleteAllTokens = func() ([]string, error) { order = append(order, "wipe"); return nil, nil }
	loginLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{Login: auth.LoginLocal, ServerURL: "http://localhost:8000", ActiveWorkspaceID: "ws-local"}, nil
	}
	var saved auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { saved = c; return nil }

	stdout, err := runLogin(t, "--server-url", "https://mcp.production.flmnt.ai", "--client-id", "c", "--auth-url", "https://a", "--token-url", "https://t")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "wipe,store" {
		t.Errorf("every token is cleared before the new one is stored; got %v", order)
	}
	if kept, err := identity.Load(); err != nil || kept.Sub != localMike.Sub {
		t.Errorf("the local identity must survive a hosted login; got %+v %v", kept, err)
	}
	if saved.Login != auth.LoginHosted || saved.ServerURL != "https://mcp.production.flmnt.ai" || saved.Sub != "u-123" || saved.Identity != "mike@flmnt.ai" || saved.LoggedInAt != "2026-10-20T16:00:00Z" {
		t.Errorf("session recorded %+v", saved)
	}
	if saved.ActiveWorkspaceID != "" {
		t.Error("a local session's workspace names nothing on the hosted server and must not carry over")
	}
	if !strings.Contains(stdout, "mike@flmnt.ai") || !strings.Contains(stdout, "https://mcp.production.flmnt.ai") {
		t.Errorf("got %q", stdout)
	}
}

/* Re-logging in to the same hosted server keeps the active workspace: it still names something there. */
func TestHostedSessionKeepsTheWorkspaceOnlyForTheSameHostedServer(t *testing.T) {
	prev := auth.CLIConfig{ServerURL: "https://mcp.production.flmnt.ai/mcp", ActiveWorkspaceID: "ws", ActiveWorkspaceName: "sift"}
	same := hostedSession(prev, "https://mcp.production.flmnt.ai", auth.Claims{Sub: "u"}, auth.CLIConfig{}, loginNow())
	if same.ActiveWorkspaceID != "ws" || same.ActiveWorkspaceName != "sift" {
		t.Errorf("same origin keeps the workspace; got %+v", same)
	}
	other := hostedSession(prev, "https://mcp.staging.flmnt.dev", auth.Claims{Sub: "u"}, auth.CLIConfig{}, loginNow())
	if other.ActiveWorkspaceID != "" {
		t.Errorf("another server does not; got %+v", other)
	}
}

/*
`logout --local` ends a LOCAL SESSION: its token and the config go, exactly as `logout` would, and the
identity stays. A hosted session is left alone — it is not what --local names. Nothing removes the
identity: who this machine is outlives every login.
*/
func TestLogoutLocalEndsTheLocalSessionAndKeepsTheIdentity(t *testing.T) {
	stubLogoutSeams(t)
	stubIdentityRing(t)
	if err := identity.Save(localMike); err != nil {
		t.Fatal(err)
	}
	logoutLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{Login: auth.LoginLocal, ServerURL: defaultLocalURL, Identity: localMike.Sub}, nil
	}
	wiped, cleared := false, false
	logoutDeleteAllTokens = func() ([]string, error) { wiped = true; return []string{defaultLocalURL + "|access_token"}, nil }
	logoutClearConfig = func() error { cleared = true; return nil }

	stdout, err := runLogout(t, "--local")
	if err != nil || !wiped || !cleared {
		t.Fatalf("err %v wiped %v cleared %v", err, wiped, cleared)
	}
	if kept, err := identity.Load(); err != nil || kept.Sub != localMike.Sub {
		t.Errorf("the identity must survive; got %+v %v", kept, err)
	}
	if !strings.Contains(stdout, "Ended the local login "+localMike.Sub) || !strings.Contains(stdout, "stays") {
		t.Errorf("got %q", stdout)
	}

	wiped, cleared = false, false
	logoutLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{Login: auth.LoginHosted, ServerURL: "https://mcp.production.flmnt.ai"}, nil
	}
	stdout, err = runLogout(t, "--local")
	if err != nil || wiped || cleared {
		t.Fatalf("--local leaves a hosted session alone: err %v wiped %v cleared %v", err, wiped, cleared)
	}
	if !strings.Contains(stdout, "hosted") || !strings.Contains(stdout, "flmnt logout") {
		t.Errorf("the line must say the session is hosted and name the command that ends it; got %q", stdout)
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
	if cfgSaved.Login != auth.LoginLocal || cfgSaved.Sub != saved.Sub || cfgSaved.ServerURL != "http://localhost:8000/mcp" || cfgSaved.TokenURL != "http://localhost:4010/token" || cfgSaved.ClientID != "flmnt-cli" {
		t.Fatalf("the session must carry the endpoints the token needs: %+v", cfgSaved)
	}
	if !strings.Contains(stdout, saved.Sub) || !strings.Contains(stdout, "Local token stored") {
		t.Fatalf("stdout: %q", stdout)
	}
	if strings.Contains(stdout, "visit") {
		t.Fatalf("nobody is sent to a browser: %q", stdout)
	}
}

/*
The token step is a convenience on top of the login, never a condition of it: a server whose discovery

	fails or advertises no device grant leaves the login recorded and printed as before, and the step
	says it was skipped and why. The device grant is not attempted. (A hosted server never gets this
	far: the login itself refuses it.)
*/
func TestLoginLocalSkipsTheTokenStepWithItsReason(t *testing.T) {
	for name, tc := range map[string]struct {
		args     []string
		discover func(string) (oauthDiscovery, error)
		reason   string
	}{
		"discovery fails": {
			args:     []string{"--server-url", "http://localhost:8000/mcp"},
			discover: func(string) (oauthDiscovery, error) { return oauthDiscovery{}, errors.New("connection refused") },
			reason:   "OAuth discovery at http://localhost:8000/mcp failed: connection refused",
		},
		"no device grant advertised": {
			args: []string{"--server-url", "http://localhost:8000/mcp"},
			discover: func(string) (oauthDiscovery, error) {
				return oauthDiscovery{TokenEndpoint: "http://localhost:4010/token"}, nil
			},
			reason: "advertises no device_authorization_endpoint",
		},
	} {
		t.Run(name, func(t *testing.T) {
			saved := stubLocalLoginSeams(t)
			deviceFlowNeverRuns(t)
			if tc.discover != nil {
				loginDiscover = tc.discover
			}
			stores := 0
			origStore := loginStoreToken
			loginStoreToken = func(string, auth.TokenSet) error { stores++; return nil }
			t.Cleanup(func() { loginStoreToken = origStore })

			stdout, err := runLogin(t, append([]string{"--local"}, tc.args...)...)
			if err != nil {
				t.Fatalf("a skipped token step must not fail the login: %v", err)
			}
			if saved.Sub == "" || !strings.Contains(stdout, saved.Sub) {
				t.Fatalf("the login must still be recorded and printed: %+v %q", saved, stdout)
			}
			if !strings.Contains(stdout, "Token step skipped: ") || !strings.Contains(stdout, tc.reason) {
				t.Fatalf("stdout lacks the reason %q:\n%s", tc.reason, stdout)
			}
			if stores != 0 {
				t.Fatal("nothing may be stored when the step is skipped")
			}
		})
	}
}

/*
When the harness IS there but refuses the self-approval (the admin port not published, a sub the broker

	does not accept), the login stands and the token step fails loudly with the broker's words — a
	silent skip here would leave every CLI command unauthenticated with nothing to explain it.
*/
func TestLoginLocalFailsTheTokenStepWithTheBrokersRefusal(t *testing.T) {
	saved := stubLocalLoginSeams(t)
	admin := &adminFixture{approveStatus: 400, approveBody: `{"code":"LOCAL_SUB_INVALID","message":"sub is not local_ + a UUIDv5"}`}
	loginDiscover = func(string) (oauthDiscovery, error) {
		return oauthDiscovery{TokenEndpoint: "http://localhost:4010/token", DeviceAuthorizationEndpoint: "http://localhost:4010/device_authorization"}, nil
	}
	origRun, origStore := loginRunDeviceFlow, loginStoreToken
	t.Cleanup(func() { loginRunDeviceFlow, loginStoreToken = origRun, origStore })
	loginRunDeviceFlow = func(_ auth.DeviceConfig, prompt auth.DevicePrompter, _ func() time.Time, _ func(time.Duration)) (auth.TokenSet, error) {
		if err := prompt(auth.DeviceAuthResponse{UserCode: "LMNP-QRST"}); err != nil {
			return auth.TokenSet{}, err
		}
		return auth.TokenSet{AccessToken: "never"}, nil
	}
	stores := 0
	loginStoreToken = func(string, auth.TokenSet) error { stores++; return nil }

	stdout, err := runLogin(t, "--local", "--server-url", "http://localhost:8000/mcp", "--admin-url", admin.serve(t))
	if err == nil || !strings.Contains(err.Error(), "LOCAL_SUB_INVALID: sub is not local_ + a UUIDv5") {
		t.Fatalf("want the broker's refusal verbatim, got %v", err)
	}
	if saved.Sub == "" || !strings.Contains(stdout, saved.Sub) {
		t.Fatalf("the login itself must stand: %+v %q", saved, stdout)
	}
	if stores != 0 {
		t.Fatal("no token may be stored after a refused approval")
	}
}

/*
The identity is recorded once and reused ever after: a machine that already holds one logs in with
THAT sub, whatever the marker derives today — the file is the identity (identity.Load never
re-derives), so a later salted identity has one and the same path. Nothing is re-saved.
*/
func TestLoginLocalReusesTheRecordedIdentity(t *testing.T) {
	saved := stubLocalLoginSeams(t)
	deviceFlowNeverRuns(t)
	stubIdentityRing(t)
	identityLoad = identity.Load
	recorded := identity.Derive("ANOTHER-MARKER", "mike")
	if err := identity.Save(recorded); err != nil {
		t.Fatal(err)
	}
	var session auth.CLIConfig
	loginSaveConfig = func(c auth.CLIConfig) error { session = c; return nil }

	stdout, err := runLogin(t, "--local")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Sub != "" {
		t.Errorf("an identity the machine already holds is not re-saved; saved %+v", *saved)
	}
	if session.Sub != recorded.Sub || !strings.Contains(stdout, recorded.Sub) {
		t.Errorf("the session and the output must carry the recorded sub %s; got %+v %q", recorded.Sub, session, stdout)
	}
}
