package cmd

import (
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
	"github.com/mmmnt/flmnt-cli/internal/identity"
	"github.com/spf13/cobra"
)

var (
	identityMarker = identity.Marker
	identityOSUser = identity.OSUser
	identitySave   = identity.Save
	identityLoad   = identity.Load
	identityRemove = identity.Remove
)

// runLocalLogin records the LOCAL LOGIN with no input: the marker the host machine already carries
// and the OS user, digested to the sub a local flmnt mints for. Only the digest is written; a machine
// with no readable marker is refused and an identity is never invented.
func runLocalLogin(cmd *cobra.Command) (identity.Identity, error) {
	marker, err := identityMarker()
	if err != nil {
		return identity.Identity{}, fmt.Errorf("local login refused: %w", err)
	}
	osUser, err := identityOSUser()
	if err != nil {
		return identity.Identity{}, fmt.Errorf("local login refused: %w", err)
	}
	id := identity.Derive(marker, osUser)
	if err := identitySave(id); err != nil {
		return identity.Identity{}, fmt.Errorf("recording the local login: %w", err)
	}
	path, _ := identity.Path()
	fmt.Fprintf(cmd.OutOrStdout(), "Local login recorded: sub %s → %s\n", id.Sub, path)
	return id, nil
}

// runLocalToken obtains the CLI's OWN credential from the local flmnt once the login is recorded: the
// device grant as `flmnt-cli`, with the CLI approving its own user code over the broker's loopback
// admin listener for the identity it just recorded — nobody is sent to a browser (QRM-ADR-015). The
// token lands under the server URL as any login's does. The step is skipped, with the reason, when no
// server resolves, when it is not a local one, or when it advertises no device grant; the local login
// itself stands either way.
func runLocalToken(cmd *cobra.Command, id identity.Identity, serverURL, adminURL string) error {
	out := cmd.OutOrStdout()
	skip := func(reason string) error {
		fmt.Fprintf(out, "Token step skipped: %s\n", reason)
		return nil
	}
	if serverURL == "" {
		return skip("no server URL resolved (--server-url, QUORUM_SERVER_URL or the checkout's .quorum.json names the local flmnt)")
	}
	if !httpx.IsLoopback(serverURL) {
		return skip(fmt.Sprintf("%s is not a local server; a hosted flmnt is signed in to with `flmnt login`", serverURL))
	}
	doc, err := loginDiscover(serverURL)
	if err != nil {
		return skip(fmt.Sprintf("OAuth discovery at %s failed: %v", serverURL, err))
	}
	if doc.DeviceAuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return skip(fmt.Sprintf("%s advertises no device_authorization_endpoint/token_endpoint", serverURL))
	}
	tokens, err := loginRunDeviceFlow(auth.DeviceConfig{
		DeviceURL: doc.DeviceAuthorizationEndpoint,
		TokenURL:  doc.TokenEndpoint,
		ClientID:  cliTool,
		Scope:     "openid email profile",
	}, func(d auth.DeviceAuthResponse) error {
		return localAdminApprove(adminURL, d.UserCode, id.Sub, id.ClientID(cliTool))
	}, nil, nil)
	if err != nil {
		return fmt.Errorf("obtaining the local token: %w", err)
	}
	if err := loginStoreToken(serverURL, tokens); err != nil {
		return fmt.Errorf("storing token: %w", err)
	}
	cfg, _ := loginLoadConfig()
	cfg.ServerURL = serverURL
	cfg.AuthURL = doc.AuthorizationEndpoint
	cfg.TokenURL = doc.TokenEndpoint
	cfg.ClientID = cliTool
	cfg.RevocationEndpoint = doc.RevocationEndpoint
	_ = loginSaveConfig(cfg)
	fmt.Fprintf(out, "Local token stored in %s for %s.\n", auth.StorageDescription(), serverURL)
	return nil
}

func runLocalLogout(cmd *cobra.Command) error {
	if err := identityRemove(); err != nil {
		return fmt.Errorf("removing the local login: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Local login removed.")
	return nil
}
