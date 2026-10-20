package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/identity"
	"github.com/spf13/pflag"
)

const testLocalSub = "local_34f68591-fdb4-5336-aa6a-d772cca9bd8a"

// stubLocalIdentity makes the local login the given identity (or its absence).
func stubLocalIdentity(t *testing.T, id identity.Identity, err error) {
	t.Helper()
	orig := identityLoad
	identityLoad = func() (identity.Identity, error) { return id, err }
	t.Cleanup(func() { identityLoad = orig })
}

// adminFixture is a fake admin listener: what it answers for GET /local/pending/<code>, and what it
// received on POST /local/approve.
type adminFixture struct {
	pendingStatus int
	pendingBody   string
	approveStatus int
	approveBody   string
	approved      []map[string]string
}

func (f *adminFixture) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/local/pending/"):
			w.WriteHeader(f.pendingStatus)
			w.Write([]byte(f.pendingBody))
		case r.Method == http.MethodPost && r.URL.Path == "/local/approve":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.approved = append(f.approved, body)
			w.WriteHeader(f.approveStatus)
			w.Write([]byte(f.approveBody))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func pendingAuthorization() *adminFixture {
	return &adminFixture{
		pendingStatus: 200,
		pendingBody:   `{"kind":"authorization","client_id":"dcr-claude","client_name":"Claude","redirect_uri":"https://claude.ai/api/mcp/auth_callback"}`,
		approveStatus: 200,
		approveBody:   `{"status":"approved"}`,
	}
}

func runLocalApprove(t *testing.T, stdin string, args ...string) (stdout string, err error) {
	t.Helper()
	localApproveCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetIn(strings.NewReader(stdin))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	rootCmd.SetArgs(append([]string{"local", "approve"}, args...))
	err = rootCmd.Execute()
	return out.String(), err
}

/*
A client (claude.ai, Claude Code) that hits the local broker's /authorize is shown a code and told to

	run `flmnt local approve <code>`. The CLI supplies the identity: the local login's sub and the
	client id it derives for the client's NAME, so the entries that client writes are attributed to
	it (QRM-ADR-014 §2, QRM-ADR-015 §1). The person is told what they are approving.
*/
func TestLocalApproveApprovesAnAuthorizationForTheDerivedClient(t *testing.T) {
	id := identity.Identity{Sub: testLocalSub}
	stubLocalIdentity(t, id, nil)
	admin := pendingAuthorization()
	base := admin.serve(t)

	stdout, err := runLocalApprove(t, "", "BCDF-GHJK", "--admin-url", base, "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if len(admin.approved) != 1 {
		t.Fatalf("approvals posted: %d", len(admin.approved))
	}
	got := admin.approved[0]
	if got["code"] != "BCDF-GHJK" || got["sub"] != testLocalSub || got["client_id"] != id.ClientID("Claude") {
		t.Fatalf("posted %v; want the sub and the client id derived for %q (%s)", got, "Claude", id.ClientID("Claude"))
	}
	for _, want := range []string{"Claude", "https://claude.ai/api/mcp/auth_callback", testLocalSub, "Approved. Return to your browser."} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

/* The person confirms: anything but y/yes aborts, and an aborted approval posts nothing. */
func TestLocalApproveAbortsOnNoWithoutPosting(t *testing.T) {
	stubLocalIdentity(t, identity.Identity{Sub: testLocalSub}, nil)
	admin := pendingAuthorization()
	base := admin.serve(t)

	stdout, err := runLocalApprove(t, "n\n", "BCDF-GHJK", "--admin-url", base)
	if err != nil {
		t.Fatal(err)
	}
	if len(admin.approved) != 0 {
		t.Fatalf("an aborted approval must post nothing; posted %v", admin.approved)
	}
	if !strings.Contains(stdout, "[y/N]") || !strings.Contains(stdout, "Nothing approved.") {
		t.Fatalf("stdout: %q", stdout)
	}
	if strings.Contains(stdout, "Approved.") {
		t.Fatalf("an aborted approval must not say approved: %q", stdout)
	}
}

func TestLocalApproveProceedsOnYes(t *testing.T) {
	stubLocalIdentity(t, identity.Identity{Sub: testLocalSub}, nil)
	admin := pendingAuthorization()
	base := admin.serve(t)

	if _, err := runLocalApprove(t, "y\n", "BCDF-GHJK", "--admin-url", base); err != nil {
		t.Fatal(err)
	}
	if len(admin.approved) != 1 {
		t.Fatalf("approvals posted: %d", len(admin.approved))
	}
}

/* --yes is the answer: stdin is never read, so a closed or empty stdin still approves. */
func TestLocalApproveYesSkipsThePrompt(t *testing.T) {
	stubLocalIdentity(t, identity.Identity{Sub: testLocalSub}, nil)
	admin := pendingAuthorization()
	base := admin.serve(t)

	stdout, err := runLocalApprove(t, "n\n", "BCDF-GHJK", "--admin-url", base, "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if len(admin.approved) != 1 {
		t.Fatalf("--yes must approve regardless of stdin; posted %d", len(admin.approved))
	}
	if strings.Contains(stdout, "[y/N]") {
		t.Fatalf("--yes must not ask: %q", stdout)
	}
}

/*
A device user code is the CLI's own sign-in (`flmnt login --local` runs the device grant as

	`flmnt-cli`), so the client id is the one derived for the CLI tool — not for the broker's device
	client id — and the question names this machine's CLI, not a redirect.
*/
func TestLocalApproveApprovesADeviceCodeForTheCLITool(t *testing.T) {
	id := identity.Identity{Sub: testLocalSub}
	stubLocalIdentity(t, id, nil)
	admin := &adminFixture{pendingStatus: 200, pendingBody: `{"kind":"device","client_id":"flmnt-cli"}`, approveStatus: 200, approveBody: `{"status":"approved"}`}
	base := admin.serve(t)

	stdout, err := runLocalApprove(t, "y\n", "LMNP-QRST", "--admin-url", base)
	if err != nil {
		t.Fatal(err)
	}
	if len(admin.approved) != 1 || admin.approved[0]["client_id"] != id.ClientID("flmnt-cli") || admin.approved[0]["sub"] != testLocalSub || admin.approved[0]["code"] != "LMNP-QRST" {
		t.Fatalf("posted %v; want the client id derived for flmnt-cli (%s)", admin.approved, id.ClientID("flmnt-cli"))
	}
	if !strings.Contains(stdout, "Approve this machine's flmnt CLI device sign-in?") {
		t.Fatalf("stdout: %q", stdout)
	}
	if !strings.Contains(stdout, "Approved.") || strings.Contains(stdout, "Return to your browser") {
		t.Fatalf("a device approval has no browser to return to: %q", stdout)
	}
}
