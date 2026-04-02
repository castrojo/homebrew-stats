package github

import (
"encoding/base64"
"encoding/json"
"fmt"
"strings"

ghcli "github.com/castrojo/homebrew-stats/internal/ghcli"
)

// TesthubPackage represents a Flatpak package from projectbluefin testhub.
type TesthubPackage struct {
Name    string `json:"name"`
Version string `json:"version,omitempty"`
HTMLURL string `json:"html_url,omitempty"`
}

// GetTrafficClones fetches 14-day clone traffic for owner/repo.
// Requires push access to the repository.
func GetTrafficClones(owner, repo string) (count, uniques int, err error) {
out, err := ghcli.Run("api",
fmt.Sprintf("repos/%s/%s/traffic/clones", owner, repo),
"--jq", "[.count, .uniques]")
if err != nil {
return 0, 0, fmt.Errorf("traffic clones for %s/%s: %w", owner, repo, err)
}
var pair [2]int
if err := json.Unmarshal(out, &pair); err != nil {
return 0, 0, fmt.Errorf("traffic clones parse for %s/%s: %w", owner, repo, err)
}
return pair[0], pair[1], nil
}

// ListDirectory returns the names of .rb files in a repo directory.
func ListDirectory(owner, repo, path string) ([]string, error) {
out, err := ghcli.Run("api",
fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, path),
"--jq", `[.[] | select(.type=="file" and (.name | endswith(".rb"))) | .name]`)
if err != nil {
return nil, fmt.Errorf("listing %s/%s/%s: %w", owner, repo, path, err)
}
var names []string
if err := json.Unmarshal(out, &names); err != nil {
return nil, fmt.Errorf("listing parse %s/%s/%s: %w", owner, repo, path, err)
}
return names, nil
}

// GetFileContent fetches the raw content of a file in a repo.
func GetFileContent(owner, repo, path string) (string, error) {
out, err := ghcli.Run("api",
fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, path),
"--jq", ".content")
if err != nil {
return "", fmt.Errorf("get file %s/%s/%s: %w", owner, repo, path, err)
}
// gh api --jq returns the value as a JSON string (quoted), unwrap it.
var b64 string
if err := json.Unmarshal(out, &b64); err != nil {
return "", fmt.Errorf("decode content json %s/%s/%s: %w", owner, repo, path, err)
}
// GitHub encodes content as base64 with embedded newlines.
cleaned := strings.ReplaceAll(b64, "\n", "")
decoded, err := base64.StdEncoding.DecodeString(cleaned)
if err != nil {
return "", fmt.Errorf("base64 decode %s/%s/%s: %w", owner, repo, path, err)
}
return string(decoded), nil
}

// GetLatestReleaseTag returns the latest release tag for owner/repo.
// Returns empty string without error if no releases exist (404).
func GetLatestReleaseTag(owner, repo string) (string, error) {
out, err := ghcli.Run("api",
fmt.Sprintf("repos/%s/%s/releases/latest", owner, repo),
"--jq", ".tag_name")
if err != nil {
// 404 means no releases — not an error for callers.
if strings.Contains(err.Error(), "404") {
return "", nil
}
return "", fmt.Errorf("latest release for %s/%s: %w", owner, repo, err)
}
var tag string
if err := json.Unmarshal(out, &tag); err != nil {
return "", fmt.Errorf("latest release parse %s/%s: %w", owner, repo, err)
}
return tag, nil
}

// pkgEntry is used for parsing gh api package listing output.
type pkgEntry struct {
Name    string `json:"name"`
HTMLURL string `json:"html_url"`
}

// ListTesthubPackages returns container packages from the given GitHub org via the Packages API.
func ListTesthubPackages(org string) ([]TesthubPackage, error) {
// Paginate manually — gh api --paginate concatenates arrays for us with --jq '[.[]]'.
out, err := ghcli.Run("api",
fmt.Sprintf("orgs/%s/packages?package_type=container&per_page=100", org),
"--paginate",
"--jq", "[.[] | {name: .name, html_url: .html_url}]")
if err != nil {
return nil, fmt.Errorf("listing packages for %s: %w", org, err)
}

// --paginate with --jq emits one JSON array per page; wrap into a single array.
var entries []pkgEntry
// Try direct array parse first (single page).
if err2 := json.Unmarshal(out, &entries); err2 != nil {
// Multi-page: concatenated arrays — use decoder to read them all.
dec := json.NewDecoder(strings.NewReader(string(out)))
for dec.More() {
var page []pkgEntry
if err3 := dec.Decode(&page); err3 != nil {
break
}
entries = append(entries, page...)
}
}

all := make([]TesthubPackage, 0, len(entries))
for _, e := range entries {
tp := TesthubPackage{Name: e.Name, HTMLURL: e.HTMLURL}

// Fetch latest version tag.
vout, verr := ghcli.Run("api",
fmt.Sprintf("orgs/%s/packages/container/%s/versions?per_page=1", org, e.Name),
"--jq", ".[0].metadata.container.tags[0]")
if verr == nil {
var tag string
if json.Unmarshal(vout, &tag) == nil {
tp.Version = tag
}
}

all = append(all, tp)
}
return all, nil
}
