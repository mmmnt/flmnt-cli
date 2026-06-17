package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Claims struct {
	Sub      string `json:"sub"`
	Email    string `json:"email"`
	Username string `json:"cognito:username"`
	Exp      int64  `json:"exp"`
}

func tokenExpiry(jwt string) (time.Time, bool) {
	c, err := DecodeUnverified(jwt)
	if err != nil || c.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(c.Exp, 0), true
}

func DecodeUnverified(jwt string) (Claims, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return Claims{}, errors.New("jwt: malformed")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return Claims{}, err
		}
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return Claims{}, err
	}
	return c, nil
}
