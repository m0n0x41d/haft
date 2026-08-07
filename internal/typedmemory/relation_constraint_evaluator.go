package typedmemory

import (
	"fmt"
	"sort"
)

const (
	DiagnosticReferenceEntityDuplicate   DiagnosticCode = "reference_entity_duplicate"
	DiagnosticReferenceSubsetMismatch    DiagnosticCode = "reference_subset_mismatch"
	DiagnosticReferencePartitionMismatch DiagnosticCode = "reference_partition_mismatch"
)

// RelationConstraintSlotView is a transient, non-canonical evaluation view.
// It is not another representation of RelationInstance: final relations and
// prospective validation inputs are both lowered into this view immediately
// before pure constraint evaluation.
type RelationConstraintSlotView interface {
	Slot() SlotKindID
	FillerCount() uint64
	relationConstraintSlotViewVariant()
	validRelationConstraintSlotView() bool
}

// RelationConstraintValueSlotView retains only the number of already-checked
// ByValue fillers. Value shape and codec verification remain separate duties.
type RelationConstraintValueSlotView struct {
	slot  SlotKindID
	count uint64
}

func NewRelationConstraintValueSlotView(
	slot SlotKindID,
	count uint64,
) (RelationConstraintValueSlotView, error) {
	if !slot.valid() {
		return RelationConstraintValueSlotView{}, fmt.Errorf("relation-constraint value slot is required")
	}
	if count == 0 {
		return RelationConstraintValueSlotView{}, fmt.Errorf("relation-constraint value slot requires at least one filler")
	}
	return RelationConstraintValueSlotView{slot: slot, count: count}, nil
}

func (view RelationConstraintValueSlotView) Slot() SlotKindID { return view.slot }

func (view RelationConstraintValueSlotView) FillerCount() uint64 { return view.count }

func (RelationConstraintValueSlotView) relationConstraintSlotViewVariant() {}

func (view RelationConstraintValueSlotView) validRelationConstraintSlotView() bool {
	return view.slot.valid() && view.count > 0
}

type relationConstraintReferenceStateKind uint8

const (
	relationConstraintReferenceResolved relationConstraintReferenceStateKind = iota + 1
	relationConstraintReferenceUnresolved
	relationConstraintReferenceBridgeMissing
	relationConstraintReferenceResolutionMissing
)

type relationConstraintReferenceState struct {
	kind      relationConstraintReferenceStateKind
	entity    EntityID
	reference string
	refKind   RefKindRef
	repair    RepairPointer
}

func (state relationConstraintReferenceState) valid() bool {
	switch state.kind {
	case relationConstraintReferenceResolved:
		return state.entity.valid() && state.reference != "" && state.refKind.valid() && !state.repair.valid()
	case relationConstraintReferenceUnresolved,
		relationConstraintReferenceBridgeMissing:
		return !state.entity.valid() && state.reference != "" && state.refKind.valid() && state.repair.valid()
	case relationConstraintReferenceResolutionMissing:
		return !state.entity.valid() && state.reference != "" && !state.refKind.valid() && state.repair.valid()
	default:
		return false
	}
}

func (state relationConstraintReferenceState) sortKey() string {
	return fmt.Sprintf(
		"%02d:%s:%s:%s:%s",
		state.kind,
		state.entity.String(),
		state.reference,
		state.refKind.String(),
		state.repair.String(),
	)
}

// RelationConstraintReferenceSlotView retains an exact candidate filler count
// plus the available closed StrongReferenceResolution results. A count larger
// than the supplied results represents missing resolution results and remains
// Underdetermined; it is never treated as an empty set.
type RelationConstraintReferenceSlotView struct {
	slot       SlotKindID
	count      uint64
	references []relationConstraintReferenceState
}

func NewProspectiveRelationConstraintReferenceSlotView(
	slot SlotKindID,
	fillerCount uint64,
	resolutions []StrongReferenceResolution,
) (RelationConstraintReferenceSlotView, error) {
	if !slot.valid() {
		return RelationConstraintReferenceSlotView{}, fmt.Errorf("relation-constraint reference slot is required")
	}
	if fillerCount == 0 {
		return RelationConstraintReferenceSlotView{}, fmt.Errorf("relation-constraint reference slot requires at least one filler")
	}
	if uint64(len(resolutions)) > fillerCount {
		return RelationConstraintReferenceSlotView{}, fmt.Errorf("relation-constraint reference results exceed the exact filler count")
	}

	states := make([]relationConstraintReferenceState, 0, fillerCount)
	for index, resolution := range resolutions {
		state, err := relationConstraintStateFromResolution(resolution)
		if err != nil {
			return RelationConstraintReferenceSlotView{}, fmt.Errorf("relation-constraint reference result %d: %w", index, err)
		}
		states = append(states, state)
	}
	for index := uint64(len(states)); index < fillerCount; index++ {
		repair, _ := NewRepairPointer(fmt.Sprintf("resolve-reference-at:slot=%s,filler=%d", slot.String(), index))
		states = append(states, relationConstraintReferenceState{
			kind:      relationConstraintReferenceResolutionMissing,
			reference: fmt.Sprintf("missing:%s:%d", slot.String(), index),
			repair:    repair,
		})
	}
	sortRelationConstraintReferenceStates(states)
	return RelationConstraintReferenceSlotView{
		slot:       slot,
		count:      fillerCount,
		references: states,
	}, nil
}

func relationConstraintStateFromResolution(
	resolution StrongReferenceResolution,
) (relationConstraintReferenceState, error) {
	switch value := resolution.(type) {
	case ResolvedStrongReference:
		if !validStrongRef(value.Reference()) ||
			!value.Entity().valid() ||
			!value.Context().valid() ||
			!value.Basis().valid() {
			return relationConstraintReferenceState{}, fmt.Errorf("resolved reference result is invalid")
		}
		return relationConstraintReferenceState{
			kind:      relationConstraintReferenceResolved,
			entity:    value.Entity(),
			reference: value.Reference().ReferenceKey(),
			refKind:   value.Reference().RefKind(),
		}, nil
	case UnresolvedStrongReference:
		if !validStrongRef(value.Reference()) ||
			!value.Context().valid() ||
			!value.Repair().valid() {
			return relationConstraintReferenceState{}, fmt.Errorf("unresolved reference result is invalid")
		}
		return relationConstraintReferenceState{
			kind:      relationConstraintReferenceUnresolved,
			reference: value.Reference().ReferenceKey(),
			refKind:   value.Reference().RefKind(),
			repair:    value.Repair(),
		}, nil
	case MissingContextBridgeResolution:
		valid := validStrongRef(value.Reference()) &&
			value.Context().valid() &&
			value.SourceContext().valid() &&
			value.SourceContext() != value.Context() &&
			value.SourceKind().valid() &&
			value.TargetKind().valid() &&
			value.Repair().valid()
		if !valid {
			return relationConstraintReferenceState{}, fmt.Errorf("missing context-bridge result is invalid")
		}
		return relationConstraintReferenceState{
			kind:      relationConstraintReferenceBridgeMissing,
			reference: value.Reference().ReferenceKey(),
			refKind:   value.Reference().RefKind(),
			repair:    value.Repair(),
		}, nil
	default:
		return relationConstraintReferenceState{}, fmt.Errorf("unknown StrongReferenceResolution variant %T", resolution)
	}
}

func (view RelationConstraintReferenceSlotView) Slot() SlotKindID { return view.slot }

func (view RelationConstraintReferenceSlotView) FillerCount() uint64 { return view.count }

func (RelationConstraintReferenceSlotView) relationConstraintSlotViewVariant() {}

func (view RelationConstraintReferenceSlotView) validRelationConstraintSlotView() bool {
	if !view.slot.valid() || view.count == 0 || uint64(len(view.references)) != view.count {
		return false
	}
	for index, state := range view.references {
		if !state.valid() {
			return false
		}
		if index > 0 && state.sortKey() < view.references[index-1].sortKey() {
			return false
		}
	}
	return true
}

func sortRelationConstraintReferenceStates(states []relationConstraintReferenceState) {
	sort.SliceStable(states, func(left, right int) bool {
		return states[left].sortKey() < states[right].sortKey()
	})
}

// RelationConstraintEvaluationView is an ephemeral exact-coordinate view. It
// deliberately has no canonical bytes or digest and cannot be persisted as a
// competing relation carrier.
type RelationConstraintEvaluationView struct {
	signature RelationSignatureRef
	slots     []RelationConstraintSlotView
}

func (RelationConstraintEvaluationView) RelationDeclarationPosture() RelationDeclarationPosture {
	return RelationDeclarationTypedFragment
}

func NewProspectiveRelationConstraintEvaluationView(
	signature RelationSignatureRef,
	slots []RelationConstraintSlotView,
) (RelationConstraintEvaluationView, error) {
	if !signature.valid() {
		return RelationConstraintEvaluationView{}, fmt.Errorf(
			"relation-constraint view requires an exact typed relation declaration fragment",
		)
	}
	normalized, err := normalizeRelationConstraintSlotViews(slots)
	if err != nil {
		return RelationConstraintEvaluationView{}, err
	}
	return RelationConstraintEvaluationView{signature: signature, slots: normalized}, nil
}

func NewRelationConstraintEvaluationViewFromInstance(
	relation RelationInstance,
) (RelationConstraintEvaluationView, error) {
	if !relation.valid() {
		return RelationConstraintEvaluationView{}, fmt.Errorf("relation-constraint view requires a valid final RelationInstance")
	}
	return newRelationConstraintEvaluationViewFromBindings(
		relation.Signature(),
		relation.Bindings(),
	)
}

// NewRelationConstraintEvaluationViewFromRelationalAssertion lowers a strong
// v3 assertion into the same transient structural constraint view. The view
// carries no modality and therefore cannot turn AffirmsObtaining into evidence
// that the direct relation obtains.
func NewRelationConstraintEvaluationViewFromRelationalAssertion(
	assertion RelationalAssertion,
) (RelationConstraintEvaluationView, error) {
	if !assertion.valid() {
		return RelationConstraintEvaluationView{}, fmt.Errorf(
			"relation-constraint view requires a valid final RelationalAssertion",
		)
	}
	return newRelationConstraintEvaluationViewFromBindings(
		assertion.Signature(),
		assertion.Bindings(),
	)
}

func newRelationConstraintEvaluationViewFromBindings(
	signature RelationSignatureRef,
	bindings []SlotBinding,
) (RelationConstraintEvaluationView, error) {
	slots := make([]RelationConstraintSlotView, 0, len(bindings))
	for _, binding := range bindings {
		view, err := relationConstraintSlotViewFromFinalBinding(binding)
		if err != nil {
			return RelationConstraintEvaluationView{}, err
		}
		slots = append(slots, view)
	}
	return NewProspectiveRelationConstraintEvaluationView(signature, slots)
}

// NewRelationConstraintEvaluationViewFromInstanceForFragment lowers one
// already-persisted final relation into the exact target-fragment coordinate
// used by activation-time revalidation. It preserves the assertion's resolved
// EntityIDs and filler counts. A reference filler is retargeted only when its
// stable RefKindID agrees with the target SlotSpec; a disagreement is retained
// so EvaluateRelationConstraints can report the ordinary reference-kind
// contradiction instead of silently relabelling it.
//
// This is not admission, schema migration, or relation rewriting. It creates
// only the transient non-canonical evaluation view shared by the existing
// relation-constraint evaluator.
func NewRelationConstraintEvaluationViewFromInstanceForFragment(
	relation RelationInstance,
	fragment TypedRelationDeclarationFragment,
) (RelationConstraintEvaluationView, error) {
	if !relation.valid() {
		return RelationConstraintEvaluationView{}, fmt.Errorf(
			"target relation-constraint view requires a valid final RelationInstance",
		)
	}
	return newRelationConstraintEvaluationViewForFragment(
		relation.Signature(),
		relation.Bindings(),
		fragment,
	)
}

// NewRelationConstraintEvaluationViewFromRelationalAssertionForFragment
// lowers an exact v3 assertion into the same transient target-fragment view.
// Modality is deliberately not part of the structural evaluator and is never
// reinterpreted as predicate truth or occurrence identity.
func NewRelationConstraintEvaluationViewFromRelationalAssertionForFragment(
	assertion RelationalAssertion,
	fragment TypedRelationDeclarationFragment,
) (RelationConstraintEvaluationView, error) {
	if !assertion.valid() {
		return RelationConstraintEvaluationView{}, fmt.Errorf(
			"target relation-constraint view requires a valid final RelationalAssertion",
		)
	}
	return newRelationConstraintEvaluationViewForFragment(
		assertion.Signature(),
		assertion.Bindings(),
		fragment,
	)
}

func newRelationConstraintEvaluationViewForFragment(
	persistedSignature RelationSignatureRef,
	bindings []SlotBinding,
	fragment TypedRelationDeclarationFragment,
) (RelationConstraintEvaluationView, error) {
	if !fragment.valid() {
		return RelationConstraintEvaluationView{}, fmt.Errorf(
			"target relation-constraint view requires a valid typed relation declaration fragment",
		)
	}
	if persistedSignature.ID() != fragment.Ref().ID() {
		return RelationConstraintEvaluationView{}, fmt.Errorf(
			"target relation-constraint fragment ID differs from the persisted relation",
		)
	}

	slots := make([]RelationConstraintSlotView, 0, len(bindings))
	for _, binding := range bindings {
		view, err := relationConstraintSlotViewFromFinalBindingForFragment(
			binding,
			fragment,
		)
		if err != nil {
			return RelationConstraintEvaluationView{}, err
		}
		slots = append(slots, view)
	}
	return NewProspectiveRelationConstraintEvaluationView(
		fragment.Ref(),
		slots,
	)
}

func relationConstraintSlotViewFromFinalBindingForFragment(
	binding SlotBinding,
	fragment TypedRelationDeclarationFragment,
) (RelationConstraintSlotView, error) {
	view, err := relationConstraintSlotViewFromFinalBinding(binding)
	if err != nil {
		return nil, err
	}
	references, isReference := view.(RelationConstraintReferenceSlotView)
	if !isReference {
		return view, nil
	}

	slot, found := fragment.Slot(binding.Name())
	if !found {
		return references, nil
	}
	target, targetIsReference := slot.Target().(ReferenceSlotTarget)
	if !targetIsReference {
		return references, nil
	}
	for index, state := range references.references {
		if state.refKind.ID() != target.ReferenceKind().ID() {
			continue
		}
		references.references[index].refKind = target.ReferenceKind()
	}
	sortRelationConstraintReferenceStates(references.references)
	return references, nil
}

// NewRelationConstraintEvaluationViewFromInstanceForSignature preserves the
// historical API spelling for exact edition replay.
func NewRelationConstraintEvaluationViewFromInstanceForSignature(
	relation RelationInstance,
	signature RelationSignature,
) (RelationConstraintEvaluationView, error) {
	return NewRelationConstraintEvaluationViewFromInstanceForFragment(
		relation,
		signature,
	)
}

// NewRelationConstraintEvaluationViewFromRelationalAssertionForSignature
// preserves the historical API spelling for exact edition replay.
func NewRelationConstraintEvaluationViewFromRelationalAssertionForSignature(
	assertion RelationalAssertion,
	signature RelationSignature,
) (RelationConstraintEvaluationView, error) {
	return NewRelationConstraintEvaluationViewFromRelationalAssertionForFragment(
		assertion,
		signature,
	)
}

func relationConstraintSlotViewFromFinalBinding(
	binding SlotBinding,
) (RelationConstraintSlotView, error) {
	fillers := binding.Fillers()
	if len(fillers) == 0 {
		return nil, fmt.Errorf("final relation slot %s has no fillers", binding.Name().String())
	}

	switch fillers[0].(type) {
	case ReferenceFiller:
		states := make([]relationConstraintReferenceState, 0, len(fillers))
		for index, filler := range fillers {
			reference, ok := filler.(ReferenceFiller)
			if !ok || !reference.validSlotFiller() {
				return nil, fmt.Errorf("final relation slot %s mixes filler modes at index %d", binding.Name().String(), index)
			}
			states = append(states, relationConstraintReferenceState{
				kind:      relationConstraintReferenceResolved,
				entity:    reference.Entity(),
				reference: reference.Reference().ReferenceKey(),
				refKind:   reference.Reference().RefKind(),
			})
		}
		sortRelationConstraintReferenceStates(states)
		return RelationConstraintReferenceSlotView{
			slot:       binding.Name(),
			count:      uint64(len(states)),
			references: states,
		}, nil
	case ValueFiller:
		for index, filler := range fillers {
			value, ok := filler.(ValueFiller)
			if !ok || !value.validSlotFiller() {
				return nil, fmt.Errorf("final relation slot %s mixes filler modes at index %d", binding.Name().String(), index)
			}
		}
		return NewRelationConstraintValueSlotView(binding.Name(), uint64(len(fillers)))
	default:
		return nil, fmt.Errorf("final relation slot %s contains an unknown filler mode", binding.Name().String())
	}
}

func (view RelationConstraintEvaluationView) Signature() RelationSignatureRef {
	return view.signature
}

func (view RelationConstraintEvaluationView) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return view.signature
}

func (view RelationConstraintEvaluationView) Slots() []RelationConstraintSlotView {
	return copyRelationConstraintSlotViews(view.slots)
}

func (view RelationConstraintEvaluationView) slot(
	slot SlotKindID,
) (RelationConstraintSlotView, bool) {
	index := sort.Search(len(view.slots), func(index int) bool {
		return view.slots[index].Slot().String() >= slot.String()
	})
	if index >= len(view.slots) || view.slots[index].Slot() != slot {
		return nil, false
	}
	return copyRelationConstraintSlotView(view.slots[index]), true
}

func (view RelationConstraintEvaluationView) valid() bool {
	if !view.signature.valid() || len(view.slots) == 0 {
		return false
	}
	for index, slot := range view.slots {
		if !validRelationConstraintSlotView(slot) {
			return false
		}
		if index > 0 && view.slots[index-1].Slot().String() >= slot.Slot().String() {
			return false
		}
	}
	return true
}

func normalizeRelationConstraintSlotViews(
	values []RelationConstraintSlotView,
) ([]RelationConstraintSlotView, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("relation-constraint view requires at least one present slot")
	}
	result := copyRelationConstraintSlotViews(values)
	for index, slot := range result {
		if !validRelationConstraintSlotView(slot) {
			return nil, fmt.Errorf("relation-constraint slot %d is invalid", index)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Slot().String() < result[right].Slot().String()
	})
	for index := 1; index < len(result); index++ {
		if result[index-1].Slot() == result[index].Slot() {
			return nil, fmt.Errorf("relation-constraint view repeats slot %q", result[index].Slot().String())
		}
	}
	return result, nil
}

func validRelationConstraintSlotView(view RelationConstraintSlotView) bool {
	switch value := view.(type) {
	case RelationConstraintValueSlotView:
		return value.validRelationConstraintSlotView()
	case RelationConstraintReferenceSlotView:
		return value.validRelationConstraintSlotView()
	default:
		return false
	}
}

func copyRelationConstraintSlotViews(
	values []RelationConstraintSlotView,
) []RelationConstraintSlotView {
	result := make([]RelationConstraintSlotView, 0, len(values))
	for _, value := range values {
		result = append(result, copyRelationConstraintSlotView(value))
	}
	return result
}

func copyRelationConstraintSlotView(
	view RelationConstraintSlotView,
) RelationConstraintSlotView {
	switch value := view.(type) {
	case RelationConstraintValueSlotView:
		return value
	case RelationConstraintReferenceSlotView:
		return RelationConstraintReferenceSlotView{
			slot:       value.slot,
			count:      value.count,
			references: append([]relationConstraintReferenceState(nil), value.references...),
		}
	default:
		return nil
	}
}

type RelationConstraintEvaluationKind string

const (
	RelationConstraintsValid           RelationConstraintEvaluationKind = "valid"
	RelationConstraintsInvalid         RelationConstraintEvaluationKind = "invalid"
	RelationConstraintsUnderdetermined RelationConstraintEvaluationKind = "underdetermined"
)

// RelationConstraintEvaluation is a closed domain outcome. It intentionally
// does not implement ValidationVerdict: a successful constraint check is not
// an AdmissionBatch and grants no persistence capability.
type RelationConstraintEvaluation interface {
	Kind() RelationConstraintEvaluationKind
	Diagnostics() []Diagnostic
	CheckedConstraints() []ConstraintID
	relationConstraintEvaluationVariant()
}

type relationConstraintsValid struct {
	checked []ConstraintID
}

func (relationConstraintsValid) Kind() RelationConstraintEvaluationKind {
	return RelationConstraintsValid
}

func (relationConstraintsValid) Diagnostics() []Diagnostic { return nil }

func (outcome relationConstraintsValid) CheckedConstraints() []ConstraintID {
	return append([]ConstraintID(nil), outcome.checked...)
}

func (relationConstraintsValid) relationConstraintEvaluationVariant() {}

type relationConstraintsInvalid struct {
	diagnostics []Diagnostic
	checked     []ConstraintID
}

func (relationConstraintsInvalid) Kind() RelationConstraintEvaluationKind {
	return RelationConstraintsInvalid
}

func (outcome relationConstraintsInvalid) Diagnostics() []Diagnostic {
	return copyDiagnostics(outcome.diagnostics)
}

func (outcome relationConstraintsInvalid) CheckedConstraints() []ConstraintID {
	return append([]ConstraintID(nil), outcome.checked...)
}

func (relationConstraintsInvalid) relationConstraintEvaluationVariant() {}

type relationConstraintsUnderdetermined struct {
	diagnostics []Diagnostic
	checked     []ConstraintID
}

func (relationConstraintsUnderdetermined) Kind() RelationConstraintEvaluationKind {
	return RelationConstraintsUnderdetermined
}

func (outcome relationConstraintsUnderdetermined) Diagnostics() []Diagnostic {
	return copyDiagnostics(outcome.diagnostics)
}

func (outcome relationConstraintsUnderdetermined) CheckedConstraints() []ConstraintID {
	return append([]ConstraintID(nil), outcome.checked...)
}

func (relationConstraintsUnderdetermined) relationConstraintEvaluationVariant() {}

// EvaluateRelationConstraints applies the three closed resolved-reference
// relation laws. It performs no IO and can therefore be called with the same
// view by admission and by activation-time revalidation.
func EvaluateRelationConstraints(
	environment TypeEnv,
	view RelationConstraintEvaluationView,
) RelationConstraintEvaluation {
	accumulator := relationConstraintEvaluationAccumulator{}
	if !view.valid() {
		accumulator.addUnderdetermined(
			DiagnosticTypeRuleUnavailable,
			"a complete relation-constraint evaluation view is required",
			"relation.constraint_view",
			"rebuild-relation-constraint-view",
			diagnosticState("complete exact-coordinate relation-constraint view"),
			MissingRuntimeValidator,
			diagnosticUnknown("the evaluation view was absent or structurally incomplete"),
		)
		return accumulator.outcome()
	}
	if !environment.Ref().valid() {
		accumulator.addUnderdetermined(
			DiagnosticTypeRuleUnavailable,
			"an exact active TypeEnv is required for relation constraints",
			"relation.signature",
			"inspect-project-active-typeenv",
			diagnosticReference(view.signature.TypeEnv().String()),
			MissingRuntimeActiveTypeEnv,
			diagnosticState("absent active TypeEnv"),
		)
		return accumulator.outcome()
	}
	if view.signature.TypeEnv() != environment.Ref() {
		accumulator.addUnderdetermined(
			DiagnosticTypeRuleUnavailable,
			"relation and constraint environment do not share the exact TypeEnv",
			"relation.signature",
			"reload-relation-and-constraints-from-one-typeenv",
			diagnosticReference(view.signature.TypeEnv().String()),
			MissingRuntimeActiveTypeEnv,
			diagnosticReference(environment.Ref().String()),
		)
		return accumulator.outcome()
	}

	fragment, found := environment.TypedRelationDeclarationFragment(view.signature)
	if !found {
		accumulator.addMissingDeclaration(
			environment.Ref(),
			DiagnosticSignatureNotActive,
			fmt.Sprintf(
				"typed relation declaration fragment %s is absent from the exact TypeEnv",
				view.signature.String(),
			),
			"relation.signature",
			"inspect-or-stage-the-required-typed-relation-declaration-fragment",
			diagnosticReference(view.signature.String()),
		)
		return accumulator.outcome()
	}

	validateRelationConstraintViewSlots(&accumulator, fragment, view)
	for _, rule := range environment.Constraints() {
		switch constraint := rule.(type) {
		case SlotCardinalityConstraint:
			if constraint.Signature() != view.signature {
				continue
			}
			accumulator.check(constraint.ID())
			evaluateSlotCardinalityConstraint(&accumulator, constraint, view)
		case ReferenceSlotSubsetConstraint:
			if constraint.Signature() != view.signature {
				continue
			}
			accumulator.check(constraint.ID())
			evaluateReferenceSlotSubsetConstraint(&accumulator, constraint, view)
		case ReferenceSlotPartitionConstraint:
			if constraint.Signature() != view.signature {
				continue
			}
			accumulator.check(constraint.ID())
			evaluateReferenceSlotPartitionConstraint(&accumulator, constraint, view)
		}
	}
	return accumulator.outcome()
}

type relationConstraintEvaluationAccumulator struct {
	diagnostics []Diagnostic
	checked     []ConstraintID
}

func (accumulator *relationConstraintEvaluationAccumulator) check(id ConstraintID) {
	accumulator.checked = append(accumulator.checked, id)
}

func (accumulator *relationConstraintEvaluationAccumulator) addInvalid(
	code DiagnosticCode,
	message string,
	path string,
	provenance DeclarationProvenance,
	expected DiagnosticDatum,
	actual DiagnosticDatum,
) {
	diagnosticPath, _ := NewDiagnosticPath(path)
	witness, _ := NewExpectedActualWitness(expected, actual)
	basis, _ := NewKnownDeclarationBasis(provenance)
	diagnostic, _ := NewInvalidDiagnosticWithDetails(
		code,
		message,
		diagnosticPath,
		witness,
		basis,
		[]RepairCandidate{defaultInvalidRepair(code, diagnosticPath, expected)},
	)
	accumulator.diagnostics = append(accumulator.diagnostics, diagnostic)
}

func (accumulator *relationConstraintEvaluationAccumulator) addUnderdetermined(
	code DiagnosticCode,
	message string,
	path string,
	repairRaw string,
	required DiagnosticDatum,
	missingKind MissingRuntimeBasisKind,
	actual DiagnosticDatum,
) {
	diagnosticPath, _ := NewDiagnosticPath(path)
	witness, _ := NewMissingBasisWitnessWithActual(required, actual)
	basis, _ := NewMissingRuntimeBasis(missingKind, required)
	repair, _ := NewRepairPointer(repairRaw)
	diagnostic, _ := NewUnderdeterminedDiagnosticWithDetails(
		code,
		message,
		diagnosticPath,
		witness,
		basis,
		repair,
		[]RepairCandidate{defaultMissingBasisRepair(code, basis, repair, required)},
	)
	accumulator.diagnostics = append(accumulator.diagnostics, diagnostic)
}

func (accumulator *relationConstraintEvaluationAccumulator) addMissingDeclaration(
	typeEnv TypeEnvRef,
	code DiagnosticCode,
	message string,
	path string,
	repairRaw string,
	required DiagnosticDatum,
) {
	diagnosticPath, _ := NewDiagnosticPath(path)
	witness, _ := NewMissingBasisWitnessWithActual(
		required,
		diagnosticUnknown("the exact TypeEnv declaration was not available"),
	)
	basis, _ := NewMissingTypeEnvDeclarationBasis(typeEnv, required)
	repair, _ := NewRepairPointer(repairRaw)
	diagnostic, _ := NewUnderdeterminedDiagnosticWithDetails(
		code,
		message,
		diagnosticPath,
		witness,
		basis,
		repair,
		[]RepairCandidate{defaultMissingBasisRepair(code, basis, repair, required)},
	)
	accumulator.diagnostics = append(accumulator.diagnostics, diagnostic)
}

func (accumulator relationConstraintEvaluationAccumulator) outcome() RelationConstraintEvaluation {
	diagnostics := append([]Diagnostic(nil), accumulator.diagnostics...)
	sort.SliceStable(diagnostics, func(left, right int) bool {
		leftKey := diagnostics[left].Path().String() + ":" + string(diagnostics[left].Code()) + ":" + diagnostics[left].Message()
		rightKey := diagnostics[right].Path().String() + ":" + string(diagnostics[right].Code()) + ":" + diagnostics[right].Message()
		return leftKey < rightKey
	})
	checked := append([]ConstraintID(nil), accumulator.checked...)
	sort.Slice(checked, func(left, right int) bool {
		return checked[left].String() < checked[right].String()
	})
	if hasDiagnosticPosture(diagnostics, DiagnosticInvalid) {
		return relationConstraintsInvalid{diagnostics: diagnostics, checked: checked}
	}
	if len(diagnostics) > 0 {
		return relationConstraintsUnderdetermined{diagnostics: diagnostics, checked: checked}
	}
	return relationConstraintsValid{checked: checked}
}

func validateRelationConstraintViewSlots(
	accumulator *relationConstraintEvaluationAccumulator,
	fragment TypedRelationDeclarationFragment,
	view RelationConstraintEvaluationView,
) {
	for _, slotView := range view.slots {
		slot, found := fragment.Slot(slotView.Slot())
		path := relationConstraintSlotPath(view.signature, slotView.Slot())
		if !found {
			accumulator.addInvalid(
				DiagnosticUnknownSlot,
				fmt.Sprintf(
					"slot %s is not declared by typed relation declaration fragment %s",
					slotView.Slot().String(),
					view.signature.String(),
				),
				path,
				fragment.Provenance(),
				diagnosticSlotNames(fragment),
				diagnosticReference(slotView.Slot().String()),
			)
			continue
		}
		switch target := slot.Target().(type) {
		case ReferenceSlotTarget:
			references, ok := slotView.(RelationConstraintReferenceSlotView)
			if !ok {
				accumulator.addInvalid(
					DiagnosticReferenceModeMismatch,
					fmt.Sprintf("slot %s requires resolved ByReference fillers", slotView.Slot().String()),
					path,
					slot.Provenance(),
					diagnosticState(SlotByReference.String()),
					diagnosticState(SlotByValue.String()),
				)
				continue
			}
			mismatchedKinds := make([]string, 0)
			for _, state := range references.references {
				if !state.refKind.valid() || state.refKind == target.ReferenceKind() {
					continue
				}
				mismatchedKinds = append(mismatchedKinds, state.refKind.String())
			}
			mismatchedKinds = uniqueSortedStrings(mismatchedKinds)
			if len(mismatchedKinds) > 0 {
				accumulator.addInvalid(
					DiagnosticReferenceKindMismatch,
					fmt.Sprintf("slot %s contains a reference from a different exact RefKind", slotView.Slot().String()),
					path,
					slot.Provenance(),
					diagnosticReference(target.ReferenceKind().String()),
					diagnosticSet(mismatchedKinds),
				)
			}
		case ValueSlotTarget:
			if _, ok := slotView.(RelationConstraintValueSlotView); !ok {
				accumulator.addInvalid(
					DiagnosticReferenceModeMismatch,
					fmt.Sprintf("slot %s requires ByValue fillers", slotView.Slot().String()),
					path,
					slot.Provenance(),
					diagnosticState(SlotByValue.String()),
					diagnosticState(SlotByReference.String()),
				)
			}
		}
	}
}

func evaluateSlotCardinalityConstraint(
	accumulator *relationConstraintEvaluationAccumulator,
	constraint SlotCardinalityConstraint,
	view RelationConstraintEvaluationView,
) {
	count := uint64(0)
	if slot, found := view.slot(constraint.Slot()); found {
		count = slot.FillerCount()
	}
	if constraint.Cardinality().Allows(count) {
		return
	}
	accumulator.addInvalid(
		DiagnosticCardinalityMismatch,
		fmt.Sprintf("slot-cardinality constraint %s is not satisfied", constraint.ID().String()),
		relationConstraintPath(view.signature, constraint.ID(), constraint.Slot()),
		constraint.Provenance(),
		diagnosticCardinality(constraint.Cardinality()),
		NewDiagnosticCountDatum(count),
	)
}

type resolvedReferenceEntitySet struct {
	entities   []EntityID
	duplicates []EntityID
	complete   bool
}

func resolvedReferenceEntities(
	accumulator *relationConstraintEvaluationAccumulator,
	constraint ConstraintRule,
	signature RelationSignatureRef,
	slotID SlotKindID,
	view RelationConstraintEvaluationView,
) resolvedReferenceEntitySet {
	slotView, present := view.slot(slotID)
	if !present {
		return resolvedReferenceEntitySet{complete: true}
	}
	references, referenceMode := slotView.(RelationConstraintReferenceSlotView)
	if !referenceMode {
		return resolvedReferenceEntitySet{}
	}

	entities := make([]EntityID, 0, len(references.references))
	complete := true
	for _, state := range references.references {
		if state.kind == relationConstraintReferenceResolved {
			entities = append(entities, state.entity)
			continue
		}
		complete = false
		path := relationConstraintPath(signature, constraint.ID(), slotID)
		code := DiagnosticReferenceUnresolved
		missingKind := MissingRuntimeResolution
		required := diagnosticState("resolved stable EntityID for " + state.reference)
		actual := diagnosticUnknown("stable EntityID resolution was not available")
		message := fmt.Sprintf(
			"constraint %s requires every %s filler to resolve to a stable EntityID",
			constraint.ID().String(),
			slotID.String(),
		)
		if state.kind == relationConstraintReferenceBridgeMissing {
			code = DiagnosticContextBridgeMissing
			missingKind = MissingRuntimeDeclaration
			required = diagnosticState("exact active context bridge for " + state.reference)
			actual = diagnosticState("context bridge absent")
			message = fmt.Sprintf(
				"constraint %s cannot use %s until its exact context bridge is active",
				constraint.ID().String(),
				slotID.String(),
			)
		}
		accumulator.addUnderdetermined(
			code,
			message,
			path,
			state.repair.String(),
			required,
			missingKind,
			actual,
		)
	}
	sort.Slice(entities, func(left, right int) bool {
		return entities[left].String() < entities[right].String()
	})

	duplicates := duplicateEntityIDs(entities)
	for _, duplicate := range duplicates {
		accumulator.addInvalid(
			DiagnosticReferenceEntityDuplicate,
			fmt.Sprintf("constraint %s rejects duplicate resolved EntityID %s in slot %s", constraint.ID().String(), duplicate.String(), slotID.String()),
			relationConstraintPath(signature, constraint.ID(), slotID),
			constraint.Provenance(),
			diagnosticState("unique resolved EntityIDs"),
			diagnosticReference(duplicate.String()),
		)
	}
	return resolvedReferenceEntitySet{
		entities:   uniqueEntityIDs(entities),
		duplicates: duplicates,
		complete:   complete,
	}
}

func evaluateReferenceSlotSubsetConstraint(
	accumulator *relationConstraintEvaluationAccumulator,
	constraint ReferenceSlotSubsetConstraint,
	view RelationConstraintEvaluationView,
) {
	subset := resolvedReferenceEntities(accumulator, constraint, view.signature, constraint.Subset(), view)
	superset := resolvedReferenceEntities(accumulator, constraint, view.signature, constraint.Superset(), view)
	// An incomplete superset could still resolve to any currently missing
	// subset entity, so absence is provable only when the superset is complete.
	// Incompleteness of the subset does not erase contradictions already
	// witnessed by its resolved entities.
	if !superset.complete {
		return
	}

	missing := entitySetDifference(subset.entities, superset.entities)
	if len(missing) == 0 {
		return
	}
	accumulator.addInvalid(
		DiagnosticReferenceSubsetMismatch,
		fmt.Sprintf("constraint %s has subset EntityIDs absent from slot %s", constraint.ID().String(), constraint.Superset().String()),
		relationConstraintPath(view.signature, constraint.ID(), constraint.Subset()),
		constraint.Provenance(),
		diagnosticState("every subset EntityID present in the superset slot"),
		diagnosticSet(entityIDStrings(missing)),
	)
}

func evaluateReferenceSlotPartitionConstraint(
	accumulator *relationConstraintEvaluationAccumulator,
	constraint ReferenceSlotPartitionConstraint,
	view RelationConstraintEvaluationView,
) {
	whole := resolvedReferenceEntities(accumulator, constraint, view.signature, constraint.Whole(), view)
	parts := make([]resolvedReferenceEntitySet, 0, len(constraint.Parts()))
	partsComplete := true
	for _, slot := range constraint.Parts() {
		part := resolvedReferenceEntities(accumulator, constraint, view.signature, slot, view)
		parts = append(parts, part)
		partsComplete = partsComplete && part.complete
	}

	partUnion := make([]EntityID, 0)
	seenPart := map[string]SlotKindID{}
	overlaps := make([]EntityID, 0)
	partSlots := constraint.Parts()
	for index, part := range parts {
		for _, entity := range part.entities {
			if _, found := seenPart[entity.String()]; found {
				overlaps = append(overlaps, entity)
				continue
			}
			seenPart[entity.String()] = partSlots[index]
			partUnion = append(partUnion, entity)
		}
	}
	overlaps = uniqueSortedEntityIDs(overlaps)
	partUnion = uniqueSortedEntityIDs(partUnion)
	if len(overlaps) > 0 {
		accumulator.addInvalid(
			DiagnosticReferencePartitionMismatch,
			fmt.Sprintf("constraint %s has EntityIDs in more than one partition part", constraint.ID().String()),
			relationConstraintPath(view.signature, constraint.ID(), constraint.Whole()),
			constraint.Provenance(),
			diagnosticState("pairwise-disjoint partition parts"),
			diagnosticSet(entityIDStrings(overlaps)),
		)
	}

	missingFromParts := []EntityID(nil)
	if partsComplete {
		missingFromParts = entitySetDifference(whole.entities, partUnion)
	}
	extraInParts := []EntityID(nil)
	if whole.complete {
		extraInParts = entitySetDifference(partUnion, whole.entities)
	}
	if len(missingFromParts) == 0 && len(extraInParts) == 0 {
		return
	}
	mismatch := append(append([]EntityID(nil), missingFromParts...), extraInParts...)
	mismatch = uniqueSortedEntityIDs(mismatch)
	accumulator.addInvalid(
		DiagnosticReferencePartitionMismatch,
		fmt.Sprintf("constraint %s requires Whole to equal the union of its parts", constraint.ID().String()),
		relationConstraintPath(view.signature, constraint.ID(), constraint.Whole()),
		constraint.Provenance(),
		diagnosticState("whole EntityID set equals the disjoint part union"),
		diagnosticSet(entityIDStrings(mismatch)),
	)
}

func duplicateEntityIDs(values []EntityID) []EntityID {
	duplicates := make([]EntityID, 0)
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] &&
			(len(duplicates) == 0 || duplicates[len(duplicates)-1] != values[index]) {
			duplicates = append(duplicates, values[index])
		}
	}
	return duplicates
}

func uniqueEntityIDs(values []EntityID) []EntityID {
	result := make([]EntityID, 0, len(values))
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func uniqueSortedEntityIDs(values []EntityID) []EntityID {
	result := append([]EntityID(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return result[left].String() < result[right].String()
	})
	return uniqueEntityIDs(result)
}

func entitySetDifference(left, right []EntityID) []EntityID {
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value.String()] = struct{}{}
	}
	result := make([]EntityID, 0)
	for _, value := range left {
		if _, found := rightSet[value.String()]; !found {
			result = append(result, value)
		}
	}
	return result
}

func entityIDStrings(values []EntityID) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func uniqueSortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	unique := make([]string, 0, len(result))
	for _, value := range result {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	return unique
}

func relationConstraintSlotPath(
	signature RelationSignatureRef,
	slot SlotKindID,
) string {
	return fmt.Sprintf("relation[%s].slots.%s", signature.ID().String(), slot.String())
}

func relationConstraintPath(
	signature RelationSignatureRef,
	constraint ConstraintID,
	slot SlotKindID,
) string {
	return fmt.Sprintf(
		"relation[%s].constraints.%s.slots.%s",
		signature.ID().String(),
		constraint.String(),
		slot.String(),
	)
}
