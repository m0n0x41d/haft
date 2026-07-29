package projecttypeenv

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const legacyProjectKindClassificationSource = `      - kind: entity_set_definition
        symbol: Haft.ProjectEntities
        enumeration_rule: haft.rule.project-entities/v1
        candidate_policy:
          kind: persisted_entities_only
      - kind: kind_signature_definition
        symbol: Haft.ProjectConcern.Signature
        value_kind: Haft.ProjectConcern
        formality: F3
        assumptions: []
        definedness_rule: haft.rule.project-concern-defined/v1
        evaluator_rule: haft.rule.project-concern-member/v1
        membership_basis: {kind: carrier_first, adapter_rule: haft.member-of.project-record-carrier/v1}
        entity_set: Haft.ProjectEntities`

const currentProjectKindClassificationSource = `      - kind: kind_classification_signature_definition
        symbol: Haft.ProjectConcern.SignatureV2
        local_kind: Haft.ProjectConcern
        candidate_value_kind: U.Entity
        formality: F4
        criterion_rule: haft.classify.project-record-carrier/v1
        slice_conditions_rule: haft.context-slice.project/v1
        reference_scheme:
          carrier_ref: haft.reference-scheme.project-memory
          edition: 1.0.0
          digest: sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
        dependencies:
          - kind: standard
            carrier_ref: haft.standard.project-record-carrier
            edition: 1.0.0
            digest: sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
        extent_rule:
          kind: none`

func TestCurrentKindClassificationCompilesLinksLowersAndDiscoversOnlyCriterionRuntime(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	source := string(carrierFixture(
		t,
		base,
		"current-kind-classification.signature",
		"1.4.0",
		"Haft",
		nil,
	))
	source = strings.Replace(
		source,
		legacyProjectKindClassificationSource,
		currentProjectKindClassificationSource,
		1,
	)
	source = strings.Replace(
		source,
		SupportedCompilerVersion,
		CurrentCompilerVersion,
		1,
	)
	source = strings.Replace(
		source,
		"    - Haft.ProjectEntities\n    - Haft.ProjectConcern.Signature",
		"    - Haft.ProjectConcern.SignatureV2",
		1,
	)
	if !strings.Contains(source, currentProjectKindClassificationSource) {
		t.Fatal("fixture did not receive the current classification declaration")
	}
	carrier := parseCarrier(t, []byte(source))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
	extension := compileAndSealExtension(t, bundle.Nodes()[0], nil)
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{extension},
	))

	assumptions := linked.CarrierAssumptions()
	if len(assumptions) != 2 {
		t.Fatalf("current signature carrier pins = %d, want ReferenceScheme plus dependency", len(assumptions))
	}
	if !hasCompositeExternalKind(linked.ExternalReferences(), CompositeExternalRule) ||
		!hasCompositeExternalKind(linked.ExternalReferences(), CompositeExternalCarrier) {
		t.Fatal("current signature did not preserve rule and exact-carrier dependencies")
	}

	target, exists := base.TypeEnvRef()
	if !exists {
		t.Fatal("base fixture has no executable TypeEnv reference")
	}
	sources := canonicalCompositeSourceDeclarations(linked)
	provenance := func(
		source compositeSourceDeclaration,
		semantic string,
	) (typedmemory.ProjectSourceProvenance, error) {
		return compositeDeclarationProvenance(linked, source, semantic)
	}
	lowered, err := lowerCompositeKindClassificationSignatureDefinitions(
		target,
		sources,
		provenance,
	)
	if err != nil {
		t.Fatalf("lower current KindSignature: %v", err)
	}
	if len(lowered) != 1 {
		t.Fatalf("lowered current KindSignatures = %d, want 1", len(lowered))
	}
	definition := lowered[0]
	if definition.LocalKind().ValueKind().ID().String() != "Haft.ProjectConcern" ||
		definition.CandidateValueKind().ID().String() != "U.Entity" ||
		definition.Criterion().String() != "haft.classify.project-record-carrier/v1" ||
		definition.SliceConditions().String() != "haft.context-slice.project/v1" {
		t.Fatalf("lowered current KindSignature = %#v", definition)
	}
	if len(definition.Dependencies()) != 1 ||
		definition.ExtentRule().CanonicalBytes() == nil {
		t.Fatal("lowered current KindSignature lost dependencies or ExtentRule posture")
	}

	discovery := DiscoverProjectTypeEnvCompositeRuntimeRequirements(base, linked)
	if discovery.Rejected() {
		t.Fatalf("runtime requirement discovery rejected: %#v", discovery.Issues())
	}
	required, exists := discovery.RequiredSet()
	if !exists {
		t.Fatal("accepted runtime requirement discovery has no set")
	}
	classificationCount := 0
	for _, requirement := range required.Requirements() {
		if requirement.SemanticReference() == "haft.classify.project-record-carrier/v1" &&
			requirement.InvocationContract() == RuntimeMechanismContractKindClassification {
			classificationCount++
		}
		if requirement.SemanticReference() == compositeLowererEvaluatorRule ||
			requirement.SemanticReference() == compositeLowererEnumerationRule ||
			requirement.SemanticReference() == compositeLowererMembershipRule {
			t.Fatalf("current source leaked historical runtime requirement %#v", requirement)
		}
	}
	if classificationCount != 1 {
		t.Fatalf("kind-classification runtime requirements = %d, want 1", classificationCount)
	}
}
