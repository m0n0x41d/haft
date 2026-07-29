package initplanning

import (
	"io/fs"
	"strings"
	"testing"
)

func TestManagedJSONCRewriteEditionPreservesUnownedSemantics(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	desired, err := NewJSONObjectEntryFragment(
		carrierPath,
		ComponentMCP,
		[]string{"context_servers", "Haft"},
		[]byte(`{"command":"haft","args":["serve"]}`),
		fs.FileMode(0o644),
		ManagedJSONCRewriteMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment: %v", err)
	}
	legacy := mustManagedFragmentLegacyRegistry(
		t,
		[]ManagedFragmentRecord{desired.Record()},
		mustLegacyOwnershipBasis(t),
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`// Zed settings
{
  "theme": "dark",
  "documentation": "https://example.test//guide",
  "agent_servers": {
    "codex-acp": {
      "type": "registry",
    },
  },
  "context_servers": {
    "Pieces": {
      "command": "/opt/pieces",
      "args": ["mcp", "start"], /* retained server */
    },
  },
}
`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		legacy,
		input,
	)
	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	if !result.Changed() {
		t.Fatal("missing managed Zed fragment did not produce a rewrite")
	}
	value, err := decodeUniqueJSON(result.Content())
	if err != nil {
		t.Fatalf("rewritten JSONC carrier is not strict JSON: %v", err)
	}
	root, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("rewritten root = %#v, want object", value)
	}
	if root["theme"] != "dark" ||
		root["documentation"] != "https://example.test//guide" {
		t.Fatalf("unowned scalar values changed: %#v", root)
	}
	agents, ok := root["agent_servers"].(map[string]any)
	if !ok || agents["codex-acp"] == nil {
		t.Fatalf("agent_servers changed: %#v", root["agent_servers"])
	}
	servers, ok := root["context_servers"].(map[string]any)
	if !ok || servers["Pieces"] == nil || servers["Haft"] == nil {
		t.Fatalf("context_servers changed: %#v", root["context_servers"])
	}
}

func TestManagedJSONCRewriteEditionRemainsFailClosed(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	jsoncFragment, err := NewJSONObjectEntryFragment(
		carrierPath,
		ComponentMCP,
		[]string{"context_servers", "Haft"},
		[]byte(`{"command":"haft","args":["serve"]}`),
		fs.FileMode(0o644),
		ManagedJSONCRewriteMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment(JSONC): %v", err)
	}
	strictFragment, err := NewJSONObjectEntryFragment(
		carrierPath,
		ComponentMCP,
		[]string{"context_servers", "Haft"},
		[]byte(`{"command":"haft","args":["serve"]}`),
		fs.FileMode(0o644),
		managedFragmentMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment(strict): %v", err)
	}
	jsoncPlan, err := BuildManagedFragmentObservationPlan(
		[]ManagedFragment{jsoncFragment},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan(JSONC): %v", err)
	}
	strictPlan, err := BuildManagedFragmentObservationPlan(
		[]ManagedFragment{strictFragment},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan(strict): %v", err)
	}
	commented := mustPresentManagedCarrier(
		t,
		carrierPath,
		"{\n  // comment\n  \"context_servers\": {},\n}\n",
	)
	if _, err := ObserveManagedCarrier(
		strictPlan,
		commented,
	); err == nil {
		t.Fatal("ordinary JSON merge edition accepted JSONC")
	}

	duplicate := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"context_servers":{"Haft":{"command":"one"},"Haft":{"command":"two"}}}`,
	)
	if _, err := ObserveManagedCarrier(
		jsoncPlan,
		duplicate,
	); err == nil || !strings.Contains(
		err.Error(),
		"duplicate JSON object key",
	) {
		t.Fatalf("duplicate JSONC key error = %v", err)
	}

	unterminated := mustPresentManagedCarrier(
		t,
		carrierPath,
		"{ /* unfinished\n  \"context_servers\": {}\n}\n",
	)
	if _, err := ObserveManagedCarrier(
		jsoncPlan,
		unterminated,
	); err == nil || !strings.Contains(
		err.Error(),
		"unterminated block comment",
	) {
		t.Fatalf("unterminated JSONC comment error = %v", err)
	}
}
