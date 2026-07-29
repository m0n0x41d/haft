package cli

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestWorkCommissionSpecAuthorityForApplicability(
	t *testing.T,
) {
	tests := []struct {
		name             string
		realizationClass projectprofile.RealizationClass
		commission       map[string]any
		specSet          project.ProjectSpecificationSet
		wantErrorCode    string
		wantCue          bool
		wantSnapshot     bool
		wantSnapshotRef  string
	}{
		{
			name:             "non-software needs no fake software waiver",
			realizationClass: projectprofile.NonSoftwareRealizationClass,
			commission:       map[string]any{},
			wantSnapshot:     true,
		},
		{
			name:             "software still requires applicable realization authority",
			realizationClass: projectprofile.SoftwareRealizationClass,
			commission:       map[string]any{},
			wantErrorCode:    commissionSpecRefsRequiredCode,
		},
		{
			name:             "software accepts a current software section",
			realizationClass: projectprofile.SoftwareRealizationClass,
			commission:       commissionWithSpecReference("SS.behavior.001"),
			specSet: project.ProjectSpecificationSet{
				Sections: []project.SpecSection{{
					ID: "SS.behavior.001",
					DocumentKind: string(
						project.SpecDocumentKindSoftwareSystem,
					),
				}},
			},
			wantSnapshot:    true,
			wantSnapshotRef: "SS.behavior.001",
		},
		{
			name:             "non-software rejects an explicit software reliance",
			realizationClass: projectprofile.NonSoftwareRealizationClass,
			commission:       commissionWithSpecReference("SS.behavior.001"),
			specSet: project.ProjectSpecificationSet{
				Sections: []project.SpecSection{{
					ID: "SS.behavior.001",
					DocumentKind: string(
						project.SpecDocumentKindSoftwareSystem,
					),
				}},
			},
			wantErrorCode: commissionSpecRefNotApplicableCode,
		},
		{
			name:             "target uncertainty blocks only explicit target reliance",
			realizationClass: projectprofile.NonSoftwareRealizationClass,
			commission:       commissionWithSpecReference("TS.boundary.001"),
			specSet: project.ProjectSpecificationSet{
				Sections: []project.SpecSection{{
					ID: "TS.boundary.001",
					DocumentKind: string(
						project.SpecDocumentKindTargetSystem,
					),
				}},
			},
			wantErrorCode: commissionSpecApplicabilityUnderdeterminedCode,
			wantCue:       true,
		},
		{
			name:             "tactical override preserves software authority boundary",
			realizationClass: projectprofile.SoftwareRealizationClass,
			commission: map[string]any{
				"spec_readiness_override": map[string]any{
					"kind":        "tactical",
					"out_of_spec": true,
					"reason":      "bounded repair before carrier admission",
				},
			},
			wantSnapshot: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			applicability := mustCommissionSpecApplicability(
				t,
				test.realizationClass,
			)
			err := ensureWorkCommissionSpecAuthorityForApplicability(
				test.commission,
				time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
				applicability,
				test.specSet,
			)
			assertCommissionSpecApplicabilityError(
				t,
				err,
				test.wantErrorCode,
				test.wantCue,
				applicability,
			)
			if test.wantSnapshot {
				assertScopeLocalCommissionSpecSnapshot(
					t,
					test.commission,
					applicability,
					test.wantSnapshotRef,
				)
			}
		})
	}
}

func TestValidateWorkCommissionSpecAuthorityForApplicabilityIsPure(
	t *testing.T,
) {
	applicability := mustCommissionSpecApplicability(
		t,
		projectprofile.NonSoftwareRealizationClass,
	)
	commission := map[string]any{}

	refs, err := validateWorkCommissionSpecAuthorityForApplicability(
		commission,
		applicability,
		project.ProjectSpecificationSet{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("refs = %#v, want none", refs)
	}
	if len(commission) != 0 {
		t.Fatalf("pure validation mutated commission: %#v", commission)
	}
}

func TestHandleHaftCommissionForProjectAdmitsNonSoftwareWithoutFakeSWESpec(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	admission := harness.AdmitNonSoftwareRevision(
		t,
		"commission-nonsoftware",
	)
	store := artifact.NewStore(harness.Database())
	commission := workCommissionFixture(
		"wc-profile-nonsoftware",
		"queued",
		"2099-01-01T00:00:00Z",
	)
	delete(commission, "spec_readiness_override")

	raw, err := handleHaftCommissionForProject(
		context.Background(),
		store,
		map[string]any{
			"action":       "create",
			"project_root": root,
			"commission":   commission,
		},
	)
	if err != nil {
		t.Fatalf("create non-software WorkCommission: %v", err)
	}
	var result map[string]map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode WorkCommission result: %v", err)
	}
	created := result["commission"]
	snapshot, ok := mapArg(created, "spec_snapshot")
	if !ok {
		t.Fatalf("scope-local snapshot missing: %#v", created)
	}
	if snapshot["scope_id"] !=
		"non-software-commission-nonsoftware" {
		t.Fatalf("scope_id = %#v", snapshot["scope_id"])
	}
	if snapshot["profile_admission_record_ref"] !=
		admission.AdmissionRecordRef().String() {
		t.Fatalf(
			"profile admission ref = %#v, want %q",
			snapshot["profile_admission_record_ref"],
			admission.AdmissionRecordRef().String(),
		)
	}
	if refs := workCommissionSpecSectionRefs(created); len(refs) != 0 {
		t.Fatalf("non-software commission gained fake spec refs: %#v", refs)
	}
	if _, found := created["spec_readiness_override"]; found {
		t.Fatalf(
			"non-software commission gained a fake tactical waiver: %#v",
			created["spec_readiness_override"],
		)
	}
}

func TestHandleHaftCommissionForProjectKeepsSoftwareAuthorityFailClosed(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "commission-software")
	store := artifact.NewStore(harness.Database())
	commission := workCommissionFixture(
		"wc-profile-software",
		"queued",
		"2099-01-01T00:00:00Z",
	)
	delete(commission, "spec_readiness_override")

	_, err := handleHaftCommissionForProject(
		context.Background(),
		store,
		map[string]any{
			"action":       "create",
			"project_root": root,
			"commission":   commission,
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), commissionSpecRefsRequiredCode) {
		t.Fatalf(
			"software WorkCommission error = %v, want %q",
			err,
			commissionSpecRefsRequiredCode,
		)
	}
	if _, loadErr := store.Get(
		context.Background(),
		"wc-profile-software",
	); loadErr == nil {
		t.Fatal("failed software authority check still persisted a commission")
	}
}

func TestHandleHaftCommissionForProjectRevalidatesProfileBeforeStart(
	t *testing.T,
) {
	tests := []struct {
		name      string
		drift     bool
		wantState string
		wantCode  string
	}{
		{
			name:      "current profile starts",
			wantState: "running",
		},
		{
			name:      "changed profile blocks",
			drift:     true,
			wantState: "blocked_stale",
			wantCode:  commissionProfileApplicabilityChangedCode,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			harness := profileadmissionfixture.New(t, root)
			harness.AdmitNonSoftwareRevision(t, "commission-start")
			store := artifact.NewStore(harness.Database())
			ctx := context.Background()
			commissionID := createProfileAwareClaimedCommission(
				t,
				ctx,
				store,
				root,
			)
			if test.drift {
				harness.AdmitNonSoftwareRevision(
					t,
					"commission-start-next",
				)
			}

			_, err := handleHaftCommissionForProject(
				ctx,
				store,
				map[string]any{
					"action":        "start_after_preflight",
					"commission_id": commissionID,
					"runner_id":     "profile-aware-test",
					"event":         "preflight_passed",
					"verdict":       "pass",
					"project_root":  root,
				},
			)
			if test.wantCode == "" && err != nil {
				t.Fatalf("start current WorkCommission: %v", err)
			}
			if test.wantCode != "" &&
				(err == nil || !strings.Contains(err.Error(), test.wantCode)) {
				t.Fatalf(
					"start error = %v, want %q",
					err,
					test.wantCode,
				)
			}
			stored, loadErr := store.Get(ctx, commissionID)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			payload := map[string]any{}
			if err := json.Unmarshal(
				[]byte(stored.StructuredData),
				&payload,
			); err != nil {
				t.Fatal(err)
			}
			if stringField(payload, "state") != test.wantState {
				t.Fatalf(
					"state = %q, want %q",
					stringField(payload, "state"),
					test.wantState,
				)
			}
		})
	}
}

func TestCommissionArgsWithProjectRootOverridesCallerSuppliedRoot(
	t *testing.T,
) {
	caller := map[string]any{
		"action":       "create",
		"project_root": "/attacker-controlled",
		"scope_id":     "software-main",
	}
	bound := commissionArgsWithProjectRoot(caller, "/trusted/project")
	if stringArg(bound, "project_root") != "/trusted/project" {
		t.Fatalf("project_root = %q", stringArg(bound, "project_root"))
	}
	if stringArg(bound, "scope_id") != "software-main" {
		t.Fatalf("scope_id = %q", stringArg(bound, "scope_id"))
	}
	if stringArg(caller, "project_root") != "/attacker-controlled" {
		t.Fatalf("caller args were mutated: %#v", caller)
	}
}

func TestCommissionCreationCommandsExposeExactScopeSelector(t *testing.T) {
	commands := []*cobra.Command{
		commissionCreateCmd,
		commissionCreateFromDecisionCmd,
		commissionCreateBatchCmd,
		commissionCreateFromPlanCmd,
		harnessPlanCmd,
		harnessRunCmd,
	}
	for _, command := range commands {
		if command.Flags().Lookup("scope-id") == nil {
			t.Fatalf("%s omitted --scope-id", command.CommandPath())
		}
	}
}

func createProfileAwareClaimedCommission(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
) string {
	t.Helper()
	haftDir := filepath.Join(projectRoot, ".haft")
	problem, _, err := artifact.FrameProblem(
		ctx,
		store,
		haftDir,
		artifact.ProblemFrameInput{
			Title:      "Profile-aware commission start",
			Signal:     "Profile authority can change after commission creation.",
			Acceptance: "Execution starts only against the admitted profile snapshot.",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	decision, _, err := artifact.Decide(
		ctx,
		store,
		haftDir,
		artifact.DecideInput{
			ProblemRef:      problem.Meta.ID,
			SelectedTitle:   "Revalidate profile before execution",
			WhySelected:     "Current-use authority must match the commission snapshot.",
			SelectionPolicy: "Fail closed only at start when profile authority changed.",
			CounterArgument: "Creation-time validation may be sufficient.",
			WeakestLink:     "A concurrent profile admission can invalidate the scope.",
			WhyNotOthers: []artifact.RejectionReason{{
				Variant: "Trust creation snapshot forever",
				Reason:  "It ignores later canonical profile admissions.",
			}},
			Rollback: &artifact.RollbackSpec{
				Triggers: []string{"Current profile cannot be resolved deterministically."},
			},
			EvidenceReqs:  []string{"go test ./internal/cli"},
			AffectedFiles: []string{"internal/cli/serve_commission.go"},
			ValidUntil:    "2099-01-01T00:00:00Z",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := handleHaftCommissionForProject(
		ctx,
		store,
		map[string]any{
			"action":        "create_from_decision",
			"decision_ref":  decision.Meta.ID,
			"repo_ref":      "local:profile-aware-test",
			"base_sha":      "profile-aware-base",
			"target_branch": "test",
			"project_root":  projectRoot,
			"valid_until":   "2099-01-01T00:00:00Z",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	created := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(raw), &created); err != nil {
		t.Fatal(err)
	}
	commissionID := stringField(created["commission"], "id")
	if commissionID == "" {
		t.Fatalf("created WorkCommission omitted id: %#v", created)
	}
	_, err = handleHaftCommission(
		ctx,
		store,
		map[string]any{
			"action":        "claim_for_preflight",
			"commission_id": commissionID,
			"runner_id":     "profile-aware-test",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return commissionID
}

func commissionWithSpecReference(
	ref string,
) map[string]any {
	return map[string]any{
		"spec_section_refs": []any{ref},
	}
}

func mustCommissionSpecApplicability(
	t *testing.T,
	realizationClass projectprofile.RealizationClass,
) project.ProjectSpecificationSetApplicability {
	t.Helper()

	const scopeName = "commission-scope"
	var scope projectprofile.RealizationScope
	switch realizationClass {
	case projectprofile.SoftwareRealizationClass:
		scope = mustCLIProjectSoftwareScope(t, scopeName)
	case projectprofile.NonSoftwareRealizationClass:
		scope = mustCLIProjectNonSoftwareScope(t, scopeName)
	default:
		t.Fatalf("unsupported realization class %q", realizationClass)
	}

	matrix := mustCLIProjectCapabilityMatrix(
		t,
		[]projectprofile.RealizationScope{scope},
	)
	scopeID, err := projectprofile.NewScopeID(scopeName)
	if err != nil {
		t.Fatal(err)
	}
	applicability, err := project.DeriveProjectSpecificationSetApplicability(
		matrix,
		scopeID,
	)
	if err != nil {
		t.Fatal(err)
	}
	return applicability
}

func assertCommissionSpecApplicabilityError(
	t *testing.T,
	err error,
	wantCode string,
	wantCue bool,
	applicability project.ProjectSpecificationSetApplicability,
) {
	t.Helper()
	if wantCode == "" {
		if err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), wantCode) {
		t.Fatalf("error = %v, want code %q", err, wantCode)
	}
	if !wantCue {
		return
	}

	var cue *commissionSpecApplicabilityCueError
	if !errors.As(err, &cue) {
		t.Fatalf("error = %T %v, want typed applicability cue", err, err)
	}
	if cue.ScopeID != applicability.ScopeID().String() ||
		cue.ProfilePayloadDigest !=
			applicability.ProfilePayloadDigest().String() {
		t.Fatalf("cue provenance = %#v, want supplied applicability", cue)
	}
	if cue.Issue.Capability !=
		projectprofile.TargetSystemSpecCapability ||
		cue.Issue.MissingBasis !=
			projectprofile.MissingAdmittedTargetSystemRelation {
		t.Fatalf("cue issue = %#v, want exact target basis", cue.Issue)
	}
}

func assertScopeLocalCommissionSpecSnapshot(
	t *testing.T,
	commission map[string]any,
	applicability project.ProjectSpecificationSetApplicability,
	wantRef string,
) {
	t.Helper()
	snapshot, ok := mapArg(commission, "spec_snapshot")
	if !ok {
		t.Fatalf("spec_snapshot = %#v, want object", commission["spec_snapshot"])
	}
	if snapshot["snapshot_source"] !=
		"scope_local_project_specification_set" {
		t.Fatalf("snapshot_source = %#v", snapshot["snapshot_source"])
	}
	if snapshot["scope_id"] != applicability.ScopeID().String() ||
		snapshot["profile_payload_digest"] !=
			applicability.ProfilePayloadDigest().String() {
		t.Fatalf("snapshot provenance = %#v", snapshot)
	}
	if wantRef != "" &&
		!containsAnyString(snapshot["section_refs"], wantRef) {
		t.Fatalf("snapshot refs = %#v", snapshot["section_refs"])
	}
}
