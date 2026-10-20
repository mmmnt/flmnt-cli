package localadmin

import (
	"encoding/json"
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

/*
POST /local/approve {code, sub, client_id} (SPEC-LOCAL-003): the CLI supplies the identity — the

	local login's sub and the client id it derived for the tool — and the broker approves the code
	for it, once.
*/
func TestApprovePostsTheIdentityForTheCode(t *testing.T) {
	var gotBody map[string]string
	var gotType string
	base := adminServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/local/approve" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		gotType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{"status":"approved"}`))
	})

	if err := Approve(base, "BCDF-GHJK", "local_34f68591-fdb4-5336-aa6a-d772cca9bd8a", "local_45dc9657-6085-5269-9247-24aca84a9cae"); err != nil {
		t.Fatal(err)
	}
	if gotType != "application/json" {
		t.Fatalf("content type %q", gotType)
	}
	want := map[string]string{"code": "BCDF-GHJK", "sub": "local_34f68591-fdb4-5336-aa6a-d772cca9bd8a", "client_id": "local_45dc9657-6085-5269-9247-24aca84a9cae"}
	if len(gotBody) != 3 || gotBody["code"] != want["code"] || gotBody["sub"] != want["sub"] || gotBody["client_id"] != want["client_id"] {
		t.Fatalf("posted %v, want %v", gotBody, want)
	}
}

func TestApproveCarriesEveryRefusalVerbatim(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
	}{
		{http.StatusBadRequest, "LOCAL_SUB_INVALID"},
		{http.StatusBadRequest, "LOCAL_CLIENT_INVALID"},
		{http.StatusNotFound, "LOCAL_APPROVAL_UNKNOWN"},
		{http.StatusConflict, "LOCAL_APPROVAL_ALREADY_APPROVED"},
		{http.StatusGone, "LOCAL_APPROVAL_EXPIRED"},
		{http.StatusForbidden, "LOCAL_ADMIN_NOT_LOOPBACK"},
	} {
		base := adminServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			w.Write([]byte(`{"code":"` + tc.code + `","message":"because"}`))
		})
		err := Approve(base, "BCDF-GHJK", "local_x", "local_y")
		var ae *Error
		if !errors.As(err, &ae) || ae.Status != tc.status || ae.Code != tc.code || ae.Message != "because" {
			t.Errorf("%s: got %T %v", tc.code, err, err)
		}
	}
}

/* An answer that is 2xx but not {status: approved} is not an approval. */
func TestApproveRefusesAnUnexpectedSuccessBody(t *testing.T) {
	base := adminServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"pending"}`))
	})
	if err := Approve(base, "BCDF-GHJK", "local_x", "local_y"); err == nil {
		t.Fatal("want an error")
	}
}
