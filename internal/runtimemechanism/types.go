package runtimemechanism

import (
	"fmt"
	"regexp"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	MaximumArtifactBytes           = 4 << 20
	MaximumArtifactEntries         = 4 << 10
	MaximumTextBytes               = 16 << 10
	MaximumRuleRefBytes            = 1 << 10
	MaximumCoordinateBytes         = 4 << 10
	MaximumSemanticCoordinateBytes = 16 << 10
	maximumEncodedEntryBytes       = 64 << 10
)

var (
	exactSemanticVersion = regexp.MustCompile(
		`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`,
	)
	exactBuildEdition = regexp.MustCompile(
		`^build-[0-9]{8}\.(0|[1-9][0-9]*)(?:\.[0-9A-Za-z-]+)*$`,
	)
)

// RuntimeMechanismRole is the closed use made of a declared entrypoint.
type RuntimeMechanismRole uint8

const (
	RuntimeMechanismRoleCodec RuntimeMechanismRole = iota + 1
	RuntimeMechanismRoleEvaluator
	RuntimeMechanismRoleCarrierMembership
)

func (role RuntimeMechanismRole) String() string {
	switch role {
	case RuntimeMechanismRoleCodec:
		return "codec"
	case RuntimeMechanismRoleEvaluator:
		return "evaluator"
	case RuntimeMechanismRoleCarrierMembership:
		return "carrier_membership"
	default:
		return ""
	}
}

func parseRuntimeMechanismRole(raw string) (RuntimeMechanismRole, error) {
	switch raw {
	case "codec":
		return RuntimeMechanismRoleCodec, nil
	case "evaluator":
		return RuntimeMechanismRoleEvaluator, nil
	case "carrier_membership":
		return RuntimeMechanismRoleCarrierMembership, nil
	default:
		return 0, fmt.Errorf("runtime mechanism role %q is not defined", raw)
	}
}

// InvocationContract is the closed calling convention promised by one
// declared runtime entrypoint.
type InvocationContract uint8

const (
	InvocationContractCodecCanonicalization InvocationContract = iota + 1
	InvocationContractEntitySetEnumeration
	InvocationContractCandidateVisibility
	InvocationContractKindDefinedness
	InvocationContractMemberOf
	InvocationContractCarrierMembershipDelivery
	InvocationContractReferenceDesignationResolution
	InvocationContractClaimInterpretation
	InvocationContractClaimMeasurement
	InvocationContractClaimEvaluation
	InvocationContractEpistemeConstitutionEvaluation
	InvocationContractKindClassification
)

func (contract InvocationContract) String() string {
	switch contract {
	case InvocationContractCodecCanonicalization:
		return "codec_canonicalization"
	case InvocationContractEntitySetEnumeration:
		return "entity_set_enumeration"
	case InvocationContractCandidateVisibility:
		return "candidate_visibility"
	case InvocationContractKindDefinedness:
		return "kind_definedness"
	case InvocationContractMemberOf:
		return "member_of"
	case InvocationContractCarrierMembershipDelivery:
		return "carrier_membership_delivery"
	case InvocationContractReferenceDesignationResolution:
		return "reference_designation_resolution"
	case InvocationContractClaimInterpretation:
		return "claim_interpretation"
	case InvocationContractClaimMeasurement:
		return "claim_measurement"
	case InvocationContractClaimEvaluation:
		return "claim_evaluation"
	case InvocationContractEpistemeConstitutionEvaluation:
		return "episteme_constitution_evaluation"
	case InvocationContractKindClassification:
		return "kind_classification"
	default:
		return ""
	}
}

func parseInvocationContract(raw string) (InvocationContract, error) {
	switch raw {
	case "codec_canonicalization":
		return InvocationContractCodecCanonicalization, nil
	case "entity_set_enumeration":
		return InvocationContractEntitySetEnumeration, nil
	case "candidate_visibility":
		return InvocationContractCandidateVisibility, nil
	case "kind_definedness":
		return InvocationContractKindDefinedness, nil
	case "member_of":
		return InvocationContractMemberOf, nil
	case "carrier_membership_delivery":
		return InvocationContractCarrierMembershipDelivery, nil
	case "reference_designation_resolution":
		return InvocationContractReferenceDesignationResolution, nil
	case "claim_interpretation":
		return InvocationContractClaimInterpretation, nil
	case "claim_measurement":
		return InvocationContractClaimMeasurement, nil
	case "claim_evaluation":
		return InvocationContractClaimEvaluation, nil
	case "episteme_constitution_evaluation":
		return InvocationContractEpistemeConstitutionEvaluation, nil
	case "kind_classification":
		return InvocationContractKindClassification, nil
	default:
		return 0, fmt.Errorf("invocation contract %q is not defined", raw)
	}
}

type SemanticCoordinateKind uint8

const (
	SemanticCoordinateKindCodecRef SemanticCoordinateKind = iota + 1
	SemanticCoordinateKindRuleRef
)

func (kind SemanticCoordinateKind) String() string {
	switch kind {
	case SemanticCoordinateKindCodecRef:
		return "codec_ref"
	case SemanticCoordinateKindRuleRef:
		return "rule_ref"
	default:
		return ""
	}
}

// SemanticCoordinate is a sealed codec-reference or rule-reference union.
type SemanticCoordinate interface {
	Kind() SemanticCoordinateKind
	semanticCoordinateVariant()
}

type CodecSemanticCoordinate struct {
	ref typedmemory.CodecRef
}

func NewCodecSemanticCoordinate(
	ref typedmemory.CodecRef,
) (CodecSemanticCoordinate, error) {
	rebuilt, err := validateCodecRef(ref)
	if err != nil {
		return CodecSemanticCoordinate{}, err
	}
	return CodecSemanticCoordinate{ref: rebuilt}, nil
}

func (coordinate CodecSemanticCoordinate) Ref() typedmemory.CodecRef {
	return coordinate.ref
}

func (CodecSemanticCoordinate) Kind() SemanticCoordinateKind {
	return SemanticCoordinateKindCodecRef
}

func (CodecSemanticCoordinate) semanticCoordinateVariant() {}

type RuleSemanticCoordinate struct {
	ref typedmemory.RuleRef
}

func NewRuleSemanticCoordinate(
	ref typedmemory.RuleRef,
) (RuleSemanticCoordinate, error) {
	rebuilt, err := validateRuleRef(ref)
	if err != nil {
		return RuleSemanticCoordinate{}, err
	}
	return RuleSemanticCoordinate{ref: rebuilt}, nil
}

func (coordinate RuleSemanticCoordinate) Ref() typedmemory.RuleRef {
	return coordinate.ref
}

func (RuleSemanticCoordinate) Kind() SemanticCoordinateKind {
	return SemanticCoordinateKindRuleRef
}

func (RuleSemanticCoordinate) semanticCoordinateVariant() {}

// RuntimeMechanismEntryV1 is one canonical role/contract/semantic tuple.
type RuntimeMechanismEntryV1 struct {
	role     RuntimeMechanismRole
	contract InvocationContract
	semantic SemanticCoordinate
}

func NewCodecCanonicalizationEntry(
	ref typedmemory.CodecRef,
) (RuntimeMechanismEntryV1, error) {
	coordinate, err := NewCodecSemanticCoordinate(ref)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	return newRuntimeMechanismEntry(
		RuntimeMechanismRoleCodec,
		InvocationContractCodecCanonicalization,
		coordinate,
	)
}

func NewEntitySetEnumerationEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractEntitySetEnumeration,
		ref,
	)
}

func NewCandidateVisibilityEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractCandidateVisibility,
		ref,
	)
}

func NewKindDefinednessEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractKindDefinedness,
		ref,
	)
}

func NewKindClassificationEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractKindClassification,
		ref,
	)
}

func NewMemberOfEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractMemberOf,
		ref,
	)
}

func NewReferenceDesignationResolutionEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractReferenceDesignationResolution,
		ref,
	)
}

func NewClaimInterpretationEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractClaimInterpretation,
		ref,
	)
}

func NewClaimMeasurementEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractClaimMeasurement,
		ref,
	)
}

func NewClaimEvaluationEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractClaimEvaluation,
		ref,
	)
}

func NewEpistemeConstitutionEvaluationEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	return newRuleRuntimeMechanismEntry(
		InvocationContractEpistemeConstitutionEvaluation,
		ref,
	)
}

func NewCarrierMembershipDeliveryEntry(
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	coordinate, err := NewRuleSemanticCoordinate(ref)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	return newRuntimeMechanismEntry(
		RuntimeMechanismRoleCarrierMembership,
		InvocationContractCarrierMembershipDelivery,
		coordinate,
	)
}

func newRuleRuntimeMechanismEntry(
	contract InvocationContract,
	ref typedmemory.RuleRef,
) (RuntimeMechanismEntryV1, error) {
	coordinate, err := NewRuleSemanticCoordinate(ref)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	return newRuntimeMechanismEntry(
		RuntimeMechanismRoleEvaluator,
		contract,
		coordinate,
	)
}

func newRuntimeMechanismEntry(
	role RuntimeMechanismRole,
	contract InvocationContract,
	semantic SemanticCoordinate,
) (RuntimeMechanismEntryV1, error) {
	entry := RuntimeMechanismEntryV1{
		role:     role,
		contract: contract,
		semantic: semantic,
	}
	return validateRuntimeMechanismEntry(entry)
}

func (entry RuntimeMechanismEntryV1) Role() RuntimeMechanismRole {
	return entry.role
}

func (entry RuntimeMechanismEntryV1) Contract() InvocationContract {
	return entry.contract
}

func (entry RuntimeMechanismEntryV1) Semantic() SemanticCoordinate {
	return entry.semantic
}

type entryShape struct {
	role         RuntimeMechanismRole
	semanticKind SemanticCoordinateKind
}

var entryShapeByContract = map[InvocationContract]entryShape{
	InvocationContractCodecCanonicalization: {
		role:         RuntimeMechanismRoleCodec,
		semanticKind: SemanticCoordinateKindCodecRef,
	},
	InvocationContractEntitySetEnumeration: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractCandidateVisibility: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractKindDefinedness: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractMemberOf: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractCarrierMembershipDelivery: {
		role:         RuntimeMechanismRoleCarrierMembership,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractReferenceDesignationResolution: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractClaimInterpretation: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractClaimMeasurement: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractClaimEvaluation: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractEpistemeConstitutionEvaluation: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
	InvocationContractKindClassification: {
		role:         RuntimeMechanismRoleEvaluator,
		semanticKind: SemanticCoordinateKindRuleRef,
	},
}

func validateRuntimeMechanismEntry(
	entry RuntimeMechanismEntryV1,
) (RuntimeMechanismEntryV1, error) {
	roleText := entry.role.String()
	role, err := parseRuntimeMechanismRole(roleText)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	contractText := entry.contract.String()
	contract, err := parseInvocationContract(contractText)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	shape, found := entryShapeByContract[contract]
	if !found {
		return RuntimeMechanismEntryV1{}, fmt.Errorf("invocation contract is not supported")
	}
	if shape.role != role {
		return RuntimeMechanismEntryV1{}, fmt.Errorf(
			"invocation contract %q requires role %q, not %q",
			contract,
			shape.role,
			role,
		)
	}
	semantic, err := validateSemanticCoordinate(entry.semantic)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	if semantic.Kind() != shape.semanticKind {
		return RuntimeMechanismEntryV1{}, fmt.Errorf(
			"invocation contract %q requires semantic coordinate %q, not %q",
			contract,
			shape.semanticKind,
			semantic.Kind(),
		)
	}
	return RuntimeMechanismEntryV1{
		role:     role,
		contract: contract,
		semantic: semantic,
	}, nil
}

func validateSemanticCoordinate(
	coordinate SemanticCoordinate,
) (SemanticCoordinate, error) {
	switch value := coordinate.(type) {
	case CodecSemanticCoordinate:
		return NewCodecSemanticCoordinate(value.ref)
	case RuleSemanticCoordinate:
		return NewRuleSemanticCoordinate(value.ref)
	default:
		return nil, fmt.Errorf("runtime mechanism semantic coordinate is required")
	}
}

func validateCodecRef(
	ref typedmemory.CodecRef,
) (typedmemory.CodecRef, error) {
	id, err := typedmemory.NewCodecID(ref.ID().String())
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec ID: %w", err)
	}
	version, err := typedmemory.NewCanonicalizationVersion(ref.Version().String())
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec canonicalization version: %w", err)
	}
	digest, err := typedmemory.NewSHA256Digest(ref.SpecificationDigest().String())
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec specification digest: %w", err)
	}
	rebuilt, err := typedmemory.NewCodecRef(id, version, digest)
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec reference: %w", err)
	}
	if rebuilt != ref {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec reference is not canonical")
	}
	values := []string{id.String(), version.String(), digest.String()}
	if err := validateTexts(values); err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec reference: %w", err)
	}
	if len(rebuilt.String()) > MaximumSemanticCoordinateBytes {
		return typedmemory.CodecRef{}, fmt.Errorf(
			"runtime codec reference exceeds %d bytes",
			MaximumSemanticCoordinateBytes,
		)
	}
	return rebuilt, nil
}

func validateRuleRef(
	ref typedmemory.RuleRef,
) (typedmemory.RuleRef, error) {
	rebuilt, err := typedmemory.NewRuleRef(ref.String())
	if err != nil {
		return typedmemory.RuleRef{}, fmt.Errorf("runtime RuleRef: %w", err)
	}
	if rebuilt != ref {
		return typedmemory.RuleRef{}, fmt.Errorf("runtime RuleRef is not canonical")
	}
	if err := validateText(rebuilt.String()); err != nil {
		return typedmemory.RuleRef{}, fmt.Errorf("runtime RuleRef: %w", err)
	}
	if len(rebuilt.String()) > MaximumRuleRefBytes {
		return typedmemory.RuleRef{}, fmt.Errorf(
			"runtime RuleRef exceeds %d bytes",
			MaximumRuleRefBytes,
		)
	}
	return rebuilt, nil
}

func validateCarrierRef(
	ref typedmemory.CarrierRef,
) (typedmemory.CarrierRef, error) {
	rebuilt, err := typedmemory.NewCarrierRef(ref.String())
	if err != nil {
		return typedmemory.CarrierRef{}, fmt.Errorf("runtime mechanism artifact reference: %w", err)
	}
	if rebuilt != ref {
		return typedmemory.CarrierRef{}, fmt.Errorf("runtime mechanism artifact reference is not canonical")
	}
	if err := validateText(rebuilt.String()); err != nil {
		return typedmemory.CarrierRef{}, fmt.Errorf("runtime mechanism artifact reference: %w", err)
	}
	if len(rebuilt.String()) > MaximumCoordinateBytes {
		return typedmemory.CarrierRef{}, fmt.Errorf(
			"runtime mechanism artifact reference exceeds %d bytes",
			MaximumCoordinateBytes,
		)
	}
	return rebuilt, nil
}

func validateEdition(
	edition typedmemory.CarrierEdition,
) (typedmemory.CarrierEdition, error) {
	rebuilt, err := typedmemory.NewCarrierEdition(edition.String())
	if err != nil {
		return typedmemory.CarrierEdition{}, fmt.Errorf("runtime mechanism edition: %w", err)
	}
	if rebuilt != edition {
		return typedmemory.CarrierEdition{}, fmt.Errorf("runtime mechanism edition is not canonical")
	}
	if err := validateText(rebuilt.String()); err != nil {
		return typedmemory.CarrierEdition{}, fmt.Errorf("runtime mechanism edition: %w", err)
	}
	if len(rebuilt.String()) > MaximumCoordinateBytes {
		return typedmemory.CarrierEdition{}, fmt.Errorf(
			"runtime mechanism edition exceeds %d bytes",
			MaximumCoordinateBytes,
		)
	}
	if !exactSemanticVersion.MatchString(rebuilt.String()) &&
		!exactBuildEdition.MatchString(rebuilt.String()) {
		return typedmemory.CarrierEdition{}, fmt.Errorf(
			"runtime mechanism edition must be an exact semantic version or immutable build edition",
		)
	}
	return rebuilt, nil
}

func validateDigest(
	digest typedmemory.SHA256Digest,
) (typedmemory.SHA256Digest, error) {
	rebuilt, err := typedmemory.NewSHA256Digest(digest.String())
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf("runtime mechanism digest: %w", err)
	}
	if rebuilt != digest {
		return typedmemory.SHA256Digest{}, fmt.Errorf("runtime mechanism digest is not canonical")
	}
	return rebuilt, nil
}

func validateTexts(values []string) error {
	for _, value := range values {
		if err := validateText(value); err != nil {
			return err
		}
	}
	return nil
}

func validateText(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("contains invalid UTF-8")
	}
	if len(value) > MaximumTextBytes {
		return fmt.Errorf("exceeds %d bytes", MaximumTextBytes)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("contains a control character")
		}
	}
	return nil
}

type EntryConflictKind string

const (
	EntryConflictDuplicate EntryConflictKind = "duplicate_entry"
)

// EntryConflictError reports a deterministic set conflict after canonical
// ordering, independent of the caller's input order.
type EntryConflictError struct {
	kind             EntryConflictKind
	role             RuntimeMechanismRole
	semantic         string
	existingContract InvocationContract
	incomingContract InvocationContract
}

func (conflict *EntryConflictError) Error() string {
	return fmt.Sprintf(
		"runtime mechanism entry %s for role %q and semantic %q: %q conflicts with %q",
		conflict.kind,
		conflict.role,
		conflict.semantic,
		conflict.existingContract,
		conflict.incomingContract,
	)
}

func (conflict *EntryConflictError) Kind() EntryConflictKind {
	return conflict.kind
}

func (conflict *EntryConflictError) Role() RuntimeMechanismRole {
	return conflict.role
}

func (conflict *EntryConflictError) SemanticCoordinate() string {
	return conflict.semantic
}

func (conflict *EntryConflictError) ExistingContract() InvocationContract {
	return conflict.existingContract
}

func (conflict *EntryConflictError) IncomingContract() InvocationContract {
	return conflict.incomingContract
}
