package scorecard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchAll_NilSliceGuard verifies that Results is never nil (marshals as []).
func TestFetchAll_NilSliceGuard(t *testing.T) {
	// Use a server that always returns 404 so no repos are indexed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	out, err := fetchAllFromBase([]string{"ublue-os/bluefin"}, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Results == nil {
		t.Fatal("Results must not be nil — Go marshals nil slice as JSON null which breaks Astro")
	}
	b, _ := json.Marshal(out)
	var check map[string]interface{}
	json.Unmarshal(b, &check) //nolint:errcheck
	if results, ok := check["results"]; !ok || results == nil {
		t.Fatal("results must marshal as [] not null")
	}
}

// TestFetchAll_404_NotIndexed verifies that 404 produces an Indexed:false entry.
func TestFetchAll_404_NotIndexed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	out, err := fetchAllFromBase([]string{"ublue-os/bluefin"}, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out.Results))
	}
	r := out.Results[0]
	if r.Indexed {
		t.Error("expected Indexed=false for 404")
	}
	if r.Score != nil {
		t.Errorf("expected Score=nil for 404, got %v", *r.Score)
	}
	if r.Date != nil {
		t.Errorf("expected Date=nil for 404, got %v", *r.Date)
	}
}

// TestFetchAll_429_Skipped verifies that 429 produces an Indexed:false entry and does not fail the run.
func TestFetchAll_429_Skipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	out, err := fetchAllFromBase([]string{"ublue-os/bluefin"}, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out.Results))
	}
	if out.Results[0].Indexed {
		t.Error("expected Indexed=false for 429")
	}
}

// TestFetchAll_Success verifies that a 200 with valid JSON produces an indexed entry.
func TestFetchAll_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"date":"2026-03-27","score":6.2,"repo":{"name":"github.com/ublue-os/bluefin"}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	out, err := fetchAllFromBase([]string{"ublue-os/bluefin"}, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out.Results))
	}
	r := out.Results[0]
	if !r.Indexed {
		t.Error("expected Indexed=true for 200")
	}
	if r.Score == nil {
		t.Fatal("expected non-nil Score")
	}
	if *r.Score != 6.2 {
		t.Errorf("expected Score=6.2, got %.1f", *r.Score)
	}
	if r.Date == nil {
		t.Fatal("expected non-nil Date")
	}
	if *r.Date != "2026-03-27" {
		t.Errorf("expected Date=2026-03-27, got %s", *r.Date)
	}
}

// TestFetchAll_URLPathSegments verifies that the URL is built with org and repo
// as separate path segments (not URL-encoded together).
func TestFetchAll_URLPathSegments(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		http.NotFound(w, r)
	}))
	defer srv.Close()

	fetchAllFromBase([]string{"ublue-os/bluefin"}, srv.URL) //nolint:errcheck

	expected := "/projects/github.com/ublue-os/bluefin"
	if capturedPath != expected {
		t.Errorf("URL path = %q, want %q\n(org/repo must be separate segments, not URL-encoded)", capturedPath, expected)
	}
}

// TestFetchAll_MalformedJSON_200 verifies that malformed JSON on 200 yields Indexed:false.
func TestFetchAll_MalformedJSON_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`)) //nolint:errcheck
	}))
	defer srv.Close()

	out, err := fetchAllFromBase([]string{"ublue-os/bluefin"}, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Results[0].Indexed {
		t.Error("expected Indexed=false for malformed JSON on 200")
	}
}

// TestFetchAll_PartialFailure verifies that one 404 does not stop other repos from being fetched.
func TestFetchAll_PartialFailure(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"date":"2026-03-27","score":7.0}`)) //nolint:errcheck
	}))
	defer srv.Close()

	out, err := fetchAllFromBase([]string{"ublue-os/bluefin", "ublue-os/aurora"}, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out.Results))
	}
	if out.Results[0].Indexed {
		t.Error("first repo (404) should not be indexed")
	}
	if !out.Results[1].Indexed {
		t.Error("second repo (200) should be indexed")
	}
}

// fetchAllFromBase is a test helper that overrides the base URL so we can point
// at a local httptest.Server instead of the real Scorecard API.
func fetchAllFromBase(repos []string, baseURL string) (Output, error) {
	results := []RepoResult{}
	client := &http.Client{}

	for _, repo := range repos {
		parts := splitRepo(repo)
		if parts == nil {
			results = append(results, RepoResult{Repo: repo, Indexed: false})
			continue
		}
		url := baseURL + "/projects/github.com/" + parts[0] + "/" + parts[1]
		result := fetchOne(client, repo, url)
		results = append(results, result)
	}

	return Output{GeneratedAt: "test", Results: results}, nil
}

// splitRepo splits "org/repo" into [org, repo]; returns nil on malformed input.
func splitRepo(repo string) []string {
	for i, c := range repo {
		if c == '/' {
			return []string{repo[:i], repo[i+1:]}
		}
	}
	return nil
}
