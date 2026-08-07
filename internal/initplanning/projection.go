package initplanning

import (
	"fmt"
	"slices"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
)

type HostAdapterProjection struct {
	host        HostID
	edition     string
	publication PublicationIdentity
	projectRoot string
	projectID   projectidentity.ProjectID
	scope       InstallScope
	components  ComponentSet
	targetRoots []string
	outputs     []RenderedOutput
	fragments   []ManagedFragment
	recovery    RecoveryOperation
}

type HostAdapterProjectionBuilder struct {
	host        HostID
	edition     string
	publication PublicationIdentity
	projectRoot string
	projectID   string
	scope       InstallScope
	components  ComponentSet
	targetRoots []string
	outputs     []RenderedOutput
	fragments   []ManagedFragment
	recovery    RecoveryOperation
}

func NewHostAdapterProjectionBuilder(host HostID) HostAdapterProjectionBuilder {
	return HostAdapterProjectionBuilder{host: host}
}

func (builder HostAdapterProjectionBuilder) AtEdition(
	edition string,
) HostAdapterProjectionBuilder {
	next := builder
	next.edition = edition
	return next
}

func (builder HostAdapterProjectionBuilder) PublishedFrom(
	publication PublicationIdentity,
) HostAdapterProjectionBuilder {
	next := builder
	next.publication = publication
	return next
}

func (builder HostAdapterProjectionBuilder) ForProject(
	root string,
	projectID string,
) HostAdapterProjectionBuilder {
	next := builder
	next.projectRoot = root
	next.projectID = projectID
	return next
}

func (builder HostAdapterProjectionBuilder) WithSelection(
	scope InstallScope,
	components ComponentSet,
) HostAdapterProjectionBuilder {
	next := builder
	next.scope = scope
	next.components = ComponentSet{values: components.Values()}
	return next
}

func (builder HostAdapterProjectionBuilder) AddTargetRoot(
	root string,
) HostAdapterProjectionBuilder {
	next := builder
	next.targetRoots = appendCopy(builder.targetRoots, root)
	return next
}

func (builder HostAdapterProjectionBuilder) AddOutput(
	output RenderedOutput,
) HostAdapterProjectionBuilder {
	next := builder
	next.outputs = appendCopy(builder.outputs, output)
	return next
}

func (builder HostAdapterProjectionBuilder) AddManagedFragment(
	fragment ManagedFragment,
) HostAdapterProjectionBuilder {
	next := builder
	next.fragments = appendCopy(builder.fragments, cloneManagedFragment(fragment))
	return next
}

func (builder HostAdapterProjectionBuilder) RecoverWith(
	operation RecoveryOperation,
) HostAdapterProjectionBuilder {
	next := builder
	next.recovery = RecoveryOperation{argv: operation.Argv()}
	return next
}

func (builder HostAdapterProjectionBuilder) Build() (HostAdapterProjection, error) {
	if _, known := knownHosts[builder.host]; !known {
		return HostAdapterProjection{}, fmt.Errorf("host adapter projection host is not canonical")
	}
	if !adapterEditionPattern.MatchString(builder.edition) {
		return HostAdapterProjection{}, fmt.Errorf("host adapter projection edition is invalid")
	}
	if !builder.publication.valid() {
		return HostAdapterProjection{}, fmt.Errorf("host adapter projection publication identity is invalid")
	}
	projectRoot, err := parseCanonicalAbsolutePath(builder.projectRoot)
	if err != nil {
		return HostAdapterProjection{}, fmt.Errorf("host adapter projection project root: %w", err)
	}
	projectID, err := projectidentity.ParseProjectID(builder.projectID)
	if err != nil {
		return HostAdapterProjection{}, fmt.Errorf("host adapter projection project identity: %w", err)
	}
	if builder.scope != ScopeProject && builder.scope != ScopeUser {
		return HostAdapterProjection{}, fmt.Errorf("host adapter projection scope is invalid")
	}
	if len(builder.components.values) == 0 {
		return HostAdapterProjection{}, fmt.Errorf("host adapter projection component set is empty")
	}
	targetRoots, err := canonicalTargetRoots(builder.targetRoots)
	if err != nil {
		return HostAdapterProjection{}, err
	}
	outputs, err := validateProjectionOutputs(
		builder.outputs,
		builder.components,
		targetRoots,
	)
	if err != nil {
		return HostAdapterProjection{}, err
	}
	fragments, err := validateProjectionManagedFragments(
		builder.fragments,
		builder.components,
		targetRoots,
		outputs,
	)
	if err != nil {
		return HostAdapterProjection{}, err
	}
	if len(outputs) == 0 && len(fragments) == 0 {
		return HostAdapterProjection{}, fmt.Errorf(
			"host adapter projection has no managed output or fragment",
		)
	}
	if len(builder.recovery.argv) == 0 {
		return HostAdapterProjection{}, fmt.Errorf("host adapter projection lacks a recovery operation")
	}
	return HostAdapterProjection{
		host:        builder.host,
		edition:     builder.edition,
		publication: builder.publication,
		projectRoot: projectRoot,
		projectID:   projectID,
		scope:       builder.scope,
		components:  ComponentSet{values: builder.components.Values()},
		targetRoots: slices.Clone(targetRoots),
		outputs:     outputs,
		fragments:   fragments,
		recovery:    RecoveryOperation{argv: builder.recovery.Argv()},
	}, nil
}

func validateProjectionOutputs(
	raw []RenderedOutput,
	components ComponentSet,
	targetRoots []string,
) ([]RenderedOutput, error) {
	outputs := cloneRenderedOutputs(raw)
	sort.Slice(outputs, func(left int, right int) bool {
		return outputs[left].path < outputs[right].path
	})
	previous := ""
	for _, output := range outputs {
		if output.path == "" || !sha256DigestPattern.MatchString(output.digest) {
			return nil, fmt.Errorf("host adapter projection output is invalid")
		}
		component, singleton := output.components.single()
		if !singleton {
			return nil, fmt.Errorf(
				"host adapter projection whole-file output %s must name exactly one component",
				output.path,
			)
		}
		if output.path == previous {
			return nil, fmt.Errorf("host adapter projection repeats path %s", output.path)
		}
		if !components.contains(component) {
			return nil, fmt.Errorf(
				"host adapter projection path %s uses unselected component %s",
				output.path,
				component,
			)
		}
		if !pathWithinAnyRoot(output.path, targetRoots) {
			return nil, fmt.Errorf("host adapter projection path %s is outside target roots", output.path)
		}
		previous = output.path
	}
	return outputs, nil
}

func validateProjectionManagedFragments(
	raw []ManagedFragment,
	components ComponentSet,
	targetRoots []string,
	outputs []RenderedOutput,
) ([]ManagedFragment, error) {
	fragments, err := canonicalDesiredManagedFragments(raw)
	if err != nil {
		return nil, fmt.Errorf("host adapter projection managed fragments: %w", err)
	}
	wholePaths := make(map[string]struct{}, len(outputs))
	for _, output := range outputs {
		wholePaths[output.path] = struct{}{}
	}
	groups := make(map[string][]ManagedFragment)
	for _, fragment := range fragments {
		path := fragment.coordinate.carrierPath
		if !components.contains(fragment.component) {
			return nil, fmt.Errorf(
				"host adapter projection fragment %s uses unselected component %s",
				fragment.coordinate.selector,
				fragment.component,
			)
		}
		if !pathWithinAnyRoot(path, targetRoots) {
			return nil, fmt.Errorf(
				"host adapter projection fragment carrier %s is outside target roots",
				path,
			)
		}
		if _, wholeOwned := wholePaths[path]; wholeOwned {
			return nil, fmt.Errorf(
				"host adapter projection cannot own whole carrier and fragment at %s",
				path,
			)
		}
		groups[path] = append(groups[path], cloneManagedFragment(fragment))
	}
	for _, group := range groups {
		_, _, _, _, _, err := validateManagedFragmentGroup(group, nil, nil)
		if err != nil {
			return nil, fmt.Errorf(
				"host adapter projection fragment group: %w",
				err,
			)
		}
	}
	return fragments, nil
}

func cloneHostAdapterProjection(
	projection HostAdapterProjection,
) HostAdapterProjection {
	return HostAdapterProjection{
		host:        projection.host,
		edition:     projection.edition,
		publication: projection.publication,
		projectRoot: projection.projectRoot,
		projectID:   projection.projectID,
		scope:       projection.scope,
		components:  ComponentSet{values: projection.components.Values()},
		targetRoots: slices.Clone(projection.targetRoots),
		outputs:     cloneRenderedOutputs(projection.outputs),
		fragments:   cloneManagedFragments(projection.fragments),
		recovery:    RecoveryOperation{argv: projection.recovery.Argv()},
	}
}

func (projection HostAdapterProjection) Host() HostID {
	return projection.host
}

func (projection HostAdapterProjection) Edition() string {
	return projection.edition
}

func (projection HostAdapterProjection) Publication() PublicationIdentity {
	return projection.publication
}

func (projection HostAdapterProjection) ProjectRoot() string {
	return projection.projectRoot
}

func (projection HostAdapterProjection) ProjectID() projectidentity.ProjectID {
	return projection.projectID
}

func (projection HostAdapterProjection) Scope() InstallScope {
	return projection.scope
}

func (projection HostAdapterProjection) Components() ComponentSet {
	return ComponentSet{values: projection.components.Values()}
}

func (projection HostAdapterProjection) TargetRoots() []string {
	return slices.Clone(projection.targetRoots)
}

func (projection HostAdapterProjection) Outputs() []RenderedOutput {
	return cloneRenderedOutputs(projection.outputs)
}

func (projection HostAdapterProjection) ManagedFragments() []ManagedFragment {
	return cloneManagedFragments(projection.fragments)
}

func (projection HostAdapterProjection) Recovery() RecoveryOperation {
	return RecoveryOperation{argv: projection.recovery.Argv()}
}
