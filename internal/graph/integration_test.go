package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
)

type projectGraphFixture struct {
	db            *sql.DB
	artifactStore *artifact.Store
	graphStore    *Store
}

type seededDecision struct {
	id         string
	title      string
	invariants []string
	files      []string
}

func setupProjectGraphFixture(t *testing.T) *projectGraphFixture {
	t.Helper()

	projectRoot := fixtureProjectRoot(t)
	dbPath := filepath.Join(t.TempDir(), "graph-integration.db")

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	rawDB := database.GetRawDB()
	artifactStore := artifact.NewStore(rawDB)
	graphStore := NewStore(rawDB)
	ctx := context.Background()

	modules := []Node{
		{ID: "mod-artifact", Path: "internal/artifact", Name: "artifact"},
		{ID: "mod-graph", Path: "internal/graph", Name: "graph"},
		{ID: "mod-codebase", Path: "internal/codebase", Name: "codebase"},
		{ID: "mod-reff", Path: "internal/reff", Name: "reff"},
		{ID: "mod-fpf", Path: "internal/fpf", Name: "fpf"},
		{ID: "mod-present", Path: "internal/present", Name: "present"},
		{ID: "mod-cli", Path: "internal/cli", Name: "cli"},
		{ID: "mod-tools", Path: "internal/tools", Name: "tools"},
		{ID: "mod-agentloop", Path: "internal/agentloop", Name: "agentloop"},
		{ID: "mod-desktop", Path: "desktop", Name: "desktop"},
		{ID: "mod-cmd-haft", Path: "cmd/haft", Name: "cmd-haft"},
	}

	for _, module := range modules {
		requireProjectPathExists(t, projectRoot, module.Path)
		seedIntegrationModule(t, rawDB, module.ID, module.Path, module.Name)
	}

	dependencies := [][2]string{
		{"mod-present", "mod-artifact"},
		{"mod-present", "mod-codebase"},
		{"mod-cli", "mod-artifact"},
		{"mod-cli", "mod-codebase"},
		{"mod-cli", "mod-fpf"},
		{"mod-cli", "mod-present"},
		{"mod-tools", "mod-artifact"},
		{"mod-tools", "mod-codebase"},
		{"mod-tools", "mod-present"},
		{"mod-agentloop", "mod-artifact"},
		{"mod-agentloop", "mod-codebase"},
		{"mod-desktop", "mod-artifact"},
		{"mod-desktop", "mod-graph"},
		{"mod-desktop", "mod-codebase"},
		{"mod-cmd-haft", "mod-cli"},
	}

	for _, dep := range dependencies {
		seedIntegrationDependency(t, rawDB, dep[0], dep[1])
	}

	decisions := []seededDecision{
		{
			id:    "dec-artifact-authority",
			title: "SQLite runtime authority for artifacts",
			invariants: []string{
				"SQLite is the runtime source of truth for artifacts",
				".haft projections are derived outputs, never edited directly",
			},
			files: []string{
				"internal/artifact/store.go",
				"internal/artifact/writer.go",
			},
		},
		{
			id:    "dec-artifact-decision-structure",
			title: "Decision fields stay in structured_data",
			invariants: []string{
				"DecisionRecord invariants live in structured_data",
			},
			files: []string{
				"internal/artifact/decision.go",
				"internal/artifact/types.go",
			},
		},
		{
			id:    "dec-graph-query-layer",
			title: "Knowledge graph query layer",
			invariants: []string{
				"Knowledge graph stays a query layer over existing tables",
				"File-to-module ownership uses longest-prefix matching",
			},
			files: []string{
				"internal/graph/query.go",
				"internal/graph/types.go",
			},
		},
		{
			id:    "dec-graph-impact-propagation",
			title: "Impact propagation follows dependents",
			invariants: []string{
				"Impact propagation follows transitive dependents of changed modules",
			},
			files: []string{
				"internal/graph/impact.go",
			},
		},
		{
			id:    "dec-graph-invariant-verification",
			title: "Invariant checks use live dependency data",
			invariants: []string{
				"Graph invariants are checked against the live dependency graph",
			},
			files: []string{
				"internal/graph/verify.go",
			},
		},
		{
			id:    "dec-codebase-import-graph",
			title: "Codebase scanner stores import edges",
			invariants: []string{
				"Module dependencies are stored as source imports target",
			},
			files: []string{
				"internal/codebase/walker.go",
				"internal/codebase/detector.go",
			},
		},
		{
			id:    "dec-reff-weakest-link",
			title: "Weakest-link evidence scoring",
			invariants: []string{
				"Effective reliability uses weakest-link semantics",
			},
			files: []string{
				"internal/reff/reff.go",
			},
		},
		{
			id:    "dec-cli-mcp-surface",
			title: "CLI surfaces stay above Core",
			invariants: []string{
				"CLI surfaces operate through Core stores, not direct desktop calls",
			},
			files: []string{
				"internal/cli/serve.go",
				"internal/cli/sync.go",
			},
		},
		{
			id:    "dec-tools-thin-handlers",
			title: "Tool handlers remain thin",
			invariants: []string{
				"Tool handlers stay thin over artifact and codebase services",
			},
			files: []string{
				"internal/tools/haft.go",
				"internal/tools/readfile.go",
			},
		},
		{
			id:    "dec-agentloop-core-orchestration",
			title: "Agent loop uses Core stores",
			invariants: []string{
				"Agent loop orchestration depends on Core stores, not projections",
			},
			files: []string{
				"internal/agentloop/coordinator.go",
				"internal/agentloop/overseer.go",
			},
		},
		{
			id:    "dec-present-derived-views",
			title: "Presentation reads derived Core state",
			invariants: []string{
				"Presentation models are derived from Core state only",
			},
			files: []string{
				"internal/present/board.go",
				"internal/present/projection.go",
			},
		},
		{
			id:    "dec-desktop-surface-boundary",
			title: "Desktop remains a surface",
			invariants: []string{
				"Desktop is a surface and does not become a source of truth",
			},
			files: []string{
				"internal/cli/desktop_rpc.go",
				"internal/cli/desktop_rpc_handlers.go",
			},
		},
		{
			id:    "dec-cmd-thin-entrypoint",
			title: "cmd/haft stays a thin entrypoint",
			invariants: []string{
				"cmd/haft remains a thin entrypoint over internal/cli",
			},
			files: []string{
				"cmd/haft/main.go",
			},
		},
	}

	if len(decisions) < 10 {
		t.Fatalf("fixture must seed at least 10 decisions, got %d", len(decisions))
	}

	for _, decision := range decisions {
		for _, file := range decision.files {
			requireProjectPathExists(t, projectRoot, file)
		}
		seedIntegrationDecision(t, ctx, artifactStore, decision)
	}

	return &projectGraphFixture{
		db:            rawDB,
		artifactStore: artifactStore,
		graphStore:    graphStore,
	}
}

func fixtureProjectRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func requireProjectPathExists(t *testing.T, projectRoot, relativePath string) {
	t.Helper()

	fullPath := filepath.Join(projectRoot, relativePath)
	if _, err := os.Stat(fullPath); err != nil {
		t.Fatalf("fixture path %s missing: %v", relativePath, err)
	}
}

func seedIntegrationModule(t *testing.T, rawDB *sql.DB, moduleID, modulePath, moduleName string) {
	t.Helper()

	_, err := rawDB.Exec(
		`INSERT INTO codebase_modules (module_id, path, name, lang, file_count, last_scanned) VALUES (?, ?, ?, 'go', 1, ?)`,
		moduleID,
		modulePath,
		moduleName,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed module %s: %v", moduleID, err)
	}
}

func seedIntegrationDependency(t *testing.T, rawDB *sql.DB, sourceModule, targetModule string) {
	t.Helper()

	_, err := rawDB.Exec(
		`INSERT INTO module_dependencies (source_module, target_module, dep_type, file_path, last_scanned) VALUES (?, ?, 'import', '', ?)`,
		sourceModule,
		targetModule,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed dependency %s -> %s: %v", sourceModule, targetModule, err)
	}
}

func seedIntegrationDecision(t *testing.T, ctx context.Context, artifactStore *artifact.Store, decision seededDecision) {
	t.Helper()

	structuredData, err := json.Marshal(artifact.DecisionFields{
		SelectedTitle: decision.title,
		WhySelected:   "Seeded integration fixture for knowledge graph queries.",
		Invariants:    decision.invariants,
	})
	if err != nil {
		t.Fatalf("marshal decision %s: %v", decision.id, err)
	}

	err = artifactStore.Create(ctx, &artifact.Artifact{
		Meta: artifact.Meta{
			ID:      decision.id,
			Kind:    artifact.KindDecisionRecord,
			Status:  artifact.StatusActive,
			Mode:    artifact.ModeStandard,
			Title:   decision.title,
			Context: "graph-integration",
		},
		Body:           "Seeded integration fixture.",
		StructuredData: string(structuredData),
	})
	if err != nil {
		t.Fatalf("create decision %s: %v", decision.id, err)
	}

	files := make([]artifact.AffectedFile, 0, len(decision.files))
	for _, file := range decision.files {
		files = append(files, artifact.AffectedFile{Path: file})
	}

	err = artifactStore.SetAffectedFiles(ctx, decision.id, files)
	if err != nil {
		t.Fatalf("set affected files for %s: %v", decision.id, err)
	}
}

func TestFindDecisionsForFile_SeededProjectData(t *testing.T) {
	fixture := setupProjectGraphFixture(t)
	ctx := context.Background()

	decisions, err := fixture.graphStore.FindDecisionsForFile(ctx, "internal/graph/query.go")
	if err != nil {
		t.Fatal(err)
	}

	decisionIDs := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		decisionIDs = append(decisionIDs, decision.ID)
	}

	slices.Sort(decisionIDs)

	expected := []string{
		"dec-graph-impact-propagation",
		"dec-graph-invariant-verification",
		"dec-graph-query-layer",
	}

	if !slices.Equal(decisionIDs, expected) {
		t.Fatalf("decision IDs = %v, want %v", decisionIDs, expected)
	}
}

func TestFindInvariantsForFile_SeededProjectData(t *testing.T) {
	fixture := setupProjectGraphFixture(t)
	ctx := context.Background()

	invariants, err := fixture.graphStore.FindInvariantsForFile(ctx, "internal/graph/query.go")
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(invariants))
	for _, invariant := range invariants {
		got = append(got, invariant.Text)
	}

	slices.Sort(got)

	expected := []string{
		"File-to-module ownership uses longest-prefix matching",
		"Graph invariants are checked against the live dependency graph",
		"Impact propagation follows transitive dependents of changed modules",
		"Knowledge graph stays a query layer over existing tables",
	}

	if !slices.Equal(got, expected) {
		t.Fatalf("invariants = %v, want %v", got, expected)
	}
}

func TestComputeImpactSet_SeededProjectData(t *testing.T) {
	fixture := setupProjectGraphFixture(t)
	ctx := context.Background()

	items, err := fixture.graphStore.ComputeImpactSet(ctx, "mod-artifact")
	if err != nil {
		t.Fatal(err)
	}

	impactByDecision := make(map[string]ImpactItem, len(items))
	for _, item := range items {
		impactByDecision[item.DecisionID] = item
	}

	expectedDecisionIDs := []string{
		"dec-agentloop-core-orchestration",
		"dec-artifact-authority",
		"dec-artifact-decision-structure",
		"dec-cli-mcp-surface",
		"dec-cmd-thin-entrypoint",
		"dec-desktop-surface-boundary",
		"dec-present-derived-views",
		"dec-tools-thin-handlers",
	}

	if len(impactByDecision) != len(expectedDecisionIDs) {
		t.Fatalf("impact size = %d, want %d (%v)", len(impactByDecision), len(expectedDecisionIDs), mapsKeys(impactByDecision))
	}

	for _, decisionID := range expectedDecisionIDs {
		item, ok := impactByDecision[decisionID]
		if !ok {
			t.Fatalf("missing impact item for %s in %v", decisionID, mapsKeys(impactByDecision))
		}

		if decisionID == "dec-artifact-authority" || decisionID == "dec-artifact-decision-structure" {
			if !item.IsDirect {
				t.Fatalf("%s should be direct impact", decisionID)
			}
			continue
		}

		if item.IsDirect {
			t.Fatalf("%s should be indirect impact", decisionID)
		}
	}

	cmdImpact := impactByDecision["dec-cmd-thin-entrypoint"]
	if cmdImpact.ModuleID != "mod-cmd-haft" {
		t.Fatalf("cmd impact module = %s, want mod-cmd-haft", cmdImpact.ModuleID)
	}
}

func mapsKeys(values map[string]ImpactItem) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	slices.Sort(keys)
	return keys
}
