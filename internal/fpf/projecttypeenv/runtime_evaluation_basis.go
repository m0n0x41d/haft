package projecttypeenv

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	runtimeEvaluationBasisCanonicalDomain  = "haft.fpf.projecttypeenv.runtime-evaluation-basis.canonical.v2"
	runtimeEvaluationBasisArtifactDomain   = "runtime-evaluation-basis-artifact.v2"
	runtimeEvaluationBasisRefPrefix        = "runtime-evaluation-basis:"
	maximumRuntimeEvaluationBasisBytes     = 4 << 20
	maximumRuntimeEvaluationBasisPins      = 4 << 10
	maximumRuntimeMechanismTextBytes       = 16 << 10
	maximumRuntimeMechanismRuleRefBytes    = 1 << 10
	maximumRuntimeMechanismCoordinateBytes = 4 << 10
)

var (
	// Keep this closed grammar byte-for-byte aligned with
	// typedmemoryevaluation.exactMechanismEdition. It is mirrored here rather
	// than imported because X is a pure identity carrier and must not depend on
	// the executable evaluator registry. A later shared strong edition type can
	// consolidate both validators without joining those layers.
	runtimeMechanismExactSemanticVersion = regexp.MustCompile(
		`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`,
	)
	runtimeMechanismExactBuildEdition = regexp.MustCompile(
		`^build-[0-9]{8}\.(0|[1-9][0-9]*)(?:\.[0-9A-Za-z-]+)*$`,
	)
)

// RuntimeMechanismRole is the closed semantic use made of one exact runtime
// mechanism. It is not a Go type name, build identity, or implementation
// registry entry.
type RuntimeMechanismRole string

const (
	RuntimeMechanismRoleCodec             RuntimeMechanismRole = "codec"
	RuntimeMechanismRoleEvaluator         RuntimeMechanismRole = "evaluator"
	RuntimeMechanismRoleCarrierMembership RuntimeMechanismRole = "carrier_membership"
)

// RuntimeMechanismInvocationContract is the exact invocation surface bound by
// an X pin. The closed vocabulary is owned by the canonical mechanism catalog.
type RuntimeMechanismInvocationContract = runtimemechanism.InvocationContract

const (
	RuntimeMechanismContractCodecCanonicalization          = runtimemechanism.InvocationContractCodecCanonicalization
	RuntimeMechanismContractEntitySetEnumeration           = runtimemechanism.InvocationContractEntitySetEnumeration
	RuntimeMechanismContractCandidateVisibility            = runtimemechanism.InvocationContractCandidateVisibility
	RuntimeMechanismContractKindDefinedness                = runtimemechanism.InvocationContractKindDefinedness
	RuntimeMechanismContractMemberOf                       = runtimemechanism.InvocationContractMemberOf
	RuntimeMechanismContractCarrierMembershipDelivery      = runtimemechanism.InvocationContractCarrierMembershipDelivery
	RuntimeMechanismContractReferenceDesignationResolution = runtimemechanism.InvocationContractReferenceDesignationResolution
	RuntimeMechanismContractClaimInterpretation            = runtimemechanism.InvocationContractClaimInterpretation
	RuntimeMechanismContractClaimMeasurement               = runtimemechanism.InvocationContractClaimMeasurement
	RuntimeMechanismContractClaimEvaluation                = runtimemechanism.InvocationContractClaimEvaluation
	RuntimeMechanismContractEpistemeConstitutionEvaluation = runtimemechanism.InvocationContractEpistemeConstitutionEvaluation
	RuntimeMechanismContractKindClassification             = runtimemechanism.InvocationContractKindClassification
)

// RuntimeMechanismArtifactPin identifies immutable mechanism content. It
// deliberately carries no executable implementation.
type RuntimeMechanismArtifactPin struct {
	artifact typedmemory.CarrierRef
	edition  typedmemory.CarrierEdition
	digest   typedmemory.SHA256Digest
}

type RuntimeMechanismArtifactPinInput struct {
	Artifact typedmemory.CarrierRef
	Edition  typedmemory.CarrierEdition
	Digest   typedmemory.SHA256Digest
}

func NewRuntimeMechanismArtifactPin(
	input RuntimeMechanismArtifactPinInput,
) (RuntimeMechanismArtifactPin, error) {
	artifact, err := validateRuntimeMechanismCarrierRef(input.Artifact)
	if err != nil {
		return RuntimeMechanismArtifactPin{}, err
	}
	edition, err := validateRuntimeMechanismEdition(input.Edition)
	if err != nil {
		return RuntimeMechanismArtifactPin{}, err
	}
	digest, err := validateRuntimeMechanismDigest(input.Digest)
	if err != nil {
		return RuntimeMechanismArtifactPin{}, err
	}
	return RuntimeMechanismArtifactPin{
		artifact: artifact,
		edition:  edition,
		digest:   digest,
	}, nil
}

func NewRuntimeMechanismArtifactPinFromArtifact(
	artifact runtimemechanism.RuntimeMechanismArtifactV1,
) (RuntimeMechanismArtifactPin, error) {
	if err := artifact.Verify(); err != nil {
		return RuntimeMechanismArtifactPin{}, fmt.Errorf(
			"verify runtime mechanism artifact: %w",
			err,
		)
	}
	identity := artifact.Identity()
	return NewRuntimeMechanismArtifactPin(RuntimeMechanismArtifactPinInput{
		Artifact: identity.Artifact(),
		Edition:  identity.Edition(),
		Digest:   identity.Digest(),
	})
}

func (pin RuntimeMechanismArtifactPin) Artifact() typedmemory.CarrierRef {
	return pin.artifact
}

func (pin RuntimeMechanismArtifactPin) Edition() typedmemory.CarrierEdition {
	return pin.edition
}

func (pin RuntimeMechanismArtifactPin) Digest() typedmemory.SHA256Digest {
	return pin.digest
}

func (pin RuntimeMechanismArtifactPin) valid() bool {
	_, artifactErr := validateRuntimeMechanismCarrierRef(pin.artifact)
	_, editionErr := validateRuntimeMechanismEdition(pin.edition)
	_, digestErr := validateRuntimeMechanismDigest(pin.digest)
	return artifactErr == nil && editionErr == nil && digestErr == nil
}

// RuntimeEvaluationBasisPin is the closed X-pin algebra. A mechanism pin and a
// registration-policy pin are distinct variants; neither attests executable
// loading or execution.
type RuntimeEvaluationBasisPin interface {
	resolvedRuntimeBasisCanonical() []byte
	runtimeEvaluationBasisPinVariant()
}

// RuntimeEvaluationMechanismPin is the mechanism-only sub-algebra retained for
// declaration/runtime-requirement comparison.
type RuntimeEvaluationMechanismPin interface {
	RuntimeEvaluationBasisPin
	Role() RuntimeMechanismRole
	InvocationContract() RuntimeMechanismInvocationContract
	resolvedRuntimeMechanismCanonical() []byte
	runtimeEvaluationMechanismPinVariant()
}

type CodecRuntimeMechanismPin struct {
	codec             typedmemory.CodecRef
	mechanism         RuntimeMechanismArtifactPin
	resolvedCanonical []byte
}

type CodecRuntimeMechanismPinInput struct {
	Codec            typedmemory.CodecRef
	Mechanism        RuntimeMechanismArtifactPin
	ResolvedArtifact *runtimemechanism.RuntimeMechanismArtifactV1
}

func NewCodecRuntimeMechanismPin(
	input CodecRuntimeMechanismPinInput,
) (CodecRuntimeMechanismPin, error) {
	codec, err := validateRuntimeCodecRef(input.Codec)
	if err != nil {
		return CodecRuntimeMechanismPin{}, err
	}
	if !input.Mechanism.valid() {
		return CodecRuntimeMechanismPin{}, fmt.Errorf("codec runtime mechanism artifact pin is required")
	}
	pin := CodecRuntimeMechanismPin{
		codec:     codec,
		mechanism: input.Mechanism,
	}
	resolved, err := validateResolvedRuntimeMechanismForPin(pin, input.ResolvedArtifact)
	if err != nil {
		return CodecRuntimeMechanismPin{}, err
	}
	pin.resolvedCanonical = resolved
	return pin, nil
}

func (pin CodecRuntimeMechanismPin) Codec() typedmemory.CodecRef { return pin.codec }

func (pin CodecRuntimeMechanismPin) Mechanism() RuntimeMechanismArtifactPin {
	return pin.mechanism
}

func (CodecRuntimeMechanismPin) Role() RuntimeMechanismRole {
	return RuntimeMechanismRoleCodec
}

func (CodecRuntimeMechanismPin) InvocationContract() RuntimeMechanismInvocationContract {
	return RuntimeMechanismContractCodecCanonicalization
}

func (pin CodecRuntimeMechanismPin) resolvedRuntimeMechanismCanonical() []byte {
	return append([]byte(nil), pin.resolvedCanonical...)
}

func (pin CodecRuntimeMechanismPin) resolvedRuntimeBasisCanonical() []byte {
	return pin.resolvedRuntimeMechanismCanonical()
}

func (CodecRuntimeMechanismPin) runtimeEvaluationBasisPinVariant() {}

func (CodecRuntimeMechanismPin) runtimeEvaluationMechanismPinVariant() {}

type EvaluatorRuntimeMechanismPin struct {
	rule              typedmemory.RuleRef
	contract          RuntimeMechanismInvocationContract
	mechanism         RuntimeMechanismArtifactPin
	resolvedCanonical []byte
}

type EvaluatorRuntimeMechanismPinInput struct {
	Rule             typedmemory.RuleRef
	Contract         RuntimeMechanismInvocationContract
	Mechanism        RuntimeMechanismArtifactPin
	ResolvedArtifact *runtimemechanism.RuntimeMechanismArtifactV1
}

func NewEvaluatorRuntimeMechanismPin(
	input EvaluatorRuntimeMechanismPinInput,
) (EvaluatorRuntimeMechanismPin, error) {
	rule, err := validateRuntimeRuleRef(input.Rule)
	if err != nil {
		return EvaluatorRuntimeMechanismPin{}, err
	}
	if !input.Mechanism.valid() {
		return EvaluatorRuntimeMechanismPin{}, fmt.Errorf("evaluator runtime mechanism artifact pin is required")
	}
	if !runtimeMechanismContractMatchesRole(input.Contract, RuntimeMechanismRoleEvaluator) {
		return EvaluatorRuntimeMechanismPin{}, fmt.Errorf(
			"evaluator invocation contract %q is invalid",
			input.Contract,
		)
	}
	pin := EvaluatorRuntimeMechanismPin{
		rule:      rule,
		contract:  input.Contract,
		mechanism: input.Mechanism,
	}
	resolved, err := validateResolvedRuntimeMechanismForPin(pin, input.ResolvedArtifact)
	if err != nil {
		return EvaluatorRuntimeMechanismPin{}, err
	}
	pin.resolvedCanonical = resolved
	return pin, nil
}

func (pin EvaluatorRuntimeMechanismPin) Rule() typedmemory.RuleRef { return pin.rule }

func (pin EvaluatorRuntimeMechanismPin) Mechanism() RuntimeMechanismArtifactPin {
	return pin.mechanism
}

func (EvaluatorRuntimeMechanismPin) Role() RuntimeMechanismRole {
	return RuntimeMechanismRoleEvaluator
}

func (pin EvaluatorRuntimeMechanismPin) InvocationContract() RuntimeMechanismInvocationContract {
	return pin.contract
}

func (pin EvaluatorRuntimeMechanismPin) resolvedRuntimeMechanismCanonical() []byte {
	return append([]byte(nil), pin.resolvedCanonical...)
}

func (pin EvaluatorRuntimeMechanismPin) resolvedRuntimeBasisCanonical() []byte {
	return pin.resolvedRuntimeMechanismCanonical()
}

func (EvaluatorRuntimeMechanismPin) runtimeEvaluationBasisPinVariant() {}

func (EvaluatorRuntimeMechanismPin) runtimeEvaluationMechanismPinVariant() {}

type CarrierMembershipRuntimeMechanismPin struct {
	rule              typedmemory.RuleRef
	mechanism         RuntimeMechanismArtifactPin
	resolvedCanonical []byte
}

type CarrierMembershipRuntimeMechanismPinInput struct {
	Rule             typedmemory.RuleRef
	Mechanism        RuntimeMechanismArtifactPin
	ResolvedArtifact *runtimemechanism.RuntimeMechanismArtifactV1
}

func NewCarrierMembershipRuntimeMechanismPin(
	input CarrierMembershipRuntimeMechanismPinInput,
) (CarrierMembershipRuntimeMechanismPin, error) {
	rule, err := validateRuntimeRuleRef(input.Rule)
	if err != nil {
		return CarrierMembershipRuntimeMechanismPin{}, err
	}
	if !input.Mechanism.valid() {
		return CarrierMembershipRuntimeMechanismPin{}, fmt.Errorf("carrier-membership runtime mechanism artifact pin is required")
	}
	pin := CarrierMembershipRuntimeMechanismPin{
		rule:      rule,
		mechanism: input.Mechanism,
	}
	resolved, err := validateResolvedRuntimeMechanismForPin(pin, input.ResolvedArtifact)
	if err != nil {
		return CarrierMembershipRuntimeMechanismPin{}, err
	}
	pin.resolvedCanonical = resolved
	return pin, nil
}

func (pin CarrierMembershipRuntimeMechanismPin) Rule() typedmemory.RuleRef {
	return pin.rule
}

func (pin CarrierMembershipRuntimeMechanismPin) Mechanism() RuntimeMechanismArtifactPin {
	return pin.mechanism
}

func (CarrierMembershipRuntimeMechanismPin) Role() RuntimeMechanismRole {
	return RuntimeMechanismRoleCarrierMembership
}

func (CarrierMembershipRuntimeMechanismPin) InvocationContract() RuntimeMechanismInvocationContract {
	return RuntimeMechanismContractCarrierMembershipDelivery
}

func (pin CarrierMembershipRuntimeMechanismPin) resolvedRuntimeMechanismCanonical() []byte {
	return append([]byte(nil), pin.resolvedCanonical...)
}

func (pin CarrierMembershipRuntimeMechanismPin) resolvedRuntimeBasisCanonical() []byte {
	return pin.resolvedRuntimeMechanismCanonical()
}

func (CarrierMembershipRuntimeMechanismPin) runtimeEvaluationBasisPinVariant() {}

func (CarrierMembershipRuntimeMechanismPin) runtimeEvaluationMechanismPinVariant() {}

// RegistrationPolicyPin binds one exact, fully decoded registration policy
// into X. The resolved bytes describe declared evaluator/delivery/mapping
// policy only and make no executable-code attestation claim.
type RegistrationPolicyPin struct {
	registration      RegistrationPolicyRef
	resolvedCanonical []byte
}

func NewRegistrationPolicyPin(
	artifact RegistrationPolicyArtifact,
) (RegistrationPolicyPin, error) {
	if err := artifact.Verify(); err != nil {
		return RegistrationPolicyPin{}, fmt.Errorf("verify registration-policy artifact: %w", err)
	}
	return RegistrationPolicyPin{
		registration:      artifact.Ref(),
		resolvedCanonical: artifact.CanonicalBytes(),
	}, nil
}

func (pin RegistrationPolicyPin) Registration() RegistrationPolicyRef {
	return pin.registration
}

func (pin RegistrationPolicyPin) resolvedRuntimeBasisCanonical() []byte {
	return append([]byte(nil), pin.resolvedCanonical...)
}

func (RegistrationPolicyPin) runtimeEvaluationBasisPinVariant() {}

// RuntimeEvaluationBasisRef is the content-derived identity of one exact X.
type RuntimeEvaluationBasisRef struct {
	digest typedmemory.SHA256Digest
}

func ParseRuntimeEvaluationBasisRef(raw string) (RuntimeEvaluationBasisRef, error) {
	digestRaw, found := strings.CutPrefix(raw, runtimeEvaluationBasisRefPrefix)
	if !found {
		return RuntimeEvaluationBasisRef{}, fmt.Errorf("runtime evaluation basis reference is malformed")
	}
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return RuntimeEvaluationBasisRef{}, fmt.Errorf("runtime evaluation basis reference: %w", err)
	}
	ref := RuntimeEvaluationBasisRef{digest: digest}
	if ref.String() != raw {
		return RuntimeEvaluationBasisRef{}, fmt.Errorf("runtime evaluation basis reference is not canonical")
	}
	return ref, nil
}

func (ref RuntimeEvaluationBasisRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref RuntimeEvaluationBasisRef) String() string {
	return runtimeEvaluationBasisRefPrefix + ref.digest.String()
}

func (ref RuntimeEvaluationBasisRef) valid() bool {
	parsed, err := ParseRuntimeEvaluationBasisRef(ref.String())
	return err == nil && parsed == ref
}

// RuntimeEvaluationBasisArtifact is immutable content-addressed X. Its bytes
// contain only exact semantic-to-mechanism pins; they cannot carry project
// state, a composite, a Stage, a head, or authority. X proves pin identity and
// closure only. A later composite builder must separately prove that the set is
// necessary and sufficient for the declarations it lowers.
type RuntimeEvaluationBasisArtifact struct {
	ref                          RuntimeEvaluationBasisRef
	canonical                    []byte
	pins                         []RuntimeEvaluationBasisPin
	resolvedMechanisms           [][]byte
	resolvedRegistrationPolicies [][]byte
}

func (artifact RuntimeEvaluationBasisArtifact) Ref() RuntimeEvaluationBasisRef {
	return artifact.ref
}

func (artifact RuntimeEvaluationBasisArtifact) Digest() typedmemory.SHA256Digest {
	return artifact.ref.Digest()
}

func (artifact RuntimeEvaluationBasisArtifact) CanonicalBytes() []byte {
	return append([]byte(nil), artifact.canonical...)
}

func (artifact RuntimeEvaluationBasisArtifact) Pins() []RuntimeEvaluationMechanismPin {
	return runtimeEvaluationMechanismPins(artifact.pins)
}

func (artifact RuntimeEvaluationBasisArtifact) AllPins() []RuntimeEvaluationBasisPin {
	return cloneRuntimeEvaluationBasisPins(artifact.pins)
}

func (artifact RuntimeEvaluationBasisArtifact) RegistrationPolicyPins() []RegistrationPolicyPin {
	return registrationPolicyPins(artifact.pins)
}

// ResolvedRegistrationPolicies returns the exact immutable policy artifacts
// carried by a fully resolved X. It exposes declared registration policy only;
// it does not claim that a runtime evaluator or delivery boundary is loaded.
func (artifact RuntimeEvaluationBasisArtifact) ResolvedRegistrationPolicies() (
	[]RegistrationPolicyArtifact,
	bool,
) {
	if err := artifact.VerifyResolvedClosure(); err != nil {
		return nil, false
	}
	policies, err := decodeResolvedRegistrationPolicies(
		artifact.resolvedRegistrationPolicies,
	)
	if err != nil {
		return nil, false
	}
	return append([]RegistrationPolicyArtifact(nil), policies...), true
}

func (artifact RuntimeEvaluationBasisArtifact) Verify() error {
	if len(artifact.canonical) == 0 {
		return fmt.Errorf("runtime evaluation basis artifact is empty")
	}
	decoded, err := DecodeRuntimeEvaluationBasisArtifact(artifact.canonical)
	if err != nil {
		return fmt.Errorf("verify runtime evaluation basis canonical bytes: %w", err)
	}
	if decoded.ref != artifact.ref {
		return fmt.Errorf("runtime evaluation basis reference is not derived from its bytes")
	}
	if !runtimeEvaluationBasisPinsEqual(decoded.pins, artifact.pins) {
		return fmt.Errorf("runtime evaluation basis stored pins do not match canonical pin order and content")
	}
	storedCanonical, err := encodeRuntimeEvaluationBasisPins(artifact.pins)
	if err != nil {
		return fmt.Errorf("verify runtime evaluation basis stored pins: %w", err)
	}
	if !bytes.Equal(storedCanonical, artifact.canonical) {
		return fmt.Errorf("runtime evaluation basis stored pins do not exactly encode its canonical bytes")
	}
	return nil
}

// VerifyResolvedClosure proves every claimed X pin against exact loaded
// mechanism-catalog and registration-policy bytes. It still makes no claim
// that executable code was loaded or executed.
func (artifact RuntimeEvaluationBasisArtifact) VerifyResolvedClosure() error {
	if err := artifact.Verify(); err != nil {
		return err
	}
	mechanisms, err := decodeResolvedRuntimeMechanisms(artifact.resolvedMechanisms)
	if err != nil {
		return err
	}
	_, err = verifyRuntimeMechanismClosure(
		runtimeEvaluationMechanismPins(artifact.pins),
		mechanisms,
	)
	if err != nil {
		return err
	}
	policies, err := decodeResolvedRegistrationPolicies(
		artifact.resolvedRegistrationPolicies,
	)
	if err != nil {
		return err
	}
	_, err = verifyRegistrationPolicyClosure(
		registrationPolicyPins(artifact.pins),
		policies,
	)
	return err
}

// SealRuntimeEvaluationBasis normalizes exact pins and returns only the result
// of decoding and resealing the derived canonical bytes.
func SealRuntimeEvaluationBasis(
	pins []RuntimeEvaluationMechanismPin,
	artifacts ...runtimemechanism.RuntimeMechanismArtifactV1,
) (RuntimeEvaluationBasisArtifact, error) {
	basisPins := make([]RuntimeEvaluationBasisPin, 0, len(pins))
	for _, pin := range pins {
		basisPins = append(basisPins, pin)
	}
	return SealRuntimeEvaluationBasisWithPins(basisPins, artifacts, nil)
}

// SealRuntimeEvaluationBasisWithPins is the X1 constructor for the complete
// mechanism + registration-policy pin algebra.
func SealRuntimeEvaluationBasisWithPins(
	pins []RuntimeEvaluationBasisPin,
	mechanisms []runtimemechanism.RuntimeMechanismArtifactV1,
	policies []RegistrationPolicyArtifact,
) (RuntimeEvaluationBasisArtifact, error) {
	normalized, err := normalizeRuntimeEvaluationBasisPins(pins)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	canonical, err := encodeRuntimeEvaluationBasisPins(normalized)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	artifact, err := DecodeRuntimeEvaluationBasisArtifact(canonical)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, fmt.Errorf("reseal runtime evaluation basis: %w", err)
	}
	resolvedMechanisms := append(
		[]runtimemechanism.RuntimeMechanismArtifactV1(nil),
		mechanisms...,
	)
	attachedMechanisms, err := runtimeMechanismsAttachedToPins(
		runtimeEvaluationMechanismPins(normalized),
	)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	resolvedMechanisms = append(resolvedMechanisms, attachedMechanisms...)
	resolvedPolicies := append([]RegistrationPolicyArtifact(nil), policies...)
	attachedPolicies, err := registrationPoliciesAttachedToPins(
		registrationPolicyPins(normalized),
	)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	resolvedPolicies = append(resolvedPolicies, attachedPolicies...)
	return ResolveRuntimeEvaluationBasisClosure(
		artifact,
		resolvedMechanisms,
		resolvedPolicies,
	)
}

// ResolveRuntimeEvaluationBasisArtifacts attaches and verifies exact catalogs
// without changing X canonical bytes or identity.
func ResolveRuntimeEvaluationBasisArtifacts(
	basis RuntimeEvaluationBasisArtifact,
	artifacts ...runtimemechanism.RuntimeMechanismArtifactV1,
) (RuntimeEvaluationBasisArtifact, error) {
	return ResolveRuntimeEvaluationBasisClosure(basis, artifacts, nil)
}

func ResolveRuntimeEvaluationBasisRegistrationPolicies(
	basis RuntimeEvaluationBasisArtifact,
	policies ...RegistrationPolicyArtifact,
) (RuntimeEvaluationBasisArtifact, error) {
	return ResolveRuntimeEvaluationBasisClosure(basis, nil, policies)
}

// ResolveRuntimeEvaluationBasisClosure verifies both closure families in one
// pass, so a mixed X can never be partially resolved.
func ResolveRuntimeEvaluationBasisClosure(
	basis RuntimeEvaluationBasisArtifact,
	artifacts []runtimemechanism.RuntimeMechanismArtifactV1,
	policies []RegistrationPolicyArtifact,
) (RuntimeEvaluationBasisArtifact, error) {
	if err := basis.Verify(); err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	existing, err := decodeResolvedRuntimeMechanisms(basis.resolvedMechanisms)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	allArtifacts := append([]runtimemechanism.RuntimeMechanismArtifactV1(nil), existing...)
	allArtifacts = append(allArtifacts, artifacts...)
	resolved, err := verifyRuntimeMechanismClosure(
		runtimeEvaluationMechanismPins(basis.pins),
		allArtifacts,
	)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	existingPolicies, err := decodeResolvedRegistrationPolicies(
		basis.resolvedRegistrationPolicies,
	)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	allPolicies := append([]RegistrationPolicyArtifact(nil), existingPolicies...)
	allPolicies = append(allPolicies, policies...)
	resolvedPolicies, err := verifyRegistrationPolicyClosure(
		registrationPolicyPins(basis.pins),
		allPolicies,
	)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	result := basis
	result.canonical = append([]byte(nil), basis.canonical...)
	result.pins = cloneRuntimeEvaluationBasisPins(basis.pins)
	result.resolvedMechanisms = runtimeMechanismCanonicalBytes(resolved)
	result.resolvedRegistrationPolicies = registrationPolicyCanonicalBytes(
		resolvedPolicies,
	)
	return result, nil
}

// DecodeRuntimeEvaluationBasisArtifact accepts exact canonical X bytes only.
func DecodeRuntimeEvaluationBasisArtifact(
	canonical []byte,
) (RuntimeEvaluationBasisArtifact, error) {
	payload, err := decodeRuntimeEvaluationBasisEnvelope(canonical)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	if !utf8.Valid(payload) {
		return RuntimeEvaluationBasisArtifact{}, fmt.Errorf("runtime evaluation basis payload contains invalid UTF-8")
	}
	encoded := runtimeEvaluationBasisCanonicalV1{}
	err = decodeStrictRuntimeEvaluationBasisJSON(payload, &encoded)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	if len(encoded.Pins) > maximumRuntimeEvaluationBasisPins {
		return RuntimeEvaluationBasisArtifact{}, fmt.Errorf(
			"runtime evaluation basis contains %d pins; limit is %d",
			len(encoded.Pins),
			maximumRuntimeEvaluationBasisPins,
		)
	}
	pins, err := runtimeEvaluationBasisPinsFromCanonical(encoded.Pins)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	normalized, err := normalizeRuntimeEvaluationBasisPins(pins)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	reencoded, err := encodeRuntimeEvaluationBasisPins(normalized)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return RuntimeEvaluationBasisArtifact{}, fmt.Errorf("runtime evaluation basis payload is not canonical")
	}
	digest, err := runtimeEvaluationBasisDigest(reencoded)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	ref := RuntimeEvaluationBasisRef{digest: digest}
	return RuntimeEvaluationBasisArtifact{
		ref:                          ref,
		canonical:                    append([]byte(nil), reencoded...),
		pins:                         cloneRuntimeEvaluationBasisPins(normalized),
		resolvedMechanisms:           nil,
		resolvedRegistrationPolicies: nil,
	}, nil
}

func VerifyRuntimeEvaluationBasisArtifact(
	expected RuntimeEvaluationBasisRef,
	canonical []byte,
) (RuntimeEvaluationBasisArtifact, error) {
	if !expected.valid() {
		return RuntimeEvaluationBasisArtifact{}, fmt.Errorf("expected runtime evaluation basis reference is invalid")
	}
	artifact, err := DecodeRuntimeEvaluationBasisArtifact(canonical)
	if err != nil {
		return RuntimeEvaluationBasisArtifact{}, err
	}
	if artifact.ref != expected {
		return RuntimeEvaluationBasisArtifact{}, fmt.Errorf(
			"runtime evaluation basis reference %q does not match canonical bytes %q",
			expected.String(),
			artifact.ref.String(),
		)
	}
	return artifact, nil
}

type runtimeEvaluationBasisCanonicalV1 struct {
	Pins []runtimeMechanismPinCanonicalV1 `json:"pins"`
}

type runtimeMechanismPinCanonicalV1 struct {
	Kind               string                               `json:"kind"`
	Role               string                               `json:"role,omitempty"`
	InvocationContract string                               `json:"invocation_contract,omitempty"`
	Codec              *runtimeCodecRefCanonicalV1          `json:"codec_ref,omitempty"`
	Rule               *string                              `json:"rule_ref,omitempty"`
	Mechanism          *runtimeMechanismArtifactCanonicalV1 `json:"mechanism,omitempty"`
	Registration       *string                              `json:"registration_policy_ref,omitempty"`
}

type runtimeCodecRefCanonicalV1 struct {
	ID                  string `json:"id"`
	Canonicalization    string `json:"canonicalization_version"`
	SpecificationDigest string `json:"specification_digest"`
}

type runtimeMechanismArtifactCanonicalV1 struct {
	Artifact string `json:"artifact_ref"`
	Edition  string `json:"edition"`
	Digest   string `json:"digest"`
}

func encodeRuntimeEvaluationBasisPins(
	pins []RuntimeEvaluationBasisPin,
) ([]byte, error) {
	normalized, err := normalizeRuntimeEvaluationBasisPins(pins)
	if err != nil {
		return nil, err
	}
	encodedPins := runtimeEvaluationBasisPinsCanonical(normalized)
	encoded := runtimeEvaluationBasisCanonicalV1{Pins: encodedPins}
	payload, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("encode runtime evaluation basis payload: %w", err)
	}
	if !utf8.Valid(payload) {
		return nil, fmt.Errorf("encoded runtime evaluation basis payload contains invalid UTF-8")
	}
	writer := newRuntimeEvaluationBasisWriter(runtimeEvaluationBasisArtifactDomain)
	writer.addBytes(payload)
	result := writer.bytes()
	if len(result) > maximumRuntimeEvaluationBasisBytes {
		return nil, fmt.Errorf(
			"runtime evaluation basis artifact exceeds %d bytes",
			maximumRuntimeEvaluationBasisBytes,
		)
	}
	return result, nil
}

func runtimeEvaluationBasisPinsCanonical(
	pins []RuntimeEvaluationBasisPin,
) []runtimeMechanismPinCanonicalV1 {
	result := make([]runtimeMechanismPinCanonicalV1, 0, len(pins))
	for _, pin := range pins {
		result = append(result, runtimeEvaluationBasisPinCanonical(pin))
	}
	return result
}

func runtimeEvaluationBasisPinCanonical(
	pin RuntimeEvaluationBasisPin,
) runtimeMechanismPinCanonicalV1 {
	switch value := pin.(type) {
	case CodecRuntimeMechanismPin:
		codec := runtimeCodecRefCanonicalV1{
			ID:                  value.codec.ID().String(),
			Canonicalization:    value.codec.Version().String(),
			SpecificationDigest: value.codec.SpecificationDigest().String(),
		}
		mechanism := runtimeMechanismArtifactCanonical(value.mechanism)
		return runtimeMechanismPinCanonicalV1{
			Kind:               "codec",
			Role:               string(value.Role()),
			InvocationContract: value.InvocationContract().String(),
			Codec:              &codec,
			Mechanism:          &mechanism,
		}
	case EvaluatorRuntimeMechanismPin:
		rule := value.rule.String()
		mechanism := runtimeMechanismArtifactCanonical(value.mechanism)
		return runtimeMechanismPinCanonicalV1{
			Kind:               "evaluator",
			Role:               string(value.Role()),
			InvocationContract: value.InvocationContract().String(),
			Rule:               &rule,
			Mechanism:          &mechanism,
		}
	case CarrierMembershipRuntimeMechanismPin:
		rule := value.rule.String()
		mechanism := runtimeMechanismArtifactCanonical(value.mechanism)
		return runtimeMechanismPinCanonicalV1{
			Kind:               "carrier_membership",
			Role:               string(value.Role()),
			InvocationContract: value.InvocationContract().String(),
			Rule:               &rule,
			Mechanism:          &mechanism,
		}
	case RegistrationPolicyPin:
		registration := value.registration.String()
		return runtimeMechanismPinCanonicalV1{
			Kind:         "registration_policy",
			Registration: &registration,
		}
	default:
		return runtimeMechanismPinCanonicalV1{}
	}
}

func runtimeMechanismArtifactCanonical(
	pin RuntimeMechanismArtifactPin,
) runtimeMechanismArtifactCanonicalV1 {
	return runtimeMechanismArtifactCanonicalV1{
		Artifact: pin.artifact.String(),
		Edition:  pin.edition.String(),
		Digest:   pin.digest.String(),
	}
}

func runtimeEvaluationBasisPinsFromCanonical(
	encoded []runtimeMechanismPinCanonicalV1,
) ([]RuntimeEvaluationBasisPin, error) {
	result := make([]RuntimeEvaluationBasisPin, 0, len(encoded))
	for index, value := range encoded {
		pin, err := runtimeEvaluationBasisPinFromCanonical(value)
		if err != nil {
			return nil, fmt.Errorf("decode runtime evaluation basis pin %d: %w", index, err)
		}
		result = append(result, pin)
	}
	return result, nil
}

func runtimeEvaluationBasisPinFromCanonical(
	encoded runtimeMechanismPinCanonicalV1,
) (RuntimeEvaluationBasisPin, error) {
	if encoded.Kind == "registration_policy" {
		return registrationPolicyPinFromCanonical(encoded)
	}
	if encoded.Mechanism == nil || encoded.Registration != nil {
		return nil, fmt.Errorf("runtime mechanism pin requires only mechanism identity")
	}
	mechanism, err := runtimeMechanismArtifactFromCanonical(*encoded.Mechanism)
	if err != nil {
		return nil, err
	}
	switch encoded.Kind {
	case "codec":
		return codecRuntimeMechanismPinFromCanonical(encoded, mechanism)
	case "evaluator":
		return evaluatorRuntimeMechanismPinFromCanonical(encoded, mechanism)
	case "carrier_membership":
		return carrierMembershipRuntimeMechanismPinFromCanonical(encoded, mechanism)
	default:
		return nil, fmt.Errorf("runtime mechanism kind %q is not supported", encoded.Kind)
	}
}

func registrationPolicyPinFromCanonical(
	encoded runtimeMechanismPinCanonicalV1,
) (RuntimeEvaluationBasisPin, error) {
	if encoded.Registration == nil || encoded.Role != "" ||
		encoded.InvocationContract != "" || encoded.Codec != nil ||
		encoded.Rule != nil || encoded.Mechanism != nil {
		return nil, fmt.Errorf("registration-policy pin requires only registration_policy_ref")
	}
	ref, err := ParseRegistrationPolicyRef(*encoded.Registration)
	if err != nil {
		return nil, err
	}
	return RegistrationPolicyPin{registration: ref}, nil
}

func codecRuntimeMechanismPinFromCanonical(
	encoded runtimeMechanismPinCanonicalV1,
	mechanism RuntimeMechanismArtifactPin,
) (RuntimeEvaluationMechanismPin, error) {
	if encoded.Role != string(RuntimeMechanismRoleCodec) {
		return nil, fmt.Errorf("codec runtime mechanism role %q is invalid", encoded.Role)
	}
	if encoded.InvocationContract != RuntimeMechanismContractCodecCanonicalization.String() {
		return nil, fmt.Errorf(
			"codec invocation contract %q is invalid",
			encoded.InvocationContract,
		)
	}
	if encoded.Codec == nil || encoded.Rule != nil {
		return nil, fmt.Errorf("codec runtime mechanism requires only codec_ref")
	}
	codec, err := runtimeCodecRefFromCanonical(*encoded.Codec)
	if err != nil {
		return nil, err
	}
	pin, err := NewCodecRuntimeMechanismPin(CodecRuntimeMechanismPinInput{
		Codec:     codec,
		Mechanism: mechanism,
	})
	if err != nil {
		return nil, err
	}
	return pin, nil
}

func evaluatorRuntimeMechanismPinFromCanonical(
	encoded runtimeMechanismPinCanonicalV1,
	mechanism RuntimeMechanismArtifactPin,
) (RuntimeEvaluationMechanismPin, error) {
	if encoded.Role != string(RuntimeMechanismRoleEvaluator) {
		return nil, fmt.Errorf("evaluator runtime mechanism role %q is invalid", encoded.Role)
	}
	if encoded.Rule == nil || encoded.Codec != nil {
		return nil, fmt.Errorf("evaluator runtime mechanism requires only rule_ref")
	}
	contract, err := parseEvaluatorRuntimeMechanismContract(encoded.InvocationContract)
	if err != nil {
		return nil, err
	}
	rule, err := typedmemory.NewRuleRef(*encoded.Rule)
	if err != nil {
		return nil, fmt.Errorf("decode evaluator RuleRef: %w", err)
	}
	pin, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
		Rule:      rule,
		Contract:  contract,
		Mechanism: mechanism,
	})
	if err != nil {
		return nil, err
	}
	return pin, nil
}

func carrierMembershipRuntimeMechanismPinFromCanonical(
	encoded runtimeMechanismPinCanonicalV1,
	mechanism RuntimeMechanismArtifactPin,
) (RuntimeEvaluationMechanismPin, error) {
	if encoded.Role != string(RuntimeMechanismRoleCarrierMembership) {
		return nil, fmt.Errorf("carrier-membership runtime mechanism role %q is invalid", encoded.Role)
	}
	if encoded.Rule == nil || encoded.Codec != nil {
		return nil, fmt.Errorf("carrier-membership runtime mechanism requires only rule_ref")
	}
	if encoded.InvocationContract != RuntimeMechanismContractCarrierMembershipDelivery.String() {
		return nil, fmt.Errorf(
			"carrier-membership invocation contract %q is invalid",
			encoded.InvocationContract,
		)
	}
	rule, err := typedmemory.NewRuleRef(*encoded.Rule)
	if err != nil {
		return nil, fmt.Errorf("decode carrier-membership RuleRef: %w", err)
	}
	pin, err := NewCarrierMembershipRuntimeMechanismPin(
		CarrierMembershipRuntimeMechanismPinInput{
			Rule:      rule,
			Mechanism: mechanism,
		},
	)
	if err != nil {
		return nil, err
	}
	return pin, nil
}

func runtimeCodecRefFromCanonical(
	encoded runtimeCodecRefCanonicalV1,
) (typedmemory.CodecRef, error) {
	id, err := typedmemory.NewCodecID(encoded.ID)
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("decode runtime CodecRef ID: %w", err)
	}
	version, err := typedmemory.NewCanonicalizationVersion(encoded.Canonicalization)
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("decode runtime CodecRef version: %w", err)
	}
	digest, err := typedmemory.NewSHA256Digest(encoded.SpecificationDigest)
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("decode runtime CodecRef specification digest: %w", err)
	}
	ref, err := typedmemory.NewCodecRef(id, version, digest)
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("decode runtime CodecRef: %w", err)
	}
	return ref, nil
}

func runtimeMechanismArtifactFromCanonical(
	encoded runtimeMechanismArtifactCanonicalV1,
) (RuntimeMechanismArtifactPin, error) {
	artifact, err := typedmemory.NewCarrierRef(encoded.Artifact)
	if err != nil {
		return RuntimeMechanismArtifactPin{}, fmt.Errorf("decode runtime mechanism artifact ref: %w", err)
	}
	edition, err := typedmemory.NewCarrierEdition(encoded.Edition)
	if err != nil {
		return RuntimeMechanismArtifactPin{}, fmt.Errorf("decode runtime mechanism edition: %w", err)
	}
	digest, err := typedmemory.NewSHA256Digest(encoded.Digest)
	if err != nil {
		return RuntimeMechanismArtifactPin{}, fmt.Errorf("decode runtime mechanism digest: %w", err)
	}
	pin, err := NewRuntimeMechanismArtifactPin(RuntimeMechanismArtifactPinInput{
		Artifact: artifact,
		Edition:  edition,
		Digest:   digest,
	})
	if err != nil {
		return RuntimeMechanismArtifactPin{}, err
	}
	return pin, nil
}

func normalizeRuntimeEvaluationBasisPins(
	pins []RuntimeEvaluationBasisPin,
) ([]RuntimeEvaluationBasisPin, error) {
	if len(pins) > maximumRuntimeEvaluationBasisPins {
		return nil, fmt.Errorf(
			"runtime evaluation basis contains %d pins; limit is %d",
			len(pins),
			maximumRuntimeEvaluationBasisPins,
		)
	}
	mechanisms := runtimeEvaluationMechanismPins(pins)
	normalizedMechanisms, err := normalizeRuntimeEvaluationMechanismPins(mechanisms)
	if err != nil {
		return nil, err
	}
	policies := registrationPolicyPins(pins)
	if len(mechanisms)+len(policies) != len(pins) {
		return nil, fmt.Errorf("pin does not belong to the closed runtime-basis algebra")
	}
	for _, policy := range policies {
		if err := validateRegistrationPolicyPin(policy); err != nil {
			return nil, err
		}
	}
	sort.Slice(policies, func(left int, right int) bool {
		return policies[left].registration.String() < policies[right].registration.String()
	})
	for index := 1; index < len(policies); index++ {
		if policies[index-1].registration == policies[index].registration {
			return nil, fmt.Errorf(
				"runtime evaluation basis repeats registration-policy coordinate %q",
				policies[index].registration.String(),
			)
		}
	}
	result := make([]RuntimeEvaluationBasisPin, 0, len(pins))
	for _, mechanism := range normalizedMechanisms {
		result = append(result, mechanism)
	}
	for _, policy := range policies {
		policy.resolvedCanonical = append([]byte(nil), policy.resolvedCanonical...)
		result = append(result, policy)
	}
	sort.Slice(result, func(left int, right int) bool {
		return runtimeEvaluationBasisPinSortKey(result[left]) <
			runtimeEvaluationBasisPinSortKey(result[right])
	})
	return result, nil
}

func validateRegistrationPolicyPin(pin RegistrationPolicyPin) error {
	if err := pin.registration.Verify(); err != nil {
		return fmt.Errorf("registration-policy pin reference is required")
	}
	if len(pin.resolvedCanonical) == 0 {
		return nil
	}
	artifact, err := VerifyRegistrationPolicyArtifact(
		pin.registration,
		pin.resolvedCanonical,
	)
	if err != nil {
		return fmt.Errorf("verify registration-policy pin closure: %w", err)
	}
	return artifact.Verify()
}

func runtimeEvaluationMechanismPins(
	pins []RuntimeEvaluationBasisPin,
) []RuntimeEvaluationMechanismPin {
	result := make([]RuntimeEvaluationMechanismPin, 0, len(pins))
	for _, pin := range pins {
		mechanism, matches := pin.(RuntimeEvaluationMechanismPin)
		if matches {
			result = append(result, mechanism)
		}
	}
	return cloneRuntimeEvaluationMechanismPins(result)
}

func registrationPolicyPins(
	pins []RuntimeEvaluationBasisPin,
) []RegistrationPolicyPin {
	result := make([]RegistrationPolicyPin, 0, len(pins))
	for _, pin := range pins {
		policy, matches := pin.(RegistrationPolicyPin)
		if !matches {
			continue
		}
		policy.resolvedCanonical = append([]byte(nil), policy.resolvedCanonical...)
		result = append(result, policy)
	}
	return result
}

func cloneRuntimeEvaluationBasisPins(
	pins []RuntimeEvaluationBasisPin,
) []RuntimeEvaluationBasisPin {
	result := make([]RuntimeEvaluationBasisPin, 0, len(pins))
	for _, pin := range pins {
		switch value := pin.(type) {
		case RuntimeEvaluationMechanismPin:
			cloned := cloneRuntimeEvaluationMechanismPins(
				[]RuntimeEvaluationMechanismPin{value},
			)
			result = append(result, cloned[0])
		case RegistrationPolicyPin:
			value.resolvedCanonical = append([]byte(nil), value.resolvedCanonical...)
			result = append(result, value)
		default:
			result = append(result, pin)
		}
	}
	return result
}

func runtimeEvaluationBasisPinsEqual(
	left []RuntimeEvaluationBasisPin,
	right []RuntimeEvaluationBasisPin,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		switch leftValue := left[index].(type) {
		case RuntimeEvaluationMechanismPin:
			rightValue, matches := right[index].(RuntimeEvaluationMechanismPin)
			if !matches || !runtimeEvaluationMechanismPinEqual(leftValue, rightValue) {
				return false
			}
		case RegistrationPolicyPin:
			rightValue, matches := right[index].(RegistrationPolicyPin)
			if !matches || leftValue.registration != rightValue.registration {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func runtimeEvaluationBasisPinSortKey(pin RuntimeEvaluationBasisPin) string {
	switch value := pin.(type) {
	case RuntimeEvaluationMechanismPin:
		return "mechanism\x00" + runtimeMechanismPinSortKey(value)
	case RegistrationPolicyPin:
		return "registration_policy\x00" + value.registration.String()
	default:
		return "unknown"
	}
}

func normalizeRuntimeEvaluationMechanismPins(
	pins []RuntimeEvaluationMechanismPin,
) ([]RuntimeEvaluationMechanismPin, error) {
	if len(pins) > maximumRuntimeEvaluationBasisPins {
		return nil, fmt.Errorf(
			"runtime evaluation basis contains %d pins; limit is %d",
			len(pins),
			maximumRuntimeEvaluationBasisPins,
		)
	}
	owned := cloneRuntimeEvaluationMechanismPins(pins)
	for index, pin := range owned {
		if err := validateRuntimeEvaluationMechanismPin(pin); err != nil {
			return nil, fmt.Errorf("runtime evaluation basis pin %d: %w", index, err)
		}
	}
	sort.Slice(owned, func(left, right int) bool {
		return runtimeMechanismPinSortKey(owned[left]) < runtimeMechanismPinSortKey(owned[right])
	})
	semanticCoordinates := make(map[string]struct{}, len(owned))
	mechanismDigests := make(map[string]string, len(owned))
	for _, pin := range owned {
		semantic := runtimeMechanismSemanticCoordinate(pin)
		if _, exists := semanticCoordinates[semantic]; exists {
			return nil, fmt.Errorf("runtime evaluation basis repeats semantic coordinate %q", semantic)
		}
		semanticCoordinates[semantic] = struct{}{}
		mechanism := runtimeMechanismArtifactForPin(pin)
		coordinate := runtimeMechanismArtifactCoordinate(mechanism)
		digest := mechanism.digest.String()
		prior, exists := mechanismDigests[coordinate]
		if exists && prior != digest {
			return nil, fmt.Errorf(
				"runtime mechanism coordinate %q has conflicting content digests %q and %q",
				coordinate,
				prior,
				digest,
			)
		}
		mechanismDigests[coordinate] = digest
	}
	return owned, nil
}

func validateRuntimeEvaluationMechanismPin(pin RuntimeEvaluationMechanismPin) error {
	switch value := pin.(type) {
	case CodecRuntimeMechanismPin:
		_, err := NewCodecRuntimeMechanismPin(CodecRuntimeMechanismPinInput{
			Codec:     value.codec,
			Mechanism: value.mechanism,
		})
		return err
	case EvaluatorRuntimeMechanismPin:
		_, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
			Rule:      value.rule,
			Contract:  value.contract,
			Mechanism: value.mechanism,
		})
		return err
	case CarrierMembershipRuntimeMechanismPin:
		_, err := NewCarrierMembershipRuntimeMechanismPin(
			CarrierMembershipRuntimeMechanismPinInput{
				Rule:      value.rule,
				Mechanism: value.mechanism,
			},
		)
		return err
	default:
		return fmt.Errorf("pin does not belong to the closed runtime mechanism algebra")
	}
}

func cloneRuntimeEvaluationMechanismPins(
	pins []RuntimeEvaluationMechanismPin,
) []RuntimeEvaluationMechanismPin {
	result := make([]RuntimeEvaluationMechanismPin, 0, len(pins))
	for _, pin := range pins {
		switch value := pin.(type) {
		case CodecRuntimeMechanismPin:
			value.resolvedCanonical = append([]byte(nil), value.resolvedCanonical...)
			result = append(result, value)
		case EvaluatorRuntimeMechanismPin:
			value.resolvedCanonical = append([]byte(nil), value.resolvedCanonical...)
			result = append(result, value)
		case CarrierMembershipRuntimeMechanismPin:
			value.resolvedCanonical = append([]byte(nil), value.resolvedCanonical...)
			result = append(result, value)
		default:
			result = append(result, pin)
		}
	}
	return result
}

func runtimeEvaluationMechanismPinEqual(
	left RuntimeEvaluationMechanismPin,
	right RuntimeEvaluationMechanismPin,
) bool {
	switch leftValue := left.(type) {
	case CodecRuntimeMechanismPin:
		rightValue, matches := right.(CodecRuntimeMechanismPin)
		return matches &&
			leftValue.codec == rightValue.codec &&
			leftValue.mechanism == rightValue.mechanism
	case EvaluatorRuntimeMechanismPin:
		rightValue, matches := right.(EvaluatorRuntimeMechanismPin)
		return matches &&
			leftValue.rule == rightValue.rule &&
			leftValue.contract == rightValue.contract &&
			leftValue.mechanism == rightValue.mechanism
	case CarrierMembershipRuntimeMechanismPin:
		rightValue, matches := right.(CarrierMembershipRuntimeMechanismPin)
		return matches &&
			leftValue.rule == rightValue.rule &&
			leftValue.mechanism == rightValue.mechanism
	default:
		return false
	}
}

func runtimeMechanismPinSortKey(pin RuntimeEvaluationMechanismPin) string {
	semantic := runtimeMechanismSemanticCoordinate(pin)
	mechanism := runtimeMechanismArtifactForPin(pin)
	coordinate := runtimeMechanismArtifactCoordinate(mechanism)
	return semantic + "\x00" + coordinate + "\x00" + mechanism.digest.String()
}

func runtimeMechanismSemanticCoordinate(pin RuntimeEvaluationMechanismPin) string {
	switch value := pin.(type) {
	case CodecRuntimeMechanismPin:
		return "codec\x00" + string(value.Role()) + "\x00" +
			value.InvocationContract().String() + "\x00" + value.codec.String()
	case EvaluatorRuntimeMechanismPin:
		return "rule\x00" + string(value.Role()) + "\x00" +
			value.InvocationContract().String() + "\x00" + value.rule.String()
	case CarrierMembershipRuntimeMechanismPin:
		return "rule\x00" + string(value.Role()) + "\x00" +
			value.InvocationContract().String() + "\x00" + value.rule.String()
	default:
		return "unknown"
	}
}

func runtimeMechanismArtifactForPin(
	pin RuntimeEvaluationMechanismPin,
) RuntimeMechanismArtifactPin {
	switch value := pin.(type) {
	case CodecRuntimeMechanismPin:
		return value.mechanism
	case EvaluatorRuntimeMechanismPin:
		return value.mechanism
	case CarrierMembershipRuntimeMechanismPin:
		return value.mechanism
	default:
		return RuntimeMechanismArtifactPin{}
	}
}

func runtimeMechanismArtifactCoordinate(pin RuntimeMechanismArtifactPin) string {
	return pin.artifact.String() + "\x00" + pin.edition.String()
}

func runtimeMechanismArtifactIdentityKey(pin RuntimeMechanismArtifactPin) string {
	return runtimeMechanismArtifactCoordinate(pin) + "\x00" + pin.digest.String()
}

func parseEvaluatorRuntimeMechanismContract(
	raw string,
) (RuntimeMechanismInvocationContract, error) {
	contracts := []RuntimeMechanismInvocationContract{
		RuntimeMechanismContractEntitySetEnumeration,
		RuntimeMechanismContractCandidateVisibility,
		RuntimeMechanismContractKindDefinedness,
		RuntimeMechanismContractMemberOf,
		RuntimeMechanismContractReferenceDesignationResolution,
		RuntimeMechanismContractClaimInterpretation,
		RuntimeMechanismContractClaimMeasurement,
		RuntimeMechanismContractClaimEvaluation,
		RuntimeMechanismContractEpistemeConstitutionEvaluation,
		RuntimeMechanismContractKindClassification,
	}
	for _, contract := range contracts {
		if contract.String() == raw {
			return contract, nil
		}
	}
	return 0, fmt.Errorf("evaluator invocation contract %q is invalid", raw)
}

func runtimeMechanismContractMatchesRole(
	contract RuntimeMechanismInvocationContract,
	role RuntimeMechanismRole,
) bool {
	switch role {
	case RuntimeMechanismRoleCodec:
		return contract == RuntimeMechanismContractCodecCanonicalization
	case RuntimeMechanismRoleEvaluator:
		_, err := parseEvaluatorRuntimeMechanismContract(contract.String())
		return err == nil
	case RuntimeMechanismRoleCarrierMembership:
		return contract == RuntimeMechanismContractCarrierMembershipDelivery
	default:
		return false
	}
}

func validateResolvedRuntimeMechanismForPin(
	pin RuntimeEvaluationMechanismPin,
	artifact *runtimemechanism.RuntimeMechanismArtifactV1,
) ([]byte, error) {
	if artifact == nil {
		return nil, nil
	}
	if err := artifact.Verify(); err != nil {
		return nil, fmt.Errorf("verify resolved runtime mechanism artifact: %w", err)
	}
	mechanism := runtimeMechanismArtifactForPin(pin)
	if !runtimeMechanismIdentityMatchesPin(mechanism, *artifact) {
		return nil, fmt.Errorf(
			"resolved runtime mechanism artifact identity does not match claimed identity %q",
			runtimeMechanismArtifactIdentityKey(mechanism),
		)
	}
	if !runtimeMechanismArtifactContainsPin(*artifact, pin) {
		return nil, fmt.Errorf(
			"resolved runtime mechanism artifact does not contain exact entry %q",
			runtimeMechanismSemanticCoordinate(pin),
		)
	}
	return artifact.CanonicalBytes(), nil
}

func runtimeMechanismIdentityMatchesPin(
	pin RuntimeMechanismArtifactPin,
	artifact runtimemechanism.RuntimeMechanismArtifactV1,
) bool {
	identity := artifact.Identity()
	return pin.artifact == identity.Artifact() &&
		pin.edition == identity.Edition() &&
		pin.digest == identity.Digest()
}

func runtimeMechanismArtifactContainsPin(
	artifact runtimemechanism.RuntimeMechanismArtifactV1,
	pin RuntimeEvaluationMechanismPin,
) bool {
	for _, entry := range artifact.Entries() {
		if entry.Role().String() != string(pin.Role()) {
			continue
		}
		if entry.Contract() != pin.InvocationContract() {
			continue
		}
		if runtimeMechanismEntryMatchesSemantic(entry, pin) {
			return true
		}
	}
	return false
}

func runtimeMechanismEntryMatchesSemantic(
	entry runtimemechanism.RuntimeMechanismEntryV1,
	pin RuntimeEvaluationMechanismPin,
) bool {
	switch value := pin.(type) {
	case CodecRuntimeMechanismPin:
		coordinate, matches := entry.Semantic().(runtimemechanism.CodecSemanticCoordinate)
		return matches && coordinate.Ref() == value.codec
	case EvaluatorRuntimeMechanismPin:
		coordinate, matches := entry.Semantic().(runtimemechanism.RuleSemanticCoordinate)
		return matches && coordinate.Ref() == value.rule
	case CarrierMembershipRuntimeMechanismPin:
		coordinate, matches := entry.Semantic().(runtimemechanism.RuleSemanticCoordinate)
		return matches && coordinate.Ref() == value.rule
	default:
		return false
	}
}

func runtimeMechanismsAttachedToPins(
	pins []RuntimeEvaluationMechanismPin,
) ([]runtimemechanism.RuntimeMechanismArtifactV1, error) {
	result := make([]runtimemechanism.RuntimeMechanismArtifactV1, 0)
	for index, pin := range pins {
		canonical := pin.resolvedRuntimeMechanismCanonical()
		if len(canonical) == 0 {
			continue
		}
		artifact, err := runtimemechanism.DecodeRuntimeMechanismArtifactV1(canonical)
		if err != nil {
			return nil, fmt.Errorf(
				"decode runtime mechanism attached to pin %d: %w",
				index,
				err,
			)
		}
		result = append(result, artifact)
	}
	return result, nil
}

func registrationPoliciesAttachedToPins(
	pins []RegistrationPolicyPin,
) ([]RegistrationPolicyArtifact, error) {
	result := make([]RegistrationPolicyArtifact, 0, len(pins))
	for index, pin := range pins {
		canonical := pin.resolvedRuntimeBasisCanonical()
		if len(canonical) == 0 {
			continue
		}
		artifact, err := VerifyRegistrationPolicyArtifact(
			pin.registration,
			canonical,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"decode registration policy attached to pin %d: %w",
				index,
				err,
			)
		}
		result = append(result, artifact)
	}
	return result, nil
}

func decodeResolvedRuntimeMechanisms(
	values [][]byte,
) ([]runtimemechanism.RuntimeMechanismArtifactV1, error) {
	result := make([]runtimemechanism.RuntimeMechanismArtifactV1, 0, len(values))
	for index, canonical := range values {
		artifact, err := runtimemechanism.DecodeRuntimeMechanismArtifactV1(canonical)
		if err != nil {
			return nil, fmt.Errorf(
				"decode resolved runtime mechanism artifact %d: %w",
				index,
				err,
			)
		}
		result = append(result, artifact)
	}
	return result, nil
}

func decodeResolvedRegistrationPolicies(
	values [][]byte,
) ([]RegistrationPolicyArtifact, error) {
	result := make([]RegistrationPolicyArtifact, 0, len(values))
	for index, canonical := range values {
		artifact, err := DecodeRegistrationPolicyArtifact(canonical)
		if err != nil {
			return nil, fmt.Errorf(
				"decode resolved registration-policy artifact %d: %w",
				index,
				err,
			)
		}
		result = append(result, artifact)
	}
	return result, nil
}

func verifyRegistrationPolicyClosure(
	pins []RegistrationPolicyPin,
	artifacts []RegistrationPolicyArtifact,
) ([]RegistrationPolicyArtifact, error) {
	normalizedPins := append([]RegistrationPolicyPin(nil), pins...)
	sort.Slice(normalizedPins, func(left int, right int) bool {
		return normalizedPins[left].Registration().String() <
			normalizedPins[right].Registration().String()
	})
	for index, pin := range normalizedPins {
		if err := validateRegistrationPolicyPin(pin); err != nil {
			return nil, fmt.Errorf("verify registration-policy pin %d: %w", index, err)
		}
	}
	for index := 1; index < len(normalizedPins); index++ {
		if normalizedPins[index-1].Registration() == normalizedPins[index].Registration() {
			return nil, fmt.Errorf(
				"runtime evaluation basis repeats registration-policy coordinate %q",
				normalizedPins[index].Registration().String(),
			)
		}
	}
	verified := make([]RegistrationPolicyArtifact, 0, len(artifacts))
	for index, artifact := range artifacts {
		if err := artifact.Verify(); err != nil {
			return nil, fmt.Errorf("verify registration-policy artifact %d: %w", index, err)
		}
		verified = append(verified, artifact)
	}
	sort.Slice(verified, func(left int, right int) bool {
		return verified[left].Ref().String() < verified[right].Ref().String()
	})
	for index := 1; index < len(verified); index++ {
		if verified[index-1].Ref() == verified[index].Ref() {
			return nil, fmt.Errorf(
				"resolved registration-policy artifact %q is duplicated",
				verified[index].Ref().String(),
			)
		}
	}
	pinned := make(map[RegistrationPolicyRef]struct{}, len(normalizedPins))
	for _, pin := range normalizedPins {
		pinned[pin.Registration()] = struct{}{}
	}
	resolved := make(map[RegistrationPolicyRef]struct{}, len(verified))
	for _, artifact := range verified {
		ref := artifact.Ref()
		resolved[ref] = struct{}{}
		if _, exists := pinned[ref]; exists {
			continue
		}
		return nil, fmt.Errorf(
			"resolved registration-policy artifact %q is not referenced by X",
			ref.String(),
		)
	}
	for _, pin := range normalizedPins {
		if _, exists := resolved[pin.Registration()]; exists {
			continue
		}
		return nil, fmt.Errorf(
			"registration-policy artifact %q required by X is not resolved",
			pin.Registration().String(),
		)
	}
	return verified, nil
}

func registrationPolicyCanonicalBytes(
	artifacts []RegistrationPolicyArtifact,
) [][]byte {
	result := make([][]byte, 0, len(artifacts))
	for _, artifact := range artifacts {
		result = append(result, artifact.CanonicalBytes())
	}
	return result
}

func verifyRuntimeMechanismClosure(
	pins []RuntimeEvaluationMechanismPin,
	artifacts []runtimemechanism.RuntimeMechanismArtifactV1,
) ([]runtimemechanism.RuntimeMechanismArtifactV1, error) {
	normalizedPins, err := normalizeRuntimeEvaluationMechanismPins(pins)
	if err != nil {
		return nil, err
	}
	normalizedArtifacts, err := normalizeResolvedRuntimeMechanisms(artifacts)
	if err != nil {
		return nil, err
	}
	byIdentity := make(map[string]runtimemechanism.RuntimeMechanismArtifactV1, len(normalizedArtifacts))
	for _, artifact := range normalizedArtifacts {
		pin, pinErr := NewRuntimeMechanismArtifactPinFromArtifact(artifact)
		if pinErr != nil {
			return nil, pinErr
		}
		byIdentity[runtimeMechanismArtifactIdentityKey(pin)] = artifact
	}
	used := make(map[string]struct{}, len(normalizedPins))
	for _, pin := range normalizedPins {
		mechanism := runtimeMechanismArtifactForPin(pin)
		identityKey := runtimeMechanismArtifactIdentityKey(mechanism)
		artifact, found := byIdentity[identityKey]
		if !found {
			return nil, fmt.Errorf(
				"runtime mechanism artifact %q required by entry %q is not resolved",
				identityKey,
				runtimeMechanismSemanticCoordinate(pin),
			)
		}
		if !runtimeMechanismArtifactContainsPin(artifact, pin) {
			return nil, fmt.Errorf(
				"runtime mechanism artifact %q does not contain exact role-contract-semantic entry %q",
				identityKey,
				runtimeMechanismSemanticCoordinate(pin),
			)
		}
		used[identityKey] = struct{}{}
	}
	for _, artifact := range normalizedArtifacts {
		pin, pinErr := NewRuntimeMechanismArtifactPinFromArtifact(artifact)
		if pinErr != nil {
			return nil, pinErr
		}
		identityKey := runtimeMechanismArtifactIdentityKey(pin)
		if _, found := used[identityKey]; found {
			continue
		}
		return nil, fmt.Errorf(
			"resolved runtime mechanism artifact %q is not referenced by X",
			identityKey,
		)
	}
	return normalizedArtifacts, nil
}

func normalizeResolvedRuntimeMechanisms(
	artifacts []runtimemechanism.RuntimeMechanismArtifactV1,
) ([]runtimemechanism.RuntimeMechanismArtifactV1, error) {
	type resolvedArtifact struct {
		artifact   runtimemechanism.RuntimeMechanismArtifactV1
		identity   string
		coordinate string
		digest     string
	}
	validated := make([]resolvedArtifact, 0, len(artifacts))
	for index, artifact := range artifacts {
		if err := artifact.Verify(); err != nil {
			return nil, fmt.Errorf("verify runtime mechanism artifact %d: %w", index, err)
		}
		pin, err := NewRuntimeMechanismArtifactPinFromArtifact(artifact)
		if err != nil {
			return nil, err
		}
		validated = append(validated, resolvedArtifact{
			artifact:   artifact,
			identity:   runtimeMechanismArtifactIdentityKey(pin),
			coordinate: runtimeMechanismArtifactCoordinate(pin),
			digest:     pin.digest.String(),
		})
	}
	sort.Slice(validated, func(left int, right int) bool {
		return validated[left].identity < validated[right].identity
	})
	coordinateDigests := make(map[string]string, len(validated))
	result := make([]runtimemechanism.RuntimeMechanismArtifactV1, 0, len(validated))
	var priorIdentity string
	for _, value := range validated {
		priorDigest, found := coordinateDigests[value.coordinate]
		if found && priorDigest != value.digest {
			return nil, fmt.Errorf(
				"runtime mechanism coordinate %q has conflicting resolved digests %q and %q",
				value.coordinate,
				priorDigest,
				value.digest,
			)
		}
		coordinateDigests[value.coordinate] = value.digest
		if value.identity == priorIdentity {
			continue
		}
		result = append(result, value.artifact)
		priorIdentity = value.identity
	}
	return result, nil
}

func runtimeMechanismCanonicalBytes(
	artifacts []runtimemechanism.RuntimeMechanismArtifactV1,
) [][]byte {
	result := make([][]byte, 0, len(artifacts))
	for _, artifact := range artifacts {
		result = append(result, artifact.CanonicalBytes())
	}
	return result
}

func validateRuntimeCodecRef(ref typedmemory.CodecRef) (typedmemory.CodecRef, error) {
	id, err := typedmemory.NewCodecID(ref.ID().String())
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec reference ID is required")
	}
	version, err := typedmemory.NewCanonicalizationVersion(ref.Version().String())
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec canonicalization version is required")
	}
	digest, err := validateRuntimeMechanismDigest(ref.SpecificationDigest())
	if err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec specification digest: %w", err)
	}
	rebuilt, err := typedmemory.NewCodecRef(id, version, digest)
	if err != nil || rebuilt != ref {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec reference is invalid")
	}
	texts := []string{id.String(), version.String(), digest.String()}
	if err := validateRuntimeMechanismTexts(texts); err != nil {
		return typedmemory.CodecRef{}, fmt.Errorf("runtime codec reference: %w", err)
	}
	return rebuilt, nil
}

func validateRuntimeRuleRef(ref typedmemory.RuleRef) (typedmemory.RuleRef, error) {
	rebuilt, err := typedmemory.NewRuleRef(ref.String())
	if err != nil || rebuilt != ref {
		return typedmemory.RuleRef{}, fmt.Errorf("runtime evaluator RuleRef is invalid")
	}
	if err := validateRuntimeMechanismText(rebuilt.String()); err != nil {
		return typedmemory.RuleRef{}, fmt.Errorf("runtime evaluator RuleRef: %w", err)
	}
	if len(rebuilt.String()) > maximumRuntimeMechanismRuleRefBytes {
		return typedmemory.RuleRef{}, fmt.Errorf(
			"runtime evaluator RuleRef exceeds %d bytes",
			maximumRuntimeMechanismRuleRefBytes,
		)
	}
	return rebuilt, nil
}

func validateRuntimeMechanismCarrierRef(
	ref typedmemory.CarrierRef,
) (typedmemory.CarrierRef, error) {
	rebuilt, err := typedmemory.NewCarrierRef(ref.String())
	if err != nil || rebuilt != ref {
		return typedmemory.CarrierRef{}, fmt.Errorf("runtime mechanism artifact reference is invalid")
	}
	if err := validateRuntimeMechanismText(rebuilt.String()); err != nil {
		return typedmemory.CarrierRef{}, fmt.Errorf("runtime mechanism artifact reference: %w", err)
	}
	if len(rebuilt.String()) > maximumRuntimeMechanismCoordinateBytes {
		return typedmemory.CarrierRef{}, fmt.Errorf(
			"runtime mechanism artifact reference exceeds %d bytes",
			maximumRuntimeMechanismCoordinateBytes,
		)
	}
	return rebuilt, nil
}

func validateRuntimeMechanismEdition(
	edition typedmemory.CarrierEdition,
) (typedmemory.CarrierEdition, error) {
	rebuilt, err := typedmemory.NewCarrierEdition(edition.String())
	if err != nil || rebuilt != edition {
		return typedmemory.CarrierEdition{}, fmt.Errorf("runtime mechanism edition is invalid")
	}
	if err := validateRuntimeMechanismText(rebuilt.String()); err != nil {
		return typedmemory.CarrierEdition{}, fmt.Errorf("runtime mechanism edition: %w", err)
	}
	if len(rebuilt.String()) > maximumRuntimeMechanismCoordinateBytes {
		return typedmemory.CarrierEdition{}, fmt.Errorf(
			"runtime mechanism edition exceeds %d bytes",
			maximumRuntimeMechanismCoordinateBytes,
		)
	}
	if !exactRuntimeMechanismEdition(rebuilt.String()) {
		return typedmemory.CarrierEdition{}, fmt.Errorf(
			"runtime mechanism edition must be an exact semantic version or immutable build edition",
		)
	}
	return rebuilt, nil
}

func validateRuntimeMechanismDigest(
	digest typedmemory.SHA256Digest,
) (typedmemory.SHA256Digest, error) {
	rebuilt, err := typedmemory.NewSHA256Digest(digest.String())
	if err != nil || rebuilt != digest {
		return typedmemory.SHA256Digest{}, fmt.Errorf("runtime mechanism SHA-256 digest is invalid")
	}
	return rebuilt, nil
}

func validateRuntimeMechanismTexts(values []string) error {
	for _, value := range values {
		if err := validateRuntimeMechanismText(value); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeMechanismText(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("contains invalid UTF-8")
	}
	if len(value) > maximumRuntimeMechanismTextBytes {
		return fmt.Errorf("exceeds %d bytes", maximumRuntimeMechanismTextBytes)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("contains a control character")
		}
	}
	return nil
}

func exactRuntimeMechanismEdition(value string) bool {
	return runtimeMechanismExactSemanticVersion.MatchString(value) ||
		runtimeMechanismExactBuildEdition.MatchString(value)
}

func runtimeEvaluationBasisDigest(
	canonical []byte,
) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	hexDigest := hex.EncodeToString(sum[:])
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hexDigest)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	return digest, nil
}

func decodeStrictRuntimeEvaluationBasisJSON(
	payload []byte,
	target *runtimeEvaluationBasisCanonicalV1,
) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode runtime evaluation basis payload: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("runtime evaluation basis JSON has a trailing value")
	}
	return fmt.Errorf("decode runtime evaluation basis trailing JSON: %w", err)
}

type runtimeEvaluationBasisWriter struct {
	buffer bytes.Buffer
}

func newRuntimeEvaluationBasisWriter(domain string) runtimeEvaluationBasisWriter {
	writer := runtimeEvaluationBasisWriter{}
	writer.addString(runtimeEvaluationBasisCanonicalDomain)
	writer.addString(domain)
	return writer
}

func (writer *runtimeEvaluationBasisWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *runtimeEvaluationBasisWriter) addBytes(value []byte) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(len(value)))
	writer.buffer.Write(encoded[:])
	writer.buffer.Write(value)
}

func (writer runtimeEvaluationBasisWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type runtimeEvaluationBasisReader struct {
	data   []byte
	offset int
}

func decodeRuntimeEvaluationBasisEnvelope(canonical []byte) ([]byte, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("runtime evaluation basis canonical bytes are required")
	}
	if len(canonical) > maximumRuntimeEvaluationBasisBytes {
		return nil, fmt.Errorf(
			"runtime evaluation basis canonical bytes exceed %d-byte limit",
			maximumRuntimeEvaluationBasisBytes,
		)
	}
	reader := &runtimeEvaluationBasisReader{data: canonical}
	root, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode runtime evaluation basis root domain: %w", err)
	}
	if root != runtimeEvaluationBasisCanonicalDomain {
		return nil, fmt.Errorf("unexpected runtime evaluation basis root domain %q", root)
	}
	domain, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode runtime evaluation basis artifact domain: %w", err)
	}
	if domain != runtimeEvaluationBasisArtifactDomain {
		return nil, fmt.Errorf("unexpected runtime evaluation basis artifact domain %q", domain)
	}
	payload, err := reader.readBytes()
	if err != nil {
		return nil, fmt.Errorf("decode runtime evaluation basis payload: %w", err)
	}
	if reader.offset != len(reader.data) {
		return nil, fmt.Errorf(
			"runtime evaluation basis payload has %d trailing bytes",
			len(reader.data)-reader.offset,
		)
	}
	return append([]byte(nil), payload...), nil
}

func (reader *runtimeEvaluationBasisReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("canonical domain contains invalid UTF-8")
	}
	return string(value), nil
}

func (reader *runtimeEvaluationBasisReader) readBytes() ([]byte, error) {
	if reader == nil || len(reader.data)-reader.offset < 8 {
		return nil, fmt.Errorf("unexpected end of length-prefixed field")
	}
	endLength := reader.offset + 8
	length := binary.BigEndian.Uint64(reader.data[reader.offset:endLength])
	reader.offset = endLength
	remaining := len(reader.data) - reader.offset
	//nolint:gosec // remaining is non-negative after the reader bounds check above.
	if length > uint64(remaining) {
		return nil, fmt.Errorf(
			"length-prefixed field %d exceeds remaining payload %d",
			length,
			remaining,
		)
	}
	if length > maximumRuntimeEvaluationBasisBytes {
		return nil, fmt.Errorf(
			"length-prefixed field exceeds %d bytes",
			maximumRuntimeEvaluationBasisBytes,
		)
	}
	boundedLength := int(length)
	end := reader.offset + boundedLength
	value := reader.data[reader.offset:end]
	reader.offset = end
	return value, nil
}
