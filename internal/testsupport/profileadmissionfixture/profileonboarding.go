package profileadmissionfixture

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/profileadmission"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// PrepareSoftwareRevision stores pre-Work support, source-native profile
// authority, its durable pre-Work resolution, and the completed Work DAG
// without consuming the single-use authority. It exists only for
// outer-boundary tests which need to exercise the real admission service.
func (harness *Harness) PrepareSoftwareRevision(
	t testing.TB,
	suffix string,
) profileadmission.ProfileDeclarationAdmissionRequest {
	t.Helper()
	payload := newIntegrationPayload(t, suffix)
	return harness.PrepareRevision(t, suffix, payload)
}

// PrepareRevision is the custom-payload counterpart of PrepareSoftwareRevision.
// The returned request is still only pre-effect input; no profile admission has
// occurred and no canonical admission token has been minted.
func (harness *Harness) PrepareRevision(
	t testing.TB,
	suffix string,
	payload projectprofile.ProfileDeclarationPayload,
) profileadmission.ProfileDeclarationAdmissionRequest {
	t.Helper()
	return harness.prepareV3AdmissionRequest(t, suffix, payload)
}
