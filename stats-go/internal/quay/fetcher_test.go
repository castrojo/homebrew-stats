package quay

// White-box tests for the quay fetcher.
// All HTTP calls are intercepted via a redirectTransport that swaps the quay.io
// host for a local httptest.Server, so no real network is touched.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Pure-function tests (no HTTP needed)
// ---------------------------------------------------------------------------

func TestSumLast(t *testing.T) {
	stats := []DailyStat{
		{Date: "2024-01-01", Count: 10},
		{Date: "2024-01-02", Count: 20},
		{Date: "2024-01-03", Count: 30},
		{Date: "2024-01-04", Count: 40},
		{Date: "2024-01-05", Count: 50},
	}

	cases := []struct {
		n    int
		want int
	}{
		{1, 50},
		{2, 90},  // 40+50
		{3, 120}, // 30+40+50
		{5, 150}, // all
		{10, 150}, // clamps to len
		{0, 0},
	}
	for _, c := range cases {
		got := sumLast(stats, c.n)
		if got != c.want {
			t.Errorf("sumLast(n=%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestSumLast_Empty(t *testing.T) {
	if got := sumLast(nil, 7); got != 0 {
		t.Errorf("sumLast(nil,7) = %d, want 0", got)
	}
}

func TestMin(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{3, 7, 3},
		{7, 3, 3},
		{5, 5, 5},
		{0, 1, 0},
	}
	for _, c := range cases {
		if got := min(c.a, c.b); got != c.want {
			t.Errorf("min(%d,%d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// HTTP-mock infrastructure
// ---------------------------------------------------------------------------

// redirectTransport rewrites every request URL so only the path+query are kept
// and the scheme+host are replaced with the test server's URL.
// This lets us intercept the hardcoded quay.io calls without changing prod code.
type redirectTransport struct {
	target *url.URL
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL = &url.URL{
		Scheme:   rt.target.Scheme,
		Host:     rt.target.Host,
		Path:     req.URL.Path,
		RawQuery: req.URL.RawQuery,
	}
	req2.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(req2)
}

// withMockServer temporarily replaces the package-level HTTP client and
// runs fn. The test server handler receives all quay.io requests with the
// same path/query as the real code would send.
func withMockServer(t *testing.T, handler http.Handler, fn func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	origClient := client
	client = &http.Client{Transport: &redirectTransport{target: target}}
	t.Cleanup(func() { client = origClient })

	fn()
}

// statsJSON returns a minimal /api/v1/repository/{ns}/{name}?includeStats=true payload.
func statsJSON(stats []DailyStat) []byte {
	type row struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	type payload struct {
		Stats []row `json:"stats"`
	}
	var rows []row
	for _, s := range stats {
		rows = append(rows, row{Date: s.Date, Count: s.Count})
	}
	b, _ := json.Marshal(payload{Stats: rows})
	return b
}

// tagsJSON returns a minimal /api/v1/repository/{ns}/{name}/tag/ payload.
func tagsJSON(tags []map[string]any) []byte {
	type payload struct {
		Tags []map[string]any `json:"tags"`
	}
	b, _ := json.Marshal(payload{Tags: tags})
	return b
}

// quayHandler builds an http.Handler that serves quay API paths.
// Paths handled:
//   - /v2/... → 404 (verifyBootcLabel fails gracefully, non-fatal)
//   - /api/v1/repository/{ns}/{name}/tag/ → tagsBody
//   - /api/v1/repository/{ns}/{name}       → statsBody
func quayHandler(statsBody, tagsBody []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case strings.HasPrefix(p, "/v2/"):
			// verifyBootcLabel calls — return 404 so it gracefully returns false.
			http.NotFound(w, r)
		case strings.Contains(p, "/tag/"):
			w.Header().Set("Content-Type", "application/json")
			w.Write(tagsBody) //nolint:errcheck
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Write(statsBody) //nolint:errcheck
		}
	}
}

// ---------------------------------------------------------------------------
// HTTP mock tests — FetchAll
// ---------------------------------------------------------------------------

func TestFetchAll_PullCountAggregation(t *testing.T) {
	daily := []DailyStat{
		{Date: "2024-01-01", Count: 100},
		{Date: "2024-01-02", Count: 200},
		{Date: "2024-01-03", Count: 300},
	}
	handler := quayHandler(
		statsJSON(daily),
		tagsJSON(nil),
	)

	withMockServer(t, handler, func() {
		data, err := FetchAll([]RepoConfig{
			{Namespace: "testns", Name: "testrepo", Label: "Test"},
		})
		if err != nil {
			t.Fatalf("FetchAll error: %v", err)
		}
		if len(data.Repos) != 1 {
			t.Fatalf("expected 1 repo, got %d", len(data.Repos))
		}
		rs := data.Repos[0]
		// 3 days: sumLast(3) = 600, sumLast(7) = 600, sumLast(30) = 600, sumLast(90) = 600
		if rs.Pulls90d != 600 {
			t.Errorf("Pulls90d = %d, want 600", rs.Pulls90d)
		}
		if rs.Pulls7d != 600 {
			t.Errorf("Pulls7d = %d, want 600", rs.Pulls7d)
		}
		if rs.LatestDate != "2024-01-03" {
			t.Errorf("LatestDate = %q, want 2024-01-03", rs.LatestDate)
		}
		if rs.LatestPulls != 300 {
			t.Errorf("LatestPulls = %d, want 300", rs.LatestPulls)
		}
	})
}

func TestFetchAll_CombinedDailyAggregation(t *testing.T) {
	// Two repos sharing the same dates: combined daily should sum.
	daily := []DailyStat{
		{Date: "2024-02-01", Count: 50},
		{Date: "2024-02-02", Count: 75},
	}
	statsBody := statsJSON(daily)
	handler := quayHandler(statsBody, tagsJSON(nil))

	withMockServer(t, handler, func() {
		repos := []RepoConfig{
			{Namespace: "ns", Name: "repo-a", Label: "A"},
			{Namespace: "ns", Name: "repo-b", Label: "B"},
		}
		data, err := FetchAll(repos)
		if err != nil {
			t.Fatalf("FetchAll error: %v", err)
		}
		if data.TotalPulls90d != 250 {
			t.Errorf("TotalPulls90d = %d, want 250 (2 repos × 125)", data.TotalPulls90d)
		}
		// CombinedDaily must be sorted by date and sums each day across repos.
		if len(data.CombinedDaily) != 2 {
			t.Fatalf("CombinedDaily len = %d, want 2", len(data.CombinedDaily))
		}
		if data.CombinedDaily[0].Count != 100 {
			t.Errorf("CombinedDaily[0].Count = %d, want 100", data.CombinedDaily[0].Count)
		}
		if data.CombinedDaily[1].Count != 150 {
			t.Errorf("CombinedDaily[1].Count = %d, want 150", data.CombinedDaily[1].Count)
		}
	})
}

func TestFetchAll_StreamDetection_SkipsSHA256AndVersioned(t *testing.T) {
	rawTags := []map[string]any{
		{"name": "stable", "last_modified": "2024-01-01", "manifest_digest": "sha256:aabbccdd1234567890", "is_manifest_list": true, "child_manifest_count": 2},
		{"name": "latest", "last_modified": "2024-01-01", "manifest_digest": "sha256:aabbccdd1234567890", "is_manifest_list": false, "child_manifest_count": 0},
		// sha256- prefix tag — must be skipped
		{"name": "sha256-abc123.att", "last_modified": "2024-01-01", "manifest_digest": "sha256:abc", "is_manifest_list": false, "child_manifest_count": 0},
		// versioned tag (starts with digit) — must be skipped
		{"name": "41.20240101.0", "last_modified": "2024-01-01", "manifest_digest": "sha256:def", "is_manifest_list": false, "child_manifest_count": 0},
	}
	handler := quayHandler(statsJSON(nil), tagsJSON(rawTags))

	withMockServer(t, handler, func() {
		data, err := FetchAll([]RepoConfig{
			{Namespace: "ns", Name: "repo", Label: "Repo"},
		})
		if err != nil {
			t.Fatalf("FetchAll error: %v", err)
		}
		streams := data.Repos[0].Streams
		if len(streams) != 2 {
			names := make([]string, len(streams))
			for i, s := range streams {
				names[i] = s.Name
			}
			t.Fatalf("expected 2 named streams (stable, latest), got %d: %v", len(streams), names)
		}
		for _, s := range streams {
			if strings.HasPrefix(s.Name, "sha256-") {
				t.Errorf("sha256- tag %q must be filtered out", s.Name)
			}
			if len(s.Name) > 0 && s.Name[0] >= '0' && s.Name[0] <= '9' {
				t.Errorf("versioned tag %q must be filtered out", s.Name)
			}
		}
	})
}

func TestFetchAll_StreamArchCount_ManifestList(t *testing.T) {
	// is_manifest_list=true with child_manifest_count=3 → ArchCount=3
	// is_manifest_list=false → ArchCount=1 regardless of child_manifest_count
	rawTags := []map[string]any{
		{"name": "stable", "last_modified": "2024-01-01", "manifest_digest": "sha256:aaaaaaaaaaaaaaaaaaa", "is_manifest_list": true, "child_manifest_count": 3},
		{"name": "latest", "last_modified": "2024-01-01", "manifest_digest": "sha256:bbbbbbbbbbbbbbbbbbb", "is_manifest_list": false, "child_manifest_count": 5},
	}
	handler := quayHandler(statsJSON(nil), tagsJSON(rawTags))

	withMockServer(t, handler, func() {
		data, err := FetchAll([]RepoConfig{
			{Namespace: "ns", Name: "repo", Label: "Repo"},
		})
		if err != nil {
			t.Fatalf("FetchAll error: %v", err)
		}
		streams := data.Repos[0].Streams
		streamMap := make(map[string]Stream, len(streams))
		for _, s := range streams {
			streamMap[s.Name] = s
		}
		if streamMap["stable"].ArchCount != 3 {
			t.Errorf("stable ArchCount = %d, want 3", streamMap["stable"].ArchCount)
		}
		if streamMap["latest"].ArchCount != 1 {
			t.Errorf("latest ArchCount = %d, want 1 (single manifest)", streamMap["latest"].ArchCount)
		}
	})
}

func TestFetchAll_DigestTruncated(t *testing.T) {
	// Digest must be truncated to 19 chars ("sha256:abcdef01234")
	rawTags := []map[string]any{
		{"name": "stable", "last_modified": "2024-01-01", "manifest_digest": "sha256:abcdef0123456789EXTRA", "is_manifest_list": false, "child_manifest_count": 0},
	}
	handler := quayHandler(statsJSON(nil), tagsJSON(rawTags))

	withMockServer(t, handler, func() {
		data, err := FetchAll([]RepoConfig{
			{Namespace: "ns", Name: "repo", Label: "Repo"},
		})
		if err != nil {
			t.Fatalf("FetchAll error: %v", err)
		}
		if len(data.Repos[0].Streams) != 1 {
			t.Fatalf("expected 1 stream")
		}
		digest := data.Repos[0].Streams[0].DigestShort
		if len(digest) != 19 {
			t.Errorf("DigestShort len = %d, want 19; got %q", len(digest), digest)
		}
	})
}

func TestFetchAll_Non200StatsEndpoint_ReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/") {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})

	withMockServer(t, handler, func() {
		_, err := FetchAll([]RepoConfig{
			{Namespace: "ns", Name: "repo", Label: "Repo"},
		})
		if err == nil {
			t.Fatal("expected an error for non-200 stats response, got nil")
		}
	})
}

func TestFetchAll_MalformedStatsJSON_ReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/") {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(r.URL.Path, "/tag/") {
			w.Write(tagsJSON(nil)) //nolint:errcheck
			return
		}
		w.Write([]byte(`not valid json`)) //nolint:errcheck
	})

	withMockServer(t, handler, func() {
		_, err := FetchAll([]RepoConfig{
			{Namespace: "ns", Name: "repo", Label: "Repo"},
		})
		if err == nil {
			t.Fatal("expected error for malformed stats JSON, got nil")
		}
	})
}

func TestFetchAll_MalformedTagsJSON_ReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/") {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(r.URL.Path, "/tag/") {
			w.Write([]byte(`{invalid`)) //nolint:errcheck
			return
		}
		w.Write(statsJSON(nil)) //nolint:errcheck
	})

	withMockServer(t, handler, func() {
		_, err := FetchAll([]RepoConfig{
			{Namespace: "ns", Name: "repo", Label: "Repo"},
		})
		if err == nil {
			t.Fatal("expected error for malformed tags JSON, got nil")
		}
	})
}

func TestFetchAll_EmptyStats_NoPanic(t *testing.T) {
	handler := quayHandler(statsJSON(nil), tagsJSON(nil))

	withMockServer(t, handler, func() {
		data, err := FetchAll([]RepoConfig{
			{Namespace: "ns", Name: "repo", Label: "Repo"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rs := data.Repos[0]
		if rs.Pulls7d != 0 || rs.Pulls30d != 0 || rs.Pulls90d != 0 {
			t.Errorf("all pull counts should be 0 for empty stats")
		}
		if rs.LatestDate != "" || rs.LatestPulls != 0 {
			t.Errorf("LatestDate/LatestPulls should be zero for empty stats")
		}
	})
}

func TestFetchAll_AvgDailyCalculation(t *testing.T) {
	// 7 days of 10 pulls each → avg7d = 70/7 = 10, avg30d = 70/7 = 10 (only 7 days)
	var daily []DailyStat
	for i := 1; i <= 7; i++ {
		daily = append(daily, DailyStat{Date: "2024-01-" + twoDigit(i), Count: 10})
	}
	handler := quayHandler(statsJSON(daily), tagsJSON(nil))

	withMockServer(t, handler, func() {
		data, err := FetchAll([]RepoConfig{
			{Namespace: "ns", Name: "repo", Label: "Repo"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rs := data.Repos[0]
		if rs.AvgDaily7d != 10 {
			t.Errorf("AvgDaily7d = %d, want 10", rs.AvgDaily7d)
		}
		if rs.AvgDaily30d != 10 {
			t.Errorf("AvgDaily30d = %d, want 10 (7/7 days available)", rs.AvgDaily30d)
		}
	})
}

func TestFetchAll_RepoLabel(t *testing.T) {
	handler := quayHandler(statsJSON(nil), tagsJSON(nil))

	withMockServer(t, handler, func() {
		data, err := FetchAll([]RepoConfig{
			{Namespace: "testns", Name: "testname", Label: "My Label"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rs := data.Repos[0]
		if rs.Label != "My Label" {
			t.Errorf("Label = %q, want %q", rs.Label, "My Label")
		}
		if rs.Namespace != "testns" {
			t.Errorf("Namespace = %q, want %q", rs.Namespace, "testns")
		}
		if rs.Repo != "testname" {
			t.Errorf("Repo = %q, want %q", rs.Repo, "testname")
		}
	})
}

// twoDigit formats an int 1-31 as a two-digit string.
func twoDigit(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
