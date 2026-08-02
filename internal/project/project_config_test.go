package project

import "testing"

func TestKnownGeneratedLegacyProjectConfigDigestIsExact(t *testing.T) {
	known := "sha256:026df268318667889d690421e63e3183ec25dafab012967fba4eb2e0c032a438"
	if !IsKnownGeneratedLegacyProjectConfigDigest(known) {
		t.Fatal("known generated project config digest was not recognized")
	}
	if IsKnownGeneratedLegacyProjectConfigDigest(
		"sha256:126df268318667889d690421e63e3183ec25dafab012967fba4eb2e0c032a438",
	) {
		t.Fatal("modified project config digest was recognized as generated")
	}
}
