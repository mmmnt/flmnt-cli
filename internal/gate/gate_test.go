package gate

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

func gqlServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
}

func TestRunReturnsReminderWhenNoKeyframe(t *testing.T) {
	srv := gqlServer(`{"data":{"memoryKeyframe":null}}`)
	defer srv.Close()
	out, _ := Run(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "p", Threshold: 30 * time.Minute})
	if out != injectionText {
		t.Fatalf("want reminder, got %q", out)
	}
}

func TestRunSilentWhenKeyframeFresh(t *testing.T) {
	fresh := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	srv := gqlServer(fmt.Sprintf(`{"data":{"memoryKeyframe":{"createdAt":%q}}}`, fresh))
	defer srv.Close()
	out, _ := Run(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "p", Threshold: 30 * time.Minute})
	if out != "" {
		t.Fatalf("want silent, got %q", out)
	}
}

func TestRunReturnsReminderWhenKeyframeStale(t *testing.T) {
	stale := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	srv := gqlServer(fmt.Sprintf(`{"data":{"memoryKeyframe":{"createdAt":%q}}}`, stale))
	defer srv.Close()
	out, _ := Run(Config{GQL: apiclient.New(srv.URL, "tok"), ProjectID: "p", Threshold: 30 * time.Minute})
	if out != injectionText {
		t.Fatalf("want reminder, got %q", out)
	}
}

func TestRunSilentWhenRouterUnreachable(t *testing.T) {
	out, err := Run(Config{GQL: apiclient.New("http://127.0.0.1:0", "tok"), ProjectID: "p", Threshold: 30 * time.Minute})
	if out != "" || err != nil {
		t.Fatalf("want silent+nil, got out=%q err=%v", out, err)
	}
}
