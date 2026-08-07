package typedmemorystore

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// IdempotencyReplayRequest is the stable request identity available to a
// high-level public intent that deliberately hides snapshot coordinates.
// Stored immutable history supplies the original exact revision and TypeEnv.
type IdempotencyReplayRequest struct {
	contractVersion   AdmissionContractVersion
	project           projectledger.ProjectID
	idempotencyKey    IdempotencyKey
	requestProvenance typedmemory.ProvenanceRef
	candidate         typedmemory.MemoryChangeSet
}

type IdempotencyReplayRequestBuilder struct {
	value              IdempotencyReplayRequest
	contractVersionSet bool
}

func NewIdempotencyReplayRequestBuilder() *IdempotencyReplayRequestBuilder {
	return &IdempotencyReplayRequestBuilder{}
}

func (builder *IdempotencyReplayRequestBuilder) SetContractVersion(
	version AdmissionContractVersion,
) *IdempotencyReplayRequestBuilder {
	builder.value.contractVersion = version
	builder.contractVersionSet = true
	return builder
}

func (builder *IdempotencyReplayRequestBuilder) SetProject(
	project projectledger.ProjectID,
) *IdempotencyReplayRequestBuilder {
	builder.value.project = project
	return builder
}

func (builder *IdempotencyReplayRequestBuilder) SetIdempotencyKey(
	key IdempotencyKey,
) *IdempotencyReplayRequestBuilder {
	builder.value.idempotencyKey = key
	return builder
}

func (builder *IdempotencyReplayRequestBuilder) SetRequestProvenance(
	provenance typedmemory.ProvenanceRef,
) *IdempotencyReplayRequestBuilder {
	builder.value.requestProvenance = provenance
	return builder
}

func (builder *IdempotencyReplayRequestBuilder) SetCandidate(
	candidate typedmemory.MemoryChangeSet,
) *IdempotencyReplayRequestBuilder {
	builder.value.candidate = candidate
	return builder
}

func (builder *IdempotencyReplayRequestBuilder) Build() (
	IdempotencyReplayRequest,
	error,
) {
	if builder == nil ||
		!builder.contractVersionSet ||
		!builder.value.contractVersion.valid() {
		return IdempotencyReplayRequest{}, fmt.Errorf(
			"idempotency replay request contract version must be explicit",
		)
	}
	value := builder.value
	project, err := projectledger.ParseProjectID(value.project.String())
	if err != nil || project != value.project {
		return IdempotencyReplayRequest{}, fmt.Errorf(
			"idempotency replay request project is invalid",
		)
	}
	if value.idempotencyKey.String() == "" ||
		value.requestProvenance.String() == "" {
		return IdempotencyReplayRequest{}, fmt.Errorf(
			"idempotency replay request key and provenance are required",
		)
	}
	if _, err := value.candidate.Digest(); err != nil {
		return IdempotencyReplayRequest{}, fmt.Errorf(
			"idempotency replay request candidate is invalid: %w",
			err,
		)
	}
	return value, nil
}

func (request IdempotencyReplayRequest) ContractVersion() AdmissionContractVersion {
	return request.contractVersion
}

// ReplayMemoryChangeSetByIdempotencyKey first recovers the original exact
// basis from immutable idempotency history, then delegates to the existing
// byte-exact replay verifier. It cannot create a graph event.
func (adapter *SQLiteAdapter) ReplayMemoryChangeSetByIdempotencyKey(
	ctx context.Context,
	request IdempotencyReplayRequest,
) (CommitReceipt, bool, error) {
	if ctx == nil {
		return CommitReceipt{}, false, fmt.Errorf(
			"replay typed-memory change set by idempotency key: context is required",
		)
	}
	if adapter == nil || adapter.database == nil {
		return CommitReceipt{}, false, ErrDatabaseRequired
	}
	if _, err := request.candidate.Digest(); err != nil ||
		!request.contractVersion.valid() ||
		request.project.String() == "" ||
		request.idempotencyKey.String() == "" ||
		request.requestProvenance.String() == "" {
		return CommitReceipt{}, false, fmt.Errorf(
			"replay typed-memory change set by idempotency key: invalid request",
		)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, adapter.database)
	if err != nil {
		return CommitReceipt{}, false, fmt.Errorf(
			"begin typed-memory idempotency replay basis lookup: %w",
			err,
		)
	}
	basis, loadErr := loadIdempotencyBasis(
		ctx,
		transaction,
		request.project,
		request.idempotencyKey,
	)
	finish := transaction.Rollback(ctx)
	if loadErr != nil {
		return CommitReceipt{}, basis.found, joinReplayFinishError(
			loadErr,
			finish.Err(),
		)
	}
	if !finish.Succeeded() {
		return CommitReceipt{}, basis.found, fmt.Errorf(
			"finish typed-memory idempotency replay basis lookup: %w",
			finish.Err(),
		)
	}
	if !basis.found {
		return CommitReceipt{}, false, nil
	}
	exact, err := NewReplayRequestBuilder().
		SetContractVersion(request.contractVersion).
		SetProject(request.project).
		SetExpectedRevision(basis.expectedRevision).
		SetExpectedTypeEnv(basis.basisTypeEnv).
		SetIdempotencyKey(request.idempotencyKey).
		SetRequestProvenance(request.requestProvenance).
		SetCandidate(request.candidate).
		Build()
	if err != nil {
		return CommitReceipt{}, true, err
	}
	receipt, found, err := adapter.ReplayMemoryChangeSet(ctx, exact)
	return receipt, found, err
}
