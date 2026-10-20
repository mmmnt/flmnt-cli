package cmd

import (
	"strings"
	"testing"
)

/*
A helper that refuses (no identity declared, no client id) is the one failure a hook cannot show —

	brief and derive stay quiet by contract. health is where it is surfaced: on a loopback server it
	reports the helper, with the actor it minted for or the helper's own reason, never a default.
*/
func TestHealthReportsTheHelperOnALoopbackServer(t *testing.T) {
	stubHelper(t, helperPrinting(t, idToken(t, `{"sub":"a478d4c8","email":"mike@flmnt.ai"}`)))

	r := helperResult(healthCmd)
	if !r.OK || r.Service != "helper" || !strings.Contains(r.Message, "mike@flmnt.ai") {
		t.Fatalf("got %+v, want ok with the minted actor", r)
	}
}

func TestHealthSurfacesTheHelpersReason(t *testing.T) {
	stubHelper(t, helper("echo 'mint-token: no identity declared — run flmnt login "+t.TempDir()+"' >&2; exit 1"))

	r := helperResult(healthCmd)
	if r.OK || !strings.Contains(r.Message, "no identity declared") {
		t.Fatalf("got %+v, want the helper's reason", r)
	}
}
