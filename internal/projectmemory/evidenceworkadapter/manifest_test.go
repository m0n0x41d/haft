package evidenceworkadapter

import (
	"bytes"
	"testing"
)

func TestMappingManifestIsExactAndKeepsEvidenceWorkBoundary(t *testing.T) {
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Verify(); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeMappingManifestV1(manifest.CanonicalBytes())
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Ref() != manifest.Ref() ||
		decoded.AdapterVersion() != manifest.AdapterVersion() {
		t.Fatal("Evidence/Work manifest round trip changed exact coordinates")
	}
	for _, required := range [][]byte{
		[]byte("AssertRelation(Haft.SupportingEpistemeRecordAtConcern,affirms_obtaining)"),
		[]byte("AssertRelation(Haft.WorkOccurrenceRecord,affirms_obtaining)"),
		[]byte("AssertRelation(Haft.EvidenceUse,affirms_obtaining)"),
		[]byte("is_not_exact_fpf_evidence"),
		[]byte("is_not_exact_u_work"),
		[]byte("work_plan_log_file_test_output_and_telemetry"),
		[]byte("returns_underdetermined_and_emits_zero_changes"),
	} {
		if !bytes.Contains(manifest.CanonicalBytes(), required) {
			t.Fatalf(
				"Evidence/Work manifest omitted semantic boundary %q",
				required,
			)
		}
	}
	if bytes.Contains(manifest.CanonicalBytes(), []byte("InstantiateRelation")) {
		t.Fatal("Evidence/Work manifest still advertises legacy unqualified relation changes")
	}
}

func TestMappingManifestRejectsSemanticBoundaryMutation(t *testing.T) {
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		t.Fatal(err)
	}
	mutated := bytes.Replace(
		manifest.CanonicalBytes(),
		[]byte("is_not_exact_u_work"),
		[]byte("is_exact_u_work____"),
		1,
	)
	if bytes.Equal(mutated, manifest.CanonicalBytes()) {
		t.Fatal("test mutation did not change canonical manifest bytes")
	}
	if _, err := DecodeMappingManifestV1(mutated); err == nil {
		t.Fatal("manifest decoder accepted semantic-boundary mutation")
	}
}
