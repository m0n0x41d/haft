package decisionrecordadapter

import (
	"bytes"
	"testing"
)

func TestMappingManifestIsCanonicalAndAuthorityBounded(t *testing.T) {
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
		t.Fatal("DecisionRecord manifest round trip changed identity")
	}
	for _, required := range [][]byte{
		[]byte("manual_h_decide_institutes_decision_record_before"),
		[]byte("adapter_accepts_no_raw_choice_or_recommendation"),
		[]byte("generic_admit_cannot_create_supersede_or_reopen"),
		[]byte("projection_grants_no_work_commission"),
	} {
		if !bytes.Contains(manifest.CanonicalBytes(), required) {
			t.Fatalf(
				"DecisionRecord manifest omits boundary %q",
				required,
			)
		}
	}
}

func TestMappingManifestRejectsSemanticMutation(t *testing.T) {
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		t.Fatalf("CurrentMappingManifestV1: %v", err)
	}
	mutated := bytes.Replace(
		manifest.CanonicalBytes(),
		[]byte("decision_record"),
		[]byte("project_record"),
		1,
	)
	if _, err := DecodeMappingManifestV1(mutated); err == nil {
		t.Fatal("DecodeMappingManifestV1 accepted a changed carrier variant")
	}
}
