package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunDeviceFlowReturnsTokenOnceAuthorized(t *testing.T) {
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device":
			_ = json.NewEncoder(w).Encode(DeviceAuthResponse{
				DeviceCode:      "dev-code-1",
				UserCode:        "AAAA-BBBB",
				VerificationURI: "https://auth.example.com/device",
				ExpiresIn:       600,
				Interval:        1,
			})
		case "/token":
			pollCount++
			if pollCount < 2 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
				return
			}
			_ = json.NewEncoder(w).Encode(TokenSet{
				AccessToken: "access-1", RefreshToken: "refresh-1",
			})
		}
	}))
	defer srv.Close()

	var prompted bool
	cfg := DeviceConfig{
		DeviceURL: srv.URL + "/device",
		TokenURL:  srv.URL + "/token",
		ClientID:  "filament-cli",
		Scope:     "openid",
	}
	now := time.Now()
	t0 := now
	tokens, err := RunDeviceFlow(cfg, func(d DeviceAuthResponse) error {
		if d.UserCode != "AAAA-BBBB" {
			t.Fatalf("unexpected user code")
		}
		prompted = true
		return nil
	}, func() time.Time { return t0 }, func(d time.Duration) { t0 = t0.Add(d) })

	if err != nil {
		t.Fatalf("RunDeviceFlow: %v", err)
	}
	if !prompted {
		t.Fatal("prompt not called")
	}
	if tokens.AccessToken != "access-1" {
		t.Fatalf("unexpected token: %+v", tokens)
	}
}

func TestRunDeviceFlowTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device":
			_ = json.NewEncoder(w).Encode(DeviceAuthResponse{
				DeviceCode: "dev", UserCode: "X", VerificationURI: "y",
				ExpiresIn: 2, Interval: 1,
			})
		case "/token":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
		}
	}))
	defer srv.Close()

	t0 := time.Now()
	_, err := RunDeviceFlow(DeviceConfig{
		DeviceURL: srv.URL + "/device", TokenURL: srv.URL + "/token",
		ClientID: "filament-cli",
	}, func(DeviceAuthResponse) error { return nil },
		func() time.Time { return t0 },
		func(d time.Duration) { t0 = t0.Add(d) })
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
