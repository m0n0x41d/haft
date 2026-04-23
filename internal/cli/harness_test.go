package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestBuildHarnessPlanSequentialDependencies(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	harnessPlanID = "plan-test"
	harnessPlanTitle = "Test harness plan"
	harnessPlanSequential = true
	commissionFromDecisionRepoRef = "local:test"
	commissionFromDecisionBaseSHA = "base-r1"
	commissionFromDecisionTargetBranch = "dev"
	commissionFromDecisionEvidence = []string{"go test ./internal/cli"}

	plan, err := buildHarnessPlan(t.TempDir(), []string{"dec-a", "dec-b", "dec-c"})
	if err != nil {
		t.Fatal(err)
	}

	if plan.ID != "plan-test" {
		t.Fatalf("plan id = %s, want plan-test", plan.ID)
	}
	if plan.Decisions[0].Ref != "dec-a" {
		t.Fatalf("first decision = %#v, want dec-a", plan.Decisions[0])
	}
	if len(plan.Decisions[0].DependsOn) != 0 {
		t.Fatalf("first depends_on = %#v, want empty", plan.Decisions[0].DependsOn)
	}
	if !stringSliceContains(plan.Decisions[1].DependsOn, "dec-a") {
		t.Fatalf("second depends_on = %#v, want dec-a", plan.Decisions[1].DependsOn)
	}
	if !stringSliceContains(plan.Decisions[2].DependsOn, "dec-b") {
		t.Fatalf("third depends_on = %#v, want dec-b", plan.Decisions[2].DependsOn)
	}
	if !stringSliceContains(plan.Defaults.EvidenceRequirements, "go test ./internal/cli") {
		t.Fatalf("evidence = %#v, want go test", plan.Defaults.EvidenceRequirements)
	}
}

func TestBuildHarnessPlanRejectsDependencyOutsidePlan(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	harnessPlanDependencies = []string{"dec-b:dec-missing"}
	commissionFromDecisionRepoRef = "local:test"
	commissionFromDecisionBaseSHA = "base-r1"
	commissionFromDecisionTargetBranch = "dev"

	_, err := buildHarnessPlan(t.TempDir(), []string{"dec-a", "dec-b"})
	if err == nil || !strings.Contains(err.Error(), "dependency source dec-missing") {
		t.Fatalf("error = %v, want missing dependency rejection", err)
	}
}

func TestResolveHarnessDecisionRefsByProblem(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Harness selector",
		Signal:     "The operator should run all decisions for one problem.",
		Acceptance: "Problem selector returns linked decisions.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "Problem selector decision",
		WhySelected:     "It belongs to the selected problem.",
		SelectionPolicy: "Keep problem-scoped harness runs explicit.",
		CounterArgument: "Listing ids is simpler.",
		WeakestLink:     "Selection must not pull unrelated decisions.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Manual ids",
			Reason:  "Too much operator ceremony.",
		}},
		Rollback:      &artifact.RollbackSpec{Triggers: []string{"Problem selector regresses."}},
		AffectedFiles: []string{"internal/cli/harness.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_ = createCommissionDecisionFixture(t, ctx, store, haftDir, "Unrelated selector decision", "internal/cli/run.go")

	harnessPlanProblems = []string{problem.Meta.ID}

	refs, err := resolveHarnessDecisionRefs(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one problem-linked decision", refs)
	}
	if refs[0] != decision.Meta.ID {
		t.Fatalf("refs = %#v, want [%s]", refs, decision.Meta.ID)
	}
}

func TestResolveHarnessDecisionRefsByContext(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	firstProblem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Context:    "mvp-harness",
		Title:      "Harness context first",
		Signal:     "The operator should run a context-scoped workset.",
		Acceptance: "Context selector returns linked decisions.",
	})
	if err != nil {
		t.Fatal(err)
	}

	secondProblem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Context:    "mvp-harness",
		Title:      "Harness context second",
		Signal:     "The operator should run a context-scoped workset.",
		Acceptance: "Context selector returns linked decisions.",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      firstProblem.Meta.ID,
		SelectedTitle:   "First context decision",
		WhySelected:     "It belongs to the selected context.",
		SelectionPolicy: "Keep problem-scoped harness runs explicit.",
		CounterArgument: "Listing ids is simpler.",
		WeakestLink:     "Selection must not pull unrelated decisions.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Manual ids",
			Reason:  "Too much operator ceremony.",
		}},
		Rollback:      &artifact.RollbackSpec{Triggers: []string{"Problem selector regresses."}},
		AffectedFiles: []string{"internal/cli/harness.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	second, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      secondProblem.Meta.ID,
		SelectedTitle:   "Second context decision",
		WhySelected:     "It belongs to the selected context.",
		SelectionPolicy: "Keep context-scoped harness runs explicit.",
		CounterArgument: "Listing ids is simpler.",
		WeakestLink:     "Selection must not pull unrelated decisions.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Manual ids",
			Reason:  "Too much operator ceremony.",
		}},
		Rollback:      &artifact.RollbackSpec{Triggers: []string{"Context selector regresses."}},
		AffectedFiles: []string{"internal/cli/harness_test.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_ = createCommissionDecisionFixture(t, ctx, store, haftDir, "Unrelated context decision", "internal/cli/run.go")

	harnessPlanContext = "mvp-harness"
	refs, err := resolveHarnessDecisionRefs(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 2 {
		t.Fatalf("refs = %#v, want two context decisions", refs)
	}
	if !stringSliceContains(refs, first.Meta.ID) || !stringSliceContains(refs, second.Meta.ID) {
		t.Fatalf("refs = %#v, want %s and %s", refs, first.Meta.ID, second.Meta.ID)
	}
}

func TestResolveHarnessDecisionRefsDefaultsToUncommissionedActive(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	commissioned := createCommissionDecisionFixture(t, ctx, store, haftDir, "Commissioned decision", "internal/cli/harness.go")
	uncommissioned := createCommissionDecisionFixture(t, ctx, store, haftDir, "Uncommissioned decision", "internal/cli/run.go")

	commission := workCommissionFixture("wc-commissioned-decision", "queued", "2099-01-01T00:00:00Z")
	commission["decision_ref"] = commissioned.Meta.ID

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err != nil {
		t.Fatal(err)
	}

	refs, err := resolveHarnessDecisionRefs(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one uncommissioned decision", refs)
	}
	if refs[0] != uncommissioned.Meta.ID {
		t.Fatalf("refs = %#v, want [%s]", refs, uncommissioned.Meta.ID)
	}
}

func TestResolveHarnessDecisionRefsAllActiveIncludesCommissioned(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	commissioned := createCommissionDecisionFixture(t, ctx, store, haftDir, "All active commissioned", "internal/cli/harness.go")
	uncommissioned := createCommissionDecisionFixture(t, ctx, store, haftDir, "All active uncommissioned", "internal/cli/run.go")

	commission := workCommissionFixture("wc-all-active-commissioned", "queued", "2099-01-01T00:00:00Z")
	commission["decision_ref"] = commissioned.Meta.ID

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err != nil {
		t.Fatal(err)
	}

	harnessPlanAllActive = true
	refs, err := resolveHarnessDecisionRefs(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 2 {
		t.Fatalf("refs = %#v, want both active decisions", refs)
	}
	if !stringSliceContains(refs, commissioned.Meta.ID) || !stringSliceContains(refs, uncommissioned.Meta.ID) {
		t.Fatalf("refs = %#v, want %s and %s", refs, commissioned.Meta.ID, uncommissioned.Meta.ID)
	}
}

func TestEnsureHarnessCommissionsSkipsExistingPlan(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	commission := workCommissionFixture("wc-existing-plan", "queued", "2099-01-01T00:00:00Z")
	commission["implementation_plan_ref"] = "plan-existing"
	commission["implementation_plan_revision"] = "p1"

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err != nil {
		t.Fatal(err)
	}

	created, result, err := ensureHarnessCommissions(ctx, store, t.TempDir(), map[string]any{
		"id":       "plan-existing",
		"revision": "p1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatalf("created = true, want existing commissions reused")
	}
	if !strings.Contains(result, "reused 1 existing commission") {
		t.Fatalf("result = %q, want reuse message", result)
	}

	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want no duplicate commissions", len(records))
	}
}

func TestDefaultHarnessRunOptionsUsesRuntimeEnv(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	t.Setenv("HAFT_OPEN_SLEIGH_RUNTIME", "/tmp/open-sleigh-runtime")

	opts := defaultHarnessRunOptions(t.TempDir(), ".haft/plans/p.yaml", map[string]any{
		"id": "plan-runtime-env",
	})

	if opts.RuntimePath != "/tmp/open-sleigh-runtime" {
		t.Fatalf("runtime path = %q, want env value", opts.RuntimePath)
	}
}

func TestValidateOpenSleighRuntimeRequiresMixExs(t *testing.T) {
	runtimePath := t.TempDir()

	err := validateOpenSleighRuntime(runtimePath)
	if err == nil || !strings.Contains(err.Error(), "missing bin/open_sleigh or mix.exs") {
		t.Fatalf("error = %v, want missing runtime marker", err)
	}
}

func TestValidateOpenSleighRuntimeAcceptsSourceRuntime(t *testing.T) {
	runtimePath := t.TempDir()
	mixPath := filepath.Join(runtimePath, "mix.exs")
	if err := os.WriteFile(mixPath, []byte("defmodule OpenSleigh.MixProject do\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath("mix"); err != nil {
		t.Skipf("mix is not installed: %v", err)
	}

	if err := validateOpenSleighRuntime(runtimePath); err != nil {
		t.Fatalf("validateOpenSleighRuntime returned error: %v", err)
	}
}

func TestValidateOpenSleighRuntimeAcceptsReleaseRuntime(t *testing.T) {
	runtimePath := t.TempDir()
	binDir := filepath.Join(runtimePath, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(binDir, "open_sleigh")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := validateOpenSleighRuntime(runtimePath); err != nil {
		t.Fatalf("validateOpenSleighRuntime returned error: %v", err)
	}
}

func TestElixirStringListLiteralEscapesArguments(t *testing.T) {
	got := elixirStringListLiteral([]string{
		"--path",
		`/tmp/a "quoted" path`,
		"line\nbreak",
	})
	want := `["--path", "/tmp/a \"quoted\" path", "line\nbreak"]`
	if got != want {
		t.Fatalf("literal = %q, want %q", got, want)
	}
}

func overrideHarnessTestFlags() func() {
	oldHarnessPlanID := harnessPlanID
	oldHarnessPlanTitle := harnessPlanTitle
	oldHarnessPlanRevision := harnessPlanRevision
	oldHarnessPlanSequential := harnessPlanSequential
	oldHarnessPlanDependencies := harnessPlanDependencies
	oldHarnessPlanProblems := harnessPlanProblems
	oldHarnessPlanContext := harnessPlanContext
	oldHarnessPlanAllActive := harnessPlanAllActive
	oldCommissionRepoRef := commissionFromDecisionRepoRef
	oldCommissionBaseSHA := commissionFromDecisionBaseSHA
	oldCommissionTargetBranch := commissionFromDecisionTargetBranch
	oldCommissionAllowedActions := commissionFromDecisionAllowedActions
	oldCommissionEvidence := commissionFromDecisionEvidence
	oldCommissionProjectionPolicy := commissionFromDecisionProjectionPolicy
	oldCommissionState := commissionFromDecisionState
	oldCommissionValidFor := commissionFromDecisionValidFor
	oldCommissionValidUntil := commissionFromDecisionValidUntil

	harnessPlanID = ""
	harnessPlanTitle = ""
	harnessPlanRevision = "p1"
	harnessPlanSequential = false
	harnessPlanDependencies = nil
	harnessPlanProblems = nil
	harnessPlanContext = ""
	harnessPlanAllActive = false
	commissionFromDecisionRepoRef = ""
	commissionFromDecisionBaseSHA = ""
	commissionFromDecisionTargetBranch = ""
	commissionFromDecisionAllowedActions = []string{"edit_files", "run_tests"}
	commissionFromDecisionEvidence = nil
	commissionFromDecisionProjectionPolicy = "local_only"
	commissionFromDecisionState = "queued"
	commissionFromDecisionValidFor = "168h"
	commissionFromDecisionValidUntil = ""

	return func() {
		harnessPlanID = oldHarnessPlanID
		harnessPlanTitle = oldHarnessPlanTitle
		harnessPlanRevision = oldHarnessPlanRevision
		harnessPlanSequential = oldHarnessPlanSequential
		harnessPlanDependencies = oldHarnessPlanDependencies
		harnessPlanProblems = oldHarnessPlanProblems
		harnessPlanContext = oldHarnessPlanContext
		harnessPlanAllActive = oldHarnessPlanAllActive
		commissionFromDecisionRepoRef = oldCommissionRepoRef
		commissionFromDecisionBaseSHA = oldCommissionBaseSHA
		commissionFromDecisionTargetBranch = oldCommissionTargetBranch
		commissionFromDecisionAllowedActions = oldCommissionAllowedActions
		commissionFromDecisionEvidence = oldCommissionEvidence
		commissionFromDecisionProjectionPolicy = oldCommissionProjectionPolicy
		commissionFromDecisionState = oldCommissionState
		commissionFromDecisionValidFor = oldCommissionValidFor
		commissionFromDecisionValidUntil = oldCommissionValidUntil
	}
}
