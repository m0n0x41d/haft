package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestHandleQuintQuerySpecBindingPreflightReturnsBoundExisting(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := specBindingPreflightCLISection(
		"TS.sql.preflight.001",
		[]string{"symbol:internal/cli/spec.go::runSpecUse"},
	)
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	artifactStore := artifact.NewStore(database.GetRawDB())
	result, err := handleQuintQuery(context.Background(), artifactStore, nil, haftDirFor(root), map[string]any{
		"action":                 "spec_binding_preflight",
		"selected_title":         "Spec use preflight",
		"decision_subject_ref":   "symbol:internal/cli/spec.go::runSpecUse",
		"governance_target_refs": []any{"symbol:internal/cli/spec.go::runSpecUse"},
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_binding_preflight returned error: %v", err)
	}

	var record specflow.SpecBindingPreflightResult
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode preflight result: %v\n%s", err, result)
	}
	if record.RecordKind != specflow.SpecBindingPreflightRecordKind {
		t.Fatalf("record_kind = %q", record.RecordKind)
	}
	if record.State != specflow.SpecBindingStateBoundExisting {
		t.Fatalf("state = %q, want bound_existing; result=%s", record.State, result)
	}
	if len(record.SelectedSectionRefs) != 1 || record.SelectedSectionRefs[0] != "TS.sql.preflight.001" {
		t.Fatalf("selected_section_refs = %#v", record.SelectedSectionRefs)
	}
	if record.AuthorityBoundary != specflow.SpecBindingPreflightBoundary {
		t.Fatalf("authority_boundary = %q", record.AuthorityBoundary)
	}
}

func TestHandleQuintQuerySpecBindingPreflightValidatesDecisionDraftObject(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := specBindingPreflightCLISection("TS.sql.preflight.002", nil)
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	artifactStore := artifact.NewStore(database.GetRawDB())
	result, err := handleQuintQuery(context.Background(), artifactStore, nil, haftDirFor(root), map[string]any{
		"action": "spec_binding_preflight",
		"decision_draft": map[string]any{
			"selected_title": "Explicit refs",
			"section_refs":   []any{"spec_section:TS.sql.preflight.002"},
		},
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_binding_preflight returned error: %v", err)
	}

	var record specflow.SpecBindingPreflightResult
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode preflight result: %v\n%s", err, result)
	}
	if record.State != specflow.SpecBindingStateProvidedRefsValid {
		t.Fatalf("state = %q, want provided_refs_valid; result=%s", record.State, result)
	}
}

func TestHandleQuintQuerySpecBindingPreflightMatchesLinkedDecisionSectionRefs(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := specBindingPreflightCLISection("TS.sql.linked.001", nil)
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	artifactStore := artifact.NewStore(database.GetRawDB())
	if err := artifactStore.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     "prob-linked-section",
			Kind:   artifact.KindProblemCard,
			Status: artifact.StatusActive,
			Mode:   artifact.ModeStandard,
			Title:  "Problem with a linked section decision",
		},
		Body:           "The preflight must recover section refs from a decision linked to this problem.",
		StructuredData: `{}`,
	}); err != nil {
		t.Fatalf("seed linked problem: %v", err)
	}
	linkedDecisionFields, err := json.Marshal(artifact.DecisionFields{
		SectionRefs: []string{"TS.sql.linked.001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := artifactStore.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     "dec-linked-section",
			Kind:   artifact.KindDecisionRecord,
			Status: artifact.StatusActive,
			Mode:   artifact.ModeStandard,
			Title:  "Linked section decision",
			Links:  []artifact.Link{{Ref: "prob-linked-section", Type: "based_on"}},
		},
		StructuredData: string(linkedDecisionFields),
	}); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(context.Background(), artifactStore, nil, haftDirFor(root), map[string]any{
		"action":         "spec_binding_preflight",
		"selected_title": "Follow existing linked decision",
		"problem_ref":    "prob-linked-section",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_binding_preflight returned error: %v", err)
	}

	var record specflow.SpecBindingPreflightResult
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode preflight result: %v\n%s", err, result)
	}
	if record.State != specflow.SpecBindingStateBoundExisting {
		t.Fatalf("state = %q, want bound_existing; result=%s", record.State, result)
	}
	if len(record.CandidateSectionRefs) != 1 || !strings.Contains(strings.Join(record.CandidateSectionRefs[0].Basis, " "), "linked decision section_refs") {
		t.Fatalf("candidate basis = %#v", record.CandidateSectionRefs)
	}
}

func TestManualDecisionBindingInputRunsSpecBindingPreflight(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := specBindingPreflightCLISection(
		"TS.sql.decide.001",
		[]string{"symbol:internal/cli/spec.go::runSpecUse"},
	)
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	artifactStore := artifact.NewStore(database.GetRawDB())
	input := specBindingPreflightDecisionInput()
	input.SelectedTitle = "Spec use preflight"
	input.DecisionSubjectRef = "symbol:internal/cli/spec.go::runSpecUse"

	prepared, err := applyDecisionSpecBindingPreflight(
		context.Background(),
		artifactStore,
		haftDirFor(root),
		input,
	)
	if err != nil {
		t.Fatalf("prepare decision with auto preflight: %v", err)
	}
	if prepared.SpecBindingPreflight == nil || prepared.SpecBindingPreflight.State != artifact.SpecBindingStateBoundExisting {
		t.Fatalf("spec_binding_preflight = %#v, want bound_existing", prepared.SpecBindingPreflight)
	}
	reservation, err := artifact.NewDecisionReservation(
		"dec-20260715-spec-preflight-a1b2c3d4",
	)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := artifact.PrepareDecision(
		context.Background(),
		artifactStore,
		haftDirFor(root),
		reservation,
		prepared,
	)
	if err != nil {
		t.Fatalf("prepare exact decision snapshot: %v", err)
	}
	resolved, ok := decision.ResolvedInput()
	if !ok {
		t.Fatal("prepared decision omitted resolved input")
	}
	if len(resolved.SectionRefs) != 1 || resolved.SectionRefs[0] != "TS.sql.decide.001" {
		t.Fatalf("section_refs = %#v, want preflight-selected section", resolved.SectionRefs)
	}
}

func TestArtifactCreateProblemExploreComparePersistSpecFit(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := specBindingPreflightCLISection("TS.sql.autofit.001", nil)
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	artifactStore := artifact.NewStore(database.GetRawDB())
	problemPayload, err := json.Marshal(artifact.ProblemFrameInput{
		Title:  "Autofit problem",
		Signal: "TS.sql.autofit.001 needs a bounded trace through artifacts.",
		Scope:  "Spec-bound workflow",
	})
	if err != nil {
		t.Fatal(err)
	}
	problemResult, err := createArtifactFromInput(context.Background(), artifactStore, haftDirFor(root), "problem.frame", problemPayload)
	if err != nil {
		t.Fatalf("create problem: %v", err)
	}
	problem, err := artifactStore.Get(context.Background(), problemResult.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := problem.UnmarshalProblemFields().SpecFit; got == nil || got.State != specflow.SpecFitStateRelatesExisting {
		t.Fatalf("problem spec_fit = %#v", got)
	}

	explorePayload, err := json.Marshal(artifact.ExploreInput{
		ProblemRef: problemResult.ID,
		Variants: []artifact.Variant{
			{Title: "Use TS.sql.autofit.001", WeakestLink: "Spec relation can drift.", NoveltyMarker: "uses active section", SteppingStone: true, SteppingStoneBasis: "opens trace surface"},
			{Title: "Ignore spec relation", WeakestLink: "Late decision binding fails.", NoveltyMarker: "keeps workflow unbound"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	portfolioResult, err := createArtifactFromInput(context.Background(), artifactStore, haftDirFor(root), "solution.explore", explorePayload)
	if err != nil {
		t.Fatalf("create portfolio: %v", err)
	}
	portfolio, err := artifactStore.Get(context.Background(), portfolioResult.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := portfolio.UnmarshalPortfolioFields().SpecFit; got == nil || len(got.VariantSpecFit) == 0 {
		t.Fatalf("portfolio spec_fit = %#v", got)
	}

	comparePayload, err := json.Marshal(artifactCompareFileInput{
		PortfolioRef: portfolioResult.ID,
		Dimensions:   []string{"spec compatibility"},
		Scores: map[string]map[string]string{
			"V1": {"spec compatibility": "high"},
			"V2": {"spec compatibility": "low"},
		},
		NonDominatedSet: []string{"V1"},
		ParetoTradeoffs: []artifact.ParetoTradeoffNote{
			{Variant: "V1", Summary: "Best spec fit."},
			{Variant: "V2", Summary: "Retained by nominal score comparison but weaker by declared spec-fit rationale."},
		},
		DominatedVariants: []artifact.DominatedVariantExplanation{{
			Variant:     "V2",
			DominatedBy: []string{"V1"},
			Summary:     "Weaker spec fit.",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = createArtifactFromInput(context.Background(), artifactStore, haftDirFor(root), "solution.compare", comparePayload)
	if err != nil {
		t.Fatalf("compare portfolio: %v", err)
	}
	compared, err := artifactStore.Get(context.Background(), portfolioResult.ID)
	if err != nil {
		t.Fatal(err)
	}
	comparison := compared.UnmarshalPortfolioFields().Comparison
	if comparison == nil || len(comparison.VariantSpecFit) == 0 {
		t.Fatalf("comparison variant_spec_fit = %#v", comparison)
	}
	if !strings.Contains(compared.Body, "Variant Spec Fit") {
		t.Fatalf("comparison body missing Variant Spec Fit section:\n%s", compared.Body)
	}
}

func TestManualDecisionBindingInputBlocksInvalidSpecRefs(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := specBindingPreflightCLISection("TS.sql.decide.002", nil)
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	artifactStore := artifact.NewStore(database.GetRawDB())
	input := specBindingPreflightDecisionInput()
	input.SectionRefs = []string{"TS.missing.999"}

	prepared, err := applyDecisionSpecBindingPreflight(
		context.Background(),
		artifactStore,
		haftDirFor(root),
		input,
	)
	if err == nil {
		reservation, reservationErr := artifact.NewDecisionReservation(
			"dec-20260715-invalid-spec-a1b2c3d4",
		)
		if reservationErr != nil {
			t.Fatal(reservationErr)
		}
		_, err = artifact.PrepareDecision(
			context.Background(),
			artifactStore,
			haftDirFor(root),
			reservation,
			prepared,
		)
	}
	if err == nil {
		t.Fatal("expected invalid spec section refs to block decision preparation")
	}
	if !strings.Contains(err.Error(), "spec_binding_preflight blocks decision creation") {
		t.Fatalf("error = %v", err)
	}
}

func TestHandleQuintQuerySpecTraceReturnsDecisionAndCodeDrilldown(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := specBindingPreflightCLISection("TS.sql.trace.001", nil)
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	artifactStore := artifact.NewStore(database.GetRawDB())
	decisionFields, err := json.Marshal(artifact.DecisionFields{
		DecisionSubjectRef: "trace-subject",
		SelectedTitle:      "Trace section",
		WhySelected:        "Trace must expose code context.",
		SectionRefs:        []string{"TS.sql.trace.001"},
		ImplementationFootprint: artifact.ImplementationFootprint{
			Files: []string{"internal/cli/spec_trace.go"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := artifactStore.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     "dec-trace-section",
			Kind:   artifact.KindDecisionRecord,
			Status: artifact.StatusActive,
			Mode:   artifact.ModeStandard,
			Title:  "Trace section",
		},
		StructuredData: string(decisionFields),
	}); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(context.Background(), artifactStore, nil, haftDirFor(root), map[string]any{
		"action":     "spec_trace",
		"section_id": "TS.sql.trace.001",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_trace returned error: %v", err)
	}

	var record specTraceRecord
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode spec trace result: %v\n%s", err, result)
	}
	if record.RecordKind != "spec_trace" {
		t.Fatalf("record_kind = %q", record.RecordKind)
	}
	if len(record.CurrentAuthority.DerivedSectionRefs) != 1 || record.CurrentAuthority.DerivedSectionRefs[0] != "dec-trace-section" {
		t.Fatalf("derived refs = %#v", record.CurrentAuthority.DerivedSectionRefs)
	}
	if len(record.CodeBindings) != 1 || len(record.CodeBindings[0].CodeContextDrilldown) == 0 {
		t.Fatalf("code bindings = %#v", record.CodeBindings)
	}
}

func TestHandleQuintQuerySpecFitProbeReturnsVariantFit(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := specBindingPreflightCLISection(
		"TS.sql.fit.001",
		[]string{"symbol:internal/cli/spec.go::runSpecUse"},
	)
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	artifactStore := artifact.NewStore(database.GetRawDB())
	result, err := handleQuintQuery(context.Background(), artifactStore, nil, haftDirFor(root), map[string]any{
		"action":         "spec_fit_probe",
		"problem_signal": "Spec use needs a comparison.",
		"target_refs":    []any{"symbol:internal/cli/spec.go::runSpecUse"},
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_fit_probe returned error: %v", err)
	}

	var record specflow.SpecFitProbeResult
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode spec_fit_probe result: %v\n%s", err, result)
	}
	if record.RecordKind != specflow.SpecFitProbeRecordKind {
		t.Fatalf("record_kind = %q", record.RecordKind)
	}
	if record.State != specflow.SpecFitStateRelatesExisting {
		t.Fatalf("state = %q, want relates_existing; result=%s", record.State, result)
	}
	if len(record.CandidateSectionRefs) != 1 || record.CandidateSectionRefs[0] != "TS.sql.fit.001" {
		t.Fatalf("candidate_section_refs = %#v", record.CandidateSectionRefs)
	}
}

func TestSpecBindingDebtReportFromSpecificationSetBucketsDecisionDebt(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := specBindingPreflightCLISection("TS.sql.status.001", nil)
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	artifactStore := artifact.NewStore(database.GetRawDB())
	createSpecBindingDebtDecision(t, artifactStore, "dec-missing", artifact.ModeStandard, artifact.DecisionFields{})
	createSpecBindingDebtDecision(t, artifactStore, "dec-invalid", artifact.ModeStandard, artifact.DecisionFields{
		SectionRefs: []string{"TS.missing.999"},
	})
	createSpecBindingDebtDecision(t, artifactStore, "dec-draft-needed", artifact.ModeStandard, artifact.DecisionFields{
		SpecBindingPreflight: &artifact.SpecBindingPreflight{State: artifact.SpecBindingStateDraftNeeded},
	})
	createSpecBindingDebtDecision(t, artifactStore, "dec-out-of-spec", artifact.ModeTactical, artifact.DecisionFields{
		SpecBindingPreflight: &artifact.SpecBindingPreflight{
			State: artifact.SpecBindingStateOutOfSpec,
			StatusDebt: artifact.SpecBindingStatusDebt{
				Message: "explicit tactical exception",
			},
		},
	})

	specificationSet, err := loadProjectSpecificationSetSQLFirst(root)
	if err != nil {
		t.Fatalf("load SQL-first specification set: %v", err)
	}
	report := specBindingDebtReportFromSpecificationSet(
		context.Background(),
		artifactStore,
		specificationSet,
	)
	if report.Summary.DecisionsMissingSpecBinding != 1 {
		t.Fatalf("missing = %d, want 1", report.Summary.DecisionsMissingSpecBinding)
	}
	if report.Summary.DecisionsWithInvalidSpecRefs != 1 {
		t.Fatalf("invalid = %d, want 1", report.Summary.DecisionsWithInvalidSpecRefs)
	}
	if report.Summary.DraftSectionNeededDebt != 1 {
		t.Fatalf("draft_needed = %d, want 1", report.Summary.DraftSectionNeededDebt)
	}
	if report.Summary.OutOfSpecDecisionDebt != 1 {
		t.Fatalf("out_of_spec = %d, want 1", report.Summary.OutOfSpecDecisionDebt)
	}
}

func createSpecBindingDebtDecision(
	t *testing.T,
	store *artifact.Store,
	id string,
	mode artifact.Mode,
	fields artifact.DecisionFields,
) {
	t.Helper()
	payload, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     id,
			Kind:   artifact.KindDecisionRecord,
			Status: artifact.StatusActive,
			Mode:   mode,
			Title:  id,
		},
		StructuredData: string(payload),
	}); err != nil {
		t.Fatal(err)
	}
}

func specBindingPreflightDecisionInput() artifact.DecideInput {
	return artifact.DecideInput{
		SelectedTitle:    "Spec-bound decision",
		ProblemStatement: "A load-bearing decision needs an explicit relation to the active ProjectSpecificationSet.",
		WhySelected:      "The selected option is governed by the current ProjectSpecificationSet.",
		SelectionPolicy:  "Prefer the option that satisfies the active spec section.",
		WeakestLink:      "Spec binding could be stale if the ProjectSpecificationSet changes.",
		CounterArgument:  "The work might belong to a different section.",
		WhyNotOthers: []artifact.RejectionReason{
			{Variant: "Leave unbound", Reason: "Load-bearing spec-enabled decisions need an explicit relation."},
		},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Spec binding is shown wrong by status or review."},
		},
		Predictions: []artifact.PredictionInput{
			{Claim: "Spec binding is visible", Observable: "decision structured data", Threshold: "section_refs present"},
		},
		Invariants:    []string{"Spec binding remains explicit."},
		AffectedFiles: []string{"internal/cli/spec.go"},
		ValidUntil:    "2026-08-08T00:00:00+04:00",
	}
}

func specBindingPreflightCLISection(id string, targetRefs []string) project.SpecSection {
	return project.SpecSection{
		ID:            id,
		Spec:          "target-system",
		Kind:          "target.boundary",
		Title:         "SQL preflight section",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		ValidUntil:    "2026-12-31",
		TargetRefs:    targetRefs,
		DocumentKind:  string(project.SpecDocumentKindTargetSystem),
		Path:          ".haft/specs/target-system.md",
	}
}
