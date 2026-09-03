package cli

import (
	"strings"
	"testing"
)

func TestPublicSkillCatalogIsExactTwelve(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

func TestRootHelpDescribesGovernanceWithoutBuiltInExecution(
	t *testing.T,
) {
	t.Parallel()

	for _, required := range []string{
		"coding-agent TUI",
		"built-in commission executors",
		"external runners",
		"WorkCommissions",
	} {
		if !strings.Contains(rootCmd.Long, required) {
			t.Fatalf("root help is missing v9 execution boundary %q", required)
		}
	}
	for _, removed := range []string{"haft run", "haft harness", "Open-Sleigh"} {
		if strings.Contains(rootCmd.Long, removed) {
			t.Fatalf("root help retains removed execution surface %q", removed)
		}
	}
}

func TestRootCommandOmitsRemovedExecutors(t *testing.T) {
	t.Parallel()

	commands := map[string]bool{}
	for _, command := range rootCmd.Commands() {
		commands[command.Name()] = true
	}
	for _, removed := range []string{"run", "harness"} {
		if commands[removed] {
			t.Fatalf("removed command %q remains registered", removed)
		}
	}
	if !commands["commission"] {
		t.Fatal("runner-neutral commission command is not registered")
	}
}

func TestREADMEAdvertisesCurrentSkillOnlySurface(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// TestHReasonSkillAppliesCurrentPUAAndPURWithoutShadowRouting verifies the
// operational guardrails needed to use the current source patterns. It does
// not treat the skill carrier as a replacement FPF specification.
func TestHReasonSkillAppliesCurrentPUAAndPURWithoutShadowRouting(t *testing.T) {
	t.Parallel()

	content := string(embeddedHReasonSkill)
	required := []string{
		"exact PatternID, SourceID, or UnitID",
		"Solution, Consequences, ordinary boundary, nearest stronger neighbor",
		"inspect current `E.11.PUR` before recommending anything",
		"`problemFrame`, `forces`, `solutionConditions`, `ordinaryBoundary`, and",
		"`resultAndReceivingUse`",
		"`applicable`,\n   `inapplicable`, or `insufficientBasis`",
		"recommend only an `applicable` candidate",
		"`unordered`,\n`partialOrder`, or `totalOrder`",
		"`prerequisiteResult` additionally requires the exact",
		"use current `E.11.PUA`",
		"`newlyCurrentSubjectResult`",
		"`preExistingWithGrounding`",
		"`expectedSubjectResultAbsent`",
		"Choose the minimal current capability set",
		"`planning draft` or `planning cue`",
		"C.2.1/A.15.2 membership basis",
		"Human Gate Brief below is Haft-local governance UX",
		"A.6/A.6.B",
		"Haft-local in-scope policy, not permission supplied by FPF",
		"skill or pattern\n  carrier does not prove `U.MethodDescription` membership",
	}
	for _, want := range required {
		if !strings.Contains(content, want) {
			t.Fatalf("h-reason current PUA/PUR guardrails missing %q", want)
		}
	}

	aggregate := strings.Index(content, "combine those aspects into exactly one aggregate")
	recommend := strings.Index(content, "recommend only an `applicable` candidate")
	if aggregate < 0 || recommend < 0 || aggregate >= recommend {
		t.Fatalf("h-reason must aggregate candidate fit before recommendation: aggregate=%d recommend=%d", aggregate, recommend)
	}

	for _, banned := range []string{
		"Choose one current capability",
		"choose only the capability that is current",
		"measured 6 of 6 Russian concerns",
		"`U.WorkPlan`-shaped",
		"use the A.6 boundary discipline",
		"matched_route_id",
		"hidden winner",
	} {
		if strings.Contains(content, banned) {
			t.Fatalf("h-reason retains banned semantic shortcut %q", banned)
		}
	}
}

// TestHReasonSkillMapsCurrentPUAAndPURChecklists keeps one operational carrier
// clause for every current E.11.PUA/E.11.PUR conformance item. The clauses are
// guardrails to inspect and apply the source; they are not an inline schema.
func TestHReasonSkillMapsCurrentPUAAndPURChecklists(t *testing.T) {
	t.Parallel()

	content := string(embeddedHReasonSkill)
	checks := []struct {
		id       string
		fragment string
	}{
		{"PUA-1", "Name the working subject or relation and\npractical question in domain language before its PatternID"},
		{"PUA-2", "full Problem frame, Problem, Forces,\n   Solution, Consequences, ordinary boundary, nearest stronger neighbor"},
		{"PUA-3", "smallest honest subject result"},
		{"PUA-4", "Ordinary selected use remains conversational. Materialize a support record\nonly when it names the exact later reliance that consumes it"},
		{"PUA-5", "Close with exactly one honest disposition"},
		{"PUA-6", "A `U.Work` claim needs\none exact A.15.1-grounded occurrence"},
		{"PUA-7", "intent asserts no obtaining relation, a realized use names\nthe exact later object and basis, and a genuine stop has no receiver"},
		{"PUA-8", "opens a named return instead\nof silently reinterpreting the same use"},
		{"PUR-1", "For every candidate\nthat is actually evaluated"},
		{"PUR-2", "compact rationale for all five current fit aspects"},
		{"PUR-3", "combine those aspects into exactly one aggregate"},
		{"PUR-4", "recommend only an `applicable` candidate"},
		{"PUR-5", "same\nbounded coordination question while remaining a distinct candidate use"},
		{"PUR-6", "`unordered`,\n`partialOrder`, or `totalOrder`"},
		{"PUR-7", "`prerequisiteResult` additionally requires the exact PUA expectation and a\ncurrent closure showing that prerequisite result exists or obtains with its\ncategory-correct direct basis"},
		{"PUR-8", "Recommendation and\ncoordination likewise assert no plan, gate, decision, authorization"},
		{"PUR-9", "resolve one exact\n`C.22.PFR` Problem occurrence"},
		{"PUR-10", "creates no Move identity\nand performs no Work or Transformation"},
	}
	for _, check := range checks {
		if !strings.Contains(content, check.fragment) {
			t.Errorf("h-reason lacks operational clause for %s: %q", check.id, check.fragment)
		}
	}
}

func TestSubstantiveSkillsDoNotCarryShadowFPFRouter(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
