package github

import (
"encoding/json"
"fmt"

ghcli "github.com/castrojo/homebrew-stats/internal/ghcli"
)

// GraphQL executes a GitHub GraphQL query via gh CLI.
func GraphQL(query string, variables map[string]any, result any) error {
args := []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", query)}
for k, v := range variables {
if v != nil {
args = append(args, "-f", fmt.Sprintf("%s=%v", k, v))
}
}
out, err := ghcli.Run(args...)
if err != nil {
return fmt.Errorf("graphql: %w", err)
}
// gh api graphql returns {"data": {...}, "errors": [...]}
var gqlResp struct {
Data   json.RawMessage `json:"data"`
Errors []struct {
Message string `json:"message"`
} `json:"errors"`
}
if err := json.Unmarshal(out, &gqlResp); err != nil {
return fmt.Errorf("graphql decode: %w", err)
}
if len(gqlResp.Errors) > 0 {
return fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
}
if result != nil && gqlResp.Data != nil {
return json.Unmarshal(gqlResp.Data, result)
}
return nil
}
