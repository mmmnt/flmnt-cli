package cmd

import (
	"fmt"
	"time"

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
//
// It is also THE login from here on: the session config records the local kind and the loopback
// server it is for (--server-url / QUORUM_SERVER_URL when they name one, else the harness default),
// so every command resolves its upstream from the login and needs no flag; and, one login at a time,
// every hosted token is cleared, so nothing hosted can answer for this person again.
func runLocalLogin(cmd *cobra.Command) error {
	serverURL := firstNonEmpty(flagString(cmd, "server-url"), envOr("QUORUM_SERVER_URL", ""), defaultLocalURL)
	if !httpx.IsLoopback(serverURL) {
		return fmt.Errorf("local login refused: %s is not a loopback server (a local login is for a local flmnt; `flmnt login` signs in to a hosted one)", serverURL)
	}
	marker, err := identityMarker()
	if err != nil {
		return fmt.Errorf("local login refused: %w", err)
	}
	osUser, err := identityOSUser()
	if err != nil {
		return fmt.Errorf("local login refused: %w", err)
	}
	id := identity.Derive(marker, osUser)
	if err := identitySave(id); err != nil {
		return fmt.Errorf("recording the local login: %w", err)
	}
	cleared, err := loginDeleteAllTokens()
	if err != nil {
		return fmt.Errorf("clearing the hosted login: %w", err)
	}
	if err := loginSaveConfig(localSession(id, serverURL, loginNow())); err != nil {
		return fmt.Errorf("recording the session: %w", err)
	}
	path, _ := identity.Path()
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Local login recorded: sub %s → %s\n", id.Sub, path)
	fmt.Fprintf(out, "Every command now talks to %s until `flmnt logout`.\n", serverURL)
	if origins := auth.OriginsOf(cleared); len(origins) > 0 {
		fmt.Fprintf(out, "The hosted login was cleared (%s): one login at a time.\n", joinComma(origins))
	}
	return nil
}

// localSession is the config a local login records. Nothing hosted carries over: no OAuth endpoints,
// no active workspace (a hosted setting that names nothing on the harness).
func localSession(id identity.Identity, serverURL string, now time.Time) auth.CLIConfig {
	return auth.CLIConfig{
		Login:      auth.LoginLocal,
		ServerURL:  serverURL,
		Sub:        id.Sub,
		Identity:   id.Sub,
		LoggedInAt: now.UTC().Format(time.RFC3339),
	}
}

// runLocalLogout removes the local login only. When it was the session, the session goes with it.
func runLocalLogout(cmd *cobra.Command) error {
	if err := identityRemove(); err != nil {
		return fmt.Errorf("removing the local login: %w", err)
	}
	if cfg, err := logoutLoadConfig(); err == nil && cfg.LoginKind() == auth.LoginLocal {
		if err := logoutClearConfig(); err != nil {
			return fmt.Errorf("clearing the session: %w", err)
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Local login removed.")
	return nil
}

func flagString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func joinComma(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ", "
		}
		out += item
	}
	return out
}
