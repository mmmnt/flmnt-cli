package localadmin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func adminServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

/*
`flmnt local approve <code>` confirms with the person before it approves, so it asks the broker's

	admin listener what the code is waiting for (SPEC-LOCAL-002): a web authorization is described
	by its client's name and redirect URI, a device code by its client id. The answer never carries a sub.
*/
func TestPendingDescribesAnAuthorizationAndADevice(t *testing.T) {
	base := adminServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
		}
		switch r.URL.Path {
		case "/local/pending/BCDF-GHJK":
			w.Write([]byte(`{"kind":"authorization","client_id":"dcr-claude","client_name":"Claude","redirect_uri":"https://claude.ai/api/mcp/auth_callback"}`))
		case "/local/pending/LMNP-QRST":
			w.Write([]byte(`{"kind":"device","client_id":"flmnt-cli"}`))
		default:
			t.Errorf("path %s", r.URL.Path)
		}
	})

	auth, err := Pending(base, "BCDF-GHJK")
	if err != nil {
		t.Fatal(err)
	}
	if auth.Kind != "authorization" || auth.ClientID != "dcr-claude" || auth.ClientName != "Claude" || auth.RedirectURI != "https://claude.ai/api/mcp/auth_callback" {
		t.Fatalf("authorization: %+v", auth)
	}
	dev, err := Pending(base, "LMNP-QRST")
	if err != nil {
		t.Fatal(err)
	}
	if dev.Kind != "device" || dev.ClientID != "flmnt-cli" {
		t.Fatalf("device: %+v", dev)
	}
}

/* The broker's refusal is the answer: its code and message are carried verbatim, nothing is substituted. */
func TestPendingCarriesTheBrokersRefusalVerbatim(t *testing.T) {
	base := adminServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		w.Write([]byte(`{"code":"LOCAL_APPROVAL_EXPIRED","message":"the code BCDF-GHJK expired"}`))
	})

	_, err := Pending(base, "BCDF-GHJK")
	var ae *Error
	if !errors.As(err, &ae) {
		t.Fatalf("want *Error, got %T %v", err, err)
	}
	if ae.Status != http.StatusGone || ae.Code != "LOCAL_APPROVAL_EXPIRED" || ae.Message != "the code BCDF-GHJK expired" {
		t.Fatalf("got %+v", ae)
	}
	if err.Error() != "LOCAL_APPROVAL_EXPIRED: the code BCDF-GHJK expired" {
		t.Fatalf("rendered %q", err.Error())
	}
}

/* A non-contract answer (a proxy page, a crash) is still an error, naming the status and the body. */
func TestPendingRefusesANonContractAnswer(t *testing.T) {
	base := adminServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`<html>bad gateway</html>`))
	})
	_, err := Pending(base, "BCDF-GHJK")
	if err == nil {
		t.Fatal("want an error")
	}
	var ae *Error
	if errors.As(err, &ae) {
		t.Fatalf("a non-contract body must not be read as a broker refusal: %+v", ae)
	}
}
