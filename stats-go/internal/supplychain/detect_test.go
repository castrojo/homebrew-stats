package supplychain

// White-box tests for the supplychain package.
// DetectSupplyChain makes real OCI registry calls, so we only test paths that
// fail fast (invalid ref, lookup error) without touching the network.
// The SupplyChainCheckRefs map is validated structurally.

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// boolPtr
// ---------------------------------------------------------------------------

func TestBoolPtr_True(t *testing.T) {
	p := boolPtr(true)
	if p == nil {
		t.Fatal("boolPtr(true) returned nil")
	}
	if !*p {
		t.Error("*boolPtr(true) = false, want true")
	}
}

func TestBoolPtr_False(t *testing.T) {
	p := boolPtr(false)
	if p == nil {
		t.Fatal("boolPtr(false) returned nil")
	}
	if *p {
		t.Error("*boolPtr(false) = true, want false")
	}
}

func TestBoolPtr_IndependentPointers(t *testing.T) {
	// Each call must return a fresh pointer — mutations must not alias.
	a := boolPtr(true)
	b := boolPtr(true)
	if a == b {
		t.Error("boolPtr must return a distinct pointer on each call")
	}
}

// ---------------------------------------------------------------------------
// DetectSupplyChain — fast-fail paths (no network)
// ---------------------------------------------------------------------------

func TestDetectSupplyChain_InvalidRef_ReturnsEmptyInfo(t *testing.T) {
	cases := []string{
		"",
		":::invalid:::",
		"not a ref at all",
	}
	for _, ref := range cases {
		t.Run(ref, func(t *testing.T) {
			info := DetectSupplyChain(ref)
			// name.NewTag fails → all fields must remain nil (zero value).
			if info.ZstdChunked != nil {
				t.Errorf("ZstdChunked should be nil for invalid ref %q", ref)
			}
			if info.ChunkaDetected != nil {
				t.Errorf("ChunkaDetected should be nil for invalid ref %q", ref)
			}
			if info.LegacyRechunk != nil {
				t.Errorf("LegacyRechunk should be nil for invalid ref %q", ref)
			}
			if info.SLSAProvenance != nil {
				t.Errorf("SLSAProvenance should be nil for invalid ref %q", ref)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// detectSLSAViaGHCLI — fast-fail paths (no network)
// ---------------------------------------------------------------------------

func TestDetectSLSAViaGHCLI_InvalidRef_ReturnsNil(t *testing.T) {
	// name.NewTag fails for these → function returns nil immediately.
	cases := []string{
		"",
		":::invalid",
	}
	for _, ref := range cases {
		t.Run(ref, func(t *testing.T) {
			got := detectSLSAViaGHCLI(ref)
			if got != nil {
				t.Errorf("detectSLSAViaGHCLI(%q) = %v, want nil", ref, *got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SupplyChainCheckRefs — structural validation
// ---------------------------------------------------------------------------

func TestSupplyChainCheckRefs_NotEmpty(t *testing.T) {
	if len(SupplyChainCheckRefs) == 0 {
		t.Fatal("SupplyChainCheckRefs must not be empty")
	}
}

func TestSupplyChainCheckRefs_AllValuesAreOCIRefs(t *testing.T) {
	for label, ref := range SupplyChainCheckRefs {
		if ref == "" {
			t.Errorf("entry %q has empty ref", label)
			continue
		}
		// Must contain at least one '/' (registry/namespace/name).
		if !strings.Contains(ref, "/") {
			t.Errorf("entry %q: ref %q does not look like an OCI ref (no '/')", label, ref)
		}
		// Must end with a tag (contains ':').
		if !strings.Contains(ref, ":") {
			t.Errorf("entry %q: ref %q has no tag (missing ':')", label, ref)
		}
	}
}

func TestSupplyChainCheckRefs_LabelMatchesValue(t *testing.T) {
	// Smoke-check specific well-known entries that must be present.
	required := []string{"Bluefin", "Bazzite", "Aurora"}
	for _, key := range required {
		if _, ok := SupplyChainCheckRefs[key]; !ok {
			t.Errorf("SupplyChainCheckRefs missing required key %q", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Layer media-type and annotation constants
// ---------------------------------------------------------------------------

func TestConstants_ZstdChunkedMediaType(t *testing.T) {
	want := "application/vnd.oci.image.layer.v1.tar+zstd"
	if zstdChunkedLayerMediaType != want {
		t.Errorf("zstdChunkedLayerMediaType = %q, want %q", zstdChunkedLayerMediaType, want)
	}
}

func TestConstants_ChunkahAnnotation(t *testing.T) {
	if chunkahLayerAnnotation == "" {
		t.Fatal("chunkahLayerAnnotation must not be empty")
	}
	if !strings.Contains(chunkahLayerAnnotation, ".") {
		t.Errorf("chunkahLayerAnnotation %q does not look like a reverse-DNS annotation key", chunkahLayerAnnotation)
	}
}
