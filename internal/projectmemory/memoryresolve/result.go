package memoryresolve

import (
	"fmt"
	"slices"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type ResolutionWitnessKind string

const (
	WitnessExactIdentifier ResolutionWitnessKind = "exact_identifier"
	WitnessExactAlias      ResolutionWitnessKind = "exact_alias"
	WitnessLexicalLabel    ResolutionWitnessKind = "lexical_label"
	WitnessLexicalAlias    ResolutionWitnessKind = "lexical_alias"
	WitnessReviewedMerge   ResolutionWitnessKind = "reviewed_identity_merge"
)

type ResolutionWitness struct {
	kind    ResolutionWitnessKind
	matched string
	basis   typedmemory.ResolutionBasisRef
}

func newResolutionWitness(
	kind ResolutionWitnessKind,
	matched string,
	basis typedmemory.ResolutionBasisRef,
) ResolutionWitness {
	return ResolutionWitness{
		kind:    kind,
		matched: matched,
		basis:   basis,
	}
}

func (witness ResolutionWitness) Kind() ResolutionWitnessKind {
	return witness.kind
}

func (witness ResolutionWitness) Matched() string {
	return witness.matched
}

func (witness ResolutionWitness) Basis() typedmemory.ResolutionBasisRef {
	return witness.basis
}

func (witness ResolutionWitness) valid() bool {
	if !slices.Contains(
		[]ResolutionWitnessKind{
			WitnessExactIdentifier,
			WitnessExactAlias,
			WitnessLexicalLabel,
			WitnessLexicalAlias,
			WitnessReviewedMerge,
		},
		witness.kind,
	) {
		return false
	}
	matched, matchedErr := exactOneLine(
		"resolution witness",
		witness.matched,
	)
	basis, basisErr := typedmemory.NewResolutionBasisRef(
		witness.basis.String(),
	)
	return matchedErr == nil &&
		matched == witness.matched &&
		basisErr == nil &&
		basis == witness.basis
}

type EntityResolutionResultKind string

const (
	ResultExactEntity         EntityResolutionResultKind = "exact_entity"
	ResultKnownAbsent         EntityResolutionResultKind = "known_absent"
	ResultEntityCandidates    EntityResolutionResultKind = "entity_candidates"
	ResultResolutionUnsettled EntityResolutionResultKind = "resolution_unsettled"
	ResultRetryRequired       EntityResolutionResultKind = "retry_required"
)

type EntityResolutionResult interface {
	Kind() EntityResolutionResultKind
	ResolutionScope() ResolutionScope
	SnapshotBasis() neighborhood.SnapshotBasis
	Interpretation() neighborhood.InterpretationContract
	isEntityResolutionResult()
}

type ExactEntity struct {
	scope          ResolutionScope
	snapshot       neighborhood.SnapshotBasis
	entity         ResolutionUnit
	witnesses      []ResolutionWitness
	interpretation neighborhood.InterpretationContract
}

func newExactEntity(
	request ResolutionRequest,
	entity ResolutionUnit,
	witnesses []ResolutionWitness,
) (ExactEntity, error) {
	result := ExactEntity{
		scope:          request.Scope(),
		snapshot:       request.SnapshotBasis(),
		entity:         entity,
		witnesses:      append([]ResolutionWitness{}, witnesses...),
		interpretation: neighborhood.InterpretationForExactEntityResolution(),
	}
	if !result.scope.Valid() ||
		!result.snapshot.Valid() ||
		!entity.Valid() ||
		!resolutionWitnessesValid(witnesses) ||
		!result.interpretation.Valid() {
		return ExactEntity{}, fmt.Errorf(
			"exact entity resolution result is invalid",
		)
	}
	return result, nil
}

func (ExactEntity) Kind() EntityResolutionResultKind {
	return ResultExactEntity
}

func (result ExactEntity) ResolutionScope() ResolutionScope {
	return result.scope
}

func (result ExactEntity) SnapshotBasis() neighborhood.SnapshotBasis {
	return result.snapshot
}

func (result ExactEntity) Entity() ResolutionUnit {
	return result.entity
}

func (result ExactEntity) ResolutionWitnesses() []ResolutionWitness {
	return append([]ResolutionWitness{}, result.witnesses...)
}

func (result ExactEntity) Interpretation() neighborhood.InterpretationContract {
	return result.interpretation
}

func (ExactEntity) isEntityResolutionResult() {}

type KnownAbsent struct {
	scope          ResolutionScope
	snapshot       neighborhood.SnapshotBasis
	index          ResolutionIndexRef
	indexVersion   ResolutionIndexVersion
	completeness   ResolutionCompletenessBasisRef
	interpretation neighborhood.InterpretationContract
}

func newKnownAbsent(
	request ResolutionRequest,
	index ResolutionIndex,
) (KnownAbsent, error) {
	result := KnownAbsent{
		scope:          request.Scope(),
		snapshot:       index.SnapshotBasis(),
		index:          index.Ref(),
		indexVersion:   index.Version(),
		completeness:   index.Completeness().Basis(),
		interpretation: neighborhood.InterpretationForKnownAbsent(),
	}
	if !result.scope.Valid() ||
		!result.snapshot.Valid() ||
		result.index.String() == "" ||
		result.indexVersion.String() == "" ||
		result.completeness.String() == "" ||
		!result.interpretation.Valid() {
		return KnownAbsent{}, fmt.Errorf(
			"known-absent resolution result is invalid",
		)
	}
	return result, nil
}

func (KnownAbsent) Kind() EntityResolutionResultKind {
	return ResultKnownAbsent
}

func (result KnownAbsent) ResolutionScope() ResolutionScope {
	return result.scope
}

func (result KnownAbsent) SnapshotBasis() neighborhood.SnapshotBasis {
	return result.snapshot
}

func (result KnownAbsent) InspectedIndex() ResolutionIndexRef {
	return result.index
}

func (result KnownAbsent) InspectedIndexVersion() ResolutionIndexVersion {
	return result.indexVersion
}

func (result KnownAbsent) CompletenessBasis() ResolutionCompletenessBasisRef {
	return result.completeness
}

func (result KnownAbsent) Interpretation() neighborhood.InterpretationContract {
	return result.interpretation
}

func (KnownAbsent) isEntityResolutionResult() {}

type EntityCandidate struct {
	entity    ResolutionUnit
	rank      uint32
	witnesses []ResolutionWitness
}

func (candidate EntityCandidate) Entity() ResolutionUnit {
	return candidate.entity
}

func (candidate EntityCandidate) Rank() uint32 {
	return candidate.rank
}

func (candidate EntityCandidate) ResolutionWitnesses() []ResolutionWitness {
	return append([]ResolutionWitness{}, candidate.witnesses...)
}

type CandidateSetCoverage struct {
	index          ResolutionIndexRef
	indexVersion   ResolutionIndexVersion
	inspected      uint64
	included       uint32
	omittedAtLeast uint64
	cursor         ResolutionCursor
}

func (coverage CandidateSetCoverage) IndexRef() ResolutionIndexRef {
	return coverage.index
}

func (coverage CandidateSetCoverage) IndexVersion() ResolutionIndexVersion {
	return coverage.indexVersion
}

func (coverage CandidateSetCoverage) InspectedCount() uint64 {
	return coverage.inspected
}

func (coverage CandidateSetCoverage) IncludedCount() uint32 {
	return coverage.included
}

func (coverage CandidateSetCoverage) OmittedAtLeast() uint64 {
	return coverage.omittedAtLeast
}

func (coverage CandidateSetCoverage) Cursor() (ResolutionCursor, bool) {
	return coverage.cursor, coverage.omittedAtLeast > 0
}

func (coverage CandidateSetCoverage) valid(
	scope ResolutionScope,
	snapshot neighborhood.SnapshotBasis,
) bool {
	if coverage.index.String() == "" ||
		coverage.indexVersion.String() == "" ||
		coverage.included == 0 ||
		coverage.inspected <
			uint64(coverage.included)+coverage.omittedAtLeast {
		return false
	}
	cursor, hasCursor := coverage.Cursor()
	if coverage.omittedAtLeast == 0 {
		return !hasCursor
	}
	return hasCursor &&
		cursor.Valid() &&
		cursor.IndexRef() == coverage.index &&
		cursor.IndexVersion() == coverage.indexVersion &&
		cursor.SnapshotBasis() == snapshot &&
		sameResolutionScope(cursor.ResolutionScope(), scope) &&
		cursor.NextOffset() == uint64(coverage.included)
}

type ResolutionBudgetApplication struct {
	requested uint32
	included  uint32
	omitted   uint64
}

func (budget ResolutionBudgetApplication) Requested() uint32 {
	return budget.requested
}

func (budget ResolutionBudgetApplication) Included() uint32 {
	return budget.included
}

func (budget ResolutionBudgetApplication) OmittedAtLeast() uint64 {
	return budget.omitted
}

func (budget ResolutionBudgetApplication) valid() bool {
	return budget.requested > 0 &&
		budget.included <= budget.requested
}

type EntityCandidates struct {
	scope          ResolutionScope
	snapshot       neighborhood.SnapshotBasis
	candidates     []EntityCandidate
	coverage       CandidateSetCoverage
	appliedBudget  ResolutionBudgetApplication
	interpretation neighborhood.InterpretationContract
}

func newEntityCandidates(
	request ResolutionRequest,
	index ResolutionIndex,
	candidates []EntityCandidate,
	coverage CandidateSetCoverage,
) (EntityCandidates, error) {
	included, fits := resolutionCandidateCount(len(candidates))
	if !fits {
		return EntityCandidates{}, fmt.Errorf(
			"entity-candidates result is invalid",
		)
	}
	result := EntityCandidates{
		scope:      request.Scope(),
		snapshot:   index.SnapshotBasis(),
		candidates: append([]EntityCandidate{}, candidates...),
		coverage:   coverage,
		appliedBudget: ResolutionBudgetApplication{
			requested: request.MaxCandidates(),
			included:  included,
			omitted:   coverage.OmittedAtLeast(),
		},
		interpretation: neighborhood.InterpretationForEntityCandidates(),
	}
	if !result.valid() {
		return EntityCandidates{}, fmt.Errorf(
			"entity-candidates result is invalid",
		)
	}
	return result, nil
}

func (EntityCandidates) Kind() EntityResolutionResultKind {
	return ResultEntityCandidates
}

func (result EntityCandidates) ResolutionScope() ResolutionScope {
	return result.scope
}

func (result EntityCandidates) SnapshotBasis() neighborhood.SnapshotBasis {
	return result.snapshot
}

func (result EntityCandidates) Candidates() []EntityCandidate {
	return append([]EntityCandidate{}, result.candidates...)
}

func (result EntityCandidates) Coverage() CandidateSetCoverage {
	return result.coverage
}

func (result EntityCandidates) AppliedBudget() ResolutionBudgetApplication {
	return result.appliedBudget
}

func (result EntityCandidates) Interpretation() neighborhood.InterpretationContract {
	return result.interpretation
}

func (EntityCandidates) isEntityResolutionResult() {}

func (result EntityCandidates) valid() bool {
	included, fits := resolutionCandidateCount(len(result.candidates))
	if !fits {
		return false
	}
	if !result.scope.Valid() ||
		!result.snapshot.Valid() ||
		len(result.candidates) == 0 ||
		!result.coverage.valid(result.scope, result.snapshot) ||
		result.coverage.IncludedCount() != included ||
		!result.appliedBudget.valid() ||
		result.appliedBudget.Included() != included ||
		result.appliedBudget.OmittedAtLeast() !=
			result.coverage.OmittedAtLeast() ||
		!result.interpretation.Valid() ||
		result.interpretation.Identity() != neighborhood.IdentityUnresolved ||
		result.interpretation.RelationalRecords() !=
			neighborhood.RelationalRecordsUnavailable {
		return false
	}
	for index, candidate := range result.candidates {
		rank, rankFits := resolutionCandidateCount(index + 1)
		if !candidate.Entity().Valid() ||
			!rankFits ||
			candidate.Rank() != rank ||
			!resolutionWitnessesValid(candidate.ResolutionWitnesses()) {
			return false
		}
	}
	cursor, hasCursor := result.coverage.Cursor()
	return result.coverage.OmittedAtLeast() == 0 ||
		(hasCursor && cursor.Valid())
}

func resolutionCandidateCount(count int) (uint32, bool) {
	if count < 0 || int64(count) > int64(^uint32(0)) {
		return 0, false
	}
	return uint32(count), true // #nosec G115 -- count is bounded to the uint32 range above.
}

func resolutionCount(count int) (uint64, bool) {
	if count < 0 {
		return 0, false
	}
	return uint64(count), true // #nosec G115 -- every non-negative int is representable as uint64.
}

func resolutionWitnessesValid(witnesses []ResolutionWitness) bool {
	if len(witnesses) == 0 {
		return false
	}
	for _, witness := range witnesses {
		if !witness.valid() {
			return false
		}
	}
	return true
}

func sameResolutionScope(left ResolutionScope, right ResolutionScope) bool {
	if left.Query().Original() != right.Query().Original() ||
		left.Context().Kind() != right.Context().Kind() {
		return false
	}
	leftExact, leftIsExact := left.Context().(ExactContext)
	rightExact, rightIsExact := right.Context().(ExactContext)
	if !leftIsExact || !rightIsExact {
		return leftIsExact == rightIsExact
	}
	return leftExact.Context() == rightExact.Context()
}

type ResolutionBasisIssueKind string

const (
	IssueContextNotResolved        ResolutionBasisIssueKind = "context_not_resolved"
	IssueAliasConflict             ResolutionBasisIssueKind = "alias_conflict"
	IssueReviewedSplitCandidates   ResolutionBasisIssueKind = "reviewed_split_candidates"
	IssueLegacyIdentityUnbound     ResolutionBasisIssueKind = "legacy_identity_unbound"
	IssueIncompleteResolutionIndex ResolutionBasisIssueKind = "incomplete_resolution_index"
)

type ResolutionBasisIssue interface {
	Kind() ResolutionBasisIssueKind
	isResolutionBasisIssue()
}

type ContextNotResolvedIssue struct {
	query string
}

func newContextNotResolvedIssue(query string) ContextNotResolvedIssue {
	return ContextNotResolvedIssue{query: query}
}

func (ContextNotResolvedIssue) Kind() ResolutionBasisIssueKind {
	return IssueContextNotResolved
}

func (issue ContextNotResolvedIssue) QueryContext() string {
	return issue.query
}

func (ContextNotResolvedIssue) isResolutionBasisIssue() {}

type AliasConflictIssue struct {
	alias      typedmemory.EntityAlias
	candidates []typedmemory.PersistedRef
}

func newAliasConflictIssue(
	alias typedmemory.EntityAlias,
	candidates []typedmemory.PersistedRef,
) AliasConflictIssue {
	return AliasConflictIssue{
		alias:      alias,
		candidates: append([]typedmemory.PersistedRef{}, candidates...),
	}
}

func (AliasConflictIssue) Kind() ResolutionBasisIssueKind {
	return IssueAliasConflict
}

func (issue AliasConflictIssue) Alias() typedmemory.EntityAlias {
	return issue.alias
}

func (issue AliasConflictIssue) CandidateEntityRefs() []typedmemory.PersistedRef {
	return append([]typedmemory.PersistedRef{}, issue.candidates...)
}

func (AliasConflictIssue) isResolutionBasisIssue() {}

// ReviewedSplitCandidatesIssue preserves the historical source and exact
// reviewed candidate set without treating any candidate as its successor.
type ReviewedSplitCandidatesIssue struct {
	historical typedmemory.EntityID
	candidates []typedmemory.PersistedRef
	history    []typedmemory.ResolutionBasisRef
}

func NewReviewedSplitCandidatesIssue(
	historical typedmemory.EntityID,
	candidates []typedmemory.PersistedRef,
	history []typedmemory.ResolutionBasisRef,
) (ReviewedSplitCandidatesIssue, error) {
	canonical := canonicalCandidateRefs(candidates)
	issue := ReviewedSplitCandidatesIssue{
		historical: historical,
		candidates: canonical,
		history:    append([]typedmemory.ResolutionBasisRef(nil), history...),
	}
	if len(canonical) != len(candidates) || !issue.valid() {
		return ReviewedSplitCandidatesIssue{}, fmt.Errorf(
			"reviewed split-candidates issue is invalid",
		)
	}
	return issue, nil
}

func (ReviewedSplitCandidatesIssue) Kind() ResolutionBasisIssueKind {
	return IssueReviewedSplitCandidates
}

func (issue ReviewedSplitCandidatesIssue) HistoricalEntity() typedmemory.EntityID {
	return issue.historical
}

func (issue ReviewedSplitCandidatesIssue) CandidateEntityRefs() []typedmemory.PersistedRef {
	return append([]typedmemory.PersistedRef(nil), issue.candidates...)
}

func (issue ReviewedSplitCandidatesIssue) Reconciliation() typedmemory.ResolutionBasisRef {
	if len(issue.history) == 0 {
		return typedmemory.ResolutionBasisRef{}
	}
	return issue.history[len(issue.history)-1]
}

func (issue ReviewedSplitCandidatesIssue) ReconciliationHistory() []typedmemory.ResolutionBasisRef {
	return append([]typedmemory.ResolutionBasisRef(nil), issue.history...)
}

func (ReviewedSplitCandidatesIssue) isResolutionBasisIssue() {}

func (issue ReviewedSplitCandidatesIssue) valid() bool {
	historical, historicalErr := typedmemory.NewEntityID(
		issue.historical.String(),
	)
	if historicalErr != nil ||
		historical != issue.historical ||
		len(issue.candidates) < 2 ||
		!validReviewedHistory(issue.history) {
		return false
	}
	canonical := canonicalCandidateRefs(issue.candidates)
	return len(canonical) == len(issue.candidates) &&
		slices.Equal(canonical, issue.candidates)
}

type LegacyIdentityUnboundIssue struct {
	legacy neighborhood.LegacyRecordRef
}

func NewLegacyIdentityUnboundIssue(
	legacy neighborhood.LegacyRecordRef,
) (LegacyIdentityUnboundIssue, error) {
	if legacy.String() == "" {
		return LegacyIdentityUnboundIssue{}, fmt.Errorf(
			"legacy identity issue requires exact reference",
		)
	}
	return LegacyIdentityUnboundIssue{legacy: legacy}, nil
}

func (LegacyIdentityUnboundIssue) Kind() ResolutionBasisIssueKind {
	return IssueLegacyIdentityUnbound
}

func (issue LegacyIdentityUnboundIssue) LegacyRef() neighborhood.LegacyRecordRef {
	return issue.legacy
}

func (LegacyIdentityUnboundIssue) isResolutionBasisIssue() {}

type IncompleteResolutionIndexIssue struct {
	index   ResolutionIndexRef
	missing neighborhood.MissingBasisRef
}

func newIncompleteResolutionIndexIssue(
	index ResolutionIndexRef,
) (IncompleteResolutionIndexIssue, error) {
	missing, err := neighborhood.NewMissingBasisRef(
		"resolution-index-completeness:" + index.String(),
	)
	if err != nil {
		return IncompleteResolutionIndexIssue{}, err
	}
	return IncompleteResolutionIndexIssue{
		index:   index,
		missing: missing,
	}, nil
}

func (IncompleteResolutionIndexIssue) Kind() ResolutionBasisIssueKind {
	return IssueIncompleteResolutionIndex
}

func (issue IncompleteResolutionIndexIssue) ProducerRef() ResolutionIndexRef {
	return issue.index
}

func (issue IncompleteResolutionIndexIssue) MissingBasis() neighborhood.MissingBasisRef {
	return issue.missing
}

func (IncompleteResolutionIndexIssue) isResolutionBasisIssue() {}

type ResolutionUnsettled struct {
	scope          ResolutionScope
	snapshot       neighborhood.SnapshotBasis
	issues         []ResolutionBasisIssue
	interpretation neighborhood.InterpretationContract
}

func newResolutionUnsettled(
	request ResolutionRequest,
	snapshot neighborhood.SnapshotBasis,
	issues []ResolutionBasisIssue,
) (ResolutionUnsettled, error) {
	result := ResolutionUnsettled{
		scope:          request.Scope(),
		snapshot:       snapshot,
		issues:         append([]ResolutionBasisIssue{}, issues...),
		interpretation: neighborhood.InterpretationForReadAbstention(),
	}
	if !result.scope.Valid() ||
		!snapshot.Valid() ||
		!resolutionIssuesValid(issues) ||
		!result.interpretation.Valid() {
		return ResolutionUnsettled{}, fmt.Errorf(
			"resolution-unsettled result is invalid",
		)
	}
	return result, nil
}

func (ResolutionUnsettled) Kind() EntityResolutionResultKind {
	return ResultResolutionUnsettled
}

func (result ResolutionUnsettled) ResolutionScope() ResolutionScope {
	return result.scope
}

func (result ResolutionUnsettled) SnapshotBasis() neighborhood.SnapshotBasis {
	return result.snapshot
}

func (result ResolutionUnsettled) Issues() []ResolutionBasisIssue {
	return append([]ResolutionBasisIssue{}, result.issues...)
}

func (result ResolutionUnsettled) Interpretation() neighborhood.InterpretationContract {
	return result.interpretation
}

func (ResolutionUnsettled) isEntityResolutionResult() {}

func resolutionIssuesValid(issues []ResolutionBasisIssue) bool {
	if len(issues) == 0 {
		return false
	}
	for _, issue := range issues {
		switch value := issue.(type) {
		case ContextNotResolvedIssue:
			_, err := exactOneLine(
				"context-not-resolved query",
				value.QueryContext(),
			)
			if err != nil {
				return false
			}
		case AliasConflictIssue:
			alias, err := typedmemory.NewEntityAlias(
				value.Alias().String(),
			)
			if err != nil ||
				alias != value.Alias() ||
				len(value.CandidateEntityRefs()) < 2 {
				return false
			}
		case ReviewedSplitCandidatesIssue:
			if !value.valid() {
				return false
			}
		case LegacyIdentityUnboundIssue:
			if value.LegacyRef().String() == "" {
				return false
			}
		case IncompleteResolutionIndexIssue:
			if value.ProducerRef().String() == "" ||
				value.MissingBasis().String() == "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

type ResolutionRetryRequired struct {
	scope          ResolutionScope
	observed       neighborhood.SnapshotBasis
	required       neighborhood.SnapshotBasis
	cause          neighborhood.WholeReadRetryCause
	operation      neighborhood.RetryOperation
	interpretation neighborhood.InterpretationContract
}

func newResolutionRetryRequired(
	request ResolutionRequest,
	required neighborhood.SnapshotBasis,
) (ResolutionRetryRequired, error) {
	cause, err := neighborhood.NewStaleSnapshotCause(
		request.SnapshotBasis(),
		required,
	)
	if err != nil {
		return ResolutionRetryRequired{}, err
	}
	result := ResolutionRetryRequired{
		scope:          request.Scope(),
		observed:       request.SnapshotBasis(),
		required:       required,
		cause:          cause,
		operation:      neighborhood.RetryReloadSnapshot,
		interpretation: neighborhood.InterpretationForReadAbstention(),
	}
	if !result.valid() {
		return ResolutionRetryRequired{}, fmt.Errorf(
			"resolution retry-required result is invalid",
		)
	}
	return result, nil
}

func (ResolutionRetryRequired) Kind() EntityResolutionResultKind {
	return ResultRetryRequired
}

func (result ResolutionRetryRequired) ResolutionScope() ResolutionScope {
	return result.scope
}

func (result ResolutionRetryRequired) SnapshotBasis() neighborhood.SnapshotBasis {
	return result.required
}

func (result ResolutionRetryRequired) ObservedSnapshot() neighborhood.SnapshotBasis {
	return result.observed
}

func (result ResolutionRetryRequired) RequiredSnapshot() neighborhood.SnapshotBasis {
	return result.required
}

func (result ResolutionRetryRequired) Cause() neighborhood.WholeReadRetryCause {
	return result.cause
}

func (result ResolutionRetryRequired) RetryOperation() neighborhood.RetryOperation {
	return result.operation
}

func (result ResolutionRetryRequired) Interpretation() neighborhood.InterpretationContract {
	return result.interpretation
}

func (ResolutionRetryRequired) isEntityResolutionResult() {}

func (result ResolutionRetryRequired) valid() bool {
	return result.scope.Valid() &&
		result.observed.Valid() &&
		result.required.Valid() &&
		result.observed != result.required &&
		result.cause != nil &&
		result.cause.Kind() == neighborhood.RetryStaleSnapshot &&
		result.operation == neighborhood.RetryReloadSnapshot &&
		result.interpretation.Valid() &&
		result.interpretation.Structure() ==
			neighborhood.StructureUnavailable
}

type ResolutionCursor struct {
	index        ResolutionIndexRef
	indexVersion ResolutionIndexVersion
	snapshot     neighborhood.SnapshotBasis
	scope        ResolutionScope
	nextOffset   uint64
	digest       typedmemory.SHA256Digest
}

func newResolutionCursor(
	index ResolutionIndex,
	snapshot neighborhood.SnapshotBasis,
	scope ResolutionScope,
	nextOffset uint64,
) (ResolutionCursor, error) {
	cursor := ResolutionCursor{
		index:        index.Ref(),
		indexVersion: index.Version(),
		snapshot:     snapshot,
		scope:        scope,
		nextOffset:   nextOffset,
	}
	digest, err := resolutionCursorDigest(cursor)
	if err != nil {
		return ResolutionCursor{}, err
	}
	cursor.digest = digest
	if !cursor.Valid() {
		return ResolutionCursor{}, fmt.Errorf("resolution cursor is invalid")
	}
	return cursor, nil
}

func (cursor ResolutionCursor) IndexRef() ResolutionIndexRef {
	return cursor.index
}

func (cursor ResolutionCursor) IndexVersion() ResolutionIndexVersion {
	return cursor.indexVersion
}

func (cursor ResolutionCursor) SnapshotBasis() neighborhood.SnapshotBasis {
	return cursor.snapshot
}

func (cursor ResolutionCursor) ResolutionScope() ResolutionScope {
	return cursor.scope
}

func (cursor ResolutionCursor) NextOffset() uint64 {
	return cursor.nextOffset
}

func (cursor ResolutionCursor) Digest() typedmemory.SHA256Digest {
	return cursor.digest
}

func (cursor ResolutionCursor) Valid() bool {
	if cursor.index.String() == "" ||
		cursor.indexVersion.String() == "" ||
		!cursor.snapshot.Valid() ||
		!cursor.scope.Valid() ||
		cursor.nextOffset == 0 {
		return false
	}
	digest, err := resolutionCursorDigest(cursor)
	return err == nil && digest == cursor.digest
}

func resolutionCursorDigest(
	cursor ResolutionCursor,
) (typedmemory.SHA256Digest, error) {
	return digestCanonical(map[string]any{
		"index_ref":     cursor.index.String(),
		"index_version": cursor.indexVersion.String(),
		"snapshot": map[string]any{
			"graph_revision": cursor.snapshot.GraphRevision().Value(),
			"type_env_ref":   cursor.snapshot.TypeEnv().String(),
			"type_env_digest": cursor.snapshot.
				TypeEnvDigest().
				String(),
		},
		"scope":       resolutionScopeCanonical(cursor.scope),
		"next_offset": cursor.nextOffset,
	})
}

func resolutionScopeCanonical(scope ResolutionScope) map[string]string {
	result := map[string]string{
		"query":        scope.Query().Original(),
		"context_kind": string(scope.Context().Kind()),
	}
	exact, ok := scope.Context().(ExactContext)
	if ok {
		result["bounded_context_ref"] = exact.Context().String()
	}
	return result
}

func canonicalCandidateRefs(
	values []typedmemory.PersistedRef,
) []typedmemory.PersistedRef {
	result := append([]typedmemory.PersistedRef{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		leftKey := result[left].RefKind().String() +
			"/" +
			result[left].ReferenceID().String()
		rightKey := result[right].RefKind().String() +
			"/" +
			result[right].ReferenceID().String()
		return leftKey < rightKey
	})
	return slices.Compact(result)
}
