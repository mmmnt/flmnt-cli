package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/httpx"
)

var ErrRefreshExpired = errors.New("refresh token expired or revoked; run `flmnt login`")

func RefreshAccessToken(tokenURL, clientID, refreshToken string) (TokenSet, error) {
	body := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	}
	resp, err := httpx.Client.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(body.Encode()))
	if err != nil {
		return TokenSet{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return TokenSet{}, ErrRefreshExpired
	}
	var t TokenSet
	if err := json.Unmarshal(raw, &t); err != nil {
		return TokenSet{}, err
	}
	if t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}
	return t, nil
}

func EnsureFreshAccessToken(tokens TokenSet, tokenURL, clientID string, threshold time.Duration, now time.Time) (TokenSet, bool, error) {
	if exp, ok := tokenExpiry(tokens.AccessToken); ok && exp.Sub(now) >= threshold {
		return tokens, false, nil
	}
	refreshed, err := RefreshAccessToken(tokenURL, clientID, tokens.RefreshToken)
	if err != nil {
		return TokenSet{}, false, err
	}
	return refreshed, true, nil
}
