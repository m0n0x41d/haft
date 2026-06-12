package method

import (
	"strings"
	"testing"
)

func TestBuiltinCatalogValidates(t *testing.T) {
	if err := ValidateCatalog(BuiltinCatalog()); err != nil {
		t.Fatalf("builtin catalog should validate: %v", err)
	}
}

func TestBuiltinCatalogContainsExpectedMethods(t *testing.T) {
	got := map[string]bool{}
	for _, definition := range BuiltinCatalog().Methods {
		got[definition.ID] = true
	}

	for _, want := range []string{
		"verification-before-completion",
		"systematic-debugging-before-fix",
		"behavior-first-testing",
		"refactor-only-under-tests",
		"domain-port-before-adapter",
		"functional-core-imperative-shell",
		"make-illegal-states-unrepresentable",
	} {
		if !got[want] {
			t.Fatalf("builtin catalog missing %q in %#v", want, got)
		}
	}
}

func TestValidateCatalogRejectsInvalidDefinitions(t *testing.T) {
	err := ValidateCatalog(Catalog{
		ID:      CatalogID,
		Version: CatalogVersion,
		Methods: []Definition{{
			ID:      "empty-gates",
			Version: CatalogVersion,
			Title:   "Empty gates",
		}},
	})
	if err == nil {
		t.Fatal("ValidateCatalog accepted a method without hard gates")
	}
	if !strings.Contains(err.Error(), "hard gate") {
		t.Fatalf("error = %v, want hard gate failure", err)
	}
}

func TestPullExternalIntegrationReturnsDomainAndVerificationMethods(t *testing.T) {
	run, err := Pull(PullInput{
		Task:             "Add Slack notification delivery",
		DeclaredTaskKind: "external-integration",
		ChangeIntent:     "add_feature",
		IntendedFiles:    []string{"internal/slack/adapter.go", "internal/domain/notification.go"},
		RiskSignals: []RiskSignal{
			{ID: "external_io", Source: "test"},
			{ID: "domain_boundary", Source: "test"},
		},
		ResponseBudget: ResponseBudget{MaxMethods: 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Methods) > 3 {
		t.Fatalf("pull returned %d methods, want max 3", len(run.Methods))
	}
	for _, card := range run.Methods {
		if len(card.HardGates) > 3 {
			t.Fatalf("%s returned %d hard gates, want max 3", card.ID, len(card.HardGates))
		}
	}
	assertMethodIDs(t, run, []string{
		"domain-port-before-adapter",
		"behavior-first-testing",
		"verification-before-completion",
	})
}

func TestPullBugfixReturnsSystematicDebugging(t *testing.T) {
	run, err := Pull(PullInput{
		Task:             "Fix failing parser test",
		DeclaredTaskKind: "bugfix",
		ChangeIntent:     "fix_bug",
		RiskSignals:      []RiskSignal{{ID: "failing_test", Source: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertMethodIDs(t, run, []string{"systematic-debugging-before-fix"})
}

func TestPullBusinessLogicRefactorReturnsFunctionalCoreMethod(t *testing.T) {
	run, err := Pull(PullInput{
		Task:             "Extract policy calculation into a pure core",
		DeclaredTaskKind: "refactor",
		ChangeIntent:     "refactor",
		RiskSignals:      []RiskSignal{{ID: "business_logic", Source: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertMethodIDs(t, run, []string{
		"refactor-only-under-tests",
		"functional-core-imperative-shell",
		"verification-before-completion",
	})
}

func TestPullStateInvariantReturnsIllegalStatesMethod(t *testing.T) {
	run, err := Pull(PullInput{
		Task:             "Replace boolean flags with explicit lifecycle states",
		DeclaredTaskKind: "feature",
		ChangeIntent:     "change_behavior",
		RiskSignals:      []RiskSignal{{ID: "state_machine", Source: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertMethodIDs(t, run, []string{
		"make-illegal-states-unrepresentable",
		"verification-before-completion",
	})
}

func TestPullMechanicalEditReturnsLowCeremonyWithoutArchitectureGates(t *testing.T) {
	run, err := Pull(PullInput{
		Task:             "Rename typo in docs",
		DeclaredTaskKind: TaskMechanicalEdit,
		ChangeIntent:     TaskMechanicalEdit,
		IntendedFiles:    []string{"README.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.TaskSignature.Ceremony != "low" {
		t.Fatalf("ceremony = %q, want low", run.TaskSignature.Ceremony)
	}
	if len(run.Methods) != 0 {
		t.Fatalf("mechanical edit methods = %#v, want none", run.Methods)
	}
}

func TestPullLowCeremonyRequestDoesNotBypassRiskGates(t *testing.T) {
	run, err := Pull(PullInput{
		Task:             "Change public API behavior",
		DeclaredTaskKind: "feature",
		ChangeIntent:     "change_behavior",
		CeremonyRequest:  "none",
		RiskSignals: []RiskSignal{
			{ID: "public_api", Source: "test"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.TaskSignature.Ceremony != "medium" {
		t.Fatalf("ceremony = %q, want medium", run.TaskSignature.Ceremony)
	}
	if len(run.Methods) == 0 {
		t.Fatal("risk-gated non-mechanical work returned no method cards")
	}
	assertMethodIDs(t, run, []string{"behavior-first-testing"})
}

func TestPullUnmatchedMediumWorkFallsBackToVerification(t *testing.T) {
	run, err := Pull(PullInput{
		Task: "Update repo behavior from a PR review finding",
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.TaskSignature.Ceremony != "medium" {
		t.Fatalf("ceremony = %q, want medium", run.TaskSignature.Ceremony)
	}
	assertMethodIDs(t, run, []string{"verification-before-completion"})

	gateResults := []GateResult{{
		GateID: "fresh_verification_before_completion",
		Status: "satisfied",
	}}
	err = ValidateClose(run, CloseInput{GateResults: gateResults})
	if err == nil {
		t.Fatal("ValidateClose accepted fallback verification without evidence")
	}
}

func TestPressureFixturesSelectExpectedMethods(t *testing.T) {
	tests := []struct {
		name          string
		input         PullInput
		wantMethodID  string
		wantNoMethods bool
	}{
		{
			name: "non trivial feature",
			input: PullInput{
				Task:             "Add export endpoint",
				DeclaredTaskKind: "feature",
				ChangeIntent:     "add_feature",
				RiskSignals:      []RiskSignal{{ID: "behavior_change"}},
			},
			wantMethodID: "behavior-first-testing",
		},
		{
			name: "bugfix",
			input: PullInput{
				Task:             "Fix nil project registry crash",
				DeclaredTaskKind: "bugfix",
				ChangeIntent:     "fix_bug",
				RiskSignals:      []RiskSignal{{ID: "panic"}},
			},
			wantMethodID: "systematic-debugging-before-fix",
		},
		{
			name: "refactor",
			input: PullInput{
				Task:             "Refactor renderer into pure core",
				DeclaredTaskKind: "refactor",
				ChangeIntent:     "refactor",
			},
			wantMethodID: "refactor-only-under-tests",
		},
		{
			name: "functional core",
			input: PullInput{
				Task:             "Split calculation core from IO shell",
				DeclaredTaskKind: "refactor",
				ChangeIntent:     "refactor",
				RiskSignals:      []RiskSignal{{ID: "side_effect_boundary"}},
			},
			wantMethodID: "functional-core-imperative-shell",
		},
		{
			name: "illegal states",
			input: PullInput{
				Task:             "Model lifecycle with explicit variants",
				DeclaredTaskKind: "feature",
				ChangeIntent:     "change_behavior",
				RiskSignals:      []RiskSignal{{ID: "invalid_state"}},
			},
			wantMethodID: "make-illegal-states-unrepresentable",
		},
		{
			name: "external integration",
			input: PullInput{
				Task:             "Persist events to a database",
				DeclaredTaskKind: "external_integration",
				ChangeIntent:     "add_feature",
				IntendedFiles:    []string{"internal/db/events.go"},
				RiskSignals:      []RiskSignal{{ID: "persistence"}},
			},
			wantMethodID: "domain-port-before-adapter",
		},
		{
			name: "mechanical edit",
			input: PullInput{
				Task:             "Fix spelling in help text",
				DeclaredTaskKind: TaskMechanicalEdit,
				ChangeIntent:     TaskMechanicalEdit,
			},
			wantNoMethods: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run, err := Pull(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantNoMethods {
				if len(run.Methods) != 0 {
					t.Fatalf("methods = %#v, want none", run.Methods)
				}
				return
			}
			assertMethodIDs(t, run, []string{tt.wantMethodID})
		})
	}
}

func TestValidateCloseRequiresHardGateEvidenceOrWaiver(t *testing.T) {
	run := MethodRun{
		Status: "open",
		Methods: []MethodCard{{
			ID: "test-method",
			HardGates: []Gate{{
				ID:               "gate-with-evidence",
				Kind:             "test_evidence",
				CheckLevel:       "deterministic",
				RequiredEvidence: []string{"command_output"},
				Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
			}},
		}},
	}

	err := ValidateClose(run, CloseInput{
		GateResults: []GateResult{{
			GateID: "gate-with-evidence",
			Status: "satisfied",
		}},
	})
	if err == nil {
		t.Fatal("ValidateClose accepted a hard gate without evidence")
	}

	err = ValidateClose(run, CloseInput{
		GateResults: []GateResult{{
			GateID:       "gate-with-evidence",
			Status:       "satisfied",
			EvidenceRefs: []string{"go test ./internal/method"},
		}},
	})
	if err != nil {
		t.Fatalf("ValidateClose rejected evidenced gate: %v", err)
	}

	err = ValidateClose(run, CloseInput{
		Waivers: []Waiver{{
			GateID: "gate-with-evidence",
			Reason: "No runnable tests exist in this fixture.",
		}},
	})
	if err != nil {
		t.Fatalf("ValidateClose rejected explicit waiver: %v", err)
	}

	err = ValidateClose(run, CloseInput{
		GateResults: []GateResult{{
			GateID:       "gate-with-evidence",
			Status:       "waived",
			WaiverReason: "No runnable tests exist in this fixture.",
		}},
	})
	if err != nil {
		t.Fatalf("ValidateClose rejected gate-result waiver reason: %v", err)
	}
}

func assertMethodIDs(t *testing.T, run MethodRun, wants []string) {
	t.Helper()

	got := map[string]bool{}
	for _, card := range run.Methods {
		got[card.ID] = true
	}
	for _, want := range wants {
		if !got[want] {
			t.Fatalf("method ids = %#v, missing %q", got, want)
		}
	}
}
