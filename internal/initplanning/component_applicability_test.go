package initplanning

import (
	"slices"
	"strings"
	"testing"
)

func TestHostComponentApplicabilityRequiresOneDispositionPerComponent(
	t *testing.T,
) {
	records := completeApplicabilityInputs()
	records = records[:len(records)-1]
	_, err := NewHostComponentApplicability(
		HostClaude,
		ScopeProject,
		records,
	)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete applicability accepted: %v", err)
	}

	records = completeApplicabilityInputs()
	records = append(records, records[0])
	_, err = NewHostComponentApplicability(
		HostClaude,
		ScopeProject,
		records,
	)
	if err == nil || !strings.Contains(err.Error(), "repeats") {
		t.Fatalf("duplicate applicability accepted: %v", err)
	}
}

func TestHostComponentApplicabilityMatchesExactSelectedComponents(
	t *testing.T,
) {
	applicability, err := NewHostComponentApplicability(
		HostPi,
		ScopeProject,
		[]ComponentApplicabilityInput{
			{
				Component:   ComponentHooks,
				Disposition: ComponentSeparateOptIn,
				Basis:       "haft overseer init",
			},
			{
				Component:   ComponentInstructions,
				Disposition: ComponentRepresentedByPackage,
				Basis:       "pi package controlled coarsening",
			},
			{
				Component:   ComponentMCP,
				Disposition: ComponentRepresentedByPackage,
				Basis:       "pi package native kernel bridge",
			},
			{
				Component:   ComponentPackage,
				Disposition: ComponentIncluded,
				Basis:       "coherent Pi package projection",
			},
			{
				Component:   ComponentSkills,
				Disposition: ComponentRepresentedByPackage,
				Basis:       "pi package controlled coarsening",
			},
		},
	)
	if err != nil {
		t.Fatalf("NewHostComponentApplicability: %v", err)
	}
	selected, err := ParseComponentSet([]string{string(ComponentPackage)})
	if err != nil {
		t.Fatalf("ParseComponentSet: %v", err)
	}
	if err := applicability.ValidateSelection(selected); err != nil {
		t.Fatalf("ValidateSelection: %v", err)
	}
	if !applicability.RequiresControlledCoarsening() {
		t.Fatal("package-represented components did not require controlled coarsening")
	}
	record, found := applicability.Record(ComponentHooks)
	if !found ||
		record.Disposition != ComponentSeparateOptIn ||
		record.Basis != "haft overseer init" {
		t.Fatalf("hooks disposition = %+v, found=%t", record, found)
	}

	wrong, err := ParseComponentSet([]string{
		string(ComponentPackage),
		string(ComponentSkills),
	})
	if err != nil {
		t.Fatalf("ParseComponentSet wrong fixture: %v", err)
	}
	err = applicability.ValidateSelection(wrong)
	if err == nil || !strings.Contains(err.Error(), "selected component skills") {
		t.Fatalf("represented package component was accepted as selected: %v", err)
	}
}

func TestHostComponentApplicabilityValidatesSupportedSubsetSelection(
	t *testing.T,
) {
	records := completeApplicabilityInputs()
	for index := range records {
		if records[index].Component != ComponentInstructions {
			continue
		}
		records[index].Disposition = ComponentIncluded
		records[index].Basis = "coherent host projection"
	}
	applicability, err := NewHostComponentApplicability(
		HostClaude,
		ScopeProject,
		records,
	)
	if err != nil {
		t.Fatalf("NewHostComponentApplicability: %v", err)
	}
	subset, err := ParseComponentSet([]string{
		string(ComponentInstructions),
		string(ComponentMCP),
	})
	if err != nil {
		t.Fatalf("ParseComponentSet: %v", err)
	}
	if err := applicability.ValidateSupportedSelection(subset); err != nil {
		t.Fatalf("ValidateSupportedSelection: %v", err)
	}

	unsupported, err := ParseComponentSet([]string{
		string(ComponentHooks),
	})
	if err != nil {
		t.Fatalf("ParseComponentSet: %v", err)
	}
	err = applicability.ValidateSupportedSelection(unsupported)
	if err == nil || !strings.Contains(
		err.Error(),
		"selected component hooks has disposition separate_opt_in_capability",
	) {
		t.Fatalf("unexpected unsupported selection error: %v", err)
	}
}

func TestHostComponentApplicabilityRejectsPackageRepresentationWithoutPackage(
	t *testing.T,
) {
	records := completeApplicabilityInputs()
	for index := range records {
		switch records[index].Component {
		case ComponentMCP:
			records[index].Disposition = ComponentRepresentedByPackage
		case ComponentPackage:
			records[index].Disposition = ComponentUnavailable
		}
	}
	_, err := NewHostComponentApplicability(
		HostGemini,
		ScopeUser,
		records,
	)
	if err == nil || !strings.Contains(err.Error(), "included package") {
		t.Fatalf("unbound package representation accepted: %v", err)
	}
}

func TestHostComponentApplicabilityRecordsAreCanonicalAndCopied(
	t *testing.T,
) {
	applicability, err := NewHostComponentApplicability(
		HostClaude,
		ScopeProject,
		completeApplicabilityInputs(),
	)
	if err != nil {
		t.Fatalf("NewHostComponentApplicability: %v", err)
	}
	records := applicability.Records()
	got := make([]Component, len(records))
	for index, record := range records {
		got[index] = record.Component
	}
	want := []Component{
		ComponentHooks,
		ComponentInstructions,
		ComponentMCP,
		ComponentPackage,
		ComponentSkills,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("canonical components = %v, want %v", got, want)
	}
	records[0].Basis = "tampered"
	again, _ := applicability.Record(ComponentHooks)
	if again.Basis == "tampered" {
		t.Fatal("applicability exposed mutable records")
	}
}

func completeApplicabilityInputs() []ComponentApplicabilityInput {
	return []ComponentApplicabilityInput{
		{
			Component:   ComponentSkills,
			Disposition: ComponentIncluded,
			Basis:       "coherent host projection",
		},
		{
			Component:   ComponentMCP,
			Disposition: ComponentIncluded,
			Basis:       "coherent host projection",
		},
		{
			Component:   ComponentHooks,
			Disposition: ComponentSeparateOptIn,
			Basis:       "haft overseer init",
		},
		{
			Component:   ComponentPackage,
			Disposition: ComponentUnavailable,
			Basis:       "no registered package carrier",
		},
		{
			Component:   ComponentInstructions,
			Disposition: ComponentUnavailable,
			Basis:       "no registered instruction carrier",
		},
	}
}
