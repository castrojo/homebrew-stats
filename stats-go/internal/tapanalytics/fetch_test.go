package tapanalytics

// White-box tests for tapanalytics.
// The package uses http.Get (http.DefaultClient) directly, so we intercept
// http.DefaultTransport with a redirectTransport that routes all calls to a
// local httptest.Server — no real network is touched.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// HTTP redirect infrastructure (same pattern as internal/quay/fetcher_test.go)
// ---------------------------------------------------------------------------

// redirectTransport rewrites every request so only the path+query are kept
// and the scheme+host come from the test server. It uses a captured original
// transport to avoid infinite recursion when http.DefaultTransport is swapped.
type redirectTransport struct {
	target *url.URL
	orig   http.RoundTripper // the transport to actually send the request
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
	return rt.orig.RoundTrip(req2)
}

// withMockTransport temporarily replaces http.DefaultTransport so that
// http.Get calls made by the package under test hit the test server instead.
func withMockTransport(t *testing.T, handler http.Handler, fn func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	// Capture the original transport BEFORE replacing it to avoid recursion.
	orig := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{target: target, orig: orig}
	t.Cleanup(func() { http.DefaultTransport = orig })

	fn()
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

// caskPayload builds a cask-install JSON response for a given period.
func caskPayload(items []struct{ Cask, Count string }) []byte {
	type row struct {
		Cask  string `json:"cask"`
		Count string `json:"count"`
	}
	type payload struct {
		Items []row `json:"items"`
	}
	var rows []row
	for _, it := range items {
		rows = append(rows, row{Cask: it.Cask, Count: it.Count})
	}
	b, _ := json.Marshal(payload{Items: rows})
	return b
}

// formulaPayload builds a formula-install JSON response.
func formulaPayload(items []struct{ Formula, Count string }) []byte {
	type row struct {
		Formula string `json:"formula"`
		Count   string `json:"count"`
	}
	type payload struct {
		Items []row `json:"items"`
	}
	var rows []row
	for _, it := range items {
		rows = append(rows, row{Formula: it.Formula, Count: it.Count})
	}
	b, _ := json.Marshal(payload{Items: rows})
	return b
}

// ---------------------------------------------------------------------------
// Fetch (ublue-os prefix filter)
// ---------------------------------------------------------------------------

func TestFetch_FiltersUblueOSPrefix(t *testing.T) {
	// The server serves the same payload for all three period URLs.
	items := []struct{ Cask, Count string }{
		{"ublue-os/tap/jetbrains-toolbox-linux", "1,234"},
		{"ublue-os/tap/another-cask", "500"},
		{"other-tap/some-cask", "9,999"}, // must be filtered out
		{"homebrew/cask/vlc", "100"},      // must be filtered out
	}
	body := caskPayload(items)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	})

	withMockTransport(t, handler, func() {
		result, err := Fetch()
		if err != nil {
			t.Fatalf("Fetch() error: %v", err)
		}

		// Only ublue-os/ prefixed casks should be present.
		for key := range result {
			if len(key) < 9 || key[:9] != "ublue-os/" {
				t.Errorf("unexpected non-ublue-os key in result: %q", key)
			}
		}

		// Each ublue-os cask appears (count repeated for all 3 periods with same body).
		if _, ok := result["ublue-os/tap/jetbrains-toolbox-linux"]; !ok {
			t.Error("expected ublue-os/tap/jetbrains-toolbox-linux in result")
		}
		if _, ok := result["other-tap/some-cask"]; ok {
			t.Error("other-tap/some-cask must be filtered out")
		}
	})
}

func TestFetch_ParsesCommaSeparatedCounts(t *testing.T) {
	items := []struct{ Cask, Count string }{
		{"ublue-os/tap/myapp", "12,345"},
	}
	body := caskPayload(items)

	callN := 0
	// Serve 30d → 12345, 90d → 22345, 365d → 32345
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callN++
		var count string
		switch callN {
		case 1:
			count = "12,345"
		case 2:
			count = "22,345"
		case 3:
			count = "32,345"
		default:
			count = "0"
		}
		_ = body // suppress unused warning
		items2 := []struct{ Cask, Count string }{{"ublue-os/tap/myapp", count}}
		w.Write(caskPayload(items2)) //nolint:errcheck
	})

	withMockTransport(t, handler, func() {
		result, err := Fetch()
		if err != nil {
			t.Fatalf("Fetch() error: %v", err)
		}
		pkg := result["ublue-os/tap/myapp"]
		if pkg.Installs30d != 12345 {
			t.Errorf("Installs30d = %d, want 12345", pkg.Installs30d)
		}
		if pkg.Installs90d != 22345 {
			t.Errorf("Installs90d = %d, want 22345", pkg.Installs90d)
		}
		if pkg.Installs365d != 32345 {
			t.Errorf("Installs365d = %d, want 32345", pkg.Installs365d)
		}
	})
}

func TestFetch_Non200_ReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	withMockTransport(t, handler, func() {
		_, err := Fetch()
		if err == nil {
			t.Fatal("expected error for non-200 response, got nil")
		}
	})
}

func TestFetch_MalformedJSON_ReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not valid}`)) //nolint:errcheck
	})
	withMockTransport(t, handler, func() {
		_, err := Fetch()
		if err == nil {
			t.Fatal("expected error for malformed JSON, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// FetchAllCasks (no prefix filter)
// ---------------------------------------------------------------------------

func TestFetchAllCasks_ReturnsAllEntries(t *testing.T) {
	items := []struct{ Cask, Count string }{
		{"some-tap/cask-a", "100"},
		{"another-tap/cask-b", "200"},
	}
	body := caskPayload(items)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body) //nolint:errcheck
	})

	withMockTransport(t, handler, func() {
		result, err := FetchAllCasks()
		if err != nil {
			t.Fatalf("FetchAllCasks() error: %v", err)
		}
		// No prefix filter — both entries must appear.
		if _, ok := result["some-tap/cask-a"]; !ok {
			t.Error("some-tap/cask-a must appear in FetchAllCasks result")
		}
		if _, ok := result["another-tap/cask-b"]; !ok {
			t.Error("another-tap/cask-b must appear in FetchAllCasks result")
		}
	})
}

func TestFetchAllCasks_Non200_ReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	withMockTransport(t, handler, func() {
		_, err := FetchAllCasks()
		if err == nil {
			t.Fatal("expected error for non-200 response")
		}
	})
}

// ---------------------------------------------------------------------------
// FetchFormulas (formula analytics, no prefix filter)
// ---------------------------------------------------------------------------

func TestFetchFormulas_ParsesFormulaField(t *testing.T) {
	items := []struct{ Formula, Count string }{
		{"neovim", "5,000"},
		{"git", "10,000"},
	}
	body := formulaPayload(items)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body) //nolint:errcheck
	})

	withMockTransport(t, handler, func() {
		result, err := FetchFormulas()
		if err != nil {
			t.Fatalf("FetchFormulas() error: %v", err)
		}
		if result["neovim"].Installs30d != 5000 {
			t.Errorf("neovim Installs30d = %d, want 5000", result["neovim"].Installs30d)
		}
		if result["git"].Installs30d != 10000 {
			t.Errorf("git Installs30d = %d, want 10000", result["git"].Installs30d)
		}
	})
}

func TestFetchFormulas_Non200_ReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	withMockTransport(t, handler, func() {
		_, err := FetchFormulas()
		if err == nil {
			t.Fatal("expected error for non-200 response")
		}
	})
}

// ---------------------------------------------------------------------------
// FetchFormulasWithAliases
// ---------------------------------------------------------------------------

func TestFetchFormulasWithAliases_AliasResolution(t *testing.T) {
	// The function calls: fetchFormulaPeriods (3×) then fetchAliasMap (1×).
	// We route formula analytics to formulaPayload, alias map to formula.json.
	callN := 0

	formulaItems := []struct{ Formula, Count string }{
		{"neovim", "5,000"},
	}
	formulaBody := formulaPayload(formulaItems)

	// Alias map: nvim → neovim
	type aliasEntry struct {
		Name    string   `json:"name"`
		Aliases []string `json:"aliases"`
	}
	aliasBody, _ := json.Marshal([]aliasEntry{
		{Name: "neovim", Aliases: []string{"nvim"}},
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callN++
		if r.URL.Path == "/api/formula.json" {
			w.Write(aliasBody) //nolint:errcheck
			return
		}
		w.Write(formulaBody) //nolint:errcheck
	})

	withMockTransport(t, handler, func() {
		result, err := FetchFormulasWithAliases()
		if err != nil {
			t.Fatalf("FetchFormulasWithAliases() error: %v", err)
		}
		// canonical entry
		if result["neovim"].Installs30d != 5000 {
			t.Errorf("neovim Installs30d = %d, want 5000", result["neovim"].Installs30d)
		}
		// alias entry
		if result["nvim"].Installs30d != 5000 {
			t.Errorf("nvim (alias for neovim) Installs30d = %d, want 5000", result["nvim"].Installs30d)
		}
	})
}

func TestFetchFormulasWithAliases_AliasMapFailure_FallsBackToCanonical(t *testing.T) {
	// If the alias map fetch fails, the function must return canonical-only results
	// (non-fatal fallback documented in source).
	callN := 0

	formulaItems := []struct{ Formula, Count string }{
		{"neovim", "1,000"},
	}
	formulaBody := formulaPayload(formulaItems)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callN++
		if r.URL.Path == "/api/formula.json" {
			// Simulate alias map failure.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(formulaBody) //nolint:errcheck
	})

	withMockTransport(t, handler, func() {
		result, err := FetchFormulasWithAliases()
		if err != nil {
			t.Fatalf("FetchFormulasWithAliases() error: %v (alias failure should be non-fatal)", err)
		}
		if result["neovim"].Installs30d != 1000 {
			t.Errorf("neovim Installs30d = %d, want 1000", result["neovim"].Installs30d)
		}
	})
}

// ---------------------------------------------------------------------------
// PkgInstalls type
// ---------------------------------------------------------------------------

func TestPkgInstalls_ZeroValue(t *testing.T) {
	var p PkgInstalls
	if p.Installs30d != 0 || p.Installs90d != 0 || p.Installs365d != 0 {
		t.Error("zero-value PkgInstalls must have all counts = 0")
	}
}

// ---------------------------------------------------------------------------
// URL construction
// ---------------------------------------------------------------------------

func TestFetch_URLPathsContainPeriod(t *testing.T) {
	// Guard that the three URLs hit /30d.json, /90d.json, /365d.json.
	var paths []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Write(caskPayload(nil)) //nolint:errcheck
	})

	withMockTransport(t, handler, func() {
		Fetch() //nolint:errcheck
	})

	periods := []string{"30d", "90d", "365d"}
	for _, p := range periods {
		found := false
		for _, path := range paths {
			if contains(path, p) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Fetch() did not hit any URL containing %q; paths: %v", p, paths)
		}
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
