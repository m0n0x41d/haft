package carrierfamily

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	bindingSchemaV1 = "haft.entity-carrier-family-binding/v1"
	sourceSchemaV1  = "haft.carrier-family-membership-source/v1"
	sourcePrefixV1  = "carrier-family-membership-source:"
)

type bindingCanonicalV1 struct {
	SchemaVersion  string `json:"schema_version"`
	ProjectID      string `json:"project_id"`
	EntityID       string `json:"entity_id"`
	Context        string `json:"bounded_context"`
	Family         string `json:"family"`
	CarrierRef     string `json:"carrier_ref"`
	CarrierEdition string `json:"carrier_edition"`
	CarrierDigest  string `json:"carrier_digest"`
	CarrierSchema  string `json:"carrier_schema_version"`
	Mapping        string `json:"mapping_manifest_ref"`
	Adapter        string `json:"adapter_version"`
}

// EntityCarrierBindingV1 correlates one exact classification carrier to its
// project and producer mapping. Family comes only from the sealed carrier.
type EntityCarrierBindingV1 struct {
	project   projectidentity.ProjectID
	entity    typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	family    familyV1
	carrier   typedmemory.CarrierRef
	edition   typedmemory.CarrierEdition
	digest    typedmemory.SHA256Digest
	schema    string
	mapping   recordmapping.MappingManifestRef
	adapter   recordmapping.AdapterVersion
	canonical []byte
}

func SealEntityCarrierBindingV1(
	project projectidentity.ProjectID,
	carrier CarrierV1,
	mapping recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) (EntityCarrierBindingV1, error) {
	parsedProject, err := projectidentity.ParseProjectID(project.String())
	if err != nil || parsedProject != project {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family binding project is invalid")
	}
	if !carrier.valid() {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family binding carrier is invalid")
	}
	if err := mapping.Verify(); err != nil {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family mapping manifest is invalid")
	}
	if err := adapter.Verify(); err != nil {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family adapter version is invalid")
	}
	encoded := bindingCanonicalV1{
		SchemaVersion:  bindingSchemaV1,
		ProjectID:      project.String(),
		EntityID:       carrier.EntityID().String(),
		Context:        carrier.BoundedContext().String(),
		Family:         carrier.family.token(),
		CarrierRef:     carrier.Ref().String(),
		CarrierEdition: carrier.Edition().String(),
		CarrierDigest:  carrier.Digest().String(),
		CarrierSchema:  carrier.SchemaVersion(),
		Mapping:        mapping.String(),
		Adapter:        adapter.String(),
	}
	canonical, err := encodeCanonical(encoded)
	if err != nil {
		return EntityCarrierBindingV1{}, err
	}
	return decodeBindingV1(canonical)
}

func DecodeEntityCarrierBindingV1(
	canonical []byte,
) (EntityCarrierBindingV1, error) {
	return decodeBindingV1(append([]byte(nil), canonical...))
}

func decodeBindingV1(canonical []byte) (EntityCarrierBindingV1, error) {
	var encoded bindingCanonicalV1
	if err := decodeCanonical(canonical, &encoded); err != nil {
		return EntityCarrierBindingV1{}, err
	}
	if encoded.SchemaVersion != bindingSchemaV1 {
		return EntityCarrierBindingV1{}, fmt.Errorf("unsupported carrier-family binding schema %q", encoded.SchemaVersion)
	}
	project, err := projectidentity.ParseProjectID(encoded.ProjectID)
	if err != nil || project.String() != encoded.ProjectID {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family binding project is invalid")
	}
	entity, err := typedmemory.NewEntityID(encoded.EntityID)
	if err != nil || entity.String() != encoded.EntityID {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family binding entity is invalid")
	}
	contextRef, err := typedmemory.NewBoundedContextRef(encoded.Context)
	if err != nil || contextRef.String() != encoded.Context {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family binding context is invalid")
	}
	family, err := parseFamilyV1(encoded.Family)
	if err != nil {
		return EntityCarrierBindingV1{}, err
	}
	carrierRef, err := typedmemory.NewCarrierRef(encoded.CarrierRef)
	if err != nil || carrierRef.String() != encoded.CarrierRef {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family binding carrier ref is invalid")
	}
	edition, err := typedmemory.NewCarrierEdition(encoded.CarrierEdition)
	if err != nil || edition.String() != encoded.CarrierEdition {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family binding edition is invalid")
	}
	digest, err := typedmemory.NewSHA256Digest(encoded.CarrierDigest)
	if err != nil || digest.String() != encoded.CarrierDigest {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family binding digest is invalid")
	}
	if !validSemanticToken(encoded.CarrierSchema) {
		return EntityCarrierBindingV1{}, fmt.Errorf("carrier-family binding schema is invalid")
	}
	mapping, err := recordmapping.ParseMappingManifestRef(encoded.Mapping)
	if err != nil {
		return EntityCarrierBindingV1{}, err
	}
	adapter, err := recordmapping.NewAdapterVersion(encoded.Adapter)
	if err != nil {
		return EntityCarrierBindingV1{}, err
	}
	return EntityCarrierBindingV1{
		project:   project,
		entity:    entity,
		context:   contextRef,
		family:    family,
		carrier:   carrierRef,
		edition:   edition,
		digest:    digest,
		schema:    encoded.CarrierSchema,
		mapping:   mapping,
		adapter:   adapter,
		canonical: append([]byte(nil), canonical...),
	}, nil
}

func (binding EntityCarrierBindingV1) ProjectID() projectidentity.ProjectID {
	return binding.project
}

func (binding EntityCarrierBindingV1) EntityID() typedmemory.EntityID {
	return binding.entity
}

func (binding EntityCarrierBindingV1) BoundedContext() typedmemory.BoundedContextRef {
	return binding.context
}

func (binding EntityCarrierBindingV1) CarrierRef() typedmemory.CarrierRef {
	return binding.carrier
}

func (binding EntityCarrierBindingV1) CarrierEdition() typedmemory.CarrierEdition {
	return binding.edition
}

func (binding EntityCarrierBindingV1) CarrierDigest() typedmemory.SHA256Digest {
	return binding.digest
}

func (binding EntityCarrierBindingV1) CarrierSchemaVersion() string {
	return binding.schema
}

func (binding EntityCarrierBindingV1) MappingManifestRef() recordmapping.MappingManifestRef {
	return binding.mapping
}

func (binding EntityCarrierBindingV1) AdapterVersion() recordmapping.AdapterVersion {
	return binding.adapter
}

func (binding EntityCarrierBindingV1) CanonicalBytes() []byte {
	return append([]byte(nil), binding.canonical...)
}

func (binding EntityCarrierBindingV1) valid() bool {
	decoded, err := DecodeEntityCarrierBindingV1(binding.canonical)
	return err == nil &&
		decoded.project == binding.project &&
		decoded.entity == binding.entity &&
		decoded.context == binding.context &&
		decoded.family == binding.family &&
		decoded.carrier == binding.carrier &&
		decoded.edition == binding.edition &&
		decoded.digest == binding.digest &&
		decoded.schema == binding.schema &&
		decoded.mapping == binding.mapping &&
		decoded.adapter == binding.adapter
}

type sourceCanonicalV1 struct {
	SchemaVersion string `json:"schema_version"`
	ProjectID     string `json:"project_id"`
	EntityID      string `json:"entity_id"`
	Context       string `json:"bounded_context"`
	Family        string `json:"family"`
	CarrierBytes  []byte `json:"carrier_bytes"`
	BindingBytes  []byte `json:"binding_bytes"`
}

// MembershipSourceV1 is exact replay material, not executable trust. Only a
// TrustedMembershipSourceDeliveryV1 accepted by the selected policy may be
// interpreted as positive classification.
type MembershipSourceV1 struct {
	project    projectidentity.ProjectID
	entity     typedmemory.EntityID
	context    typedmemory.BoundedContextRef
	family     familyV1
	carrier    CarrierV1
	binding    EntityCarrierBindingV1
	observable typedmemory.MemberOfObservableInput
	digest     typedmemory.SHA256Digest
	canonical  []byte
}

func SealMembershipSourceV1(
	project projectidentity.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	carrier CarrierV1,
	binding EntityCarrierBindingV1,
) (MembershipSourceV1, error) {
	if err := correlateSource(project, entity, contextRef, carrier, binding); err != nil {
		return MembershipSourceV1{}, err
	}
	encoded := sourceCanonicalV1{
		SchemaVersion: sourceSchemaV1,
		ProjectID:     project.String(),
		EntityID:      entity.String(),
		Context:       contextRef.String(),
		Family:        carrier.family.token(),
		CarrierBytes:  carrier.CanonicalBytes(),
		BindingBytes:  binding.CanonicalBytes(),
	}
	canonical, err := encodeCanonical(encoded)
	if err != nil {
		return MembershipSourceV1{}, err
	}
	return decodeMembershipSourceV1(canonical)
}

func DecodeMembershipSourceV1(canonical []byte) (MembershipSourceV1, error) {
	return decodeMembershipSourceV1(append([]byte(nil), canonical...))
}

func decodeMembershipSourceV1(canonical []byte) (MembershipSourceV1, error) {
	var encoded sourceCanonicalV1
	if err := decodeCanonical(canonical, &encoded); err != nil {
		return MembershipSourceV1{}, err
	}
	if encoded.SchemaVersion != sourceSchemaV1 {
		return MembershipSourceV1{}, fmt.Errorf("unsupported carrier-family source schema %q", encoded.SchemaVersion)
	}
	project, err := projectidentity.ParseProjectID(encoded.ProjectID)
	if err != nil || project.String() != encoded.ProjectID {
		return MembershipSourceV1{}, fmt.Errorf("carrier-family source project is invalid")
	}
	entity, err := typedmemory.NewEntityID(encoded.EntityID)
	if err != nil || entity.String() != encoded.EntityID {
		return MembershipSourceV1{}, fmt.Errorf("carrier-family source entity is invalid")
	}
	contextRef, err := typedmemory.NewBoundedContextRef(encoded.Context)
	if err != nil || contextRef.String() != encoded.Context {
		return MembershipSourceV1{}, fmt.Errorf("carrier-family source context is invalid")
	}
	family, err := parseFamilyV1(encoded.Family)
	if err != nil {
		return MembershipSourceV1{}, err
	}
	carrier, err := DecodeCarrierV1(encoded.CarrierBytes)
	if err != nil {
		return MembershipSourceV1{}, fmt.Errorf("carrier-family source carrier: %w", err)
	}
	binding, err := DecodeEntityCarrierBindingV1(encoded.BindingBytes)
	if err != nil {
		return MembershipSourceV1{}, fmt.Errorf("carrier-family source binding: %w", err)
	}
	if family != carrier.family {
		return MembershipSourceV1{}, fmt.Errorf("carrier-family source outer family is substituted")
	}
	if err := correlateSource(project, entity, contextRef, carrier, binding); err != nil {
		return MembershipSourceV1{}, err
	}
	digest, err := digestCanonical(canonical)
	if err != nil {
		return MembershipSourceV1{}, err
	}
	reference, err := typedmemory.NewObservableInputRef(sourcePrefixV1 + digest.String())
	if err != nil {
		return MembershipSourceV1{}, err
	}
	observable, err := typedmemory.NewMemberOfObservableInput(reference, digest)
	if err != nil {
		return MembershipSourceV1{}, err
	}
	return MembershipSourceV1{
		project:    project,
		entity:     entity,
		context:    contextRef,
		family:     family,
		carrier:    carrier,
		binding:    binding,
		observable: observable,
		digest:     digest,
		canonical:  append([]byte(nil), canonical...),
	}, nil
}

func VerifyMembershipSourceV1(
	expected typedmemory.MemberOfObservableInput,
	canonical []byte,
) (MembershipSourceV1, error) {
	source, err := DecodeMembershipSourceV1(canonical)
	if err != nil {
		return MembershipSourceV1{}, err
	}
	if source.observable.Reference() != expected.Reference() ||
		source.observable.Digest() != expected.Digest() {
		return MembershipSourceV1{}, fmt.Errorf("carrier-family observable does not match canonical source bytes")
	}
	return source, nil
}

func IsMembershipSourceReference(reference typedmemory.ObservableInputRef) bool {
	return strings.HasPrefix(reference.String(), sourcePrefixV1)
}

func correlateSource(
	project projectidentity.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	carrier CarrierV1,
	binding EntityCarrierBindingV1,
) error {
	if !carrier.valid() || !binding.valid() {
		return fmt.Errorf("carrier-family source requires exact carrier and binding")
	}
	matches := binding.project == project &&
		binding.entity == entity &&
		binding.context == contextRef &&
		binding.family == carrier.family &&
		carrier.entity == entity &&
		carrier.context == contextRef &&
		binding.carrier == carrier.ref &&
		binding.edition == carrier.edition &&
		binding.digest == carrier.digest &&
		binding.schema == carrier.SchemaVersion()
	if !matches {
		return fmt.Errorf("carrier-family source coordinates do not correlate")
	}
	return nil
}

func (source MembershipSourceV1) ProjectID() projectidentity.ProjectID {
	return source.project
}

func (source MembershipSourceV1) EntityID() typedmemory.EntityID { return source.entity }

func (source MembershipSourceV1) BoundedContext() typedmemory.BoundedContextRef {
	return source.context
}

func (source MembershipSourceV1) Carrier() CarrierV1 { return source.carrier }

func (source MembershipSourceV1) Binding() EntityCarrierBindingV1 {
	return source.binding
}

func (source MembershipSourceV1) EvaluatorRule() typedmemory.RuleRef {
	return source.carrier.EvaluatorRule()
}

func (source MembershipSourceV1) ObservableInput() typedmemory.MemberOfObservableInput {
	return source.observable
}

func (source MembershipSourceV1) CanonicalBytes() []byte {
	return append([]byte(nil), source.canonical...)
}

type TrustedMembershipSourceDeliveryV1 struct {
	source MembershipSourceV1
}

func NewTrustedMembershipSourceDeliveryV1(
	policy recordmembershipregistration.RegistrationArtifactV1,
	expected typedmemory.MemberOfObservableInput,
	canonical []byte,
) (TrustedMembershipSourceDeliveryV1, error) {
	if err := policy.Verify(); err != nil {
		return TrustedMembershipSourceDeliveryV1{}, fmt.Errorf("carrier-family policy: %w", err)
	}
	source, err := VerifyMembershipSourceV1(expected, canonical)
	if err != nil {
		return TrustedMembershipSourceDeliveryV1{}, err
	}
	rule := source.EvaluatorRule()
	if policy.Evaluator().Rule() != rule ||
		policy.SourceDeliveryBoundary().Rule() != rule {
		return TrustedMembershipSourceDeliveryV1{}, fmt.Errorf("carrier-family policy belongs to another evaluator rule")
	}
	decision, err := policy.EvaluateMappingPolicy(
		source.binding.mapping,
		source.binding.adapter,
	)
	if err != nil {
		return TrustedMembershipSourceDeliveryV1{}, err
	}
	if decision.Kind() != recordmembershipregistration.MappingAccepted {
		return TrustedMembershipSourceDeliveryV1{}, fmt.Errorf("carrier-family mapping is not accepted by the exact policy")
	}
	return TrustedMembershipSourceDeliveryV1{source: source}, nil
}

func (delivery TrustedMembershipSourceDeliveryV1) Source() MembershipSourceV1 {
	return delivery.source
}
