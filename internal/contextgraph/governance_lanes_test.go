package contextgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
)

func TestFetchCodeContextSeparatesExactBindingAndModuleContext(
	t *testing.T,
) {
	store, graphStore, database := setupContextDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	mustContextExec(
		t,
		database,
		`INSERT INTO codebase_modules
			(module_id, path, name, lang, file_count, last_scanned)
		 VALUES
			('mod-root', '', 'root', 'go', 1, ?),
			('mod-cli', 'internal/cli', 'cli', 'go', 3, ?),
			('mod-cli-sub', 'internal/cli/sub', 'cli-sub', 'go', 1, ?),
			('mod-client', 'internal/client', 'client', 'go', 1, ?)`,
		now,
		now,
		now,
		now,
	)
	for _, filePath := range []string{
		"internal/cli/handler.go",
		"internal/cli/module.go",
		"internal/cli/exact.go",
		"internal/cli/generated/child.go",
		"internal/cli/sub/nested.go",
	} {
		mustContextExec(
			t,
			database,
			`INSERT INTO code_files (file_path) VALUES (?)`,
			filePath,
		)
	}

	createDecision := func(
		id string,
		status artifact.Status,
		affectedPath string,
		fields artifact.DecisionFields,
	) {
		t.Helper()
		structured, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		item := &artifact.Artifact{
			Meta: artifact.Meta{
				ID:        id,
				Kind:      artifact.KindDecisionRecord,
				Status:    status,
				Title:     id,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Body:           "fixture",
			StructuredData: string(structured),
		}
		if err := store.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
		if affectedPath != "" {
			mustContextExec(
				t,
				database,
				`INSERT INTO affected_files (artifact_id, file_path) VALUES (?, ?)`,
				id,
				affectedPath,
			)
		}
	}

	createDecision(
		"dec-binding",
		artifact.StatusActive,
		"",
		artifact.DecisionFields{
			Invariants: []string{"exact binding invariant"},
			BindingTargets: []artifact.BindingTarget{{
				Kind:       artifact.BindingTargetSymbol,
				FilePath:   "internal/cli/handler.go",
				SymbolName: "Handle",
				Line:       10,
				EndLine:    20,
			}},
		},
	)
	createDecision(
		"dec-explicit-module",
		artifact.StatusActive,
		"",
		artifact.DecisionFields{
			Invariants: []string{"explicit module invariant"},
			BindingTargets: []artifact.BindingTarget{{
				Kind:       artifact.BindingTargetModule,
				ModulePath: "internal/cli",
			}},
		},
	)
	createDecision(
		"dec-20260716-11f33e36",
		artifact.StatusActive,
		"internal/cli",
		artifact.DecisionFields{
			Invariants: []string{"typed-memory architecture invariant"},
			BindingTargets: []artifact.BindingTarget{{
				Kind:     artifact.BindingTargetSymbol,
				FilePath: "db/migrations.go",
			}},
		},
	)
	createDecision(
		"dec-typed-sibling-file",
		artifact.StatusActive,
		"internal/cli/module.go",
		artifact.DecisionFields{
			Invariants: []string{"typed sibling must not widen"},
			BindingTargets: []artifact.BindingTarget{{
				Kind:     artifact.BindingTargetSymbol,
				FilePath: "db/migrations.go",
			}},
		},
	)
	createDecision(
		"dec-20260713-9ed66ef0",
		artifact.StatusSuperseded,
		"internal/cli",
		artifact.DecisionFields{
			Invariants: []string{"superseded typed-memory invariant"},
			BindingTargets: []artifact.BindingTarget{{
				Kind:     artifact.BindingTargetSymbol,
				FilePath: "db/migrations.go",
			}},
		},
	)
	createDecision(
		"dec-footprint",
		artifact.StatusActive,
		"internal/cli/handler.go",
		artifact.DecisionFields{
			Invariants: []string{"must never become authority"},
			ImplementationFootprint: artifact.ImplementationFootprint{
				Files: []string{"internal/cli/handler.go"},
			},
		},
	)
	createDecision(
		"dec-module",
		artifact.StatusRefreshDue,
		"internal/cli/module.go",
		artifact.DecisionFields{
			Invariants: []string{"module context invariant"},
		},
	)
	createDecision(
		"dec-module-root",
		artifact.StatusActive,
		"internal/cli",
		artifact.DecisionFields{
			Invariants: []string{"directory root context invariant"},
		},
	)
	createDecision(
		"dec-nonmodule-directory",
		artifact.StatusActive,
		"internal/cli/generated",
		artifact.DecisionFields{
			Invariants: []string{"non-module directory must not widen"},
		},
	)
	createDecision(
		"dec-exact-sibling",
		artifact.StatusActive,
		"internal/cli/exact.go",
		artifact.DecisionFields{
			GovernanceMode: artifact.GovernanceModeExact,
			Invariants:     []string{"exact sibling must not widen"},
		},
	)
	createDecision(
		"dec-terminal",
		artifact.StatusSuperseded,
		"internal/cli/handler.go",
		artifact.DecisionFields{
			Invariants: []string{"terminal invariant"},
			BindingTargets: []artifact.BindingTarget{{
				Kind:     artifact.BindingTargetWholeFileFallback,
				FilePath: "internal/cli/handler.go",
			}},
		},
	)
	createDecision(
		"dec-nested",
		artifact.StatusActive,
		"internal/cli/sub/nested.go",
		artifact.DecisionFields{
			Invariants: []string{"nested module invariant"},
		},
	)

	codeContext, err := FetchCodeContext(
		ctx,
		store,
		graphStore,
		Target{
			File:   "internal/cli/handler.go",
			Symbol: "Handle",
			Line:   15,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	assertArtifactIDs(
		t,
		codeContext.ExactBindingDecisions,
		[]string{"dec-binding"},
	)
	assertArtifactIDs(
		t,
		codeContext.AffectedPathContextDecisions,
		[]string{"dec-footprint"},
	)
	assertNodeIDs(
		t,
		codeContext.ModuleDecisions,
		[]string{
			"dec-20260716-11f33e36",
			"dec-explicit-module",
			"dec-module",
			"dec-module-root",
		},
	)
	relevantIndex := -1
	for index, decision := range codeContext.ModuleDecisions {
		if decision.ID == "dec-20260716-11f33e36" {
			relevantIndex = index
			break
		}
	}
	if relevantIndex < 0 || relevantIndex >= 5 {
		t.Fatalf(
			"active typed-memory module decision index = %d, decisions = %+v",
			relevantIndex,
			codeContext.ModuleDecisions,
		)
	}
	assertArtifactIDs(t, codeContext.Decisions, []string{"dec-binding"})

	if len(codeContext.Invariants) != 1 ||
		codeContext.Invariants[0].DecisionID != "dec-binding" {
		t.Fatalf("binding invariants = %+v", codeContext.Invariants)
	}
	contextIDs := make(map[string]bool)
	for _, invariant := range codeContext.ContextInvariants {
		contextIDs[invariant.DecisionID] = true
	}
	for _, expected := range []string{
		"dec-20260716-11f33e36",
		"dec-explicit-module",
		"dec-module",
		"dec-module-root",
	} {
		if !contextIDs[expected] {
			t.Fatalf(
				"module invariant %s missing: %+v",
				expected,
				codeContext.ContextInvariants,
			)
		}
	}
	for _, excluded := range []string{
		"dec-footprint",
		"dec-nonmodule-directory",
		"dec-exact-sibling",
		"dec-terminal",
		"dec-20260713-9ed66ef0",
		"dec-typed-sibling-file",
		"dec-nested",
	} {
		if contextIDs[excluded] {
			t.Fatalf(
				"non-module context %s leaked into invariants: %+v",
				excluded,
				codeContext.ContextInvariants,
			)
		}
	}
}

func assertArtifactIDs(
	t *testing.T,
	items []*artifact.Artifact,
	expected []string,
) {
	t.Helper()
	actual := make([]string, 0, len(items))
	for _, item := range items {
		actual = append(actual, item.Meta.ID)
	}
	assertStringSet(t, actual, expected)
}

func assertNodeIDs(
	t *testing.T,
	items []graph.Node,
	expected []string,
) {
	t.Helper()
	actual := make([]string, 0, len(items))
	for _, item := range items {
		actual = append(actual, item.ID)
	}
	assertStringSet(t, actual, expected)
}

func assertStringSet(t *testing.T, actual, expected []string) {
	t.Helper()
	actualSet := make(map[string]bool, len(actual))
	for _, item := range actual {
		actualSet[item] = true
	}
	if len(actualSet) != len(expected) {
		t.Fatalf("actual=%v expected=%v", actual, expected)
	}
	for _, item := range expected {
		if !actualSet[item] {
			t.Fatalf("actual=%v expected=%v", actual, expected)
		}
	}
}

func mustContextExec(
	t *testing.T,
	database interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	},
	statement string,
	args ...any,
) {
	t.Helper()
	if _, err := database.ExecContext(
		context.Background(),
		statement,
		args...,
	); err != nil {
		t.Fatal(err)
	}
}
