// Package decisionrecordadapter projects an already-existing manually bound
// DecisionRecord into typed project memory. It never creates or supersedes the
// binding artifact.
package decisionrecordadapter

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
	MappingManifestIDV1      = "haft.decision-choice-at-concern"
	MappingManifestVersionV1 = "2.0.0"
	AdapterVersionV1         = "haft-decision-record-adapter/2.0.0"
)

type mappingManifestCanonicalV1 struct {
	SchemaVersion          string   `json:"schema_version"`
	ManifestID             string   `json:"manifest_id"`
	ManifestVersion        string   `json:"manifest_version"`
	AdapterVersion         string   `json:"adapter_version"`
	AcceptedInputShape     []string `json:"accepted_input_shape"`
	CarrierVariant         string   `json:"carrier_variant"`
	EmittedChanges         []string `json:"emitted_changes"`
	EmittedSignature       string   `json:"emitted_signature"`
	EmittedSlots           []string `json:"emitted_slots"`
	AuthorityLifecyclePath string   `json:"authority_lifecycle_path"`
	SemanticBoundary       []string `json:"semantic_boundary"`
	UnsettledPolicy        string   `json:"unsettled_policy"`
	RoundTripFixtures      []string `json:"round_trip_fixtures"`
}

type MappingManifestV1 struct {
	canonical []byte
	ref       recordmapping.MappingManifestRef
	adapter   recordmapping.AdapterVersion
}

func CurrentMappingManifestV1() (MappingManifestV1, error) {
	canonical, err := json.Marshal(canonicalMappingManifestV1())
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"encode DecisionRecord mapping manifest: %w",
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
	encoded := mappingManifestCanonicalV1{}
	if err := decoder.Decode(&encoded); err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"decode DecisionRecord mapping manifest: %w",
			err,
		)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return MappingManifestV1{}, err
	}
	expected, err := json.Marshal(canonicalMappingManifestV1())
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"encode expected DecisionRecord mapping manifest: %w",
			err,
		)
	}
	if !bytes.Equal(canonical, expected) {
		return MappingManifestV1{}, fmt.Errorf(
			"DecisionRecord mapping manifest is unsupported or not canonical",
		)
	}
	sum := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive DecisionRecord mapping manifest digest: %w",
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
			"derive DecisionRecord mapping manifest reference: %w",
			err,
		)
	}
	adapter, err := recordmapping.NewAdapterVersion(AdapterVersionV1)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive DecisionRecord adapter version: %w",
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
			"DecisionRecord mapping manifest identity does not match canonical bytes",
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
			"existing_active_or_refresh_due_decision_record_loaded_from_project_store",
			"stored_choice_result_with_choose_now_and_at_least_two_options",
			"explicit_source_digest_bound_legacy_artifact_context_to_typed_bounded_context_projection",
			"exact_pre_resolved_entity_of_concern_reference",
			"total_bijection_from_stored_option_labels_to_existing_project_record_references",
			"exact_optional_problem_portfolio_and_explicit_comparison_record_references",
		},
		CarrierVariant: "decision_record",
		EmittedChanges: []string{
			"DeclareEntity(decision_record_projection)",
			"AssertRelation(Haft.DecisionChoiceAtConcern,affirms_obtaining)",
		},
		EmittedSignature: "Haft.DecisionChoiceAtConcern",
		EmittedSlots: []string{
			"Haft.DecisionChoiceAtConcern.DecisionRecordSlot=Haft.DecisionRecordRef",
			"Haft.DecisionChoiceAtConcern.EntityOfConcernSlot=U.EntityRef",
			"Haft.DecisionChoiceAtConcern.ProblemRecordSlot=Haft.ProjectRecordRef{0..1}",
			"Haft.DecisionChoiceAtConcern.PortfolioRecordSlot=Haft.ProjectRecordRef{0..1}",
			"Haft.DecisionChoiceAtConcern.OptionSlot=Haft.ProjectRecordRef{2..unbounded}",
			"Haft.DecisionChoiceAtConcern.ChosenOptionSlot=Haft.ProjectRecordRef",
			"Haft.DecisionChoiceAtConcern.RejectedOptionSlot=Haft.ProjectRecordRef{1..unbounded}",
			"Haft.DecisionChoiceAtConcern.ComparisonRecordSlot=Haft.ProjectRecordRef{0..1}",
			"Haft.DecisionChoiceAtConcern.ClaimGraphSlot=U.ClaimGraph@ByValue",
		},
		AuthorityLifecyclePath: "manual_h_decide_institutes_decision_record_before_non_binding_projection_and_canonical_admission",
		SemanticBoundary: []string{
			"adapter_accepts_no_raw_choice_or_recommendation",
			"legacy_artifact_context_description_is_not_typed_bounded_context_identity",
			"generic_admit_cannot_create_supersede_or_reopen_a_decision_record",
			"chosen_and_rejected_sets_are_derived_from_the_stored_choice_result",
			"comparison_reference_does_not_select_a_winner",
			"projection_grants_no_work_commission_or_implementation_authority",
		},
		UnsettledPolicy: "missing_or_noncanonical_decision_source_mapping_concern_or_selected_type_environment_returns_underdetermined_or_invalid_and_emits_zero_changes",
		RoundTripFixtures: []string{
			"active_manual_decision_with_exact_option_partition",
			"option_permutation_identity",
			"recommendation_without_choice_rejected",
			"chosen_outside_option_set_rejected",
			"unresolved_concern_zero_changes",
			"adapter_emits_no_decision_or_authority_effect",
		},
	}
}

func requireJSONEnd(decoder *json.Decoder) error {
	trailing := json.RawMessage{}
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf(
			"decode trailing DecisionRecord mapping manifest content: %w",
			err,
		)
	}
	return fmt.Errorf(
		"DecisionRecord mapping manifest contains trailing JSON content",
	)
}
