package cli

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestCurrentStandardSkillProjectionsDeriveFromOneExactBundle(t *testing.T) {
	t.Parallel()

	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("build current skill bundle: %v", err)
	}
	adapters, err := buildCurrentSkillAdapterRegistry()
	if err != nil {
		t.Fatalf("build current skill adapters: %v", err)
	}
	roots := make(map[initplanning.HostID]string, len(adapters))
	baseRoot := t.TempDir()
	for _, adapter := range adapters {
		roots[adapter.definition.host] = filepath.Join(baseRoot, string(adapter.definition.host))
	}
	projections := make(
		[]initplanning.SkillComponentProjection,
		0,
		len(adapters),
	)
	for _, adapter := range adapters {
		projection, err := adapter.renderer.Render(
			bundle,
			roots[adapter.definition.host],
		)
		if err != nil {
			t.Fatalf(
				"render %s skill projection: %v",
				adapter.definition.host,
				err,
			)
		}
		projections = append(projections, projection)
	}
	report, err := initplanning.VerifySkillDerivationParity(bundle, projections)
	if err != nil {
		t.Fatalf("verify current standard projection parity: %v", err)
	}
	wantHosts := []initplanning.HostID{
		initplanning.HostAir,
		initplanning.HostAntigravity,
		initplanning.HostClaude,
		initplanning.HostCodex,
		initplanning.HostCursor,
		initplanning.HostGrok,
		initplanning.HostHermes,
		initplanning.HostOpenCode,
	}
	if !slices.Equal(report.Hosts, wantHosts) {
		t.Fatalf("standard skill hosts = %v, want %v", report.Hosts, wantHosts)
	}
	if report.SkillCount != len(allSkills) || len(report.Projections) != len(wantHosts) {
		t.Fatalf("parity summary = %#v", report)
	}

	adaptersByHost := make(map[initplanning.HostID]currentSkillAdapter, len(adapters))
	for _, adapter := range adapters {
		adaptersByHost[adapter.definition.host] = adapter
	}
	for _, projection := range projections {
		adapter := adaptersByHost[projection.Host()]
		assertCurrentProjectionMatchesLegacyBytes(t, bundle, adapter, projection)
	}

	hermes := projectionForHost(t, projections, initplanning.HostHermes)
	hermesSuffix := filepath.Join("haft", "h-reason", "SKILL.md")
	if !projectionHasSuffix(hermes, hermesSuffix) {
		t.Fatalf("Hermes projection lacks category path suffix %s", hermesSuffix)
	}
	codex := projectionForHost(t, projections, initplanning.HostCodex)
	if len(codex.Outputs()) != len(allSkills)*2 {
		t.Fatalf("Codex output count = %d, want %d", len(codex.Outputs()), len(allSkills)*2)
	}
	for _, projection := range projections {
		if projection.Host() == initplanning.HostCodex {
			continue
		}
		if len(projection.Outputs()) != len(allSkills) {
			t.Fatalf("host %s output count = %d, want %d", projection.Host(), len(projection.Outputs()), len(allSkills))
		}
	}
}

func TestCurrentSkillRewriteRegistryPreservesPriorRenderedBytes(t *testing.T) {
	t.Parallel()

	corpus := strings.Join([]string{
		"/h-frame /h-decide",
		"Slash commands slash commands Slash command slash command",
		"Quint quint",
		"mcp__haft__haft_problem mcp__haft__haft_solution",
		"mcp__haft__haft_decision mcp__haft__haft_query",
		"mcp__haft__haft_note mcp__haft__haft_refresh",
		"mcp__haft__haft_commission mcp__haft__haft_spec_section",
		"mcp__haft__haft_onboard mcp__haft__haft_entity",
		"mcp__haft__haft_method",
	}, "\n")
	adapters, err := buildCurrentSkillAdapterRegistry()
	if err != nil {
		t.Fatalf("build current skill adapters: %v", err)
	}
	for _, adapter := range adapters {
		t.Run(string(adapter.definition.host), func(t *testing.T) {
			rendered, err := adapter.rewrite.Apply([]byte(corpus))
			if err != nil {
				t.Fatalf("transform current skill references: %v", err)
			}
			got := string(rendered)
			want := priorSkillTransform(adapter.definition.platform, corpus)
			if got != want {
				t.Fatalf("rendered bytes changed\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestCurrentSkillAdapterRegistryRejectsUnsupportedPlatforms(
	t *testing.T,
) {
	t.Parallel()

	if _, err := currentSkillAdapterForPlatform("gemini"); err == nil {
		t.Fatal("non-skill Gemini adapter was accepted as a standard skill renderer")
	}
	if _, err := currentSkillAdapterForPlatform("pi"); err == nil {
		t.Fatal("condensed Pi package was accepted as an undeclared standard skill renderer")
	}
}

func assertCurrentProjectionMatchesLegacyBytes(
	t *testing.T,
	bundle initplanning.SkillSourceBundle,
	adapter currentSkillAdapter,
	projection initplanning.SkillComponentProjection,
) {
	t.Helper()
	outputs := make(map[string]initplanning.RenderedOutput, len(projection.Outputs()))
	for _, output := range projection.Outputs() {
		outputs[output.Path()] = output
	}
	for _, source := range bundle.Skills() {
		record := projectionRecord(t, projection, source.Name())
		output, exists := outputs[record.RenderedSkillPath]
		if !exists {
			t.Fatalf("host %s skill %s output is missing", projection.Host(), source.Name())
		}
		want := priorSkillTransform(adapter.definition.platform, string(source.Content()))
		if !bytes.Equal(output.Content(), []byte(want)) {
			t.Fatalf("host %s skill %s differs from prior rendered bytes", projection.Host(), source.Name())
		}
		if projection.Host() != initplanning.HostCodex {
			continue
		}
		policyOutput, exists := outputs[record.RenderedPolicyPath]
		if !exists {
			t.Fatalf("Codex skill %s policy output is missing", source.Name())
		}
		allowImplicit := source.InvocationPolicy() == initplanning.SkillInvocationImplicitAllowed
		wantPolicy := "policy:\n  allow_implicit_invocation: "
		if allowImplicit {
			wantPolicy += "true\n"
		}
		if !allowImplicit {
			wantPolicy += "false\n"
		}
		if string(policyOutput.Content()) != wantPolicy {
			t.Fatalf("Codex skill %s policy = %q, want %q", source.Name(), policyOutput.Content(), wantPolicy)
		}
	}
}

func priorSkillTransform(platform string, content string) string {
	switch platform {
	case "codex":
		return strings.NewReplacer(
			"/h-", "$h-",
			"Slash commands", "Explicit skill invocations",
			"slash commands", "explicit skill invocations",
			"Slash command", "Explicit skill",
			"slash command", "explicit skill",
			"Quint", "Haft",
			"quint", "haft",
		).Replace(content)
	case "agy", "hermes":
		return priorToolTransform(content, "haft_", false)
	case "grok":
		return priorToolTransform(content, "haft__haft_", true)
	default:
		return content
	}
}

func priorToolTransform(
	content string,
	targetPrefix string,
	includeMethod bool,
) string {
	toolNames := []string{
		"problem",
		"solution",
		"decision",
		"query",
		"note",
		"refresh",
		"commission",
		"spec_section",
		"onboard",
		"entity",
	}
	if includeMethod {
		toolNames = append(toolNames, "method")
	}
	arguments := make([]string, 0, len(toolNames)*2)
	for _, toolName := range toolNames {
		arguments = append(
			arguments,
			"mcp__haft__haft_"+toolName,
			targetPrefix+toolName,
		)
	}
	return strings.NewReplacer(arguments...).Replace(content)
}

func projectionForHost(
	t *testing.T,
	projections []initplanning.SkillComponentProjection,
	host initplanning.HostID,
) initplanning.SkillComponentProjection {
	t.Helper()
	for _, projection := range projections {
		if projection.Host() == host {
			return projection
		}
	}
	t.Fatalf("projection for host %s is missing", host)
	return initplanning.SkillComponentProjection{}
}

func projectionRecord(
	t *testing.T,
	projection initplanning.SkillComponentProjection,
	skillName string,
) initplanning.RenderedSkillRecord {
	t.Helper()
	for _, record := range projection.Records() {
		if record.Name == skillName {
			return record
		}
	}
	t.Fatalf("host %s record for skill %s is missing", projection.Host(), skillName)
	return initplanning.RenderedSkillRecord{}
}

func projectionHasSuffix(
	projection initplanning.SkillComponentProjection,
	suffix string,
) bool {
	for _, output := range projection.Outputs() {
		if strings.HasSuffix(output.Path(), suffix) {
			return true
		}
	}
	return false
}
