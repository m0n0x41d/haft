package sqlite

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

// productionFreshCurrentAssertionCarrier keeps the production E2E fixtures
// honest about the current writer contract. Fresh admissions are exact v3
// assertions with an explicit posture; they are never legacy occurrences.
func productionFreshCurrentAssertionCarrier(
	t *testing.T,
	active typedmemorystore.CurrentActiveAssertion,
) typedmemorystore.CurrentAssertionCarrier {
	t.Helper()

	carrier := active.Carrier()
	if carrier.Kind() != typedmemorystore.CurrentRelationalAssertionV3Carrier {
		t.Fatalf(
			"fresh current assertion %s carrier = %q, want exact v3 assertion",
			active.AssertionID(),
			carrier.Kind(),
		)
	}
	if _, legacy := active.LegacyRelation(); legacy {
		t.Fatalf(
			"fresh current assertion %s was reinterpreted as a legacy occurrence",
			active.AssertionID(),
		)
	}
	assertion, exact := active.RelationalAssertion()
	if !exact {
		t.Fatalf(
			"fresh current assertion %s omitted its exact v3 carrier",
			active.AssertionID(),
		)
	}
	modality, explicit := active.Posture().ExplicitModality()
	assertionModality := assertion.Modality()
	assertionModalityKind := assertionModality.Kind()
	if !explicit || modality != assertionModalityKind {
		t.Fatalf(
			"fresh current assertion %s posture = %q/%t, carrier modality = %q",
			active.AssertionID(),
			modality,
			explicit,
			assertionModalityKind,
		)
	}
	if modality != typedmemory.AssertionModalityAffirmsObtaining {
		t.Fatalf(
			"fresh production assertion %s modality = %q, want affirms_obtaining",
			active.AssertionID(),
			modality,
		)
	}
	return carrier
}
