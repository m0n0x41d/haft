package carrier

import "testing"

func TestCapturedEditionsDoNotResolvePinnedDependencies(t *testing.T) {
	current := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, current)
	e := edit(t, fixture(t, "valid/evidence.md"), func(r *Record) { r.Uses[0].Target = ar + "#L1" })
	e, _ = snap(t, e, InterpretationBasis{}, current)
	p := ProjectCaptured([]Document{a, e}, current, nil)
	if p.Entries[0].State != Active || p.Resolve("spec:order-cancel#L1").Kind != "found" {
		t.Fatal("ordinary unpersisted live carrier lost its verified identity")
	}
	if p.Resolve(ar+"#L1").Kind != "unresolved" || !hasCode(p.Entries[1].Diagnostics, "unresolved_evidence_target") || len(p.UsesFor(ar+"#L1")) != 0 {
		t.Fatal("computed current bytes repaired a missing pinned evidence target")
	}
	matches := p.EvidenceUsesFor(ar + "#L1")
	if len(matches) != 1 || matches[0].TargetStatus != "unresolved" || matches[0].State != Active {
		t.Fatal("unresolved evidence observation lost its owner and availability posture")
	}
	// Admission explicitly supplies the exact snapshot that the same publication
	// will save. This is distinct from an ordinary store reader's durable basis.
	publication := map[string][]byte{a.Edition: current[a.Edition]}
	admitted := ProjectCaptured([]Document{a, e}, current, publication)
	if admitted.Resolve(ar+"#L1").Kind != "found" || len(admitted.UsesFor(ar+"#L1")) != 1 || HasErrors(admitted.Entries[1].Diagnostics) {
		t.Fatal("explicit publication snapshot set did not resolve exact prepared input")
	}
	if Project([]Document{a, e}, current).Resolve(ar+"#L1").Kind != "found" {
		t.Fatal("pure Project single-set contract changed")
	}
}

func TestMissingPinnedPredecessorCannotSuppressCurrentLiveRecord(t *testing.T) {
	current := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, current)
	b := edit(t, a, func(r *Record) {
		r.ID = "spec-20260923-00000002"
		r.Claims[0].Text = "Cancellation resets total"
		r.Supersedes = []string{ar}
		r.SupersedeReason = "Explicit accepted revision"
	})
	b, _ = snap(t, b, InterpretationBasis{}, current)
	p := ProjectCaptured([]Document{a, b}, current, nil)
	if p.Entries[0].State != Active || p.Entries[1].State != Invalid || !hasCode(p.Entries[1].Diagnostics, "unresolved_predecessor") {
		t.Fatal("missing durable predecessor was silently superseded")
	}
	resolved := ProjectCaptured([]Document{a, b}, current, map[string][]byte{a.Edition: current[a.Edition]})
	if resolved.Entries[0].State != Superseded || resolved.Entries[1].State != Active {
		t.Fatal("exact saved predecessor did not restore valid succession")
	}
}

func TestPinnedOptionsAndLinksUseTheDependencySnapshotSet(t *testing.T) {
	current := map[string][]byte{}
	o, optionsRef := snap(t, fixture(t, "valid/options.md"), InterpretationBasis{}, current)
	d := edit(t, fixture(t, "valid/decision.md"), func(r *Record) { r.OptionsRef = optionsRef })
	d, _ = snap(t, d, InterpretationBasis{}, current)
	n := edit(t, fixture(t, "valid/note.md"), func(r *Record) {
		r.Links = []Link{{Kind: "relies_on", Target: optionsRef, Reason: "Exact historical option analysis"}}
	})
	n, _ = snap(t, n, InterpretationBasis{}, current)
	p := ProjectCaptured([]Document{o, d, n}, current, nil)
	if !hasCode(p.Entries[1].Diagnostics, "unresolved_options") || !hasCode(p.Entries[2].Diagnostics, "unresolved_pinned_link") {
		t.Fatal("other typed pinned dependencies bypassed snapshot availability")
	}
	p = ProjectCaptured([]Document{o, d, n}, current, map[string][]byte{o.Edition: current[o.Edition]})
	if hasCode(p.Entries[1].Diagnostics, "unresolved_options") || hasCode(p.Entries[2].Diagnostics, "unresolved_pinned_link") {
		t.Fatal("persisted option basis did not resolve")
	}
}
