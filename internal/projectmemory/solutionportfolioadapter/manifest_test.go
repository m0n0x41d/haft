package solutionportfolioadapter

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
		t.Fatal("SolutionPortfolio manifest round trip changed its identity")
	}
	copyBytes := manifest.CanonicalBytes()
	copyBytes[0] ^= 0xff
	if bytes.Equal(copyBytes, manifest.CanonicalBytes()) {
		t.Fatal("MappingManifestV1 leaked mutable canonical bytes")
	}
}

func TestMappingManifestV1DeclaresAllOptionsWithoutSelection(t *testing.T) {
	manifest := canonicalMappingManifestV1()
	if manifest.EmittedSignature != "Haft.SolutionPortfolioAtConcern" {
		t.Fatalf(
			"emitted signature = %q, want Haft.SolutionPortfolioAtConcern",
			manifest.EmittedSignature,
		)
	}
	wantSlots := []string{
		"Haft.SolutionPortfolioAtConcern.PortfolioSlot=Haft.ProjectRecordRef",
		"Haft.SolutionPortfolioAtConcern.EntityOfConcernSlot=U.EntityRef",
		"Haft.SolutionPortfolioAtConcern.ClaimGraphSlot=U.ClaimGraph@ByValue",
		"Haft.SolutionPortfolioAtConcern.OptionSlot=Haft.ProjectRecordRef{2..unbounded}",
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
