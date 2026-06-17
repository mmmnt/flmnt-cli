package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/mmmnt/flmnt-cli/internal/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func decodeGQL(t *testing.T, r *http.Request) (string, map[string]any) {
	t.Helper()
	var req struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Fatalf("decode gql: %v", err)
	}
	return req.Query, req.Variables
}

func writeData(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func runWorkspaceArgs(t *testing.T, gqlURL string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	orig := newWorkspaceClient
	newWorkspaceClient = func(*cobra.Command) (*apiclient.Client, error) {
		return apiclient.New(gqlURL, "tok"), nil
	}
	t.Cleanup(func() { newWorkspaceClient = orig })
	for _, c := range workspaceCmd.Commands() {
		c.Flags().VisitAll(func(f *pflag.Flag) {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		})
	}
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs(append([]string{"workspace"}, args...))
	err = rootCmd.Execute()
	return out.String(), errb.String(), err
}

func TestResolveGraphQLEndpointPrecedence(t *testing.T) {
	if got := resolveGraphQLEndpoint("https://flag/graphql", "https://env/graphql", "https://disc/graphql", "https://srv"); got != "https://flag/graphql" {
		t.Fatalf("flag wins: %q", got)
	}
	if got := resolveGraphQLEndpoint("", "https://env/graphql", "https://disc/graphql", "https://srv"); got != "https://env/graphql" {
		t.Fatalf("env next: %q", got)
	}
	if got := resolveGraphQLEndpoint("", "", "https://disc/graphql", "https://srv"); got != "https://disc/graphql" {
		t.Fatalf("discovered next: %q", got)
	}
	if got := resolveGraphQLEndpoint("", "", "", "https://srv/"); got != "https://srv/graphql" {
		t.Fatalf("derived: %q", got)
	}
	if got := resolveGraphQLEndpoint("", "", "", ""); got != "" {
		t.Fatalf("empty: %q", got)
	}
}

func TestWorkspaceListMarksActiveAndRole(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = auth.SaveConfig(auth.CLIConfig{ActiveWorkspaceID: "id-own"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]any{"me": map[string]any{"workspaces": []map[string]any{
			{"id": "id-own", "name": "mine", "isOwner": true},
			{"id": "id-shared", "name": "theirs", "isOwner": false},
		}}})
	}))
	defer srv.Close()

	stdout, _, err := runWorkspaceArgs(t, srv.URL, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(stdout, "* mine") || !strings.Contains(stdout, "(owner)") {
		t.Fatalf("owner/active marker missing: %q", stdout)
	}
	if !strings.Contains(stdout, "theirs") || !strings.Contains(stdout, "(shared)") {
		t.Fatalf("shared row missing: %q", stdout)
	}
}

func TestWorkspaceCreateSetsActive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]any{"createWorkspace": map[string]any{"id": "new-id", "name": "foo"}})
	}))
	defer srv.Close()

	if _, _, err := runWorkspaceArgs(t, srv.URL, "create", "foo"); err != nil {
		t.Fatalf("create: %v", err)
	}
	cfg, _ := auth.LoadConfig()
	if cfg.ActiveWorkspaceID != "new-id" || cfg.ActiveWorkspaceName != "foo" {
		t.Fatalf("active not set: %+v", cfg)
	}
}

func TestWorkspaceUseResolvesNameToID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]any{"me": map[string]any{"workspaces": []map[string]any{
			{"id": "id-foo", "name": "foo", "isOwner": true},
		}}})
	}))
	defer srv.Close()

	if _, _, err := runWorkspaceArgs(t, srv.URL, "use", "foo"); err != nil {
		t.Fatalf("use: %v", err)
	}
	cfg, _ := auth.LoadConfig()
	if cfg.ActiveWorkspaceID != "id-foo" {
		t.Fatalf("active id: %q", cfg.ActiveWorkspaceID)
	}
	if _, _, err := runWorkspaceArgs(t, srv.URL, "use", "nope"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestWorkspaceRenameResolvesIDAndSendsNewName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var renameVars map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q, vars := decodeGQL(t, r)
		if strings.Contains(q, "renameWorkspace") {
			renameVars = vars
			writeData(w, map[string]any{"renameWorkspace": map[string]any{"id": "id-foo", "name": "bar"}})
			return
		}
		writeData(w, map[string]any{"me": map[string]any{"workspaces": []map[string]any{{"id": "id-foo", "name": "foo", "isOwner": true}}}})
	}))
	defer srv.Close()

	if _, _, err := runWorkspaceArgs(t, srv.URL, "rename", "foo", "bar"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renameVars["id"] != "id-foo" || renameVars["new"] != "bar" {
		t.Fatalf("rename vars: %+v", renameVars)
	}
}

func TestWorkspaceDeleteRequiresYesAndClearsActive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = auth.SaveConfig(auth.CLIConfig{ActiveWorkspaceID: "id-foo", ActiveWorkspaceName: "foo"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q, _ := decodeGQL(t, r)
		if strings.Contains(q, "deleteWorkspace") {
			writeData(w, map[string]any{"deleteWorkspace": true})
			return
		}
		writeData(w, map[string]any{"me": map[string]any{"workspaces": []map[string]any{{"id": "id-foo", "name": "foo", "isOwner": true}}}})
	}))
	defer srv.Close()

	if _, _, err := runWorkspaceArgs(t, srv.URL, "delete", "foo"); err == nil {
		t.Fatal("expected refusal without --yes")
	}
	if _, _, err := runWorkspaceArgs(t, srv.URL, "delete", "foo", "--yes"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	cfg, _ := auth.LoadConfig()
	if cfg.ActiveWorkspaceID != "" {
		t.Fatalf("active not cleared: %+v", cfg)
	}
}

func TestWorkspaceMembersLists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q, _ := decodeGQL(t, r)
		if strings.Contains(q, "Members") {
			writeData(w, map[string]any{"workspace": map[string]any{
				"id": "id-foo", "name": "foo", "ownerUsername": "mike",
				"members": []map[string]any{{"username": "anna", "displayName": "Anna", "userSub": "s-anna", "addedAt": "x"}},
			}})
			return
		}
		writeData(w, map[string]any{"me": map[string]any{"workspaces": []map[string]any{{"id": "id-foo", "name": "foo", "isOwner": true}}}})
	}))
	defer srv.Close()

	stdout, _, err := runWorkspaceArgs(t, srv.URL, "members", "foo")
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if !strings.Contains(stdout, "owner @mike") || !strings.Contains(stdout, "@anna") {
		t.Fatalf("members output: %q", stdout)
	}
}

func TestWorkspaceAddMemberStripsAt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var addVars map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q, vars := decodeGQL(t, r)
		if strings.Contains(q, "addWorkspaceMember") {
			addVars = vars
			writeData(w, map[string]any{"addWorkspaceMember": map[string]any{"id": "id-foo"}})
			return
		}
		writeData(w, map[string]any{"me": map[string]any{"workspaces": []map[string]any{{"id": "id-foo", "name": "foo", "isOwner": true}}}})
	}))
	defer srv.Close()

	if _, _, err := runWorkspaceArgs(t, srv.URL, "add-member", "foo", "@anna"); err != nil {
		t.Fatalf("add-member: %v", err)
	}
	if addVars["u"] != "anna" {
		t.Fatalf("username not stripped: %+v", addVars)
	}
}

func TestWorkspaceRemoveMemberResolvesUsernameToSub(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var removeVars map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q, vars := decodeGQL(t, r)
		switch {
		case strings.Contains(q, "findUserByUsername"):
			writeData(w, map[string]any{"findUserByUsername": map[string]any{"sub": "s-anna", "username": "anna"}})
		case strings.Contains(q, "removeWorkspaceMember"):
			removeVars = vars
			writeData(w, map[string]any{"removeWorkspaceMember": map[string]any{"id": "id-foo"}})
		default:
			writeData(w, map[string]any{"me": map[string]any{"workspaces": []map[string]any{{"id": "id-foo", "name": "foo", "isOwner": true}}}})
		}
	}))
	defer srv.Close()

	if _, _, err := runWorkspaceArgs(t, srv.URL, "remove-member", "foo", "@anna"); err != nil {
		t.Fatalf("remove-member: %v", err)
	}
	if removeVars["sub"] != "s-anna" {
		t.Fatalf("sub not resolved: %+v", removeVars)
	}
}
