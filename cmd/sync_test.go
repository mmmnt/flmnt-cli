package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/identity"
	"github.com/mmmnt/flmnt-cli/internal/sync"
)

func TestSyncCommandsAreRegisteredWithExpectedFlags(t *testing.T) {
	sub := map[string]bool{}
	for _, c := range syncCmd.Commands() {
		sub[c.Name()] = true
		if c.Flags().Lookup("dry-run") == nil {
			t.Fatalf("%s missing --dry-run flag", c.Name())
		}
		if c.Flags().Lookup("local-auth-cmd") != nil {
			t.Fatalf("%s still carries --local-auth-cmd; the helper it named is retired", c.Name())
		}
	}
	if !sub["push"] || !sub["pull"] {
		t.Fatalf("expected push and pull subcommands, got %v", sub)
	}
}

func TestResolveRemoteServerURLFallsBackToLoginConfig(t *testing.T) {
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	orig := authHeaderLoadConfig
	authHeaderLoadConfig = func() (auth.CLIConfig, error) {
		return auth.CLIConfig{ServerURL: "https://mcp.staging.flmnt.dev"}, nil
	}
	t.Cleanup(func() { authHeaderLoadConfig = orig })

	if got := resolveRemoteServerURL(syncPushCmd); got != "https://mcp.staging.flmnt.dev" {
		t.Fatalf("resolveRemoteServerURL = %q, want the login-config ServerURL", got)
	}
}

func TestResolveRemoteServerURLPrefersRemoteURLFlag(t *testing.T) {
	if err := syncPushCmd.Flags().Set("remote-url", "https://override.example"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syncPushCmd.Flags().Set("remote-url", "") })

	if got := resolveRemoteServerURL(syncPushCmd); got != "https://override.example" {
		t.Fatalf("resolveRemoteServerURL = %q, want the --remote-url override", got)
	}
}

/*
sync's local side ran the checkout's headers-helper (quorum's scripts/mcp-auth-header.sh) for its

	bearer and took the workspace from the X-Workspace-Id it printed. The script is gone: the local
	bearer is the keychain token `flmnt login --local` stores under the local server URL, and the
	workspace is --local-workspace, default "quorum".
*/
func TestResolveLocalEndpointAuthenticatesFromTheKeychain(t *testing.T) {
	stubKeychain(t, freshJWT(t, "local-login"), nil)

	ep, err := resolveLocalEndpoint(syncPushCmd)
	if err != nil {
		t.Fatalf("resolveLocalEndpoint: %v", err)
	}
	if ep.Ref != defaultLocalURL || ep.Workspace != defaultLocalWorkspace {
		t.Fatalf("defaults wrong: %s / %s", ep.Ref, ep.Workspace)
	}
	if ep.GQL == nil {
		t.Fatal("expected an authenticated GraphQL client from the stored local token")
	}
}

/* No stored token and no local identity: refused with the login that records one, nothing substituted. */
func TestResolveLocalEndpointRefusesWithoutALocalLogin(t *testing.T) {
	stubKeychain(t, "", auth.ErrNotFound)
	stubLocalIdentity(t, identity.Identity{}, identity.ErrNotFound)

	_, err := resolveLocalEndpoint(syncPushCmd)
	want := "not logged in to http://localhost:8000: run `flmnt login --local --server-url http://localhost:8000`"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}

/*
No stored token but a local identity: the token is minted again from the identity on the spot — the

	device grant as flmnt-cli, self-approved over the admin listener — and stored, so a hosted login's
	sweep never leaves the local side unreachable (QRM-ADR-016 §5f).
*/
func TestResolveLocalEndpointMintsTheLocalTokenFromTheIdentityWhenTheKeychainIsEmpty(t *testing.T) {
	stubKeychain(t, "", auth.ErrNotFound)
	id := identity.Identity{Sub: testLocalSub}
	stubLocalIdentity(t, id, nil)
	admin := &adminFixture{approveStatus: 200, approveBody: `{"status":"approved"}`}
	adminURL := admin.serve(t)
	origDisc, origRun, origStore := loginDiscover, loginRunDeviceFlow, loginStoreToken
	t.Cleanup(func() { loginDiscover, loginRunDeviceFlow, loginStoreToken = origDisc, origRun, origStore })
	loginDiscover = func(string) (oauthDiscovery, error) {
		return oauthDiscovery{TokenEndpoint: "http://localhost:4010/token", DeviceAuthorizationEndpoint: "http://localhost:4010/device_authorization"}, nil
	}
	loginRunDeviceFlow = func(_ auth.DeviceConfig, prompt auth.DevicePrompter, _ func() time.Time, _ func(time.Duration)) (auth.TokenSet, error) {
		if err := prompt(auth.DeviceAuthResponse{UserCode: "LMNP-QRST"}); err != nil {
			return auth.TokenSet{}, err
		}
		return auth.TokenSet{AccessToken: freshJWT(t, "local-login")}, nil
	}
	var storedURL string
	loginStoreToken = func(u string, _ auth.TokenSet) error { storedURL = u; return nil }
	_ = syncPushCmd.Flags().Set("admin-url", adminURL)
	t.Cleanup(func() { _ = syncPushCmd.Flags().Set("admin-url", "http://127.0.0.1:4012") })
	var out bytes.Buffer
	syncPushCmd.SetOut(&out)
	t.Cleanup(func() { syncPushCmd.SetOut(nil) })

	ep, err := resolveLocalEndpoint(syncPushCmd)
	if err != nil {
		t.Fatal(err)
	}
	if ep.GQL == nil || storedURL != defaultLocalURL {
		t.Fatalf("the minted token must be stored under the local server URL and used; stored %q", storedURL)
	}
	if len(admin.approved) != 1 || admin.approved[0]["sub"] != testLocalSub || admin.approved[0]["client_id"] != id.ClientID("flmnt-cli") {
		t.Fatalf("self-approval posted %v", admin.approved)
	}
	if !strings.Contains(out.String(), "Local token minted for "+testLocalSub) {
		t.Fatalf("got %q", out.String())
	}
}

/* --take decides every collision alike; the interactive resolver asks l or c per record and stops on anything else. */
func TestCollisionResolverTakesAllOrAsksPerRecord(t *testing.T) {
	cs := []sync.Collision{
		{StreamSuffix: "domain", CorrelationID: "d1", Existing: `{"payload":{"content":"local"},"actor":{"sub":"local_a","clientId":"c"}}`, Incoming: `{"payload":{"content":"cloud"},"actor":{"sub":"u-1","clientId":"c"}}`},
		{StreamSuffix: "domain", CorrelationID: "d2", Existing: `{"payload":{"content":"x"}}`, Incoming: `{"payload":{"content":"y"}}`},
	}
	all, err := collisionResolver(syncPullCmd, "local")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := all(cs)
	if len(got) != 2 || got[0].Take != "EXISTING" || got[1].Take != "EXISTING" {
		t.Fatalf("--take local keeps every record: %v", got)
	}
	cloud, _ := collisionResolver(syncPullCmd, "cloud")
	got, _ = cloud(cs)
	if got[0].Take != "INCOMING" {
		t.Fatalf("--take cloud takes every record: %v", got)
	}
	if _, err := collisionResolver(syncPullCmd, "sideways"); err == nil {
		t.Fatal("an unknown --take is refused")
	}

	var out bytes.Buffer
	syncPullCmd.SetOut(&out)
	syncPullCmd.SetIn(strings.NewReader("c\nl\n"))
	t.Cleanup(func() { syncPullCmd.SetOut(nil); syncPullCmd.SetIn(nil) })
	ask, _ := collisionResolver(syncPullCmd, "")
	got, err = ask(cs)
	if err != nil || len(got) != 2 || got[0].Take != "INCOMING" || got[1].Take != "EXISTING" {
		t.Fatalf("answers c then l: %v %v", got, err)
	}
	for _, want := range []string{"[1/2] domain / d1", "payload", "local: {\"content\":\"local\"}", "cloud: {\"content\":\"cloud\"}", "actor", "[l/c]"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("the diff names what differs; want %q in %q", want, out.String())
		}
	}

	syncPullCmd.SetIn(strings.NewReader(""))
	ask, _ = collisionResolver(syncPullCmd, "")
	if _, err := ask(cs); err == nil || !strings.Contains(err.Error(), "--take") {
		t.Fatalf("no answer stops the pull and names --take: %v", err)
	}
}
