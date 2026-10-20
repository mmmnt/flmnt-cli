package health

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config names what to probe. ServerURL is the flmnt MCP the session talks to — the harness on
// localhost or the hosted one — and is probed at its origin's /health. Core, Engine and Proxy describe
// the host-port compose of the older local stack and are probed only when declared; the harness
// publishes none of them, so an undeclared one is not a service that is down.
type Config struct {
	ServerURL string
	CoreURL   string
	EngineURL string
	ProxyURL  string
}

type Result struct {
	Service string
	OK      bool
	Message string
}

var client = &http.Client{Timeout: 5 * time.Second}

func Check(cfg Config) []Result {
	var results []Result
	for _, p := range []struct{ service, base string }{
		{"mcp", origin(cfg.ServerURL)},
		{"core", cfg.CoreURL},
		{"engine", cfg.EngineURL},
		{"proxy", cfg.ProxyURL},
	} {
		if p.base == "" {
			continue
		}
		results = append(results, probe(p.service, strings.TrimRight(p.base, "/")+"/health"))
	}
	return results
}

// origin strips the MCP path (/mcp) so the probe lands on the server's /health.
func origin(serverURL string) string {
	if serverURL == "" {
		return ""
	}
	u, err := url.Parse(serverURL)
	if err != nil || u.Host == "" {
		return serverURL
	}
	return u.Scheme + "://" + u.Host
}

func probe(service, url string) Result {
	resp, err := client.Get(url)
	if err != nil {
		return Result{Service: service, OK: false, Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{Service: service, OK: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return Result{Service: service, OK: true, Message: "ok"}
}
