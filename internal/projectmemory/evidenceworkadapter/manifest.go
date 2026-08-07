package evidenceworkadapter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	MappingManifestSchemaV1  = "haft.task-adapter-mapping-manifest/v1"
	MappingManifestIDV1      = "haft.evidence-work"
	MappingManifestVersionV1 = "2.0.0"
	AdapterVersionV1         = "haft-evidence-work-adapter/2.0.0"
)

type mappingManifestCanonicalV1 struct {
	SchemaVersion      string   `json:"schema_version"`
	ManifestID         string   `json:"manifest_id"`
	ManifestVersion    string   `json:"manifest_version"`
	AdapterVersion     string   `json:"adapter_version"`
	AcceptedInputShape []string `json:"accepted_input_shape"`
	RecordVariants     []string `json:"record_variants"`
	CarrierFamilies    []string `json:"carrier_families"`
	EmittedChanges     []string `json:"emitted_changes"`
	EmittedSignatures  []string `json:"emitted_signatures"`
	SemanticBoundary   []string `json:"semantic_boundary"`
	UnsettledPolicy    string   `json:"unsettled_policy"`
	RoundTripFixtures  []string `json:"round_trip_fixtures"`
}

// MappingManifestV1 owns the local Evidence/Work task mapping. It does not
// classify any record as exact FPF Evidence or any occurrence as U.Work.
type MappingManifestV1 struct {
	canonical []byte
	ref       recordmapping.MappingManifestRef
	adapter   recordmapping.AdapterVersion
}

func CurrentMappingManifestV1() (MappingManifestV1, error) {
	canonical, err := json.Marshal(canonicalMappingManifestV1())
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"encode Evidence/Work mapping manifest: %w",
			err,
		)
	}
	return decodeMappingManifestV1(canonical)
}

func DecodeMappingManifestV1(
	canonical []byte,
) (MappingManifestV1, error) {
	return decodeMappingManifestV1(append([]byte(nil), canonical...))
}

func decodeMappingManifestV1(
	canonical []byte,
) (MappingManifestV1, error) {
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	var encoded mappingManifestCanonicalV1
	if err := decoder.Decode(&encoded); err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"decode Evidence/Work mapping manifest: %w",
			err,
		)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return MappingManifestV1{}, err
	}
	expected, err := json.Marshal(canonicalMappingManifestV1())
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"encode expected Evidence/Work mapping manifest: %w",
			err,
		)
	}
	if !bytes.Equal(canonical, expected) {
		return MappingManifestV1{}, fmt.Errorf(
			"Evidence/Work mapping manifest is unsupported or not canonical",
		)
	}
	sum := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive Evidence/Work mapping manifest digest: %w",
			err,
		)
	}
	ref, err := recordmapping.NewMappingManifestRef(
		MappingManifestIDV1,
		MappingManifestVersionV1,
		digest,
	)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive Evidence/Work mapping manifest reference: %w",
			err,
		)
	}
	adapter, err := recordmapping.NewAdapterVersion(AdapterVersionV1)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive Evidence/Work adapter version: %w",
			err,
		)
	}
	return MappingManifestV1{
		canonical: append([]byte(nil), canonical...),
		ref:       ref,
		adapter:   adapter,
	}, nil
}

func (manifest MappingManifestV1) Ref() recordmapping.MappingManifestRef {
	return manifest.ref
}

func (manifest MappingManifestV1) AdapterVersion() recordmapping.AdapterVersion {
	return manifest.adapter
}

func (manifest MappingManifestV1) CanonicalBytes() []byte {
	return append([]byte(nil), manifest.canonical...)
}

func (manifest MappingManifestV1) Verify() error {
	decoded, err := DecodeMappingManifestV1(manifest.canonical)
	if err != nil {
		return err
	}
	if decoded.ref != manifest.ref || decoded.adapter != manifest.adapter {
		return fmt.Errorf(
			"Evidence/Work mapping manifest identity does not match canonical bytes",
		)
	}
	return nil
}

func canonicalMappingManifestV1() mappingManifestCanonicalV1 {
	return mappingManifestCanonicalV1{
		SchemaVersion:   MappingManifestSchemaV1,
		ManifestID:      MappingManifestIDV1,
		ManifestVersion: MappingManifestVersionV1,
		AdapterVersion:  AdapterVersionV1,
		AcceptedInputShape: []string{
			"exact_project_id_selected_type_environment_and_context_slice",
			"new_evidence_supporting_episteme_and_work_record_identities",
			"new_performed_work_occurrence_identity",
			"exact_pre_resolved_concern_performer_claim_and_carrier_edition_references",
			"closed_evidence_use_qualifier_and_performed_interval_values",
			"canonical_supporting_and_work_claim_graphs",
		},
		RecordVariants: []string{
			"evidence_record",
			"supporting_episteme_record",
			"work_record",
		},
		CarrierFamilies: []string{
			"project_record",
			"performed_work_occurrence",
		},
		EmittedChanges: []string{
			"DeclareEntity(evidence_record)",
			"DeclareEntity(supporting_episteme_record)",
			"DeclareEntity(work_record)",
			"DeclareEntity(performed_work_occurrence)",
			"AssertRelation(Haft.SupportingEpistemeRecordAtConcern,affirms_obtaining)",
			"AssertRelation(Haft.WorkOccurrenceRecord,affirms_obtaining)",
			"AssertRelation(Haft.EvidenceUse,affirms_obtaining)",
		},
		EmittedSignatures: []string{
			"Haft.SupportingEpistemeRecordAtConcern",
			"Haft.WorkOccurrenceRecord",
			"Haft.EvidenceUse",
		},
		SemanticBoundary: []string{
			"local_evidence_use_is_claim_bound_but_is_not_exact_fpf_evidence_by_label",
			"local_performed_work_occurrence_is_not_exact_u_work_by_label",
			"work_plan_log_file_test_output_and_telemetry_are_not_silently_upgraded_to_performed_work",
			"carrier_record_occurrence_claim_provenance_and_interpretation_remain_distinct",
			"adapter_emits_no_truth_authority_decision_or_spec_lifecycle_effect",
		},
		UnsettledPolicy: "missing_observation_classification_reference_signature_codec_or_registered_mapping_returns_underdetermined_and_emits_zero_changes",
		RoundTripFixtures: []string{
			"claim_bound_evidence_use_with_completed_work_occurrence",
			"in_flight_occurrence_without_completion_claim",
			"missing_project_claim_zero_changes",
			"work_plan_input_rejected_as_performed_occurrence",
			"unregistered_mapping_zero_changes",
			"no_exact_fpf_evidence_or_u_work_kind_claim",
		},
	}
}

func requireJSONEnd(decoder *json.Decoder) error {
	var trailing json.RawMessage
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf(
			"decode trailing Evidence/Work mapping manifest content: %w",
			err,
		)
	}
	return fmt.Errorf(
		"Evidence/Work mapping manifest contains trailing JSON content",
	)
}
