package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DeviceConfig struct {
	DeviceURL string
	TokenURL  string
	ClientID  string
	Scope     string
}

type DeviceAuthResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type DevicePrompter func(DeviceAuthResponse) error

func RunDeviceFlow(cfg DeviceConfig, prompt DevicePrompter, nowFn func() time.Time, sleepFn func(time.Duration)) (TokenSet, error) {
	if nowFn == nil {
		nowFn = time.Now
	}
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	dar, err := requestDeviceAuthorization(cfg)
	if err != nil {
		return TokenSet{}, err
	}
	if err := prompt(dar); err != nil {
		return TokenSet{}, err
	}

	interval := dar.Interval
	if interval < 1 {
		interval = 5
	}
	deadline := nowFn().Add(time.Duration(dar.ExpiresIn) * time.Second)

	for nowFn().Before(deadline) {
		sleepFn(time.Duration(interval) * time.Second)
		t, err := pollDeviceToken(cfg, dar.DeviceCode)
		if err == nil {
			return t, nil
		}
		switch err.(*devicePollError).code {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
		default:
			return TokenSet{}, err
		}
	}
	return TokenSet{}, fmt.Errorf("device login timed out")
}

func requestDeviceAuthorization(cfg DeviceConfig) (DeviceAuthResponse, error) {
	body := url.Values{
		"client_id": {cfg.ClientID},
		"scope":     {cfg.Scope},
	}
	resp, err := http.Post(cfg.DeviceURL, "application/x-www-form-urlencoded", strings.NewReader(body.Encode()))
	if err != nil {
		return DeviceAuthResponse{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return DeviceAuthResponse{}, fmt.Errorf("device authorization failed (%d): %s", resp.StatusCode, raw)
	}
	var dar DeviceAuthResponse
	if err := json.Unmarshal(raw, &dar); err != nil {
		return DeviceAuthResponse{}, err
	}
	return dar, nil
}

type devicePollError struct {
	code string
	msg  string
}

func (e *devicePollError) Error() string { return fmt.Sprintf("%s: %s", e.code, e.msg) }

func pollDeviceToken(cfg DeviceConfig, deviceCode string) (TokenSet, error) {
	body := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
		"client_id":   {cfg.ClientID},
	}
	resp, err := http.Post(cfg.TokenURL, "application/x-www-form-urlencoded", strings.NewReader(body.Encode()))
	if err != nil {
		return TokenSet{}, &devicePollError{code: "network_error", msg: err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		var t TokenSet
		if err := json.Unmarshal(raw, &t); err != nil {
			return TokenSet{}, &devicePollError{code: "invalid_response", msg: err.Error()}
		}
		return t, nil
	}
	var errResp struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(raw, &errResp)
	return TokenSet{}, &devicePollError{code: errResp.Error, msg: errResp.ErrorDescription}
}
