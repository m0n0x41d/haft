package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestScanGovernanceAttention_SurfacesProblemCountsOrphansAndInvariantViolations(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	mustExecGovernanceAttentionSQL(t, store,
		`INSERT INTO codebase_modules(module_id, path, name, lang, last_scanned) VALUES
			('mod-api', 'internal/api', 'api', 'go', CURRENT_TIMESTAMP),
			('mod-db', 'internal/database', 'database', 'go', CURRENT_TIMESTAMP)`,
		`INSERT INTO module_dependencies(source_module, target_module, dep_type, last_scanned) VALUES
			('mod-api', 'mod-db', 'import', CURRENT_TIMESTAMP)`,
	)

	backlogProblem := &artifact.Artifact{
		Meta: artifact.Meta{ID: "prob-backlog", Kind: artifact.KindProblemCard, Status: artifact.StatusActive, Title: "Backlog problem"},
		Body: "problem",
	}
	if err := store.Create(ctx, backlogProblem); err != nil {
		t.Fatal(err)
	}

	inProgressProblem := &artifact.Artifact{
		Meta: artifact.Meta{ID: "prob-progress", Kind: artifact.KindProblemCard, Status: artifact.StatusActive, Title: "In progress problem"},
		Body: "problem",
	}
	if err := store.Create(ctx, inProgressProblem); err != nil {
		t.Fatal(err)
	}

	portfolio := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     "sol-001",
			Kind:   artifact.KindSolutionPortfolio,
			Status: artifact.StatusActive,
			Title:  "Portfolio",
			Links:  []artifact.Link{{Ref: inProgressProblem.Meta.ID, Type: "based_on"}},
		},
		Body: "portfolio",
	}
	if err := store.Create(ctx, portfolio); err != nil {
		t.Fatal(err)
	}

	addressedProblem := &artifact.Artifact{
		Meta: artifact.Meta{ID: "prob-addressed", Kind: artifact.KindProblemCard, Status: artifact.StatusAddressed, Title: "Addressed without decision"},
		Body: "problem",
	}
	if err := store.Create(ctx, addressedProblem); err != nil {
		t.Fatal(err)
	}

	structuredJSON, err := json.Marshal(artifact.DecisionFields{
		Invariants: []string{"no dependency from api to database"},
	})
	if err != nil {
		t.Fatal(err)
	}

	decision := &artifact.Artifact{
		Meta: artifact.Meta{ID: "dec-001", Kind: artifact.KindDecisionRecord, Status: artifact.StatusActive, Title: "Invariant decision"},
		Body: "decision",
		StructuredData: string(structuredJSON),
	}
	if err := store.Create(ctx, decision); err != nil {
		t.Fatal(err)
	}

	attention := scanGovernanceAttention(ctx, store)

	if attention.BacklogCount != 1 {
		t.Fatalf("BacklogCount = %d, want 1", attention.BacklogCount)
	}
	if attention.InProgressCount != 1 {
		t.Fatalf("InProgressCount = %d, want 1", attention.InProgressCount)
	}
	if len(attention.AddressedWithoutDecision) != 1 {
		t.Fatalf("AddressedWithoutDecision = %#v, want 1 item", attention.AddressedWithoutDecision)
	}
	if attention.AddressedWithoutDecision[0].ProblemID != "prob-addressed" {
		t.Fatalf("orphan problem = %#v", attention.AddressedWithoutDecision[0])
	}
	if len(attention.InvariantViolations) != 1 {
		t.Fatalf("InvariantViolations = %#v, want 1 item", attention.InvariantViolations)
	}
	if attention.InvariantViolations[0].DecisionID != "dec-001" {
		t.Fatalf("invariant violation = %#v", attention.InvariantViolations[0])
	}
}

func mustExecGovernanceAttentionSQL(t *testing.T, store *artifact.Store, statements ...string) {
	t.Helper()

	for _, statement := range statements {
		if _, err := store.DB().Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}
