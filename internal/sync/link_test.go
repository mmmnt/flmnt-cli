package sync

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

/*
`flmnt link` sends this machine's identity and the subs it replaced as one linkLocalIdentity mutation

	(SPEC-LINK-001) and reads back which subs were linked now and which the account already held.
*/
func TestLinkSendsTheIdentityAndReadsTheReport(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(raw, &req)
		got = req.Variables
		_, _ = io.WriteString(w, `{"data":{"linkLocalIdentity":{"linked":["local_a"],"held":["local_old"]}}}`)
	}))
	defer srv.Close()

	report, err := Link(Endpoint{GQL: apiclient.New(srv.URL, "t"), Workspace: "ws", Ref: srv.URL}, []string{"local_a", "local_old"})
	if err != nil {
		t.Fatal(err)
	}
	subs, _ := got["localSubs"].([]any)
	if len(subs) != 2 || subs[0] != "local_a" || subs[1] != "local_old" {
		t.Fatalf("localSubs sent: %v", got)
	}
	if len(report.Linked) != 1 || report.Linked[0] != "local_a" || len(report.Held) != 1 || report.Held[0] != "local_old" {
		t.Fatalf("report: %+v", report)
	}
}

// A refusal is the server's sentence (the spec's code), surfaced as the error.
func TestLinkSurfacesTheServersRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[{"message":"LINK_SUB_TAKEN: local_a is linked to another account; one hosted account per identity, ever"}]}`)
	}))
	defer srv.Close()
	_, err := Link(Endpoint{GQL: apiclient.New(srv.URL, "t"), Workspace: "ws", Ref: srv.URL}, []string{"local_a"})
	if err == nil || !strings.Contains(err.Error(), "LINK_SUB_TAKEN") {
		t.Fatalf("want the refusal code in the error, got %v", err)
	}
}
