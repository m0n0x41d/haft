package scopedrecall

import (
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
)

type ProducerCoverageKind string

const (
	ProducerCoverageComplete    ProducerCoverageKind = "complete"
	ProducerCoveragePartial     ProducerCoverageKind = "partial"
	ProducerCoverageUnavailable ProducerCoverageKind = "unavailable"
)

type ProducerCoverage interface {
	Kind() ProducerCoverageKind
	ProducerRef() ProducerRef
	InspectedCount() uint64
	isProducerCoverage()
}

type CompleteProducerCoverage struct {
	producer  ProducerRef
	inspected uint64
}

func NewCompleteProducerCoverage(
	producer ProducerRef,
	inspected uint64,
) (CompleteProducerCoverage, error) {
	if producer.String() == "" {
		return CompleteProducerCoverage{}, fmt.Errorf(
			"complete producer coverage requires producer",
		)
	}
	return CompleteProducerCoverage{
		producer:  producer,
		inspected: inspected,
	}, nil
}

func (CompleteProducerCoverage) Kind() ProducerCoverageKind {
	return ProducerCoverageComplete
}

func (coverage CompleteProducerCoverage) ProducerRef() ProducerRef {
	return coverage.producer
}

func (coverage CompleteProducerCoverage) InspectedCount() uint64 {
	return coverage.inspected
}

func (CompleteProducerCoverage) isProducerCoverage() {}

type RecallCursor struct {
	scope       ExactRecallScope
	snapshot    neighborhood.SnapshotBasis
	producer    ProducerRef
	queryDigest string
	nextOffset  uint64
	digest      string
}

func NewRecallCursor(
	scope ExactRecallScope,
	snapshot neighborhood.SnapshotBasis,
	producer ProducerRef,
	query RecallQuery,
	nextOffset uint64,
) (RecallCursor, error) {
	cursor := RecallCursor{
		scope:       scope,
		snapshot:    snapshot,
		producer:    producer,
		queryDigest: query.Digest(),
		nextOffset:  nextOffset,
	}
	digestValue, err := recallCursorDigest(cursor)
	if err != nil {
		return RecallCursor{}, err
	}
	cursor.digest = digestValue
	if !cursor.Valid() {
		return RecallCursor{}, fmt.Errorf("recall cursor is invalid")
	}
	return cursor, nil
}

func (cursor RecallCursor) Scope() ExactRecallScope {
	return cursor.scope
}

func (cursor RecallCursor) SnapshotBasis() neighborhood.SnapshotBasis {
	return cursor.snapshot
}

func (cursor RecallCursor) ProducerRef() ProducerRef {
	return cursor.producer
}

func (cursor RecallCursor) QueryDigest() string {
	return cursor.queryDigest
}

func (cursor RecallCursor) NextOffset() uint64 {
	return cursor.nextOffset
}

func (cursor RecallCursor) Digest() string {
	return cursor.digest
}

func (cursor RecallCursor) Valid() bool {
	if !cursor.scope.Valid() ||
		!cursor.snapshot.Valid() ||
		cursor.producer.String() == "" ||
		cursor.queryDigest == "" ||
		cursor.nextOffset == 0 {
		return false
	}
	digestValue, err := recallCursorDigest(cursor)
	return err == nil && digestValue == cursor.digest
}

func recallCursorDigest(cursor RecallCursor) (string, error) {
	digestValue, err := digestCanonical(map[string]any{
		"scope": map[string]string{
			"entity_reference_kind": cursor.scope.Entity().
				RefKind().
				String(),
			"entity_reference_id": cursor.scope.Entity().
				ReferenceID().
				String(),
			"bounded_context_ref": cursor.scope.Context().String(),
			"projection_profile_ref": cursor.scope.
				ProfileRef().
				String(),
		},
		"snapshot": map[string]any{
			"graph_revision": cursor.snapshot.GraphRevision().Value(),
			"type_env_ref":   cursor.snapshot.TypeEnv().String(),
			"type_env_digest": cursor.snapshot.
				TypeEnvDigest().
				String(),
		},
		"producer_ref": cursor.producer.String(),
		"query_digest": cursor.queryDigest,
		"next_offset":  cursor.nextOffset,
	})
	if err != nil {
		return "", err
	}
	return digestValue.String(), nil
}

type PartialProducerCoverage struct {
	producer       ProducerRef
	inspected      uint64
	omittedAtLeast uint64
	cursor         RecallCursor
}

func NewPartialProducerCoverage(
	producer ProducerRef,
	inspected uint64,
	omittedAtLeast uint64,
	cursor RecallCursor,
) (PartialProducerCoverage, error) {
	coverage := PartialProducerCoverage{
		producer:       producer,
		inspected:      inspected,
		omittedAtLeast: omittedAtLeast,
		cursor:         cursor,
	}
	if producer.String() == "" ||
		omittedAtLeast == 0 ||
		!cursor.Valid() ||
		cursor.ProducerRef() != producer {
		return PartialProducerCoverage{}, fmt.Errorf(
			"partial producer coverage is invalid",
		)
	}
	return coverage, nil
}

func (PartialProducerCoverage) Kind() ProducerCoverageKind {
	return ProducerCoveragePartial
}

func (coverage PartialProducerCoverage) ProducerRef() ProducerRef {
	return coverage.producer
}

func (coverage PartialProducerCoverage) InspectedCount() uint64 {
	return coverage.inspected
}

func (coverage PartialProducerCoverage) OmittedAtLeast() uint64 {
	return coverage.omittedAtLeast
}

func (coverage PartialProducerCoverage) Cursor() RecallCursor {
	return coverage.cursor
}

func (PartialProducerCoverage) isProducerCoverage() {}

type UnavailableProducerCoverage struct {
	producer ProducerRef
	missing  neighborhood.MissingBasisRef
}

func NewUnavailableProducerCoverage(
	producer ProducerRef,
	missing neighborhood.MissingBasisRef,
) (UnavailableProducerCoverage, error) {
	if producer.String() == "" || missing.String() == "" {
		return UnavailableProducerCoverage{}, fmt.Errorf(
			"unavailable producer coverage is invalid",
		)
	}
	return UnavailableProducerCoverage{
		producer: producer,
		missing:  missing,
	}, nil
}

func (UnavailableProducerCoverage) Kind() ProducerCoverageKind {
	return ProducerCoverageUnavailable
}

func (coverage UnavailableProducerCoverage) ProducerRef() ProducerRef {
	return coverage.producer
}

func (UnavailableProducerCoverage) InspectedCount() uint64 {
	return 0
}

func (coverage UnavailableProducerCoverage) MissingBasis() neighborhood.MissingBasisRef {
	return coverage.missing
}

func (UnavailableProducerCoverage) isProducerCoverage() {}

type ScopedRecallResultKind string

const (
	ScopedResultCandidateSet  ScopedRecallResultKind = "scoped_memory_candidate_set"
	ScopedResultAbstained     ScopedRecallResultKind = "abstained"
	ScopedResultRetryRequired ScopedRecallResultKind = "retry_required"
)

type ScopedRecallResult interface {
	Kind() ScopedRecallResultKind
	Scope() ExactRecallScope
	SnapshotBasis() neighborhood.SnapshotBasis
	Interpretation() neighborhood.InterpretationContract
	isScopedRecallResult()
}

type ScopedMemoryCandidateSet struct {
	scope          ExactRecallScope
	snapshot       neighborhood.SnapshotBasis
	candidates     []RecallCandidate
	coverage       []ProducerCoverage
	appliedBudget  CandidateBudgetApplication
	interpretation neighborhood.InterpretationContract
}

func newScopedMemoryCandidateSet(
	request ScopedRecallRequest,
	candidates []RecallCandidate,
	coverage []ProducerCoverage,
) (ScopedMemoryCandidateSet, error) {
	included, fits := scopedCandidateCount(len(candidates))
	if !fits {
		return ScopedMemoryCandidateSet{}, fmt.Errorf(
			"scoped memory candidate count exceeds uint32 range",
		)
	}
	result := ScopedMemoryCandidateSet{
		scope:      request.Scope(),
		snapshot:   request.SnapshotBasis(),
		candidates: append([]RecallCandidate{}, candidates...),
		coverage:   append([]ProducerCoverage{}, coverage...),
		appliedBudget: CandidateBudgetApplication{
			requested: request.Budget().MaxCandidates(),
			included:  included,
			omitted:   producerCoverageOmitted(coverage),
		},
		interpretation: neighborhood.InterpretationForScopedCandidates(),
	}
	if !result.valid() {
		return ScopedMemoryCandidateSet{}, fmt.Errorf(
			"scoped memory candidate set is invalid",
		)
	}
	return result, nil
}

func (ScopedMemoryCandidateSet) Kind() ScopedRecallResultKind {
	return ScopedResultCandidateSet
}

func (result ScopedMemoryCandidateSet) Scope() ExactRecallScope {
	return result.scope
}

func (result ScopedMemoryCandidateSet) SnapshotBasis() neighborhood.SnapshotBasis {
	return result.snapshot
}

func (result ScopedMemoryCandidateSet) Candidates() []RecallCandidate {
	return append([]RecallCandidate{}, result.candidates...)
}

func (result ScopedMemoryCandidateSet) ProducerCoverage() []ProducerCoverage {
	return append([]ProducerCoverage{}, result.coverage...)
}

func (result ScopedMemoryCandidateSet) AppliedBudget() CandidateBudgetApplication {
	return result.appliedBudget
}

func (result ScopedMemoryCandidateSet) Interpretation() neighborhood.InterpretationContract {
	return result.interpretation
}

func (ScopedMemoryCandidateSet) isScopedRecallResult() {}

func (result ScopedMemoryCandidateSet) valid() bool {
	included, fits := scopedCandidateCount(len(result.candidates))
	if !fits {
		return false
	}
	if !result.scope.Valid() ||
		!result.snapshot.Valid() ||
		len(result.candidates) == 0 ||
		len(result.coverage) == 0 ||
		!result.interpretation.Valid() ||
		result.interpretation.Identity() != neighborhood.IdentityExact ||
		result.interpretation.RelationalRecords() !=
			neighborhood.RelationalRecordsCandidateAssertions {
		return false
	}
	for index, candidate := range result.candidates {
		if !candidate.Unit().Valid() ||
			candidate.Unit().Scope() != result.scope ||
			candidate.Unit().SnapshotBasis() != result.snapshot ||
			candidate.Rank() != uint32(index+1) ||
			candidate.ProducerRef().String() == "" {
			return false
		}
	}
	return validProducerCoverages(result.coverage) &&
		result.appliedBudget.Included() == included
}

func scopedCandidateCount(count int) (uint32, bool) {
	if count < 0 || count > math.MaxUint32 {
		return 0, false
	}
	return uint32(count), true // #nosec G115 -- count is bounded by math.MaxUint32 above.
}

// ScopedRetryRequired keeps an exact recall scope while refusing to expose
// candidates from a stale graph/TypeEnv snapshot. It shares the neighborhood
// retry grammar rather than disguising stale data as abstention.
type ScopedRetryRequired struct {
	scope          ExactRecallScope
	observed       neighborhood.SnapshotBasis
	retry          neighborhood.RetryRequiredResult
	interpretation neighborhood.InterpretationContract
}

func NewScopedRetryRequired(
	request ScopedRecallRequest,
	retry neighborhood.RetryRequiredResult,
) (ScopedRetryRequired, error) {
	result := ScopedRetryRequired{
		scope:          request.Scope(),
		observed:       request.SnapshotBasis(),
		retry:          retry,
		interpretation: retry.Interpretation(),
	}
	if !result.valid() {
		return ScopedRetryRequired{}, fmt.Errorf(
			"scoped recall retry-required result is invalid",
		)
	}
	return result, nil
}

func (ScopedRetryRequired) Kind() ScopedRecallResultKind {
	return ScopedResultRetryRequired
}

func (result ScopedRetryRequired) Scope() ExactRecallScope {
	return result.scope
}

func (result ScopedRetryRequired) SnapshotBasis() neighborhood.SnapshotBasis {
	return result.observed
}

func (result ScopedRetryRequired) Cause() neighborhood.WholeReadRetryCause {
	return result.retry.Cause()
}

func (result ScopedRetryRequired) RequiredSnapshot() neighborhood.SnapshotBasis {
	return result.retry.RequiredSnapshot()
}

func (result ScopedRetryRequired) RetryOperation() neighborhood.RetryOperation {
	return result.retry.RetryOperation()
}

func (result ScopedRetryRequired) Interpretation() neighborhood.InterpretationContract {
	return result.interpretation
}

func (ScopedRetryRequired) isScopedRecallResult() {}

func (result ScopedRetryRequired) valid() bool {
	return result.scope.Valid() &&
		result.observed.Valid() &&
		result.scope.Entity().RefKind().TypeEnv() ==
			result.observed.TypeEnv() &&
		result.retry.Kind() == neighborhood.ResultRetryRequired &&
		result.retry.RequiredSnapshot().Valid() &&
		result.retry.RequiredSnapshot() != result.observed &&
		result.interpretation.Valid() &&
		result.interpretation.Structure() ==
			neighborhood.StructureUnavailable &&
		result.interpretation.Authority() ==
			neighborhood.AuthorityNotGranted
}

type CandidateBudgetApplication struct {
	requested uint32
	included  uint32
	omitted   uint64
}

func (application CandidateBudgetApplication) Requested() uint32 {
	return application.requested
}

func (application CandidateBudgetApplication) Included() uint32 {
	return application.included
}

func (application CandidateBudgetApplication) OmittedAtLeast() uint64 {
	return application.omitted
}

type ScopedRecallAbstentionBasisKind string

const (
	AbstentionNoMatchingMemory ScopedRecallAbstentionBasisKind = "no_matching_memory"
	AbstentionNoUsableProducer ScopedRecallAbstentionBasisKind = "no_usable_producer"
)

type ScopedRecallAbstentionBasis interface {
	Kind() ScopedRecallAbstentionBasisKind
	isScopedRecallAbstentionBasis()
}

type NoMatchingMemoryBasis struct {
	completeProducers []ProducerRef
}

func NewNoMatchingMemoryBasis(
	producers []ProducerRef,
) (NoMatchingMemoryBasis, error) {
	values := canonicalProducerRefs(producers)
	if len(values) == 0 || len(values) != len(producers) {
		return NoMatchingMemoryBasis{}, fmt.Errorf(
			"no-matching-memory basis requires distinct complete producers",
		)
	}
	return NoMatchingMemoryBasis{completeProducers: values}, nil
}

func (NoMatchingMemoryBasis) Kind() ScopedRecallAbstentionBasisKind {
	return AbstentionNoMatchingMemory
}

func (basis NoMatchingMemoryBasis) CompleteProducerRefs() []ProducerRef {
	return append([]ProducerRef{}, basis.completeProducers...)
}

func (NoMatchingMemoryBasis) isScopedRecallAbstentionBasis() {}

type NoUsableProducerBasis struct {
	unavailable []ProducerRef
	missing     neighborhood.MissingBasisRef
}

func NewNoUsableProducerBasis(
	producers []ProducerRef,
	missing neighborhood.MissingBasisRef,
) (NoUsableProducerBasis, error) {
	values := canonicalProducerRefs(producers)
	if len(values) == 0 ||
		len(values) != len(producers) ||
		missing.String() == "" {
		return NoUsableProducerBasis{}, fmt.Errorf(
			"no-usable-producer basis is invalid",
		)
	}
	return NoUsableProducerBasis{
		unavailable: values,
		missing:     missing,
	}, nil
}

func (NoUsableProducerBasis) Kind() ScopedRecallAbstentionBasisKind {
	return AbstentionNoUsableProducer
}

func (basis NoUsableProducerBasis) UnavailableProducerRefs() []ProducerRef {
	return append([]ProducerRef{}, basis.unavailable...)
}

func (basis NoUsableProducerBasis) MissingBasis() neighborhood.MissingBasisRef {
	return basis.missing
}

func (NoUsableProducerBasis) isScopedRecallAbstentionBasis() {}

type ScopedRecallAbstained struct {
	scope          ExactRecallScope
	snapshot       neighborhood.SnapshotBasis
	inspected      []ProducerRef
	basis          ScopedRecallAbstentionBasis
	interpretation neighborhood.InterpretationContract
}

func newScopedRecallAbstained(
	request ScopedRecallRequest,
	inspected []ProducerRef,
	basis ScopedRecallAbstentionBasis,
) (ScopedRecallAbstained, error) {
	result := ScopedRecallAbstained{
		scope:          request.Scope(),
		snapshot:       request.SnapshotBasis(),
		inspected:      canonicalProducerRefs(inspected),
		basis:          basis,
		interpretation: neighborhood.InterpretationForReadAbstention(),
	}
	if !result.valid() {
		return ScopedRecallAbstained{}, fmt.Errorf(
			"scoped recall abstention is invalid",
		)
	}
	return result, nil
}

func NewScopedRecallAbstained(
	request ScopedRecallRequest,
	inspected []ProducerRef,
	basis ScopedRecallAbstentionBasis,
) (ScopedRecallAbstained, error) {
	return newScopedRecallAbstained(request, inspected, basis)
}

func (ScopedRecallAbstained) Kind() ScopedRecallResultKind {
	return ScopedResultAbstained
}

func (result ScopedRecallAbstained) Scope() ExactRecallScope {
	return result.scope
}

func (result ScopedRecallAbstained) SnapshotBasis() neighborhood.SnapshotBasis {
	return result.snapshot
}

func (result ScopedRecallAbstained) InspectedProducers() []ProducerRef {
	return append([]ProducerRef{}, result.inspected...)
}

func (result ScopedRecallAbstained) Basis() ScopedRecallAbstentionBasis {
	return result.basis
}

func (result ScopedRecallAbstained) Interpretation() neighborhood.InterpretationContract {
	return result.interpretation
}

func (ScopedRecallAbstained) isScopedRecallResult() {}

func (result ScopedRecallAbstained) valid() bool {
	if !result.scope.Valid() ||
		!result.snapshot.Valid() ||
		len(result.inspected) == 0 ||
		result.basis == nil ||
		!result.interpretation.Valid() ||
		result.interpretation.Structure() !=
			neighborhood.StructureUnavailable {
		return false
	}
	switch basis := result.basis.(type) {
	case NoMatchingMemoryBasis:
		return slices.Equal(
			basis.CompleteProducerRefs(),
			result.inspected,
		)
	case NoUsableProducerBasis:
		return len(basis.UnavailableProducerRefs()) > 0 &&
			basis.MissingBasis().String() != ""
	default:
		return false
	}
}

func validProducerCoverages(values []ProducerCoverage) bool {
	seen := make(map[string]struct{}, len(values))
	for _, coverage := range values {
		if coverage == nil || coverage.ProducerRef().String() == "" {
			return false
		}
		if _, found := seen[coverage.ProducerRef().String()]; found {
			return false
		}
		seen[coverage.ProducerRef().String()] = struct{}{}
		switch value := coverage.(type) {
		case CompleteProducerCoverage:
		case PartialProducerCoverage:
			if value.OmittedAtLeast() == 0 || !value.Cursor().Valid() {
				return false
			}
		case UnavailableProducerCoverage:
			if value.MissingBasis().String() == "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func producerCoverageOmitted(values []ProducerCoverage) uint64 {
	result := uint64(0)
	for _, value := range values {
		partial, ok := value.(PartialProducerCoverage)
		if !ok {
			continue
		}
		result += partial.OmittedAtLeast()
	}
	return result
}

func canonicalProducerRefs(values []ProducerRef) []ProducerRef {
	result := append([]ProducerRef{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].String() < result[right].String()
	})
	return slices.Compact(result)
}
