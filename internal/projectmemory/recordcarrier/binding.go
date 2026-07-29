package recordcarrier

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type entityRecordCarrierBindingCanonicalV1 struct {
	SchemaVersion        string `json:"schema_version"`
	ProjectID            string `json:"project_id"`
	EntityID             string `json:"entity_id"`
	BoundedContext       string `json:"bounded_context"`
	RecordVariant        string `json:"record_variant"`
	CarrierRef           string `json:"carrier_ref"`
	CarrierEdition       string `json:"carrier_edition"`
	CarrierDigest        string `json:"carrier_digest"`
	CarrierSchema        string `json:"carrier_schema_version"`
	MappingManifestRef   string `json:"mapping_manifest_ref"`
	RecordAdapterVersion string `json:"adapter_version"`
}

// EntityRecordCarrierBindingV1 binds one exact carrier observation to the
// project/entity/context in which it may be used. It repeats the closed carrier
// variant solely to detect cross-substitution; it never accepts or stores a
// caller-supplied KindID and carries no approval semantics.
type EntityRecordCarrierBindingV1 struct {
	ref             EntityRecordCarrierBindingRef
	digest          typedmemory.SHA256Digest
	project         projectidentity.ProjectID
	entity          typedmemory.EntityID
	context         typedmemory.BoundedContextRef
	variant         ProjectRecordCarrierVariantV1
	carrierRef      typedmemory.CarrierRef
	carrierEdition  typedmemory.CarrierEdition
	carrierDigest   typedmemory.SHA256Digest
	carrierSchema   string
	mappingManifest MappingManifestRef
	adapterVersion  AdapterVersion
	canonical       []byte
}

func SealEntityRecordCarrierBindingV1(
	project projectidentity.ProjectID,
	carrier ProjectRecordCarrierV1,
	mappingManifest MappingManifestRef,
	adapterVersion AdapterVersion,
) (EntityRecordCarrierBindingV1, error) {
	if err := validateProjectID(project); err != nil {
		return EntityRecordCarrierBindingV1{}, err
	}
	if !carrier.valid() {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("project-record carrier is invalid")
	}
	if err := mappingManifest.Verify(); err != nil {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("mapping-manifest reference is invalid")
	}
	if err := adapterVersion.Verify(); err != nil {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("record adapter version is invalid")
	}
	encoded := entityRecordCarrierBindingCanonicalV1{
		SchemaVersion:        EntityRecordCarrierBindingSchemaVersionV1,
		ProjectID:            project.String(),
		EntityID:             carrier.EntityID().String(),
		BoundedContext:       carrier.BoundedContext().String(),
		RecordVariant:        carrier.Variant().Token(),
		CarrierRef:           carrier.Ref().String(),
		CarrierEdition:       carrier.Edition().String(),
		CarrierDigest:        carrier.Digest().String(),
		CarrierSchema:        carrier.SchemaVersion(),
		MappingManifestRef:   mappingManifest.String(),
		RecordAdapterVersion: adapterVersion.String(),
	}
	canonical, err := encodeCanonicalJSON(encoded)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, err
	}
	return decodeEntityRecordCarrierBindingV1(canonical)
}

func DecodeEntityRecordCarrierBindingV1(
	canonical []byte,
) (EntityRecordCarrierBindingV1, error) {
	return decodeEntityRecordCarrierBindingV1(append([]byte(nil), canonical...))
}

func decodeEntityRecordCarrierBindingV1(
	canonical []byte,
) (EntityRecordCarrierBindingV1, error) {
	var encoded entityRecordCarrierBindingCanonicalV1
	if err := decodeStrictCanonicalJSON(canonical, &encoded); err != nil {
		return EntityRecordCarrierBindingV1{}, err
	}
	if encoded.SchemaVersion != EntityRecordCarrierBindingSchemaVersionV1 {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf(
			"unsupported entity-record carrier binding schema %q",
			encoded.SchemaVersion,
		)
	}
	project, err := projectidentity.ParseProjectID(encoded.ProjectID)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("binding project: %w", err)
	}
	entity, err := parseExactEntityID(encoded.EntityID)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("binding entity: %w", err)
	}
	context, err := parseExactBoundedContext(encoded.BoundedContext)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("binding context: %w", err)
	}
	variant, err := parseProjectRecordCarrierVariantV1(encoded.RecordVariant)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, err
	}
	carrierRef, err := parseExactCarrierRef(encoded.CarrierRef)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("binding carrier reference: %w", err)
	}
	carrierEdition, err := parseExactCarrierEdition(encoded.CarrierEdition)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("binding carrier edition: %w", err)
	}
	carrierDigest, err := parseExactDigest(encoded.CarrierDigest)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("binding carrier digest: %w", err)
	}
	expectedCarrierRef := "project-record-carrier:" + carrierDigest.String()
	if carrierRef.String() != expectedCarrierRef {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf(
			"binding carrier reference does not match its content digest",
		)
	}
	if encoded.CarrierSchema != ProjectRecordCarrierSchemaVersionV1 {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf(
			"unsupported bound project-record carrier schema %q",
			encoded.CarrierSchema,
		)
	}
	mappingManifest, err := ParseMappingManifestRef(encoded.MappingManifestRef)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, err
	}
	adapterVersion, err := NewAdapterVersion(encoded.RecordAdapterVersion)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, err
	}
	if err := requireCanonicalJSON(canonical, encoded); err != nil {
		return EntityRecordCarrierBindingV1{}, err
	}
	digest := digestCanonical(canonical)
	ref, err := ParseEntityRecordCarrierBindingRef(
		"entity-record-carrier-binding:" + digest.String(),
	)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, err
	}
	return EntityRecordCarrierBindingV1{
		ref:             ref,
		digest:          digest,
		project:         project,
		entity:          entity,
		context:         context,
		variant:         variant,
		carrierRef:      carrierRef,
		carrierEdition:  carrierEdition,
		carrierDigest:   carrierDigest,
		carrierSchema:   encoded.CarrierSchema,
		mappingManifest: mappingManifest,
		adapterVersion:  adapterVersion,
		canonical:       append([]byte(nil), canonical...),
	}, nil
}

func VerifyEntityRecordCarrierBindingV1(
	expected EntityRecordCarrierBindingRef,
	canonical []byte,
) (EntityRecordCarrierBindingV1, error) {
	if !expected.valid() {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("expected entity-record carrier binding reference is invalid")
	}
	binding, err := DecodeEntityRecordCarrierBindingV1(canonical)
	if err != nil {
		return EntityRecordCarrierBindingV1{}, err
	}
	if binding.ref != expected {
		return EntityRecordCarrierBindingV1{}, fmt.Errorf("entity-record carrier binding reference does not match canonical bytes")
	}
	return binding, nil
}

func (binding EntityRecordCarrierBindingV1) Ref() EntityRecordCarrierBindingRef {
	return binding.ref
}

func (binding EntityRecordCarrierBindingV1) Digest() typedmemory.SHA256Digest {
	return binding.digest
}

func (binding EntityRecordCarrierBindingV1) SchemaVersion() string {
	return EntityRecordCarrierBindingSchemaVersionV1
}

func (binding EntityRecordCarrierBindingV1) ProjectID() projectidentity.ProjectID {
	return binding.project
}

func (binding EntityRecordCarrierBindingV1) EntityID() typedmemory.EntityID {
	return binding.entity
}

func (binding EntityRecordCarrierBindingV1) BoundedContext() typedmemory.BoundedContextRef {
	return binding.context
}

func (binding EntityRecordCarrierBindingV1) RecordVariant() ProjectRecordCarrierVariantV1 {
	return binding.variant
}

func (binding EntityRecordCarrierBindingV1) CarrierRef() typedmemory.CarrierRef {
	return binding.carrierRef
}

func (binding EntityRecordCarrierBindingV1) CarrierEdition() typedmemory.CarrierEdition {
	return binding.carrierEdition
}

func (binding EntityRecordCarrierBindingV1) CarrierDigest() typedmemory.SHA256Digest {
	return binding.carrierDigest
}

func (binding EntityRecordCarrierBindingV1) CarrierSchemaVersion() string {
	return binding.carrierSchema
}

func (binding EntityRecordCarrierBindingV1) MappingManifestRef() MappingManifestRef {
	return binding.mappingManifest
}

func (binding EntityRecordCarrierBindingV1) AdapterVersion() AdapterVersion {
	return binding.adapterVersion
}

func (binding EntityRecordCarrierBindingV1) CanonicalBytes() []byte {
	return append([]byte(nil), binding.canonical...)
}

func (binding EntityRecordCarrierBindingV1) valid() bool {
	if len(binding.canonical) == 0 {
		return false
	}
	decoded, err := DecodeEntityRecordCarrierBindingV1(binding.canonical)
	if err != nil {
		return false
	}
	return decoded.ref == binding.ref &&
		decoded.digest == binding.digest &&
		decoded.project == binding.project &&
		decoded.entity == binding.entity &&
		decoded.context == binding.context &&
		sameProjectRecordCarrierVariantV1(decoded.variant, binding.variant) &&
		decoded.carrierRef == binding.carrierRef &&
		decoded.carrierEdition == binding.carrierEdition &&
		decoded.carrierDigest == binding.carrierDigest &&
		decoded.carrierSchema == binding.carrierSchema &&
		decoded.mappingManifest == binding.mappingManifest &&
		decoded.adapterVersion == binding.adapterVersion
}

func validateProjectID(project projectidentity.ProjectID) error {
	parsed, err := projectidentity.ParseProjectID(project.String())
	if err != nil || parsed != project {
		return fmt.Errorf("record carrier project ID is invalid")
	}
	return nil
}
