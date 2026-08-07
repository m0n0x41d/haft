package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestBuildProfileAwareProcessCheckReportOmitsSWEMethodPackForNonSoftware(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "process-nonsoftware")
	report, err := buildProfileAwareProcessCheckReport(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
		processCheckOptions{
			Profile: "core",
			Now:     time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	result := processCheckTestResult(
		t,
		report.Results,
		"methodpack_carrier_currentness",
	)
	if result.Status != processCheckStatusNotApplicable {
		t.Fatalf("non-software MethodPack status = %q", result.Status)
	}
	if strings.Contains(result.Finding, "missing=") ||
		strings.Contains(result.Finding, "stale=") {
		t.Fatalf("non-software result scanned SWE carriers: %#v", result)
	}
	for _, checkID := range []string{
		"method_run_hard_gates",
		"carry_through_acceptance_ref_posture",
	} {
		result := processCheckTestResult(t, report.Results, checkID)
		if result.Status != processCheckStatusNotApplicable {
			t.Fatalf("non-software %s status = %q, want not_applicable", checkID, result.Status)
		}
		evidence := strings.Join(result.EvidenceRefs, " ")
		if !strings.Contains(evidence, "capability:process_checks=not_applicable") {
			t.Fatalf("non-software %s lost exact applicability basis: %#v", checkID, result)
		}
		if !strings.Contains(result.Finding, "were not scanned") {
			t.Fatalf("non-software %s does not prove the SWE scan was skipped: %#v", checkID, result)
		}
	}
	for _, checkID := range []string{
		"generated_contract_runtime_schema",
		"binding_actions_fail_closed",
		"default_status_compact",
		"interface_discovery_compact",
	} {
		result := processCheckTestResult(t, report.Results, checkID)
		if result.Status == processCheckStatusNotApplicable {
			t.Fatalf("generic check %s was suppressed by a non-software profile: %#v", checkID, result)
		}
	}
}

func TestNonSoftwareProcessApplicabilitySkipsMethodRunStoreReads(t *testing.T) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "process-no-methodrun-read")
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	results, err := processCheckProjectMethodResultsForProfileResolution(
		context.Background(),
		nil,
		time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		resolution,
	)
	if err != nil {
		t.Fatal(err)
	}
	if results.HardGates.Status != processCheckStatusNotApplicable ||
		results.CarryThrough.Status != processCheckStatusNotApplicable {
		t.Fatalf("non-software MethodRun results = %#v", results)
	}
}

func TestBuildProfileAwareProcessCheckReportRetainsSWEMethodPackForSoftware(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "process-software")
	report, err := buildProfileAwareProcessCheckReport(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
		processCheckOptions{
			Profile: "core",
			Now:     time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	result := processCheckTestResult(
		t,
		report.Results,
		"methodpack_carrier_currentness",
	)
	if result.Status != processCheckStatusDegraded {
		t.Fatalf("software MethodPack status = %q", result.Status)
	}
	if !strings.Contains(result.Finding, "missing=") {
		t.Fatalf("software result omitted carrier scan: %#v", result)
	}
	carryThrough := processCheckTestResult(
		t,
		report.Results,
		"carry_through_acceptance_ref_posture",
	)
	if carryThrough.Status != processCheckStatusPass {
		t.Fatalf("software MethodRun checks were not executed: %#v", carryThrough)
	}
}

func TestBuildProfileAwareProcessCheckReportKeepsMissingProfileNeutral(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	report, err := buildProfileAwareProcessCheckReport(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
		processCheckOptions{
			Profile: "core",
			Now:     time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	result := processCheckTestResult(
		t,
		report.Results,
		"methodpack_carrier_currentness",
	)
	if result.Status != processCheckStatusUnknown {
		t.Fatalf("underdetermined MethodPack status = %q", result.Status)
	}
	if !strings.Contains(result.Finding, "profile is underdetermined") ||
		strings.Contains(result.Finding, "missing=") {
		t.Fatalf("underdetermined result is not neutral: %#v", result)
	}
	for _, checkID := range []string{
		"method_run_hard_gates",
		"carry_through_acceptance_ref_posture",
	} {
		result := processCheckTestResult(t, report.Results, checkID)
		if result.Status != processCheckStatusUnknown {
			t.Fatalf("underdetermined %s status = %q, want unknown", checkID, result.Status)
		}
		if !strings.Contains(result.Finding, "were not scanned") {
			t.Fatalf("underdetermined %s scanned MethodRuns: %#v", checkID, result)
		}
	}
}

func TestBuildProfileAwareProcessCheckReportPreservesExactScopeMiss(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "process-exact-miss")
	report, err := buildProfileAwareProcessCheckReport(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
		processCheckOptions{
			Profile: "core",
			ScopeID: "missing-scope",
			Now:     time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	result := processCheckTestResult(
		t,
		report.Results,
		"methodpack_carrier_currentness",
	)
	if result.Status != processCheckStatusUnknown {
		t.Fatalf("exact-scope miss status = %q", result.Status)
	}
	if !strings.Contains(
		result.Finding,
		`Requested project scope "missing-scope" is absent`,
	) || !strings.Contains(
		result.Finding,
		"non-software-process-exact-miss",
	) {
		t.Fatalf("exact-scope miss result = %#v", result)
	}
	if strings.Contains(result.Finding, "missing=") {
		t.Fatalf("exact-scope miss scanned SWE carriers: %#v", result)
	}
}
