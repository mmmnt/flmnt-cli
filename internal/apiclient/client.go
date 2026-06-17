package apiclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func MapWorkspaceError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "Access denied"):
		return errors.New("not permitted — you must be the workspace owner (or a member to read it)")
	case strings.Contains(msg, "user_not_found"):
		return errors.New("no user found with that username")
	case strings.Contains(msg, "cannot_add_owner_as_member"):
		return errors.New("the owner is already a member; cannot add the owner")
	case strings.Contains(msg, "cannot_remove_owner"):
		return errors.New("cannot remove the workspace owner")
	case strings.Contains(msg, "workspace_not_found"):
		return errors.New("workspace not found")
	case strings.Contains(msg, "TransactionCanceled") || strings.Contains(msg, "ConditionalCheckFailed"):
		return errors.New("a workspace with that name already exists")
	case strings.Contains(msg, "UNAUTHENTICATED"):
		return errors.New("not logged in or token expired; run `flmnt login`")
	default:
		return err
	}
}

type Client struct {
	endpoint    string
	accessToken string
	httpClient  *http.Client
}

func New(endpoint, accessToken string) *Client {
	return &Client{
		endpoint:    endpoint,
		accessToken: accessToken,
		httpClient:  http.DefaultClient,
	}
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlError struct {
	Message    string `json:"message"`
	Extensions struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	} `json:"extensions"`
}

// gqlErrorMessage unwraps the federated-router envelope: when the Cosmo router
// wraps a subgraph failure it reports "Failed to fetch from Subgraph 'x'" at the
// top level and puts the real subgraph error under extensions.errors[]. Prefer
// the nested message so MapWorkspaceError can recognize it.
func gqlErrorMessage(e gqlError) string {
	if len(e.Extensions.Errors) > 0 && e.Extensions.Errors[0].Message != "" {
		return e.Extensions.Errors[0].Message
	}
	return e.Message
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors,omitempty"`
}

func (c *Client) Query(query string, variables map[string]any, out any) error {
	reqBody, err := json.Marshal(gqlRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql HTTP %d: %s", resp.StatusCode, raw)
	}
	var envelope gqlResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("invalid graphql response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", gqlErrorMessage(envelope.Errors[0]))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}
