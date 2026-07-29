package artifact

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestComputeProjectStateView_EmptyProjectHasNoImpliedPhase(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	state := ComputeProjectStateView(ctx, store, "test-ctx")

	if !state.Problems.Known || !state.Options.Known || !state.Decisions.Known {
		t.Fatalf("artifact facets should be known: %#v", state)
	}
	if len(state.Problems.Open) != 0 {
		t.Fatalf("open problems = %#v, want none", state.Problems.Open)
	}
	if len(state.Options.Sets) != 0 {
		t.Fatalf("option sets = %#v, want none", state.Options.Sets)
	}
	if len(state.Decisions.Active) != 0 {
		t.Fatalf("active decisions = %#v, want none", state.Decisions.Active)
	}
}

func TestComputeProjectStateView_FacetsCoexist(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	contextName := "coexisting-facets"

	createProjectStateArtifact(t, ctx, store, &Artifact{
		Meta: Meta{
			ID:      "prob-coexisting",
			Kind:    KindProblemCard,
			Status:  StatusActive,
			Context: contextName,
			Mode:    ModeStandard,
			Title:   "Keep the question open",
		},
	})
	createProjectStateArtifact(t, ctx, store, &Artifact{
		Meta: Meta{
			ID:      "sol-coexisting",
			Kind:    KindSolutionPortfolio,
			Status:  StatusActive,
			Context: contextName,
			Mode:    ModeStandard,
			Title:   "Current options",
		},
		Body: "## Comparison\n\nThe options were compared.",
	})
	createProjectStateArtifact(t, ctx, store, &Artifact{
		Meta: Meta{
			ID:      "sol-open-comparison",
			Kind:    KindSolutionPortfolio,
			Status:  StatusActive,
			Context: contextName,
			Mode:    ModeTactical,
			Title:   "Options not yet compared",
		},
	})
	createProjectStateArtifact(t, ctx, store, &Artifact{
		Meta: Meta{
			ID:      "dec-coexisting",
			Kind:    KindDecisionRecord,
			Status:  StatusActive,
			Context: contextName,
			Mode:    ModeTactical,
			Title:   "A current decision",
		},
	})
	createProjectStateCommission(t, ctx, store, contextName, map[string]any{
		"id":           "wc-coexisting",
		"decision_ref": "dec-coexisting",
		"state":        "queued",
		"valid_until":  "2026-07-20T00:00:00Z",
	})

	state := ComputeProjectStateView(ctx, store, contextName)

	if len(state.Problems.Open) != 1 {
		t.Fatalf("open problems = %#v, want one", state.Problems.Open)
	}
	if len(state.Options.Sets) != 2 {
		t.Fatalf("option sets = %#v, want two", state.Options.Sets)
	}
	if !state.Options.Sets[0].ComparisonRecorded {
		t.Fatalf("option set should retain its local comparison fact: %#v", state.Options.Sets[0])
	}
	if state.Options.Sets[1].ComparisonRecorded {
		t.Fatalf("uncompared option set should remain visible: %#v", state.Options.Sets[1])
	}
	if len(state.Decisions.Active) != 1 {
		t.Fatalf("active decisions = %#v, want one", state.Decisions.Active)
	}
	if len(state.Work.Active) != 1 || state.Work.Active[0].ID != "wc-coexisting" {
		t.Fatalf("active work = %#v, want wc-coexisting", state.Work.Active)
	}
}

func TestComputeProjectStateView_EvidencePressureDoesNotReplaceOtherFacets(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	contextName := "stale-and-current"

	createProjectStateArtifact(t, ctx, store, &Artifact{
		Meta: Meta{
			ID:      "prob-stale-and-current",
			Kind:    KindProblemCard,
			Status:  StatusActive,
			Context: contextName,
			Title:   "An open question remains visible",
		},
	})
	createProjectStateArtifact(t, ctx, store, &Artifact{
		Meta: Meta{
			ID:         "dec-stale-other-context",
			Kind:       KindDecisionRecord,
			Status:     StatusRefreshDue,
			Context:    "other-context",
			Title:      "Unrelated stale decision",
			ValidUntil: "2026-07-01",
		},
	})
	createProjectStateArtifact(t, ctx, store, &Artifact{
		Meta: Meta{
			ID:         "dec-stale-and-current",
			Kind:       KindDecisionRecord,
			Status:     StatusRefreshDue,
			Context:    contextName,
			Title:      "Decision needing new evidence",
			ValidUntil: "2026-07-01",
		},
	})

	state := ComputeProjectStateView(ctx, store, contextName)

	if len(state.Problems.Open) != 1 || len(state.Decisions.Active) != 1 {
		t.Fatalf("current facets were collapsed by evidence pressure: %#v", state)
	}
	if state.StaleCount != 1 || len(state.StaleItems) != 1 {
		t.Fatalf("evidence pressure = %d %#v, want one item", state.StaleCount, state.StaleItems)
	}
}

func TestComputeProjectStateView_WorkFacetKeepsOnlyNonTerminalCommissions(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	contextName := "commission-context"

	createProjectStateArtifact(t, ctx, store, &Artifact{
		Meta: Meta{
			ID:      "dec-commission-context",
			Kind:    KindDecisionRecord,
			Status:  StatusActive,
			Context: contextName,
			Title:   "Decision commissioning work",
		},
	})
	createProjectStateCommission(t, ctx, store, contextName, map[string]any{
		"id":           "wc-current",
		"decision_ref": "dec-commission-context",
		"state":        "queued",
		"valid_until":  "2026-07-20T00:00:00Z",
	})
	createProjectStateCommission(t, ctx, store, contextName, map[string]any{
		"id":           "wc-completed",
		"decision_ref": "dec-commission-context",
		"state":        "completed",
		"valid_until":  "2026-07-20T00:00:00Z",
	})

	state := ComputeProjectStateView(ctx, store, contextName)

	if !state.Work.Known {
		t.Fatal("work facet should be known")
	}
	if len(state.Work.Active) != 1 || state.Work.Active[0].ID != "wc-current" {
		t.Fatalf("active work = %#v, want only wc-current", state.Work.Active)
	}
	if len(state.Work.Active[0].SuggestedActions) == 0 {
		t.Fatalf("local WorkCommission actions should be retained: %#v", state.Work.Active[0])
	}
}

func TestDeriveProjectStateView_IsInvariantToArtifactOrder(t *testing.T) {
	problem := &Artifact{Meta: Meta{
		ID: "prob-order", Kind: KindProblemCard, Status: StatusActive, Title: "Problem",
	}}
	portfolio := &Artifact{Meta: Meta{
		ID: "sol-order", Kind: KindSolutionPortfolio, Status: StatusActive, Title: "Options",
	}}
	decision := &Artifact{Meta: Meta{
		ID: "dec-order", Kind: KindDecisionRecord, Status: StatusActive, Title: "Decision",
	}}
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)

	left := deriveProjectStateView(projectStateInput{
		Artifacts:      []*Artifact{problem, portfolio, decision},
		ArtifactsKnown: true,
		EvidenceKnown:  true,
		WorkKnown:      true,
		Now:            now,
	})
	right := deriveProjectStateView(projectStateInput{
		Artifacts:      []*Artifact{decision, problem, portfolio},
		ArtifactsKnown: true,
		EvidenceKnown:  true,
		WorkKnown:      true,
		Now:            now,
	})

	if !reflect.DeepEqual(left, right) {
		t.Fatalf("artifact order changed the project state view:\nleft=%#v\nright=%#v", left, right)
	}
}

func TestDeriveProjectStateView_UnknownIsDistinctFromKnownEmpty(t *testing.T) {
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	unknown := deriveProjectStateView(projectStateInput{Now: now})
	knownEmpty := deriveProjectStateView(projectStateInput{
		ArtifactsKnown: true,
		EvidenceKnown:  true,
		WorkKnown:      true,
		Now:            now,
	})

	if unknown.Problems.Known || unknown.Options.Known || unknown.Decisions.Known {
		t.Fatalf("unavailable artifact input was reported as known: %#v", unknown)
	}
	if unknown.Work.Known || unknown.EvidenceKnown {
		t.Fatalf("unavailable work/evidence input was reported as known: %#v", unknown)
	}
	if !knownEmpty.Problems.Known || !knownEmpty.Options.Known || !knownEmpty.Decisions.Known {
		t.Fatalf("known-empty artifact input lost availability: %#v", knownEmpty)
	}
	if !knownEmpty.Work.Known || !knownEmpty.EvidenceKnown {
		t.Fatalf("known-empty work/evidence input lost availability: %#v", knownEmpty)
	}
}

func TestProjectStateView_HasNoGlobalPhaseOrNextActionFields(t *testing.T) {
	viewType := reflect.TypeOf(ProjectStateView{})
	for _, fieldName := range []string{"DerivedStatus", "NextAction", "Mode"} {
		if _, found := viewType.FieldByName(fieldName); found {
			t.Fatalf("ProjectStateView must not expose global field %q", fieldName)
		}
	}
}

func TestComputeProjectStateView_ReadsLegacyArtifactsWithoutMigration(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	legacy := &Artifact{
		Meta: Meta{
			ID:      "prob-legacy-carrier",
			Kind:    KindProblemCard,
			Status:  StatusActive,
			Context: "legacy-context",
			Title:   "Legacy problem carrier",
		},
		Body: "Legacy markdown body without ProjectStateView fields.",
	}
	createProjectStateArtifact(t, ctx, store, legacy)

	state := ComputeProjectStateView(ctx, store, "legacy-context")
	loaded, err := store.Get(ctx, legacy.Meta.ID)
	if err != nil {
		t.Fatalf("load legacy artifact: %v", err)
	}

	if len(state.Problems.Open) != 1 || state.Problems.Open[0].ID != legacy.Meta.ID {
		t.Fatalf("legacy artifact missing from state view: %#v", state.Problems.Open)
	}
	if loaded.Body != legacy.Body || loaded.Meta.Status != StatusActive {
		t.Fatalf("state projection mutated legacy carrier: %#v", loaded)
	}
}

func TestComputeNavState_CompatibilityEntrypointReturnsProjectStateView(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	legacyEntrypoint := ComputeNavState(ctx, store, "compatibility")
	canonicalEntrypoint := ComputeProjectStateView(ctx, store, "compatibility")

	if !reflect.DeepEqual(legacyEntrypoint, canonicalEntrypoint) {
		t.Fatalf("compatibility entrypoint diverged:\nlegacy=%#v\ncanonical=%#v", legacyEntrypoint, canonicalEntrypoint)
	}
}

func createProjectStateArtifact(
	t *testing.T,
	ctx context.Context,
	store ArtifactStore,
	item *Artifact,
) {
	t.Helper()
	if err := store.Create(ctx, item); err != nil {
		t.Fatalf("create %s: %v", item.Meta.ID, err)
	}
}

func createProjectStateCommission(
	t *testing.T,
	ctx context.Context,
	store ArtifactStore,
	contextName string,
	payload map[string]any,
) {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode WorkCommission: %v", err)
	}

	id := payload["id"].(string)
	validUntil := payload["valid_until"].(string)
	item := &Artifact{
		Meta: Meta{
			ID:         id,
			Kind:       KindWorkCommission,
			Status:     StatusActive,
			Context:    contextName,
			Title:      "WorkCommission " + id,
			ValidUntil: validUntil,
		},
		StructuredData: string(encoded),
	}
	createProjectStateArtifact(t, ctx, store, item)
}
