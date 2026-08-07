package projectmemory

import (
	"bytes"
	"testing"
)

func TestProjectEntityUniverseMappingManifestHasStableExactIdentity(
	t *testing.T,
) {
	manifest, err := CurrentProjectEntityUniverseMappingManifestV1()
	if err != nil {
		t.Fatalf("CurrentProjectEntityUniverseMappingManifestV1: %v", err)
	}
	if err := manifest.Verify(); err != nil {
		t.Fatalf("manifest Verify: %v", err)
	}
	decoded, err := DecodeProjectEntityUniverseMappingManifestV1(
		manifest.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("DecodeProjectEntityUniverseMappingManifestV1: %v", err)
	}
	if decoded.Ref() != manifest.Ref() {
		t.Fatal("decoded project-entity mapping changed its exact reference")
	}
	if decoded.AdapterVersion() != manifest.AdapterVersion() {
		t.Fatal("decoded project-entity mapping changed its adapter version")
	}
	mutated := manifest.CanonicalBytes()
	mutated[0] ^= 0xff
	if bytes.Equal(mutated, manifest.CanonicalBytes()) {
		t.Fatal("project-entity mapping leaked mutable canonical bytes")
	}
}

func TestProjectEntityUniverseMappingManifestRejectsUnknownAndTrailingContent(
	t *testing.T,
) {
	manifest, err := CurrentProjectEntityUniverseMappingManifestV1()
	if err != nil {
		t.Fatal(err)
	}
	canonical := manifest.CanonicalBytes()
	unknown := append([]byte(nil), canonical[:len(canonical)-1]...)
	unknown = append(unknown, []byte(`,"unknown":true}`)...)
	if _, err := DecodeProjectEntityUniverseMappingManifestV1(unknown); err == nil {
		t.Fatal("project-entity mapping accepted an unknown field")
	}
	trailing := append(append([]byte(nil), canonical...), []byte(`{}`)...)
	if _, err := DecodeProjectEntityUniverseMappingManifestV1(trailing); err == nil {
		t.Fatal("project-entity mapping accepted trailing content")
	}
}
