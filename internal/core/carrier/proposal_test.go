package carrier

import (
	"reflect"
	"sort"
	"testing"
)

func TestProposalLeavesAreSeparateFromGoverningHeadsAndAliases(t *testing.T) {
	snaps := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, snaps)
	proposal := func(base Document, id string, predecessors []string) (Document, string) {
		d := edit(t, base, func(r *Record) {
			r.ID = id
			r.Status = Proposed
			r.Origin = "agent_proposal"
			r.OperatorConfirmed = false
			r.Supersedes = predecessors
			r.SupersedeReason = "Refine proposed content"
		})
		return snap(t, d, InterpretationBasis{}, snaps)
	}
	b, br := proposal(a, "spec-20260923-00000002", []string{ar})
	c, cr := proposal(b, "spec-20260923-00000003", []string{br})
	p := Project([]Document{c, a, b}, snaps)
	for _, id := range []string{a.Record.ID, b.Record.ID, c.Record.ID} {
		if !reflect.DeepEqual(p.Heads(id), []string{ar}) || !reflect.DeepEqual(p.ProposalHeads(id), []string{cr}) {
			t.Fatal("governing and authoring heads collapsed")
		}
	}
	if got := p.Resolve("spec:order-cancel"); got.Kind != "found" || got.Document.Record.ID != a.Record.ID {
		t.Fatal("proposal took active alias")
	}
	if len(p.ProposalHeads("spec-20260923-99999999")) != 0 {
		t.Fatal("unknown identity acquired a proposal")
	}
	d, dr := proposal(a, "spec-20260923-00000004", []string{ar})
	p = Project([]Document{d, c, a, b}, snaps)
	want := []string{cr, dr}
	sort.Strings(want)
	if !reflect.DeepEqual(p.ProposalHeads(b.Record.ID), want) || !reflect.DeepEqual(p.Heads(a.Record.ID), []string{ar}) {
		t.Fatal("competing proposals were ordered into a winner")
	}
	accepted := edit(t, c, func(r *Record) {
		r.ID = "spec-20260923-00000005"
		r.Status = Active
		r.Origin = "operator_request"
		r.OperatorConfirmed = true
		r.Supersedes = []string{ar, cr, dr}
		r.SupersedeReason = "Explicitly accept resolved content"
	})
	accepted, acceptedRef := snap(t, accepted, InterpretationBasis{}, snaps)
	p = Project([]Document{a, b, c, d, accepted}, snaps)
	if len(p.ProposalHeads(b.Record.ID)) != 0 || !reflect.DeepEqual(p.Heads(b.Record.ID), []string{acceptedRef}) {
		t.Fatal("accepted proposal stayed a current authoring leaf")
	}
}
