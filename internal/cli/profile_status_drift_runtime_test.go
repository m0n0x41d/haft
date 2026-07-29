package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestProjectProfileDriftProjectionKeepsDeclaredAuthoritySeparate(
	t *testing.T,
) {
	inspection := profileInspectionResponse{
		CanonicalProfile: canonicalProfileView{
			Kind:                  "declared",
			SemanticRole:          "canonical_admitted_profile",
			LedgerRevision:        7,
			PayloadDigest:         "sha256:declared-payload",
			AdmissionRecordRef:    "profile-admission:declared",
			AdmissionRecordDigest: "sha256:declared-admission",
			RecordedAt:            "2026-07-19T00:00:00Z",
			Scopes: []canonicalProfileScopeView{{
				ScopeID:         "documents",
				RealizationKind: "non_software",
			}},
		},
		Suggestion: profileSuggestionView{
			SuggestionRef:     "profile-suggestion:sha256:observed",
			DetectorVersion:   "profile-detector.v1",
			PolicyVersion:     "profile-detector-policy.v1",
			ObservationBasis:  "normalized_project_relative_file_paths",
			ObservationDigest: "sha256:observed",
			Classification:    "software_signals",
			ConfidencePosture: "supported",
			Scan: profileScanView{
				ScannedFileCount: 3,
			},
		},
		Relation: newProfileInspectionRelation(
			"conflicts_with_declared",
			"sqlite_profile_admission_ledger",
			"detector orientation differs from the declaration",
		),
	}
	drift, found := projectProfileDriftFromInspection(inspection)
	if !found || !drift.valid() {
		t.Fatalf("typed profile drift = %#v, found=%v", drift, found)
	}
	if drift.DeclaredBasis.AdmissionRecordRef != inspection.CanonicalProfile.AdmissionRecordRef {
		t.Fatalf("declared basis changed: %#v", drift.DeclaredBasis)
	}
	if drift.DetectorBasis.ObservationDigest != inspection.Suggestion.ObservationDigest {
		t.Fatalf("detector basis changed: %#v", drift.DetectorBasis)
	}
	if drift.Relation.DetectorMayMutate || drift.Relation.DetectorMaySatisfyGate {
		t.Fatalf("read-only drift granted detector authority: %#v", drift.Relation)
	}
}

func TestStatusSurfacesDeclaredDetectedProfileDriftWithoutMutation(
	t *testing.T,
) {
	harness := profileadmissionfixture.New(t, t.TempDir())
	root := harness.Root().String()
	admission := harness.AdmitNonSoftwareRevision(t, "status-profile-drift")
	writeProfileInspectionFixture(t, root, "go.mod")
	writeProfileInspectionFixture(t, root, "internal/kernel.go")

	inspection, err := executeProfileInspection(context.Background(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Relation.Kind != "conflicts_with_declared" {
		t.Fatalf("inspection relation = %#v, want declared/detected conflict", inspection.Relation)
	}
	if inspection.Suggestion.Classification != string(profiledetector.SoftwareSignals) {
		t.Fatalf("detector classification = %q", inspection.Suggestion.Classification)
	}

	before := profileStatusLedgerCounts(t, harness)
	store := artifact.NewStore(harness.Database())
	haftDir := filepath.Join(root, ".haft")
	compact, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{"action": "status"},
	)
	if err != nil {
		t.Fatal(err)
	}
	full, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{"action": "status", "full": true},
	)
	if err != nil {
		t.Fatal(err)
	}
	after := profileStatusLedgerCounts(t, harness)
	if before != after {
		t.Fatalf("read-only status changed profile ledger counts: before=%v after=%v", before, after)
	}

	exactMarkers := []string{
		"Profile drift: kind=" + projectProfileDriftRecordKind,
		"semantic_role=read_only_profile_attention",
		"scope_ids=[non-software-status-profile-drift]",
		"realization_kinds=[non_software]",
		"admission_record_ref=" + admission.AdmissionRecordRef().String(),
		"admission_record_digest=" + admission.AdmissionRecordDigest().String(),
		"profile_payload_digest=" + admission.PayloadDigest().String(),
		fmt.Sprintf("ledger_revision=%d", admission.LedgerRevision().Value()),
		"suggestion_ref=" + inspection.Suggestion.SuggestionRef,
		"detector_version=" + inspection.Suggestion.DetectorVersion,
		"policy_version=" + inspection.Suggestion.PolicyVersion,
		"observation_basis=" + inspection.Suggestion.ObservationBasis,
		"observation_digest=" + inspection.Suggestion.ObservationDigest,
		"classification=" + inspection.Suggestion.Classification,
		"relation=conflicts_with_declared",
		"binding_source=sqlite_profile_admission_ledger",
		"detector_observation_cannot_mutate_or_displace_declared_profile",
		"no mutation was performed",
	}
	for _, output := range []struct {
		name string
		text string
	}{
		{name: "compact", text: compact},
		{name: "full", text: full},
	} {
		for _, marker := range exactMarkers {
			if !strings.Contains(output.text, marker) {
				t.Fatalf("%s status omitted exact profile-drift marker %q:\n%s", output.name, marker, output.text)
			}
		}
	}

	for _, marker := range []string{
		"Capability applicability (authority=canonical_profile_capability_matrix.v1)",
		"capability=code_doctrine_and_index; applicability=not_applicable",
		"capability=process_checks; applicability=not_applicable",
		"capability=software_system_spec; applicability=not_applicable",
		"capability=swe_methodpack; applicability=not_applicable",
		"capability=target_system_spec; applicability=underdetermined",
		"missing_basis=admitted_target_system_relation",
		"Detector candidates (non-binding)",
	} {
		if !strings.Contains(full, marker) {
			t.Fatalf("full status omitted capability/drift detail %q:\n%s", marker, full)
		}
	}
	if strings.Contains(compact, "Capability applicability (authority=") {
		t.Fatalf("compact status inlined full capability matrix:\n%s", compact)
	}
}

type profileStatusCounts struct {
	admissionsV2 int
	admissionsV3 int
	revisionsV2  int
	revisionsV3  int
}

func profileStatusLedgerCounts(
	t *testing.T,
	harness *profileadmissionfixture.Harness,
) profileStatusCounts {
	t.Helper()
	counts := profileStatusCounts{}
	if err := harness.Database().QueryRow(
		"SELECT COUNT(*) FROM project_profile_admissions_v2",
	).Scan(&counts.admissionsV2); err != nil {
		t.Fatal(err)
	}
	if err := harness.Database().QueryRow(
		"SELECT COUNT(*) FROM project_profile_admissions_v3",
	).Scan(&counts.admissionsV3); err != nil {
		t.Fatal(err)
	}
	if err := harness.Database().QueryRow(
		"SELECT COUNT(*) FROM project_profile_revisions_v2",
	).Scan(&counts.revisionsV2); err != nil {
		t.Fatal(err)
	}
	if err := harness.Database().QueryRow(
		"SELECT COUNT(*) FROM project_profile_revisions_v3",
	).Scan(&counts.revisionsV3); err != nil {
		t.Fatal(err)
	}
	return counts
}
