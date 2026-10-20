package cmd

import (
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/auth"
)

/*
A missing harness credential is the one failure a hook cannot show — brief and derive stay quiet by

	contract. health is where it is surfaced: on a loopback server it reports the token `flmnt login
	--local` stored, with who it is for, or that there is none and the login that stores one. It used
	to run the checkout's headers-helper here; that script is retired (QRM-ADR-015 §6).
*/
func TestHealthReportsTheStoredLocalTokenOnALoopbackServer(t *testing.T) {
	stubKeychain(t, idToken(t, `{"sub":"local_a478d4c8","email":"mike@flmnt.ai","exp":4102444800}`), nil)

	r := localTokenResult(healthCmd, "http://localhost:8000/mcp")
	if !r.OK || r.Service != "token" || !strings.Contains(r.Message, "mike@flmnt.ai") {
		t.Fatalf("got %+v, want ok with who the token is for", r)
	}
}

func TestHealthNamesTheLocalLoginWhenNoTokenIsStored(t *testing.T) {
	stubKeychain(t, "", auth.ErrNotFound)

	r := localTokenResult(healthCmd, "http://localhost:8000/mcp")
	want := "not logged in to http://localhost:8000/mcp: run `flmnt login --local --server-url http://localhost:8000/mcp`"
	if r.OK || r.Message != want {
		t.Fatalf("got %+v, want down with %q", r, want)
	}
}
