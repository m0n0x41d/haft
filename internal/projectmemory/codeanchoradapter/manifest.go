package codeanchoradapter

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
	MappingManifestIDV1      = "haft.code-anchor"
	MappingManifestVersionV1 = "2.0.0"
	AdapterVersionV1         = "haft-code-anchor-adapter/2.0.0"
)

type mappingManifestCanonicalV1 struct {
	SchemaVersion      string   `json:"schema_version"`
	ManifestID         string   `json:"manifest_id"`
	ManifestVersion    string   `json:"manifest_version"`
	AdapterVersion     string   `json:"adapter_version"`
	AcceptedInputShape []string `json:"accepted_input_shape"`
	CarrierFamily      string   `json:"carrier_family"`
	EmittedChanges     []string `json:"emitted_changes"`
	EmittedSignatures  []string `json:"emitted_signatures"`
	SemanticBoundary   []string `json:"semantic_boundary"`
	UnsettledPolicy    string   `json:"unsettled_policy"`
	RoundTripFixtures  []string `json:"round_trip_fixtures"`
}

// MappingManifestV1 owns the exact task mapping for CodeAnchorDefinition and
// explicit semantic links. It is neither code discovery nor evidence that the
// linked claim is realized or that the linked work occurred.
type MappingManifestV1 struct {
	canonical []byte
	ref       recordmapping.MappingManifestRef
	adapter   recordmapping.AdapterVersion
}

func CurrentMappingManifestV1() (MappingManifestV1, error) {
	canonical, err := json.Marshal(canonicalMappingManifestV1())
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"encode CodeAnchor mapping manifest: %w",
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
			"decode CodeAnchor mapping manifest: %w",
			err,
		)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return MappingManifestV1{}, err
	}
	expected, err := json.Marshal(canonicalMappingManifestV1())
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"encode expected CodeAnchor mapping manifest: %w",
			err,
		)
	}
	if !bytes.Equal(canonical, expected) {
		return MappingManifestV1{}, fmt.Errorf(
			"CodeAnchor mapping manifest is unsupported or not canonical",
		)
	}
	sum := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive CodeAnchor mapping manifest digest: %w",
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
			"derive CodeAnchor mapping manifest reference: %w",
			err,
		)
	}
	adapter, err := recordmapping.NewAdapterVersion(AdapterVersionV1)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive CodeAnchor adapter version: %w",
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
			"CodeAnchor mapping manifest identity does not match canonical bytes",
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
			"exact_project_id_and_selected_type_environment",
			"new_code_anchor_identity",
			"exact_repository_revision_and_file_or_symbol_locator",
			"one_or_more_explicit_pre_resolved_claim_or_performed_work_links",
			"exact_context_slice_and_provenance",
		},
		CarrierFamily: "code_anchor",
		EmittedChanges: []string{
			"DeclareEntity(code_anchor)",
			"AssertRelation(Haft.CodeAnchorDefinition,affirms_obtaining)",
			"AssertRelation(Haft.CodeRealizesClaim,affirms_obtaining)[0..unbounded]",
			"AssertRelation(Haft.CodeChangedByWork,affirms_obtaining)[0..unbounded]",
		},
		EmittedSignatures: []string{
			"Haft.CodeAnchorDefinition",
			"Haft.CodeRealizesClaim",
			"Haft.CodeChangedByWork",
		},
		SemanticBoundary: []string{
			"code_anchor_is_a_haft_local_entity_not_an_fpf_core_kind_by_label",
			"locator_structure_does_not_prove_repository_or_revision_truth",
			"claim_and_work_links_are_explicit_assertions_not_proximity_inference",
			"affected_files_backlinks_and_search_rank_never_create_a_semantic_link",
			"adapter_emits_no_decision_evidence_work_or_spec_lifecycle_effect",
		},
		UnsettledPolicy: "missing_locator_target_resolution_signature_codec_or_registered_mapping_returns_underdetermined_and_emits_zero_changes",
		RoundTripFixtures: []string{
			"file_anchor_with_explicit_claim_link",
			"symbol_anchor_with_explicit_work_link",
			"claim_and_work_link_permutation_identity",
			"unresolved_semantic_target_zero_changes",
			"unregistered_mapping_zero_changes",
			"no_inferred_link_from_locator_or_affected_files",
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
			"decode trailing CodeAnchor mapping manifest content: %w",
			err,
		)
	}
	return fmt.Errorf(
		"CodeAnchor mapping manifest contains trailing JSON content",
	)
}
