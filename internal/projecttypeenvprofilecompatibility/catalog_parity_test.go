package projecttypeenvprofilecompatibility_test

import (
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/projectionprofile"
)

func TestCompatibilityDescriptorsExactlyMatchNeighborhoodProfiles(t *testing.T) {
	descriptors := projectionprofile.Installed()
	profiles := neighborhood.BuiltinProjectionProfiles()
	if len(descriptors) != len(profiles) {
		t.Fatalf(
			"compatibility descriptor count = %d, neighborhood profile count = %d",
			len(descriptors),
			len(profiles),
		)
	}
	for index, descriptor := range descriptors {
		profile := profiles[index]
		if descriptor.Ref() != profile.Ref() ||
			descriptor.Edition() != profile.Edition() ||
			descriptor.Digest() != profile.Digest() {
			t.Fatalf("profile %d identity differs from its compatibility descriptor", index)
		}
		if !slices.Equal(descriptor.Facets(), profile.Facets()) {
			t.Fatalf("profile %q facets differ from its compatibility descriptor", descriptor.Ref().String())
		}
		if !slices.Equal(descriptor.SlotReads(), profile.SlotReads()) {
			t.Fatalf("profile %q SlotKind reads differ from its compatibility descriptor", descriptor.Ref().String())
		}
	}
}
