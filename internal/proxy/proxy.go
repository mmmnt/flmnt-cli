package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type Config struct {
	TargetURL    string
	TokenFetcher func() (string, error)
}

type Handler struct {
	cfg      Config
	target   *url.URL
	reverseP *httputil.ReverseProxy
}

func New(cfg Config) (*Handler, error) {
	target, err := url.Parse(cfg.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy target URL %q: %w", cfg.TargetURL, err)
	}
	rp := httputil.NewSingleHostReverseProxy(target)
	return &Handler{cfg: cfg, target: target, reverseP: rp}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		h.health(w, r)
		return
	}

	token, err := h.cfg.TokenFetcher()
	if err != nil || token == "" {
		http.Error(w, "unauthenticated: no token available", http.StatusUnauthorized)
		return
	}

	r = r.Clone(r.Context())
	r.Host = h.target.Host
	r.Header.Set("Authorization", "Bearer "+token)
	h.reverseP.ServeHTTP(w, r)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	token, _ := h.cfg.TokenFetcher()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","authenticated":%v}`, token != "")
}

// ListenAddr binds the proxy to loopback only. Because the proxy injects the user's bearer token on
// every forwarded request, it must never be reachable off the local host.
func ListenAddr(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// newServer builds the proxy's HTTP server with a ReadHeaderTimeout so a slow client cannot hold a
// connection open indefinitely (Slowloris).
func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func ListenAndServe(addr string, cfg Config) error {
	h, err := New(cfg)
	if err != nil {
		return err
	}
	return newServer(addr, h).ListenAndServe()
}
