package supplychain

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/castrojo/homebrew-stats/internal/builds"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	zstdChunkedLayerMediaType = "application/vnd.oci.image.layer.v1.tar+zstd"
	chunkahLayerAnnotation    = "org.chunkah.component"
	legacyRechunkAnnotation   = "dev.hhd.rechunk.info"

	slsaSigstoreBundleArtifactType = "application/vnd.dev.sigstore.bundle.v0.3+json"
	slsaGithubAttestArtifactType   = "application/vnd.github.attestation+json"
)

type referrersResponse struct {
	Manifests []struct {
		ArtifactType string `json:"artifactType"`
	} `json:"manifests"`
}

// DetectSupplyChain inspects an OCI image ref and returns tri-state supply chain facts.
// Fields are nil when detection could not be completed (e.g., registry/network/auth errors).
func DetectSupplyChain(ref string) builds.ImageSupplyChainInfo {
	info := builds.ImageSupplyChainInfo{}

	tag, err := name.NewTag(ref, name.WeakValidation)
	if err != nil {
		return info
	}

	desc, err := remote.Get(tag, remote.WithAuth(authn.Anonymous))
	if err != nil {
		return info
	}
	img, err := desc.Image()
	if err != nil {
		return info
	}
	manifest, err := img.Manifest()
	if err != nil {
		return info
	}

	zstd := false
	chunka := false
	for _, layer := range manifest.Layers {
		if string(layer.MediaType) == zstdChunkedLayerMediaType {
			zstd = true
		}
		if layer.Annotations != nil && layer.Annotations[chunkahLayerAnnotation] != "" {
			chunka = true
		}
	}

	legacy := manifest.Annotations != nil && manifest.Annotations[legacyRechunkAnnotation] != ""

	info.ZstdChunked = boolPtr(zstd)
	info.ChunkaDetected = boolPtr(chunka)
	info.LegacyRechunk = boolPtr(legacy)

	digest, err := img.Digest()
	if err != nil {
		return info
	}
	info.SLSAProvenance = detectSLSAReferrers(tag, digest)

	return info
}

func detectSLSAReferrers(tag name.Tag, digest v1.Hash) *bool {
	referrersURL := fmt.Sprintf(
		"https://%s/v2/%s/referrers/%s",
		tag.Context().RegistryStr(),
		tag.Context().RepositoryStr(),
		digest.String(),
	)

	req, err := http.NewRequest(http.MethodGet, referrersURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var payload referrersResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}

	for _, m := range payload.Manifests {
		if m.ArtifactType == slsaSigstoreBundleArtifactType || m.ArtifactType == slsaGithubAttestArtifactType {
			return boolPtr(true)
		}
	}
	return boolPtr(false)
}

func boolPtr(v bool) *bool {
	return &v
}
