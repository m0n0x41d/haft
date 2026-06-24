package present_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/present"
)

func TestNavStrip_AvailableGuardLine(t *testing.T) {
	state := artifact.NavState{
		DerivedStatus: artifact.DerivedFramed,
		Mode:          artifact.ModeTactical,
		NextAction:    `/h-explore (generate variants) | /h-decide (decide directly)`,
	}

	output := present.NavStrip(state)

	if !strings.Contains(output, "Available:") {
		t.Errorf("should contain 'Available:', got:\n%s", output)
	}
	if !strings.Contains(output, "do not auto-execute") {
		t.Errorf("should contain guard line, got:\n%s", output)
	}
}

func TestNavStrip_NoGuardWhenDecided(t *testing.T) {
	state := artifact.NavState{
		DerivedStatus: artifact.DerivedDecided,
		DecisionInfo:  "Use Redis",
	}

	output := present.NavStrip(state)

	if strings.Contains(output, "Available:") {
		t.Errorf("DECIDED state should NOT show Available, got:\n%s", output)
	}
}

func TestNavStrip_AllFieldsRendered(t *testing.T) {
	state := artifact.NavState{
		Context:       "payments",
		Mode:          artifact.ModeStandard,
		DerivedStatus: artifact.DerivedExploring,
		PortfolioInfo: "API redesign",
		StaleCount:    2,
		NextAction:    "/h-compare (compare variants)",
	}

	output := present.NavStrip(state)

	for _, want := range []string{"Context: payments", "Mode: standard", "Status: EXPLORING", "Portfolio: API redesign", "Stale: 2", "Available:", "/h-compare"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

func TestDriftResponseSummary_StaysCompactOnLargeReports(t *testing.T) {
	// Simulate a bloated report — 1 decision with 5000 added files (e.g. a
	// Tauri/vendor subtree drift). Summary mode must NOT dump per-file rows.
	files := []artifact.DriftItem{
		{Path: "src/main.go", Status: artifact.DriftModified},
		{Path: "src/util.go", Status: artifact.DriftModified},
		{Path: "src/handler.go", Status: artifact.DriftModified},
	}
	for i := 0; i < 5000; i++ {
		files = append(files, artifact.DriftItem{Path: fmt.Sprintf("vendor/lib%d/file.go", i), Status: artifact.DriftAdded})
	}
	for i := 0; i < 7; i++ {
		files = append(files, artifact.DriftItem{Path: fmt.Sprintf("internal/removed%d.go", i), Status: artifact.DriftMissing})
	}
	reports := []artifact.DriftReport{
		{
			DecisionID:    "dec-vendor-bloat",
			DecisionTitle: "Bloat case",
			HasBaseline:   true,
			Files:         files,
		},
	}

	summary := present.DriftResponseSummary(reports, "")
	verbose := present.DriftResponse(reports, "")

	if !strings.Contains(summary, "3 modified, 5000 added, 7 missing") {
		t.Fatalf("summary should report counts; got:\n%s", summary)
	}
	if strings.Contains(summary, "vendor/lib100/file.go") {
		t.Fatalf("summary must NOT dump added paths (got per-file noise):\n%s", summary)
	}
	if strings.Contains(summary, "removed3.go") {
		t.Fatalf("summary must NOT dump missing paths:\n%s", summary)
	}
	if !strings.Contains(summary, "src/main.go") {
		t.Fatalf("summary should include top modified paths (actionable signal):\n%s", summary)
	}

	summaryLines := strings.Count(summary, "\n")
	if summaryLines > 30 {
		t.Fatalf("summary should stay compact on a 5010-file drift (≤30 lines); got %d lines", summaryLines)
	}

	verboseLines := strings.Count(verbose, "\n")
	if verboseLines < 5000 {
		t.Fatalf("verbose should dump everything (≥5000 lines); got %d", verboseLines)
	}
}

func TestDriftResponseSummary_StaysCompactOnImpactFanout(t *testing.T) {
	reports := make([]artifact.DriftReport, 0, 8)
	for r := 0; r < 8; r++ {
		impacts := make([]artifact.ModuleImpact, 0, 20)
		for i := 0; i < 20; i++ {
			impacts = append(impacts, artifact.ModuleImpact{
				ModulePath: fmt.Sprintf("internal/mod-%02d-%02d", r, i),
				DecisionIDs: []string{
					"dec-a",
					"dec-b",
					"dec-c",
					"dec-d",
					"dec-e",
					"dec-f",
				},
				DecisionTitles: map[string]string{
					"dec-a": "Decision A",
					"dec-b": "Decision B",
					"dec-c": "Decision C",
					"dec-d": "Decision D",
					"dec-e": "Decision E",
					"dec-f": "Decision F",
				},
			})
		}
		reports = append(reports, artifact.DriftReport{
			DecisionID:        fmt.Sprintf("dec-impact-%02d", r),
			DecisionTitle:     fmt.Sprintf("Impact %02d", r),
			HasBaseline:       true,
			Files:             []artifact.DriftItem{{Path: fmt.Sprintf("internal/file-%02d.go", r), Status: artifact.DriftModified}},
			ImpactedModules:   impacts,
			LikelyImplemented: true,
		})
	}

	summary := present.DriftResponseSummary(reports, "")

	for _, want := range []string{
		"Impact propagation for **Impact 00** `dec-impact-00`",
		"internal/mod-00-00",
		"**Decision A** `dec-a`",
		"... +2",
		"... and 15 more impacted module(s)",
		"... and 5 more decision(s) with impact propagation omitted from summary",
		"verbose: true",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
	for _, banned := range []string{"internal/mod-00-19", "Impact propagation for dec-impact-07"} {
		if strings.Contains(summary, banned) {
			t.Fatalf("summary leaked uncapped impact detail %q:\n%s", banned, summary)
		}
	}

	summaryLines := strings.Count(summary, "\n")
	if summaryLines > 80 {
		t.Fatalf("impact summary should stay compact; got %d lines\n%s", summaryLines, summary)
	}
}

func TestDriftResponseSummary_EmptyAndNoBaseline(t *testing.T) {
	if got := present.DriftResponseSummary(nil, ""); !strings.Contains(got, "No drift detected") {
		t.Fatalf("empty reports should say so; got:\n%s", got)
	}

	reports := []artifact.DriftReport{
		{
			DecisionID:        "dec-001",
			DecisionTitle:     "Implemented decision",
			HasBaseline:       false,
			LikelyImplemented: true,
			Files:             []artifact.DriftItem{{Path: "app.go", Status: artifact.DriftNoBaseline}},
		},
	}
	got := present.DriftResponseSummary(reports, "")
	if !strings.Contains(got, "git activity detected after decision date") {
		t.Fatalf("summary should preserve LikelyImplemented hint:\n%s", got)
	}
	if !strings.Contains(got, "**Implemented decision** `dec-001`") {
		t.Fatalf("summary should render no-baseline decisions title-first with ref:\n%s", got)
	}
}

func TestScanResponseSummary_StaysCompactAndKeepsVerboseRecoveryHint(t *testing.T) {
	items := make([]artifact.StaleItem, 0, 25)
	for i := 0; i < 25; i++ {
		items = append(items, artifact.StaleItem{
			ID:     fmt.Sprintf("dec-stale-%02d", i),
			Title:  fmt.Sprintf("Stale %02d", i),
			Kind:   string(artifact.KindDecisionRecord),
			Reason: "evidence expired",
		})
	}

	summary := present.ScanResponseSummary(items, "")

	if !strings.Contains(summary, "Refresh Due (25 artifact(s)) — summary") {
		t.Fatalf("summary missing heading:\n%s", summary)
	}
	if !strings.Contains(summary, "dec-stale-09") {
		t.Fatalf("summary should include top stale items:\n%s", summary)
	}
	if !strings.Contains(summary, "**Stale 00** `dec-stale-00`") {
		t.Fatalf("summary should render stale items title-first with ref:\n%s", summary)
	}
	if strings.Contains(summary, "dec-stale-10") {
		t.Fatalf("summary should omit stale item 11+:\n%s", summary)
	}
	if !strings.Contains(summary, "15 more refresh-due artifact(s)") {
		t.Fatalf("summary should report omitted stale count:\n%s", summary)
	}
	if !strings.Contains(summary, "verbose: true") {
		t.Fatalf("summary should explain full recovery:\n%s", summary)
	}
}

func TestDriftResponse_LikelyImplemented(t *testing.T) {
	reports := []artifact.DriftReport{
		{
			DecisionID:        "dec-001",
			DecisionTitle:     "Implemented decision",
			HasBaseline:       false,
			LikelyImplemented: true,
			Files:             []artifact.DriftItem{{Path: "app.go", Status: artifact.DriftNoBaseline}},
		},
		{
			DecisionID:    "dec-002",
			DecisionTitle: "Not started decision",
			HasBaseline:   false,
			Files:         []artifact.DriftItem{{Path: "other.go", Status: artifact.DriftNoBaseline}},
		},
	}

	output := present.DriftResponse(reports, "")

	if !strings.Contains(output, "git activity detected after decision date") {
		t.Errorf("should report git activity for decision with commits:\n%s", output)
	}
	if !strings.Contains(output, "no git activity detected after decision date") {
		t.Errorf("should report no git activity for decision without commits:\n%s", output)
	}
}

func TestProblemResponse_ShowsRecall(t *testing.T) {
	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "prob-001",
			Kind:  artifact.KindProblemCard,
			Title: "Test problem",
			Mode:  artifact.ModeStandard,
		},
		Body: "# Test\n\n## Signal\n\nSomething\n\n## Related History\n\n- [decision] **Redis cache** `dec-001`\n",
	}

	response := present.ProblemResponse("frame", a, "/tmp/test.md", "\n-- nav --\n")

	if !strings.Contains(response, "Problem framed: **Test problem** `prob-001`") {
		t.Fatalf("frame response should pair problem title with ref:\n%s", response)
	}
	if strings.Contains(response, "\nID: prob-001\n") {
		t.Fatalf("frame response should not expose a standalone bare problem ID:\n%s", response)
	}
	if !strings.Contains(response, "Related History") {
		t.Error("frame response should surface Related History from body")
	}
	if !strings.Contains(response, "Redis cache") {
		t.Error("frame response should show recalled artifact")
	}
}

func TestProblemResponse_PreservesBodyVerbatim(t *testing.T) {
	relatedHistory := "## Related History\n\n- [decision] **Fix DecisionRecord parser** `dec-001`\n"
	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "prob-001",
			Kind:  artifact.KindProblemCard,
			Title: "ProblemCard migration",
			Mode:  artifact.ModeStandard,
		},
		Body: "# Test\n\n" + relatedHistory,
	}

	response := present.ProblemResponse("frame", a, "", "\n-- nav --\n")

	if !strings.Contains(response, relatedHistory) {
		t.Fatalf("expected related history slice to stay verbatim, got:\n%s", response)
	}
}

func TestProblemResponse_NoRecallWhenAbsent(t *testing.T) {
	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "prob-001",
			Kind:  artifact.KindProblemCard,
			Title: "Test problem",
			Mode:  artifact.ModeStandard,
		},
		Body: "# Test\n\n## Signal\n\nSomething\n",
	}

	response := present.ProblemResponse("frame", a, "", "\n-- nav --\n")

	if strings.Contains(response, "Related History") {
		t.Error("frame response should NOT show Related History when not in body")
	}
}

func TestProblemResponse_ShowsProblemType(t *testing.T) {
	fields, err := json.Marshal(artifact.ProblemFields{
		ProblemType: artifact.ProblemTypeDiagnosis,
		Signal:      "signal",
	})
	if err != nil {
		t.Fatal(err)
	}

	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "prob-001",
			Kind:  artifact.KindProblemCard,
			Title: "Investigate webhook failures",
			Mode:  artifact.ModeStandard,
		},
		Body:           "# Test\n\n## Signal\n\nSomething\n",
		StructuredData: string(fields),
	}

	response := present.ProblemResponse("frame", a, "", "\n-- nav --\n")

	if !strings.Contains(response, "Type: diagnosis") {
		t.Fatalf("expected problem type in frame response, got:\n%s", response)
	}
}

// Issue #71 regression: explore responses must inline the canonical variant
// id-to-title index. Without it, the agent's only path to the ids was scraping
// a "Generated id: V1" log line from explore output, which ChatGPT/Codex agents
// routinely missed and then sent free-form titles to compare.
func TestSolutionResponse_ExploreShowsVariantsIndexAndUsageHint(t *testing.T) {
	fields, err := json.Marshal(artifact.PortfolioFields{
		Variants: []artifact.Variant{
			{ID: "V1", Title: "Kafka"},
			{ID: "V2", Title: "NATS JetStream"},
			{ID: "V3", Title: "Redis Streams"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "sol-001",
			Kind:  artifact.KindSolutionPortfolio,
			Title: "Transport portfolio",
		},
		StructuredData: string(fields),
	}

	response := present.SolutionResponse("explore", a, "/tmp/sol.md", "")

	required := []string{
		"Portfolio created: **Transport portfolio** `sol-001`",
		"Variants:",
		"V1 — Kafka",
		"V2 — NATS JetStream",
		"V3 — Redis Streams",
		"`scores`",
		"`dominated_variants",
		"`pareto_tradeoffs",
		"`non_dominated_set`",
		"`selected_ref`",
		`haft_solution(action="compare")`,
	}
	for _, want := range required {
		if !strings.Contains(response, want) {
			t.Fatalf("explore response missing %q:\n%s", want, response)
		}
	}
	if strings.Contains(response, "\nID: sol-001\n") {
		t.Fatalf("explore response should not expose a standalone bare portfolio ID:\n%s", response)
	}
}

func TestSolutionResponse_CompareShowsNarrativeSummary(t *testing.T) {
	fields, err := json.Marshal(artifact.PortfolioFields{
		Variants: []artifact.Variant{
			{ID: "V1", Title: "Kafka"},
			{ID: "V2", Title: "NATS"},
			{ID: "V3", Title: "Redis Streams"},
		},
		Comparison: &artifact.ComparisonResult{
			NonDominatedSet: []string{"V1", "V2"},
			DominatedVariants: []artifact.DominatedVariantExplanation{
				{
					Variant:     "V3",
					DominatedBy: []string{"V2"},
					Summary:     "Lower throughput with no compensating operations win.",
				},
			},
			ParetoTradeoffs: []artifact.ParetoTradeoffNote{
				{Variant: "V1", Summary: "Best throughput, but highest ops cost."},
				{Variant: "V2", Summary: "Best ops simplicity, but lower headroom than Kafka."},
			},
			PolicyApplied:           "Minimize operations load above the throughput floor.",
			SelectedRef:             "V2",
			RecommendationRationale: "Meets the throughput floor while minimizing operational burden.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "sol-001",
			Kind:  artifact.KindSolutionPortfolio,
			Title: "Transport portfolio",
		},
		StructuredData: string(fields),
	}

	response := present.SolutionResponse("compare", a, "/tmp/sol.md", "\n-- nav --\n")

	required := []string{
		"Comparison added to: **Transport portfolio** `sol-001`",
		"File: /tmp/sol.md",
		"Computed Pareto front: Kafka `V1`, NATS `V2`",
		"Dominated variant elimination:",
		"Redis Streams `V3`: dominated by NATS `V2`. Lower throughput with no compensating operations win.",
		"Pareto-front trade-offs:",
		"Kafka `V1`: Best throughput, but highest ops cost.",
		"Recommendation (advisory): NATS `V2`",
		"Recommendation rationale: Meets the throughput floor while minimizing operational burden.",
		"Human choice remains open until decide.",
	}

	for _, want := range required {
		if !strings.Contains(response, want) {
			t.Fatalf("compare response missing %q:\n%s", want, response)
		}
	}
	if strings.Contains(response, "\nID: sol-001\n") {
		t.Fatalf("compare response should not expose a standalone bare portfolio ID:\n%s", response)
	}
}

func TestSearchResponse_PreservesQueryTitleAndBodyVerbatim(t *testing.T) {
	query := "DecisionRecord"
	title := "Fix DecisionRecord parser"
	body := "# Summary\n\nDecisionRecord must stay verbatim here."
	results := []*artifact.Artifact{{
		Meta: artifact.Meta{
			ID:    "dec-001",
			Kind:  artifact.KindDecisionRecord,
			Title: title,
		},
		Body: body,
	}}

	response := present.SearchResponse(results, query)

	if !strings.Contains(response, "## Search: "+query+" (1 results)") {
		t.Fatalf("expected query to stay verbatim, got:\n%s", response)
	}
	if !strings.Contains(response, title) {
		t.Fatalf("expected title to stay verbatim, got:\n%s", response)
	}
	if !strings.Contains(response, "DecisionRecord must stay verbatim here.") {
		t.Fatalf("expected body preview to stay verbatim, got:\n%s", response)
	}
}

// TestStatusResponse_RendersDriftSection — H1 of V2
// (dec-20260526-9fdd33ed). /h-status must surface drift summary when
// CheckDrift returned non-empty reports via StatusData.Drift.
func TestStatusResponse_RendersDriftSection(t *testing.T) {
	data := artifact.StatusData{
		Drift: []artifact.DriftReport{
			{
				DecisionID:    "dec-foo",
				DecisionTitle: "Foo decision",
				HasBaseline:   true,
				Files: []artifact.DriftItem{
					{Path: "internal/foo/a.go", Status: artifact.DriftModified},
					{Path: "internal/foo/b.go", Status: artifact.DriftModified},
					{Path: "internal/foo/new.go", Status: artifact.DriftAdded},
				},
			},
			{
				DecisionID:    "dec-bar",
				DecisionTitle: "Bar decision",
				HasBaseline:   true,
				Files: []artifact.DriftItem{
					{Path: "internal/bar/gone.go", Status: artifact.DriftMissing},
				},
			},
		},
	}

	response := present.StatusResponse(data)

	wants := []string{
		"### Drift Detected (2 decision(s))",
		"Foo decision",
		"dec-foo",
		"2 modified",
		"1 added",
		"Bar decision",
		"1 missing",
		"/h-verify",
		"/h-refresh scan",
	}
	for _, w := range wants {
		if !strings.Contains(response, w) {
			t.Errorf("StatusResponse drift section missing %q\nfull response:\n%s", w, response)
		}
	}
}

// TestStatusResponse_NoDriftSectionWhenEmpty — drift section MUST NOT
// appear when StatusData.Drift is nil/empty. Avoids noise in solo-dev
// sessions where projectRoot wasn't passed (e.g., test fixtures) or
// where CheckDrift returned zero drifted decisions.
func TestStatusResponse_NoDriftSectionWhenEmpty(t *testing.T) {
	data := artifact.StatusData{
		HealthyDecisions: []*artifact.Artifact{
			{Meta: artifact.Meta{ID: "dec-x", Title: "Some decision"}},
		},
	}
	response := present.StatusResponse(data)
	if strings.Contains(response, "Drift Detected") {
		t.Errorf("Drift section should not appear when StatusData.Drift is empty; got:\n%s", response)
	}
}

func TestCockpitStatusResponse_CompactsDefaultAndNamesDrilldowns(t *testing.T) {
	data := artifact.StatusData{
		HealthyDecisions: []*artifact.Artifact{
			{Meta: artifact.Meta{ID: "dec-healthy", Title: "Healthy decision"}},
		},
		PendingDecisions: []*artifact.Artifact{
			{Meta: artifact.Meta{ID: "dec-pending", Title: "Pending decision"}},
		},
		UnassessedDecisions: []*artifact.Artifact{
			{Meta: artifact.Meta{ID: "dec-unassessed", Title: "Unassessed decision"}},
		},
		StaleItems: []artifact.StaleItem{
			{ID: "dec-stale-a", Title: "Stale A", Reason: "expired"},
			{ID: "dec-stale-b", Title: "Stale B", Reason: "at risk"},
			{ID: "dec-stale-c", Title: "Stale C", Reason: "hidden by cap"},
		},
		Drift: []artifact.DriftReport{{
			DecisionID:    "dec-drift",
			DecisionTitle: "Drifted decision",
			Files: []artifact.DriftItem{
				{Path: "internal/a.go", Status: artifact.DriftModified},
				{Path: "internal/b.go", Status: artifact.DriftAdded},
			},
		}},
		InProgressProblems: []*artifact.Artifact{
			{Meta: artifact.Meta{ID: "prob-progress", Title: "Progress problem"}},
		},
		InProgressBy:    map[string]string{"prob-progress": "sol-001"},
		PortfolioTitles: map[string]string{"sol-001": "Progress portfolio"},
		RecentNotes: []*artifact.Artifact{
			{Meta: artifact.Meta{ID: "note-hidden", Title: "Hidden note"}},
		},
		ReconciliationCues: artifact.ReconciliationCueReport{
			Summary: artifact.ReconciliationCueSummary{
				HighFanoutEvents:       1,
				MaxFanout:              4,
				ReconciliationGroups:   2,
				OperatorRequiredGroups: 2,
				GoverningConflictSets:  1,
				GoverningOverlapSets:   1,
			},
			Cues: []artifact.ReconciliationCue{{
				Kind: artifact.ReconciliationCueHighFanout,
			}},
			Commands: []string{
				artifact.StatusCompactDriftEventsCommand,
				artifact.StatusCompactDecisionReconcileCommand,
				artifact.StatusCompactGoverningSetCommand,
			},
		},
	}

	output := present.CockpitStatusResponse(data)

	for _, want := range []string{
		"### Operator Cockpit",
		"**Refresh due** (3)",
		"Stale A",
		"Stale B",
		"... and 1 more; run `haft_refresh(action=\"scan\", verbose=true)`",
		"**Audit-only drift events**: 1 unique event(s), 1 impacted decision(s)",
		"**Binding resolution needed**: 1 unique event(s), 1 impacted decision(s) need precise binding targets",
		"**Reconciliation cues**: 1 high-fanout drift event(s), max fanout 4; 2 reconciliation group(s), 2 operator-required; 1 governing conflict set(s), 1 overlap set(s)",
		`drill down with haft_query(action="drift_events", limit=5) / haft_query(action="decision_reconcile", limit=5) / haft_query(action="governing_set", limit=5)`,
		"**Progress problem** `prob-progress` → **Progress portfolio** `sol-001`",
		"Full status: `haft_query(action=\"status\", full=true)`",
		"Coverage: `haft_query(action=\"coverage\")`",
		"Drift events: `haft_query(action=\"drift_events\", limit=5)`; decision reconciliation: `haft_query(action=\"decision_reconcile\", limit=5)`; governing set: `haft_query(action=\"governing_set\", limit=5)`.",
		"Maintenance plan: `haft_refresh(action=\"plan\")` (compact); full work order: `haft_refresh(action=\"plan\", verbose=true)`",
		"Judgment review: `haft_refresh(action=\"review\")` / `haft overseer judgment --json --limit 20`",
		"Safe drain preview: `haft_refresh(action=\"drain\", dry_run=true)`",
		"Default status omits shipped/pending decision lists",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("cockpit output missing %q:\n%s", want, output)
		}
	}

	for _, unwanted := range []string{
		"### Shipped / Healthy",
		"### Pending",
		"### Recent Notes",
		"Hidden note",
		"Stale C",
		"**Drift events**",
	} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("cockpit output should omit %q:\n%s", unwanted, output)
		}
	}
}

func TestCockpitStatusResponse_DoesNotReportResolvedDriftEventsAsAttention(t *testing.T) {
	data := artifact.StatusData{
		Drift: []artifact.DriftReport{{
			DecisionID: "dec-resolved",
			Files: []artifact.DriftItem{{
				Path:   "internal/a.go",
				Status: artifact.DriftModified,
			}},
		}},
		DriftEvents: artifact.DriftEventReport{
			SchemaVersion: 2,
			Summary: artifact.DriftEventSummary{
				UniqueEvents:           1,
				ImpactedDecisions:      1,
				MaterialEvents:         1,
				ResolvedByLedgerEvents: 1,
				MaxFanout:              1,
			},
			Events: []artifact.DriftEvent{{
				EventID:          "drift-event-resolved",
				ChangedTargetRef: "symbol:internal/a.go::func:Done",
				Materiality:      artifact.DriftMaterialityMaterialSymbol,
				Fanout:           1,
				ImpactedDecisions: []artifact.DriftEventDecision{{
					DecisionID: "dec-resolved",
				}},
				RootCause:        artifact.DriftEventRootCauseSemanticTargetChanged,
				ResolutionStatus: artifact.DriftEventResolutionResolved,
				ResolutionRecord: &artifact.DriftEventResolution{
					EventID: "drift-event-resolved",
					Status:  artifact.DriftEventResolutionResolved,
					Reason:  "verified externally",
				},
			}},
		},
	}

	output := present.CockpitStatusResponse(data)

	if strings.Contains(output, "**Drift events**") {
		t.Fatalf("resolved drift event should not appear as active cockpit drift:\n%s", output)
	}
	if strings.Contains(output, "Decision Health") {
		t.Fatalf("resolved-only drift should not create decision health noise:\n%s", output)
	}
	if !strings.Contains(output, "No operator-blocking refresh, drift, or commission items") {
		t.Fatalf("resolved-only drift should leave cockpit calm:\n%s", output)
	}
}

func TestCockpitStatusResponse_GroupsAuditOnlyDrift(t *testing.T) {
	data := artifact.StatusData{
		HealthyDecisions: []*artifact.Artifact{
			{Meta: artifact.Meta{ID: "dec-healthy", Title: "Healthy decision"}},
		},
		Drift: []artifact.DriftReport{
			{
				DecisionID:    "dec-audit",
				DecisionTitle: "Audit-only decision",
				Files: []artifact.DriftItem{{
					Path:        "internal/shared.go",
					Status:      artifact.DriftModified,
					Materiality: artifact.DriftMaterialityAdjacentFileChurn,
					AuditOnly:   true,
				}},
			},
			{
				DecisionID:    "dec-audit-2",
				DecisionTitle: "Second audit-only decision",
				Files: []artifact.DriftItem{{
					Path:        "internal/shared.go",
					Status:      artifact.DriftModified,
					Materiality: artifact.DriftMaterialityAdjacentFileChurn,
					AuditOnly:   true,
				}},
			},
		},
	}

	output := present.CockpitStatusResponse(data)

	for _, want := range []string{
		"**Audit-only drift events**: 1 unique event(s), 2 impacted decision(s), 0 material governed-symbol changes",
		"audit details available",
		"Drift: 0 material event(s), 1 audit-only event(s)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("cockpit output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "**Drift events**") {
		t.Fatalf("audit-only drift should not render as material drift:\n%s", output)
	}
}

func TestCockpitStatusResponse_GroupsBindingResolutionDrift(t *testing.T) {
	data := artifact.StatusData{
		Drift: []artifact.DriftReport{{
			DecisionID:    "dec-needs-binding",
			DecisionTitle: "Whole-file fallback decision",
			HasBaseline:   true,
			Files: []artifact.DriftItem{{
				Path:        "notes.txt",
				Status:      artifact.DriftModified,
				Materiality: artifact.DriftMaterialityNeedsBindingResolution,
			}},
		}},
	}

	output := present.CockpitStatusResponse(data)
	for _, want := range []string{
		"**Binding resolution needed**",
		"Drift: 0 material event(s), 0 audit-only event(s), 1 needs-binding event(s)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "**Drift events**") {
		t.Fatalf("binding-resolution drift should not render as material drift:\n%s", output)
	}
}

func TestCockpitStatusResponse_GroupsLegacyFileFallbackAsBindingResolution(t *testing.T) {
	data := artifact.StatusData{
		Drift: []artifact.DriftReport{{
			DecisionID:    "dec-legacy-file",
			DecisionTitle: "Legacy whole-file decision",
			HasBaseline:   true,
			Files: []artifact.DriftItem{{
				Path:        "internal/tools/haft.go",
				Status:      artifact.DriftModified,
				Materiality: artifact.DriftMaterialityUnknownLegacyFileScope,
			}},
		}},
	}

	output := present.CockpitStatusResponse(data)
	for _, want := range []string{
		"**Binding resolution needed**",
		"Drift: 0 material event(s), 0 audit-only event(s), 1 needs-binding event(s)",
		"haft_query(action=\"decision_reconcile\", limit=5)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "**Drift events**") {
		t.Fatalf("legacy file fallback should not render as material drift:\n%s", output)
	}
}

func TestStatusResponse_ShowsDerivedDecisionHealth(t *testing.T) {
	data := artifact.StatusData{
		HealthyDecisions: []*artifact.Artifact{
			{
				Meta: artifact.Meta{
					ID:    "dec-healthy",
					Title: "Healthy decision",
				},
			},
		},
		PendingDecisions: []*artifact.Artifact{
			{
				Meta: artifact.Meta{
					ID:    "dec-pending",
					Title: "Pending decision",
				},
			},
		},
		UnassessedDecisions: []*artifact.Artifact{
			{
				Meta: artifact.Meta{
					ID:    "dec-unassessed",
					Title: "Unassessed decision",
				},
			},
		},
		DecisionHealth: map[string]artifact.DecisionHealth{
			"dec-stale": {
				Maturity:  artifact.DecisionMaturityShipped,
				Freshness: artifact.DecisionFreshnessStale,
			},
		},
		StaleItems: []artifact.StaleItem{
			{
				ID:     "dec-stale",
				Title:  "Stale decision",
				Reason: "evidence degraded (R_eff: 0.40)",
			},
		},
		OpenCommissions: []artifact.WorkCommissionStatus{
			{
				ID:               "wc-stale",
				Title:            "Drain stale decision",
				State:            "queued",
				DecisionRef:      "dec-stale",
				DecisionTitle:    "Stale decision",
				AttentionReason:  "open longer than 24h0m0s",
				SuggestedActions: []string{"inspect", "requeue", "cancel"},
			},
		},
		CommissionAttention: []artifact.WorkCommissionStatus{
			{
				ID:               "wc-stale",
				Title:            "Drain stale decision",
				State:            "queued",
				DecisionRef:      "dec-stale",
				DecisionTitle:    "Stale decision",
				AttentionReason:  "open longer than 24h0m0s",
				SuggestedActions: []string{"inspect", "requeue", "cancel"},
			},
		},
	}

	output := present.StatusResponse(data)

	for _, want := range []string{
		"### Shipped / Healthy (1)",
		"### Pending (1)",
		"### Unassessed (1)",
		"**Stale decision** `dec-stale` — Shipped / Stale — evidence degraded (R_eff: 0.40)",
		"### WorkCommissions Need Attention (1)",
		"**Drain stale decision** `wc-stale` queued → **Stale decision** `dec-stale` — open longer than 24h0m0s — actions: inspect, requeue, cancel",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestStatusResponse_ShowsProblemTypeInListings(t *testing.T) {
	backlogFields, err := json.Marshal(artifact.ProblemFields{ProblemType: artifact.ProblemTypeSearch})
	if err != nil {
		t.Fatal(err)
	}
	inProgressFields, err := json.Marshal(artifact.ProblemFields{ProblemType: artifact.ProblemTypeDiagnosis})
	if err != nil {
		t.Fatal(err)
	}

	data := artifact.StatusData{
		BacklogProblems: []*artifact.Artifact{
			{
				Meta:           artifact.Meta{ID: "prob-backlog", Title: "Backlog problem"},
				StructuredData: string(backlogFields),
			},
		},
		InProgressProblems: []*artifact.Artifact{
			{
				Meta:           artifact.Meta{ID: "prob-progress", Title: "In progress problem"},
				StructuredData: string(inProgressFields),
			},
		},
		InProgressBy:    map[string]string{"prob-progress": "sol-001"},
		PortfolioTitles: map[string]string{"sol-001": "Progress portfolio"},
		AddressedProblems: []*artifact.Artifact{
			{
				Meta: artifact.Meta{ID: "prob-addressed", Title: "Addressed problem"},
			},
		},
		AddressedBy:    map[string]string{"prob-addressed": "dec-001"},
		DecisionTitles: map[string]string{"dec-001": "Accepted decision"},
	}

	output := present.StatusResponse(data)

	for _, want := range []string{
		"**In progress problem (diagnosis)** `prob-progress` → **Progress portfolio** `sol-001`",
		"**Backlog problem (search)** `prob-backlog`",
		"**Addressed problem** `prob-addressed` → **Accepted decision** `dec-001`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestProblemsListResponse_ShowsProblemTypeInHeading(t *testing.T) {
	fields, err := json.Marshal(artifact.ProblemFields{ProblemType: artifact.ProblemTypeSynthesis})
	if err != nil {
		t.Fatal(err)
	}

	output := present.ProblemsListResponse([]artifact.ProblemListItem{
		{
			Problem: &artifact.Artifact{
				Meta: artifact.Meta{
					ID:        "prob-001",
					Title:     "Design the deployment path",
					CreatedAt: mustParseTime(t, "2026-04-14T00:00:00Z"),
				},
				StructuredData: string(fields),
			},
		},
	}, "")

	if !strings.Contains(output, "### 1. Design the deployment path (synthesis) [prob-001]") {
		t.Fatalf("expected problem type in heading, got:\n%s", output)
	}
}

func TestGovernanceAttentionResponse_ShowsOrphansAndInvariantViolations(t *testing.T) {
	output := present.GovernanceAttentionResponse(artifact.GovernanceAttention{
		BacklogCount:    2,
		InProgressCount: 1,
		AddressedWithoutDecision: []artifact.AddressedProblemGap{
			{ProblemID: "prob-001", Title: "Orphan problem"},
		},
		InvariantViolations: []artifact.InvariantViolationFinding{
			{
				DecisionID:    "dec-001",
				DecisionTitle: "Boundary decision",
				Invariant:     "no dependency from api to database",
				Reason:        "Forbidden dependency detected: internal/api → internal/database",
			},
		},
	})

	for _, want := range []string{
		"Problems: 2 backlog, 1 in progress",
		"Addressed without linked decision (1)",
		"**Orphan problem** `prob-001`",
		"Invariant violations (1)",
		"**Boundary decision** `dec-001` — no dependency from api to database",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("governance attention missing %q:\n%s", want, output)
		}
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}

	return parsed
}

func TestDecisionResponse_PreservesDecisionBodyVerbatim(t *testing.T) {
	body := "# Decision\n\nFix DecisionRecord parser without changing DecisionRecord wording."
	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "dec-001",
			Kind:  artifact.KindDecisionRecord,
			Title: "Fix DecisionRecord parser",
		},
		Body: body,
	}

	response := present.DecisionResponse("decide", a, "", "", "\n-- nav --\n")

	if !strings.Contains(response, "Decision recorded: **Fix DecisionRecord parser** `dec-001`") {
		t.Fatalf("decide response should pair decision title with ref:\n%s", response)
	}
	if strings.Contains(response, "\nID: dec-001\n") {
		t.Fatalf("decide response should not expose a standalone bare decision ID:\n%s", response)
	}
	if !strings.Contains(response, body) {
		t.Fatalf("expected decision body to stay verbatim, got:\n%s", response)
	}
}

func TestNoteResponse_PairsTitleWithRef(t *testing.T) {
	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "note-001",
			Kind:  artifact.KindNote,
			Title: "Operator transparency invariant",
		},
	}

	response := present.NoteResponse(a, "/tmp/note.md", artifact.NoteValidation{OK: true}, "\n-- nav --\n")

	if !strings.Contains(response, "Recorded: **Operator transparency invariant** `note-001`") {
		t.Fatalf("note response should pair note title with ref:\n%s", response)
	}
	if strings.Contains(response, "\nID: note-001\n") {
		t.Fatalf("note response should not expose a standalone bare note ID:\n%s", response)
	}
}

func TestBaselineResponse_PairsDecisionTitleWithRef(t *testing.T) {
	response := present.BaselineResponse(
		"Operator transparency decision",
		"dec-001",
		[]artifact.AffectedFile{{Path: "internal/present/format.go", Hash: "1234567890abcdef"}},
		"\n-- nav --\n",
	)

	if !strings.Contains(response, "Baseline set for **Operator transparency decision** `dec-001`") {
		t.Fatalf("baseline response should pair decision title with ref:\n%s", response)
	}
	if strings.Contains(response, "Baseline set for dec-001") {
		t.Fatalf("baseline response should not expose a bare decision ref:\n%s", response)
	}
}

func TestProjectionResponse_RendersAudienceViewsFromSameGraph(t *testing.T) {
	graph := artifact.ProjectionGraph{
		Problems: []artifact.ProblemProjection{
			{
				Meta: artifact.Meta{
					ID:     "prob-001",
					Title:  "Transport choice",
					Mode:   artifact.ModeStandard,
					Status: artifact.StatusActive,
				},
				Signal:              "Latency variance between protocols",
				Acceptance:          "Choose the transport with the best latency trade-off",
				OptimizationTargets: []string{"latency", "operational cost"},
				PortfolioRefs:       []string{"sol-001"},
				DecisionRefs:        []string{"dec-001"},
				Evidence: artifact.ProjectionEvidenceSummary{
					WLNK: artifact.WLNKSummary{Summary: "no evidence attached"},
				},
			},
		},
		Portfolios: []artifact.PortfolioProjection{
			{
				Meta: artifact.Meta{
					ID:     "sol-001",
					Title:  "Solutions for: Transport choice",
					Mode:   artifact.ModeStandard,
					Status: artifact.StatusActive,
				},
				ProblemRefs:  []string{"prob-001"},
				DecisionRefs: []string{"dec-001"},
				Variants: []artifact.Variant{
					{ID: "V1", Title: "REST"},
					{ID: "V2", Title: "gRPC"},
				},
				Comparison: &artifact.ComparisonResult{
					NonDominatedSet: []string{"V2"},
					DominatedVariants: []artifact.DominatedVariantExplanation{
						{
							Variant:     "V1",
							DominatedBy: []string{"V2"},
							Summary:     "Higher latency with no compensating cost advantage.",
						},
					},
					ParetoTradeoffs: []artifact.ParetoTradeoffNote{
						{Variant: "V2", Summary: "Best latency, but more tooling overhead than REST."},
					},
					PolicyApplied:           "Minimize latency within the accepted cost envelope.",
					SelectedRef:             "V2",
					RecommendationRationale: "It keeps latency low while staying inside the current budget tolerance.",
				},
				Evidence: artifact.ProjectionEvidenceSummary{
					WLNK: artifact.WLNKSummary{Summary: "no evidence attached"},
				},
			},
		},
		Decisions: []artifact.DecisionProjection{
			{
				Meta: artifact.Meta{
					ID:         "dec-001",
					Title:      "gRPC",
					Mode:       artifact.ModeStandard,
					Status:     artifact.StatusActive,
					ValidUntil: "2026-12-31T00:00:00Z",
				},
				ProblemRefs:     []string{"prob-001"},
				PortfolioRefs:   []string{"sol-001"},
				AffectedFiles:   []string{"internal/transport/contracts.proto", "internal/transport/grpc.go"},
				SelectedTitle:   "gRPC",
				WhySelected:     "It meets the latency target with acceptable operating cost.",
				SelectionPolicy: "Minimize latency within the accepted cost envelope.",
				CounterArgument: "Tooling and local debugging remain weaker than the simpler HTTP baseline.",
				WeakestLink:     "Operational confidence still depends on limited production-grade evidence.",
				WhyNotOthers: []artifact.RejectionReason{
					{
						Variant: "REST",
						Reason:  "Higher latency with no compensating cost advantage.",
					},
				},
				Invariants:    []string{"p99 latency remains below 50ms during cutover"},
				Admissibility: []string{"No silent message loss during protocol migration"},
				Predictions: []artifact.DecisionPrediction{
					{
						Claim:      "Latency stays under 50ms",
						Observable: "publish latency p99",
						Threshold:  "< 50ms",
						Status:     artifact.ClaimStatusSupported,
					},
					{
						Claim:      "Throughput stays above 100k events/sec",
						Observable: "throughput",
						Threshold:  "> 100k events/sec",
						Status:     artifact.ClaimStatusInconclusive,
					},
				},
				RollbackTriggers: []string{"Error budget exceeds 2% during canary"},
				Measured:         true,
				Evidence: artifact.ProjectionEvidenceSummary{
					MeasurementCount:   1,
					MeasurementVerdict: "partial",
					WLNK: artifact.WLNKSummary{
						Summary:             "R_eff=0.60 · 2 evidence item(s) · 1 supporting · 1 weakening",
						REff:                0.60,
						FEff:                2,
						FormalityScaleID:    "haft-legacy-f0-f3",
						FormalityBridgeLoss: "legacy-scale-has-fewer-buckets",
						WeakestCL:           1,
						MinFreshness:        "2026-07-01T00:00:00Z",
						CoverageGaps:        []string{"operational-cost"},
					},
				},
			},
		},
	}

	cases := []struct {
		view  artifact.ProjectionView
		wants []string
	}{
		{
			view: artifact.ProjectionViewEngineer,
			wants: []string{
				"## Engineer View",
				"Signal: Latency variance between protocols",
				"Portfolios: **Solutions for: Transport choice** `sol-001`",
				"Decisions: **gRPC** `dec-001`",
				"Selected: gRPC",
				"Predictions:",
				"supported: Latency stays under 50ms (observable: publish latency p99; threshold: < 50ms)",
			},
		},
		{
			view: artifact.ProjectionViewManager,
			wants: []string{
				"## Manager/Status View",
				"Problems: 0 backlog, 0 in progress, 1 addressed",
				"Decisions: 0 unassessed, 0 pending follow-through, 1 measured/shipped, 0 refresh due",
			},
		},
		{
			view: artifact.ProjectionViewAudit,
			wants: []string{
				"## Audit/Evidence View",
				"Selection policy: Minimize latency within the accepted cost envelope.",
				"inconclusive: Throughput stays above 100k events/sec (observable: throughput; threshold: > 100k events/sec)",
				"Coverage gaps: operational-cost",
				"Assurance: R_eff=0.60 | F_eff=2 scale=haft-legacy-f0-f3 bridge_loss=legacy-scale-has-fewer-buckets | weakest CL=1",
				"Assurance boundary: evidence/formality are diagnostics, not approval, gate passage, claim truth, global truth, or publication.",
			},
		},
		{
			view: artifact.ProjectionViewCompare,
			wants: []string{
				"## Compare/Pareto View",
				"Problems: **Transport choice** `prob-001`",
				"Decisions: **gRPC** `dec-001`",
				"Computed Pareto front: gRPC `V2`",
				"Dominated variant elimination:",
				"Recommendation (advisory): gRPC `V2`",
			},
		},
		{
			view: artifact.ProjectionViewDelegatedAgent,
			wants: []string{
				"## Delegated-Agent Brief",
				"Selected decision: gRPC `dec-001`",
				"Affected files: internal/transport/contracts.proto, internal/transport/grpc.go",
				"Invariants: p99 latency remains below 50ms during cutover",
				"Admissibility: No silent message loss during protocol migration",
				"Rollback triggers: Error budget exceeds 2% during canary",
				"Open claim risks:",
				"weakest link: Operational confidence still depends on limited production-grade evidence.",
				"inconclusive: Throughput stays above 100k events/sec (observable: throughput; threshold: > 100k events/sec)",
			},
		},
		{
			view: artifact.ProjectionViewChangeRationale,
			wants: []string{
				"## PR/Change Rationale",
				"Selected change: gRPC `dec-001`",
				"Problem signal: Latency variance between protocols",
				"Selected variant: gRPC",
				"Why selected: It meets the latency target with acceptable operating cost.",
				"Rejected alternatives:",
				"- REST: Higher latency with no compensating cost advantage.",
				"Rollback summary: Error budget exceeds 2% during canary",
				"Latest measurement verdict: partial",
			},
		},
	}

	for _, tc := range cases {
		output := present.ProjectionResponse(graph, tc.view)
		for _, want := range tc.wants {
			if !strings.Contains(output, want) {
				t.Fatalf("view %s missing %q:\n%s", tc.view, want, output)
			}
		}
	}
}

func TestProjectionResponse_ChangesWhenPredictionStatusChanges(t *testing.T) {
	graph := artifact.ProjectionGraph{
		Decisions: []artifact.DecisionProjection{
			{
				Meta: artifact.Meta{
					ID:    "dec-001",
					Title: "gRPC",
				},
				Predictions: []artifact.DecisionPrediction{
					{
						Claim:      "Latency stays under 50ms",
						Observable: "publish latency p99",
						Threshold:  "< 50ms",
						Status:     artifact.ClaimStatusUnverified,
					},
				},
			},
		},
	}

	engineerBefore := present.ProjectionResponse(graph, artifact.ProjectionViewEngineer)
	auditBefore := present.ProjectionResponse(graph, artifact.ProjectionViewAudit)

	graph.Decisions[0].Predictions[0].Status = artifact.ClaimStatusSupported

	engineerAfter := present.ProjectionResponse(graph, artifact.ProjectionViewEngineer)
	auditAfter := present.ProjectionResponse(graph, artifact.ProjectionViewAudit)

	if !strings.Contains(engineerBefore, "unverified: Latency stays under 50ms") {
		t.Fatalf("expected engineer projection to show initial prediction status, got:\n%s", engineerBefore)
	}
	if !strings.Contains(auditBefore, "unverified: Latency stays under 50ms") {
		t.Fatalf("expected audit projection to show initial prediction status, got:\n%s", auditBefore)
	}
	if !strings.Contains(engineerAfter, "supported: Latency stays under 50ms") {
		t.Fatalf("expected engineer projection to show updated prediction status, got:\n%s", engineerAfter)
	}
	if !strings.Contains(auditAfter, "supported: Latency stays under 50ms") {
		t.Fatalf("expected audit projection to show updated prediction status, got:\n%s", auditAfter)
	}
	if engineerBefore == engineerAfter {
		t.Fatalf("expected engineer projection output to change after status update")
	}
	if auditBefore == auditAfter {
		t.Fatalf("expected audit projection output to change after status update")
	}
}

func TestProjectionResponse_ManagerStatusSeparatesUnassessedDecisions(t *testing.T) {
	graph := artifact.ProjectionGraph{
		Decisions: []artifact.DecisionProjection{
			{
				Meta: artifact.Meta{
					ID:    "dec-unassessed",
					Title: "Unassessed decision",
				},
				Health: artifact.DecisionHealth{
					Maturity: artifact.DecisionMaturityUnassessed,
				},
			},
			{
				Meta: artifact.Meta{
					ID:    "dec-pending",
					Title: "Pending decision",
				},
				Health: artifact.DecisionHealth{
					Maturity: artifact.DecisionMaturityPending,
				},
			},
			{
				Meta: artifact.Meta{
					ID:    "dec-shipped",
					Title: "Shipped decision",
				},
				NeedsRefresh: true,
				Health: artifact.DecisionHealth{
					Maturity: artifact.DecisionMaturityShipped,
				},
			},
		},
	}

	output := present.ProjectionResponse(graph, artifact.ProjectionViewManager)

	for _, want := range []string{
		"Decisions: 1 unassessed, 1 pending follow-through, 1 measured/shipped, 1 refresh due",
		"- **Unassessed decision** `dec-unassessed` — unassessed",
		"- **Pending decision** `dec-pending` — waiting for measurement",
		"- **Shipped decision** `dec-shipped` — measured, refresh due",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("manager projection missing %q:\n%s", want, output)
		}
	}
}

func TestProjectionResponse_DelegatedBriefKeepsSupportedClaimsOutOfOpenRiskList(t *testing.T) {
	graph := artifact.ProjectionGraph{
		Decisions: []artifact.DecisionProjection{
			{
				Meta: artifact.Meta{
					ID:    "dec-001",
					Title: "gRPC",
				},
				SelectedTitle: "gRPC",
				WeakestLink:   "Operational confidence still depends on limited production-grade evidence.",
				Predictions: []artifact.DecisionPrediction{
					{
						Claim:      "Latency stays under 50ms",
						Observable: "publish latency p99",
						Threshold:  "< 50ms",
						Status:     artifact.ClaimStatusSupported,
					},
					{
						Claim:      "Throughput stays above 100k events/sec",
						Observable: "throughput",
						Threshold:  "> 100k events/sec",
						Status:     artifact.ClaimStatusRefuted,
					},
				},
			},
		},
	}

	output := present.ProjectionResponse(graph, artifact.ProjectionViewDelegatedAgent)

	if strings.Contains(output, "supported: Latency stays under 50ms") {
		t.Fatalf("expected supported claim to stay out of open risk list, got:\n%s", output)
	}

	required := []string{
		"weakest link: Operational confidence still depends on limited production-grade evidence.",
		"refuted: Throughput stays above 100k events/sec (observable: throughput; threshold: > 100k events/sec)",
	}
	for _, want := range required {
		if !strings.Contains(output, want) {
			t.Fatalf("expected delegated brief to contain %q, got:\n%s", want, output)
		}
	}
}
