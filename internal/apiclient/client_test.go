package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQuerySendsAuthorizationHeaderAndDecodesData(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "myWorkspaces") {
			t.Fatalf("expected query body, got %s", raw)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"myWorkspaces": []map[string]string{{"id": "uuid-1", "name": "foo"}},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok-1")
	var out struct {
		MyWorkspaces []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"myWorkspaces"`
	}
	if err := c.Query("query { myWorkspaces { id name } }", nil, &out); err != nil {
		t.Fatalf("Query: %v", err)
	}
	if seenAuth != "Bearer tok-1" {
		t.Fatalf("auth header: %q", seenAuth)
	}
	if len(out.MyWorkspaces) != 1 || out.MyWorkspaces[0].ID != "uuid-1" {
		t.Fatalf("unexpected data: %+v", out)
	}
}

func TestQueryReturnsGraphqlErrorFromEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "Access denied"}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok-1")
	err := c.Query("mutation { deleteWorkspace(id: \"x\") }", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Access denied") {
		t.Fatalf("expected raw graphql message, got %v", err)
	}
}

func TestMapWorkspaceError(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"graphql error: Access denied", "owner"},
		{"graphql error: user_not_found: bob", "no user"},
		{"graphql error: cannot_add_owner_as_member", "owner is already"},
		{"graphql error: cannot_remove_owner", "cannot remove the workspace owner"},
		{"graphql error: workspace_not_found: x", "workspace not found"},
		{"graphql HTTP 400: TransactionCanceledException", "name already exists"},
		{"graphql error: ConditionalCheckFailed", "name already exists"},
		{"graphql error: UNAUTHENTICATED: bad token", "flmnt login"},
	}
	for _, tc := range cases {
		got := MapWorkspaceError(fmt.Errorf("%s", tc.raw))
		if got == nil || !strings.Contains(got.Error(), tc.want) {
			t.Fatalf("MapWorkspaceError(%q) = %v; want substring %q", tc.raw, got, tc.want)
		}
	}
	if MapWorkspaceError(nil) != nil {
		t.Fatal("nil should map to nil")
	}
	passthrough := fmt.Errorf("some other failure")
	if MapWorkspaceError(passthrough) != passthrough {
		t.Fatal("unmapped errors should pass through unchanged")
	}
}
