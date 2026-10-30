package session

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

type gqlCall struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// fakeFlmnt answers memoryStream with stream (a JSON value, "null" for none) and records every call.
func fakeFlmnt(t *testing.T, stream string) (*httptest.Server, *[]gqlCall) {
	t.Helper()
	var calls []gqlCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c gqlCall
		_ = json.NewDecoder(r.Body).Decode(&c)
		calls = append(calls, c)
		if strings.Contains(c.Query, "createMemoryStream") {
			_, _ = w.Write([]byte(`{"data":{"createMemoryStream":{"streamId":"ws-1::sess-1::keyframe","projectId":"ws-1","streamName":"session sess-1","streamType":"keyframe"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"memoryStream":` + stream + `}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestStreamIDIsTheSessionKeyframeStream(t *testing.T) {
	if got := StreamID("ws-1", "sess-1"); got != "ws-1::sess-1::keyframe" {
		t.Fatalf("got %q", got)
	}
}

func TestEnsureCreatesTheSessionKeyframeStreamWhenItIsAbsent(t *testing.T) {
	srv, calls := fakeFlmnt(t, "null")
	id, err := Ensure(apiclient.New(srv.URL, "tok"), "ws-1", "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if id != "ws-1::sess-1::keyframe" {
		t.Fatalf("id = %q", id)
	}
	if len(*calls) != 2 || !strings.Contains((*calls)[1].Query, "createMemoryStream") {
		t.Fatalf("want a read then a create; got %+v", *calls)
	}
	v := (*calls)[1].Variables
	if v["projectId"] != "ws-1" || v["type"] != "keyframe" || v["sessionId"] != "sess-1" {
		t.Fatalf("create variables = %v", v)
	}
}

func TestEnsureCreatesNothingWhenTheStreamExists(t *testing.T) {
	srv, calls := fakeFlmnt(t, `{"streamId":"ws-1::sess-1::keyframe"}`)
	id, err := Ensure(apiclient.New(srv.URL, "tok"), "ws-1", "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if id != "ws-1::sess-1::keyframe" {
		t.Fatalf("id = %q", id)
	}
	if len(*calls) != 1 {
		t.Fatalf("want one read and no create; got %+v", *calls)
	}
	v := (*calls)[0].Variables
	if v["streamId"] != "ws-1::sess-1::keyframe" || v["projectId"] != "ws-1" {
		t.Fatalf("read variables = %v", v)
	}
}

func TestEnsureReturnsAnUnreachableErrorWhenNothingAnswers(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	_ = l.Close()
	_, err := Ensure(apiclient.New("http://"+addr+"/graphql", "tok"), "ws-1", "sess-1")
	if !apiclient.IsUnreachable(err) {
		t.Fatalf("want an unreachable error, got %v", err)
	}
}

func TestEnsureReturnsTheRefusalWhenTheServerRefuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"unauthorized"}]}`))
	}))
	defer srv.Close()
	_, err := Ensure(apiclient.New(srv.URL, "tok"), "ws-1", "sess-1")
	if err == nil || apiclient.IsUnreachable(err) {
		t.Fatalf("want the server's refusal, got %v", err)
	}
}

func TestLineNamesTheStreamToPassToWriteKeyframeAndReadKeyframe(t *testing.T) {
	line := Line("ws-1::sess-1::keyframe")
	for _, want := range []string{`"ws-1::sess-1::keyframe"`, "write_keyframe", "read_keyframe", "stream_id"} {
		if !strings.Contains(line, want) {
			t.Errorf("line lacks %s: %q", want, line)
		}
	}
}
