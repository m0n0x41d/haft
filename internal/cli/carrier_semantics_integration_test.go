package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFreshHostAndPiCarriersPreserveIndependentSourceFirstSemantics(t *testing.T) {
	projectRoot := physicalInitTestTempDir(t)
	homeRoot := physicalInitTestTempDir(t)
	t.Setenv("HOME", homeRoot)
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initAgents = true
	initLocal = true
	initPi = true
	cmd := newPublicInitTestCommand()
	var agents bool
	var local bool
	var selectedPi bool
	cmd.Flags().BoolVar(&agents, "agents", false, "")
	cmd.Flags().BoolVar(&local, "local", false, "")
	cmd.Flags().BoolVar(&selectedPi, "pi", false, "")
	for _, flag := range []string{"agents", "local", "pi"} {
		if err := cmd.Flags().Set(flag, "true"); err != nil {
			t.Fatalf("set %s flag: %v", flag, err)
		}
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("install fresh typed carriers: %v", err)
	}
	hostRoot := filepath.Join(projectRoot, ".agents", "skills")
	piRoot := filepath.Join(projectRoot, ".haft", "pi", "haft-pi")

	wantNames := []string{
		"h-commission",
		"h-compare",
		"h-decide",
		"h-diagnose",
		"h-explore",
		"h-frame",
		"h-note",
		"h-onboard",
		"h-reason",
		"h-spec",
		"h-status",
		"h-verify",
	}
	assertCarrierDirectoryNames(t, hostRoot, wantNames)
	piSkillRoot := filepath.Join(piRoot, "skills")
	assertCarrierDirectoryNames(t, piSkillRoot, wantNames)
	piPromptRoot := filepath.Join(piRoot, "prompts")
	assertCarrierPromptNames(t, piPromptRoot, wantNames)

	host := readMaterializedSkillSet(t, hostRoot, wantNames)
	pi := readMaterializedPiSet(t, piRoot, wantNames)

	assertCarrierFragments(t, "host h-reason", host["h-reason"], []string{
		`action="fpf"`,
		`mode="concern"`,
		`mode="lookup"`,
		`mode="inspect"`,
		"The result is source material, not applicability",
		"selected direct pattern by `PatternID`, title, and stable source reference",
		"current use explicitly requires trace or audit",
		"Capabilities are independent entries, not phases",
		"A.15.2",
		"U.WorkPlan",
		"There is no public `h-plan` phase",
		"attention signals; they are not project-wide stop conditions",
		"Never ask for bare `OK`, `yes`, or `да`",
	})
	assertCarrierFragments(t, "Pi h-reason", pi["h-reason"], []string{
		`"action": "fpf"`,
		`"mode": "concern"`,
		`"mode": "lookup"`,
		`"mode": "inspect"`,
		"not applicability",
		"In ordinary working use, identify the selected direct pattern by `PatternID`",
		"Request trace or audit provenance only when the current use requires it",
		"independent entries, not a project sequence",
		"A.15.2",
		"U.WorkPlan",
		"There is no public `h-plan`",
		"attention, not project-wide human gates",
		"Never ask for bare `OK`, `yes`, or `да`",
	})

	assertCarrierFragments(t, "host h-frame", host["h-frame"], []string{
		"Default to an inline conversational frame",
		"ProblemCard@Context",
		"operator explicitly asks to record or save",
		"named receiving use",
		"never prescribe `h-explore` as the automatic next stage",
	})
	assertCarrierFragments(t, "Pi h-frame", pi["h-frame"], []string{
		`action="fpf"`,
		`mode="concern"`,
		`mode="inspect"`,
		"conversational frame by default",
		"ProblemCard@Context",
		"operator asks to save",
		"named receiving use",
		"Do not prescribe `h-explore`",
	})

	assertCarrierFragments(t, "host h-explore", host["h-explore"], []string{
		"In ordinary use, return candidates in conversation and stop",
		"explicit save intent",
		"named reliance-bearing receiving use",
		"none is an automatic next step",
	})
	assertCarrierFragments(t, "Pi h-explore", pi["h-explore"], []string{
		"Return candidates conversationally by default",
		"explicit save intent",
		"named receiving use",
		"do not prescribe comparison next",
	})
	assertCarrierFragments(t, "host h-compare", host["h-compare"], []string{
		"Retrieve the current source before applying the comparison distillate",
		`action="fpf"`,
		`mode="concern"`,
		`mode="inspect"`,
		"not a selected governing pattern",
	})
	assertCarrierFragments(t, "Pi h-compare", pi["h-compare"], []string{
		"Retrieve current FPF source first",
		`action="fpf"`,
		`mode="concern"`,
		`mode="inspect"`,
		"not a selected governing pattern",
	})

	assertCarrierFragments(t, "host h-spec", host["h-spec"], []string{
		"exact current direct pattern body and source identity",
		"does not establish compatibility with a newer FPF source revision",
		"sealed legacy compatibility spelling",
		"Never repair source drift by blind token replacement",
		"source compatibility, implementation evidence, and SpecSection baseline currentness",
		"haft spec next --json",
		"project/scope-level",
		`action="spec_trace"`,
		`action="spec_use"`,
		"Never pass `section_id`",
	})
	assertCarrierFragments(t, "Pi h-spec", pi["h-spec"], []string{
		"exact current pattern body and source identity",
		"does not prove compatibility with a newer FPF",
		"sealed legacy",
		"MemberOf",
		"EntitySet",
		"baseline currentness separate",
		"haft spec next --json",
		"project/scope-level",
		`action="spec_trace"`,
		`action="spec_use"`,
		"Never pass `section_id`",
	})

	assertCarrierFragments(t, "host h-status", host["h-status"], []string{
		`action="coverage"`,
		"bounded exact `affected_files`",
		"does not prove that a file is undocumented",
		"unavailable rather than an empty-clean result",
		"Coverage is not a quality score",
		"Do not call refresh mutations",
		"Attention is not interruption",
		"do not block unrelated already-authorized Work",
		"canonical project profile has several scopes",
		`scope_id="<exact emitted ScopeID>"`,
		"retry the same status call",
		"Never select the first value",
	})
	assertCarrierFragments(t, "Pi h-status", pi["h-status"], []string{
		`"action": "coverage"`,
		"bounded exact `affected_files`",
		"not proof that a file is undocumented",
		"unavailable rather than empty-clean",
		"omitted detail is not evidence of absence",
		"Do not mutate artifacts",
		"attention, not project-wide human gates",
		"Never ask for bare approval",
		"several canonical project scopes",
		`"scope_id": "<exact emitted ScopeID>"`,
		"retry the same read-only",
		"Never select the first value",
	})

	assertManualCarrierBoundary(t, hostRoot, pi, "h-decide")
	assertManualCarrierBoundary(t, hostRoot, pi, "h-commission")
	assertCarrierFragments(t, "Pi h-decide", pi["h-decide"], []string{
		"haft artifact create decision.decide --input-file",
		"explicit_h_decide",
		"strict_cli_speech_act",
		"sole human gate",
		"fails closed",
	})
	assertCarrierFragments(t, "Pi h-commission", pi["h-commission"], []string{
		"haft commission create-from-decision",
		"Default MCP",
		"fails closed",
	})
	if strings.Contains(pi["h-status"], "haft_refresh") {
		t.Fatal("Pi h-status claims a read-only posture but still invokes mutating haft_refresh")
	}
	assertNoRetiredRoutingSurface(t, host, pi)
}

func assertCarrierDirectoryNames(t *testing.T, root string, want []string) {
	t.Helper()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read carrier directory %s: %v", root, err)
	}

	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			got = append(got, entry.Name())
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("carrier directories in %s = %v, want %v", root, got, want)
	}
}

func assertCarrierPromptNames(t *testing.T, root string, want []string) {
	t.Helper()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read prompt directory %s: %v", root, err)
	}

	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryName := entry.Name()
		extension := filepath.Ext(entryName)
		if entry.IsDir() || extension != ".md" {
			continue
		}
		name := strings.TrimSuffix(entryName, ".md")
		got = append(got, name)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("Pi prompt names in %s = %v, want %v", root, got, want)
	}
}

func readMaterializedSkillSet(t *testing.T, root string, names []string) map[string]string {
	t.Helper()

	content := make(map[string]string, len(names))
	for _, name := range names {
		path := filepath.Join(root, name, "SKILL.md")
		body := readMaterializedCarrier(t, path)
		content[name] = body
	}
	return content
}

func readMaterializedPiSet(t *testing.T, root string, names []string) map[string]string {
	t.Helper()

	content := make(map[string]string, len(names))
	for _, name := range names {
		skillPath := filepath.Join(root, "skills", name, "SKILL.md")
		promptPath := filepath.Join(root, "prompts", name+".md")
		skill := readMaterializedCarrier(t, skillPath)
		prompt := readMaterializedCarrier(t, promptPath)
		content[name] = skill + "\n" + prompt
	}
	return content
}

func readMaterializedCarrier(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read materialized carrier %s: %v", path, err)
	}
	return string(content)
}

func assertCarrierFragments(t *testing.T, label, content string, fragments []string) {
	t.Helper()

	normalizedContent := normalizeCarrierWhitespace(content)
	for _, fragment := range fragments {
		normalizedFragment := normalizeCarrierWhitespace(fragment)
		if !strings.Contains(normalizedContent, normalizedFragment) {
			t.Fatalf("%s missing semantic marker %q", label, fragment)
		}
	}
}

func normalizeCarrierWhitespace(content string) string {
	fields := strings.Fields(content)
	return strings.Join(fields, " ")
}

func assertManualCarrierBoundary(t *testing.T, hostRoot string, pi map[string]string, name string) {
	t.Helper()

	hostPath := filepath.Join(hostRoot, name, "SKILL.md")
	host := readMaterializedCarrier(t, hostPath)
	policyPath := filepath.Join(hostRoot, name, "agents", "openai.yaml")
	policy := readMaterializedCarrier(t, policyPath)
	assertCarrierFragments(t, "host "+name, host, []string{
		"MANUAL ONLY",
		"disable-model-invocation: true",
		"operator_confirmation_required",
		"binding actions require explicit operator/manual authorization",
	})
	assertCarrierFragments(t, "host policy "+name, policy, []string{
		"allow_implicit_invocation: false",
	})
	assertCarrierFragments(t, "Pi "+name, pi[name], []string{
		"Manual-only",
		"MANUAL GATE",
		"operator_confirmation_required",
		"binding actions require explicit operator/manual authorization",
	})
}

func assertNoRetiredRoutingSurface(t *testing.T, host, pi map[string]string) {
	t.Helper()

	retired := []string{
		"PatternUse Gateway",
		`action="pattern_use"`,
		`"action": "pattern_use"`,
		`action="pattern_recall"`,
		`"action": "pattern_recall"`,
		"Canonical FPF flow",
		"h-frame → h-explore",
		"h-frame -> h-explore",
		"h-spec-cover",
		"h-abduct",
		"h-boundary-unpack",
		"h-semio-review",
		"h-refresh",
	}
	assertCarrierSetExcludes(t, "host", host, retired)
	assertCarrierSetExcludes(t, "Pi", pi, retired)
}

func assertCarrierSetExcludes(t *testing.T, surface string, carriers map[string]string, retired []string) {
	t.Helper()

	names := make([]string, 0, len(carriers))
	for name := range carriers {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		content := carriers[name]
		for _, fragment := range retired {
			if strings.Contains(content, fragment) {
				t.Fatalf("fresh %s carrier %s exposes retired routing surface %q", surface, name, fragment)
			}
		}
	}
}
