package authority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

const (
	speechActSourceV2Schema                 = "haft.authority.speech-act-source/v2"
	speechActSourceV2DigestDomain           = "haft.authority.speech-act-source/v2"
	speechActSourceV2MaxCanonicalBytes      = 256 * 1024
	speechActSourceV2MaxParameterBindings   = 128
	speechActSourceV2MaxInputRefs           = 128
	speechActSourceV2MaxOutputRefs          = 128
	speechActSourceV2MaxAffectedRefs        = 128
	speechActSourceV2MaxAuditTraceRefs      = 64
	speechActSourceV2MaxDescriptionCarriers = 16
)

// VerifiedSpeechActSourceV2 is a sealed, reliance-bearing description of one
// already-verified generic SpeechAct source. It adds the full Work/source
// anchors needed by future domain-specific authority protocols but is not an
// authority resolution, permission, use, head selection, or domain effect.
//
// There is intentionally no SQLite writer for this type. The existing
// speech_acts table admits only v1 canonical rows; durable v2 storage requires
// a later additive schema owned by the consuming protocol.
type VerifiedSpeechActSourceV2 struct {
	state *verifiedSpeechActSourceV2State
}

type verifiedSpeechActSourceV2State struct {
	basis         VerifiedSpeechActSource
	anchors       SpeechActSourceV2Anchors
	digest        Digest
	canonicalJSON []byte
}

type speechActSourceV2Projection struct {
	Schema                        string                               `json:"schema"`
	SourceBasisSpeechActDigest    string                               `json:"source_basis_speech_act_digest"`
	SpeechActRef                  string                               `json:"speech_act_ref"`
	WorkRef                       string                               `json:"work_ref"`
	ProjectRoot                   string                               `json:"project_root"`
	WorkKind                      string                               `json:"work_kind"`
	ActTypeRefs                   []string                             `json:"act_type_refs"`
	PerformedByRef                string                               `json:"performed_by_role_assignment_ref"`
	PerformedByDigest             string                               `json:"performed_by_role_assignment_digest"`
	MethodRef                     string                               `json:"method_ref"`
	MethodDescriptionRef          string                               `json:"method_description_ref"`
	MethodDescriptionDigest       string                               `json:"method_description_digest"`
	ExecutedWithinRef             string                               `json:"executed_within_system_ref"`
	BoundedContextRef             string                               `json:"bounded_context_ref"`
	WindowFrom                    string                               `json:"window_from"`
	WindowUntil                   string                               `json:"window_until"`
	Parameters                    []workParameterProjection            `json:"parameters"`
	InputRefs                     []string                             `json:"input_refs"`
	OutputRefs                    []string                             `json:"output_refs"`
	ResourceLedgerRef             string                               `json:"resource_ledger_ref"`
	AffectedRefs                  []string                             `json:"affected_refs"`
	StatePlaneRef                 string                               `json:"state_plane_ref"`
	DeltaPredicateRef             string                               `json:"delta_predicate_ref"`
	OutcomeRef                    string                               `json:"outcome_ref"`
	AcceptancePostureRef          string                               `json:"acceptance_posture_ref"`
	AuditTraceRefs                []string                             `json:"audit_trace_refs"`
	Description                   descriptionRefProjection             `json:"description"`
	DescriptionCarriers           []observableCarrierBindingProjection `json:"description_carriers"`
	TerminalCaptureRef            string                               `json:"terminal_capture_carrier_ref"`
	TerminalCaptureDigest         string                               `json:"terminal_capture_carrier_digest"`
	ReviewSubjectRef              string                               `json:"review_subject_ref"`
	ReviewSubjectDigest           string                               `json:"review_subject_digest"`
	InstitutedObjectRef           string                               `json:"instituted_object_ref"`
	PolicyUtteranceDescriptionRef string                               `json:"policy_utterance_description_ref"`
	ContextPolicyRef              string                               `json:"context_policy_ref"`
	ContextPolicyDigest           string                               `json:"context_policy_digest"`
}

type descriptionRefProjection struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type observableCarrierBindingProjection struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

func NewVerifiedSpeechActSourceV2(
	basis VerifiedSpeechActSource,
	anchors SpeechActSourceV2Anchors,
) (VerifiedSpeechActSourceV2, error) {
	state, err := canonicalVerifiedSpeechActSourceV2(basis, anchors)
	if err != nil {
		return VerifiedSpeechActSourceV2{}, err
	}
	return VerifiedSpeechActSourceV2{state: &state}, nil
}

func canonicalVerifiedSpeechActSourceV2(
	basis VerifiedSpeechActSource,
	anchors SpeechActSourceV2Anchors,
) (verifiedSpeechActSourceV2State, error) {
	if !basis.valid() || !anchors.valid() {
		return verifiedSpeechActSourceV2State{}, fmt.Errorf(
			"SpeechAct source v2 requires a package-verified v1 basis and canonical anchors",
		)
	}
	if err := validateSpeechActSourceV2BasisCollectionLimits(basis); err != nil {
		return verifiedSpeechActSourceV2State{}, err
	}
	if err := validateSpeechActSourceV2ReferenceSeparation(basis, anchors); err != nil {
		return verifiedSpeechActSourceV2State{}, err
	}
	projection := projectSpeechActSourceV2(basis, anchors)
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return verifiedSpeechActSourceV2State{}, fmt.Errorf(
			"encode SpeechAct source v2: %w",
			err,
		)
	}
	if len(canonicalJSON) > speechActSourceV2MaxCanonicalBytes {
		return verifiedSpeechActSourceV2State{}, fmt.Errorf(
			"SpeechAct source v2 canonical material exceeds %d bytes",
			speechActSourceV2MaxCanonicalBytes,
		)
	}
	writer := newAuthorityDigestWriter(speechActSourceV2DigestDomain)
	writer.add(string(canonicalJSON))
	return verifiedSpeechActSourceV2State{
		basis:         basis,
		anchors:       anchors,
		digest:        writer.digest(),
		canonicalJSON: canonicalJSON,
	}, nil
}

func validateSpeechActSourceV2BasisCollectionLimits(
	basis VerifiedSpeechActSource,
) error {
	act := basis.state.speechAct.state
	withinLimits := len(act.parameters) <= speechActSourceV2MaxParameterBindings &&
		len(act.inputRefs) <= speechActSourceV2MaxInputRefs &&
		len(act.outputRefs) <= speechActSourceV2MaxOutputRefs &&
		len(act.affected) <= speechActSourceV2MaxAffectedRefs
	if withinLimits {
		return nil
	}
	return fmt.Errorf(
		"SpeechAct source v2 basis collections exceed reviewed limits",
	)
}

func validateSpeechActSourceV2ReferenceSeparation(
	basis VerifiedSpeechActSource,
	anchors SpeechActSourceV2Anchors,
) error {
	act := basis.state.speechAct.state
	anchorState := anchors.state
	description := anchorState.descriptionRef.String()
	speechAct := act.ref.String()
	work := anchorState.workRef.String()
	capture := act.captureCarrierRef.String()
	centralRefsDistinct := speechAct != work &&
		speechAct != description &&
		work != description &&
		capture != speechAct &&
		capture != work &&
		capture != description
	if !centralRefsDistinct {
		return fmt.Errorf(
			"SpeechActRef, WorkRef, DescriptionRef, and terminal CaptureRef must remain distinct",
		)
	}
	if description != act.reviewSubjectRef.String() {
		return fmt.Errorf(
			"SpeechAct source v2 DescriptionRef must be the exact reviewed description",
		)
	}
	forbiddenCarrierRefs := map[string]struct{}{
		speechAct:   {},
		work:        {},
		description: {},
		capture:     {},
	}
	if slices.ContainsFunc(anchorState.descriptionCarriers, func(binding ObservableCarrierBinding) bool {
		_, forbidden := forbiddenCarrierRefs[binding.ref.String()]
		return forbidden
	}) {
		return fmt.Errorf(
			"observable description CarrierRef must not collapse with act, Work, description, or terminal capture",
		)
	}
	return nil
}

func projectSpeechActSourceV2(
	basis VerifiedSpeechActSource,
	anchors SpeechActSourceV2Anchors,
) speechActSourceV2Projection {
	act := basis.state.speechAct.state
	policy := basis.state.intent.state.contextPolicy.state
	anchorState := anchors.state
	return speechActSourceV2Projection{
		Schema:                     speechActSourceV2Schema,
		SourceBasisSpeechActDigest: act.digest.String(),
		SpeechActRef:               act.ref.String(),
		WorkRef:                    anchorState.workRef.String(),
		ProjectRoot:                act.projectRoot.String(),
		WorkKind:                   act.workKind,
		ActTypeRefs:                []string{act.actType.String()},
		PerformedByRef:             act.performedByRef.String(),
		PerformedByDigest:          act.performedByDigest.String(),
		MethodRef:                  act.methodRef.String(),
		MethodDescriptionRef:       act.methodDescriptionRef.String(),
		MethodDescriptionDigest:    act.methodDescriptionDigest.String(),
		ExecutedWithinRef:          act.executedWithin.String(),
		BoundedContextRef:          act.boundedContext.String(),
		WindowFrom:                 formatAuthorityTime(act.window.from),
		WindowUntil:                formatAuthorityTime(act.window.until),
		Parameters:                 projectWorkParameters(act.parameters),
		InputRefs:                  append([]string{}, act.inputRefs...),
		OutputRefs:                 append([]string{}, act.outputRefs...),
		ResourceLedgerRef:          anchorState.resourceLedgerRef.String(),
		AffectedRefs:               projectAffectedRefs(act.affected),
		StatePlaneRef:              act.statePlane.String(),
		DeltaPredicateRef:          act.deltaPredicate.String(),
		OutcomeRef:                 act.outcome.String(),
		AcceptancePostureRef:       anchorState.acceptancePosture.String(),
		AuditTraceRefs:             projectAuditTraceRefs(anchorState.auditTraceRefs, 0),
		Description: descriptionRefProjection{
			Kind:   string(anchorState.descriptionRef.Kind()),
			Ref:    anchorState.descriptionRef.String(),
			Digest: act.reviewSubjectDigest.String(),
		},
		DescriptionCarriers:           projectObservableCarrierBindings(anchorState.descriptionCarriers, 0),
		TerminalCaptureRef:            act.captureCarrierRef.String(),
		TerminalCaptureDigest:         act.captureCarrierDigest.String(),
		ReviewSubjectRef:              act.reviewSubjectRef.String(),
		ReviewSubjectDigest:           act.reviewSubjectDigest.String(),
		InstitutedObjectRef:           act.institutedObjectRef.String(),
		PolicyUtteranceDescriptionRef: policy.effectRule.utteranceDescription.String(),
		ContextPolicyRef:              policy.ref.String(),
		ContextPolicyDigest:           policy.digest.String(),
	}
}

func projectAuditTraceRefs(values []AuditTraceRef, index int) []string {
	if index >= len(values) {
		return []string{}
	}
	result := []string{values[index].String()}
	return append(result, projectAuditTraceRefs(values, index+1)...)
}

func projectObservableCarrierBindings(
	values []ObservableCarrierBinding,
	index int,
) []observableCarrierBindingProjection {
	if index >= len(values) {
		return []observableCarrierBindingProjection{}
	}
	result := []observableCarrierBindingProjection{{
		Ref:    values[index].ref.String(),
		Digest: values[index].digest.String(),
	}}
	return append(result, projectObservableCarrierBindings(values, index+1)...)
}

func (source VerifiedSpeechActSourceV2) valid() bool {
	if source.state == nil {
		return false
	}
	rebuilt, err := canonicalVerifiedSpeechActSourceV2(
		source.state.basis,
		source.state.anchors,
	)
	return err == nil &&
		rebuilt.digest == source.state.digest &&
		slices.Equal(rebuilt.canonicalJSON, source.state.canonicalJSON)
}

func (source VerifiedSpeechActSourceV2) Valid() bool { return source.valid() }

func (source VerifiedSpeechActSourceV2) Digest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.digest, true
}

func (source VerifiedSpeechActSourceV2) CanonicalJSON() ([]byte, bool) {
	if !source.valid() {
		return nil, false
	}
	return append([]byte{}, source.state.canonicalJSON...), true
}

func (source VerifiedSpeechActSourceV2) SpeechActRef() (SpeechActRef, bool) {
	if !source.valid() {
		return SpeechActRef{}, false
	}
	return source.state.basis.state.speechAct.state.ref, true
}

func (source VerifiedSpeechActSourceV2) SourceBasisSpeechActDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.basis.state.speechAct.state.digest, true
}

func (source VerifiedSpeechActSourceV2) WorkRef() (WorkRef, bool) {
	if !source.valid() {
		return WorkRef{}, false
	}
	return source.state.anchors.state.workRef, true
}

func (source VerifiedSpeechActSourceV2) ProjectRoot() (ProjectRoot, bool) {
	if !source.valid() {
		return ProjectRoot{}, false
	}
	return source.state.basis.state.speechAct.state.projectRoot, true
}

func (source VerifiedSpeechActSourceV2) ActTypeRefs() ([]SpeechActTypeRef, bool) {
	if !source.valid() {
		return nil, false
	}
	return []SpeechActTypeRef{source.state.basis.state.speechAct.state.actType}, true
}

func (source VerifiedSpeechActSourceV2) MethodDescription() (SpeechActMethodDescription, bool) {
	if !source.valid() {
		return SpeechActMethodDescription{}, false
	}
	return source.state.basis.state.intent.state.executionFrame.state.methodDescription, true
}

func (source VerifiedSpeechActSourceV2) PerformedByRoleAssignment() (AuthorityRoleAssignment, bool) {
	if !source.valid() {
		return AuthorityRoleAssignment{}, false
	}
	return source.state.basis.state.authorizer, true
}

func (source VerifiedSpeechActSourceV2) ExecutedWithin() (SystemRef, bool) {
	if !source.valid() {
		return SystemRef{}, false
	}
	return source.state.basis.state.speechAct.state.executedWithin, true
}

func (source VerifiedSpeechActSourceV2) BoundedContext() (BoundedContextRef, bool) {
	if !source.valid() {
		return BoundedContextRef{}, false
	}
	return source.state.basis.state.speechAct.state.boundedContext, true
}

func (source VerifiedSpeechActSourceV2) WorkWindow() (TimeWindow, bool) {
	if !source.valid() {
		return TimeWindow{}, false
	}
	return source.state.basis.state.speechAct.state.window, true
}

func (source VerifiedSpeechActSourceV2) ParameterBindings() ([]WorkParameterBinding, bool) {
	if !source.valid() {
		return nil, false
	}
	return append([]WorkParameterBinding{}, source.state.basis.state.speechAct.state.parameters...), true
}

func (source VerifiedSpeechActSourceV2) InputRefs() ([]string, bool) {
	if !source.valid() {
		return nil, false
	}
	return append([]string{}, source.state.basis.state.speechAct.state.inputRefs...), true
}

func (source VerifiedSpeechActSourceV2) OutputRefs() ([]string, bool) {
	if !source.valid() {
		return nil, false
	}
	return append([]string{}, source.state.basis.state.speechAct.state.outputRefs...), true
}

func (source VerifiedSpeechActSourceV2) AffectedRefs() ([]AffectedRef, bool) {
	if !source.valid() {
		return nil, false
	}
	return append([]AffectedRef{}, source.state.basis.state.speechAct.state.affected...), true
}

func (source VerifiedSpeechActSourceV2) StatePlaneRef() (StatePlaneRef, bool) {
	if !source.valid() {
		return StatePlaneRef{}, false
	}
	return source.state.basis.state.speechAct.state.statePlane, true
}

func (source VerifiedSpeechActSourceV2) DeltaPredicateRef() (DeltaPredicateRef, bool) {
	if !source.valid() {
		return DeltaPredicateRef{}, false
	}
	return source.state.basis.state.speechAct.state.deltaPredicate, true
}

func (source VerifiedSpeechActSourceV2) DescriptionRef() (DescriptionRef, bool) {
	if !source.valid() {
		return DescriptionRef{}, false
	}
	return source.state.anchors.state.descriptionRef, true
}

func (source VerifiedSpeechActSourceV2) DescriptionDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.basis.state.speechAct.state.reviewSubjectDigest, true
}

func (source VerifiedSpeechActSourceV2) DescriptionCarriers() ([]ObservableCarrierBinding, bool) {
	if !source.valid() {
		return nil, false
	}
	return append([]ObservableCarrierBinding{}, source.state.anchors.state.descriptionCarriers...), true
}

func (source VerifiedSpeechActSourceV2) TerminalCaptureRef() (CarrierRef, bool) {
	if !source.valid() {
		return CarrierRef{}, false
	}
	return source.state.basis.state.speechAct.state.captureCarrierRef, true
}

func (source VerifiedSpeechActSourceV2) TerminalCaptureDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.basis.state.speechAct.state.captureCarrierDigest, true
}

func (source VerifiedSpeechActSourceV2) ResourceLedgerRef() (ResourceLedgerRef, bool) {
	if !source.valid() {
		return ResourceLedgerRef{}, false
	}
	return source.state.anchors.state.resourceLedgerRef, true
}

func (source VerifiedSpeechActSourceV2) OutcomeRef() (WorkOutcomeRef, bool) {
	if !source.valid() {
		return WorkOutcomeRef{}, false
	}
	return source.state.basis.state.speechAct.state.outcome, true
}

func (source VerifiedSpeechActSourceV2) AcceptancePostureRef() (AcceptancePostureRef, bool) {
	if !source.valid() {
		return AcceptancePostureRef{}, false
	}
	return source.state.anchors.state.acceptancePosture, true
}

func (source VerifiedSpeechActSourceV2) AuditTraceRefs() ([]AuditTraceRef, bool) {
	if !source.valid() {
		return nil, false
	}
	return append([]AuditTraceRef{}, source.state.anchors.state.auditTraceRefs...), true
}

func (source VerifiedSpeechActSourceV2) ContextPolicy() (SpeechActContextPolicy, bool) {
	if !source.valid() {
		return SpeechActContextPolicy{}, false
	}
	return source.state.basis.state.intent.state.contextPolicy, true
}

func (source VerifiedSpeechActSourceV2) InstitutedObjectRef() (InstitutedObjectRef, bool) {
	if !source.valid() {
		return InstitutedObjectRef{}, false
	}
	return source.state.basis.state.speechAct.state.institutedObjectRef, true
}

func (assignment AuthorityRoleAssignment) Ref() (RoleAssignmentRef, bool) {
	if assignment.state == nil || !assignment.state.ref.valid() {
		return RoleAssignmentRef{}, false
	}
	return assignment.state.ref, true
}

func (assignment AuthorityRoleAssignment) Digest() (Digest, bool) {
	if assignment.state == nil || !assignment.state.digest.valid() {
		return Digest{}, false
	}
	return assignment.state.digest, true
}

func (assignment AuthorityRoleAssignment) HolderSystemRef() (SystemRef, bool) {
	if assignment.state == nil || !assignment.state.holderSystemRef.valid() {
		return SystemRef{}, false
	}
	return assignment.state.holderSystemRef, true
}

func (assignment AuthorityRoleAssignment) AdmittedHolderKind() (string, bool) {
	if assignment.state == nil || assignment.state.admittedHolderKind == "" {
		return "", false
	}
	return assignment.state.admittedHolderKind, true
}

func (assignment AuthorityRoleAssignment) RoleRef() (RoleRef, bool) {
	if assignment.state == nil {
		return RoleRef{}, false
	}
	role, err := NewRoleRef(assignment.state.roleRef)
	return role, err == nil
}

func (assignment AuthorityRoleAssignment) BoundedContext() (BoundedContextRef, bool) {
	if assignment.state == nil || !assignment.state.boundedContextRef.valid() {
		return BoundedContextRef{}, false
	}
	return assignment.state.boundedContextRef, true
}

func (assignment AuthorityRoleAssignment) AssignmentWindow() (TimeWindow, bool) {
	if assignment.state == nil || !assignment.state.assignmentWindow.valid() {
		return TimeWindow{}, false
	}
	return assignment.state.assignmentWindow, true
}

// DecodeVerifiedSpeechActSourceV2 strictly reconstructs canonical v2 material
// against the exact package-verified v1 source basis. It performs no I/O and
// cannot write or downgrade into the v1 speech_acts table.
func DecodeVerifiedSpeechActSourceV2(
	basis VerifiedSpeechActSource,
	canonicalJSON []byte,
	digest Digest,
) (VerifiedSpeechActSourceV2, error) {
	canonicalSizeValid := len(canonicalJSON) > 0 &&
		len(canonicalJSON) <= speechActSourceV2MaxCanonicalBytes
	if !basis.valid() || !canonicalSizeValid || !digest.valid() {
		return VerifiedSpeechActSourceV2{}, fmt.Errorf(
			"SpeechAct source v2 decode requires exact canonical material of at most %d bytes and a verified basis",
			speechActSourceV2MaxCanonicalBytes,
		)
	}
	projection := speechActSourceV2Projection{}
	decoder := json.NewDecoder(bytes.NewReader(canonicalJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&projection); err != nil {
		return VerifiedSpeechActSourceV2{}, fmt.Errorf("decode SpeechAct source v2: %w", err)
	}
	if err := requireSpeechActSourceV2JSONEOF(decoder); err != nil {
		return VerifiedSpeechActSourceV2{}, err
	}
	if err := validateSpeechActSourceV2ProjectionCollectionLimits(projection); err != nil {
		return VerifiedSpeechActSourceV2{}, err
	}
	anchors, err := parseSpeechActSourceV2Anchors(projection)
	if err != nil {
		return VerifiedSpeechActSourceV2{}, err
	}
	rebuilt, err := NewVerifiedSpeechActSourceV2(basis, anchors)
	if err != nil {
		return VerifiedSpeechActSourceV2{}, err
	}
	rebuiltJSON, _ := rebuilt.CanonicalJSON()
	rebuiltDigest, _ := rebuilt.Digest()
	exact := rebuiltDigest == digest && slices.Equal(rebuiltJSON, canonicalJSON)
	if !exact {
		return VerifiedSpeechActSourceV2{}, fmt.Errorf(
			"SpeechAct source v2 is not exact canonical material for its verified basis",
		)
	}
	return rebuilt, nil
}

func validateSpeechActSourceV2ProjectionCollectionLimits(
	projection speechActSourceV2Projection,
) error {
	withinLimits := len(projection.ActTypeRefs) <= 1 &&
		len(projection.Parameters) <= speechActSourceV2MaxParameterBindings &&
		len(projection.InputRefs) <= speechActSourceV2MaxInputRefs &&
		len(projection.OutputRefs) <= speechActSourceV2MaxOutputRefs &&
		len(projection.AffectedRefs) <= speechActSourceV2MaxAffectedRefs &&
		len(projection.AuditTraceRefs) <= speechActSourceV2MaxAuditTraceRefs &&
		len(projection.DescriptionCarriers) <= speechActSourceV2MaxDescriptionCarriers
	if withinLimits {
		return nil
	}
	return fmt.Errorf(
		"SpeechAct source v2 collections exceed reviewed decode limits",
	)
}

// DecodeRecordedSpeechActSourceV2 is the read-only compatibility seam for a
// v1 source already recovered by the strict v1 loader. It remains a pure
// decoder; no v2 persistence fallback is exposed.
func DecodeRecordedSpeechActSourceV2(
	basis RecordedSpeechActSource,
	canonicalJSON []byte,
	digest Digest,
) (VerifiedSpeechActSourceV2, error) {
	if !basis.Valid() {
		return VerifiedSpeechActSourceV2{}, fmt.Errorf(
			"SpeechAct source v2 decode requires an exact recorded v1 basis",
		)
	}
	verified := VerifiedSpeechActSource(basis)
	return DecodeVerifiedSpeechActSourceV2(verified, canonicalJSON, digest)
}

func requireSpeechActSourceV2JSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode trailing SpeechAct source v2 material: %w", err)
	}
	return fmt.Errorf("SpeechAct source v2 contains trailing JSON material")
}

func parseSpeechActSourceV2Anchors(
	projection speechActSourceV2Projection,
) (SpeechActSourceV2Anchors, error) {
	if projection.Schema != speechActSourceV2Schema {
		return SpeechActSourceV2Anchors{}, fmt.Errorf(
			"unsupported SpeechAct source schema %q",
			projection.Schema,
		)
	}
	workRef, err := NewWorkRef(projection.WorkRef)
	if err != nil {
		return SpeechActSourceV2Anchors{}, err
	}
	descriptionRef, err := newDescriptionRef(
		DescriptionRefKind(projection.Description.Kind),
		projection.Description.Ref,
	)
	if err != nil {
		return SpeechActSourceV2Anchors{}, err
	}
	resourceLedger, err := NewResourceLedgerRef(projection.ResourceLedgerRef)
	if err != nil {
		return SpeechActSourceV2Anchors{}, err
	}
	acceptance, err := NewAcceptancePostureRef(projection.AcceptancePostureRef)
	if err != nil {
		return SpeechActSourceV2Anchors{}, err
	}
	builder := NewSpeechActSourceV2AnchorsBuilder(workRef, descriptionRef)
	builder = builder.WithResourceLedger(resourceLedger)
	builder = builder.WithAcceptancePosture(acceptance)
	builder, err = addParsedAuditTraceRefs(builder, projection.AuditTraceRefs, 0)
	if err != nil {
		return SpeechActSourceV2Anchors{}, err
	}
	builder, err = addParsedObservableCarrierBindings(
		builder,
		projection.DescriptionCarriers,
		0,
	)
	if err != nil {
		return SpeechActSourceV2Anchors{}, err
	}
	return builder.Build()
}

func addParsedAuditTraceRefs(
	builder SpeechActSourceV2AnchorsBuilder,
	values []string,
	index int,
) (SpeechActSourceV2AnchorsBuilder, error) {
	if index >= len(values) {
		return builder, nil
	}
	ref, err := NewAuditTraceRef(values[index])
	if err != nil {
		return SpeechActSourceV2AnchorsBuilder{}, err
	}
	next := builder.WithAuditTrace(ref)
	return addParsedAuditTraceRefs(next, values, index+1)
}

func addParsedObservableCarrierBindings(
	builder SpeechActSourceV2AnchorsBuilder,
	values []observableCarrierBindingProjection,
	index int,
) (SpeechActSourceV2AnchorsBuilder, error) {
	if index >= len(values) {
		return builder, nil
	}
	ref, err := NewCarrierRef(values[index].Ref)
	if err != nil {
		return SpeechActSourceV2AnchorsBuilder{}, err
	}
	digest, err := NewDigest(values[index].Digest)
	if err != nil {
		return SpeechActSourceV2AnchorsBuilder{}, err
	}
	binding, err := NewObservableCarrierBinding(ref, digest)
	if err != nil {
		return SpeechActSourceV2AnchorsBuilder{}, err
	}
	next := builder.WithDescriptionCarrier(binding)
	return addParsedObservableCarrierBindings(next, values, index+1)
}
