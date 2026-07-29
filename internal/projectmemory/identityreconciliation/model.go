// Package identityreconciliation owns the internal reviewed merge/split
// effect. It is deliberately separate from generic typed-memory admission:
// similarity, ranking, and an arbitrary basis reference cannot mint this
// capability or mutate entity identity.
package identityreconciliation

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var (
	ErrDatabaseRequired     = errors.New("identity-reconciliation database is required")
	ErrClockRequired        = errors.New("identity-reconciliation clock is required")
	ErrSchemaUnavailable    = errors.New("identity-reconciliation schema is unavailable")
	ErrStaleGraphRevision   = errors.New("identity-reconciliation graph revision is stale")
	ErrActiveTypeEnvChanged = errors.New("identity-reconciliation active TypeEnv changed")
	ErrIdempotencyConflict  = errors.New("identity-reconciliation idempotency key was consumed by another request")
	ErrEntityBasisMissing   = errors.New("identity-reconciliation exact entity/context basis is missing")
	ErrIdentityConflict     = errors.New("identity-reconciliation conflicts with existing identity history")
	ErrStoredIntegrity      = errors.New("stored identity-reconciliation integrity failure")
	ErrCommitOutcomeUnknown = errors.New("identity-reconciliation commit outcome is unknown")
	ErrProjectionBasis      = errors.New("identity-reconciliation projection debt lacks its exact event basis")
)

type IdempotencyKey struct{ value string }

func NewIdempotencyKey(raw string) (IdempotencyKey, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw || len(value) > 512 {
		return IdempotencyKey{}, fmt.Errorf("identity-reconciliation idempotency key must be 1-512 bytes of canonical text")
	}
	return IdempotencyKey{value: value}, nil
}

func (key IdempotencyKey) String() string { return key.value }

type Request struct {
	project   projectledger.ProjectID
	key       IdempotencyKey
	admission typedmemory.ReviewedIdentityReconciliationAdmission
}

type RequestBuilder struct {
	value Request
}

func NewRequestBuilder() *RequestBuilder { return &RequestBuilder{} }

func (builder *RequestBuilder) SetProject(project projectledger.ProjectID) *RequestBuilder {
	builder.value.project = project
	return builder
}

func (builder *RequestBuilder) SetIdempotencyKey(key IdempotencyKey) *RequestBuilder {
	builder.value.key = key
	return builder
}

func (builder *RequestBuilder) SetAdmission(
	admission typedmemory.ReviewedIdentityReconciliationAdmission,
) *RequestBuilder {
	builder.value.admission = admission
	return builder
}

func (builder *RequestBuilder) Build() (Request, error) {
	if builder == nil {
		return Request{}, fmt.Errorf("identity-reconciliation request builder is required")
	}
	value := builder.value
	project, err := projectledger.ParseProjectID(value.project.String())
	if err != nil || project != value.project {
		return Request{}, fmt.Errorf("identity-reconciliation project identity is invalid")
	}
	if value.key.String() == "" {
		return Request{}, fmt.Errorf("identity-reconciliation idempotency key is required")
	}
	basis := value.admission.Basis()
	if len(value.admission.CanonicalBytes()) == 0 || value.admission.Digest().String() == "" {
		return Request{}, fmt.Errorf("sealed reviewed identity-reconciliation admission is required")
	}
	if basis.GraphRevision().Value() > math.MaxInt64-1 {
		return Request{}, fmt.Errorf("identity-reconciliation graph revision exceeds SQLite range")
	}
	return value, nil
}

func (request Request) Project() projectledger.ProjectID { return request.project }

func (request Request) IdempotencyKey() IdempotencyKey { return request.key }

func (request Request) Admission() typedmemory.ReviewedIdentityReconciliationAdmission {
	return request.admission
}

type CommitDisposition string

const (
	CommitApplied CommitDisposition = "applied"
	CommitReplay  CommitDisposition = "replay"
)

type Receipt struct {
	disposition       CommitDisposition
	reconciliationRef string
	eventRef          string
	commitRef         string
	revision          typedmemory.GraphRevision
	resultDigest      typedmemory.SHA256Digest
}

func (receipt Receipt) Disposition() CommitDisposition { return receipt.disposition }

func (receipt Receipt) ReconciliationRef() string { return receipt.reconciliationRef }

func (receipt Receipt) EventRef() string { return receipt.eventRef }

func (receipt Receipt) CommitRef() string { return receipt.commitRef }

func (receipt Receipt) GraphRevision() typedmemory.GraphRevision { return receipt.revision }

func (receipt Receipt) ResultDigest() typedmemory.SHA256Digest { return receipt.resultDigest }

type HistoricalResolution interface {
	Entity() typedmemory.EntityID
	Context() typedmemory.BoundedContextRef
	historicalResolutionVariant()
}

type CurrentIdentity struct {
	entity  typedmemory.EntityID
	context typedmemory.BoundedContextRef
}

func (resolution CurrentIdentity) Entity() typedmemory.EntityID { return resolution.entity }

func (resolution CurrentIdentity) Context() typedmemory.BoundedContextRef { return resolution.context }

func (CurrentIdentity) historicalResolutionVariant() {}

type MergedIdentity struct {
	requested typedmemory.EntityID
	current   typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	history   []string
}

func (resolution MergedIdentity) Entity() typedmemory.EntityID { return resolution.requested }

func (resolution MergedIdentity) Current() typedmemory.EntityID { return resolution.current }

func (resolution MergedIdentity) Context() typedmemory.BoundedContextRef { return resolution.context }

func (resolution MergedIdentity) ReconciliationHistory() []string {
	return append([]string(nil), resolution.history...)
}

func (MergedIdentity) historicalResolutionVariant() {}

type SplitIdentityCandidates struct {
	source     typedmemory.EntityID
	candidates []typedmemory.EntityID
	context    typedmemory.BoundedContextRef
	history    []string
}

func (resolution SplitIdentityCandidates) Entity() typedmemory.EntityID { return resolution.source }

func (resolution SplitIdentityCandidates) Candidates() []typedmemory.EntityID {
	return append([]typedmemory.EntityID(nil), resolution.candidates...)
}

func (resolution SplitIdentityCandidates) Context() typedmemory.BoundedContextRef {
	return resolution.context
}

func (resolution SplitIdentityCandidates) ReconciliationRef() string {
	if len(resolution.history) == 0 {
		return ""
	}
	return resolution.history[len(resolution.history)-1]
}

func (resolution SplitIdentityCandidates) ReconciliationHistory() []string {
	return append([]string(nil), resolution.history...)
}

func (SplitIdentityCandidates) historicalResolutionVariant() {}

type IdentityAbsent struct {
	entity  typedmemory.EntityID
	context typedmemory.BoundedContextRef
}

func (resolution IdentityAbsent) Entity() typedmemory.EntityID { return resolution.entity }

func (resolution IdentityAbsent) Context() typedmemory.BoundedContextRef { return resolution.context }

func (IdentityAbsent) historicalResolutionVariant() {}

type ProjectionDebtReceipt struct {
	debtRef      string
	debtEventRef string
}

func (receipt ProjectionDebtReceipt) DebtRef() string { return receipt.debtRef }

func (receipt ProjectionDebtReceipt) DebtEventRef() string { return receipt.debtEventRef }

type ProjectionDebtReason struct{ value string }

func NewProjectionDebtReason(raw string) (ProjectionDebtReason, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw || len(value) > 256 {
		return ProjectionDebtReason{}, fmt.Errorf("projection-debt reason must be 1-256 bytes of canonical text")
	}
	return ProjectionDebtReason{value: value}, nil
}

func (reason ProjectionDebtReason) String() string { return reason.value }

type ProjectionDebtDetail struct{ value string }

func NewProjectionDebtDetail(raw string) (ProjectionDebtDetail, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw || len(value) > 4096 {
		return ProjectionDebtDetail{}, fmt.Errorf("projection-debt detail must be 1-4096 bytes of canonical text")
	}
	return ProjectionDebtDetail{value: value}, nil
}

func (detail ProjectionDebtDetail) String() string { return detail.value }

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
