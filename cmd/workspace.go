package cmd

import (
	"fmt"
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/mmmnt/flmnt-cli/internal/setup"
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage flmnt workspaces",
}

type wsMember struct {
	UserSub     string `json:"userSub"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AddedAt     string `json:"addedAt"`
}

type workspace struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	OwnerSub      string     `json:"ownerSub"`
	OwnerUsername string     `json:"ownerUsername"`
	CreatedAt     string     `json:"createdAt"`
	UpdatedAt     string     `json:"updatedAt"`
	IsOwner       bool       `json:"isOwner"`
	Members       []wsMember `json:"members"`
}

type userSearchResult struct {
	Sub         string `json:"sub"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

func resolveGraphQLEndpoint(flag, env, discovered, serverURL string) string {
	if flag != "" {
		return flag
	}
	if env != "" {
		return env
	}
	if discovered != "" {
		return discovered
	}
	if serverURL == "" {
		return ""
	}
	return strings.TrimRight(serverURL, "/") + "/graphql"
}

func workspaceClient(cmd *cobra.Command) (*apiclient.Client, error) {
	serverURL, _ := cmd.Flags().GetString("server-url")
	if serverURL == "" {
		serverURL = envOr("QUORUM_SERVER_URL", "")
	}
	if serverURL == "" {
		if pc, err := setup.LoadProjectConfig(""); err == nil {
			serverURL = pc.ServerURL
		}
	}
	if serverURL == "" {
		if cfg, err := auth.LoadConfig(); err == nil {
			serverURL = cfg.ServerURL
		}
	}
	flag, _ := cmd.Flags().GetString("graphql-endpoint")
	env := envOr("FILAMENT_GRAPHQL_ENDPOINT", "")
	discovered := ""
	if flag == "" && env == "" && serverURL != "" {
		if doc, derr := discoverOAuth(serverURL); derr == nil {
			discovered = doc.GraphqlEndpoint
		}
	}
	endpoint := resolveGraphQLEndpoint(flag, env, discovered, serverURL)
	if endpoint == "" {
		return nil, fmt.Errorf("--graphql-endpoint, FILAMENT_GRAPHQL_ENDPOINT, or a server URL is required")
	}
	if serverURL == "" {
		serverURL = endpoint
	}
	tokens, err := auth.LoadToken(serverURL)
	if err != nil {
		return nil, fmt.Errorf("not logged in (run `flmnt login`): %w", err)
	}
	return apiclient.New(endpoint, tokens.AccessToken), nil
}

var newWorkspaceClient = workspaceClient

func myWorkspaces(client *apiclient.Client) ([]workspace, error) {
	var out struct {
		Me struct {
			Workspaces []workspace `json:"workspaces"`
		} `json:"me"`
	}
	q := `query { me { workspaces { id name isOwner ownerUsername } } }`
	if err := client.Query(q, nil, &out); err != nil {
		return nil, apiclient.MapWorkspaceError(err)
	}
	return out.Me.Workspaces, nil
}

func resolveWorkspaceID(client *apiclient.Client, arg string) (id, name string, err error) {
	wss, err := myWorkspaces(client)
	if err != nil {
		return "", "", err
	}
	for _, ws := range wss {
		if ws.ID == arg || ws.Name == arg {
			return ws.ID, ws.Name, nil
		}
	}
	return "", "", fmt.Errorf("workspace not found: %s", arg)
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspaces you own or are a member of",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newWorkspaceClient(cmd)
		if err != nil {
			return err
		}
		wss, err := myWorkspaces(client)
		if err != nil {
			return err
		}
		cfg, _ := auth.LoadConfig()
		for _, ws := range wss {
			marker := " "
			if ws.ID == cfg.ActiveWorkspaceID {
				marker = "*"
			}
			role := "shared"
			if ws.IsOwner {
				role = "owner"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %-24s %s  (%s)\n", marker, ws.Name, ws.ID, role)
		}
		return nil
	},
}

var workspaceCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a workspace and make it active",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newWorkspaceClient(cmd)
		if err != nil {
			return err
		}
		var out struct {
			CreateWorkspace workspace `json:"createWorkspace"`
		}
		q := `mutation Create($name: String!) { createWorkspace(name: $name) { id name } }`
		if err := client.Query(q, map[string]any{"name": args[0]}, &out); err != nil {
			return apiclient.MapWorkspaceError(err)
		}
		ws := out.CreateWorkspace
		cfg, _ := auth.LoadConfig()
		cfg.ActiveWorkspaceID = ws.ID
		cfg.ActiveWorkspaceName = ws.Name
		_ = auth.SaveConfig(cfg)
		fmt.Fprintf(cmd.OutOrStdout(), "Created workspace %s (%s) — now active\n", ws.Name, ws.ID)
		return nil
	},
}

var workspaceUseCmd = &cobra.Command{
	Use:   "use [name|id]",
	Short: "Set the active workspace (used as X-Workspace-Id by `flmnt mcp auth-header`)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newWorkspaceClient(cmd)
		if err != nil {
			return err
		}
		id, name, err := resolveWorkspaceID(client, args[0])
		if err != nil {
			return err
		}
		cfg, _ := auth.LoadConfig()
		cfg.ActiveWorkspaceID = id
		cfg.ActiveWorkspaceName = name
		if err := auth.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Active workspace: %s (%s)\n", name, id)
		return nil
	},
}

var workspaceRenameCmd = &cobra.Command{
	Use:   "rename [name|id] [new-name]",
	Short: "Rename a workspace you own",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newWorkspaceClient(cmd)
		if err != nil {
			return err
		}
		id, _, err := resolveWorkspaceID(client, args[0])
		if err != nil {
			return err
		}
		var out struct {
			RenameWorkspace workspace `json:"renameWorkspace"`
		}
		q := `mutation Rename($id: ID!, $new: String!) { renameWorkspace(id: $id, newName: $new) { id name } }`
		if err := client.Query(q, map[string]any{"id": id, "new": args[1]}, &out); err != nil {
			return apiclient.MapWorkspaceError(err)
		}
		cfg, _ := auth.LoadConfig()
		if cfg.ActiveWorkspaceID == id {
			cfg.ActiveWorkspaceName = out.RenameWorkspace.Name
			_ = auth.SaveConfig(cfg)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Renamed: %s → %s\n", args[0], out.RenameWorkspace.Name)
		return nil
	},
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete [name|id]",
	Short: "Delete a workspace you own",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			return fmt.Errorf("refusing to delete without --yes")
		}
		client, err := newWorkspaceClient(cmd)
		if err != nil {
			return err
		}
		id, name, err := resolveWorkspaceID(client, args[0])
		if err != nil {
			return err
		}
		var out struct {
			DeleteWorkspace bool `json:"deleteWorkspace"`
		}
		q := `mutation Delete($id: ID!) { deleteWorkspace(id: $id) }`
		if err := client.Query(q, map[string]any{"id": id}, &out); err != nil {
			return apiclient.MapWorkspaceError(err)
		}
		cfg, _ := auth.LoadConfig()
		if cfg.ActiveWorkspaceID == id {
			cfg.ActiveWorkspaceID = ""
			cfg.ActiveWorkspaceName = ""
			_ = auth.SaveConfig(cfg)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted: %s\n", name)
		return nil
	},
}

var workspaceMembersCmd = &cobra.Command{
	Use:   "members [name|id]",
	Short: "List members of a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newWorkspaceClient(cmd)
		if err != nil {
			return err
		}
		id, _, err := resolveWorkspaceID(client, args[0])
		if err != nil {
			return err
		}
		var out struct {
			Workspace workspace `json:"workspace"`
		}
		q := `query Members($id: ID!) { workspace(id: $id) { id name ownerUsername members { username displayName userSub addedAt } } }`
		if err := client.Query(q, map[string]any{"id": id}, &out); err != nil {
			return apiclient.MapWorkspaceError(err)
		}
		ws := out.Workspace
		fmt.Fprintf(cmd.OutOrStdout(), "%s — owner @%s\n", ws.Name, ws.OwnerUsername)
		for _, m := range ws.Members {
			fmt.Fprintf(cmd.OutOrStdout(), "  @%-20s %s\n", m.Username, m.DisplayName)
		}
		return nil
	},
}

var workspaceAddMemberCmd = &cobra.Command{
	Use:   "add-member [name|id] [@username]",
	Short: "Add a member to a workspace you own",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newWorkspaceClient(cmd)
		if err != nil {
			return err
		}
		id, name, err := resolveWorkspaceID(client, args[0])
		if err != nil {
			return err
		}
		username := strings.TrimPrefix(args[1], "@")
		q := `mutation Add($id: ID!, $u: String!) { addWorkspaceMember(id: $id, username: $u) { id } }`
		if err := client.Query(q, map[string]any{"id": id, "u": username}, nil); err != nil {
			return apiclient.MapWorkspaceError(err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added @%s to %s\n", username, name)
		return nil
	},
}

var workspaceRemoveMemberCmd = &cobra.Command{
	Use:   "remove-member [name|id] [@username|sub]",
	Short: "Remove a member from a workspace you own",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newWorkspaceClient(cmd)
		if err != nil {
			return err
		}
		id, name, err := resolveWorkspaceID(client, args[0])
		if err != nil {
			return err
		}
		target := args[1]
		username := strings.TrimPrefix(target, "@")
		sub := target
		if strings.HasPrefix(target, "@") {
			var found struct {
				FindUserByUsername *userSearchResult `json:"findUserByUsername"`
			}
			fq := `query Find($u: String!) { findUserByUsername(username: $u) { sub username } }`
			if err := client.Query(fq, map[string]any{"u": username}, &found); err != nil {
				return apiclient.MapWorkspaceError(err)
			}
			if found.FindUserByUsername == nil {
				return fmt.Errorf("no user found with username @%s", username)
			}
			sub = found.FindUserByUsername.Sub
		}
		q := `mutation Remove($id: ID!, $sub: String!) { removeWorkspaceMember(id: $id, userSub: $sub) { id } }`
		if err := client.Query(q, map[string]any{"id": id, "sub": sub}, nil); err != nil {
			return apiclient.MapWorkspaceError(err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from %s\n", target, name)
		return nil
	},
}

func init() {
	subs := []*cobra.Command{
		workspaceListCmd, workspaceCreateCmd, workspaceUseCmd, workspaceRenameCmd,
		workspaceDeleteCmd, workspaceMembersCmd, workspaceAddMemberCmd, workspaceRemoveMemberCmd,
	}
	for _, c := range subs {
		c.Flags().String("server-url", "", "Quorum server URL")
		c.Flags().String("graphql-endpoint", "", "flmnt GraphQL endpoint")
	}
	workspaceDeleteCmd.Flags().BoolP("yes", "y", false, "Confirm deletion")
	workspaceCmd.AddCommand(subs...)
	rootCmd.AddCommand(workspaceCmd)
}
