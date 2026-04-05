package artifact

import (
	"context"
	"reflect"
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
		AffectedFiles: []string{"internal/events/producer.go", "internal/events/consumer.go"},
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

	reloaded, err := store.Get(ctx, a.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	reloadedFields := reloaded.UnmarshalDecisionFields()
	if !reflect.DeepEqual(reloadedFields.ProblemRefs, fields.ProblemRefs) {
		t.Fatalf("reloaded problem refs = %#v, want %#v", reloadedFields.ProblemRefs, fields.ProblemRefs)
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

	// Check affected files in DB
	files, _ := store.GetAffectedFiles(ctx, a.Meta.ID)
	if len(files) != 2 {
		t.Errorf("expected 2 affected files, got %d", len(files))
	}
}

func TestDecide_Tactical(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	a, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle:   "x/time/rate for rate limiting",
		WhySelected:     "Zero deps, per-IP tracking testable in Go",
		SelectionPolicy: "Prefer the least operationally complex limiter that still keeps per-IP enforcement local to the service.",
		CounterArgument: "An in-process limiter could fragment enforcement if traffic shifts toward multi-instance bursts.",
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
		SelectedTitle:   "gRPC | v2",
		WhySelected:     "Line 1\nLine 2 | more",
		SelectionPolicy: "Prefer the transport that stays within the latency budget with the fewest avoidable moving parts.",
		CounterArgument: "Migration friction could outweigh the latency gain if the rollout path is rougher than expected.",
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

func TestApply_ReturnsBody(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	dec, _, _ := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle:   "NATS JetStream",
		WhySelected:     "Ops simplicity",
		SelectionPolicy: "Prefer the messaging option that reduces operator load without sacrificing delivery guarantees.",
		CounterArgument: "Operational simplicity could hide capacity limits that only appear under real production traffic.",
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
	wantPredictions := newDecisionPredictions(input.Predictions)
	if !reflect.DeepEqual(fields.Predictions, wantPredictions) {
		t.Fatalf("predictions in structured state = %#v, want %#v", fields.Predictions, wantPredictions)
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
	if !reflect.DeepEqual(reloadedFields.Predictions, wantPredictions) {
		t.Fatalf("reloaded predictions = %#v, want %#v", reloadedFields.Predictions, wantPredictions)
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

	wantPredictions := []DecisionPrediction{{
		Claim:      "Latency stays under 50ms",
		Observable: "publish latency p99",
		Threshold:  "< 50ms",
		Status:     ClaimStatusUnverified,
	}}

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
