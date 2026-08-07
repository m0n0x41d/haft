package projecttypeenv

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestDeriveContextKindAvailabilityPlanUsesSourceGroundsWithoutAuthoredAvailability(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	source := carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil)
	for _, forbidden := range [][]byte{[]byte("kind_admission"), []byte("context_kind_availability")} {
		if bytes.Contains(source, forbidden) {
			t.Fatalf("Local-Practice fixture unexpectedly authors %q", forbidden)
		}
	}
	carrier := parseCarrier(t, source)
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
	artifact := compileAndSealExtension(t, bundle.Nodes()[0], nil)
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{artifact},
	))

	plan := acceptedContextKindAvailabilityPlanForTest(
		t,
		DeriveContextKindAvailabilityPlan(linked),
	)
	concern := contextKindAvailabilityInputForTest(
		t,
		plan,
		"haft-project",
		"Alpha.ProjectConcern",
	)
	grounds := concern.Grounds()
	if len(grounds) < 5 {
		t.Fatalf("Alpha.ProjectConcern grounds = %d; want all declaration/use grounds", len(grounds))
	}
	if !hasContextKindAvailabilityGroundKind(grounds, LocalKindDeclarationGroundKind) {
		t.Fatal("derived availability omitted exact local kind declaration ground")
	}
	if !hasContextKindAvailabilityGroundKind(grounds, DirectKindUseGroundKind) {
		t.Fatal("derived availability omitted exact direct-use grounds")
	}
	for _, ground := range grounds {
		if ground.ContextSource().Value() != "haft-project" {
			t.Fatalf("ground context = %q; want exact carrier context", ground.ContextSource().Value())
		}
		if ground.ApplicabilitySource().Value() != "haft-project" {
			t.Fatalf("ground Applicability = %q; want exact applicability context", ground.ApplicabilitySource().Value())
		}
		if ground.EvidenceSource().Span().Start() == 0 {
			t.Fatal("ground omitted exact source coordinate")
		}
	}
	contextKindAvailabilityInputForTest(t, plan, "haft-project", "U.Entity")
	contextKindAvailabilityInputForTest(t, plan, "haft-project", "U.ClaimGraph")

	if len(plan.CanonicalBytes()) == 0 {
		t.Fatal("derived availability plan has empty canonical bytes")
	}
	mutatedCanonical := plan.CanonicalBytes()
	mutatedCanonical[0] ^= 0xff
	if bytes.Equal(mutatedCanonical, plan.CanonicalBytes()) {
		t.Fatal("CanonicalBytes leaked mutable plan storage")
	}
	mutatedInputs := plan.Inputs()
	mutatedInputs[0] = ContextKindAvailabilityInput{}
	if plan.Inputs()[0].Context().String() == "" {
		t.Fatal("Inputs leaked mutable plan storage")
	}
}

func TestDeriveContextKindAvailabilityPlanCanonicalizesCallerPermutation(t *testing.T) {
	base := loadBaseArtifact(t)
	alphaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
		"alpha-project",
	))
	betaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
		"beta-project",
	))
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{betaCarrier, alphaCarrier},
	)
	nodes := nodesByCoordinate(bundle.Nodes())
	alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	beta := compileAndSealExtension(
		t,
		nodes["beta.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{alpha},
	)
	leftLinked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	))
	rightLinked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{alpha, beta},
	))

	left := acceptedContextKindAvailabilityPlanForTest(
		t,
		DeriveContextKindAvailabilityPlan(leftLinked),
	)
	right := acceptedContextKindAvailabilityPlanForTest(
		t,
		DeriveContextKindAvailabilityPlan(rightLinked),
	)
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("extension caller permutation changed availability plan bytes")
	}
}

func TestDeriveContextKindAvailabilityPlanAcceptsSameContextImportedKindUse(t *testing.T) {
	base := loadBaseArtifact(t)
	alphaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
		"shared-project",
	))
	betaSource := carrierFixtureInContext(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
		"shared-project",
	)
	betaSource = replaceCompositeRefKindValue(t, betaSource, "Alpha.ProjectConcern")
	betaSource = removeAvailabilityTestMembershipDeclarations(t, betaSource, "Beta")
	betaCarrier := parseCarrier(t, betaSource)
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{betaCarrier, alphaCarrier},
	)
	nodes := nodesByCoordinate(bundle.Nodes())
	alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	beta := compileAndSealExtension(
		t,
		nodes["beta.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{alpha},
	)
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	))

	plan := acceptedContextKindAvailabilityPlanForTest(
		t,
		DeriveContextKindAvailabilityPlan(linked),
	)
	input := contextKindAvailabilityInputForTest(
		t,
		plan,
		"shared-project",
		"Alpha.ProjectConcern",
	)
	foundImportedUse := false
	for _, ground := range input.Grounds() {
		direct, ok := ground.(DirectKindUseGround)
		if !ok {
			continue
		}
		provider, providerOK := direct.Provider().(ExtensionCompositeSymbolProvider)
		if providerOK &&
			provider.Ref() == alpha.Ref() &&
			direct.ConsumerRef() == beta.Ref() &&
			direct.DependencyScope() == CompositeDependencyImported {
			foundImportedUse = true
		}
	}
	if !foundImportedUse {
		t.Fatal("same-context imported exact kind use was not retained as a direct ground")
	}
}

func TestDeriveContextKindAvailabilityPlanRejectsCrossContextKindUseWithoutBridge(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	alpha, beta := compositeImportedDependencyFixture(
		t,
		base,
		"Alpha.ProjectConcern",
		false,
	)
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	))

	resolution := DeriveContextKindAvailabilityPlan(linked)
	if !resolution.Rejected() {
		t.Fatal("cross-context local kind use without KindBridge was accepted")
	}
	assertIssueListContains(t, resolution.Issues(), IssueContextKindBridgeMissing)
	for _, issue := range resolution.Issues() {
		if issue.Code() != IssueContextKindBridgeMissing {
			continue
		}
		if !strings.Contains(issue.Repair(), "exact KindBridge") {
			t.Fatalf("bridge-missing repair = %q; want actionable exact-bridge repair", issue.Repair())
		}
	}
}

func TestDeriveContextKindAvailabilityPlanUsesEveryExactExtensionBridgeWitness(
	t *testing.T,
) {
	bridgeSpecs := []availabilityBridgeSourceSpec{
		{
			symbol:        "Beta.Bridge.AlphaToBeta",
			sourceContext: "alpha-project",
			sourceKind:    "Alpha.ProjectConcern",
			targetContext: "beta-project",
			targetKind:    "Beta.ProjectConcern",
			direction:     "one_way",
		},
		{
			symbol:        "Beta.Bridge.BetaToAlphaTwoWay",
			sourceContext: "beta-project",
			sourceKind:    "Beta.ProjectConcern",
			targetContext: "alpha-project",
			targetKind:    "Alpha.ProjectConcern",
			direction:     "two_way",
		},
	}
	base, alpha, beta := contextKindAvailabilityBridgeArtifacts(
		t,
		bridgeSpecs,
		"Alpha.ProjectConcern",
	)
	linkResolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	)
	linked := acceptedCompositeIR(t, linkResolution)

	planResolution := DeriveContextKindAvailabilityPlan(linked)
	plan := acceptedContextKindAvailabilityPlanForTest(t, planResolution)
	input := contextKindAvailabilityInputForTest(
		t,
		plan,
		"beta-project",
		"Beta.ProjectConcern",
	)
	witnesses := make(map[string]typedmemory.ContextBridge)
	for _, ground := range input.Grounds() {
		bridged, ok := ground.(BridgedKindUseGround)
		if !ok {
			continue
		}
		bridge := bridged.Bridge()
		witnesses[bridge.ID().String()] = bridge
	}
	if len(witnesses) != 2 {
		t.Fatalf("extension bridge witnesses = %#v, want both exact directional bridges", witnesses)
	}
	forward := witnesses["Beta.Bridge.AlphaToBeta"]
	reverse := witnesses["Beta.Bridge.BetaToAlphaTwoWay"]
	if forward.Direction() != typedmemory.OneWayBridge ||
		reverse.Direction() != typedmemory.TwoWayBridge {
		t.Fatalf(
			"bridge directions = %q / %q",
			forward.Direction().String(),
			reverse.Direction().String(),
		)
	}
	if forward.Source().Context().String() != "alpha-project" ||
		forward.Target().Context().String() != "beta-project" ||
		forward.Mapping().SourceKind().String() != "Alpha.ProjectConcern" ||
		forward.Mapping().TargetKind().String() != "Beta.ProjectConcern" ||
		forward.Source().Edition().String() != "alpha-project-v1" ||
		forward.Target().Edition().String() != "beta-project-v1" ||
		forward.OrderCoverage() != typedmemory.NoOrderLinksCovered ||
		forward.KindCongruence().Value() != 2 ||
		len(forward.LossNotes().Values()) != 1 ||
		len(forward.DefinednessArea().Values()) != 1 {
		t.Fatal("extension bridge witness lost exact source contract")
	}
	provenance, ok := forward.Provenance().(typedmemory.ProjectSourceProvenance)
	if !ok {
		t.Fatalf("bridge provenance type = %T", forward.Provenance())
	}
	if provenance.Carrier().String() != "beta.signature" ||
		provenance.BaseTypeEnv() != linked.BaseTypeEnvRef() ||
		provenance.ManifestBasis().Manifest().ID() != "beta.signature" ||
		provenance.ManifestBasis().Direction() != typedmemory.ManifestProvide ||
		provenance.ManifestBasis().Symbol().Kind() != typedmemory.BridgeSymbol {
		t.Fatalf("extension bridge provenance = %#v", provenance)
	}
}

func TestDeriveContextKindAvailabilityPlanRejectsWrongDirectionExtensionBridge(
	t *testing.T,
) {
	bridgeSpecs := []availabilityBridgeSourceSpec{
		{
			symbol:        "Beta.Bridge.BetaToAlphaOneWay",
			sourceContext: "beta-project",
			sourceKind:    "Beta.ProjectConcern",
			targetContext: "alpha-project",
			targetKind:    "Alpha.ProjectConcern",
			direction:     "one_way",
		},
	}
	base, alpha, beta := contextKindAvailabilityBridgeArtifacts(
		t,
		bridgeSpecs,
		"Alpha.ProjectConcern",
	)
	linkResolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{alpha, beta},
	)
	linked := acceptedCompositeIR(t, linkResolution)

	resolution := DeriveContextKindAvailabilityPlan(linked)
	if !resolution.Rejected() {
		t.Fatal("reverse-only one-way extension bridge was accepted for alpha-to-beta use")
	}
	assertIssueListContains(t, resolution.Issues(), IssueContextKindBridgeMissing)
}

func TestExtensionKindBridgeGhostDependencyFailsBeforeAvailability(t *testing.T) {
	bridgeSpecs := []availabilityBridgeSourceSpec{
		{
			symbol:        "Beta.Bridge.GhostToBeta",
			sourceContext: "ghost-project",
			sourceKind:    "Ghost.ProjectConcern",
			targetContext: "beta-project",
			targetKind:    "Beta.ProjectConcern",
			direction:     "one_way",
		},
	}
	base, alpha, beta := contextKindAvailabilityBridgeArtifacts(t, bridgeSpecs, "")

	resolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	)
	if !resolution.Rejected() {
		t.Fatal("extension KindBridge with a ghost source kind was linked")
	}
	assertIssueListContains(t, resolution.Issues(), IssueGhostDependency)
}

func TestExtensionKindBridgeAvailabilityIsCallerOrderInvariant(t *testing.T) {
	bridgeSpecs := []availabilityBridgeSourceSpec{
		{
			symbol:        "Beta.Bridge.AlphaToBeta",
			sourceContext: "alpha-project",
			sourceKind:    "Alpha.ProjectConcern",
			targetContext: "beta-project",
			targetKind:    "Beta.ProjectConcern",
			direction:     "one_way",
		},
	}
	base, alpha, beta := contextKindAvailabilityBridgeArtifacts(
		t,
		bridgeSpecs,
		"Alpha.ProjectConcern",
	)
	leftLinkResolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	)
	leftLinked := acceptedCompositeIR(t, leftLinkResolution)
	rightLinkResolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{alpha, beta},
	)
	rightLinked := acceptedCompositeIR(t, rightLinkResolution)
	leftResolution := DeriveContextKindAvailabilityPlan(leftLinked)
	left := acceptedContextKindAvailabilityPlanForTest(t, leftResolution)
	rightResolution := DeriveContextKindAvailabilityPlan(rightLinked)
	right := acceptedContextKindAvailabilityPlanForTest(t, rightResolution)
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("extension caller permutation changed bridge-grounded availability bytes")
	}
}

func TestMatchingContextKindBridgesPreservesEveryExactDirectionalWitness(t *testing.T) {
	base := loadBaseArtifact(t)
	environment, err := typeEnvForAvailabilityBridgeTest(base)
	if err != nil {
		t.Fatalf("lower base fixture: %v", err)
	}
	definitions := environment.KindDefinitions()
	if len(definitions) == 0 {
		t.Fatal("base fixture has no declaration provenance")
	}
	provenance := definitions[0].Provenance()
	provider := mustAvailabilityContextRef(t, "alpha-project")
	consumer := mustAvailabilityContextRef(t, "beta-project")
	providerKind := mustAvailabilityKindID(t, "Alpha.ProjectConcern")
	consumerKind := mustAvailabilityKindID(t, "Beta.ProjectConcern")
	otherKind := mustAvailabilityKindID(t, "Alpha.OtherConcern")
	forward := mustAvailabilityBridge(
		t,
		"bridge-alpha-beta-forward",
		provider,
		consumer,
		providerKind,
		consumerKind,
		typedmemory.OneWayBridge,
		provenance,
	)
	reverseOneWay := mustAvailabilityBridge(
		t,
		"bridge-beta-alpha-one-way",
		consumer,
		provider,
		consumerKind,
		providerKind,
		typedmemory.OneWayBridge,
		provenance,
	)
	reverseTwoWay := mustAvailabilityBridge(
		t,
		"bridge-beta-alpha-two-way",
		consumer,
		provider,
		consumerKind,
		providerKind,
		typedmemory.TwoWayBridge,
		provenance,
	)
	wrongMapping := mustAvailabilityBridge(
		t,
		"bridge-alpha-beta-wrong-kind",
		provider,
		consumer,
		otherKind,
		otherKind,
		typedmemory.TwoWayBridge,
		provenance,
	)

	matches := matchingContextKindBridges(
		[]typedmemory.ContextBridge{reverseTwoWay, wrongMapping, reverseOneWay, forward},
		provider,
		consumer,
		providerKind,
	)
	if len(matches) != 2 {
		t.Fatalf("matching exact bridges = %d; want both exact directional witnesses", len(matches))
	}
	if matches[0].bridge.ID().String() != "bridge-alpha-beta-forward" ||
		matches[1].bridge.ID().String() != "bridge-beta-alpha-two-way" {
		t.Fatalf(
			"canonical matching bridges = %q, %q",
			matches[0].bridge.ID().String(),
			matches[1].bridge.ID().String(),
		)
	}
	for _, match := range matches {
		if match.consumerKind != consumerKind {
			t.Fatalf(
				"mapped consumer kind = %q; want %q",
				match.consumerKind.String(),
				consumerKind.String(),
			)
		}
	}
	retained := matches[0].bridge
	if retained.Source().Edition().String() != "availability-source-v1" ||
		retained.Target().Edition().String() != "availability-target-v1" ||
		retained.OrderCoverage() != typedmemory.NoOrderLinksCovered ||
		retained.KindCongruence().Value() != 2 ||
		len(retained.LossNotes().Values()) != 1 ||
		len(retained.DefinednessArea().Values()) != 1 {
		t.Fatal("matching bridge lost its full runtime C.3.3 contract")
	}
}

func TestDeriveContextKindAvailabilityPlanRejectsDriftedLinkedIR(t *testing.T) {
	base, artifact := compositeStandaloneFixture(t, "alpha.signature", "Alpha")
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{artifact},
	))
	linked.canonical = append([]byte(nil), linked.canonical...)
	linked.canonical[0] ^= 0xff

	resolution := DeriveContextKindAvailabilityPlan(linked)
	if !resolution.Rejected() {
		t.Fatal("drifted linked composite IR was accepted")
	}
	assertIssueListContains(t, resolution.Issues(), IssueContextKindAvailabilityInputInvalid)
}

func acceptedContextKindAvailabilityPlanForTest(
	t *testing.T,
	resolution ContextKindAvailabilityPlanResolution,
) ContextKindAvailabilityPlan {
	t.Helper()
	if resolution.Rejected() {
		t.Fatalf("DeriveContextKindAvailabilityPlan() rejected: %#v", resolution.Issues())
	}
	plan, exists := resolution.Plan()
	if !exists {
		t.Fatal("accepted availability resolution has no plan")
	}
	return plan
}

func contextKindAvailabilityInputForTest(
	t *testing.T,
	plan ContextKindAvailabilityPlan,
	context string,
	kindID string,
) ContextKindAvailabilityInput {
	t.Helper()
	for _, input := range plan.Inputs() {
		if input.Context().String() == context && input.KindID().String() == kindID {
			return input
		}
	}
	t.Fatalf("availability input %s/%s was not derived", context, kindID)
	return ContextKindAvailabilityInput{}
}

func hasContextKindAvailabilityGroundKind(
	grounds []ContextKindAvailabilityGround,
	want ContextKindAvailabilityGroundKind,
) bool {
	for _, ground := range grounds {
		if ground.GroundKind() == want {
			return true
		}
	}
	return false
}

func removeAvailabilityTestMembershipDeclarations(
	t *testing.T,
	source []byte,
	prefix string,
) []byte {
	t.Helper()
	result := string(source)
	fragments := []string{
		"    - " + prefix + ".ProjectEntities\n",
		"    - " + prefix + ".ProjectConcern.Signature\n",
		"      - kind: entity_set_definition\n" +
			"        symbol: " + prefix + ".ProjectEntities\n" +
			"        enumeration_rule: haft.rule.project-entities/v1\n" +
			"        candidate_policy:\n" +
			"          kind: persisted_entities_only\n",
		"      - kind: kind_signature_definition\n" +
			"        symbol: " + prefix + ".ProjectConcern.Signature\n" +
			"        value_kind: " + prefix + ".ProjectConcern\n" +
			"        formality: F3\n" +
			"        assumptions: []\n" +
			"        definedness_rule: haft.rule.project-concern-defined/v1\n" +
			"        evaluator_rule: haft.rule.project-concern-member/v1\n" +
			"        membership_basis: {kind: carrier_first, adapter_rule: haft.member-of.project-record-carrier/v1}\n" +
			"        entity_set: " + prefix + ".ProjectEntities\n",
	}
	for _, fragment := range fragments {
		if !strings.Contains(result, fragment) {
			t.Fatalf("membership fixture fragment was not found:\n%s", fragment)
		}
		result = strings.Replace(result, fragment, "", 1)
	}
	return []byte(result)
}

type availabilityBridgeSourceSpec struct {
	symbol        string
	sourceContext string
	sourceKind    string
	targetContext string
	targetKind    string
	direction     string
}

func contextKindAvailabilityBridgeArtifacts(
	t *testing.T,
	bridges []availabilityBridgeSourceSpec,
	crossContextRefKind string,
) (
	typeenv.BaseTypeEnvArtifact,
	ProjectTypeEnvExtensionArtifact,
	ProjectTypeEnvExtensionArtifact,
) {
	t.Helper()
	base := loadBaseArtifact(t)
	alphaSource := carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
		"alpha-project",
	)
	alphaCarrier := parseCarrier(t, alphaSource)
	betaSource := carrierFixtureInContext(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
		"beta-project",
	)
	if crossContextRefKind != "" {
		betaSource = replaceCompositeRefKindValue(t, betaSource, crossContextRefKind)
	}
	betaSource = addAvailabilityBridgeDeclarations(t, betaSource, bridges)
	betaCarrier := parseCarrier(t, betaSource)
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{betaCarrier, alphaCarrier},
	)
	nodes := nodesByCoordinate(bundle.Nodes())
	alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	beta := compileAndSealExtension(
		t,
		nodes["beta.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{alpha},
	)
	return base, alpha, beta
}

func addAvailabilityBridgeDeclarations(
	t *testing.T,
	source []byte,
	bridges []availabilityBridgeSourceSpec,
) []byte {
	t.Helper()
	providesAnchor := []byte("    - Beta.Constraint.ConcernOrEvidence\n")
	if !bytes.Contains(source, providesAnchor) {
		t.Fatal("availability bridge fixture has no provides anchor")
	}
	provides := make([]string, 0, len(bridges))
	declarations := make([]string, 0, len(bridges))
	for _, bridge := range bridges {
		provides = append(provides, "    - "+bridge.symbol+"\n")
		declarations = append(declarations, availabilityBridgeDeclarationBlock(bridge))
	}
	providesText := strings.Join(provides, "")
	providesReplacement := append([]byte(nil), providesAnchor...)
	providesReplacement = append(providesReplacement, []byte(providesText)...)
	result := bytes.Replace(
		source,
		providesAnchor,
		providesReplacement,
		1,
	)
	declarationAnchor := []byte("  laws:\n")
	if !bytes.Contains(result, declarationAnchor) {
		t.Fatal("availability bridge fixture has no declaration anchor")
	}
	declarationsText := strings.Join(declarations, "")
	declarationsReplacement := []byte(declarationsText)
	declarationsReplacement = append(declarationsReplacement, declarationAnchor...)
	return bytes.Replace(
		result,
		declarationAnchor,
		declarationsReplacement,
		1,
	)
}

func availabilityBridgeDeclarationBlock(bridge availabilityBridgeSourceSpec) string {
	return fmt.Sprintf(
		"      - kind: kind_bridge\n"+
			"        symbol: %s\n"+
			"        endpoints:\n"+
			"          source:\n"+
			"            bounded_context_ref: %s\n"+
			"            edition: %s-v1\n"+
			"          target:\n"+
			"            bounded_context_ref: %s\n"+
			"            edition: %s-v1\n"+
			"        mapping:\n"+
			"          kind: named_target\n"+
			"          source_kind: %s\n"+
			"          target_kind: %s\n"+
			"        direction: %s\n"+
			"        order_preservation: no_links_covered\n"+
			"        kind_congruence: 2\n"+
			"        loss_notes:\n"+
			"          - Exact source representation is not preserved.\n"+
			"        definedness_area:\n"+
			"          - Both exact context editions remain active.\n",
		bridge.symbol,
		bridge.sourceContext,
		bridge.sourceContext,
		bridge.targetContext,
		bridge.targetContext,
		bridge.sourceKind,
		bridge.targetKind,
		bridge.direction,
	)
}

func typeEnvForAvailabilityBridgeTest(
	base typeenv.BaseTypeEnvArtifact,
) (typedmemory.TypeEnv, error) {
	return typeenv.LowerBaseTypeEnvArtifact(base)
}

func mustAvailabilityContextRef(
	t *testing.T,
	raw string,
) typedmemory.BoundedContextRef {
	t.Helper()
	value, err := typedmemory.NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("NewBoundedContextRef(%q): %v", raw, err)
	}
	return value
}

func mustAvailabilityKindID(t *testing.T, raw string) typedmemory.KindID {
	t.Helper()
	value, err := typedmemory.NewKindID(raw)
	if err != nil {
		t.Fatalf("NewKindID(%q): %v", raw, err)
	}
	return value
}

func mustAvailabilityBridge(
	t *testing.T,
	id string,
	sourceContext typedmemory.BoundedContextRef,
	targetContext typedmemory.BoundedContextRef,
	sourceKind typedmemory.KindID,
	targetKind typedmemory.KindID,
	direction typedmemory.BridgeDirection,
	provenance typedmemory.DeclarationProvenance,
) typedmemory.ContextBridge {
	t.Helper()
	bridgeID, err := typedmemory.NewContextBridgeID(id)
	if err != nil {
		t.Fatalf("NewContextBridgeID(%q): %v", id, err)
	}
	sourceEdition, err := typedmemory.NewContextEdition("availability-source-v1")
	if err != nil {
		t.Fatalf("NewContextEdition(source): %v", err)
	}
	targetEdition, err := typedmemory.NewContextEdition("availability-target-v1")
	if err != nil {
		t.Fatalf("NewContextEdition(target): %v", err)
	}
	source, err := typedmemory.NewContextBridgeEndpoint(sourceContext, sourceEdition)
	if err != nil {
		t.Fatalf("NewContextBridgeEndpoint(source): %v", err)
	}
	target, err := typedmemory.NewContextBridgeEndpoint(targetContext, targetEdition)
	if err != nil {
		t.Fatalf("NewContextBridgeEndpoint(target): %v", err)
	}
	mapping, err := typedmemory.NewNamedTargetKindMapping(sourceKind, targetKind)
	if err != nil {
		t.Fatalf("NewNamedTargetKindMapping(): %v", err)
	}
	congruence, err := typedmemory.NewKindCongruenceLevel(2)
	if err != nil {
		t.Fatalf("NewKindCongruenceLevel(): %v", err)
	}
	lossNotes, err := typedmemory.NewKindBridgeLossNotes([]string{
		"No source SubkindOf links are covered by this availability bridge.",
	})
	if err != nil {
		t.Fatalf("NewKindBridgeLossNotes(): %v", err)
	}
	definedness, err := typedmemory.NewKindBridgeDefinednessArea([]string{
		"The exact availability fixture context editions are active.",
	})
	if err != nil {
		t.Fatalf("NewKindBridgeDefinednessArea(): %v", err)
	}
	bridge, err := typedmemory.NewContextBridge(typedmemory.ContextBridgeInput{
		ID:              bridgeID,
		Source:          source,
		Target:          target,
		Mapping:         mapping,
		Direction:       direction,
		OrderCoverage:   typedmemory.NoOrderLinksCovered,
		KindCongruence:  congruence,
		LossNotes:       lossNotes,
		DefinednessArea: definedness,
		Provenance:      provenance,
	})
	if err != nil {
		t.Fatalf("NewContextBridge(%q): %v", id, err)
	}
	return bridge
}
