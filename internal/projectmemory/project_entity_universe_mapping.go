package projectmemory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectEntityUniverseMappingSchemaV1  = "haft.project-entity-universe-mapping-manifest/v1"
	projectEntityUniverseMappingIDV1      = "haft.project-entity-universe"
	projectEntityUniverseMappingVersionV1 = "1.0.0"
	projectEntityUniverseAdapterVersionV1 = "haft-project-entity-universe/1.0.0"
)

type projectEntityUniverseMappingCanonicalV1 struct {
	SchemaVersion       string   `json:"schema_version"`
	ManifestID          string   `json:"manifest_id"`
	ManifestVersion     string   `json:"manifest_version"`
	AdapterVersion      string   `json:"adapter_version"`
	ObservableSchema    string   `json:"observable_schema"`
	RequiredCoordinates []string `json:"required_coordinates"`
	MembershipPosture   string   `json:"membership_posture"`
	TrustBoundary       string   `json:"trust_boundary"`
}

// ProjectEntityUniverseMappingManifestV1 names the exact store-owned adapter
// that turns one transaction-correlated persisted-entity universe into the
// observable input used by the project-entity C.3.2 evaluator. It is mapping
// policy identity, not a membership result or authority grant.
type ProjectEntityUniverseMappingManifestV1 struct {
	canonical []byte
	ref       recordmapping.MappingManifestRef
	adapter   recordmapping.AdapterVersion
}

func CurrentProjectEntityUniverseMappingManifestV1() (
	ProjectEntityUniverseMappingManifestV1,
	error,
) {
	canonical, err := json.Marshal(canonicalProjectEntityUniverseMappingV1())
	if err != nil {
		return ProjectEntityUniverseMappingManifestV1{}, fmt.Errorf(
			"encode project-entity universe mapping manifest: %w",
			err,
		)
	}
	return DecodeProjectEntityUniverseMappingManifestV1(canonical)
}

func DecodeProjectEntityUniverseMappingManifestV1(
	canonical []byte,
) (ProjectEntityUniverseMappingManifestV1, error) {
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	var encoded projectEntityUniverseMappingCanonicalV1
	if err := decoder.Decode(&encoded); err != nil {
		return ProjectEntityUniverseMappingManifestV1{}, fmt.Errorf(
			"decode project-entity universe mapping manifest: %w",
			err,
		)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ProjectEntityUniverseMappingManifestV1{}, fmt.Errorf(
			"project-entity universe mapping manifest has trailing content",
		)
	}
	expected, err := json.Marshal(canonicalProjectEntityUniverseMappingV1())
	if err != nil {
		return ProjectEntityUniverseMappingManifestV1{}, err
	}
	if !bytes.Equal(canonical, expected) {
		return ProjectEntityUniverseMappingManifestV1{}, fmt.Errorf(
			"project-entity universe mapping manifest is unsupported or not canonical",
		)
	}
	digestBytes := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(digestBytes[:]),
	)
	if err != nil {
		return ProjectEntityUniverseMappingManifestV1{}, err
	}
	ref, err := recordmapping.NewMappingManifestRef(
		projectEntityUniverseMappingIDV1,
		projectEntityUniverseMappingVersionV1,
		digest,
	)
	if err != nil {
		return ProjectEntityUniverseMappingManifestV1{}, err
	}
	adapter, err := recordmapping.NewAdapterVersion(
		projectEntityUniverseAdapterVersionV1,
	)
	if err != nil {
		return ProjectEntityUniverseMappingManifestV1{}, err
	}
	return ProjectEntityUniverseMappingManifestV1{
		canonical: append([]byte(nil), canonical...),
		ref:       ref,
		adapter:   adapter,
	}, nil
}

func (manifest ProjectEntityUniverseMappingManifestV1) Ref() recordmapping.MappingManifestRef {
	return manifest.ref
}

func (manifest ProjectEntityUniverseMappingManifestV1) AdapterVersion() recordmapping.AdapterVersion {
	return manifest.adapter
}

func (manifest ProjectEntityUniverseMappingManifestV1) CanonicalBytes() []byte {
	return append([]byte(nil), manifest.canonical...)
}

func (manifest ProjectEntityUniverseMappingManifestV1) Verify() error {
	verified, err := DecodeProjectEntityUniverseMappingManifestV1(
		manifest.canonical,
	)
	if err != nil {
		return err
	}
	if verified.ref != manifest.ref || verified.adapter != manifest.adapter {
		return fmt.Errorf(
			"project-entity universe mapping identity differs from canonical bytes",
		)
	}
	return nil
}

func canonicalProjectEntityUniverseMappingV1() projectEntityUniverseMappingCanonicalV1 {
	return projectEntityUniverseMappingCanonicalV1{
		SchemaVersion:    projectEntityUniverseMappingSchemaV1,
		ManifestID:       projectEntityUniverseMappingIDV1,
		ManifestVersion:  projectEntityUniverseMappingVersionV1,
		AdapterVersion:   projectEntityUniverseAdapterVersionV1,
		ObservableSchema: "haft.typed-memory.persisted-entity-universe/v1",
		RequiredCoordinates: []string{
			"exact_project_id",
			"exact_bounded_context_ref",
			"exact_pre_state_graph_revision",
			"canonical_sorted_entity_ids",
			"content_addressed_observable_ref_and_digest",
		},
		MembershipPosture: "entity_presence_is_candidate_basis_then_full_c3_2_prerequisites_apply",
		TrustBoundary:     "constructed_from_one_store_owned_snapshot_or_transaction_read_set_not_caller_bytes",
	}
}
