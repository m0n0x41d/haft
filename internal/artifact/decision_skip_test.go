package artifact

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TestDecide_TacticalSkipMechanism_BypassRequiredFields verifies that a
// tactical-mode decision with explicit _skips can omit required fields
// the standard validator would reject.
func TestDecide_TacticalSkipMechanism_BypassRequiredFields(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Tactical decision skipping all anti-self-deception fields with an
	// explicit operator-acknowledged reason.
	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		ProblemStatement: "The current map implementation does not provide safe concurrent access.",
		SelectedTitle:    "Use sync.Map",
		WhySelected:      "Concurrent reads dominate; sync.Map is idiomatic.",
		Mode:             string(ModeTactical),
		Skips: []string{
			"selection_policy",
			"counterargument",
			"weakest_link",
			"why_not_others",
			"rollback",
		},
		SkipReason: "Tactical 5-line change, reversible by file revert; full DRR ceremony overhead exceeds decision blast radius",
	})
	if err != nil {
		t.Fatalf("tactical-mode decision with explicit skips should accept; got error: %v", err)
	}
}

func TestDecide_TacticalSkipMechanism_PersistsAcknowledgement(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	skips := []string{
		"selection_policy",
		"counterargument",
		"weakest_link",
		"why_not_others",
		"rollback",
	}
	reason := "Tactical 5-line change, reversible by file revert; full DRR ceremony overhead exceeds decision blast radius"

	decision, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		ProblemStatement: "The current map implementation does not provide safe concurrent access.",
		SelectedTitle:    "Use sync.Map",
		WhySelected:      "Concurrent reads dominate; sync.Map is idiomatic.",
		Mode:             string(ModeTactical),
		Skips:            skips,
		SkipReason:       reason,
	})
	if err != nil {
		t.Fatalf("tactical-mode decision with explicit skips should accept; got error: %v", err)
	}

	reloaded, err := store.Get(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields := reloaded.UnmarshalDecisionFields()

	if !slices.Equal(fields.Skips, skips) {
		t.Fatalf("persisted skips = %v, want %v", fields.Skips, skips)
	}
	if fields.SkipReason != reason {
		t.Fatalf("persisted skip reason = %q, want %q", fields.SkipReason, reason)
	}
}

// TestDecide_SkipsRejectedInStandardMode verifies that standard mode
// cannot bypass required fields even when operator declares _skips.
func TestDecide_SkipsRejectedInStandardMode(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle: "Use sync.Map",
		WhySelected:   "Concurrent reads dominate.",
		Mode:          string(ModeStandard),
		Skips:         []string{"selection_policy"},
		SkipReason:    "Don't want to think about it",
	})
	if err == nil {
		t.Fatal("standard mode should reject _skips usage")
	}
	if !strings.Contains(err.Error(), "tactical or note mode") {
		t.Fatalf("error should mention tactical-only restriction; got: %v", err)
	}
}

// TestDecide_SkipsWithoutReason_Rejected verifies that _skips usage
// without _skip_reason is rejected — operator must acknowledge with
// rationale, not silent bypass.
func TestDecide_SkipsWithoutReason_Rejected(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle: "Use sync.Map",
		WhySelected:   "Concurrent reads dominate.",
		Mode:          string(ModeTactical),
		Skips:         []string{"rollback"},
		// _skip_reason omitted on purpose
	})
	if err == nil {
		t.Fatal("tactical _skips without _skip_reason should be rejected")
	}
	if !strings.Contains(err.Error(), "_skip_reason is required") {
		t.Fatalf("error should mention required _skip_reason; got: %v", err)
	}
}

// TestDecide_UnknownSkipField_Rejected verifies that a skip field
// outside the allowlist is rejected — prevents typos from silently
// disabling validation for fields the operator didn't intend.
func TestDecide_UnknownSkipField_Rejected(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle: "Use sync.Map",
		WhySelected:   "Concurrent reads dominate.",
		Mode:          string(ModeTactical),
		Skips:         []string{"selecton_policy"}, // typo
		SkipReason:    "tactical change",
	})
	if err == nil {
		t.Fatal("unknown skip field name should be rejected")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error should mention unknown skip field; got: %v", err)
	}
}

// TestDecide_SelectedTitleNotSkippable verifies that selected_title is
// not in the allowlist — a decision without a selection has no identity
// and the skip mechanism cannot bypass that.
func TestDecide_SelectedTitleNotSkippable(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		// SelectedTitle missing
		WhySelected: "doesn't matter",
		Mode:        string(ModeTactical),
		Skips:       []string{"selected_title"},
		SkipReason:  "trying to skip identity",
	})
	if err == nil {
		t.Fatal("selected_title should not be skippable")
	}
	// Should fail either at "unknown skip field" (selected_title not in
	// allowlist) or at the validate step (missing field). Either is
	// correct — both close the loophole.
}

// TestDecide_StructuredErrorIncludesReferences verifies that validation
// errors include FPF spec references and how-to-proceed options so the
// agent can self-correct without operator hand-holding.
func TestDecide_StructuredErrorIncludesReferences(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := Decide(ctx, store, t.TempDir(), DecideInput{
		SelectedTitle: "x",
		WhySelected:   "y",
		Mode:          string(ModeStandard),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	got := err.Error()

	for _, marker := range []string{
		"FPF discipline violation",
		"Missing required fields:",
		"How to proceed:",
		"mode\": \"tactical\"",
		"_skips\":",
		"_skip_reason\":",
		"References:",
		"FPF E.9",
		"haft_query(action=\"fpf\", mode=\"inspect\", identifier=\"E.9\")",
		"haft_query(action=\"fpf\", mode=\"inspect\", identifier=\"DEC-01\")",
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("validation error missing %q in output:\n%s", marker, got)
		}
	}
	if strings.Contains(got, `haft_query(action="fpf", query=`) {
		t.Fatalf("validation error advertises the retired FPF query shape:\n%s", got)
	}
}
