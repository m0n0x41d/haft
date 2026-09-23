package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/code"
	"github.com/m0n0x41d/haft/internal/core/store"
)

func contextDecision(id, status string, selectors ...string) carrier.Record {
	return carrier.Record{Format: "haft/1", ID: id, Kind: "decision", Title: "Keep the declared cancellation boundary", Status: status, Origin: "operator_request", OperatorConfirmed: true, About: "domain:Billing.Order", CreatedAt: "2026-09-23T10:00:00Z", Object: "Cancellation", Question: "What boundary is accepted?", Disposition: "choose_now", Chosen: "domain-only", Why: "Bound the admitted operation", NoAlternativeReason: "The operator supplied one exact bounded choice", Constrains: selectors}
}

func contextFixtureView(t *testing.T, records ...carrier.Record) store.View {
	t.Helper()
	v := store.View{Files: map[string][]byte{}, Snapshots: map[string][]byte{}, CurrentSnapshots: map[string][]byte{}, Generation: "fixture-generation", Coverage: "complete"}
	for _, record := range records {
		contextAppendRecord(t, &v, record)
	}
	v.Projection = carrier.Project(v.Documents, v.Snapshots)
	return v
}

func contextAppendRecord(t *testing.T, v *store.View, record carrier.Record) string {
	t.Helper()
	raw := []byte(encode(t, record, []byte("Authored explanation for the context fixture.\n")))
	_, snapshot, digest, err := carrier.NewSnapshot(raw, carrier.InterpretationBasis{})
	if err != nil {
		t.Fatal(err)
	}
	d := carrier.Parse(raw)
	if !d.Valid() {
		t.Fatalf("invalid context fixture: %+v", d.Diagnostics)
	}
	d.Edition = digest
	v.Documents = append(v.Documents, d)
	path := "fixture/" + record.ID + ".md"
	v.DocumentPaths = append(v.DocumentPaths, path)
	v.Files[path] = raw
	v.Snapshots[digest] = snapshot
	v.CurrentSnapshots[digest] = snapshot
	return record.ID + "@" + digest
}

func contextIndex(t *testing.T) code.Index {
	t.Helper()
	i, err := code.IndexFromFiles(map[string][]byte{
		"go.mod":        []byte("module example.test/context\n\ngo 1.25\n"),
		"order.go":      []byte("package orders\nfunc Cancel() {}\nfunc Other() {}\n"),
		"policy.go":     []byte("package orders\nfunc Allowed() bool { return true }\n"),
		"order_test.go": []byte("package orders\nfunc TestCancel() {}\n"),
	}, code.Config{GOOS: "linux", GOARCH: "amd64", Toolchain: "go1.25.1", IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func contextBands(t *testing.T, v store.View, q Request) (Result, map[string]any) {
	t.Helper()
	r := Result{Kind: "results", Coverage: v.Coverage, Basis: map[string]string{}}
	data := map[string]any{}
	enrichContext(q, &r, data, v, contextIndex(t))
	return r, data
}

func contextIDs(rows ContextPage[ContextRecord]) map[string]string {
	out := map[string]string{}
	for _, row := range rows.Items {
		out[row.ID] = row.State
	}
	return out
}

func TestContextDeclaredConstraintsKeepPostureAndPackageProximitySeparate(t *testing.T) {
	a := contextDecision("dec-20260923-00000001", carrier.Active, "file:order.go")
	p := contextDecision("dec-20260923-00000002", carrier.Proposed, "sym:order.go::Cancel")
	p.Origin, p.OperatorConfirmed = "agent_proposal", false
	h := contextDecision("dec-20260923-00000003", carrier.Active, "dir:.")
	h.Origin, h.OperatorConfirmed = "migrated_9x", false
	neighbor := contextDecision("dec-20260923-00000004", carrier.Active, "file:policy.go")
	implementation := carrier.Record{Format: "haft/1", ID: "spec-20260923-00000001", Kind: "spec", Title: "Implementation trace only", Status: carrier.Active, Origin: "operator_request", OperatorConfirmed: true, About: a.About, CreatedAt: a.CreatedAt, Slug: "trace", ReceivingUse: "Review an implementation candidate", Claims: []carrier.Claim{{ID: "cancel", Kind: "law", Text: "Cancellation preserves totals", ImplementedBy: []carrier.Binding{{Ref: "sym:order.go::Cancel", Covers: "Direct domain path"}}, Unchecked: "Runner observation absent"}}}
	v := contextFixtureView(t, a, p, h, neighbor, implementation)
	_, data := contextBands(t, v, Request{Ref: "sym:order.go::Cancel"})
	constraints := data["constrained"].(ContextPage[ContextRecord])
	if got := contextIDs(constraints); !reflect.DeepEqual(got, map[string]string{a.ID: carrier.Active}) {
		t.Fatalf("false or absent governing constraint: %+v", got)
	}
	if constraints.Items[0].MatchKinds[0] != "declared_file_scope" {
		t.Fatal(constraints)
	}
	if got := contextIDs(data["proposed"].(ContextPage[ContextRecord])); got[p.ID] != carrier.Proposed {
		t.Fatal("proposed constraint absent", got)
	}
	related := contextIDs(data["related"].(ContextPage[ContextRecord]))
	if related[h.ID] != carrier.Historical || related[neighbor.ID] != carrier.Active || related[implementation.ID] != carrier.Active {
		t.Fatal("historical, package or implementation candidate absent", related)
	}
	bindings := data["bindings"].(ContextPage[ContextBinding])
	if bindings.Total != 1 || bindings.Items[0].OwnerState != carrier.Active || bindings.Items[0].Covers != "Direct domain path" {
		t.Fatal(bindings)
	}
	_, data = contextBands(t, v, Request{Ref: "sym:order.go::Other"})
	if got := contextIDs(data["constrained"].(ContextPage[ContextRecord])); len(got) != 1 || got[a.ID] != carrier.Active {
		t.Fatal("symbol-local constraint spread to other symbol", got)
	}
}

func TestContextContestedConstraintsNeverPickLatestBranch(t *testing.T) {
	a := contextDecision("dec-20260923-00000001", carrier.Active, "file:order.go")
	v := contextFixtureView(t, a)
	ar := v.Projection.Entries[0].Ref
	for _, id := range []string{"dec-20260923-00000002", "dec-20260923-00000003"} {
		b := a
		b.ID = id
		b.Supersedes = []string{ar}
		b.SupersedeReason = "Competing authored successor"
		contextAppendRecord(t, &v, b)
	}
	v.Projection = carrier.Project(v.Documents, v.Snapshots)
	_, data := contextBands(t, v, Request{Ref: "file:order.go"})
	constraints := data["constrained"].(ContextPage[ContextRecord])
	if constraints.Total != 2 || constraints.Items[0].State != carrier.Contested || constraints.Items[1].State != carrier.Contested {
		t.Fatal(constraints)
	}
	if got := contextIDs(data["related"].(ContextPage[ContextRecord])); got[a.ID] != carrier.Superseded {
		t.Fatal("old constraint not retained as history", got)
	}
}

func TestContextEvidencePaginationPreservesNegativeScopesAndOldTargets(t *testing.T) {
	a := contextDecision("dec-20260923-00000001", carrier.Active, "file:order.go")
	v := contextFixtureView(t, a)
	ar := v.Projection.Entries[0].Ref
	e := carrier.Record{Format: "haft/1", ID: "ev-20260923-00000001", Kind: "evidence", Title: "Opposing cancellation observations", Status: carrier.Active, Origin: "agent_proposal", About: a.About, CreatedAt: a.CreatedAt, ObservedAt: a.CreatedAt, Claim: "Observations differ by scope", Method: "Bounded fixture check", Source: "runs/result.json", Basis: &carrier.EvidenceBasis{Kind: "code", Ref: "runs/result.json"}, Uses: []carrier.EvidenceUse{{ID: "a-positive", Target: ar, Polarity: "supports", Scope: "Direct domain, paid orders"}, {ID: "b-negative", Target: ar, Polarity: "weakens", Scope: "Concurrent cancellation, untested transport assumption"}}}
	contextAppendRecord(t, &v, e)
	b := a
	b.ID = "dec-20260923-00000002"
	b.Supersedes = []string{ar}
	b.SupersedeReason = "Accepted narrower boundary"
	contextAppendRecord(t, &v, b)
	v.Projection = carrier.Project(v.Documents, v.Snapshots)
	_, first := contextBands(t, v, Request{Ref: "file:order.go", Limit: 1})
	ep := first["evidence"].(ContextEvidencePage)
	if ep.Total != 2 || !ep.Truncated || ep.NextOffset == nil || *ep.NextOffset != 1 || ep.PolarityCounts["supports"] != 1 || ep.PolarityCounts["weakens"] != 1 || !reflect.DeepEqual(ep.MixedPolarityTargets, []string{ar}) {
		t.Fatal(ep)
	}
	if ep.Items[0].TargetStatus != "found" || ep.Items[0].TargetCurrentness != "changed" || ep.Items[0].Use.Target != ar || ep.Items[0].CodeCurrentness != "unknown" || ep.Items[0].CheckCurrentness != "unknown" {
		t.Fatal("old observation silently transferred", ep)
	}
	_, second := contextBands(t, v, Request{Ref: "file:order.go", Limit: 1, Offset: 1})
	next := second["evidence"].(ContextEvidencePage)
	if next.Items[0].Use.Polarity != "weakens" || next.Items[0].Use.Scope != e.Uses[1].Scope || next.NextOffset != nil || !next.Truncated || next.Total != 2 {
		t.Fatal("negative evidence lost on continuation", next)
	}
	if !first["output_truncated"].(bool) {
		t.Fatal("output truncation not explicit")
	}
	// Removing durable bytes does not remove the observation or reattach its use.
	delete(v.Snapshots, strings.Split(ar, "@")[1])
	v.Projection = carrier.ProjectCaptured(v.Documents, v.CurrentSnapshots, v.Snapshots)
	_, missing := contextBands(t, v, Request{Ref: ar})
	ep = missing["evidence"].(ContextEvidencePage)
	if ep.Total != 2 || ep.Items[0].TargetStatus != "unresolved" || ep.Items[0].TargetCurrentness != "unknown" {
		t.Fatal("missing exact target hid the observation", ep)
	}
}

func TestImpactTypedEdgesMentionsAndSourceLocators(t *testing.T) {
	a := contextDecision("dec-20260923-00000001", carrier.Active, "file:order.go")
	a.Sources = []carrier.Source{{Ref: "A.6", PublicationPath: "FPF-Spec.md", SourceRevision: carrier.SourceRevision{Kind: "unknown", Reason: "Imported locator has no captured original"}}}
	a.Links = []carrier.Link{{Kind: "relates_to", Target: "file:missing.go", Reason: "Review after file removal"}, {Kind: "relies_on", Target: "note-20260923-00000001", Reason: "Declared premise"}}
	n := carrier.Record{Format: "haft/1", ID: "note-20260923-00000001", Kind: "note", Title: "Boundary note", Status: carrier.Active, Origin: "agent_proposal", About: a.About, CreatedAt: a.CreatedAt}
	v := contextFixtureView(t, a, n)
	v.Files["notes/unknown.md"] = []byte("Unknown document mentions file:order.go; no typed assertion.\n")
	_, data := contextBands(t, v, Request{Ref: "file:order.go"})
	kinds := map[string]bool{}
	for _, edge := range data["linked"].(ContextPage[ContextEdge]).Items {
		kinds[edge.Kind] = true
		if edge.From == "" || edge.To == "" || edge.Direction == "" || edge.Path == "" {
			t.Fatal(edge)
		}
		if edge.Kind == "constraint" && edge.Direction != "incoming" {
			t.Fatal("constraint direction is relative to its requested file target", edge)
		}
	}
	for _, kind := range []string{"constraint", "navigation", "premise", "source"} {
		if !kinds[kind] {
			t.Fatal("typed relation absent", kind, kinds)
		}
	}
	unresolved := data["unresolved"].(ContextPage[ContextEdge])
	if unresolved.Total != 2 {
		t.Fatal("missing file and unpinned source need separate unresolved paths", unresolved)
	}
	mentions := data["mentioned"].(ContextPage[ContextMention])
	foundUnknown := false
	for _, mention := range mentions.Items {
		if mention.Path == "notes/unknown.md" {
			foundUnknown = true
			if mention.RecordID != "" {
				t.Fatal("text mention promoted", mention)
			}
		}
	}
	if !foundUnknown {
		t.Fatal("untyped mention omitted")
	}
	for _, locator := range []string{"A.6", "source:A.6", "FPF-Spec.md"} {
		r, result := contextBands(t, v, Request{Ref: locator})
		if r.Kind != "results" || contextIDs(result["related"].(ContextPage[ContextRecord]))[a.ID] != carrier.Active {
			t.Fatal("source reverse review path missing", locator, result)
		}
		for _, edge := range result["linked"].(ContextPage[ContextEdge]).Items {
			if edge.Kind == "source" && (edge.Direction != "incoming" || edge.Source == nil || edge.Source.PublicationPath != "FPF-Spec.md") {
				t.Fatal("source provenance or direction lost", edge)
			}
		}
	}
}

func TestContextPublicBoundaryAndStablePagination(t *testing.T) {
	s := service(t)
	a := contextDecision("dec-20260923-00000001", carrier.Active, "file:order.go")
	a.Sources = []carrier.Source{{Ref: "A.6.B", PublicationPath: "FPF-Spec.md", SourceRevision: carrier.SourceRevision{Kind: "unknown", Reason: "Only the authored locator was captured"}}}
	run(t, s, Request{Operation: "remember", Carrier: encode(t, a, nil)}, "written")
	r := run(t, s, Request{Operation: "context", Ref: "file:order.go", Limit: 1}, "exact")
	data := r.Data.(map[string]any)
	if got := contextIDs(data["constrained"].(ContextPage[ContextRecord])); got[a.ID] != carrier.Active {
		t.Fatal("public context omitted claimless decision constraint", got)
	}
	if data["search_scope"].(map[string]any)["memory_generation"] != r.Basis["memory_generation"] {
		t.Fatal("scope did not report captured memory basis")
	}
	for _, locator := range []string{"A.6.B", "source:A.6.B", "FPF-Spec.md"} {
		impact := run(t, s, Request{Operation: "impact", Ref: locator}, "results")
		bands := impact.Data.(map[string]any)
		if got := contextIDs(bands["related"].(ContextPage[ContextRecord])); got[a.ID] != carrier.Active {
			t.Fatal("public source traversal lost its referring carrier", locator, got)
		}
		if _, present := bands["memory"]; present {
			t.Fatal("valid source locator retained an invalid record resolution", bands["memory"])
		}
		for _, diagnostic := range impact.Diagnostics {
			if diagnostic.Code == "invalid_ref" {
				t.Fatal("source locator reported as malformed native ID", impact)
			}
		}
	}
	// A complete local capture remains complete when only its output is paged.
	root := t.TempDir()
	for path, raw := range map[string]string{"go.mod": "module example.test/paging\n\ngo 1.25\n", "a.go": "package paging\nfunc A() {}\nfunc B() {}\nfunc C() {}\n"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paging := Service{Root: root}
	r = run(t, paging, Request{Operation: "context", Limit: 1}, "results")
	data = r.Data.(map[string]any)
	page := data["symbols"].(ContextPage[code.Symbol])
	if page.Total != 3 || !page.Truncated || page.NextOffset == nil || !data["output_truncated"].(bool) || r.Coverage != "complete" {
		t.Fatal(r, page)
	}
	r2 := run(t, paging, Request{Operation: "context", Limit: 1, Offset: 1}, "results")
	p2 := r2.Data.(map[string]any)["symbols"].(ContextPage[code.Symbol])
	if p2.Items[0].Anchor == page.Items[0].Anchor || p2.Total != 3 {
		t.Fatal("pagination repeated first symbol", p2)
	}
	for _, offset := range []int{-1, 100001} {
		got := paging.Execute(context.Background(), Request{Format: Format, Operation: "context", Offset: offset})
		if got.Kind != "invalid" || got.Diagnostics[0].Code != "invalid_offset" {
			t.Fatal(got)
		}
	}
}
