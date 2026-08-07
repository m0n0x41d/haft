package typedmemory

import (
	"fmt"
	"sort"
	"strings"
)

type MissingBasis struct {
	value string
}

func NewMissingBasis(raw string) (MissingBasis, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return MissingBasis{}, fmt.Errorf("missing basis is required")
	}
	return MissingBasis{value: value}, nil
}

func (basis MissingBasis) String() string { return basis.value }

func (basis MissingBasis) valid() bool { return basis.value != "" }

type EntityResolution interface {
	entityResolutionVariant()
}

// AliasAvailability is deliberately disjoint from EntityResolution. Entity
// absence cannot prove an alias is free, and alias availability cannot prove
// an EntityID is absent.
type AliasAvailability interface {
	aliasAvailabilityVariant()
}

type ExactEntityResolution struct {
	entity  EntityID
	context BoundedContextRef
	basis   ResolutionBasisRef
}

func NewExactEntityResolution(
	entity EntityID,
	context BoundedContextRef,
	basis ResolutionBasisRef,
) (ExactEntityResolution, error) {
	if !entity.valid() {
		return ExactEntityResolution{}, fmt.Errorf("exact entity resolution requires an entity ID")
	}
	if !context.valid() {
		return ExactEntityResolution{}, fmt.Errorf("exact entity resolution requires a bounded context")
	}
	if !basis.valid() {
		return ExactEntityResolution{}, fmt.Errorf("exact entity resolution requires a basis")
	}
	return ExactEntityResolution{entity: entity, context: context, basis: basis}, nil
}

func (resolution ExactEntityResolution) Entity() EntityID { return resolution.entity }

func (resolution ExactEntityResolution) Context() BoundedContextRef { return resolution.context }

func (resolution ExactEntityResolution) Basis() ResolutionBasisRef { return resolution.basis }

func (ExactEntityResolution) entityResolutionVariant() {}

// AbsentEntityResolution is positive snapshot evidence that one exact entity
// identity is not allocated in one bounded context. Unsettled resolution is
// deliberately insufficient for DeclareEntity.
type AbsentEntityResolution struct {
	entity  EntityID
	context BoundedContextRef
	basis   ResolutionBasisRef
}

func NewAbsentEntityResolution(
	entity EntityID,
	context BoundedContextRef,
	basis ResolutionBasisRef,
) (AbsentEntityResolution, error) {
	if !entity.valid() || !context.valid() || !basis.valid() {
		return AbsentEntityResolution{}, fmt.Errorf("absent entity resolution requires entity, context, and basis")
	}
	return AbsentEntityResolution{entity: entity, context: context, basis: basis}, nil
}

func (resolution AbsentEntityResolution) Entity() EntityID { return resolution.entity }

func (resolution AbsentEntityResolution) Context() BoundedContextRef { return resolution.context }

func (resolution AbsentEntityResolution) Basis() ResolutionBasisRef { return resolution.basis }

func (AbsentEntityResolution) entityResolutionVariant() {}

// UnknownEntityResolution reports that one exact entity lookup could not be
// settled because named resolution bases are missing. It remains distinct from
// label-based UnsettledEntityResolution: callers can correlate this result to
// the exact EntityID and bounded context they queried.
type UnknownEntityResolution struct {
	entity       EntityID
	context      BoundedContextRef
	missingBasis []MissingBasis
}

func NewUnknownEntityResolution(
	entity EntityID,
	context BoundedContextRef,
	missingBasis []MissingBasis,
) (UnknownEntityResolution, error) {
	if !entity.valid() {
		return UnknownEntityResolution{}, fmt.Errorf("unknown entity resolution requires an entity ID")
	}
	if !context.valid() {
		return UnknownEntityResolution{}, fmt.Errorf("unknown entity resolution requires a bounded context")
	}
	basis, err := normalizeDistinctMissingBasis(missingBasis)
	if err != nil {
		return UnknownEntityResolution{}, err
	}
	return UnknownEntityResolution{
		entity:       entity,
		context:      context,
		missingBasis: basis,
	}, nil
}

func (resolution UnknownEntityResolution) Entity() EntityID { return resolution.entity }

func (resolution UnknownEntityResolution) Context() BoundedContextRef {
	return resolution.context
}

func (resolution UnknownEntityResolution) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), resolution.missingBasis...)
}

func (UnknownEntityResolution) entityResolutionVariant() {}

type BoundAliasResolution struct {
	entity  EntityID
	alias   EntityAlias
	context BoundedContextRef
	basis   ResolutionBasisRef
}

func NewBoundAliasResolution(
	alias EntityAlias,
	entity EntityID,
	context BoundedContextRef,
	basis ResolutionBasisRef,
) (BoundAliasResolution, error) {
	if !alias.valid() || !entity.valid() || !context.valid() || !basis.valid() {
		return BoundAliasResolution{}, fmt.Errorf("bound alias resolution requires alias, entity, context, and basis")
	}
	return BoundAliasResolution{
		entity:  entity,
		alias:   alias,
		context: context,
		basis:   basis,
	}, nil
}

func (resolution BoundAliasResolution) Entity() EntityID { return resolution.entity }

func (resolution BoundAliasResolution) Alias() EntityAlias { return resolution.alias }

func (resolution BoundAliasResolution) Context() BoundedContextRef { return resolution.context }

func (resolution BoundAliasResolution) Basis() ResolutionBasisRef { return resolution.basis }

func (BoundAliasResolution) aliasAvailabilityVariant() {}

// UnboundAliasResolution is positive evidence that one exact alias is free in
// one context. It is separate from unsettled alias availability, whose missing
// basis must remain Underdetermined.
type UnboundAliasResolution struct {
	alias   EntityAlias
	context BoundedContextRef
	basis   ResolutionBasisRef
}

func NewUnboundAliasResolution(
	alias EntityAlias,
	context BoundedContextRef,
	basis ResolutionBasisRef,
) (UnboundAliasResolution, error) {
	if !alias.valid() || !context.valid() || !basis.valid() {
		return UnboundAliasResolution{}, fmt.Errorf("unbound alias resolution requires alias, context, and basis")
	}
	return UnboundAliasResolution{alias: alias, context: context, basis: basis}, nil
}

func (resolution UnboundAliasResolution) Alias() EntityAlias { return resolution.alias }

func (resolution UnboundAliasResolution) Context() BoundedContextRef { return resolution.context }

func (resolution UnboundAliasResolution) Basis() ResolutionBasisRef { return resolution.basis }

func (UnboundAliasResolution) aliasAvailabilityVariant() {}

type EntityCandidate struct {
	entity         EntityID
	matchedAliases []EntityAlias
	contexts       []BoundedContextRef
	basis          ResolutionBasisRef
}

func NewEntityCandidate(
	entity EntityID,
	matchedAliases []EntityAlias,
	contexts []BoundedContextRef,
	basis ResolutionBasisRef,
) (EntityCandidate, error) {
	if !entity.valid() {
		return EntityCandidate{}, fmt.Errorf("entity candidate requires an entity ID")
	}
	if !basis.valid() {
		return EntityCandidate{}, fmt.Errorf("entity candidate requires a resolution basis")
	}

	aliases, err := normalizeAliases(matchedAliases)
	if err != nil {
		return EntityCandidate{}, err
	}
	boundedContexts, err := normalizeContexts(contexts)
	if err != nil {
		return EntityCandidate{}, err
	}
	if len(aliases) == 0 && len(boundedContexts) == 0 {
		return EntityCandidate{}, fmt.Errorf("entity candidate requires a matched alias or context")
	}

	return EntityCandidate{
		entity:         entity,
		matchedAliases: aliases,
		contexts:       boundedContexts,
		basis:          basis,
	}, nil
}

func (candidate EntityCandidate) Entity() EntityID { return candidate.entity }

func (candidate EntityCandidate) MatchedAliases() []EntityAlias {
	return append([]EntityAlias(nil), candidate.matchedAliases...)
}

func (candidate EntityCandidate) Contexts() []BoundedContextRef {
	return append([]BoundedContextRef(nil), candidate.contexts...)
}

func (candidate EntityCandidate) Basis() ResolutionBasisRef { return candidate.basis }

type CandidateEntityResolution struct {
	candidates []EntityCandidate
}

func NewCandidateEntityResolution(candidates []EntityCandidate) (CandidateEntityResolution, error) {
	if len(candidates) == 0 {
		return CandidateEntityResolution{}, fmt.Errorf("candidate entity resolution requires candidates")
	}

	seen := make(map[string]struct{}, len(candidates))
	values := make([]EntityCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		key := candidate.Entity().String()
		if key == "" {
			return CandidateEntityResolution{}, fmt.Errorf("candidate entity resolution contains an invalid candidate")
		}
		if _, exists := seen[key]; exists {
			return CandidateEntityResolution{}, fmt.Errorf("candidate entity resolution repeats entity %q", key)
		}
		seen[key] = struct{}{}
		values = append(values, candidate)
	}

	return CandidateEntityResolution{candidates: values}, nil
}

func (resolution CandidateEntityResolution) Candidates() []EntityCandidate {
	return append([]EntityCandidate(nil), resolution.candidates...)
}

func (CandidateEntityResolution) entityResolutionVariant() {}

type CandidateAliasResolution struct {
	alias      EntityAlias
	context    BoundedContextRef
	candidates []EntityCandidate
}

func NewCandidateAliasResolution(
	alias EntityAlias,
	context BoundedContextRef,
	candidates []EntityCandidate,
) (CandidateAliasResolution, error) {
	if !alias.valid() || !context.valid() {
		return CandidateAliasResolution{}, fmt.Errorf("candidate alias resolution requires alias and context")
	}
	resolution, err := NewCandidateEntityResolution(candidates)
	if err != nil {
		return CandidateAliasResolution{}, err
	}
	for _, candidate := range resolution.candidates {
		if !candidateMatchesAliasQuery(candidate, alias, context) {
			return CandidateAliasResolution{}, fmt.Errorf(
				"alias candidate %q does not match the exact alias/context query",
				candidate.Entity().String(),
			)
		}
	}
	return CandidateAliasResolution{
		alias:      alias,
		context:    context,
		candidates: append([]EntityCandidate(nil), resolution.candidates...),
	}, nil
}

func (resolution CandidateAliasResolution) Alias() EntityAlias { return resolution.alias }

func (resolution CandidateAliasResolution) Context() BoundedContextRef {
	return resolution.context
}

func (resolution CandidateAliasResolution) Candidates() []EntityCandidate {
	return append([]EntityCandidate(nil), resolution.candidates...)
}

func (CandidateAliasResolution) aliasAvailabilityVariant() {}

type UnsettledEntityResolution struct {
	proposedLabel string
	missingBasis  []MissingBasis
}

func NewUnsettledEntityResolution(
	proposedLabel string,
	missingBasis []MissingBasis,
) (UnsettledEntityResolution, error) {
	label := strings.TrimSpace(proposedLabel)
	if label == "" {
		return UnsettledEntityResolution{}, fmt.Errorf("unsettled entity resolution requires a proposed label")
	}
	if len(missingBasis) == 0 {
		return UnsettledEntityResolution{}, fmt.Errorf("unsettled entity resolution requires missing basis")
	}

	basis, err := normalizeMissingBasis(missingBasis)
	if err != nil {
		return UnsettledEntityResolution{}, err
	}

	return UnsettledEntityResolution{proposedLabel: label, missingBasis: basis}, nil
}

func (resolution UnsettledEntityResolution) ProposedLabel() string {
	return resolution.proposedLabel
}

func (resolution UnsettledEntityResolution) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), resolution.missingBasis...)
}

func (UnsettledEntityResolution) entityResolutionVariant() {}

type UnsettledAliasResolution struct {
	alias        EntityAlias
	context      BoundedContextRef
	missingBasis []MissingBasis
}

func NewUnsettledAliasResolution(
	alias EntityAlias,
	context BoundedContextRef,
	missingBasis []MissingBasis,
) (UnsettledAliasResolution, error) {
	if !alias.valid() || !context.valid() {
		return UnsettledAliasResolution{}, fmt.Errorf("unsettled alias resolution requires alias and context")
	}
	basis, err := normalizeMissingBasis(missingBasis)
	if err != nil {
		return UnsettledAliasResolution{}, err
	}
	return UnsettledAliasResolution{
		alias:        alias,
		context:      context,
		missingBasis: basis,
	}, nil
}

func (resolution UnsettledAliasResolution) Alias() EntityAlias { return resolution.alias }

func (resolution UnsettledAliasResolution) Context() BoundedContextRef {
	return resolution.context
}

func (resolution UnsettledAliasResolution) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), resolution.missingBasis...)
}

func (UnsettledAliasResolution) aliasAvailabilityVariant() {}

func candidateMatchesAliasQuery(
	candidate EntityCandidate,
	alias EntityAlias,
	context BoundedContextRef,
) bool {
	aliasMatched := false
	for _, candidateAlias := range candidate.matchedAliases {
		if candidateAlias == alias {
			aliasMatched = true
			break
		}
	}
	if !aliasMatched {
		return false
	}
	for _, candidateContext := range candidate.contexts {
		if candidateContext == context {
			return true
		}
	}
	return false
}

func normalizeMissingBasis(values []MissingBasis) ([]MissingBasis, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("resolution requires missing basis")
	}
	basis := make([]MissingBasis, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !value.valid() {
			return nil, fmt.Errorf("resolution contains empty missing basis")
		}
		if _, exists := seen[value.String()]; exists {
			continue
		}
		seen[value.String()] = struct{}{}
		basis = append(basis, value)
	}
	sort.Slice(basis, func(left, right int) bool {
		return basis[left].String() < basis[right].String()
	})
	return basis, nil
}

func normalizeDistinctMissingBasis(values []MissingBasis) ([]MissingBasis, error) {
	basis, err := normalizeMissingBasis(values)
	if err != nil {
		return nil, err
	}
	if len(basis) != len(values) {
		return nil, fmt.Errorf("resolution repeats missing basis")
	}
	return basis, nil
}

type IdentityChange interface {
	identityChangeVariant()
	validIdentityChange() bool
}

func validIdentityChangeVariant(change IdentityChange) bool {
	switch value := change.(type) {
	case AdmitAlias:
		return value.validIdentityChange()
	case SupersedeAlias:
		return value.validIdentityChange()
	case MergeEntities:
		return value.validIdentityChange()
	case SplitEntity:
		return value.validIdentityChange()
	default:
		return false
	}
}

type AdmitAlias struct {
	entity     EntityID
	alias      EntityAlias
	context    BoundedContextRef
	provenance ProvenanceRef
}

func NewAdmitAlias(
	entity EntityID,
	alias EntityAlias,
	context BoundedContextRef,
	provenance ProvenanceRef,
) (AdmitAlias, error) {
	if !entity.valid() || !alias.valid() || !context.valid() || !provenance.valid() {
		return AdmitAlias{}, fmt.Errorf("alias admission requires entity, alias, context, and provenance")
	}
	return AdmitAlias{entity: entity, alias: alias, context: context, provenance: provenance}, nil
}

func (change AdmitAlias) Entity() EntityID { return change.entity }

func (change AdmitAlias) Alias() EntityAlias { return change.alias }

func (change AdmitAlias) Context() BoundedContextRef { return change.context }

func (change AdmitAlias) Provenance() ProvenanceRef { return change.provenance }

func (AdmitAlias) identityChangeVariant() {}

func (change AdmitAlias) validIdentityChange() bool {
	return change.entity.valid() && change.alias.valid() && change.context.valid() && change.provenance.valid()
}

type SupersedeAlias struct {
	entity      EntityID
	oldAlias    EntityAlias
	replacement EntityAlias
	context     BoundedContextRef
	provenance  ProvenanceRef
}

func NewSupersedeAlias(
	entity EntityID,
	oldAlias EntityAlias,
	replacement EntityAlias,
	context BoundedContextRef,
	provenance ProvenanceRef,
) (SupersedeAlias, error) {
	if !entity.valid() || !oldAlias.valid() || !replacement.valid() || !context.valid() || !provenance.valid() {
		return SupersedeAlias{}, fmt.Errorf("alias supersession requires entity, aliases, context, and provenance")
	}
	if oldAlias.String() == replacement.String() {
		return SupersedeAlias{}, fmt.Errorf("replacement alias must differ from superseded alias")
	}
	return SupersedeAlias{
		entity:      entity,
		oldAlias:    oldAlias,
		replacement: replacement,
		context:     context,
		provenance:  provenance,
	}, nil
}

func (change SupersedeAlias) Entity() EntityID { return change.entity }

func (change SupersedeAlias) OldAlias() EntityAlias { return change.oldAlias }

func (change SupersedeAlias) Replacement() EntityAlias { return change.replacement }

func (change SupersedeAlias) Context() BoundedContextRef { return change.context }

func (change SupersedeAlias) Provenance() ProvenanceRef { return change.provenance }

func (SupersedeAlias) identityChangeVariant() {}

func (change SupersedeAlias) validIdentityChange() bool {
	return change.entity.valid() &&
		change.oldAlias.valid() &&
		change.replacement.valid() &&
		change.oldAlias.String() != change.replacement.String() &&
		change.context.valid() &&
		change.provenance.valid()
}

type MergeEntities struct {
	survivor EntityID
	merged   []EntityID
	context  BoundedContextRef
	basis    ReconciliationBasisRef
}

func NewMergeEntities(
	survivor EntityID,
	merged []EntityID,
	context BoundedContextRef,
	basis ReconciliationBasisRef,
) (MergeEntities, error) {
	if !survivor.valid() || !context.valid() || !basis.valid() {
		return MergeEntities{}, fmt.Errorf("entity merge requires survivor, context, and reconciliation basis")
	}
	entities, err := normalizeDistinctEntities(merged, survivor)
	if err != nil {
		return MergeEntities{}, err
	}
	if len(entities) == 0 {
		return MergeEntities{}, fmt.Errorf("entity merge requires at least one non-surviving entity")
	}
	return MergeEntities{survivor: survivor, merged: entities, context: context, basis: basis}, nil
}

func (change MergeEntities) Survivor() EntityID { return change.survivor }

func (change MergeEntities) Merged() []EntityID { return append([]EntityID(nil), change.merged...) }

func (change MergeEntities) Context() BoundedContextRef { return change.context }

func (change MergeEntities) Basis() ReconciliationBasisRef { return change.basis }

func (MergeEntities) identityChangeVariant() {}

func (change MergeEntities) validIdentityChange() bool {
	return change.survivor.valid() && len(change.merged) > 0 && change.context.valid() && change.basis.valid()
}

type SplitEntity struct {
	source  EntityID
	targets []EntityID
	context BoundedContextRef
	basis   ReconciliationBasisRef
}

func NewSplitEntity(
	source EntityID,
	targets []EntityID,
	context BoundedContextRef,
	basis ReconciliationBasisRef,
) (SplitEntity, error) {
	if !source.valid() || !context.valid() || !basis.valid() {
		return SplitEntity{}, fmt.Errorf("entity split requires source, context, and reconciliation basis")
	}
	entities, err := normalizeDistinctEntities(targets, source)
	if err != nil {
		return SplitEntity{}, err
	}
	if len(entities) < 2 {
		return SplitEntity{}, fmt.Errorf("entity split requires at least two target entities")
	}
	return SplitEntity{source: source, targets: entities, context: context, basis: basis}, nil
}

func (change SplitEntity) Source() EntityID { return change.source }

func (change SplitEntity) Targets() []EntityID { return append([]EntityID(nil), change.targets...) }

func (change SplitEntity) Context() BoundedContextRef { return change.context }

func (change SplitEntity) Basis() ReconciliationBasisRef { return change.basis }

func (SplitEntity) identityChangeVariant() {}

func (change SplitEntity) validIdentityChange() bool {
	return change.source.valid() && len(change.targets) >= 2 && change.context.valid() && change.basis.valid()
}

func normalizeAliases(values []EntityAlias) ([]EntityAlias, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]EntityAlias, 0, len(values))
	for _, value := range values {
		if !value.valid() {
			return nil, fmt.Errorf("entity alias is required")
		}
		if _, exists := seen[value.String()]; exists {
			continue
		}
		seen[value.String()] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].String() < result[right].String()
	})
	return result, nil
}

func normalizeContexts(values []BoundedContextRef) ([]BoundedContextRef, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]BoundedContextRef, 0, len(values))
	for _, value := range values {
		if !value.valid() {
			return nil, fmt.Errorf("bounded context is required")
		}
		if _, exists := seen[value.String()]; exists {
			continue
		}
		seen[value.String()] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].String() < result[right].String()
	})
	return result, nil
}

func normalizeDistinctEntities(values []EntityID, excluded EntityID) ([]EntityID, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]EntityID, 0, len(values))
	for _, value := range values {
		if !value.valid() {
			return nil, fmt.Errorf("entity ID is required")
		}
		if value.String() == excluded.String() {
			return nil, fmt.Errorf("entity %q cannot appear on both sides of identity change", value.String())
		}
		if _, exists := seen[value.String()]; exists {
			return nil, fmt.Errorf("identity change repeats entity %q", value.String())
		}
		seen[value.String()] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].String() < result[right].String()
	})
	return result, nil
}
