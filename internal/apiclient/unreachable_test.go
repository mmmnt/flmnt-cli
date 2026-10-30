package apiclient

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// refusedURL names a loopback port nothing listens on: it is bound, read and released, so a request
// to it is refused at the transport, the way a stopped flmnt refuses one.
func refusedURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return "http://" + addr + "/graphql"
}

func TestIsUnreachableWhenTheConnectionIsRefused(t *testing.T) {
	err := New(refusedURL(t), "tok").Query("query { x }", nil, nil)
	if err == nil || !IsUnreachable(err) {
		t.Fatalf("a refused connection is unreachable; got %v", err)
	}
}

func TestIsUnreachableFalseWhenTheServerAnswersWithAnHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()
	err := New(srv.URL, "tok").Query("query { x }", nil, nil)
	if err == nil || IsUnreachable(err) {
		t.Fatalf("a server that answers 401 is reachable; got %v", err)
	}
}

func TestIsUnreachableFalseWhenTheServerAnswersWithAGraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"boom"}]}`))
	}))
	defer srv.Close()
	err := New(srv.URL, "tok").Query("query { x }", nil, nil)
	if err == nil || IsUnreachable(err) {
		t.Fatalf("a server that answers a GraphQL error is reachable; got %v", err)
	}
}

func TestIsUnreachableFalseForNil(t *testing.T) {
	if IsUnreachable(nil) {
		t.Fatal("no error is not unreachable")
	}
}

func TestAnsweredWronglyNamesTheEndpointAndTheStatusOfAnHTTPError(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", status)
		}))
		err := New(srv.URL+"/mcp/graphql", "tok").Query("query { x }", nil, nil)
		srv.Close()
		werr, ok := AnsweredWrongly(err)
		if !ok || werr.Endpoint != srv.URL+"/mcp/graphql" || werr.StatusCode != status {
			t.Fatalf("HTTP %d is a wrong answer at the endpoint; got %#v ok=%v (%v)", status, werr, ok, err)
		}
	}
}

func TestAnsweredWronglyForABodyThatIsNotGraphQL(t *testing.T) {
	for _, body := range []string{"<!doctype html><html></html>", `{"jsonrpc":"2.0","id":1}`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		err := New(srv.URL, "tok").Query("query { x }", nil, nil)
		srv.Close()
		if _, ok := AnsweredWrongly(err); !ok {
			t.Fatalf("a 200 whose body %q is not a GraphQL response is a wrong answer; got %v", body, err)
		}
	}
}

func TestAnsweredWronglyFalseForAGraphQLAnswer(t *testing.T) {
	for _, body := range []string{`{"errors":[{"message":"boom"}]}`, `{"data":{"x":null}}`, `{"data":null,"errors":[{"message":"no"}]}`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		err := New(srv.URL, "tok").Query("query { x }", nil, &struct{}{})
		srv.Close()
		if _, ok := AnsweredWrongly(err); ok {
			t.Fatalf("a GraphQL answer %q is not a wrong answer; got %v", body, err)
		}
	}
	if _, ok := AnsweredWrongly(New(refusedURL(t), "tok").Query("query { x }", nil, nil)); ok {
		t.Fatal("nothing answering is unreachable, not a wrong answer")
	}
}
