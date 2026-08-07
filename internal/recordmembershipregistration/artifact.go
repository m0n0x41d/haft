package recordmembershipregistration

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type mechanismCoordinateCanonicalV1 struct {
	Role        string `json:"role"`
	RuleRef     string `json:"rule_ref"`
	ArtifactRef string `json:"artifact_ref"`
	Edition     string `json:"edition"`
	Digest      string `json:"digest"`
}

type acceptedMappingCanonicalV1 struct {
	MappingManifestRef string `json:"mapping_manifest_ref"`
	AdapterVersion     string `json:"adapter_version"`
}

type registrationCanonicalV1 struct {
	SchemaVersion  string                         `json:"schema_version"`
	Evaluator      mechanismCoordinateCanonicalV1 `json:"evaluator"`
	SourceDelivery mechanismCoordinateCanonicalV1 `json:"source_delivery_boundary"`
	Mappings       []acceptedMappingCanonicalV1   `json:"accepted_mappings"`
}

// RegistrationArtifactV1 is the content-addressed policy carrier. Its
// canonical bytes do not authenticate executable code, select a TypeEnv, or
// confer trust on a record-membership source. X may pin this exact identity.
type RegistrationArtifactV1 struct {
	ref            RegistrationRef
	evaluator      MechanismCoordinate
	sourceDelivery MechanismCoordinate
	mappings       []AcceptedMapping
	canonical      []byte
}

type RegistrationArtifactInputV1 struct {
	Evaluator      MechanismCoordinate
	SourceDelivery MechanismCoordinate
	Mappings       []AcceptedMapping
}

func SealRegistrationArtifactV1(
	input RegistrationArtifactInputV1,
) (RegistrationArtifactV1, error) {
	if !input.Evaluator.valid() || input.Evaluator.Role() != EvaluatorMechanism {
		return RegistrationArtifactV1{}, fmt.Errorf("record-membership evaluator coordinate is required")
	}
	if !input.SourceDelivery.valid() ||
		input.SourceDelivery.Role() != SourceDeliveryBoundaryMechanism {
		return RegistrationArtifactV1{}, fmt.Errorf("record-membership source-delivery boundary is required")
	}
	mappings, err := normalizeAcceptedMappings(input.Mappings)
	if err != nil {
		return RegistrationArtifactV1{}, err
	}
	encoded := registrationCanonicalV1{
		SchemaVersion:  RegistrationSchemaV1,
		Evaluator:      encodeMechanismCoordinate(input.Evaluator),
		SourceDelivery: encodeMechanismCoordinate(input.SourceDelivery),
		Mappings:       encodeAcceptedMappings(mappings),
	}
	canonical, err := encodeCanonicalRegistration(encoded)
	if err != nil {
		return RegistrationArtifactV1{}, err
	}
	return DecodeRegistrationArtifactV1(canonical)
}

func DecodeRegistrationArtifactV1(
	canonical []byte,
) (RegistrationArtifactV1, error) {
	if len(canonical) == 0 {
		return RegistrationArtifactV1{}, fmt.Errorf("record-membership registration bytes are required")
	}
	if len(canonical) > MaximumRegistrationBytes {
		return RegistrationArtifactV1{}, fmt.Errorf(
			"record-membership registration exceeds %d bytes",
			MaximumRegistrationBytes,
		)
	}
	if !utf8.Valid(canonical) {
		return RegistrationArtifactV1{}, fmt.Errorf("record-membership registration bytes contain invalid UTF-8")
	}
	encoded, err := decodeStrictRegistration(canonical)
	if err != nil {
		return RegistrationArtifactV1{}, err
	}
	if encoded.SchemaVersion != RegistrationSchemaV1 {
		return RegistrationArtifactV1{}, fmt.Errorf(
			"unsupported record-membership registration schema %q",
			encoded.SchemaVersion,
		)
	}
	evaluator, err := decodeMechanismCoordinate(encoded.Evaluator)
	if err != nil {
		return RegistrationArtifactV1{}, fmt.Errorf("decode evaluator coordinate: %w", err)
	}
	if evaluator.Role() != EvaluatorMechanism {
		return RegistrationArtifactV1{}, fmt.Errorf("evaluator coordinate has the wrong role")
	}
	sourceDelivery, err := decodeMechanismCoordinate(encoded.SourceDelivery)
	if err != nil {
		return RegistrationArtifactV1{}, fmt.Errorf("decode source-delivery coordinate: %w", err)
	}
	if sourceDelivery.Role() != SourceDeliveryBoundaryMechanism {
		return RegistrationArtifactV1{}, fmt.Errorf("source-delivery coordinate has the wrong role")
	}
	mappings, err := decodeAcceptedMappings(encoded.Mappings)
	if err != nil {
		return RegistrationArtifactV1{}, err
	}
	normalized, err := normalizeAcceptedMappings(mappings)
	if err != nil {
		return RegistrationArtifactV1{}, err
	}
	reencoded := registrationCanonicalV1{
		SchemaVersion:  RegistrationSchemaV1,
		Evaluator:      encodeMechanismCoordinate(evaluator),
		SourceDelivery: encodeMechanismCoordinate(sourceDelivery),
		Mappings:       encodeAcceptedMappings(normalized),
	}
	exact, err := encodeCanonicalRegistration(reencoded)
	if err != nil {
		return RegistrationArtifactV1{}, err
	}
	if !bytes.Equal(exact, canonical) {
		return RegistrationArtifactV1{}, fmt.Errorf("record-membership registration bytes are not canonical")
	}
	digest, err := digestRegistration(canonical)
	if err != nil {
		return RegistrationArtifactV1{}, err
	}
	ref := RegistrationRef{digest: digest}
	return RegistrationArtifactV1{
		ref:            ref,
		evaluator:      evaluator,
		sourceDelivery: sourceDelivery,
		mappings:       normalized,
		canonical:      append([]byte(nil), canonical...),
	}, nil
}

func VerifyRegistrationArtifactV1(
	expected RegistrationRef,
	canonical []byte,
) (RegistrationArtifactV1, error) {
	if !expected.valid() {
		return RegistrationArtifactV1{}, fmt.Errorf("expected record-membership registration ref is invalid")
	}
	artifact, err := DecodeRegistrationArtifactV1(canonical)
	if err != nil {
		return RegistrationArtifactV1{}, err
	}
	if artifact.ref != expected {
		return RegistrationArtifactV1{}, fmt.Errorf(
			"record-membership registration ref does not match canonical bytes",
		)
	}
	return artifact, nil
}

func (artifact RegistrationArtifactV1) Ref() RegistrationRef { return artifact.ref }

func (artifact RegistrationArtifactV1) Evaluator() MechanismCoordinate {
	return artifact.evaluator
}

func (artifact RegistrationArtifactV1) SourceDeliveryBoundary() MechanismCoordinate {
	return artifact.sourceDelivery
}

func (artifact RegistrationArtifactV1) AcceptedMappings() []AcceptedMapping {
	return append([]AcceptedMapping(nil), artifact.mappings...)
}

func (artifact RegistrationArtifactV1) CanonicalBytes() []byte {
	return append([]byte(nil), artifact.canonical...)
}

func (artifact RegistrationArtifactV1) Verify() error {
	verified, err := VerifyRegistrationArtifactV1(artifact.ref, artifact.canonical)
	if err != nil {
		return err
	}
	if verified.evaluator != artifact.evaluator ||
		verified.sourceDelivery != artifact.sourceDelivery ||
		!slices.Equal(verified.mappings, artifact.mappings) {
		return fmt.Errorf("record-membership registration fields differ from canonical bytes")
	}
	return nil
}

// EvaluateMappingPolicy checks only whether one exact producer mapping pair is
// accepted by this registration. It cannot create a trusted source
// delivery or a MemberOf judgement.
func (artifact RegistrationArtifactV1) EvaluateMappingPolicy(
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) (MappingPolicyDecision, error) {
	if err := artifact.Verify(); err != nil {
		return MappingPolicyDecision{}, err
	}
	exactManifest, err := exactMappingManifestRef(manifest)
	if err != nil {
		return MappingPolicyDecision{}, err
	}
	exactAdapter, err := exactAdapterVersion(adapter)
	if err != nil {
		return MappingPolicyDecision{}, err
	}
	index, found := slices.BinarySearchFunc(
		artifact.mappings,
		exactManifest,
		compareMappingManifest,
	)
	if !found {
		return MappingPolicyDecision{
			kind:            MappingManifestNotAccepted,
			manifest:        exactManifest,
			observedAdapter: exactAdapter,
		}, nil
	}
	expected := artifact.mappings[index].adapter
	if expected != exactAdapter {
		return MappingPolicyDecision{
			kind:            MappingAdapterMismatch,
			manifest:        exactManifest,
			observedAdapter: exactAdapter,
			expectedAdapter: expected,
		}, nil
	}
	return MappingPolicyDecision{
		kind:            MappingAccepted,
		manifest:        exactManifest,
		observedAdapter: exactAdapter,
	}, nil
}

func normalizeAcceptedMappings(
	values []AcceptedMapping,
) ([]AcceptedMapping, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("record-membership registration requires accepted mappings")
	}
	if len(values) > MaximumAcceptedMappings {
		return nil, fmt.Errorf(
			"record-membership registration has %d mappings; maximum is %d",
			len(values),
			MaximumAcceptedMappings,
		)
	}
	normalized := append([]AcceptedMapping(nil), values...)
	for index, mapping := range normalized {
		if !mapping.valid() {
			return nil, fmt.Errorf("record-membership accepted mapping %d is invalid", index)
		}
	}
	slices.SortFunc(normalized, compareAcceptedMappings)
	for index := 1; index < len(normalized); index++ {
		previous := normalized[index-1]
		current := normalized[index]
		if previous.manifest != current.manifest {
			continue
		}
		adapters := []recordmapping.AdapterVersion{previous.adapter, current.adapter}
		slices.SortFunc(adapters, compareAdapterVersions)
		kind := ConflictingAdapterForManifest
		if previous.adapter == current.adapter {
			kind = DuplicateAcceptedMapping
		}
		return nil, AcceptedMappingConflict{
			kind:     kind,
			manifest: current.manifest,
			adapters: adapters,
		}
	}
	return normalized, nil
}

func compareAcceptedMappings(left, right AcceptedMapping) int {
	if order := cmp.Compare(left.manifest.String(), right.manifest.String()); order != 0 {
		return order
	}
	return cmp.Compare(left.adapter.String(), right.adapter.String())
}

func compareMappingManifest(
	mapping AcceptedMapping,
	manifest recordmapping.MappingManifestRef,
) int {
	return cmp.Compare(mapping.manifest.String(), manifest.String())
}

func compareAdapterVersions(
	left recordmapping.AdapterVersion,
	right recordmapping.AdapterVersion,
) int {
	return cmp.Compare(left.String(), right.String())
}

func encodeMechanismCoordinate(
	coordinate MechanismCoordinate,
) mechanismCoordinateCanonicalV1 {
	return mechanismCoordinateCanonicalV1{
		Role:        coordinate.role.String(),
		RuleRef:     coordinate.rule.String(),
		ArtifactRef: coordinate.artifact.String(),
		Edition:     coordinate.edition.String(),
		Digest:      coordinate.digest.String(),
	}
}

func decodeMechanismCoordinate(
	encoded mechanismCoordinateCanonicalV1,
) (MechanismCoordinate, error) {
	role, err := parseMechanismRole(encoded.Role)
	if err != nil {
		return MechanismCoordinate{}, err
	}
	rule, err := typedmemory.NewRuleRef(encoded.RuleRef)
	if err != nil {
		return MechanismCoordinate{}, fmt.Errorf("mechanism RuleRef: %w", err)
	}
	artifact, err := typedmemory.NewCarrierRef(encoded.ArtifactRef)
	if err != nil {
		return MechanismCoordinate{}, fmt.Errorf("mechanism artifact: %w", err)
	}
	edition, err := typedmemory.NewCarrierEdition(encoded.Edition)
	if err != nil {
		return MechanismCoordinate{}, fmt.Errorf("mechanism edition: %w", err)
	}
	digest, err := typedmemory.NewSHA256Digest(encoded.Digest)
	if err != nil {
		return MechanismCoordinate{}, fmt.Errorf("mechanism digest: %w", err)
	}
	return NewMechanismCoordinate(MechanismCoordinateInput{
		Role:     role,
		Rule:     rule,
		Artifact: artifact,
		Edition:  edition,
		Digest:   digest,
	})
}

func encodeAcceptedMappings(values []AcceptedMapping) []acceptedMappingCanonicalV1 {
	result := make([]acceptedMappingCanonicalV1, 0, len(values))
	for _, value := range values {
		result = append(result, acceptedMappingCanonicalV1{
			MappingManifestRef: value.manifest.String(),
			AdapterVersion:     value.adapter.String(),
		})
	}
	return result
}

func decodeAcceptedMappings(
	values []acceptedMappingCanonicalV1,
) ([]AcceptedMapping, error) {
	if len(values) > MaximumAcceptedMappings {
		return nil, fmt.Errorf(
			"record-membership registration has %d mappings; maximum is %d",
			len(values),
			MaximumAcceptedMappings,
		)
	}
	result := make([]AcceptedMapping, 0, len(values))
	for index, value := range values {
		manifest, err := recordmapping.ParseMappingManifestRef(value.MappingManifestRef)
		if err != nil {
			return nil, fmt.Errorf("decode accepted mapping %d manifest: %w", index, err)
		}
		adapter, err := recordmapping.NewAdapterVersion(value.AdapterVersion)
		if err != nil {
			return nil, fmt.Errorf("decode accepted mapping %d adapter: %w", index, err)
		}
		mapping, err := NewAcceptedMapping(AcceptedMappingInput{
			Manifest: manifest,
			Adapter:  adapter,
		})
		if err != nil {
			return nil, fmt.Errorf("decode accepted mapping %d: %w", index, err)
		}
		result = append(result, mapping)
	}
	return result, nil
}

func encodeCanonicalRegistration(value registrationCanonicalV1) ([]byte, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode record-membership registration: %w", err)
	}
	if len(canonical) > MaximumRegistrationBytes {
		return nil, fmt.Errorf(
			"record-membership registration exceeds %d bytes",
			MaximumRegistrationBytes,
		)
	}
	return canonical, nil
}

func decodeStrictRegistration(canonical []byte) (registrationCanonicalV1, error) {
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	var value registrationCanonicalV1
	if err := decoder.Decode(&value); err != nil {
		return registrationCanonicalV1{}, fmt.Errorf(
			"decode record-membership registration: %w",
			err,
		)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		return registrationCanonicalV1{}, fmt.Errorf("record-membership registration has trailing content")
	}
	return value, nil
}

func digestRegistration(canonical []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	hexValue := hex.EncodeToString(sum[:])
	digestRaw := "sha256:" + hexValue
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf("derive registration digest: %w", err)
	}
	return digest, nil
}
