package tools

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/present"

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
			claim_refs TEXT DEFAULT '[]',
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

func TestHaftQueryTool_FPFPassesExperimentalMode(t *testing.T) {
	store := setupHaftToolStore(t)
	tool := NewHaftQueryTool(store, func(request FPFSearchRequest) (string, error) {
		if request.Mode != fpf.SpecSearchModeTree {
			t.Fatalf("unexpected mode %q", request.Mode)
		}
		return "### A.6.B — Boundary Norm Square\ntier: drilldown · tree drill-down leaf A.6.B\n", nil
	})

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"action": "fpf",
		"query":  "boundary deontics",
		"mode":   fpf.SpecSearchModeTree,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.DisplayText, "drilldown") {
		t.Fatalf("unexpected result: %s", result.DisplayText)
	}
}

func TestHaftQueryTool_ProjectionRendersSelectedView(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	tool := NewHaftQueryTool(fixture.store, nil)

	result, err := tool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action": "projection",
		"view":   "compare",
	}))
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"## Compare/Pareto View",
		fixture.comparedPortfolio.Meta.ID,
		"Computed Pareto front:",
	}

	for _, want := range required {
		if !strings.Contains(result.DisplayText, want) {
			t.Fatalf("projection response missing %q:\n%s", want, result.DisplayText)
		}
	}
}

func TestHaftQueryTool_ProjectionDelegatedBriefUsesCanonicalHandoffFields(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	decisionTool := NewHaftDecisionTool(fixture.store, fixture.haftDir, t.TempDir(), nil)

	_, err := decisionTool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"portfolio_ref":  fixture.comparedPortfolio.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Delegated handoff should render canonical fields only.",
		"weakest_link":   "Operational confidence still depends on limited production-grade evidence.",
		"invariants":     []string{"p99 latency remains below 50ms during cutover"},
		"admissibility":  []string{"No silent message loss during protocol migration"},
		"affected_files": []string{"internal/transport/grpc.go", "internal/transport/contracts.proto"},
		"predictions": []map[string]any{
			{
				"claim":      "Latency stays under 50ms",
				"observable": "publish latency p99",
				"threshold":  "< 50ms",
			},
			{
				"claim":      "Throughput stays above 100k events/sec",
				"observable": "throughput",
				"threshold":  "> 100k events/sec",
			},
		},
		"rollback": map[string]any{
			"triggers": []string{"Error budget exceeds 2% during canary"},
		},
	})))
	if err != nil {
		t.Fatal(err)
	}

	queryTool := NewHaftQueryTool(fixture.store, nil)
	result, err := queryTool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action": "projection",
		"view":   "brief",
	}))
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"## Delegated-Agent Brief",
		"Selected decision: gRPC",
		"Affected files: internal/transport/contracts.proto, internal/transport/grpc.go",
		"Invariants: p99 latency remains below 50ms during cutover",
		"Admissibility: No silent message loss during protocol migration",
		"Rollback triggers: Error budget exceeds 2% during canary",
		"Open claim risks:",
		"weakest link: Operational confidence still depends on limited production-grade evidence.",
		"unverified: Throughput stays above 100k events/sec (observable: throughput; threshold: > 100k events/sec)",
	}

	for _, want := range required {
		if !strings.Contains(result.DisplayText, want) {
			t.Fatalf("projection response missing %q:\n%s", want, result.DisplayText)
		}
	}
}

func TestHaftQueryTool_ProjectionChangeRationaleUsesCanonicalDecisionState(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	decisionTool := NewHaftDecisionTool(fixture.store, fixture.haftDir, t.TempDir(), nil)

	decisionResult, err := decisionTool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"portfolio_ref":  fixture.comparedPortfolio.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "It meets the latency target with acceptable operating cost.",
		"why_not_others": []map[string]any{{
			"variant": "REST",
			"reason":  "Higher steady-state latency with no decisive cost advantage.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"Error budget exceeds 2% during canary"},
		},
	})))
	if err != nil {
		t.Fatal(err)
	}

	if decisionResult.Meta == nil {
		t.Fatal("expected decision artifact metadata in tool response")
	}

	decisionID := strings.TrimSpace(decisionResult.Meta.ArtifactRef)
	if decisionID == "" {
		t.Fatal("expected decision artifact id in tool response")
	}

	_, err = artifact.Measure(fixture.ctx, fixture.store, fixture.haftDir, artifact.MeasureInput{
		DecisionRef: decisionID,
		Findings:    "Latency passed, rollout is still partially blocked on throughput headroom.",
		CriteriaMet: []string{
			"publish latency p99 < 50ms (observed: 44ms)",
		},
		CriteriaNotMet: []string{
			"Throughput stays above 100k events/sec (observed: 87k events/sec)",
		},
		Verdict: "partial",
	})
	if err != nil {
		t.Fatal(err)
	}

	queryTool := NewHaftQueryTool(fixture.store, nil)
	result, err := queryTool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action": "projection",
		"view":   "rationale",
	}))
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"## PR/Change Rationale",
		"Selected change: gRPC",
		"Problem signal: Latency variance between protocols",
		"Selected variant: gRPC",
		"Why selected: It meets the latency target with acceptable operating cost.",
		"Rejected alternatives:",
		"- REST: Higher steady-state latency with no decisive cost advantage.",
		"Rollback summary: Error budget exceeds 2% during canary",
		"Latest measurement verdict: weakens",
	}

	for _, want := range required {
		if !strings.Contains(result.DisplayText, want) {
			t.Fatalf("projection response missing %q:\n%s", want, result.DisplayText)
		}
	}
}

func TestHaftQueryTool_ProjectionHonorsContextFilter(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	for _, item := range []struct {
		title   string
		context string
	}{
		{title: "Payments transport choice", context: "payments"},
		{title: "Orders transport choice", context: "orders"},
	} {
		_, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
			Title:      item.title,
			Signal:     "Latency variance between protocols",
			Acceptance: "Choose the transport with the best latency trade-off",
			Context:    item.context,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	tool := NewHaftQueryTool(store, nil)
	result, err := tool.Execute(ctx, mustJSON(t, map[string]any{
		"action":  "projection",
		"view":    "engineer",
		"context": "payments",
	}))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.DisplayText, "Payments transport choice") {
		t.Fatalf("expected filtered projection to include payments context:\n%s", result.DisplayText)
	}
	if strings.Contains(result.DisplayText, "Orders transport choice") {
		t.Fatalf("expected filtered projection to exclude other contexts:\n%s", result.DisplayText)
	}
}

func TestHaftProblemTool_AdoptFindsLinkedDecisionWithoutFTSCarrierMatch(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Latency variance between protocols",
		Acceptance: "Choose the transport with the best latency trade-off",
		Context:    "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	portfolio, _, err := artifact.ExploreSolutions(ctx, store, haftDir, artifact.ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []artifact.Variant{
			{
				ID:            "V1",
				Title:         "REST",
				WeakestLink:   "chatty payloads",
				NoveltyMarker: "Keep JSON request-response semantics",
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
		PortfolioRef: portfolio.Meta.ID,
		Results: artifact.ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"V1": {"latency": "42ms"},
				"V2": {"latency": "18ms"},
			},
			NonDominatedSet: []string{"V2"},
			DominatedVariants: []artifact.DominatedVariantExplanation{
				{
					Variant:     "V1",
					DominatedBy: []string{"V2"},
					Summary:     "Higher latency with no compensating advantage.",
				},
			},
			ParetoTradeoffs: []artifact.ParetoTradeoffNote{
				{Variant: "V2", Summary: "Best latency in the compared set."},
			},
			SelectedRef:   "V2",
			PolicyApplied: "Minimize latency within budget.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		PortfolioRef:    portfolio.Meta.ID,
		SelectedTitle:   "gRPC",
		WhySelected:     "Lower latency is worth the tooling overhead for the current scope.",
		SelectionPolicy: "Prefer the transport that minimizes latency within the accepted budget envelope.",
		CounterArgument: "The tooling overhead could outweigh the latency gain outside the current workload profile.",
		WeakestLink:     "Operational confidence still depends on limited production-grade evidence.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "REST",
			Reason:  "Higher latency with no compensating advantage for the current scope.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Latency budget regresses after rollout"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	searchResults, err := artifact.FetchSearchResults(ctx, store, problem.Meta.ID, 20)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range searchResults {
		if item.Meta.ID == decision.Meta.ID {
			t.Fatalf("FTS unexpectedly found decision %q by problem ref; adopt regression needs a link-only lookup scenario", decision.Meta.ID)
		}
	}

	problemTool := NewHaftProblemTool(store, haftDir)
	result, err := problemTool.Execute(ctx, mustJSON(t, map[string]any{
		"action": "adopt",
		"ref":    problem.Meta.ID,
	}))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.DisplayText, "Found portfolio: "+portfolio.Meta.ID) {
		t.Fatalf("expected adopt result to surface linked portfolio:\n%s", result.DisplayText)
	}
	if !strings.Contains(result.DisplayText, "Found decision: "+decision.Meta.ID) {
		t.Fatalf("expected adopt result to surface linked decision:\n%s", result.DisplayText)
	}
	if result.Meta == nil {
		t.Fatal("expected adopt metadata")
	}
	if result.Meta.AdoptPortfolioRef != portfolio.Meta.ID {
		t.Fatalf("AdoptPortfolioRef = %q, want %q", result.Meta.AdoptPortfolioRef, portfolio.Meta.ID)
	}
	if result.Meta.ComparedPortfolioRef != portfolio.Meta.ID {
		t.Fatalf("ComparedPortfolioRef = %q, want %q", result.Meta.ComparedPortfolioRef, portfolio.Meta.ID)
	}
	if result.Meta.AdoptDecisionRef != decision.Meta.ID {
		t.Fatalf("AdoptDecisionRef = %q, want %q", result.Meta.AdoptDecisionRef, decision.Meta.ID)
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
			{"name": "cost", "role": "target"},
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

func TestHaftDecisionTool_DecideDisplayIncludesFullDecisionBody(t *testing.T) {
	fixture := setupDecisionToolFixture(t)

	result, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":           "decide",
		"problem_ref":      fixture.problem.Meta.ID,
		"selected_title":   "gRPC",
		"why_selected":     "Latency wins inside the compared portfolio.",
		"selection_policy": "Minimize latency within the approved cost envelope.",
		"counterargument":  "Protocol migration complexity could erase the latency gain.",
		"why_not_others": []map[string]any{{
			"variant": "REST",
			"reason":  "Latency tails stay above the accepted bound.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"Latency exceeds the approved p99 after rollout"},
		},
	})))
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"## 2. Decision",
		"**Selection policy:** Minimize latency within the approved cost envelope.",
		"## 3. Rationale",
		"**Counterargument:** Protocol migration complexity could erase the latency gain.",
		"**Rejected alternatives:**",
		"## 4. Consequences",
		"Latency exceeds the approved p99 after rollout",
	}

	for _, want := range required {
		if !strings.Contains(result.DisplayText, want) {
			t.Fatalf("expected direct decide display to include %q, got:\n%s", want, result.DisplayText)
		}
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

	foreignProblem, _, err := artifact.FrameProblem(fixture.ctx, fixture.store, fixture.haftDir, artifact.ProblemFrameInput{
		Title:      "Streaming fallback transport",
		Signal:     "Current transport cannot survive mobile reconnect churn.",
		Acceptance: "Select a transport that keeps reconnect latency predictable.",
	})
	if err != nil {
		t.Fatal(err)
	}

	foreignPortfolio, _, err := artifact.ExploreSolutions(fixture.ctx, fixture.store, fixture.haftDir, artifact.ExploreInput{
		ProblemRef: foreignProblem.Meta.ID,
		Variants: []artifact.Variant{
			{
				ID:            "Y1",
				Title:         "WebSocket",
				WeakestLink:   "connection lifecycle complexity",
				NoveltyMarker: "Keep long-lived duplex sessions",
			},
			{
				ID:            "Y2",
				Title:         "SSE",
				WeakestLink:   "server-to-client only",
				NoveltyMarker: "Use unidirectional event streams",
			},
		},
		NoSteppingStoneRationale: "Both variants are valid responses to the separate reconnect problem.",
	})
	if err != nil {
		t.Fatal(err)
	}

	foreignDecision, _, err := artifact.Decide(fixture.ctx, fixture.store, fixture.haftDir, artifact.DecideInput{
		ProblemRef:      foreignProblem.Meta.ID,
		PortfolioRef:    foreignPortfolio.Meta.ID,
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
		"claim_refs",
		"claim_scope",
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

func TestHaftDecisionTool_SchemaIncludesExtendedDecideInputFields(t *testing.T) {
	tool := NewHaftDecisionTool(setupHaftToolStore(t), t.TempDir(), t.TempDir(), nil)
	schema := tool.Schema()

	properties, ok := schema.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %T, want map[string]any", schema.Parameters["properties"])
	}

	for _, key := range []string{
		"context",
		"task_context",
		"problem_refs",
		"pre_conditions",
		"evidence_requirements",
		"refresh_triggers",
		"search_keywords",
	} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("schema missing %q", key)
		}
	}
}

func TestHaftDecisionTool_DecideUsesTaskContextInArtifactID(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	tool := NewHaftDecisionTool(fixture.store, fixture.haftDir, t.TempDir(), nil)

	result, err := tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"portfolio_ref":  fixture.comparedPortfolio.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Tool-mode decide should pass task_context into the DecisionRecord ID.",
		"task_context":   "Task #4: API/CLI cleanup",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta == nil {
		t.Fatal("expected decision artifact metadata")
	}

	pattern := regexp.MustCompile(`^dec-\d{8}-task-4-api-cli-cleanup-[0-9a-f]{8}$`)
	if !pattern.MatchString(result.Meta.ArtifactRef) {
		t.Fatalf("artifact ref = %q, want sanitized task_context slug before 8-hex suffix", result.Meta.ArtifactRef)
	}

	decision, err := fixture.store.Get(fixture.ctx, result.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if fields.TaskContext != "task-4-api-cli-cleanup" {
		t.Fatalf("structured task_context = %q, want sanitized slug", fields.TaskContext)
	}
}

func TestHaftDecisionTool_SchemaIncludesMeasureMeasurementsAndCompletePredictions(t *testing.T) {
	tool := NewHaftDecisionTool(setupHaftToolStore(t), t.TempDir(), t.TempDir(), nil)
	schema := tool.Schema()

	properties, ok := schema.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %T, want map[string]any", schema.Parameters["properties"])
	}

	if _, ok := properties["measurements"]; !ok {
		t.Fatal("schema missing \"measurements\"")
	}

	predictions, ok := properties["predictions"].(map[string]any)
	if !ok {
		t.Fatalf("predictions schema = %T, want map[string]any", properties["predictions"])
	}

	items, ok := predictions["items"].(map[string]any)
	if !ok {
		t.Fatalf("prediction items schema = %T, want map[string]any", predictions["items"])
	}

	required, ok := items["required"].([]string)
	if !ok {
		t.Fatalf("prediction required schema = %T, want []string", items["required"])
	}

	want := []string{"claim", "observable", "threshold"}
	if strings.Join(required, ",") != strings.Join(want, ",") {
		t.Fatalf("prediction required fields = %v, want %v", required, want)
	}
}

func TestHaftDecisionTool_DecideRoundTripsExtendedDecideInputFields(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	tool := NewHaftDecisionTool(fixture.store, fixture.haftDir, t.TempDir(), nil)

	additionalProblem, _, err := artifact.FrameProblem(fixture.ctx, fixture.store, fixture.haftDir, artifact.ProblemFrameInput{
		Title:      "Transport rollback coverage",
		Signal:     "Rollback criteria are under-specified",
		Acceptance: "Decision ties transport rollout to rollback evidence",
		Context:    "transport",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":                "decide",
		"context":               "transport",
		"problem_ref":           fixture.problem.Meta.ID,
		"problem_refs":          []string{fixture.problem.Meta.ID, additionalProblem.Meta.ID},
		"portfolio_ref":         fixture.comparedPortfolio.Meta.ID,
		"selected_title":        "gRPC",
		"why_selected":          "Lower latency with schema-checked client generation.",
		"pre_conditions":        []string{"Benchmarks reproduced in CI", "Consumer schema freeze approved"},
		"evidence_requirements": []string{"p99 latency stays below 20ms", "Generated clients compile in CI"},
		"refresh_triggers":      []string{"Latency budget regresses after rollout", "Generated clients fail across two releases"},
		"search_keywords":       "grpc transport latency schema rollback",
	})))
	if err != nil {
		t.Fatal(err)
	}

	decision, err := fixture.store.Get(fixture.ctx, result.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}

	if decision.Meta.Context != "transport" {
		t.Fatalf("decision context = %q, want transport", decision.Meta.Context)
	}
	if decision.SearchKeywords != "grpc transport latency schema rollback" {
		t.Fatalf("search keywords = %q", decision.SearchKeywords)
	}

	for _, want := range []string{
		"**Pre-conditions:**",
		"- [ ] Benchmarks reproduced in CI",
		"- [ ] Consumer schema freeze approved",
		"**Evidence requirements:**",
		"- p99 latency stays below 20ms",
		"- Generated clients compile in CI",
		"**Refresh triggers:**",
		"- Latency budget regresses after rollout",
		"- Generated clients fail across two releases",
	} {
		if !strings.Contains(decision.Body, want) {
			t.Fatalf("decision body missing %q:\n%s", want, decision.Body)
		}
	}

	links, err := fixture.store.GetLinks(fixture.ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	expectedRefs := map[string]bool{
		fixture.problem.Meta.ID:           false,
		additionalProblem.Meta.ID:         false,
		fixture.comparedPortfolio.Meta.ID: false,
	}

	for _, link := range links {
		if _, ok := expectedRefs[link.Ref]; ok {
			expectedRefs[link.Ref] = true
		}
	}

	for ref, found := range expectedRefs {
		if !found {
			t.Fatalf("decision links missing ref %q: %+v", ref, links)
		}
	}
}

func TestHaftDecisionTool_DecideLegacyPayloadStillWorksWithoutExtendedFields(t *testing.T) {
	fixture := setupDecisionToolFixture(t)

	result, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Lower latency is worth the tooling overhead for the current scope.",
	})))
	if err != nil {
		t.Fatal(err)
	}

	decision, err := fixture.store.Get(fixture.ctx, result.Meta.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}

	if decision.SearchKeywords != "" {
		t.Fatalf("expected empty search keywords for legacy payload, got %q", decision.SearchKeywords)
	}

	for _, unexpected := range []string{
		"**Pre-conditions:**",
		"**Evidence requirements:**",
		"**Refresh triggers:**",
	} {
		if strings.Contains(decision.Body, unexpected) {
			t.Fatalf("legacy payload should omit %q:\n%s", unexpected, decision.Body)
		}
	}
}

func TestHaftDecisionTool_DecideRejectsPartialPredictions(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	tool := NewHaftDecisionTool(fixture.store, fixture.haftDir, t.TempDir(), nil)

	_, err := tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"portfolio_ref":  fixture.comparedPortfolio.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Incomplete prediction payloads should be rejected before persistence.",
		"predictions": []map[string]any{
			{"claim": "Latency stays below 20ms"},
		},
	})))
	if err == nil {
		t.Fatal("expected validation error for partial predictions")
	}
	if !strings.Contains(err.Error(), "predictions[0] must include claim, observable, and threshold") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestHaftDecisionTool_MeasureRoundTripsMeasurements(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	tool := NewHaftDecisionTool(fixture.store, fixture.haftDir, t.TempDir(), nil)

	decision, _, err := artifact.Decide(fixture.ctx, fixture.store, fixture.haftDir, artifact.DecideInput{
		ProblemRef:      fixture.problem.Meta.ID,
		PortfolioRef:    fixture.comparedPortfolio.Meta.ID,
		SelectedTitle:   "gRPC",
		WhySelected:     "Measurement transport should preserve concrete measured values.",
		SelectionPolicy: "Minimize latency within budget.",
		CounterArgument: "Measured improvements could disappear outside the replay environment.",
		WeakestLink:     "Real workload variance may differ from staged replay traffic.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "REST",
			Reason:  "Higher latency with no compensating advantage in the measured workload.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Latency budget regresses after rollout"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = tool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action":           "measure",
		"decision_ref":     decision.Meta.ID,
		"findings":         "Replay benchmark stayed within the latency budget.",
		"measurements":     []string{"p99 latency: 18ms", "error rate: 0.05%"},
		"criteria_met":     []string{"p99 latency stays below 20ms"},
		"criteria_not_met": []string{},
		"verdict":          "accepted",
	}))
	if err != nil {
		t.Fatal(err)
	}

	updated, err := fixture.store.Get(fixture.ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"**Measurements:**",
		"- p99 latency: 18ms",
		"- error rate: 0.05%",
	}

	for _, want := range required {
		if !strings.Contains(updated.Body, want) {
			t.Fatalf("measurement body missing %q:\n%s", want, updated.Body)
		}
	}
}

func TestHaftDecisionTool_MeasureRejectsMalformedMeasurements(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	tool := NewHaftDecisionTool(fixture.store, fixture.haftDir, t.TempDir(), nil)

	decision, _, err := artifact.Decide(fixture.ctx, fixture.store, fixture.haftDir, artifact.DecideInput{
		ProblemRef:      fixture.problem.Meta.ID,
		PortfolioRef:    fixture.comparedPortfolio.Meta.ID,
		SelectedTitle:   "gRPC",
		WhySelected:     "Malformed measurement transport must fail instead of dropping values.",
		SelectionPolicy: "Minimize latency within budget.",
		CounterArgument: "Strict parsing could reject callers with broken measurement payloads.",
		WeakestLink:     "Broken payloads at the transport boundary still depend on caller discipline.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "REST",
			Reason:  "Higher latency with no compensating advantage in the measured workload.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Latency budget regresses after rollout"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = tool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action":       "measure",
		"decision_ref": decision.Meta.ID,
		"findings":     "Replay benchmark payload was malformed.",
		"measurements": []any{"p99 latency: 18ms", 42},
		"verdict":      "accepted",
	}))
	if err == nil {
		t.Fatal("expected malformed measurements to be rejected")
	}

	if !strings.Contains(err.Error(), "measurements must be an array of strings") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHaftQueryTool_ProjectionEngineerRendersDecisionRuntimeFields(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	decisionTool := NewHaftDecisionTool(fixture.store, fixture.haftDir, t.TempDir(), nil)

	_, err := decisionTool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":                "decide",
		"problem_ref":           fixture.problem.Meta.ID,
		"portfolio_ref":         fixture.comparedPortfolio.Meta.ID,
		"selected_title":        "gRPC",
		"why_selected":          "Engineer projection should surface the structured operational contract.",
		"pre_conditions":        []string{"Benchmarks reproduced in CI", "Consumer schema freeze approved"},
		"evidence_requirements": []string{"p99 latency stays below 20ms", "Generated clients compile in CI"},
		"refresh_triggers":      []string{"Latency budget regresses after rollout"},
		"rollback": map[string]any{
			"triggers": []string{"Cutover error rate exceeds the accepted ceiling"},
		},
	})))
	if err != nil {
		t.Fatal(err)
	}

	queryTool := NewHaftQueryTool(fixture.store, nil)
	result, err := queryTool.Execute(fixture.ctx, mustJSON(t, map[string]any{
		"action": "projection",
		"view":   "engineer",
	}))
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"Pre-conditions: Benchmarks reproduced in CI, Consumer schema freeze approved",
		"Evidence requirements: p99 latency stays below 20ms, Generated clients compile in CI",
		"Rollback triggers: Cutover error rate exceeds the accepted ceiling",
		"Refresh triggers: Latency budget regresses after rollout",
	}

	for _, want := range required {
		if !strings.Contains(result.DisplayText, want) {
			t.Fatalf("projection response missing %q:\n%s", want, result.DisplayText)
		}
	}
}

func TestHaftDecisionTool_DecideRejectsMalformedPredictions(t *testing.T) {
	fixture := setupDecisionToolFixture(t)

	_, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Lower latency is worth the tooling overhead for the current scope.",
		"predictions": []any{
			map[string]any{
				"claim":      "Latency improves after rollout",
				"observable": 42,
				"threshold":  "p99 < 20ms",
			},
		},
	})))
	if err == nil {
		t.Fatal("expected malformed predictions to be rejected")
	}

	if !strings.Contains(err.Error(), "predictions must be an array of prediction objects") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHaftDecisionTool_DecideRejectsMalformedExtendedStringArray(t *testing.T) {
	fixture := setupDecisionToolFixture(t)

	_, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Lower latency is worth the tooling overhead for the current scope.",
		"pre_conditions": []any{"Benchmarks reproduced in CI", 42},
	})))
	if err == nil {
		t.Fatal("expected malformed pre_conditions to be rejected")
	}

	if !strings.Contains(err.Error(), "pre_conditions must be an array of strings") {
		t.Fatalf("unexpected error: %v", err)
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

func TestHaftDecisionTool_MeasureWrongKindUsesPlainLanguage(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	tool := NewHaftDecisionTool(store, t.TempDir(), t.TempDir(), nil)

	problem, _, err := artifact.FrameProblem(ctx, store, t.TempDir(), artifact.ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Latency variance between protocols",
		Acceptance: "Choose the transport with the best latency trade-off",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := tool.Execute(ctx, mustJSON(t, map[string]any{
		"action":       "measure",
		"decision_ref": problem.Meta.ID,
	}))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result.DisplayText, "DecisionRecord") {
		t.Fatalf("expected plain-language kind label, got %q", result.DisplayText)
	}
	if !strings.Contains(result.DisplayText, "not a decision") {
		t.Fatalf("expected plain-language mismatch message, got %q", result.DisplayText)
	}
	if issues := present.LintGeneratedText(result.DisplayText); len(issues) != 0 {
		t.Fatalf("expected lint-clean generated message, got %+v\n%s", issues, result.DisplayText)
	}
}

func TestHaftDecisionTool_EvidenceAttachesToDecision(t *testing.T) {

	fixture := setupDecisionToolFixture(t)

	decisionResult, err := fixture.tool.Execute(fixture.ctx, mustJSON(t, completeDecisionArgs(map[string]any{
		"action":         "decide",
		"problem_ref":    fixture.problem.Meta.ID,
		"selected_title": "gRPC",
		"why_selected":   "Latency wins inside the compared portfolio.",
		"predictions": []map[string]any{
			{
				"claim":      "First request after warmup stays below 20ms",
				"observable": "latency",
				"threshold":  "< 20ms",
			},
		},
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
		"claim_refs":       []string{"claim-001"},
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
	if got := strings.Join(item.ClaimRefs, ","); got != "claim-001" {
		t.Fatalf("claim refs = %q, want claim-001", got)
	}
	if got := strings.Join(item.ClaimScope, ","); got != "First request after warmup stays below 20ms" {
		t.Fatalf("claim scope = %q, want derived fallback scope", got)
	}
}
