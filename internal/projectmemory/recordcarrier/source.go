package recordcarrier

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type recordMembershipSourceCanonicalV1 struct {
	SchemaVersion  string `json:"schema_version"`
	ProjectID      string `json:"project_id"`
	EntityID       string `json:"entity_id"`
	BoundedContext string `json:"bounded_context"`
	RecordVariant  string `json:"record_variant"`
	CarrierBytes   []byte `json:"carrier_bytes"`
	BindingBytes   []byte `json:"binding_bytes"`
}

// RecordMembershipSourceV1 is a verified, content-addressed correlation of an
// exact carrier and its exact project binding. It embeds both canonical
// artifacts so a later pure evaluator needs no storage lookup. This package
// does not interpret the source as Member or NotMember and exposes no storage,
// admission, activation, or approval operation. Constructing one locally does
// not make it a trusted observable; the later store/registry boundary owns that
// producer check.
type RecordMembershipSourceV1 struct {
	project    projectidentity.ProjectID
	entity     typedmemory.EntityID
	context    typedmemory.BoundedContextRef
	variant    ProjectRecordCarrierVariantV1
	carrier    ProjectRecordCarrierV1
	binding    EntityRecordCarrierBindingV1
	observable typedmemory.MemberOfObservableInput
	canonical  []byte
	digest     typedmemory.SHA256Digest
}

func SealRecordMembershipSourceV1(
	expectedProject projectidentity.ProjectID,
	expectedEntity typedmemory.EntityID,
	expectedContext typedmemory.BoundedContextRef,
	carrier ProjectRecordCarrierV1,
	binding EntityRecordCarrierBindingV1,
) (RecordMembershipSourceV1, error) {
	if err := validateSourceCorrelation(
		expectedProject,
		expectedEntity,
		expectedContext,
		carrier,
		binding,
	); err != nil {
		return RecordMembershipSourceV1{}, err
	}
	encoded := recordMembershipSourceCanonicalV1{
		SchemaVersion:  RecordMembershipSourceSchemaVersionV1,
		ProjectID:      expectedProject.String(),
		EntityID:       expectedEntity.String(),
		BoundedContext: expectedContext.String(),
		RecordVariant:  carrier.Variant().Token(),
		CarrierBytes:   carrier.CanonicalBytes(),
		BindingBytes:   binding.CanonicalBytes(),
	}
	canonical, err := encodeCanonicalJSON(encoded)
	if err != nil {
		return RecordMembershipSourceV1{}, err
	}
	return decodeRecordMembershipSourceV1(canonical)
}

// NewRecordMembershipSourceV1 is the ordinary constructor spelling retained
// for callers that do not need to distinguish sealing from decoding.
func NewRecordMembershipSourceV1(
	expectedProject projectidentity.ProjectID,
	expectedEntity typedmemory.EntityID,
	expectedContext typedmemory.BoundedContextRef,
	carrier ProjectRecordCarrierV1,
	binding EntityRecordCarrierBindingV1,
) (RecordMembershipSourceV1, error) {
	return SealRecordMembershipSourceV1(
		expectedProject,
		expectedEntity,
		expectedContext,
		carrier,
		binding,
	)
}

func DecodeRecordMembershipSourceV1(
	canonical []byte,
) (RecordMembershipSourceV1, error) {
	return decodeRecordMembershipSourceV1(append([]byte(nil), canonical...))
}

func decodeRecordMembershipSourceV1(
	canonical []byte,
) (RecordMembershipSourceV1, error) {
	var encoded recordMembershipSourceCanonicalV1
	if err := decodeStrictCanonicalJSON(canonical, &encoded); err != nil {
		return RecordMembershipSourceV1{}, err
	}
	if encoded.SchemaVersion != RecordMembershipSourceSchemaVersionV1 {
		return RecordMembershipSourceV1{}, fmt.Errorf(
			"unsupported record membership source schema %q",
			encoded.SchemaVersion,
		)
	}
	project, err := projectidentity.ParseProjectID(encoded.ProjectID)
	if err != nil {
		return RecordMembershipSourceV1{}, fmt.Errorf("membership source project: %w", err)
	}
	entity, err := parseExactEntityID(encoded.EntityID)
	if err != nil {
		return RecordMembershipSourceV1{}, fmt.Errorf("membership source entity: %w", err)
	}
	context, err := parseExactBoundedContext(encoded.BoundedContext)
	if err != nil {
		return RecordMembershipSourceV1{}, fmt.Errorf("membership source context: %w", err)
	}
	variant, err := parseProjectRecordCarrierVariantV1(encoded.RecordVariant)
	if err != nil {
		return RecordMembershipSourceV1{}, err
	}
	carrier, err := DecodeProjectRecordCarrierV1(encoded.CarrierBytes)
	if err != nil {
		return RecordMembershipSourceV1{}, fmt.Errorf("membership source carrier: %w", err)
	}
	binding, err := DecodeEntityRecordCarrierBindingV1(encoded.BindingBytes)
	if err != nil {
		return RecordMembershipSourceV1{}, fmt.Errorf("membership source binding: %w", err)
	}
	if err := validateSourceCorrelation(project, entity, context, carrier, binding); err != nil {
		return RecordMembershipSourceV1{}, err
	}
	if !sameProjectRecordCarrierVariantV1(variant, carrier.Variant()) {
		return RecordMembershipSourceV1{}, fmt.Errorf(
			"record membership source outer variant mismatch: got %q, want %q",
			variant.Token(),
			carrier.Variant().Token(),
		)
	}
	if err := requireCanonicalJSON(canonical, encoded); err != nil {
		return RecordMembershipSourceV1{}, err
	}
	digest := digestCanonical(canonical)
	inputRef, err := typedmemory.NewObservableInputRef(
		"record-membership-source:" + digest.String(),
	)
	if err != nil {
		return RecordMembershipSourceV1{}, fmt.Errorf("derive record membership source reference: %w", err)
	}
	observable, err := typedmemory.NewMemberOfObservableInput(inputRef, digest)
	if err != nil {
		return RecordMembershipSourceV1{}, fmt.Errorf("derive record membership observable: %w", err)
	}
	return RecordMembershipSourceV1{
		project:    project,
		entity:     entity,
		context:    context,
		variant:    variant,
		carrier:    carrier,
		binding:    binding,
		observable: observable,
		canonical:  append([]byte(nil), canonical...),
		digest:     digest,
	}, nil
}

func VerifyRecordMembershipSourceV1(
	expected typedmemory.MemberOfObservableInput,
	canonical []byte,
) (RecordMembershipSourceV1, error) {
	source, err := DecodeRecordMembershipSourceV1(canonical)
	if err != nil {
		return RecordMembershipSourceV1{}, err
	}
	if source.observable.Reference() != expected.Reference() ||
		source.observable.Digest() != expected.Digest() {
		return RecordMembershipSourceV1{}, fmt.Errorf(
			"record membership observable does not match canonical source bytes",
		)
	}
	return source, nil
}

func validateSourceCorrelation(
	expectedProject projectidentity.ProjectID,
	expectedEntity typedmemory.EntityID,
	expectedContext typedmemory.BoundedContextRef,
	carrier ProjectRecordCarrierV1,
	binding EntityRecordCarrierBindingV1,
) error {
	if err := validateProjectID(expectedProject); err != nil {
		return err
	}
	if err := validateEntityID(expectedEntity); err != nil {
		return err
	}
	if err := validateBoundedContext(expectedContext); err != nil {
		return err
	}
	if !carrier.valid() {
		return fmt.Errorf("project-record carrier is invalid")
	}
	if !binding.valid() {
		return fmt.Errorf("entity-record carrier binding is invalid")
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{name: "project", got: binding.ProjectID().String(), want: expectedProject.String()},
		{name: "entity lookup", got: carrier.EntityID().String(), want: expectedEntity.String()},
		{name: "context lookup", got: carrier.BoundedContext().String(), want: expectedContext.String()},
		{name: "bound entity", got: binding.EntityID().String(), want: carrier.EntityID().String()},
		{name: "bound context", got: binding.BoundedContext().String(), want: carrier.BoundedContext().String()},
		{name: "carrier reference", got: binding.CarrierRef().String(), want: carrier.Ref().String()},
		{name: "carrier edition", got: binding.CarrierEdition().String(), want: carrier.Edition().String()},
		{name: "carrier digest", got: binding.CarrierDigest().String(), want: carrier.Digest().String()},
		{name: "carrier schema", got: binding.CarrierSchemaVersion(), want: carrier.SchemaVersion()},
	}
	for _, check := range checks {
		if check.got != check.want {
			return fmt.Errorf(
				"record membership source %s mismatch: got %q, want %q",
				check.name,
				check.got,
				check.want,
			)
		}
	}
	if !sameProjectRecordCarrierVariantV1(binding.RecordVariant(), carrier.Variant()) {
		return fmt.Errorf(
			"record membership source carrier variant mismatch: got %q, want %q",
			binding.RecordVariant().Token(),
			carrier.Variant().Token(),
		)
	}
	return nil
}

func (source RecordMembershipSourceV1) SchemaVersion() string {
	return RecordMembershipSourceSchemaVersionV1
}

func (source RecordMembershipSourceV1) ProjectID() projectidentity.ProjectID {
	return source.project
}

func (source RecordMembershipSourceV1) EntityID() typedmemory.EntityID { return source.entity }

func (source RecordMembershipSourceV1) BoundedContext() typedmemory.BoundedContextRef {
	return source.context
}

func (source RecordMembershipSourceV1) RecordVariant() ProjectRecordCarrierVariantV1 {
	return source.variant
}

func (source RecordMembershipSourceV1) Carrier() ProjectRecordCarrierV1 {
	return source.carrier
}

func (source RecordMembershipSourceV1) Binding() EntityRecordCarrierBindingV1 {
	return source.binding
}

func (source RecordMembershipSourceV1) ObservableInput() typedmemory.MemberOfObservableInput {
	return source.observable
}

func (source RecordMembershipSourceV1) Digest() typedmemory.SHA256Digest {
	return source.digest
}

func (source RecordMembershipSourceV1) CanonicalBytes() []byte {
	return append([]byte(nil), source.canonical...)
}
