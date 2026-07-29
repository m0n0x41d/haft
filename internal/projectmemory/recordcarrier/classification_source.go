package recordcarrier

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type recordClassificationSourceCanonicalV1 struct {
	SchemaVersion  string `json:"schema_version"`
	ProjectID      string `json:"project_id"`
	EntityID       string `json:"entity_id"`
	BoundedContext string `json:"bounded_context"`
	RecordVariant  string `json:"record_variant"`
	CarrierBytes   []byte `json:"carrier_bytes"`
	BindingBytes   []byte `json:"binding_bytes"`
}

// RecordClassificationSourceV1 is the current immutable delivery carrier for
// direct candidate features extracted from one exact project-record carrier.
// It is neither Evidence nor a classification judgement. Its byte domain is
// deliberately disjoint from the historical RecordMembershipSourceV1 domain.
type RecordClassificationSourceV1 struct {
	project   projectidentity.ProjectID
	entity    typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	variant   ProjectRecordCarrierVariantV1
	carrier   ProjectRecordCarrierV1
	binding   EntityRecordCarrierBindingV1
	reference typedmemory.CarrierRef
	canonical []byte
	digest    typedmemory.SHA256Digest
}

func SealRecordClassificationSourceV1(
	expectedProject projectidentity.ProjectID,
	expectedEntity typedmemory.EntityID,
	expectedContext typedmemory.BoundedContextRef,
	carrier ProjectRecordCarrierV1,
	binding EntityRecordCarrierBindingV1,
) (RecordClassificationSourceV1, error) {
	if err := validateSourceCorrelation(
		expectedProject,
		expectedEntity,
		expectedContext,
		carrier,
		binding,
	); err != nil {
		return RecordClassificationSourceV1{}, err
	}
	encoded := recordClassificationSourceCanonicalV1{
		SchemaVersion:  RecordClassificationSourceSchemaVersionV1,
		ProjectID:      expectedProject.String(),
		EntityID:       expectedEntity.String(),
		BoundedContext: expectedContext.String(),
		RecordVariant:  carrier.Variant().Token(),
		CarrierBytes:   carrier.CanonicalBytes(),
		BindingBytes:   binding.CanonicalBytes(),
	}
	canonical, err := encodeCanonicalJSON(encoded)
	if err != nil {
		return RecordClassificationSourceV1{}, err
	}
	return decodeRecordClassificationSourceV1(canonical)
}

func DecodeRecordClassificationSourceV1(
	canonical []byte,
) (RecordClassificationSourceV1, error) {
	return decodeRecordClassificationSourceV1(append([]byte(nil), canonical...))
}

func decodeRecordClassificationSourceV1(
	canonical []byte,
) (RecordClassificationSourceV1, error) {
	var encoded recordClassificationSourceCanonicalV1
	if err := decodeStrictCanonicalJSON(canonical, &encoded); err != nil {
		return RecordClassificationSourceV1{}, err
	}
	if encoded.SchemaVersion != RecordClassificationSourceSchemaVersionV1 {
		return RecordClassificationSourceV1{}, fmt.Errorf(
			"unsupported record classification source schema %q",
			encoded.SchemaVersion,
		)
	}
	project, err := projectidentity.ParseProjectID(encoded.ProjectID)
	if err != nil {
		return RecordClassificationSourceV1{}, fmt.Errorf(
			"classification source project: %w",
			err,
		)
	}
	entity, err := parseExactEntityID(encoded.EntityID)
	if err != nil {
		return RecordClassificationSourceV1{}, fmt.Errorf(
			"classification source entity: %w",
			err,
		)
	}
	context, err := parseExactBoundedContext(encoded.BoundedContext)
	if err != nil {
		return RecordClassificationSourceV1{}, fmt.Errorf(
			"classification source context: %w",
			err,
		)
	}
	variant, err := parseProjectRecordCarrierVariantV1(encoded.RecordVariant)
	if err != nil {
		return RecordClassificationSourceV1{}, err
	}
	carrier, err := DecodeProjectRecordCarrierV1(encoded.CarrierBytes)
	if err != nil {
		return RecordClassificationSourceV1{}, fmt.Errorf(
			"classification source carrier: %w",
			err,
		)
	}
	binding, err := DecodeEntityRecordCarrierBindingV1(encoded.BindingBytes)
	if err != nil {
		return RecordClassificationSourceV1{}, fmt.Errorf(
			"classification source binding: %w",
			err,
		)
	}
	if err := validateSourceCorrelation(
		project,
		entity,
		context,
		carrier,
		binding,
	); err != nil {
		return RecordClassificationSourceV1{}, err
	}
	if !sameProjectRecordCarrierVariantV1(variant, carrier.Variant()) {
		return RecordClassificationSourceV1{}, fmt.Errorf(
			"record classification source outer variant mismatch",
		)
	}
	if err := requireCanonicalJSON(canonical, encoded); err != nil {
		return RecordClassificationSourceV1{}, err
	}
	digest := digestCanonical(canonical)
	reference, err := typedmemory.NewCarrierRef(
		"record-classification-source:" + digest.String(),
	)
	if err != nil {
		return RecordClassificationSourceV1{}, fmt.Errorf(
			"derive record classification source reference: %w",
			err,
		)
	}
	return RecordClassificationSourceV1{
		project:   project,
		entity:    entity,
		context:   context,
		variant:   variant,
		carrier:   carrier,
		binding:   binding,
		reference: reference,
		canonical: append([]byte(nil), canonical...),
		digest:    digest,
	}, nil
}

func VerifyRecordClassificationSourceV1(
	expectedReference typedmemory.CarrierRef,
	expectedDigest typedmemory.SHA256Digest,
	canonical []byte,
) (RecordClassificationSourceV1, error) {
	source, err := DecodeRecordClassificationSourceV1(canonical)
	if err != nil {
		return RecordClassificationSourceV1{}, err
	}
	if source.reference != expectedReference || source.digest != expectedDigest {
		return RecordClassificationSourceV1{}, fmt.Errorf(
			"record classification source coordinate does not match canonical bytes",
		)
	}
	return source, nil
}

func (source RecordClassificationSourceV1) SchemaVersion() string {
	return RecordClassificationSourceSchemaVersionV1
}

func (source RecordClassificationSourceV1) ProjectID() projectidentity.ProjectID {
	return source.project
}

func (source RecordClassificationSourceV1) EntityID() typedmemory.EntityID {
	return source.entity
}

func (source RecordClassificationSourceV1) BoundedContext() typedmemory.BoundedContextRef {
	return source.context
}

func (source RecordClassificationSourceV1) RecordVariant() ProjectRecordCarrierVariantV1 {
	return source.variant
}

func (source RecordClassificationSourceV1) Carrier() ProjectRecordCarrierV1 {
	return source.carrier
}

func (source RecordClassificationSourceV1) Binding() EntityRecordCarrierBindingV1 {
	return source.binding
}

func (source RecordClassificationSourceV1) Ref() typedmemory.CarrierRef {
	return source.reference
}

func (source RecordClassificationSourceV1) Digest() typedmemory.SHA256Digest {
	return source.digest
}

func (source RecordClassificationSourceV1) CanonicalBytes() []byte {
	return append([]byte(nil), source.canonical...)
}
