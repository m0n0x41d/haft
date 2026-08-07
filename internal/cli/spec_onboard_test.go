package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestRunSpecOnboardJSONReturnsOneNeutralCueWithoutCanonicalProfile(
	t *testing.T,
) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreJSON := stubSpecOnboardJSON(t, true)
	defer restoreJSON()
	restoreScopeID := stubSpecOnboardScopeID(t, "")
	defer restoreScopeID()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecOnboard(cmd, nil); err != nil {
		t.Fatalf("runSpecOnboard returned error: %v", err)
	}

	var result publicSpecNextStepResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", err, output.String())
	}
	if result.WorkflowIntent != nil {
		t.Fatalf("underdetermined profile fabricated next step: %#v", result)
	}
	if result.ProfileApplicability.Kind !=
		string(projectSpecificationProfileUnderdetermined) ||
		result.ProfileApplicability.Cue == nil {
		t.Fatalf(
			"profile applicability = %#v",
			result.ProfileApplicability,
		)
	}
	if result.ProfileApplicability.Cue.RecoverySurface !=
		projectSpecificationProfileRecoverySurface ||
		result.ProfileApplicability.Cue.NextAction !=
			projectSpecificationProfileRecoveryNextAction {
		t.Fatalf(
			"profile recovery cue = %#v",
			result.ProfileApplicability.Cue,
		)
	}
}

func TestRunSpecOnboardSummaryRendersOneNeutralProfileCue(t *testing.T) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreJSON := stubSpecOnboardJSON(t, false)
	defer restoreJSON()
	restoreScopeID := stubSpecOnboardScopeID(t, "")
	defer restoreScopeID()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecOnboard(cmd, nil); err != nil {
		t.Fatalf("runSpecOnboard returned error: %v", err)
	}

	got := output.String()
	for _, want := range []string{
		"haft spec onboard: not evaluated (profile_underdetermined)",
		"Profile cue:",
		"Missing basis: current_canonical_profile_admission",
		"Recovery surface: haft_onboard",
		"Next: " + projectSpecificationProfileRecoveryNextAction,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Count(got, "Profile cue:") != 1 {
		t.Fatalf("onboard emitted repeated profile cues:\n%s", got)
	}
	if strings.Contains(got, "software-system") {
		t.Fatalf("onboard emitted speculative software pressure:\n%s", got)
	}
}

func TestRunSpecOnboardReadsCurrentSQLEditionsBeforeCarriers(t *testing.T) {
	root := setupSpecSyncProject(t)
	profileHarness := profileadmissionfixture.OpenExisting(t, root)
	profileHarness.AdmitSoftwareRevisionWithTargetEntity(
		t,
		"spec-onboard-sql",
		"entity:spec-onboard-target",
	)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.onboard.001",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "SQL onboard section",
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

	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()
	restoreJSON := stubSpecOnboardJSON(t, true)
	defer restoreJSON()
	restoreScopeID := stubSpecOnboardScopeID(t, "")
	defer restoreScopeID()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecOnboard(cmd, nil); err != nil {
		t.Fatalf("runSpecOnboard returned error: %v", err)
	}

	var result publicSpecNextStepResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", err, output.String())
	}
	if result.WorkflowIntent == nil {
		t.Fatalf("resolved profile omitted workflow intent: %#v", result)
	}
	intent := result.WorkflowIntent
	if len(intent.BlockingFindings) == 0 {
		t.Fatalf("expected missing-baseline finding for SQL edition: %#v", intent)
	}
	if intent.BlockingFindings[0].SectionID != "TS.sql.onboard.001" {
		t.Fatalf("onboard read carrier section instead of SQL edition: %#v", intent.BlockingFindings)
	}
}

func TestRunSpecOnboardJSONMatchesMCPNextStepKeys(t *testing.T) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	root := fixture.root
	haftDir := filepath.Join(root, ".haft")

	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()
	restoreJSON := stubSpecOnboardJSON(t, true)
	defer restoreJSON()
	restoreScopeID := stubSpecOnboardScopeID(t, "")
	defer restoreScopeID()

	var cliOutput bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&cliOutput)
	if err := runSpecOnboard(cmd, nil); err != nil {
		t.Fatalf("runSpecOnboard returned error: %v", err)
	}

	mcpOutput, _, err := handleHaftSpecSectionWithProjectionRef(context.Background(), haftDir, map[string]any{
		"action":       "next_step",
		"project_root": root,
	})
	if err != nil {
		t.Fatalf("next_step returned error: %v", err)
	}

	cliKeys := jsonObjectKeys(t, cliOutput.Bytes())
	mcpKeys := jsonObjectKeys(t, []byte(mcpOutput))
	if !reflect.DeepEqual(cliKeys, mcpKeys) {
		t.Fatalf("CLI/MCP WorkflowIntent keys differ:\nCLI=%v\nMCP=%v", cliKeys, mcpKeys)
	}
}

func jsonObjectKeys(t *testing.T, raw []byte) []string {
	t.Helper()

	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode JSON object: %v\nraw: %s", err, string(raw))
	}

	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stubSpecOnboardJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := specOnboardJSON
	specOnboardJSON = value
	return func() { specOnboardJSON = prev }
}

func stubSpecOnboardScopeID(t *testing.T, value string) func() {
	t.Helper()
	prev := specOnboardScopeID
	specOnboardScopeID = value
	return func() { specOnboardScopeID = prev }
}
