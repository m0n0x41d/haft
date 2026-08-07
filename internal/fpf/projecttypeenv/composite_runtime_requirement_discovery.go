package projecttypeenv

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// CompositeRuntimeRequirementDiscovery is a closed pure result for the one
// bootstrap question that precedes X: which exact semantic runtime coordinates
// does this verified B + linked E closure require? It deliberately exposes
// neither a runtime implementation choice nor a TypeEnv. Only a later final C,
// sealed with a complete X, can be prepared and selected.
type CompositeRuntimeRequirementDiscovery interface {
	Rejected() bool
	Issues() []ProjectTypeEnvCompositeLoweringIssue
	RequiredSet() (CompositeRuntimeRequirementSet, bool)
	compositeRuntimeRequirementDiscoveryVariant()
}

type discoveredCompositeRuntimeRequirements struct {
	required CompositeRuntimeRequirementSet
}

func (discoveredCompositeRuntimeRequirements) Rejected() bool { return false }

func (discoveredCompositeRuntimeRequirements) Issues() []ProjectTypeEnvCompositeLoweringIssue {
	return nil
}

func (discovery discoveredCompositeRuntimeRequirements) RequiredSet() (
	CompositeRuntimeRequirementSet,
	bool,
) {
	return cloneCompositeRuntimeRequirementSet(discovery.required), true
}

func (discoveredCompositeRuntimeRequirements) compositeRuntimeRequirementDiscoveryVariant() {
}

type rejectedCompositeRuntimeRequirementDiscovery struct {
	issues []ProjectTypeEnvCompositeLoweringIssue
}

func (rejectedCompositeRuntimeRequirementDiscovery) Rejected() bool { return true }

func (discovery rejectedCompositeRuntimeRequirementDiscovery) Issues() []ProjectTypeEnvCompositeLoweringIssue {
	return append([]ProjectTypeEnvCompositeLoweringIssue(nil), discovery.issues...)
}

func (rejectedCompositeRuntimeRequirementDiscovery) RequiredSet() (
	CompositeRuntimeRequirementSet,
	bool,
) {
	return CompositeRuntimeRequirementSet{}, false
}

func (rejectedCompositeRuntimeRequirementDiscovery) compositeRuntimeRequirementDiscoveryVariant() {
}

// DiscoverProjectTypeEnvCompositeRuntimeRequirements derives the exact runtime
// semantics authored by verified B plus the linked symbolic E DAG. It consumes
// neither X nor C: source declarations name codec, classification, historical
// enumeration/MemberOf, and carrier-delivery semantics before implementations
// are selected. Callers must then build exact X, derive C from B/E/X, and let
// ResolveProjectTypeEnvCompositeRuntimeRequirements verify the lowered closure.
func DiscoverProjectTypeEnvCompositeRuntimeRequirements(
	base typeenv.BaseTypeEnvArtifact,
	linked LinkedProjectTypeEnvCompositeIR,
) CompositeRuntimeRequirementDiscovery {
	baseEnvironment, verifiedLinked, err := verifyCompositeRuntimeRequirementSources(
		base,
		linked,
	)
	if err != nil {
		return rejectCompositeRuntimeRequirementDiscovery(
			CompositeLoweringIssueLinkedInvalid,
			"source-B-E",
			fmt.Sprintf("verify source runtime-requirement closure: %v", err),
			"repair and relink the exact verified B and E source artifacts",
		)
	}
	required, err := deriveSourceCompositeRuntimeRequirementSet(
		baseEnvironment,
		verifiedLinked,
	)
	if err != nil {
		return rejectCompositeRuntimeRequirementDiscovery(
			CompositeLoweringIssueRuntimeClosure,
			"source-B-E-runtime-requirements",
			fmt.Sprintf("derive exact source runtime requirements: %v", err),
			"repair the source declaration or its exact dependency closure",
		)
	}
	return discoveredCompositeRuntimeRequirements{required: required}
}

func verifyCompositeRuntimeRequirementSources(
	base typeenv.BaseTypeEnvArtifact,
	linked LinkedProjectTypeEnvCompositeIR,
) (typedmemory.TypeEnv, LinkedProjectTypeEnvCompositeIR, error) {
	if err := base.Verify(); err != nil {
		return typedmemory.TypeEnv{}, LinkedProjectTypeEnvCompositeIR{}, fmt.Errorf(
			"verify base TypeEnv artifact: %w",
			err,
		)
	}
	baseRef, exists := base.TypeEnvRef()
	if !exists {
		return typedmemory.TypeEnv{}, LinkedProjectTypeEnvCompositeIR{}, fmt.Errorf(
			"base TypeEnv artifact is coverage-only",
		)
	}
	verifiedLinked, err := verifyLinkedProjectTypeEnvCompositeIR(linked)
	if err != nil {
		return typedmemory.TypeEnv{}, LinkedProjectTypeEnvCompositeIR{}, fmt.Errorf(
			"verify linked B/E source: %w",
			err,
		)
	}
	if verifiedLinked.BaseTypeEnvRef() != baseRef ||
		!bytes.Equal(verifiedLinked.BaseArtifact().CanonicalBytes(), base.CanonicalBytes()) {
		return typedmemory.TypeEnv{}, LinkedProjectTypeEnvCompositeIR{}, fmt.Errorf(
			"explicit B does not byte-match the base authenticated by linked E",
		)
	}
	baseEnvironment, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		baseRef,
	)
	if err != nil {
		return typedmemory.TypeEnv{}, LinkedProjectTypeEnvCompositeIR{}, fmt.Errorf(
			"lower executable source B at its own identity: %w",
			err,
		)
	}
	return baseEnvironment, verifiedLinked, nil
}

func deriveSourceCompositeRuntimeRequirementSet(
	base typedmemory.TypeEnv,
	linked LinkedProjectTypeEnvCompositeIR,
) (CompositeRuntimeRequirementSet, error) {
	requirements := make([]CompositeRuntimeRequirement, 0)
	for _, binding := range base.ValueBindings() {
		requirements = append(
			requirements,
			newCompositeCodecRuntimeRequirement(binding.Codec()),
		)
	}
	for _, definition := range base.EntitySetDefinitions() {
		requirements = append(
			requirements,
			compositeEntitySetDefinitionRuntimeRequirements(definition)...,
		)
	}
	requirements = append(
		requirements,
		compositeKindSignatureRuntimeRequirements(base.KindSignatureDefinitions())...,
	)
	requirements = append(
		requirements,
		compositeKindClassificationRuntimeRequirements(
			base.KindClassificationSignatureDefinitions(),
		)...,
	)

	sources := canonicalCompositeSourceDeclarations(linked)
	codecRequirements, err := compositeSourceCodecRuntimeRequirements(
		base,
		linked,
		sources,
	)
	if err != nil {
		return CompositeRuntimeRequirementSet{}, err
	}
	requirements = append(requirements, codecRequirements...)
	declarationRequirements, err := compositeSourceEvaluatorRuntimeRequirements(sources)
	if err != nil {
		return CompositeRuntimeRequirementSet{}, err
	}
	requirements = append(requirements, declarationRequirements...)
	membershipRequirements, err := compositeSourceMembershipRequirements(linked)
	if err != nil {
		return CompositeRuntimeRequirementSet{}, err
	}
	requirements = append(requirements, membershipRequirements...)
	return newCompositeRuntimeRequirementSet(requirements)
}

func compositeEntitySetDefinitionRuntimeRequirements(
	definition typedmemory.EntitySetDefinition,
) []CompositeRuntimeRequirement {
	result := []CompositeRuntimeRequirement{
		newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleEvaluator,
			RuntimeMechanismContractEntitySetEnumeration,
			definition.EnumerationRule(),
		),
	}
	policy, visible := definition.CandidatePolicy().(typedmemory.PriorBatchDeclarationsVisible)
	if !visible {
		return result
	}
	return append(
		result,
		newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleEvaluator,
			RuntimeMechanismContractCandidateVisibility,
			policy.EvaluationRule(),
		),
	)
}

func compositeSourceCodecRuntimeRequirements(
	base typedmemory.TypeEnv,
	linked LinkedProjectTypeEnvCompositeIR,
	sources []compositeSourceDeclaration,
) ([]CompositeRuntimeRequirement, error) {
	inherited := make(map[string]typedmemory.ValueShapeRef)
	for _, declaration := range base.ValueShapes() {
		inherited[declaration.Ref().ID().String()] = declaration.Ref()
	}
	provenance := func(
		source compositeSourceDeclaration,
		semantic string,
	) (typedmemory.ProjectSourceProvenance, error) {
		return compositeDeclarationProvenance(linked, source, semantic)
	}
	_, shapes, err := lowerCompositeValueShapes(sources, inherited, provenance)
	if err != nil {
		return nil, fmt.Errorf("derive source value-shape closure: %w", err)
	}
	_, declarations, err := lowerCompositePartitionValueDeclarations(sources)
	if err != nil {
		return nil, fmt.Errorf("partition source codec declarations: %w", err)
	}
	result := make([]CompositeRuntimeRequirement, 0, len(declarations))
	for _, declaration := range declarations {
		specification, _, specificationErr := deriveCompositeCodecSpecification(
			declaration,
			shapes,
		)
		if specificationErr != nil {
			return nil, specificationErr
		}
		result = append(
			result,
			newCompositeCodecRuntimeRequirement(specification.Ref()),
		)
	}
	return result, nil
}

func compositeSourceEvaluatorRuntimeRequirements(
	sources []compositeSourceDeclaration,
) ([]CompositeRuntimeRequirement, error) {
	result := make([]CompositeRuntimeRequirement, 0)
	for _, source := range sources {
		switch source.value.Kind() {
		case localpractice.DeclarationEntitySet:
			entitySet, err := compositeSourceEntitySetRuntimeRequirements(source)
			if err != nil {
				return nil, err
			}
			result = append(result, entitySet...)
		case localpractice.DeclarationKindSignature:
			kindSignature, err := compositeSourceKindSignatureRuntimeRequirements(source)
			if err != nil {
				return nil, err
			}
			result = append(result, kindSignature...)
		case localpractice.DeclarationKindClassificationSignature:
			kindSignature, err := compositeSourceKindClassificationRuntimeRequirements(source)
			if err != nil {
				return nil, err
			}
			result = append(result, kindSignature...)
		}
	}
	explicit, err := compositeExplicitSourceEvaluatorRuntimeRequirements(sources)
	if err != nil {
		return nil, err
	}
	result = append(result, explicit...)
	return result, nil
}

func compositeExplicitSourceEvaluatorRuntimeRequirements(
	sources []compositeSourceDeclaration,
) ([]CompositeRuntimeRequirement, error) {
	result := make([]CompositeRuntimeRequirement, 0)
	coordinates := make(map[string]string)
	for _, source := range sources {
		if source.value.Kind() != localpractice.DeclarationRuntimeEvaluatorRequirement {
			continue
		}
		requirement, err := compositeSourceRuntimeEvaluatorRequirement(source)
		if err != nil {
			return nil, err
		}
		key := compositeRuntimeRequirementKey(requirement)
		prior, duplicate := coordinates[key]
		if duplicate {
			return nil, fmt.Errorf(
				"runtime evaluator requirements %q and %q repeat semantic coordinate %q",
				prior,
				source.value.Symbol().Value(),
				requirement.SemanticReference(),
			)
		}
		coordinates[key] = source.value.Symbol().Value()
		result = append(result, requirement)
	}
	return result, nil
}

func compositeSourceRuntimeEvaluatorRequirement(
	source compositeSourceDeclaration,
) (CompositeRuntimeRequirement, error) {
	rule, err := compositeRuleFact(source.value, "rule_ref")
	if err != nil {
		return CompositeRuntimeRequirement{}, compositeDeclarationError(
			source,
			"runtime evaluator RuleRef",
			err,
		)
	}
	contractSource, err := requiredDeclarationFact(
		source.value,
		"invocation_contract",
	)
	if err != nil {
		return CompositeRuntimeRequirement{}, compositeDeclarationError(
			source,
			"runtime evaluator invocation contract",
			err,
		)
	}
	contract, err := parseEvaluatorRuntimeMechanismContract(contractSource.Value())
	if err != nil {
		return CompositeRuntimeRequirement{}, compositeDeclarationError(
			source,
			"runtime evaluator invocation contract",
			err,
		)
	}
	return newCompositeRuleRuntimeRequirement(
		RuntimeMechanismRoleEvaluator,
		contract,
		rule,
	), nil
}

func compositeSourceEntitySetRuntimeRequirements(
	source compositeSourceDeclaration,
) ([]CompositeRuntimeRequirement, error) {
	enumeration, err := compositeRuleFact(source.value, "enumeration_rule")
	if err != nil {
		return nil, compositeDeclarationError(source, "EntitySet enumeration rule", err)
	}
	result := []CompositeRuntimeRequirement{
		newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleEvaluator,
			RuntimeMechanismContractEntitySetEnumeration,
			enumeration,
		),
	}
	policy, err := compositeEntitySetPolicy(source)
	if err != nil {
		return nil, err
	}
	visible, includesPriorBatch := policy.(typedmemory.PriorBatchDeclarationsVisible)
	if !includesPriorBatch {
		return result, nil
	}
	return append(
		result,
		newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleEvaluator,
			RuntimeMechanismContractCandidateVisibility,
			visible.EvaluationRule(),
		),
	), nil
}

func compositeSourceKindSignatureRuntimeRequirements(
	source compositeSourceDeclaration,
) ([]CompositeRuntimeRequirement, error) {
	definedness, err := compositeRuleFact(source.value, "definedness_rule")
	if err != nil {
		return nil, compositeDeclarationError(source, "KindSignature definedness", err)
	}
	evaluator, err := compositeRuleFact(source.value, "evaluator_rule")
	if err != nil {
		return nil, compositeDeclarationError(source, "KindSignature evaluator", err)
	}
	return []CompositeRuntimeRequirement{
		newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleEvaluator,
			RuntimeMechanismContractKindDefinedness,
			definedness,
		),
		newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleEvaluator,
			RuntimeMechanismContractMemberOf,
			evaluator,
		),
	}, nil
}

func compositeSourceKindClassificationRuntimeRequirements(
	source compositeSourceDeclaration,
) ([]CompositeRuntimeRequirement, error) {
	criterion, err := compositeRuleFact(source.value, "criterion_rule")
	if err != nil {
		return nil, compositeDeclarationError(
			source,
			"current KindSignature criterion",
			err,
		)
	}
	return []CompositeRuntimeRequirement{
		newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleEvaluator,
			RuntimeMechanismContractKindClassification,
			criterion,
		),
	}, nil
}

func rejectCompositeRuntimeRequirementDiscovery(
	code ProjectTypeEnvCompositeLoweringIssueCode,
	subject string,
	detail string,
	repair string,
) CompositeRuntimeRequirementDiscovery {
	issue := newCompositeLoweringIssue(code, subject, detail, repair)
	return rejectedCompositeRuntimeRequirementDiscovery{
		issues: []ProjectTypeEnvCompositeLoweringIssue{issue},
	}
}
