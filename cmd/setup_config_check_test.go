package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/spf13/cobra"
)

func TestSetupRefusesAMalformedFlmntJSONBeforeResolvingTheWorkspace(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	seed := "{\n  \"server_url\": \"http://localhost:8000/mcp\",\n  \"sift\": {\"store\": \".sift\",}\n}"
	if err := os.WriteFile(filepath.Join(dir, ".flmnt.json"), []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	unreachable := httptest.NewServer(http.NotFoundHandler())
	defer unreachable.Close()
	orig := newWorkspaceClient
	defer func() { newWorkspaceClient = orig }()
	newWorkspaceClient = func(*cobra.Command) (*apiclient.Client, error) { return apiclient.New(unreachable.URL, "t"), nil }
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs([]string{"setup", "--server-url", "http://127.0.0.1:1/mcp", "--project", "p"})

	err := rootCmd.Execute()

	if err == nil || !strings.Contains(err.Error(), ".flmnt.json") || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("setup error = %v, want one naming .flmnt.json and line 3", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".flmnt.json")); string(b) != seed {
		t.Fatalf(".flmnt.json rewritten: %s", b)
	}
}
