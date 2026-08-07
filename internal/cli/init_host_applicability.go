package cli

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

type currentHostApplicabilityDefinition struct {
	key                  currentCoherentHostKey
	included             []initplanning.Component
	representedByPackage []initplanning.Component
}

func currentCoherentHostApplicabilityRegistry() (
	map[currentCoherentHostKey]initplanning.HostComponentApplicability,
	error,
) {
	definitions := []currentHostApplicabilityDefinition{
		currentStandardHostApplicability(
			initplanning.HostClaude,
			initplanning.ScopeProject,
			true,
		),
		currentStandardHostApplicability(
			initplanning.HostCursor,
			initplanning.ScopeProject,
			false,
		),
		currentStandardHostApplicability(
			initplanning.HostCodex,
			initplanning.ScopeProject,
			true,
		),
		currentStandardHostApplicability(
			initplanning.HostOpenCode,
			initplanning.ScopeProject,
			false,
		),
		currentStandardHostApplicability(
			initplanning.HostGrok,
			initplanning.ScopeProject,
			true,
		),
		currentStandardHostApplicability(
			initplanning.HostAir,
			initplanning.ScopeProject,
			false,
		),
		currentStandardHostApplicability(
			initplanning.HostAntigravity,
			initplanning.ScopeUser,
			false,
		),
		{
			key: currentCoherentHostKey{
				host:  initplanning.HostGemini,
				scope: initplanning.ScopeUser,
			},
			included: []initplanning.Component{
				initplanning.ComponentMCP,
			},
		},
		{
			key: currentCoherentHostKey{
				host:  initplanning.HostZed,
				scope: initplanning.ScopeUser,
			},
			included: []initplanning.Component{
				initplanning.ComponentMCP,
			},
		},
		{
			key: currentCoherentHostKey{
				host:  initplanning.HostPi,
				scope: initplanning.ScopeProject,
			},
			included: []initplanning.Component{
				initplanning.ComponentPackage,
			},
			representedByPackage: []initplanning.Component{
				initplanning.ComponentInstructions,
				initplanning.ComponentMCP,
				initplanning.ComponentSkills,
			},
		},
		currentStandardHostApplicability(
			initplanning.HostHermes,
			initplanning.ScopeUser,
			false,
		),
	}
	result := make(
		map[currentCoherentHostKey]initplanning.HostComponentApplicability,
		len(definitions),
	)
	for _, definition := range definitions {
		if _, duplicate := result[definition.key]; duplicate {
			return nil, fmt.Errorf(
				"coherent host applicability repeats %s/%s",
				definition.key.host,
				definition.key.scope,
			)
		}
		inputs := currentComponentApplicabilityInputs(definition)
		applicability, err := initplanning.NewHostComponentApplicability(
			definition.key.host,
			definition.key.scope,
			inputs,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"build %s/%s component applicability: %w",
				definition.key.host,
				definition.key.scope,
				err,
			)
		}
		result[definition.key] = applicability
	}
	return result, nil
}

func currentStandardHostApplicability(
	host initplanning.HostID,
	scope initplanning.InstallScope,
	instructions bool,
) currentHostApplicabilityDefinition {
	included := []initplanning.Component{
		initplanning.ComponentMCP,
		initplanning.ComponentSkills,
	}
	if instructions {
		included = append(
			included,
			initplanning.ComponentInstructions,
		)
	}
	return currentHostApplicabilityDefinition{
		key: currentCoherentHostKey{
			host:  host,
			scope: scope,
		},
		included: included,
	}
}

func currentComponentApplicabilityInputs(
	definition currentHostApplicabilityDefinition,
) []initplanning.ComponentApplicabilityInput {
	components := []initplanning.Component{
		initplanning.ComponentHooks,
		initplanning.ComponentInstructions,
		initplanning.ComponentMCP,
		initplanning.ComponentPackage,
		initplanning.ComponentSkills,
	}
	result := make(
		[]initplanning.ComponentApplicabilityInput,
		0,
		len(components),
	)
	for _, component := range components {
		disposition := initplanning.ComponentUnavailable
		basis := "no registered carrier in this coherent host projection"
		switch {
		case component == initplanning.ComponentHooks:
			disposition = initplanning.ComponentSeparateOptIn
			basis = "overseer post-commit hook is available only through explicit `haft overseer init` or `haft init --overseer`"
		case slices.Contains(definition.included, component):
			disposition = initplanning.ComponentIncluded
			basis = "included by the registered coherent host projection"
		case slices.Contains(
			definition.representedByPackage,
			component,
		):
			disposition = initplanning.ComponentRepresentedByPackage
			basis = "represented inside the source-pinned Pi package under its controlled-coarsening declaration"
		}
		result = append(
			result,
			initplanning.ComponentApplicabilityInput{
				Component:   component,
				Disposition: disposition,
				Basis:       basis,
			},
		)
	}
	return result
}
