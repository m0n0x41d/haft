package typedmemorystore

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ExactPersistedStrongReferenceEngine resolves only a stable persisted
// reference whose EntityID is present in the transaction-correlated entity
// universe. It performs no IO and cannot broaden the universe supplied by the
// storage shell.
type ExactPersistedStrongReferenceEngine struct{}

func NewExactPersistedStrongReferenceEngine() ExactPersistedStrongReferenceEngine {
	return ExactPersistedStrongReferenceEngine{}
}

func (ExactPersistedStrongReferenceEngine) ResolveStrongReference(
	ctx context.Context,
	input StrongReferenceResolutionInput,
) (typedmemory.StrongReferenceResolution, error) {
	if ctx == nil {
		return nil, fmt.Errorf("resolve persisted strong reference: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateStrongReferenceResolutionInput(input); err != nil {
		return nil, err
	}
	entity, err := typedmemory.NewEntityID(input.reference.ReferenceID().String())
	if err != nil || !input.universe.(ExactPersistedEntityUniverse).Contains(entity) {
		return unresolvedStrongReference(input)
	}
	basis, err := snapshotResolutionBasis(input.project, input.graphRevision)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewResolvedStrongReference(
		input.reference,
		entity,
		input.context,
		basis,
	)
}

func validateStrongReferenceResolutionInput(
	input StrongReferenceResolutionInput,
) error {
	project, err := projectledger.ParseProjectID(input.project.String())
	if err != nil || project != input.project {
		return fmt.Errorf("resolve persisted strong reference: exact project is required")
	}
	if input.environment.Ref().String() == "" ||
		input.reference.RefKind().TypeEnv() != input.environment.Ref() {
		return fmt.Errorf("resolve persisted strong reference: active TypeEnv does not own the RefKind")
	}
	if _, found := input.environment.RefKindDefinition(input.reference.RefKind()); !found {
		return fmt.Errorf("resolve persisted strong reference: RefKind is absent from the active TypeEnv")
	}
	if _, found := input.environment.BoundedContext(input.context); !found {
		return fmt.Errorf("resolve persisted strong reference: bounded context is absent from the active TypeEnv")
	}
	exact, ok := input.universe.(ExactPersistedEntityUniverse)
	if !ok || !exact.Valid() ||
		exact.ProjectID() != input.project ||
		exact.BoundedContext() != input.context ||
		exact.GraphRevision() != input.graphRevision {
		return fmt.Errorf("resolve persisted strong reference: exact persisted entity universe is required")
	}
	return nil
}

func unresolvedStrongReference(
	input StrongReferenceResolutionInput,
) (typedmemory.StrongReferenceResolution, error) {
	repair, err := typedmemory.NewRepairPointer(referenceSnapshotRepair)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewUnresolvedStrongReference(
		input.reference,
		input.context,
		repair,
	)
}
