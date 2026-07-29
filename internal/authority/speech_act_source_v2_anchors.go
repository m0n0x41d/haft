package authority

import (
	"fmt"
	"slices"
)

// SpeechActSourceV2Anchors contains only the reliance-bearing anchors missing
// from the v1 generic SpeechAct source. It contains no authority judgement,
// permission, use, or domain effect.
type SpeechActSourceV2Anchors struct {
	state *speechActSourceV2AnchorsState
}

type speechActSourceV2AnchorsState struct {
	workRef             WorkRef
	descriptionRef      DescriptionRef
	resourceLedgerRef   ResourceLedgerRef
	acceptancePosture   AcceptancePostureRef
	auditTraceRefs      []AuditTraceRef
	descriptionCarriers []ObservableCarrierBinding
}

type SpeechActSourceV2AnchorsBuilder struct {
	value speechActSourceV2AnchorsState
}

func NewSpeechActSourceV2AnchorsBuilder(
	workRef WorkRef,
	descriptionRef DescriptionRef,
) SpeechActSourceV2AnchorsBuilder {
	return SpeechActSourceV2AnchorsBuilder{value: speechActSourceV2AnchorsState{
		workRef:        workRef,
		descriptionRef: descriptionRef,
	}}
}

func (builder SpeechActSourceV2AnchorsBuilder) WithResourceLedger(
	ref ResourceLedgerRef,
) SpeechActSourceV2AnchorsBuilder {
	builder.value.resourceLedgerRef = ref
	return builder
}

func (builder SpeechActSourceV2AnchorsBuilder) WithAcceptancePosture(
	ref AcceptancePostureRef,
) SpeechActSourceV2AnchorsBuilder {
	builder.value.acceptancePosture = ref
	return builder
}

func (builder SpeechActSourceV2AnchorsBuilder) WithAuditTrace(
	ref AuditTraceRef,
) SpeechActSourceV2AnchorsBuilder {
	builder.value.auditTraceRefs = append(builder.value.auditTraceRefs, ref)
	return builder
}

func (builder SpeechActSourceV2AnchorsBuilder) WithDescriptionCarrier(
	binding ObservableCarrierBinding,
) SpeechActSourceV2AnchorsBuilder {
	builder.value.descriptionCarriers = append(builder.value.descriptionCarriers, binding)
	return builder
}

func (builder SpeechActSourceV2AnchorsBuilder) Build() (SpeechActSourceV2Anchors, error) {
	state, err := canonicalSpeechActSourceV2Anchors(builder.value)
	if err != nil {
		return SpeechActSourceV2Anchors{}, err
	}
	return SpeechActSourceV2Anchors{state: &state}, nil
}

func canonicalSpeechActSourceV2Anchors(
	value speechActSourceV2AnchorsState,
) (speechActSourceV2AnchorsState, error) {
	identitiesValid := value.workRef.valid() &&
		value.descriptionRef.valid() &&
		value.resourceLedgerRef.valid() &&
		value.acceptancePosture.valid()
	if !identitiesValid {
		return speechActSourceV2AnchorsState{}, fmt.Errorf(
			"SpeechAct source v2 anchors are incomplete",
		)
	}
	if len(value.auditTraceRefs) == 0 ||
		len(value.auditTraceRefs) > speechActSourceV2MaxAuditTraceRefs {
		return speechActSourceV2AnchorsState{}, fmt.Errorf(
			"SpeechAct source v2 requires 1..%d audit-trace refs",
			speechActSourceV2MaxAuditTraceRefs,
		)
	}
	if len(value.descriptionCarriers) == 0 ||
		len(value.descriptionCarriers) > speechActSourceV2MaxDescriptionCarriers {
		return speechActSourceV2AnchorsState{}, fmt.Errorf(
			"SpeechAct source v2 requires 1..%d observable description carriers",
			speechActSourceV2MaxDescriptionCarriers,
		)
	}
	auditTraceRefs := append([]AuditTraceRef{}, value.auditTraceRefs...)
	if slices.ContainsFunc(auditTraceRefs, func(ref AuditTraceRef) bool {
		return !ref.valid()
	}) {
		return speechActSourceV2AnchorsState{}, fmt.Errorf(
			"SpeechAct source v2 requires non-empty canonical audit-trace refs",
		)
	}
	slices.SortFunc(auditTraceRefs, func(left AuditTraceRef, right AuditTraceRef) int {
		return compareStrings(left.String(), right.String())
	})
	if adjacentAuditTraceRefDuplicate(auditTraceRefs, 1) {
		return speechActSourceV2AnchorsState{}, fmt.Errorf(
			"SpeechAct source v2 audit-trace refs must be unique",
		)
	}
	carriers := append([]ObservableCarrierBinding{}, value.descriptionCarriers...)
	if slices.ContainsFunc(carriers, func(binding ObservableCarrierBinding) bool {
		return !binding.valid()
	}) {
		return speechActSourceV2AnchorsState{}, fmt.Errorf(
			"SpeechAct source v2 requires at least one observable description carrier",
		)
	}
	slices.SortFunc(carriers, func(left ObservableCarrierBinding, right ObservableCarrierBinding) int {
		return compareStrings(left.ref.String(), right.ref.String())
	})
	if adjacentObservableCarrierDuplicate(carriers, 1) {
		return speechActSourceV2AnchorsState{}, fmt.Errorf(
			"SpeechAct source v2 description-carrier refs must be unique",
		)
	}
	value.auditTraceRefs = auditTraceRefs
	value.descriptionCarriers = carriers
	return value, nil
}

func adjacentAuditTraceRefDuplicate(values []AuditTraceRef, index int) bool {
	if index >= len(values) {
		return false
	}
	if values[index-1].String() == values[index].String() {
		return true
	}
	return adjacentAuditTraceRefDuplicate(values, index+1)
}

func adjacentObservableCarrierDuplicate(
	values []ObservableCarrierBinding,
	index int,
) bool {
	if index >= len(values) {
		return false
	}
	if values[index-1].ref == values[index].ref {
		return true
	}
	return adjacentObservableCarrierDuplicate(values, index+1)
}

func (anchors SpeechActSourceV2Anchors) valid() bool {
	if anchors.state == nil {
		return false
	}
	rebuilt, err := canonicalSpeechActSourceV2Anchors(*anchors.state)
	return err == nil && speechActSourceV2AnchorStatesEqual(rebuilt, *anchors.state)
}

func speechActSourceV2AnchorStatesEqual(
	left speechActSourceV2AnchorsState,
	right speechActSourceV2AnchorsState,
) bool {
	return left.workRef == right.workRef &&
		left.descriptionRef == right.descriptionRef &&
		left.resourceLedgerRef == right.resourceLedgerRef &&
		left.acceptancePosture == right.acceptancePosture &&
		slices.Equal(left.auditTraceRefs, right.auditTraceRefs) &&
		slices.Equal(left.descriptionCarriers, right.descriptionCarriers)
}
