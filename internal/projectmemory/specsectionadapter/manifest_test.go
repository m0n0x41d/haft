package specsectionadapter

import (
	"bytes"
	"reflect"
	"testing"
)

func TestMappingManifestV1HasStableRoundTripIdentity(t *testing.T) {
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		t.Fatalf("CurrentMappingManifestV1() error = %v", err)
	}
	if err := manifest.Verify(); err != nil {
		t.Fatalf("MappingManifestV1.Verify() error = %v", err)
	}
	decoded, err := DecodeMappingManifestV1(manifest.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeMappingManifestV1() error = %v", err)
	}
	if decoded.Ref() != manifest.Ref() ||
		decoded.AdapterVersion() != manifest.AdapterVersion() {
		t.Fatal("SpecSection mapping manifest round trip changed identity")
	}
	copyBytes := manifest.CanonicalBytes()
	copyBytes[0] ^= 0xff
	if bytes.Equal(copyBytes, manifest.CanonicalBytes()) {
		t.Fatal("MappingManifestV1 leaked mutable canonical bytes")
	}
}

func TestMappingManifestV1DeclaresSpecializedSpecSectionMapping(t *testing.T) {
	manifest := canonicalMappingManifestV1()
	if manifest.CarrierVariant != "spec_section_record" {
		t.Fatalf(
			"carrier variant = %q, want spec_section_record",
			manifest.CarrierVariant,
		)
	}
	if manifest.EmittedSignature != "Haft.SpecSectionAtConcern" {
		t.Fatalf(
			"emitted signature = %q, want Haft.SpecSectionAtConcern",
			manifest.EmittedSignature,
		)
	}
	wantChanges := []string{
		"DeclareEntity(spec_section_record)",
		"AssertRelation(Haft.SpecSectionAtConcern,affirms_obtaining)",
	}
	if !reflect.DeepEqual(manifest.EmittedChanges, wantChanges) {
		t.Fatalf("emitted changes = %#v, want %#v", manifest.EmittedChanges, wantChanges)
	}
	wantSlots := []string{
		"Haft.SpecSectionAtConcern.SpecSectionRecordSlot=Haft.SpecSectionRecordRef",
		"Haft.SpecSectionAtConcern.EntityOfConcernSlot=U.EntityRef",
		"Haft.SpecSectionAtConcern.ClaimGraphSlot=U.ClaimGraph@ByValue",
	}
	if !reflect.DeepEqual(manifest.EmittedSlots, wantSlots) {
		t.Fatalf("emitted slots = %#v, want %#v", manifest.EmittedSlots, wantSlots)
	}
}

func TestMappingManifestV1RejectsUnknownAndTrailingContent(t *testing.T) {
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		t.Fatalf("CurrentMappingManifestV1() error = %v", err)
	}
	canonical := manifest.CanonicalBytes()
	unknown := append([]byte(nil), canonical[:len(canonical)-1]...)
	unknown = append(unknown, []byte(`,"unknown":true}`)...)
	if _, err := DecodeMappingManifestV1(unknown); err == nil {
		t.Fatal("DecodeMappingManifestV1() accepted an unknown field")
	}
	trailing := append(append([]byte(nil), canonical...), []byte(`{}`)...)
	if _, err := DecodeMappingManifestV1(trailing); err == nil {
		t.Fatal("DecodeMappingManifestV1() accepted trailing JSON")
	}
}
