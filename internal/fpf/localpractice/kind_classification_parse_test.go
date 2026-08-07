package localpractice

import (
	"strings"
	"testing"
)

func TestParseCurrentKindClassificationSignatureHasNoEntitySetOrMemberOfFields(
	t *testing.T,
) {
	source := string(readGoldenCarrier(t))
	legacy := `      - kind: entity_set_definition
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
	current := `      - kind: kind_classification_signature_definition
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
	updated := strings.Replace(source, legacy, current, 1)
	if updated == source {
		t.Fatal("fixture did not contain the sealed legacy declaration block")
	}
	parsed, err := Parse([]byte(updated))
	if err != nil {
		t.Fatalf("Parse(current KindSignature): %v", err)
	}
	declarations := parsed.Carrier().Signature().Vocabulary().Declarations()
	var signature KindClassificationSignatureDeclaration
	found := false
	for _, declaration := range declarations {
		candidate, ok := declaration.(KindClassificationSignatureDeclaration)
		if !ok {
			continue
		}
		signature = candidate
		found = true
	}
	if !found {
		t.Fatal("current KindClassificationSignature declaration is absent")
	}
	if signature.LocalKind().Value() != "Haft.ProjectConcern" ||
		signature.CandidateValueKind().Value() != "U.Entity" ||
		signature.CriterionRule().Value() != "haft.classify.project-record-carrier/v1" ||
		signature.SliceConditionsRule().Value() != "haft.context-slice.project/v1" {
		t.Fatalf("current signature coordinates = %#v", signature)
	}
	dependencies := signature.Dependencies()
	if len(dependencies) != 1 ||
		dependencies[0].Kind() != KindClassificationDependencyStandard {
		t.Fatalf("current signature dependencies = %#v", dependencies)
	}
	if signature.ReferenceScheme().Edition().Value() != "1.0.0" ||
		signature.ExtentRule().Kind() != KindClassificationNoExtentRule {
		t.Fatal("current signature lost ReferenceScheme or explicit ExtentRule posture")
	}
}

func TestParseCurrentKindClassificationSignatureFailsClosed(t *testing.T) {
	source := string(readGoldenCarrier(t))
	legacy := `      - kind: entity_set_definition
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
	current := `      - kind: kind_classification_signature_definition
        symbol: Haft.ProjectConcern.SignatureV2
        local_kind: Haft.ProjectConcern
        candidate_value_kind: U.Entity
        formality: F4
        criterion_rule: haft.classify.project-record-carrier/v1
        slice_conditions_rule: haft.context-slice.project/v1
        reference_scheme:
          carrier_ref: haft.reference-scheme.project-memory
          edition: 1.0.0
          digest: not-a-digest
        dependencies: []
        extent_rule:
          kind: none`
	updated := strings.Replace(source, legacy, current, 1)
	_, err := Parse([]byte(updated))
	if err == nil || !strings.Contains(err.Error(), "reference_scheme.digest") {
		t.Fatalf("malformed current signature error = %v", err)
	}
}
