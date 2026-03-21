package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const graphqlEndpoint = "https://api.github.com/graphql"

// GraphQL executes a GitHub GraphQL query using the existing OAuth2-authenticated
// HTTP client. No additional dependencies needed — c.gh.Client() returns the
// oauth2-wrapped *http.Client which injects Authorization: bearer TOKEN automatically.
func (c *Client) GraphQL(query string, variables map[string]any, result any) error {
	body, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return fmt.Errorf("graphql marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, graphqlEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.gh.Client().Do(req)
	if err != nil {
		return fmt.Errorf("graphql execute: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql HTTP %d", resp.StatusCode)
	}

	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("graphql decode: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}
	if result != nil && gqlResp.Data != nil {
		if err := json.Unmarshal(gqlResp.Data, result); err != nil {
			return fmt.Errorf("graphql unmarshal result: %w", err)
		}
	}
	return nil
}
