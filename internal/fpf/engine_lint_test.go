package fpf

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLintFPFEngineTextAllowsTermSheetDeprecation(t *testing.T) {
	lints := LintFPFEngineText(".context/fpf-engine-term-sheet.md", []byte("PatternPull is deprecated as a formal term."))
	if len(lints) != 0 {
		t.Fatalf("lints = %#v, want none", lints)
	}
}

func TestLintFPFEngineTextRejectsFormalPatternPull(t *testing.T) {
	lints := LintFPFEngineText(".context/active-plan.md", []byte("PatternPull fallback remains the runtime subsystem."))
	assertEngineLintRule(t, lints, "deprecated_patternpull_formal_term")
}

func TestLintFPFEngineTextRejectsPatternAtlasAuthority(t *testing.T) {
	lints := LintFPFEngineText("docs/plan.md", []byte("PatternAtlas supports routes and should approve the match."))
	assertEngineLintRule(t, lints, "patternatlas_authority_overclaim")
}

func TestLintFPFEngineTextRejectsMethodPackAsSource(t *testing.T) {
	lints := LintFPFEngineText("docs/plan.md", []byte("MethodPack is DPF source for software engineering."))
	assertEngineLintRule(t, lints, "methodpack_source_authority_overclaim")
}

func TestLintFPFEngineTextRejectsWeakAffordance(t *testing.T) {
	lints := LintFPFEngineText("docs/matrix.yaml", []byte("routing_class: weak_affordance"))
	assertEngineLintRule(t, lints, "weak_affordance_deprecated")
}

func TestLintFPFEngineTextExcludesHistoricalPackets(t *testing.T) {
	lints := LintFPFEngineText(".context/external-review/old/packet.md", []byte("PatternPull fallback remains the runtime subsystem."))
	if len(lints) != 0 {
		t.Fatalf("historical packet lints = %#v, want none", lints)
	}
}

func TestCompactPatternUseOutputDoesNotExposeRoutingAffordanceCandidate(t *testing.T) {
	compact := RecommendPatternUseCompact("what time is it")
	data, err := json.Marshal(compact)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), RoutingClassAffordanceCandidate) {
		t.Fatalf("compact PatternUse output exposed %s:\n%s", RoutingClassAffordanceCandidate, data)
	}
}

func TestActiveFPFEngineCarriersPassTerminologyLint(t *testing.T) {
	root := testRepoRoot(t)
	relPaths := []string{
		"AGENTS.md",
		"CLAUDE.md",
		"CHANGELOG.md",
		"internal/cli/claude_md_template.md",
		"internal/cli/skill/h-reason/SKILL.md",
		"internal/cli/skill/h-status/SKILL.md",
		"internal/cli/skill/h-verify/SKILL.md",
		"packages/haft-pi/prompts/h-reason.md",
		"packages/haft-pi/skills/h-status/SKILL.md",
		".context/fpf-engine-term-sheet.md",
		".context/fpf-engine-coverage-matrix-c0.md",
	}

	lints, err := LintFPFEngineFiles(root, relPaths)
	if err != nil {
		t.Fatal(err)
	}
	if len(lints) != 0 {
		t.Fatalf("active carrier terminology lints: %#v", lints)
	}
}

func assertEngineLintRule(t *testing.T, lints []FPFEngineLint, ruleID string) {
	t.Helper()
	for _, lint := range lints {
		if lint.RuleID == ruleID {
			return
		}
	}
	t.Fatalf("lints = %#v, want rule %s", lints, ruleID)
}
