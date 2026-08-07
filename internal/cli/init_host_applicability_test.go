package cli

import (
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestCurrentCoherentHostApplicabilityIsCompleteAndHookFree(
	t *testing.T,
) {
	t.Parallel()

	applicability, err := currentCoherentHostApplicabilityRegistry()
	if err != nil {
		t.Fatalf("currentCoherentHostApplicabilityRegistry: %v", err)
	}
	faces := currentCoherentHostFaceRegistry()
	if len(applicability) != len(faces) {
		t.Fatalf(
			"component applicability records = %d, coherent faces = %d",
			len(applicability),
			len(faces),
		)
	}
	for key := range faces {
		record, found := applicability[key]
		if !found {
			t.Fatalf(
				"coherent face %s/%s lacks component applicability",
				key.host,
				key.scope,
			)
		}
		hooks, found := record.Record(initplanning.ComponentHooks)
		if !found ||
			hooks.Disposition != initplanning.ComponentSeparateOptIn ||
			hooks.Basis == "" {
			t.Fatalf(
				"coherent face %s/%s hooks disposition = %+v, found=%t",
				key.host,
				key.scope,
				hooks,
				found,
			)
		}
	}
}

func TestCurrentPiApplicabilityRequiresControlledPackageCoarsening(
	t *testing.T,
) {
	t.Parallel()

	registry, err := currentCoherentHostApplicabilityRegistry()
	if err != nil {
		t.Fatalf("currentCoherentHostApplicabilityRegistry: %v", err)
	}
	key := currentCoherentHostKey{
		host:  initplanning.HostPi,
		scope: initplanning.ScopeProject,
	}
	pi := registry[key]
	if !pi.RequiresControlledCoarsening() {
		t.Fatal("Pi component applicability does not require controlled coarsening")
	}
	for _, component := range []initplanning.Component{
		initplanning.ComponentInstructions,
		initplanning.ComponentMCP,
		initplanning.ComponentSkills,
	} {
		record, found := pi.Record(component)
		if !found ||
			record.Disposition !=
				initplanning.ComponentRepresentedByPackage {
			t.Fatalf(
				"Pi component %s disposition = %+v, found=%t",
				component,
				record,
				found,
			)
		}
	}
	packageRecord, found := pi.Record(initplanning.ComponentPackage)
	if !found ||
		packageRecord.Disposition != initplanning.ComponentIncluded {
		t.Fatalf(
			"Pi package disposition = %+v, found=%t",
			packageRecord,
			found,
		)
	}
}

func TestOnlyPiRepresentsComponentsInsideAPackage(t *testing.T) {
	t.Parallel()

	registry, err := currentCoherentHostApplicabilityRegistry()
	if err != nil {
		t.Fatalf("currentCoherentHostApplicabilityRegistry: %v", err)
	}
	coarsened := make([]initplanning.HostID, 0)
	for _, applicability := range registry {
		if applicability.RequiresControlledCoarsening() {
			coarsened = append(coarsened, applicability.Host())
		}
	}
	if !slices.Equal(coarsened, []initplanning.HostID{initplanning.HostPi}) {
		t.Fatalf("package-coarsened hosts = %v, want [pi]", coarsened)
	}
}
