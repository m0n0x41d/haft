// Package portfoliocomparisonadapter owns the sealed mapping coordinates for
// the PortfolioComparison task adapter. The manifest is not a TypeEnv,
// comparison policy, selection, DecisionRecord, or authority grant.
package portfoliocomparisonadapter

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
	MappingManifestIDV1      = "haft.portfolio-comparison"
	MappingManifestVersionV1 = "2.0.0"
	AdapterVersionV1         = "haft-portfolio-comparison-adapter/2.0.0"
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
	UnsettledPolicy        string   `json:"unsettled_policy"`
	RoundTripFixtures      []string `json:"round_trip_fixtures"`
}

type MappingManifestV1 struct {
	canonical []byte
	ref       recordmapping.MappingManifestRef
	adapter   recordmapping.AdapterVersion
}

func CurrentMappingManifestV1() (MappingManifestV1, error) {
	encoded := canonicalMappingManifestV1()
	canonical, err := json.Marshal(encoded)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"encode PortfolioComparison mapping manifest: %w",
			err,
		)
	}
	return decodeMappingManifestV1(canonical)
}

func DecodeMappingManifestV1(canonical []byte) (MappingManifestV1, error) {
	owned := append([]byte(nil), canonical...)
	return decodeMappingManifestV1(owned)
}

func decodeMappingManifestV1(canonical []byte) (MappingManifestV1, error) {
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	var encoded mappingManifestCanonicalV1
	if err := decoder.Decode(&encoded); err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"decode PortfolioComparison mapping manifest: %w",
			err,
		)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return MappingManifestV1{}, err
	}
	expected := canonicalMappingManifestV1()
	expectedBytes, err := json.Marshal(expected)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"encode expected PortfolioComparison mapping manifest: %w",
			err,
		)
	}
	if !bytes.Equal(canonical, expectedBytes) {
		return MappingManifestV1{}, fmt.Errorf(
			"PortfolioComparison mapping manifest is unsupported or not canonical",
		)
	}
	digest := sha256.Sum256(canonical)
	digestValue, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(digest[:]),
	)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive PortfolioComparison mapping manifest digest: %w",
			err,
		)
	}
	ref, err := recordmapping.NewMappingManifestRef(
		MappingManifestIDV1,
		MappingManifestVersionV1,
		digestValue,
	)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive PortfolioComparison mapping manifest reference: %w",
			err,
		)
	}
	adapter, err := recordmapping.NewAdapterVersion(AdapterVersionV1)
	if err != nil {
		return MappingManifestV1{}, fmt.Errorf(
			"derive PortfolioComparison adapter version: %w",
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
			"PortfolioComparison mapping manifest identity does not match canonical bytes",
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
			"exact_project_id",
			"new_generic_project_record_identity",
			"exact_selected_type_environment_and_codec_registry",
			"exact_context_slice",
			"exact_pre_resolved_entity_of_concern_reference",
			"exact_persisted_solution_portfolio_reference",
			"at_least_two_exact_persisted_compared_option_references",
			"non_empty_non_dominated_reference_subset",
			"closed_claim_graph_value",
		},
		CarrierVariant: "project_record",
		EmittedChanges: []string{
			"DeclareEntity(portfolio_comparison_record)",
			"AssertRelation(Haft.PortfolioComparison,affirms_obtaining)",
		},
		EmittedSignature: "Haft.PortfolioComparison",
		EmittedSlots: []string{
			"Haft.PortfolioComparison.ComparisonSlot=Haft.ProjectRecordRef",
			"Haft.PortfolioComparison.PortfolioSlot=Haft.ProjectRecordRef",
			"Haft.PortfolioComparison.EntityOfConcernSlot=U.EntityRef",
			"Haft.PortfolioComparison.ComparedOptionSlot=Haft.ProjectRecordRef{2..unbounded}",
			"Haft.PortfolioComparison.NonDominatedOptionSlot=Haft.ProjectRecordRef{1..unbounded}",
			"Haft.PortfolioComparison.ClaimGraphSlot=U.ClaimGraph@ByValue",
		},
		AuthorityLifecyclePath: "non_binding_candidate_then_canonical_admission_service",
		UnsettledPolicy:        "missing_or_drifted_mapping_or_concern_basis_returns_underdetermined_and_emits_zero_changes",
		RoundTripFixtures: []string{
			"valid_portfolio_comparison_at_exact_concern",
			"compared_option_permutation_identity",
			"non_dominated_subset_without_winner",
			"dominated_options_remain_addressable",
			"unresolved_concern_zero_changes",
			"mapping_drift_zero_changes",
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
			"decode trailing PortfolioComparison mapping manifest content: %w",
			err,
		)
	}
	return fmt.Errorf(
		"PortfolioComparison mapping manifest contains trailing JSON content",
	)
}
