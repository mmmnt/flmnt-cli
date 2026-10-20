// Package localadmin is the CLI's client for the local broker's ADMIN listener — the loopback-only
// port (default http://127.0.0.1:4012) the harness publishes to 127.0.0.1 and the edge never routes
// (QRM-ADR-015 §3). Reaching it is the proof that the caller is this machine; the CLI supplies the
// identity the broker approves for. A refusal is carried verbatim: the broker's code and message are
// the answer, nothing is substituted.
package localadmin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/httpx"
)

// DefaultBaseURL is where the harness publishes the admin listener (BROKER_LOCAL_ADMIN_PORT).
const DefaultBaseURL = "http://127.0.0.1:4012"

// Error is a refusal the broker answered under its contract: {code, message} with the HTTP status.
type Error struct {
	Status  int
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Approval is what a code is waiting for (SPEC-LOCAL-002): an authorization names the client and its
// redirect URI, a device code names the device's client id. It never carries a sub.
type Approval struct {
	Kind        string `json:"kind"`
	ClientID    string `json:"client_id"`
	ClientName  string `json:"client_name"`
	RedirectURI string `json:"redirect_uri"`
}

// Pending asks GET /local/pending/<code>.
func Pending(baseURL, code string) (Approval, error) {
	resp, err := httpx.Client.Get(strings.TrimRight(baseURL, "/") + "/local/pending/" + url.PathEscape(code))
	if err != nil {
		return Approval{}, err
	}
	var p Approval
	if err := decode(resp, &p); err != nil {
		return Approval{}, err
	}
	return p, nil
}

// decode reads a 2xx body into out, or turns the broker's {code, message} refusal into an *Error.
// A body that is not the contract's is reported with its status and text so a proxy page or a crash
// is never mistaken for a refusal.
func decode(resp *http.Response, out any) error {
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("admin listener answered %d with a body that is not JSON: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		}
		return nil
	}
	var e Error
	if err := json.Unmarshal(raw, &e); err != nil || e.Code == "" {
		return fmt.Errorf("admin listener answered %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	e.Status = resp.StatusCode
	return &e
}

// Approve asks POST /local/approve {code, sub, client_id}: the broker approves the code for the
// identity the CLI supplies — the local login's sub and the client id derived for the tool
// (SPEC-LOCAL-003). An answer that is not {status: "approved"} is not an approval.
func Approve(baseURL, code, sub, clientID string) error {
	body, err := json.Marshal(map[string]string{"code": code, "sub": sub, "client_id": clientID})
	if err != nil {
		return err
	}
	resp, err := httpx.Client.Post(strings.TrimRight(baseURL, "/")+"/local/approve", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := decode(resp, &out); err != nil {
		return err
	}
	if out.Status != "approved" {
		return fmt.Errorf("admin listener answered status %q, not approved", out.Status)
	}
	return nil
}
