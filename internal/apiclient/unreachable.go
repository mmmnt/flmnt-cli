package apiclient

import (
	"errors"
	"net/url"
)

// IsUnreachable says whether a request failed because nothing answered it: the connection was
// refused, the host did not resolve, or the request timed out before a response. The HTTP client
// reports every such failure as a *url.Error; a server that answered, with an HTTP error status or a
// GraphQL error, is reachable and is not this case.
func IsUnreachable(err error) bool {
	var uerr *url.Error
	return errors.As(err, &uerr)
}
