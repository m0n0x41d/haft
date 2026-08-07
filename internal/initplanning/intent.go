// Package initplanning owns the effect-free core of project initialization.
// Host discovery, terminal input, filesystem inspection, rendering, and apply
// effects remain outside this package.
package initplanning

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectidentity"
)

type InvocationPolicy string

const (
	InvocationInteractive       InvocationPolicy = "interactive_selection"
	InvocationExplicit          InvocationPolicy = "explicit_non_interactive"
	InvocationManifestReconcile InvocationPolicy = "manifest_reconcile"
)

func parseInvocationPolicy(raw string) (InvocationPolicy, error) {
	policy := InvocationPolicy(raw)
	switch policy {
	case InvocationInteractive,
		InvocationExplicit,
		InvocationManifestReconcile:
		return policy, nil
	default:
		return "", fmt.Errorf("initialization invocation policy is required and must be closed")
	}
}

type HostID string

const (
	HostClaude      HostID = "claude"
	HostCodex       HostID = "codex"
	HostCursor      HostID = "cursor"
	HostGemini      HostID = "gemini"
	HostAntigravity HostID = "antigravity"
	HostGrok        HostID = "grok"
	HostHermes      HostID = "hermes"
	HostZed         HostID = "zed"
	HostOpenCode    HostID = "opencode"
	HostAir         HostID = "air"
	HostPi          HostID = "pi"
)

var knownHosts = map[HostID]struct{}{
	HostClaude:      {},
	HostCodex:       {},
	HostCursor:      {},
	HostGemini:      {},
	HostAntigravity: {},
	HostGrok:        {},
	HostHermes:      {},
	HostZed:         {},
	HostOpenCode:    {},
	HostAir:         {},
	HostPi:          {},
}

func ParseHostID(raw string) (HostID, error) {
	host := HostID(raw)
	_, known := knownHosts[host]
	if raw != strings.TrimSpace(raw) || !known {
		return "", fmt.Errorf("unknown canonical host ID %q", raw)
	}
	return host, nil
}

type InstallScope string

const (
	ScopeProject InstallScope = "project"
	ScopeUser    InstallScope = "user"
)

func parseInstallScope(raw string) (InstallScope, error) {
	scope := InstallScope(raw)
	switch scope {
	case ScopeProject, ScopeUser:
		return scope, nil
	default:
		return "", fmt.Errorf("host install scope must be project or user")
	}
}

type Component string

const (
	ComponentMCP          Component = "mcp"
	ComponentSkills       Component = "skills"
	ComponentInstructions Component = "instructions"
	ComponentHooks        Component = "hooks"
	ComponentPackage      Component = "package"
)

var knownComponents = map[Component]struct{}{
	ComponentMCP:          {},
	ComponentSkills:       {},
	ComponentInstructions: {},
	ComponentHooks:        {},
	ComponentPackage:      {},
}

type ComponentSet struct {
	values []Component
}

func ParseComponentSet(raw []string) (ComponentSet, error) {
	if len(raw) == 0 {
		return ComponentSet{}, fmt.Errorf("host component set cannot be empty")
	}
	seen := make(map[Component]struct{}, len(raw))
	values := make([]Component, 0, len(raw))
	for _, candidate := range raw {
		component := Component(candidate)
		_, known := knownComponents[component]
		if candidate != strings.TrimSpace(candidate) || !known {
			return ComponentSet{}, fmt.Errorf("unknown canonical host component %q", candidate)
		}
		if _, duplicate := seen[component]; duplicate {
			return ComponentSet{}, fmt.Errorf("host component %q is repeated", component)
		}
		seen[component] = struct{}{}
		values = append(values, component)
	}
	sort.Slice(values, func(left int, right int) bool {
		return values[left] < values[right]
	})
	return ComponentSet{values: values}, nil
}

func (set ComponentSet) Values() []Component {
	return slices.Clone(set.values)
}

func (set ComponentSet) contains(component Component) bool {
	return slices.Contains(set.values, component)
}

func (set ComponentSet) equal(other ComponentSet) bool {
	return slices.Equal(set.values, other.values)
}

func (set ComponentSet) single() (Component, bool) {
	if len(set.values) != 1 {
		return "", false
	}
	return set.values[0], true
}

func singletonComponentSet(component Component) (ComponentSet, error) {
	return ParseComponentSet([]string{string(component)})
}

func validateComponentSet(set ComponentSet) error {
	raw := make([]string, len(set.values))
	for index, component := range set.values {
		raw[index] = string(component)
	}
	canonical, err := ParseComponentSet(raw)
	if err != nil {
		return err
	}
	if !set.equal(canonical) {
		return fmt.Errorf("host component set is not canonical")
	}
	return nil
}

type WeakHostSelection struct {
	Host       string
	Scope      string
	Components []string
}

type HostSelection struct {
	host       HostID
	scope      InstallScope
	components ComponentSet
}

type HostBindingID struct {
	host  HostID
	scope InstallScope
}

func NewHostBindingID(
	host HostID,
	scope InstallScope,
) (HostBindingID, error) {
	if _, err := ParseHostID(string(host)); err != nil {
		return HostBindingID{}, err
	}
	if scope != ScopeProject && scope != ScopeUser {
		return HostBindingID{}, fmt.Errorf(
			"host binding scope must be project or user",
		)
	}
	return HostBindingID{
		host:  host,
		scope: scope,
	}, nil
}

func (binding HostBindingID) Host() HostID {
	return binding.host
}

func (binding HostBindingID) Scope() InstallScope {
	return binding.scope
}

func (binding HostBindingID) String() string {
	return string(binding.host) + "/" + string(binding.scope)
}

func parseHostSelection(input WeakHostSelection) (HostSelection, error) {
	host, err := ParseHostID(input.Host)
	if err != nil {
		return HostSelection{}, err
	}
	scope, err := parseInstallScope(input.Scope)
	if err != nil {
		return HostSelection{}, fmt.Errorf("host %s: %w", host, err)
	}
	components, err := ParseComponentSet(input.Components)
	if err != nil {
		return HostSelection{}, fmt.Errorf("host %s: %w", host, err)
	}
	return HostSelection{
		host:       host,
		scope:      scope,
		components: components,
	}, nil
}

func (selection HostSelection) Host() HostID {
	return selection.host
}

func (selection HostSelection) Scope() InstallScope {
	return selection.scope
}

func (selection HostSelection) Components() ComponentSet {
	return ComponentSet{values: selection.components.Values()}
}

func (selection HostSelection) BindingID() HostBindingID {
	return HostBindingID{
		host:  selection.host,
		scope: selection.scope,
	}
}

type SelectedHostSet struct {
	values []HostSelection
}

func parseSelectedHostSet(raw []WeakHostSelection) (SelectedHostSet, error) {
	seen := make(map[HostBindingID]struct{}, len(raw))
	values := make([]HostSelection, 0, len(raw))
	for _, input := range raw {
		selection, err := parseHostSelection(input)
		if err != nil {
			return SelectedHostSet{}, err
		}
		binding := selection.BindingID()
		if _, duplicate := seen[binding]; duplicate {
			return SelectedHostSet{}, fmt.Errorf(
				"host binding %s is selected more than once",
				binding.String(),
			)
		}
		seen[binding] = struct{}{}
		values = append(values, selection)
	}
	sort.Slice(values, func(left int, right int) bool {
		leftBinding := values[left].BindingID()
		rightBinding := values[right].BindingID()
		if leftBinding.host != rightBinding.host {
			return leftBinding.host < rightBinding.host
		}
		return leftBinding.scope < rightBinding.scope
	})
	return SelectedHostSet{values: values}, nil
}

func (set SelectedHostSet) Values() []HostSelection {
	result := make([]HostSelection, len(set.values))
	for index, selection := range set.values {
		result[index] = HostSelection{
			host:       selection.host,
			scope:      selection.scope,
			components: selection.Components(),
		}
	}
	return result
}

type WeakInitIntent struct {
	InvocationPolicy string
	ProjectRoot      string
	ProjectID        string
	Hosts            []WeakHostSelection
}

type InitIntent struct {
	policy      InvocationPolicy
	projectRoot string
	projectID   projectidentity.ProjectID
	hosts       SelectedHostSet
}

func ParseInitIntent(input WeakInitIntent) (InitIntent, error) {
	policy, err := parseInvocationPolicy(input.InvocationPolicy)
	if err != nil {
		return InitIntent{}, err
	}
	projectRoot, err := parseCanonicalAbsolutePath(input.ProjectRoot)
	if err != nil {
		return InitIntent{}, fmt.Errorf("parse project root: %w", err)
	}
	projectID, err := projectidentity.ParseProjectID(input.ProjectID)
	if err != nil {
		return InitIntent{}, fmt.Errorf("parse project identity: %w", err)
	}
	hosts, err := parseSelectedHostSet(input.Hosts)
	if err != nil {
		return InitIntent{}, err
	}
	return InitIntent{
		policy:      policy,
		projectRoot: projectRoot,
		projectID:   projectID,
		hosts:       hosts,
	}, nil
}

func (intent InitIntent) InvocationPolicy() InvocationPolicy {
	return intent.policy
}

func (intent InitIntent) ProjectRoot() string {
	return intent.projectRoot
}

func (intent InitIntent) ProjectID() projectidentity.ProjectID {
	return intent.projectID
}

func (intent InitIntent) SelectedHosts() SelectedHostSet {
	return SelectedHostSet{values: intent.hosts.Values()}
}

func parseCanonicalAbsolutePath(raw string) (string, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("path is required in exact form")
	}
	if !filepath.IsAbs(raw) || filepath.Clean(raw) != raw {
		return "", fmt.Errorf("path must be canonical and absolute")
	}
	volumeRoot := filepath.VolumeName(raw) + string(filepath.Separator)
	if raw == volumeRoot {
		return "", fmt.Errorf("filesystem root cannot be an initialization target")
	}
	return raw, nil
}

type DiscoveryObservation struct {
	host    HostID
	basis   string
	posture DiscoveryPosture
}

type DiscoveryPosture string

const (
	DiscoveryDetected    DiscoveryPosture = "detected"
	DiscoveryNotDetected DiscoveryPosture = "not_detected"
)

func NewDiscoveryObservation(
	host HostID,
	basis string,
	posture DiscoveryPosture,
) (DiscoveryObservation, error) {
	if _, known := knownHosts[host]; !known {
		return DiscoveryObservation{}, fmt.Errorf("discovery host is not canonical")
	}
	if basis == "" || basis != strings.TrimSpace(basis) {
		return DiscoveryObservation{}, fmt.Errorf("discovery basis is required")
	}
	if posture != DiscoveryDetected && posture != DiscoveryNotDetected {
		return DiscoveryObservation{}, fmt.Errorf("discovery posture is not closed")
	}
	return DiscoveryObservation{
		host:    host,
		basis:   basis,
		posture: posture,
	}, nil
}

func (observation DiscoveryObservation) Host() HostID {
	return observation.host
}

func (observation DiscoveryObservation) Basis() string {
	return observation.basis
}

func (observation DiscoveryObservation) Posture() DiscoveryPosture {
	return observation.posture
}
