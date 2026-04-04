package tools

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/fpf"

	_ "modernc.org/sqlite"
)

func setupHaftToolStore(t *testing.T) *artifact.Store {
	t.Helper()

	db, err := sql.Open("sqlite", t.TempDir()+"/tools.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	stmts := []string{
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY, kind TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active', context TEXT, mode TEXT,
			title TEXT NOT NULL, content TEXT NOT NULL, file_path TEXT,
			valid_until TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			search_keywords TEXT DEFAULT '', structured_data TEXT DEFAULT '')`,
		`CREATE TABLE artifact_links (
			source_id TEXT NOT NULL, target_id TEXT NOT NULL, link_type TEXT NOT NULL,
			created_at TEXT NOT NULL, PRIMARY KEY (source_id, target_id, link_type))`,
		`CREATE TABLE evidence_items (
			id TEXT PRIMARY KEY, artifact_ref TEXT NOT NULL, type TEXT NOT NULL,
			content TEXT NOT NULL, verdict TEXT, carrier_ref TEXT,
			congruence_level INTEGER DEFAULT 3, formality_level INTEGER DEFAULT 5,
			claim_scope TEXT DEFAULT '[]', valid_until TEXT, created_at TEXT NOT NULL)`,
		`CREATE TABLE affected_files (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL, file_hash TEXT,
			PRIMARY KEY (artifact_id, file_path))`,
		`CREATE TABLE codebase_modules (
			module_id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL, lang TEXT, file_count INTEGER DEFAULT 0,
			last_scanned TEXT NOT NULL)`,
		`CREATE TABLE module_dependencies (
			source_module TEXT NOT NULL, target_module TEXT NOT NULL,
			dep_type TEXT NOT NULL DEFAULT 'import', file_path TEXT,
			last_scanned TEXT NOT NULL,
			PRIMARY KEY (source_module, target_module, dep_type))`,
		`CREATE VIRTUAL TABLE artifacts_fts USING fts5(id, title, content, kind, search_keywords, tokenize='porter unicode61')`,
		`CREATE TRIGGER artifacts_fts_insert AFTER INSERT ON artifacts BEGIN
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords) VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_update AFTER UPDATE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords) VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_delete AFTER DELETE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
		END`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup: %v\nSQL: %s", err, stmt)
		}
	}

	return artifact.NewStore(db)
}

func TestHaftQueryTool_FPFUsesInjectedSearch(t *testing.T) {
	store := setupHaftToolStore(t)
	tool := NewHaftQueryTool(store, func(request FPFSearchRequest) (string, error) {
		if request.Query != "A.6" {
			t.Fatalf("unexpected query %q", request.Query)
		}
		if request.Limit != fpf.DefaultSpecSearchLimit {
			t.Fatalf("unexpected limit %d", request.Limit)
		}
		if request.Full {
			t.Fatal("expected full=false by default")
		}
		if request.Explain {
			t.Fatal("expected explain=false by default")
		}
		return "### A.6 — Signature Stack & Boundary Discipline\ntier: pattern · exact pattern id\n", nil
	})

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"action": "fpf",
		"query":  "A.6",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.DisplayText, "A.6") {
		t.Fatalf("unexpected result: %s", result.DisplayText)
	}
}

func TestHaftQueryTool_FPFPassesOptionalSearchControls(t *testing.T) {
	store := setupHaftToolStore(t)
	tool := NewHaftQueryTool(store, func(request FPFSearchRequest) (string, error) {
		if request.Query != "boundary routing" {
			t.Fatalf("unexpected query %q", request.Query)
		}
		if request.Limit != 3 {
			t.Fatalf("unexpected limit %d", request.Limit)
		}
		if !request.Full {
			t.Fatal("expected full=true")
		}
		if !request.Explain {
			t.Fatal("expected explain=true")
		}
		return "### A.6 — Signature Stack & Boundary Discipline\ntier: route · Boundary discipline and routing\n", nil
	})

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"action":  "fpf",
		"query":   "boundary routing",
		"limit":   3,
		"full":    true,
		"explain": true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.DisplayText, "Boundary discipline and routing") {
		t.Fatalf("unexpected result: %s", result.DisplayText)
	}
}

func TestHaftSolutionTool_ExploreAcceptsNoSteppingStoneRationale(t *testing.T) {
	store := setupHaftToolStore(t)
	tool := NewHaftSolutionTool(store, t.TempDir(), nil)

	args := mustJSON(t, map[string]any{
		"action": "explore",
		"variants": []map[string]any{
			{
				"title":          "In-process queue",
				"weakest_link":   "shared process failure domain",
				"novelty_marker": "Keep execution inside the current service boundary",
			},
			{
				"title":          "Dedicated worker",
				"weakest_link":   "deployment complexity",
				"novelty_marker": "Split execution into a separately deployable worker plane",
				"diversity_role": "operational isolation",
			},
		},
		"no_stepping_stone_rationale": "Both variants are intended as direct end-state candidates.",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta == nil {
		t.Fatal("expected artifact metadata")
	}

	portfolio, err := store.Get(context.Background(), result.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}

	fields := portfolio.UnmarshalPortfolioFields()
	if fields.NoSteppingStoneRationale == "" {
		t.Fatal("expected no_stepping_stone_rationale to be stored")
	}
	if fields.Variants[0].NoveltyMarker == "" {
		t.Fatal("expected novelty_marker to round-trip through tool execution")
	}
}

func TestHaftSolutionTool_CompareAcceptsStructuredParityPlanInDeepMode(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problemTool := NewHaftProblemTool(store, haftDir)
	solutionTool := NewHaftSolutionTool(store, haftDir, nil)

	frameResult, err := problemTool.Execute(ctx, mustJSON(t, map[string]any{
		"action": "frame",
		"title":  "Transport choice",
		"signal": "Latency variance between protocols",
		"mode":   "deep",
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = problemTool.Execute(ctx, mustJSON(t, map[string]any{
		"action":      "characterize",
		"problem_ref": frameResult.Meta.ArtifactRef,
		"dimensions": []map[string]any{
			{"name": "latency", "role": "target", "valid_until": "2027-01-01"},
			{"name": "cost", "role": "constraint"},
		},
		"parity_plan": map[string]any{
			"baseline_set":        []string{"REST", "gRPC"},
			"window":              "same 15m replay window",
			"budget":              "$200/month",
			"missing_data_policy": artifact.MissingDataPolicyExplicitAbstain,
			"normalization":       []map[string]any{{"dimension": "latency", "method": "p99"}},
			"pinned_conditions":   []string{"Same dataset", "Same region"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	problem, err := store.Get(ctx, frameResult.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}
	charFields := problem.UnmarshalProblemFields()
	if len(charFields.Characterizations) != 1 || charFields.Characterizations[0].ParityPlan == nil {
		t.Fatal("expected characterize tool to persist structured parity plan")
	}

	exploreResult, err := solutionTool.Execute(ctx, mustJSON(t, map[string]any{
		"action":      "explore",
		"problem_ref": frameResult.Meta.ArtifactRef,
		"variants": []map[string]any{
			{
				"title":          "REST",
				"weakest_link":   "chatty payloads",
				"novelty_marker": "Keep the existing request-response semantics",
			},
			{
				"title":          "gRPC",
				"weakest_link":   "tooling overhead",
				"novelty_marker": "Adopt binary RPC with generated clients",
			},
		},
		"no_stepping_stone_rationale": "Both transports are evaluated as direct target architectures.",
	}))
	if err != nil {
		t.Fatal(err)
	}

	compareResult, err := solutionTool.Execute(ctx, mustJSON(t, map[string]any{
		"action":        "compare",
		"portfolio_ref": exploreResult.Meta.ArtifactRef,
		"dimensions":    []string{"latency", "cost"},
		"scores": map[string]map[string]string{
			"REST": {"latency": "42ms", "cost": "$120"},
			"gRPC": {"latency": "18ms", "cost": "$180"},
		},
		"non_dominated_set": []string{"REST", "gRPC"},
		"selected_ref":      "gRPC",
		"policy_applied":    "Minimize latency within budget.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if compareResult.Meta == nil {
		t.Fatal("expected comparison artifact metadata")
	}
	if compareResult.Meta.ComparedPortfolioRef != exploreResult.Meta.ArtifactRef {
		t.Fatalf("ComparedPortfolioRef = %q, want %q", compareResult.Meta.ComparedPortfolioRef, exploreResult.Meta.ArtifactRef)
	}

	portfolio, err := store.Get(ctx, compareResult.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}

	fields := portfolio.UnmarshalPortfolioFields()
	if fields.Comparison == nil || fields.Comparison.ParityPlan == nil {
		t.Fatal("expected structured parity plan to round-trip through compare tool")
	}
	if got := fields.Comparison.ParityPlan.Window; got != "same 15m replay window" {
		t.Fatalf("parity window = %q", got)
	}
}

func TestResolveComparedPortfolioRef_RequiresPersistedComparison(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:  "Transport choice",
		Signal: "Latency variance between protocols",
	})
	if err != nil {
		t.Fatal(err)
	}

	portfolio, _, err := artifact.ExploreSolutions(ctx, store, haftDir, artifact.ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []artifact.Variant{
			{
				Title:         "REST",
				WeakestLink:   "chatty payloads",
				NoveltyMarker: "Keep the existing request-response semantics",
			},
			{
				Title:         "gRPC",
				WeakestLink:   "tooling overhead",
				NoveltyMarker: "Adopt binary RPC with generated clients",
			},
		},
		NoSteppingStoneRationale: "Both transports are direct target architectures.",
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := resolveComparedPortfolioRef(ctx, store, portfolio.Meta.ID); got != "" {
		t.Fatalf("resolveComparedPortfolioRef = %q, want empty before compare", got)
	}

	_, _, err = artifact.CompareSolutions(ctx, store, haftDir, artifact.CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: artifact.ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"REST": {"latency": "42ms"},
				"gRPC": {"latency": "18ms"},
			},
			NonDominatedSet: []string{"gRPC"},
			SelectedRef:     "gRPC",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := resolveComparedPortfolioRef(ctx, store, portfolio.Meta.ID); got != portfolio.Meta.ID {
		t.Fatalf("resolveComparedPortfolioRef = %q, want %q after compare", got, portfolio.Meta.ID)
	}
}

func TestHaftDecisionTool_DecideRepairsLegacyComparedPortfolioRef(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:  "Transport choice",
		Signal: "Latency variance between protocols",
	})
	if err != nil {
		t.Fatal(err)
	}

	portfolio, _, err := artifact.ExploreSolutions(ctx, store, haftDir, artifact.ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []artifact.Variant{
			{
				Title:         "REST",
				WeakestLink:   "chatty payloads",
				NoveltyMarker: "Keep the existing request-response semantics",
			},
			{
				Title:         "gRPC",
				WeakestLink:   "tooling overhead",
				NoveltyMarker: "Adopt binary RPC with generated clients",
			},
		},
		NoSteppingStoneRationale: "Both transports are direct target architectures.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = artifact.CompareSolutions(ctx, store, haftDir, artifact.CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: artifact.ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"REST": {"latency": "42ms"},
				"gRPC": {"latency": "18ms"},
			},
			NonDominatedSet: []string{"gRPC"},
			SelectedRef:     "gRPC",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	activeCycle := &agent.Cycle{
		ID:           "cyc-legacy",
		Status:       agent.CycleActive,
		ProblemRef:   problem.Meta.ID,
		PortfolioRef: portfolio.Meta.ID,
		Phase:        agent.PhaseExplorer,
	}

	var persisted *agent.Cycle
	registry := &Registry{}
	registry.SetCycleResolver(func(context.Context) *agent.Cycle {
		return activeCycle
	})
	registry.SetCycleUpdater(func(_ context.Context, repaired *agent.Cycle) error {
		copy := *repaired
		persisted = &copy
		activeCycle = &copy
		return nil
	})
	registry.SetConsentChecker(func(context.Context) bool {
		return true
	})

	tool := NewHaftDecisionTool(store, haftDir, t.TempDir(), registry)
	result, err := tool.Execute(ctx, mustJSON(t, map[string]any{
		"action":         "decide",
		"problem_ref":    problem.Meta.ID,
		"portfolio_ref":  portfolio.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Persisted comparison already established the active portfolio as the best latency trade-off.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta == nil {
		t.Fatal("expected decision artifact metadata")
	}
	if persisted == nil {
		t.Fatal("expected repaired cycle to be persisted")
	}
	if persisted.ComparedPortfolioRef != portfolio.Meta.ID {
		t.Fatalf("ComparedPortfolioRef = %q, want %q", persisted.ComparedPortfolioRef, portfolio.Meta.ID)
	}
	if persisted.Phase != agent.PhaseDecider {
		t.Fatalf("Phase = %s, want %s", persisted.Phase, agent.PhaseDecider)
	}
}

func TestHaftDecisionTool_PersistsFirstModuleCoverageFlag(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, err := store.DB().ExecContext(ctx, `
		INSERT INTO codebase_modules (module_id, path, name, lang, file_count, last_scanned)
		VALUES ('mod-api', 'internal/api', 'api', 'go', 2, '2026-03-18T12:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}

	tool := NewHaftDecisionTool(store, haftDir, t.TempDir(), nil)
	result, err := tool.Execute(ctx, mustJSON(t, map[string]any{
		"action":         "decide",
		"selected_title": "Introduce API gateway",
		"why_selected":   "Need a consistent ingress boundary",
		"affected_files": []string{"internal/api/router.go"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.DisplayText, "First decision governing module") {
		t.Fatalf("expected first-module coverage warning, got: %s", result.DisplayText)
	}

	decision, err := store.Get(ctx, result.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if !fields.FirstModuleCoverage {
		t.Fatal("expected first_module_coverage flag to be persisted")
	}
}

func TestHaftDecisionTool_SuppressesFirstModuleCoverageWarningWhenGoverned(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, err := store.DB().ExecContext(ctx, `
		INSERT INTO codebase_modules (module_id, path, name, lang, file_count, last_scanned)
		VALUES ('mod-api', 'internal/api', 'api', 'go', 2, '2026-03-18T12:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}

	tool := NewHaftDecisionTool(store, haftDir, t.TempDir(), nil)
	_, err = tool.Execute(ctx, mustJSON(t, map[string]any{
		"action":         "decide",
		"selected_title": "Existing API gateway",
		"why_selected":   "The module already has a boundary decision",
		"affected_files": []string{"internal/api/router.go"},
	}))
	if err != nil {
		t.Fatal(err)
	}

	result, err := tool.Execute(ctx, mustJSON(t, map[string]any{
		"action":         "decide",
		"selected_title": "Follow-up API change",
		"why_selected":   "Need to refine the existing ingress decision",
		"affected_files": []string{"internal/api/server.go"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.DisplayText, "First decision governing module") {
		t.Fatalf("expected follow-up decision to skip first-module warning, got: %s", result.DisplayText)
	}

	decision, err := store.Get(ctx, result.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if fields.FirstModuleCoverage {
		t.Fatal("expected first_module_coverage to remain false for governed module")
	}
}
