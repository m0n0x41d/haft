package carrier

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) Document {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return Parse(raw)
}
func requireValid(t *testing.T, d Document) {
	t.Helper()
	if !d.Valid() {
		t.Fatalf("invalid fixture: %+v", d.Diagnostics)
	}
}
func hasCode(ds []Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}
func edit(t *testing.T, d Document, fn func(*Record)) Document {
	t.Helper()
	r := d.Record
	fn(&r)
	raw, err := Encode(r, d.Body)
	if err != nil {
		t.Fatal(err)
	}
	out := Parse(raw)
	requireValid(t, out)
	return out
}
func snap(t *testing.T, d Document, basis InterpretationBasis, store map[string][]byte) (Document, string) {
	t.Helper()
	_, raw, digest, err := NewSnapshot(d.Raw, basis)
	if err != nil {
		t.Fatal(err)
	}
	store[digest] = raw
	d.Edition = digest
	return d, d.Record.ID + "@" + digest
}

func TestFixturesLosslessAndNormalization(t *testing.T) {
	for _, name := range []string{"decision", "spec", "note", "problem", "options", "evidence"} {
		t.Run(name, func(t *testing.T) {
			d := fixture(t, "valid/"+name+".md")
			requireValid(t, d)
			if ds := Validate(d.Record, true); HasErrors(ds) {
				t.Fatalf("strict: %+v", ds)
			}
			if !bytes.Equal(d.Raw, d.Bytes()) {
				t.Fatal("lossless read altered bytes")
			}
			raw, err := Normalize(d)
			if err != nil {
				t.Fatal(err)
			}
			n := Parse(raw)
			requireValid(t, n)
			if !reflect.DeepEqual(d.Record, n.Record) || !bytes.Equal(d.Body, n.Body) {
				t.Fatal("normalization dropped structured fields or body")
			}
		})
	}
	s := fixture(t, "valid/spec.md")
	if s.Record.Extra["extension"] == nil || s.Record.Claims[0].Extra["custom_claim"] == nil {
		t.Fatal("unknown data not captured")
	}
	crlf := bytes.ReplaceAll(s.Raw, []byte("\n"), []byte("\r\n"))
	d := Parse(crlf)
	requireValid(t, d)
	if !bytes.Equal(d.Bytes(), crlf) {
		t.Fatal("line endings changed")
	}
}
func TestOpaqueAndInvalidContentRemainsReadable(t *testing.T) {
	for _, name := range []string{"duplicate", "unknown-format"} {
		d := fixture(t, "invalid/"+name+".md")
		if d.Valid() {
			t.Fatal("invalid accepted")
		}
		p := Project([]Document{d}, nil)
		if p.Entries[0].State != Invalid || !bytes.Equal(p.Entries[0].Document.Raw, d.Raw) {
			t.Fatal("invalid bytes lost or governed")
		}
		if _, err := Normalize(d); err == nil {
			t.Fatal("opaque document normalized")
		}
	}
	d := fixture(t, "valid/note.md")
	for _, raw := range [][]byte{bytes.Replace(d.Raw, []byte("kind: note"), []byte("kind: alien"), 1), bytes.Replace(d.Raw, []byte("status: active"), []byte("status: superseded"), 1), bytes.Replace(d.Raw, []byte("title:"), []byte("<<: {status: active}\ntitle:"), 1), append(d.Raw, 0xff)} {
		if Parse(raw).Valid() {
			t.Fatalf("invalid carrier accepted: %q", raw)
		}
	}
	raw := bytes.Replace(d.Raw, []byte("title:"), []byte("extension: {x: 1, x: 2}\ntitle:"), 1)
	if !hasCode(Parse(raw).Diagnostics, "duplicate_key") {
		t.Fatal("nested duplicate not found")
	}
}
func TestReferencesAndSelectors(t *testing.T) {
	full := "spec-20260923-00000001@sha256:" + strings.Repeat("a", 64) + "#L1"
	for _, s := range []string{"dec-20260923-00000001", "spec:order-cancel", "spec:order-cancel#L1", full} {
		r, err := ParseRef(s)
		if err != nil || r.String() != s {
			t.Fatalf("%s: %+v %v", s, r, err)
		}
	}
	for _, s := range []string{"dec-20260230-00000001", "dec-20260923-DEADBEEF", "spec:x@sha256:" + strings.Repeat("a", 64), "spec-20260923-00000001@sha256:abcd#L1", full + "#L2", "dec-20260923-legacy-long-id"} {
		if _, err := ParseRef(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	for _, s := range []string{"file:../escape.go", "sym:/outside.go::Name", "file:a/../../b", "dir:a\\b", "sym:a.go"} {
		if ValidateSelector(s) == nil {
			t.Errorf("accepted selector %q", s)
		}
	}
	for _, s := range []string{"file:a.go", "dir:internal", "sym:internal/order.go::Order.Cancel"} {
		if err := ValidateSelector(s); err != nil {
			t.Fatal(err)
		}
	}
}
func TestAuthorityAndDecisionShapes(t *testing.T) {
	d := fixture(t, "valid/decision.md")
	r := d.Record
	r.Status = "active"
	if !hasCode(Validate(r, false), "operator_confirmation_required") {
		t.Fatal("proposal claimed acceptance")
	}
	r.Origin = "migrated_9x"
	raw, _ := Encode(r, d.Body)
	p := Project([]Document{Parse(raw)}, nil)
	if p.Entries[0].State != Historical {
		t.Fatal("migration became current authority")
	}
	r.OperatorConfirmed = true
	if !hasCode(Validate(r, false), "invalid_confirmation") {
		t.Fatal("migration invented confirmation")
	}
	r = d.Record
	r.Options = nil
	r.NoAlternativeReason = "No other deployable option exists"
	if HasErrors(Validate(r, false)) {
		t.Fatal("explicit no-alternative reason rejected")
	}
	r = d.Record
	r.Disposition = "probe_again"
	r.Probe = "restart broker"
	r.Constrains = nil
	r.Chosen = ""
	if HasErrors(Validate(r, false)) {
		t.Fatal("probe incorrectly requires chosen")
	}
	r.Constrains = []string{"dir:internal"}
	if !hasCode(Validate(r, false), "constraint_kind") {
		t.Fatal("probe introduced constraint")
	}
	r = d.Record
	r.Options = []Option{{ID: r.Chosen, Verdict: "rejected", Reason: "Cannot operate it"}}
	if !hasCode(Validate(r, false), "chosen_option_rejected") || !hasCode(Validate(r, false), "alternative_basis_required") {
		t.Fatal("chosen option served as its own rejected alternative")
	}
}
func TestClaimsQuadrantsAndValidationBasis(t *testing.T) {
	d := fixture(t, "valid/spec.md")
	r := d.Record
	r.Claims[0].Refs = []string{"A1"}
	if !hasCode(Validate(r, true), "quadrant_dependency") {
		t.Fatal("L depends on A")
	}
	r = fixture(t, "valid/spec.md").Record
	r.Claims[0].Checks = nil
	r.Claims[0].Unchecked = ""
	if HasErrors(Validate(r, false)) || !HasErrors(Validate(r, true)) {
		t.Fatal("strict missing validation basis is not distinguished")
	}
	r.Claims[0].Unchecked = "No relevant oracle exists"
	if HasErrors(Validate(r, true)) {
		t.Fatal("honest unchecked rejected")
	}
	r.Claims[0].ImplementedBy[0].Covers = ""
	if !hasCode(Validate(r, true), "required") {
		t.Fatal("implementation scope omitted")
	}
	r = fixture(t, "valid/spec.md").Record
	r.RetiredClaimIDs = []string{"L1"}
	if !hasCode(Validate(r, false), "invalid_claim_id") {
		t.Fatal("retired claim reused")
	}
	r = fixture(t, "valid/spec.md").Record
	r.Claims[0].Refs = []string{"missing"}
	if !hasCode(Validate(r, false), "unresolved_claim") {
		t.Fatal("dangling dependency accepted")
	}
}
func TestSnapshotByteIdentityAndInterpretation(t *testing.T) {
	d := fixture(t, "valid/spec.md")
	termRaw, err := os.ReadFile("testdata/terms.md")
	if err != nil {
		t.Fatal(err)
	}
	basis := InterpretationBasis{Namespace: "repository:fixture", Terms: &BasisFile{Name: "terms.md", Bytes: termRaw}, Other: []BasisFile{{Name: "z", Bytes: []byte("z")}, {Name: "a", Bytes: []byte("a")}}}
	s, raw, hash, err := NewSnapshot(d.Raw, basis)
	if err != nil {
		t.Fatal(err)
	}
	_, again, hash2, err := NewSnapshot(d.Raw, basis)
	if err != nil || hash != hash2 || !bytes.Equal(raw, again) {
		t.Fatal("unstable serialization")
	}
	read, err := ReadSnapshot(raw, hash)
	if err != nil || !bytes.Equal(read.Raw, d.Raw) || !bytes.Equal(read.Interpretation.Terms.Bytes, termRaw) {
		t.Fatal("snapshot lost interpretation or raw bytes")
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSnapshot(pretty.Bytes(), hash); err == nil {
		t.Fatal("reader hashed reserialization instead of raw bytes")
	}
	if _, err := ReadSnapshot(pretty.Bytes(), Digest(pretty.Bytes())); err != nil {
		t.Fatal("valid alternate formatting rejected", err)
	}
	for _, mutation := range []func(*Snapshot){func(s *Snapshot) { s.Raw = append(s.Raw, ' ') }, func(s *Snapshot) { s.Interpretation.Terms.Bytes = append(s.Interpretation.Terms.Bytes, ' ') }, func(s *Snapshot) { s.Interpretation.Namespace = "different" }} {
		copyS := s
		copyS.Raw = bytes.Clone(s.Raw)
		copyS.Interpretation.Terms = &BasisFile{Name: s.Interpretation.Terms.Name, Bytes: bytes.Clone(s.Interpretation.Terms.Bytes)}
		mutation(&copyS)
		b, _ := json.Marshal(copyS)
		if Digest(b) == hash {
			t.Fatal("exact snapshot omitted changed material")
		}
	}
	_, nonUTF, h, err := NewSnapshot([]byte{0xff, 0xfe}, InterpretationBasis{})
	if err != nil {
		t.Fatal(err)
	}
	n, err := ReadSnapshot(nonUTF, h)
	if err != nil || !bytes.Equal(n.Raw, []byte{0xff, 0xfe}) {
		t.Fatal("historical non-UTF8 bytes lost")
	}
	if n.Document().Valid() {
		t.Fatal("non-UTF8 became live record")
	}
	dup := []byte(`{"format":"haft.snapshot/1","format":"haft.snapshot/1"}`)
	if _, err := ReadSnapshot(dup, Digest(dup)); err == nil {
		t.Fatal("duplicate snapshot key accepted")
	}
}
func TestSnapshotGolden(t *testing.T) {
	_, raw, digest, err := NewSnapshot([]byte("raw\x00\xff"), InterpretationBasis{})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"format\":\"haft.snapshot/1\",\"carrier_bytes_base64\":\"cmF3AP8=\",\"interpretation\":{\"version\":\"haft.interpretation/1\",\"namespace\":\"\",\"terms\":null,\"other\":[]}}\n"
	if string(raw) != want {
		t.Fatalf("serialization changed\ngot %s\nwant %s", raw, want)
	}
	if digest != Digest([]byte(want)) {
		t.Fatal("digest did not cover exact golden")
	}
}
func TestProjectionAcceptanceConflictAndOrder(t *testing.T) {
	snaps := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, snaps)
	b := edit(t, a, func(r *Record) {
		r.ID = "spec-20260923-00000002"
		r.Status = "proposed"
		r.Origin = "agent_proposal"
		r.OperatorConfirmed = false
		r.Supersedes = []string{ar}
		r.SupersedeReason = "Propose a wider law"
	})
	b, br := snap(t, b, InterpretationBasis{}, snaps)
	p := Project([]Document{a, b}, snaps)
	if p.Entries[0].State != Active || p.Entries[1].State != Proposed {
		t.Fatalf("proposal suppressed active: %+v", p.Entries)
	}
	if got := p.Resolve("spec:order-cancel"); got.Kind != "found" || got.Document.Record.ID != a.Record.ID {
		t.Fatal("proposal hijacked alias")
	}
	c := edit(t, b, func(r *Record) {
		r.ID = "spec-20260923-00000003"
		r.Status = "active"
		r.Origin = "operator_request"
		r.OperatorConfirmed = true
		r.Supersedes = []string{br}
	})
	if !hasCode(ValidateSuccessor(c.Record, p, snaps, []string{ar}), "missing_predecessor") {
		t.Fatal("proposal-only acceptance silently retired A")
	}
	bad := Project([]Document{a, b, c}, snaps)
	if bad.Entries[0].State != Active || bad.Entries[2].State != Invalid {
		t.Fatal("bad acceptance entered projection")
	}
	c = edit(t, c, func(r *Record) { r.Supersedes = []string{ar, br} })
	c, cr := snap(t, c, InterpretationBasis{}, snaps)
	if ds := ValidateSuccessor(c.Record, p, snaps, []string{ar}); HasErrors(ds) {
		t.Fatal(ds)
	}
	for _, docs := range [][]Document{{a, b, c}, {c, b, a}, {b, c, a}} {
		out := Project(docs, snaps)
		if got := out.Heads(a.Record.ID); !reflect.DeepEqual(got, []string{cr}) {
			t.Fatalf("order changed heads: %v", got)
		}
	}
	d := edit(t, c, func(r *Record) { r.ID = "spec-20260923-00000004"; r.Title = "Competing accepted proposal" })
	conflict := Project([]Document{a, b, c, d}, snaps)
	if conflict.Resolve("spec:order-cancel").Kind != "conflict" || len(conflict.Heads(a.Record.ID)) != 2 {
		t.Fatal("competing accepted heads had a winner")
	}
	if !hasCode(ValidateSuccessor(c.Record, p, snaps, []string{br}), "head_conflict") {
		t.Fatal("stale expected heads passed")
	}
}
func TestUnresolvedSnapshotsAndChangedPredecessor(t *testing.T) {
	snaps := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, snaps)
	b := edit(t, a, func(r *Record) {
		r.ID = "spec-20260923-00000002"
		r.Supersedes = []string{ar}
		r.SupersedeReason = "New version"
	})
	p := Project([]Document{a, b}, nil)
	if p.Entries[1].State != Invalid || !hasCode(p.Entries[1].Diagnostics, "unresolved_predecessor") {
		t.Fatal("missing snapshot fell back to live")
	}
	a2 := edit(t, a, func(r *Record) { r.Title = "Manual edit with same ID" })
	p = Project([]Document{a2, b}, snaps)
	if p.Entries[0].State == Superseded || !hasCode(p.Entries[1].Diagnostics, "predecessor_edition_changed") {
		t.Fatal("new bytes were silently retired")
	}
	if !hasCode(ValidateSuccessor(b.Record, Project([]Document{a2}, snaps), snaps, []string{ar}), "stale_predecessor") {
		t.Fatal("changed predecessor accepted")
	}
	p = Project([]Document{a, a}, snaps)
	if p.Entries[0].State != Invalid || p.Entries[1].State != Invalid {
		t.Fatal("duplicate ID picked a winner")
	}
	b = edit(t, b, func(r *Record) { r.About = "system:different" })
	if !hasCode(ValidateSuccessor(b.Record, Project([]Document{a}, snaps), snaps, []string{ar}), "lineage_mismatch") {
		t.Fatal("different subject superseded")
	}
}
func TestEvidenceNeverTransfersAcrossEditions(t *testing.T) {
	snaps := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, snaps)
	e := edit(t, fixture(t, "valid/evidence.md"), func(r *Record) { r.Uses[0].Target = ar + "#L1" })
	e, er := snap(t, e, InterpretationBasis{}, snaps)
	a2 := edit(t, a, func(r *Record) { r.Claims[0].Text = "Cancel sets total to zero" })
	a2, newRef := snap(t, a2, InterpretationBasis{}, snaps)
	p := Project([]Document{a2, e}, snaps)
	if len(p.UsesFor(ar+"#L1")) != 1 || len(p.UsesFor(newRef+"#L1")) != 0 || len(p.UsesFor("spec:order-cancel#L1")) != 0 {
		t.Fatal("evidence passed through live/changed target")
	}
	old := p.Resolve(ar + "#L1")
	if old.Kind != "found" || old.Claim.Text != "Cancel preserves total" {
		t.Fatal("old target changed", old)
	}
	r := a2.Record
	r.Claims[2].EvidenceInputs = []EvidenceInput{{Ref: er + "#u1", Applicability: "Only for the observed new/paid cases"}}
	if HasErrors(Validate(r, true)) || HasErrors(p.ValidateReferences(r)) {
		t.Fatal("valid A evidence input prohibited")
	}
	r.Claims[0].EvidenceInputs = r.Claims[2].EvidenceInputs
	if !hasCode(Validate(r, true), "quadrant_dependency") {
		t.Fatal("L consumed E as a normative dependency")
	}
	r = fixture(t, "valid/spec.md").Record
	r.Claims[0].Refs = []string{newRef + "#A1"}
	if !hasCode(p.ValidateReferences(r), "quadrant_dependency") {
		t.Fatal("cross-section L→A escaped validation")
	}
}
func TestTermsAndSourceSnapshots(t *testing.T) {
	raw, err := os.ReadFile("testdata/terms.md")
	if err != nil {
		t.Fatal(err)
	}
	d := ParseTerms(raw)
	if HasErrors(d.Diagnostics) {
		t.Fatal(d.Diagnostics)
	}
	spec := fixture(t, "valid/spec.md")
	if HasErrors(ValidateTermRefs(spec.Record, d.Terms)) {
		t.Fatal("declared term not resolved")
	}
	spec.Record.Terms = []string{"Absent.Term"}
	if !hasCode(ValidateTermRefs(spec.Record, d.Terms), "unresolved_term") {
		t.Fatal("missing explicit term hidden")
	}
	if !hasCode(ParseTerms(append(raw, 0xff)).Diagnostics, "invalid_utf8") {
		t.Fatal("invalid UTF8 terms accepted")
	}
	b, err := EncodeTerms(d.Terms, d.Body)
	if err != nil || !reflect.DeepEqual(ParseTerms(b).Terms, d.Terms) {
		t.Fatal("terms normalization lost data")
	}
	body := []byte("Exact source slice\n")
	source := Source{Ref: "A.6.B", SourceRevision: SourceRevision{Kind: "git", Repository: "fpf", Commit: strings.Repeat("a", 40)}, PublicationPath: "FPF.md", BodyDigest: Digest(body), Lines: []int{100, 101}, Extra: Extra{"complete_pattern": false}}
	_, b, digest, err := NewSourceSnapshot(body, source)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ReadSourceSnapshot(b, digest)
	if err != nil || !bytes.Equal(s.Raw, body) || s.Provenance.SourceRevision.Commit != source.SourceRevision.Commit {
		t.Fatal("source provenance lost", err)
	}
	if s.Provenance.Extra["complete_pattern"] != false {
		t.Fatal("excerpt became complete pattern")
	}
	source.BodyDigest = Digest([]byte("other"))
	if _, _, _, err := NewSourceSnapshot(body, source); err == nil {
		t.Fatal("wrong source bytes accepted")
	}
}

func TestProjectionRejectsExternalQuadrantAndRestoresPredecessor(t *testing.T) {
	snaps := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, snaps)
	external := edit(t, a, func(r *Record) {
		r.ID = "spec-20260923-00000009"
		r.Slug = "other"
		r.Claims = []Claim{{ID: "D1", Kind: "prescription", Text: "An operator must authorize this action"}}
	})
	external, er := snap(t, external, InterpretationBasis{}, snaps)
	b := edit(t, a, func(r *Record) {
		r.ID = "spec-20260923-00000002"
		r.Supersedes = []string{ar}
		r.SupersedeReason = "Change law"
		r.Claims[0].Refs = []string{er + "#D1"}
	})
	p := Project([]Document{a, b, external}, snaps)
	if p.Entries[1].State != Invalid || !hasCode(p.Entries[1].Diagnostics, "quadrant_dependency") || p.Entries[0].State != Active {
		t.Fatalf("invalid external dependency suppressed predecessor: %+v", p.Entries)
	}
}

func TestMissingLiveAncestorsStillConnectCompetingBranches(t *testing.T) {
	snaps := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, snaps)
	branch := func(id, previous string) Document {
		return edit(t, a, func(r *Record) {
			r.ID = id
			r.Supersedes = []string{previous}
			r.SupersedeReason = "Continue the same law"
		})
	}
	b, br := snap(t, branch("spec-20260923-00000002", ar), InterpretationBasis{}, snaps)
	_, _ = b, br
	c := branch("spec-20260923-00000003", br)
	d := branch("spec-20260923-00000004", ar)
	p := Project([]Document{c, d}, snaps)
	if p.Entries[0].State != Contested || p.Entries[1].State != Contested || p.Resolve("spec:order-cancel").Kind != "conflict" {
		t.Fatal("missing live intermediate split one lineage into unrelated winners")
	}
	if len(p.Heads(a.Record.ID)) != 2 || p.Resolve(a.Record.ID).Kind != "conflict" {
		t.Fatal("missing original ID hid its live heads")
	}
}

func TestAdmissionRejectsAliasReuseAndClaimTombstoneLoss(t *testing.T) {
	snaps := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, snaps)
	p := Project([]Document{a}, snaps)
	r := a.Record
	r.ID = "spec-20260923-00000002"
	if !hasCode(ValidateSuccessor(r, p, snaps, nil), "alias_conflict") {
		t.Fatal("independent spec stole an alias")
	}
	r.Supersedes = []string{ar}
	r.SupersedeReason = "Remove a claim"
	r.Claims = r.Claims[:2]
	if !hasCode(ValidateSuccessor(r, p, snaps, []string{ar}), "removed_claim_not_retired") {
		t.Fatal("claim ID disappearance was not recorded")
	}
	r.RetiredClaimIDs = []string{"A1"}
	if ds := ValidateSuccessor(r, p, snaps, []string{ar}); HasErrors(ds) {
		t.Fatal(ds)
	}
}

func TestHistoricalEvidenceRetainsScopeWithoutCurrentSupport(t *testing.T) {
	snaps := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, snaps)
	e := edit(t, fixture(t, "valid/evidence.md"), func(r *Record) { r.Origin = "migrated_9x"; r.Uses[0].Target = ar + "#L1" })
	p := Project([]Document{a, e}, snaps)
	if len(p.UsesFor(ar+"#L1")) != 0 {
		t.Fatal("historical evidence presented as current support")
	}
	matches := p.EvidenceUsesFor(ar + "#L1")
	if len(matches) != 1 || matches[0].State != Historical || matches[0].OwnerRef == "" || matches[0].Use.Scope == "" {
		t.Fatal("historical scope/provenance lost")
	}
}

func TestPureFunctionsDoNotMutateInputs(t *testing.T) {
	d := fixture(t, "valid/spec.md")
	original := bytes.Clone(d.Raw)
	p := Project([]Document{d}, nil)
	p.Entries[0].Document.Raw[0] = 'x'
	if !bytes.Equal(d.Raw, original) {
		t.Fatal("projection shares mutable raw input")
	}
	basis := InterpretationBasis{Other: []BasisFile{{Name: "z", Bytes: []byte("z")}, {Name: "a", Bytes: []byte("a")}}}
	_, _, _, err := NewSnapshot(d.Raw, basis)
	if err != nil {
		t.Fatal(err)
	}
	if basis.Other[0].Name != "z" {
		t.Fatal("snapshot sorted caller's basis in place")
	}
}

func TestProjectionRejectsStaleOrUnverifiableDeclaredEdition(t *testing.T) {
	snaps := map[string][]byte{}
	a, ar := snap(t, fixture(t, "valid/spec.md"), InterpretationBasis{}, snaps)
	e := edit(t, fixture(t, "valid/evidence.md"), func(r *Record) { r.Uses[0].Target = ar + "#L1" })
	changed := edit(t, a, func(r *Record) { r.Claims[0].Text = "Cancel changes total" })
	changed.Edition = a.Edition
	p := Project([]Document{changed, e}, snaps)
	if p.Entries[0].State != Invalid || !hasCode(p.Entries[0].Diagnostics, "captured_edition_mismatch") {
		t.Fatal("stale declared edition projected new text at old evidence target")
	}
	if res := p.Resolve("spec:order-cancel#L1"); res.Kind == "found" {
		t.Fatal("invalid current carrier became a live claim")
	}
	if res := p.Resolve(ar + "#L1"); res.Kind != "found" || res.Claim.Text != "Cancel preserves total" {
		t.Fatal("rejecting stale association corrupted old pinned target")
	}
	p = Project([]Document{a}, nil)
	if p.Entries[0].State != Invalid || !hasCode(p.Entries[0].Diagnostics, "unresolved_captured_edition") {
		t.Fatal("unverifiable supplied digest was trusted")
	}
}
