package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/identity"
	"github.com/mmmnt/flmnt-cli/internal/sync"
	"github.com/spf13/cobra"
)

func stubLinkRemote(t *testing.T, ep sync.Endpoint, claims auth.Claims, err error) {
	t.Helper()
	orig := linkResolveRemote
	linkResolveRemote = func(*cobra.Command) (sync.Endpoint, auth.Claims, error) { return ep, claims, err }
	t.Cleanup(func() { linkResolveRemote = orig })
}

func runLink(t *testing.T) (string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs([]string{"link"})
	err := rootCmd.Execute()
	return out.String(), err
}

/*
`flmnt link` links this machine's identity and the subs it replaced to the hosted account it is logged

	into (QRM-ADR-016 §5d, SPEC-LINK-001): one mutation under the hosted token, and the person reads
	what was linked now and what the account already held.
*/
func TestLinkLinksTheIdentityAndItsHistoryToTheHostedAccount(t *testing.T) {
	stubLocalIdentity(t, identity.Identity{Sub: "local_a", History: []identity.Replaced{{Sub: "local_old"}}}, nil)
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(raw, &req)
		got = req.Variables
		_, _ = io.WriteString(w, `{"data":{"linkLocalIdentity":{"linked":["local_a"],"held":["local_old"]}}}`)
	}))
	defer srv.Close()
	stubLinkRemote(t, sync.Endpoint{GQL: apiclient.New(srv.URL, "hosted"), Workspace: "ws-cloud", Ref: srv.URL}, auth.Claims{Sub: "u-mike", Email: "mike@example.com"}, nil)

	stdout, err := runLink(t)
	if err != nil {
		t.Fatal(err)
	}
	subs, _ := got["localSubs"].([]any)
	if len(subs) != 2 || subs[0] != "local_a" || subs[1] != "local_old" {
		t.Fatalf("the identity and its history are sent: %v", got)
	}
	for _, want := range []string{"Linked local_a to mike@example.com (u-mike).", "Already linked to mike@example.com (u-mike): local_old."} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

// Without a local identity there is nothing to link; without a hosted login there is nobody to link it to.
func TestLinkRefusesWithoutBothLogins(t *testing.T) {
	stubLocalIdentity(t, identity.Identity{}, errors.New("no identity"))
	if _, err := runLink(t); err == nil || !strings.Contains(err.Error(), "flmnt login --local") {
		t.Fatalf("want a refusal naming the local login, got %v", err)
	}
	stubLocalIdentity(t, identity.Identity{Sub: "local_a"}, nil)
	stubLinkRemote(t, sync.Endpoint{}, auth.Claims{}, errors.New("not logged in to https://mcp (run `flmnt login`)"))
	if _, err := runLink(t); err == nil || !strings.Contains(err.Error(), "flmnt login") {
		t.Fatalf("want a refusal naming the hosted login, got %v", err)
	}
}

// The server's refusal reaches the person as it was sent.
func TestLinkSurfacesARefusedSub(t *testing.T) {
	stubLocalIdentity(t, identity.Identity{Sub: "local_a"}, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[{"message":"LINK_SUB_TAKEN: local_a is linked to another account; one hosted account per identity, ever"}]}`)
	}))
	defer srv.Close()
	stubLinkRemote(t, sync.Endpoint{GQL: apiclient.New(srv.URL, "hosted"), Workspace: "ws-cloud", Ref: srv.URL}, auth.Claims{Sub: "u-mike"}, nil)
	if _, err := runLink(t); err == nil || !strings.Contains(err.Error(), "LINK_SUB_TAKEN") {
		t.Fatalf("want the refusal, got %v", err)
	}
}
