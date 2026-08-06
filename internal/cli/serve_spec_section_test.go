package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

const (
	baselineTestSectionID = "TS.environment-change.001"
	baselineTestProjectID = "qnt_ba5e1e55"
)

func newBaselineTestProject(t *testing.T) (string, string) {
	t.Helper()

	homeDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	haftDir := filepath.Join(root, ".haft")
	specsDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configBody := "id: " + baselineTestProjectID + "\nname: baseline-test\n"
	if err := os.WriteFile(filepath.Join(haftDir, "project.yaml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}

	dbDir := filepath.Join(homeDir, ".haft", "projects", baselineTestProjectID)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := openCurrentKernelTestStore(
		filepath.Join(dbDir, "haft.db"),
	)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := projectledger.BindInitialized(
		context.Background(),
		root,
		time.Date(2026, time.July, 22, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("bind initialized baseline fixture: %v", err)
	}

	writeBaselineTestSection(t, root, "Initial environment statement")

	termMap := "```yaml term-map\nentries:\n  - term: HarnessableProject\n    category: target\n    definition: A repository ready for harness engineering.\n```\n"
	if err := os.WriteFile(filepath.Join(specsDir, "term-map.md"), []byte(termMap), 0o644); err != nil {
		t.Fatal(err)
	}

	return root, haftDir
}

func makeBaselineDBUnopenable(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(os.Getenv("HOME"), ".haft", "projects", baselineTestProjectID, "haft.db")
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove baseline db: %v", err)
	}
	if err := os.MkdirAll(dbPath, 0o755); err != nil {
		t.Fatalf("replace baseline db with directory: %v", err)
	}
}

func writeBaselineTestSection(t *testing.T, root, title string) {
	t.Helper()

	body := "## " + baselineTestSectionID + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + baselineTestSectionID + "\n" +
		"spec: target-system\n" +
		"kind: target.environment\n" +
		"title: " + title + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2026-12-31\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(root, ".haft", "specs", "target-system.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func overwriteSectionStatus(t *testing.T, root, status string) {
	t.Helper()

	path := filepath.Join(root, ".haft", "specs", "target-system.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "status: active", "status: "+status, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func overwriteSectionClaimLayer(t *testing.T, root, claimLayer string) {
	t.Helper()

	path := filepath.Join(root, ".haft", "specs", "target-system.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	updated := strings.Replace(content, "claim_layer: object", "claim_layer: "+claimLayer, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func removeTermMapCategory(t *testing.T, root string) {
	t.Helper()

	path := filepath.Join(root, ".haft", "specs", "term-map.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	updated := strings.Replace(content, "    category: target\n", "", 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mutateCarrierTitle(t *testing.T, root string) {
	t.Helper()
	writeBaselineTestSection(t, root, "Sharper environment statement")
}

func callHandleSpecSection(t *testing.T, haftDir string, args map[string]any) SpecSectionBaselineResult {
	t.Helper()

	raw, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, args)
	if err != nil {
		t.Fatalf("handleHaftSpecSectionWithProjectionRef: %v", err)
	}

	var result SpecSectionBaselineResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode baseline result: %v\nraw: %s", err, raw)
	}
	return result
}

func TestHandleHaftSpecSection_NextStepReturnsOneNeutralCueWithoutProfile(
	t *testing.T,
) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	haftDir := filepath.Join(fixture.root, ".haft")

	args := map[string]any{
		"action":       "next_step",
		"project_root": fixture.root,
	}

	result, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, args)
	if err != nil {
		t.Fatalf("handleHaftSpecSectionWithProjectionRef returned error: %v", err)
	}

	var next publicSpecNextStepResult
	if err := json.Unmarshal([]byte(result), &next); err != nil {
		t.Fatalf("decode next step: %v\nraw: %s", err, result)
	}

	if next.WorkflowIntent != nil {
		t.Fatalf("underdetermined profile fabricated next step: %#v", next)
	}
	if next.ProfileApplicability.Kind !=
		string(projectSpecificationProfileUnderdetermined) ||
		next.ProfileApplicability.Cue == nil {
		t.Fatalf("profile applicability = %#v", next.ProfileApplicability)
	}
}

func TestHandleHaftSpecSection_NextStepPropagatesBaselineStoreError(t *testing.T) {
	_, haftDir := newBaselineTestProject(t)
	makeBaselineDBUnopenable(t)

	_, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action": "next_step",
	})
	if err == nil {
		t.Fatal("next_step ignored baseline store error")
	}
}

func TestHandleHaftSpecSection_ProjectLifecycleRejectsExactSectionID(
	t *testing.T,
) {
	t.Parallel()

	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	for _, action := range []string{"lifecycle", "next_step"} {
		t.Run(action, func(t *testing.T) {
			_, _, err := handleHaftSpecSectionWithProjectionRef(
				context.Background(),
				haftDir,
				map[string]any{
					"action":     action,
					"section_id": "SS.interfaces.code-graph.001",
				},
			)
			if err == nil {
				t.Fatalf("%s silently ignored section_id", action)
			}
			for _, required := range []string{
				"section_id_not_applicable",
				"project/scope-level",
				`haft_query(action="spec_trace"`,
				`haft_query(action="spec_use"`,
				"SS.interfaces.code-graph.001",
			} {
				if !strings.Contains(err.Error(), required) {
					t.Fatalf(
						"%s error lacks %q: %v",
						action,
						required,
						err,
					)
				}
			}
		})
	}
}

func TestHandleHaftSpecSection_LifecycleReturnsOneNeutralCueWithoutProfile(
	t *testing.T,
) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	haftDir := filepath.Join(fixture.root, ".haft")

	result, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action":       "lifecycle",
		"project_root": fixture.root,
	})
	if err != nil {
		t.Fatalf("handleHaftSpecSectionWithProjectionRef returned error: %v", err)
	}

	var lifecycle publicSpecLifecycleResult
	if err := json.Unmarshal([]byte(result), &lifecycle); err != nil {
		t.Fatalf("decode lifecycle: %v\nraw: %s", err, result)
	}

	if lifecycle.SpecLifecycleProjection != nil {
		t.Fatalf("underdetermined profile fabricated lifecycle: %#v", lifecycle)
	}
	if lifecycle.ProfileApplicability.Kind !=
		string(projectSpecificationProfileUnderdetermined) ||
		lifecycle.ProfileApplicability.Cue == nil {
		t.Fatalf("profile applicability = %#v", lifecycle.ProfileApplicability)
	}
}

func TestHandleHaftSpecSection_DefaultsToServerBoundProjectRoot(t *testing.T) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	haftDir := filepath.Join(fixture.root, ".haft")

	// project_root not provided — handler should derive from haftDir parent.
	args := map[string]any{
		"action": "next_step",
	}

	result, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, args)
	if err != nil {
		t.Fatalf("handleHaftSpecSectionWithProjectionRef returned error: %v", err)
	}

	var next publicSpecNextStepResult
	if err := json.Unmarshal([]byte(result), &next); err != nil {
		t.Fatalf("decode next step: %v", err)
	}

	if next.ProfileApplicability.ProjectRoot != fixture.root {
		t.Fatalf(
			"project root = %q, want server-bound %q",
			next.ProfileApplicability.ProjectRoot,
			fixture.root,
		)
	}
}

func TestHandleHaftSpecSection_NextStepStartsTargetSpecWithoutEntityRelation(
	t *testing.T,
) {
	root := mustCLIProfileOnboardPhysicalPath(t, t.TempDir())
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "spec-next")
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(filepath.Join(haftDir, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}

	raw, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action":       "next_step",
		"project_root": root,
	})
	if err != nil {
		t.Fatalf("next_step: %v", err)
	}
	var next publicSpecNextStepResult
	if err := json.Unmarshal([]byte(raw), &next); err != nil {
		t.Fatalf("decode next step: %v\nraw: %s", err, raw)
	}
	if next.WorkflowIntent == nil {
		t.Fatalf("resolved applicability omitted next-step projection: %#v", next)
	}
	if next.WorkflowIntent.Phase != specflow.PhaseTargetEnvironmentDraft ||
		next.WorkflowIntent.ApplicabilityCue != nil {
		t.Fatalf(
			"next step did not start target-system lifecycle: %#v; raw = %s",
			next,
			raw,
		)
	}
	if next.ProfileApplicability.ScopeID != "software-spec-next" {
		t.Fatalf(
			"scope_id = %q, want software-spec-next",
			next.ProfileApplicability.ScopeID,
		)
	}
}

func TestProjectSpecificationScopeRequestFromSpecSectionArgs(t *testing.T) {
	t.Parallel()

	automatic, err := projectSpecificationScopeRequestFromSpecSectionArgs(
		map[string]any{},
	)
	if err != nil || automatic.kind != projectSpecificationScopeAutomatic {
		t.Fatalf("automatic request = %#v, err = %v", automatic, err)
	}
	exact, err := projectSpecificationScopeRequestFromSpecSectionArgs(
		map[string]any{"scope_id": "documents"},
	)
	if err != nil ||
		exact.kind != projectSpecificationScopeExact ||
		exact.scopeID.String() != "documents" {
		t.Fatalf("exact request = %#v, err = %v", exact, err)
	}
	for _, args := range []map[string]any{
		{"scope_id": " documents "},
		{"scope_id": 17},
	} {
		if _, err := projectSpecificationScopeRequestFromSpecSectionArgs(args); err == nil {
			t.Fatalf("invalid scope request accepted: %#v", args)
		}
	}
}

func TestHandleHaftSpecSection_RejectsMissingAction(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{})
	if err == nil {
		t.Fatalf("handleHaftSpecSectionWithProjectionRef should reject missing action")
	}
}

func TestHandleHaftSpecSection_RejectsUnknownAction(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{"action": "vibe-check"})
	if err == nil {
		t.Fatalf("handleHaftSpecSectionWithProjectionRef should reject unknown action")
	}
}

func TestHandleHaftSpecSection_ApproveRecordsBaselineForActiveSection(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	admitSoftwareSpecLifecycleTestProfile(t, root, "spec-section-approve-lifecycle")

	result := callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
		"approved_by":  "human",
	})

	if result.SectionID != baselineTestSectionID {
		t.Fatalf("section_id = %q, want %q", result.SectionID, baselineTestSectionID)
	}
	if result.Hash == "" {
		t.Fatalf("hash should be recorded; got empty result: %#v", result)
	}
	if result.BaselineKind != specflow.BaselineKindSpecSectionApproval {
		t.Fatalf("baseline_kind = %q, want %q", result.BaselineKind, specflow.BaselineKindSpecSectionApproval)
	}
	if result.BaselineProfile == nil {
		t.Fatal("baseline_profile missing")
	}
	if result.BaselineProfile.Object != "SpecSectionApprovalBaseline" {
		t.Fatalf("baseline_profile.object = %q", result.BaselineProfile.Object)
	}
	if result.ApprovedBy != "human" {
		t.Fatalf("approved_by = %q, want human", result.ApprovedBy)
	}

	// next_step should now advance past the environment phase.
	projection := buildAutomaticPublicSpecLifecycleProjectionForTest(t, root)
	intent := projection.WorkflowIntent
	if intent.Phase == specflow.PhaseTargetEnvironmentDraft && intent.Audience == "human" {
		t.Fatalf("environment phase still blocking after approve: %#v", intent)
	}
}

func TestHandleHaftSpecSection_ApproveReadsCurrentSQLEditionsBeforeCarriers(t *testing.T) {
	root := setupSpecSyncProject(t)
	haftDir := filepath.Join(root, ".haft")
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.approve.001",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "SQL approve section",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		ValidUntil:    "2026-12-31",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	result := callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   "TS.sql.approve.001",
		"approved_by":  "human",
	})

	if result.SectionID != "TS.sql.approve.001" {
		t.Fatalf("section_id = %q, want SQL edition section", result.SectionID)
	}
	if result.Hash != specflow.HashSection(section) {
		t.Fatalf("hash = %q, want SQL section hash %q", result.Hash, specflow.HashSection(section))
	}
	if result.BaselineKind != specflow.BaselineKindSpecSectionApproval {
		t.Fatalf("baseline_kind = %q, want %q", result.BaselineKind, specflow.BaselineKindSpecSectionApproval)
	}
}

func TestHandleHaftSpecSection_ApproveRefusesDraftSection(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	overwriteSectionStatus(t, root, "draft")

	_, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})
	if err == nil {
		t.Fatalf("approve should refuse a draft section")
	}
}

func TestHandleHaftSpecSection_ApproveRefusesSpecCheckFindings(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	overwriteSectionClaimLayer(t, root, "carrier")

	_, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})
	if err == nil {
		t.Fatalf("approve should refuse when spec check has findings")
	}
	if !strings.Contains(err.Error(), "spec_section_mixed_authority") {
		t.Fatalf("approve error = %v, want spec_section_mixed_authority", err)
	}

	store, projectID, closeFn, err := projectBaseline(root)
	defer closeFn()
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Get(projectID, baselineTestSectionID)
	if !errors.Is(err, specflow.ErrBaselineNotFound) {
		t.Fatalf("baseline lookup err = %v, want ErrBaselineNotFound", err)
	}
}

func TestHandleHaftSpecSection_ApproveRefusesTermMapFindings(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	removeTermMapCategory(t, root)

	_, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})
	if err == nil {
		t.Fatalf("approve should refuse when term-map check has findings")
	}
	if !strings.Contains(err.Error(), "term_map_missing_category") {
		t.Fatalf("approve error = %v, want term_map_missing_category", err)
	}

	store, projectID, closeFn, err := projectBaseline(root)
	defer closeFn()
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Get(projectID, baselineTestSectionID)
	if !errors.Is(err, specflow.ErrBaselineNotFound) {
		t.Fatalf("baseline lookup err = %v, want ErrBaselineNotFound", err)
	}
}

func TestHandleHaftSpecSection_ApproveRefusesWhenBaselineDiffersAndNoRebaseline(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)

	// First approve to lay a baseline.
	_ = callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})

	// Mutate the carrier so the hash diverges.
	mutateCarrierTitle(t, root)

	_, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})
	if err == nil {
		t.Fatalf("approve should refuse when baseline already exists with a different hash")
	}
}

func TestHandleHaftSpecSection_RebaselineRequiresReason(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	_, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action":       "rebaseline",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})
	if err == nil {
		t.Fatalf("rebaseline should require reason")
	}
}

func TestHandleHaftSpecSection_ReopenRequiresReason(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	_, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action":       "reopen",
		"project_root": root,
		"section_id":   baselineTestSectionID,
		"reason":       "   ",
	})
	if err == nil {
		t.Fatal("reopen accepted empty reason")
	}
	if !strings.Contains(err.Error(), "reason is required for reopen") {
		t.Fatalf("error = %v, want reopen reason requirement", err)
	}
}

func TestHandleHaftSpecSection_RebaselineOverwritesAndReportsReason(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	_ = callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})

	mutateCarrierTitle(t, root)

	result := callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "rebaseline",
		"project_root": root,
		"section_id":   baselineTestSectionID,
		"reason":       "valid evolution: tightened title",
	})

	if result.Reason == "" {
		t.Fatalf("rebaseline result must echo reason: %#v", result)
	}
	if result.BaselineKind != specflow.BaselineKindSpecSectionApproval {
		t.Fatalf("baseline_kind = %q, want %q", result.BaselineKind, specflow.BaselineKindSpecSectionApproval)
	}
	if result.BaselineProfile == nil {
		t.Fatal("baseline_profile missing")
	}
	if result.Hash == "" {
		t.Fatalf("rebaseline result must include new hash: %#v", result)
	}
}

func TestHandleHaftSpecSection_ReopenDeletesBaselineAndBlocksNextStep(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	admitSoftwareSpecLifecycleTestProfile(t, root, "spec-section-reopen-lifecycle")
	_ = callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})

	result := callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "reopen",
		"project_root": root,
		"section_id":   baselineTestSectionID,
		"reason":       "needs review",
	})
	if result.BaselineKind != specflow.BaselineKindSpecSectionApproval {
		t.Fatalf("baseline_kind = %q, want %q", result.BaselineKind, specflow.BaselineKindSpecSectionApproval)
	}

	projection := buildAutomaticPublicSpecLifecycleProjectionForTest(t, root)
	intent := projection.WorkflowIntent
	if intent.Phase != specflow.PhaseTargetEnvironmentDraft || intent.Audience != "human" {
		t.Fatalf("expected environment phase to block after reopen; got %#v", intent)
	}
}
