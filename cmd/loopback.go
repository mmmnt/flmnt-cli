package cmd

import "net/url"

// isLoopback says whether a server URL names this machine — the local harness rather than a hosted
// flmnt. Credentials for a loopback server come from the checkout's headers-helper; for anything else
// the keychain `flmnt login` filled is consulted first.
func isLoopback(serverURL string) bool {
	u, err := url.Parse(serverURL)
	if err != nil {
		return false
	}
	return loopbackHost(u.Hostname())
}

func loopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
