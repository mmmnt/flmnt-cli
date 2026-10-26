package cmd

import (
	"errors"
	"os"
	"os/exec"

	"github.com/mmmnt/flmnt-cli/internal/harness"
	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

// runCompose runs one docker compose call on this checkout's stack, with every store's data under the
// node's volumes path.
func runCompose(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	pc, err := setup.LoadProjectConfig(dir)
	if errors.Is(err, os.ErrNotExist) {
		pc = &setup.ProjectConfig{}
	} else if err != nil {
		return err
	}
	volumes, err := harness.VolumesPath(pc)
	if err != nil {
		return err
	}
	argv, err := harness.ComposeArgs(dir, pc, args)
	if err != nil {
		return err
	}
	c := exec.Command("docker", argv...)
	c.Dir = dir
	c.Env = append(os.Environ(), "FLMNT_VOLUMES="+volumes)
	c.Stdin, c.Stdout, c.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
	return c.Run()
}

var localUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the local flmnt node",
	Long: "Starts the local node: flmnt's baseline harness with the checkout's compose_files (.flmnt.json)\n" +
		"stacked on it in order, every store's data kept under the node's volumes path (volumes_path in\n" +
		".flmnt.json, default ~/.flmnt/volumes).",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error { return runCompose(cmd, []string{"up", "-d"}) },
}

var localDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the local flmnt node",
	Long:  "Stops the local node. Every store's data stays under the node's volumes path; deleting it is\nthe person's act, outside flmnt.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runCompose(cmd, []string{"down", "--remove-orphans"})
	},
}

var localStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "List the local node's services",
	Args:  cobra.NoArgs,
	RunE:  func(cmd *cobra.Command, _ []string) error { return runCompose(cmd, []string{"ps"}) },
}

var localComposeCmd = &cobra.Command{
	Use:                "compose [args...]",
	Short:              "Run any docker compose call on the local node's stack",
	Long:               "Runs `docker compose <args>` on the same stack `flmnt local up` starts, with the same volumes path:\nfor a seed job, a restart, a log tail.",
	DisableFlagParsing: true,
	RunE:               func(cmd *cobra.Command, args []string) error { return runCompose(cmd, args) },
}

func init() {
	localCmd.AddCommand(localUpCmd, localDownCmd, localStatusCmd, localComposeCmd)
}
