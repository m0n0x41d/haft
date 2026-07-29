package typedmemorystore

import (
	"bytes"
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestExactPersistedEntityUniverseCanonicalizesAndDefensivelyCopies(t *testing.T) {
	projectValue, projectErr := projectledger.ParseProjectID(
		"qnt_e1717001",
	)
	project := mustPersistedEntityUniverseValue(t, projectValue, projectErr)
	contextValue, contextErr := typedmemory.NewBoundedContextRef(
		"context:persisted-entity-universe",
	)
	contextRef := mustPersistedEntityUniverseValue(t, contextValue, contextErr)
	firstValue, firstErr := typedmemory.NewEntityID("entity:first")
	first := mustPersistedEntityUniverseValue(t, firstValue, firstErr)
	secondValue, secondErr := typedmemory.NewEntityID("entity:second")
	second := mustPersistedEntityUniverseValue(t, secondValue, secondErr)
	input := []typedmemory.EntityID{second, first}
	universeValue, universeErr := newExactPersistedEntityUniverse(
		project,
		contextRef,
		typedmemory.NewGraphRevision(7),
		input,
	)
	universe := mustPersistedEntityUniverseValue(t, universeValue, universeErr)
	input[0] = first

	wantMembers := []typedmemory.EntityID{first, second}
	if !slices.Equal(universe.Members(), wantMembers) {
		t.Fatalf("Members() = %#v, want %#v", universe.Members(), wantMembers)
	}
	if !universe.Valid() {
		t.Fatal("exact persisted entity universe is not self-validating")
	}
	left := universe.Members()
	left[0] = second
	if !slices.Equal(universe.Members(), wantMembers) {
		t.Fatal("Members() leaked mutable backing storage")
	}
	canonical := universe.CanonicalBytes()
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, universe.CanonicalBytes()) {
		t.Fatal("CanonicalBytes() leaked mutable backing storage")
	}
}

func TestExactPersistedEntityUniverseRejectsDuplicateIdentity(t *testing.T) {
	projectValue, projectErr := projectledger.ParseProjectID(
		"qnt_e1717002",
	)
	project := mustPersistedEntityUniverseValue(t, projectValue, projectErr)
	contextValue, contextErr := typedmemory.NewBoundedContextRef(
		"context:persisted-entity-universe-duplicate",
	)
	contextRef := mustPersistedEntityUniverseValue(t, contextValue, contextErr)
	entityValue, entityErr := typedmemory.NewEntityID("entity:duplicate")
	entity := mustPersistedEntityUniverseValue(t, entityValue, entityErr)
	_, err := newExactPersistedEntityUniverse(
		project,
		contextRef,
		typedmemory.NewGraphRevision(0),
		[]typedmemory.EntityID{entity, entity},
	)
	if err == nil {
		t.Fatal("newExactPersistedEntityUniverse() accepted duplicate EntityID")
	}
}

func TestEmptyExactPersistedEntityUniverseIsDistinctFromUnavailable(t *testing.T) {
	projectValue, projectErr := projectledger.ParseProjectID(
		"qnt_e1717003",
	)
	project := mustPersistedEntityUniverseValue(t, projectValue, projectErr)
	contextValue, contextErr := typedmemory.NewBoundedContextRef(
		"context:persisted-entity-universe-empty",
	)
	contextRef := mustPersistedEntityUniverseValue(t, contextValue, contextErr)
	exactValue, exactErr := newExactPersistedEntityUniverse(
		project,
		contextRef,
		typedmemory.NewGraphRevision(0),
		nil,
	)
	exact := mustPersistedEntityUniverseValue(t, exactValue, exactErr)
	var exactPosture PersistedEntityUniverse = exact
	var unavailablePosture PersistedEntityUniverse = memberofevaluation.NewPersistedEntityUniverseUnavailable()
	if _, ok := exactPosture.(ExactPersistedEntityUniverse); !ok {
		t.Fatalf("exact posture = %T", exactPosture)
	}
	if _, ok := unavailablePosture.(PersistedEntityUniverseUnavailable); !ok {
		t.Fatalf("unavailable posture = %T", unavailablePosture)
	}
	if len(exact.Members()) != 0 || !exact.Valid() {
		t.Fatal("empty exact universe is not a valid, defined persisted set")
	}
}

func mustPersistedEntityUniverseValue[T any](
	t *testing.T,
	value T,
	err error,
) T {
	t.Helper()
	if err != nil {
		t.Fatalf("fixture construction: %v", err)
	}
	return value
}
