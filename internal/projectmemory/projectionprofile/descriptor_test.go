package projectionprofile_test

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/projectionprofile"
)

func TestInstalledDescriptorsAreClosedAndImmutable(t *testing.T) {
	descriptors := projectionprofile.Installed()
	if len(descriptors) != 6 {
		t.Fatalf("installed descriptor count = %d, want 6", len(descriptors))
	}
	for index, descriptor := range descriptors {
		if !descriptor.Valid() {
			t.Fatalf("installed descriptor %d is invalid", index)
		}
		if index > 0 && descriptors[index-1].Ref().String() >= descriptor.Ref().String() {
			t.Fatal("installed descriptors are not in canonical identity order")
		}
	}

	first := descriptors[0]
	facets := first.Facets()
	facets[0] = projectionprofile.FacetUnresolved
	slots := first.SlotReads()
	slots = slots[:0]
	reloaded, found := projectionprofile.Lookup(first.Ref())
	if !found || !reloaded.Valid() {
		t.Fatal("mutating returned descriptor views changed the installed catalog")
	}
	if reloaded.Facets()[0] != projectionprofile.FacetEpistemes || len(reloaded.SlotReads()) == len(slots) {
		t.Fatal("installed descriptor did not retain immutable facets and SlotKind reads")
	}
}
