package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestBuildCheckReportOmitsSoftwareSpecHealthForDeclaredNonSoftwareProject(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "check-nonsoftware")

	report, err := buildCheckReport(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSoftwareSpecHealth(t, report.SpecHealth)
	if profileCheckContainsFindingCode(
		report.SpecHealth,
		"profile_capability_applicability_underdetermined",
	) {
		t.Fatalf(
			"non-software check retained a resolved target-system applicability gate: %#v",
			report.SpecHealth,
		)
	}
	if !profileCheckContainsFindingCode(
		report.SpecHealth,
		"spec_carrier_missing_file",
	) {
		t.Fatalf(
			"non-software check suppressed required TargetSystemSpec health: %#v",
			report.SpecHealth,
		)
	}
	if !profileCheckContainsFindingCode(
		report.SpecHealth,
		"term_map_missing_file",
	) {
		t.Fatalf(
			"non-software check suppressed required generic spec health: %#v",
			report.SpecHealth,
		)
	}
}

func TestBuildCheckReportKeepsSoftwareSpecHealthWithoutCanonicalProfile(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)

	report, err := buildCheckReport(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSoftwareSpecHealth(report.SpecHealth) {
		t.Fatalf(
			"profile-underdetermined check silently suppressed software health: %#v",
			report.SpecHealth,
		)
	}
}

func TestBuildCheckReportKeepsSoftwareSpecHealthForDeclaredSoftwareProject(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "check-software")

	report, err := buildCheckReport(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSoftwareSpecHealth(report.SpecHealth) {
		t.Fatalf(
			"software project check omitted required SoftwareSystemSpec health: %#v",
			report.SpecHealth,
		)
	}
}

func TestMCPCheckOmitsSoftwareSpecHealthForDeclaredNonSoftwareProject(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "mcp-check-nonsoftware")
	haftDir := filepath.Join(root, ".haft")

	result, err := handleQuintQuery(
		context.Background(),
		artifact.NewStore(harness.Database()),
		nil,
		haftDir,
		map[string]any{"action": "check"},
	)
	if err != nil {
		t.Fatal(err)
	}
	var report checkReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatal(err)
	}
	assertNoSoftwareSpecHealth(t, report.SpecHealth)
}

func TestOverseerMaintenanceOmitsSoftwareSpecHealthForDeclaredNonSoftwareProject(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "overseer-check-nonsoftware")

	run, err := buildAndStoreOverseerMaintenance(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSoftwareOverseerSignals(t, run.Signals)
}

func assertNoSoftwareSpecHealth(
	t *testing.T,
	findings []project.SpecCheckFinding,
) {
	t.Helper()
	if containsSoftwareSpecHealth(findings) {
		t.Fatalf(
			"non-software project emitted SoftwareSystemSpec health: %#v",
			findings,
		)
	}
}

func containsSoftwareSpecHealth(findings []project.SpecCheckFinding) bool {
	for _, finding := range findings {
		text := strings.Join(
			[]string{
				finding.Path,
				finding.SectionID,
				finding.Message,
				finding.NextAction,
			},
			" ",
		)
		if strings.Contains(strings.ToLower(text), "software-system") ||
			strings.HasPrefix(strings.ToUpper(finding.SectionID), "SS.") {
			return true
		}
	}
	return false
}

func profileCheckContainsFindingCode(
	findings []project.SpecCheckFinding,
	code string,
) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func assertNoSoftwareOverseerSignals(
	t *testing.T,
	signals []overseer.StatusSignal,
) {
	t.Helper()
	for _, signal := range signals {
		text := strings.ToLower(signal.Title + " " + signal.Detail)
		if strings.Contains(text, "software-system") {
			t.Fatalf(
				"non-software overseer run emitted SoftwareSystemSpec health: %#v",
				signals,
			)
		}
	}
}
