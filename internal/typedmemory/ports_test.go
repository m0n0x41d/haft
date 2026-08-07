package typedmemory

import "testing"

func TestResolvedStrongReferenceCarriesStableEntityIdentity(t *testing.T) {
	refKind := portsTestRefKind(t)
	var err error
	referenceID, err := NewReferenceID("entity:authorization-service")
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	reference, err := NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}
	entity, err := NewEntityID("entity:authorization-service")
	if err != nil {
		t.Fatalf("NewEntityID: %v", err)
	}
	contextRef, err := NewBoundedContextRef("context:software-system")
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	basis := mustResolutionBasisRef(t, "snapshot:reference-resolution-index")

	resolved, err := NewResolvedStrongReference(reference, entity, contextRef, basis)
	if err != nil {
		t.Fatalf("NewResolvedStrongReference: %v", err)
	}
	if resolved.Reference() != reference {
		t.Fatal("resolved reference changed the exact strong reference")
	}
	if resolved.Entity() != entity {
		t.Fatal("resolved reference did not retain the stable EntityID")
	}
	if resolved.Context() != contextRef {
		t.Fatal("resolved reference changed the bounded context")
	}
	if resolved.Basis() != basis {
		t.Fatal("resolved reference did not retain the exact resolution basis")
	}
}

func TestResolvedStrongReferenceRejectsMissingStableEntityIdentity(t *testing.T) {
	refKind := portsTestRefKind(t)
	var err error
	referenceID, err := NewReferenceID("entity:authorization-service")
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	reference, err := NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}
	contextRef, err := NewBoundedContextRef("context:software-system")
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}

	basis := mustResolutionBasisRef(t, "snapshot:reference-resolution-index")
	_, err = NewResolvedStrongReference(reference, EntityID{}, contextRef, basis)
	if err == nil {
		t.Fatal("NewResolvedStrongReference accepted an absent stable EntityID")
	}
}

func TestResolvedStrongReferenceRejectsMissingResolutionBasis(t *testing.T) {
	refKind := portsTestRefKind(t)
	referenceID, err := NewReferenceID("entity:authorization-service")
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	reference, err := NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}
	entity := mustEntityID(t, "entity:authorization-service")
	contextRef, err := NewBoundedContextRef("context:software-system")
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}

	_, err = NewResolvedStrongReference(reference, entity, contextRef, ResolutionBasisRef{})
	if err == nil {
		t.Fatal("NewResolvedStrongReference accepted an absent resolution basis")
	}
}

func TestActiveAssertionCarriesExactResolutionBasis(t *testing.T) {
	assertion, err := NewAssertionID("assertion:episteme-slot-relation")
	if err != nil {
		t.Fatalf("NewAssertionID: %v", err)
	}
	basis := validationTestRule(t)

	state, err := NewActiveAssertion(assertion, basis)
	if err != nil {
		t.Fatalf("NewActiveAssertion: %v", err)
	}
	if state.Assertion() != assertion {
		t.Fatal("active assertion changed the exact assertion ID")
	}
	if state.Basis() != basis {
		t.Fatal("active assertion did not retain the exact resolution basis")
	}
	_, err = NewActiveAssertion(assertion, RuleRef{})
	if err == nil {
		t.Fatal("NewActiveAssertion accepted an absent resolution basis")
	}
}

func portsTestRefKind(t *testing.T) RefKindRef {
	t.Helper()
	digest, err := NewSHA256Digest(
		"sha256:4141414141414141414141414141414141414141414141414141414141414141",
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	typeEnv, err := NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	refKindID, err := NewRefKindID("refkind:entity")
	if err != nil {
		t.Fatalf("NewRefKindID: %v", err)
	}
	refKind, err := NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatalf("NewRefKindRef: %v", err)
	}
	return refKind
}
