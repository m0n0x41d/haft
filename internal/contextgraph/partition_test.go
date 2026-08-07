package contextgraph

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
)

func partitionInvariants(
	invariants []graph.Invariant,
	symbol string,
	symbolLinked []*artifact.Artifact,
) (binding []graph.Invariant, contextInv []graph.Invariant) {
	if symbol == "" {
		return invariants, nil
	}
	symbolDecisions := make(map[string]bool, len(symbolLinked))
	for _, item := range symbolLinked {
		if item == nil ||
			item.Meta.Kind != artifact.KindDecisionRecord {
			continue
		}
		symbolDecisions[item.Meta.ID] = true
	}
	for _, invariant := range invariants {
		if symbolDecisions[invariant.DecisionID] {
			binding = append(binding, invariant)
			continue
		}
		contextInv = append(contextInv, invariant)
	}
	return binding, contextInv
}

// TestPartitionInvariants is the regression for the over-surfacing fix: a symbol
// view asserts "must hold here" only for invariants whose decision governs the
// symbol directly; a module-level (e.g. roadmap) invariant becomes context.
func TestPartitionInvariants(t *testing.T) {
	invs := []graph.Invariant{
		{Text: "symbol-bound rule", DecisionID: "dec-sym", DecisionTitle: "Symbol decision"},
		{Text: "phase-3 roadmap gate", DecisionID: "dec-mod", DecisionTitle: "Module roadmap"},
	}
	symbolLinked := []*artifact.Artifact{
		{Meta: artifact.Meta{ID: "dec-sym", Kind: artifact.KindDecisionRecord}},
	}

	// File-level view (no symbol): every invariant binds the file.
	bind, ctxInv := partitionInvariants(invs, "", nil)
	if len(bind) != 2 || len(ctxInv) != 0 {
		t.Fatalf("file view: expected 2 binding / 0 context, got %d / %d", len(bind), len(ctxInv))
	}

	// Symbol view: the symbol-governing decision binds; the module decision is context.
	bind, ctxInv = partitionInvariants(invs, "RelatedToFile", symbolLinked)
	if len(bind) != 1 || bind[0].DecisionID != "dec-sym" {
		t.Errorf("symbol-bound invariant must hold here, got %+v", bind)
	}
	if len(ctxInv) != 1 || ctxInv[0].DecisionID != "dec-mod" {
		t.Errorf("module-level roadmap invariant must be context, not 'must hold here', got %+v", ctxInv)
	}
}
