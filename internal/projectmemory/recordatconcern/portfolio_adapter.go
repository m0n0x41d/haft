package recordatconcern

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// SolutionPortfolioDraft is the source-shaped domain input for one
// Haft.SolutionPortfolioAtConcern candidate. Options are existing,
// independently addressable project records; their order has no semantic
// meaning and no option is selected by this adapter.
type SolutionPortfolioDraft struct {
	record  Draft
	options []typedmemory.PersistedRef
}

type SolutionPortfolioDraftInput struct {
	Record  DraftInput
	Options []typedmemory.PersistedRef
}

func NewSolutionPortfolioDraft(
	input SolutionPortfolioDraftInput,
) (SolutionPortfolioDraft, error) {
	record, err := NewDraft(input.Record)
	if err != nil {
		return SolutionPortfolioDraft{}, err
	}
	options, err := normalizePersistedReferenceSet(
		"solution portfolio options",
		input.Options,
		2,
	)
	if err != nil {
		return SolutionPortfolioDraft{}, err
	}
	return SolutionPortfolioDraft{
		record:  record,
		options: options,
	}, nil
}

func (draft SolutionPortfolioDraft) Record() Draft {
	return draft.record
}

func (draft SolutionPortfolioDraft) Options() []typedmemory.PersistedRef {
	return append([]typedmemory.PersistedRef(nil), draft.options...)
}

// PortfolioComparisonDraft carries an explicit comparison result. Compared
// options remain addressable and the non-dominated set is only a subset; this
// type has no winner, recommendation, or DecisionRecord field.
type PortfolioComparisonDraft struct {
	record       Draft
	portfolio    typedmemory.PersistedRef
	compared     []typedmemory.PersistedRef
	nonDominated []typedmemory.PersistedRef
}

type PortfolioComparisonDraftInput struct {
	Record              DraftInput
	Portfolio           typedmemory.PersistedRef
	ComparedOptions     []typedmemory.PersistedRef
	NonDominatedOptions []typedmemory.PersistedRef
}

func NewPortfolioComparisonDraft(
	input PortfolioComparisonDraftInput,
) (PortfolioComparisonDraft, error) {
	record, err := NewDraft(input.Record)
	if err != nil {
		return PortfolioComparisonDraft{}, err
	}
	if err := requireExactPersistedRef(input.Portfolio); err != nil {
		return PortfolioComparisonDraft{}, fmt.Errorf(
			"portfolio comparison portfolio reference: %w",
			err,
		)
	}
	compared, err := normalizePersistedReferenceSet(
		"portfolio comparison compared options",
		input.ComparedOptions,
		2,
	)
	if err != nil {
		return PortfolioComparisonDraft{}, err
	}
	nonDominated, err := normalizePersistedReferenceSet(
		"portfolio comparison non-dominated options",
		input.NonDominatedOptions,
		1,
	)
	if err != nil {
		return PortfolioComparisonDraft{}, err
	}
	return PortfolioComparisonDraft{
		record:       record,
		portfolio:    input.Portfolio,
		compared:     compared,
		nonDominated: nonDominated,
	}, nil
}

func (draft PortfolioComparisonDraft) Record() Draft {
	return draft.record
}

func (draft PortfolioComparisonDraft) Portfolio() typedmemory.PersistedRef {
	return draft.portfolio
}

func (draft PortfolioComparisonDraft) ComparedOptions() []typedmemory.PersistedRef {
	return append([]typedmemory.PersistedRef(nil), draft.compared...)
}

func (draft PortfolioComparisonDraft) NonDominatedOptions() []typedmemory.PersistedRef {
	return append([]typedmemory.PersistedRef(nil), draft.nonDominated...)
}

type resolvedSolutionPortfolioMapping struct {
	fragment        typedmemory.TypedRelationDeclarationFragmentRef
	recordSlot      typedmemory.SlotKindID
	recordRefKind   typedmemory.RefKindRef
	concernSlot     typedmemory.SlotKindID
	concernRefKind  typedmemory.RefKindRef
	claimGraphSlot  typedmemory.SlotKindID
	claimGraphKind  typedmemory.ValueKindRef
	claimGraphShape typedmemory.ValueShapeRef
	claimGraphCodec typedmemory.CodecRef
	codec           typedmemory.CodecImplementation
	optionSlot      typedmemory.SlotKindID
	optionRefKind   typedmemory.RefKindRef
}

type resolvedPortfolioComparisonMapping struct {
	fragment            typedmemory.TypedRelationDeclarationFragmentRef
	recordSlot          typedmemory.SlotKindID
	recordRefKind       typedmemory.RefKindRef
	portfolioSlot       typedmemory.SlotKindID
	portfolioRefKind    typedmemory.RefKindRef
	concernSlot         typedmemory.SlotKindID
	concernRefKind      typedmemory.RefKindRef
	comparedSlot        typedmemory.SlotKindID
	comparedRefKind     typedmemory.RefKindRef
	nonDominatedSlot    typedmemory.SlotKindID
	nonDominatedRefKind typedmemory.RefKindRef
	claimGraphSlot      typedmemory.SlotKindID
	claimGraphKind      typedmemory.ValueKindRef
	claimGraphShape     typedmemory.ValueShapeRef
	claimGraphCodec     typedmemory.CodecRef
	codec               typedmemory.CodecImplementation
}

// AdaptSolutionPortfolio is a pure candidate producer. It validates only the
// exact source-derived mapping and explicit inputs; it neither ranks options
// nor selects a preferred solution.
func AdaptSolutionPortfolio(
	contract Contract,
	draft SolutionPortfolioDraft,
	runtime RuntimeBasis,
	concern ConcernBinding,
) Result {
	if !contract.valid() {
		return underdeterminedFor(
			"mapping_contract",
			"repair:reload-solution-portfolio-contract",
		)
	}
	if !solutionPortfolioDraftPresent(draft) {
		return invalidResult(
			"solution_portfolio_draft_invalid",
			"the solution-portfolio draft is incomplete",
		)
	}
	claimGraph, graphReady := draft.record.claimGraph.(ExactClaimGraph)
	if !graphReady {
		return missingClaimGraphResult(contract, draft.record.claimGraph)
	}
	exactRuntime, runtimeReady := runtime.(ExactRuntimeBasis)
	if !runtimeReady {
		return missingRuntimeResult(runtime)
	}
	if exactRuntime.project != draft.record.projectID {
		return invalidResult(
			"runtime_project_mismatch",
			"the selected runtime and solution-portfolio draft belong to different projects",
		)
	}
	if result, accepted := requireRegisteredMapping(contract, exactRuntime); !accepted {
		return result
	}
	mapping, missing := resolveSolutionPortfolioMapping(
		contract,
		exactRuntime.environment,
		exactRuntime.codecs,
		draft.record.contextSlice.Context(),
	)
	if len(missing) > 0 {
		return underdetermined{missing: missing}
	}
	exactConcern, result, ready := requireExactConcern(
		concern,
		draft.record.contextSlice.Context(),
		mapping.concernRefKind,
	)
	if !ready {
		return result
	}
	claimResult := buildClaimGraphCandidate(
		contract,
		claimGraph.Value(),
		resolvedMapping{
			claimGraphKind:  mapping.claimGraphKind,
			claimGraphShape: mapping.claimGraphShape,
			claimGraphCodec: mapping.claimGraphCodec,
			codec:           mapping.codec,
		},
	)
	claim, claimReady := claimResult.(claimGraphCandidateReady)
	if !claimReady {
		rejected := claimResult.(claimGraphCandidateRejected)
		return rejected.result
	}
	bindings, err := buildSolutionPortfolioBindings(
		draft,
		exactConcern,
		mapping,
		claim.candidate,
	)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "relation_binding_invalid"),
			err.Error(),
		)
	}
	return buildCandidateWithBindings(
		draft.record,
		mapping.fragment,
		bindings,
		contract,
	)
}

// AdaptPortfolioComparison preserves the explicit compared and non-dominated
// sets. The selected TypeEnv validates the source-owned subset constraint;
// this adapter never turns the frontier into a winner.
func AdaptPortfolioComparison(
	contract Contract,
	draft PortfolioComparisonDraft,
	runtime RuntimeBasis,
	concern ConcernBinding,
) Result {
	if !contract.valid() {
		return underdeterminedFor(
			"mapping_contract",
			"repair:reload-portfolio-comparison-contract",
		)
	}
	if !portfolioComparisonDraftPresent(draft) {
		return invalidResult(
			"portfolio_comparison_draft_invalid",
			"the portfolio-comparison draft is incomplete",
		)
	}
	claimGraph, graphReady := draft.record.claimGraph.(ExactClaimGraph)
	if !graphReady {
		return missingClaimGraphResult(contract, draft.record.claimGraph)
	}
	exactRuntime, runtimeReady := runtime.(ExactRuntimeBasis)
	if !runtimeReady {
		return missingRuntimeResult(runtime)
	}
	if exactRuntime.project != draft.record.projectID {
		return invalidResult(
			"runtime_project_mismatch",
			"the selected runtime and portfolio-comparison draft belong to different projects",
		)
	}
	if result, accepted := requireRegisteredMapping(contract, exactRuntime); !accepted {
		return result
	}
	mapping, missing := resolvePortfolioComparisonMapping(
		contract,
		exactRuntime.environment,
		exactRuntime.codecs,
		draft.record.contextSlice.Context(),
	)
	if len(missing) > 0 {
		return underdetermined{missing: missing}
	}
	exactConcern, result, ready := requireExactConcern(
		concern,
		draft.record.contextSlice.Context(),
		mapping.concernRefKind,
	)
	if !ready {
		return result
	}
	claimResult := buildClaimGraphCandidate(
		contract,
		claimGraph.Value(),
		resolvedMapping{
			claimGraphKind:  mapping.claimGraphKind,
			claimGraphShape: mapping.claimGraphShape,
			claimGraphCodec: mapping.claimGraphCodec,
			codec:           mapping.codec,
		},
	)
	claim, claimReady := claimResult.(claimGraphCandidateReady)
	if !claimReady {
		rejected := claimResult.(claimGraphCandidateRejected)
		return rejected.result
	}
	bindings, err := buildPortfolioComparisonBindings(
		draft,
		exactConcern,
		mapping,
		claim.candidate,
	)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "relation_binding_invalid"),
			err.Error(),
		)
	}
	return buildCandidateWithBindings(
		draft.record,
		mapping.fragment,
		bindings,
		contract,
	)
}

func missingClaimGraphResult(
	contract Contract,
	basis ClaimGraphBasis,
) Result {
	missing, ok := basis.(MissingClaimGraph)
	if !ok {
		return underdeterminedFor(
			"claim_graph",
			contract.definition.claimGraphRepair,
		)
	}
	return underdetermined{missing: missing.MissingBasis()}
}

func missingRuntimeResult(runtime RuntimeBasis) Result {
	missing, ok := runtime.(MissingRuntimeBasis)
	if !ok {
		return underdeterminedFor(
			"selected_type_environment",
			"repair:resolve-project-typeenv-head",
		)
	}
	return underdetermined{missing: missing.MissingBasis()}
}

func requireExactConcern(
	concern ConcernBinding,
	context typedmemory.BoundedContextRef,
	refKind typedmemory.RefKindRef,
) (ExactConcernBinding, Result, bool) {
	exact, ready := concern.(ExactConcernBinding)
	if !ready {
		unsettled, ok := concern.(UnsettledConcernBinding)
		if !ok {
			return ExactConcernBinding{}, underdeterminedFor(
				"entity_of_concern_resolution",
				"repair:resolve-entity-of-concern",
			), false
		}
		return ExactConcernBinding{}, underdetermined{
			missing: unsettled.MissingBasis(),
		}, false
	}
	if exact.context != context {
		return ExactConcernBinding{}, invalidResult(
			"concern_context_mismatch",
			"the exact EntityOfConcern resolution and record ContextSlice use different bounded contexts",
		), false
	}
	if exact.reference.RefKind() != refKind {
		return ExactConcernBinding{}, invalidResult(
			"concern_reference_kind_mismatch",
			"the exact EntityOfConcern reference does not use the U.EntityRef required by the selected relation fragment",
		), false
	}
	if exact.reference.ReferenceID().String() != exact.entity.String() {
		return ExactConcernBinding{}, invalidResult(
			"concern_reference_identity_mismatch",
			"the exact EntityOfConcern reference and stable EntityID do not name one identity",
		), false
	}
	return exact, nil, true
}

func resolveSolutionPortfolioMapping(
	contract Contract,
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	context typedmemory.BoundedContextRef,
) (resolvedSolutionPortfolioMapping, []MissingBasis) {
	fragment, missing := resolvePortfolioFragment(contract, environment, context)
	if len(missing) > 0 {
		return resolvedSolutionPortfolioMapping{}, missing
	}
	recordSlot := mustSlotKindID(contract.definition.recordSlotID)
	concernSlot := mustSlotKindID(contract.definition.concernSlotID)
	claimSlot := mustSlotKindID(contract.definition.claimGraphSlotID)
	optionSlot := mustSlotKindID(contract.definition.optionSlotID)
	recordTarget, recordReady := exactReferenceSlot(
		fragment,
		recordSlot,
		projectRecordKindID,
		projectRecordRefID,
	)
	concernTarget, concernReady := exactReferenceSlot(
		fragment,
		concernSlot,
		entityKindID,
		entityRefID,
	)
	claimTarget, claimReady := exactValueSlot(
		fragment,
		claimSlot,
		claimGraphKindID,
	)
	optionTarget, optionReady := unboundedReferenceSlot(
		fragment,
		optionSlot,
		projectRecordKindID,
		projectRecordRefID,
		2,
	)
	if len(fragment.Slots()) != 4 ||
		!recordReady ||
		!concernReady ||
		!claimReady ||
		!optionReady {
		return resolvedSolutionPortfolioMapping{}, []MissingBasis{
			mustMissingBasis(
				diagnosticCode(contract, "mapping_slots"),
				contract.definition.mappingRepair,
			),
		}
	}
	claim, claimMissing := resolveClaimGraphMapping(
		contract,
		environment,
		codecs,
		claimTarget,
	)
	if len(claimMissing) > 0 {
		return resolvedSolutionPortfolioMapping{}, claimMissing
	}
	return resolvedSolutionPortfolioMapping{
		fragment:        fragment.Ref(),
		recordSlot:      recordSlot,
		recordRefKind:   recordTarget.ReferenceKind(),
		concernSlot:     concernSlot,
		concernRefKind:  concernTarget.ReferenceKind(),
		claimGraphSlot:  claimSlot,
		claimGraphKind:  claim.kind,
		claimGraphShape: claim.shape,
		claimGraphCodec: claim.codecRef,
		codec:           claim.codec,
		optionSlot:      optionSlot,
		optionRefKind:   optionTarget.ReferenceKind(),
	}, nil
}

func resolvePortfolioComparisonMapping(
	contract Contract,
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	context typedmemory.BoundedContextRef,
) (resolvedPortfolioComparisonMapping, []MissingBasis) {
	fragment, missing := resolvePortfolioFragment(contract, environment, context)
	if len(missing) > 0 {
		return resolvedPortfolioComparisonMapping{}, missing
	}
	recordSlot := mustSlotKindID(contract.definition.recordSlotID)
	portfolioSlot := mustSlotKindID(contract.definition.portfolioSlotID)
	concernSlot := mustSlotKindID(contract.definition.concernSlotID)
	comparedSlot := mustSlotKindID(contract.definition.comparedOptionSlotID)
	nonDominatedSlot := mustSlotKindID(contract.definition.nonDominatedSlotID)
	claimSlot := mustSlotKindID(contract.definition.claimGraphSlotID)
	recordTarget, recordReady := exactReferenceSlot(
		fragment,
		recordSlot,
		projectRecordKindID,
		projectRecordRefID,
	)
	portfolioTarget, portfolioReady := exactReferenceSlot(
		fragment,
		portfolioSlot,
		projectRecordKindID,
		projectRecordRefID,
	)
	concernTarget, concernReady := exactReferenceSlot(
		fragment,
		concernSlot,
		entityKindID,
		entityRefID,
	)
	comparedTarget, comparedReady := unboundedReferenceSlot(
		fragment,
		comparedSlot,
		projectRecordKindID,
		projectRecordRefID,
		2,
	)
	nonDominatedTarget, nonDominatedReady := unboundedReferenceSlot(
		fragment,
		nonDominatedSlot,
		projectRecordKindID,
		projectRecordRefID,
		1,
	)
	claimTarget, claimReady := exactValueSlot(
		fragment,
		claimSlot,
		claimGraphKindID,
	)
	if len(fragment.Slots()) != 6 ||
		!recordReady ||
		!portfolioReady ||
		!concernReady ||
		!comparedReady ||
		!nonDominatedReady ||
		!claimReady {
		return resolvedPortfolioComparisonMapping{}, []MissingBasis{
			mustMissingBasis(
				diagnosticCode(contract, "mapping_slots"),
				contract.definition.mappingRepair,
			),
		}
	}
	claim, claimMissing := resolveClaimGraphMapping(
		contract,
		environment,
		codecs,
		claimTarget,
	)
	if len(claimMissing) > 0 {
		return resolvedPortfolioComparisonMapping{}, claimMissing
	}
	return resolvedPortfolioComparisonMapping{
		fragment:            fragment.Ref(),
		recordSlot:          recordSlot,
		recordRefKind:       recordTarget.ReferenceKind(),
		portfolioSlot:       portfolioSlot,
		portfolioRefKind:    portfolioTarget.ReferenceKind(),
		concernSlot:         concernSlot,
		concernRefKind:      concernTarget.ReferenceKind(),
		comparedSlot:        comparedSlot,
		comparedRefKind:     comparedTarget.ReferenceKind(),
		nonDominatedSlot:    nonDominatedSlot,
		nonDominatedRefKind: nonDominatedTarget.ReferenceKind(),
		claimGraphSlot:      claimSlot,
		claimGraphKind:      claim.kind,
		claimGraphShape:     claim.shape,
		claimGraphCodec:     claim.codecRef,
		codec:               claim.codec,
	}, nil
}

func resolvePortfolioFragment(
	contract Contract,
	environment typedmemory.TypeEnv,
	context typedmemory.BoundedContextRef,
) (typedmemory.TypedRelationDeclarationFragment, []MissingBasis) {
	fragmentID := mustSignatureID(contract.definition.signatureID)
	fragmentRef, err := typedmemory.NewTypedRelationDeclarationFragmentRef(
		environment.Ref(),
		fragmentID,
	)
	if err != nil {
		return typedmemory.TypedRelationDeclarationFragment{}, []MissingBasis{
			mustMissingBasis(
				diagnosticCode(contract, "typed_relation_declaration_fragment"),
				contract.definition.mappingRepair,
			),
		}
	}
	fragment, found := environment.TypedRelationDeclarationFragment(fragmentRef)
	if !found || !fragmentAllowsContext(fragment, context) {
		return typedmemory.TypedRelationDeclarationFragment{}, []MissingBasis{
			mustMissingBasis(
				diagnosticCode(contract, "typed_relation_declaration_fragment"),
				"repair:select-typeenv-with-"+contract.definition.relationDiagnosticName,
			),
		}
	}
	return fragment, nil
}

type resolvedClaimGraphMapping struct {
	kind     typedmemory.ValueKindRef
	shape    typedmemory.ValueShapeRef
	codecRef typedmemory.CodecRef
	codec    typedmemory.CodecImplementation
}

func resolveClaimGraphMapping(
	contract Contract,
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	target typedmemory.ValueSlotTarget,
) (resolvedClaimGraphMapping, []MissingBasis) {
	binding, found := environment.ValueBinding(target.ValueKind())
	if !found {
		return resolvedClaimGraphMapping{}, []MissingBasis{
			mustMissingBasis(
				"claim_graph_value_binding",
				"repair:select-typeenv-with-claim-graph-binding",
			),
		}
	}
	shape, found := environment.ValueShape(binding.ValueShape())
	if !found || shape.Shape().Kind() != typedmemory.ValueShapeClaimGraph {
		return resolvedClaimGraphMapping{}, []MissingBasis{
			mustMissingBasis(
				"claim_graph_shape",
				"repair:refresh-selected-claim-graph-shape",
			),
		}
	}
	codec, found := codecs.Resolve(binding.Codec())
	if !found {
		return resolvedClaimGraphMapping{}, []MissingBasis{
			mustMissingBasis(
				"claim_graph_codec",
				"repair:resolve-selected-claim-graph-codec",
			),
		}
	}
	return resolvedClaimGraphMapping{
		kind:     target.ValueKind(),
		shape:    binding.ValueShape(),
		codecRef: binding.Codec(),
		codec:    codec,
	}, nil
}

func buildSolutionPortfolioBindings(
	draft SolutionPortfolioDraft,
	concern ExactConcernBinding,
	mapping resolvedSolutionPortfolioMapping,
	claim typedmemory.TypedValueCandidate,
) ([]typedmemory.CandidateSlotBinding, error) {
	recordReference, err := typedmemory.NewLocalRef(
		mapping.recordRefKind,
		draft.record.recordLocalRef,
	)
	if err != nil {
		return nil, err
	}
	recordBinding, err := singleReferenceBinding(
		mapping.recordSlot,
		recordReference,
	)
	if err != nil {
		return nil, err
	}
	concernBinding, err := singleReferenceBinding(
		mapping.concernSlot,
		concern.reference,
	)
	if err != nil {
		return nil, err
	}
	claimBinding, err := singleValueBinding(
		mapping.claimGraphSlot,
		claim,
	)
	if err != nil {
		return nil, err
	}
	optionBinding, err := persistedReferenceBinding(
		mapping.optionSlot,
		mapping.optionRefKind,
		draft.options,
	)
	if err != nil {
		return nil, err
	}
	return []typedmemory.CandidateSlotBinding{
		recordBinding,
		concernBinding,
		claimBinding,
		optionBinding,
	}, nil
}

func buildPortfolioComparisonBindings(
	draft PortfolioComparisonDraft,
	concern ExactConcernBinding,
	mapping resolvedPortfolioComparisonMapping,
	claim typedmemory.TypedValueCandidate,
) ([]typedmemory.CandidateSlotBinding, error) {
	recordReference, err := typedmemory.NewLocalRef(
		mapping.recordRefKind,
		draft.record.recordLocalRef,
	)
	if err != nil {
		return nil, err
	}
	recordBinding, err := singleReferenceBinding(
		mapping.recordSlot,
		recordReference,
	)
	if err != nil {
		return nil, err
	}
	portfolioBinding, err := singlePersistedReferenceBinding(
		mapping.portfolioSlot,
		mapping.portfolioRefKind,
		draft.portfolio,
	)
	if err != nil {
		return nil, err
	}
	concernBinding, err := singleReferenceBinding(
		mapping.concernSlot,
		concern.reference,
	)
	if err != nil {
		return nil, err
	}
	comparedBinding, err := persistedReferenceBinding(
		mapping.comparedSlot,
		mapping.comparedRefKind,
		draft.compared,
	)
	if err != nil {
		return nil, err
	}
	nonDominatedBinding, err := persistedReferenceBinding(
		mapping.nonDominatedSlot,
		mapping.nonDominatedRefKind,
		draft.nonDominated,
	)
	if err != nil {
		return nil, err
	}
	claimBinding, err := singleValueBinding(
		mapping.claimGraphSlot,
		claim,
	)
	if err != nil {
		return nil, err
	}
	return []typedmemory.CandidateSlotBinding{
		recordBinding,
		portfolioBinding,
		concernBinding,
		comparedBinding,
		nonDominatedBinding,
		claimBinding,
	}, nil
}

func singleReferenceBinding(
	slot typedmemory.SlotKindID,
	reference typedmemory.StrongRef,
) (typedmemory.CandidateSlotBinding, error) {
	filler, err := typedmemory.NewByReferenceCandidate(reference)
	if err != nil {
		return typedmemory.CandidateSlotBinding{}, err
	}
	return typedmemory.NewCandidateSlotBinding(
		slot,
		[]typedmemory.CandidateSlotFiller{filler},
	)
}

func singlePersistedReferenceBinding(
	slot typedmemory.SlotKindID,
	refKind typedmemory.RefKindRef,
	reference typedmemory.PersistedRef,
) (typedmemory.CandidateSlotBinding, error) {
	if reference.RefKind() != refKind {
		return typedmemory.CandidateSlotBinding{}, fmt.Errorf(
			"reference for slot %s uses %s, want %s",
			slot.String(),
			reference.RefKind().String(),
			refKind.String(),
		)
	}
	return singleReferenceBinding(slot, reference)
}

func persistedReferenceBinding(
	slot typedmemory.SlotKindID,
	refKind typedmemory.RefKindRef,
	references []typedmemory.PersistedRef,
) (typedmemory.CandidateSlotBinding, error) {
	fillers := make(
		[]typedmemory.CandidateSlotFiller,
		0,
		len(references),
	)
	for _, reference := range references {
		if reference.RefKind() != refKind {
			return typedmemory.CandidateSlotBinding{}, fmt.Errorf(
				"reference for slot %s uses %s, want %s",
				slot.String(),
				reference.RefKind().String(),
				refKind.String(),
			)
		}
		filler, err := typedmemory.NewByReferenceCandidate(reference)
		if err != nil {
			return typedmemory.CandidateSlotBinding{}, err
		}
		fillers = append(fillers, filler)
	}
	return typedmemory.NewCandidateSlotBinding(slot, fillers)
}

func singleValueBinding(
	slot typedmemory.SlotKindID,
	value typedmemory.TypedValueCandidate,
) (typedmemory.CandidateSlotBinding, error) {
	filler, err := typedmemory.NewByValueCandidate(value)
	if err != nil {
		return typedmemory.CandidateSlotBinding{}, err
	}
	return typedmemory.NewCandidateSlotBinding(
		slot,
		[]typedmemory.CandidateSlotFiller{filler},
	)
}

func unboundedReferenceSlot(
	fragment typedmemory.TypedRelationDeclarationFragment,
	slotID typedmemory.SlotKindID,
	wantKind string,
	wantRefKind string,
	minimum uint64,
) (typedmemory.ReferenceSlotTarget, bool) {
	slot, found := fragment.Slot(slotID)
	if !found || !unboundedCardinalityAt(slot.Cardinality(), minimum) {
		return typedmemory.ReferenceSlotTarget{}, false
	}
	target, ok := slot.Target().(typedmemory.ReferenceSlotTarget)
	if !ok ||
		target.ValueKind().ID().String() != wantKind ||
		target.ReferenceKind().ID().String() != wantRefKind {
		return typedmemory.ReferenceSlotTarget{}, false
	}
	return target, true
}

func unboundedCardinalityAt(
	cardinality typedmemory.Cardinality,
	minimum uint64,
) bool {
	if cardinality.Minimum() != minimum {
		return false
	}
	_, bounded := cardinality.Maximum().BoundedValue()
	return !bounded
}

func normalizePersistedReferenceSet(
	name string,
	values []typedmemory.PersistedRef,
	minimum int,
) ([]typedmemory.PersistedRef, error) {
	if len(values) < minimum {
		return nil, fmt.Errorf(
			"%s requires at least %d references",
			name,
			minimum,
		)
	}
	owned := append([]typedmemory.PersistedRef(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		return persistedReferenceKey(owned[left]) <
			persistedReferenceKey(owned[right])
	})
	for index, value := range owned {
		if err := requireExactPersistedRef(value); err != nil {
			return nil, fmt.Errorf("%s at index %d: %w", name, index, err)
		}
		if index > 0 &&
			persistedReferenceKey(owned[index-1]) ==
				persistedReferenceKey(value) {
			return nil, fmt.Errorf(
				"%s repeats reference %s",
				name,
				value.ReferenceID().String(),
			)
		}
	}
	return owned, nil
}

func persistedReferenceKey(reference typedmemory.PersistedRef) string {
	return reference.RefKind().String() + "\x00" + reference.ReferenceID().String()
}

func solutionPortfolioDraftPresent(draft SolutionPortfolioDraft) bool {
	return draft.record.projectID.String() != "" &&
		len(draft.options) >= 2
}

func portfolioComparisonDraftPresent(draft PortfolioComparisonDraft) bool {
	return draft.record.projectID.String() != "" &&
		draft.portfolio.ReferenceID().String() != "" &&
		len(draft.compared) >= 2 &&
		len(draft.nonDominated) >= 1
}
