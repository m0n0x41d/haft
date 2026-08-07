package cli

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
)

func TestOverseerCodeGovernancePathSemanticsMatrix(t *testing.T) {
	t.Parallel()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, statement := range []string{
		`CREATE TABLE code_files (file_path TEXT PRIMARY KEY)`,
		`INSERT INTO codebase_modules
			(module_id, path, name, lang, file_count, last_scanned)
		 VALUES
			('mod-root', '', 'root', 'go', 1, CURRENT_TIMESTAMP),
			('mod-cli', 'internal/cli', 'cli', 'go', 4, CURRENT_TIMESTAMP),
			('mod-cli-sub', 'internal/cli/sub', 'cli-sub', 'go', 1, CURRENT_TIMESTAMP),
			('mod-client', 'internal/client', 'client', 'go', 1, CURRENT_TIMESTAMP)`,
		`INSERT INTO code_files (file_path) VALUES
			('internal/cli/a.go'),
			('internal/cli/b.go'),
			('internal/cli/exact.go'),
			('internal/cli/generated/child.go'),
			('internal/cli/sub/nested.go'),
			('internal/client/client.go')`,
	} {
		if _, err := store.DB().Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	createDecision := func(
		id string,
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
				Status:    artifact.StatusActive,
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
			if err := store.SetAffectedFiles(
				ctx,
				id,
				[]artifact.AffectedFile{{Path: affectedPath}},
			); err != nil {
				t.Fatal(err)
			}
		}
	}

	createDecision(
		"dec-module",
		"internal/cli/a.go",
		artifact.DecisionFields{},
	)
	createDecision(
		"dec-module-root",
		"internal/cli",
		artifact.DecisionFields{},
	)
	createDecision(
		"dec-binding",
		"",
		artifact.DecisionFields{
			BindingTargets: []artifact.BindingTarget{{
				Kind:       artifact.BindingTargetSymbol,
				FilePath:   "internal/cli/b.go",
				SymbolName: "Run",
			}},
		},
	)
	createDecision(
		"dec-explicit-module",
		"",
		artifact.DecisionFields{
			BindingTargets: []artifact.BindingTarget{{
				Kind:       artifact.BindingTargetModule,
				ModulePath: "internal/cli",
			}},
		},
	)
	createDecision(
		"dec-unmatched-footprint",
		"internal/cli/b.go",
		artifact.DecisionFields{
			ImplementationFootprint: artifact.ImplementationFootprint{
				Files: []string{"internal/cli/b.go", "internal/client/client.go"},
			},
			BindingTargets: []artifact.BindingTarget{{
				Kind:       artifact.BindingTargetSymbol,
				FilePath:   "internal/client/client.go",
				SymbolName: "Client",
			}},
		},
	)
	createDecision(
		"dec-footprint",
		"internal/cli/b.go",
		artifact.DecisionFields{
			ImplementationFootprint: artifact.ImplementationFootprint{
				Files: []string{"internal/cli/b.go"},
			},
		},
	)
	createDecision(
		"dec-exact",
		"internal/cli/exact.go",
		artifact.DecisionFields{
			GovernanceMode: artifact.GovernanceModeExact,
		},
	)
	createDecision(
		"dec-nested",
		"internal/cli/sub/nested.go",
		artifact.DecisionFields{},
	)
	createDecision(
		"dec-nonmodule-directory",
		"internal/cli/generated",
		artifact.DecisionFields{},
	)

	decisions, err := decisionsForChangedPath(
		ctx,
		store,
		graph.NewStore(store.DB()),
		"internal/cli/b.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	actual := make(map[string]bool)
	for _, decision := range decisions {
		actual[decision.Meta.ID] = true
	}
	for _, expected := range []string{
		"dec-binding",
		"dec-explicit-module",
		"dec-module",
		"dec-module-root",
	} {
		if !actual[expected] {
			t.Fatalf("%s missing from Overseer scope: %v", expected, actual)
		}
	}
	for _, excluded := range []string{
		"dec-unmatched-footprint",
		"dec-footprint",
		"dec-exact",
		"dec-nested",
		"dec-nonmodule-directory",
	} {
		if actual[excluded] {
			t.Fatalf("%s leaked into Overseer scope: %v", excluded, actual)
		}
	}
}
