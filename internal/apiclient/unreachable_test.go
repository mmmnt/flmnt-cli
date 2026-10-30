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
