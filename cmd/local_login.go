package cmd

import (
	"fmt"

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
func runLocalLogin(cmd *cobra.Command) error {
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
	path, _ := identity.Path()
	fmt.Fprintf(cmd.OutOrStdout(), "Local login recorded: sub %s → %s\n", id.Sub, path)
	return nil
}

func runLocalLogout(cmd *cobra.Command) error {
	if err := identityRemove(); err != nil {
		return fmt.Errorf("removing the local login: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Local login removed.")
	return nil
}
