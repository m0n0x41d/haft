package problemcardadapter

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
	if decoded.Ref() != manifest.Ref() {
		t.Fatalf("decoded manifest ref = %q, want %q", decoded.Ref(), manifest.Ref())
	}
	if decoded.AdapterVersion() != manifest.AdapterVersion() {
		t.Fatalf(
			"decoded adapter version = %q, want %q",
			decoded.AdapterVersion().String(),
			manifest.AdapterVersion().String(),
		)
	}
	copyBytes := manifest.CanonicalBytes()
	copyBytes[0] ^= 0xff
	if bytes.Equal(copyBytes, manifest.CanonicalBytes()) {
		t.Fatal("MappingManifestV1 leaked mutable canonical bytes")
	}
}

func TestMappingManifestV1DeclaresExactProblemCardAtConcernMapping(t *testing.T) {
	manifest := canonicalMappingManifestV1()
	if manifest.EmittedSignature != "Haft.ProblemCardAtConcern" {
		t.Fatalf(
			"emitted signature = %q, want Haft.ProblemCardAtConcern",
			manifest.EmittedSignature,
		)
	}
	wantChanges := []string{
		"DeclareEntity(problem_card_record)",
		"AssertRelation(Haft.ProblemCardAtConcern,affirms_obtaining)",
	}
	if !reflect.DeepEqual(manifest.EmittedChanges, wantChanges) {
		t.Fatalf("emitted changes = %#v, want %#v", manifest.EmittedChanges, wantChanges)
	}
	wantSlots := []string{
		"Haft.ProblemCardAtConcern.ProblemCardSlot=Haft.ProjectRecordRef",
		"Haft.ProblemCardAtConcern.EntityOfConcernSlot=U.EntityRef",
		"Haft.ProblemCardAtConcern.ClaimGraphSlot=U.ClaimGraph@ByValue",
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
