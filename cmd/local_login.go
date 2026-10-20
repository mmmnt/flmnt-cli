package cmd

import (
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/identity"
	"github.com/spf13/cobra"
)

var (
	identityMarker = identity.Marker
	identitySave   = identity.Save
	identityLoad   = identity.Load
)

// runLocalLogin records the LOCAL LOGIN: the email the person declares plus the marker the host
// machine already carries. The sub and every per-tool client id derive from those two, so a refused
// login — no email, no readable marker — writes nothing; an identity is never invented.
func runLocalLogin(cmd *cobra.Command) error {
	email, _ := cmd.Flags().GetString("email")
	if email == "" {
		return fmt.Errorf("--email is required with --local: the local login is the person it names")
	}
	marker, err := identityMarker()
	if err != nil {
		return fmt.Errorf("local login refused: %w", err)
	}
	id := identity.Identity{Email: email, Marker: marker}
	if err := identitySave(id); err != nil {
		return fmt.Errorf("recording the local login: %w", err)
	}
	path, _ := identity.Path()
	fmt.Fprintf(cmd.OutOrStdout(), "Local login recorded: %s on this machine (sub %s) → %s\n", email, id.Sub(), path)
	return nil
}
