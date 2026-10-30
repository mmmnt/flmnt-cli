package cmd

import (
	"errors"
	"fmt"
	"io"
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
// past either. A GraphQL refusal in an answer from the router is not reported here.
func reportNotConnected(w io.Writer, err error) bool {
	if werr, ok := apiclient.AnsweredWrongly(err); ok {
		fmt.Fprintln(w, wrongEndpointLine(werr))
		return true
	}
	if !apiclient.IsUnreachable(err) {
		return false
	}
	fmt.Fprintln(w, notConnectedLine(err))
	return true
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
