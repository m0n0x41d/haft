package artifact

import (
	"context"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestDecide_FullDRR(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Set up problem and portfolio
	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Event infrastructure", Signal: "DB polling at 70% CPU", Context: "events",
		Constraints: []string{"Must maintain <500ms p99"},
		Acceptance:  "All producers on new infra, p99 < 50ms",
	})
	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("Kafka", "ops complexity", "Max throughput with established broker operations"),
			testVariant("NATS JetStream", "ecosystem maturity", "Lean embedded broker with simpler cluster operations"),
		},
		NoSteppingStoneRationale: "Both candidates are production-target options rather than exploratory stepping stones.",
	})

	input := DecideInput{
		ProblemRef:      prob.Meta.ID,
		PortfolioRef:    portfolio.Meta.ID,
		SelectedTitle:   "NATS JetStream",
		WhySelected:     "2x throughput headroom, minimal ops for 4-person team",
		SelectionPolicy: "Prefer the variant with enough throughput headroom that still minimizes operational load for the four-person team.",
		CounterArgument: "Lower ecosystem maturity could leave the team exposed when traffic exceeds the current forecast.",
		WhyNotOthers: []RejectionReason{
			{Variant: "Kafka", Reason: "Ops burden disproportionate at current scale"},
		},
		Invariants:    []string{"Every event delivered at-least-once", "Ordering preserved per stream"},
		PreConditions: []string{"NATS cluster provisioned in staging", "Load test harness ready"},
		PostConditions: []string{
			"All 12 producers migrated",
			"Load test passed: 100k events/sec, p99 < 50ms",
			"DB polling path alive as hot standby for 30 days",
		},
		Admissibility:   []string{"Fire-and-forget delivery", "Single-node production deployment"},
		EvidenceReqs:    []string{"Load test at 100k events/sec", "Producer error rate < 0.1%"},
		RefreshTriggers: []string{"Throughput exceeds 80k/s sustained", "NATS major CVE"},
		WeakestLink:     "Ecosystem maturity — fewer case studies at >50k events/sec",
		ValidUntil:      "2026-09-16T00:00:00Z",
		Rollback: &RollbackSpec{
			Triggers:    []string{"Producer error rate > 1% for > 5 minutes"},
			Steps:       []string{"Feature flag: route events back to DB polling", "Drain NATS queues"},
			BlastRadius: "All 12 services see temporary dual-delivery",
		},
		AffectedFiles:      []string{"internal/events/producer.go", "internal/events/consumer.go"},
		DecisionSubjectRef: "subject:event-infrastructure",
		ImplementationFootprint: ImplementationFootprint{
			Files:    []string{"internal/events/producer.go"},
			WorkRefs: []string{"wc-event-migration"},
		},
		GovernanceTargets: []GovernanceTarget{{
			Kind: "api_contract",
			Ref:  "events/producer-contract",
		}},
		DriftWatchTargets: []DriftWatchTarget{{
			TargetRef: "events/producer-contract",
			Trigger:   "schema_or_behavior_changed",
		}},
	}

	a, filePath, err := Decide(ctx, store, haftDir, input)
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.Kind != KindDecisionRecord {
		t.Errorf("kind = %q", a.Meta.Kind)
	}
	if a.Meta.Context != "events" {
		t.Errorf("context = %q, want events (inherited)", a.Meta.Context)
	}
	if filePath == "" {
		t.Error("file path should not be empty")
	}

	// Check FPF E.9 four-component structure
	requiredSections := []string{
		"## 1. Problem Frame",
		"## 2. Decision",
		"## 3. Rationale",
		"## 4. Consequences",
	}
	for _, section := range requiredSections {
		if !strings.Contains(a.Body, section) {
			t.Errorf("missing FPF E.9 component: %s", section)
		}
	}

	// Check Problem Frame pulled from ProblemCard
	if !strings.Contains(a.Body, "DB polling at 70% CPU") {
		t.Error("Problem Frame should contain signal from ProblemCard")
	}
	if !strings.Contains(a.Body, "500ms p99") {
		t.Error("Problem Frame should contain constraints from ProblemCard")
	}

	// Check Decision contract
	if !strings.Contains(a.Body, "NATS JetStream") {
		t.Error("missing selected variant name")
	}
	if !strings.Contains(a.Body, "Selection policy") {
		t.Error("missing selection policy")
	}
	if !strings.Contains(a.Body, "at-least-once") {
		t.Error("missing invariant content")
	}
	if !strings.Contains(a.Body, "NOT: Fire-and-forget") {
		t.Error("missing admissibility content")
	}
	if !strings.Contains(a.Body, "- [ ] All 12 producers migrated") {
		t.Error("missing post-condition checklist")
	}

	// Check Rationale
	if !strings.Contains(a.Body, "Kafka") && !strings.Contains(a.Body, "Rejected") {
		t.Error("missing rejection rationale")
	}
	if !strings.Contains(a.Body, "Counterargument") {
		t.Error("missing counterargument")
	}
	if !strings.Contains(a.Body, "Ecosystem maturity") {
		t.Error("missing weakest link")
	}

	// Check Consequences
	if !strings.Contains(a.Body, "Rollback") {
		t.Error("missing rollback plan")
	}
	if !strings.Contains(a.Body, "Refresh triggers") {
		t.Error("missing refresh triggers")
	}
	if !strings.Contains(a.Body, "producer.go") {
		t.Error("missing affected files")
	}

	// Check links
	links, _ := store.GetLinks(ctx, a.Meta.ID)
	if len(links) != 2 {
		t.Errorf("expected 2 links (problem + portfolio), got %d", len(links))
	}

	fields := a.UnmarshalDecisionFields()
	if !reflect.DeepEqual(fields.ProblemRefs, []string{prob.Meta.ID}) {
		t.Fatalf("problem refs in structured state = %#v, want [%q]", fields.ProblemRefs, prob.Meta.ID)
	}
	if fields.ChoiceResult == nil {
		t.Fatal("expected h-decide to persist exact choice_result")
	}
	if fields.ChoiceResult.NextMove != ChoiceNextMoveChooseNow {
		t.Fatalf("choice_result.next_move = %q, want %q", fields.ChoiceResult.NextMove, ChoiceNextMoveChooseNow)
	}
	if fields.ChoiceResult.SubjectRef != "operator" {
		t.Fatalf("choice_result.subject_ref = %q, want operator", fields.ChoiceResult.SubjectRef)
	}
	if !reflect.DeepEqual(fields.ChoiceResult.OptionSet, []string{"NATS JetStream", "Kafka"}) {
		t.Fatalf("choice_result.option_set = %#v, want selected plus rejected variants", fields.ChoiceResult.OptionSet)
	}
	if fields.ChoiceResult.ChoiceRule != input.SelectionPolicy {
		t.Fatalf("choice_result.choice_rule = %q, want selection policy", fields.ChoiceResult.ChoiceRule)
	}
	if !stringInSlice(fields.ChoiceResult.ComparisonBasis, "selected NATS JetStream: 2x throughput headroom, minimal ops for 4-person team") {
		t.Fatalf("choice_result.comparison_basis missing selected rationale: %#v", fields.ChoiceResult.ComparisonBasis)
	}
	if !stringInSlice(fields.ChoiceResult.ComparisonBasis, "rejected Kafka: Ops burden disproportionate at current scale") {
		t.Fatalf("choice_result.comparison_basis missing rejected rationale: %#v", fields.ChoiceResult.ComparisonBasis)
	}
	if fields.ChoiceResult.VariantRef != "NATS JetStream" {
		t.Fatalf("choice_result.variant_ref = %q, want selected title", fields.ChoiceResult.VariantRef)
	}
	if !reflect.DeepEqual(fields.ChoiceResult.ProblemRefs, []string{prob.Meta.ID}) {
		t.Fatalf("choice_result.problem_refs = %#v, want [%q]", fields.ChoiceResult.ProblemRefs, prob.Meta.ID)
	}
	if fields.ChoiceResult.PortfolioRef != portfolio.Meta.ID {
		t.Fatalf("choice_result.portfolio_ref = %q, want %q", fields.ChoiceResult.PortfolioRef, portfolio.Meta.ID)
	}
	if fields.ChoiceResult.ReopenCondition != "reopen choice if rollback triggers occur: Producer error rate > 1% for > 5 minutes" {
		t.Fatalf("choice_result.reopen_condition = %q", fields.ChoiceResult.ReopenCondition)
	}
	if fields.SelectionPolicy == "" {
		t.Error("expected selection_policy in structured data")
	}
	if fields.CounterArgument == "" {
		t.Error("expected counterargument in structured data")
	}
	if len(fields.WhyNotOthers) != 1 {
		t.Fatalf("expected one rejected alternative in structured data, got %#v", fields.WhyNotOthers)
	}
	if len(fields.RollbackTriggers) != 1 {
		t.Fatalf("expected rollback trigger in structured data, got %#v", fields.RollbackTriggers)
	}
	if !reflect.DeepEqual(fields.PreConditions, input.PreConditions) {
		t.Fatalf("pre-conditions in structured state = %#v, want %#v", fields.PreConditions, input.PreConditions)
	}
	if !reflect.DeepEqual(fields.EvidenceRequirements, input.EvidenceReqs) {
		t.Fatalf("evidence requirements in structured state = %#v, want %#v", fields.EvidenceRequirements, input.EvidenceReqs)
	}
	if !reflect.DeepEqual(fields.RefreshTriggers, input.RefreshTriggers) {
		t.Fatalf("refresh triggers in structured state = %#v, want %#v", fields.RefreshTriggers, input.RefreshTriggers)
	}
	if fields.DecisionSubjectRef != input.DecisionSubjectRef {
		t.Fatalf("decision_subject_ref = %q, want %q", fields.DecisionSubjectRef, input.DecisionSubjectRef)
	}
	if !reflect.DeepEqual(fields.ImplementationFootprint.Files, input.ImplementationFootprint.Files) {
		t.Fatalf("implementation_footprint.files = %#v, want %#v", fields.ImplementationFootprint.Files, input.ImplementationFootprint.Files)
	}
	if len(fields.GovernanceTargets) != 1 || fields.GovernanceTargets[0].Ref != "events/producer-contract" {
		t.Fatalf("governance_targets = %#v", fields.GovernanceTargets)
	}
	if len(fields.DriftWatchTargets) != 1 || fields.DriftWatchTargets[0].Trigger != "schema_or_behavior_changed" {
		t.Fatalf("drift_watch_targets = %#v", fields.DriftWatchTargets)
	}

	reloaded, err := store.Get(ctx, a.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	reloadedFields := reloaded.UnmarshalDecisionFields()
	if !reflect.DeepEqual(reloadedFields.ProblemRefs, fields.ProblemRefs) {
		t.Fatalf("reloaded problem refs = %#v, want %#v", reloadedFields.ProblemRefs, fields.ProblemRefs)
	}
	if !reflect.DeepEqual(reloadedFields.ChoiceResult, fields.ChoiceResult) {
		t.Fatalf("reloaded choice_result = %#v, want %#v", reloadedFields.ChoiceResult, fields.ChoiceResult)
	}
	if !reflect.DeepEqual(reloadedFields.PreConditions, fields.PreConditions) {
		t.Fatalf("reloaded pre-conditions = %#v, want %#v", reloadedFields.PreConditions, fields.PreConditions)
	}
	if !reflect.DeepEqual(reloadedFields.EvidenceRequirements, fields.EvidenceRequirements) {
		t.Fatalf("reloaded evidence requirements = %#v, want %#v", reloadedFields.EvidenceRequirements, fields.EvidenceRequirements)
	}
	if !reflect.DeepEqual(reloadedFields.RefreshTriggers, fields.RefreshTriggers) {
		t.Fatalf("reloaded refresh triggers = %#v, want %#v", reloadedFields.RefreshTriggers, fields.RefreshTriggers)
	}
	if !reflect.DeepEqual(reloadedFields.ImplementationFootprint, fields.ImplementationFootprint) {
		t.Fatalf("reloaded implementation footprint = %#v, want %#v", reloadedFields.ImplementationFootprint, fields.ImplementationFootprint)
	}
	if !reflect.DeepEqual(reloadedFields.GovernanceTargets, fields.GovernanceTargets) {
		t.Fatalf("reloaded governance targets = %#v, want %#v", reloadedFields.GovernanceTargets, fields.GovernanceTargets)
	}
	if !reflect.DeepEqual(reloadedFields.DriftWatchTargets, fields.DriftWatchTargets) {
		t.Fatalf("reloaded drift watch targets = %#v, want %#v", reloadedFields.DriftWatchTargets, fields.DriftWatchTargets)
	}

	// Check affected files in DB
	files, _ := store.GetAffectedFiles(ctx, a.Meta.ID)
	if len(files) != 2 {
		t.Errorf("expected 2 affected files, got %d", len(files))
	}
}

func TestDecide_DirectProblemStatementRendersWithoutProblemRefs(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	problemStatement := "The compiled PatternUse router hides source provenance and imposes a project order that FPF does not prescribe."

	input := completeDecision(DecideInput{
		ProblemStatement: problemStatement,
		SelectedTitle:    "Use source-native FPF Query",
		WhySelected:      "The agent can inspect author-owned navigation layers without a shadow ontology.",
		SelectionPolicy:  "Prefer source provenance and no hidden pattern selection.",
		WhyNotOthers: []RejectionReason{{
			Variant: "Keep compiled PatternUse",
			Reason:  "Its authored routes can drift from the FPF source.",
		}},
		ChoiceResult: &ChoiceResult{
			SubjectRef: "operator",
			OptionSet:  []string{"Use source-native FPF Query", "Keep compiled PatternUse"},
			ChoiceRule: "Prefer source provenance and no hidden pattern selection.",
			NextMove:   ChoiceNextMoveChooseNow,
			VariantRef: "Use source-native FPF Query",
			Reason:     "The agent can inspect author-owned navigation layers without a shadow ontology.",
		},
	})

	decision, _, err := Decide(ctx, store, t.TempDir(), input)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(decision.Body, "**Problem statement:** "+problemStatement) {
		t.Fatalf("direct decision did not render its inline Problem Frame:\n%s", decision.Body)
	}

	fields := decision.UnmarshalDecisionFields()
	if fields.ProblemStatement != problemStatement {
		t.Fatalf("problem_statement = %q, want %q", fields.ProblemStatement, problemStatement)
	}
	if len(fields.ProblemRefs) != 0 {
		t.Fatalf("direct decision invented problem refs: %#v", fields.ProblemRefs)
	}
	if fields.ChoiceResult == nil || !reflect.DeepEqual(fields.ChoiceResult.OptionSet, input.ChoiceResult.OptionSet) {
		t.Fatalf("choice_result.option_set = %#v, want %#v", fields.ChoiceResult, input.ChoiceResult.OptionSet)
	}
}

func TestDecide_RejectsMissingProblemBasis(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	input := completeDecision(DecideInput{
		SelectedTitle: "Use source-native FPF Query",
		WhySelected:   "The source-native path removes the shadow router.",
	})
	input.ProblemStatement = ""

	_, _, err := Decide(ctx, store, t.TempDir(), input)
	if err == nil {
		t.Fatal("expected a decision without problem refs or problem_statement to fail")
	}
	if !strings.Contains(err.Error(), "problem_statement") {
		t.Fatalf("missing direct problem-basis guidance in error: %v", err)
	}
	if !strings.Contains(err.Error(), "problem_ref/problem_refs") {
		t.Fatalf("error does not explain the linked ProblemCard alternative: %v", err)
	}
}

func TestDecide_RejectsChoiceCorrelationDriftBeforePersistence(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	input := completeDecision(DecideInput{
		ProblemStatement: "Duplicated decision fields can disagree before projection.",
		SelectedTitle:    "Keep one canonical choice",
		WhySelected:      "The persisted decision and its choice projection must agree.",
		SelectionPolicy:  "Prefer one canonical decision representation.",
		CounterArgument:  "Correlation checks add validation work at the boundary.",
		WeakestLink:      "A new duplicated field could escape the correlation set.",
		WhyNotOthers:     []RejectionReason{{Variant: "Permit drift", Reason: "Late projection failure leaves partial state."}},
		Rollback:         &RollbackSpec{Triggers: []string{"Canonical projection no longer round-trips."}},
		ChoiceResult: &ChoiceResult{
			SubjectRef: "operator",
			OptionSet: []string{
				"Keep one canonical choice",
				"Permit drift",
			},
			ChoiceRule: "A contradictory rule that must be rejected.",
			NextMove:   ChoiceNextMoveChooseNow,
			VariantRef: "Keep one canonical choice",
			Reason:     "The persisted decision and its choice projection must agree.",
		},
	})

	decision, filePath, err := Decide(ctx, store, haftDir, input)
	if err == nil {
		t.Fatal("expected contradictory choice fields to fail")
	}
	if decision != nil || filePath != "" {
		t.Fatalf("failed decision returned artifact=%#v file=%q", decision, filePath)
	}
	if !strings.Contains(
		err.Error(),
		"choice_result.choice_rule must equal selection_policy",
	) {
		t.Fatalf("error = %v, want choice correlation diagnostic", err)
	}

	decisions, listErr := store.ListByKind(
		ctx,
		KindDecisionRecord,
		10,
	)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(decisions) != 0 {
		t.Fatalf(
			"invalid decision persisted %d DecisionRecord(s)",
			len(decisions),
		)
	}
}

func TestDecide_LinkedProblemCardRemainsValidWithoutInlineStatement(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	problem, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:      "Source provenance is hidden",
		Signal:     "Compiled routes cannot show the exact author source that justified a candidate.",
		Acceptance: "A decision can still bind directly to a persisted ProblemCard.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		SelectedTitle: "Use source-native FPF Query",
		WhySelected:   "The source-native path preserves author provenance.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(decision.Body, "Compiled routes cannot show the exact author source") {
		t.Fatalf("linked ProblemCard no longer renders in the Problem Frame:\n%s", decision.Body)
	}
	if fields := decision.UnmarshalDecisionFields(); fields.ProblemStatement != "" {
		t.Fatalf("linked legacy-style decision invented problem_statement %q", fields.ProblemStatement)
	}
}

func TestDecide_PortfolioResolvedProblemCardSuppliesProblemBasis(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	problem, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:  "Compiled routes drift",
		Signal: "The route catalog can disagree with the author-owned FPF source.",
	})
	if err != nil {
		t.Fatal(err)
	}
	portfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("Source-native query", "publication grammar changes", "Keeps provenance on every candidate"),
			testVariant("Compiled routes", "semantic drift", "Keeps the existing interface"),
		},
		NoSteppingStoneRationale: "Both variants are complete replacement choices.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		PortfolioRef:  portfolio.Meta.ID,
		SelectedTitle: "Source-native query",
		WhySelected:   "Every retrieval candidate retains its author source.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(decision.Body, "The route catalog can disagree with the author-owned FPF source") {
		t.Fatalf("portfolio-derived ProblemCard was not rendered:\n%s", decision.Body)
	}
	if strings.Contains(decision.Body, "**Problem statement:** \n") {
		t.Fatalf("portfolio-derived decision rendered an empty inline Problem Frame:\n%s", decision.Body)
	}
}

func TestDecideEnrichesAffectedFilesWithPreciseBindingTargets(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	writeTestFile(t, projectRoot, "worker.go", "package main\n\nfunc Run() string { return \"ok\" }\n")

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Use worker Run",
		WhySelected:   "The worker entry point is the governed code object.",
		AffectedFiles: []string{"worker.go"},
	}))
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if len(fields.BindingTargets) != 1 {
		t.Fatalf("binding_targets = %+v, want one precise target", fields.BindingTargets)
	}
	target := fields.BindingTargets[0]
	if target.Kind != BindingTargetSymbol || target.SymbolName != "Run" || target.BodyHash == "" {
		t.Fatalf("binding target = %+v, want Run symbol with body_hash", target)
	}
	if target.ResolutionSource != BindingResolutionSourceSingleSymbolFile {
		t.Fatalf("resolution_source = %q, want %q", target.ResolutionSource, BindingResolutionSourceSingleSymbolFile)
	}

	symbols, err := store.GetAffectedSymbols(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 0 {
		t.Fatalf("affected symbols = %+v, want no baseline symbol mutation during Decide", symbols)
	}
}

func TestDecideDoesNotInventFallbackForAmbiguousAffectedFile(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	writeTestFile(t, projectRoot, "worker.go", `package main

func Start() string { return "start" }
func Stop() string { return "stop" }
`)

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Use worker file",
		WhySelected:   "The decision names the file but not a specific governed symbol.",
		AffectedFiles: []string{"worker.go"},
	}))
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if len(fields.BindingTargets) != 0 {
		t.Fatalf("binding_targets = %+v, want no invented fallback for ambiguous file", fields.BindingTargets)
	}
	if !reflect.DeepEqual(fields.ImplementationFootprint.Files, []string{"worker.go"}) {
		t.Fatalf("implementation_footprint.files = %+v, want worker.go", fields.ImplementationFootprint.Files)
	}

	writeTestFile(t, projectRoot, "worker.go", `package main

func Start() string { return "changed" }
func Stop() string { return "stop" }
`)
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("drift reports = %+v, want footprint-only decision skipped", reports)
	}
}

func TestBaselineRejectsFootprintOnlyDecisionWithoutExplicitAuthority(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	writeTestFile(t, projectRoot, "worker.go", `package main

func Start() string { return "start" }
func Stop() string { return "stop" }
`)

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Use worker file",
		WhySelected:   "The decision names the implementation footprint but not a governed symbol.",
		AffectedFiles: []string{"worker.go"},
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: decision.Meta.ID})
	if err == nil {
		t.Fatal("expected footprint-only baseline without explicit authority to fail closed")
	}
	if !strings.Contains(err.Error(), "implementation_footprint only") {
		t.Fatalf("error = %q, want implementation_footprint repair hint", err.Error())
	}
}

func TestBaselineAllowsExplicitWholeFileAuthorityForFootprintOnlyDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	writeTestFile(t, projectRoot, "worker.go", `package main

func Start() string { return "start" }
func Stop() string { return "stop" }
`)

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Use worker file",
		WhySelected:   "The decision starts as implementation footprint until explicit authority is supplied.",
		AffectedFiles: []string{"worker.go"},
	}))
	if err != nil {
		t.Fatal(err)
	}

	files, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:           decision.Meta.ID,
		BindingScope:          BindingScopeWholeFile,
		BindingFallbackReason: "operator explicitly chose whole-file governance for this legacy file",
	})
	if err != nil {
		t.Fatalf("Baseline: %v", err)
	}
	if len(files) != 1 || files[0].Path != "worker.go" || files[0].Hash == "" {
		t.Fatalf("baseline files = %+v, want worker.go hash", files)
	}
}

func TestDecidePreservesExplicitBindingTargets(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	writeTestFile(t, projectRoot, "worker.go", `package main

func Start() string { return "start" }
func Stop() string { return "stop" }
`)
	explicit := BindingTarget{
		Kind:             BindingTargetSymbol,
		FilePath:         "worker.go",
		SymbolName:       "Stop",
		SymbolKind:       "func",
		BodyHash:         "explicit-hash",
		ResolutionSource: BindingResolutionSourceExplicitTargets,
	}

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle:      "Use explicit Stop",
		WhySelected:        "The operator selected the exact governed symbol.",
		AffectedFiles:      []string{"worker.go"},
		DecisionSubjectRef: "subject:worker-stop",
		BindingTargets:     []BindingTarget{explicit},
		BindingHints:       []string{"Start"},
		BindingScope:       BindingScopeAuto,
	}))
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if len(fields.BindingTargets) != 1 {
		t.Fatalf("binding_targets = %+v, want explicit target", fields.BindingTargets)
	}
	if fields.BindingTargets[0].SymbolName != "Stop" || fields.BindingTargets[0].BodyHash != "explicit-hash" {
		t.Fatalf("binding target = %+v, want explicit Stop target preserved", fields.BindingTargets[0])
	}
}

func TestValidateChoiceResultRejectsUnknownNextMove(t *testing.T) {
	err := ValidateChoiceResult(&ChoiceResult{
		SubjectRef: "operator",
		NextMove:   ChoiceNextMove("approve_this"),
		VariantRef: "V1",
	})
	if err == nil {
		t.Fatal("expected invalid next_move error")
	}
	if !strings.Contains(err.Error(), "choose_now") {
		t.Fatalf("expected valid next_move values in error, got %v", err)
	}
}

func TestNormalizeChoiceResultPreservesReversibilityAndReopenCondition(t *testing.T) {
	choice := NormalizeChoiceResult(&ChoiceResult{
		SubjectRef:      " operator ",
		NextMove:        ChoiceNextMoveChooseNow,
		VariantRef:      " V1 ",
		Reversibility:   " two-week rollback ",
		ReopenCondition: " reopen if rollback triggers occur ",
	})

	if choice.Reversibility != "two-week rollback" {
		t.Fatalf("reversibility = %q", choice.Reversibility)
	}
	if choice.ReopenCondition != "reopen if rollback triggers occur" {
		t.Fatalf("reopen_condition = %q", choice.ReopenCondition)
	}
}

func TestValidateChoiceResultRequiresVariantForChooseNow(t *testing.T) {
	err := ValidateChoiceResult(&ChoiceResult{
		SubjectRef: "operator",
		NextMove:   ChoiceNextMoveChooseNow,
	})
	if err == nil {
		t.Fatal("expected missing variant_ref error")
	}
	if !strings.Contains(err.Error(), "variant_ref") {
		t.Fatalf("expected variant_ref error, got %v", err)
	}
}

func TestValidateChoiceResultRejectsVariantOutsideOptionSet(t *testing.T) {
	err := ValidateChoiceResult(&ChoiceResult{
		SubjectRef: "operator",
		OptionSet:  []string{"V1", "V2"},
		NextMove:   ChoiceNextMoveChooseNow,
		VariantRef: "V3",
	})
	if err == nil {
		t.Fatal("expected variant outside option_set error")
	}
	if !strings.Contains(err.Error(), "option_set") {
		t.Fatalf("expected option_set error, got %v", err)
	}
}

func TestDecide_PersistsExplicitTransformationRecord(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	record := &TransformationRecord{
		TransformedEntity: "ProblemCard profile",
		InitialState:      "Problem posture is implicit prose.",
		PostState:         "Problem posture is typed as cue/thin/deep with readiness blockers.",
		Relation:          "makes explicit",
		Context:           "semantic-spine ProblemCard slice",
		Window:            "2026-Q3",
		MethodRefs:        []string{" mpull-problem-profile ", ""},
		WorkRefs:          []string{"wc-problem-profile"},
		EvidenceRefs:      []string{"evid-problem-profile"},
		PublicationRefs:   []string{"pub-problem-profile"},
	}

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle:        "Add ProblemCard profile fields",
		WhySelected:          "The target transformation needs to be explicit without implying work or evidence authority.",
		TransformationRecord: record,
	}))
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if fields.TransformationRecord == nil {
		t.Fatal("transformation_record missing")
	}
	if fields.TransformationRecord.SchemaVersion != TransformationRecordSchemaVersion {
		t.Fatalf("schema_version = %d, want %d", fields.TransformationRecord.SchemaVersion, TransformationRecordSchemaVersion)
	}
	if fields.TransformationRecord.TransformedEntity != record.TransformedEntity {
		t.Fatalf("transformed_entity = %q, want %q", fields.TransformationRecord.TransformedEntity, record.TransformedEntity)
	}
	if fields.TransformationRecord.Window != "2026-Q3" {
		t.Fatalf("window = %q, want 2026-Q3", fields.TransformationRecord.Window)
	}
	if !reflect.DeepEqual(fields.TransformationRecord.MethodRefs, []string{"mpull-problem-profile"}) {
		t.Fatalf("method_refs = %#v", fields.TransformationRecord.MethodRefs)
	}
	if !reflect.DeepEqual(fields.TransformationRecord.WorkRefs, []string{"wc-problem-profile"}) {
		t.Fatalf("work_refs = %#v", fields.TransformationRecord.WorkRefs)
	}
	if !reflect.DeepEqual(fields.TransformationRecord.EvidenceRefs, []string{"evid-problem-profile"}) {
		t.Fatalf("evidence_refs = %#v", fields.TransformationRecord.EvidenceRefs)
	}
	if !reflect.DeepEqual(fields.TransformationRecord.PublicationRefs, []string{"pub-problem-profile"}) {
		t.Fatalf("publication_refs = %#v", fields.TransformationRecord.PublicationRefs)
	}
	if !strings.Contains(decision.Body, "Transformation record") {
		t.Fatalf("decision body missing transformation section:\n%s", decision.Body)
	}
	if !strings.Contains(decision.Body, "not method, work authorization, evidence, or publication") {
		t.Fatalf("decision body does not preserve separation boundary:\n%s", decision.Body)
	}
	for _, want := range []string{
		"Window: 2026-Q3",
		"Method refs: mpull-problem-profile",
		"Work refs: wc-problem-profile",
		"Evidence refs: evid-problem-profile",
		"Publication refs: pub-problem-profile",
	} {
		if !strings.Contains(decision.Body, want) {
			t.Fatalf("decision body missing %q:\n%s", want, decision.Body)
		}
	}

	reloaded, err := store.Get(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	reloadedFields := reloaded.UnmarshalDecisionFields()
	if !reflect.DeepEqual(reloadedFields.TransformationRecord, fields.TransformationRecord) {
		t.Fatalf("reloaded transformation_record = %#v, want %#v", reloadedFields.TransformationRecord, fields.TransformationRecord)
	}
}

func TestValidateTransformationRecordRejectsIncompleteRecord(t *testing.T) {
	err := ValidateTransformationRecord(&TransformationRecord{
		TransformedEntity: "DecisionRecord",
		InitialState:      "Legacy aggregate",
		Relation:          "separates",
		Context:           "semantic spine",
	})
	if err == nil {
		t.Fatal("expected incomplete transformation_record error")
	}
	if !strings.Contains(err.Error(), "post_state") {
		t.Fatalf("expected post_state validation error, got %v", err)
	}
}

func TestValidateTransformationRecordRejectsUnsupportedSchemaVersion(t *testing.T) {
	err := ValidateTransformationRecord(&TransformationRecord{
		SchemaVersion:     2,
		TransformedEntity: "DecisionRecord",
		InitialState:      "Legacy aggregate",
		PostState:         "Typed transformation object",
		Relation:          "separates",
		Context:           "semantic spine",
	})
	if err == nil {
		t.Fatal("expected unsupported transformation_record schema error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported schema error, got %v", err)
	}
}

func TestArtifact_UnmarshalDecisionFields_DoesNotInventTransformationRecord(t *testing.T) {
	decision := &Artifact{
		Meta: Meta{ID: "dec-legacy", Kind: KindDecisionRecord, Title: "Legacy decision"},
		StructuredData: `{
			"selected_title":"Legacy decision",
			"why_selected":"Already shipped",
			"post_conditions":["new state exists"]
		}`,
	}

	fields := decision.UnmarshalDecisionFields()
	if fields.TransformationRecord != nil {
		t.Fatalf("legacy decision invented transformation_record: %#v", fields.TransformationRecord)
	}
	if fields.ProblemStatement != "" {
		t.Fatalf("legacy decision invented problem_statement: %q", fields.ProblemStatement)
	}
}

func TestDecide_TaskContextSlugInIDAndFilename(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	a, filePath, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Use gRPC",
		WhySelected:   "Task-scoped decisions should remain navigable without weakening random suffix collision safety.",
		TaskContext:   "Task #4: API/CLI cleanup",
	}))
	if err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^dec-\d{8}-task-4-api-cli-cleanup-[0-9a-f]{8}$`)
	if !pattern.MatchString(a.Meta.ID) {
		t.Fatalf("decision ID = %q, want task-context slug before 8-hex suffix", a.Meta.ID)
	}

	filename := filepath.Base(filePath)
	if filename != a.Meta.ID+".md" {
		t.Fatalf("filename = %q, want %q", filename, a.Meta.ID+".md")
	}

	fields := a.UnmarshalDecisionFields()
	if fields.TaskContext != "task-4-api-cli-cleanup" {
		t.Fatalf("structured task_context = %q, want sanitized slug", fields.TaskContext)
	}
}

func TestDecide_ContextDoesNotChangeDefaultIDFormat(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	a, _, err := Decide(ctx, store, t.TempDir(), completeDecision(DecideInput{
		SelectedTitle: "Use gRPC",
		WhySelected:   "Existing context metadata must remain separate from filename task context.",
		Context:       "transport",
	}))
	if err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^dec-\d{8}-[0-9a-f]{8}$`)
	if !pattern.MatchString(a.Meta.ID) {
		t.Fatalf("decision ID = %q, want default format when task_context is omitted", a.Meta.ID)
	}
}

func TestDecide_PersistsSpecSectionRefsWithoutInvalidArtifactLinks(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	sectionRefs := []string{"TS.checkout.001", "TS.checkout.002"}
	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Cover checkout spec",
		WhySelected:   "Spec-linked decisions must stay traceable to the exact governed sections.",
		SectionRefs:   sectionRefs,
	}))
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if !reflect.DeepEqual(fields.SectionRefs, sectionRefs) {
		t.Fatalf("structured section refs = %#v, want %#v", fields.SectionRefs, sectionRefs)
	}
	if !strings.Contains(decision.Body, "TS.checkout.001") {
		t.Fatalf("decision body missing section refs:\n%s", decision.Body)
	}

	links, err := store.GetLinks(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, link := range links {
		if link.Type == "governs" {
			t.Fatalf("DecisionRecord projected cross-carrier SpecSection ref as artifact link: %#v", link)
		}
	}
}

func TestDecide_SpecBindingPreflightSuppliesSectionRefs(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Bind through preflight",
		WhySelected:   "Spec binding preflight selected the governing section.",
		SpecBindingPreflight: &SpecBindingPreflight{
			State:               SpecBindingStateBoundExisting,
			SelectedSectionRefs: []string{"TS.checkout.001"},
		},
		SpecBindingRequired: true,
	}))
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if !reflect.DeepEqual(fields.SectionRefs, []string{"TS.checkout.001"}) {
		t.Fatalf("section_refs = %#v, want preflight-selected section", fields.SectionRefs)
	}
	if fields.SpecBindingPreflight == nil || fields.SpecBindingPreflight.State != SpecBindingStateBoundExisting {
		t.Fatalf("spec_binding_preflight = %#v, want persisted bound_existing receipt", fields.SpecBindingPreflight)
	}
	if !strings.Contains(decision.Body, "TS.checkout.001") {
		t.Fatalf("decision body missing preflight-supplied section refs:\n%s", decision.Body)
	}
}

func TestDecide_SpecBindingPreflightBlocksInvalidStates(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	for _, state := range []string{
		SpecBindingStateInvalidRefs,
		SpecBindingStateConflict,
		SpecBindingStateAmbiguous,
		SpecBindingStateDraftNeeded,
	} {
		_, _, err := Decide(ctx, store, t.TempDir(), completeDecision(DecideInput{
			SelectedTitle: "Blocked spec binding",
			WhySelected:   "The preflight state should block this DecisionRecord.",
			SpecBindingPreflight: &SpecBindingPreflight{
				State:                  state,
				OperatorActionRequired: "choose_section",
			},
		}))
		if err == nil {
			t.Fatalf("state %s: expected decision creation to fail", state)
		}
		if !strings.Contains(err.Error(), "spec_binding_preflight blocks decision creation") {
			t.Fatalf("state %s: error = %v", state, err)
		}
	}
}

func TestDecide_SpecBindingPreflightAllowsNoSpecs(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), completeDecision(DecideInput{
		SelectedTitle: "No specs tactical compatibility",
		WhySelected:   "Haft must allow ordinary decisions in projects without specs.",
		SpecBindingPreflight: &SpecBindingPreflight{
			State: SpecBindingStateNoSpecs,
		},
		SpecBindingRequired: true,
	}))
	if err != nil {
		t.Fatal(err)
	}
}

func TestDecide_SpecBindingPreflightAllowsTacticalOutOfSpecOnly(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), completeDecision(DecideInput{
		SelectedTitle: "Standard out of spec",
		WhySelected:   "Standard decisions cannot silently sit outside the active spec.",
		SpecBindingPreflight: &SpecBindingPreflight{
			State: SpecBindingStateOutOfSpec,
			StatusDebt: SpecBindingStatusDebt{
				Severity: "high",
				Message:  "out of spec",
			},
		},
	}))
	if err == nil {
		t.Fatal("expected standard out-of-spec decision to fail")
	}

	_, _, err = Decide(ctx, store, t.TempDir(), completeDecision(DecideInput{
		SelectedTitle: "Tactical out of spec",
		WhySelected:   "Tactical decisions can carry explicit out-of-spec debt.",
		Mode:          string(ModeTactical),
		SpecBindingPreflight: &SpecBindingPreflight{
			State: SpecBindingStateOutOfSpec,
			StatusDebt: SpecBindingStatusDebt{
				Severity: "high",
				Message:  "out of spec",
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
}

func TestNormalizePredictionInputs_PreservesVerifyAfter(t *testing.T) {
	input := []PredictionInput{
		{
			Claim:       "Claim",
			Observable:  "observable",
			Threshold:   "threshold",
			VerifyAfter: " 2026-05-01 ",
		},
	}

	got := normalizePredictionInputs(input)

	if len(got) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(got))
	}
	if got[0].VerifyAfter != "2026-05-01" {
		t.Fatalf("VerifyAfter = %q, want %q", got[0].VerifyAfter, "2026-05-01")
	}
}

func TestDecide_Tactical(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	a, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		ProblemStatement: "The service needs per-IP rate limiting without adding an external coordination dependency.",
		SelectedTitle:    "x/time/rate for rate limiting",
		WhySelected:      "Zero deps, per-IP tracking testable in Go",
		SelectionPolicy:  "Prefer the least operationally complex limiter that still keeps per-IP enforcement local to the service.",
		CounterArgument:  "An in-process limiter could fragment enforcement if traffic shifts toward multi-instance bursts.",
		WhyNotOthers: []RejectionReason{
			{Variant: "Redis-backed limiter", Reason: "Cross-process coordination was unnecessary at current traffic levels."},
		},
		Invariants:      []string{"Rate limit applied per-IP"},
		RefreshTriggers: []string{"Traffic > 10x current"},
		WeakestLink:     "Burst coordination breaks down once the service scales horizontally.",
		Rollback: &RollbackSpec{
			Triggers: []string{"429 rate remains above the accepted ceiling after rollout"},
		},
		Mode: "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.Mode != ModeTactical {
		t.Errorf("mode = %q, want tactical", a.Meta.Mode)
	}
	if !strings.Contains(a.Body, "Rate limit applied per-IP") {
		t.Error("tactical mode should still have invariants")
	}
	// Tactical without problem_ref: Problem Frame section exists but minimal
	if !strings.Contains(a.Body, "## 1. Problem Frame") {
		t.Error("even tactical DRR should have Problem Frame section")
	}
}

func TestDecide_EscapesRejectedAlternativeTableCells(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	a, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		ProblemStatement: "The current transport no longer stays within the latency budget.",
		SelectedTitle:    "gRPC | v2",
		WhySelected:      "Line 1\nLine 2 | more",
		SelectionPolicy:  "Prefer the transport that stays within the latency budget with the fewest avoidable moving parts.",
		CounterArgument:  "Migration friction could outweigh the latency gain if the rollout path is rougher than expected.",
		WhyNotOthers: []RejectionReason{
			{
				Variant: "REST | v1\nlegacy",
				Reason:  "Higher latency\nNeeds | extra gateways",
			},
		},
		WeakestLink: "Rollout complexity under mixed-protocol traffic.",
		Rollback: &RollbackSpec{
			Triggers: []string{"Latency budget regresses after cutover"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	selectedRow := "| gRPC \\| v2 | **Selected** | Line 1<br>Line 2 \\| more |"
	if !strings.Contains(a.Body, selectedRow) {
		t.Fatalf("selected rationale row did not escape table cell content:\n%s", a.Body)
	}

	rejectedRow := "| REST \\| v1<br>legacy | Rejected | Higher latency<br>Needs \\| extra gateways |"
	if !strings.Contains(a.Body, rejectedRow) {
		t.Fatalf("rejected rationale row did not escape table cell content:\n%s", a.Body)
	}
}

func TestDecide_MissingRequired(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		WhySelected: "because",
	})
	if err == nil {
		t.Error("expected error for missing selected_title")
	}

	_, _, err = Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle: "something",
	})
	if err == nil {
		t.Error("expected error for missing why_selected")
	}
}

func TestDecide_MissingAntiSelfDeceptionFields(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle: "NATS JetStream",
		WhySelected:   "Lower operational overhead wins.",
	})
	if err == nil {
		t.Fatal("expected error for missing anti-self-deception fields")
	}

	// Validator now emits structured per-field rows ("- <field> — <hint>")
	// instead of inline "<field> is required" prose. Match by field name
	// + " — " separator so the assertion survives hint text edits.
	required := []string{
		"- selection_policy — ",
		"- counterargument — ",
		"- weakest_link — ",
		"- why_not_others — ",
		"- rollback — ",
	}

	for _, want := range required {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing validation message %q in %q", want, err.Error())
		}
	}
}

func TestDecide_RejectsSelectedVariantAsRejectedAlternative(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle:   "NATS JetStream",
		WhySelected:     "Lower operational overhead wins.",
		SelectionPolicy: "Prefer the broker with enough throughput headroom and less operational burden.",
		CounterArgument: "The simpler broker could run out of ecosystem leverage under sustained scale growth.",
		WeakestLink:     "Ecosystem maturity at the upper traffic envelope.",
		WhyNotOthers: []RejectionReason{
			{Variant: "NATS JetStream", Reason: "This should never repeat the selected title."},
		},
		Rollback: &RollbackSpec{
			Triggers: []string{"Producer errors spike after cutover"},
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid why_not_others")
	}
	if !strings.Contains(err.Error(), "must not repeat selected_title") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestDecide_RejectsDuplicateLiveDecisionForProblem(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:  "Event pipeline redesign",
		Signal: "Current broker drops messages during failover.",
	})
	if err != nil {
		t.Fatal(err)
	}

	firstPortfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("NATS", "cluster tuning", "Lean broker with smaller operational surface."),
			testVariant("Kafka", "ops overhead", "Mature streaming platform with heavier operations."),
		},
		NoSteppingStoneRationale: "Both variants are direct production paths.",
	})
	if err != nil {
		t.Fatal(err)
	}

	firstDecision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		PortfolioRef:  firstPortfolio.Meta.ID,
		SelectedTitle: "NATS",
		WhySelected:   "Smaller operational surface wins at the current scale.",
	}))
	if err != nil {
		t.Fatal(err)
	}

	secondPortfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("RabbitMQ quorum", "migration risk", "Reuse existing AMQP tooling with a more resilient queue mode."),
			testVariant("Kafka", "ops overhead", "Mature streaming platform with heavier operations."),
		},
		NoSteppingStoneRationale: "Both variants still target the same production decision.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Decide(ctx, store, haftDir, completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		PortfolioRef:  secondPortfolio.Meta.ID,
		SelectedTitle: "RabbitMQ quorum",
		WhySelected:   "It would reduce migration effort.",
	}))
	if err == nil {
		t.Fatal("expected duplicate live decision error")
	}
	if !strings.Contains(err.Error(), problem.Meta.ID) {
		t.Fatalf("expected problem_ref in error, got %v", err)
	}
	if !strings.Contains(err.Error(), firstDecision.Meta.ID) {
		t.Fatalf("expected existing decision ref in error, got %v", err)
	}
}

func TestDecide_RejectsPortfolioOnlyDecisionWhenProblemAlreadyHasLiveDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:  "Auth token rotation",
		Signal: "Refresh tokens stay valid after key rotation.",
	})
	if err != nil {
		t.Fatal(err)
	}

	firstPortfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("Rotate signing key in place", "cache invalidation", "Smaller code change, but clients may cache the old verifier."),
			testVariant("Dual-key grace period", "operational complexity", "Accept both keys during the migration window."),
		},
		NoSteppingStoneRationale: "Both variants are immediate production candidates.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Decide(ctx, store, haftDir, completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		PortfolioRef:  firstPortfolio.Meta.ID,
		SelectedTitle: "Dual-key grace period",
		WhySelected:   "It avoids breaking active sessions during rotation.",
	}))
	if err != nil {
		t.Fatal(err)
	}

	secondPortfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("Opaque sessions", "storage growth", "Move token validity to server-side session lookup."),
			testVariant("Dual-key grace period", "operational complexity", "Keep the current token model with safer rotation."),
		},
		NoSteppingStoneRationale: "Both variants remain valid responses to the same problem.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Decide(ctx, store, haftDir, completeDecision(DecideInput{
		PortfolioRef:  secondPortfolio.Meta.ID,
		SelectedTitle: "Opaque sessions",
		WhySelected:   "It centralizes revocation control.",
	}))
	if err == nil {
		t.Fatal("expected duplicate live decision error for portfolio-only lineage")
	}
	if !strings.Contains(err.Error(), problem.Meta.ID) {
		t.Fatalf("expected derived problem_ref in error, got %v", err)
	}
}

func TestDecide_AllowsReplacementAfterSupersede(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:  "Background job queue",
		Signal: "Queue latency is rising faster than expected.",
	})
	if err != nil {
		t.Fatal(err)
	}

	firstPortfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("Single Redis queue", "head-of-line blocking", "Keep the architecture simple and local."),
			testVariant("Sharded Redis queues", "routing complexity", "Split heavy and light jobs into separate lanes."),
		},
		NoSteppingStoneRationale: "Both variants are direct implementation paths.",
	})
	if err != nil {
		t.Fatal(err)
	}

	firstDecision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		PortfolioRef:  firstPortfolio.Meta.ID,
		SelectedTitle: "Single Redis queue",
		WhySelected:   "It is enough until job volume proves otherwise.",
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = SupersedeArtifact(ctx, store, haftDir, firstDecision.Meta.ID, "", "A broader queue redesign replaced the first choice.")
	if err != nil {
		t.Fatal(err)
	}

	secondPortfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("Sharded Redis queues", "routing complexity", "Split heavy and light jobs into separate lanes."),
			testVariant("Postgres-backed queue", "db load", "Reuse existing operational tooling for jobs."),
		},
		NoSteppingStoneRationale: "Both variants remain immediate candidates after the supersession.",
	})
	if err != nil {
		t.Fatal(err)
	}

	replacementDecision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		PortfolioRef:  secondPortfolio.Meta.ID,
		SelectedTitle: "Sharded Redis queues",
		WhySelected:   "Volume now justifies splitting hot and cold workloads.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if replacementDecision.Meta.ID == "" {
		t.Fatal("expected replacement decision to be created")
	}
}

func TestApply_ReturnsBody(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	dec, _, _ := Decide(ctx, store, t.TempDir(), DecideInput{
		ProblemStatement: "The current event transport imposes more operational load than the team can sustain.",
		SelectedTitle:    "NATS JetStream",
		WhySelected:      "Ops simplicity",
		SelectionPolicy:  "Prefer the messaging option that reduces operator load without sacrificing delivery guarantees.",
		CounterArgument:  "Operational simplicity could hide capacity limits that only appear under real production traffic.",
		WhyNotOthers: []RejectionReason{
			{Variant: "Kafka", Reason: "The extra operating surface was not justified at the current scale."},
		},
		Invariants:  []string{"At-least-once delivery"},
		WeakestLink: "Capacity evidence is thinner than for the more mature alternative.",
		Rollback: &RollbackSpec{
			Triggers: []string{"Delivery errors increase after migration"},
		},
	})

	body, err := Apply(ctx, store, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Apply now returns the DRR body directly
	if !strings.Contains(body, "NATS JetStream") {
		t.Error("apply should return DRR body with decision content")
	}
	if !strings.Contains(body, "At-least-once delivery") {
		t.Error("apply should return DRR body with invariants")
	}
}

func TestApply_NotFound(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := Apply(ctx, store, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent decision")
	}
}

func TestDecide_InheritsContext(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Auth redesign", Signal: "Token expiry issues", Context: "auth",
	})

	a, _, err := Decide(ctx, store, haftDir, DecideInput{
		ProblemRef:      prob.Meta.ID,
		SelectedTitle:   "JWT with refresh tokens",
		WhySelected:     "Standard approach, well-understood",
		SelectionPolicy: "Prefer the approach with the strongest operator familiarity while still supporting token rotation.",
		CounterArgument: "Refresh-token sprawl can increase revocation complexity and session abuse risk.",
		WhyNotOthers: []RejectionReason{
			{Variant: "Opaque sessions", Reason: "Extra session-store coordination was not needed for the current auth boundary."},
		},
		WeakestLink: "Revocation logic is easy to get subtly wrong once multiple clients cache refresh tokens.",
		Rollback: &RollbackSpec{
			Triggers: []string{"Token refresh error rate rises after deployment"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.Context != "auth" {
		t.Errorf("context = %q, want auth (inherited from problem)", a.Meta.Context)
	}
}

func TestDecide_PersistsPredictionsInStructuredStateAndReload(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	input := completeDecision(DecideInput{
		SelectedTitle: "NATS JetStream",
		WhySelected:   "Operational simplicity still leaves room to verify the throughput bet explicitly.",
		Predictions: []PredictionInput{
			{
				Claim:      "Migration keeps p99 publish latency below 50ms",
				Observable: "publish latency p99",
				Threshold:  "< 50ms under projected load",
			},
			{
				Claim:      "Producer error rate stays below 0.1%",
				Observable: "producer error rate",
				Threshold:  "< 0.1% during rollout week",
			},
		},
	})

	decision, _, err := Decide(ctx, store, t.TempDir(), input)
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	wantClaims := newDecisionClaims(input.Predictions)
	wantPredictions := decisionPredictionsFromClaims(newDecisionClaims(input.Predictions))
	if !reflect.DeepEqual(fields.Claims, wantClaims) {
		t.Fatalf("claims in structured state = %#v, want %#v", fields.Claims, wantClaims)
	}
	if !reflect.DeepEqual(fields.Predictions, wantPredictions) {
		t.Fatalf("predictions in structured state = %#v, want %#v", fields.Predictions, wantPredictions)
	}
	if !strings.Contains(decision.StructuredData, "\"claims\"") {
		t.Fatalf("decision structured data should persist canonical claims:\n%s", decision.StructuredData)
	}
	if strings.Contains(decision.StructuredData, "\"predictions\"") {
		t.Fatalf("decision structured data should not persist legacy predictions:\n%s", decision.StructuredData)
	}

	if !strings.Contains(decision.Body, "**Predictions:**") {
		t.Fatalf("decision body should render predictions:\n%s", decision.Body)
	}
	if !strings.Contains(decision.Body, "| Claim | Observable | Threshold |") {
		t.Fatalf("decision body should render predictions in canonical table form:\n%s", decision.Body)
	}
	if !strings.Contains(decision.Body, "publish latency p99") {
		t.Fatalf("decision body should include rendered prediction details:\n%s", decision.Body)
	}

	reloaded, err := store.Get(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	reloadedFields := reloaded.UnmarshalDecisionFields()
	if !reflect.DeepEqual(reloadedFields.Claims, wantClaims) {
		t.Fatalf("reloaded claims = %#v, want %#v", reloadedFields.Claims, wantClaims)
	}
	if !reflect.DeepEqual(reloadedFields.Predictions, wantPredictions) {
		t.Fatalf("reloaded predictions = %#v, want %#v", reloadedFields.Predictions, wantPredictions)
	}
}

func TestDecide_PersistsExplicitClaimLifecycleState(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	input := completeDecision(DecideInput{
		SelectedTitle: "Claim lifecycle",
		WhySelected:   "Explicit claim lifecycle lets one stale claim stop governing without retiring the whole decision.",
		Claims: []DecisionClaim{
			{
				ID:                   "claim-current",
				Claim:                "Current invariant still holds",
				Observable:           "invariant check",
				Threshold:            "passes",
				GovernanceTargetRefs: []string{"invariant:current"},
			},
			{
				ID:              "claim-old",
				Claim:           "Old invariant was replaced",
				Observable:      "replacement decision exists",
				Threshold:       "successor present",
				LifecycleStatus: ClaimLifecycleSuperseded,
				SuccessorRef:    "dec-new#claim-replacement",
				RetiredReason:   "superseded by narrower invariant",
			},
		},
	})

	decision, _, err := Decide(ctx, store, t.TempDir(), input)
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if len(fields.Claims) != 2 {
		t.Fatalf("claims = %#v, want two explicit claims", fields.Claims)
	}
	if fields.Claims[0].LifecycleStatus != "" {
		t.Fatalf("legacy-active lifecycle should stay omitted, got %q", fields.Claims[0].LifecycleStatus)
	}
	if EffectiveClaimLifecycleStatus(fields.Claims[0]) != ClaimLifecycleActive {
		t.Fatalf("effective current lifecycle = %q", EffectiveClaimLifecycleStatus(fields.Claims[0]))
	}
	if fields.Claims[1].LifecycleStatus != ClaimLifecycleSuperseded {
		t.Fatalf("claim-old lifecycle = %q", fields.Claims[1].LifecycleStatus)
	}
	if fields.Claims[1].SuccessorRef != "dec-new#claim-replacement" {
		t.Fatalf("successor_ref = %q", fields.Claims[1].SuccessorRef)
	}
	if fields.Claims[1].RetiredReason != "superseded by narrower invariant" {
		t.Fatalf("retired_reason = %q", fields.Claims[1].RetiredReason)
	}
	if len(fields.Predictions) != 2 {
		t.Fatalf("compatibility predictions = %#v, want two", fields.Predictions)
	}
	if !strings.Contains(decision.StructuredData, "\"lifecycle_status\":\"superseded\"") {
		t.Fatalf("structured data missing lifecycle status:\n%s", decision.StructuredData)
	}
	if strings.Contains(decision.StructuredData, "\"predictions\"") {
		t.Fatalf("structured data should keep predictions as compatibility projection only:\n%s", decision.StructuredData)
	}
}

func TestDecide_PredictionsRemainOptionalAndLegacyDecisionsReload(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	decision, _, err := Decide(ctx, store, t.TempDir(), completeDecision(DecideInput{
		SelectedTitle: "NATS JetStream",
		WhySelected:   "The prediction section should stay absent when nothing was declared.",
	}))
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if len(fields.Claims) != 0 {
		t.Fatalf("expected no structured claims, got %#v", fields.Claims)
	}
	if len(fields.Predictions) != 0 {
		t.Fatalf("expected no structured predictions, got %#v", fields.Predictions)
	}
	if len(fields.ProblemRefs) != 0 {
		t.Fatalf("expected no structured problem refs, got %#v", fields.ProblemRefs)
	}
	if len(fields.PreConditions) != 0 {
		t.Fatalf("expected no structured pre-conditions, got %#v", fields.PreConditions)
	}
	if len(fields.EvidenceRequirements) != 0 {
		t.Fatalf("expected no structured evidence requirements, got %#v", fields.EvidenceRequirements)
	}
	if len(fields.RefreshTriggers) != 0 {
		t.Fatalf("expected no structured refresh triggers, got %#v", fields.RefreshTriggers)
	}
	if strings.Contains(decision.Body, "**Predictions:**") {
		t.Fatalf("decision body should omit the predictions section when none were declared:\n%s", decision.Body)
	}

	legacy := &Artifact{
		Meta: Meta{
			ID:     "dec-legacy-predictions",
			Kind:   KindDecisionRecord,
			Title:  "Legacy decision",
			Status: StatusActive,
		},
		Body:           "# Legacy decision\n",
		StructuredData: `{"selected_title":"Legacy decision","why_selected":"Already shipped"}`,
	}
	if err := store.Create(ctx, legacy); err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Get(ctx, legacy.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	reloadedFields := reloaded.UnmarshalDecisionFields()
	if len(reloadedFields.Claims) != 0 {
		t.Fatalf("legacy decision should decode with no claims, got %#v", reloadedFields.Claims)
	}
	if len(reloadedFields.Predictions) != 0 {
		t.Fatalf("legacy decision should decode with no predictions, got %#v", reloadedFields.Predictions)
	}
	if len(reloadedFields.ProblemRefs) != 0 {
		t.Fatalf("legacy decision should decode with no problem refs, got %#v", reloadedFields.ProblemRefs)
	}
	if len(reloadedFields.PreConditions) != 0 {
		t.Fatalf("legacy decision should decode with no pre-conditions, got %#v", reloadedFields.PreConditions)
	}
	if len(reloadedFields.EvidenceRequirements) != 0 {
		t.Fatalf("legacy decision should decode with no evidence requirements, got %#v", reloadedFields.EvidenceRequirements)
	}
	if len(reloadedFields.RefreshTriggers) != 0 {
		t.Fatalf("legacy decision should decode with no refresh triggers, got %#v", reloadedFields.RefreshTriggers)
	}
}

func TestArtifact_UnmarshalDecisionFields_DefaultsLegacyPredictionStatus(t *testing.T) {
	decision := &Artifact{
		StructuredData: `{
			"selected_title":"Legacy decision",
			"why_selected":"Already shipped",
			"predictions":[
				{"claim":"Latency stays under 50ms","observable":"publish latency p99","threshold":"< 50ms"}
			]
		}`,
	}

	fields := decision.UnmarshalDecisionFields()

	wantClaims := []DecisionClaim{{
		ID:         "claim-001",
		Claim:      "Latency stays under 50ms",
		Observable: "publish latency p99",
		Threshold:  "< 50ms",
		Status:     ClaimStatusUnverified,
	}}
	wantPredictions := []DecisionPrediction{{
		Claim:      "Latency stays under 50ms",
		Observable: "publish latency p99",
		Threshold:  "< 50ms",
		Status:     ClaimStatusUnverified,
	}}

	if !reflect.DeepEqual(fields.Claims, wantClaims) {
		t.Fatalf("legacy claims = %#v, want %#v", fields.Claims, wantClaims)
	}
	if !reflect.DeepEqual(fields.Predictions, wantPredictions) {
		t.Fatalf("legacy predictions = %#v, want %#v", fields.Predictions, wantPredictions)
	}
}

func TestDecide_RejectsPartialPredictions(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), completeDecision(DecideInput{
		SelectedTitle: "NATS JetStream",
		WhySelected:   "Predictions must be complete before they become canonical runtime state.",
		Predictions: []PredictionInput{
			{
				Claim: "Latency stays below 50ms",
			},
		},
	}))
	if err == nil {
		t.Fatal("expected validation error for partial prediction")
	}

	required := []string{
		"predictions[0].observable is required",
		"predictions[0].threshold is required",
	}

	for _, want := range required {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing validation message %q in %q", want, err.Error())
		}
	}
}

func TestDecide_RejectsEmptyPredictions(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	testCases := []struct {
		name        string
		predictions []PredictionInput
	}{
		{
			name:        "all empty",
			predictions: []PredictionInput{{}},
		},
		{
			name: "whitespace only",
			predictions: []PredictionInput{{
				Claim:      "   ",
				Observable: "\t",
				Threshold:  "\n",
			}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, err := Decide(ctx, store, t.TempDir(), completeDecision(DecideInput{
				SelectedTitle: "NATS JetStream",
				WhySelected:   "Empty predictions must be rejected instead of silently disappearing.",
				Predictions:   testCase.predictions,
			}))
			if err == nil {
				t.Fatal("expected validation error for empty prediction")
			}

			required := []string{
				"predictions[0] must include claim, observable, and threshold together",
				"predictions[0].claim is required",
				"predictions[0].observable is required",
				"predictions[0].threshold is required",
			}

			for _, want := range required {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("missing validation message %q in %q", want, err.Error())
				}
			}
		})
	}
}
