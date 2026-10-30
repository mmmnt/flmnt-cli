package cmd

import (
	"fmt"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/env"
	"github.com/mmmnt/flmnt-cli/internal/httpx"
	"github.com/mmmnt/flmnt-cli/internal/identity"
	"github.com/spf13/cobra"
)

var (
	identityMarker = identity.Marker
	identityOSUser = identity.OSUser
	identitySave   = identity.Save
	identityLoad   = identity.Load
)

// runLocalLogin records the LOCAL LOGIN with no input: the identity the machine already holds, or on
// the first login the marker the host machine carries and the OS user, digested to the sub a local
// flmnt mints for. Only the digest is written; a machine with no readable marker is refused and an
// identity is never invented.
//
// It is also THE login from here on: the session config records the local kind and the loopback
// server it is for (--server-url / FLMNT_SERVER_URL when they name one, else the harness default),
// so every command resolves its upstream from the login and needs no flag; and, one login at a time,
// every hosted token is cleared, so nothing hosted can answer for this person again. The CLI's own
// harness token is then obtained for that server (runLocalToken).
func runLocalLogin(cmd *cobra.Command) error {
	serverURL := firstNonEmpty(flagString(cmd, "server-url"), env.Get("SERVER_URL"), defaultLocalURL)
	if !httpx.IsLoopback(serverURL) {
		return fmt.Errorf("local login refused: %s is not a loopback server (a local login is for a local flmnt; `flmnt login` signs in to a hosted one)", serverURL)
	}
	renew, _ := cmd.Flags().GetBool("new")
	id, err := recordedOrDerivedIdentity(renew)
	if err != nil {
		return err
	}
	cleared, err := loginDeleteAllTokens()
	if err != nil {
		return fmt.Errorf("clearing the hosted login: %w", err)
	}
	session := localSession(id, serverURL, loginNow())
	if err := loginSaveConfig(session); err != nil {
		return fmt.Errorf("recording the session: %w", err)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Local login recorded: sub %s (kept in %s)\n", id.Sub, identity.StorageDescription())
	if formerly := id.Formerly(); len(formerly) > 0 {
		fmt.Fprintf(out, "This machine formerly answered as %s; the history travels with the identity so those records link to the same account.\n", joinComma(formerly))
	}
	fmt.Fprintf(out, "Every command now talks to %s until `flmnt logout`.\n", serverURL)
	if origins := auth.OriginsOf(cleared); len(origins) > 0 {
		fmt.Fprintf(out, "The hosted login was cleared (%s): one login at a time.\n", joinComma(origins))
	}
	return runLocalToken(cmd, id, session, flagString(cmd, "admin-url"))
}

// recordedOrDerivedIdentity is the identity the machine already holds, or — on the first local login
// — a freshly SALTED one derived from its marker, the OS user and a random salt, and recorded
// (QRM-ADR-016 §5g: unique by construction). The recorded identity is the identity from then on: a
// login never re-derives over it, so the sub a machine answers with never changes underneath its
// records. `--new` re-mints on purpose — a refused connection on a teammate's harness, a fresh start —
// and the old sub is kept in the identity's history, never asked of anyone.
func recordedOrDerivedIdentity(renew bool) (identity.Identity, error) {
	held, err := identityLoad()
	if err == nil && !renew {
		return held, nil
	}
	marker, err := identityMarker()
	if err != nil {
		return identity.Identity{}, fmt.Errorf("local login refused: %w", err)
	}
	osUser, err := identityOSUser()
	if err != nil {
		return identity.Identity{}, fmt.Errorf("local login refused: %w", err)
	}
	var id identity.Identity
	if renew && held.Sub != "" {
		id = identity.Remint(held, marker, osUser, loginNow())
	} else {
		id = identity.DeriveSalted(marker, osUser, identity.NewSalt())
	}
	if err := identitySave(id); err != nil {
		return identity.Identity{}, fmt.Errorf("recording the local login: %w", err)
	}
	return id, nil
}

// runLocalToken obtains the CLI's OWN credential from the local flmnt once the login is recorded: the
// device grant as `flmnt-cli`, with the CLI approving its own user code over the broker's loopback
// admin listener for the identity it just recorded — nobody is sent to a browser (QRM-ADR-015). The
// token lands under the session's server URL as any login's does, and the session records the OAuth
// endpoints refresh and logout need. The step is skipped, with the reason, when the server advertises
// no device grant; the local login itself stands either way.
func runLocalToken(cmd *cobra.Command, id identity.Identity, session auth.CLIConfig, adminURL string) error {
	out := cmd.OutOrStdout()
	skip := func(reason string) error {
		fmt.Fprintf(out, "Token step skipped: %s\n", reason)
		return nil
	}
	serverURL := session.ServerURL
	doc, err := loginDiscover(serverURL)
	if err != nil {
		return skip(fmt.Sprintf("OAuth discovery at %s failed: %v", serverURL, err))
	}
	if doc.DeviceAuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return skip(fmt.Sprintf("%s advertises no device_authorization_endpoint/token_endpoint", serverURL))
	}
	if _, err := obtainLocalToken(id, serverURL, adminURL, doc); err != nil {
		return err
	}
	session.AuthURL = doc.AuthorizationEndpoint
	session.TokenURL = doc.TokenEndpoint
	session.ClientID = cliTool
	session.RevocationEndpoint = doc.RevocationEndpoint
	_ = loginSaveConfig(session)
	fmt.Fprintf(out, "Local token stored in %s for %s.\n", auth.StorageDescription(), serverURL)
	return nil
}

// obtainLocalToken is the CLI's own harness credential, minted from the identity: the device grant as
// `flmnt-cli`, self-approved over the loopback admin listener, stored under the server URL. A session
// artifact (QRM-ADR-016 §5f): `login --local` mints it, and so does any command that finds none in the
// keychain after a hosted login swept it — the identity is what persists, the token is re-obtained.
func obtainLocalToken(id identity.Identity, serverURL, adminURL string, doc oauthDiscovery) (auth.TokenSet, error) {
	tokens, err := loginRunDeviceFlow(auth.DeviceConfig{
		DeviceURL: doc.DeviceAuthorizationEndpoint,
		TokenURL:  doc.TokenEndpoint,
		ClientID:  cliTool,
		Scope:     "openid email profile",
	}, func(d auth.DeviceAuthResponse) error {
		return localAdminApprove(adminURL, d.UserCode, id.Sub, id.ClientID(cliTool))
	}, nil, nil)
	if err != nil {
		return auth.TokenSet{}, fmt.Errorf("obtaining the local token: %w", err)
	}
	if err := loginStoreToken(serverURL, tokens); err != nil {
		return auth.TokenSet{}, fmt.Errorf("storing token: %w", err)
	}
	return tokens, nil
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

// runLocalLogout ends a LOCAL session: its token and the config go, as `logout` would, and the
// identity stays — who this machine is outlives every login. A hosted session is not what --local
// names and is left alone, with the command that does end it.
func runLocalLogout(cmd *cobra.Command) error {
	cfg, _ := logoutLoadConfig()
	out := cmd.OutOrStdout()
	if cfg.LoginKind() != auth.LoginLocal {
		if cfg.LoginKind() == auth.LoginHosted {
			fmt.Fprintf(out, "The session is a hosted login (%s), not a local one; `flmnt logout` ends it.\n", firstNonEmpty(cfg.Identity, cfg.ServerURL))
			return nil
		}
		fmt.Fprintln(out, "No local login to end.")
		return nil
	}
	removed, err := logoutDeleteAllTokens()
	if err != nil {
		return fmt.Errorf("clearing keychain: %w", err)
	}
	if err := logoutClearConfig(); err != nil {
		return fmt.Errorf("clearing the session: %w", err)
	}
	identitySub := ""
	if id, err := identityLoad(); err == nil {
		identitySub = id.Sub
	}
	fmt.Fprintln(out, logoutLine(cfg, auth.OriginsOf(removed), identitySub))
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
