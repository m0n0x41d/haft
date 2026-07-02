package method

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
		"graph-preflight-before-governed-edit",
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

func TestBuiltinCatalogMethodsCarrySupportSourcePosture(t *testing.T) {
	for _, definition := range BuiltinCatalog().Methods {
		posture := definition.SourcePosture
		if posture.SourceKind != MethodSourceKind {
			t.Fatalf("%s source_kind = %q", definition.ID, posture.SourceKind)
		}
		if posture.SourceEdition != CatalogID+"@"+CatalogVersion {
			t.Fatalf("%s source_edition = %q", definition.ID, posture.SourceEdition)
		}
		if posture.Normativity != MethodSourceNormativity {
			t.Fatalf("%s normativity = %q", definition.ID, posture.Normativity)
		}
		if !strings.Contains(posture.AuthorityBoundary, "do not define normative FPF source material") {
			t.Fatalf("%s authority boundary does not block normative masquerade: %q", definition.ID, posture.AuthorityBoundary)
		}
	}
}

func TestBuiltinCatalogMethodsCarryLifecycleDiscoveryMetadata(t *testing.T) {
	for _, definition := range BuiltinCatalog().Methods {
		if definition.Lifecycle.Status != LifecycleCurrent {
			t.Fatalf("%s lifecycle.status = %q, want current", definition.ID, definition.Lifecycle.Status)
		}
		if definition.Lifecycle.ValidFrom == "" {
			t.Fatalf("%s lifecycle.valid_from missing", definition.ID)
		}
		if definition.FirstUsefulMove == "" {
			t.Fatalf("%s first_useful_move missing", definition.ID)
		}
		if len(definition.ExpectedOutputKinds) == 0 {
			t.Fatalf("%s expected_output_kinds missing", definition.ID)
		}
		if len(definition.FitFunctionRefs) == 0 {
			t.Fatalf("%s fit_function_refs missing", definition.ID)
		}
		if len(definition.CarrierRefs) == 0 {
			t.Fatalf("%s carrier_refs missing", definition.ID)
		}
	}
}

func TestBuiltinCatalogMethodsCarryDocumentarySourcePatternRefs(t *testing.T) {
	for _, definition := range BuiltinCatalog().Methods {
		if len(definition.SourcePatternRefs) == 0 {
			t.Fatalf("%s source_pattern_refs missing", definition.ID)
		}
		for _, ref := range definition.SourcePatternRefs {
			if !validSourcePatternRefPrefix(ref) {
				t.Fatalf("%s invalid source_pattern_ref %q", definition.ID, ref)
			}
		}
	}
}

func TestInstallDefaultCatalogMaterializesManifestMatchingBuiltinCatalog(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")
	if err := InstallDefaultCatalog(haftDir); err != nil {
		t.Fatalf("InstallDefaultCatalog: %v", err)
	}
	methodDir := filepath.Join(haftDir, "methods", CatalogID)

	data, err := os.ReadFile(filepath.Join(methodDir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("read materialized manifest: %v", err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse materialized manifest: %v", err)
	}

	want := map[string]bool{}
	for _, definition := range BuiltinCatalog().Methods {
		want[definition.ID] = true
	}
	got := map[string]bool{}
	for _, id := range manifest.Methods {
		got[id] = true
	}

	for id := range want {
		if !got[id] {
			t.Fatalf("manifest missing builtin method %q", id)
		}
	}
	for id := range got {
		if !want[id] {
			t.Fatalf("manifest contains unknown method %q", id)
		}
	}

	entries, err := os.ReadDir(methodDir)
	if err != nil {
		t.Fatalf("read materialized method dir: %v", err)
	}
	yamlMethods := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "manifest.yaml" || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		id := strings.TrimSuffix(name, ".yaml")
		yamlMethods[id] = true
	}
	for id := range want {
		if !yamlMethods[id] {
			t.Fatalf("materialized method YAML missing %q", id)
		}
	}
	for id := range yamlMethods {
		if !want[id] {
			t.Fatalf("materialized method YAML contains unknown method %q", id)
		}
	}
}

func TestInstallDefaultCatalogMaterializesCurrentCarrierMetadata(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")
	if err := InstallDefaultCatalog(haftDir); err != nil {
		t.Fatalf("InstallDefaultCatalog: %v", err)
	}
	for _, definition := range BuiltinCatalog().Methods {
		path := filepath.Join(haftDir, "methods", CatalogID, definition.ID+".yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var carrier Definition
		if err := yaml.Unmarshal(data, &carrier); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if carrier.ID != definition.ID {
			t.Fatalf("%s id = %q, want %q", path, carrier.ID, definition.ID)
		}
		if carrier.SourcePosture != definition.SourcePosture {
			t.Fatalf("%s source posture = %+v, want %+v", path, carrier.SourcePosture, definition.SourcePosture)
		}
		if carrier.Lifecycle.Status != definition.Lifecycle.Status {
			t.Fatalf("%s lifecycle.status = %q, want %q", path, carrier.Lifecycle.Status, definition.Lifecycle.Status)
		}
		if !sameStringSet(carrier.SourcePatternRefs, definition.SourcePatternRefs) {
			t.Fatalf("%s source_pattern_refs = %#v, want %#v", path, carrier.SourcePatternRefs, definition.SourcePatternRefs)
		}
	}
}

func TestValidateCatalogRejectsInvalidDefinitions(t *testing.T) {
	err := ValidateCatalog(Catalog{
		ID:      CatalogID,
		Version: CatalogVersion,
		Methods: []Definition{{
			ID:            "empty-gates",
			Version:       CatalogVersion,
			Title:         "Empty gates",
			SourcePosture: testMethodSourcePosture(),
		}},
	})
	if err == nil {
		t.Fatal("ValidateCatalog accepted a method without hard gates")
	}
	if !strings.Contains(err.Error(), "hard gate") {
		t.Fatalf("error = %v, want hard gate failure", err)
	}
}

func TestValidateCatalogRejectsNormativeFPFMethodSourcePosture(t *testing.T) {
	err := ValidateCatalog(Catalog{
		ID:      CatalogID,
		Version: CatalogVersion,
		Methods: []Definition{{
			ID:      "normative-masquerade",
			Version: CatalogVersion,
			Title:   "Normative masquerade",
			SourcePosture: SourcePosture{
				SourceKind:        MethodSourceKind,
				SourceEdition:     CatalogID + "@" + CatalogVersion,
				Normativity:       "normative_fpf_source",
				AuthorityBoundary: MethodAuthorityBoundary,
			},
			HardGates: []Gate{{
				ID:         "gate",
				Kind:       "test",
				CheckLevel: "deterministic",
			}},
		}},
	})
	if err == nil {
		t.Fatal("ValidateCatalog accepted a MethodPack card as normative FPF source")
	}
	if !strings.Contains(err.Error(), "source_posture.normativity") {
		t.Fatalf("error = %v, want source_posture.normativity failure", err)
	}
}

func TestValidateCatalogRejectsInvalidSourcePatternRefPrefix(t *testing.T) {
	definition := validCatalogTestDefinition("bad-source-ref")
	definition.SourcePatternRefs = []string{"docs:A.10"}

	err := ValidateCatalog(Catalog{
		ID:      CatalogID,
		Version: CatalogVersion,
		Methods: []Definition{
			definition,
		},
	})
	if err == nil {
		t.Fatal("ValidateCatalog accepted invalid source_pattern_refs prefix")
	}
	if !strings.Contains(err.Error(), "source_pattern_refs") {
		t.Fatalf("error = %v, want source_pattern_refs failure", err)
	}
}

func TestValidateCatalogRejectsInvalidLifecycle(t *testing.T) {
	definition := validCatalogTestDefinition("bad-lifecycle")
	definition.Lifecycle.Status = "retired"

	err := ValidateCatalog(Catalog{
		ID:      CatalogID,
		Version: CatalogVersion,
		Methods: []Definition{
			definition,
		},
	})
	if err == nil {
		t.Fatal("ValidateCatalog accepted invalid lifecycle status")
	}
	if !strings.Contains(err.Error(), "lifecycle.status") {
		t.Fatalf("error = %v, want lifecycle.status failure", err)
	}
}

func TestDiscoverCatalogFiltersByLifecycle(t *testing.T) {
	report, err := DiscoverCatalog(LifecycleCurrent)
	if err != nil {
		t.Fatal(err)
	}
	if report.Kind != CatalogReportKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.FilterStatus != LifecycleCurrent {
		t.Fatalf("filter = %q, want current", report.FilterStatus)
	}
	if report.Summary.Returned != report.Summary.Current || report.Summary.Returned == 0 {
		t.Fatalf("summary = %+v, want returned current methods", report.Summary)
	}
	for _, entry := range report.Methods {
		if entry.Lifecycle.Status != LifecycleCurrent {
			t.Fatalf("current report included %s lifecycle=%s", entry.ID, entry.Lifecycle.Status)
		}
		if !strings.Contains(entry.SourcePosture.Normativity, "support_carrier_non_normative_fpf") {
			t.Fatalf("entry %s missing support source posture: %+v", entry.ID, entry.SourcePosture)
		}
		if len(entry.SourcePatternRefs) == 0 {
			t.Fatalf("entry %s missing source_pattern_refs", entry.ID)
		}
	}
}

func TestDiscoverCatalogRejectsUnsupportedStatus(t *testing.T) {
	_, err := DiscoverCatalog("governing")
	if err == nil {
		t.Fatal("DiscoverCatalog accepted unsupported lifecycle status")
	}
	if !strings.Contains(err.Error(), "unsupported method catalog status") {
		t.Fatalf("error = %v, want unsupported status failure", err)
	}
}

func TestPullIgnoresNonCurrentMethods(t *testing.T) {
	current := validCatalogTestDefinition("current-method")
	current.AppliesTo = Applicability{TaskKinds: []string{"feature"}}
	current.Priority = 50
	superseded := validCatalogTestDefinition("superseded-method")
	superseded.AppliesTo = Applicability{TaskKinds: []string{"feature"}}
	superseded.Lifecycle = Lifecycle{
		Status:        LifecycleSuperseded,
		ValidFrom:     "2026-06-25",
		SuccessorRefs: []string{"current-method"},
	}
	superseded.Priority = 1
	deprecated := validCatalogTestDefinition("deprecated-method")
	deprecated.AppliesTo = Applicability{TaskKinds: []string{"feature"}}
	deprecated.Lifecycle = Lifecycle{
		Status:           LifecycleDeprecated,
		ValidFrom:        "2026-06-25",
		RetirementReason: "No longer matches current work shape.",
	}
	deprecated.Priority = 2

	cards := matchCards([]Definition{current, superseded, deprecated}, PullInput{
		DeclaredTaskKind: "feature",
	})
	if len(cards) != 1 {
		t.Fatalf("cards = %#v, want only current method", cards)
	}
	if cards[0].ID != "current-method" {
		t.Fatalf("card = %s, want current-method", cards[0].ID)
	}
}

func TestPullCardsCarrySupportSourcePosture(t *testing.T) {
	run, err := Pull(PullInput{
		Task:             "Add public behavior",
		DeclaredTaskKind: "feature",
		ChangeIntent:     "add_feature",
		RiskSignals:      []RiskSignal{{ID: "behavior_change"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Methods) == 0 {
		t.Fatal("expected method cards")
	}
	for _, card := range run.Methods {
		if card.SourcePosture.SourceKind != MethodSourceKind {
			t.Fatalf("%s source_kind = %q", card.ID, card.SourcePosture.SourceKind)
		}
		if card.SourcePosture.Normativity != MethodSourceNormativity {
			t.Fatalf("%s normativity = %q", card.ID, card.SourcePosture.Normativity)
		}
		if len(card.SourcePatternRefs) == 0 {
			t.Fatalf("%s source_pattern_refs missing", card.ID)
		}
	}
}

func TestSourcePatternRefsDoNotSatisfyHardGateEvidence(t *testing.T) {
	run := MethodRun{
		Status: "open",
		Methods: []MethodCard{{
			ID:                "test-method",
			SourcePatternRefs: []string{"fpf:A.10"},
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
		t.Fatal("ValidateClose accepted source_pattern_refs as gate evidence")
	}
	if !strings.Contains(err.Error(), "needs evidence_refs") {
		t.Fatalf("error = %v, want missing evidence_refs", err)
	}
}

func TestPatternUseRecommendationRefsDoNotSatisfyHardGateEvidence(t *testing.T) {
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
	cases := []string{
		"pattern_use:F.18",
		"retrieved_uncompiled",
		"support_level=retrieved_uncompiled",
		"PatternUseRecommendation:F.18",
		`haft_query(action="pattern_use", mode="compact", query="именуй нормально")`,
		`{"action":"pattern_use","support_level":"retrieved_uncompiled"}`,
	}

	for _, evidenceRef := range cases {
		t.Run(evidenceRef, func(t *testing.T) {
			err := ValidateClose(run, CloseInput{
				GateResults: []GateResult{{
					GateID:       "gate-with-evidence",
					Status:       "satisfied",
					EvidenceRefs: []string{evidenceRef},
				}},
			})
			if err == nil {
				t.Fatal("ValidateClose accepted PatternUse recommendation as gate evidence")
			}
			if !strings.Contains(err.Error(), "not PatternUse recommendations") {
				t.Fatalf("error = %v, want PatternUse evidence boundary", err)
			}
		})
	}
}

func TestPatternUseRuntimeObservationsCanSatisfyHardGateEvidence(t *testing.T) {
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
	cases := []string{
		`go run ./cmd/haft pattern-use recommend "build PatternUseIndex compiler from FPF routes/search substrate" --mode compact --json => retrieved_uncompiled with candidate_pattern_use_set`,
		`pattern-use audit: 29 prompts, rows_passing=29, authority_violations=0; RU 250-card prompt retrieved_uncompiled not F.18.`,
		`go test ./internal/method -run 'Test(PatternUseRecommendationRefsDoNotSatisfyHardGateEvidence|ValidateCloseAllowsPatternUseCarryThroughContext)'`,
		`MCP haft_query(action="pattern_use", mode="compact", ...) for all C1 target prompts -> expected route IDs and controls`,
		`retrieved-uncompiled progressive-disclosure fixture audit flags compact-only application as fail/progressive_disclosure_bypass`,
	}

	for _, evidenceRef := range cases {
		t.Run(evidenceRef, func(t *testing.T) {
			err := ValidateClose(run, CloseInput{
				GateResults: []GateResult{{
					GateID:       "gate-with-evidence",
					Status:       "satisfied",
					EvidenceRefs: []string{evidenceRef},
				}},
			})
			if err != nil {
				t.Fatalf("ValidateClose rejected runtime observation evidence: %v", err)
			}
		})
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

func TestPullGovernedCodeReturnsGraphPreflightMethod(t *testing.T) {
	run, err := Pull(PullInput{
		Task:             "Patch governed CLI behavior",
		DeclaredTaskKind: "feature",
		ChangeIntent:     "change_behavior",
		IntendedFiles:    []string{"internal/cli/interface.go"},
		RiskSignals: []RiskSignal{
			{ID: "governed_file", Source: "test"},
			{ID: "behavior_change", Source: "test"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertMethodIDs(t, run, []string{
		"graph-preflight-before-governed-edit",
		"behavior-first-testing",
		"verification-before-completion",
	})
}

func TestValidateCloseRequiresGraphPreflightEvidence(t *testing.T) {
	run, err := Pull(PullInput{
		Task:        "Patch governed method behavior",
		RiskSignals: []RiskSignal{{ID: "governed_file", Source: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ValidateClose(run, CloseInput{
		GateResults: []GateResult{{
			GateID: "graph_preflight_recorded_before_governed_edit",
			Status: "satisfied",
		}},
	})
	if err == nil {
		t.Fatal("ValidateClose accepted graph preflight without evidence")
	}
	if !strings.Contains(err.Error(), "graph_preflight_recorded_before_governed_edit needs evidence_refs") {
		t.Fatalf("error = %v, want graph preflight evidence failure", err)
	}

	waivers := []Waiver{{
		GateID: "graph_preflight_recorded_before_governed_edit",
		Reason: "Fixture covers waived graph gate behavior.",
	}, {
		GateID: "fresh_verification_before_completion",
		Reason: "Fixture covers waived verification behavior.",
	}}
	if err := ValidateClose(run, CloseInput{Waivers: waivers}); err != nil {
		t.Fatalf("ValidateClose rejected explicit graph gate waiver: %v", err)
	}
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

func TestPullStoresCarryThroughItemsAsPending(t *testing.T) {
	run, err := Pull(PullInput{
		Task: "Apply accepted review finding",
		CarryThrough: []CarryThroughItem{{
			SourceRef:     "review:external",
			SourceItemRef: "finding-1",
			AcceptanceRef: "operator:accepted",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.CarryThrough) != 1 {
		t.Fatalf("carry through items = %#v", run.CarryThrough)
	}
	item := run.CarryThrough[0]
	if item.Disposition != CarryDispositionPending {
		t.Fatalf("disposition = %q, want pending", item.Disposition)
	}
	if item.AcceptanceRefKind != CarryAcceptanceKindOperatorMessage ||
		item.AcceptanceRefStatus != CarryAcceptanceStatusExternallyAsserted {
		t.Fatalf("acceptance posture = %s/%s, want operator_message/externally_asserted", item.AcceptanceRefKind, item.AcceptanceRefStatus)
	}
}

func TestPullRejectsMalformedCarryThroughAcceptanceRef(t *testing.T) {
	_, err := Pull(PullInput{
		Task: "Apply accepted review finding",
		CarryThrough: []CarryThroughItem{{
			SourceRef:     "review:external",
			SourceItemRef: "finding-1",
			AcceptanceRef: "accepted",
		}},
	})
	if err == nil {
		t.Fatal("Pull accepted malformed carry-through acceptance_ref")
	}
	if !strings.Contains(err.Error(), "malformed acceptance_ref") {
		t.Fatalf("error = %v, want malformed acceptance_ref", err)
	}
}

func TestCarryThroughAcceptancePostureRecognizesVerifiedRefs(t *testing.T) {
	cases := []struct {
		ref        string
		wantKind   string
		wantStatus string
	}{
		{ref: "operator_message:msg-1", wantKind: CarryAcceptanceKindOperatorMessage, wantStatus: CarryAcceptanceStatusVerified},
		{ref: "review_disposition:review-1", wantKind: CarryAcceptanceKindReviewDisposition, wantStatus: CarryAcceptanceStatusVerified},
		{ref: "dec-20260627-example", wantKind: CarryAcceptanceKindDecisionRecord, wantStatus: CarryAcceptanceStatusVerified},
		{ref: "manual_cli:receipt-1", wantKind: CarryAcceptanceKindManualCLIReceipt, wantStatus: CarryAcceptanceStatusVerified},
		{ref: "external:ticket-1", wantKind: CarryAcceptanceKindExternalUnverified, wantStatus: CarryAcceptanceStatusExternallyAsserted},
	}

	for _, tc := range cases {
		got := InferCarryThroughAcceptancePosture(tc.ref)
		if got.Kind != tc.wantKind || got.Status != tc.wantStatus {
			t.Fatalf("%s posture = %s/%s, want %s/%s", tc.ref, got.Kind, got.Status, tc.wantKind, tc.wantStatus)
		}
	}
}

func TestValidateCloseRequiresCarryThroughDisposition(t *testing.T) {
	run := MethodRun{
		Status: "open",
		CarryThrough: []CarryThroughItem{{
			SourceRef:     "review:external",
			SourceItemRef: "finding-1",
			AcceptanceRef: "operator:accepted",
			Disposition:   CarryDispositionPending,
		}},
	}

	err := ValidateClose(run, CloseInput{})
	if err == nil {
		t.Fatal("ValidateClose accepted undisposed carry-through item")
	}
	for _, want := range []string{"review:external#finding-1", "carry_through close disposition"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}

	err = ValidateClose(run, CloseInput{
		CarryThrough: []CarryThroughItem{{
			SourceRef:     "review:external",
			SourceItemRef: "finding-1",
			AcceptanceRef: "operator:accepted",
			Disposition:   CarryDispositionApplied,
		}},
	})
	if err == nil {
		t.Fatal("ValidateClose accepted applied carry-through without target_refs")
	}
	if !strings.Contains(err.Error(), "applied disposition needs target_refs") {
		t.Fatalf("error = %v, want target_refs failure", err)
	}

	err = ValidateClose(run, CloseInput{
		CarryThrough: []CarryThroughItem{{
			SourceRef:     "review:external",
			SourceItemRef: "finding-1",
			AcceptanceRef: "operator:accepted",
			Disposition:   CarryDispositionApplied,
			TargetRefs:    []string{"internal/method/run.go::ValidateClose"},
		}},
	})
	if err != nil {
		t.Fatalf("ValidateClose rejected applied carry-through with target refs: %v", err)
	}
}

func TestValidateCloseAllowsPatternUseCarryThroughContext(t *testing.T) {
	run := MethodRun{
		Status: "open",
		CarryThrough: []CarryThroughItem{{
			SourceRef:     "pattern_use:compact",
			SourceItemRef: "query:именуй-нормально",
			AcceptanceRef: "operator_message:msg-1",
			Disposition:   CarryDispositionPending,
		}},
	}

	err := ValidateClose(run, CloseInput{
		CarryThrough: []CarryThroughItem{{
			SourceRef:     "pattern_use:compact",
			SourceItemRef: "query:именуй-нормально",
			AcceptanceRef: "operator_message:msg-1",
			Disposition:   CarryDispositionApplied,
			TargetRefs:    []string{"internal/fpf/patternuse.go"},
			Reason:        "PatternUse recommendation carried context only; tests remain the gate evidence.",
		}},
	})
	if err != nil {
		t.Fatalf("ValidateClose rejected PatternUse carry-through context: %v", err)
	}
}

func TestValidateCloseAllowsCarryThroughWaiver(t *testing.T) {
	run := MethodRun{
		Status: "open",
		CarryThrough: []CarryThroughItem{{
			SourceRef:     "review:external",
			SourceItemRef: "finding-1",
			AcceptanceRef: "operator:accepted",
			Disposition:   CarryDispositionPending,
		}},
	}

	err := ValidateClose(run, CloseInput{
		Waivers: []Waiver{{
			GateID: CarryThroughGateID,
			Reason: "Operator accepted leaving this item pending for a later slice.",
		}},
	})
	if err != nil {
		t.Fatalf("ValidateClose rejected carry-through waiver: %v", err)
	}
}

func TestValidateCloseReportsExpectedGateResultShape(t *testing.T) {
	run := MethodRun{
		Status: "open",
		Methods: []MethodCard{{
			ID: "test-method",
			HardGates: []Gate{{
				ID:               "gate-with-evidence",
				RequiredEvidence: []string{"command_output"},
				Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
			}},
		}},
	}

	err := ValidateClose(run, CloseInput{
		GateResults: []GateResult{{
			Status: "pass",
		}},
	})
	if err == nil {
		t.Fatal("ValidateClose accepted malformed close input")
	}
	for _, want := range []string{"gate_results[0] missing gate_id", "gate-with-evidence missing gate result", "expected gate_results[] shape", "status", "evidence_refs"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ValidateClose error missing %q: %v", want, err)
		}
	}
}

func TestBuildCloseTemplateNamesRequiredGateFields(t *testing.T) {
	run := MethodRun{
		ID: "mpull-test",
		Methods: []MethodCard{{
			HardGates: []Gate{{
				ID:               "gate-with-evidence",
				RequiredEvidence: []string{"command_output"},
			}},
		}},
		CarryThrough: []CarryThroughItem{{
			SourceRef:     "review:external",
			SourceItemRef: "finding-1",
			AcceptanceRef: "operator:accepted",
			Disposition:   CarryDispositionPending,
		}},
	}

	template := BuildCloseTemplate(run)
	if template.Action != "close" || template.PullID != "mpull-test" {
		t.Fatalf("template identity wrong: %+v", template)
	}
	if len(template.GateResults) != 1 {
		t.Fatalf("template gate count = %d, want 1", len(template.GateResults))
	}
	result := template.GateResults[0]
	if result.GateID != "gate-with-evidence" || result.Status != "satisfied" || len(result.EvidenceRefs) != 1 {
		t.Fatalf("template gate result missing required close fields: %+v", result)
	}
	if len(template.CarryThrough) != 1 {
		t.Fatalf("template carry_through = %#v, want one item", template.CarryThrough)
	}
	carry := template.CarryThrough[0]
	if carry.Disposition != CarryDispositionApplied || len(carry.TargetRefs) == 0 {
		t.Fatalf("template carry-through missing applied target hint: %+v", carry)
	}
	if carry.AcceptanceRefKind != CarryAcceptanceKindOperatorMessage ||
		carry.AcceptanceRefStatus != CarryAcceptanceStatusExternallyAsserted {
		t.Fatalf("template carry-through missing acceptance posture: %+v", carry)
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

func testMethodSourcePosture() SourcePosture {
	return SourcePosture{
		SourceKind:        MethodSourceKind,
		SourceEdition:     CatalogID + "@" + CatalogVersion,
		Normativity:       MethodSourceNormativity,
		AuthorityBoundary: MethodAuthorityBoundary,
	}
}

func validCatalogTestDefinition(id string) Definition {
	return Definition{
		ID:            id,
		Version:       CatalogVersion,
		Title:         "Valid " + id,
		SourcePosture: testMethodSourcePosture(),
		Lifecycle: Lifecycle{
			Status:    LifecycleCurrent,
			ValidFrom: "2026-06-25",
		},
		HardGates: []Gate{{
			ID:         "gate",
			Kind:       "test",
			CheckLevel: "deterministic",
		}},
		Waiver: WaiverPolicy{Allowed: true, RequiresReason: true},
	}
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := map[string]int{}
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
}
