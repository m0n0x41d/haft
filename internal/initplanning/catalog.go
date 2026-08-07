package initplanning

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var adapterEditionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

type scopeCapability struct {
	scope      InstallScope
	components ComponentSet
}

type AdapterCapability struct {
	host     HostID
	edition  string
	variants []scopeCapability
}

type AdapterCapabilityBuilder struct {
	host     HostID
	edition  string
	variants []scopeCapability
}

func NewAdapterCapabilityBuilder(host HostID) AdapterCapabilityBuilder {
	return AdapterCapabilityBuilder{host: host}
}

func (builder AdapterCapabilityBuilder) AtEdition(edition string) AdapterCapabilityBuilder {
	next := builder
	next.edition = edition
	return next
}

func (builder AdapterCapabilityBuilder) Allow(
	scope InstallScope,
	components ComponentSet,
) AdapterCapabilityBuilder {
	next := builder
	next.variants = appendCopy(
		builder.variants,
		scopeCapability{
			scope:      scope,
			components: ComponentSet{values: components.Values()},
		},
	)
	return next
}

func (builder AdapterCapabilityBuilder) Build() (AdapterCapability, error) {
	if _, known := knownHosts[builder.host]; !known {
		return AdapterCapability{}, fmt.Errorf("adapter capability host is not canonical")
	}
	if !adapterEditionPattern.MatchString(builder.edition) {
		return AdapterCapability{}, fmt.Errorf("adapter edition is not canonical")
	}
	if len(builder.variants) == 0 {
		return AdapterCapability{}, fmt.Errorf("adapter capability needs at least one scope variant")
	}
	seen := make(map[InstallScope]struct{}, len(builder.variants))
	variants := make([]scopeCapability, 0, len(builder.variants))
	for _, variant := range builder.variants {
		if variant.scope != ScopeProject && variant.scope != ScopeUser {
			return AdapterCapability{}, fmt.Errorf("adapter capability has an invalid scope")
		}
		if len(variant.components.values) == 0 {
			return AdapterCapability{}, fmt.Errorf("adapter capability scope %s has no components", variant.scope)
		}
		if _, duplicate := seen[variant.scope]; duplicate {
			return AdapterCapability{}, fmt.Errorf("adapter capability repeats scope %s", variant.scope)
		}
		seen[variant.scope] = struct{}{}
		variants = append(variants, scopeCapability{
			scope:      variant.scope,
			components: ComponentSet{values: variant.components.Values()},
		})
	}
	sort.Slice(variants, func(left int, right int) bool {
		return variants[left].scope < variants[right].scope
	})
	return AdapterCapability{
		host:     builder.host,
		edition:  builder.edition,
		variants: variants,
	}, nil
}

func (capability AdapterCapability) Host() HostID {
	return capability.host
}

func (capability AdapterCapability) Edition() string {
	return capability.edition
}

func (capability AdapterCapability) validate(selection HostSelection) error {
	if selection.host != capability.host {
		return fmt.Errorf(
			"adapter capability %s cannot validate host %s",
			capability.host,
			selection.host,
		)
	}
	for _, variant := range capability.variants {
		if variant.scope != selection.scope {
			continue
		}
		for _, component := range selection.components.values {
			if !variant.components.contains(component) {
				return fmt.Errorf(
					"host %s does not support component %s at %s scope",
					selection.host,
					component,
					selection.scope,
				)
			}
		}
		return nil
	}
	return fmt.Errorf(
		"host %s does not support %s scope",
		selection.host,
		selection.scope,
	)
}

type AdapterCatalog struct {
	capabilities map[HostBindingID]AdapterCapability
}

func NewAdapterCatalog(capabilities []AdapterCapability) (AdapterCatalog, error) {
	if len(capabilities) == 0 {
		return AdapterCatalog{}, fmt.Errorf("adapter catalog cannot be empty")
	}
	result := make(map[HostBindingID]AdapterCapability, len(capabilities))
	for _, capability := range capabilities {
		if capability.host == "" || capability.edition == "" || len(capability.variants) == 0 {
			return AdapterCatalog{}, fmt.Errorf("adapter catalog contains an invalid capability")
		}
		for _, variant := range capability.variants {
			binding, err := NewHostBindingID(
				capability.host,
				variant.scope,
			)
			if err != nil {
				return AdapterCatalog{}, err
			}
			if _, duplicate := result[binding]; duplicate {
				return AdapterCatalog{}, fmt.Errorf(
					"adapter catalog repeats host binding %s",
					binding.String(),
				)
			}
			result[binding] = cloneAdapterCapability(capability)
		}
	}
	return AdapterCatalog{capabilities: result}, nil
}

func (catalog AdapterCatalog) validate(selection HostSelection) error {
	binding := selection.BindingID()
	capability, ok := catalog.capabilities[binding]
	if !ok {
		if catalog.containsHost(selection.host) {
			return fmt.Errorf(
				"host %s does not support %s scope",
				selection.host,
				selection.scope,
			)
		}
		return fmt.Errorf(
			"selected host binding %s has no adapter capability",
			binding.String(),
		)
	}
	return capability.validate(selection)
}

func (catalog AdapterCatalog) containsHost(host HostID) bool {
	for binding := range catalog.capabilities {
		if binding.host == host {
			return true
		}
	}
	return false
}

func (catalog AdapterCatalog) edition(
	binding HostBindingID,
) (string, error) {
	capability, ok := catalog.capabilities[binding]
	if !ok {
		return "", fmt.Errorf(
			"selected host binding %s has no adapter capability",
			binding.String(),
		)
	}
	return capability.edition, nil
}

func cloneAdapterCapability(capability AdapterCapability) AdapterCapability {
	variants := make([]scopeCapability, len(capability.variants))
	for index, variant := range capability.variants {
		variants[index] = scopeCapability{
			scope:      variant.scope,
			components: ComponentSet{values: variant.components.Values()},
		}
	}
	return AdapterCapability{
		host:     capability.host,
		edition:  capability.edition,
		variants: variants,
	}
}

func appendCopy[T any](source []T, value T) []T {
	result := make([]T, len(source), len(source)+1)
	copy(result, source)
	return append(result, value)
}

func validateReason(raw string, label string) (string, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("%s is required in exact form", label)
	}
	return raw, nil
}
