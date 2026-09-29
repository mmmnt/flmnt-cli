package corpus

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

func TestFetchReadsTheWholeStreamOldestFirst(t *testing.T) {
	var gotStream string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(b, &req)
		gotStream, _ = req.Variables["s"].(string)
		_, _ = w.Write([]byte(`{"data":{"memorySlice":[{"id":"a","causationId":"a","entryType":"decision.made","timestamp":"t","content":"first"},{"id":"b","causationId":"a","entryType":"decision.made","timestamp":"t2","content":"second"}]}}`))
	}))
	defer srv.Close()

	got, err := Fetch(apiclient.New(srv.URL, "tok"), "ws::domain")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotStream != "ws::domain" {
		t.Errorf("want the stream queried, got %q", gotStream)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].Content != "second" {
		t.Errorf("want both entries in stream order, got %+v", got)
	}
}
