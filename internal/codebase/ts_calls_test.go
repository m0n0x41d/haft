package codebase

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestTSCrossModuleCallEdges verifies import-resolved call edges for TypeScript: a
// named import `{ helper }` and a namespace `ns.shared()` both resolve to their
// cross-file definitions, while an unresolved name and an instance-method call are
// dropped (no wrong edge).
func TestTSCrossModuleCallEdges(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()

	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	bar := `export function helper() { return 1 }
export function shared() { return 2 }
`
	main := `import { helper } from './bar'
import * as bar from './bar'

function run() {
  helper()
  bar.shared()
  missing()
  obj.method()
}
`
	write := func(rel, src string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join("pkg", "bar.ts"), bar)
	mainRel := filepath.Join("pkg", "main.ts")
	write(mainRel, main)

	jsts := &JSTSLang{}
	edges, err := jsts.ResolveFileEdges(ctx, root, mainRel, st)
	if err != nil {
		t.Fatal(err)
	}

	calls := map[string]bool{}
	for _, e := range edges {
		if e.Kind == EdgeCall {
			calls[edgeName(t, ctx, st, e.SrcID)+"->"+edgeName(t, ctx, st, e.DstID)] = true
		}
	}
	if !calls["run->helper"] {
		t.Errorf("missing named-import call edge run->helper; got %v", calls)
	}
	if !calls["run->shared"] {
		t.Errorf("missing namespace call edge run->shared; got %v", calls)
	}
	if len(calls) != 2 {
		t.Errorf("expected exactly 2 call edges (missing() and obj.method() dropped), got %d: %v", len(calls), calls)
	}
}

func TestTSResolverPersistsTruthfulOutcomes(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, source string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join("pkg", "target.ts"), "export function helper() { return 1 }\n")
	mainRel := filepath.Join("pkg", "main.ts")
	write(mainRel, `import { helper } from './target'

function run() {
  helper()
  missing()
  client.fetch()
}
`)

	resolver := &JSTSLang{}
	outcomes, err := resolver.ResolveFileEdgeOutcomes(ctx, root, mainRel, st)
	if err != nil {
		t.Fatal(err)
	}
	edges, diagnostics := PartitionEdgeResolutions(outcomes)
	if len(edges) != 1 {
		t.Fatalf("resolved edges = %+v", edges)
	}
	if edges[0].Origin != EdgeOriginNamedImport || edges[0].ResolutionMethod != ResolutionMethodExactSymbol || edges[0].SourceSnapshotHash == "" {
		t.Fatalf("resolved metadata = %+v", edges[0])
	}
	if len(diagnostics) != 2 {
		t.Fatalf("resolution diagnostics = %+v", diagnostics)
	}
	reasons := map[ResolutionReason]bool{}
	for _, diagnostic := range diagnostics {
		reasons[diagnostic.Reason] = true
	}
	if !reasons[ResolutionReasonNoCandidate] || !reasons[ResolutionReasonUnsupportedForm] {
		t.Fatalf("diagnostic reasons = %+v", reasons)
	}

	edgeStore := NewEdgeStore(st.db)
	if err := edgeStore.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := edgeStore.ReplaceFileResolutions(ctx, mainRel, outcomes); err != nil {
		t.Fatal(err)
	}
	stored, err := edgeStore.DiagnosticsByFile(ctx, mainRel)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("persisted diagnostics = %+v", stored)
	}
}

func TestTSResolverClassifiesExternalImport(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()
	rel := "external.ts"
	source := `import { readFile } from "node:fs"

export function run(): void {
  readFile("x", () => {})
}
`
	if err := os.WriteFile(filepath.Join(root, rel), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
		t.Fatal(err)
	}
	outcomes, err := (&JSTSLang{}).ResolveFileEdgeOutcomes(ctx, root, rel, st)
	if err != nil {
		t.Fatal(err)
	}
	edges, diagnostics := PartitionEdgeResolutions(outcomes)
	if len(edges) != 0 {
		t.Fatalf("external import emitted local edge: %+v", edges)
	}
	if len(diagnostics) != 1 || diagnostics[0].Reason != ResolutionReasonExternalDependency {
		t.Fatalf("external import diagnostics = %+v", diagnostics)
	}
}

// TestTSEmitterSynthesis confirms an intra-file EventEmitter dispatch synthesizes
// a dispatcher->handler edge: this.on("change", this.handleChange) paired with
// this.emit("change") wires update -> handleChange. A high-fan-out event is not
// tested here (capped); a generic event would be skipped.
func TestTSEmitterSynthesis(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()

	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `class Store {
  setup() {
    this.on('change', this.handleChange)
  }

  handleChange() {
    return 1
  }

  update() {
    this.emit('change')
  }
}
`
	rel := filepath.Join("pkg", "store.ts")
	if err := os.WriteFile(filepath.Join(root, rel), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
		t.Fatal(err)
	}

	jsts := &JSTSLang{}
	edges, err := jsts.ResolveFileEdges(ctx, root, rel, st)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range edges {
		if e.Kind == EdgeCallback && e.Provenance == ProvenanceHeuristic &&
			edgeName(t, ctx, st, e.SrcID) == "update" && edgeName(t, ctx, st, e.DstID) == "handleChange" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected synthesized emit->handler edge update->handleChange; got %v", edges)
	}
}

// TestTSCallbackEdges confirms a function passed as a callback argument gets a
// heuristic callback edge from the enclosing function, while a non-function
// argument is dropped.
func TestTSCallbackEdges(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()

	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	handlers := `export function onClick() { return 1 }
`
	main := `import { onClick } from './handlers'

function setup(btn) {
  btn.addEventListener('click', onClick)
  console.log(btn)
}
`
	write := func(rel, src string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join("pkg", "handlers.ts"), handlers)
	mainRel := filepath.Join("pkg", "main.ts")
	write(mainRel, main)

	jsts := &JSTSLang{}
	edges, err := jsts.ResolveFileEdges(ctx, root, mainRel, st)
	if err != nil {
		t.Fatal(err)
	}

	cb := 0
	found := false
	for _, e := range edges {
		if e.Kind != EdgeCallback {
			continue
		}
		cb++
		if edgeName(t, ctx, st, e.SrcID) == "setup" && edgeName(t, ctx, st, e.DstID) == "onClick" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing callback edge setup->onClick")
	}
	if cb != 1 {
		t.Errorf("expected exactly 1 callback edge (console.log(btn) arg is not a function), got %d", cb)
	}
}

// TestTSArrowAndObjectMethodCallEdges is the first idiomatic-TypeScript graph
// gate: exported const arrows become callable nodes, and calls inside exported
// object-literal methods are attributed to the method rather than the enclosing
// object constant. An unimported same-name function in a sibling module must not
// steal the imported edge.
func TestTSArrowAndObjectMethodCallEdges(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()

	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join("pkg", "helpers.ts"): `export const helper = () => 1
`,
		filepath.Join("pkg", "unrelated.ts"): `export const helper = () => 99
`,
		filepath.Join("pkg", "main.ts"): `import { helper } from './helpers'

const advanceAuthBoundaryRevision = () => 1
const persistSession = () => 2

export const run = () => helper()

export const session = {
  selectDevPersona() {
    advanceAuthBoundaryRevision()
    persistSession()
  },
}
`,
	}
	for rel, source := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
			t.Fatal(err)
		}
	}

	mainRel := filepath.Join("pkg", "main.ts")
	jsts := &JSTSLang{}
	edges, err := jsts.ResolveFileEdges(ctx, root, mainRel, st)
	if err != nil {
		t.Fatal(err)
	}

	calls := map[string]bool{}
	targetFiles := map[string]string{}
	for _, edge := range edges {
		if edge.Kind != EdgeCall {
			continue
		}
		src := edgeName(t, ctx, st, edge.SrcID)
		dst := edgeName(t, ctx, st, edge.DstID)
		calls[src+"->"+dst] = true
		target, ok, getErr := st.GetByID(ctx, edge.DstID)
		if getErr != nil || !ok {
			t.Fatalf("resolve edge target: %+v / %v", target, getErr)
		}
		targetFiles[src+"->"+dst] = target.FilePath
	}
	for _, expected := range []string{
		"run->helper",
		"selectDevPersona->advanceAuthBoundaryRevision",
		"selectDevPersona->persistSession",
	} {
		if !calls[expected] {
			t.Errorf("missing %s; got %v", expected, calls)
		}
	}
	if targetFiles["run->helper"] != filepath.Join("pkg", "helpers.ts") {
		t.Fatalf("import proof lost: run->helper targeted %q", targetFiles["run->helper"])
	}
	if len(calls) != 3 {
		t.Fatalf("expected exactly 3 sound calls, got %d: %v", len(calls), calls)
	}
}

func TestTSDirectCallShadowDoesNotBindImport(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()
	files := map[string]string{
		"helper.ts": `export function helper(): string { return "imported" }
`,
		"main.ts": `import { helper } from "./helper"

export function run(helper: () => string): string {
  return helper()
}
`,
	}
	for rel, source := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
			t.Fatal(err)
		}
	}
	outcomes, err := (&JSTSLang{}).ResolveFileEdgeOutcomes(ctx, root, "main.ts", st)
	if err != nil {
		t.Fatal(err)
	}
	edges, diagnostics := PartitionEdgeResolutions(outcomes)
	for _, edge := range edges {
		if edge.Kind == EdgeCall {
			t.Fatalf("shadowed parameter bound imported call: %+v", edge)
		}
	}
	foundShadowed := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == EdgeCall && diagnostic.Reason == ResolutionReasonShadowedBinding {
			foundShadowed = true
		}
	}
	if !foundShadowed {
		t.Fatalf("shadowed call diagnostic missing: %+v", diagnostics)
	}
}
