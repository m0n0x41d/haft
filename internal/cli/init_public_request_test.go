package cli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestCompilePublicInitRequestPreservesExplicitFlagPublicationContract(
	t *testing.T,
) {
	t.Parallel()

	projectRoot := filepath.Join(t.TempDir(), "project")
	projectID := "qnt_e3149c17"
	testCases := []struct {
		name             string
		hosts            initHostOptions
		local            bool
		agents           bool
		mcpOnly          bool
		omitInstructions bool
		wantBindings     []string
		wantAgents       publicAgentSkillsTarget
	}{
		{
			name:  "claude default scope keeps project MCP and user skills",
			hosts: initHostOptions{claude: true},
			wantBindings: []string{
				"claude/project:instructions,mcp",
				"claude/user:skills",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name:             "claude local coalesces one project binding",
			hosts:            initHostOptions{claude: true},
			local:            true,
			omitInstructions: true,
			wantBindings: []string{
				"claude/project:mcp,skills",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name:  "codex default scope keeps project MCP instructions and user skills",
			hosts: initHostOptions{codex: true},
			wantBindings: []string{
				"codex/project:instructions,mcp",
				"codex/user:skills",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name:   "codex full integration coalesces redundant agents target",
			hosts:  initHostOptions{codex: true},
			agents: true,
			wantBindings: []string{
				"codex/project:instructions,mcp",
				"codex/user:skills",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name: "claude and codex local omit instructions coalesces agents",
			hosts: initHostOptions{
				claude: true,
				codex:  true,
			},
			local:            true,
			agents:           true,
			omitInstructions: true,
			wantBindings: []string{
				"claude/project:mcp,skills",
				"codex/project:mcp,skills",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name:    "codex mcp only compatibility suppresses skills and instructions",
			hosts:   initHostOptions{codex: true},
			mcpOnly: true,
			wantBindings: []string{
				"codex/project:mcp",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name:    "claude mcp only compatibility suppresses skills and instructions",
			hosts:   initHostOptions{claude: true},
			mcpOnly: true,
			wantBindings: []string{
				"claude/project:mcp",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name:         "agents local has no implicit host",
			local:        true,
			agents:       true,
			wantBindings: []string{},
			wantAgents:   publicAgentSkillsProject,
		},
		{
			name:  "all means claude and codex without agents",
			hosts: initHostOptions{all: true},
			wantBindings: []string{
				"claude/project:instructions,mcp",
				"claude/user:skills",
				"codex/project:instructions,mcp",
				"codex/user:skills",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name:    "all mcp only keeps the stable host set",
			hosts:   initHostOptions{all: true},
			mcpOnly: true,
			wantBindings: []string{
				"claude/project:mcp",
				"codex/project:mcp",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name:  "antigravity local splits user MCP and project skills",
			hosts: initHostOptions{agy: true},
			local: true,
			wantBindings: []string{
				"antigravity/project:skills",
				"antigravity/user:mcp",
			},
			wantAgents: publicAgentSkillsNone,
		},
		{
			name:  "grok local owns exact project coherent components",
			hosts: initHostOptions{grok: true},
			local: true,
			wantBindings: []string{
				"grok/project:instructions,mcp,skills",
			},
			wantAgents: publicAgentSkillsNone,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request, err := compilePublicInitRequest(
				weakPublicInitRequest{
					invocation:         initplanning.InvocationExplicit,
					projectRoot:        projectRoot,
					projectID:          projectID,
					hosts:              testCase.hosts,
					local:              testCase.local,
					agents:             testCase.agents,
					mcpOnly:            testCase.mcpOnly,
					omitInstructions:   testCase.omitInstructions,
					profileScopeID:     "",
					overseer:           publicOverseerWeakDisabled(),
					hermesHomeInput:    "",
					hermesProfileInput: "",
				},
			)
			if err != nil {
				t.Fatalf("compilePublicInitRequest: %v", err)
			}

			gotBindings := publicBindingLabels(request.hostBindings)
			if !slices.Equal(gotBindings, testCase.wantBindings) {
				t.Fatalf(
					"bindings = %v, want %v",
					gotBindings,
					testCase.wantBindings,
				)
			}
			if request.agentSkills != testCase.wantAgents {
				t.Fatalf(
					"agent skills target = %q, want %q",
					request.agentSkills,
					testCase.wantAgents,
				)
			}
		})
	}
}

func TestCompilePublicInitRequestRejectsMCPOnlyWithoutExplicitHost(
	t *testing.T,
) {
	t.Parallel()

	_, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: filepath.Join(t.TempDir(), "project"),
			projectID:   "qnt_e3149c17",
			mcpOnly:     true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "requires an explicit host flag") {
		t.Fatalf("mcp-only validation error = %v", err)
	}
}

func TestCompilePublicInitRequestNeverInfersAHost(
	t *testing.T,
) {
	t.Parallel()

	projectRoot := filepath.Join(t.TempDir(), "project")
	base := weakPublicInitRequest{
		invocation:  initplanning.InvocationExplicit,
		projectRoot: projectRoot,
		projectID:   "qnt_e3149c17",
		overseer:    publicOverseerWeakDisabled(),
	}

	hostlessRequest, err := compilePublicInitRequest(base)
	if err != nil {
		t.Fatalf("compile hostless request: %v", err)
	}
	if len(hostlessRequest.hostBindings) != 0 {
		t.Fatalf(
			"hostless request inferred bindings: %v",
			hostlessRequest.hostBindings,
		)
	}

	agentsOnly := base
	agentsOnly.agents = true
	agentRequest, err := compilePublicInitRequest(agentsOnly)
	if err != nil {
		t.Fatalf("compile agents request: %v", err)
	}
	if len(agentRequest.hostBindings) != 0 {
		t.Fatalf("agents-only request selected hosts: %v", agentRequest.hostBindings)
	}
	if agentRequest.agentSkills != publicAgentSkillsUser {
		t.Fatalf("agents target = %q", agentRequest.agentSkills)
	}
}

func TestCompilePublicInitRequestRepresentsAncillaryOptionsWithoutBooleans(
	t *testing.T,
) {
	t.Parallel()

	projectRoot := filepath.Join(t.TempDir(), "project")
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:     initplanning.InvocationExplicit,
			projectRoot:    projectRoot,
			projectID:      "qnt_e3149c17",
			hosts:          initHostOptions{hermes: true},
			profileScopeID: "software",
			overseer: publicOverseerWeakConfiguration{
				reviewer:     "command",
				command:      "reviewer",
				reviewOnHook: true,
				timeout:      90,
			},
			hermesHomeInput:    filepath.Join(t.TempDir(), "hermes"),
			hermesProfileInput: "engineering",
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	if request.profileScope.kind != publicProfileScopeExact ||
		request.profileScope.scopeID != "software" {
		t.Fatalf("profile scope = %#v", request.profileScope)
	}
	if request.overseer.kind != publicOverseerConfigure ||
		request.overseer.reviewer != "command" ||
		request.overseer.command != "reviewer" ||
		request.overseer.hook != publicOverseerHookEnabled ||
		request.overseer.timeout != 90 {
		t.Fatalf("overseer = %#v", request.overseer)
	}
	if request.hermes.kind != publicHermesConfigure ||
		request.hermes.homeInput == "" ||
		request.hermes.profileInput != "engineering" {
		t.Fatalf("hermes options = %#v", request.hermes)
	}
}

func TestCompilePublicInitRequestRejectsCoreOnlyPublicationCombinations(
	t *testing.T,
) {
	t.Parallel()

	projectRoot := filepath.Join(t.TempDir(), "project")
	testCases := []weakPublicInitRequest{
		{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			hosts:       initHostOptions{codex: true},
			overseer:    publicOverseerWeakDisabled(),
		},
		{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			agents:      true,
			overseer:    publicOverseerWeakDisabled(),
		},
		{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			overseer: publicOverseerWeakConfiguration{
				reviewer: "manual",
				timeout:  90,
			},
		},
	}

	for index, testCase := range testCases {
		if _, err := compilePublicInitRequest(testCase); err == nil {
			t.Fatalf("case %d accepted an impossible core-only combination", index)
		}
	}
}

func TestCompilePublicInitRequestRejectsHermesOptionsWithoutHermes(
	t *testing.T,
) {
	t.Parallel()

	projectRoot := filepath.Join(t.TempDir(), "project")
	for _, input := range []weakPublicInitRequest{
		{
			invocation:      initplanning.InvocationExplicit,
			projectRoot:     projectRoot,
			projectID:       "qnt_e3149c17",
			hosts:           initHostOptions{codex: true},
			overseer:        publicOverseerWeakDisabled(),
			hermesHomeInput: filepath.Join(t.TempDir(), "hermes"),
		},
		{
			invocation:         initplanning.InvocationExplicit,
			projectRoot:        projectRoot,
			projectID:          "qnt_e3149c17",
			hosts:              initHostOptions{codex: true},
			overseer:           publicOverseerWeakDisabled(),
			hermesProfileInput: "engineering",
		},
	} {
		if _, err := compilePublicInitRequest(input); err == nil {
			t.Fatalf("accepted Hermes options without --hermes: %#v", input)
		}
	}
}

func publicBindingLabels(
	bindings []publicHostBinding,
) []string {
	labels := make([]string, len(bindings))
	for index, binding := range bindings {
		components := binding.components.Values()
		componentNames := make([]string, len(components))
		for componentIndex, component := range components {
			componentNames[componentIndex] = string(component)
		}
		labels[index] = string(binding.host) +
			"/" + string(binding.scope) +
			":" + strings.Join(componentNames, ",")
	}
	return labels
}
