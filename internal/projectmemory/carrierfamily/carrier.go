package carrierfamily

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	carrierSchemaV1  = "haft.carrier-family-classification/v1"
	carrierEditionV1 = "1"
)

type SourcePayloadV1 struct {
	ref       typedmemory.CarrierRef
	edition   typedmemory.CarrierEdition
	digest    typedmemory.SHA256Digest
	schema    string
	canonical []byte
}

func NewSourcePayloadV1(
	ref typedmemory.CarrierRef,
	edition typedmemory.CarrierEdition,
	digest typedmemory.SHA256Digest,
	schema string,
	canonical []byte,
) (SourcePayloadV1, error) {
	parsedRef, err := typedmemory.NewCarrierRef(ref.String())
	if err != nil || parsedRef != ref {
		return SourcePayloadV1{}, fmt.Errorf("carrier-family payload reference is invalid")
	}
	parsedEdition, err := typedmemory.NewCarrierEdition(edition.String())
	if err != nil || parsedEdition != edition {
		return SourcePayloadV1{}, fmt.Errorf("carrier-family payload edition is invalid")
	}
	if !validSemanticToken(schema) || len(canonical) == 0 {
		return SourcePayloadV1{}, fmt.Errorf("carrier-family payload schema and bytes are required")
	}
	actual, err := digestCanonical(canonical)
	if err != nil || actual != digest {
		return SourcePayloadV1{}, fmt.Errorf("carrier-family payload digest does not match canonical bytes")
	}
	return SourcePayloadV1{
		ref:       ref,
		edition:   edition,
		digest:    digest,
		schema:    schema,
		canonical: append([]byte(nil), canonical...),
	}, nil
}

func (payload SourcePayloadV1) Ref() typedmemory.CarrierRef { return payload.ref }

func (payload SourcePayloadV1) Edition() typedmemory.CarrierEdition {
	return payload.edition
}

func (payload SourcePayloadV1) Digest() typedmemory.SHA256Digest { return payload.digest }

func (payload SourcePayloadV1) SchemaVersion() string { return payload.schema }

func (payload SourcePayloadV1) CanonicalBytes() []byte {
	return append([]byte(nil), payload.canonical...)
}

func (payload SourcePayloadV1) valid() bool {
	rebuilt, err := NewSourcePayloadV1(
		payload.ref,
		payload.edition,
		payload.digest,
		payload.schema,
		payload.canonical,
	)
	return err == nil && bytes.Equal(rebuilt.canonical, payload.canonical)
}

type carrierCanonicalV1 struct {
	SchemaVersion  string `json:"schema_version"`
	Family         string `json:"family"`
	EntityID       string `json:"entity_id"`
	Context        string `json:"bounded_context"`
	PayloadRef     string `json:"payload_ref"`
	PayloadEdition string `json:"payload_edition"`
	PayloadDigest  string `json:"payload_digest"`
	PayloadSchema  string `json:"payload_schema_version"`
	PayloadBytes   []byte `json:"payload_bytes"`
}

// CarrierV1 is a positive classification carrier over one exact payload. Its
// private family is selected only by the four family-specific constructors.
type CarrierV1 struct {
	ref       typedmemory.CarrierRef
	edition   typedmemory.CarrierEdition
	digest    typedmemory.SHA256Digest
	family    familyV1
	entity    typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	payload   SourcePayloadV1
	canonical []byte
}

func SealCarrierEditionCarrierV1(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	payload SourcePayloadV1,
) (CarrierV1, error) {
	return sealCarrierV1(carrierEditionFamilyV1, entity, contextRef, payload)
}

func SealProjectClaimCarrierV1(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	payload SourcePayloadV1,
) (CarrierV1, error) {
	return sealCarrierV1(projectClaimFamilyV1, entity, contextRef, payload)
}

func SealPerformedWorkOccurrenceCarrierV1(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	payload SourcePayloadV1,
) (CarrierV1, error) {
	return sealCarrierV1(performedWorkOccurrenceFamilyV1, entity, contextRef, payload)
}

func SealCodeAnchorCarrierV1(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	payload SourcePayloadV1,
) (CarrierV1, error) {
	return sealCarrierV1(codeAnchorFamilyV1, entity, contextRef, payload)
}

func sealCarrierV1(
	family familyV1,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	payload SourcePayloadV1,
) (CarrierV1, error) {
	if family.token() == "" || !payload.valid() {
		return CarrierV1{}, fmt.Errorf("carrier-family and payload are required")
	}
	parsedEntity, err := typedmemory.NewEntityID(entity.String())
	if err != nil || parsedEntity != entity {
		return CarrierV1{}, fmt.Errorf("carrier-family EntityID is invalid")
	}
	parsedContext, err := typedmemory.NewBoundedContextRef(contextRef.String())
	if err != nil || parsedContext != contextRef {
		return CarrierV1{}, fmt.Errorf("carrier-family bounded context is invalid")
	}
	encoded := carrierCanonicalV1{
		SchemaVersion:  carrierSchemaV1,
		Family:         family.token(),
		EntityID:       entity.String(),
		Context:        contextRef.String(),
		PayloadRef:     payload.Ref().String(),
		PayloadEdition: payload.Edition().String(),
		PayloadDigest:  payload.Digest().String(),
		PayloadSchema:  payload.SchemaVersion(),
		PayloadBytes:   payload.CanonicalBytes(),
	}
	canonical, err := encodeCanonical(encoded)
	if err != nil {
		return CarrierV1{}, err
	}
	return decodeCarrierV1(canonical)
}

func DecodeCarrierV1(canonical []byte) (CarrierV1, error) {
	return decodeCarrierV1(append([]byte(nil), canonical...))
}

func decodeCarrierV1(canonical []byte) (CarrierV1, error) {
	var encoded carrierCanonicalV1
	if err := decodeCanonical(canonical, &encoded); err != nil {
		return CarrierV1{}, err
	}
	if encoded.SchemaVersion != carrierSchemaV1 {
		return CarrierV1{}, fmt.Errorf("unsupported carrier-family schema %q", encoded.SchemaVersion)
	}
	family, err := parseFamilyV1(encoded.Family)
	if err != nil {
		return CarrierV1{}, err
	}
	entity, err := typedmemory.NewEntityID(encoded.EntityID)
	if err != nil || entity.String() != encoded.EntityID {
		return CarrierV1{}, fmt.Errorf("carrier-family EntityID is invalid")
	}
	contextRef, err := typedmemory.NewBoundedContextRef(encoded.Context)
	if err != nil || contextRef.String() != encoded.Context {
		return CarrierV1{}, fmt.Errorf("carrier-family context is invalid")
	}
	payloadRef, err := typedmemory.NewCarrierRef(encoded.PayloadRef)
	if err != nil || payloadRef.String() != encoded.PayloadRef {
		return CarrierV1{}, fmt.Errorf("carrier-family payload ref is invalid")
	}
	payloadEdition, err := typedmemory.NewCarrierEdition(encoded.PayloadEdition)
	if err != nil || payloadEdition.String() != encoded.PayloadEdition {
		return CarrierV1{}, fmt.Errorf("carrier-family payload edition is invalid")
	}
	payloadDigest, err := typedmemory.NewSHA256Digest(encoded.PayloadDigest)
	if err != nil || payloadDigest.String() != encoded.PayloadDigest {
		return CarrierV1{}, fmt.Errorf("carrier-family payload digest is invalid")
	}
	payload, err := NewSourcePayloadV1(
		payloadRef,
		payloadEdition,
		payloadDigest,
		encoded.PayloadSchema,
		encoded.PayloadBytes,
	)
	if err != nil {
		return CarrierV1{}, err
	}
	digest, err := digestCanonical(canonical)
	if err != nil {
		return CarrierV1{}, err
	}
	ref, err := typedmemory.NewCarrierRef("carrier-family-classification:" + digest.String())
	if err != nil {
		return CarrierV1{}, err
	}
	edition, err := typedmemory.NewCarrierEdition(carrierEditionV1)
	if err != nil {
		return CarrierV1{}, err
	}
	return CarrierV1{
		ref:       ref,
		edition:   edition,
		digest:    digest,
		family:    family,
		entity:    entity,
		context:   contextRef,
		payload:   payload,
		canonical: append([]byte(nil), canonical...),
	}, nil
}

func (carrier CarrierV1) Ref() typedmemory.CarrierRef { return carrier.ref }

func (carrier CarrierV1) Edition() typedmemory.CarrierEdition { return carrier.edition }

func (carrier CarrierV1) Digest() typedmemory.SHA256Digest { return carrier.digest }

func (carrier CarrierV1) SchemaVersion() string { return carrierSchemaV1 }

func (carrier CarrierV1) EntityID() typedmemory.EntityID { return carrier.entity }

func (carrier CarrierV1) BoundedContext() typedmemory.BoundedContextRef {
	return carrier.context
}

func (carrier CarrierV1) Payload() SourcePayloadV1 { return carrier.payload }

func (carrier CarrierV1) EvaluatorRule() typedmemory.RuleRef {
	rule, _ := carrier.family.rule()
	return rule
}

func (carrier CarrierV1) CanonicalBytes() []byte {
	return append([]byte(nil), carrier.canonical...)
}

func (carrier CarrierV1) valid() bool {
	decoded, err := DecodeCarrierV1(carrier.canonical)
	return err == nil &&
		decoded.ref == carrier.ref &&
		decoded.edition == carrier.edition &&
		decoded.digest == carrier.digest &&
		decoded.family == carrier.family &&
		decoded.entity == carrier.entity &&
		decoded.context == carrier.context &&
		bytes.Equal(decoded.canonical, carrier.canonical)
}
