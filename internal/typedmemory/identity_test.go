package typedmemory

import (
	"bytes"
	"testing"
)

func TestUnknownEntityResolutionCarriesExactQueryAndImmutableNormalizedBasis(t *testing.T) {
	entity := mustEntityID(t, "entity:auth-service")
	contextRef := mustBoundedContextRef(t, "haft-v9-development")
	identityIndex := mustMissingBasis(t, "identity index unavailable")
	contextIndex := mustMissingBasis(t, "context index unavailable")

	resolution, err := NewUnknownEntityResolution(
		entity,
		contextRef,
		[]MissingBasis{identityIndex, contextIndex},
	)
	if err != nil {
		t.Fatalf("NewUnknownEntityResolution: %v", err)
	}
	if resolution.Entity() != entity {
		t.Fatalf("entity = %q, want %q", resolution.Entity().String(), entity.String())
	}
	if resolution.Context() != contextRef {
		t.Fatalf("context = %q, want %q", resolution.Context().String(), contextRef.String())
	}

	basis := resolution.MissingBasis()
	if got := []string{basis[0].String(), basis[1].String()}; got[0] != "context index unavailable" || got[1] != "identity index unavailable" {
		t.Fatalf("normalized missing basis = %#v", got)
	}
	basis[0] = mustMissingBasis(t, "mutated caller copy")
	if resolution.MissingBasis()[0].String() != "context index unavailable" {
		t.Fatal("MissingBasis accessor exposed mutable internal storage")
	}

	var _ EntityResolution = resolution
}

func TestUnknownEntityResolutionRejectsInvalidOrDuplicateBasis(t *testing.T) {
	entity := mustEntityID(t, "entity:auth-service")
	contextRef := mustBoundedContextRef(t, "haft-v9-development")
	basis := mustMissingBasis(t, "identity index unavailable")

	tests := []struct {
		name    string
		entity  EntityID
		context BoundedContextRef
		basis   []MissingBasis
	}{
		{name: "zero entity", context: contextRef, basis: []MissingBasis{basis}},
		{name: "zero context", entity: entity, basis: []MissingBasis{basis}},
		{name: "no missing basis", entity: entity, context: contextRef},
		{name: "zero missing basis", entity: entity, context: contextRef, basis: []MissingBasis{{}}},
		{
			name:    "duplicate missing basis",
			entity:  entity,
			context: contextRef,
			basis:   []MissingBasis{basis, basis},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewUnknownEntityResolution(test.entity, test.context, test.basis); err == nil {
				t.Fatal("NewUnknownEntityResolution accepted invalid input")
			}
		})
	}
}

func TestCandidateEntityResolutionKeepsAlternativesWithoutMerging(t *testing.T) {
	contextRef := mustBoundedContextRef(t, "haft-v9-development")
	basis := mustResolutionBasisRef(t, "exact-alias-index:v1")
	alias := mustEntityAlias(t, "authorization service")

	first, err := NewEntityCandidate(
		mustEntityID(t, "entity:auth-service"),
		[]EntityAlias{alias},
		[]BoundedContextRef{contextRef},
		basis,
	)
	if err != nil {
		t.Fatalf("first candidate: %v", err)
	}
	second, err := NewEntityCandidate(
		mustEntityID(t, "entity:authorization-policy"),
		[]EntityAlias{alias},
		[]BoundedContextRef{contextRef},
		basis,
	)
	if err != nil {
		t.Fatalf("second candidate: %v", err)
	}

	resolution, err := NewCandidateEntityResolution([]EntityCandidate{first, second})
	if err != nil {
		t.Fatalf("candidate resolution: %v", err)
	}
	if len(resolution.Candidates()) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(resolution.Candidates()))
	}
}

func TestMergeEntitiesRequiresReconciliationBasisAndKeepsSurvivorStable(t *testing.T) {
	survivor := mustEntityID(t, "entity:canonical-auth")
	merged := []EntityID{mustEntityID(t, "entity:auth-b"), mustEntityID(t, "entity:auth-a")}
	contextRef := mustBoundedContextRef(t, "haft-v9-development")

	_, err := NewMergeEntities(survivor, merged, contextRef, ReconciliationBasisRef{})
	if err == nil {
		t.Fatal("merge without reconciliation basis succeeded")
	}

	change, err := NewMergeEntities(
		survivor,
		merged,
		contextRef,
		mustReconciliationBasisRef(t, "review:entity-merge-1"),
	)
	if err != nil {
		t.Fatalf("merge entities: %v", err)
	}
	if change.Survivor() != survivor {
		t.Fatalf("survivor = %q, want %q", change.Survivor().String(), survivor.String())
	}
	if got := change.Merged(); got[0].String() != "entity:auth-a" || got[1].String() != "entity:auth-b" {
		t.Fatalf("canonical merged entities = %#v", got)
	}
}

func TestSplitEntityRequiresTwoDistinctTargets(t *testing.T) {
	source := mustEntityID(t, "entity:old-auth")
	contextRef := mustBoundedContextRef(t, "haft-v9-development")
	basis := mustReconciliationBasisRef(t, "review:entity-split-1")

	_, err := NewSplitEntity(
		source,
		[]EntityID{mustEntityID(t, "entity:new-auth")},
		contextRef,
		basis,
	)
	if err == nil {
		t.Fatal("split with one target succeeded")
	}

	_, err = NewSplitEntity(
		source,
		[]EntityID{source, mustEntityID(t, "entity:new-auth")},
		contextRef,
		basis,
	)
	if err == nil {
		t.Fatal("split retaining the source as a target succeeded")
	}
}

func TestReconciliationBasisResolutionRequiresStrongCorrelatedPayload(t *testing.T) {
	basis := mustReconciliationBasisRef(t, "reconciliation:auth-merge")
	contextRef := mustBoundedContextRef(t, "haft-v9-development")
	primary := mustEntityID(t, "entity:canonical-auth")
	related := []EntityID{
		mustEntityID(t, "entity:legacy-auth-b"),
		mustEntityID(t, "entity:legacy-auth-a"),
	}
	typeEnv := typeEnvTestTypeEnvRef(t, 0x93)
	digest := typeEnvTestDigest(t, 0x94)
	provenance := typeEnvTestProvenanceRef(t, "memory:identity-reconciliation-review")

	resolution, err := NewResolvedReconciliationBasis(
		basis,
		ReconciliationMergeEntities,
		contextRef,
		primary,
		related,
		NewGraphRevision(17),
		typeEnv,
		digest,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewResolvedReconciliationBasis: %v", err)
	}
	if got := resolution.Related(); got[0].String() != "entity:legacy-auth-a" || got[1].String() != "entity:legacy-auth-b" {
		t.Fatalf("normalized related entities = %#v", got)
	}
	callerCopy := resolution.Related()
	callerCopy[0] = primary
	if resolution.Related()[0] == primary {
		t.Fatal("Related accessor exposed mutable internal storage")
	}
	if resolution.PayloadDigest() != digest || resolution.Provenance() != provenance {
		t.Fatal("resolved basis lost strong payload digest or provenance")
	}

	_, err = NewResolvedReconciliationBasis(
		basis,
		ReconciliationMergeEntities,
		contextRef,
		primary,
		related,
		NewGraphRevision(17),
		typeEnv,
		SHA256Digest{},
		provenance,
	)
	if err == nil {
		t.Fatal("resolved basis without payload digest succeeded")
	}

	missing, err := NewMissingReconciliationBasis(basis, contextRef)
	if err != nil {
		t.Fatalf("NewMissingReconciliationBasis: %v", err)
	}
	conflicting, err := NewConflictingReconciliationBasis(
		basis,
		contextRef,
		[]SHA256Digest{typeEnvTestDigest(t, 0x95), typeEnvTestDigest(t, 0x96)},
	)
	if err != nil {
		t.Fatalf("NewConflictingReconciliationBasis: %v", err)
	}
	var _ ReconciliationBasisResolution = resolution
	var _ ReconciliationBasisResolution = missing
	var _ ReconciliationBasisResolution = conflicting
}

func TestReviewedIdentityReconciliationAdmissionSealsExactMergeBasis(t *testing.T) {
	survivor := mustEntityID(t, "entity:canonical-auth")
	merged := []EntityID{
		mustEntityID(t, "entity:legacy-auth-b"),
		mustEntityID(t, "entity:legacy-auth-a"),
	}
	contextRef := mustBoundedContextRef(t, "haft-v9-development")
	basisRef := mustReconciliationBasisRef(t, "reconciliation:reviewed-auth-merge")
	change, err := NewMergeEntities(survivor, merged, contextRef, basisRef)
	if err != nil {
		t.Fatalf("NewMergeEntities: %v", err)
	}
	basis, err := NewResolvedReconciliationBasis(
		basisRef,
		ReconciliationMergeEntities,
		contextRef,
		survivor,
		merged,
		NewGraphRevision(23),
		typeEnvTestTypeEnvRef(t, 0xa1),
		typeEnvTestDigest(t, 0xa2),
		typeEnvTestProvenanceRef(t, "memory:reviewed-auth-merge"),
	)
	if err != nil {
		t.Fatalf("NewResolvedReconciliationBasis: %v", err)
	}
	admission, err := NewReviewedIdentityReconciliationAdmission(change, basis)
	if err != nil {
		t.Fatalf("NewReviewedIdentityReconciliationAdmission: %v", err)
	}
	canonical := admission.CanonicalBytes()
	if len(canonical) == 0 || admission.Digest().String() == "" {
		t.Fatal("reviewed admission omitted canonical bytes or digest")
	}
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, admission.CanonicalBytes()) {
		t.Fatal("CanonicalBytes exposed mutable internal storage")
	}
	if err := VerifyStoredReviewedIdentityReconciliation(
		change,
		basis,
		admission.CanonicalBytes(),
		admission.Digest(),
	); err != nil {
		t.Fatalf("VerifyStoredReviewedIdentityReconciliation: %v", err)
	}
	corrupt := admission.CanonicalBytes()
	corrupt[len(corrupt)-1] ^= 0xff
	if err := VerifyStoredReviewedIdentityReconciliation(
		change,
		basis,
		corrupt,
		admission.Digest(),
	); err == nil {
		t.Fatal("stored reviewed admission accepted corrupt bytes")
	}
}

func TestReviewedIdentityReconciliationAdmissionRejectsMismatchedOrUnreviewedEffect(t *testing.T) {
	survivor := mustEntityID(t, "entity:canonical-auth")
	merged := []EntityID{mustEntityID(t, "entity:legacy-auth")}
	contextRef := mustBoundedContextRef(t, "haft-v9-development")
	basisRef := mustReconciliationBasisRef(t, "reconciliation:reviewed-auth-merge")
	change, err := NewMergeEntities(survivor, merged, contextRef, basisRef)
	if err != nil {
		t.Fatalf("NewMergeEntities: %v", err)
	}
	mismatched, err := NewResolvedReconciliationBasis(
		mustReconciliationBasisRef(t, "reconciliation:different-review"),
		ReconciliationMergeEntities,
		contextRef,
		survivor,
		merged,
		NewGraphRevision(23),
		typeEnvTestTypeEnvRef(t, 0xa3),
		typeEnvTestDigest(t, 0xa4),
		typeEnvTestProvenanceRef(t, "memory:different-review"),
	)
	if err != nil {
		t.Fatalf("mismatched NewResolvedReconciliationBasis: %v", err)
	}
	if _, err := NewReviewedIdentityReconciliationAdmission(change, mismatched); err == nil {
		t.Fatal("reviewed admission accepted a different basis reference")
	}
	alias, err := NewAdmitAlias(
		survivor,
		mustEntityAlias(t, "canonical-auth"),
		contextRef,
		typeEnvTestProvenanceRef(t, "memory:alias"),
	)
	if err != nil {
		t.Fatalf("NewAdmitAlias: %v", err)
	}
	if _, err := NewReviewedIdentityReconciliationAdmission(alias, mismatched); err == nil {
		t.Fatal("reviewed merge/split admission accepted an alias effect")
	}
}

func TestKindIdentifierDoesNotUseEntityOfConcernWordBan(t *testing.T) {
	_, err := NewKindID("EntityOfConcern")
	if err != nil {
		t.Fatalf("lexical KindID parser must not encode an ontology word ban: %v", err)
	}
}

func mustEntityID(t *testing.T, raw string) EntityID {
	t.Helper()
	value, err := NewEntityID(raw)
	if err != nil {
		t.Fatalf("entity ID %q: %v", raw, err)
	}
	return value
}

func mustMissingBasis(t *testing.T, raw string) MissingBasis {
	t.Helper()
	value, err := NewMissingBasis(raw)
	if err != nil {
		t.Fatalf("missing basis %q: %v", raw, err)
	}
	return value
}

func mustBoundedContextRef(t *testing.T, raw string) BoundedContextRef {
	t.Helper()
	value, err := NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("bounded context %q: %v", raw, err)
	}
	return value
}

func mustResolutionBasisRef(t *testing.T, raw string) ResolutionBasisRef {
	t.Helper()
	value, err := NewResolutionBasisRef(raw)
	if err != nil {
		t.Fatalf("resolution basis %q: %v", raw, err)
	}
	return value
}

func mustReconciliationBasisRef(t *testing.T, raw string) ReconciliationBasisRef {
	t.Helper()
	value, err := NewReconciliationBasisRef(raw)
	if err != nil {
		t.Fatalf("reconciliation basis %q: %v", raw, err)
	}
	return value
}

func mustEntityAlias(t *testing.T, raw string) EntityAlias {
	t.Helper()
	value, err := NewEntityAlias(raw)
	if err != nil {
		t.Fatalf("entity alias %q: %v", raw, err)
	}
	return value
}
