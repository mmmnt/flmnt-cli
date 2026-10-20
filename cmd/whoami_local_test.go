package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

func runWhoami(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs(append([]string{"whoami"}, args...))
	t.Cleanup(func() { _ = whoamiCmd.Flags().Set("server-url", "") })
	err = rootCmd.Execute()
	return out.String(), errb.String(), err
}

func idToken(t *testing.T, claims string) string {
	t.Helper()
	return "h." + strings.TrimRight(base64url([]byte(claims)), "=") + ".s"
}

/* On a loopback server the helper's credential is the identity; "Not logged in." was the keychain's answer, not the harness's. */
func TestWhoamiDecodesTheHelpersBearerForALoopbackServer(t *testing.T) {
	stubKeychain(t, "", auth.ErrNotFound)
	stubHelper(t, helperPrinting(t, idToken(t, `{"sub":"local-test-user-0001"}`)))
	t.Chdir(t.TempDir())

	stdout, _, err := runWhoami(t, "--server-url", "http://localhost:8000/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "local-test-user-0001") || strings.Contains(stdout, "Not logged in") {
		t.Fatalf("got %q", stdout)
	}
}
