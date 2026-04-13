package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHaftDecisionTool_EvidencePersistsValidUntil(t *testing.T) {
	store := setupHaftToolStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Keep benchmark evidence fresh",
		WhySelected:     "Need a target artifact for evidence attachment",
		SelectionPolicy: "Prefer the smallest decision artifact that still exercises evidence attachment against a real decision.",
		CounterArgument: "A synthetic decision record can miss coupling that appears in a real compare-driven decision.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Attach evidence to a note",
			Reason:  "This test explicitly needs a decision artifact target.",
		}},
		WeakestLink: "The decision is synthetic and therefore weaker than a real compared choice.",
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Evidence attachment stops preserving valid_until metadata"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	validUntil := time.Now().Add(96 * time.Hour).UTC().Format(time.RFC3339)
	tool := NewHaftDecisionTool(store, haftDir, t.TempDir(), nil)
	result, err := tool.Execute(ctx, mustJSON(t, map[string]any{
		"action":           "evidence",
		"artifact_ref":     decision.Meta.ID,
		"evidence_content": "Benchmark remains valid until the next load-test cycle.",
		"evidence_type":    "benchmark",
		"evidence_verdict": "supports",
		"valid_until":      validUntil,
	}))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.DisplayText, "Evidence attached:") {
		t.Fatalf("unexpected display text: %s", result.DisplayText)
	}

	items, err := store.GetEvidenceItems(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(items))
	}
	if items[0].ValidUntil != validUntil {
		t.Fatalf("valid_until = %q, want %q", items[0].ValidUntil, validUntil)
	}
}

func TestHaftDecisionTool_SchemaMarksValidUntilForEvidence(t *testing.T) {
	tool := NewHaftDecisionTool(setupHaftToolStore(t), t.TempDir(), t.TempDir(), nil)
	schema := tool.Schema()

	properties, ok := schema.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %T, want map[string]any", schema.Parameters["properties"])
	}

	validUntilField, ok := properties["valid_until"].(map[string]any)
	if !ok {
		t.Fatalf("valid_until schema = %T, want map[string]any", properties["valid_until"])
	}

	description, _ := validUntilField["description"].(string)
	if !strings.Contains(description, "evidence") {
		t.Fatalf("valid_until description should mention evidence, got %q", description)
	}
}
