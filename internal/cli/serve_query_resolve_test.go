package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestHandleQuintQueryResolveTerm_RejectsMissingTerm(t *testing.T) {
	root := newResolveTermProject(t)
	haftDir := filepath.Join(root, ".haft")

	_, err := handleQuintQueryResolveTerm(context.Background(), nil, haftDir, map[string]any{})
	if err == nil {
		t.Fatalf("expected error when term missing")
	}
}

func TestHandleQuintQueryResolveTerm_AbsentWhenNoMatches(t *testing.T) {
	root := newResolveTermProject(t)
	haftDir := filepath.Join(root, ".haft")

	raw, err := handleQuintQueryResolveTerm(context.Background(), nil, haftDir, map[string]any{
		"term": "completely-unknown-term",
	})
	if err != nil {
		t.Fatalf("handleQuintQueryResolveTerm: %v", err)
	}

	var result ResolveTermResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, raw)
	}

	if result.Resolution != "absent" {
		t.Fatalf("resolution = %q, want absent", result.Resolution)
	}
	if !strings.Contains(result.NextAction, "term-map") {
		t.Fatalf("absent next_action should suggest term-map; got %q", result.NextAction)
	}
}

func TestHandleQuintQueryResolveTerm_ResolvedWhenSingleTermMapEntry(t *testing.T) {
	root := newResolveTermProject(t)
	writeResolveTermMap(t, root, "Harnessability\n    category: target\n    definition: Project ready for harness engineering.")
	haftDir := filepath.Join(root, ".haft")

	raw, err := handleQuintQueryResolveTerm(context.Background(), nil, haftDir, map[string]any{
		"term": "Harnessability",
	})
	if err != nil {
		t.Fatalf("handleQuintQueryResolveTerm: %v", err)
	}

	var result ResolveTermResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if result.Resolution != "resolved" {
		t.Fatalf("resolution = %q, want resolved", result.Resolution)
	}
	if len(result.TermMapEntries) != 1 {
		t.Fatalf("term_map_entries = %d, want 1", len(result.TermMapEntries))
	}
	if result.TermMapEntries[0].Term != "Harnessability" {
		t.Fatalf("term = %q, want Harnessability", result.TermMapEntries[0].Term)
	}
	if result.TermMapEntries[0].Category != "target" {
		t.Fatalf("category = %q, want target", result.TermMapEntries[0].Category)
	}
	if result.TermMapEntries[0].Domain != "target" {
		t.Fatalf("legacy domain mirror = %q, want target", result.TermMapEntries[0].Domain)
	}
}

func TestHandleQuintQueryResolveTerm_AmbiguousWithMultipleSpecSections(t *testing.T) {
	root := newResolveTermProject(t)
	// One term-map entry + two sections that mention the term — agent must
	// surface both candidates instead of guessing.
	writeResolveTermMap(t, root, "Harnessability\n    category: target\n    definition: Defined.")
	writeResolveTermSections(t, root)

	haftDir := filepath.Join(root, ".haft")
	raw, err := handleQuintQueryResolveTerm(context.Background(), nil, haftDir, map[string]any{
		"term": "Harnessability",
	})
	if err != nil {
		t.Fatalf("handleQuintQueryResolveTerm: %v", err)
	}

	var result ResolveTermResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(result.SpecSectionRefs) < 2 {
		t.Fatalf("expected at least two spec section refs; got %d", len(result.SpecSectionRefs))
	}
	if result.Resolution != "ambiguous" {
		t.Fatalf("resolution = %q, want ambiguous", result.Resolution)
	}
	if !strings.Contains(result.NextAction, "ambiguous") {
		t.Fatalf("ambiguous next_action should mention ambiguity; got %q", result.NextAction)
	}
}

func TestHandleQuintQueryResolveTerm_CaseInsensitiveTermMap(t *testing.T) {
	root := newResolveTermProject(t)
	writeResolveTermMap(t, root, "HarnessableProject\n    category: target\n    definition: Foo.")
	haftDir := filepath.Join(root, ".haft")

	raw, err := handleQuintQueryResolveTerm(context.Background(), nil, haftDir, map[string]any{
		"term": "harnessableproject",
	})
	if err != nil {
		t.Fatalf("handleQuintQueryResolveTerm: %v", err)
	}

	var result ResolveTermResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(result.TermMapEntries) != 1 {
		t.Fatalf("case-insensitive lookup should find term; got %d entries", len(result.TermMapEntries))
	}
}

func TestHandleQuintQueryResolveTermReadsSQLSectionsAndCarrierTermMap(t *testing.T) {
	root := setupSpecSyncProject(t)
	haftDir := filepath.Join(root, ".haft")
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.resolve.001",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "SQL resolve section",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		ValidUntil:    "2026-12-31",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
		Terms:         []string{"HarnessableProject"},
	}
	edition := specflow.NewSpecSectionEdition("qnt_spec_sync_test", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	raw, err := handleQuintQueryResolveTerm(context.Background(), nil, haftDir, map[string]any{
		"term": "HarnessableProject",
	})
	if err != nil {
		t.Fatalf("handleQuintQueryResolveTerm: %v", err)
	}

	var result ResolveTermResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, raw)
	}
	if len(result.TermMapEntries) != 1 {
		t.Fatalf("term-map carrier entry was not preserved: %#v", result.TermMapEntries)
	}
	if len(result.SpecSectionRefs) != 1 || result.SpecSectionRefs[0].ID != "TS.sql.resolve.001" {
		t.Fatalf("spec refs should come from SQL editions: %#v", result.SpecSectionRefs)
	}
}

// newResolveTermProject mirrors the readiness fixture but skips the
// active spec carriers (resolve_term doesn't need readiness=ready).
// Just .haft/specs/ exists so LoadProjectSpecificationSet succeeds.
func newResolveTermProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".haft", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeResolveTermMap(t *testing.T, root string, entry string) {
	t.Helper()
	body := "```yaml term-map\nentries:\n  - term: " + entry + "\n```\n"
	if err := os.WriteFile(filepath.Join(root, ".haft", "specs", "term-map.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeResolveTermSections(t *testing.T, root string) {
	t.Helper()
	body := "## TS.first.001\n\n" +
		"```yaml spec-section\n" +
		"id: TS.first.001\n" +
		"spec: target-system\n" +
		"kind: target.environment\n" +
		"title: First section using term\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2099-01-01\n" +
		"terms: [Harnessability]\n" +
		"```\n\n" +
		"## TS.second.001\n\n" +
		"```yaml spec-section\n" +
		"id: TS.second.001\n" +
		"spec: target-system\n" +
		"kind: target.role\n" +
		"title: Second section using term\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2099-01-01\n" +
		"terms: [Harnessability]\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(root, ".haft", "specs", "target-system.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
