package streamtruth

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var exactStreamTruthLabels = []string{
	"CURRENT PRODUCT",
	"V9 CONTRACT",
	"EXACT-CANDIDATE EVIDENCE",
	"DEFERRED RESEARCH",
}

var exactV9MCPToolNames = []string{
	"haft_note",
	"haft_problem",
	"haft_solution",
	"haft_decision",
	"haft_refresh",
	"haft_query",
	"haft_method",
	"haft_commission",
	"haft_spec_section",
	"haft_onboard",
	"haft_entity",
	"haft_memory",
}

func TestREADMEDeclaresExactlyTheFourStreamTruthLabels(t *testing.T) {
	readme := readTruthRepoFile(t, "README.md")
	section := exactMarkedSection(
		t,
		readme,
		"<!-- haft:truth-labels:start -->",
		"<!-- haft:truth-labels:end -->",
	)
	lines := strings.Split(section, "\n")
	actual := make([]string, 0, len(exactStreamTruthLabels))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- **") {
			continue
		}
		labelEnd := strings.Index(strings.TrimPrefix(trimmed, "- **"), "**")
		if labelEnd < 0 {
			t.Fatalf("malformed stream truth label line %q", line)
		}
		actual = append(actual, strings.TrimPrefix(trimmed, "- **")[:labelEnd])
	}
	if strings.Join(actual, "\x00") != strings.Join(exactStreamTruthLabels, "\x00") {
		t.Fatalf("stream truth labels = %#v, want exact ordered set %#v", actual, exactStreamTruthLabels)
	}

	normalizedReadme := normalizeTruthProse(readme)
	for _, requiredBoundary := range []string{
		"Contract inclusion specifies behavior; it is not delivery evidence",
		"installed-runtime readiness claims require current P14 evidence tied to one exact candidate",
		"Source, schema, skill, or local-test presence is not installed-runtime proof",
		"does not grant RC or release status",
		"additionally requires release authority",
		"No such superiority claim is made before the separate benchmark",
		"silently promotes a contract to CURRENT PRODUCT",
	} {
		if !strings.Contains(normalizedReadme, requiredBoundary) {
			t.Fatalf("README truth boundary missing %q", requiredBoundary)
		}
	}
}

func TestCurrentFacingV9ProseHasNoUnsupportedTruthClaim(t *testing.T) {
	carriers := currentFacingTruthCarriers(t)
	for name, content := range carriers {
		assertNoUnsupportedTruthClaim(t, name, content)
	}

	for _, carrier := range []string{
		"AGENTS.md",
		"CLAUDE.md",
		"internal/cli/claude_md_template.md",
	} {
		content := carriers[carrier]
		normalized := normalizeTruthProse(content)
		for _, marker := range exactStreamTruthLabels {
			if !strings.Contains(normalized, marker) {
				t.Errorf("%s omits current runtime truth marker %q", carrier, marker)
			}
		}
		for _, distinction := range []string{
			"`haft_onboard` is the normal setup surface",
			"`haft init` installs default project memory automatically",
			"`profile_prepare` may materialize or reuse only a non-binding review carrier",
			"require a direct, unambiguous operator choice before",
			"`haft onboard profile apply`",
			"`host_routed_operator_request`",
		} {
			if !strings.Contains(normalized, distinction) {
				t.Errorf("%s omits profile/DecisionRecord authority distinction %q", carrier, distinction)
			}
		}
	}

	requiredCarrierMarkers := map[string][]string{
		"internal/cli/skill/h-reason/SKILL.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
			"DEFERRED RESEARCH",
		},
		"internal/cli/skill/h-status/SKILL.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
		},
		"internal/cli/skill/h-onboard/SKILL.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
			"haft_onboard",
		},
		"data/haft/local-practice/typed-memory/README.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
		},
		"packages/haft-pi/README.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
			"DEFERRED RESEARCH",
			"experimental",
		},
		"packages/haft-pi/skills/h-reason/SKILL.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
			"DEFERRED RESEARCH",
			"experimental compatibility carrier",
		},
		"packages/haft-pi/prompts/h-reason.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
			"DEFERRED RESEARCH",
			"experimental compatibility carrier",
		},
		"packages/haft-pi/skills/h-status/SKILL.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
			"Pi support",
			"experimental",
		},
		"packages/haft-pi/prompts/h-status.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
			"Pi support",
			"experimental",
		},
		"packages/haft-pi/skills/h-onboard/SKILL.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
			"Pi support",
			"experimental",
			"native Pi `haft_onboard` tool",
			"`profile_prepare`",
			"`haft onboard profile apply`",
			"default project memory",
			"direct, unambiguous operator selection",
		},
		"packages/haft-pi/prompts/h-onboard.md": {
			"V9 CONTRACT",
			"EXACT-CANDIDATE EVIDENCE",
			"Pi support",
			"experimental",
			"native Pi `haft_onboard` tool",
			"`profile_prepare`",
			"`haft onboard profile apply`",
			"default project memory",
			"direct, unambiguous operator selection",
		},
	}
	for carrier, markers := range requiredCarrierMarkers {
		normalized := normalizeTruthProse(carriers[carrier])
		for _, marker := range markers {
			if !strings.Contains(normalized, marker) {
				t.Errorf("%s omits runtime truth marker %q", carrier, marker)
			}
		}
	}
	for _, carrier := range []string{
		"packages/haft-pi/skills/h-onboard/SKILL.md",
		"packages/haft-pi/prompts/h-onboard.md",
	} {
		normalized := normalizeTruthProse(carriers[carrier])
		for _, lowLevelRoute := range []string{
			"`haft profile inspect",
			"`haft profile propose",
			"`haft profile declare",
		} {
			if strings.Contains(normalized, lowLevelRoute) {
				t.Errorf(
					"%s exposes retired low-level onboarding route %q",
					carrier,
					lowLevelRoute,
				)
			}
		}
	}

	unreleased := carriers["CHANGELOG.md [Unreleased]"]
	normalizedUnreleased := normalizeTruthProse(unreleased)
	for _, marker := range []string{
		"V9 CONTRACT",
		"EXACT-CANDIDATE EVIDENCE",
		"release authority",
		"PatternRecall are no longer v9 product concepts",
		"outside the v9 contract and P14 acceptance basis",
	} {
		if !strings.Contains(normalizedUnreleased, marker) {
			t.Errorf("current changelog section omits %q", marker)
		}
	}
	for name, content := range carriers {
		normalized := normalizeTruthProse(content)
		for _, retired := range []string{"WORKING V9 DEV", "V9 DESIGN DRAFT"} {
			if strings.Contains(normalized, retired) {
				t.Errorf("%s retains retired current-facing truth label %q", name, retired)
			}
		}
	}
	for _, historical := range []string{
		"## [8.2.0]",
		"PatternRecall source-card recall surface",
		"default Claude config",
		"`h-method`",
		"`fpf-development`",
		"`fpf-semiotics`",
	} {
		if strings.Contains(unreleased, historical) {
			t.Errorf("truth audit leaked historical changelog content %q", historical)
		}
	}
}

func TestPublicMCPDocsNameExactV9DiscoverySurface(t *testing.T) {
	protocol := readTruthRepoFile(t, "spec/integration/MCP_PROTOCOL.md")
	readme := readTruthRepoFile(t, "README.md")

	protocolTools := markdownToolTableNames(
		headingSection(t, protocol, "## Tool Surface", "## Host Agents"),
	)
	readmeTools := markdownToolTableNames(
		headingSection(t, readme, "### Twelve MCP tools", "### Twelve skills"),
	)
	want := strings.Join(exactV9MCPToolNames, "\x00")
	for carrier, tools := range map[string][]string{
		"spec/integration/MCP_PROTOCOL.md": protocolTools,
		"README.md":                        readmeTools,
	} {
		if strings.Join(tools, "\x00") != want {
			t.Errorf(
				"%s MCP tools = %#v, want exact ordered discovery surface %#v",
				carrier,
				tools,
				exactV9MCPToolNames,
			)
		}
	}

	normalized := normalizeTruthProse(protocol)
	for _, boundary := range []string{
		"`tools/list` is atomic",
		"stay advertised before project-profile onboarding completes",
		"return a closed recovery result instead of disappearing",
		"not a blanket stability promise for every diagnostic, migration, or compatibility action",
		"`haft_memory` validate/admit is an expert diagnostic or implementation surface",
		"through a nested `request` envelope",
		"Users and ordinary agents do not choose, declare, or need to understand an internal memory schema",
		"Only Claude Code and Codex are stable-host acceptance targets",
	} {
		if !strings.Contains(normalized, boundary) {
			t.Errorf("MCP protocol omits contract boundary %q", boundary)
		}
	}
	for _, retired := range []string{
		"The 7 MCP tools",
		"v7 product support",
		"`--claude` (default)",
		"ProjectTypeEnvHead",
		"TypeEnv",
	} {
		if strings.Contains(protocol, retired) {
			t.Errorf("MCP protocol retains retired surface %q", retired)
		}
	}
}

func TestUnreleasedCompatibilityRecallStaysOutsideV9Acceptance(t *testing.T) {
	unreleased := unreleasedChangelogSection(
		t,
		readTruthRepoFile(t, "CHANGELOG.md"),
	)
	triggers := []string{
		"PatternRecall source-card",
		`action="pattern_recall"`,
		"PatternUseGateway",
		"route/intent vectors",
		"section vectors",
		"PatternUse direct-skill",
		"PatternUse FPF-wide",
		"PatternUse/MethodPack",
		"PatternUse gateway",
		"PatternUse audit",
		"CrossHybrid",
		"haft-embed",
		"embedding sidecar",
		"semantic-recall",
		"hybrid recall",
	}
	allowedBoundaries := []string{
		"no longer v9 product concepts",
		"outside the v9 contract and P14 acceptance basis",
		"outside v9 contract and P14 acceptance basis",
	}
	for _, block := range markdownBulletBlocks(unreleased) {
		if !containsAny(block, triggers) {
			continue
		}
		if containsAny(block, allowedBoundaries) {
			continue
		}
		t.Errorf(
			"Unreleased compatibility/retrieval bullet lacks v9/P14 boundary %q",
			block,
		)
	}
}

func TestAgentDisciplineTemplateKeepsOneTruthContract(t *testing.T) {
	carriers := currentFacingTruthCarriers(t)
	agents := markedHaftDiscipline(t, carriers["AGENTS.md"])
	claude := markedHaftDiscipline(t, carriers["CLAUDE.md"])
	template := strings.TrimSpace(carriers["internal/cli/claude_md_template.md"])

	if agents != claude {
		t.Error("AGENTS.md and CLAUDE.md install different Haft discipline sections")
	}
	if agents != template {
		t.Error("AGENTS.md and internal/cli/claude_md_template.md install different Haft discipline sections")
	}

	for _, requiredBoundary := range []string{
		"`haft_onboard` is the normal setup surface",
		"`haft init` installs default project memory automatically",
		"`profile_prepare` may materialize or reuse only a non-binding review carrier",
		"direct, unambiguous operator request",
		"without a confirmation round trip",
		"`host_routed_operator_request`",
		"skill token is not a receipt",
		"`/h-commission` remains manual-only",
	} {
		if !strings.Contains(normalizeTruthProse(agents), requiredBoundary) {
			t.Errorf("installed agent discipline omits authority boundary %q", requiredBoundary)
		}
	}
}

func TestMaintainerHostAndHistoricalModePointersStayCurrent(t *testing.T) {
	for _, carrier := range []string{"AGENTS.md", "CLAUDE.md"} {
		content := readTruthRepoFile(t, carrier)
		if !strings.Contains(
			content,
			"(Claude Code, Codex, OpenCode, Cursor)",
		) {
			t.Errorf("%s omits the exact maintainer host list", carrier)
		}
		if strings.Contains(content, "(Codex, Codex, OpenCode, Cursor)") {
			t.Errorf("%s retains duplicated Codex host entry", carrier)
		}
	}

	modeOntology := readTruthRepoFile(t, "spec/target-system/MODE_ONTOLOGY.md")
	if strings.Contains(modeOntology, "dec-20260713-9ed66ef0.md") {
		t.Error("historical mode carrier points active authority to superseded decision")
	}
}

func TestCurrentFacingCarriersRejectCanonicalPhaseFlow(t *testing.T) {
	carriers := currentFacingTruthCarriers(t)
	banned := []string{
		"## canonical fpf flow",
		"canonical fpf phase names",
		"canonical evolution loop",
		"/h-frame → /h-explore → /h-compare → /h-decide",
		"/h-frame -> /h-explore -> /h-compare -> /h-decide",
		"workflow skills: describe the problem, h-frame fires; explore + compare follow",
	}

	for name, content := range carriers {
		lower := strings.ToLower(content)
		for _, phrase := range banned {
			if strings.Contains(lower, phrase) {
				t.Errorf("%s presents a current canonical phase flow %q", name, phrase)
			}
		}
	}
}

func TestCurrentFacingCarriersMakeNoRetrievalSuperiorityClaim(t *testing.T) {
	carriers := currentFacingTruthCarriers(t)
	for name, content := range carriers {
		assertRetrievalClaimsRemainDeferred(t, name, content)
	}
}

func currentFacingTruthCarriers(t *testing.T) map[string]string {
	t.Helper()
	root := truthRepoRoot(t)
	paths := []string{
		"README.md",
		"AGENTS.md",
		"CLAUDE.md",
		"internal/cli/claude_md_template.md",
		"data/haft/local-practice/typed-memory/README.md",
		"packages/haft-pi/README.md",
		"open-sleigh/SPEC.md",
		"spec/integration/MCP_PROTOCOL.md",
		"docs/src/pages/docs/agent-prompt.astro",
		"docs/src/pages/docs/concepts.astro",
		"docs/src/pages/docs/commands.astro",
		"docs/src/pages/docs/first-steps.astro",
		"docs/src/pages/docs/migration.astro",
	}
	globPatterns := []string{
		"internal/cli/skill/*/SKILL.md",
		"packages/haft-pi/skills/*/SKILL.md",
		"packages/haft-pi/prompts/*.md",
	}
	for _, pattern := range globPatterns {
		matchedPaths, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			t.Fatalf("glob current-facing carriers for %s: %v", pattern, err)
		}
		paths = append(paths, matchedPaths...)
	}

	carriers := make(map[string]string, len(paths)+1)
	for _, candidate := range paths {
		path := candidate
		if filepath.IsAbs(candidate) {
			relative, relativeErr := filepath.Rel(root, candidate)
			if relativeErr != nil {
				t.Fatalf("relativize current-facing carrier %s: %v", candidate, relativeErr)
			}
			path = filepath.ToSlash(relative)
		}
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatalf("read current-facing carrier %s: %v", path, readErr)
		}
		carriers[path] = string(content)
	}
	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("read changelog: %v", err)
	}
	carriers["CHANGELOG.md [Unreleased]"] = unreleasedChangelogSection(t, string(changelog))
	return carriers
}

func markedHaftDiscipline(t *testing.T, content string) string {
	t.Helper()
	section := exactMarkedSection(
		t,
		content,
		"<!-- haft:start -->",
		"<!-- haft:end -->",
	)
	return strings.TrimSpace(section)
}

func assertRetrievalClaimsRemainDeferred(
	t *testing.T,
	carrier string,
	content string,
) {
	t.Helper()
	paragraphs := strings.Split(normalizeTruthNewlines(content), "\n\n")
	if carrier == "CHANGELOG.md [Unreleased]" {
		paragraphs = markdownBulletBlocks(content)
	}
	for _, paragraph := range paragraphs {
		lower := strings.ToLower(normalizeTruthProse(paragraph))
		if !containsAny(lower, []string{"dense", "hybrid", "superior", "outperform", "better than", " beats "}) {
			continue
		}
		if containsAny(lower, []string{
			"deferred",
			"no such superiority claim",
			"no superiority claim",
			"without its exact evidence",
			"does not claim superiority",
			"retained compatibility/migration",
			"outside the v9 contract and p14 acceptance basis",
			"outside v9 contract and p14 acceptance basis",
		}) {
			continue
		}
		t.Errorf("%s contains an undeferred retrieval-superiority paragraph %q", carrier, paragraph)
	}
}

func containsAny(content string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(content, candidate) {
			return true
		}
	}
	return false
}

func normalizeTruthNewlines(content string) string {
	unix := strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(unix, "\r", "\n")
}

func readTruthRepoFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(truthRepoRoot(t), path))
	if err != nil {
		t.Fatalf("read repository carrier %s: %v", path, err)
	}
	return string(content)
}

func truthRepoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate stream-truth acceptance source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func unreleasedChangelogSection(t *testing.T, changelog string) string {
	t.Helper()
	start := strings.Index(changelog, "## [Unreleased]")
	if start < 0 {
		t.Fatal("CHANGELOG.md has no Unreleased section")
	}
	remainder := changelog[start:]
	next := strings.Index(remainder[len("## [Unreleased]"):], "\n## [")
	if next < 0 {
		return remainder
	}
	return remainder[:len("## [Unreleased]")+next]
}

func exactMarkedSection(
	t *testing.T,
	content string,
	startMarker string,
	endMarker string,
) string {
	t.Helper()
	start := strings.Index(content, startMarker)
	if start < 0 {
		t.Fatalf("missing start marker %q", startMarker)
	}
	bodyStart := start + len(startMarker)
	end := strings.Index(content[bodyStart:], endMarker)
	if end < 0 {
		t.Fatalf("missing end marker %q", endMarker)
	}
	return content[bodyStart : bodyStart+end]
}

func headingSection(
	t *testing.T,
	content string,
	startHeading string,
	endHeading string,
) string {
	t.Helper()
	start := strings.Index(content, startHeading)
	if start < 0 {
		t.Fatalf("missing start heading %q", startHeading)
	}
	bodyStart := start + len(startHeading)
	end := strings.Index(content[bodyStart:], endHeading)
	if end < 0 {
		t.Fatalf("missing end heading %q", endHeading)
	}
	return content[bodyStart : bodyStart+end]
}

func markdownToolTableNames(section string) []string {
	var names []string
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| `haft_") {
			continue
		}
		fields := strings.Split(trimmed, "|")
		if len(fields) < 3 {
			continue
		}
		name := strings.Trim(strings.TrimSpace(fields[1]), "`")
		names = append(names, name)
	}
	return names
}

func markdownBulletBlocks(content string) []string {
	var blocks []string
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		blocks = append(blocks, normalizeTruthProse(strings.Join(current, "\n")))
		current = nil
	}
	for _, line := range strings.Split(normalizeTruthNewlines(content), "\n") {
		if strings.HasPrefix(line, "- ") {
			flush()
			current = append(current, line)
			continue
		}
		if len(current) > 0 && (strings.HasPrefix(line, "  ") || strings.TrimSpace(line) == "") {
			current = append(current, line)
			continue
		}
		flush()
	}
	flush()
	return blocks
}

func assertNoUnsupportedTruthClaim(
	t *testing.T,
	carrier string,
	content string,
) {
	t.Helper()
	lower := strings.ToLower(normalizeTruthProse(content))
	for _, unsupported := range []string{
		"haft is better than grep",
		"haft is better than `rg`",
		"fpf query is better than",
		"fpf query beats",
		"fpf query outperforms",
		"is a release candidate",
		"release-ready",
		"ready for release",
		"production-ready",
		"installed runtime is proven",
		"live runtime is working",
		"live mcp is working",
		"p14 is green",
	} {
		if strings.Contains(lower, unsupported) {
			t.Errorf("%s contains unsupported current-facing claim %q", carrier, unsupported)
		}
	}
	for _, line := range strings.Split(strings.ToLower(content), "\n") {
		if !strings.Contains(line, "acausal ontology") {
			continue
		}
		if strings.Contains(line, "not") ||
			strings.Contains(line, "never") ||
			strings.Contains(line, "do not") {
			continue
		}
		t.Errorf("%s contains unqualified acausal-ontology claim %q", carrier, strings.TrimSpace(line))
	}
}

func normalizeTruthProse(content string) string {
	return strings.Join(strings.Fields(content), " ")
}
