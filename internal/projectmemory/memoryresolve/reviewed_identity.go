package memoryresolve

import (
	"fmt"
	"slices"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ReviewedIdentityResolution is a read-only projection of one durable manual
// merge or split. It contains no admission, mutation, head-selection, or
// authority capability.
type ReviewedIdentityResolution interface {
	HistoricalEntity() typedmemory.EntityID
	Context() typedmemory.BoundedContextRef
	isReviewedIdentityResolution()
}

type ReviewedMergedIdentity struct {
	historical typedmemory.EntityID
	current    typedmemory.EntityID
	context    typedmemory.BoundedContextRef
	history    []typedmemory.ResolutionBasisRef
}

func NewReviewedMergedIdentity(
	historical typedmemory.EntityID,
	current typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	history []typedmemory.ResolutionBasisRef,
) (ReviewedMergedIdentity, error) {
	value := ReviewedMergedIdentity{
		historical: historical,
		current:    current,
		context:    contextRef,
		history:    append([]typedmemory.ResolutionBasisRef(nil), history...),
	}
	if !value.valid() {
		return ReviewedMergedIdentity{}, fmt.Errorf(
			"reviewed merged-identity resolution is invalid",
		)
	}
	return value, nil
}

func (resolution ReviewedMergedIdentity) HistoricalEntity() typedmemory.EntityID {
	return resolution.historical
}

func (resolution ReviewedMergedIdentity) CurrentEntity() typedmemory.EntityID {
	return resolution.current
}

func (resolution ReviewedMergedIdentity) Context() typedmemory.BoundedContextRef {
	return resolution.context
}

func (resolution ReviewedMergedIdentity) ReconciliationHistory() []typedmemory.ResolutionBasisRef {
	return append([]typedmemory.ResolutionBasisRef(nil), resolution.history...)
}

func (ReviewedMergedIdentity) isReviewedIdentityResolution() {}

func (resolution ReviewedMergedIdentity) valid() bool {
	return validReviewedEntity(resolution.historical) &&
		validReviewedEntity(resolution.current) &&
		resolution.historical != resolution.current &&
		validReviewedContext(resolution.context) &&
		validReviewedHistory(resolution.history)
}

type ReviewedSplitIdentity struct {
	historical typedmemory.EntityID
	candidates []typedmemory.EntityID
	context    typedmemory.BoundedContextRef
	history    []typedmemory.ResolutionBasisRef
}

func NewReviewedSplitIdentity(
	historical typedmemory.EntityID,
	candidates []typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	history []typedmemory.ResolutionBasisRef,
) (ReviewedSplitIdentity, error) {
	canonical := canonicalReviewedCandidates(candidates)
	value := ReviewedSplitIdentity{
		historical: historical,
		candidates: canonical,
		context:    contextRef,
		history:    append([]typedmemory.ResolutionBasisRef(nil), history...),
	}
	if len(canonical) != len(candidates) || !value.valid() {
		return ReviewedSplitIdentity{}, fmt.Errorf(
			"reviewed split-identity resolution is invalid",
		)
	}
	return value, nil
}

func (resolution ReviewedSplitIdentity) HistoricalEntity() typedmemory.EntityID {
	return resolution.historical
}

func (resolution ReviewedSplitIdentity) Candidates() []typedmemory.EntityID {
	return append([]typedmemory.EntityID(nil), resolution.candidates...)
}

func (resolution ReviewedSplitIdentity) Context() typedmemory.BoundedContextRef {
	return resolution.context
}

func (resolution ReviewedSplitIdentity) Reconciliation() typedmemory.ResolutionBasisRef {
	if len(resolution.history) == 0 {
		return typedmemory.ResolutionBasisRef{}
	}
	return resolution.history[len(resolution.history)-1]
}

func (resolution ReviewedSplitIdentity) ReconciliationHistory() []typedmemory.ResolutionBasisRef {
	return append([]typedmemory.ResolutionBasisRef(nil), resolution.history...)
}

func (ReviewedSplitIdentity) isReviewedIdentityResolution() {}

func (resolution ReviewedSplitIdentity) valid() bool {
	if !validReviewedEntity(resolution.historical) ||
		!validReviewedContext(resolution.context) ||
		len(resolution.candidates) < 2 ||
		!validReviewedHistory(resolution.history) {
		return false
	}
	for _, candidate := range resolution.candidates {
		if !validReviewedEntity(candidate) || candidate == resolution.historical {
			return false
		}
	}
	return slices.Equal(
		resolution.candidates,
		canonicalReviewedCandidates(resolution.candidates),
	)
}

// ReviewedIdentityIndex binds the complete reviewed reconciliation projection
// to the same immutable snapshot as the entity directory.
type ReviewedIdentityIndex struct {
	snapshot neighborhood.SnapshotBasis
	digest   typedmemory.SHA256Digest
	entries  []ReviewedIdentityResolution
}

func NewReviewedIdentityIndex(
	snapshot neighborhood.SnapshotBasis,
	digest typedmemory.SHA256Digest,
	entries []ReviewedIdentityResolution,
) (ReviewedIdentityIndex, error) {
	owned := append([]ReviewedIdentityResolution(nil), entries...)
	sort.Slice(owned, func(left int, right int) bool {
		return reviewedIdentityKey(owned[left]) < reviewedIdentityKey(owned[right])
	})
	index := ReviewedIdentityIndex{
		snapshot: snapshot,
		digest:   digest,
		entries:  owned,
	}
	if !index.Valid() {
		return ReviewedIdentityIndex{}, fmt.Errorf(
			"reviewed identity index is invalid",
		)
	}
	return index, nil
}

func (index ReviewedIdentityIndex) SnapshotBasis() neighborhood.SnapshotBasis {
	return index.snapshot
}

func (index ReviewedIdentityIndex) Digest() typedmemory.SHA256Digest {
	return index.digest
}

func (index ReviewedIdentityIndex) Entries() []ReviewedIdentityResolution {
	return append([]ReviewedIdentityResolution(nil), index.entries...)
}

func (index ReviewedIdentityIndex) Valid() bool {
	if !index.snapshot.Valid() || index.digest.String() == "" {
		return false
	}
	for position, entry := range index.entries {
		if !reviewedIdentityResolutionValid(entry) {
			return false
		}
		if position > 0 &&
			reviewedIdentityKey(index.entries[position-1]) == reviewedIdentityKey(entry) {
			return false
		}
	}
	return true
}

// ResolveWithReviewedIdentity applies committed reconciliation before ordinary
// alias or lexical resolution. A merge returns the canonical current entity
// with each durable reconciliation step as an explicit witness. A split
// returns ResolutionUnsettled with the exact candidate set and never ranks or
// selects a successor.
func ResolveWithReviewedIdentity(
	request ResolutionRequest,
	index ResolutionIndex,
	reviewed ReviewedIdentityIndex,
) (EntityResolutionResult, error) {
	if !index.Valid() || !reviewed.Valid() {
		return nil, fmt.Errorf("reviewed identity resolution indexes are invalid")
	}
	if request.SnapshotBasis() != index.SnapshotBasis() {
		return newResolutionRetryRequired(request, index.SnapshotBasis())
	}
	if reviewed.SnapshotBasis() != index.SnapshotBasis() {
		return nil, fmt.Errorf(
			"reviewed identity index differs from entity resolution snapshot",
		)
	}
	matches, err := reviewedIdentityMatches(
		request,
		index.Units(),
		reviewed.Entries(),
	)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return Resolve(request, index)
	}
	if reviewedIdentityContextIsAmbiguous(request, index, matches) {
		issue := newContextNotResolvedIssue(request.Query().Original())
		return newResolutionUnsettled(
			request,
			index.SnapshotBasis(),
			[]ResolutionBasisIssue{issue},
		)
	}
	switch resolution := matches[0].(type) {
	case ReviewedMergedIdentity:
		return resolveReviewedMerge(request, index, resolution)
	case ReviewedSplitIdentity:
		return resolveReviewedSplit(request, index, resolution)
	default:
		return nil, fmt.Errorf(
			"unsupported reviewed identity resolution %T",
			matches[0],
		)
	}
}

func resolveReviewedMerge(
	request ResolutionRequest,
	index ResolutionIndex,
	resolution ReviewedMergedIdentity,
) (EntityResolutionResult, error) {
	unit, found := exactResolutionUnit(
		index.Units(),
		resolution.CurrentEntity(),
		resolution.Context(),
	)
	if !found {
		return nil, fmt.Errorf(
			"reviewed merge current entity is absent from the exact resolution index",
		)
	}
	witnesses := make([]ResolutionWitness, 0, len(resolution.ReconciliationHistory()))
	for _, basis := range resolution.ReconciliationHistory() {
		witnesses = append(witnesses, newResolutionWitness(
			WitnessReviewedMerge,
			request.Query().Original(),
			basis,
		))
	}
	return newExactEntity(request, unit, witnesses)
}

func resolveReviewedSplit(
	request ResolutionRequest,
	index ResolutionIndex,
	resolution ReviewedSplitIdentity,
) (EntityResolutionResult, error) {
	refs := make([]typedmemory.PersistedRef, 0, len(resolution.Candidates()))
	for _, candidate := range resolution.Candidates() {
		unit, found := exactResolutionUnit(
			index.Units(),
			candidate,
			resolution.Context(),
		)
		if !found {
			return nil, fmt.Errorf(
				"reviewed split candidate is absent from the exact resolution index",
			)
		}
		refs = append(refs, unit.Entity())
	}
	issue, err := NewReviewedSplitCandidatesIssue(
		resolution.HistoricalEntity(),
		refs,
		resolution.ReconciliationHistory(),
	)
	if err != nil {
		return nil, err
	}
	return newResolutionUnsettled(
		request,
		index.SnapshotBasis(),
		[]ResolutionBasisIssue{issue},
	)
}

func reviewedIdentityMatches(
	request ResolutionRequest,
	units []ResolutionUnit,
	entries []ReviewedIdentityResolution,
) ([]ReviewedIdentityResolution, error) {
	result := make([]ReviewedIdentityResolution, 0)
	for _, entry := range entries {
		historical, found := exactResolutionUnit(
			units,
			entry.HistoricalEntity(),
			entry.Context(),
		)
		if !found {
			return nil, fmt.Errorf(
				"reviewed historical entity is absent from the exact resolution index",
			)
		}
		if !queryMatchesReviewedEntity(request.Query(), historical.Entity()) ||
			!queryContextCovers(request.Context(), entry.Context()) {
			continue
		}
		result = append(result, entry)
	}
	return result, nil
}

func reviewedIdentityContextIsAmbiguous(
	request ResolutionRequest,
	index ResolutionIndex,
	matches []ReviewedIdentityResolution,
) bool {
	if len(matches) != 1 {
		return true
	}
	if request.Context().Kind() == QueryExactContext {
		return false
	}
	normal := exactResolutionMatches(request.Query(), index.Units())
	for _, match := range normal {
		if match.unit.Context() != matches[0].Context() {
			return true
		}
	}
	return false
}

func exactResolutionUnit(
	units []ResolutionUnit,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) (ResolutionUnit, bool) {
	for _, unit := range units {
		if unit.Entity().ReferenceID().String() == entity.String() &&
			unit.Context() == contextRef {
			return unit, true
		}
	}
	return ResolutionUnit{}, false
}

func queryMatchesReviewedEntity(
	query ResolutionQuery,
	entity typedmemory.PersistedRef,
) bool {
	exactReference := entity.RefKind().String() +
		"/reference/" +
		entity.ReferenceID().String()
	return query.Original() == entity.ReferenceID().String() ||
		query.Original() == exactReference
}

func queryContextCovers(
	query QueryContext,
	contextRef typedmemory.BoundedContextRef,
) bool {
	if query.Kind() == QueryAnyContext {
		return true
	}
	exact, ok := query.(ExactContext)
	return ok && exact.Context() == contextRef
}

func reviewedIdentityResolutionValid(value ReviewedIdentityResolution) bool {
	switch resolution := value.(type) {
	case ReviewedMergedIdentity:
		return resolution.valid()
	case ReviewedSplitIdentity:
		return resolution.valid()
	default:
		return false
	}
}

func reviewedIdentityKey(value ReviewedIdentityResolution) string {
	return value.Context().String() + "\x1f" + value.HistoricalEntity().String()
}

func validReviewedEntity(entity typedmemory.EntityID) bool {
	canonical, err := typedmemory.NewEntityID(entity.String())
	return err == nil && canonical == entity
}

func validReviewedContext(contextRef typedmemory.BoundedContextRef) bool {
	canonical, err := typedmemory.NewBoundedContextRef(contextRef.String())
	return err == nil && canonical == contextRef
}

func validReviewedBasis(basis typedmemory.ResolutionBasisRef) bool {
	canonical, err := typedmemory.NewResolutionBasisRef(basis.String())
	return err == nil && canonical == basis
}

func validReviewedHistory(history []typedmemory.ResolutionBasisRef) bool {
	if len(history) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(history))
	for _, basis := range history {
		if !validReviewedBasis(basis) {
			return false
		}
		if _, found := seen[basis.String()]; found {
			return false
		}
		seen[basis.String()] = struct{}{}
	}
	return true
}

func canonicalReviewedCandidates(
	values []typedmemory.EntityID,
) []typedmemory.EntityID {
	result := append([]typedmemory.EntityID(nil), values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].String() < result[right].String()
	})
	return slices.Compact(result)
}
