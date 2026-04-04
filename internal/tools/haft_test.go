package tools

import (
	"context"
	"database/sql"
	"fmt"
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

func completeDecisionArgs(args map[string]any) map[string]any {
	complete := make(map[string]any, len(args)+5)
	for key, value := range args {
		complete[key] = value
	}

	if _, ok := complete["selection_policy"]; !ok {
		complete["selection_policy"] = "Prefer the option that best satisfies the active acceptance criteria with the least avoidable complexity."
	}
	if _, ok := complete["counterargument"]; !ok {
		complete["counterargument"] = "The chosen option could fail if the simplifying assumptions behind the current comparison do not survive production traffic."
	}
	if _, ok := complete["weakest_link"]; !ok {
		complete["weakest_link"] = "Operational confidence still depends on limited production-grade evidence."
	}
	if _, ok := complete["why_not_others"]; !ok {
		complete["why_not_others"] = []map[string]any{{
			"variant": "Fallback alternative",
			"reason":  "It carries more cost or complexity without enough compensating value for the current scope.",
		}}
	}
	if _, ok := complete["rollback"]; !ok {
		complete["rollback"] = map[string]any{
			"triggers": []string{"Primary acceptance check regresses after rollout"},
		}
	}

	return complete
}

type decisionToolFixture struct {
	ctx               context.Context
	store             *artifact.Store
	haftDir           string
	problem           *artifact.Artifact
	comparedPortfolio *artifact.Artifact
	otherPortfolio    *artifact.Artifact
	activeCycle       *agent.Cycle
	registry          *Registry
	tool              *HaftDecisionTool
}

func setupDecisionToolFixture(t *testing.T) decisionToolFixture {
	t.Helper()

	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Latency variance between protocols",
		Acceptance: "Choose the transport with the best latency trade-off",
	})
	if err != nil {
		t.Fatal(err)
	}

	comparedPortfolio, _, err := artifact.ExploreSolutions(ctx, store, haftDir, artifact.ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []artifact.Variant{
			{
				ID:            "V1",
				Title:         "REST",
				WeakestLink:   "chatty payloads",
				NoveltyMarker: "Keep the existing request-response semantics",
			},
			{
				ID:            "V2",
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
		PortfolioRef: comparedPortfolio.Meta.ID,
		Results: artifact.ComparisonResult{
			Dimensions: []string{"latency", "cost"},
			Scores: map[string]map[string]string{
				"V1": {"latency": "42ms", "cost": "$120"},
				"V2": {"latency": "18ms", "cost": "$180"},
			},
			NonDominatedSet: []string{"V1", "V2"},
			ParetoTradeoffs: []artifact.ParetoTradeoffNote{
				{Variant: "V1", Summary: "Lower cost, but higher latency."},
				{Variant: "V2", Summary: "Lower latency, but higher cost."},
			},
			SelectedRef:   "V2",
			PolicyApplied: "Minimize latency within budget.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	otherPortfolio, _, err := artifact.ExploreSolutions(ctx, store, haftDir, artifact.ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []artifact.Variant{
			{
				ID:            "X1",
				Title:         "WebSocket",
				WeakestLink:   "connection lifecycle complexity",
				NoveltyMarker: "Keep long-lived duplex sessions",
			},
			{
				ID:            "X2",
				Title:         "SSE",
				WeakestLink:   "server-to-client only",
				NoveltyMarker: "Use unidirectional event streams",
			},
		},
		NoSteppingStoneRationale: "Both alternatives are transport candidates outside the compared portfolio.",
	})
	if err != nil {
		t.Fatal(err)
	}

	activeCycle := &agent.Cycle{
		ID:                   "cyc-active",
		Status:               agent.CycleActive,
		Phase:                agent.PhaseDecider,
		ProblemRef:           problem.Meta.ID,
		PortfolioRef:         comparedPortfolio.Meta.ID,
		ComparedPortfolioRef: comparedPortfolio.Meta.ID,
		SelectedPortfolioRef: comparedPortfolio.Meta.ID,
		SelectedVariantRef:   "V2",
	}

	registry := &Registry{}
	registry.SetCycleResolver(func(context.Context) *agent.Cycle {
		return activeCycle
	})

	tool := NewHaftDecisionTool(store, haftDir, t.TempDir(), registry)

	return decisionToolFixture{
		ctx:               ctx,
		store:             store,
		haftDir:           haftDir,
		problem:           problem,
		comparedPortfolio: comparedPortfolio,
		otherPortfolio:    otherPortfolio,
		activeCycle:       activeCycle,
		registry:          registry,
		tool:              tool,
	}
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

func TestHaftSolutionTool_ExploreDefaultsMissingProblemRefToActiveCycle(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	tool := NewHaftSolutionTool(fixture.store, fixture.haftDir, fixture.registry)

	result, err := tool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action": "explore",
		"variants": []map[string]any{
			{
				"title":          "Batch worker",
				"weakest_link":   "queue drain latency",
				"novelty_marker": "Move the heavy work into periodic batches",
			},
			{
				"title":          "Streaming worker",
				"weakest_link":   "steady-state operational cost",
				"novelty_marker": "Process each task as a continuously running stream",
			},
		},
		"no_stepping_stone_rationale": "Both worker layouts are intended as direct end-state candidates.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta == nil {
		t.Fatal("expected artifact metadata")
	}

	portfolio, err := fixture.store.Get(fixture.ctx, result.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}
	if !hasArtifactLink(portfolio.Meta.Links, fixture.problem.Meta.ID, "based_on") {
		t.Fatalf("expected explored portfolio to link to active problem %q, links=%v", fixture.problem.Meta.ID, portfolio.Meta.Links)
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
		"pareto_tradeoffs": []map[string]any{
			{"variant": "REST", "summary": "Lower cost, but slower latency."},
			{"variant": "gRPC", "summary": "Lowest latency, but higher cost."},
		},
		"selected_ref":             "gRPC",
		"recommendation_rationale": "Latency is the decisive dimension within the accepted budget.",
		"policy_applied":           "Minimize latency within budget.",
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
	if !strings.Contains(compareResult.DisplayText, "Recommendation (advisory): gRPC") {
		t.Fatalf("expected advisory recommendation in compare display, got %q", compareResult.DisplayText)
	}
	if !strings.Contains(compareResult.DisplayText, "Pareto-front trade-offs:") {
		t.Fatalf("expected compare display to include Pareto trade-offs, got %q", compareResult.DisplayText)
	}
	if !strings.Contains(compareResult.DisplayText, "Recommendation rationale: Latency is the decisive dimension within the accepted budget.") {
		t.Fatalf("expected compare display to include recommendation rationale, got %q", compareResult.DisplayText)
	}
	if strings.Contains(compareResult.DisplayText, "Selected:") {
		t.Fatalf("compare display still presents recommendation as selection: %q", compareResult.DisplayText)
	}

	portfolio, err := store.Get(ctx, compareResult.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}

	fields := portfolio.UnmarshalPortfolioFields()
	if fields.Comparison == nil || fields.Comparison.ParityPlan == nil {
		t.Fatal("expected structured parity plan to round-trip through compare tool")
	}
	if len(fields.Comparison.ParetoTradeoffs) != 2 {
		t.Fatalf("expected persisted Pareto trade-offs, got %+v", fields.Comparison.ParetoTradeoffs)
	}
	if got := fields.Comparison.RecommendationRationale; got != "Latency is the decisive dimension within the accepted budget." {
		t.Fatalf("unexpected recommendation rationale: %q", got)
	}
	if got := fields.Comparison.ParityPlan.Window; got != "same 15m replay window" {
		t.Fatalf("parity window = %q", got)
	}
}

func TestHaftSolutionTool_CompareDefaultsMissingPortfolioRefToActiveCycle(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	tool := NewHaftSolutionTool(fixture.store, fixture.haftDir, fixture.registry)

	result, err := tool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action":     "compare",
		"dimensions": []string{"latency", "cost"},
		"scores": map[string]map[string]string{
			"V1": {"latency": "42ms", "cost": "$120"},
			"V2": {"latency": "18ms", "cost": "$180"},
		},
		"non_dominated_set": []string{"V1", "V2"},
		"pareto_tradeoffs": []map[string]any{
			{"variant": "V1", "summary": "Lower cost, but slower latency."},
			{"variant": "V2", "summary": "Lower latency, but higher cost."},
		},
		"selected_ref":   "V2",
		"policy_applied": "Minimize latency within budget.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta == nil {
		t.Fatal("expected comparison artifact metadata")
	}
	if result.Meta.ComparedPortfolioRef != fixture.comparedPortfolio.Meta.ID {
		t.Fatalf("ComparedPortfolioRef = %q, want %q", result.Meta.ComparedPortfolioRef, fixture.comparedPortfolio.Meta.ID)
	}
}

func TestHaftSolutionTool_CompareRejectsForeignPortfolioRef(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	tool := NewHaftSolutionTool(fixture.store, fixture.haftDir, fixture.registry)

	result, err := tool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action":        "compare",
		"portfolio_ref": fixture.otherPortfolio.Meta.ID,
		"dimensions":    []string{"latency"},
		"scores": map[string]map[string]string{
			"X1": {"latency": "30ms"},
			"X2": {"latency": "25ms"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta != nil {
		t.Fatal("expected guardrail result, not comparison metadata")
	}
	if !strings.Contains(result.DisplayText, "active portfolio") {
		t.Fatalf("unexpected guardrail: %s", result.DisplayText)
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
			DominatedVariants: []artifact.DominatedVariantExplanation{
				{
					Variant:     "REST",
					DominatedBy: []string{"gRPC"},
					Summary:     "Higher latency with no compensating advantage in this comparison.",
				},
			},
			ParetoTradeoffs: []artifact.ParetoTradeoffNote{
				{Variant: "gRPC", Summary: "Lowest latency in the compared transport pair."},
			},
			SelectedRef: "gRPC",
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
			DominatedVariants: []artifact.DominatedVariantExplanation{
				{
					Variant:     "REST",
					DominatedBy: []string{"gRPC"},
					Summary:     "Higher latency with no compensating advantage in this comparison.",
				},
			},
			ParetoTradeoffs: []artifact.ParetoTradeoffNote{
				{Variant: "gRPC", Summary: "Lowest latency in the compared transport pair."},
			},
			SelectedRef: "gRPC",
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
	registry.SetDecisionBoundaryChecker(func(_ context.Context, cycle *agent.Cycle) (bool, error) {
		return cycle != nil, nil
	})

	tool := NewHaftDecisionTool(store, haftDir, t.TempDir(), registry)
	result, err := tool.Execute(ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    problem.Meta.ID,
		"portfolio_ref":  portfolio.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Persisted comparison already established the active portfolio as the best latency trade-off.",
	})))
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

func TestHaftDecisionTool_DecideFailsWhenComparedRepairCannotPersist(t *testing.T) {
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
			DominatedVariants: []artifact.DominatedVariantExplanation{
				{
					Variant:     "REST",
					DominatedBy: []string{"gRPC"},
					Summary:     "Higher latency with no compensating advantage in this comparison.",
				},
			},
			ParetoTradeoffs: []artifact.ParetoTradeoffNote{
				{Variant: "gRPC", Summary: "Lowest latency in the compared transport pair."},
			},
			SelectedRef: "gRPC",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	registry := &Registry{}
	registry.SetCycleResolver(func(context.Context) *agent.Cycle {
		return &agent.Cycle{
			ID:           "cyc-legacy",
			Status:       agent.CycleActive,
			ProblemRef:   problem.Meta.ID,
			PortfolioRef: portfolio.Meta.ID,
			Phase:        agent.PhaseExplorer,
		}
	})
	registry.SetCycleUpdater(func(context.Context, *agent.Cycle) error {
		return fmt.Errorf("disk full")
	})
	registry.SetDecisionBoundaryChecker(func(_ context.Context, cycle *agent.Cycle) (bool, error) {
		return cycle != nil, nil
	})

	tool := NewHaftDecisionTool(store, haftDir, t.TempDir(), registry)
	result, err := tool.Execute(ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    problem.Meta.ID,
		"portfolio_ref":  portfolio.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Persisted comparison already established the active portfolio as the best latency trade-off.",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta != nil {
		t.Fatal("expected guardrail result, not a decision artifact")
	}
	if !strings.Contains(result.DisplayText, "could not persist the compared-portfolio repair") {
		t.Fatalf("unexpected guardrail: %s", result.DisplayText)
	}
}

func TestHaftDecisionTool_DecideRequiresSelectedVariantToMatchUserChoice(t *testing.T) {
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
			DominatedVariants: []artifact.DominatedVariantExplanation{
				{
					Variant:     "REST",
					DominatedBy: []string{"gRPC"},
					Summary:     "Higher latency with no compensating advantage in this comparison.",
				},
			},
			ParetoTradeoffs: []artifact.ParetoTradeoffNote{
				{Variant: "gRPC", Summary: "Lowest latency in the compared transport pair."},
			},
			SelectedRef: "gRPC",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	registry := &Registry{}
	registry.SetCycleResolver(func(context.Context) *agent.Cycle {
		return &agent.Cycle{
			ID:                   "cyc-select",
			Status:               agent.CycleActive,
			ProblemRef:           problem.Meta.ID,
			PortfolioRef:         portfolio.Meta.ID,
			ComparedPortfolioRef: portfolio.Meta.ID,
			SelectedPortfolioRef: portfolio.Meta.ID,
			SelectedVariantRef:   "V2",
			Phase:                agent.PhaseDecider,
		}
	})
	registry.SetDecisionBoundaryChecker(func(_ context.Context, cycle *agent.Cycle) (bool, error) {
		return agent.HasDecisionSelection(cycle), nil
	})

	tool := NewHaftDecisionTool(store, haftDir, t.TempDir(), registry)
	result, err := tool.Execute(ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    problem.Meta.ID,
		"portfolio_ref":  portfolio.Meta.ID,
		"selected_title": "REST",
		"why_selected":   "Pretend the agent ignored the user's chosen variant.",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta != nil {
		t.Fatal("expected guardrail result, not a decision artifact")
	}
	if !strings.Contains(result.DisplayText, "does not match the human-selected variant") {
		t.Fatalf("unexpected guardrail: %s", result.DisplayText)
	}
}

func TestHaftDecisionTool_DecideRejectsMismatchedPortfolioRef(t *testing.T) {
	fixture := setupDecisionToolFixture(t)

	result, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"portfolio_ref":  fixture.otherPortfolio.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Latency wins inside the compared portfolio.",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta != nil {
		t.Fatal("expected guardrail result, not a decision artifact")
	}
	if !strings.Contains(result.DisplayText, "active compared portfolio") {
		t.Fatalf("unexpected guardrail: %s", result.DisplayText)
	}
}

func TestHaftDecisionTool_DecideDefaultsMissingPortfolioRefToActiveCycle(t *testing.T) {
	fixture := setupDecisionToolFixture(t)

	result, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Latency wins inside the compared portfolio.",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta == nil {
		t.Fatal("expected decision artifact metadata")
	}

	decision, err := fixture.store.Get(fixture.ctx, result.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}
	if !hasArtifactLink(decision.Meta.Links, fixture.comparedPortfolio.Meta.ID, "based_on") {
		t.Fatalf("expected decision to link to active portfolio %q, links=%v", fixture.comparedPortfolio.Meta.ID, decision.Meta.Links)
	}

	fields := decision.UnmarshalDecisionFields()
	if fields.SelectionPolicy == "" {
		t.Fatal("expected selection_policy in structured data")
	}
	if fields.CounterArgument == "" {
		t.Fatal("expected counterargument in structured data")
	}
	if len(fields.WhyNotOthers) == 0 {
		t.Fatal("expected rejected alternatives in structured data")
	}
	if len(fields.RollbackTriggers) == 0 {
		t.Fatal("expected rollback triggers in structured data")
	}
}

func TestHaftDecisionTool_MeasureRejectsForeignDecisionRef(t *testing.T) {
	fixture := setupDecisionToolFixture(t)

	activeDecision, _, err := artifact.Decide(fixture.ctx, fixture.store, fixture.haftDir, artifact.DecideInput{
		ProblemRef:      fixture.problem.Meta.ID,
		PortfolioRef:    fixture.comparedPortfolio.Meta.ID,
		SelectedTitle:   "gRPC",
		WhySelected:     "Latency wins inside the active compared portfolio.",
		SelectionPolicy: "Minimize latency within budget.",
		CounterArgument: "Operational cost could outweigh the latency benefit.",
		WeakestLink:     "Production evidence is still limited.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "REST", Reason: "Higher latency with no compensating advantage for the current scope."}},
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"Latency regressions after rollout"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	foreignDecision, _, err := artifact.Decide(fixture.ctx, fixture.store, fixture.haftDir, artifact.DecideInput{
		ProblemRef:      fixture.problem.Meta.ID,
		PortfolioRef:    fixture.otherPortfolio.Meta.ID,
		SelectedTitle:   "WebSocket",
		WhySelected:     "The foreign portfolio keeps duplex connections available.",
		SelectionPolicy: "Prioritize persistent duplex transport.",
		CounterArgument: "Connection lifecycle complexity may be unjustified for the current workload.",
		WeakestLink:     "Stateful connections raise operational complexity.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "SSE", Reason: "It cannot satisfy the duplex requirement."}},
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"Connection churn exceeds the operating budget"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	fixture.activeCycle.DecisionRef = activeDecision.Meta.ID

	result, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action":       "measure",
		"decision_ref": foreignDecision.Meta.ID,
		"findings":     "Measured the wrong decision on purpose.",
		"verdict":      "accepted",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta != nil {
		t.Fatal("expected guardrail result, not measurement metadata")
	}
	if !strings.Contains(result.DisplayText, "active decision") {
		t.Fatalf("unexpected guardrail: %s", result.DisplayText)
	}
}

func TestHaftDecisionTool_DecideAcceptsLegacyComparedPortfolioSelection(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Need to keep a legacy compared portfolio working",
		Acceptance: "Respect the user's selected variant from a legacy compared portfolio",
	})
	if err != nil {
		t.Fatal(err)
	}

	legacyPortfolio := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:      "sol-legacy-compared",
			Kind:    artifact.KindSolutionPortfolio,
			Title:   "Legacy compared portfolio",
			Context: "transport",
			Mode:    artifact.ModeStandard,
		},
		Body: `# Legacy compared portfolio

## Variants (2)

### V1. REST

**Weakest link:** chatty payloads

### V2. gRPC

**Weakest link:** tooling overhead

## Comparison

**Pareto front:** gRPC
`,
		StructuredData: `{}`,
	}
	if err := store.Create(ctx, legacyPortfolio); err != nil {
		t.Fatal(err)
	}

	activeCycle := &agent.Cycle{
		ID:                   "cyc-legacy-selection",
		Status:               agent.CycleActive,
		ProblemRef:           problem.Meta.ID,
		PortfolioRef:         legacyPortfolio.Meta.ID,
		ComparedPortfolioRef: legacyPortfolio.Meta.ID,
		SelectedPortfolioRef: legacyPortfolio.Meta.ID,
		SelectedVariantRef:   "V2",
		Phase:                agent.PhaseDecider,
	}

	registry := &Registry{}
	registry.SetCycleResolver(func(context.Context) *agent.Cycle {
		return activeCycle
	})
	registry.SetDecisionBoundaryChecker(func(_ context.Context, cycle *agent.Cycle) (bool, error) {
		return agent.HasDecisionSelection(cycle), nil
	})

	tool := NewHaftDecisionTool(store, haftDir, t.TempDir(), registry)
	result, err := tool.Execute(ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    problem.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "The human already chose the legacy gRPC variant after compare.",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta == nil {
		t.Fatalf("expected decision artifact metadata, got guardrail: %s", result.DisplayText)
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
	result, err := tool.Execute(ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"selected_title": "Introduce API gateway",
		"why_selected":   "Need a consistent ingress boundary",
		"affected_files": []string{"internal/api/router.go"},
	})))
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

func hasArtifactLink(links []artifact.Link, ref, linkType string) bool {
	for _, link := range links {
		if link.Ref == ref && link.Type == linkType {
			return true
		}
	}

	return false
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
	_, err = tool.Execute(ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"selected_title": "Existing API gateway",
		"why_selected":   "The module already has a boundary decision",
		"affected_files": []string{"internal/api/router.go"},
	})))
	if err != nil {
		t.Fatal(err)
	}

	result, err := tool.Execute(ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"selected_title": "Follow-up API change",
		"why_selected":   "Need to refine the existing ingress decision",
		"affected_files": []string{"internal/api/server.go"},
	})))
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

func TestHaftDecisionTool_SchemaIncludesEvidenceAction(t *testing.T) {
	tool := NewHaftDecisionTool(setupHaftToolStore(t), t.TempDir(), t.TempDir(), nil)
	schema := tool.Schema()

	properties, ok := schema.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %T, want map[string]any", schema.Parameters["properties"])
	}

	actionProp, ok := properties["action"].(map[string]any)
	if !ok {
		t.Fatalf("action property = %T, want map[string]any", properties["action"])
	}

	enum, ok := actionProp["enum"].([]string)
	if !ok {
		t.Fatalf("action enum = %T, want []string", actionProp["enum"])
	}

	foundEvidence := false
	for _, action := range enum {
		if action == "evidence" {
			foundEvidence = true
			break
		}
	}
	if !foundEvidence {
		t.Fatalf("action enum %v does not include evidence", enum)
	}

	for _, key := range []string{
		"artifact_ref",
		"evidence_content",
		"evidence_type",
		"evidence_verdict",
		"carrier_ref",
		"congruence_level",
	} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("schema missing %q evidence field", key)
		}
	}
}

func TestHaftDecisionTool_SchemaIncludesAntiSelfDeceptionFields(t *testing.T) {
	tool := NewHaftDecisionTool(setupHaftToolStore(t), t.TempDir(), t.TempDir(), nil)
	schema := tool.Schema()

	properties, ok := schema.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %T, want map[string]any", schema.Parameters["properties"])
	}

	for _, key := range []string{
		"selection_policy",
		"counterargument",
		"why_not_others",
		"rollback",
		"weakest_link",
	} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("schema missing %q", key)
		}
	}
}

func TestHaftDecisionTool_DecideRejectsIncompleteAntiSelfDeceptionRecord(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	tool := NewHaftDecisionTool(store, t.TempDir(), t.TempDir(), nil)

	_, err := tool.Execute(ctx, mustJSON(t, map[string]any{
		"action":         "decide",
		"selected_title": "Introduce API gateway",
		"why_selected":   "Need a consistent ingress boundary",
	}))
	if err == nil {
		t.Fatal("expected validation error for incomplete decision record")
	}

	required := []string{
		"selection_policy is required",
		"counterargument is required",
		"weakest_link is required",
		"why_not_others is required",
		"rollback.triggers is required",
	}

	for _, want := range required {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing validation message %q in %q", want, err.Error())
		}
	}
}

func TestHaftDecisionTool_EvidenceAttachesToDecision(t *testing.T) {

	fixture := setupDecisionToolFixture(t)

	decisionResult, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Latency wins inside the compared portfolio.",
	})))
	if err != nil {
		t.Fatal(err)
	}

	decisionRef := decisionResult.Meta.ArtifactRef
	result, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action":           "evidence",
		"artifact_ref":     decisionRef,
		"evidence_content": "Load test on staging sustained 18ms p95 under the expected request mix.",
		"evidence_type":    "benchmark",
		"evidence_verdict": "supports",
		"carrier_ref":      "reports/loadtest.txt",
		"congruence_level": 2,
	}))
	if err != nil {
		t.Fatal(err)
	}

	if result.Meta == nil {
		t.Fatal("expected evidence metadata")
	}
	if result.Meta.Kind != "evidence" {
		t.Fatalf("meta kind = %q, want evidence", result.Meta.Kind)
	}
	if result.Meta.ArtifactRef != decisionRef {
		t.Fatalf("meta artifact ref = %q, want %q", result.Meta.ArtifactRef, decisionRef)
	}
	if result.Meta.Operation != "evidence" {
		t.Fatalf("meta operation = %q, want evidence", result.Meta.Operation)
	}
	if !strings.Contains(result.DisplayText, "Evidence attached:") {
		t.Fatalf("unexpected display text: %s", result.DisplayText)
	}
	if !strings.Contains(result.DisplayText, "WLNK:") {
		t.Fatalf("expected WLNK summary in display text: %s", result.DisplayText)
	}

	items, err := fixture.store.GetEvidenceItems(fixture.ctx, decisionRef)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(items))
	}

	item := items[0]
	if item.Type != "benchmark" {
		t.Fatalf("evidence type = %q, want benchmark", item.Type)
	}
	if item.Verdict != "supports" {
		t.Fatalf("evidence verdict = %q, want supports", item.Verdict)
	}
	if item.CarrierRef != "reports/loadtest.txt" {
		t.Fatalf("carrier ref = %q, want reports/loadtest.txt", item.CarrierRef)
	}
	if item.CongruenceLevel != 2 {
		t.Fatalf("congruence level = %d, want 2", item.CongruenceLevel)
	}
}
