package cli

import (
	"strings"
	"testing"
)

func TestPublicSkillCatalogIsExactTwelve(t *testing.T) {
	want := []string{
		"h-reason",
		"h-decide",
		"h-frame",
		"h-diagnose",
		"h-explore",
		"h-compare",
		"h-verify",
		"h-spec",
		"h-status",
		"h-onboard",
		"h-note",
		"h-commission",
	}
	if len(allSkills) != len(want) {
		t.Fatalf("public skill count = %d, want %d", len(allSkills), len(want))
	}
	for index, skill := range allSkills {
		if skill.Name != want[index] {
			t.Fatalf("public skill[%d] = %q, want %q", index, skill.Name, want[index])
		}
	}
}

func TestRetiredDeterministicRoutingCheckIsNotPublic(t *testing.T) {
	for _, command := range checkCmd.Commands() {
		if command.Name() == "routing" {
			t.Fatal("retired deterministic `haft check routing` command remains public")
		}
	}
	for _, help := range []string{rootCmd.Long, checkCmd.Long} {
		if strings.Contains(help, "check routing") {
			t.Fatalf("active CLI help retains retired deterministic routing check: %q", help)
		}
	}
}

func TestRootHelpDistinguishesDroppedAgentTUIFromCurrentCLIPresentation(
	t *testing.T,
) {
	if strings.Contains(rootCmd.Long, "TUI surfaces were dropped") {
		t.Fatal("root help still drops current CLI presentation with the retired agent TUI")
	}
	for _, required := range []string{
		"coding-agent TUI",
		"haft board",
		"haft run",
		"remains supported",
	} {
		if !strings.Contains(rootCmd.Long, required) {
			t.Fatalf("root help is missing current presentation boundary %q", required)
		}
	}
}

func TestREADMEAdvertisesCurrentSkillOnlySurface(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	for _, want := range []string{"12 skills", "### Twelve skills installed by `haft init`"} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README missing current skill catalog marker %q", want)
		}
	}
	for _, retired := range []string{"16 skills", "Skills + slash commands", "haft fpf search"} {
		if strings.Contains(readme, retired) {
			t.Fatalf("README retains retired public surface %q", retired)
		}
	}
}

// TestHDecideSkillRoutesOnlyDirectOperatorRequests verifies that h-decide may
// route implicitly while the skill token itself remains non-authoritative.
func TestHDecideSkillRoutesOnlyDirectOperatorRequests(t *testing.T) {
	content := string(embeddedHDecideSkill)

	required := []string{
		`disable-model-invocation: false`,
		`host_routed_operator_request`,
		`invocation creates no communicative act`,
		`operator_confirmation_required`,
	}

	for _, want := range required {
		if !strings.Contains(content, want) {
			t.Fatalf("h-decide skill missing host-routed authority marker %q", want)
		}
	}
}

// TestHReasonSkill_IsSourceFirstUmbrella verifies that h-reason is the compact
// FPF entrypoint without rebuilding a shadow router or a universal work order.
func TestHReasonSkill_IsSourceFirstUmbrella(t *testing.T) {
	content := string(embeddedHReasonSkill)

	for _, skill := range []string{"h-frame", "h-diagnose", "h-explore", "h-compare", "h-decide", "h-verify", "h-spec"} {
		if !strings.Contains(content, skill) {
			t.Fatalf("h-reason must reference independent capability %q", skill)
		}
	}
	for _, want := range []string{
		`action="fpf"`,
		`mode="concern"`,
		`mode="lookup"`,
		`mode="inspect"`,
		"README practical-use cards",
		"Table of Contents",
		"The full pattern body governs",
		"retrieval rank != applicability",
		"Exact identifier namespaces",
		"wrong_identifier_namespace",
		`action="related"`,
		`artifact_ref="<id>"`,
		`action="memory"`,
		"memory_request",
		`"mode":"resolve"`,
		`mcp__haft__haft_onboard(action="status")`,
		`mcp__haft__haft_entity`,
		"`known_absent` says only",
		"operator-named or agent-inferred",
		"establish the minimum EntityOfConcern without asking for separate permission",
		"selected direct pattern by `PatternID`, title, and stable source reference",
		"source span, provenance, hashes, or repository-local paths only when the",
		"current use explicitly requires trace or audit",
		"Capabilities are independent entries, not phases",
		"caller abstention is the correct result: skip FPF",
		"not a fabricated\n`QueryResult(kind=\"abstained\")`; no query ran",
		"Do not automatically create ProblemCard",
		"Never claim that FPF is an acausal ontology",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("h-reason source-first contract missing %q", want)
		}
	}
	for _, banned := range []string{
		"should_use_pattern",
		"suggested_haft_surface",
		"recommended_pattern_use",
		"required_next_action",
		"matched_route_id",
		"selected direct pattern and source span",
		"ProjectTypeEnvHead",
		"TypeEnv",
		"haft memory typeenv",
	} {
		if strings.Contains(content, banned) {
			t.Fatalf("h-reason contains retired router field %q", banned)
		}
	}
}

func TestSubstantiveSkillsDoNotCarryShadowFPFRouter(t *testing.T) {
	skills := map[string][]byte{
		"h-frame":    embeddedHFrameSkill,
		"h-diagnose": embeddedHDiagnoseSkill,
		"h-explore":  embeddedHExploreSkill,
		"h-compare":  embeddedHCompareSkill,
		"h-verify":   embeddedHVerifySkill,
	}

	for name, contentBytes := range skills {
		t.Run(name, func(t *testing.T) {
			content := string(contentBytes)
			for _, banned := range []string{
				"should_use_pattern",
				"suggested_haft_surface",
				"recommended_pattern_use",
				"required_next_action",
				"matched_route_id",
				"Naming/terminology requests should route",
				"Architecture requests should route",
				"SoTA/current-practice requests should route",
			} {
				if strings.Contains(content, banned) {
					t.Fatalf("%s contains shadow-router fragment %q", name, banned)
				}
			}
		})
	}
}

func TestIndependentAutoSkillsCarryConditionalMemoryOrientation(t *testing.T) {
	reasoningSkills := map[string][]byte{
		"h-frame":    embeddedHFrameSkill,
		"h-diagnose": embeddedHDiagnoseSkill,
		"h-explore":  embeddedHExploreSkill,
		"h-compare":  embeddedHCompareSkill,
		"h-verify":   embeddedHVerifySkill,
		"h-spec":     embeddedHSpecSkill,
	}

	for name, contentBytes := range reasoningSkills {
		t.Run(name, func(t *testing.T) {
			content := string(contentBytes)
			for _, want := range []string{
				"Conditional project-memory orientation",
				"context-heavy",
				`action="memory"`,
				"memory_request",
				`"mode":"resolve"`,
				"agent_orientation.v2",
				"non-blocking",
				"code-graph preflight",
				"agent-inferred",
			} {
				if !strings.Contains(content, want) {
					t.Fatalf("%s memory-orientation contract missing %q", name, want)
				}
			}
		})
	}

	note := string(embeddedHNoteSkill)
	for _, want := range []string{
		"Conditional project-memory orientation",
		`action="memory"`,
		"memory_request",
		`"mode":"resolve"`,
		"agent_orientation.v2",
		`mcp__haft__haft_entity`,
		`mcp__haft__haft_onboard(action="status")`,
		"explicit save request",
		"non-blocking",
		"code-graph preflight",
	} {
		if !strings.Contains(note, want) {
			t.Fatalf("h-note memory-orientation contract missing %q", want)
		}
	}

	onboard := string(embeddedHOnboardSkill)
	for _, want := range []string{
		`mcp__haft__haft_onboard(action="status")`,
		`action="profile_prepare"`,
		"non-binding review carrier",
		"haft onboard profile apply",
		"installs default project memory",
		"ask the operator to enable, defer, select, or understand a memory schema",
	} {
		if !strings.Contains(onboard, want) {
			t.Fatalf("h-onboard task-level setup contract missing %q", want)
		}
	}

	for name, contentBytes := range map[string][]byte{
		"h-frame":    embeddedHFrameSkill,
		"h-diagnose": embeddedHDiagnoseSkill,
		"h-explore":  embeddedHExploreSkill,
		"h-compare":  embeddedHCompareSkill,
		"h-verify":   embeddedHVerifySkill,
		"h-spec":     embeddedHSpecSkill,
		"h-onboard":  embeddedHOnboardSkill,
		"h-note":     embeddedHNoteSkill,
	} {
		content := string(contentBytes)
		for _, forbidden := range []string{
			"ProjectTypeEnvHead",
			"TypeEnv",
			"haft memory typeenv",
			`haft_memory(action="admit")`,
		} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s exposes low-level memory UX %q", name, forbidden)
			}
		}
	}
}
