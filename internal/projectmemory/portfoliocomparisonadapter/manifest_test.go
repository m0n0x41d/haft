package portfoliocomparisonadapter

import (
	"bytes"
	"reflect"
	"testing"
)

func TestMappingManifestV1HasStableExactIdentity(t *testing.T) {
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
		t.Fatal("PortfolioComparison manifest round trip changed its identity")
	}
	copyBytes := manifest.CanonicalBytes()
	copyBytes[0] ^= 0xff
	if bytes.Equal(copyBytes, manifest.CanonicalBytes()) {
		t.Fatal("MappingManifestV1 leaked mutable canonical bytes")
	}
}

func TestMappingManifestV1HasNoWinnerOrDecisionSlot(t *testing.T) {
	manifest := canonicalMappingManifestV1()
	wantSlots := []string{
		"Haft.PortfolioComparison.ComparisonSlot=Haft.ProjectRecordRef",
		"Haft.PortfolioComparison.PortfolioSlot=Haft.ProjectRecordRef",
		"Haft.PortfolioComparison.EntityOfConcernSlot=U.EntityRef",
		"Haft.PortfolioComparison.ComparedOptionSlot=Haft.ProjectRecordRef{2..unbounded}",
		"Haft.PortfolioComparison.NonDominatedOptionSlot=Haft.ProjectRecordRef{1..unbounded}",
		"Haft.PortfolioComparison.ClaimGraphSlot=U.ClaimGraph@ByValue",
	}
	if !reflect.DeepEqual(manifest.EmittedSlots, wantSlots) {
		t.Fatalf("emitted slots = %#v, want %#v", manifest.EmittedSlots, wantSlots)
	}
	for _, slot := range manifest.EmittedSlots {
		if slot == "Haft.PortfolioComparison.ChosenOptionSlot" {
			t.Fatal("PortfolioComparison mapping smuggled in a chosen option")
		}
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
