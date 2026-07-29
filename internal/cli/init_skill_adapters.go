package cli

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

type currentSkillAdapterDefinition struct {
	platform     string
	host         initplanning.HostID
	edition      string
	prefix       []string
	policy       initplanning.SkillPolicyCarrierKind
	rewriteID    string
	rewriteRules []initplanning.SkillRewriteInput
}

type currentSkillAdapter struct {
	definition currentSkillAdapterDefinition
	rewrite    initplanning.SkillRewriteSet
	renderer   initplanning.SkillComponentRenderer
}

func currentSkillAdapterDefinitions() []currentSkillAdapterDefinition {
	return []currentSkillAdapterDefinition{
		newExactSkillAdapterDefinition("claude", initplanning.HostClaude),
		newCodexSkillAdapterDefinition(),
		newExactSkillAdapterDefinition("cursor", initplanning.HostCursor),
		newToolRewriteSkillAdapterDefinition(
			"agy",
			initplanning.HostAntigravity,
			"haft_",
			false,
		),
		newToolRewriteSkillAdapterDefinition(
			"grok",
			initplanning.HostGrok,
			"haft__haft_",
			true,
		),
		newHermesSkillAdapterDefinition(),
		newExactSkillAdapterDefinition("opencode", initplanning.HostOpenCode),
		newExactSkillAdapterDefinition("air", initplanning.HostAir),
	}
}

func newExactSkillAdapterDefinition(
	platform string,
	host initplanning.HostID,
) currentSkillAdapterDefinition {
	edition := string(host) + ".skills.v1"
	return currentSkillAdapterDefinition{
		platform:  platform,
		host:      host,
		edition:   edition,
		policy:    initplanning.SkillPolicyInSourceFrontmatter,
		rewriteID: string(host) + ".exact-source.v1",
	}
}

func newCodexSkillAdapterDefinition() currentSkillAdapterDefinition {
	return currentSkillAdapterDefinition{
		platform:  "codex",
		host:      initplanning.HostCodex,
		edition:   "codex.skills.v1",
		policy:    initplanning.SkillPolicyCodexOpenAIYAML,
		rewriteID: "codex.skill-syntax.v1",
		rewriteRules: []initplanning.SkillRewriteInput{
			{From: "/h-", To: "$h-"},
			{From: "Slash commands", To: "Explicit skill invocations"},
			{From: "slash commands", To: "explicit skill invocations"},
			{From: "Slash command", To: "Explicit skill"},
			{From: "slash command", To: "explicit skill"},
			{From: "Quint", To: "Haft"},
			{From: "quint", To: "haft"},
		},
	}
}

func newToolRewriteSkillAdapterDefinition(
	platform string,
	host initplanning.HostID,
	targetPrefix string,
	includeMethod bool,
) currentSkillAdapterDefinition {
	return currentSkillAdapterDefinition{
		platform:     platform,
		host:         host,
		edition:      string(host) + ".skills.v1",
		policy:       initplanning.SkillPolicyInSourceFrontmatter,
		rewriteID:    string(host) + ".tool-names.v1",
		rewriteRules: haftToolRewriteRules(targetPrefix, includeMethod),
	}
}

func newHermesSkillAdapterDefinition() currentSkillAdapterDefinition {
	definition := newToolRewriteSkillAdapterDefinition(
		"hermes",
		initplanning.HostHermes,
		"haft_",
		false,
	)
	definition.prefix = []string{"haft"}
	return definition
}

func haftToolRewriteRules(
	targetPrefix string,
	includeMethod bool,
) []initplanning.SkillRewriteInput {
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
	rules := make([]initplanning.SkillRewriteInput, len(toolNames))
	for index, toolName := range toolNames {
		rules[index] = initplanning.SkillRewriteInput{
			From: "mcp__haft__haft_" + toolName,
			To:   targetPrefix + toolName,
		}
	}
	return rules
}

func buildCurrentSkillAdapterRegistry() ([]currentSkillAdapter, error) {
	definitions := currentSkillAdapterDefinitions()
	adapters := make([]currentSkillAdapter, 0, len(definitions))
	seenPlatforms := make(map[string]struct{}, len(definitions))
	seenHosts := make(map[initplanning.HostID]struct{}, len(definitions))
	for _, definition := range definitions {
		if definition.platform == "" {
			return nil, fmt.Errorf("current skill adapter platform is empty")
		}
		if _, duplicate := seenPlatforms[definition.platform]; duplicate {
			return nil, fmt.Errorf("current skill adapter repeats platform %s", definition.platform)
		}
		if _, duplicate := seenHosts[definition.host]; duplicate {
			return nil, fmt.Errorf("current skill adapter repeats host %s", definition.host)
		}
		rewrite, err := initplanning.NewSkillRewriteSet(
			definition.rewriteID,
			definition.rewriteRules,
		)
		if err != nil {
			return nil, fmt.Errorf("build %s skill rewrite set: %w", definition.host, err)
		}
		renderer, err := initplanning.NewSkillComponentRenderer(
			definition.host,
			definition.edition,
			definition.prefix,
			definition.policy,
			rewrite,
		)
		if err != nil {
			return nil, fmt.Errorf("build %s skill renderer: %w", definition.host, err)
		}
		sealedDefinition := definition
		sealedDefinition.prefix = slices.Clone(definition.prefix)
		sealedDefinition.rewriteRules = slices.Clone(definition.rewriteRules)
		adapters = append(adapters, currentSkillAdapter{
			definition: sealedDefinition,
			rewrite:    rewrite,
			renderer:   renderer,
		})
		seenPlatforms[definition.platform] = struct{}{}
		seenHosts[definition.host] = struct{}{}
	}
	return adapters, nil
}

func currentSkillAdapterForPlatform(
	platform string,
) (currentSkillAdapter, error) {
	adapters, err := buildCurrentSkillAdapterRegistry()
	if err != nil {
		return currentSkillAdapter{}, err
	}
	for _, adapter := range adapters {
		if adapter.definition.platform == platform {
			return adapter, nil
		}
	}
	return currentSkillAdapter{}, fmt.Errorf("platform %s has no standard skill adapter", platform)
}
