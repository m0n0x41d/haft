package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestRunSpecStatusSummaryShowsOneNeutralCueWithoutCanonicalProfile(t *testing.T) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreJSON := stubSpecStatusJSON(t, false)
	defer restoreJSON()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecStatus(cmd, nil); err != nil {
		t.Fatalf("runSpecStatus returned error: %v", err)
	}

	got := output.String()
	for _, want := range []string{
		"haft spec status: not evaluated (profile_underdetermined)",
		"Profile cue:",
		"Missing basis: current_canonical_profile_admission",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Count(got, "Profile cue:") != 1 {
		t.Fatalf("status emitted repeated profile cues:\n%s", got)
	}
	if strings.Contains(got, "software-system") {
		t.Fatalf("status emitted speculative software pressure:\n%s", got)
	}
}

func TestRunSpecNextJSONReturnsOneNeutralCueWithoutCanonicalProfile(t *testing.T) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreJSON := stubSpecNextJSON(t, true)
	defer restoreJSON()
	restoreScopeID := stubSpecNextScopeID(t, "")
	defer restoreScopeID()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecNext(cmd, nil); err != nil {
		t.Fatalf("runSpecNext returned error: %v", err)
	}

	var result publicSpecLifecycleResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", err, output.String())
	}
	if result.SpecLifecycleProjection != nil {
		t.Fatalf("underdetermined profile fabricated lifecycle: %#v", result)
	}
	if result.ProfileApplicability.Kind !=
		string(projectSpecificationProfileUnderdetermined) {
		t.Fatalf(
			"profile applicability kind = %q",
			result.ProfileApplicability.Kind,
		)
	}
	if result.ProfileApplicability.Cue == nil ||
		result.ProfileApplicability.Cue.Code !=
			string(projectSpecificationProfileUnderdetermined) {
		t.Fatalf("profile applicability cue = %#v", result.ProfileApplicability.Cue)
	}
}

func TestBuildPublicSpecLifecycleReadsCurrentSQLEditionsBeforeCarriers(t *testing.T) {
	root := setupSpecSyncProject(t)
	admitSoftwareSpecLifecycleTestProfile(t, root, "spec-lifecycle-sql-first")
	database := openSpecSyncDB(t, root)
	defer database.Close()
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.status.001",
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "target.environment",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	projection := buildAutomaticPublicSpecLifecycleProjectionForTest(t, root)
	if projection.SectionID != "TS.sql.status.001" {
		t.Fatalf("SectionID = %q, want SQL edition section", projection.SectionID)
	}
}

func TestLoadProjectSpecificationSetSQLFirstPreservesCarrierTermMapEntries(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.term-map.001",
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "target.environment",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	specSet, err := loadProjectSpecificationSetSQLFirst(root)
	if err != nil {
		t.Fatalf("loadProjectSpecificationSetSQLFirst: %v", err)
	}
	if len(specSet.Sections) != 1 || specSet.Sections[0].ID != "TS.sql.term-map.001" {
		t.Fatalf("sections should come from SQL editions only: %#v", specSet.Sections)
	}
	if len(specSet.TermMapEntries) != 1 || specSet.TermMapEntries[0].Term != "HarnessableProject" {
		t.Fatalf("term-map entries should be preserved from typed carrier: %#v", specSet.TermMapEntries)
	}
	for _, document := range specSet.Documents {
		if document.Kind == project.SpecDocumentKindTermMap {
			return
		}
	}
	t.Fatalf("SQL-first spec set should retain typed term-map document: %#v", specSet.Documents)
}

func TestLoadProjectSpecificationSetSQLFirstIgnoresStaleSectionCarrierFindings(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.check.001",
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "target.environment",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	writeSpecCheckCLIFile(t, filepath.Join(root, ".haft", "specs", "target-system.md"), strings.Join([]string{
		"```yaml spec-section",
		"id: TS.carrier.stale.001",
		"kind: [",
		"```",
		"",
	}, "\n"))

	specificationSet, err := loadProjectSpecificationSetSQLFirst(root)
	if err != nil {
		t.Fatalf("loadProjectSpecificationSetSQLFirst: %v", err)
	}
	report := project.SpecCheckReportFromSpecificationSet(specificationSet)
	if report.HasFindings() {
		t.Fatalf("SQL-first spec check should ignore stale section carrier findings: %#v", report.Findings)
	}
	if report.Summary.SpecSections != 1 || report.Summary.ActiveSpecSections != 1 {
		t.Fatalf("summary = %#v, want one active SQL section", report.Summary)
	}
	if report.Summary.TermMapEntries != 1 {
		t.Fatalf("term-map entries = %d, want carrier support term-map", report.Summary.TermMapEntries)
	}
}

func TestSQLFirstLifecyclePreservesLegacyMigrationBlock(t *testing.T) {
	root := setupSpecSyncProject(t)
	admitSoftwareSpecLifecycleTestProfile(t, root, "spec-lifecycle-legacy-migration")
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.Remove(filepath.Join(specDir, "software-system.md")); err != nil {
		t.Fatal(err)
	}
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "enabling-system.md"), validCLISpecSectionCarrier("ES.architecture.001", "enabling.architecture"))

	database := openSpecSyncDB(t, root)
	defer database.Close()
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.migration.001",
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "target.environment",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	specSet, err := loadProjectSpecificationSetSQLFirst(root)
	if err != nil {
		t.Fatalf("loadProjectSpecificationSetSQLFirst: %v", err)
	}
	if !containsSpecFindingCode(specSet.Findings, project.SpecMigrationRequiredFindingCode) {
		t.Fatalf("findings = %#v, want migration block", specSet.Findings)
	}

	projection := buildAutomaticPublicSpecLifecycleProjectionForTest(t, root)
	if projection.Action != specflow.LifecycleActionClarify || projection.Carrier != ".haft/specs/enabling-system.md" {
		t.Fatalf("projection = %#v", projection)
	}
}

func containsSpecFindingCode(findings []project.SpecCheckFinding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func TestBuildPublicSpecLifecyclePropagatesCanonicalLedgerOpenError(t *testing.T) {
	root, _ := newBaselineTestProject(t)
	admitSoftwareSpecLifecycleTestProfile(t, root, "spec-lifecycle-ledger-error")
	makeBaselineDBUnopenable(t)

	_, err := buildPublicSpecLifecycle(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err == nil {
		t.Fatal("buildPublicSpecLifecycle ignored canonical ledger open error")
	}
}

func admitSoftwareSpecLifecycleTestProfile(
	t *testing.T,
	root string,
	suffix string,
) {
	t.Helper()
	harness := profileadmissionfixture.OpenExisting(t, root)
	harness.AdmitSoftwareRevisionWithTargetEntity(
		t,
		suffix,
		"entity:"+suffix+"-target",
	)
	if err := harness.Close(); err != nil {
		t.Fatalf("close profile admission fixture: %v", err)
	}
}

func buildAutomaticPublicSpecLifecycleProjectionForTest(
	t *testing.T,
	root string,
) specflow.SpecLifecycleProjection {
	t.Helper()
	result, err := buildPublicSpecLifecycle(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatalf("build public spec lifecycle: %v", err)
	}
	if result.SpecLifecycleProjection == nil {
		t.Fatalf(
			"resolved software profile omitted lifecycle projection: %#v",
			result.ProfileApplicability,
		)
	}
	if result.ProfileApplicability.Basis == nil {
		t.Fatalf(
			"resolved lifecycle omitted canonical profile basis: %#v",
			result.ProfileApplicability,
		)
	}
	return *result.SpecLifecycleProjection
}

func stubSpecStatusJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := specStatusJSON
	specStatusJSON = value
	return func() { specStatusJSON = prev }
}

func stubSpecNextJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := specNextJSON
	specNextJSON = value
	return func() { specNextJSON = prev }
}

func stubSpecNextScopeID(t *testing.T, value string) func() {
	t.Helper()
	prev := specNextScopeID
	specNextScopeID = value
	return func() { specNextScopeID = prev }
}
