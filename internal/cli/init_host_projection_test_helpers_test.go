package cli

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func buildCurrentCoherentHostProjection(
	projectRoot string,
	projectID string,
	host initplanning.HostID,
	scope initplanning.InstallScope,
	candidates []currentStandardSkillCandidate,
	bundle initplanning.SkillSourceBundle,
	publication initplanning.PublicationIdentity,
	runtime currentHostPublicationRuntime,
) (initplanning.HostAdapterProjection, error) {
	key := currentCoherentHostKey{host: host, scope: scope}
	applicabilityRegistry, err :=
		currentCoherentHostApplicabilityRegistry()
	if err != nil {
		return initplanning.HostAdapterProjection{}, err
	}
	applicability, available := applicabilityRegistry[key]
	if !available {
		return initplanning.HostAdapterProjection{}, fmt.Errorf(
			"host %s has no coherent %s projection component applicability",
			host,
			scope,
		)
	}
	components, err := parseCurrentCoherentComponents(
		currentIncludedComponents(applicability),
	)
	if err != nil {
		return initplanning.HostAdapterProjection{}, err
	}
	return buildSelectedCurrentCoherentHostProjection(
		projectRoot,
		projectID,
		host,
		scope,
		components,
		candidates,
		bundle,
		publication,
		runtime,
	)
}

func currentIncludedComponents(
	applicability initplanning.HostComponentApplicability,
) []initplanning.Component {
	components := make([]initplanning.Component, 0)
	for _, record := range applicability.Records() {
		if record.Disposition != initplanning.ComponentIncluded {
			continue
		}
		components = append(components, record.Component)
	}
	return components
}
