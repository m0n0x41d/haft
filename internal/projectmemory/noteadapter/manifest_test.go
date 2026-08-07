package noteadapter

import (
	"bytes"
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
