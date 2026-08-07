package typedmemorystore

import (
	"context"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// TypeEnvLoader reconstructs an executable environment from an immutable
// stored snapshot. The SQLite adapter rechecks the returned exact ref before
// using it for transaction-time pure validation.
type TypeEnvLoader interface {
	LoadTypeEnv(TypeEnvSnapshot) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error)
}

type SnapshotPort interface {
	PutTypeEnvSnapshot(context.Context, TypeEnvSnapshot) error
	LoadTypeEnvSnapshot(context.Context, typedmemory.TypeEnvRef) (TypeEnvSnapshot, error)
}

// ProjectGraphInitializationPort exposes only the dormant revision-zero graph
// bootstrap. It cannot admit memory, select a project TypeEnv head, or append a
// graph event.
type ProjectGraphInitializationPort interface {
	InitializeProjectGraphAtBaseTypeEnv(
		context.Context,
		projectledger.ProjectID,
		TypeEnvSnapshot,
	) (ProjectGraphInitializationResult, error)
}

type CommitPort interface {
	CommitMemoryChangeSet(context.Context, CommitRequest) (CommitReceipt, error)
}

type ReplayPort interface {
	ReplayMemoryChangeSet(
		context.Context,
		ReplayRequest,
	) (CommitReceipt, bool, error)
}

// IdempotencyReplayPort resolves a public high-level request by its durable
// idempotency coordinate without requiring the caller to retain or expose the
// internal TypeEnv/revision basis. The store recovers those exact coordinates
// from immutable history before applying the ordinary exact replay verifier.
type IdempotencyReplayPort interface {
	ReplayMemoryChangeSetByIdempotencyKey(
		context.Context,
		IdempotencyReplayRequest,
	) (CommitReceipt, bool, error)
}

type ReadPort interface {
	LoadHead(context.Context, projectledger.ProjectID) (GraphHead, error)
	LoadEntity(
		context.Context,
		projectledger.ProjectID,
		typedmemory.EntityID,
	) (StoredEntity, error)
}

type EntityContextReadPort interface {
	LoadEntityContext(
		context.Context,
		projectledger.ProjectID,
		typedmemory.EntityID,
		typedmemory.BoundedContextRef,
	) (StoredEntity, error)
}
