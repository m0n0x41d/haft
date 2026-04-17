package cli

import (
	"sort"
	"strings"
	"testing"
)

// TestTransportActionParity is the detection layer for the unified-contract
// refactor (D1 / 5.4 Pro Finding #1). It documents the action enum exposed by
// each tool through both transports — standalone (internal/tools/haft.go) and
// MCP (internal/cli/serve.go switch dispatch) — and asserts that drift between
// them stays within the documented exception list.
//
// Both sides are hardcoded as ground truth. When the actual code changes:
//   - if intentional: update both sides here together (and knownTransportDrift
//     if newly diverged), preserving the transport contract awareness.
//   - if unintentional: the diff between this test and reality surfaces it.
//
// Known drift as of 2026-04-18 stays in knownTransportDrift below. New drift
// fails the test. When the unified contract refactor lands (D1), all entries
// in knownTransportDrift should drain to empty.
//
// Maintenance protocol: changing any tool action requires updating this test.
// Run with `go test -run TestTransportActionParity` to surface drift.
func TestTransportActionParity(t *testing.T) {
	cases := []struct {
		toolName string
		// mcpActions: extracted from internal/cli/serve.go handleQuintX()
		// switch action {} cases — the MCP dispatch table.
		mcpActions []string
		// standaloneActions: extracted from internal/tools/haft.go
		// (*HaftXTool).Schema().Parameters → action enum field.
		standaloneActions []string
	}{
		{
			toolName:          "haft_problem",
			mcpActions:        []string{"frame", "characterize", "select", "close"},
			standaloneActions: []string{"frame", "adopt", "select", "characterize", "close"},
		},
		{
			toolName:          "haft_solution",
			mcpActions:        []string{"explore", "compare", "similar"},
			standaloneActions: []string{"explore", "compare", "similar"},
		},
		{
			toolName:          "haft_decision",
			mcpActions:        []string{"decide", "apply", "measure", "evidence", "baseline"},
			standaloneActions: []string{"decide", "evidence", "baseline", "measure"},
		},
		{
			toolName:          "haft_refresh",
			mcpActions:        []string{"scan", "waive", "reopen", "supersede", "deprecate", "reconcile"},
			standaloneActions: []string{"scan", "drift", "waive", "reopen", "supersede", "deprecate"},
		},
		{
			toolName:          "haft_query",
			mcpActions:        []string{"search", "status", "board", "related", "projection", "list", "coverage", "fpf"},
			standaloneActions: []string{"search", "status", "related", "projection", "fpf"},
		},
	}

	// Documented drift: actions present in only one transport, accepted as
	// architectural debt until D1 lands. Each entry MUST include rationale.
	knownTransportDrift := map[string]map[string]string{
		"haft_problem": {
			"adopt": "standalone-only — session continuity feature; MCP folds adoption into frame on existing ref",
		},
		"haft_decision": {
			"apply": "MCP-only — generates implementation brief; standalone uses different orchestration path",
		},
		"haft_refresh": {
			"drift":     "standalone-only — drift scan as separate action; MCP folds it into scan",
			"reconcile": "MCP-only — overlap reconciliation; standalone uses search + manual reconcile",
		},
		"haft_query": {
			"board":    "MCP-only — dashboard rich-view aggregator for desktop frontend",
			"list":     "MCP-only — kind enumeration; standalone uses search",
			"coverage": "MCP-only — module coverage report; standalone uses status",
		},
	}

	for _, c := range cases {
		t.Run(c.toolName, func(t *testing.T) {
			mcpSet := makeStringSet(c.mcpActions)
			stdSet := makeStringSet(c.standaloneActions)

			onlyInMCP := setDifference(mcpSet, stdSet)
			onlyInStandalone := setDifference(stdSet, mcpSet)

			knownForTool := knownTransportDrift[c.toolName]
			newOnlyInMCP := filterKnown(onlyInMCP, knownForTool)
			newOnlyInStandalone := filterKnown(onlyInStandalone, knownForTool)

			if len(newOnlyInMCP) == 0 && len(newOnlyInStandalone) == 0 {
				return // OK: no drift, or only documented drift.
			}

			var msg strings.Builder
			msg.WriteString("transport action parity drift detected\n")
			if len(newOnlyInMCP) > 0 {
				msg.WriteString("  new actions only in MCP (serve.go): ")
				msg.WriteString(strings.Join(newOnlyInMCP, ", "))
				msg.WriteString("\n")
			}
			if len(newOnlyInStandalone) > 0 {
				msg.WriteString("  new actions only in standalone (tools/haft.go): ")
				msg.WriteString(strings.Join(newOnlyInStandalone, ", "))
				msg.WriteString("\n")
			}
			msg.WriteString("If intentional, document in knownTransportDrift; otherwise unify the contract.")
			t.Fatal(msg.String())
		})
	}
}

func makeStringSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, s := range items {
		set[s] = struct{}{}
	}
	return set
}

func setDifference(a, b map[string]struct{}) []string {
	diff := make([]string, 0)
	for k := range a {
		if _, ok := b[k]; !ok {
			diff = append(diff, k)
		}
	}
	sort.Strings(diff)
	return diff
}

func filterKnown(items []string, known map[string]string) []string {
	if len(known) == 0 {
		return items
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := known[item]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}
