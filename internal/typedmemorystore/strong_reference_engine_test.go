package typedmemorystore

import (
	"context"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestExactPersistedStrongReferenceEngineUsesTransactionCorrelatedUniverse(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	revision := typedmemory.NewGraphRevision(9)
	entity := mustGenericEntityID(t, "entity:authorization-service")
	universe, err := newExactPersistedEntityUniverse(
		fixture.base.project,
		fixture.base.context,
		revision,
		[]typedmemory.EntityID{entity},
	)
	if err != nil {
		t.Fatalf("newExactPersistedEntityUniverse: %v", err)
	}
	referenceID, err := typedmemory.NewReferenceID(entity.String())
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	reference, err := typedmemory.NewPersistedRef(fixture.entityRefKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}
	input := newStrongReferenceResolutionInput(
		fixture.base.project,
		fixture.environment,
		revision,
		reference,
		fixture.base.context,
		universe,
	)
	resolution, err := NewExactPersistedStrongReferenceEngine().ResolveStrongReference(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatalf("ResolveStrongReference: %v", err)
	}
	resolved, ok := resolution.(typedmemory.ResolvedStrongReference)
	if !ok || resolved.Entity() != entity {
		t.Fatalf("ResolveStrongReference = %T; want exact entity", resolution)
	}

	absentID, err := typedmemory.NewReferenceID("entity:absent")
	if err != nil {
		t.Fatalf("NewReferenceID(absent): %v", err)
	}
	absentRef, err := typedmemory.NewPersistedRef(fixture.entityRefKind, absentID)
	if err != nil {
		t.Fatalf("NewPersistedRef(absent): %v", err)
	}
	absentInput := newStrongReferenceResolutionInput(
		fixture.base.project,
		fixture.environment,
		revision,
		absentRef,
		fixture.base.context,
		universe,
	)
	absent, err := NewExactPersistedStrongReferenceEngine().ResolveStrongReference(
		context.Background(),
		absentInput,
	)
	if err != nil {
		t.Fatalf("ResolveStrongReference(absent): %v", err)
	}
	if _, ok := absent.(typedmemory.UnresolvedStrongReference); !ok {
		t.Fatalf("absent resolution = %T; want UnresolvedStrongReference", absent)
	}
}
