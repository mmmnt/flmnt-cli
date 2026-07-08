package auth

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/httpx"
)

func RevokeRefreshToken(revokeURL, clientID, refreshToken string) error {
	if revokeURL == "" || refreshToken == "" {
		return nil
	}
	body := url.Values{
		"token":     {refreshToken},
		"client_id": {clientID},
	}
	resp, err := httpx.Client.Post(revokeURL, "application/x-www-form-urlencoded", strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("revoke failed (%d): %s", resp.StatusCode, raw)
}
