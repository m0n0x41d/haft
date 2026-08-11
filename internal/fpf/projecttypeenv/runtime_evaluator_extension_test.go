package projecttypeenv

import (
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	runtimeEvaluatorCandidateV1Path  = "../../../data/haft/local-practice/typed-memory/candidates/1.0.0.yaml"
	runtimeEvaluatorCandidateV16Path = "../../../data/haft/local-practice/typed-memory/candidates/1.6.0.yaml"
)

var runtimeEvaluatorRequirementCoordinates = map[RuntimeMechanismInvocationContract]string{
	RuntimeMechanismContractReferenceDesignationResolution: "haft.reference-scheme.project-memory/v1/designation-resolution",
	RuntimeMechanismContractClaimInterpretation:            "haft.reference-scheme.project-memory/v1/claim-interpretation",
	RuntimeMechanismContractClaimMeasurement:               "haft.reference-scheme.project-memory/v1/claim-measurement",
	RuntimeMechanismContractClaimEvaluation:                "haft.reference-scheme.project-memory/v1/claim-evaluation",
	RuntimeMechanismContractEpistemeConstitutionEvaluation: "haft.episteme-constitution.project-memory/v1/evaluate",
}

type runtimeEvaluatorExtensionFixture struct {
	base     typeenv.BaseTypeEnvArtifact
	ir       ProjectTypeEnvExtensionIR
	artifact ProjectTypeEnvExtensionArtifact
	linked   LinkedProjectTypeEnvCompositeIR
}

func TestRuntimeEvaluatorDeclarationsCompileAndLinkExactSourceContract(t *testing.T) {
	fixture := newRuntimeEvaluatorExtensionFixture(t)
	decoded, err := DecodeProjectTypeEnvExtensionArtifact(
		fixture.artifact.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("decode runtime evaluator E artifact: %v", err)
	}
	if decoded.Ref() != fixture.artifact.Ref() {
		t.Fatal("runtime evaluator E artifact changed identity across decode")
	}
	input := runtimeEvaluatorDeclarationBySymbol(
		t,
		fixture.ir,
		"Haft.ProjectEpistemeConstitutionBasis",
	)
	if input.Kind() != localpractice.DeclarationRuntimeEvaluatorInput {
		t.Fatalf("constitution basis kind = %q", input.Kind())
	}

	wantExports := []string{
		"Haft.ProjectEpistemeConstitutionBasis",
		"Haft.ProjectEpistemeConstitutionBasis.ClaimGraphSlot",
		"Haft.ProjectEpistemeConstitutionBasis.EntityOfConcernSlot",
		"Haft.ProjectEpistemeConstitutionBasis.ReferenceSchemeSlot",
	}
	gotExports := runtimeEvaluatorScalarValues(input.Exports())
	sort.Strings(wantExports)
	if !slices.Equal(gotExports, wantExports) {
		t.Fatalf("constitution input exports = %#v, want %#v", gotExports, wantExports)
	}

	wantFacts := map[string]string{
		"evaluator_requirement": "Haft.RuntimeRequirement.ProjectEpistemeConstitutionEvaluationV1",
	}
	runtimeEvaluatorAddSlotFacts(
		wantFacts,
		"Haft.ProjectEpistemeConstitutionBasis.ClaimGraphSlot",
		"U.ClaimGraph",
		"by_value",
		"",
	)
	runtimeEvaluatorAddSlotFacts(
		wantFacts,
		"Haft.ProjectEpistemeConstitutionBasis.EntityOfConcernSlot",
		"U.Entity",
		"reference",
		"U.EntityRef",
	)
	runtimeEvaluatorAddSlotFacts(
		wantFacts,
		"Haft.ProjectEpistemeConstitutionBasis.ReferenceSchemeSlot",
		"U.ReferenceScheme",
		"by_value",
		"",
	)
	if got := runtimeEvaluatorFactMap(input.Facts()); !runtimeEvaluatorStringMapsEqual(got, wantFacts) {
		t.Fatalf("constitution input facts = %#v, want %#v", got, wantFacts)
	}

	wantDependencies := map[string]string{
		"evaluator_requirement": "Haft.RuntimeRequirement.ProjectEpistemeConstitutionEvaluationV1",
		keyedPath("slots", "Haft.ProjectEpistemeConstitutionBasis.ClaimGraphSlot") + ".value_kind":             "U.ClaimGraph",
		keyedPath("slots", "Haft.ProjectEpistemeConstitutionBasis.EntityOfConcernSlot") + ".value_kind":        "U.Entity",
		keyedPath("slots", "Haft.ProjectEpistemeConstitutionBasis.EntityOfConcernSlot") + ".ref_mode.ref_kind": "U.EntityRef",
		keyedPath("slots", "Haft.ProjectEpistemeConstitutionBasis.ReferenceSchemeSlot") + ".value_kind":        "U.ReferenceScheme",
	}
	if got := runtimeEvaluatorDependencyMap(input.Dependencies()); !runtimeEvaluatorStringMapsEqual(got, wantDependencies) {
		t.Fatalf("constitution input dependencies = %#v, want %#v", got, wantDependencies)
	}

	runtimeEvaluatorAssertRequirementIR(t, fixture.ir)
	runtimeEvaluatorAssertLinkedRequirementDependency(t, fixture.linked)
	runtimeEvaluatorAssertExternalRuleCoordinates(t, fixture.linked)
	runtimeEvaluatorAssertInputIsNotRelation(t, fixture)
}

func TestRuntimeEvaluatorDiscoveryEmitsExactlyFiveExplicitRequirements(t *testing.T) {
	fixture := newRuntimeEvaluatorExtensionFixture(t)
	discovery := DiscoverProjectTypeEnvCompositeRuntimeRequirements(
		fixture.base,
		fixture.linked,
	)
	if discovery.Rejected() {
		t.Fatalf("runtime evaluator discovery rejected: %#v", discovery.Issues())
	}
	required, exists := discovery.RequiredSet()
	if !exists {
		t.Fatal("accepted runtime evaluator discovery has no requirement set")
	}

	got := make(map[RuntimeMechanismInvocationContract]string)
	for _, requirement := range required.Requirements() {
		if _, expected := runtimeEvaluatorRequirementCoordinates[requirement.InvocationContract()]; !expected {
			continue
		}
		if requirement.Role() != RuntimeMechanismRoleEvaluator {
			t.Fatalf(
				"runtime evaluator requirement %q has role %q",
				requirement.SemanticReference(),
				requirement.Role(),
			)
		}
		if _, duplicate := got[requirement.InvocationContract()]; duplicate {
			t.Fatalf("duplicate discovered contract %q", requirement.InvocationContract())
		}
		got[requirement.InvocationContract()] = requirement.SemanticReference()
	}
	if !runtimeEvaluatorContractMapsEqual(got, runtimeEvaluatorRequirementCoordinates) {
		t.Fatalf("explicit runtime evaluator requirements = %#v", got)
	}

	sources := canonicalCompositeSourceDeclarations(fixture.linked)
	duplicate := runtimeEvaluatorSourceByKind(
		t,
		sources,
		localpractice.DeclarationRuntimeEvaluatorRequirement,
	)
	duplicate.value.symbol.value += ".Duplicate"
	_, err := compositeExplicitSourceEvaluatorRuntimeRequirements(
		append(sources, duplicate),
	)
	if err == nil || !strings.Contains(err.Error(), "repeat semantic coordinate") {
		t.Fatalf("duplicate explicit requirement error = %v", err)
	}
}

func TestRuntimeEvaluatorRequirementsFailClosedAgainstInexactX(t *testing.T) {
	required := runtimeEvaluatorRequiredSet(t)
	exactPins := runtimeEvaluatorExactPins(t, required)
	if issues := compareCompositeRuntimeRequirements(required, exactPins); len(issues) != 0 {
		t.Fatalf("exact runtime evaluator pins produced issues: %#v", issues)
	}

	claimEvaluationRule := runtimeEvaluatorRequirementCoordinates[RuntimeMechanismContractClaimEvaluation]
	t.Run("missing", func(t *testing.T) {
		pins := compositeRuntimePinsWithout(
			exactPins,
			RuntimeMechanismRoleEvaluator,
			claimEvaluationRule,
		)
		runtimeEvaluatorAssertOneRequirementIssue(
			t,
			compareCompositeRuntimeRequirements(required, pins),
			CompositeRuntimeIssueMissing,
			RuntimeMechanismContractClaimEvaluation,
			0,
		)
	})
	t.Run("wrong contract", func(t *testing.T) {
		pins := compositeRuntimePinsWithout(
			exactPins,
			RuntimeMechanismRoleEvaluator,
			claimEvaluationRule,
		)
		wrong := runtimeEvaluatorMechanismPinWithContract(
			t,
			claimEvaluationRule,
			RuntimeMechanismContractClaimMeasurement,
			"artifact:runtime-evaluator-wrong-contract",
			"1.0.0",
			0xd1,
		)
		pins = append(pins, wrong)
		runtimeEvaluatorAssertOneRequirementIssue(
			t,
			compareCompositeRuntimeRequirements(required, pins),
			CompositeRuntimeIssueWrongContract,
			RuntimeMechanismContractClaimEvaluation,
			RuntimeMechanismContractClaimMeasurement,
		)
	})
	t.Run("wrong role", func(t *testing.T) {
		pins := compositeRuntimePinsWithout(
			exactPins,
			RuntimeMechanismRoleEvaluator,
			claimEvaluationRule,
		)
		wrong := runtimeCarrierMembershipMechanismPin(
			t,
			claimEvaluationRule,
			"artifact:runtime-evaluator-wrong-role",
			"1.0.0",
			0xd2,
		)
		pins = append(pins, wrong)
		issues := compareCompositeRuntimeRequirements(required, pins)
		runtimeEvaluatorAssertOneRequirementIssue(
			t,
			issues,
			CompositeRuntimeIssueWrongRole,
			RuntimeMechanismContractClaimEvaluation,
			RuntimeMechanismContractCarrierMembershipDelivery,
		)
		if issues[0].ExpectedRole() != RuntimeMechanismRoleEvaluator ||
			issues[0].ActualRole() != RuntimeMechanismRoleCarrierMembership {
			t.Fatalf("wrong-role issue = %#v", issues[0])
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		duplicate := append(
			append([]RuntimeEvaluationMechanismPin(nil), exactPins...),
			exactPins[0],
		)
		_, err := SealRuntimeEvaluationBasis(duplicate)
		if err == nil || !strings.Contains(err.Error(), "repeats semantic coordinate") {
			t.Fatalf("duplicate runtime evaluator pin error = %v", err)
		}
	})
}

func TestRuntimeEvaluatorInputRequiresRuntimeRequirementDeclarationProvider(t *testing.T) {
	fixture := newRuntimeEvaluatorExtensionFixture(t)
	mutated := fixture.ir
	input := runtimeEvaluatorDeclarationPointerBySymbol(
		t,
		&mutated,
		"Haft.ProjectEpistemeConstitutionBasis",
	)
	constraint := runtimeEvaluatorDeclarationByKind(
		t,
		mutated,
		localpractice.DeclarationConstraint,
	)
	for index := range input.facts {
		if input.facts[index].path == "evaluator_requirement" {
			input.facts[index].value.value = constraint.Symbol().Value()
		}
	}
	for index := range input.dependencies {
		if input.dependencies[index].role == "evaluator_requirement" {
			input.dependencies[index].target.value = constraint.Symbol().Value()
		}
	}
	artifact := sealExtension(t, mutated)
	resolution := LinkProjectTypeEnvCompositeIR(
		fixture.base,
		[]ProjectTypeEnvExtensionArtifact{artifact},
	)
	assertCompositeIssue(t, resolution, IssueDependencyKindMismatch)
}

func TestRuntimeEvaluatorCompilerPreservesHistoricalProductionV1(t *testing.T) {
	source, err := os.ReadFile(runtimeEvaluatorCandidateV1Path)
	if err != nil {
		t.Fatalf("read historical production carrier: %v", err)
	}
	parsed, err := localpractice.Parse(source)
	if err != nil {
		t.Fatalf("parse historical production carrier: %v", err)
	}
	carrier := parsed.Carrier()
	manifest := carrier.Manifest()
	node := ResolvedManifestNode{
		carrier: parsed,
		coordinate: newManifestCoordinate(
			manifest.ID().Value(),
			manifest.Version().Value(),
		),
	}
	ir, err := CompileProjectTypeEnvExtensionIR(node, nil)
	if err != nil {
		t.Fatalf("compile historical production carrier: %v", err)
	}
	artifact, err := SealProjectTypeEnvExtension(ir)
	if err != nil {
		t.Fatalf("seal historical production carrier: %v", err)
	}
	if err := artifact.Verify(); err != nil {
		t.Fatalf("verify historical production E: %v", err)
	}
	for _, declaration := range ir.Signature().Vocabulary().Declarations() {
		if declaration.Kind() == localpractice.DeclarationRuntimeEvaluatorInput ||
			declaration.Kind() == localpractice.DeclarationRuntimeEvaluatorRequirement {
			t.Fatalf("historical production E gained declaration kind %q", declaration.Kind())
		}
	}
}

func newRuntimeEvaluatorExtensionFixture(
	t *testing.T,
) runtimeEvaluatorExtensionFixture {
	t.Helper()
	base := loadBaseArtifact(t)
	source, err := os.ReadFile(runtimeEvaluatorCandidateV16Path)
	if err != nil {
		t.Fatalf("read runtime evaluator production carrier: %v", err)
	}
	parsed := parseCarrier(t, source)
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{parsed},
	)
	ir := compileExtensionIR(t, bundle.Nodes()[0], nil)
	artifact := sealExtension(t, ir)
	linked := acceptedCompositeIR(
		t,
		LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{artifact},
		),
	)
	return runtimeEvaluatorExtensionFixture{
		base:     base,
		ir:       ir,
		artifact: artifact,
		linked:   linked,
	}
}

func runtimeEvaluatorAssertRequirementIR(
	t *testing.T,
	ir ProjectTypeEnvExtensionIR,
) {
	t.Helper()
	want := make(map[string]struct{}, len(runtimeEvaluatorRequirementCoordinates))
	for contract, rule := range runtimeEvaluatorRequirementCoordinates {
		want[contract.String()+"\x00"+rule] = struct{}{}
	}
	got := make(map[string]struct{}, len(want))
	for _, declaration := range ir.Signature().Vocabulary().Declarations() {
		if declaration.Kind() != localpractice.DeclarationRuntimeEvaluatorRequirement {
			continue
		}
		if len(declaration.Exports()) != 1 ||
			declaration.Exports()[0].Value() != declaration.Symbol().Value() {
			t.Fatalf("runtime requirement %q exports = %#v", declaration.Symbol().Value(), declaration.Exports())
		}
		if len(declaration.Dependencies()) != 0 {
			t.Fatalf("runtime requirement %q dependencies = %#v", declaration.Symbol().Value(), declaration.Dependencies())
		}
		facts := runtimeEvaluatorFactMap(declaration.Facts())
		if len(facts) != 2 || facts["rule_ref"] == "" || facts["invocation_contract"] == "" {
			t.Fatalf("runtime requirement %q facts = %#v", declaration.Symbol().Value(), facts)
		}
		got[facts["invocation_contract"]+"\x00"+facts["rule_ref"]] = struct{}{}
	}
	if !runtimeEvaluatorSetMapsEqual(got, want) {
		t.Fatalf("runtime requirement IR coordinates = %#v, want %#v", got, want)
	}
}

func runtimeEvaluatorAssertLinkedRequirementDependency(
	t *testing.T,
	linked LinkedProjectTypeEnvCompositeIR,
) {
	t.Helper()
	const origin = "declaration:Haft.ProjectEpistemeConstitutionBasis"
	for _, dependency := range linked.DependencyResolutions() {
		if dependency.Origin() != origin || dependency.Role() != "evaluator_requirement" {
			continue
		}
		provider, ok := dependency.Provider().(ExtensionCompositeSymbolProvider)
		if !ok || provider.declarationKind != localpractice.DeclarationRuntimeEvaluatorRequirement {
			t.Fatalf("evaluator requirement provider = %#v", dependency.Provider())
		}
		if dependency.Scope() != CompositeDependencyOwn ||
			provider.RawSymbol() != "Haft.RuntimeRequirement.ProjectEpistemeConstitutionEvaluationV1" {
			t.Fatalf("evaluator requirement dependency = %#v", dependency)
		}
		return
	}
	t.Fatal("constitution input evaluator requirement dependency was not resolved")
}

func runtimeEvaluatorAssertExternalRuleCoordinates(
	t *testing.T,
	linked LinkedProjectTypeEnvCompositeIR,
) {
	t.Helper()
	want := make(map[string]struct{}, len(runtimeEvaluatorRequirementCoordinates))
	for _, rule := range runtimeEvaluatorRequirementCoordinates {
		want[rule] = struct{}{}
	}
	got := make(map[string]struct{}, len(want))
	for _, reference := range linked.ExternalReferences() {
		if reference.Role() != "rule_ref" ||
			!strings.HasPrefix(reference.Origin(), "declaration:Haft.RuntimeRequirement.") {
			continue
		}
		if reference.Kind() != CompositeExternalRule {
			t.Fatalf("runtime requirement external kind = %q", reference.Kind())
		}
		got[reference.Source().Value()] = struct{}{}
	}
	if !runtimeEvaluatorSetMapsEqual(got, want) {
		t.Fatalf("runtime requirement external RuleRefs = %#v, want %#v", got, want)
	}
}

func runtimeEvaluatorAssertInputIsNotRelation(
	t *testing.T,
	fixture runtimeEvaluatorExtensionFixture,
) {
	t.Helper()
	sources := canonicalCompositeSourceDeclarations(fixture.linked)
	cardinalities, err := compositeSlotCardinalityIndex(sources)
	if err != nil {
		t.Fatalf("derive relation cardinalities: %v", err)
	}
	target, exists := fixture.base.TypeEnvRef()
	if !exists {
		t.Fatal("runtime evaluator fixture base has no TypeEnvRef")
	}
	provenance := func(
		source compositeSourceDeclaration,
		semantic string,
	) (typedmemory.ProjectSourceProvenance, error) {
		return compositeDeclarationProvenance(fixture.linked, source, semantic)
	}
	fragments, _, err := lowerCompositeRelationDeclarationFragments(
		target,
		sources,
		cardinalities,
		provenance,
	)
	if err != nil {
		t.Fatalf("lower typed relation declaration fragments: %v", err)
	}
	relationDeclarations := declarationsOfKind(
		sources,
		localpractice.DeclarationRelationSignature,
	)
	if len(fragments) != len(relationDeclarations) {
		t.Fatalf(
			"lowered relation fragments = %d, declared relation aliases = %d",
			len(fragments),
			len(relationDeclarations),
		)
	}
	for _, fragment := range fragments {
		if fragment.Ref().ID().String() == "Haft.ProjectEpistemeConstitutionBasis" {
			t.Fatal("runtime evaluator input was lowered as a RelationSignature")
		}
	}
}

func runtimeEvaluatorRequiredSet(t *testing.T) CompositeRuntimeRequirementSet {
	t.Helper()
	requirements := make([]CompositeRuntimeRequirement, 0, len(runtimeEvaluatorRequirementCoordinates))
	for contract, rawRule := range runtimeEvaluatorRequirementCoordinates {
		rule, err := typedmemory.NewRuleRef(rawRule)
		if err != nil {
			t.Fatalf("NewRuleRef(%q): %v", rawRule, err)
		}
		requirements = append(
			requirements,
			newCompositeRuleRuntimeRequirement(
				RuntimeMechanismRoleEvaluator,
				contract,
				rule,
			),
		)
	}
	set, err := newCompositeRuntimeRequirementSet(requirements)
	if err != nil {
		t.Fatalf("newCompositeRuntimeRequirementSet(): %v", err)
	}
	return set
}

func runtimeEvaluatorExactPins(
	t *testing.T,
	required CompositeRuntimeRequirementSet,
) []RuntimeEvaluationMechanismPin {
	t.Helper()
	requirements := required.Requirements()
	pins := make([]RuntimeEvaluationMechanismPin, 0, len(requirements))
	for index, requirement := range requirements {
		pin := runtimeEvaluatorMechanismPinWithContract(
			t,
			requirement.SemanticReference(),
			requirement.InvocationContract(),
			"artifact:runtime-evaluator-exact-"+requirement.InvocationContract().String(),
			"1.0.0",
			byte(0xa0+index),
		)
		pins = append(pins, pin)
	}
	return pins
}

func runtimeEvaluatorAssertOneRequirementIssue(
	t *testing.T,
	issues []CompositeRuntimeRequirementIssue,
	code CompositeRuntimeRequirementIssueCode,
	expected RuntimeMechanismInvocationContract,
	actual RuntimeMechanismInvocationContract,
) {
	t.Helper()
	if len(issues) != 1 || issues[0].Code() != code ||
		issues[0].ExpectedContract() != expected ||
		issues[0].ActualContract() != actual {
		t.Fatalf("runtime requirement issues = %#v", issues)
	}
}

func runtimeEvaluatorDeclarationBySymbol(
	t *testing.T,
	ir ProjectTypeEnvExtensionIR,
	symbol string,
) SymbolicDeclaration {
	t.Helper()
	for _, declaration := range ir.Signature().Vocabulary().Declarations() {
		if declaration.Symbol().Value() == symbol {
			return declaration
		}
	}
	t.Fatalf("symbolic declaration %q was not found", symbol)
	return SymbolicDeclaration{}
}

func runtimeEvaluatorDeclarationPointerBySymbol(
	t *testing.T,
	ir *ProjectTypeEnvExtensionIR,
	symbol string,
) *SymbolicDeclaration {
	t.Helper()
	for index := range ir.signature.vocabulary.declarations {
		declaration := &ir.signature.vocabulary.declarations[index]
		if declaration.symbol.value == symbol {
			return declaration
		}
	}
	t.Fatalf("symbolic declaration %q was not found", symbol)
	return nil
}

func runtimeEvaluatorDeclarationByKind(
	t *testing.T,
	ir ProjectTypeEnvExtensionIR,
	kind localpractice.DeclarationKind,
) SymbolicDeclaration {
	t.Helper()
	for _, declaration := range ir.Signature().Vocabulary().Declarations() {
		if declaration.Kind() == kind {
			return declaration
		}
	}
	t.Fatalf("symbolic declaration kind %q was not found", kind)
	return SymbolicDeclaration{}
}

func runtimeEvaluatorSourceByKind(
	t *testing.T,
	sources []compositeSourceDeclaration,
	kind localpractice.DeclarationKind,
) compositeSourceDeclaration {
	t.Helper()
	for _, source := range sources {
		if source.value.Kind() == kind {
			return source
		}
	}
	t.Fatalf("composite source declaration kind %q was not found", kind)
	return compositeSourceDeclaration{}
}

func runtimeEvaluatorScalarValues(values []SourceScalar) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Value())
	}
	sort.Strings(result)
	return result
}

func runtimeEvaluatorFactMap(facts []SourceFact) map[string]string {
	result := make(map[string]string, len(facts))
	for _, fact := range facts {
		result[fact.Path()] = fact.Value().Value()
	}
	return result
}

func runtimeEvaluatorDependencyMap(
	dependencies []SymbolicDependency,
) map[string]string {
	result := make(map[string]string, len(dependencies))
	for _, dependency := range dependencies {
		result[dependency.Role()] = dependency.Target().Value()
	}
	return result
}

func runtimeEvaluatorAddSlotFacts(
	facts map[string]string,
	slot string,
	valueKind string,
	mode string,
	refKind string,
) {
	prefix := keyedPath("slots", slot)
	facts[prefix+".slot_kind"] = slot
	facts[prefix+".value_kind"] = valueKind
	facts[prefix+".ref_mode.kind"] = mode
	if refKind != "" {
		facts[prefix+".ref_mode.ref_kind"] = refKind
	}
}

func runtimeEvaluatorStringMapsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func runtimeEvaluatorSetMapsEqual(
	left map[string]struct{},
	right map[string]struct{},
) bool {
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if _, exists := right[key]; !exists {
			return false
		}
	}
	return true
}

func runtimeEvaluatorContractMapsEqual(
	left map[RuntimeMechanismInvocationContract]string,
	right map[RuntimeMechanismInvocationContract]string,
) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
