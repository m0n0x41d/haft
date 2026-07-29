package typedmemorystore

import (
	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// These aliases keep the storage shell as the authority that observes one
// transaction-correlated universe while the immutable value algebra lives
// below both storage and runtime dispatch. This direction avoids a runtime ->
// store dependency cycle.
type PersistedEntityUniverse = memberofevaluation.PersistedEntityUniverse
type ExactPersistedEntityUniverse = memberofevaluation.ExactPersistedEntityUniverse
type PersistedEntityUniverseUnavailable = memberofevaluation.PersistedEntityUniverseUnavailable

func newExactPersistedEntityUniverse(
	project projectledger.ProjectID,
	contextRef typedmemory.BoundedContextRef,
	revision typedmemory.GraphRevision,
	members []typedmemory.EntityID,
) (ExactPersistedEntityUniverse, error) {
	return memberofevaluation.NewExactPersistedEntityUniverse(
		project,
		contextRef,
		revision,
		members,
	)
}
