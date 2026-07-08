// Package httpx provides the shared HTTP client for the CLI's auth + API calls.
package httpx

import (
	"net/http"
	"time"
)

// Client is the shared HTTP client for every auth and API call. The 10s timeout stops a black-holed
// endpoint (SYN drop, hung load balancer) from blocking a command forever — these calls also run
// inside Claude Code hooks on every prompt, so an unbounded wait would freeze the editor session.
var Client = &http.Client{Timeout: 10 * time.Second}
