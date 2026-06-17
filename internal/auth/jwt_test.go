package auth

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func TestDecodeUnverifiedExtractsEmailAndSub(t *testing.T) {
	payload := `{"sub":"abc-123","email":"mike@flmnt.ai","cognito:username":"mike"}`
	jwt := "header." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
	c, err := DecodeUnverified(jwt)
	if err != nil {
		t.Fatalf("DecodeUnverified: %v", err)
	}
	if c.Email != "mike@flmnt.ai" || c.Sub != "abc-123" || c.Username != "mike" {
		t.Fatalf("unexpected claims: %+v", c)
	}
}

func TestDecodeUnverifiedRejectsMalformed(t *testing.T) {
	if _, err := DecodeUnverified("not-a-jwt"); err == nil {
		t.Fatal("expected error")
	}
}

func TestTokenExpiryReturnsExpClaim(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	payload := fmt.Sprintf(`{"sub":"s","exp":%d}`, exp)
	jwt := "h." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
	got, ok := tokenExpiry(jwt)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Unix() != exp {
		t.Fatalf("exp mismatch: got %d want %d", got.Unix(), exp)
	}
}

func TestTokenExpiryFalseOnMalformed(t *testing.T) {
	if _, ok := tokenExpiry("nope"); ok {
		t.Fatal("expected ok=false on malformed jwt")
	}
}

func TestTokenExpiryFalseWhenNoExpClaim(t *testing.T) {
	payload := `{"sub":"s"}`
	jwt := "h." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
	if _, ok := tokenExpiry(jwt); ok {
		t.Fatal("expected ok=false when exp absent")
	}
}
