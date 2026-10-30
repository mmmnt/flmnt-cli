package apiclient

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// IsUnreachable says whether a request failed because nothing answered it: the connection was
// refused, the host did not resolve, or the request timed out before a response. The HTTP client
// reports every such failure as a *url.Error; a server that answered, with an HTTP error status or a
// GraphQL error, is reachable and is not this case.
func IsUnreachable(err error) bool {
	var uerr *url.Error
	return errors.As(err, &uerr)
}

// WrongAnswerError is a request that something answered, but not as a GraphQL router does: an HTTP
// status other than 200, or a body that is not a GraphQL response (an HTML page, another service's
// JSON). It is what a wrong endpoint looks like, e.g. the MCP's /mcp/graphql when a checkout declares no
// local_graphql_endpoint. A GraphQL response carrying errors is the router answering, and is not this.
type WrongAnswerError struct {
	Endpoint    string
	StatusCode  int
	ContentType string
	Body        string
	NotGraphQL  bool // the status was 200 but the body was not a GraphQL response
}

// Detail is the error as one short phrase: the status, or that the body was not GraphQL, and the start
// of what came back.
func (e *WrongAnswerError) Detail() string {
	what := fmt.Sprintf("HTTP %d", e.StatusCode)
	if e.NotGraphQL {
		what = "not a GraphQL response"
		if e.ContentType != "" {
			what += " (" + e.ContentType + ")"
		}
	}
	if body := snippet(e.Body); body != "" {
		what += ": " + body
	}
	return what
}

func (e *WrongAnswerError) Error() string {
	if e.NotGraphQL {
		return "invalid graphql response from " + e.Endpoint + ": " + e.Detail()
	}
	return fmt.Sprintf("graphql HTTP %d: %s", e.StatusCode, e.Body)
}

// AnsweredWrongly says whether err is a request answered by something that is not a GraphQL router.
func AnsweredWrongly(err error) (*WrongAnswerError, bool) {
	var werr *WrongAnswerError
	if errors.As(err, &werr) {
		return werr, true
	}
	return nil, false
}

// snippet is the start of a body on one line, so a page of HTML never floods a hook's stderr.
func snippet(body string) string {
	s := strings.Join(strings.Fields(body), " ")
	if r := []rune(s); len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return s
}
