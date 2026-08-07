package codeanchoradapter

import (
	"bytes"
	"strings"
	"testing"
)

func TestMappingManifestIsExactAndRejectsSemanticInference(t *testing.T) {
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		t.Fatalf("CurrentMappingManifestV1: %v", err)
	}
	if err := manifest.Verify(); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	decoded, err := DecodeMappingManifestV1(manifest.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeMappingManifestV1: %v", err)
	}
	if decoded.Ref() != manifest.Ref() ||
		decoded.AdapterVersion() != manifest.AdapterVersion() {
		t.Fatal("CodeAnchor manifest round trip changed exact identity")
	}
	canonical := string(manifest.CanonicalBytes())
	required := []string{
		"Haft.CodeAnchorDefinition",
		"Haft.CodeRealizesClaim",
		"Haft.CodeChangedByWork",
		"AssertRelation(Haft.CodeAnchorDefinition,affirms_obtaining)",
		"affected_files_backlinks_and_search_rank_never_create_a_semantic_link",
	}
	for _, value := range required {
		if !strings.Contains(canonical, value) {
			t.Fatalf("CodeAnchor manifest omitted %q", value)
		}
	}
	if strings.Contains(canonical, "InstantiateRelation") {
		t.Fatal("CodeAnchor manifest still advertises legacy unqualified relation changes")
	}
	mutated := bytes.Replace(
		manifest.CanonicalBytes(),
		[]byte("explicit_assertions"),
		[]byte("implicit_assertion"),
		1,
	)
	if _, err := DecodeMappingManifestV1(mutated); err == nil {
		t.Fatal("CodeAnchor manifest accepted semantic-boundary mutation")
	}
}
