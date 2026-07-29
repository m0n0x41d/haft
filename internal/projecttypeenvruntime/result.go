package projecttypeenvruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/kindclassificationruntime"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

var ErrExactTargetRuntimeRegistryNotSerializable = errors.New(
	"ExactTargetRuntimeRegistry is an in-process runtime observation and cannot be serialized",
)

// Each evaluator contract owns a distinct typed registry; there is no
// universal untyped mechanism map. MemberOf is generic only at its exact
// store-facing C.3.2 boundary: each RuleRef still selects one package-owned
// evaluator family and exact mechanism identity.
type MemberOfEvaluatorRegistry = memberofruntime.Registry

type KindClassificationEvaluatorRegistry = kindclassificationruntime.Registry

type EntitySetEnumerationEvaluatorRegistry = typedmemoryevaluation.EntitySetEnumerationRegistry

type CandidateVisibilityEvaluatorRegistry = typedmemoryevaluation.CandidateVisibilityRegistry

type KindDefinednessEvaluatorRegistry = typedmemoryevaluation.KindDefinednessRegistry

type ReferenceDesignationResolutionEvaluatorRegistry = typedmemoryevaluation.ReferenceDesignationResolutionRegistry

type ClaimInterpretationEvaluatorRegistry = typedmemoryevaluation.ClaimInterpretationRegistry

type ClaimMeasurementEvaluatorRegistry = typedmemoryevaluation.ClaimMeasurementRegistry

type ClaimEvaluationEvaluatorRegistry = typedmemoryevaluation.ClaimEvaluationRegistry

type EpistemeConstitutionEvaluatorRegistry = typedmemoryevaluation.EpistemeConstitutionEvaluationRegistry

// ResolutionKind is the closed top-level posture of runtime observation.
type ResolutionKind uint8

const (
	ResolutionMatched ResolutionKind = iota + 1
	ResolutionInvalid
	ResolutionUnavailable
	ResolutionDrifted
)

func (kind ResolutionKind) String() string {
	switch kind {
	case ResolutionMatched:
		return "matched"
	case ResolutionInvalid:
		return "invalid"
	case ResolutionUnavailable:
		return "unavailable"
	case ResolutionDrifted:
		return "drifted"
	default:
		return ""
	}
}

// Resolution is a closed exact-runtime comparison result.
type Resolution interface {
	Kind() ResolutionKind
	runtimeRegistryResolutionVariant()
}

// TargetRegistrationPolicy is the closed presence posture of a policy selected
// by X. ExactTargetRegistrationPolicy is available only after identity and
// mechanism-coordinate comparison.
type TargetRegistrationPolicy interface {
	targetRegistrationPolicyVariant()
}

type NoTargetRegistrationPolicy struct{}

func (NoTargetRegistrationPolicy) targetRegistrationPolicyVariant() {}

type ExactTargetRegistrationPolicy struct {
	artifact recordmembershipregistration.RegistrationArtifactV1
}

func (ExactTargetRegistrationPolicy) targetRegistrationPolicyVariant() {}

func (policy ExactTargetRegistrationPolicy) Artifact() (
	recordmembershipregistration.RegistrationArtifactV1,
	bool,
) {
	if err := policy.artifact.Verify(); err != nil {
		return recordmembershipregistration.RegistrationArtifactV1{}, false
	}
	return policy.artifact, true
}

// TargetRegistrationPolicies is the closed policy posture of one exact X.
// Membership-free X has NoTargetRegistrationPolicies. Membership-capable X
// has one immutable exact policy per MemberOf evaluator RuleRef.
type TargetRegistrationPolicies interface {
	Len() int
	targetRegistrationPoliciesVariant()
}

type NoTargetRegistrationPolicies struct{}

func (NoTargetRegistrationPolicies) Len() int { return 0 }

func (NoTargetRegistrationPolicies) targetRegistrationPoliciesVariant() {}

// ExactTargetRegistrationPolicyRegistry is canonically ordered by evaluator
// RuleRef. Its private storage makes duplicate and substituted family policies
// inexpressible after runtime refinement.
type ExactTargetRegistrationPolicyRegistry struct {
	artifacts []recordmembershipregistration.RegistrationArtifactV1
}

func (registry ExactTargetRegistrationPolicyRegistry) Len() int {
	return len(registry.artifacts)
}

func (ExactTargetRegistrationPolicyRegistry) targetRegistrationPoliciesVariant() {}

func (registry ExactTargetRegistrationPolicyRegistry) Artifacts() (
	[]recordmembershipregistration.RegistrationArtifactV1,
	bool,
) {
	if err := verifyExactTargetRegistrationPolicyRegistry(registry); err != nil {
		return nil, false
	}
	return append(
		[]recordmembershipregistration.RegistrationArtifactV1(nil),
		registry.artifacts...,
	), true
}

func (registry ExactTargetRegistrationPolicyRegistry) Lookup(
	evaluatorRule typedmemory.RuleRef,
) (ExactTargetRegistrationPolicy, bool) {
	if err := verifyExactTargetRegistrationPolicyRegistry(registry); err != nil {
		return ExactTargetRegistrationPolicy{}, false
	}
	parsed, err := typedmemory.NewRuleRef(evaluatorRule.String())
	if err != nil || parsed != evaluatorRule {
		return ExactTargetRegistrationPolicy{}, false
	}
	index := sort.Search(len(registry.artifacts), func(index int) bool {
		return registry.artifacts[index].Evaluator().Rule().String() >= evaluatorRule.String()
	})
	if index == len(registry.artifacts) ||
		registry.artifacts[index].Evaluator().Rule() != evaluatorRule {
		return ExactTargetRegistrationPolicy{}, false
	}
	return ExactTargetRegistrationPolicy{artifact: registry.artifacts[index]}, true
}

type invalidTargetRegistrationPolicies struct{}

func (invalidTargetRegistrationPolicies) Len() int { return 0 }

func (invalidTargetRegistrationPolicies) targetRegistrationPoliciesVariant() {}

func newExactTargetRegistrationPolicyRegistry(
	artifacts []recordmembershipregistration.RegistrationArtifactV1,
) (ExactTargetRegistrationPolicyRegistry, error) {
	owned := append(
		[]recordmembershipregistration.RegistrationArtifactV1(nil),
		artifacts...,
	)
	sort.Slice(owned, func(left int, right int) bool {
		return owned[left].Evaluator().Rule().String() <
			owned[right].Evaluator().Rule().String()
	})
	registry := ExactTargetRegistrationPolicyRegistry{artifacts: owned}
	if err := verifyExactTargetRegistrationPolicyRegistry(registry); err != nil {
		return ExactTargetRegistrationPolicyRegistry{}, err
	}
	return registry, nil
}

func verifyExactTargetRegistrationPolicyRegistry(
	registry ExactTargetRegistrationPolicyRegistry,
) error {
	if len(registry.artifacts) == 0 {
		return fmt.Errorf("exact target registration-policy registry is empty")
	}
	prior := ""
	for index, artifact := range registry.artifacts {
		if err := artifact.Verify(); err != nil {
			return fmt.Errorf("verify target registration policy %d: %w", index, err)
		}
		rule := artifact.Evaluator().Rule().String()
		parsed, err := typedmemory.NewRuleRef(rule)
		if err != nil || parsed != artifact.Evaluator().Rule() {
			return fmt.Errorf("target registration policy %d has invalid evaluator RuleRef", index)
		}
		if prior != "" && prior >= rule {
			return fmt.Errorf("target registration-policy evaluator RuleRef %q is duplicated or unordered", rule)
		}
		prior = rule
	}
	return nil
}

type exactTargetRuntimeRegistryCapability struct{}

type exactTargetRuntimeRegistryState struct {
	runtimeBasis                   projecttypeenv.RuntimeEvaluationBasisArtifact
	codecs                         typedmemory.CodecRegistry
	entitySetEnumeration           EntitySetEnumerationEvaluatorRegistry
	candidateVisibility            CandidateVisibilityEvaluatorRegistry
	kindDefinedness                KindDefinednessEvaluatorRegistry
	memberOf                       MemberOfEvaluatorRegistry
	kindClassification             KindClassificationEvaluatorRegistry
	referenceDesignationResolution ReferenceDesignationResolutionEvaluatorRegistry
	claimInterpretation            ClaimInterpretationEvaluatorRegistry
	claimMeasurement               ClaimMeasurementEvaluatorRegistry
	claimEvaluation                ClaimEvaluationEvaluatorRegistry
	epistemeConstitution           EpistemeConstitutionEvaluatorRegistry
	mechanismCatalogs              []runtimemechanism.RuntimeMechanismArtifactV1
	policies                       TargetRegistrationPolicies
	digest                         typedmemory.SHA256Digest
}

// ExactTargetRuntimeRegistry is the non-serializable result of matching the
// current in-process registries against one exact X. It proves configured
// coordinates and callable presence, not executable-byte attestation, head
// selection, authority, or performed evaluation. Evaluator getters expose
// only registrations pinned by this X; unrelated installed callables are not
// executable through the refinement.
type ExactTargetRuntimeRegistry struct {
	state      *exactTargetRuntimeRegistryState
	capability *exactTargetRuntimeRegistryCapability
}

func (registry ExactTargetRuntimeRegistry) Valid() bool {
	if registry.state == nil || registry.capability == nil {
		return false
	}
	return verifyExactTargetRuntimeRegistryState(*registry.state) == nil
}

func (registry ExactTargetRuntimeRegistry) RuntimeBasisRef() (
	projecttypeenv.RuntimeEvaluationBasisRef,
	bool,
) {
	if !registry.Valid() {
		return projecttypeenv.RuntimeEvaluationBasisRef{}, false
	}
	return registry.state.runtimeBasis.Ref(), true
}

func (registry ExactTargetRuntimeRegistry) CoordinateDigest() (
	typedmemory.SHA256Digest,
	bool,
) {
	if !registry.Valid() {
		return typedmemory.SHA256Digest{}, false
	}
	return registry.state.digest, true
}

func (registry ExactTargetRuntimeRegistry) CodecRegistry() (
	typedmemory.CodecRegistry,
	bool,
) {
	if !registry.Valid() {
		return typedmemory.CodecRegistry{}, false
	}
	return registry.state.codecs, true
}

func (registry ExactTargetRuntimeRegistry) MemberOfRegistry() (
	MemberOfEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return MemberOfEvaluatorRegistry{}, false
	}
	return registry.state.memberOf.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) KindClassificationRegistry() (
	KindClassificationEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return KindClassificationEvaluatorRegistry{}, false
	}
	return registry.state.kindClassification.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) EntitySetEnumerationRegistry() (
	EntitySetEnumerationEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return EntitySetEnumerationEvaluatorRegistry{}, false
	}
	return registry.state.entitySetEnumeration.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) KindDefinednessRegistry() (
	KindDefinednessEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return KindDefinednessEvaluatorRegistry{}, false
	}
	return registry.state.kindDefinedness.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) CandidateVisibilityRegistry() (
	CandidateVisibilityEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return CandidateVisibilityEvaluatorRegistry{}, false
	}
	return registry.state.candidateVisibility.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) ReferenceDesignationResolutionRegistry() (
	ReferenceDesignationResolutionEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return ReferenceDesignationResolutionEvaluatorRegistry{}, false
	}
	return registry.state.referenceDesignationResolution.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) ClaimInterpretationRegistry() (
	ClaimInterpretationEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return ClaimInterpretationEvaluatorRegistry{}, false
	}
	return registry.state.claimInterpretation.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) ClaimMeasurementRegistry() (
	ClaimMeasurementEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return ClaimMeasurementEvaluatorRegistry{}, false
	}
	return registry.state.claimMeasurement.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) ClaimEvaluationRegistry() (
	ClaimEvaluationEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return ClaimEvaluationEvaluatorRegistry{}, false
	}
	return registry.state.claimEvaluation.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) EpistemeConstitutionEvaluationRegistry() (
	EpistemeConstitutionEvaluatorRegistry,
	bool,
) {
	if !registry.Valid() {
		return EpistemeConstitutionEvaluatorRegistry{}, false
	}
	return registry.state.epistemeConstitution.Clone(), true
}

func (registry ExactTargetRuntimeRegistry) RegistrationPolicy() (
	TargetRegistrationPolicy,
	bool,
) {
	if !registry.Valid() {
		return nil, false
	}
	switch policies := registry.state.policies.(type) {
	case NoTargetRegistrationPolicies:
		return NoTargetRegistrationPolicy{}, true
	case ExactTargetRegistrationPolicyRegistry:
		if policies.Len() != 1 {
			return nil, false
		}
		return ExactTargetRegistrationPolicy{artifact: policies.artifacts[0]}, true
	default:
		return nil, false
	}
}

// RegistrationPolicies is the canonical heterogeneous policy surface. The
// singular RegistrationPolicy compatibility adapter above intentionally fails
// closed when more than one MemberOf family is selected by X.
func (registry ExactTargetRuntimeRegistry) RegistrationPolicies() (
	TargetRegistrationPolicies,
	bool,
) {
	if !registry.Valid() {
		return nil, false
	}
	switch policies := registry.state.policies.(type) {
	case NoTargetRegistrationPolicies:
		return policies, true
	case ExactTargetRegistrationPolicyRegistry:
		cloned, err := newExactTargetRegistrationPolicyRegistry(policies.artifacts)
		if err != nil {
			return nil, false
		}
		return cloned, true
	default:
		return nil, false
	}
}

func (ExactTargetRuntimeRegistry) MarshalJSON() ([]byte, error) {
	return nil, ErrExactTargetRuntimeRegistryNotSerializable
}

func (*ExactTargetRuntimeRegistry) UnmarshalJSON([]byte) error {
	return ErrExactTargetRuntimeRegistryNotSerializable
}

var (
	_ json.Marshaler   = ExactTargetRuntimeRegistry{}
	_ json.Unmarshaler = (*ExactTargetRuntimeRegistry)(nil)
)

// Matched is an unforgeable successful exact-runtime refinement. Callers can
// inspect it only after ObserveCurrentTargetRuntime has constructed the
// private implementation.
type Matched interface {
	Resolution
	Registry() (ExactTargetRuntimeRegistry, bool)
	matchedRuntimeRegistryResolution()
}

type matched struct {
	registry ExactTargetRuntimeRegistry
}

func (matched) Kind() ResolutionKind { return ResolutionMatched }

func (matched) runtimeRegistryResolutionVariant() {}

func (matched) matchedRuntimeRegistryResolution() {}

func (result matched) Registry() (ExactTargetRuntimeRegistry, bool) {
	if !result.registry.Valid() {
		return ExactTargetRuntimeRegistry{}, false
	}
	return result.registry, true
}

// Invalid is an unforgeable malformed-input refinement.
type Invalid interface {
	Resolution
	Issues() []Issue
	invalidRuntimeRegistryResolution()
}

type invalid struct {
	issues []Issue
}

func (invalid) Kind() ResolutionKind { return ResolutionInvalid }

func (invalid) runtimeRegistryResolutionVariant() {}

func (invalid) invalidRuntimeRegistryResolution() {}

func (result invalid) Issues() []Issue {
	return append([]Issue(nil), result.issues...)
}

// Unavailable may additionally report identity drift discovered during the
// same complete comparison. Its posture means at least one required callable
// or exact current registry coordinate was unavailable, so no matched
// observation exists.
type Unavailable interface {
	Resolution
	Issues() []Issue
	unavailableRuntimeRegistryResolution()
}

type unavailable struct {
	issues []Issue
}

func (unavailable) Kind() ResolutionKind { return ResolutionUnavailable }

func (unavailable) runtimeRegistryResolutionVariant() {}

func (unavailable) unavailableRuntimeRegistryResolution() {}

func (result unavailable) Issues() []Issue {
	return append([]Issue(nil), result.issues...)
}

// Drifted is an unforgeable exact-coordinate mismatch refinement.
type Drifted interface {
	Resolution
	Issues() []Issue
	driftedRuntimeRegistryResolution()
}

type drifted struct {
	issues []Issue
}

func (drifted) Kind() ResolutionKind { return ResolutionDrifted }

func (drifted) runtimeRegistryResolutionVariant() {}

func (drifted) driftedRuntimeRegistryResolution() {}

func (result drifted) Issues() []Issue {
	return append([]Issue(nil), result.issues...)
}

func resolutionFromIssues(issues []Issue) Resolution {
	normalized := normalizeIssues(issues)
	if len(normalized) == 0 {
		fallback := newIssue(
			IssueRuntimeBasisInvalid,
			"runtime registry comparison",
			"a nonempty rejected issue set",
			"empty",
			"repair the internal comparison pipeline",
		)
		return invalid{issues: []Issue{fallback}}
	}
	if containsIssueKind(normalized, IssueKindInvalid) {
		return invalid{issues: normalized}
	}
	if containsIssueKind(normalized, IssueKindUnavailable) {
		return unavailable{issues: normalized}
	}
	if len(normalized) > 0 {
		return drifted{issues: normalized}
	}
	fallback := newIssue(
		IssueRuntimeBasisInvalid,
		"runtime registry comparison",
		"a nonempty rejected issue set",
		fmt.Sprintf("%d issues", len(normalized)),
		"repair the internal comparison pipeline",
	)
	return invalid{issues: []Issue{fallback}}
}
