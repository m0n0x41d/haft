package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBindingSurfaceInventoryIsUniqueAndClassified(t *testing.T) {
	seen := map[string]struct{}{}
	for _, entry := range bindingSurfaceInventory() {
		if strings.TrimSpace(entry.Tool) == "" {
			t.Fatalf("inventory entry with empty tool: %#v", entry)
		}
		key := entry.Tool + ":" + entry.Action
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate inventory entry for %s", key)
		}
		seen[key] = struct{}{}
		if !validBindingSurfaceClass(entry.Class) {
			t.Fatalf("%s has invalid class %q", key, entry.Class)
		}
		if !validBindingSurfaceEnforcement(entry.Enforcement) {
			t.Fatalf("%s has invalid enforcement %q", key, entry.Enforcement)
		}
		if entry.Enforcement == bindingEnforcementMCPOperatorConfirmationRequired && strings.TrimSpace(entry.AllowedPath) == "" {
			t.Fatalf("%s requires operator confirmation but has empty allowed path", key)
		}
	}
}

func TestMCPBindingGateMatchesBindingSurfaceInventory(t *testing.T) {
	for _, entry := range bindingSurfaceInventory() {
		got := mcpActionRequiresOperatorConfirmation(entry.Tool, entry.Action)
		want := entry.Enforcement == bindingEnforcementMCPOperatorConfirmationRequired
		if got != want {
			t.Fatalf("%s:%s gate = %v, want %v from inventory entry %#v", entry.Tool, entry.Action, got, want, entry)
		}
	}
}

func TestBindingSurfaceInventoryCoversKnownAuthorityMutations(t *testing.T) {
	for _, item := range []struct {
		tool   string
		action string
		class  string
	}{
		{tool: "haft_decision", action: "decide", class: bindingSurfaceBindingAuthorityMutation},
		{tool: "haft_commission", action: "create", class: bindingSurfaceExecutionAuthorityMutation},
		{tool: "haft_commission", action: "create_from_decision", class: bindingSurfaceExecutionAuthorityMutation},
		{tool: "haft_commission", action: "create_batch_from_decisions", class: bindingSurfaceExecutionAuthorityMutation},
		{tool: "haft_commission", action: "create_from_plan", class: bindingSurfaceExecutionAuthorityMutation},
		{tool: "haft_spec_section", action: "approve", class: bindingSurfaceLifecycleAuthorityMutation},
		{tool: "haft_spec_section", action: "rebaseline", class: bindingSurfaceLifecycleAuthorityMutation},
		{tool: "haft_spec_section", action: "reopen", class: bindingSurfaceLifecycleAuthorityMutation},
		{tool: "haft_refresh", action: "waive", class: bindingSurfaceLifecycleAuthorityMutation},
		{tool: "haft_refresh", action: "reopen", class: bindingSurfaceLifecycleAuthorityMutation},
		{tool: "haft_refresh", action: "supersede", class: bindingSurfaceLifecycleAuthorityMutation},
		{tool: "haft_refresh", action: "deprecate", class: bindingSurfaceLifecycleAuthorityMutation},
	} {
		entry, ok := lookupBindingSurface(item.tool, item.action)
		if !ok {
			t.Fatalf("missing inventory entry for %s:%s", item.tool, item.action)
		}
		if entry.Class != item.class {
			t.Fatalf("%s:%s class = %q, want %q", item.tool, item.action, entry.Class, item.class)
		}
		if entry.Enforcement != bindingEnforcementMCPOperatorConfirmationRequired {
			t.Fatalf("%s:%s enforcement = %q, want operator confirmation", item.tool, item.action, entry.Enforcement)
		}
	}
}

func TestAuthorityBoundaryCarrierWording(t *testing.T) {
	projectRoot := testProjectRoot(t)
	requiredBoundaryPhrases := map[string][]string{
		"README.md": {
			"model-supplied",
			"not proof of operator authorization",
		},
		".haft/specs/target-system.md": {
			"model-supplied",
			"satisfy neither predicate",
		},
		".haft/specs/software-system.md": {
			"model-supplied",
			"as substitutes for",
		},
		"internal/cli/skill/h-decide/SKILL.md": {
			"model-supplied",
			"not proof of operator authorization",
		},
		"internal/cli/skill/h-commission/SKILL.md": {
			"model-supplied",
			"not proof of operator authorization",
		},
	}
	for path, phrases := range requiredBoundaryPhrases {
		text := normalizedCarrierText(readProjectFile(t, projectRoot, path))
		for _, phrase := range phrases {
			if !strings.Contains(text, phrase) {
				t.Fatalf("%s missing authority-boundary phrase %q", path, phrase)
			}
		}
	}

	bannedClaims := []string{
		"mcp can bind decisions",
		"mcp can create binding decisions",
		"model-supplied arguments are proof",
		"prompt text is proof of operator authorization",
		"prompt convention is authority proof",
	}
	for _, path := range []string{
		"README.md",
		".haft/specs/target-system.md",
		".haft/specs/software-system.md",
		"internal/cli/skill/h-decide/SKILL.md",
		"internal/cli/skill/h-commission/SKILL.md",
		"internal/fpf/server.go",
		"internal/cli/interface.go",
	} {
		text := strings.ToLower(readProjectFile(t, projectRoot, path))
		for _, claim := range bannedClaims {
			if strings.Contains(text, claim) {
				t.Fatalf("%s contains banned authority claim %q", path, claim)
			}
		}
	}
}

func normalizedCarrierText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

func validBindingSurfaceClass(value string) bool {
	switch value {
	case bindingSurfaceReadOnly,
		bindingSurfaceDraftOnly,
		bindingSurfaceEvidenceRecording,
		bindingSurfaceBindingAuthorityMutation,
		bindingSurfaceLifecycleAuthorityMutation,
		bindingSurfaceExecutionAuthorityMutation:
		return true
	default:
		return false
	}
}

func validBindingSurfaceEnforcement(value string) bool {
	switch value {
	case bindingEnforcementMCPAllowed,
		bindingEnforcementMCPAllowedExistingArtifact,
		bindingEnforcementMCPAllowedMaintenancePolicyGated,
		bindingEnforcementMCPOperatorConfirmationRequired:
		return true
	default:
		return false
	}
}

func testProjectRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func readProjectFile(t *testing.T, root string, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
