package neighborhood

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const NeighborhoodContractV1 = "haft.neighborhood-result/v1"

type SnapshotBasis struct {
	graphRevision typedmemory.GraphRevision
	typeEnv       typedmemory.TypeEnvRef
	typeEnvDigest typedmemory.SHA256Digest
}

func NewSnapshotBasis(
	graphRevision typedmemory.GraphRevision,
	typeEnv typedmemory.TypeEnvRef,
	typeEnvDigest typedmemory.SHA256Digest,
) (SnapshotBasis, error) {
	basis := SnapshotBasis{
		graphRevision: graphRevision,
		typeEnv:       typeEnv,
		typeEnvDigest: typeEnvDigest,
	}
	if !basis.Valid() {
		return SnapshotBasis{}, fmt.Errorf("snapshot basis is invalid")
	}
	return basis, nil
}

func (basis SnapshotBasis) GraphRevision() typedmemory.GraphRevision {
	return basis.graphRevision
}

func (basis SnapshotBasis) TypeEnv() typedmemory.TypeEnvRef {
	return basis.typeEnv
}

func (basis SnapshotBasis) TypeEnvDigest() typedmemory.SHA256Digest {
	return basis.typeEnvDigest
}

func (basis SnapshotBasis) Valid() bool {
	typeEnv, typeEnvErr := typedmemory.ParseTypeEnvRef(basis.typeEnv.String())
	digest, digestErr := typedmemory.NewSHA256Digest(
		basis.typeEnvDigest.String(),
	)
	return basis.graphRevision.Value() > 0 &&
		typeEnvErr == nil &&
		digestErr == nil &&
		typeEnv == basis.typeEnv &&
		digest == basis.typeEnvDigest &&
		basis.typeEnv.Digest() == basis.typeEnvDigest
}

type MemoryViewContext struct {
	entity  typedmemory.PersistedRef
	context typedmemory.BoundedContextRef
	profile ProjectionProfileRef
}

func NewMemoryViewContext(
	entity typedmemory.PersistedRef,
	context typedmemory.BoundedContextRef,
	profile ProjectionProfileRef,
) (MemoryViewContext, error) {
	view := MemoryViewContext{
		entity:  entity,
		context: context,
		profile: profile,
	}
	if !view.Valid() {
		return MemoryViewContext{}, fmt.Errorf("memory view context is invalid")
	}
	return view, nil
}

func (view MemoryViewContext) Entity() typedmemory.PersistedRef {
	return view.entity
}

func (view MemoryViewContext) Context() typedmemory.BoundedContextRef {
	return view.context
}

func (view MemoryViewContext) ProfileRef() ProjectionProfileRef {
	return view.profile
}

func (view MemoryViewContext) Valid() bool {
	context, contextErr := typedmemory.NewBoundedContextRef(
		view.context.String(),
	)
	_, profileFound := LookupProjectionProfile(view.profile)
	return validPersistedRef(view.entity) &&
		contextErr == nil &&
		context == view.context &&
		profileFound
}

type WholeReadRetryCauseKind string

const (
	RetryStaleSnapshot             WholeReadRetryCauseKind = "stale_snapshot"
	RetryStaleCursor               WholeReadRetryCauseKind = "stale_cursor"
	RetryProjectionRebuildRequired WholeReadRetryCauseKind = "projection_rebuild_required"
)

type WholeReadRetryCause interface {
	Kind() WholeReadRetryCauseKind
	isWholeReadRetryCause()
}

type StaleSnapshotCause struct {
	observed SnapshotBasis
	required SnapshotBasis
}

func NewStaleSnapshotCause(
	observed SnapshotBasis,
	required SnapshotBasis,
) (StaleSnapshotCause, error) {
	cause := StaleSnapshotCause{
		observed: observed,
		required: required,
	}
	if !observed.Valid() ||
		!required.Valid() ||
		observed == required {
		return StaleSnapshotCause{}, fmt.Errorf(
			"stale-snapshot retry cause is invalid",
		)
	}
	return cause, nil
}

func (StaleSnapshotCause) Kind() WholeReadRetryCauseKind {
	return RetryStaleSnapshot
}

func (cause StaleSnapshotCause) Observed() SnapshotBasis {
	return cause.observed
}

func (cause StaleSnapshotCause) Required() SnapshotBasis {
	return cause.required
}

func (StaleSnapshotCause) isWholeReadRetryCause() {}

type StaleCursorCause struct {
	cursor   SnapshotCursor
	required SnapshotBasis
}

func NewStaleCursorCause(
	cursor SnapshotCursor,
	required SnapshotBasis,
) (StaleCursorCause, error) {
	cause := StaleCursorCause{
		cursor:   cursor,
		required: required,
	}
	sameSnapshot := cursor.GraphRevision() == required.GraphRevision() &&
		cursor.TypeEnv() == required.TypeEnv()
	if !cursor.Valid() || !required.Valid() || sameSnapshot {
		return StaleCursorCause{}, fmt.Errorf(
			"stale-cursor retry cause is invalid",
		)
	}
	return cause, nil
}

func (StaleCursorCause) Kind() WholeReadRetryCauseKind {
	return RetryStaleCursor
}

func (cause StaleCursorCause) Cursor() SnapshotCursor {
	return cause.cursor
}

func (cause StaleCursorCause) Required() SnapshotBasis {
	return cause.required
}

func (StaleCursorCause) isWholeReadRetryCause() {}

type ProjectionRebuildRequiredCause struct {
	projection    ProjectionRef
	observedEpoch uint64
	requiredEpoch uint64
}

func NewProjectionRebuildRequiredCause(
	projection ProjectionRef,
	observedEpoch uint64,
	requiredEpoch uint64,
) (ProjectionRebuildRequiredCause, error) {
	cause := ProjectionRebuildRequiredCause{
		projection:    projection,
		observedEpoch: observedEpoch,
		requiredEpoch: requiredEpoch,
	}
	if projection.String() == "" ||
		observedEpoch == 0 ||
		requiredEpoch == 0 ||
		observedEpoch == requiredEpoch {
		return ProjectionRebuildRequiredCause{}, fmt.Errorf(
			"projection-rebuild retry cause is invalid",
		)
	}
	return cause, nil
}

func (ProjectionRebuildRequiredCause) Kind() WholeReadRetryCauseKind {
	return RetryProjectionRebuildRequired
}

func (cause ProjectionRebuildRequiredCause) ProjectionRef() ProjectionRef {
	return cause.projection
}

func (cause ProjectionRebuildRequiredCause) ObservedEpoch() uint64 {
	return cause.observedEpoch
}

func (cause ProjectionRebuildRequiredCause) RequiredEpoch() uint64 {
	return cause.requiredEpoch
}

func (ProjectionRebuildRequiredCause) isWholeReadRetryCause() {}

type RetryOperation string

const (
	RetryReloadSnapshot    RetryOperation = "reload_snapshot"
	RetryRestartFromCursor RetryOperation = "restart_from_snapshot"
	RetryRebuildProjection RetryOperation = "rebuild_projection"
)

type NeighborhoodResultKind string

const (
	ResultExactNeighborhood NeighborhoodResultKind = "exact_neighborhood"
	ResultRetryRequired     NeighborhoodResultKind = "retry_required"
	ResultAbstained         NeighborhoodResultKind = "abstained"
)

type NeighborhoodResult interface {
	Kind() NeighborhoodResultKind
	ContractVersion() string
	Interpretation() InterpretationContract
	isNeighborhoodResult()
}

type RetryRequiredResult struct {
	cause          WholeReadRetryCause
	required       SnapshotBasis
	operation      RetryOperation
	interpretation InterpretationContract
}

func NewRetryRequiredResult(
	cause WholeReadRetryCause,
	required SnapshotBasis,
) (RetryRequiredResult, error) {
	operation, valid := retryOperationFor(cause)
	result := RetryRequiredResult{
		cause:          cause,
		required:       required,
		operation:      operation,
		interpretation: interpretationForRetryOrAbstention(),
	}
	if !valid || !result.valid() {
		return RetryRequiredResult{}, fmt.Errorf(
			"retry-required result is invalid",
		)
	}
	return result, nil
}

func (RetryRequiredResult) Kind() NeighborhoodResultKind {
	return ResultRetryRequired
}

func (RetryRequiredResult) ContractVersion() string {
	return NeighborhoodContractV1
}

func (result RetryRequiredResult) Cause() WholeReadRetryCause {
	return result.cause
}

func (result RetryRequiredResult) RequiredSnapshot() SnapshotBasis {
	return result.required
}

func (result RetryRequiredResult) RetryOperation() RetryOperation {
	return result.operation
}

func (result RetryRequiredResult) Interpretation() InterpretationContract {
	return result.interpretation
}

func (RetryRequiredResult) isNeighborhoodResult() {}

func (result RetryRequiredResult) valid() bool {
	return result.cause != nil &&
		result.required.Valid() &&
		result.interpretation.Valid() &&
		result.interpretation.Structure() == StructureUnavailable &&
		retryCauseRequiresSnapshot(result.cause, result.required)
}

type ReadAbstentionBasisKind string

const (
	AbstainEntityOrContextNotFound ReadAbstentionBasisKind = "entity_or_context_not_found"
	AbstainNoAdmissibleFacet       ReadAbstentionBasisKind = "no_admissible_facet"
)

type ReadAbstentionBasis interface {
	Kind() ReadAbstentionBasisKind
	isReadAbstentionBasis()
}

type EntityOrContextNotFoundBasis struct {
	entity   typedmemory.PersistedRef
	context  typedmemory.BoundedContextRef
	snapshot SnapshotBasis
}

func NewEntityOrContextNotFoundBasis(
	entity typedmemory.PersistedRef,
	context typedmemory.BoundedContextRef,
	snapshot SnapshotBasis,
) (EntityOrContextNotFoundBasis, error) {
	basis := EntityOrContextNotFoundBasis{
		entity:   entity,
		context:  context,
		snapshot: snapshot,
	}
	if !basis.valid() {
		return EntityOrContextNotFoundBasis{}, fmt.Errorf(
			"entity-or-context-not-found basis is invalid",
		)
	}
	return basis, nil
}

func (EntityOrContextNotFoundBasis) Kind() ReadAbstentionBasisKind {
	return AbstainEntityOrContextNotFound
}

func (basis EntityOrContextNotFoundBasis) Entity() typedmemory.PersistedRef {
	return basis.entity
}

func (basis EntityOrContextNotFoundBasis) Context() typedmemory.BoundedContextRef {
	return basis.context
}

func (basis EntityOrContextNotFoundBasis) Snapshot() SnapshotBasis {
	return basis.snapshot
}

func (EntityOrContextNotFoundBasis) isReadAbstentionBasis() {}

func (basis EntityOrContextNotFoundBasis) valid() bool {
	context, err := typedmemory.NewBoundedContextRef(basis.context.String())
	return validPersistedRef(basis.entity) &&
		err == nil &&
		context == basis.context &&
		basis.snapshot.Valid()
}

type NoAdmissibleFacetBasis struct {
	issues []FacetBasisIssue
}

func NewNoAdmissibleFacetBasis(
	issues []FacetBasisIssue,
) (NoAdmissibleFacetBasis, error) {
	basis := NoAdmissibleFacetBasis{
		issues: append([]FacetBasisIssue{}, issues...),
	}
	if !basis.valid() {
		return NoAdmissibleFacetBasis{}, fmt.Errorf(
			"no-admissible-facet basis requires typed issues",
		)
	}
	return basis, nil
}

func (NoAdmissibleFacetBasis) Kind() ReadAbstentionBasisKind {
	return AbstainNoAdmissibleFacet
}

func (basis NoAdmissibleFacetBasis) Issues() []FacetBasisIssue {
	return append([]FacetBasisIssue{}, basis.issues...)
}

func (NoAdmissibleFacetBasis) isReadAbstentionBasis() {}

func (basis NoAdmissibleFacetBasis) valid() bool {
	if len(basis.issues) == 0 {
		return false
	}
	for _, issue := range basis.issues {
		if !validFacetBasisIssue(issue) {
			return false
		}
	}
	return true
}

type InspectedSourceRef struct{ value string }

func NewInspectedSourceRef(raw string) (InspectedSourceRef, error) {
	value, err := exactReference("inspected source", raw)
	if err != nil {
		return InspectedSourceRef{}, err
	}
	return InspectedSourceRef{value: value}, nil
}

func (ref InspectedSourceRef) String() string { return ref.value }

type AbstainedResult struct {
	basis          ReadAbstentionBasis
	inspected      []InspectedSourceRef
	interpretation InterpretationContract
}

func NewAbstainedResult(
	basis ReadAbstentionBasis,
	inspected []InspectedSourceRef,
) (AbstainedResult, error) {
	result := AbstainedResult{
		basis:          basis,
		inspected:      append([]InspectedSourceRef{}, inspected...),
		interpretation: interpretationForRetryOrAbstention(),
	}
	if !result.valid() {
		return AbstainedResult{}, fmt.Errorf("abstained result is invalid")
	}
	return result, nil
}

func (AbstainedResult) Kind() NeighborhoodResultKind {
	return ResultAbstained
}

func (AbstainedResult) ContractVersion() string {
	return NeighborhoodContractV1
}

func (result AbstainedResult) Basis() ReadAbstentionBasis {
	return result.basis
}

func (result AbstainedResult) InspectedSources() []InspectedSourceRef {
	return append([]InspectedSourceRef{}, result.inspected...)
}

func (result AbstainedResult) Interpretation() InterpretationContract {
	return result.interpretation
}

func (AbstainedResult) isNeighborhoodResult() {}

func (result AbstainedResult) valid() bool {
	if result.basis == nil ||
		len(result.inspected) == 0 ||
		!result.interpretation.Valid() ||
		result.interpretation.Structure() != StructureUnavailable {
		return false
	}
	if !validReadAbstentionBasis(result.basis) {
		return false
	}
	values := make([]string, 0, len(result.inspected))
	for _, ref := range result.inspected {
		if ref.String() == "" {
			return false
		}
		values = append(values, ref.String())
	}
	sorted := append([]string{}, values...)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)
	return slices.Equal(values, sorted)
}

func retryOperationFor(
	cause WholeReadRetryCause,
) (RetryOperation, bool) {
	switch cause.(type) {
	case StaleSnapshotCause:
		return RetryReloadSnapshot, true
	case StaleCursorCause:
		return RetryRestartFromCursor, true
	case ProjectionRebuildRequiredCause:
		return RetryRebuildProjection, true
	default:
		return "", false
	}
}

func retryCauseRequiresSnapshot(
	cause WholeReadRetryCause,
	required SnapshotBasis,
) bool {
	switch value := cause.(type) {
	case StaleSnapshotCause:
		return value.Required() == required
	case StaleCursorCause:
		return value.Required() == required
	case ProjectionRebuildRequiredCause:
		return value.ProjectionRef().String() != ""
	default:
		return false
	}
}

func validReadAbstentionBasis(basis ReadAbstentionBasis) bool {
	switch value := basis.(type) {
	case EntityOrContextNotFoundBasis:
		return value.valid()
	case NoAdmissibleFacetBasis:
		return value.valid()
	default:
		return false
	}
}
