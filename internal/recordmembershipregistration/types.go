package recordmembershipregistration

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var (
	exactSemanticVersion = regexp.MustCompile(
		`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`,
	)
	exactBuildEdition = regexp.MustCompile(
		`^build-[0-9]{8}\.(0|[1-9][0-9]*)(?:\.[0-9A-Za-z-]+)*$`,
	)
)

const (
	RegistrationSchemaV1  = "haft.record-membership-evaluator-registration-candidate/v1"
	registrationRefPrefix = "record-membership-evaluator-registration-candidate:"

	MaximumRegistrationBytes = 1 << 20
	MaximumAcceptedMappings  = 4 << 10
)

// RegistrationRef is the content-derived identity of one exact registration
// policy. It is not a RuntimeEvaluationBasisRef or a TypeEnvRef.
type RegistrationRef struct {
	digest typedmemory.SHA256Digest
}

func ParseRegistrationRef(raw string) (RegistrationRef, error) {
	digestRaw, found := strings.CutPrefix(raw, registrationRefPrefix)
	if !found {
		return RegistrationRef{}, fmt.Errorf("record-membership registration reference is malformed")
	}
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return RegistrationRef{}, fmt.Errorf(
			"record-membership registration reference digest: %w",
			err,
		)
	}
	ref := RegistrationRef{digest: digest}
	if ref.String() != raw {
		return RegistrationRef{}, fmt.Errorf("record-membership registration reference is not canonical")
	}
	return ref, nil
}

func (ref RegistrationRef) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref RegistrationRef) String() string {
	return registrationRefPrefix + ref.digest.String()
}

func (ref RegistrationRef) valid() bool {
	parsed, err := ParseRegistrationRef(ref.String())
	return err == nil && parsed == ref
}

func (ref RegistrationRef) Verify() error {
	if !ref.valid() {
		return fmt.Errorf("record-membership registration reference is invalid")
	}
	return nil
}

// MechanismRole keeps the pure evaluator and the trusted-delivery boundary
// coordinates non-interchangeable even when both use the same RuleRef.
type MechanismRole uint8

const (
	EvaluatorMechanism MechanismRole = iota + 1
	SourceDeliveryBoundaryMechanism
)

func (role MechanismRole) String() string {
	switch role {
	case EvaluatorMechanism:
		return "evaluator"
	case SourceDeliveryBoundaryMechanism:
		return "source_delivery_boundary"
	default:
		return ""
	}
}

func parseMechanismRole(raw string) (MechanismRole, error) {
	switch raw {
	case EvaluatorMechanism.String():
		return EvaluatorMechanism, nil
	case SourceDeliveryBoundaryMechanism.String():
		return SourceDeliveryBoundaryMechanism, nil
	default:
		return 0, fmt.Errorf("record-membership mechanism role %q is unsupported", raw)
	}
}

// MechanismCoordinate is an exact declared implementation coordinate. It is
// replay provenance and policy identity, not an attestation that executable
// code with these bytes is loaded in the current process.
type MechanismCoordinate struct {
	role     MechanismRole
	rule     typedmemory.RuleRef
	artifact typedmemory.CarrierRef
	edition  typedmemory.CarrierEdition
	digest   typedmemory.SHA256Digest
}

type MechanismCoordinateInput struct {
	Role     MechanismRole
	Rule     typedmemory.RuleRef
	Artifact typedmemory.CarrierRef
	Edition  typedmemory.CarrierEdition
	Digest   typedmemory.SHA256Digest
}

func NewMechanismCoordinate(
	input MechanismCoordinateInput,
) (MechanismCoordinate, error) {
	if input.Role.String() == "" {
		return MechanismCoordinate{}, fmt.Errorf("record-membership mechanism role is required")
	}
	rule, err := exactRuleRef(input.Rule)
	if err != nil {
		return MechanismCoordinate{}, err
	}
	artifact, err := exactCarrierRef(input.Artifact)
	if err != nil {
		return MechanismCoordinate{}, err
	}
	edition, err := exactCarrierEdition(input.Edition)
	if err != nil {
		return MechanismCoordinate{}, err
	}
	digest, err := exactDigest(input.Digest)
	if err != nil {
		return MechanismCoordinate{}, err
	}
	return MechanismCoordinate{
		role:     input.Role,
		rule:     rule,
		artifact: artifact,
		edition:  edition,
		digest:   digest,
	}, nil
}

func (coordinate MechanismCoordinate) Role() MechanismRole { return coordinate.role }

func (coordinate MechanismCoordinate) Rule() typedmemory.RuleRef { return coordinate.rule }

func (coordinate MechanismCoordinate) Artifact() typedmemory.CarrierRef {
	return coordinate.artifact
}

func (coordinate MechanismCoordinate) Edition() typedmemory.CarrierEdition {
	return coordinate.edition
}

func (coordinate MechanismCoordinate) Digest() typedmemory.SHA256Digest {
	return coordinate.digest
}

func (coordinate MechanismCoordinate) valid() bool {
	rebuilt, err := NewMechanismCoordinate(MechanismCoordinateInput{
		Role:     coordinate.role,
		Rule:     coordinate.rule,
		Artifact: coordinate.artifact,
		Edition:  coordinate.edition,
		Digest:   coordinate.digest,
	})
	return err == nil && rebuilt == coordinate
}

// AcceptedMapping binds one exact mapping-manifest content identity to the
// one adapter version allowed to produce its carrier binding.
type AcceptedMapping struct {
	manifest recordmapping.MappingManifestRef
	adapter  recordmapping.AdapterVersion
}

type AcceptedMappingInput struct {
	Manifest recordmapping.MappingManifestRef
	Adapter  recordmapping.AdapterVersion
}

func NewAcceptedMapping(input AcceptedMappingInput) (AcceptedMapping, error) {
	manifest, err := exactMappingManifestRef(input.Manifest)
	if err != nil {
		return AcceptedMapping{}, err
	}
	adapter, err := exactAdapterVersion(input.Adapter)
	if err != nil {
		return AcceptedMapping{}, err
	}
	return AcceptedMapping{
		manifest: manifest,
		adapter:  adapter,
	}, nil
}

func (mapping AcceptedMapping) Manifest() recordmapping.MappingManifestRef {
	return mapping.manifest
}

func (mapping AcceptedMapping) Adapter() recordmapping.AdapterVersion {
	return mapping.adapter
}

func (mapping AcceptedMapping) valid() bool {
	rebuilt, err := NewAcceptedMapping(AcceptedMappingInput{
		Manifest: mapping.manifest,
		Adapter:  mapping.adapter,
	})
	return err == nil && rebuilt == mapping
}

type MappingPolicyDecisionKind uint8

const (
	MappingAccepted MappingPolicyDecisionKind = iota + 1
	MappingManifestNotAccepted
	MappingAdapterMismatch
)

func (kind MappingPolicyDecisionKind) String() string {
	switch kind {
	case MappingAccepted:
		return "accepted"
	case MappingManifestNotAccepted:
		return "manifest_not_accepted"
	case MappingAdapterMismatch:
		return "adapter_mismatch"
	default:
		return ""
	}
}

// MappingPolicyDecision is only a producer-policy result. A non-accepted
// result must not be translated to NotMember, while an accepted result still
// does not mint a trusted delivery or establish membership.
type MappingPolicyDecision struct {
	kind            MappingPolicyDecisionKind
	manifest        recordmapping.MappingManifestRef
	observedAdapter recordmapping.AdapterVersion
	expectedAdapter recordmapping.AdapterVersion
}

func (decision MappingPolicyDecision) Kind() MappingPolicyDecisionKind {
	return decision.kind
}

func (decision MappingPolicyDecision) Manifest() recordmapping.MappingManifestRef {
	return decision.manifest
}

func (decision MappingPolicyDecision) ObservedAdapter() recordmapping.AdapterVersion {
	return decision.observedAdapter
}

func (decision MappingPolicyDecision) ExpectedAdapter() (
	recordmapping.AdapterVersion,
	bool,
) {
	if decision.kind != MappingAdapterMismatch {
		return recordmapping.AdapterVersion{}, false
	}
	return decision.expectedAdapter, true
}

type AcceptedMappingConflictKind uint8

const (
	DuplicateAcceptedMapping AcceptedMappingConflictKind = iota + 1
	ConflictingAdapterForManifest
)

func (kind AcceptedMappingConflictKind) String() string {
	switch kind {
	case DuplicateAcceptedMapping:
		return "duplicate_accepted_mapping"
	case ConflictingAdapterForManifest:
		return "conflicting_adapter_for_manifest"
	default:
		return ""
	}
}

type AcceptedMappingConflict struct {
	kind     AcceptedMappingConflictKind
	manifest recordmapping.MappingManifestRef
	adapters []recordmapping.AdapterVersion
}

func (conflict AcceptedMappingConflict) Error() string {
	return fmt.Sprintf(
		"record-membership registration %s for mapping manifest %q",
		conflict.kind.String(),
		conflict.manifest.String(),
	)
}

func (conflict AcceptedMappingConflict) Kind() AcceptedMappingConflictKind {
	return conflict.kind
}

func (conflict AcceptedMappingConflict) Manifest() recordmapping.MappingManifestRef {
	return conflict.manifest
}

func (conflict AcceptedMappingConflict) Adapters() []recordmapping.AdapterVersion {
	return append([]recordmapping.AdapterVersion(nil), conflict.adapters...)
}

func exactRuleRef(value typedmemory.RuleRef) (typedmemory.RuleRef, error) {
	parsed, err := typedmemory.NewRuleRef(value.String())
	if err != nil || parsed != value {
		return typedmemory.RuleRef{}, fmt.Errorf("record-membership mechanism RuleRef is invalid")
	}
	return parsed, nil
}

func exactCarrierRef(value typedmemory.CarrierRef) (typedmemory.CarrierRef, error) {
	parsed, err := typedmemory.NewCarrierRef(value.String())
	if err != nil || parsed != value {
		return typedmemory.CarrierRef{}, fmt.Errorf("record-membership mechanism artifact is invalid")
	}
	return parsed, nil
}

func exactCarrierEdition(
	value typedmemory.CarrierEdition,
) (typedmemory.CarrierEdition, error) {
	parsed, err := typedmemory.NewCarrierEdition(value.String())
	exact := exactSemanticVersion.MatchString(value.String()) ||
		exactBuildEdition.MatchString(value.String())
	if err != nil || parsed != value || !exact {
		return typedmemory.CarrierEdition{}, fmt.Errorf("record-membership mechanism edition is invalid")
	}
	return parsed, nil
}

func exactDigest(value typedmemory.SHA256Digest) (typedmemory.SHA256Digest, error) {
	parsed, err := typedmemory.NewSHA256Digest(value.String())
	if err != nil || parsed != value {
		return typedmemory.SHA256Digest{}, fmt.Errorf("record-membership mechanism digest is invalid")
	}
	return parsed, nil
}

func exactMappingManifestRef(
	value recordmapping.MappingManifestRef,
) (recordmapping.MappingManifestRef, error) {
	parsed, err := recordmapping.ParseMappingManifestRef(value.String())
	if err != nil || parsed != value {
		return recordmapping.MappingManifestRef{}, fmt.Errorf("record-membership mapping manifest is invalid")
	}
	return parsed, nil
}

func exactAdapterVersion(
	value recordmapping.AdapterVersion,
) (recordmapping.AdapterVersion, error) {
	parsed, err := recordmapping.NewAdapterVersion(value.String())
	if err != nil || parsed != value {
		return recordmapping.AdapterVersion{}, fmt.Errorf("record-membership adapter version is invalid")
	}
	return parsed, nil
}
