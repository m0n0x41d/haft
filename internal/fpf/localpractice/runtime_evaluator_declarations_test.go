package localpractice

import (
	"os"
	"strings"
	"testing"
)

const runtimeEvaluatorDeclarationAnchor = "      - kind: value_kind\n        symbol: Haft.ProjectConcern\n"

func TestRuntimeEvaluatorRequirementDeclarationPreservesExactCoordinate(t *testing.T) {
	declaration := `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleV1
        rule_ref: haft.example/v1/evaluate
        invocation_contract: example_evaluation
`
	parsed, err := parseCarrierWithRuntimeDeclarations(t, declaration)
	if err != nil {
		t.Fatalf("Parse(runtime evaluator requirement): %v", err)
	}
	requirement, exists := runtimeRequirementBySymbol(parsed, "Haft.RuntimeRequirement.ExampleV1")
	if !exists {
		t.Fatal("parsed carrier is missing runtime evaluator requirement")
	}
	if got := requirement.RuleRef().Value(); got != "haft.example/v1/evaluate" {
		t.Fatalf("RuleRef = %q", got)
	}
	if got := requirement.InvocationContract().Value(); got != "example_evaluation" {
		t.Fatalf("invocation contract = %q", got)
	}
	if requirement.Span().Start() == 0 || requirement.Span().End() < requirement.Span().Start() {
		t.Fatalf("requirement source span = %#v", requirement.Span())
	}
}

func TestRuntimeEvaluatorRequirementDeclarationRejectsClosedGrammarViolations(t *testing.T) {
	testCases := []struct {
		name        string
		declaration string
		want        string
	}{
		{
			name: "missing RuleRef",
			declaration: `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleV1
        invocation_contract: example_evaluation
`,
			want: `is missing required field "rule_ref"`,
		},
		{
			name: "missing invocation contract",
			declaration: `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleV1
        rule_ref: haft.example/v1/evaluate
`,
			want: `is missing required field "invocation_contract"`,
		},
		{
			name: "non-string RuleRef",
			declaration: `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleV1
        rule_ref: 42
        invocation_contract: example_evaluation
`,
			want: `.rule_ref must be a string`,
		},
		{
			name: "non-string invocation contract",
			declaration: `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleV1
        rule_ref: haft.example/v1/evaluate
        invocation_contract: [example_evaluation]
`,
			want: `.invocation_contract must be a string`,
		},
		{
			name: "implementation is not source grammar",
			declaration: `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleV1
        rule_ref: haft.example/v1/evaluate
        invocation_contract: example_evaluation
        implementation: example.Call
`,
			want: `contains unknown field "implementation"`,
		},
		{
			name: "artifact pin is not source grammar",
			declaration: `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleV1
        rule_ref: haft.example/v1/evaluate
        invocation_contract: example_evaluation
        artifact_ref: artifact:example
`,
			want: `contains unknown field "artifact_ref"`,
		},
		{
			name: "role is fixed by declaration kind",
			declaration: `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleV1
        rule_ref: haft.example/v1/evaluate
        invocation_contract: example_evaluation
        role: evaluator
`,
			want: `contains unknown field "role"`,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := parseCarrierWithRuntimeDeclarations(t, testCase.declaration)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Parse() error = %v, want containing %q", err, testCase.want)
			}
		})
	}
}

func TestRuntimeEvaluatorRequirementDeclarationRejectsDuplicateCoordinate(t *testing.T) {
	declarations := `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleA
        rule_ref: haft.example/v1/evaluate
        invocation_contract: example_evaluation
      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleB
        rule_ref: haft.example/v1/evaluate
        invocation_contract: example_evaluation
`
	_, err := parseCarrierWithRuntimeDeclarations(t, declarations)
	want := `duplicate runtime evaluator requirement contract "example_evaluation" for RuleRef "haft.example/v1/evaluate"`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Parse() error = %v, want containing %q", err, want)
	}
}

func TestRuntimeEvaluatorRequirementDeclarationAllowsOneRuleAcrossContracts(t *testing.T) {
	declarations := `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleA
        rule_ref: haft.example/v1/evaluate
        invocation_contract: example_evaluation_a
      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleB
        rule_ref: haft.example/v1/evaluate
        invocation_contract: example_evaluation_b
`
	if _, err := parseCarrierWithRuntimeDeclarations(t, declarations); err != nil {
		t.Fatalf("Parse(distinct contracts for one RuleRef): %v", err)
	}
}

func TestRuntimeEvaluatorInputDeclarationUsesNonRelationalClosedGrammar(t *testing.T) {
	declarations := `      - kind: runtime_evaluator_requirement
        symbol: Haft.RuntimeRequirement.ExampleV1
        rule_ref: haft.example/v1/evaluate
        invocation_contract: example_evaluation
      - kind: runtime_evaluator_input
        symbol: Haft.ExampleEvaluationInput
        evaluator_requirement: Haft.RuntimeRequirement.ExampleV1
        slots:
          - slot_kind: Haft.ExampleEvaluationInput.ClaimGraphSlot
            value_kind: U.ClaimGraph
            ref_mode:
              kind: by_value
`
	parsed, err := parseCarrierWithRuntimeDeclarations(t, declarations)
	if err != nil {
		t.Fatalf("Parse(runtime evaluator input): %v", err)
	}
	for _, declaration := range parsed.Carrier().Signature().Vocabulary().Declarations() {
		if declaration.Symbol().Value() != "Haft.ExampleEvaluationInput" {
			continue
		}
		input, ok := declaration.(RuntimeEvaluatorInputDeclaration)
		if !ok {
			t.Fatalf("input declaration = %T, want RuntimeEvaluatorInputDeclaration", declaration)
		}
		if got := input.EvaluatorRequirement().Value(); got != "Haft.RuntimeRequirement.ExampleV1" {
			t.Fatalf("evaluator requirement = %q", got)
		}
		if len(input.Slots()) != 1 {
			t.Fatalf("input slots = %d, want 1", len(input.Slots()))
		}
		return
	}
	t.Fatal("parsed carrier is missing runtime evaluator input")
}

func parseCarrierWithRuntimeDeclarations(
	t *testing.T,
	declarations string,
) (ParsedCarrier, error) {
	t.Helper()
	source, err := os.ReadFile("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("read valid carrier fixture: %v", err)
	}
	text := string(source)
	if strings.Count(text, runtimeEvaluatorDeclarationAnchor) != 1 {
		t.Fatal("valid carrier fixture declaration anchor is not unique")
	}
	mutated := strings.Replace(text, runtimeEvaluatorDeclarationAnchor, declarations+runtimeEvaluatorDeclarationAnchor, 1)
	return Parse([]byte(mutated))
}

func runtimeRequirementBySymbol(
	parsed ParsedCarrier,
	symbol string,
) (RuntimeEvaluatorRequirementDeclaration, bool) {
	for _, declaration := range parsed.Carrier().Signature().Vocabulary().Declarations() {
		if declaration.Symbol().Value() != symbol {
			continue
		}
		requirement, ok := declaration.(RuntimeEvaluatorRequirementDeclaration)
		return requirement, ok
	}
	return RuntimeEvaluatorRequirementDeclaration{}, false
}
