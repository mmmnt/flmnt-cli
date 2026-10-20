// Package localclaim is the CLI's client for the local broker's PUBLIC claim routes (SPEC-LOCAL-007,
// QRM-ADR-016 §5a), reached through the harness's edge like every other public route: a dialer on
// another machine binds their own identity to a pending approval code before the host approves it.
// Refusals are carried verbatim, as the admin client carries them.
package localclaim

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/httpx"
	"github.com/mmmnt/flmnt-cli/internal/localadmin"
)

// Authorization is what a code is waiting for, as the dialer may see it: the client's registered
// name and redirect URI, the words the host's prompt shows. Nothing else is told.
type Authorization struct {
	ClientName  string `json:"client_name"`
	RedirectURI string `json:"redirect_uri"`
}

func originOf(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}

// Describe asks GET <origin>/local/claim?code=<code>.
func Describe(serverURL, code string) (Authorization, error) {
	origin, err := originOf(serverURL)
	if err != nil {
		return Authorization{}, err
	}
	resp, err := httpx.Client.Get(origin + "/local/claim?code=" + url.QueryEscape(code))
	if err != nil {
		return Authorization{}, err
	}
	var a Authorization
	if err := localadmin.Decode(resp, &a); err != nil {
		return Authorization{}, err
	}
	return a, nil
}

// Claim asks POST <origin>/local/claim {code, sub, client_id}: the broker records the dialer's identity
// on the pending approval, which the host's approval then keeps. An answer that is not
// {status: "claimed"} is not a claim.
func Claim(serverURL, code, sub, clientID string) error {
	origin, err := originOf(serverURL)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"code": code, "sub": sub, "client_id": clientID})
	if err != nil {
		return err
	}
	resp, err := httpx.Client.Post(origin+"/local/claim", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := localadmin.Decode(resp, &out); err != nil {
		return err
	}
	if out.Status != "claimed" {
		return &localadmin.Error{Status: resp.StatusCode, Code: "LOCAL_CLAIM_NOT_MADE", Message: "the broker answered status " + strings.TrimSpace(out.Status) + ", not claimed"}
	}
	return nil
}
