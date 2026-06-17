package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PKCEConfig struct {
	AuthURL     string
	TokenURL    string
	ClientID    string
	RedirectURI string
}

type pkceFlow struct {
	cfg          PKCEConfig
	codeVerifier string
	authCode     chan string
}

func RunPKCEFlow(cfg PKCEConfig, openBrowser func(string) error) (TokenSet, error) {
	verifier, err := generateCodeVerifier()
	if err != nil {
		return TokenSet{}, err
	}
	challenge := codeChallenge(verifier)

	f := &pkceFlow{cfg: cfg, codeVerifier: verifier, authCode: make(chan string, 1)}

	listener, err := net.Listen("tcp", "127.0.0.1:9877")
	if err != nil {
		return TokenSet{}, fmt.Errorf("cannot open callback listener: %w", err)
	}

	srv := &http.Server{Handler: f}
	go srv.Serve(listener)
	defer srv.Shutdown(context.Background())

	authURL := buildAuthURL(cfg, challenge)
	if err := openBrowser(authURL); err != nil {
		return TokenSet{}, fmt.Errorf("cannot open browser: %w", err)
	}

	select {
	case code := <-f.authCode:
		return exchangeCode(cfg, code, verifier)
	case <-time.After(5 * time.Minute):
		return TokenSet{}, fmt.Errorf("login timed out waiting for browser callback")
	}
}

func (f *pkceFlow) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	fmt.Fprint(w, "Login successful — you may close this tab.")
	f.authCode <- code
}

func buildAuthURL(cfg PKCEConfig, challenge string) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {cfg.RedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return cfg.AuthURL + "?" + params.Encode()
}

func exchangeCode(cfg PKCEConfig, code, verifier string) (TokenSet, error) {
	body := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {cfg.ClientID},
		"code":          {code},
		"redirect_uri":  {cfg.RedirectURI},
		"code_verifier": {verifier},
	}
	resp, err := http.Post(cfg.TokenURL, "application/x-www-form-urlencoded",
		strings.NewReader(body.Encode()))
	if err != nil {
		return TokenSet{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return TokenSet{}, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, raw)
	}
	var t TokenSet
	if err := json.Unmarshal(raw, &t); err != nil {
		return TokenSet{}, err
	}
	return t, nil
}

func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func codeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
