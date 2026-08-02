package codeintel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/contextgraph"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

// TestTypeScriptQualifiedNodeRetainsGovernanceFusion proves the new TypeScript
// nodes are not a bare parallel graph. A qualified object method resolves through
// the public codeintel service, returns byte-exact source and call edges, and each
// view still carries the governing decision, invariant, and SpecSection ref.
func TestTypeScriptQualifiedNodeRetainsGovernanceFusion(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	files := map[string]string{
		filepath.Join("src", "helpers.ts"): `export const advanceAuthBoundaryRevision = () => 1
export const persistSession = () => 2
`,
		filepath.Join("src", "session.ts"): `import { advanceAuthBoundaryRevision, persistSession } from './helpers'

export const session = {
  selectDevPersona(consultantId: string): void {
    advanceAuthBoundaryRevision()
    persistSession()
  },
}
`,
	}
	for rel, source := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	legacy, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(root, "haft.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = legacy.Close() })
	raw := legacy.GetRawDB()
	store := artifact.NewStore(raw)
	now := time.Now().UTC()
	section := project.SpecSection{
		ID:            "TS.behavior.001",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "Session transitions remain explicit",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "human",
		Status:        "active",
		ValidUntil:    now.AddDate(0, 3, 0).Format("2006-01-02"),
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	sectionJSON, err := json.Marshal(section)
	if err != nil {
		t.Fatal(err)
	}
	sectionHash := specflow.HashSection(section)
	if _, err := raw.ExecContext(ctx,
		`INSERT INTO spec_section_editions
		 (project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"project-ts-fusion",
		section.ID,
		sectionHash,
		string(sectionJSON),
		"carrier_import",
		section.Path,
		now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx,
		`INSERT INTO spec_section_baselines (project_id, section_id, hash, captured_at, approved_by)
		 VALUES (?, ?, ?, ?, ?)`,
		"project-ts-fusion",
		section.ID,
		sectionHash,
		now,
		"operator",
	); err != nil {
		t.Fatal(err)
	}
	decision := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        "dec-ts-fusion",
			Kind:      artifact.KindDecisionRecord,
			Status:    artifact.StatusActive,
			Title:     "Keep session transitions explicit",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body:           "Session transitions stay explicit.",
		StructuredData: `{"invariants":["session transitions remain explicit"],"section_refs":["TS.behavior.001"],"binding_targets":[{"kind":"whole_file_fallback","file_path":"src/session.ts"}]}`,
	}
	if err := store.Create(ctx, decision); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx,
		`INSERT INTO affected_files (artifact_id, file_path) VALUES (?, ?)`,
		decision.Meta.ID,
		filepath.Join("src", "session.ts"),
	); err != nil {
		t.Fatal(err)
	}

	service := NewService(store)
	view, err := service.Node(ctx, root, "session.selectDevPersona", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Found || len(view.Overloads) != 1 {
		t.Fatalf("qualified TypeScript method not resolved: %+v", view)
	}
	overload := view.Overloads[0]
	if overload.Symbol.Name != "selectDevPersona" || overload.Symbol.Receiver != "session" {
		t.Fatalf("qualified seed resolved wrong node: %+v", overload.Symbol)
	}
	if !overload.BodyOK || !strings.Contains(overload.Body, "selectDevPersona") {
		t.Fatalf("node body is not byte-exact/current: ok=%v body=%q", overload.BodyOK, overload.Body)
	}
	if len(overload.Callees) != 2 {
		t.Fatalf("expected two imported callees, got %+v", overload.Callees)
	}
	if len(overload.Context.Decisions) != 1 || overload.Context.Decisions[0].Meta.ID != decision.Meta.ID {
		t.Fatalf("TypeScript node lost decision fusion: %+v", overload.Context)
	}
	if len(overload.Context.Invariants) != 1 || overload.Context.Invariants[0].Text != "session transitions remain explicit" {
		t.Fatalf("TypeScript node lost invariant context: %+v", overload.Context)
	}
	if len(overload.Context.Specs) != 1 {
		t.Fatalf("TypeScript node lost typed SpecSection context: %+v", overload.Context)
	}
	if overload.Context.Specs[0].ID != section.ID || overload.Context.Specs[0].Resolution != contextgraph.SpecResolutionResolved {
		t.Fatalf("TypeScript node resolved wrong SpecSection: %+v", overload.Context.Specs[0])
	}
	if overload.Context.Specs[0].BaselineState != contextgraph.SpecBaselineCurrent {
		t.Fatalf("TypeScript node lost current SpecSection baseline: %+v", overload.Context.Specs[0])
	}
}

func TestSplitQualifiedSymbolName(t *testing.T) {
	tests := map[string][2]string{
		"session.selectDevPersona": {"selectDevPersona", "session"},
		"Store.Get":                {"Get", "Store"},
		"plain":                    {"plain", ""},
	}
	for input, expected := range tests {
		name, receiver := splitQualifiedSymbolName(input)
		if name != expected[0] || receiver != expected[1] {
			t.Errorf("splitQualifiedSymbolName(%q) = %q/%q, want %q/%q", input, name, receiver, expected[0], expected[1])
		}
	}
}
