package projectprofile_test

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestScopedCapabilityApplicabilityRequiresExactScopeInMixedProfile(
	t *testing.T,
) {
	payload := mustCapabilityMatrixPayload(
		t,
		[]projectprofile.RealizationScope{
			mustSoftwareScope(t, "software"),
			mustNonSoftwareScope(t, "documents"),
		},
	)
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}

	software, err := projectprofile.ResolveScopedCapabilityApplicability(
		matrix,
		mustScopeID(t, "software"),
		projectprofile.SWEMethodPackCapability,
	)
	if err != nil {
		t.Fatalf("resolve software capability: %v", err)
	}
	documents, err := projectprofile.ResolveScopedCapabilityApplicability(
		matrix,
		mustScopeID(t, "documents"),
		projectprofile.SWEMethodPackCapability,
	)
	if err != nil {
		t.Fatalf("resolve documents capability: %v", err)
	}

	if software.Kind() != projectprofile.CapabilityRequired {
		t.Fatalf("software kind = %q", software.Kind())
	}
	if documents.Kind() != projectprofile.CapabilityNotApplicable {
		t.Fatalf("documents kind = %q", documents.Kind())
	}
	if software.ProfilePayloadDigest() != matrix.ProfilePayloadDigest() ||
		documents.ProfilePayloadDigest() != matrix.ProfilePayloadDigest() {
		t.Fatal("scoped projections lost canonical profile-payload identity")
	}
}

func TestScopedCapabilityApplicabilityPreservesNeutralUnderdeterminedBasis(
	t *testing.T,
) {
	payload := mustCapabilityMatrixPayload(
		t,
		[]projectprofile.RealizationScope{
			mustNonSoftwareScope(t, "knowledge-model"),
		},
	)
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}

	result, err := projectprofile.ResolveScopedCapabilityApplicability(
		matrix,
		mustScopeID(t, "knowledge-model"),
		projectprofile.TargetSystemSpecCapability,
	)
	if err != nil {
		t.Fatalf("ResolveScopedCapabilityApplicability: %v", err)
	}
	if result.Kind() != projectprofile.CapabilityUnderdetermined {
		t.Fatalf("kind = %q", result.Kind())
	}
	missing, present := result.MissingBasis()
	if !present || missing != projectprofile.MissingAdmittedTargetSystemRelation {
		t.Fatalf("missing basis = %q, present=%t", missing, present)
	}
}

func TestScopedCapabilityApplicabilityRejectsUnknownExactScope(t *testing.T) {
	payload := mustCapabilityMatrixPayload(
		t,
		[]projectprofile.RealizationScope{
			mustSoftwareScope(t, "software"),
		},
	)
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}

	_, err = projectprofile.ResolveScopedCapabilityApplicability(
		matrix,
		mustScopeID(t, "another-scope"),
		projectprofile.SWEMethodPackCapability,
	)
	if err == nil || !strings.Contains(err.Error(), "not present") {
		t.Fatalf("unknown-scope error = %v", err)
	}
}

func TestZeroScopedCapabilityApplicabilityIsInvalid(t *testing.T) {
	if (projectprofile.ScopedCapabilityApplicability{}).Valid() {
		t.Fatal("zero scoped capability applicability is valid")
	}
}
