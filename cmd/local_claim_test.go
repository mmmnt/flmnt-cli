package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/identity"
	"github.com/mmmnt/flmnt-cli/internal/localclaim"
	"github.com/spf13/pflag"
)

// claimFixture is a fake harness edge: what it answers for GET /local/claim, and what it received on
// POST /local/claim.
type claimFixture struct {
	describeStatus int
	describeBody   string
	claimStatus    int
	claimBody      string
	claimed        []map[string]string
}

func (f *claimFixture) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/local/claim":
			w.WriteHeader(f.describeStatus)
			w.Write([]byte(f.describeBody))
		case r.Method == http.MethodPost && r.URL.Path == "/local/claim":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.claimed = append(f.claimed, body)
			w.WriteHeader(f.claimStatus)
			w.Write([]byte(f.claimBody))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func claimable() *claimFixture {
	return &claimFixture{
		describeStatus: 200,
		describeBody:   `{"client_name":"Claude","redirect_uri":"https://claude.ai/api/mcp/auth_callback"}`,
		claimStatus:    200,
		claimBody:      `{"status":"claimed"}`,
	}
}

func runLocalClaim(t *testing.T, stdin string, args ...string) (stdout string, err error) {
	t.Helper()
	localClaimCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	t.Setenv("QUORUM_SERVER_URL", "")
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetIn(strings.NewReader(stdin))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	rootCmd.SetArgs(append([]string{"local", "claim"}, args...))
	err = rootCmd.Execute()
	return out.String(), err
}

/*
A teammate's harness shows the dialer a code. `flmnt local claim <code> --server-url <their flmnt>`

	asks the harness what the code is waiting for, confirms with the person, and posts the dialer's OWN
	identity — the sub `flmnt login --local` recorded here and the client id derived for the client's
	name — so the host's approval keeps the dialer, not the host (SPEC-LOCAL-007, QRM-ADR-016 §5a).
*/
func TestLocalClaimPostsThisMachinesIdentityForTheCode(t *testing.T) {
	id := identity.Identity{Sub: testLocalSub}
	stubLocalIdentity(t, id, nil)
	stubConfig(t, auth.CLIConfig{})
	edge := claimable()
	base := edge.serve(t)

	stdout, err := runLocalClaim(t, "y\n", "BCDF-GHJK", "--server-url", base+"/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if len(edge.claimed) != 1 {
		t.Fatalf("claims posted: %d", len(edge.claimed))
	}
	got := edge.claimed[0]
	if got["code"] != "BCDF-GHJK" || got["sub"] != testLocalSub || got["client_id"] != id.ClientID("Claude") {
		t.Fatalf("posted %v; want this machine's sub and the client id derived for Claude (%s)", got, id.ClientID("Claude"))
	}
	for _, want := range []string{"Claim Claude (https://claude.ai/api/mcp/auth_callback)", testLocalSub, "[y/N]", "Claimed.", "flmnt local approve BCDF-GHJK"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

/* No identity here, or no server named: refused before anything is asked of the harness. */
func TestLocalClaimRefusesWithoutAnIdentityOrAServer(t *testing.T) {
	stubLocalIdentity(t, identity.Identity{}, identity.ErrNotFound)
	stubConfig(t, auth.CLIConfig{})
	edge := &claimFixture{}
	base := edge.serve(t)
	t.Chdir(t.TempDir())
	if _, err := runLocalClaim(t, "y\n", "BCDF-GHJK", "--server-url", base); err == nil || !strings.Contains(err.Error(), "flmnt login --local") {
		t.Fatalf("without an identity: %v", err)
	}

	stubLocalIdentity(t, identity.Identity{Sub: testLocalSub}, nil)
	if _, err := runLocalClaim(t, "y\n", "BCDF-GHJK"); err == nil || !strings.Contains(err.Error(), "--server-url") {
		t.Fatalf("without a server: %v", err)
	}
	if len(edge.claimed) != 0 {
		t.Fatal("nothing may be posted")
	}
}

/* A "no" claims nothing; the broker's refusals are the answer, verbatim. */
func TestLocalClaimDeclinesAndCarriesRefusals(t *testing.T) {
	stubLocalIdentity(t, identity.Identity{Sub: testLocalSub}, nil)
	stubConfig(t, auth.CLIConfig{})
	edge := claimable()
	base := edge.serve(t)

	stdout, err := runLocalClaim(t, "n\n", "BCDF-GHJK", "--server-url", base)
	if err != nil || !strings.Contains(stdout, "Nothing claimed.") || len(edge.claimed) != 0 {
		t.Fatalf("err %v stdout %q posted %d", err, stdout, len(edge.claimed))
	}

	edge.claimStatus, edge.claimBody = 409, `{"code":"LOCAL_CLAIM_ALREADY_MADE","message":"That code was already claimed by another identity."}`
	_, err = runLocalClaim(t, "", "BCDF-GHJK", "--server-url", base, "--yes")
	var ce *localclaim.Error
	if !errors.As(err, &ce) || err.Error() != "LOCAL_CLAIM_ALREADY_MADE: That code was already claimed by another identity." {
		t.Fatalf("want the broker's refusal verbatim, got %v", err)
	}

	edge.describeStatus, edge.describeBody = 404, `{"code":"LOCAL_APPROVAL_UNKNOWN","message":"No approval or device holds that code."}`
	if _, err := runLocalClaim(t, "", "ZZZZ-ZZZZ", "--server-url", base, "--yes"); err == nil || !strings.Contains(err.Error(), "LOCAL_APPROVAL_UNKNOWN") {
		t.Fatalf("got %v", err)
	}
}
