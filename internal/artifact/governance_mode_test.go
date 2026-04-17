package artifact

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGovernanceMode_DefaultIsModule(t *testing.T) {
	df := DecisionFields{}
	if df.EffectiveGovernanceMode() != GovernanceModeModule {
		t.Fatalf("empty governance_mode should default to module, got %q", df.EffectiveGovernanceMode())
	}

	df.GovernanceMode = GovernanceModeExact
	if df.EffectiveGovernanceMode() != GovernanceModeExact {
		t.Fatalf("explicit exact should be returned as-is, got %q", df.EffectiveGovernanceMode())
	}
}

func TestParseGovernanceMode(t *testing.T) {
	tests := []struct {
		input   string
		want    GovernanceMode
		wantErr bool
	}{
		{"", "", false}, // empty is allowed (defaulted on read)
		{"module", GovernanceModeModule, false},
		{"exact", GovernanceModeExact, false},
		{"  exact  ", GovernanceModeExact, false},
		{"both", "", true},
		{"strict", "", true},
	}
	for _, tc := range tests {
		got, err := ParseGovernanceMode(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseGovernanceMode(%q): err=%v wantErr=%v", tc.input, err, tc.wantErr)
		}
		if got != tc.want {
			t.Errorf("ParseGovernanceMode(%q): got=%q want=%q", tc.input, got, tc.want)
		}
	}
}

func TestDecide_GovernanceModeRejectsInvalid(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, _, err := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle:   "Use Postgres",
		WhySelected:     "ops familiarity",
		SelectionPolicy: "lowest cognitive load for the on-call team",
		CounterArgument: "operational maturity may not hold under new scale",
		WeakestLink:     "operational maturity is binding",
		WhyNotOthers: []RejectionReason{
			{Variant: "MySQL", Reason: "no production experience at this scale"},
		},
		Rollback: &RollbackSpec{
			Triggers: []string{"operational load makes Postgres untenable"},
		},
		GovernanceMode: "strict", // invalid
	})

	if err == nil {
		t.Fatal("expected error for invalid governance_mode, got nil")
	}
	if !strings.Contains(err.Error(), "governance_mode") {
		t.Fatalf("error should mention governance_mode, got: %v", err)
	}
}

// TestBaseline_ExactModeSkipsModuleScopeManifests verifies that exact-mode
// decisions do NOT auto-widen affected_files to parent directories. This is
// the core fix for 5.4 Pro Finding #3 — opt-in scope governance instead of
// silent inflation.
func TestBaseline_ExactModeSkipsModuleScopeManifests(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	projectRoot := t.TempDir()

	// Set up project with one tracked file and one sibling.
	pkgDir := filepath.Join(projectRoot, "pkg", "auth")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(pkgDir, "login.go")
	sibling := filepath.Join(pkgDir, "logout.go")
	if err := os.WriteFile(tracked, []byte("package auth\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sibling, []byte("package auth\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Decide with GovernanceMode=exact.
	a, _, err := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle:   "Track login.go only",
		WhySelected:     "this decision is specifically about login.go, not the whole auth package",
		SelectionPolicy: "narrow scope; sibling files are not governed",
		CounterArgument: "scope might widen later; force re-decide if so",
		WeakestLink:     "future contributors may add files assuming module-level governance",
		WhyNotOthers: []RejectionReason{
			{Variant: "Module-level governance", Reason: "would silently capture sibling files as governed drift"},
		},
		Rollback: &RollbackSpec{
			Triggers: []string{"if scope creep makes file-level tracking infeasible"},
		},
		AffectedFiles:  []string{"pkg/auth/login.go"},
		GovernanceMode: string(GovernanceModeExact),
	})
	if err != nil {
		t.Fatalf("Decide failed: %v", err)
	}

	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: a.Meta.ID})
	if err != nil {
		t.Fatalf("Baseline failed: %v", err)
	}

	// Reload and verify DriftManifests is empty for exact mode.
	reloaded, err := store.Get(ctx, a.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields := reloaded.UnmarshalDecisionFields()
	if fields.EffectiveGovernanceMode() != GovernanceModeExact {
		t.Fatalf("governance_mode not persisted; got %q want exact", fields.EffectiveGovernanceMode())
	}
	if len(fields.DriftManifests) != 0 {
		t.Fatalf("exact mode should produce zero DriftManifests, got %d: %+v", len(fields.DriftManifests), fields.DriftManifests)
	}
}

// TestBaseline_ModuleModeBuildsScopeManifests verifies that module mode (the
// default) preserves the pre-6.2.x behavior of capturing sibling files as
// governed scope. Required to keep backward compatibility intact.
func TestBaseline_ModuleModeBuildsScopeManifests(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	projectRoot := t.TempDir()

	pkgDir := filepath.Join(projectRoot, "pkg", "billing")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "invoice.go"), []byte("package billing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a, _, err := Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle:   "Track billing module",
		WhySelected:     "billing module is governed as a unit",
		SelectionPolicy: "module-level governance preferred for cohesive subsystems",
		CounterArgument: "may capture unrelated sibling additions as governed drift",
		WeakestLink:     "module boundary is implicit, not declared",
		WhyNotOthers: []RejectionReason{
			{Variant: "Exact file tracking", Reason: "would miss new files that join the module"},
		},
		Rollback: &RollbackSpec{
			Triggers: []string{"module split makes single-scope tracking misleading"},
		},
		AffectedFiles: []string{"pkg/billing/invoice.go"},
		// GovernanceMode unset → defaults to module
	})
	if err != nil {
		t.Fatalf("Decide failed: %v", err)
	}

	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: a.Meta.ID})
	if err != nil {
		t.Fatalf("Baseline failed: %v", err)
	}

	reloaded, err := store.Get(ctx, a.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields := reloaded.UnmarshalDecisionFields()
	if fields.EffectiveGovernanceMode() != GovernanceModeModule {
		t.Fatalf("default governance_mode should be module, got %q", fields.EffectiveGovernanceMode())
	}
	if len(fields.DriftManifests) == 0 {
		t.Fatal("module mode should produce DriftManifests, got zero")
	}
}
