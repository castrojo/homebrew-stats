package supplychain

import (
"os/exec"
"strings"

"github.com/castrojo/homebrew-stats/internal/builds"
ghcli "github.com/castrojo/homebrew-stats/internal/ghcli"
"github.com/google/go-containerregistry/pkg/authn"
"github.com/google/go-containerregistry/pkg/name"
"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
zstdChunkedLayerMediaType = "application/vnd.oci.image.layer.v1.tar+zstd"
chunkahLayerAnnotation    = "org.chunkah.component"
legacyRechunkAnnotation   = "dev.hhd.rechunk.info"
)

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

info.SLSAProvenance = detectSLSAViaGHCLI(ref)

return info
}

// detectSLSAViaGHCLI uses `gh attestation verify` to check for SLSA provenance.
// Returns nil if detection is not possible (gh not available, parse error).
// Returns *true if attestation verified, *false if gh ran but found nothing.
func detectSLSAViaGHCLI(imageRef string) *bool {
tag, err := name.NewTag(imageRef, name.WeakValidation)
if err != nil {
return nil
}
// Extract owner from repo path: ghcr.io/ublue-os/bluefin → "ublue-os"
parts := strings.SplitN(tag.Context().RepositoryStr(), "/", 2)
if len(parts) < 1 {
return nil
}
owner := parts[0]

_, err = ghcli.Run("attestation", "verify", imageRef, "--owner", owner)
if err == nil {
return boolPtr(true)
}
// Distinguish "gh not found" from "attestation not found".
if _, lookErr := exec.LookPath("gh"); lookErr != nil {
return nil // gh not available → unknown
}
return boolPtr(false)
}

func boolPtr(v bool) *bool {
return &v
}
