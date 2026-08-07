package typedmemory

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const maxCanonicalDecodeBytes = 64 * 1024 * 1024

type strictCanonicalReader struct {
	reader     canonicalReader
	fieldCount uint64
}

// DecodeCanonicalRelationInstance reconstructs the exact strong relation value
// represented by persisted canonical bytes. It authenticates canonical
// structure and content-derived references and digests only. It does not prove
// storage admission, graph currentness, or validity under a target TypeEnv.
func DecodeCanonicalRelationInstance(raw []byte) (RelationInstance, error) {
	reader, err := newStrictCanonicalReader(raw, "validated-relation-instance.v2")
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation instance: %w", err)
	}

	assertionRaw, err := reader.readString()
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation assertion: %w", err)
	}
	assertion, err := NewAssertionID(assertionRaw)
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation assertion: %w", err)
	}
	if assertion.String() != assertionRaw {
		return RelationInstance{}, fmt.Errorf("canonical relation assertion is not canonical")
	}

	signatureRaw, err := reader.readString()
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation signature: %w", err)
	}
	signature, err := parseCanonicalRelationSignatureRef(signatureRaw)
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation signature: %w", err)
	}

	sliceRefRaw, err := reader.readString()
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation ContextSlice ref: %w", err)
	}
	sliceRef, err := parseCanonicalContextSliceRef(sliceRefRaw)
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation ContextSlice ref: %w", err)
	}
	sliceRaw, err := reader.readBytes()
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation ContextSlice: %w", err)
	}
	slice, err := DecodeCanonicalContextSlice(sliceRaw)
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation ContextSlice: %w", err)
	}
	if slice.Ref() != sliceRef {
		return RelationInstance{}, fmt.Errorf(
			"canonical relation ContextSlice ref %s does not match content-derived ref %s",
			sliceRef.String(),
			slice.Ref().String(),
		)
	}

	tail, err := reader.readRemainingBytes()
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation tail: %w", err)
	}
	if len(tail) < 2 {
		return RelationInstance{}, fmt.Errorf(
			"canonical relation requires at least one slot binding and provenance",
		)
	}

	bindings := make([]SlotBinding, 0, len(tail)-1)
	for index, encoded := range tail[:len(tail)-1] {
		binding, decodeErr := decodeCanonicalSlotBinding(encoded)
		if decodeErr != nil {
			return RelationInstance{}, fmt.Errorf(
				"canonical relation binding %d: %w",
				index,
				decodeErr,
			)
		}
		bindings = append(bindings, binding)
	}

	provenanceRaw, err := strictUTF8String(tail[len(tail)-1])
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation provenance: %w", err)
	}
	provenance, err := NewProvenanceRef(provenanceRaw)
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation provenance: %w", err)
	}
	if provenance.String() != provenanceRaw {
		return RelationInstance{}, fmt.Errorf("canonical relation provenance is not canonical")
	}

	relation := RelationInstance{
		assertion:  assertion,
		signature:  signature,
		slice:      slice,
		bindings:   append([]SlotBinding(nil), bindings...),
		provenance: provenance,
	}
	if !relation.valid() {
		return RelationInstance{}, fmt.Errorf(
			"canonical relation does not form a valid normalized RelationInstance",
		)
	}
	canonical, err := relation.CanonicalBytes()
	if err != nil {
		return RelationInstance{}, fmt.Errorf("canonical relation reconstruction: %w", err)
	}
	if !bytes.Equal(canonical, raw) {
		return RelationInstance{}, fmt.Errorf(
			"canonical relation bytes are not in normalized canonical form",
		)
	}
	return relation, nil
}

// DecodeCanonicalRelationalAssertion reconstructs the exact strong v3
// assertion represented by canonical bytes. It authenticates canonical
// structure and content-derived references only. In particular, a positive
// modality remains assertion content and is not evidence that the direct
// relation obtains or that an occurrence exists.
func DecodeCanonicalRelationalAssertion(raw []byte) (RelationalAssertion, error) {
	reader, err := newStrictCanonicalReader(raw, validatedRelationalAssertionCanonicalDomain)
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion: %w", err)
	}

	assertionRaw, err := reader.readString()
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion ID: %w", err)
	}
	assertionID, err := NewAssertionID(assertionRaw)
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion ID: %w", err)
	}
	if assertionID.String() != assertionRaw {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion ID is not canonical")
	}

	signatureRaw, err := reader.readString()
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion signature: %w", err)
	}
	signature, err := parseCanonicalRelationSignatureRef(signatureRaw)
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion signature: %w", err)
	}

	sliceRefRaw, err := reader.readString()
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion ContextSlice ref: %w", err)
	}
	sliceRef, err := parseCanonicalContextSliceRef(sliceRefRaw)
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion ContextSlice ref: %w", err)
	}
	sliceRaw, err := reader.readBytes()
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion ContextSlice: %w", err)
	}
	slice, err := DecodeCanonicalContextSlice(sliceRaw)
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion ContextSlice: %w", err)
	}
	if slice.Ref() != sliceRef {
		return RelationalAssertion{}, fmt.Errorf(
			"canonical relational assertion ContextSlice ref %s does not match content-derived ref %s",
			sliceRef.String(),
			slice.Ref().String(),
		)
	}

	modalityRaw, err := reader.readString()
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion modality: %w", err)
	}
	modality, err := parseCanonicalAssertionModality(modalityRaw)
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion modality: %w", err)
	}

	tail, err := reader.readRemainingBytes()
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion tail: %w", err)
	}
	if len(tail) < 2 {
		return RelationalAssertion{}, fmt.Errorf(
			"canonical relational assertion requires at least one slot binding and provenance",
		)
	}

	bindings := make([]SlotBinding, 0, len(tail)-1)
	for index, encoded := range tail[:len(tail)-1] {
		binding, decodeErr := decodeCanonicalSlotBinding(encoded)
		if decodeErr != nil {
			return RelationalAssertion{}, fmt.Errorf(
				"canonical relational assertion binding %d: %w",
				index,
				decodeErr,
			)
		}
		bindings = append(bindings, binding)
	}

	provenanceRaw, err := strictUTF8String(tail[len(tail)-1])
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion provenance: %w", err)
	}
	provenance, err := NewProvenanceRef(provenanceRaw)
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion provenance: %w", err)
	}
	if provenance.String() != provenanceRaw {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion provenance is not canonical")
	}

	assertion := RelationalAssertion{
		assertion:  assertionID,
		signature:  signature,
		slice:      slice,
		modality:   modality,
		bindings:   append([]SlotBinding(nil), bindings...),
		provenance: provenance,
	}
	if !assertion.valid() {
		return RelationalAssertion{}, fmt.Errorf(
			"canonical bytes do not form a valid normalized RelationalAssertion",
		)
	}
	canonical, err := assertion.CanonicalBytes()
	if err != nil {
		return RelationalAssertion{}, fmt.Errorf("canonical relational assertion reconstruction: %w", err)
	}
	if !bytes.Equal(canonical, raw) {
		return RelationalAssertion{}, fmt.Errorf(
			"canonical relational assertion bytes are not in normalized canonical form",
		)
	}
	return assertion, nil
}

func parseCanonicalAssertionModality(raw string) (AssertionModality, error) {
	kind := AssertionModalityKind(raw)
	switch kind {
	case AssertionModalityAffirmsObtaining:
		return NewAffirmsObtaining(), nil
	case AssertionModalityDeniesObtaining:
		return NewDeniesObtaining(), nil
	case AssertionModalityObtainingUnknown:
		return NewObtainingUnknown(), nil
	default:
		return nil, fmt.Errorf("unknown assertion modality %q", raw)
	}
}

// DecodeCanonicalContextSlice reconstructs the exact content-addressed context
// selection represented by canonical bytes. It authenticates canonical
// structure and the content-derived ContextSliceRef only. It does not prove
// storage admission, graph currentness, or validity under a target TypeEnv.
func DecodeCanonicalContextSlice(raw []byte) (ContextSlice, error) {
	reader, err := newStrictCanonicalReader(raw, contextSliceCanonicalDomain)
	if err != nil {
		return ContextSlice{}, fmt.Errorf("canonical ContextSlice: %w", err)
	}

	contextRaw, err := reader.readString()
	if err != nil {
		return ContextSlice{}, fmt.Errorf("canonical ContextSlice context: %w", err)
	}
	context, err := NewBoundedContextRef(contextRaw)
	if err != nil {
		return ContextSlice{}, fmt.Errorf("canonical ContextSlice context: %w", err)
	}
	if context.String() != contextRaw {
		return ContextSlice{}, fmt.Errorf("canonical ContextSlice context is not canonical")
	}

	standards, err := decodeCanonicalStandardPins(reader)
	if err != nil {
		return ContextSlice{}, err
	}
	environment, err := decodeCanonicalEnvironmentSelectors(reader)
	if err != nil {
		return ContextSlice{}, err
	}
	vocabularies, err := decodeCanonicalVocabularyPins(reader)
	if err != nil {
		return ContextSlice{}, err
	}
	roleSets, err := decodeCanonicalRoleSetPins(reader)
	if err != nil {
		return ContextSlice{}, err
	}

	gammaRaw, err := reader.readBytes()
	if err != nil {
		return ContextSlice{}, fmt.Errorf("canonical ContextSlice Gamma selector: %w", err)
	}
	gamma, err := decodeCanonicalGammaTimeSelector(gammaRaw)
	if err != nil {
		return ContextSlice{}, fmt.Errorf("canonical ContextSlice Gamma selector: %w", err)
	}
	if err := reader.requireEnd(); err != nil {
		return ContextSlice{}, fmt.Errorf("canonical ContextSlice: %w", err)
	}

	slice, err := NewContextSlice(ContextSliceInput{
		Context:              context,
		StandardPins:         standards,
		EnvironmentSelectors: environment,
		VocabularyPins:       vocabularies,
		RoleSetPins:          roleSets,
		GammaTime:            gamma,
	})
	if err != nil {
		return ContextSlice{}, fmt.Errorf("canonical ContextSlice reconstruction: %w", err)
	}
	if !bytes.Equal(slice.CanonicalBytes(), raw) {
		return ContextSlice{}, fmt.Errorf(
			"canonical ContextSlice bytes are not in normalized canonical form",
		)
	}
	return slice, nil
}

func decodeCanonicalStandardPins(reader *strictCanonicalReader) ([]StandardPin, error) {
	count, err := reader.readCount()
	if err != nil {
		return nil, fmt.Errorf("canonical ContextSlice Standard count: %w", err)
	}
	result := make([]StandardPin, 0, count)
	for index := uint64(0); index < count; index++ {
		raw, readErr := reader.readBytes()
		if readErr != nil {
			return nil, fmt.Errorf("canonical ContextSlice Standard pin %d: %w", index, readErr)
		}
		reference, edition, digest, decodeErr := decodeCanonicalVersionedPin(
			raw,
			"context-slice-standard-pin.v1",
		)
		if decodeErr != nil {
			return nil, fmt.Errorf("canonical ContextSlice Standard pin %d: %w", index, decodeErr)
		}
		pin, constructErr := NewStandardPin(reference, edition, digest)
		if constructErr != nil {
			return nil, fmt.Errorf("canonical ContextSlice Standard pin %d: %w", index, constructErr)
		}
		if !bytes.Equal(pin.contextPinBytes(), raw) {
			return nil, fmt.Errorf(
				"canonical ContextSlice Standard pin %d is not canonical",
				index,
			)
		}
		result = append(result, pin)
	}
	return result, nil
}

func decodeCanonicalEnvironmentSelectors(
	reader *strictCanonicalReader,
) ([]EnvironmentSelector, error) {
	count, err := reader.readCount()
	if err != nil {
		return nil, fmt.Errorf("canonical ContextSlice environment count: %w", err)
	}
	result := make([]EnvironmentSelector, 0, count)
	for index := uint64(0); index < count; index++ {
		raw, readErr := reader.readBytes()
		if readErr != nil {
			return nil, fmt.Errorf(
				"canonical ContextSlice environment selector %d: %w",
				index,
				readErr,
			)
		}
		selector, decodeErr := decodeCanonicalEnvironmentSelector(raw)
		if decodeErr != nil {
			return nil, fmt.Errorf(
				"canonical ContextSlice environment selector %d: %w",
				index,
				decodeErr,
			)
		}
		result = append(result, selector)
	}
	return result, nil
}

func decodeCanonicalVocabularyPins(reader *strictCanonicalReader) ([]VocabularyPin, error) {
	count, err := reader.readCount()
	if err != nil {
		return nil, fmt.Errorf("canonical ContextSlice vocabulary count: %w", err)
	}
	result := make([]VocabularyPin, 0, count)
	for index := uint64(0); index < count; index++ {
		raw, readErr := reader.readBytes()
		if readErr != nil {
			return nil, fmt.Errorf("canonical ContextSlice vocabulary pin %d: %w", index, readErr)
		}
		reference, edition, digest, decodeErr := decodeCanonicalVersionedPin(
			raw,
			"context-slice-vocabulary-pin.v1",
		)
		if decodeErr != nil {
			return nil, fmt.Errorf("canonical ContextSlice vocabulary pin %d: %w", index, decodeErr)
		}
		pin, constructErr := NewVocabularyPin(reference, edition, digest)
		if constructErr != nil {
			return nil, fmt.Errorf(
				"canonical ContextSlice vocabulary pin %d: %w",
				index,
				constructErr,
			)
		}
		if !bytes.Equal(pin.contextPinBytes(), raw) {
			return nil, fmt.Errorf(
				"canonical ContextSlice vocabulary pin %d is not canonical",
				index,
			)
		}
		result = append(result, pin)
	}
	return result, nil
}

func decodeCanonicalRoleSetPins(reader *strictCanonicalReader) ([]RoleSetPin, error) {
	count, err := reader.readCount()
	if err != nil {
		return nil, fmt.Errorf("canonical ContextSlice role-set count: %w", err)
	}
	result := make([]RoleSetPin, 0, count)
	for index := uint64(0); index < count; index++ {
		raw, readErr := reader.readBytes()
		if readErr != nil {
			return nil, fmt.Errorf("canonical ContextSlice role-set pin %d: %w", index, readErr)
		}
		reference, edition, digest, decodeErr := decodeCanonicalVersionedPin(
			raw,
			"context-slice-role-set-pin.v1",
		)
		if decodeErr != nil {
			return nil, fmt.Errorf("canonical ContextSlice role-set pin %d: %w", index, decodeErr)
		}
		pin, constructErr := NewRoleSetPin(reference, edition, digest)
		if constructErr != nil {
			return nil, fmt.Errorf(
				"canonical ContextSlice role-set pin %d: %w",
				index,
				constructErr,
			)
		}
		if !bytes.Equal(pin.contextPinBytes(), raw) {
			return nil, fmt.Errorf(
				"canonical ContextSlice role-set pin %d is not canonical",
				index,
			)
		}
		result = append(result, pin)
	}
	return result, nil
}

func decodeCanonicalVersionedPin(
	raw []byte,
	domain string,
) (CarrierRef, CarrierEdition, SHA256Digest, error) {
	reader, err := newStrictCanonicalReader(raw, domain)
	if err != nil {
		return CarrierRef{}, CarrierEdition{}, SHA256Digest{}, err
	}
	referenceRaw, err := reader.readString()
	if err != nil {
		return CarrierRef{}, CarrierEdition{}, SHA256Digest{}, err
	}
	editionRaw, err := reader.readString()
	if err != nil {
		return CarrierRef{}, CarrierEdition{}, SHA256Digest{}, err
	}
	digestRaw, err := reader.readString()
	if err != nil {
		return CarrierRef{}, CarrierEdition{}, SHA256Digest{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return CarrierRef{}, CarrierEdition{}, SHA256Digest{}, err
	}

	reference, err := NewCarrierRef(referenceRaw)
	if err != nil {
		return CarrierRef{}, CarrierEdition{}, SHA256Digest{}, err
	}
	edition, err := NewCarrierEdition(editionRaw)
	if err != nil {
		return CarrierRef{}, CarrierEdition{}, SHA256Digest{}, err
	}
	digest, err := NewSHA256Digest(digestRaw)
	if err != nil {
		return CarrierRef{}, CarrierEdition{}, SHA256Digest{}, err
	}
	if reference.String() != referenceRaw ||
		edition.String() != editionRaw ||
		digest.String() != digestRaw {
		return CarrierRef{}, CarrierEdition{}, SHA256Digest{}, fmt.Errorf(
			"versioned pin fields are not canonical",
		)
	}
	return reference, edition, digest, nil
}

func decodeCanonicalEnvironmentSelector(raw []byte) (EnvironmentSelector, error) {
	reader, err := newStrictCanonicalReader(raw, "context-slice-environment-selector.v1")
	if err != nil {
		return EnvironmentSelector{}, err
	}
	keyRaw, err := reader.readString()
	if err != nil {
		return EnvironmentSelector{}, err
	}
	valueRaw, err := reader.readString()
	if err != nil {
		return EnvironmentSelector{}, err
	}
	digestRaw, err := reader.readString()
	if err != nil {
		return EnvironmentSelector{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return EnvironmentSelector{}, err
	}

	key, err := NewEnvironmentSelectorKey(keyRaw)
	if err != nil {
		return EnvironmentSelector{}, err
	}
	value, err := NewEnvironmentSelectorValue(valueRaw)
	if err != nil {
		return EnvironmentSelector{}, err
	}
	digest, err := NewSHA256Digest(digestRaw)
	if err != nil {
		return EnvironmentSelector{}, err
	}
	selector, err := NewEnvironmentSelector(key, value, digest)
	if err != nil {
		return EnvironmentSelector{}, err
	}
	if !bytes.Equal(selector.canonicalBytes(), raw) {
		return EnvironmentSelector{}, fmt.Errorf("environment selector is not canonical")
	}
	return selector, nil
}

func decodeCanonicalGammaTimeSelector(raw []byte) (GammaTimeSelector, error) {
	domain, err := strictCanonicalDomain(raw)
	if err != nil {
		return nil, err
	}
	switch domain {
	case "context-slice-gamma-point.v1":
		return decodeCanonicalGammaPoint(raw)
	case "context-slice-gamma-window.v1":
		return decodeCanonicalGammaWindow(raw)
	case "context-slice-gamma-policy-application.v1":
		return decodeCanonicalGammaPolicyApplication(raw)
	default:
		return nil, fmt.Errorf("unsupported Gamma selector domain %q", domain)
	}
}

func decodeCanonicalResolvedGammaTimeSelector(raw []byte) (ResolvedGammaTimeSelector, error) {
	domain, err := strictCanonicalDomain(raw)
	if err != nil {
		return nil, err
	}
	switch domain {
	case "context-slice-gamma-point.v1":
		return decodeCanonicalGammaPoint(raw)
	case "context-slice-gamma-window.v1":
		return decodeCanonicalGammaWindow(raw)
	default:
		return nil, fmt.Errorf("%s policy resolved selector domain %q is not exact", "Gamma", domain)
	}
}

func decodeCanonicalGammaPoint(raw []byte) (GammaPoint, error) {
	reader, err := newStrictCanonicalReader(raw, "context-slice-gamma-point.v1")
	if err != nil {
		return GammaPoint{}, err
	}
	atRaw, err := reader.readString()
	if err != nil {
		return GammaPoint{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return GammaPoint{}, err
	}
	at, err := time.Parse(time.RFC3339Nano, atRaw)
	if err != nil {
		return GammaPoint{}, fmt.Errorf("%s point is not RFC3339Nano: %w", "Gamma", err)
	}
	point, err := NewGammaPoint(at)
	if err != nil {
		return GammaPoint{}, err
	}
	if !bytes.Equal(point.CanonicalBytes(), raw) {
		return GammaPoint{}, fmt.Errorf("%s point is not canonical UTC", "Gamma")
	}
	return point, nil
}

func decodeCanonicalGammaWindow(raw []byte) (GammaWindow, error) {
	reader, err := newStrictCanonicalReader(raw, "context-slice-gamma-window.v1")
	if err != nil {
		return GammaWindow{}, err
	}
	startRaw, err := reader.readString()
	if err != nil {
		return GammaWindow{}, err
	}
	endRaw, err := reader.readString()
	if err != nil {
		return GammaWindow{}, err
	}
	startBoundaryRaw, err := reader.readString()
	if err != nil {
		return GammaWindow{}, err
	}
	endBoundaryRaw, err := reader.readString()
	if err != nil {
		return GammaWindow{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return GammaWindow{}, err
	}

	start, err := time.Parse(time.RFC3339Nano, startRaw)
	if err != nil {
		return GammaWindow{}, fmt.Errorf("%s window start is not RFC3339Nano: %w", "Gamma", err)
	}
	end, err := time.Parse(time.RFC3339Nano, endRaw)
	if err != nil {
		return GammaWindow{}, fmt.Errorf("%s window end is not RFC3339Nano: %w", "Gamma", err)
	}
	startBoundary, err := parseCanonicalGammaBoundary(startBoundaryRaw)
	if err != nil {
		return GammaWindow{}, err
	}
	endBoundary, err := parseCanonicalGammaBoundary(endBoundaryRaw)
	if err != nil {
		return GammaWindow{}, err
	}
	window, err := NewGammaWindow(start, end, startBoundary, endBoundary)
	if err != nil {
		return GammaWindow{}, err
	}
	if !bytes.Equal(window.CanonicalBytes(), raw) {
		return GammaWindow{}, fmt.Errorf("%s window is not canonical UTC", "Gamma")
	}
	return window, nil
}

func decodeCanonicalGammaPolicyApplication(raw []byte) (GammaPolicyApplication, error) {
	reader, err := newStrictCanonicalReader(
		raw,
		"context-slice-gamma-policy-application.v1",
	)
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	policyRefRaw, err := reader.readString()
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	policyEditionRaw, err := reader.readString()
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	policyDigestRaw, err := reader.readString()
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	anchorRaw, err := reader.readBytes()
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	resolvedRaw, err := reader.readBytes()
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return GammaPolicyApplication{}, err
	}

	policyRef, err := NewCarrierRef(policyRefRaw)
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	policyEdition, err := NewCarrierEdition(policyEditionRaw)
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	policyDigest, err := NewSHA256Digest(policyDigestRaw)
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	anchor, err := decodeCanonicalGammaPoint(anchorRaw)
	if err != nil {
		return GammaPolicyApplication{}, fmt.Errorf("%s policy anchor: %w", "Gamma", err)
	}
	resolved, err := decodeCanonicalResolvedGammaTimeSelector(resolvedRaw)
	if err != nil {
		return GammaPolicyApplication{}, fmt.Errorf("%s policy resolution: %w", "Gamma", err)
	}
	application, err := NewGammaPolicyApplication(
		policyRef,
		policyEdition,
		policyDigest,
		anchor,
		resolved,
	)
	if err != nil {
		return GammaPolicyApplication{}, err
	}
	if !bytes.Equal(application.CanonicalBytes(), raw) {
		return GammaPolicyApplication{}, fmt.Errorf("%s policy application is not canonical", "Gamma")
	}
	return application, nil
}

func parseCanonicalGammaBoundary(raw string) (GammaBoundary, error) {
	switch raw {
	case GammaBoundaryInclusive.String():
		return GammaBoundaryInclusive, nil
	case GammaBoundaryExclusive.String():
		return GammaBoundaryExclusive, nil
	default:
		return 0, fmt.Errorf("unknown Gamma boundary %q", raw)
	}
}

func decodeCanonicalSlotBinding(raw []byte) (SlotBinding, error) {
	reader, err := newStrictCanonicalReader(raw, "validated-slot-binding.v1")
	if err != nil {
		return SlotBinding{}, err
	}
	nameRaw, err := reader.readString()
	if err != nil {
		return SlotBinding{}, err
	}
	name, err := NewSlotKindID(nameRaw)
	if err != nil {
		return SlotBinding{}, err
	}
	if name.String() != nameRaw {
		return SlotBinding{}, fmt.Errorf("slot name is not canonical")
	}
	fillerBytes, err := reader.readRemainingBytes()
	if err != nil {
		return SlotBinding{}, err
	}
	if len(fillerBytes) == 0 {
		return SlotBinding{}, fmt.Errorf("slot binding requires at least one filler")
	}
	fillers := make([]SlotFiller, 0, len(fillerBytes))
	for index, encoded := range fillerBytes {
		filler, decodeErr := decodeCanonicalSlotFiller(encoded)
		if decodeErr != nil {
			return SlotBinding{}, fmt.Errorf("slot filler %d: %w", index, decodeErr)
		}
		fillers = append(fillers, filler)
	}
	binding := newSlotBinding(name, fillers)
	if !binding.valid() {
		return SlotBinding{}, fmt.Errorf("slot binding does not form a valid normalized binding")
	}
	if !bytes.Equal(binding.CanonicalBytes(), raw) {
		return SlotBinding{}, fmt.Errorf("slot binding is not in normalized canonical form")
	}
	return binding, nil
}

func decodeCanonicalSlotFiller(raw []byte) (SlotFiller, error) {
	domain, err := strictCanonicalDomain(raw)
	if err != nil {
		return nil, err
	}
	switch domain {
	case "validated-by-reference.v2":
		return decodeCanonicalReferenceFiller(raw)
	case "validated-by-value.v1":
		return decodeCanonicalValueFiller(raw)
	default:
		return nil, fmt.Errorf("unsupported slot filler domain %q", domain)
	}
}

func decodeCanonicalReferenceFiller(raw []byte) (ReferenceFiller, error) {
	reader, err := newStrictCanonicalReader(raw, "validated-by-reference.v2")
	if err != nil {
		return ReferenceFiller{}, err
	}
	refKindRaw, err := reader.readString()
	if err != nil {
		return ReferenceFiller{}, err
	}
	referenceKeyRaw, err := reader.readString()
	if err != nil {
		return ReferenceFiller{}, err
	}
	entityRaw, err := reader.readString()
	if err != nil {
		return ReferenceFiller{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return ReferenceFiller{}, err
	}

	refKind, err := parseCanonicalRefKindRef(refKindRaw)
	if err != nil {
		return ReferenceFiller{}, err
	}
	referenceID, err := parseCanonicalPersistedReferenceKey(referenceKeyRaw)
	if err != nil {
		return ReferenceFiller{}, err
	}
	reference, err := NewPersistedRef(refKind, referenceID)
	if err != nil {
		return ReferenceFiller{}, err
	}
	entity, err := NewEntityID(entityRaw)
	if err != nil {
		return ReferenceFiller{}, err
	}
	if entity.String() != entityRaw {
		return ReferenceFiller{}, fmt.Errorf("reference filler entity ID is not canonical")
	}
	filler := newReferenceFiller(reference, entity)
	if !filler.validSlotFiller() {
		return ReferenceFiller{}, fmt.Errorf("reference filler is incomplete")
	}
	if !bytes.Equal(filler.CanonicalBytes(), raw) {
		return ReferenceFiller{}, fmt.Errorf("reference filler is not canonical")
	}
	return filler, nil
}

func decodeCanonicalValueFiller(raw []byte) (ValueFiller, error) {
	reader, err := newStrictCanonicalReader(raw, "validated-by-value.v1")
	if err != nil {
		return ValueFiller{}, err
	}
	valueKindRaw, err := reader.readString()
	if err != nil {
		return ValueFiller{}, err
	}
	valueShapeRaw, err := reader.readString()
	if err != nil {
		return ValueFiller{}, err
	}
	codecRaw, err := reader.readString()
	if err != nil {
		return ValueFiller{}, err
	}
	canonicalBytes, err := reader.readBytes()
	if err != nil {
		return ValueFiller{}, err
	}
	if len(canonicalBytes) == 0 {
		return ValueFiller{}, fmt.Errorf("value filler canonical bytes are empty")
	}
	digestRaw, err := reader.readString()
	if err != nil {
		return ValueFiller{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return ValueFiller{}, err
	}

	valueKind, err := parseCanonicalValueKindRef(valueKindRaw)
	if err != nil {
		return ValueFiller{}, fmt.Errorf("value filler kind: %w", err)
	}
	valueShape, err := parseCanonicalValueShapeRef(valueShapeRaw)
	if err != nil {
		return ValueFiller{}, fmt.Errorf("value filler shape: %w", err)
	}
	codec, err := parseCanonicalCodecRef(codecRaw)
	if err != nil {
		return ValueFiller{}, fmt.Errorf("value filler codec: %w", err)
	}
	digest, err := NewSHA256Digest(digestRaw)
	if err != nil {
		return ValueFiller{}, fmt.Errorf("value filler digest: %w", err)
	}
	computed := digestTypedValue(valueKind, valueShape, codec, canonicalBytes)
	if computed != digest {
		return ValueFiller{}, fmt.Errorf(
			"value filler digest %s does not match content-derived digest %s",
			digest.String(),
			computed.String(),
		)
	}

	value := verifiedTypedValue{
		valueKind:      valueKind,
		valueShape:     valueShape,
		codec:          codec,
		canonicalBytes: append([]byte(nil), canonicalBytes...),
		digest:         digest,
	}
	if !validVerifiedTypedValue(value) {
		return ValueFiller{}, fmt.Errorf("value filler does not form a complete stored typed value")
	}
	filler := newValueFiller(value)
	if !bytes.Equal(filler.CanonicalBytes(), raw) {
		return ValueFiller{}, fmt.Errorf("value filler is not canonical")
	}
	return filler, nil
}

func parseCanonicalRelationSignatureRef(raw string) (RelationSignatureRef, error) {
	typeEnvRaw, signatureRaw, found := strings.Cut(raw, "/signature/")
	if !found {
		return RelationSignatureRef{}, fmt.Errorf("canonical RelationSignatureRef is malformed")
	}
	typeEnv, err := ParseTypeEnvRef(typeEnvRaw)
	if err != nil {
		return RelationSignatureRef{}, err
	}
	signature, err := NewSignatureID(signatureRaw)
	if err != nil {
		return RelationSignatureRef{}, err
	}
	ref, err := NewRelationSignatureRef(typeEnv, signature)
	if err != nil {
		return RelationSignatureRef{}, err
	}
	if ref.String() != raw {
		return RelationSignatureRef{}, fmt.Errorf("RelationSignatureRef is not canonical")
	}
	return ref, nil
}

func parseCanonicalRefKindRef(raw string) (RefKindRef, error) {
	typeEnvRaw, refKindRaw, found := strings.Cut(raw, "/ref-kind/")
	if !found {
		return RefKindRef{}, fmt.Errorf("canonical RefKindRef is malformed")
	}
	typeEnv, err := ParseTypeEnvRef(typeEnvRaw)
	if err != nil {
		return RefKindRef{}, err
	}
	refKind, err := NewRefKindID(refKindRaw)
	if err != nil {
		return RefKindRef{}, err
	}
	ref, err := NewRefKindRef(typeEnv, refKind)
	if err != nil {
		return RefKindRef{}, err
	}
	if ref.String() != raw {
		return RefKindRef{}, fmt.Errorf("RefKindRef is not canonical")
	}
	return ref, nil
}

func parseCanonicalContextSliceRef(raw string) (ContextSliceRef, error) {
	digestRaw, found := strings.CutPrefix(raw, "context-slice:")
	if !found {
		return ContextSliceRef{}, fmt.Errorf("canonical ContextSliceRef is malformed")
	}
	digest, err := NewSHA256Digest(digestRaw)
	if err != nil {
		return ContextSliceRef{}, err
	}
	ref, err := NewContextSliceRef(digest)
	if err != nil {
		return ContextSliceRef{}, err
	}
	if ref.String() != raw {
		return ContextSliceRef{}, fmt.Errorf("ContextSliceRef is not canonical")
	}
	return ref, nil
}

func parseCanonicalPersistedReferenceKey(raw string) (ReferenceID, error) {
	referenceRaw, found := strings.CutPrefix(raw, "persisted:")
	if !found {
		return ReferenceID{}, fmt.Errorf("reference filler key is not persisted")
	}
	reference, err := NewReferenceID(referenceRaw)
	if err != nil {
		return ReferenceID{}, err
	}
	if "persisted:"+reference.String() != raw {
		return ReferenceID{}, fmt.Errorf("persisted reference key is not canonical")
	}
	return reference, nil
}

func newStrictCanonicalReader(raw []byte, domain string) (*strictCanonicalReader, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("canonical bytes are empty")
	}
	if len(raw) > maxCanonicalDecodeBytes {
		return nil, fmt.Errorf(
			"canonical value size %d exceeds limit %d",
			len(raw),
			maxCanonicalDecodeBytes,
		)
	}
	reader := &strictCanonicalReader{
		reader: canonicalReader{input: append([]byte(nil), raw...)},
	}
	envelope, err := reader.readString()
	if err != nil {
		return nil, err
	}
	if envelope != canonicalEnvelopeDomain {
		return nil, fmt.Errorf("unexpected canonical envelope %q", envelope)
	}
	actualDomain, err := reader.readString()
	if err != nil {
		return nil, err
	}
	if actualDomain != domain {
		return nil, fmt.Errorf("unexpected canonical domain %q", actualDomain)
	}
	return reader, nil
}

func strictCanonicalDomain(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("canonical bytes are empty")
	}
	if len(raw) > maxCanonicalDecodeBytes {
		return "", fmt.Errorf(
			"canonical value size %d exceeds limit %d",
			len(raw),
			maxCanonicalDecodeBytes,
		)
	}
	reader := &strictCanonicalReader{
		reader: canonicalReader{input: append([]byte(nil), raw...)},
	}
	envelope, err := reader.readString()
	if err != nil {
		return "", err
	}
	if envelope != canonicalEnvelopeDomain {
		return "", fmt.Errorf("unexpected canonical envelope %q", envelope)
	}
	return reader.readString()
}

func (reader *strictCanonicalReader) readBytes() ([]byte, error) {
	if reader.fieldCount >= maxCollectionItems {
		return nil, fmt.Errorf("canonical field count exceeds limit %d", maxCollectionItems)
	}
	remaining := len(reader.reader.input) - reader.reader.offset
	if remaining < 8 {
		return nil, fmt.Errorf("truncated canonical length prefix")
	}
	lengthStart := reader.reader.offset
	lengthEnd := lengthStart + 8
	length := binary.BigEndian.Uint64(reader.reader.input[lengthStart:lengthEnd])
	payloadRemaining := len(reader.reader.input) - lengthEnd
	payloadRemainingValue, err := strconv.ParseUint(strconv.Itoa(payloadRemaining), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("canonical remaining byte count is invalid: %w", err)
	}
	if length > payloadRemainingValue {
		return nil, fmt.Errorf(
			"canonical field length %d exceeds remaining bytes %d",
			length,
			payloadRemaining,
		)
	}
	if length > maxCanonicalDecodeBytes {
		return nil, fmt.Errorf(
			"canonical field length %d exceeds limit %d",
			length,
			maxCanonicalDecodeBytes,
		)
	}
	payloadEnd := lengthEnd + int(length)
	value := append([]byte(nil), reader.reader.input[lengthEnd:payloadEnd]...)
	reader.reader.offset = payloadEnd
	reader.fieldCount++
	return value, nil
}

func (reader *strictCanonicalReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	return strictUTF8String(value)
}

func (reader *strictCanonicalReader) readCount() (uint64, error) {
	raw, err := reader.readBytes()
	if err != nil {
		return 0, err
	}
	if len(raw) != 8 {
		return 0, fmt.Errorf("canonical count requires exactly 8 bytes")
	}
	value := binary.BigEndian.Uint64(raw)
	if value > maxCollectionItems {
		return 0, fmt.Errorf("canonical collection count %d exceeds limit", value)
	}
	remaining := len(reader.reader.input) - reader.reader.offset
	maxFramedItems, err := strconv.ParseUint(strconv.Itoa(remaining/8), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("canonical framed-field capacity is invalid: %w", err)
	}
	if value > maxFramedItems {
		return 0, fmt.Errorf(
			"canonical collection count %d exceeds remaining framed-field capacity %d",
			value,
			maxFramedItems,
		)
	}
	return value, nil
}

func (reader *strictCanonicalReader) readRemainingBytes() ([][]byte, error) {
	result := make([][]byte, 0)
	for reader.reader.offset < len(reader.reader.input) {
		value, err := reader.readBytes()
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (reader *strictCanonicalReader) requireEnd() error {
	remaining := len(reader.reader.input) - reader.reader.offset
	if remaining != 0 {
		return fmt.Errorf("canonical value has %d trailing bytes", remaining)
	}
	return nil
}

func strictUTF8String(raw []byte) (string, error) {
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("canonical string is not valid UTF-8")
	}
	return string(raw), nil
}
