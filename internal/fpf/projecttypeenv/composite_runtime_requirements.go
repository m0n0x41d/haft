package projecttypeenv

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const compositeRuntimeRequirementCanonicalDomain = "haft.fpf.projecttypeenv.composite-runtime-requirements.v2"

const (
	CompositeRuntimeIssueArtifactInvalid       CompositeRuntimeRequirementIssueCode = "composite_artifact_invalid"
	CompositeRuntimeIssueLinkedIRInvalid       CompositeRuntimeRequirementIssueCode = "linked_ir_invalid"
	CompositeRuntimeIssueLinkedIRMismatch      CompositeRuntimeRequirementIssueCode = "linked_ir_mismatch"
	CompositeRuntimeIssueRuntimeBasisInvalid   CompositeRuntimeRequirementIssueCode = "runtime_basis_invalid"
	CompositeRuntimeIssueRuntimeBasisMismatch  CompositeRuntimeRequirementIssueCode = "runtime_basis_mismatch"
	CompositeRuntimeIssueCandidateInvalid      CompositeRuntimeRequirementIssueCode = "candidate_typeenv_invalid"
	CompositeRuntimeIssueCandidateRefMismatch  CompositeRuntimeRequirementIssueCode = "candidate_typeenv_ref_mismatch"
	CompositeRuntimeIssueRequirementInvalid    CompositeRuntimeRequirementIssueCode = "runtime_requirement_invalid"
	CompositeRuntimeIssueMissing               CompositeRuntimeRequirementIssueCode = "runtime_requirement_missing"
	CompositeRuntimeIssueExtra                 CompositeRuntimeRequirementIssueCode = "runtime_pin_extra"
	CompositeRuntimeIssueWrongRole             CompositeRuntimeRequirementIssueCode = "runtime_pin_wrong_role"
	CompositeRuntimeIssueWrongContract         CompositeRuntimeRequirementIssueCode = "runtime_pin_wrong_contract"
	CompositeRuntimeIssueWrongArtifact         CompositeRuntimeRequirementIssueCode = "runtime_pin_wrong_artifact"
	CompositeRuntimeIssueRegistrationMissing   CompositeRuntimeRequirementIssueCode = "registration_policy_missing"
	CompositeRuntimeIssueRegistrationDuplicate CompositeRuntimeRequirementIssueCode = "registration_policy_duplicate"
	CompositeRuntimeIssueRegistrationExtra     CompositeRuntimeRequirementIssueCode = "registration_policy_extra"
)

type CompositeRuntimeRequirementIssueCode string

// CompositeRuntimeRequirement is one exact role/invocation-contract/semantic
// coordinate whose runtime realization must be pinned in X. Its private
// representation is a closed sum: codec requirements carry only CodecRef;
// evaluator and carrier-membership requirements carry only RuleRef.
type CompositeRuntimeRequirement struct {
	role     RuntimeMechanismRole
	contract RuntimeMechanismInvocationContract
	codec    typedmemory.CodecRef
	rule     typedmemory.RuleRef
	hasCodec bool
}

func (requirement CompositeRuntimeRequirement) Role() RuntimeMechanismRole {
	return requirement.role
}

func (requirement CompositeRuntimeRequirement) InvocationContract() RuntimeMechanismInvocationContract {
	return requirement.contract
}

func (requirement CompositeRuntimeRequirement) Codec() (typedmemory.CodecRef, bool) {
	return requirement.codec, requirement.hasCodec
}

func (requirement CompositeRuntimeRequirement) Rule() (typedmemory.RuleRef, bool) {
	return requirement.rule, !requirement.hasCodec
}

func (requirement CompositeRuntimeRequirement) SemanticReference() string {
	if requirement.hasCodec {
		return requirement.codec.String()
	}
	return requirement.rule.String()
}

// CompositeRuntimeRequirementSet is the immutable canonical semantic closure
// required by one final candidate plus its exact linked E source. It contains
// no implementation artifacts, mechanism digests, project state, or authority.
type CompositeRuntimeRequirementSet struct {
	requirements []CompositeRuntimeRequirement
	canonical    []byte
}

func (set CompositeRuntimeRequirementSet) Requirements() []CompositeRuntimeRequirement {
	return append([]CompositeRuntimeRequirement(nil), set.requirements...)
}

func (set CompositeRuntimeRequirementSet) CanonicalBytes() []byte {
	return append([]byte(nil), set.canonical...)
}

type CompositeRuntimeRequirementIssue struct {
	code             CompositeRuntimeRequirementIssueCode
	semantic         string
	expectedRole     RuntimeMechanismRole
	actualRole       RuntimeMechanismRole
	expectedContract RuntimeMechanismInvocationContract
	actualContract   RuntimeMechanismInvocationContract
	detail           string
	repair           string
}

func (issue CompositeRuntimeRequirementIssue) Code() CompositeRuntimeRequirementIssueCode {
	return issue.code
}

func (issue CompositeRuntimeRequirementIssue) SemanticReference() string {
	return issue.semantic
}

func (issue CompositeRuntimeRequirementIssue) ExpectedRole() RuntimeMechanismRole {
	return issue.expectedRole
}

func (issue CompositeRuntimeRequirementIssue) ActualRole() RuntimeMechanismRole {
	return issue.actualRole
}

func (issue CompositeRuntimeRequirementIssue) ExpectedContract() RuntimeMechanismInvocationContract {
	return issue.expectedContract
}

func (issue CompositeRuntimeRequirementIssue) ActualContract() RuntimeMechanismInvocationContract {
	return issue.actualContract
}

func (issue CompositeRuntimeRequirementIssue) Detail() string { return issue.detail }

func (issue CompositeRuntimeRequirementIssue) Repair() string { return issue.repair }

type CompositeRuntimeRequirementsResolution interface {
	Rejected() bool
	Issues() []CompositeRuntimeRequirementIssue
	RequiredSet() CompositeRuntimeRequirementSet
	compositeRuntimeRequirementsResolutionVariant()
}

type acceptedCompositeRuntimeRequirementsResolution struct {
	required CompositeRuntimeRequirementSet
}

func (acceptedCompositeRuntimeRequirementsResolution) Rejected() bool { return false }

func (acceptedCompositeRuntimeRequirementsResolution) Issues() []CompositeRuntimeRequirementIssue {
	return nil
}

func (resolution acceptedCompositeRuntimeRequirementsResolution) RequiredSet() CompositeRuntimeRequirementSet {
	return cloneCompositeRuntimeRequirementSet(resolution.required)
}

func (acceptedCompositeRuntimeRequirementsResolution) compositeRuntimeRequirementsResolutionVariant() {
}

type rejectedCompositeRuntimeRequirementsResolution struct {
	required CompositeRuntimeRequirementSet
	issues   []CompositeRuntimeRequirementIssue
}

func (rejectedCompositeRuntimeRequirementsResolution) Rejected() bool { return true }

func (resolution rejectedCompositeRuntimeRequirementsResolution) Issues() []CompositeRuntimeRequirementIssue {
	return append([]CompositeRuntimeRequirementIssue(nil), resolution.issues...)
}

func (resolution rejectedCompositeRuntimeRequirementsResolution) RequiredSet() CompositeRuntimeRequirementSet {
	return cloneCompositeRuntimeRequirementSet(resolution.required)
}

func (rejectedCompositeRuntimeRequirementsResolution) compositeRuntimeRequirementsResolutionVariant() {
}

// ResolveProjectTypeEnvCompositeRuntimeRequirements re-verifies C, linked B/E,
// final candidate, and X before deriving and comparing the exact semantic
// closure. It performs no lowering, mechanism lookup, staging, or head change.
func ResolveProjectTypeEnvCompositeRuntimeRequirements(
	composite ProjectTypeEnvCompositeArtifact,
	candidate typedmemory.TypeEnv,
	linked LinkedProjectTypeEnvCompositeIR,
	runtimeBasis RuntimeEvaluationBasisArtifact,
) CompositeRuntimeRequirementsResolution {
	if err := composite.Verify(); err != nil {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueArtifactInvalid,
			fmt.Sprintf("verify composite artifact: %v", err),
			"supply the exact sealed ProjectTypeEnvCompositeArtifact",
		)
	}
	verifiedLinked, err := verifyLinkedProjectTypeEnvCompositeIR(linked)
	if err != nil {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueLinkedIRInvalid,
			fmt.Sprintf("verify linked B/E IR: %v", err),
			"relink the exact verified B and E artifacts",
		)
	}
	if err := verifyCompositeRuntimeLinkedRecipe(composite, verifiedLinked); err != nil {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueLinkedIRMismatch,
			err.Error(),
			"use the exact linked B/E proof from which C was sealed",
		)
	}
	if err := runtimeBasis.Verify(); err != nil {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueRuntimeBasisInvalid,
			fmt.Sprintf("verify runtime evaluation basis: %v", err),
			"supply the exact verified RuntimeEvaluationBasisArtifact",
		)
	}
	if composite.RuntimeEvaluationBasisRef() != runtimeBasis.Ref() {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueRuntimeBasisMismatch,
			fmt.Sprintf(
				"composite binds X %q but supplied X is %q",
				composite.RuntimeEvaluationBasisRef().String(),
				runtimeBasis.Ref().String(),
			),
			"use the exact X bound into C",
		)
	}
	if err := runtimeBasis.VerifyResolvedClosure(); err != nil {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueWrongArtifact,
			fmt.Sprintf("verify resolved runtime mechanism closure: %v", err),
			"load the exact canonical runtime mechanism artifacts claimed by X",
		)
	}
	if candidate.Ref() != composite.Ref() {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueCandidateRefMismatch,
			fmt.Sprintf(
				"candidate TypeEnv ref %q does not equal composite C %q",
				candidate.Ref().String(),
				composite.Ref().String(),
			),
			"lower the final candidate at the exact composite C",
		)
	}
	verifiedCandidate, err := verifyCompositeRuntimeCandidate(candidate)
	if err != nil {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueCandidateInvalid,
			fmt.Sprintf("verify final candidate TypeEnv: %v", err),
			"supply a TypeEnv that rebuilds under the closed typedmemory validators",
		)
	}
	required, err := deriveCompositeRuntimeRequirementSet(verifiedCandidate, verifiedLinked)
	if err != nil {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueRequirementInvalid,
			fmt.Sprintf("derive runtime requirement closure: %v", err),
			"repair the typed candidate or exact source membership declaration",
		)
	}
	issues := compareCompositeRuntimeRequirements(required, runtimeBasis.Pins())
	policyRules, err := compositeRegistrationPolicyRules(required, verifiedLinked)
	if err != nil {
		return rejectedCompositeRuntimeInput(
			CompositeRuntimeIssueRequirementInvalid,
			fmt.Sprintf("derive registration-policy closure: %v", err),
			"repair the exact source KindSignature evaluator declarations",
		)
	}
	registrationIssues := compareCompositeRegistrationPolicyRequirements(
		policyRules,
		runtimeBasis,
	)
	issues = append(issues, registrationIssues...)
	sort.Slice(issues, func(left, right int) bool {
		return compositeRuntimeRequirementIssueKey(issues[left]) <
			compositeRuntimeRequirementIssueKey(issues[right])
	})
	if len(issues) > 0 {
		return rejectedCompositeRuntimeRequirementsResolution{
			required: cloneCompositeRuntimeRequirementSet(required),
			issues:   append([]CompositeRuntimeRequirementIssue(nil), issues...),
		}
	}
	return acceptedCompositeRuntimeRequirementsResolution{
		required: cloneCompositeRuntimeRequirementSet(required),
	}
}

func verifyCompositeRuntimeLinkedRecipe(
	composite ProjectTypeEnvCompositeArtifact,
	linked LinkedProjectTypeEnvCompositeIR,
) error {
	if composite.BaseTypeEnvRef() != linked.BaseTypeEnvRef() {
		return fmt.Errorf(
			"composite B %q does not equal linked B %q",
			composite.BaseTypeEnvRef().String(),
			linked.BaseTypeEnvRef().String(),
		)
	}
	linkedRefs := projectTypeEnvCompositeExtensionRefs(linked.Extensions())
	if !projectTypeEnvExtensionRefsEqual(composite.ExtensionRefs(), linkedRefs) {
		return fmt.Errorf("composite E order does not equal the canonical linked E DAG")
	}
	return nil
}

func verifyCompositeRuntimeCandidate(
	candidate typedmemory.TypeEnv,
) (typedmemory.TypeEnv, error) {
	builder := typedmemory.NewTypeEnvBuilder(candidate.Ref())
	builder.SetSourceRevision(candidate.SourceRevision())
	builder.SetCompilerSchemaVersion(candidate.CompilerSchemaVersion())
	builder.SetCoverageManifest(candidate.CoverageManifest())
	for _, context := range candidate.BoundedContexts() {
		builder.AddBoundedContext(context)
	}
	for _, definition := range candidate.KindDefinitions() {
		builder.AddKindDefinition(definition)
	}
	for _, definition := range candidate.EntitySetDefinitions() {
		builder.AddEntitySetDefinition(definition)
	}
	for _, definition := range candidate.KindSignatureDefinitions() {
		builder.AddKindSignatureDefinition(definition)
	}
	for _, definition := range candidate.KindClassificationSignatureDefinitions() {
		builder.AddKindClassificationSignatureDefinition(definition)
	}
	for _, definition := range candidate.RefKindDefinitions() {
		builder.AddRefKindDefinition(definition)
	}
	for _, availability := range candidate.ContextKindAvailabilities() {
		builder.AddContextKindAvailability(availability)
	}
	for _, relation := range candidate.SubkindRelations() {
		builder.AddSubkindRelation(relation)
	}
	for _, bridge := range candidate.ContextBridges() {
		builder.AddContextBridge(bridge)
	}
	for _, fragment := range candidate.TypedRelationDeclarationFragments() {
		builder.AddTypedRelationDeclarationFragment(fragment)
	}
	for _, shape := range candidate.ValueShapes() {
		builder.AddValueShape(shape)
	}
	for _, binding := range candidate.ValueBindings() {
		builder.AddValueBinding(binding)
	}
	for _, constraint := range candidate.Constraints() {
		builder.AddConstraint(constraint)
	}
	verified, err := builder.Build()
	if err != nil {
		return typedmemory.TypeEnv{}, err
	}
	return verified, nil
}

func deriveCompositeRuntimeRequirementSet(
	candidate typedmemory.TypeEnv,
	linked LinkedProjectTypeEnvCompositeIR,
) (CompositeRuntimeRequirementSet, error) {
	requirements := make([]CompositeRuntimeRequirement, 0)
	for _, binding := range candidate.ValueBindings() {
		requirements = append(
			requirements,
			newCompositeCodecRuntimeRequirement(binding.Codec()),
		)
	}
	for _, definition := range candidate.EntitySetDefinitions() {
		requirements = append(
			requirements,
			newCompositeRuleRuntimeRequirement(
				RuntimeMechanismRoleEvaluator,
				RuntimeMechanismContractEntitySetEnumeration,
				definition.EnumerationRule(),
			),
		)
		policy := definition.CandidatePolicy()
		switch value := policy.(type) {
		case typedmemory.PersistedEntitiesOnly:
		case typedmemory.PriorBatchDeclarationsVisible:
			requirements = append(
				requirements,
				newCompositeRuleRuntimeRequirement(
					RuntimeMechanismRoleEvaluator,
					RuntimeMechanismContractCandidateVisibility,
					value.EvaluationRule(),
				),
			)
		default:
			return CompositeRuntimeRequirementSet{}, fmt.Errorf(
				"EntitySet %q has an unsupported candidate policy %T",
				definition.Ref().String(),
				policy,
			)
		}
	}
	kindSignatureRequirements := compositeKindSignatureRuntimeRequirements(
		candidate.KindSignatureDefinitions(),
	)
	requirements = append(requirements, kindSignatureRequirements...)
	classificationRequirements := compositeKindClassificationRuntimeRequirements(
		candidate.KindClassificationSignatureDefinitions(),
	)
	requirements = append(requirements, classificationRequirements...)
	sourceRequirements, err := compositeExplicitSourceEvaluatorRuntimeRequirements(
		canonicalCompositeSourceDeclarations(linked),
	)
	if err != nil {
		return CompositeRuntimeRequirementSet{}, err
	}
	requirements = append(requirements, sourceRequirements...)
	membershipRequirements, err := compositeSourceMembershipRequirements(linked)
	if err != nil {
		return CompositeRuntimeRequirementSet{}, err
	}
	requirements = append(requirements, membershipRequirements...)
	return newCompositeRuntimeRequirementSet(requirements)
}

func compositeKindSignatureRuntimeRequirements(
	definitions []typedmemory.KindSignatureDefinition,
) []CompositeRuntimeRequirement {
	requirements := make([]CompositeRuntimeRequirement, 0, len(definitions)*2)
	for _, definition := range definitions {
		requirements = append(
			requirements,
			newCompositeRuleRuntimeRequirement(
				RuntimeMechanismRoleEvaluator,
				RuntimeMechanismContractKindDefinedness,
				definition.DefinednessRule(),
			),
		)
		requirements = append(
			requirements,
			newCompositeRuleRuntimeRequirement(
				RuntimeMechanismRoleEvaluator,
				RuntimeMechanismContractMemberOf,
				definition.Evaluator(),
			),
		)
	}
	return requirements
}

func compositeKindClassificationRuntimeRequirements(
	definitions []typedmemory.KindClassificationSignatureDefinition,
) []CompositeRuntimeRequirement {
	requirements := make([]CompositeRuntimeRequirement, 0, len(definitions))
	for _, definition := range definitions {
		requirements = append(
			requirements,
			newCompositeRuleRuntimeRequirement(
				RuntimeMechanismRoleEvaluator,
				RuntimeMechanismContractKindClassification,
				definition.Criterion(),
			),
		)
	}
	return requirements
}

func compositeSourceMembershipRequirements(
	linked LinkedProjectTypeEnvCompositeIR,
) ([]CompositeRuntimeRequirement, error) {
	result := make([]CompositeRuntimeRequirement, 0)
	for _, extension := range linked.Extensions() {
		declarations := extension.Artifact().IR().Signature().Vocabulary().Declarations()
		for _, declaration := range declarations {
			if declaration.Kind() != localpractice.DeclarationKindSignature {
				continue
			}
			basisValues := factsAtPath(declaration.Facts(), "membership_basis.kind")
			if len(basisValues) != 1 {
				return nil, fmt.Errorf(
					"source KindSignature %q requires one membership_basis.kind fact",
					declaration.Symbol().Value(),
				)
			}
			switch localpractice.KindSignatureMembershipBasisKind(basisValues[0].Value()) {
			case localpractice.KindSignatureDirectObservableInputs:
			case localpractice.KindSignatureCarrierFirst:
				adapterValues := factsAtPath(
					declaration.Facts(),
					"membership_basis.adapter_rule",
				)
				if len(adapterValues) != 1 {
					return nil, fmt.Errorf(
						"source carrier-first KindSignature %q requires one exact adapter rule",
						declaration.Symbol().Value(),
					)
				}
				adapter, parseErr := typedmemory.NewRuleRef(adapterValues[0].Value())
				if parseErr != nil || adapter.String() != adapterValues[0].Value() {
					return nil, fmt.Errorf(
						"source carrier-first KindSignature %q adapter is not a canonical RuleRef",
						declaration.Symbol().Value(),
					)
				}
				result = append(
					result,
					newCompositeRuleRuntimeRequirement(
						RuntimeMechanismRoleCarrierMembership,
						RuntimeMechanismContractCarrierMembershipDelivery,
						adapter,
					),
				)
			default:
				return nil, fmt.Errorf(
					"source KindSignature %q has unsupported membership basis %q",
					declaration.Symbol().Value(),
					basisValues[0].Value(),
				)
			}
		}
	}
	return result, nil
}

func newCompositeCodecRuntimeRequirement(
	codec typedmemory.CodecRef,
) CompositeRuntimeRequirement {
	return CompositeRuntimeRequirement{
		role:     RuntimeMechanismRoleCodec,
		contract: RuntimeMechanismContractCodecCanonicalization,
		codec:    codec,
		hasCodec: true,
	}
}

func newCompositeRuleRuntimeRequirement(
	role RuntimeMechanismRole,
	contract RuntimeMechanismInvocationContract,
	rule typedmemory.RuleRef,
) CompositeRuntimeRequirement {
	return CompositeRuntimeRequirement{
		role:     role,
		contract: contract,
		rule:     rule,
	}
}

func newCompositeRuntimeRequirementSet(
	values []CompositeRuntimeRequirement,
) (CompositeRuntimeRequirementSet, error) {
	unique := make(map[string]CompositeRuntimeRequirement, len(values))
	for _, value := range values {
		if err := validateCompositeRuntimeRequirement(value); err != nil {
			return CompositeRuntimeRequirementSet{}, err
		}
		unique[compositeRuntimeRequirementKey(value)] = value
	}
	requirements := make([]CompositeRuntimeRequirement, 0, len(unique))
	for _, value := range unique {
		requirements = append(requirements, value)
	}
	sort.Slice(requirements, func(left, right int) bool {
		return compositeRuntimeRequirementKey(requirements[left]) <
			compositeRuntimeRequirementKey(requirements[right])
	})
	if len(requirements) > maximumRuntimeEvaluationBasisPins {
		return CompositeRuntimeRequirementSet{}, fmt.Errorf(
			"runtime requirement closure contains %d coordinates; limit is %d",
			len(requirements),
			maximumRuntimeEvaluationBasisPins,
		)
	}
	canonical := canonicalCompositeRuntimeRequirements(requirements)
	return CompositeRuntimeRequirementSet{
		requirements: append([]CompositeRuntimeRequirement(nil), requirements...),
		canonical:    append([]byte(nil), canonical...),
	}, nil
}

func validateCompositeRuntimeRequirement(requirement CompositeRuntimeRequirement) error {
	if requirement.hasCodec {
		if requirement.role != RuntimeMechanismRoleCodec {
			return fmt.Errorf("codec requirement has role %q", requirement.role)
		}
		if requirement.contract != RuntimeMechanismContractCodecCanonicalization {
			return fmt.Errorf("codec requirement has invocation contract %q", requirement.contract)
		}
		parsed, err := validateRuntimeCodecRef(requirement.codec)
		if err != nil || parsed != requirement.codec {
			return fmt.Errorf("codec requirement is invalid")
		}
		return nil
	}
	if requirement.role != RuntimeMechanismRoleEvaluator &&
		requirement.role != RuntimeMechanismRoleCarrierMembership {
		return fmt.Errorf("rule requirement has role %q", requirement.role)
	}
	if !runtimeMechanismContractMatchesRole(requirement.contract, requirement.role) {
		return fmt.Errorf(
			"rule requirement role %q does not admit invocation contract %q",
			requirement.role,
			requirement.contract,
		)
	}
	parsed, err := validateRuntimeRuleRef(requirement.rule)
	if err != nil || parsed != requirement.rule {
		return fmt.Errorf("rule requirement is invalid")
	}
	return nil
}

func compareCompositeRuntimeRequirements(
	required CompositeRuntimeRequirementSet,
	pins []RuntimeEvaluationMechanismPin,
) []CompositeRuntimeRequirementIssue {
	requiredByKey := make(map[string]CompositeRuntimeRequirement, len(required.requirements))
	for _, requirement := range required.requirements {
		requiredByKey[compositeRuntimeRequirementKey(requirement)] = requirement
	}
	actualByKey := make(map[string]CompositeRuntimeRequirement, len(pins))
	for _, pin := range pins {
		requirement := compositeRuntimeRequirementForPin(pin)
		actualByKey[compositeRuntimeRequirementKey(requirement)] = requirement
	}
	for key := range requiredByKey {
		if _, exists := actualByKey[key]; !exists {
			continue
		}
		delete(requiredByKey, key)
		delete(actualByKey, key)
	}

	issues := make([]CompositeRuntimeRequirementIssue, 0)
	missingKeys := sortedCompositeRuntimeRequirementKeys(requiredByKey)
	for _, missingKey := range missingKeys {
		requirement, exists := requiredByKey[missingKey]
		if !exists {
			continue
		}
		actualKey, actual, found := compositeRuntimeWrongRoleMatch(requirement, actualByKey)
		if !found {
			continue
		}
		issues = append(issues, CompositeRuntimeRequirementIssue{
			code:             CompositeRuntimeIssueWrongRole,
			semantic:         requirement.SemanticReference(),
			expectedRole:     requirement.role,
			actualRole:       actual.role,
			expectedContract: requirement.contract,
			actualContract:   actual.contract,
			detail: fmt.Sprintf(
				"runtime semantic %q is pinned under role %q instead of required role %q",
				requirement.SemanticReference(),
				actual.role,
				requirement.role,
			),
			repair: "pin the exact rule under the required semantic role",
		})
		delete(requiredByKey, missingKey)
		delete(actualByKey, actualKey)
	}
	missingKeys = sortedCompositeRuntimeRequirementKeys(requiredByKey)
	for _, missingKey := range missingKeys {
		requirement, exists := requiredByKey[missingKey]
		if !exists {
			continue
		}
		actualKey, actual, found := compositeRuntimeWrongContractMatch(
			requirement,
			actualByKey,
		)
		if !found {
			continue
		}
		issues = append(issues, CompositeRuntimeRequirementIssue{
			code:             CompositeRuntimeIssueWrongContract,
			semantic:         requirement.SemanticReference(),
			expectedRole:     requirement.role,
			actualRole:       actual.role,
			expectedContract: requirement.contract,
			actualContract:   actual.contract,
			detail: fmt.Sprintf(
				"runtime semantic %q is pinned under invocation contract %q instead of required contract %q",
				requirement.SemanticReference(),
				actual.contract,
				requirement.contract,
			),
			repair: "pin the exact semantic reference under the required invocation contract",
		})
		delete(requiredByKey, missingKey)
		delete(actualByKey, actualKey)
	}
	for _, key := range sortedCompositeRuntimeRequirementKeys(requiredByKey) {
		requirement := requiredByKey[key]
		issues = append(issues, CompositeRuntimeRequirementIssue{
			code:             CompositeRuntimeIssueMissing,
			semantic:         requirement.SemanticReference(),
			expectedRole:     requirement.role,
			expectedContract: requirement.contract,
			detail: fmt.Sprintf(
				"required runtime semantic coordinate %q under role %q and contract %q is absent from X",
				requirement.SemanticReference(),
				requirement.role,
				requirement.contract,
			),
			repair: "add the exact required semantic pin and reseal X and C",
		})
	}
	for _, key := range sortedCompositeRuntimeRequirementKeys(actualByKey) {
		actual := actualByKey[key]
		issues = append(issues, CompositeRuntimeRequirementIssue{
			code:           CompositeRuntimeIssueExtra,
			semantic:       actual.SemanticReference(),
			actualRole:     actual.role,
			actualContract: actual.contract,
			detail: fmt.Sprintf(
				"runtime semantic coordinate %q under role %q and contract %q is pinned in X but not required",
				actual.SemanticReference(),
				actual.role,
				actual.contract,
			),
			repair: "remove the extra semantic pin and reseal X and C",
		})
	}
	sort.Slice(issues, func(left, right int) bool {
		return compositeRuntimeRequirementIssueKey(issues[left]) <
			compositeRuntimeRequirementIssueKey(issues[right])
	})
	return issues
}

func compositeRegistrationPolicyRules(
	required CompositeRuntimeRequirementSet,
	linked LinkedProjectTypeEnvCompositeIR,
) ([]typedmemory.RuleRef, error) {
	byRef := make(map[string]typedmemory.RuleRef)
	for _, requirement := range required.requirements {
		if requirement.role == RuntimeMechanismRoleEvaluator &&
			requirement.contract == RuntimeMechanismContractMemberOf {
			byRef[requirement.rule.String()] = requirement.rule
		}
	}
	for _, extension := range linked.Extensions() {
		declarations := extension.Artifact().IR().Signature().Vocabulary().Declarations()
		for _, declaration := range declarations {
			if declaration.Kind() != localpractice.DeclarationKindSignature {
				continue
			}
			evaluatorValues := factsAtPath(declaration.Facts(), "evaluator_rule")
			if len(evaluatorValues) != 1 {
				return nil, fmt.Errorf(
					"source KindSignature %q requires one evaluator_rule fact",
					declaration.Symbol().Value(),
				)
			}
			rule, err := typedmemory.NewRuleRef(evaluatorValues[0].Value())
			if err != nil || rule.String() != evaluatorValues[0].Value() {
				return nil, fmt.Errorf(
					"source KindSignature %q evaluator is not a canonical RuleRef",
					declaration.Symbol().Value(),
				)
			}
			byRef[rule.String()] = rule
		}
	}
	refs := make([]string, 0, len(byRef))
	for ref := range byRef {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	result := make([]typedmemory.RuleRef, 0, len(refs))
	for _, ref := range refs {
		result = append(result, byRef[ref])
	}
	return result, nil
}

func compareCompositeRegistrationPolicyRequirements(
	required []typedmemory.RuleRef,
	runtimeBasis RuntimeEvaluationBasisArtifact,
) []CompositeRuntimeRequirementIssue {
	policies, err := decodeResolvedRegistrationPolicies(
		runtimeBasis.resolvedRegistrationPolicies,
	)
	if err != nil {
		return []CompositeRuntimeRequirementIssue{{
			code:     CompositeRuntimeIssueRuntimeBasisInvalid,
			semantic: "membership-registration-policies",
			detail:   fmt.Sprintf("decode resolved registration-policy closure: %v", err),
			repair:   "reload and verify the exact registration policies resolved by X",
		}}
	}
	requiredByRule := make(map[string]typedmemory.RuleRef, len(required))
	for _, rule := range required {
		requiredByRule[rule.String()] = rule
	}
	policiesByRule := make(
		map[string][]RegistrationPolicyArtifact,
		len(policies),
	)
	for _, policy := range policies {
		rule := policy.Evaluator().Rule()
		policiesByRule[rule.String()] = append(policiesByRule[rule.String()], policy)
	}
	issues := make([]CompositeRuntimeRequirementIssue, 0)
	for _, rule := range required {
		matches := policiesByRule[rule.String()]
		if len(matches) == 0 {
			issues = append(issues, CompositeRuntimeRequirementIssue{
				code:     CompositeRuntimeIssueRegistrationMissing,
				semantic: rule.String(),
				detail:   "required MemberOf evaluator RuleRef has no exact registration-policy pin in X",
				repair:   "pin and resolve one exact registration policy for this MemberOf evaluator, then reseal X and C",
			})
			continue
		}
		if len(matches) > 1 {
			issues = append(issues, CompositeRuntimeRequirementIssue{
				code:     CompositeRuntimeIssueRegistrationDuplicate,
				semantic: rule.String(),
				detail: fmt.Sprintf(
					"MemberOf evaluator RuleRef has %d registration policies in X",
					len(matches),
				),
				repair: "retain exactly one registration policy for this MemberOf evaluator RuleRef",
			})
		}
	}
	policyRules := make([]string, 0, len(policiesByRule))
	for rule := range policiesByRule {
		policyRules = append(policyRules, rule)
	}
	sort.Strings(policyRules)
	for _, rule := range policyRules {
		if _, exists := requiredByRule[rule]; exists {
			continue
		}
		issues = append(issues, CompositeRuntimeRequirementIssue{
			code:     CompositeRuntimeIssueRegistrationExtra,
			semantic: rule,
			detail:   "X pins a registration policy for a MemberOf evaluator RuleRef that the composite does not require",
			repair:   "remove the unexpected registration-policy pin, then reseal X and C",
		})
	}
	return issues
}

func compositeRuntimeRequirementForPin(
	pin RuntimeEvaluationMechanismPin,
) CompositeRuntimeRequirement {
	switch value := pin.(type) {
	case CodecRuntimeMechanismPin:
		return newCompositeCodecRuntimeRequirement(value.Codec())
	case EvaluatorRuntimeMechanismPin:
		return newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleEvaluator,
			value.InvocationContract(),
			value.Rule(),
		)
	case CarrierMembershipRuntimeMechanismPin:
		return newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleCarrierMembership,
			value.InvocationContract(),
			value.Rule(),
		)
	default:
		return CompositeRuntimeRequirement{}
	}
}

func compositeRuntimeWrongRoleMatch(
	required CompositeRuntimeRequirement,
	actual map[string]CompositeRuntimeRequirement,
) (string, CompositeRuntimeRequirement, bool) {
	keys := sortedCompositeRuntimeRequirementKeys(actual)
	for _, key := range keys {
		candidate := actual[key]
		if candidate.SemanticReference() != required.SemanticReference() ||
			candidate.role == required.role {
			continue
		}
		return key, candidate, true
	}
	return "", CompositeRuntimeRequirement{}, false
}

func compositeRuntimeWrongContractMatch(
	required CompositeRuntimeRequirement,
	actual map[string]CompositeRuntimeRequirement,
) (string, CompositeRuntimeRequirement, bool) {
	keys := sortedCompositeRuntimeRequirementKeys(actual)
	for _, key := range keys {
		candidate := actual[key]
		if candidate.SemanticReference() != required.SemanticReference() ||
			candidate.role != required.role ||
			candidate.contract == required.contract {
			continue
		}
		return key, candidate, true
	}
	return "", CompositeRuntimeRequirement{}, false
}

func compositeRuntimeRequirementKey(requirement CompositeRuntimeRequirement) string {
	coordinateKind := "rule"
	if requirement.hasCodec {
		coordinateKind = "codec"
	}
	return string(requirement.role) + "\x00" + requirement.contract.String() +
		"\x00" + coordinateKind + "\x00" + requirement.SemanticReference()
}

func sortedCompositeRuntimeRequirementKeys(
	values map[string]CompositeRuntimeRequirement,
) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compositeRuntimeRequirementIssueKey(issue CompositeRuntimeRequirementIssue) string {
	return string(issue.code) + "\x00" + issue.semantic + "\x00" +
		string(issue.expectedRole) + "\x00" + string(issue.actualRole) + "\x00" +
		issue.expectedContract.String() + "\x00" + issue.actualContract.String()
}

func canonicalCompositeRuntimeRequirements(
	requirements []CompositeRuntimeRequirement,
) []byte {
	writer := compositeRuntimeRequirementWriter{}
	writer.addString(compositeRuntimeRequirementCanonicalDomain)
	writer.addUint64(uint64(len(requirements)))
	for _, requirement := range requirements {
		writer.addString(string(requirement.role))
		writer.addString(requirement.contract.String())
		coordinateKind := "rule"
		if requirement.hasCodec {
			coordinateKind = "codec"
		}
		writer.addString(coordinateKind)
		writer.addString(requirement.SemanticReference())
	}
	return writer.bytes()
}

type compositeRuntimeRequirementWriter struct {
	buffer bytes.Buffer
}

func (writer *compositeRuntimeRequirementWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *compositeRuntimeRequirementWriter) addUint64(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.buffer.Write(encoded[:])
}

func (writer *compositeRuntimeRequirementWriter) addBytes(value []byte) {
	writer.addUint64(uint64(len(value)))
	writer.buffer.Write(value)
}

func (writer compositeRuntimeRequirementWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

func cloneCompositeRuntimeRequirementSet(
	set CompositeRuntimeRequirementSet,
) CompositeRuntimeRequirementSet {
	return CompositeRuntimeRequirementSet{
		requirements: append([]CompositeRuntimeRequirement(nil), set.requirements...),
		canonical:    append([]byte(nil), set.canonical...),
	}
}

func rejectedCompositeRuntimeInput(
	code CompositeRuntimeRequirementIssueCode,
	detail string,
	repair string,
) CompositeRuntimeRequirementsResolution {
	return rejectedCompositeRuntimeRequirementsResolution{
		issues: []CompositeRuntimeRequirementIssue{{
			code:   code,
			detail: detail,
			repair: repair,
		}},
	}
}
