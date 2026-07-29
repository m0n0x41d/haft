package recordcarrier

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type projectRecordCarrierCanonicalV1 struct {
	SchemaVersion  string `json:"schema_version"`
	EntityID       string `json:"entity_id"`
	BoundedContext string `json:"bounded_context"`
	Variant        string `json:"variant"`
}

// ProjectRecordCarrierV1 is an immutable, content-addressed observable of one
// project-record classification. It is not the represented ProjectRecord,
// claim evidence, approval, or authority. Its closed variant is inert input
// that only a trusted, separately registered evaluator may interpret; this
// package does not itself return generic or specialized membership.
type ProjectRecordCarrierV1 struct {
	ref       typedmemory.CarrierRef
	edition   typedmemory.CarrierEdition
	digest    typedmemory.SHA256Digest
	entity    typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	variant   ProjectRecordCarrierVariantV1
	canonical []byte
}

func SealProjectRecordCarrierV1(
	entity typedmemory.EntityID,
	context typedmemory.BoundedContextRef,
	variant ProjectRecordCarrierVariantV1,
) (ProjectRecordCarrierV1, error) {
	if err := validateEntityID(entity); err != nil {
		return ProjectRecordCarrierV1{}, err
	}
	if err := validateBoundedContext(context); err != nil {
		return ProjectRecordCarrierV1{}, err
	}
	if err := validateProjectRecordCarrierVariantV1(variant); err != nil {
		return ProjectRecordCarrierV1{}, err
	}
	encoded := projectRecordCarrierCanonicalV1{
		SchemaVersion:  ProjectRecordCarrierSchemaVersionV1,
		EntityID:       entity.String(),
		BoundedContext: context.String(),
		Variant:        variant.Token(),
	}
	canonical, err := encodeCanonicalJSON(encoded)
	if err != nil {
		return ProjectRecordCarrierV1{}, err
	}
	return decodeProjectRecordCarrierV1(canonical)
}

func DecodeProjectRecordCarrierV1(canonical []byte) (ProjectRecordCarrierV1, error) {
	return decodeProjectRecordCarrierV1(append([]byte(nil), canonical...))
}

func decodeProjectRecordCarrierV1(canonical []byte) (ProjectRecordCarrierV1, error) {
	var encoded projectRecordCarrierCanonicalV1
	if err := decodeStrictCanonicalJSON(canonical, &encoded); err != nil {
		return ProjectRecordCarrierV1{}, err
	}
	if encoded.SchemaVersion != ProjectRecordCarrierSchemaVersionV1 {
		return ProjectRecordCarrierV1{}, fmt.Errorf(
			"unsupported project-record carrier schema %q",
			encoded.SchemaVersion,
		)
	}
	entity, err := parseExactEntityID(encoded.EntityID)
	if err != nil {
		return ProjectRecordCarrierV1{}, fmt.Errorf("project-record carrier entity: %w", err)
	}
	context, err := parseExactBoundedContext(encoded.BoundedContext)
	if err != nil {
		return ProjectRecordCarrierV1{}, fmt.Errorf("project-record carrier context: %w", err)
	}
	variant, err := parseProjectRecordCarrierVariantV1(encoded.Variant)
	if err != nil {
		return ProjectRecordCarrierV1{}, err
	}
	if err := requireCanonicalJSON(canonical, encoded); err != nil {
		return ProjectRecordCarrierV1{}, err
	}
	digest := digestCanonical(canonical)
	refRaw := "project-record-carrier:" + digest.String()
	ref, err := parseExactCarrierRef(refRaw)
	if err != nil {
		return ProjectRecordCarrierV1{}, fmt.Errorf("derive project-record carrier reference: %w", err)
	}
	edition, err := parseExactCarrierEdition(ProjectRecordCarrierEditionV1)
	if err != nil {
		return ProjectRecordCarrierV1{}, fmt.Errorf("derive project-record carrier edition: %w", err)
	}
	return ProjectRecordCarrierV1{
		ref:       ref,
		edition:   edition,
		digest:    digest,
		entity:    entity,
		context:   context,
		variant:   variant,
		canonical: append([]byte(nil), canonical...),
	}, nil
}

func VerifyProjectRecordCarrierV1(
	expectedRef typedmemory.CarrierRef,
	expectedEdition typedmemory.CarrierEdition,
	expectedDigest typedmemory.SHA256Digest,
	canonical []byte,
) (ProjectRecordCarrierV1, error) {
	carrier, err := DecodeProjectRecordCarrierV1(canonical)
	if err != nil {
		return ProjectRecordCarrierV1{}, err
	}
	if carrier.ref != expectedRef {
		return ProjectRecordCarrierV1{}, fmt.Errorf("project-record carrier reference does not match canonical bytes")
	}
	if carrier.edition != expectedEdition {
		return ProjectRecordCarrierV1{}, fmt.Errorf("project-record carrier edition does not match canonical bytes")
	}
	if carrier.digest != expectedDigest {
		return ProjectRecordCarrierV1{}, fmt.Errorf("project-record carrier digest does not match canonical bytes")
	}
	return carrier, nil
}

func (carrier ProjectRecordCarrierV1) SchemaVersion() string {
	return ProjectRecordCarrierSchemaVersionV1
}

func (carrier ProjectRecordCarrierV1) Ref() typedmemory.CarrierRef { return carrier.ref }

func (carrier ProjectRecordCarrierV1) Edition() typedmemory.CarrierEdition {
	return carrier.edition
}

func (carrier ProjectRecordCarrierV1) Digest() typedmemory.SHA256Digest {
	return carrier.digest
}

func (carrier ProjectRecordCarrierV1) EntityID() typedmemory.EntityID { return carrier.entity }

func (carrier ProjectRecordCarrierV1) BoundedContext() typedmemory.BoundedContextRef {
	return carrier.context
}

func (carrier ProjectRecordCarrierV1) Variant() ProjectRecordCarrierVariantV1 {
	return carrier.variant
}

func (carrier ProjectRecordCarrierV1) CanonicalBytes() []byte {
	return append([]byte(nil), carrier.canonical...)
}

func (carrier ProjectRecordCarrierV1) valid() bool {
	if len(carrier.canonical) == 0 {
		return false
	}
	decoded, err := DecodeProjectRecordCarrierV1(carrier.canonical)
	if err != nil {
		return false
	}
	return decoded.ref == carrier.ref &&
		decoded.edition == carrier.edition &&
		decoded.digest == carrier.digest &&
		decoded.entity == carrier.entity &&
		decoded.context == carrier.context &&
		sameProjectRecordCarrierVariantV1(decoded.variant, carrier.variant)
}

func validateEntityID(entity typedmemory.EntityID) error {
	parsed, err := typedmemory.NewEntityID(entity.String())
	if err != nil || parsed != entity {
		return fmt.Errorf("project-record carrier EntityID is invalid")
	}
	return nil
}

func validateBoundedContext(context typedmemory.BoundedContextRef) error {
	parsed, err := typedmemory.NewBoundedContextRef(context.String())
	if err != nil || parsed != context {
		return fmt.Errorf("project-record carrier bounded context is invalid")
	}
	return nil
}
