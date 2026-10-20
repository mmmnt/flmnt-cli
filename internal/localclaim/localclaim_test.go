package localclaim

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/localadmin"
)

/*
`flmnt local claim <code>` asks the harness, through its public edge, what the code is waiting for

	(SPEC-LOCAL-007): the client's name and redirect URI, nothing else. The server URL may carry a
	path (the MCP's /mcp); the claim routes live at the origin.
*/
func TestDescribeAsksTheOriginAndReadsTheClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/local/claim" || r.URL.Query().Get("code") != "BCDF-GHJK" {
			t.Errorf("%s %s", r.Method, r.URL.String())
		}
		w.Write([]byte(`{"client_name":"Claude","redirect_uri":"https://claude.ai/api/mcp/auth_callback"}`))
	}))
	t.Cleanup(srv.Close)

	got, err := Describe(srv.URL+"/mcp", "BCDF-GHJK")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Authorization{ClientName: "Claude", RedirectURI: "https://claude.ai/api/mcp/auth_callback"}) {
		t.Fatalf("got %+v", got)
	}
}

/* The claim posts exactly {code, sub, client_id} and is a claim only when the broker says so. */
func TestClaimPostsTheIdentityAndCarriesRefusalsVerbatim(t *testing.T) {
	var posted map[string]string
	status, body := 200, `{"status":"claimed"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/local/claim" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&posted)
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	if err := Claim(srv.URL, "BCDF-GHJK", "local_9e1c2b3a-4d5e-5f60-8a7b-1c2d3e4f5a6b", "local_7a6b5c4d-3e2f-5a1b-8c9d-0e1f2a3b4c5d"); err != nil {
		t.Fatal(err)
	}
	if len(posted) != 3 || posted["code"] != "BCDF-GHJK" || posted["sub"] != "local_9e1c2b3a-4d5e-5f60-8a7b-1c2d3e4f5a6b" || posted["client_id"] != "local_7a6b5c4d-3e2f-5a1b-8c9d-0e1f2a3b4c5d" {
		t.Fatalf("posted %v", posted)
	}

	status, body = 409, `{"code":"LOCAL_CLAIM_ALREADY_MADE","message":"That code was already claimed by another identity."}`
	err := Claim(srv.URL, "BCDF-GHJK", "local_9e1c2b3a-4d5e-5f60-8a7b-1c2d3e4f5a6b", "local_7a6b5c4d-3e2f-5a1b-8c9d-0e1f2a3b4c5d")
	var ae *localadmin.Error
	if !errors.As(err, &ae) || ae.Status != 409 || ae.Code != "LOCAL_CLAIM_ALREADY_MADE" || err.Error() != "LOCAL_CLAIM_ALREADY_MADE: That code was already claimed by another identity." {
		t.Fatalf("want the broker's refusal verbatim, got %T %v", err, err)
	}

	status, body = 200, `{"status":"pending"}`
	if err := Claim(srv.URL, "BCDF-GHJK", "local_9e1c2b3a-4d5e-5f60-8a7b-1c2d3e4f5a6b", "local_7a6b5c4d-3e2f-5a1b-8c9d-0e1f2a3b4c5d"); err == nil || !errors.As(err, &ae) || ae.Code != "LOCAL_CLAIM_NOT_MADE" {
		t.Fatalf("an answer that is not claimed is not a claim: %v", err)
	}
}
