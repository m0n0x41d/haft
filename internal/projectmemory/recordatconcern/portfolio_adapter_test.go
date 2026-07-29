package recordatconcern

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/adaptersource"
)

func TestCurrentRecordMappingDoesNotRequireHistoricalMembershipRegistration(
	t *testing.T,
) {
	runtime := ExactRuntimeBasis{
		sourceMode: adaptersource.CurrentKindClassification(),
	}
	result, accepted := requireRegisteredMapping(Contract{}, runtime)
	if !accepted || result != nil {
		t.Fatalf("current mapping registration = %T / %t, want accepted", result, accepted)
	}
}

func TestHistoricalRecordMappingStillFailsClosedWithoutRegistration(
	t *testing.T,
) {
	runtime := ExactRuntimeBasis{
		sourceMode: adaptersource.HistoricalMembership(),
	}
	result, accepted := requireRegisteredMapping(Contract{}, runtime)
	if accepted || result == nil {
		t.Fatalf("historical mapping registration = %T / %t, want unresolved", result, accepted)
	}
}
