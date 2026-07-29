package agent

import (
	"slices"
	"testing"
)

func TestVisibleSubagentsExposeIndependentCapabilities(t *testing.T) {
	visible := VisibleSubagents()
	names := make([]string, 0, len(visible))
	for _, def := range visible {
		names = append(names, def.Name)
	}

	want := []string{"explore", "verify", "plan"}
	if !slices.Equal(names, want) {
		t.Fatalf("visible subagents = %v, want independent capabilities %v", names, want)
	}

	legacyPhases := []string{"framer", "explorer", "comparer", "decider", "worker", "measure"}
	for _, phase := range legacyPhases {
		if slices.Contains(names, phase) {
			t.Fatalf("visible subagents include legacy ordered phase %q", phase)
		}
	}
}

func TestBuiltinSubagentsRetainHiddenProtocolHelpers(t *testing.T) {
	defs := BuiltinSubagents()
	for _, name := range []string{"title", "compact"} {
		def, ok := defs[name]
		if !ok {
			t.Fatalf("missing hidden protocol helper %q", name)
		}
		if !def.Hidden {
			t.Fatalf("protocol helper %q must remain hidden", name)
		}
	}
}
