// Package httpx provides the shared HTTP client for the CLI's auth + API calls.
package httpx

import (
	"net/http"
	"net/url"
	"time"
)

// Client is the shared HTTP client for every auth and API call. The 10s timeout stops a black-holed
// endpoint (SYN drop, hung load balancer) from blocking a command forever — these calls also run
// inside Claude Code hooks on every prompt, so an unbounded wait would freeze the editor session.
var Client = &http.Client{Timeout: 10 * time.Second}

// IsLoopback says whether a server URL names this machine — the local harness rather than a hosted
// flmnt. Credentials for a loopback server come from the checkout's headers-helper; for anything else
// the keychain `flmnt login` filled is consulted first.
func IsLoopback(serverURL string) bool {
	u, err := url.Parse(serverURL)
	if err != nil {
		return false
	}
	return LoopbackHost(u.Hostname())
}

func LoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
