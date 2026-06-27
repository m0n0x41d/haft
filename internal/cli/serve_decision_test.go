package cli

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintDecision_DecidePersistsPredictions(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, _, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":           "decide",
		"selected_title":   "gRPC",
		"why_selected":     "Plugin-mode decide should persist falsifiable predictions through the serve path.",
		"selection_policy": "Prefer the lowest latency option that stays inside the operational budget.",
		"counterargument":  "The simplified benchmark can miss production load variance.",
		"weakest_link":     "Operational confidence still depends on limited production-grade evidence.",
		"why_not_others": []map[string]any{{
			"variant": "REST",
			"reason":  "Higher steady-state latency with no decisive compensating advantage.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"Latency regresses in production."},
		},
		"predictions": []map[string]any{{
			"claim":      "Throughput stays above 100k events/sec",
			"observable": "throughput",
			"threshold":  "> 100k events/sec",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	decisions, err := store.ListByKind(ctx, artifact.KindDecisionRecord, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}

	decision, err := store.Get(ctx, decisions[0].Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if len(fields.Predictions) != 1 {
		t.Fatalf("expected 1 prediction, got %+v", fields.Predictions)
	}

	prediction := fields.Predictions[0]
	if prediction.Claim != "Throughput stays above 100k events/sec" {
		t.Fatalf("prediction claim = %q", prediction.Claim)
	}
	if prediction.Observable != "throughput" {
		t.Fatalf("prediction observable = %q", prediction.Observable)
	}
	if prediction.Threshold != "> 100k events/sec" {
		t.Fatalf("prediction threshold = %q", prediction.Threshold)
	}
}

func TestHandleQuintDecision_DecidePersistsTransformationRecord(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, ref, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":           "decide",
		"selected_title":   "Separate transformation description",
		"why_selected":     "The target state change should be explicit without implying implementation work.",
		"selection_policy": "Prefer explicit semantic objects that preserve legacy DecisionRecord compatibility.",
		"counterargument":  "A nested object can be ignored by old carriers if discovery is weak.",
		"weakest_link":     "Agents may still confuse transformed state with completed work unless the boundary is visible.",
		"why_not_others": []map[string]any{{
			"variant": "Post-conditions only",
			"reason":  "Post-conditions do not name the transformed entity and initial state.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"DecisionRecord compatibility parsing regresses."},
		},
		"transformation_record": map[string]any{
			"transformed_entity": "DecisionRecord compatibility projection",
			"initial_state":      "choice, transformation, work, and evidence are described in one prose aggregate",
			"post_state":         "transformation has a first-class object-state description",
			"relation":           "separates",
			"context":            "semantic-spine TransformationRecord v1",
			"window":             "2026-Q3",
			"method_refs":        []any{"mpull-transformation-record"},
			"work_refs":          []any{"wc-transformation-record"},
			"evidence_refs":      []any{"evid-transformation-record"},
			"publication_refs":   []any{"pub-transformation-record"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}

	record := decision.UnmarshalDecisionFields().TransformationRecord
	if record == nil {
		t.Fatal("transformation_record missing")
	}
	if record.SchemaVersion != artifact.TransformationRecordSchemaVersion {
		t.Fatalf("schema_version = %d, want %d", record.SchemaVersion, artifact.TransformationRecordSchemaVersion)
	}
	if record.TransformedEntity != "DecisionRecord compatibility projection" {
		t.Fatalf("transformed_entity = %q", record.TransformedEntity)
	}
	if record.Window != "2026-Q3" {
		t.Fatalf("window = %q", record.Window)
	}
	if len(record.MethodRefs) != 1 || record.MethodRefs[0] != "mpull-transformation-record" {
		t.Fatalf("method_refs = %#v", record.MethodRefs)
	}
	if len(record.WorkRefs) != 1 || record.WorkRefs[0] != "wc-transformation-record" {
		t.Fatalf("work_refs = %#v", record.WorkRefs)
	}
	if len(record.EvidenceRefs) != 1 || record.EvidenceRefs[0] != "evid-transformation-record" {
		t.Fatalf("evidence_refs = %#v", record.EvidenceRefs)
	}
	if len(record.PublicationRefs) != 1 || record.PublicationRefs[0] != "pub-transformation-record" {
		t.Fatalf("publication_refs = %#v", record.PublicationRefs)
	}
}

func TestHandleQuintDecision_DecidePersistsC11ChoiceResult(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, ref, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":           "decide",
		"selected_title":   "Use exact choice semantics",
		"why_selected":     "The choice payload needs subject, options, basis, rule, and next move.",
		"selection_policy": "Prefer the payload that names C.11 fields explicitly.",
		"counterargument":  "Legacy consumers may ignore the new optional fields.",
		"weakest_link":     "Agents may still treat DecisionRecord as the primitive choice object.",
		"why_not_others": []map[string]any{{
			"variant": "Keep minimal ChoiceResult",
			"reason":  "It does not expose the option set or choice rule.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"Legacy DecisionRecord parsing regresses."},
		},
		"choice_result": map[string]any{
			"subject_ref":      "operator",
			"option_set":       []any{"Use exact choice semantics", "Keep minimal ChoiceResult"},
			"comparison_basis": []any{"selected Use exact choice semantics: names C.11 fields", "rejected Keep minimal ChoiceResult: leaves fields implicit"},
			"choice_rule":      "Prefer the payload that names C.11 fields explicitly.",
			"next_move":        string(artifact.ChoiceNextMoveChooseNow),
			"variant_ref":      "Use exact choice semantics",
			"reason":           "The operator invoked h-decide.",
			"reversibility":    "two-week rollback",
			"reopen_condition": "reopen if rollback triggers occur",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}

	choice := decision.UnmarshalDecisionFields().ChoiceResult
	if choice == nil {
		t.Fatal("choice_result missing")
	}
	if choice.ChoiceRule != "Prefer the payload that names C.11 fields explicitly." {
		t.Fatalf("choice_rule = %q", choice.ChoiceRule)
	}
	if len(choice.OptionSet) != 2 {
		t.Fatalf("option_set = %#v, want two options", choice.OptionSet)
	}
	if choice.Reversibility != "two-week rollback" {
		t.Fatalf("reversibility = %q", choice.Reversibility)
	}
	if choice.ReopenCondition != "reopen if rollback triggers occur" {
		t.Fatalf("reopen_condition = %q", choice.ReopenCondition)
	}
}

func TestHandleQuintDecision_DecideUsesTaskContextInArtifactID(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, ref, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":           "decide",
		"selected_title":   "Use gRPC",
		"why_selected":     "Serve-mode decide should pass task_context into the DecisionRecord ID.",
		"selection_policy": "Prefer a transport decision that remains traceable to the implementation task.",
		"counterargument":  "Filename context can be mistaken for the decision's semantic authority.",
		"weakest_link":     "The slug is metadata only and can go stale if the task changes.",
		"task_context":     "Task #4: API/CLI cleanup",
		"why_not_others": []map[string]any{{
			"variant": "REST",
			"reason":  "It does not exercise the optional DecisionRecord slug path.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"Decision IDs lose their random suffix."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^dec-\d{8}-task-4-api-cli-cleanup-[0-9a-f]{8}$`)
	if !pattern.MatchString(ref) {
		t.Fatalf("created ref = %q, want sanitized task_context slug before 8-hex suffix", ref)
	}

	decision, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}

	fields := decision.UnmarshalDecisionFields()
	if fields.TaskContext != "task-4-api-cli-cleanup" {
		t.Fatalf("structured task_context = %q, want sanitized slug", fields.TaskContext)
	}
}

// TestHandleQuintDecision_DecideReturnsArtifactID verifies that the decide
// action returns the canonical artifact ID as the second return value. This
// closes the cross-project recall bug where the global index was keyed by
// selected_title (collision-prone) instead of the real DecisionRecord ID.
//
// Two decisions with the same selected_title in the same project must produce
// distinct IDs — otherwise the cross-project index silently overwrites the
// first decision's entry on the second decide call.
func TestHandleQuintDecision_DecideReturnsArtifactID(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	args := func() map[string]any {
		return map[string]any{
			"action":           "decide",
			"selected_title":   "Use Postgres",
			"why_selected":     "The team already operates Postgres at scale; default storage choice.",
			"selection_policy": "Prefer the storage system the on-call team already operates at scale.",
			"counterargument":  "Postgres at scale assumes operational maturity that may not hold for new services.",
			"weakest_link":     "Operational maturity in this team is the binding factor.",
			"why_not_others": []map[string]any{{
				"variant": "MySQL",
				"reason":  "Team has no production operational experience with MySQL at the required scale.",
			}},
			"rollback": map[string]any{
				"triggers": []string{"Operational load makes Postgres untenable."},
			},
		}
	}

	_, ref1, err := handleQuintDecision(ctx, store, haftDir, args())
	if err != nil {
		t.Fatalf("first decide: %v", err)
	}
	if ref1 == "" {
		t.Fatal("first decide returned empty createdRef; expected canonical artifact ID")
	}

	_, ref2, err := handleQuintDecision(ctx, store, haftDir, args())
	if err != nil {
		t.Fatalf("second decide: %v", err)
	}
	if ref2 == "" {
		t.Fatal("second decide returned empty createdRef; expected canonical artifact ID")
	}

	if ref1 == ref2 {
		t.Fatalf("two decisions with same selected_title produced identical IDs (%q); cross-project index would collide", ref1)
	}

	// Both refs must match real persisted artifacts.
	for _, ref := range []string{ref1, ref2} {
		a, err := store.Get(ctx, ref)
		if err != nil || a == nil {
			t.Fatalf("createdRef %q does not resolve to a stored artifact: %v", ref, err)
		}
		if a.Meta.Kind != artifact.KindDecisionRecord {
			t.Fatalf("createdRef %q resolved to %s, want DecisionRecord", ref, a.Meta.Kind)
		}
	}
}

func TestHandleQuintDecision_DecideTreatsUnresolvedAffectedFilesAsFootprintOnly(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	if err := os.WriteFile(filepath.Join(projectRoot, "worker.go"), []byte(`package main

func Start() string { return "start" }
func Stop() string { return "stop" }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, ref, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":           "decide",
		"selected_title":   "Use worker file",
		"why_selected":     "The affected file is an implementation footprint until a precise governed target is named.",
		"selection_policy": "Prefer recording the footprint without creating drift authority.",
		"counterargument":  "A later baseline may still be needed after target selection.",
		"weakest_link":     "Agents may mistake file footprint for governance authority.",
		"why_not_others": []map[string]any{{
			"variant": "Auto whole-file baseline",
			"reason":  "It creates broad needs-binding drift noise.",
		}},
		"rollback": map[string]any{
			"triggers": []string{"status noise increases"},
		},
		"affected_files": []any{"worker.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "implementation footprint only") || !strings.Contains(out, "no drift baseline created") {
		t.Fatalf("response missing footprint-only baseline note:\n%s", out)
	}

	decision, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	fields := decision.UnmarshalDecisionFields()
	if !fields.IsImplementationFootprintOnly() {
		t.Fatalf("decision fields = %+v, want implementation-footprint only", fields)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, "worker.go"), []byte(`package main

func Start() string { return "changed" }
func Stop() string { return "stop" }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	reports, err := artifact.CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("drift reports = %+v, want footprint-only decision skipped", reports)
	}
}

// TestHandleQuintDecision_NonDecideActionsReturnEmptyRef verifies that actions
// other than "decide" do not return a createdRef. Cross-project indexing is
// only triggered for decide; other actions mutate or read existing artifacts.
func TestHandleQuintDecision_NonDecideActionsReturnEmptyRef(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Apply against missing decision returns plain-language stub, not error.
	_, ref, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action": "apply",
	})
	if err != nil {
		t.Fatalf("apply with no decision: %v", err)
	}
	if ref != "" {
		t.Fatalf("apply returned createdRef %q; expected empty for non-creating action", ref)
	}
}

// TestHandleQuintDecision_BaselineAcceptsArtifactRef regression-tests issue #77:
// LLM clients naturally pass artifact_ref (the universal key in haft_refresh and
// the only documented ref for evidence). Before the fix, baseline/measure silently
// ignored artifact_ref and fell through to ListByKind auto-detect, landing on the
// wrong decision. Now both keys are accepted and the auto-detect is gone.
func TestHandleQuintDecision_BaselineAcceptsArtifactRef(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	older, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Older decision — the intended target",
		WhySelected:     "This is the decision we want to baseline by passing artifact_ref.",
		SelectionPolicy: "Prefer explicit identification over implicit recency.",
		CounterArgument: "Auto-detect may seem convenient but corrupts the artifact graph.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "auto-detect", Reason: "silent misrouting"}},
		WeakestLink:     "LLM clients may still confuse parameter names — schema docs must be clear.",
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Newer decision exists after — this is what ListByKind(...,1) would pick.
	_, _, err = artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Newer decision — should NOT be picked",
		WhySelected:     "If auto-detect leaks, this newer decision would steal the baseline.",
		SelectionPolicy: "Most-recent default is the trap we're closing.",
		CounterArgument: "None.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "older decision", Reason: "test fixture"}},
		WeakestLink:     "None.",
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Touch a file so baseline has something to hash.
	tmpFile := t.TempDir() + "/sample.go"
	if werr := os.WriteFile(tmpFile, []byte("package sample\n"), 0o644); werr != nil {
		t.Fatal(werr)
	}

	out, _, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":         "baseline",
		"artifact_ref":   older.Meta.ID,
		"affected_files": []any{tmpFile},
	})
	if err != nil {
		t.Fatalf("baseline with artifact_ref: %v", err)
	}
	if !strings.Contains(out, older.Meta.ID) {
		t.Fatalf("baseline response %q does not mention intended target %q", out, older.Meta.ID)
	}
}

// TestHandleQuintDecision_BaselineRequiresRef verifies that the silent auto-detect
// fall-through (issue #77) is gone — calling baseline without any ref errors loudly
// instead of picking the most-recent decision.
func TestHandleQuintDecision_BaselineRequiresRef(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Some decision",
		WhySelected:     "Exists so the bug-prone auto-detect would have something to grab.",
		SelectionPolicy: "Explicit refs only.",
		CounterArgument: "None.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "implicit", Reason: "unsafe"}},
		WeakestLink:     "None.",
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action": "baseline",
	})
	if err != nil {
		t.Fatalf("baseline without ref should return guidance, not error: %v", err)
	}
	if !strings.Contains(out, "requires decision_ref") {
		t.Fatalf("baseline without ref should ask for decision_ref; got: %q", out)
	}
}

// TestHandleQuintDecision_MeasureAcceptsArtifactRef — same fix on the measure side.
func TestHandleQuintDecision_MeasureAcceptsArtifactRef(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	older, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Older decision — measure target",
		WhySelected:     "We want measure() to land here when artifact_ref names it.",
		SelectionPolicy: "Explicit refs only.",
		CounterArgument: "None.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "auto-detect", Reason: "corrupts graph"}},
		WeakestLink:     "None.",
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Newer decision — must not be picked",
		WhySelected:     "Sanity guard for the regression test.",
		SelectionPolicy: "Explicit refs only.",
		CounterArgument: "None.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "older", Reason: "fixture"}},
		WeakestLink:     "None.",
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"regressions"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":       "measure",
		"artifact_ref": older.Meta.ID,
		"findings":     "evidence landed on the intended decision",
		"verdict":      "accepted",
	})
	if err != nil {
		t.Fatalf("measure with artifact_ref: %v", err)
	}
	if !strings.Contains(out, older.Meta.ID) {
		t.Fatalf("measure response %q does not reference intended target %q", out, older.Meta.ID)
	}
	for _, want := range []string{"not approval", "not gate passage", "not claim truth", "not global truth", "not publication", "EvidencePath"} {
		if !strings.Contains(out, want) {
			t.Fatalf("measure response missing authority boundary %q:\n%s", want, out)
		}
	}
}

func TestHandleQuintDecision_MeasureRejectsMalformedMeasurements(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Keep measurement parsing strict",
		WhySelected:     "Serve-mode measure should reject malformed arrays instead of truncating them.",
		SelectionPolicy: "Prefer payload validation that preserves semantic parity with the direct tool path.",
		CounterArgument: "Strict parsing can reject callers that relied on historical truncation.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Lenient truncation",
			Reason:  "Silently losing measured values corrupts the decision record.",
		}},
		WeakestLink: "Broken clients may need a migration step before relying on strict validation.",
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Plugin payloads become incompatible with the serve handler."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = handleQuintDecision(ctx, store, haftDir, map[string]any{
		"action":       "measure",
		"decision_ref": decision.Meta.ID,
		"findings":     "The payload mixed strings and numbers.",
		"measurements": []any{"p99 latency: 18ms", 42},
		"verdict":      "partial",
	})
	if err == nil {
		t.Fatal("expected malformed measurements to be rejected")
	}
	if !strings.Contains(err.Error(), "measurements must be an array of strings") {
		t.Fatalf("unexpected error: %v", err)
	}
}
