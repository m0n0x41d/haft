package carrierfamily

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	mappingManifestSchemaV1  = "haft.carrier-family-mapping-manifest/v1"
	mappingManifestVersionV1 = "1.0.0"
)

type mappingManifestCanonicalV1 struct {
	SchemaVersion     string   `json:"schema_version"`
	ManifestID        string   `json:"manifest_id"`
	ManifestVersion   string   `json:"manifest_version"`
	AdapterVersion    string   `json:"adapter_version"`
	Family            string   `json:"family"`
	EvaluatorRule     string   `json:"evaluator_rule"`
	AcceptedInput     []string `json:"accepted_input_shape"`
	EmittedCarrier    string   `json:"emitted_carrier"`
	UnsettledPolicy   string   `json:"unsettled_policy"`
	RoundTripFixtures []string `json:"round_trip_fixtures"`
}

// MappingManifestV1 is one package-owned producer contract. It describes how
// an exact source payload is wrapped as a positive classification carrier; it
// does not itself implement an adapter, persist a source, or establish
// membership.
type MappingManifestV1 struct {
	family    familyV1
	canonical []byte
	ref       recordmapping.MappingManifestRef
	adapter   recordmapping.AdapterVersion
}

func CurrentCarrierEditionMappingManifestV1() (MappingManifestV1, error) {
	return currentMappingManifestV1(carrierEditionFamilyV1)
}

func CurrentProjectClaimMappingManifestV1() (MappingManifestV1, error) {
	return currentMappingManifestV1(projectClaimFamilyV1)
}

func CurrentPerformedWorkOccurrenceMappingManifestV1() (MappingManifestV1, error) {
	return currentMappingManifestV1(performedWorkOccurrenceFamilyV1)
}

func CurrentCodeAnchorMappingManifestV1() (MappingManifestV1, error) {
	return currentMappingManifestV1(codeAnchorFamilyV1)
}

func currentMappingManifestV1(family familyV1) (MappingManifestV1, error) {
	canonical, err := encodeCanonical(canonicalMappingManifestV1(family))
	if err != nil {
		return MappingManifestV1{}, err
	}
	return decodeMappingManifestV1(canonical)
}

func DecodeMappingManifestV1(canonical []byte) (MappingManifestV1, error) {
	return decodeMappingManifestV1(append([]byte(nil), canonical...))
}

func decodeMappingManifestV1(canonical []byte) (MappingManifestV1, error) {
	var encoded mappingManifestCanonicalV1
	if err := decodeCanonical(canonical, &encoded); err != nil {
		return MappingManifestV1{}, err
	}
	family, err := parseFamilyV1(encoded.Family)
	if err != nil {
		return MappingManifestV1{}, err
	}
	expected := canonicalMappingManifestV1(family)
	expectedBytes, err := encodeCanonical(expected)
	if err != nil {
		return MappingManifestV1{}, err
	}
	if !bytes.Equal(expectedBytes, canonical) || encoded.SchemaVersion != mappingManifestSchemaV1 {
		return MappingManifestV1{}, fmt.Errorf("carrier-family mapping manifest is unsupported or noncanonical")
	}
	digest, err := digestCanonical(canonical)
	if err != nil {
		return MappingManifestV1{}, err
	}
	ref, err := recordmapping.NewMappingManifestRef(
		expected.ManifestID,
		expected.ManifestVersion,
		digest,
	)
	if err != nil {
		return MappingManifestV1{}, err
	}
	adapter, err := recordmapping.NewAdapterVersion(expected.AdapterVersion)
	if err != nil {
		return MappingManifestV1{}, err
	}
	return MappingManifestV1{
		family:    family,
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

func (manifest MappingManifestV1) EvaluatorRule() typedmemory.RuleRef {
	rule, _ := manifest.family.rule()
	return rule
}

func (manifest MappingManifestV1) CanonicalBytes() []byte {
	return append([]byte(nil), manifest.canonical...)
}

func (manifest MappingManifestV1) Verify() error {
	decoded, err := DecodeMappingManifestV1(manifest.canonical)
	if err != nil {
		return err
	}
	if decoded.family != manifest.family ||
		decoded.ref != manifest.ref ||
		decoded.adapter != manifest.adapter {
		return fmt.Errorf("carrier-family mapping manifest identity differs from canonical bytes")
	}
	return nil
}

func canonicalMappingManifestV1(family familyV1) mappingManifestCanonicalV1 {
	return mappingManifestCanonicalV1{
		SchemaVersion:   mappingManifestSchemaV1,
		ManifestID:      "haft.carrier-family." + family.token(),
		ManifestVersion: mappingManifestVersionV1,
		AdapterVersion:  "haft-" + family.token() + "-source-adapter/1.0.0",
		Family:          family.token(),
		EvaluatorRule:   family.ruleRaw(),
		AcceptedInput: []string{
			"exact_project_entity_and_bounded_context",
			"exact_source_carrier_ref_edition_digest_and_schema",
			"canonical_source_payload_bytes_matching_digest",
		},
		EmittedCarrier:  "haft.carrier-family-classification/v1",
		UnsettledPolicy: "missing_malformed_unregistered_or_ambiguous_source_yields_member_of_undefined",
		RoundTripFixtures: []string{
			"exact_positive_source",
			"payload_digest_substitution_rejected",
			"cross_family_substitution_rejected",
			"unregistered_mapping_rejected",
		},
	}
}
