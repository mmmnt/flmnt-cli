package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
)

// reportNotConnected says FLMNT_NOT_CONNECTED on w when err is a request nothing answered (PRD §14.12
// Q1, §14.17 item 8), naming the target it tried, this machine or the host's url, and what fixes it.
// It reports whether it did, so a hook can still exit cleanly: the code is said, never raised.
//
// It says the same code when something answered at the endpoint but not as flmnt's GraphQL router (an
// HTTP error status, a body that is not GraphQL): that is a wrong endpoint, which no hook can work
// past either. An HTTP 401 or 403 is the endpoint refusing the CLI's credentials, not a wrong
// endpoint: the same code, with the hint to sign in again (ruling 5c53b6f5). A GraphQL refusal in an
// answer from the router is not reported here.
func reportNotConnected(w io.Writer, err error) bool {
	if werr, ok := apiclient.AnsweredWrongly(err); ok {
		if werr.StatusCode == http.StatusUnauthorized || werr.StatusCode == http.StatusForbidden {
			fmt.Fprintln(w, signInAgainLine(werr))
			return true
		}
		fmt.Fprintln(w, wrongEndpointLine(werr))
		return true
	}
	if !apiclient.IsUnreachable(err) {
		return false
	}
	fmt.Fprintln(w, notConnectedLine(err))
	return true
}

// signInAgainLine names the endpoint and the status, and the login that refreshes the token the CLI
// sent: on loopback the CLI's own harness token comes from `flmnt login --local`, elsewhere from
// `flmnt login`.
func signInAgainLine(werr *apiclient.WrongAnswerError) string {
	login := "`flmnt login`"
	if httpx.IsLoopback(werr.Endpoint) {
		login = "`flmnt login --local`"
	}
	return fmt.Sprintf("FLMNT_NOT_CONNECTED: %s refused the CLI's credentials (%s); sign in again with %s", werr.Endpoint, werr.Detail(), login)
}

func wrongEndpointLine(werr *apiclient.WrongAnswerError) string {
	return fmt.Sprintf("FLMNT_NOT_CONNECTED: %s answered, but not as flmnt's GraphQL endpoint (%s); declare local_graphql_endpoint in .flmnt.json, or check the server URL (--server-url, FLMNT_SERVER_URL or `flmnt login`)", werr.Endpoint, werr.Detail())
}

func notConnectedLine(err error) string {
	target := ""
	var uerr *url.Error
	if errors.As(err, &uerr) {
		target = uerr.URL
	}
	if httpx.IsLoopback(target) {
		return fmt.Sprintf("FLMNT_NOT_CONNECTED: no flmnt answered on this machine (%s); start it with `flmnt local up`", target)
	}
	return fmt.Sprintf("FLMNT_NOT_CONNECTED: no flmnt answered at %s; check the server URL (--server-url, FLMNT_SERVER_URL or `flmnt login`) and your connection", target)
}
