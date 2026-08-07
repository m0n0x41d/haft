package carrierfamily

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	ClassificationSourceSchemaVersionV1 = "haft.carrier-family-classification-source/v1"
	classificationSourcePrefixV1        = "carrier-family-classification-source:"
)

type classificationSourceCanonicalV1 struct {
	SchemaVersion string `json:"schema_version"`
	ProjectID     string `json:"project_id"`
	EntityID      string `json:"entity_id"`
	Context       string `json:"bounded_context"`
	Family        string `json:"family"`
	CarrierBytes  []byte `json:"carrier_bytes"`
	BindingBytes  []byte `json:"binding_bytes"`
}

// ClassificationSourceV1 is the current immutable delivery carrier used to
// derive direct governed features for one exact carrier-family candidate. It
// is not Evidence, a classification judgement, or a historical MemberOf
// observable. Its schema and reference domain are disjoint from
// MembershipSourceV1.
type ClassificationSourceV1 struct {
	project   projectidentity.ProjectID
	entity    typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	family    familyV1
	carrier   CarrierV1
	binding   EntityCarrierBindingV1
	reference typedmemory.CarrierRef
	digest    typedmemory.SHA256Digest
	canonical []byte
}

func SealClassificationSourceV1(
	project projectidentity.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	carrier CarrierV1,
	binding EntityCarrierBindingV1,
) (ClassificationSourceV1, error) {
	if err := correlateSource(project, entity, contextRef, carrier, binding); err != nil {
		return ClassificationSourceV1{}, err
	}
	encoded := classificationSourceCanonicalV1{
		SchemaVersion: ClassificationSourceSchemaVersionV1,
		ProjectID:     project.String(),
		EntityID:      entity.String(),
		Context:       contextRef.String(),
		Family:        carrier.family.token(),
		CarrierBytes:  carrier.CanonicalBytes(),
		BindingBytes:  binding.CanonicalBytes(),
	}
	canonical, err := encodeCanonical(encoded)
	if err != nil {
		return ClassificationSourceV1{}, err
	}
	return decodeClassificationSourceV1(canonical)
}

func DecodeClassificationSourceV1(
	canonical []byte,
) (ClassificationSourceV1, error) {
	return decodeClassificationSourceV1(append([]byte(nil), canonical...))
}

func decodeClassificationSourceV1(
	canonical []byte,
) (ClassificationSourceV1, error) {
	var encoded classificationSourceCanonicalV1
	if err := decodeCanonical(canonical, &encoded); err != nil {
		return ClassificationSourceV1{}, err
	}
	if encoded.SchemaVersion != ClassificationSourceSchemaVersionV1 {
		return ClassificationSourceV1{}, fmt.Errorf(
			"unsupported carrier-family classification source schema %q",
			encoded.SchemaVersion,
		)
	}
	project, err := projectidentity.ParseProjectID(encoded.ProjectID)
	if err != nil || project.String() != encoded.ProjectID {
		return ClassificationSourceV1{}, fmt.Errorf(
			"carrier-family classification source project is invalid",
		)
	}
	entity, err := typedmemory.NewEntityID(encoded.EntityID)
	if err != nil || entity.String() != encoded.EntityID {
		return ClassificationSourceV1{}, fmt.Errorf(
			"carrier-family classification source entity is invalid",
		)
	}
	contextRef, err := typedmemory.NewBoundedContextRef(encoded.Context)
	if err != nil || contextRef.String() != encoded.Context {
		return ClassificationSourceV1{}, fmt.Errorf(
			"carrier-family classification source context is invalid",
		)
	}
	family, err := parseFamilyV1(encoded.Family)
	if err != nil {
		return ClassificationSourceV1{}, err
	}
	carrier, err := DecodeCarrierV1(encoded.CarrierBytes)
	if err != nil {
		return ClassificationSourceV1{}, fmt.Errorf(
			"carrier-family classification source carrier: %w",
			err,
		)
	}
	binding, err := DecodeEntityCarrierBindingV1(encoded.BindingBytes)
	if err != nil {
		return ClassificationSourceV1{}, fmt.Errorf(
			"carrier-family classification source binding: %w",
			err,
		)
	}
	if family != carrier.family {
		return ClassificationSourceV1{}, fmt.Errorf(
			"carrier-family classification source outer family is substituted",
		)
	}
	if err := correlateSource(project, entity, contextRef, carrier, binding); err != nil {
		return ClassificationSourceV1{}, err
	}
	digest, err := digestCanonical(canonical)
	if err != nil {
		return ClassificationSourceV1{}, err
	}
	reference, err := typedmemory.NewCarrierRef(
		classificationSourcePrefixV1 + digest.String(),
	)
	if err != nil {
		return ClassificationSourceV1{}, fmt.Errorf(
			"derive carrier-family classification source reference: %w",
			err,
		)
	}
	return ClassificationSourceV1{
		project:   project,
		entity:    entity,
		context:   contextRef,
		family:    family,
		carrier:   carrier,
		binding:   binding,
		reference: reference,
		digest:    digest,
		canonical: append([]byte(nil), canonical...),
	}, nil
}

func VerifyClassificationSourceV1(
	expectedReference typedmemory.CarrierRef,
	expectedDigest typedmemory.SHA256Digest,
	canonical []byte,
) (ClassificationSourceV1, error) {
	source, err := DecodeClassificationSourceV1(canonical)
	if err != nil {
		return ClassificationSourceV1{}, err
	}
	if source.reference != expectedReference || source.digest != expectedDigest {
		return ClassificationSourceV1{}, fmt.Errorf(
			"carrier-family classification source coordinate does not match canonical bytes",
		)
	}
	return source, nil
}

func IsClassificationSourceReference(reference typedmemory.CarrierRef) bool {
	return strings.HasPrefix(reference.String(), classificationSourcePrefixV1)
}

func (source ClassificationSourceV1) SchemaVersion() string {
	return ClassificationSourceSchemaVersionV1
}

func (source ClassificationSourceV1) ProjectID() projectidentity.ProjectID {
	return source.project
}

func (source ClassificationSourceV1) EntityID() typedmemory.EntityID {
	return source.entity
}

func (source ClassificationSourceV1) BoundedContext() typedmemory.BoundedContextRef {
	return source.context
}

func (source ClassificationSourceV1) FamilyToken() string {
	return source.family.token()
}

func (source ClassificationSourceV1) Carrier() CarrierV1 { return source.carrier }

func (source ClassificationSourceV1) Binding() EntityCarrierBindingV1 {
	return source.binding
}

func (source ClassificationSourceV1) Ref() typedmemory.CarrierRef {
	return source.reference
}

func (source ClassificationSourceV1) Digest() typedmemory.SHA256Digest {
	return source.digest
}

func (source ClassificationSourceV1) CanonicalBytes() []byte {
	return append([]byte(nil), source.canonical...)
}
