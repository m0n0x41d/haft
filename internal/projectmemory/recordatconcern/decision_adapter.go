package recordatconcern

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	decisionRecordKindID     = "Haft.DecisionRecord"
	decisionRecordRefID      = "Haft.DecisionRecordRef"
	decisionChoiceTextKindID = "Haft.Text"
)

// DecisionChoiceSource is an immutable projection of one already-existing
// DecisionRecord ChoiceResult. It is descriptive source material, not a
// decision, recommendation, approval receipt, or authority grant.
type DecisionChoiceSource struct {
	recordRef     string
	recordVersion int
	recordDigest  typedmemory.SHA256Digest
	choiceJSON    []byte
	subject       string
	options       []string
	chosen        string
	problemRefs   []string
	portfolioRef  string
}

type DecisionChoiceSourceInput struct {
	RecordRef     string
	RecordVersion int
	RecordDigest  typedmemory.SHA256Digest
	ChoiceJSON    []byte
	Subject       string
	Options       []string
	Chosen        string
	ProblemRefs   []string
	PortfolioRef  string
}

func NewDecisionChoiceSource(
	input DecisionChoiceSourceInput,
) (DecisionChoiceSource, error) {
	recordRef := strings.TrimSpace(input.RecordRef)
	subject := strings.TrimSpace(input.Subject)
	chosen := strings.TrimSpace(input.Chosen)
	portfolioRef := strings.TrimSpace(input.PortfolioRef)
	if recordRef == "" || recordRef != input.RecordRef {
		return DecisionChoiceSource{}, fmt.Errorf(
			"decision choice source requires an exact DecisionRecord reference",
		)
	}
	if input.RecordVersion < 1 {
		return DecisionChoiceSource{}, fmt.Errorf(
			"decision choice source requires a positive DecisionRecord version",
		)
	}
	digest, err := typedmemory.NewSHA256Digest(input.RecordDigest.String())
	if err != nil || digest != input.RecordDigest {
		return DecisionChoiceSource{}, fmt.Errorf(
			"decision choice source requires an exact DecisionRecord digest",
		)
	}
	if len(input.ChoiceJSON) == 0 || !utf8.Valid(input.ChoiceJSON) {
		return DecisionChoiceSource{}, fmt.Errorf(
			"decision choice source requires canonical UTF-8 ChoiceResult bytes",
		)
	}
	if subject == "" || subject != input.Subject {
		return DecisionChoiceSource{}, fmt.Errorf(
			"decision choice source requires an exact deciding subject",
		)
	}
	options, err := normalizeDecisionChoiceLabels(
		"decision choice source options",
		input.Options,
		2,
	)
	if err != nil {
		return DecisionChoiceSource{}, err
	}
	if chosen == "" || chosen != input.Chosen ||
		!containsDecisionChoiceLabel(options, chosen) {
		return DecisionChoiceSource{}, fmt.Errorf(
			"decision choice source chosen option must be one exact member of the option set",
		)
	}
	problemRefs, err := normalizeDecisionChoiceLabels(
		"decision choice source problem references",
		input.ProblemRefs,
		0,
	)
	if err != nil {
		return DecisionChoiceSource{}, err
	}
	return DecisionChoiceSource{
		recordRef:     recordRef,
		recordVersion: input.RecordVersion,
		recordDigest:  digest,
		choiceJSON:    append([]byte(nil), input.ChoiceJSON...),
		subject:       subject,
		options:       options,
		chosen:        chosen,
		problemRefs:   problemRefs,
		portfolioRef:  portfolioRef,
	}, nil
}

func (source DecisionChoiceSource) RecordRef() string {
	return source.recordRef
}

func (source DecisionChoiceSource) RecordVersion() int {
	return source.recordVersion
}

func (source DecisionChoiceSource) RecordDigest() typedmemory.SHA256Digest {
	return source.recordDigest
}

func (source DecisionChoiceSource) ChoiceJSON() []byte {
	return append([]byte(nil), source.choiceJSON...)
}

func (source DecisionChoiceSource) Subject() string {
	return source.subject
}

func (source DecisionChoiceSource) Options() []string {
	return append([]string(nil), source.options...)
}

func (source DecisionChoiceSource) Chosen() string {
	return source.chosen
}

func (source DecisionChoiceSource) ProblemRefs() []string {
	return append([]string(nil), source.problemRefs...)
}

func (source DecisionChoiceSource) PortfolioRef() string {
	return source.portfolioRef
}

func (source DecisionChoiceSource) valid() bool {
	rebuilt, err := NewDecisionChoiceSource(DecisionChoiceSourceInput{
		RecordRef:     source.recordRef,
		RecordVersion: source.recordVersion,
		RecordDigest:  source.recordDigest,
		ChoiceJSON:    source.choiceJSON,
		Subject:       source.subject,
		Options:       source.options,
		Chosen:        source.chosen,
		ProblemRefs:   source.problemRefs,
		PortfolioRef:  source.portfolioRef,
	})
	return err == nil &&
		rebuilt.recordRef == source.recordRef &&
		rebuilt.recordVersion == source.recordVersion &&
		rebuilt.recordDigest == source.recordDigest &&
		bytes.Equal(rebuilt.choiceJSON, source.choiceJSON) &&
		rebuilt.subject == source.subject &&
		slices.Equal(rebuilt.options, source.options) &&
		rebuilt.chosen == source.chosen &&
		slices.Equal(rebuilt.problemRefs, source.problemRefs) &&
		rebuilt.portfolioRef == source.portfolioRef
}

// DecisionOptionBinding maps one exact ChoiceResult label to one already
// resolved ProjectRecord reference. The adapter verifies a total bijection
// between this label set and the stored option set.
type DecisionOptionBinding struct {
	label     string
	reference typedmemory.PersistedRef
}

func NewDecisionOptionBinding(
	label string,
	reference typedmemory.PersistedRef,
) (DecisionOptionBinding, error) {
	canonicalLabel := strings.TrimSpace(label)
	if canonicalLabel == "" || canonicalLabel != label {
		return DecisionOptionBinding{}, fmt.Errorf(
			"decision option binding requires an exact non-empty label",
		)
	}
	if err := requireExactPersistedRef(reference); err != nil {
		return DecisionOptionBinding{}, fmt.Errorf(
			"decision option %q reference: %w",
			label,
			err,
		)
	}
	if reference.RefKind().ID().String() != projectRecordRefID {
		return DecisionOptionBinding{}, fmt.Errorf(
			"decision option %q uses %s, want %s",
			label,
			reference.RefKind().ID(),
			projectRecordRefID,
		)
	}
	return DecisionOptionBinding{
		label:     canonicalLabel,
		reference: reference,
	}, nil
}

func (binding DecisionOptionBinding) Label() string {
	return binding.label
}

func (binding DecisionOptionBinding) Reference() typedmemory.PersistedRef {
	return binding.reference
}

type OptionalProjectRecordReferenceKind string

const (
	OptionalProjectRecordAbsent OptionalProjectRecordReferenceKind = "absent"
	OptionalProjectRecordExact  OptionalProjectRecordReferenceKind = "exact"
)

// OptionalProjectRecordReference is a closed optional exact reference. A zero
// value is invalid, so absence must be stated explicitly.
type OptionalProjectRecordReference struct {
	kind      OptionalProjectRecordReferenceKind
	sourceRef string
	reference typedmemory.PersistedRef
}

func NoProjectRecordReference() OptionalProjectRecordReference {
	return OptionalProjectRecordReference{kind: OptionalProjectRecordAbsent}
}

func NewExactProjectRecordReference(
	sourceRef string,
	reference typedmemory.PersistedRef,
) (OptionalProjectRecordReference, error) {
	canonicalSource := strings.TrimSpace(sourceRef)
	if canonicalSource == "" || canonicalSource != sourceRef {
		return OptionalProjectRecordReference{}, fmt.Errorf(
			"exact project-record mapping requires a source reference",
		)
	}
	if err := requireExactPersistedRef(reference); err != nil {
		return OptionalProjectRecordReference{}, err
	}
	if reference.RefKind().ID().String() != projectRecordRefID {
		return OptionalProjectRecordReference{}, fmt.Errorf(
			"project-record mapping uses %s, want %s",
			reference.RefKind().ID(),
			projectRecordRefID,
		)
	}
	return OptionalProjectRecordReference{
		kind:      OptionalProjectRecordExact,
		sourceRef: canonicalSource,
		reference: reference,
	}, nil
}

func (reference OptionalProjectRecordReference) Kind() OptionalProjectRecordReferenceKind {
	return reference.kind
}

func (reference OptionalProjectRecordReference) SourceRef() string {
	return reference.sourceRef
}

func (reference OptionalProjectRecordReference) Reference() (
	typedmemory.PersistedRef,
	bool,
) {
	return reference.reference, reference.kind == OptionalProjectRecordExact
}

func (reference OptionalProjectRecordReference) valid() bool {
	switch reference.kind {
	case OptionalProjectRecordAbsent:
		return reference.sourceRef == "" &&
			reference.reference.ReferenceID().String() == ""
	case OptionalProjectRecordExact:
		rebuilt, err := NewExactProjectRecordReference(
			reference.sourceRef,
			reference.reference,
		)
		return err == nil &&
			rebuilt.sourceRef == reference.sourceRef &&
			rebuilt.reference == reference.reference
	default:
		return false
	}
}

type DecisionProjectionDraftInput struct {
	ProjectID      projectidentity.ProjectID
	RecordEntity   typedmemory.EntityID
	RecordLocalRef typedmemory.BatchLocalRef
	RecordLabel    typedmemory.EntityLabel
	AssertionID    typedmemory.AssertionID
	ContextSlice   typedmemory.ContextSlice
	Source         DecisionChoiceSource
	Concern        ExactConcernBinding
	Problem        OptionalProjectRecordReference
	Portfolio      OptionalProjectRecordReference
	Options        []DecisionOptionBinding
	Comparison     OptionalProjectRecordReference
	Provenance     typedmemory.ProvenanceRef
}

type DecisionProjectionDraft struct {
	record     Draft
	source     DecisionChoiceSource
	concern    ExactConcernBinding
	problem    OptionalProjectRecordReference
	portfolio  OptionalProjectRecordReference
	options    []DecisionOptionBinding
	chosen     typedmemory.PersistedRef
	rejected   []typedmemory.PersistedRef
	comparison OptionalProjectRecordReference
}

func NewDecisionProjectionDraft(
	input DecisionProjectionDraftInput,
) (DecisionProjectionDraft, error) {
	if !input.Source.valid() {
		return DecisionProjectionDraft{}, fmt.Errorf(
			"decision projection requires an exact existing DecisionRecord source",
		)
	}
	emptyGraph, err := typedmemory.NewClaimGraphValue(nil, nil)
	if err != nil {
		return DecisionProjectionDraft{}, err
	}
	graph, err := NewExactClaimGraph(emptyGraph)
	if err != nil {
		return DecisionProjectionDraft{}, err
	}
	record, err := NewDraft(DraftInput{
		ProjectID:      input.ProjectID,
		RecordEntity:   input.RecordEntity,
		RecordLocalRef: input.RecordLocalRef,
		RecordLabel:    input.RecordLabel,
		AssertionID:    input.AssertionID,
		ContextSlice:   input.ContextSlice,
		ClaimGraph:     graph,
		Provenance:     input.Provenance,
	})
	if err != nil {
		return DecisionProjectionDraft{}, err
	}
	if input.Concern.context != input.ContextSlice.Context() {
		return DecisionProjectionDraft{}, fmt.Errorf(
			"decision concern and ContextSlice use different contexts",
		)
	}
	if !input.Problem.valid() ||
		!input.Portfolio.valid() ||
		!input.Comparison.valid() {
		return DecisionProjectionDraft{}, fmt.Errorf(
			"decision optional project-record references require explicit exact or absent variants",
		)
	}
	if err := validateDecisionSourceRecordMappings(
		input.Source,
		input.Problem,
		input.Portfolio,
	); err != nil {
		return DecisionProjectionDraft{}, err
	}
	options, chosen, rejected, err := normalizeDecisionOptionBindings(
		input.Source,
		input.Options,
	)
	if err != nil {
		return DecisionProjectionDraft{}, err
	}
	return DecisionProjectionDraft{
		record:     record,
		source:     input.Source,
		concern:    input.Concern,
		problem:    input.Problem,
		portfolio:  input.Portfolio,
		options:    options,
		chosen:     chosen,
		rejected:   rejected,
		comparison: input.Comparison,
	}, nil
}

func (draft DecisionProjectionDraft) Record() Draft {
	return draft.record
}

func (draft DecisionProjectionDraft) Source() DecisionChoiceSource {
	return draft.source
}

func (draft DecisionProjectionDraft) Options() []DecisionOptionBinding {
	return append([]DecisionOptionBinding(nil), draft.options...)
}

type resolvedDecisionChoiceMapping struct {
	fragment          typedmemory.TypedRelationDeclarationFragmentRef
	recordSlot        typedmemory.SlotKindID
	recordRefKind     typedmemory.RefKindRef
	concernSlot       typedmemory.SlotKindID
	concernRefKind    typedmemory.RefKindRef
	problemSlot       typedmemory.SlotKindID
	problemRefKind    typedmemory.RefKindRef
	portfolioSlot     typedmemory.SlotKindID
	portfolioRefKind  typedmemory.RefKindRef
	optionSlot        typedmemory.SlotKindID
	optionRefKind     typedmemory.RefKindRef
	chosenSlot        typedmemory.SlotKindID
	chosenRefKind     typedmemory.RefKindRef
	rejectedSlot      typedmemory.SlotKindID
	rejectedRefKind   typedmemory.RefKindRef
	comparisonSlot    typedmemory.SlotKindID
	comparisonRefKind typedmemory.RefKindRef
	claimGraphSlot    typedmemory.SlotKindID
	claimGraphKind    typedmemory.ValueKindRef
	claimGraphShape   typedmemory.ValueShapeRef
	claimGraphCodec   typedmemory.CodecRef
	codec             typedmemory.CodecImplementation
	textKind          typedmemory.ValueKindRef
}

// AdaptDecisionProjection maps one already-bound DecisionRecord into typed
// memory. It neither creates nor supersedes a DecisionRecord and grants no
// implementation authority.
func AdaptDecisionProjection(
	contract Contract,
	draft DecisionProjectionDraft,
	runtime RuntimeBasis,
) Result {
	if !contract.valid() {
		return underdeterminedFor(
			"mapping_contract",
			"repair:reload-decision-choice-contract",
		)
	}
	if !draft.source.valid() {
		return invalidResult(
			"decision_source_invalid",
			"the existing DecisionRecord projection source is incomplete",
		)
	}
	exactRuntime, runtimeReady := runtime.(ExactRuntimeBasis)
	if !runtimeReady {
		return missingRuntimeResult(runtime)
	}
	if exactRuntime.project != draft.record.projectID {
		return invalidResult(
			"runtime_project_mismatch",
			"the selected runtime and DecisionRecord projection belong to different projects",
		)
	}
	if result, accepted := requireRegisteredMapping(contract, exactRuntime); !accepted {
		return result
	}
	mapping, missing := resolveDecisionChoiceMapping(
		contract,
		exactRuntime.environment,
		exactRuntime.codecs,
		draft.record.contextSlice.Context(),
	)
	if len(missing) > 0 {
		return underdetermined{missing: missing}
	}
	concern, result, ready := requireExactConcern(
		draft.concern,
		draft.record.contextSlice.Context(),
		mapping.concernRefKind,
	)
	if !ready {
		return result
	}
	graph, err := decisionChoiceClaimGraph(draft.source, mapping.textKind)
	if err != nil {
		return invalidResult(
			"decision_choice_claim_graph_invalid",
			err.Error(),
		)
	}
	claimResult := buildClaimGraphCandidate(
		contract,
		graph,
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
	bindings, err := buildDecisionChoiceBindings(
		draft,
		concern,
		mapping,
		claim.candidate,
	)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "relation_binding_invalid"),
			err.Error(),
		)
	}
	record := draft.record
	record.claimGraph = ExactClaimGraph{graph: graph}
	return buildCandidateWithBindings(
		record,
		mapping.fragment,
		bindings,
		contract,
	)
}

func resolveDecisionChoiceMapping(
	contract Contract,
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	contextRef typedmemory.BoundedContextRef,
) (resolvedDecisionChoiceMapping, []MissingBasis) {
	fragment, missing := resolvePortfolioFragment(
		contract,
		environment,
		contextRef,
	)
	if len(missing) > 0 {
		return resolvedDecisionChoiceMapping{}, missing
	}
	recordSlot := mustSlotKindID(contract.definition.recordSlotID)
	concernSlot := mustSlotKindID(contract.definition.concernSlotID)
	problemSlot := mustSlotKindID(contract.definition.problemSlotID)
	portfolioSlot := mustSlotKindID(contract.definition.portfolioSlotID)
	optionSlot := mustSlotKindID(contract.definition.optionSlotID)
	chosenSlot := mustSlotKindID(contract.definition.chosenOptionSlotID)
	rejectedSlot := mustSlotKindID(contract.definition.rejectedOptionSlotID)
	comparisonSlot := mustSlotKindID(contract.definition.comparisonSlotID)
	claimSlot := mustSlotKindID(contract.definition.claimGraphSlotID)
	recordTarget, recordReady := exactReferenceSlot(
		fragment,
		recordSlot,
		decisionRecordKindID,
		decisionRecordRefID,
	)
	concernTarget, concernReady := exactReferenceSlot(
		fragment,
		concernSlot,
		entityKindID,
		entityRefID,
	)
	problemTarget, problemReady := optionalReferenceSlot(
		fragment,
		problemSlot,
		projectRecordKindID,
		projectRecordRefID,
	)
	portfolioTarget, portfolioReady := optionalReferenceSlot(
		fragment,
		portfolioSlot,
		projectRecordKindID,
		projectRecordRefID,
	)
	optionTarget, optionReady := unboundedReferenceSlot(
		fragment,
		optionSlot,
		projectRecordKindID,
		projectRecordRefID,
		2,
	)
	chosenTarget, chosenReady := exactReferenceSlot(
		fragment,
		chosenSlot,
		projectRecordKindID,
		projectRecordRefID,
	)
	rejectedTarget, rejectedReady := unboundedReferenceSlot(
		fragment,
		rejectedSlot,
		projectRecordKindID,
		projectRecordRefID,
		1,
	)
	comparisonTarget, comparisonReady := optionalReferenceSlot(
		fragment,
		comparisonSlot,
		projectRecordKindID,
		projectRecordRefID,
	)
	claimTarget, claimReady := exactValueSlot(
		fragment,
		claimSlot,
		claimGraphKindID,
	)
	if len(fragment.Slots()) != 9 ||
		!recordReady ||
		!concernReady ||
		!problemReady ||
		!portfolioReady ||
		!optionReady ||
		!chosenReady ||
		!rejectedReady ||
		!comparisonReady ||
		!claimReady {
		return resolvedDecisionChoiceMapping{}, []MissingBasis{
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
		return resolvedDecisionChoiceMapping{}, claimMissing
	}
	textID, err := typedmemory.NewKindID(decisionChoiceTextKindID)
	if err != nil {
		return resolvedDecisionChoiceMapping{}, []MissingBasis{
			mustMissingBasis(
				"decision_choice_text_kind",
				"repair:select-typeenv-with-haft-text",
			),
		}
	}
	textKind, err := typedmemory.NewValueKindRef(environment.Ref(), textID)
	if err != nil {
		return resolvedDecisionChoiceMapping{}, []MissingBasis{
			mustMissingBasis(
				"decision_choice_text_kind",
				"repair:select-typeenv-with-haft-text",
			),
		}
	}
	if _, found := environment.ValueBinding(textKind); !found {
		return resolvedDecisionChoiceMapping{}, []MissingBasis{
			mustMissingBasis(
				"decision_choice_text_binding",
				"repair:select-typeenv-with-haft-text-binding",
			),
		}
	}
	return resolvedDecisionChoiceMapping{
		fragment:          fragment.Ref(),
		recordSlot:        recordSlot,
		recordRefKind:     recordTarget.ReferenceKind(),
		concernSlot:       concernSlot,
		concernRefKind:    concernTarget.ReferenceKind(),
		problemSlot:       problemSlot,
		problemRefKind:    problemTarget.ReferenceKind(),
		portfolioSlot:     portfolioSlot,
		portfolioRefKind:  portfolioTarget.ReferenceKind(),
		optionSlot:        optionSlot,
		optionRefKind:     optionTarget.ReferenceKind(),
		chosenSlot:        chosenSlot,
		chosenRefKind:     chosenTarget.ReferenceKind(),
		rejectedSlot:      rejectedSlot,
		rejectedRefKind:   rejectedTarget.ReferenceKind(),
		comparisonSlot:    comparisonSlot,
		comparisonRefKind: comparisonTarget.ReferenceKind(),
		claimGraphSlot:    claimSlot,
		claimGraphKind:    claim.kind,
		claimGraphShape:   claim.shape,
		claimGraphCodec:   claim.codecRef,
		codec:             claim.codec,
		textKind:          textKind,
	}, nil
}

func buildDecisionChoiceBindings(
	draft DecisionProjectionDraft,
	concern ExactConcernBinding,
	mapping resolvedDecisionChoiceMapping,
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
	optionReferences := make([]typedmemory.PersistedRef, 0, len(draft.options))
	for _, option := range draft.options {
		optionReferences = append(optionReferences, option.reference)
	}
	optionBinding, err := persistedReferenceBinding(
		mapping.optionSlot,
		mapping.optionRefKind,
		optionReferences,
	)
	if err != nil {
		return nil, err
	}
	chosenBinding, err := singlePersistedReferenceBinding(
		mapping.chosenSlot,
		mapping.chosenRefKind,
		draft.chosen,
	)
	if err != nil {
		return nil, err
	}
	rejectedBinding, err := persistedReferenceBinding(
		mapping.rejectedSlot,
		mapping.rejectedRefKind,
		draft.rejected,
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
	bindings := []typedmemory.CandidateSlotBinding{
		recordBinding,
		concernBinding,
		optionBinding,
		chosenBinding,
		rejectedBinding,
		claimBinding,
	}
	bindings, err = appendOptionalProjectRecordBinding(
		bindings,
		draft.problem,
		mapping.problemSlot,
		mapping.problemRefKind,
	)
	if err != nil {
		return nil, err
	}
	bindings, err = appendOptionalProjectRecordBinding(
		bindings,
		draft.portfolio,
		mapping.portfolioSlot,
		mapping.portfolioRefKind,
	)
	if err != nil {
		return nil, err
	}
	return appendOptionalProjectRecordBinding(
		bindings,
		draft.comparison,
		mapping.comparisonSlot,
		mapping.comparisonRefKind,
	)
}

func appendOptionalProjectRecordBinding(
	bindings []typedmemory.CandidateSlotBinding,
	optional OptionalProjectRecordReference,
	slot typedmemory.SlotKindID,
	refKind typedmemory.RefKindRef,
) ([]typedmemory.CandidateSlotBinding, error) {
	reference, present := optional.Reference()
	if !present {
		return bindings, nil
	}
	binding, err := singlePersistedReferenceBinding(
		slot,
		refKind,
		reference,
	)
	if err != nil {
		return nil, err
	}
	return append(bindings, binding), nil
}

func optionalReferenceSlot(
	fragment typedmemory.TypedRelationDeclarationFragment,
	slotID typedmemory.SlotKindID,
	wantKind string,
	wantRefKind string,
) (typedmemory.ReferenceSlotTarget, bool) {
	slot, found := fragment.Slot(slotID)
	if !found || !optionalSingleCardinality(slot.Cardinality()) {
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

func optionalSingleCardinality(cardinality typedmemory.Cardinality) bool {
	if cardinality.Minimum() != 0 {
		return false
	}
	maximum, bounded := cardinality.Maximum().BoundedValue()
	return bounded && maximum == 1
}

func decisionChoiceClaimGraph(
	source DecisionChoiceSource,
	textKind typedmemory.ValueKindRef,
) (typedmemory.ClaimGraphValue, error) {
	digest := strings.TrimPrefix(source.recordDigest.String(), "sha256:")
	nodeID, err := typedmemory.NewClaimNodeID(
		"decision-choice:" + digest,
	)
	if err != nil {
		return nil, err
	}
	node, err := typedmemory.NewClaimNode(
		nodeID,
		textKind,
		typedmemory.NewTextValue(string(source.choiceJSON)),
	)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewClaimGraphValue(
		[]typedmemory.ClaimNode{node},
		nil,
	)
}

func validateDecisionSourceRecordMappings(
	source DecisionChoiceSource,
	problem OptionalProjectRecordReference,
	portfolio OptionalProjectRecordReference,
) error {
	if len(source.problemRefs) > 1 {
		return fmt.Errorf(
			"DecisionChoiceAtConcern v1 supports at most one problem record per relation; source %s names %d",
			source.recordRef,
			len(source.problemRefs),
		)
	}
	if len(source.problemRefs) == 0 {
		if problem.kind != OptionalProjectRecordAbsent {
			return fmt.Errorf(
				"decision source has no problem reference but an exact problem mapping was supplied",
			)
		}
		return validateDecisionPortfolioMapping(source, portfolio)
	}
	if problem.kind != OptionalProjectRecordExact ||
		problem.sourceRef != source.problemRefs[0] {
		return fmt.Errorf(
			"decision problem mapping must exactly match stored ChoiceResult problem_refs",
		)
	}
	return validateDecisionPortfolioMapping(source, portfolio)
}

func validateDecisionPortfolioMapping(
	source DecisionChoiceSource,
	portfolio OptionalProjectRecordReference,
) error {
	if source.portfolioRef == "" {
		if portfolio.kind != OptionalProjectRecordAbsent {
			return fmt.Errorf(
				"decision source has no portfolio reference but an exact portfolio mapping was supplied",
			)
		}
		return nil
	}
	if portfolio.kind != OptionalProjectRecordExact ||
		portfolio.sourceRef != source.portfolioRef {
		return fmt.Errorf(
			"decision portfolio mapping must exactly match stored ChoiceResult portfolio_ref",
		)
	}
	return nil
}

func normalizeDecisionOptionBindings(
	source DecisionChoiceSource,
	values []DecisionOptionBinding,
) ([]DecisionOptionBinding, typedmemory.PersistedRef, []typedmemory.PersistedRef, error) {
	if len(values) != len(source.options) {
		return nil, typedmemory.PersistedRef{}, nil, fmt.Errorf(
			"decision option mapping count = %d, want exact stored option count %d",
			len(values),
			len(source.options),
		)
	}
	byLabel := make(map[string]DecisionOptionBinding, len(values))
	byReference := make(map[string]string, len(values))
	for _, value := range values {
		rebuilt, err := NewDecisionOptionBinding(value.label, value.reference)
		if err != nil {
			return nil, typedmemory.PersistedRef{}, nil, err
		}
		if _, exists := byLabel[rebuilt.label]; exists {
			return nil, typedmemory.PersistedRef{}, nil, fmt.Errorf(
				"decision option mapping repeats label %q",
				rebuilt.label,
			)
		}
		referenceKey := persistedReferenceKey(rebuilt.reference)
		if previous, exists := byReference[referenceKey]; exists {
			return nil, typedmemory.PersistedRef{}, nil, fmt.Errorf(
				"decision options %q and %q map to one ProjectRecord",
				previous,
				rebuilt.label,
			)
		}
		byLabel[rebuilt.label] = rebuilt
		byReference[referenceKey] = rebuilt.label
	}
	ordered := make([]DecisionOptionBinding, 0, len(source.options))
	rejected := make([]typedmemory.PersistedRef, 0, len(source.options)-1)
	chosen := typedmemory.PersistedRef{}
	for _, label := range source.options {
		binding, present := byLabel[label]
		if !present {
			return nil, typedmemory.PersistedRef{}, nil, fmt.Errorf(
				"decision option mapping omits stored option %q",
				label,
			)
		}
		ordered = append(ordered, binding)
		if label == source.chosen {
			chosen = binding.reference
			continue
		}
		rejected = append(rejected, binding.reference)
	}
	if chosen.ReferenceID().String() == "" || len(rejected) < 1 {
		return nil, typedmemory.PersistedRef{}, nil, fmt.Errorf(
			"decision option partition requires exactly one chosen and at least one rejected option",
		)
	}
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].label < ordered[right].label
	})
	sort.Slice(rejected, func(left, right int) bool {
		return persistedReferenceKey(rejected[left]) <
			persistedReferenceKey(rejected[right])
	})
	return ordered, chosen, rejected, nil
}

func normalizeDecisionChoiceLabels(
	name string,
	values []string,
	minimum int,
) ([]string, error) {
	owned := make([]string, 0, len(values))
	for index, value := range values {
		canonical := strings.TrimSpace(value)
		if canonical == "" || canonical != value {
			return nil, fmt.Errorf(
				"%s at index %d is empty or noncanonical",
				name,
				index,
			)
		}
		owned = append(owned, canonical)
	}
	sort.Strings(owned)
	for index, value := range owned {
		if index > 0 && owned[index-1] == value {
			return nil, fmt.Errorf("%s repeats %q", name, value)
		}
	}
	if len(owned) < minimum {
		return nil, fmt.Errorf(
			"%s requires at least %d values",
			name,
			minimum,
		)
	}
	return owned, nil
}

func containsDecisionChoiceLabel(values []string, wanted string) bool {
	index := sort.SearchStrings(values, wanted)
	return index < len(values) && values[index] == wanted
}
