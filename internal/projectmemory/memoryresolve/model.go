// Package memoryresolve owns snapshot-bound EntityOfConcern identity discovery.
// Its candidates contain no current project relations and cannot be used as an
// exact neighborhood scope until one entity/context is resolved.
package memoryresolve

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type ResolutionIndexRef struct{ value string }
type ResolutionIndexVersion struct{ value string }
type ResolutionCompletenessBasisRef struct{ value string }

func NewResolutionIndexRef(raw string) (ResolutionIndexRef, error) {
	value, err := exactOneLine("resolution index", raw)
	if err != nil {
		return ResolutionIndexRef{}, err
	}
	return ResolutionIndexRef{value: value}, nil
}

func NewResolutionIndexVersion(raw string) (ResolutionIndexVersion, error) {
	value, err := exactOneLine("resolution index version", raw)
	if err != nil {
		return ResolutionIndexVersion{}, err
	}
	return ResolutionIndexVersion{value: value}, nil
}

func NewResolutionCompletenessBasisRef(
	raw string,
) (ResolutionCompletenessBasisRef, error) {
	value, err := exactOneLine("resolution completeness basis", raw)
	if err != nil {
		return ResolutionCompletenessBasisRef{}, err
	}
	return ResolutionCompletenessBasisRef{value: value}, nil
}

func (ref ResolutionIndexRef) String() string { return ref.value }
func (version ResolutionIndexVersion) String() string {
	return version.value
}
func (ref ResolutionCompletenessBasisRef) String() string {
	return ref.value
}

type ResolutionUnit struct {
	entity     typedmemory.PersistedRef
	context    typedmemory.BoundedContextRef
	label      neighborhood.ReadableItemText
	aliases    []typedmemory.EntityAlias
	provenance typedmemory.ProvenanceRef
	basis      typedmemory.ResolutionBasisRef
}

func NewResolutionUnit(
	entity typedmemory.PersistedRef,
	context typedmemory.BoundedContextRef,
	label neighborhood.ReadableItemText,
	aliases []typedmemory.EntityAlias,
	provenance typedmemory.ProvenanceRef,
	basis typedmemory.ResolutionBasisRef,
) (ResolutionUnit, error) {
	unit := ResolutionUnit{
		entity:     entity,
		context:    context,
		label:      label,
		aliases:    canonicalAliases(aliases),
		provenance: provenance,
		basis:      basis,
	}
	if !unit.Valid() {
		return ResolutionUnit{}, fmt.Errorf("resolution unit is invalid")
	}
	return unit, nil
}

func (unit ResolutionUnit) Entity() typedmemory.PersistedRef {
	return unit.entity
}

func (unit ResolutionUnit) Context() typedmemory.BoundedContextRef {
	return unit.context
}

func (unit ResolutionUnit) Label() neighborhood.ReadableItemText {
	return unit.label
}

func (unit ResolutionUnit) Aliases() []typedmemory.EntityAlias {
	return append([]typedmemory.EntityAlias{}, unit.aliases...)
}

func (unit ResolutionUnit) Provenance() typedmemory.ProvenanceRef {
	return unit.provenance
}

func (unit ResolutionUnit) Basis() typedmemory.ResolutionBasisRef {
	return unit.basis
}

func (unit ResolutionUnit) Valid() bool {
	if !validPersistedRef(unit.entity) || unit.label.String() == "" {
		return false
	}
	context, contextErr := typedmemory.NewBoundedContextRef(
		unit.context.String(),
	)
	provenance, provenanceErr := typedmemory.NewProvenanceRef(
		unit.provenance.String(),
	)
	basis, basisErr := typedmemory.NewResolutionBasisRef(
		unit.basis.String(),
	)
	return contextErr == nil &&
		provenanceErr == nil &&
		basisErr == nil &&
		context == unit.context &&
		provenance == unit.provenance &&
		basis == unit.basis &&
		aliasesValidAndCanonical(unit.aliases)
}

func (unit ResolutionUnit) key() string {
	return unit.entity.RefKind().String() +
		"|" +
		unit.entity.ReferenceID().String() +
		"|" +
		unit.context.String()
}

type ResolutionIndexCompletenessKind string

const (
	CompletenessAllContexts   ResolutionIndexCompletenessKind = "all_contexts"
	CompletenessNamedContexts ResolutionIndexCompletenessKind = "named_contexts"
)

type ResolutionIndexCompleteness interface {
	Kind() ResolutionIndexCompletenessKind
	Basis() ResolutionCompletenessBasisRef
	Covers(context typedmemory.BoundedContextRef) bool
	CoversAllContexts() bool
	isResolutionIndexCompleteness()
}

type CompleteAllContexts struct {
	basis ResolutionCompletenessBasisRef
}

func NewCompleteAllContexts(
	basis ResolutionCompletenessBasisRef,
) (CompleteAllContexts, error) {
	if basis.String() == "" {
		return CompleteAllContexts{}, fmt.Errorf(
			"all-context completeness requires exact basis",
		)
	}
	return CompleteAllContexts{basis: basis}, nil
}

func (CompleteAllContexts) Kind() ResolutionIndexCompletenessKind {
	return CompletenessAllContexts
}

func (completeness CompleteAllContexts) Basis() ResolutionCompletenessBasisRef {
	return completeness.basis
}

func (CompleteAllContexts) Covers(typedmemory.BoundedContextRef) bool {
	return true
}

func (CompleteAllContexts) CoversAllContexts() bool        { return true }
func (CompleteAllContexts) isResolutionIndexCompleteness() {}

type CompleteNamedContexts struct {
	contexts []typedmemory.BoundedContextRef
	basis    ResolutionCompletenessBasisRef
}

func NewCompleteNamedContexts(
	contexts []typedmemory.BoundedContextRef,
	basis ResolutionCompletenessBasisRef,
) (CompleteNamedContexts, error) {
	values := canonicalContexts(contexts)
	if len(values) == 0 ||
		len(values) != len(contexts) ||
		basis.String() == "" {
		return CompleteNamedContexts{}, fmt.Errorf(
			"named-context completeness is invalid",
		)
	}
	return CompleteNamedContexts{
		contexts: values,
		basis:    basis,
	}, nil
}

func (CompleteNamedContexts) Kind() ResolutionIndexCompletenessKind {
	return CompletenessNamedContexts
}

func (completeness CompleteNamedContexts) Basis() ResolutionCompletenessBasisRef {
	return completeness.basis
}

func (completeness CompleteNamedContexts) Covers(
	context typedmemory.BoundedContextRef,
) bool {
	return slices.Contains(completeness.contexts, context)
}

func (CompleteNamedContexts) CoversAllContexts() bool        { return false }
func (CompleteNamedContexts) isResolutionIndexCompleteness() {}

type ResolutionIndex struct {
	ref          ResolutionIndexRef
	version      ResolutionIndexVersion
	snapshot     neighborhood.SnapshotBasis
	completeness ResolutionIndexCompleteness
	units        []ResolutionUnit
}

func NewResolutionIndex(
	ref ResolutionIndexRef,
	version ResolutionIndexVersion,
	snapshot neighborhood.SnapshotBasis,
	completeness ResolutionIndexCompleteness,
	units []ResolutionUnit,
) (ResolutionIndex, error) {
	index := ResolutionIndex{
		ref:          ref,
		version:      version,
		snapshot:     snapshot,
		completeness: completeness,
		units:        canonicalUnits(units),
	}
	if !index.Valid() {
		return ResolutionIndex{}, fmt.Errorf("resolution index is invalid")
	}
	return index, nil
}

func (index ResolutionIndex) Ref() ResolutionIndexRef {
	return index.ref
}

func (index ResolutionIndex) Version() ResolutionIndexVersion {
	return index.version
}

func (index ResolutionIndex) SnapshotBasis() neighborhood.SnapshotBasis {
	return index.snapshot
}

func (index ResolutionIndex) Completeness() ResolutionIndexCompleteness {
	return index.completeness
}

func (index ResolutionIndex) Units() []ResolutionUnit {
	return append([]ResolutionUnit{}, index.units...)
}

func (index ResolutionIndex) Valid() bool {
	if index.ref.String() == "" ||
		index.version.String() == "" ||
		!index.snapshot.Valid() ||
		index.completeness == nil {
		return false
	}
	seen := make(map[string]struct{}, len(index.units))
	for _, unit := range index.units {
		if !unit.Valid() ||
			unit.Entity().RefKind().TypeEnv() != index.snapshot.TypeEnv() {
			return false
		}
		if _, found := seen[unit.key()]; found {
			return false
		}
		seen[unit.key()] = struct{}{}
	}
	switch value := index.completeness.(type) {
	case CompleteAllContexts:
		return value.Basis().String() != ""
	case CompleteNamedContexts:
		return len(value.contexts) > 0 && value.Basis().String() != ""
	default:
		return false
	}
}

type QueryContextKind string

const (
	QueryAnyContext   QueryContextKind = "any_context"
	QueryExactContext QueryContextKind = "exact_context"
)

type QueryContext interface {
	Kind() QueryContextKind
	isQueryContext()
}

type AnyContext struct{}

func (AnyContext) Kind() QueryContextKind { return QueryAnyContext }
func (AnyContext) isQueryContext()        {}

type ExactContext struct {
	context typedmemory.BoundedContextRef
}

func NewExactContext(
	context typedmemory.BoundedContextRef,
) (ExactContext, error) {
	parsed, err := typedmemory.NewBoundedContextRef(context.String())
	if err != nil || parsed != context {
		return ExactContext{}, fmt.Errorf(
			"exact resolution context is invalid",
		)
	}
	return ExactContext{context: context}, nil
}

func (ExactContext) Kind() QueryContextKind { return QueryExactContext }
func (context ExactContext) Context() typedmemory.BoundedContextRef {
	return context.context
}
func (ExactContext) isQueryContext() {}

type ResolutionQuery struct {
	original string
	terms    []string
}

func NewResolutionQuery(raw string) (ResolutionQuery, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw {
		return ResolutionQuery{}, fmt.Errorf(
			"resolution query must be exact and non-empty",
		)
	}
	terms := lexicalTerms(value)
	if len(terms) == 0 {
		return ResolutionQuery{}, fmt.Errorf(
			"resolution query has no searchable terms",
		)
	}
	return ResolutionQuery{
		original: value,
		terms:    terms,
	}, nil
}

func (query ResolutionQuery) Original() string {
	return query.original
}

func (query ResolutionQuery) Terms() []string {
	return append([]string{}, query.terms...)
}

type ResolutionRequest struct {
	query         ResolutionQuery
	context       QueryContext
	snapshot      neighborhood.SnapshotBasis
	maxCandidates uint32
}

func NewResolutionRequest(
	query ResolutionQuery,
	context QueryContext,
	snapshot neighborhood.SnapshotBasis,
	maxCandidates uint32,
) (ResolutionRequest, error) {
	request := ResolutionRequest{
		query:         query,
		context:       context,
		snapshot:      snapshot,
		maxCandidates: maxCandidates,
	}
	if query.Original() == "" ||
		context == nil ||
		!snapshot.Valid() ||
		maxCandidates == 0 ||
		!validQueryContext(context) {
		return ResolutionRequest{}, fmt.Errorf(
			"entity resolution request is invalid",
		)
	}
	return request, nil
}

func (request ResolutionRequest) Query() ResolutionQuery {
	return request.query
}

func (request ResolutionRequest) Context() QueryContext {
	return request.context
}

func (request ResolutionRequest) SnapshotBasis() neighborhood.SnapshotBasis {
	return request.snapshot
}

func (request ResolutionRequest) MaxCandidates() uint32 {
	return request.maxCandidates
}

func (request ResolutionRequest) Scope() ResolutionScope {
	return ResolutionScope{
		query:   request.query,
		context: request.context,
	}
}

type ResolutionScope struct {
	query   ResolutionQuery
	context QueryContext
}

func (scope ResolutionScope) Query() ResolutionQuery {
	return scope.query
}

func (scope ResolutionScope) Context() QueryContext {
	return scope.context
}

func (scope ResolutionScope) Valid() bool {
	return scope.query.Original() != "" &&
		validQueryContext(scope.context)
}

func validQueryContext(context QueryContext) bool {
	switch value := context.(type) {
	case AnyContext:
		return true
	case ExactContext:
		parsed, err := typedmemory.NewBoundedContextRef(
			value.Context().String(),
		)
		return err == nil && parsed == value.Context()
	default:
		return false
	}
}

func canonicalAliases(values []typedmemory.EntityAlias) []typedmemory.EntityAlias {
	result := append([]typedmemory.EntityAlias{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].String() < result[right].String()
	})
	return slices.Compact(result)
}

func aliasesValidAndCanonical(values []typedmemory.EntityAlias) bool {
	for _, alias := range values {
		parsed, err := typedmemory.NewEntityAlias(alias.String())
		if err != nil || parsed != alias {
			return false
		}
	}
	return slices.Equal(values, canonicalAliases(values))
}

func canonicalContexts(
	values []typedmemory.BoundedContextRef,
) []typedmemory.BoundedContextRef {
	result := append([]typedmemory.BoundedContextRef{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].String() < result[right].String()
	})
	return slices.Compact(result)
}

func canonicalUnits(values []ResolutionUnit) []ResolutionUnit {
	result := append([]ResolutionUnit{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].key() < result[right].key()
	})
	return result
}

func validPersistedRef(ref typedmemory.PersistedRef) bool {
	kind, kindErr := typedmemory.NewRefKindRef(
		ref.RefKind().TypeEnv(),
		ref.RefKind().ID(),
	)
	id, idErr := typedmemory.NewReferenceID(ref.ReferenceID().String())
	canonical, canonicalErr := typedmemory.NewPersistedRef(kind, id)
	return kindErr == nil &&
		idErr == nil &&
		canonicalErr == nil &&
		canonical == ref
}

func exactOneLine(label string, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw || strings.ContainsAny(value, "\r\n\t") {
		return "", fmt.Errorf("%s must be exact and one line", label)
	}
	return value, nil
}
