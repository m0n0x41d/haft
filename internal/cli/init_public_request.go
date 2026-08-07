package cli

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

type publicCoreMode string

const (
	publicCoreInitializeOrMigrate publicCoreMode = "initialize_or_migrate"
	publicCoreOnly                publicCoreMode = "core_only"
)

type publicAgentSkillsTarget string

const (
	publicAgentSkillsNone    publicAgentSkillsTarget = "none"
	publicAgentSkillsProject publicAgentSkillsTarget = "project"
	publicAgentSkillsUser    publicAgentSkillsTarget = "user"
)

type publicInstructionPolicy string

const (
	publicInstructionsInclude publicInstructionPolicy = "include"
	publicInstructionsOmit    publicInstructionPolicy = "omit"
)

type publicHostPublicationMode string

const (
	publicHostFullIntegration publicHostPublicationMode = "full_integration"
	publicHostMCPOnly         publicHostPublicationMode = "mcp_only"
)

type publicProfileScopeKind string

const (
	publicProfileScopeAutomatic publicProfileScopeKind = "automatic"
	publicProfileScopeExact     publicProfileScopeKind = "exact"
)

type publicProfileScope struct {
	kind    publicProfileScopeKind
	scopeID string
}

type publicOverseerKind string

const (
	publicOverseerDisabled  publicOverseerKind = "disabled"
	publicOverseerConfigure publicOverseerKind = "configure"
)

type publicOverseerHookMode string

const (
	publicOverseerHookDisabled publicOverseerHookMode = "disabled"
	publicOverseerHookEnabled  publicOverseerHookMode = "enabled"
)

type publicOverseerSelection struct {
	kind     publicOverseerKind
	reviewer string
	command  string
	hook     publicOverseerHookMode
	timeout  int
}

type publicHermesKind string

const (
	publicHermesDisabled  publicHermesKind = "disabled"
	publicHermesConfigure publicHermesKind = "configure"
)

type publicHermesOptions struct {
	kind         publicHermesKind
	homeInput    string
	profileInput string
}

type publicHostBinding struct {
	host       initplanning.HostID
	scope      initplanning.InstallScope
	components initplanning.ComponentSet
}

type publicInitRequest struct {
	invocation   initplanning.InvocationPolicy
	projectRoot  string
	projectID    string
	core         publicCoreMode
	local        bool
	hostBindings []publicHostBinding
	hostMode     publicHostPublicationMode
	agentSkills  publicAgentSkillsTarget
	instructions publicInstructionPolicy
	profileScope publicProfileScope
	overseer     publicOverseerSelection
	hermes       publicHermesOptions
}

type publicOverseerWeak interface {
	publicOverseerWeakKind() publicOverseerKind
}

type publicOverseerWeakDisabledSelection struct{}

func (publicOverseerWeakDisabledSelection) publicOverseerWeakKind() publicOverseerKind {
	return publicOverseerDisabled
}

type publicOverseerWeakConfiguration struct {
	reviewer     string
	command      string
	reviewOnHook bool
	timeout      int
}

func (publicOverseerWeakConfiguration) publicOverseerWeakKind() publicOverseerKind {
	return publicOverseerConfigure
}

func publicOverseerWeakDisabled() publicOverseerWeak {
	return publicOverseerWeakDisabledSelection{}
}

type weakPublicInitRequest struct {
	invocation         initplanning.InvocationPolicy
	projectRoot        string
	projectID          string
	hosts              initHostOptions
	local              bool
	agents             bool
	mcpOnly            bool
	coreOnly           bool
	omitInstructions   bool
	profileScopeID     string
	overseer           publicOverseerWeak
	hermesHomeInput    string
	hermesProfileInput string
}

type publicHostBlueprint struct {
	host                    initplanning.HostID
	selected                bool
	fixed                   []publicBindingComponents
	hasVariableSkillBinding bool
	hasInstructionComponent bool
}

type publicBindingComponents struct {
	scope      initplanning.InstallScope
	components []initplanning.Component
}

type publicBindingKey struct {
	host  initplanning.HostID
	scope initplanning.InstallScope
}

func compilePublicInitRequest(
	input weakPublicInitRequest,
) (publicInitRequest, error) {
	base, err := parsePublicInitBase(input)
	if err != nil {
		return publicInitRequest{}, err
	}
	profileScope, err := parsePublicProfileScope(input.profileScopeID)
	if err != nil {
		return publicInitRequest{}, err
	}
	overseer, err := parsePublicOverseer(input.overseer)
	if err != nil {
		return publicInitRequest{}, err
	}
	if err := validatePublicCoreOnly(input, overseer); err != nil {
		return publicInitRequest{}, err
	}
	hostMode, err := parsePublicHostPublicationMode(input)
	if err != nil {
		return publicInitRequest{}, err
	}

	hosts := selectPublicInitHosts(input)
	if err := validatePublicHermesOptions(input, hosts); err != nil {
		return publicInitRequest{}, err
	}
	instructions := publicInstructionsInclude
	if input.omitInstructions {
		instructions = publicInstructionsOmit
	}
	bindings, err := compilePublicHostBindings(
		hosts,
		input.local,
		instructions,
		hostMode,
	)
	if err != nil {
		return publicInitRequest{}, err
	}
	agentSkills := selectPublicAgentSkills(
		input.agents,
		input.local,
		hosts,
		hostMode,
	)
	core := publicCoreInitializeOrMigrate
	if input.coreOnly {
		core = publicCoreOnly
	}
	hermes := publicHermesOptions{
		homeInput:    input.hermesHomeInput,
		profileInput: input.hermesProfileInput,
	}
	if hosts.hermes {
		hermes.kind = publicHermesConfigure
	} else {
		hermes.kind = publicHermesDisabled
	}
	return publicInitRequest{
		invocation:   base.InvocationPolicy(),
		projectRoot:  base.ProjectRoot(),
		projectID:    base.ProjectID().String(),
		core:         core,
		local:        input.local,
		hostBindings: bindings,
		hostMode:     hostMode,
		agentSkills:  agentSkills,
		instructions: instructions,
		profileScope: profileScope,
		overseer:     overseer,
		hermes:       hermes,
	}, nil
}

func parsePublicHostPublicationMode(
	input weakPublicInitRequest,
) (publicHostPublicationMode, error) {
	if !input.mcpOnly {
		return publicHostFullIntegration, nil
	}
	if input.coreOnly {
		return "", fmt.Errorf(
			"--mcp-only cannot be combined with --core-only",
		)
	}
	if !hasInitHost(input.hosts) {
		return "", fmt.Errorf(
			"--mcp-only requires an explicit host flag or --all",
		)
	}
	return publicHostMCPOnly, nil
}

func validatePublicHermesOptions(
	input weakPublicInitRequest,
	hosts initHostOptions,
) error {
	hasHome := strings.TrimSpace(input.hermesHomeInput) != ""
	hasProfile := strings.TrimSpace(input.hermesProfileInput) != ""
	if hosts.hermes || (!hasHome && !hasProfile) {
		return nil
	}
	return fmt.Errorf(
		"--hermes-home and --profile require explicit --hermes",
	)
}

func parsePublicInitBase(
	input weakPublicInitRequest,
) (initplanning.InitIntent, error) {
	weak := initplanning.WeakInitIntent{
		InvocationPolicy: string(input.invocation),
		ProjectRoot:      input.projectRoot,
		ProjectID:        input.projectID,
	}
	base, err := initplanning.ParseInitIntent(weak)
	if err != nil {
		return initplanning.InitIntent{}, fmt.Errorf(
			"parse public initialization basis: %w",
			err,
		)
	}
	return base, nil
}

func parsePublicProfileScope(
	raw string,
) (publicProfileScope, error) {
	if raw == "" {
		return publicProfileScope{
			kind: publicProfileScopeAutomatic,
		}, nil
	}
	if raw != strings.TrimSpace(raw) {
		return publicProfileScope{}, fmt.Errorf(
			"profile scope ID must be supplied in exact form",
		)
	}
	return publicProfileScope{
		kind:    publicProfileScopeExact,
		scopeID: raw,
	}, nil
}

func parsePublicOverseer(
	weak publicOverseerWeak,
) (publicOverseerSelection, error) {
	if weak == nil {
		return publicOverseerSelection{}, fmt.Errorf(
			"overseer initialization posture is required",
		)
	}
	switch typed := weak.(type) {
	case publicOverseerWeakDisabledSelection:
		return publicOverseerSelection{
			kind: publicOverseerDisabled,
			hook: publicOverseerHookDisabled,
		}, nil
	case publicOverseerWeakConfiguration:
		if typed.timeout <= 0 {
			return publicOverseerSelection{}, fmt.Errorf(
				"overseer review timeout must be positive",
			)
		}
		hook := publicOverseerHookDisabled
		if typed.reviewOnHook {
			hook = publicOverseerHookEnabled
		}
		return publicOverseerSelection{
			kind:     publicOverseerConfigure,
			reviewer: strings.TrimSpace(typed.reviewer),
			command:  strings.TrimSpace(typed.command),
			hook:     hook,
			timeout:  typed.timeout,
		}, nil
	default:
		return publicOverseerSelection{}, fmt.Errorf(
			"overseer initialization posture is not closed",
		)
	}
}

func validatePublicCoreOnly(
	input weakPublicInitRequest,
	overseer publicOverseerSelection,
) error {
	if !input.coreOnly {
		return nil
	}
	hasHost := hasInitHost(input.hosts)
	hasPublication := hasHost || input.agents
	hasPublication = hasPublication ||
		overseer.kind == publicOverseerConfigure
	if hasPublication {
		return fmt.Errorf(
			"core-only initialization cannot contain publication effects",
		)
	}
	return nil
}

func selectPublicInitHosts(
	input weakPublicInitRequest,
) initHostOptions {
	if input.coreOnly {
		return initHostOptions{}
	}
	requested := input.hosts
	if requested.all {
		requested.claude = true
		requested.codex = true
		requested.all = false
	}
	return requested
}

func selectPublicAgentSkills(
	requested bool,
	local bool,
	hosts initHostOptions,
	hostMode publicHostPublicationMode,
) publicAgentSkillsTarget {
	if !requested {
		return publicAgentSkillsNone
	}
	if hosts.codex && hostMode == publicHostFullIntegration {
		return publicAgentSkillsNone
	}
	if local {
		return publicAgentSkillsProject
	}
	return publicAgentSkillsUser
}

func compilePublicHostBindings(
	hosts initHostOptions,
	local bool,
	instructions publicInstructionPolicy,
	mode publicHostPublicationMode,
) ([]publicHostBinding, error) {
	blueprints := publicHostBlueprints(hosts)
	componentsByBinding := make(
		map[publicBindingKey]map[initplanning.Component]struct{},
	)
	for _, blueprint := range blueprints {
		if !blueprint.selected {
			continue
		}
		if mode == publicHostMCPOnly {
			if !addPublicMCPOnlyBinding(
				componentsByBinding,
				blueprint,
			) {
				return nil, fmt.Errorf(
					"--mcp-only is unavailable for host %s",
					blueprint.host,
				)
			}
			continue
		}
		addPublicFixedBindings(componentsByBinding, blueprint)
		addPublicVariableSkillBinding(
			componentsByBinding,
			blueprint,
			local,
		)
		addPublicInstructionBinding(
			componentsByBinding,
			blueprint,
			instructions,
		)
	}
	return buildPublicHostBindings(componentsByBinding)
}

func publicHostBlueprints(
	hosts initHostOptions,
) []publicHostBlueprint {
	return []publicHostBlueprint{
		{
			host:                    initplanning.HostClaude,
			selected:                hosts.claude,
			fixed:                   publicProjectMCPBinding(),
			hasVariableSkillBinding: true,
			hasInstructionComponent: true,
		},
		{
			host:                    initplanning.HostCursor,
			selected:                hosts.cursor,
			fixed:                   publicProjectMCPBinding(),
			hasVariableSkillBinding: true,
		},
		{
			host:     initplanning.HostGemini,
			selected: hosts.gemini,
			fixed:    publicUserMCPBinding(),
		},
		{
			host:                    initplanning.HostCodex,
			selected:                hosts.codex,
			fixed:                   publicProjectMCPBinding(),
			hasVariableSkillBinding: true,
			hasInstructionComponent: true,
		},
		{
			host:     initplanning.HostAir,
			selected: hosts.air,
			fixed: []publicBindingComponents{{
				scope: initplanning.ScopeProject,
				components: []initplanning.Component{
					initplanning.ComponentMCP,
					initplanning.ComponentSkills,
				},
			}},
		},
		{
			host:                    initplanning.HostOpenCode,
			selected:                hosts.opencode,
			fixed:                   publicProjectMCPBinding(),
			hasVariableSkillBinding: true,
		},
		{
			host:     initplanning.HostZed,
			selected: hosts.zed,
			fixed:    publicUserMCPBinding(),
		},
		{
			host:                    initplanning.HostAntigravity,
			selected:                hosts.agy,
			fixed:                   publicUserMCPBinding(),
			hasVariableSkillBinding: true,
		},
		{
			host:     initplanning.HostPi,
			selected: hosts.pi,
			fixed: []publicBindingComponents{{
				scope: initplanning.ScopeProject,
				components: []initplanning.Component{
					initplanning.ComponentPackage,
				},
			}},
		},
		{
			host:                    initplanning.HostGrok,
			selected:                hosts.grok,
			fixed:                   publicProjectMCPBinding(),
			hasVariableSkillBinding: true,
			hasInstructionComponent: true,
		},
	}
}

func addPublicMCPOnlyBinding(
	target map[publicBindingKey]map[initplanning.Component]struct{},
	blueprint publicHostBlueprint,
) bool {
	added := false
	for _, binding := range blueprint.fixed {
		if !slices.Contains(
			binding.components,
			initplanning.ComponentMCP,
		) {
			continue
		}
		addPublicBindingComponents(
			target,
			publicBindingKey{
				host:  blueprint.host,
				scope: binding.scope,
			},
			[]initplanning.Component{
				initplanning.ComponentMCP,
			},
		)
		added = true
	}
	return added
}

func publicProjectMCPBinding() []publicBindingComponents {
	return []publicBindingComponents{{
		scope: initplanning.ScopeProject,
		components: []initplanning.Component{
			initplanning.ComponentMCP,
		},
	}}
}

func publicUserMCPBinding() []publicBindingComponents {
	return []publicBindingComponents{{
		scope: initplanning.ScopeUser,
		components: []initplanning.Component{
			initplanning.ComponentMCP,
		},
	}}
}

func addPublicFixedBindings(
	target map[publicBindingKey]map[initplanning.Component]struct{},
	blueprint publicHostBlueprint,
) {
	for _, binding := range blueprint.fixed {
		addPublicBindingComponents(
			target,
			publicBindingKey{
				host:  blueprint.host,
				scope: binding.scope,
			},
			binding.components,
		)
	}
}

func addPublicVariableSkillBinding(
	target map[publicBindingKey]map[initplanning.Component]struct{},
	blueprint publicHostBlueprint,
	local bool,
) {
	if !blueprint.hasVariableSkillBinding {
		return
	}
	scope := initplanning.ScopeUser
	if local {
		scope = initplanning.ScopeProject
	}
	addPublicBindingComponents(
		target,
		publicBindingKey{
			host:  blueprint.host,
			scope: scope,
		},
		[]initplanning.Component{
			initplanning.ComponentSkills,
		},
	)
}

func addPublicInstructionBinding(
	target map[publicBindingKey]map[initplanning.Component]struct{},
	blueprint publicHostBlueprint,
	policy publicInstructionPolicy,
) {
	if !blueprint.hasInstructionComponent ||
		policy != publicInstructionsInclude {
		return
	}
	addPublicBindingComponents(
		target,
		publicBindingKey{
			host:  blueprint.host,
			scope: initplanning.ScopeProject,
		},
		[]initplanning.Component{
			initplanning.ComponentInstructions,
		},
	)
}

func addPublicBindingComponents(
	target map[publicBindingKey]map[initplanning.Component]struct{},
	key publicBindingKey,
	components []initplanning.Component,
) {
	values, exists := target[key]
	if !exists {
		values = make(map[initplanning.Component]struct{})
		target[key] = values
	}
	for _, component := range components {
		values[component] = struct{}{}
	}
}

func buildPublicHostBindings(
	source map[publicBindingKey]map[initplanning.Component]struct{},
) ([]publicHostBinding, error) {
	keys := make([]publicBindingKey, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left int, right int) bool {
		leftHost := string(keys[left].host)
		rightHost := string(keys[right].host)
		if leftHost != rightHost {
			return leftHost < rightHost
		}
		return keys[left].scope < keys[right].scope
	})

	result := make([]publicHostBinding, 0, len(keys))
	for _, key := range keys {
		raw := publicComponentNames(source[key])
		components, err := initplanning.ParseComponentSet(raw)
		if err != nil {
			return nil, err
		}
		result = append(result, publicHostBinding{
			host:       key.host,
			scope:      key.scope,
			components: components,
		})
	}
	return result, nil
}

func publicComponentNames(
	source map[initplanning.Component]struct{},
) []string {
	result := make([]string, 0, len(source))
	for component := range source {
		result = append(result, string(component))
	}
	sort.Strings(result)
	return result
}
