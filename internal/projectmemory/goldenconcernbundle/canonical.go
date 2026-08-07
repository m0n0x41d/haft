package goldenconcernbundle

import (
	"encoding/json"
	"fmt"
)

type receiptCanonicalV1 struct {
	Disposition   string `json:"disposition"`
	EventRef      string `json:"event_ref"`
	CommitRef     string `json:"commit_ref"`
	GraphRevision uint64 `json:"graph_revision"`
	ResultDigest  string `json:"result_digest"`
}

type declarationCanonicalV1 struct {
	EntityID   string `json:"entity_id"`
	LocalRef   string `json:"batch_local_ref"`
	Context    string `json:"bounded_context_ref"`
	Label      string `json:"label"`
	Provenance string `json:"provenance_ref"`
}

type concernAdmissionCanonicalV1 struct {
	ProjectID        string                 `json:"project_id"`
	ReferenceKind    string                 `json:"reference_kind"`
	ReferenceID      string                 `json:"reference_id"`
	Declaration      declarationCanonicalV1 `json:"declaration"`
	CandidateDigest  string                 `json:"candidate_digest"`
	CandidateBytes   []byte                 `json:"candidate_bytes"`
	AdmissionReceipt receiptCanonicalV1     `json:"admission_receipt"`
}

type adapterAdmissionCanonicalV1 struct {
	ProjectID       string `json:"project_id"`
	MappingManifest string `json:"mapping_manifest_ref"`
	AdapterVersion  string `json:"adapter_version"`
	CandidateDigest string `json:"candidate_digest"`
	CandidateBytes  []byte `json:"candidate_bytes"`
	// relation_signatures is the sealed v1 compatibility key. Values are
	// current TypedRelationDeclarationFragment IDs, not complete FPF signatures.
	Signatures       []string                 `json:"relation_signatures"`
	Declarations     []declarationCanonicalV1 `json:"declarations"`
	AdmissionReceipt receiptCanonicalV1       `json:"admission_receipt"`
}

type itemCanonicalV1 struct {
	Role                  string `json:"role"`
	ReferenceKind         string `json:"reference_kind"`
	ReferenceID           string `json:"reference_id"`
	EntityID              string `json:"entity_id"`
	Label                 string `json:"label"`
	Provenance            string `json:"provenance_ref"`
	AdmissionEventRef     string `json:"admission_event_ref"`
	AdmittedGraphRevision uint64 `json:"admitted_graph_revision"`
	ObservedGraphRevision uint64 `json:"observed_graph_revision"`
	ObservedAt            string `json:"observed_at"`
	FreshnessPosture      string `json:"freshness_posture"`
}

type relationPathCanonicalV1 struct {
	Assertion         string `json:"assertion_id"`
	Signature         string `json:"signature_id"`
	Context           string `json:"bounded_context_ref"`
	Slot              string `json:"slot_id"`
	TargetRefKind     string `json:"target_reference_kind"`
	TargetReferenceID string `json:"target_reference_id"`
	Provenance        string `json:"provenance_ref"`
	AdmissionEventRef string `json:"admission_event_ref"`
}

type valueWitnessCanonicalV1 struct {
	Assertion         string `json:"assertion_id"`
	Signature         string `json:"signature_id"`
	Slot              string `json:"slot_id"`
	ValueKind         string `json:"value_kind_ref"`
	ValueShape        string `json:"value_shape_ref"`
	Codec             string `json:"codec_ref"`
	InputDigest       string `json:"candidate_input_digest"`
	AdmissionEventRef string `json:"admission_event_ref"`
}

type bundleCanonicalV1 struct {
	Schema                 string                        `json:"schema"`
	ProjectID              string                        `json:"project_id"`
	EntityOfConcernRef     string                        `json:"entity_of_concern_ref"`
	BoundedContextRef      string                        `json:"bounded_context_ref"`
	TypeEnvRef             string                        `json:"type_env_ref"`
	GraphRevision          uint64                        `json:"graph_revision"`
	ObservedAt             string                        `json:"observed_at"`
	ConcernAdmission       concernAdmissionCanonicalV1   `json:"concern_admission"`
	AdapterAdmissions      []adapterAdmissionCanonicalV1 `json:"adapter_admissions"`
	Items                  []itemCanonicalV1             `json:"items"`
	ExpectedRelationPaths  []relationPathCanonicalV1     `json:"expected_relation_paths"`
	ValueWitnesses         []valueWitnessCanonicalV1     `json:"candidate_value_witnesses"`
	InterpretationBoundary []string                      `json:"interpretation_boundary"`
}

func encodeBundleCanonical(bundle Bundle) ([]byte, error) {
	encoded := bundleCanonicalV1{
		Schema:                 SchemaV1,
		ProjectID:              bundle.project.String(),
		EntityOfConcernRef:     bundle.concern.reference.ReferenceID().String(),
		BoundedContextRef:      bundle.coordinate.context.String(),
		TypeEnvRef:             bundle.coordinate.typeEnv.String(),
		GraphRevision:          bundle.coordinate.revision.Value(),
		ObservedAt:             bundle.coordinate.observedAt.Format(canonicalTimeLayout),
		ConcernAdmission:       encodeConcernAdmission(bundle.concern),
		AdapterAdmissions:      encodeAdapterAdmissions(bundle.admissions),
		Items:                  encodeItems(bundle.items),
		ExpectedRelationPaths:  encodeRelationPaths(bundle.paths),
		ValueWitnesses:         encodeValueWitnesses(bundle.values),
		InterpretationBoundary: interpretationBoundaryV1(),
	}
	canonical, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf(
			"encode GoldenConcernBundle canonical bytes: %w",
			err,
		)
	}
	return canonical, nil
}

func interpretationBoundaryV1() []string {
	return []string{
		"bundle_is_read_only_acceptance_evidence_not_semantic_write",
		"canonical_order_is_not_causal_temporal_method_or_work_order",
		"relation_paths_are_exact_inclusion_witnesses_not_applicability_or_recommendation",
		"freshness_means_present_at_exact_snapshot_not_current_truth",
		"bundle_contains_no_capability_continuation_global_phase_or_next_action",
	}
}

func encodeConcernAdmission(
	admission ConcernAdmission,
) concernAdmissionCanonicalV1 {
	return concernAdmissionCanonicalV1{
		ProjectID:        admission.project.String(),
		ReferenceKind:    admission.reference.RefKind().String(),
		ReferenceID:      admission.reference.ReferenceID().String(),
		Declaration:      encodeDeclaration(admission.declaration),
		CandidateDigest:  admission.candidateDigest.String(),
		CandidateBytes:   append([]byte(nil), admission.canonicalChanges...),
		AdmissionReceipt: encodeReceipt(admission.receipt),
	}
}

func encodeAdapterAdmissions(
	admissions []AdapterAdmission,
) []adapterAdmissionCanonicalV1 {
	result := make([]adapterAdmissionCanonicalV1, 0, len(admissions))
	for _, admission := range admissions {
		signatures := make([]string, 0, len(admission.signatures))
		for _, signature := range admission.signatures {
			signatures = append(signatures, signature.String())
		}
		declarations := make(
			[]declarationCanonicalV1,
			0,
			len(admission.declarations),
		)
		for _, declaration := range admission.declarations {
			declarations = append(
				declarations,
				encodeDeclaration(declaration),
			)
		}
		result = append(result, adapterAdmissionCanonicalV1{
			ProjectID:       admission.project.String(),
			MappingManifest: admission.manifest.String(),
			AdapterVersion:  admission.adapter.String(),
			CandidateDigest: admission.candidateDigest.String(),
			CandidateBytes: append(
				[]byte(nil),
				admission.canonicalChanges...,
			),
			Signatures:       signatures,
			Declarations:     declarations,
			AdmissionReceipt: encodeReceipt(admission.receipt),
		})
	}
	return result
}

func encodeItems(items []BundleItem) []itemCanonicalV1 {
	result := make([]itemCanonicalV1, 0, len(items))
	for _, item := range items {
		result = append(result, itemCanonicalV1{
			Role:                  item.role.String(),
			ReferenceKind:         item.reference.RefKind().String(),
			ReferenceID:           item.reference.ReferenceID().String(),
			EntityID:              item.entity.String(),
			Label:                 item.label.String(),
			Provenance:            item.provenance.String(),
			AdmissionEventRef:     item.admissionEventRef,
			AdmittedGraphRevision: item.admittedRevision.Value(),
			ObservedGraphRevision: item.observedRevision.Value(),
			ObservedAt:            item.observedAt.Format(canonicalTimeLayout),
			FreshnessPosture:      freshnessPostureV1,
		})
	}
	return result
}

func encodeRelationPaths(
	paths []RelationPath,
) []relationPathCanonicalV1 {
	result := make([]relationPathCanonicalV1, 0, len(paths))
	for _, path := range paths {
		result = append(result, relationPathCanonicalV1{
			Assertion:         path.assertion.String(),
			Signature:         path.signature.String(),
			Context:           path.context.String(),
			Slot:              path.slot.String(),
			TargetRefKind:     path.target.RefKind().String(),
			TargetReferenceID: path.target.ReferenceID().String(),
			Provenance:        path.provenance.String(),
			AdmissionEventRef: path.admissionEventRef,
		})
	}
	return result
}

func encodeValueWitnesses(
	values []ValueWitness,
) []valueWitnessCanonicalV1 {
	result := make([]valueWitnessCanonicalV1, 0, len(values))
	for _, value := range values {
		result = append(result, valueWitnessCanonicalV1{
			Assertion:         value.assertion.String(),
			Signature:         value.signature.String(),
			Slot:              value.slot.String(),
			ValueKind:         value.valueKind.String(),
			ValueShape:        value.valueShape.String(),
			Codec:             value.codec.String(),
			InputDigest:       value.inputDigest.String(),
			AdmissionEventRef: value.admissionEventRef,
		})
	}
	return result
}

func encodeDeclaration(
	declaration DeclarationWitness,
) declarationCanonicalV1 {
	return declarationCanonicalV1{
		EntityID:   declaration.entity.String(),
		LocalRef:   declaration.localRef.String(),
		Context:    declaration.context.String(),
		Label:      declaration.label.String(),
		Provenance: declaration.provenance.String(),
	}
}

func encodeReceipt(receipt receiptWitness) receiptCanonicalV1 {
	return receiptCanonicalV1{
		Disposition:   string(receipt.disposition),
		EventRef:      receipt.eventRef,
		CommitRef:     receipt.commitRef,
		GraphRevision: receipt.revision.Value(),
		ResultDigest:  receipt.result.String(),
	}
}
