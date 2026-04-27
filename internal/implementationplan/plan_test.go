package implementationplan

import (
	"strings"
	"testing"
)

func TestParsePayloadModelsDAG(t *testing.T) {
	plan, err := ParsePayload(map[string]any{
		"id":       "plan-core-001",
		"revision": "p1",
		"decisions": []any{
			map[string]any{
				"ref":     "dec-a",
				"lockset": []any{"internal/a.go"},
			},
			map[string]any{
				"ref":        "dec-b",
				"depends_on": []any{"dec-a"},
				"lockset":    []any{"internal/b.go"},
			},
			map[string]any{
				"ref":        "dec-c",
				"depends_on": []any{"dec-a", "dec-b"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if plan.ID != "plan-core-001" {
		t.Fatalf("id = %q, want plan-core-001", plan.ID)
	}
	if plan.Revision != "p1" {
		t.Fatalf("revision = %q, want p1", plan.Revision)
	}

	refs := plan.DecisionRefs()
	if len(refs) != 3 {
		t.Fatalf("decision refs = %#v, want three", refs)
	}
	if refs[0] != "dec-a" || refs[1] != "dec-b" || refs[2] != "dec-c" {
		t.Fatalf("decision refs = %#v, want input order", refs)
	}

	edges := plan.DependencyEdges()
	if len(edges) != 3 {
		t.Fatalf("dependency edges = %#v, want three", edges)
	}
	if edges[0].DecisionRef != "dec-b" || edges[0].DependsOn != "dec-a" {
		t.Fatalf("first dependency edge = %#v, want dec-b -> dec-a", edges[0])
	}
}

func TestParsePayloadRejectsCycle(t *testing.T) {
	_, err := ParsePayload(map[string]any{
		"id":       "plan-core-cycle",
		"revision": "p1",
		"decisions": []any{
			map[string]any{
				"ref":        "dec-a",
				"depends_on": []any{"dec-b"},
			},
			map[string]any{
				"ref":        "dec-b",
				"depends_on": []any{"dec-a"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("error = %v, want dependency cycle", err)
	}
}

func TestParsePayloadRejectsImpossibleDependency(t *testing.T) {
	_, err := ParsePayload(map[string]any{
		"id":       "plan-core-impossible",
		"revision": "p1",
		"decisions": []any{
			map[string]any{
				"ref":        "dec-a",
				"depends_on": []any{"dec-missing"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "depends on unknown decision dec-missing") {
		t.Fatalf("error = %v, want unknown dependency rejection", err)
	}
}

func TestParsePayloadRejectsNonStringDependencyCarrier(t *testing.T) {
	_, err := ParsePayload(map[string]any{
		"id":       "plan-core-carrier",
		"revision": "p1",
		"decisions": []any{
			map[string]any{
				"ref":        "dec-a",
				"depends_on": []any{123},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "depends_on[0] must be a string") {
		t.Fatalf("error = %v, want strict dependency carrier rejection", err)
	}
}

func TestDependenciesSatisfied(t *testing.T) {
	satisfied := map[string]bool{
		"wc-a": true,
		"wc-b": true,
		"wc-c": false,
	}

	if !DependenciesSatisfied([]string{"wc-a", "wc-b"}, satisfied) {
		t.Fatal("dependencies satisfied = false, want true")
	}
	if DependenciesSatisfied([]string{"wc-a", "wc-c"}, satisfied) {
		t.Fatal("dependencies satisfied = true, want false for incomplete dependency")
	}
	if DependenciesSatisfied([]string{"wc-a", "wc-missing"}, satisfied) {
		t.Fatal("dependencies satisfied = true, want false for missing dependency")
	}
}

func TestLocksetsOverlap(t *testing.T) {
	if !LocksetsOverlap([]string{"internal/cli/**"}, []string{"internal/cli/serve.go"}) {
		t.Fatal("locksets overlap = false, want true")
	}
	if !LocksetsOverlap([]string{"**/*"}, []string{"open-sleigh/lib/open_sleigh/work_commission.ex"}) {
		t.Fatal("wildcard locksets overlap = false, want true")
	}
	if LocksetsOverlap([]string{"internal/cli/**"}, []string{"open-sleigh/**"}) {
		t.Fatal("locksets overlap = true, want false")
	}
}
