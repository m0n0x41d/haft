package initplanning

import (
	"fmt"
	"slices"
	"sort"
)

// ComponentDisposition states how one canonical component relates to one
// coherent host/scope projection. It is inventory metadata, not an install
// effect or an authority grant.
type ComponentDisposition string

const (
	ComponentIncluded             ComponentDisposition = "included"
	ComponentRepresentedByPackage ComponentDisposition = "represented_by_package"
	ComponentSeparateOptIn        ComponentDisposition = "separate_opt_in_capability"
	ComponentUnavailable          ComponentDisposition = "unavailable"
)

type ComponentApplicabilityInput struct {
	Component   Component
	Disposition ComponentDisposition
	Basis       string
}

type ComponentApplicabilityRecord struct {
	Component   Component
	Disposition ComponentDisposition
	Basis       string
}

type HostComponentApplicability struct {
	host    HostID
	scope   InstallScope
	records []ComponentApplicabilityRecord
}

func NewHostComponentApplicability(
	host HostID,
	scope InstallScope,
	inputs []ComponentApplicabilityInput,
) (HostComponentApplicability, error) {
	if _, known := knownHosts[host]; !known {
		return HostComponentApplicability{}, fmt.Errorf(
			"host component applicability host is not canonical",
		)
	}
	if scope != ScopeProject && scope != ScopeUser {
		return HostComponentApplicability{}, fmt.Errorf(
			"host component applicability scope is invalid",
		)
	}
	records := make(
		[]ComponentApplicabilityRecord,
		0,
		len(inputs),
	)
	seen := make(map[Component]struct{}, len(inputs))
	for _, input := range inputs {
		if _, known := knownComponents[input.Component]; !known {
			return HostComponentApplicability{}, fmt.Errorf(
				"host component applicability component %q is not canonical",
				input.Component,
			)
		}
		if _, duplicate := seen[input.Component]; duplicate {
			return HostComponentApplicability{}, fmt.Errorf(
				"host component applicability repeats component %s",
				input.Component,
			)
		}
		if !validComponentDisposition(input.Disposition) {
			return HostComponentApplicability{}, fmt.Errorf(
				"host component applicability disposition for %s is invalid",
				input.Component,
			)
		}
		basis, err := validateReason(
			input.Basis,
			"host component applicability basis",
		)
		if err != nil {
			return HostComponentApplicability{}, err
		}
		records = append(
			records,
			ComponentApplicabilityRecord{
				Component:   input.Component,
				Disposition: input.Disposition,
				Basis:       basis,
			},
		)
		seen[input.Component] = struct{}{}
	}
	if len(records) != len(knownComponents) {
		return HostComponentApplicability{}, fmt.Errorf(
			"host component applicability inventory is incomplete",
		)
	}
	sort.Slice(records, func(left int, right int) bool {
		return records[left].Component < records[right].Component
	})
	applicability := HostComponentApplicability{
		host:    host,
		scope:   scope,
		records: records,
	}
	if err := applicability.validatePackageRepresentation(); err != nil {
		return HostComponentApplicability{}, err
	}
	return applicability, nil
}

func validComponentDisposition(disposition ComponentDisposition) bool {
	switch disposition {
	case ComponentIncluded,
		ComponentRepresentedByPackage,
		ComponentSeparateOptIn,
		ComponentUnavailable:
		return true
	default:
		return false
	}
}

func (
	applicability HostComponentApplicability,
) validatePackageRepresentation() error {
	packageRecord, found := applicability.Record(ComponentPackage)
	if !found {
		return fmt.Errorf(
			"host component applicability lacks package disposition",
		)
	}
	for _, record := range applicability.records {
		if record.Disposition != ComponentRepresentedByPackage {
			continue
		}
		if record.Component == ComponentPackage {
			return fmt.Errorf(
				"package component cannot be represented by itself",
			)
		}
		if packageRecord.Disposition != ComponentIncluded {
			return fmt.Errorf(
				"component %s representation lacks an included package",
				record.Component,
			)
		}
	}
	return nil
}

func (
	applicability HostComponentApplicability,
) ValidateSelection(selected ComponentSet) error {
	if err := applicability.ValidateSupportedSelection(selected); err != nil {
		return err
	}
	for _, record := range applicability.records {
		if record.Disposition != ComponentIncluded {
			continue
		}
		if selected.contains(record.Component) {
			continue
		}
		return fmt.Errorf(
			"included component %s is absent from the projection",
			record.Component,
		)
	}
	return nil
}

func (
	applicability HostComponentApplicability,
) ValidateSupportedSelection(selected ComponentSet) error {
	if err := validateComponentSet(selected); err != nil {
		return fmt.Errorf(
			"host component applicability selection: %w",
			err,
		)
	}
	for _, record := range applicability.records {
		isSelected := selected.contains(record.Component)
		supported := record.Disposition == ComponentIncluded
		if isSelected && !supported {
			return fmt.Errorf(
				"selected component %s has disposition %s",
				record.Component,
				record.Disposition,
			)
		}
	}
	return nil
}

func (
	applicability HostComponentApplicability,
) RequiresControlledCoarsening() bool {
	for _, record := range applicability.records {
		if record.Disposition == ComponentRepresentedByPackage {
			return true
		}
	}
	return false
}

func (
	applicability HostComponentApplicability,
) Record(component Component) (ComponentApplicabilityRecord, bool) {
	for _, record := range applicability.records {
		if record.Component == component {
			return record, true
		}
	}
	return ComponentApplicabilityRecord{}, false
}

func (applicability HostComponentApplicability) Host() HostID {
	return applicability.host
}

func (applicability HostComponentApplicability) Scope() InstallScope {
	return applicability.scope
}

func (
	applicability HostComponentApplicability,
) Records() []ComponentApplicabilityRecord {
	return slices.Clone(applicability.records)
}
