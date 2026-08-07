package projectprofile

import (
	"bytes"
	"fmt"
)

type profileOnboardingEffectResultJSONV1 struct {
	Kind                       string `json:"kind"`
	OutputRef                  string `json:"output_ref"`
	PayloadDigest              string `json:"payload_digest,omitempty"`
	ObservedProjectBasisRef    string `json:"observed_project_basis_ref,omitempty"`
	ObservedProjectBasisDigest string `json:"observed_project_basis_digest,omitempty"`
	MissingBasisDigest         string `json:"missing_basis_digest,omitempty"`
}

type profileOnboardingEffectJSONV1 struct {
	Schema                     string                              `json:"schema"`
	Ref                        string                              `json:"ref"`
	WorkRecordRef              string                              `json:"work_record_ref"`
	WorkRef                    string                              `json:"work_ref"`
	WorkRecordDigest           string                              `json:"work_record_digest"`
	Result                     profileOnboardingEffectResultJSONV1 `json:"result"`
	AffectedEntityRefs         []string                            `json:"affected_entity_of_concern_refs"`
	StatePlaneRef              string                              `json:"state_plane_ref"`
	StateWitness               workStateTransitionJSONV1           `json:"state_witness"`
	EvidenceProvenancePathRefs []string                            `json:"evidence_provenance_path_refs"`
}

type profileOnboardingAcceptanceVerdictJSONV1 struct {
	Kind               string `json:"kind"`
	ReasonRef          string `json:"reason_ref,omitempty"`
	MissingBasisDigest string `json:"missing_basis_digest,omitempty"`
}

type profileOnboardingOutcomeAssessmentJSONV1 struct {
	Schema                     string                                   `json:"schema"`
	Ref                        string                                   `json:"ref"`
	EffectRef                  string                                   `json:"effect_ref"`
	EffectDigest               string                                   `json:"effect_digest"`
	WorkRecordRef              string                                   `json:"work_record_ref"`
	WorkRef                    string                                   `json:"work_ref"`
	WorkRecordDigest           string                                   `json:"work_record_digest"`
	AcceptanceStandardRef      string                                   `json:"acceptance_standard_ref"`
	AcceptanceStandardEdition  string                                   `json:"acceptance_standard_edition"`
	ComparatorRef              string                                   `json:"comparator_ref"`
	ComparatorEdition          string                                   `json:"comparator_edition"`
	Verdict                    profileOnboardingAcceptanceVerdictJSONV1 `json:"verdict"`
	EvidenceProvenancePathRefs []string                                 `json:"evidence_provenance_path_refs"`
}

func EncodeProfileOnboardingEffectV1CanonicalJSON(
	value ProfileOnboardingEffectV1,
) ([]byte, error) {
	exact, err := exactProfileOnboardingEffectV1(value)
	if err != nil {
		return nil, err
	}
	dto, err := profileOnboardingEffectToJSONV1(exact)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileOnboardingEffectV1CanonicalJSON(
	data []byte,
) (ProfileOnboardingEffectV1, error) {
	var dto profileOnboardingEffectJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return nil, err
	}
	value, err := profileOnboardingEffectFromJSONV1(dto)
	if err != nil {
		return nil, err
	}
	canonical, err := EncodeProfileOnboardingEffectV1CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("ProfileOnboardingEffect JSON is not canonical")
	}
	return value, nil
}

func DigestProfileOnboardingEffectV1(
	value ProfileOnboardingEffectV1,
) (ContentDigest, error) {
	canonical, err := EncodeProfileOnboardingEffectV1CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	return digestProfileOnboardingEffectCanonicalJSONV1(canonical), nil
}

// DecodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON requires the exact
// effect it claims to assess. This prevents a standalone carrier from being
// decoded into a semantically valid assessment without resolving its bound
// effect first.
func DecodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(
	data []byte,
	effect ProfileOnboardingEffectV1,
) (ProfileOnboardingOutcomeAssessmentV1, error) {
	var dto profileOnboardingOutcomeAssessmentJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return nil, err
	}
	value, err := profileOnboardingOutcomeAssessmentFromJSONV1(dto, effect)
	if err != nil {
		return nil, err
	}
	canonical, err := EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("ProfileOnboardingOutcomeAssessment JSON is not canonical")
	}
	return value, nil
}

func EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(
	value ProfileOnboardingOutcomeAssessmentV1,
) ([]byte, error) {
	exact, err := exactProfileOnboardingOutcomeAssessmentV1(value)
	if err != nil {
		return nil, err
	}
	dto, err := profileOnboardingOutcomeAssessmentToJSONV1(exact)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func DigestProfileOnboardingOutcomeAssessmentV1(
	value ProfileOnboardingOutcomeAssessmentV1,
) (ContentDigest, error) {
	canonical, err := EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	return digestProfileOnboardingOutcomeAssessmentCanonicalJSONV1(canonical), nil
}

// ProfileOnboardingEffectJSONCarrierV1 is a carrier for the effect claim; it
// is not the Work occurrence, affected entity, evidence path, or effect.
type ProfileOnboardingEffectJSONCarrierV1 interface {
	Schema() string
	MediaType() string
	CanonicalJSON() []byte
	ContentDigest() ContentDigest
	profileOnboardingEffectJSONCarrierV1()
}

type profileOnboardingEffectJSONCarrierV1 struct {
	canonicalJSON []byte
	digest        ContentDigest
}

func (profileOnboardingEffectJSONCarrierV1) profileOnboardingEffectJSONCarrierV1() {}
func (profileOnboardingEffectJSONCarrierV1) Schema() string {
	return profileOnboardingEffectJSONSchemaV1
}
func (profileOnboardingEffectJSONCarrierV1) MediaType() string { return "application/json" }
func (carrier profileOnboardingEffectJSONCarrierV1) CanonicalJSON() []byte {
	return append([]byte{}, carrier.canonicalJSON...)
}
func (carrier profileOnboardingEffectJSONCarrierV1) ContentDigest() ContentDigest {
	return carrier.digest
}

func CarryProfileOnboardingEffectV1(
	value ProfileOnboardingEffectV1,
) (ProfileOnboardingEffectJSONCarrierV1, error) {
	canonical, err := EncodeProfileOnboardingEffectV1CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	digest := digestProfileOnboardingEffectCanonicalJSONV1(canonical)
	return profileOnboardingEffectJSONCarrierV1{
		canonicalJSON: append([]byte{}, canonical...),
		digest:        digest,
	}, nil
}

// ProfileOnboardingOutcomeAssessmentJSONCarrierV1 is a carrier for an
// assessment relation. It does not become the comparator, verdict evidence,
// effect, or Work by carrying their references.
type ProfileOnboardingOutcomeAssessmentJSONCarrierV1 interface {
	Schema() string
	MediaType() string
	CanonicalJSON() []byte
	ContentDigest() ContentDigest
	profileOnboardingOutcomeAssessmentJSONCarrierV1()
}

type profileOnboardingOutcomeAssessmentJSONCarrierV1 struct {
	canonicalJSON []byte
	digest        ContentDigest
}

func (profileOnboardingOutcomeAssessmentJSONCarrierV1) profileOnboardingOutcomeAssessmentJSONCarrierV1() {
}
func (profileOnboardingOutcomeAssessmentJSONCarrierV1) Schema() string {
	return profileOnboardingOutcomeAssessmentJSONSchemaV1
}
func (profileOnboardingOutcomeAssessmentJSONCarrierV1) MediaType() string {
	return "application/json"
}
func (carrier profileOnboardingOutcomeAssessmentJSONCarrierV1) CanonicalJSON() []byte {
	return append([]byte{}, carrier.canonicalJSON...)
}
func (carrier profileOnboardingOutcomeAssessmentJSONCarrierV1) ContentDigest() ContentDigest {
	return carrier.digest
}

func CarryProfileOnboardingOutcomeAssessmentV1(
	value ProfileOnboardingOutcomeAssessmentV1,
) (ProfileOnboardingOutcomeAssessmentJSONCarrierV1, error) {
	canonical, err := EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	digest := digestProfileOnboardingOutcomeAssessmentCanonicalJSONV1(canonical)
	return profileOnboardingOutcomeAssessmentJSONCarrierV1{
		canonicalJSON: append([]byte{}, canonical...),
		digest:        digest,
	}, nil
}

func profileOnboardingEffectToJSONV1(
	value profileOnboardingEffectV1,
) (profileOnboardingEffectJSONV1, error) {
	result, err := profileOnboardingEffectResultToJSONV1(value.result)
	if err != nil {
		return profileOnboardingEffectJSONV1{}, err
	}
	witness, err := workStateTransitionToJSONV1(value.stateWitness)
	if err != nil {
		return profileOnboardingEffectJSONV1{}, err
	}
	return profileOnboardingEffectJSONV1{
		Schema:                     profileOnboardingEffectJSONSchemaV1,
		Ref:                        value.ref.String(),
		WorkRecordRef:              value.workRecordRef.String(),
		WorkRef:                    value.workRef.String(),
		WorkRecordDigest:           value.workRecordDigest.String(),
		Result:                     result,
		AffectedEntityRefs:         entityOfConcernRefStringsV1(value.affectedEntityRefs),
		StatePlaneRef:              value.statePlaneRef.String(),
		StateWitness:               witness,
		EvidenceProvenancePathRefs: evidenceProvenancePathRefStringsV1(value.evidencePathRefs),
	}, nil
}

func profileOnboardingEffectFromJSONV1(
	dto profileOnboardingEffectJSONV1,
) (ProfileOnboardingEffectV1, error) {
	if dto.Schema != profileOnboardingEffectJSONSchemaV1 {
		return nil, fmt.Errorf("unsupported ProfileOnboardingEffect JSON schema %q", dto.Schema)
	}
	if dto.AffectedEntityRefs == nil || dto.EvidenceProvenancePathRefs == nil {
		return nil, fmt.Errorf("ProfileOnboardingEffect ref lists must be explicit arrays")
	}
	ref, err := NewProfileOnboardingEffectRefV1(dto.Ref)
	if err != nil {
		return nil, err
	}
	workRecordRef, err := NewProfileOnboardingWorkRecordRef(dto.WorkRecordRef)
	if err != nil {
		return nil, err
	}
	workRef, err := NewWorkRef(dto.WorkRef)
	if err != nil {
		return nil, err
	}
	workRecordDigest, err := NewContentDigest(dto.WorkRecordDigest)
	if err != nil {
		return nil, err
	}
	result, err := profileOnboardingEffectResultFromJSONV1(dto.Result)
	if err != nil {
		return nil, err
	}
	affectedRefs, err := refsFromStringsV1(dto.AffectedEntityRefs, NewEntityRef)
	if err != nil {
		return nil, err
	}
	statePlaneRef, err := NewStatePlaneRef(dto.StatePlaneRef)
	if err != nil {
		return nil, err
	}
	stateWitness, err := workStateTransitionFromJSONV1(dto.StateWitness)
	if err != nil {
		return nil, err
	}
	evidenceRefs, err := refsFromStringsV1(
		dto.EvidenceProvenancePathRefs,
		NewEvidenceProvenancePathRefV1,
	)
	if err != nil {
		return nil, err
	}
	return NewProfileOnboardingEffectV1(
		ref,
		workRecordRef,
		workRef,
		workRecordDigest,
		result,
		affectedRefs,
		statePlaneRef,
		stateWitness,
		evidenceRefs,
	)
}

func profileOnboardingEffectResultToJSONV1(
	value ProfileOnboardingEffectResultV1,
) (profileOnboardingEffectResultJSONV1, error) {
	switch exact := value.(type) {
	case profileOnboardingCandidateResultV1:
		return profileOnboardingEffectResultJSONV1{
			Kind:                       profileOnboardingCandidateResultKindV1Value,
			OutputRef:                  exact.outputRef.String(),
			PayloadDigest:              exact.payloadDigest.String(),
			ObservedProjectBasisRef:    exact.observedProjectBasisRef.String(),
			ObservedProjectBasisDigest: exact.observedProjectBasisDigest.String(),
		}, nil
	case profileOnboardingUnderdeterminedResultV1:
		return profileOnboardingEffectResultJSONV1{
			Kind:               profileOnboardingUnderdeterminedKindV1Value,
			OutputRef:          exact.outputRef.String(),
			MissingBasisDigest: exact.missingBasisDigest.String(),
		}, nil
	default:
		return profileOnboardingEffectResultJSONV1{}, fmt.Errorf("unknown ProfileOnboardingEffect result variant")
	}
}

func profileOnboardingEffectResultFromJSONV1(
	dto profileOnboardingEffectResultJSONV1,
) (ProfileOnboardingEffectResultV1, error) {
	outputRef, err := NewWorkOutputRef(dto.OutputRef)
	if err != nil {
		return nil, err
	}
	switch dto.Kind {
	case profileOnboardingCandidateResultKindV1Value:
		if dto.MissingBasisDigest != "" {
			return nil, fmt.Errorf("candidate effect result contains underdetermined fields")
		}
		payloadDigest, digestErr := NewContentDigest(dto.PayloadDigest)
		if digestErr != nil {
			return nil, digestErr
		}
		basisRef, refErr := NewObservedProjectBasisRefV1(dto.ObservedProjectBasisRef)
		if refErr != nil {
			return nil, refErr
		}
		basisDigest, basisDigestErr := NewContentDigest(dto.ObservedProjectBasisDigest)
		if basisDigestErr != nil {
			return nil, basisDigestErr
		}
		return NewProfileOnboardingCandidateResultV1(
			outputRef,
			payloadDigest,
			basisRef,
			basisDigest,
		)
	case profileOnboardingUnderdeterminedKindV1Value:
		if dto.PayloadDigest != "" || dto.ObservedProjectBasisRef != "" || dto.ObservedProjectBasisDigest != "" {
			return nil, fmt.Errorf("underdetermined effect result contains candidate fields")
		}
		missingDigest, digestErr := NewContentDigest(dto.MissingBasisDigest)
		if digestErr != nil {
			return nil, digestErr
		}
		return NewProfileOnboardingUnderdeterminedResultV1(outputRef, missingDigest)
	default:
		return nil, fmt.Errorf("unknown ProfileOnboardingEffect result kind %q", dto.Kind)
	}
}

func profileOnboardingOutcomeAssessmentToJSONV1(
	value profileOnboardingOutcomeAssessmentV1,
) (profileOnboardingOutcomeAssessmentJSONV1, error) {
	verdict, err := profileOnboardingAcceptanceVerdictToJSONV1(value.verdict)
	if err != nil {
		return profileOnboardingOutcomeAssessmentJSONV1{}, err
	}
	return profileOnboardingOutcomeAssessmentJSONV1{
		Schema:                     profileOnboardingOutcomeAssessmentJSONSchemaV1,
		Ref:                        value.ref.String(),
		EffectRef:                  value.effectRef.String(),
		EffectDigest:               value.effectDigest.String(),
		WorkRecordRef:              value.workRecordRef.String(),
		WorkRef:                    value.workRef.String(),
		WorkRecordDigest:           value.workRecordDigest.String(),
		AcceptanceStandardRef:      value.acceptanceStandardRef.String(),
		AcceptanceStandardEdition:  value.acceptanceStandardEdition.String(),
		ComparatorRef:              value.comparatorRef.String(),
		ComparatorEdition:          value.comparatorEdition.String(),
		Verdict:                    verdict,
		EvidenceProvenancePathRefs: evidenceProvenancePathRefStringsV1(value.evidencePathRefs),
	}, nil
}

func profileOnboardingOutcomeAssessmentFromJSONV1(
	dto profileOnboardingOutcomeAssessmentJSONV1,
	effect ProfileOnboardingEffectV1,
) (ProfileOnboardingOutcomeAssessmentV1, error) {
	if dto.Schema != profileOnboardingOutcomeAssessmentJSONSchemaV1 {
		return nil, fmt.Errorf("unsupported outcome-assessment JSON schema %q", dto.Schema)
	}
	if dto.EvidenceProvenancePathRefs == nil {
		return nil, fmt.Errorf("outcome-assessment evidence-provenance refs must be an explicit array")
	}
	exactEffect, err := exactProfileOnboardingEffectV1(effect)
	if err != nil {
		return nil, err
	}
	effectDigest, err := DigestProfileOnboardingEffectV1(exactEffect)
	if err != nil {
		return nil, err
	}
	err = validateOutcomeAssessmentJSONBindingV1(dto, exactEffect, effectDigest)
	if err != nil {
		return nil, err
	}
	ref, err := NewProfileOnboardingOutcomeAssessmentRefV1(dto.Ref)
	if err != nil {
		return nil, err
	}
	contract, err := ProfileOnboardingMethodContractV1Value()
	if err != nil {
		return nil, err
	}
	if dto.AcceptanceStandardRef != contract.AcceptanceStandardRef().String() {
		return nil, fmt.Errorf("outcome assessment references another acceptance standard")
	}
	acceptanceEdition, err := NewProfileOnboardingAcceptanceStandardEditionV1(
		dto.AcceptanceStandardEdition,
	)
	if err != nil {
		return nil, err
	}
	comparatorRef, err := NewProfileOnboardingComparatorRefV1(dto.ComparatorRef)
	if err != nil {
		return nil, err
	}
	comparatorEdition, err := NewProfileOnboardingComparatorEditionV1(dto.ComparatorEdition)
	if err != nil {
		return nil, err
	}
	verdict, err := profileOnboardingAcceptanceVerdictFromJSONV1(dto.Verdict)
	if err != nil {
		return nil, err
	}
	evidenceRefs, err := refsFromStringsV1(
		dto.EvidenceProvenancePathRefs,
		NewEvidenceProvenancePathRefV1,
	)
	if err != nil {
		return nil, err
	}
	return NewProfileOnboardingOutcomeAssessmentV1(
		ref,
		exactEffect,
		contract.AcceptanceStandardRef(),
		acceptanceEdition,
		comparatorRef,
		comparatorEdition,
		verdict,
		evidenceRefs,
	)
}

func profileOnboardingAcceptanceVerdictToJSONV1(
	value ProfileOnboardingAcceptanceVerdictV1,
) (profileOnboardingAcceptanceVerdictJSONV1, error) {
	switch exact := value.(type) {
	case profileOnboardingAcceptancePassedV1:
		return profileOnboardingAcceptanceVerdictJSONV1{Kind: "passed"}, nil
	case profileOnboardingAcceptanceFailedV1:
		return profileOnboardingAcceptanceVerdictJSONV1{
			Kind:      "failed",
			ReasonRef: exact.reasonRef.String(),
		}, nil
	case profileOnboardingAcceptanceUndeterminedV1:
		return profileOnboardingAcceptanceVerdictJSONV1{
			Kind:               "undetermined",
			MissingBasisDigest: exact.missingBasisDigest.String(),
		}, nil
	default:
		return profileOnboardingAcceptanceVerdictJSONV1{}, fmt.Errorf("unknown acceptance verdict variant")
	}
}

func profileOnboardingAcceptanceVerdictFromJSONV1(
	dto profileOnboardingAcceptanceVerdictJSONV1,
) (ProfileOnboardingAcceptanceVerdictV1, error) {
	switch dto.Kind {
	case "passed":
		if dto.ReasonRef != "" || dto.MissingBasisDigest != "" {
			return nil, fmt.Errorf("passed acceptance verdict contains failure fields")
		}
		return ProfileOnboardingAcceptancePassedV1Value(), nil
	case "failed":
		if dto.MissingBasisDigest != "" {
			return nil, fmt.Errorf("failed acceptance verdict contains undetermined fields")
		}
		reasonRef, err := NewProfileOnboardingAcceptanceReasonRefV1(dto.ReasonRef)
		if err != nil {
			return nil, err
		}
		return NewProfileOnboardingAcceptanceFailedV1(reasonRef)
	case "undetermined":
		if dto.ReasonRef != "" {
			return nil, fmt.Errorf("undetermined acceptance verdict contains failure fields")
		}
		missingDigest, err := NewContentDigest(dto.MissingBasisDigest)
		if err != nil {
			return nil, err
		}
		return NewProfileOnboardingAcceptanceUndeterminedV1(missingDigest)
	default:
		return nil, fmt.Errorf("unknown acceptance verdict kind %q", dto.Kind)
	}
}

func validateOutcomeAssessmentJSONBindingV1(
	dto profileOnboardingOutcomeAssessmentJSONV1,
	effect profileOnboardingEffectV1,
	effectDigest ContentDigest,
) error {
	if dto.EffectRef != effect.ref.String() || dto.EffectDigest != effectDigest.String() {
		return fmt.Errorf("outcome-assessment carrier does not bind the supplied effect")
	}
	if dto.WorkRecordRef != effect.workRecordRef.String() || dto.WorkRef != effect.workRef.String() {
		return fmt.Errorf("outcome-assessment carrier Work identity does not match effect")
	}
	if dto.WorkRecordDigest != effect.workRecordDigest.String() {
		return fmt.Errorf("outcome-assessment carrier Work digest does not match effect")
	}
	return nil
}

func digestProfileOnboardingEffectCanonicalJSONV1(canonical []byte) ContentDigest {
	writer := newCanonicalDigestWriter(profileOnboardingEffectDigestV1)
	writer.add(string(canonical))
	return writer.digest()
}

func digestProfileOnboardingOutcomeAssessmentCanonicalJSONV1(
	canonical []byte,
) ContentDigest {
	writer := newCanonicalDigestWriter(profileOnboardingOutcomeAssessmentDigestV1)
	writer.add(string(canonical))
	return writer.digest()
}
