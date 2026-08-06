package sqlite

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"sort"
	"testing"

	basetypeenvartifacts "github.com/m0n0x41d/haft/data/haft/base-typeenv/artifacts"
	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/codeanchoradapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/decisionrecordadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/evidenceworkadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projectmemory/noteadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/portfoliocomparisonadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/problemcardadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projectmemory/solutionportfolioadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/specsectionadapter"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const productionTypedMemoryCarrierPath = "../../../data/haft/local-practice/typed-memory/candidates/1.2.0.yaml"

type productionLocalPracticeETarget struct {
	base         typeenv.BaseTypeEnvArtifact
	extension    projecttypeenv.ProjectTypeEnvExtensionArtifact
	linked       projecttypeenv.LinkedProjectTypeEnvCompositeIR
	requirements projecttypeenv.CompositeRuntimeRequirementSet
	runtime      projecttypeenv.RuntimeEvaluationBasisArtifact
	mechanism    runtimemechanism.RuntimeMechanismArtifactV1
	policies     []recordmembershipregistration.RegistrationArtifactV1
	composite    projecttypeenv.ProjectTypeEnvCompositeArtifact
	preparation  projecttypeenv.ProjectTypeEnvCompositePreparation
	installed    projecttypeenvruntime.InstalledRuntimeRegistryInput
}

func TestProductionLocalPracticeEFixtureCarriesNoteAndExactRuntimeClosure(
	t *testing.T,
) {
	t.Parallel()

	fixture := newProductionLocalPracticeETarget(t)
	if fixture.extension.Ref().String() == "" {
		t.Fatal("production Local-Practice E has no content-derived reference")
	}
	if got := len(fixture.linked.Extensions()); got != 1 {
		t.Fatalf("linked production E count = %d, want 1", got)
	}
	if !productionExtensionHasDeclaration(
		fixture.extension,
		localpractice.DeclarationRelationSignature,
		"Haft.NoteAtConcern",
	) {
		t.Fatal("production Local-Practice E has no Haft.NoteAtConcern relation signature")
	}
	if !productionExtensionHasDeclaration(
		fixture.extension,
		localpractice.DeclarationRelationSignature,
		"Haft.ProblemCardAtConcern",
	) {
		t.Fatal("production Local-Practice E has no Haft.ProblemCardAtConcern relation signature")
	}
	if !productionExtensionHasDeclaration(
		fixture.extension,
		localpractice.DeclarationRelationSignature,
		"Haft.SolutionPortfolioAtConcern",
	) {
		t.Fatal("production Local-Practice E has no Haft.SolutionPortfolioAtConcern relation signature")
	}
	if !productionExtensionHasDeclaration(
		fixture.extension,
		localpractice.DeclarationRelationSignature,
		"Haft.PortfolioComparison",
	) {
		t.Fatal("production Local-Practice E has no Haft.PortfolioComparison relation signature")
	}

	counts := productionRequirementContractCounts(fixture.requirements)
	assertProductionRequirementCount(
		t,
		counts,
		projecttypeenv.RuntimeMechanismContractEntitySetEnumeration,
		1,
	)
	assertProductionRequirementCount(
		t,
		counts,
		projecttypeenv.RuntimeMechanismContractCandidateVisibility,
		1,
	)
	assertProductionRequirementCount(
		t,
		counts,
		projecttypeenv.RuntimeMechanismContractKindDefinedness,
		1,
	)
	assertProductionRequirementCount(
		t,
		counts,
		projecttypeenv.RuntimeMechanismContractMemberOf,
		6,
	)
	assertProductionRequirementCount(
		t,
		counts,
		projecttypeenv.RuntimeMechanismContractCarrierMembershipDelivery,
		6,
	)
	if got := len(fixture.policies); got != 6 {
		t.Fatalf("production registration policies = %d, want 6 exact MemberOf policies", got)
	}
	assertProductionProjectRecordMappings(t, fixture.policies)
	assertProductionCodeAnchorMappings(t, fixture.policies)

	environment, ok := fixture.preparation.Environment()
	if !ok {
		t.Fatal("prepared production Local-Practice target has no executable environment")
	}
	final := projecttypeenv.ResolveProjectTypeEnvCompositeRuntimeRequirements(
		fixture.composite,
		environment,
		fixture.linked,
		fixture.runtime,
	)
	if final.Rejected() {
		t.Fatalf("final production B/E/X/C runtime closure rejected: %#v", final.Issues())
	}
	if !bytes.Equal(
		fixture.requirements.CanonicalBytes(),
		final.RequiredSet().CanonicalBytes(),
	) {
		t.Fatal("final C runtime closure differs from pure provisional discovery")
	}
}

func TestProductionLocalPracticeRuntimeDiscoveryIsOrderInvariantAndNotACompleteX(
	t *testing.T,
) {
	t.Parallel()

	fixture := newProductionLocalPracticeETarget(t)
	reversed := fixture.requirements.Requirements()
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	reversedRuntime, _, _ := newProductionLocalPracticeRuntime(t, reversed)
	if reversedRuntime.Ref() != fixture.runtime.Ref() {
		t.Fatal("requirement input order changed canonical X")
	}
	reversedComposite, err := projecttypeenv.ResealHistoricalProjectTypeEnvCompositeV1(
		fixture.linked,
		reversedRuntime,
	)
	if err != nil {
		t.Fatalf("seal order-invariant C: %v", err)
	}
	if reversedComposite.Ref() != fixture.composite.Ref() {
		t.Fatal("requirement input order changed canonical C")
	}

	incomplete := append(
		[]projecttypeenv.CompositeRuntimeRequirement(nil),
		fixture.requirements.Requirements()[1:]...,
	)
	incompleteRuntime, _, _ := newProductionLocalPracticeRuntime(t, incomplete)
	incompleteComposite, err := projecttypeenv.ResealHistoricalProjectTypeEnvCompositeV1(
		fixture.linked,
		incompleteRuntime,
	)
	if err != nil {
		t.Fatalf("seal deliberately incomplete C recipe: %v", err)
	}
	if incompleteComposite.Ref() == fixture.composite.Ref() {
		t.Fatal("changed X retained the final C identity")
	}
	incompletePreparation := projecttypeenv.PrepareProjectTypeEnvComposite(
		projecttypeenv.ProjectTypeEnvCompositePreparationInput{
			Base:         fixture.base,
			Linked:       fixture.linked,
			RuntimeBasis: incompleteRuntime,
			Composite:    incompleteComposite,
		},
	)
	if !incompletePreparation.Rejected() {
		t.Fatal("incomplete X prepared an executable production Local-Practice C")
	}
}

func TestProductionProblemCardMappingChangesCandidateXAndCompositeC(
	t *testing.T,
) {
	t.Parallel()

	fixture := newProductionLocalPracticeETarget(t)
	noteOnly := func(
		t *testing.T,
		rule typedmemory.RuleRef,
	) []productionRegistrationMapping {
		mappings := productionRegistrationMappingsForRule(t, rule)
		recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
		if rule != recordRule {
			return mappings
		}
		return append([]productionRegistrationMapping(nil), mappings[:1]...)
	}
	noteOnlyRuntime, _, noteOnlyPolicies :=
		newProductionLocalPracticeRuntimeWithResolver(
			t,
			fixture.requirements.Requirements(),
			noteOnly,
		)
	if noteOnlyRuntime.Ref() == fixture.runtime.Ref() {
		t.Fatal("removing the ProblemCard mapping retained candidate X identity")
	}
	noteOnlyComposite, err := projecttypeenv.ResealHistoricalProjectTypeEnvCompositeV1(
		fixture.linked,
		noteOnlyRuntime,
	)
	if err != nil {
		t.Fatalf("seal Note-only candidate C: %v", err)
	}
	if noteOnlyComposite.Ref() == fixture.composite.Ref() {
		t.Fatal("removing the ProblemCard mapping retained composite C identity")
	}
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	noteOnlyPolicy := productionPolicyForRule(t, noteOnlyPolicies, recordRule)
	if got := len(noteOnlyPolicy.AcceptedMappings()); got != 1 {
		t.Fatalf("Note-only accepted mappings = %d, want 1", got)
	}

	reversed := func(
		t *testing.T,
		rule typedmemory.RuleRef,
	) []productionRegistrationMapping {
		mappings := productionRegistrationMappingsForRule(t, rule)
		slices.Reverse(mappings)
		return mappings
	}
	reversedRuntime, _, reversedPolicies :=
		newProductionLocalPracticeRuntimeWithResolver(
			t,
			fixture.requirements.Requirements(),
			reversed,
		)
	if reversedRuntime.Ref() != fixture.runtime.Ref() {
		t.Fatal("mapping input order changed canonical candidate X")
	}
	reversedPolicy := productionPolicyForRule(t, reversedPolicies, recordRule)
	currentPolicy := productionPolicyForRule(t, fixture.policies, recordRule)
	if reversedPolicy.Ref() != currentPolicy.Ref() {
		t.Fatal("mapping input order changed canonical registration policy")
	}
}

func TestProductionPortfolioMappingsEachChangeCandidateXAndCompositeC(
	t *testing.T,
) {
	t.Parallel()

	fixture := newProductionLocalPracticeETarget(t)
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	removedIDs := []string{
		solutionportfolioadapter.MappingManifestIDV1,
		portfoliocomparisonadapter.MappingManifestIDV1,
	}
	for _, removedID := range removedIDs {
		t.Run(removedID, func(t *testing.T) {
			withoutOne := func(
				t *testing.T,
				rule typedmemory.RuleRef,
			) []productionRegistrationMapping {
				mappings := productionRegistrationMappingsForRule(t, rule)
				if rule != recordRule {
					return mappings
				}
				filtered := make(
					[]productionRegistrationMapping,
					0,
					len(mappings)-1,
				)
				for _, mapping := range mappings {
					if mapping.manifest.ID() != removedID {
						filtered = append(filtered, mapping)
					}
				}
				return filtered
			}
			runtime, _, policies :=
				newProductionLocalPracticeRuntimeWithResolver(
					t,
					fixture.requirements.Requirements(),
					withoutOne,
				)
			if runtime.Ref() == fixture.runtime.Ref() {
				t.Fatalf(
					"removing %s retained candidate X identity",
					removedID,
				)
			}
			composite, err := projecttypeenv.ResealHistoricalProjectTypeEnvCompositeV1(
				fixture.linked,
				runtime,
			)
			if err != nil {
				t.Fatalf(
					"seal candidate C without %s: %v",
					removedID,
					err,
				)
			}
			if composite.Ref() == fixture.composite.Ref() {
				t.Fatalf(
					"removing %s retained composite C identity",
					removedID,
				)
			}
			policy := productionPolicyForRule(t, policies, recordRule)
			if got := len(policy.AcceptedMappings()); got != 12 {
				t.Fatalf(
					"accepted mappings without current and target-reviewed %s = %d, want 12",
					removedID,
					got,
				)
			}
		})
	}
}

func newProductionLocalPracticeETarget(
	t *testing.T,
) productionLocalPracticeETarget {
	t.Helper()
	base := productionHistoricalV1_2BaseArtifact(t)
	source, err := os.ReadFile(productionTypedMemoryCarrierPath)
	if err != nil {
		t.Fatalf("read production Local-Practice carrier: %v", err)
	}
	target, err := localpracticeruntime.Build(base, source)
	if err != nil {
		t.Fatalf("build production Local-Practice runtime: %v", err)
	}
	return productionLocalPracticeETarget{
		base:         target.Base(),
		extension:    target.Extension(),
		linked:       target.Linked(),
		requirements: target.Requirements(),
		runtime:      target.RuntimeBasis(),
		mechanism:    target.Mechanism(),
		policies:     target.RegistrationPolicies(),
		composite:    target.Composite(),
		preparation:  target.Preparation(),
		installed:    target.InstalledRuntime(),
	}
}

func productionHistoricalV1_2BaseArtifact(
	t *testing.T,
) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvRef(basetypeenvartifacts.HistoricalV3Ref)
	if err != nil {
		t.Fatalf("parse historical production Base reference: %v", err)
	}
	artifact, err := basetypeenvartifacts.LoadExact(ref)
	if err != nil {
		t.Fatalf("load historical production Base artifact: %v", err)
	}
	return artifact
}

func newProductionLocalPracticeRuntime(
	t *testing.T,
	requirements []projecttypeenv.CompositeRuntimeRequirement,
) (
	projecttypeenv.RuntimeEvaluationBasisArtifact,
	runtimemechanism.RuntimeMechanismArtifactV1,
	[]recordmembershipregistration.RegistrationArtifactV1,
) {
	return newProductionLocalPracticeRuntimeWithResolver(
		t,
		requirements,
		productionRegistrationMappingsForRule,
	)
}

type productionRegistrationMappingResolver func(
	*testing.T,
	typedmemory.RuleRef,
) []productionRegistrationMapping

func newProductionLocalPracticeRuntimeWithResolver(
	t *testing.T,
	requirements []projecttypeenv.CompositeRuntimeRequirement,
	resolver productionRegistrationMappingResolver,
) (
	projecttypeenv.RuntimeEvaluationBasisArtifact,
	runtimemechanism.RuntimeMechanismArtifactV1,
	[]recordmembershipregistration.RegistrationArtifactV1,
) {
	t.Helper()
	entries := make([]runtimemechanism.RuntimeMechanismEntryV1, 0, len(requirements))
	for _, requirement := range requirements {
		entry, err := productionLocalPracticeMechanismEntry(requirement)
		if err != nil {
			t.Fatalf("build production runtime mechanism entry: %v", err)
		}
		entries = append(entries, entry)
	}
	artifactRef, err := typedmemory.NewCarrierRef("artifact:haft-production-local-practice-runtime")
	if err != nil {
		t.Fatalf("NewCarrierRef(production runtime): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.2.0")
	if err != nil {
		t.Fatalf("NewCarrierEdition(production runtime): %v", err)
	}
	mechanism, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		entries,
	)
	if err != nil {
		t.Fatalf("seal production runtime mechanism catalog: %v", err)
	}
	mechanismPin, err := projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(mechanism)
	if err != nil {
		t.Fatalf("pin production runtime mechanism catalog: %v", err)
	}
	pins := make([]projecttypeenv.RuntimeEvaluationBasisPin, 0, len(requirements)+1)
	for _, requirement := range requirements {
		pin, pinErr := genesisE2EMechanismPin(requirement, mechanismPin, mechanism)
		if pinErr != nil {
			t.Fatalf("build production runtime mechanism pin: %v", pinErr)
		}
		pins = append(pins, pin)
	}
	policies := newProductionRegistrationPoliciesWithResolver(
		t,
		mechanism,
		requirements,
		resolver,
	)
	for _, policy := range policies {
		policyPin, pinErr := projecttypeenv.NewRegistrationPolicyPin(policy)
		if pinErr != nil {
			t.Fatalf("pin production registration policy: %v", pinErr)
		}
		pins = append(pins, policyPin)
	}
	runtime, err := projecttypeenv.SealRuntimeEvaluationBasisWithPins(
		pins,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("seal production exact X: %v", err)
	}
	return runtime, mechanism, policies
}

func productionLocalPracticeMechanismEntry(
	requirement projecttypeenv.CompositeRuntimeRequirement,
) (runtimemechanism.RuntimeMechanismEntryV1, error) {
	if codec, present := requirement.Codec(); present {
		return runtimemechanism.NewCodecCanonicalizationEntry(codec)
	}
	rule, present := requirement.Rule()
	if !present {
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"production Local-Practice requirement %q has no rule",
			requirement.SemanticReference(),
		)
	}
	constructors := map[projecttypeenv.RuntimeMechanismInvocationContract]func(
		typedmemory.RuleRef,
	) (runtimemechanism.RuntimeMechanismEntryV1, error){
		projecttypeenv.RuntimeMechanismContractEntitySetEnumeration:           runtimemechanism.NewEntitySetEnumerationEntry,
		projecttypeenv.RuntimeMechanismContractCandidateVisibility:            runtimemechanism.NewCandidateVisibilityEntry,
		projecttypeenv.RuntimeMechanismContractKindDefinedness:                runtimemechanism.NewKindDefinednessEntry,
		projecttypeenv.RuntimeMechanismContractMemberOf:                       runtimemechanism.NewMemberOfEntry,
		projecttypeenv.RuntimeMechanismContractCarrierMembershipDelivery:      runtimemechanism.NewCarrierMembershipDeliveryEntry,
		projecttypeenv.RuntimeMechanismContractReferenceDesignationResolution: runtimemechanism.NewReferenceDesignationResolutionEntry,
		projecttypeenv.RuntimeMechanismContractClaimInterpretation:            runtimemechanism.NewClaimInterpretationEntry,
		projecttypeenv.RuntimeMechanismContractClaimMeasurement:               runtimemechanism.NewClaimMeasurementEntry,
		projecttypeenv.RuntimeMechanismContractClaimEvaluation:                runtimemechanism.NewClaimEvaluationEntry,
		projecttypeenv.RuntimeMechanismContractEpistemeConstitutionEvaluation: runtimemechanism.NewEpistemeConstitutionEvaluationEntry,
	}
	constructor, present := constructors[requirement.InvocationContract()]
	if !present {
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"unsupported production Local-Practice invocation contract %q",
			requirement.InvocationContract(),
		)
	}
	return constructor(rule)
}

type productionRegistrationMapping struct {
	manifest recordmapping.MappingManifestRef
	adapter  recordmapping.AdapterVersion
}

func newProductionRegistrationPolicies(
	t *testing.T,
	mechanism runtimemechanism.RuntimeMechanismArtifactV1,
	requirements []projecttypeenv.CompositeRuntimeRequirement,
) []recordmembershipregistration.RegistrationArtifactV1 {
	return newProductionRegistrationPoliciesWithResolver(
		t,
		mechanism,
		requirements,
		productionRegistrationMappingsForRule,
	)
}

func newProductionRegistrationPoliciesWithResolver(
	t *testing.T,
	mechanism runtimemechanism.RuntimeMechanismArtifactV1,
	requirements []projecttypeenv.CompositeRuntimeRequirement,
	resolver productionRegistrationMappingResolver,
) []recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	rules := make(map[string]typedmemory.RuleRef)
	for _, requirement := range requirements {
		if requirement.InvocationContract() != projecttypeenv.RuntimeMechanismContractMemberOf {
			continue
		}
		rule, found := requirement.Rule()
		if !found {
			t.Fatal("production MemberOf requirement has no RuleRef")
		}
		rules[rule.String()] = rule
	}
	ordered := make([]string, 0, len(rules))
	for raw := range rules {
		ordered = append(ordered, raw)
	}
	sort.Strings(ordered)
	policies := make([]recordmembershipregistration.RegistrationArtifactV1, 0, len(ordered))
	for _, raw := range ordered {
		mappings := resolver(t, rules[raw])
		policies = append(
			policies,
			newProductionRegistrationPolicy(t, mechanism, rules[raw], mappings),
		)
	}
	return policies
}

func newProductionRegistrationPolicy(
	t *testing.T,
	mechanism runtimemechanism.RuntimeMechanismArtifactV1,
	rule typedmemory.RuleRef,
	mappings []productionRegistrationMapping,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	identity := mechanism.Identity()
	evaluator, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.EvaluatorMechanism,
			Rule:     rule,
			Artifact: identity.Artifact(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	)
	if err != nil {
		t.Fatalf("build production membership evaluator coordinate: %v", err)
	}
	delivery, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.SourceDeliveryBoundaryMechanism,
			Rule:     rule,
			Artifact: identity.Artifact(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	)
	if err != nil {
		t.Fatalf("build production membership delivery coordinate: %v", err)
	}
	accepted := newProductionAcceptedMappings(t, mappings)
	policy, err := recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings:       accepted,
		},
	)
	if err != nil {
		t.Fatalf("seal production registration policy: %v", err)
	}
	return policy
}

func newProductionAcceptedMappings(
	t *testing.T,
	mappings []productionRegistrationMapping,
) []recordmembershipregistration.AcceptedMapping {
	t.Helper()
	accepted := make(
		[]recordmembershipregistration.AcceptedMapping,
		0,
		len(mappings),
	)
	for _, mapping := range mappings {
		value, err := recordmembershipregistration.NewAcceptedMapping(
			recordmembershipregistration.AcceptedMappingInput{
				Manifest: mapping.manifest,
				Adapter:  mapping.adapter,
			},
		)
		if err != nil {
			t.Fatalf("build production accepted mapping: %v", err)
		}
		accepted = append(accepted, value)
	}
	return accepted
}

func productionRegistrationMappingsForRule(
	t *testing.T,
	rule typedmemory.RuleRef,
) []productionRegistrationMapping {
	t.Helper()
	projectEntityRule, err := typedmemory.NewRuleRef("haft.member-of.project-entity/v1")
	if err != nil {
		t.Fatal(err)
	}
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	if rule == projectEntityRule {
		manifest, manifestErr := projectmemory.CurrentProjectEntityUniverseMappingManifestV1()
		if manifestErr != nil {
			t.Fatalf("load project-entity mapping manifest: %v", manifestErr)
		}
		return []productionRegistrationMapping{{
			manifest: manifest.Ref(),
			adapter:  manifest.AdapterVersion(),
		}}
	}
	if rule == recordRule {
		note, noteErr := noteadapter.CurrentMappingManifestV1()
		if noteErr != nil {
			t.Fatalf("load Note mapping manifest: %v", noteErr)
		}
		problem, problemErr := problemcardadapter.CurrentMappingManifestV1()
		if problemErr != nil {
			t.Fatalf("load ProblemCard mapping manifest: %v", problemErr)
		}
		portfolio, portfolioErr :=
			solutionportfolioadapter.CurrentMappingManifestV1()
		if portfolioErr != nil {
			t.Fatalf("load SolutionPortfolio mapping manifest: %v", portfolioErr)
		}
		comparison, comparisonErr :=
			portfoliocomparisonadapter.CurrentMappingManifestV1()
		if comparisonErr != nil {
			t.Fatalf("load PortfolioComparison mapping manifest: %v", comparisonErr)
		}
		specSection, specSectionErr :=
			specsectionadapter.CurrentMappingManifestV1()
		if specSectionErr != nil {
			t.Fatalf("load SpecSection mapping manifest: %v", specSectionErr)
		}
		evidenceWork, evidenceWorkErr :=
			evidenceworkadapter.CurrentMappingManifestV1()
		if evidenceWorkErr != nil {
			t.Fatalf("load Evidence/Work mapping manifest: %v", evidenceWorkErr)
		}
		decision, decisionErr :=
			decisionrecordadapter.CurrentMappingManifestV1()
		if decisionErr != nil {
			t.Fatalf("load DecisionRecord mapping manifest: %v", decisionErr)
		}
		current := []productionRegistrationMapping{
			{
				manifest: note.Ref(),
				adapter:  note.AdapterVersion(),
			},
			{
				manifest: problem.Ref(),
				adapter:  problem.AdapterVersion(),
			},
			{
				manifest: portfolio.Ref(),
				adapter:  portfolio.AdapterVersion(),
			},
			{
				manifest: comparison.Ref(),
				adapter:  comparison.AdapterVersion(),
			},
			{
				manifest: specSection.Ref(),
				adapter:  specSection.AdapterVersion(),
			},
			{
				manifest: evidenceWork.Ref(),
				adapter:  evidenceWork.AdapterVersion(),
			},
			{
				manifest: decision.Ref(),
				adapter:  decision.AdapterVersion(),
			},
		}
		compatible, compatibilityErr :=
			localpracticeruntime.ProjectRecordTargetReviewedCompatibilityMappingsV1()
		if compatibilityErr != nil {
			t.Fatalf("load ProjectRecord target-reviewed mappings: %v", compatibilityErr)
		}
		return appendProductionAcceptedMappings(current, compatible)
	}
	if rule == carrierfamily.CodeAnchorEvaluatorRuleV1() {
		family, familyErr :=
			carrierfamily.CurrentCodeAnchorMappingManifestV1()
		if familyErr != nil {
			t.Fatalf("load CodeAnchor carrier-family mapping manifest: %v", familyErr)
		}
		task, taskErr := codeanchoradapter.CurrentMappingManifestV1()
		if taskErr != nil {
			t.Fatalf("load CodeAnchor task mapping manifest: %v", taskErr)
		}
		current := []productionRegistrationMapping{
			{
				manifest: family.Ref(),
				adapter:  family.AdapterVersion(),
			},
			{
				manifest: task.Ref(),
				adapter:  task.AdapterVersion(),
			},
		}
		compatible, compatibilityErr :=
			localpracticeruntime.CodeAnchorTargetReviewedCompatibilityMappingsV1()
		if compatibilityErr != nil {
			t.Fatalf("load CodeAnchor target-reviewed mappings: %v", compatibilityErr)
		}
		return appendProductionAcceptedMappings(current, compatible)
	}
	manifest, manifestErr := productionCarrierFamilyManifest(rule)
	if manifestErr != nil {
		t.Fatalf("load carrier-family mapping manifest for %q: %v", rule.String(), manifestErr)
	}
	return []productionRegistrationMapping{{
		manifest: manifest.Ref(),
		adapter:  manifest.AdapterVersion(),
	}}
}

func appendProductionAcceptedMappings(
	current []productionRegistrationMapping,
	compatible []recordmembershipregistration.AcceptedMapping,
) []productionRegistrationMapping {
	result := append([]productionRegistrationMapping(nil), current...)
	for _, mapping := range compatible {
		result = append(result, productionRegistrationMapping{
			manifest: mapping.Manifest(),
			adapter:  mapping.Adapter(),
		})
	}
	return result
}

func assertProductionProjectRecordMappings(
	t *testing.T,
	policies []recordmembershipregistration.RegistrationArtifactV1,
) {
	t.Helper()
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	var matches []recordmembershipregistration.RegistrationArtifactV1
	for _, policy := range policies {
		if policy.Evaluator().Rule() == recordRule {
			matches = append(matches, policy)
		}
	}
	if len(matches) != 1 {
		t.Fatalf(
			"project-record registration policies = %d, want exactly 1",
			len(matches),
		)
	}
	policy := matches[0]
	if got := len(policy.AcceptedMappings()); got != 14 {
		t.Fatalf(
			"project-record accepted mappings = %d, want current and target-reviewed shipped-v1 coordinates for seven producer families",
			got,
		)
	}
	note, noteErr := noteadapter.CurrentMappingManifestV1()
	if noteErr != nil {
		t.Fatalf("load exact Note mapping manifest: %v", noteErr)
	}
	problem, problemErr := problemcardadapter.CurrentMappingManifestV1()
	if problemErr != nil {
		t.Fatalf("load exact ProblemCard mapping manifest: %v", problemErr)
	}
	portfolio, portfolioErr :=
		solutionportfolioadapter.CurrentMappingManifestV1()
	if portfolioErr != nil {
		t.Fatalf("load exact SolutionPortfolio mapping manifest: %v", portfolioErr)
	}
	comparison, comparisonErr :=
		portfoliocomparisonadapter.CurrentMappingManifestV1()
	if comparisonErr != nil {
		t.Fatalf("load exact PortfolioComparison mapping manifest: %v", comparisonErr)
	}
	specSection, specSectionErr :=
		specsectionadapter.CurrentMappingManifestV1()
	if specSectionErr != nil {
		t.Fatalf("load exact SpecSection mapping manifest: %v", specSectionErr)
	}
	evidenceWork, evidenceWorkErr :=
		evidenceworkadapter.CurrentMappingManifestV1()
	if evidenceWorkErr != nil {
		t.Fatalf("load exact Evidence/Work mapping manifest: %v", evidenceWorkErr)
	}
	decision, decisionErr :=
		decisionrecordadapter.CurrentMappingManifestV1()
	if decisionErr != nil {
		t.Fatalf("load exact DecisionRecord mapping manifest: %v", decisionErr)
	}
	assertProductionMappingAccepted(
		t,
		policy,
		note.Ref(),
		note.AdapterVersion(),
	)
	assertProductionMappingAccepted(
		t,
		policy,
		problem.Ref(),
		problem.AdapterVersion(),
	)
	assertProductionMappingAccepted(
		t,
		policy,
		portfolio.Ref(),
		portfolio.AdapterVersion(),
	)
	assertProductionMappingAccepted(
		t,
		policy,
		comparison.Ref(),
		comparison.AdapterVersion(),
	)
	assertProductionMappingAccepted(
		t,
		policy,
		specSection.Ref(),
		specSection.AdapterVersion(),
	)
	assertProductionMappingAccepted(
		t,
		policy,
		evidenceWork.Ref(),
		evidenceWork.AdapterVersion(),
	)
	assertProductionMappingAccepted(
		t,
		policy,
		decision.Ref(),
		decision.AdapterVersion(),
	)
	compatible, compatibilityErr :=
		localpracticeruntime.ProjectRecordTargetReviewedCompatibilityMappingsV1()
	if compatibilityErr != nil {
		t.Fatalf("load ProjectRecord target-reviewed mappings: %v", compatibilityErr)
	}
	for _, mapping := range compatible {
		assertProductionMappingAccepted(
			t,
			policy,
			mapping.Manifest(),
			mapping.Adapter(),
		)
	}
}

func assertProductionCodeAnchorMappings(
	t *testing.T,
	policies []recordmembershipregistration.RegistrationArtifactV1,
) {
	t.Helper()
	rule := carrierfamily.CodeAnchorEvaluatorRuleV1()
	policy := productionPolicyForRule(t, policies, rule)
	if got := len(policy.AcceptedMappings()); got != 3 {
		t.Fatalf(
			"CodeAnchor accepted mappings = %d, want carrier-family plus current and target-reviewed shipped-v1 task adapters",
			got,
		)
	}
	family, familyErr := carrierfamily.CurrentCodeAnchorMappingManifestV1()
	if familyErr != nil {
		t.Fatalf("load exact CodeAnchor carrier-family mapping: %v", familyErr)
	}
	task, taskErr := codeanchoradapter.CurrentMappingManifestV1()
	if taskErr != nil {
		t.Fatalf("load exact CodeAnchor task mapping: %v", taskErr)
	}
	assertProductionMappingAccepted(
		t,
		policy,
		family.Ref(),
		family.AdapterVersion(),
	)
	assertProductionMappingAccepted(
		t,
		policy,
		task.Ref(),
		task.AdapterVersion(),
	)
	compatible, compatibilityErr :=
		localpracticeruntime.CodeAnchorTargetReviewedCompatibilityMappingsV1()
	if compatibilityErr != nil {
		t.Fatalf("load CodeAnchor target-reviewed mappings: %v", compatibilityErr)
	}
	for _, mapping := range compatible {
		assertProductionMappingAccepted(
			t,
			policy,
			mapping.Manifest(),
			mapping.Adapter(),
		)
	}
}

func assertProductionMappingAccepted(
	t *testing.T,
	policy recordmembershipregistration.RegistrationArtifactV1,
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) {
	t.Helper()
	decision, err := policy.EvaluateMappingPolicy(manifest, adapter)
	if err != nil {
		t.Fatalf("evaluate production accepted mapping: %v", err)
	}
	if decision.Kind() != recordmembershipregistration.MappingAccepted {
		t.Fatalf(
			"production mapping %s = %s, want accepted",
			manifest.String(),
			decision.Kind().String(),
		)
	}
}

func productionCarrierFamilyManifest(
	rule typedmemory.RuleRef,
) (carrierfamily.MappingManifestV1, error) {
	constructors := map[string]func() (carrierfamily.MappingManifestV1, error){
		carrierfamily.CarrierEditionEvaluatorRuleV1().String():          carrierfamily.CurrentCarrierEditionMappingManifestV1,
		carrierfamily.ProjectClaimEvaluatorRuleV1().String():            carrierfamily.CurrentProjectClaimMappingManifestV1,
		carrierfamily.PerformedWorkOccurrenceEvaluatorRuleV1().String(): carrierfamily.CurrentPerformedWorkOccurrenceMappingManifestV1,
		carrierfamily.CodeAnchorEvaluatorRuleV1().String():              carrierfamily.CurrentCodeAnchorMappingManifestV1,
	}
	constructor, found := constructors[rule.String()]
	if !found {
		return carrierfamily.MappingManifestV1{}, fmt.Errorf(
			"unsupported production MemberOf rule %q",
			rule.String(),
		)
	}
	return constructor()
}

func productionExtensionHasDeclaration(
	extension projecttypeenv.ProjectTypeEnvExtensionArtifact,
	kind localpractice.DeclarationKind,
	symbol string,
) bool {
	declarations := extension.IR().Signature().Vocabulary().Declarations()
	for _, declaration := range declarations {
		if declaration.Kind() == kind && declaration.Symbol().Value() == symbol {
			return true
		}
	}
	return false
}

func productionRequirementContractCounts(
	requirements projecttypeenv.CompositeRuntimeRequirementSet,
) map[projecttypeenv.RuntimeMechanismInvocationContract]int {
	counts := make(map[projecttypeenv.RuntimeMechanismInvocationContract]int)
	for _, requirement := range requirements.Requirements() {
		counts[requirement.InvocationContract()]++
	}
	return counts
}

func assertProductionRequirementCount(
	t *testing.T,
	counts map[projecttypeenv.RuntimeMechanismInvocationContract]int,
	contract projecttypeenv.RuntimeMechanismInvocationContract,
	minimum int,
) {
	t.Helper()
	if counts[contract] < minimum {
		t.Fatalf(
			"production runtime %s requirements = %d, want at least %d",
			contract,
			counts[contract],
			minimum,
		)
	}
}
