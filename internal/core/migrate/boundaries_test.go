package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/store"
)

func publishFixture(t *testing.T, root, id string, outputs ...store.Output) store.View {
	t.Helper()
	ctx := context.Background()
	s := store.Store{Root: root}
	v, err := s.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(id)
	result, err := s.Publish(ctx, store.Request{RequestID: id, PayloadDigest: carrier.Digest(payload), RequestPayload: payload, ExpectedGeneration: v.Generation, Outputs: outputs})
	if err != nil || result.Kind != "written" {
		t.Fatal(result, err)
	}
	v, err = s.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestAssistancePinsTermsAndRejectsFalseOrEphemeralProvenance(t *testing.T) {
	ctx := context.Background()
	sourceRoot := t.TempDir()
	terms, err := carrier.EncodeTerms(carrier.TermMap{Format: "haft.terms/1", Terms: []carrier.Term{{ID: "Domain.Total", Definition: "Recorded total in this domain."}}}, []byte("Exact retained definitions.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "terms.md"), terms, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "old.md"), []byte("A useful old statement without structured identity.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := Request{CarrierRoot: sourceRoot, OutputRoot: t.TempDir(), CreatedAt: stamp, DryRun: true}
	result, err := Run(ctx, r)
	if err != nil || result.Kind != "staged" {
		t.Fatal(result, err)
	}
	items, err := Queue(ctx, r.OutputRoot)
	if err != nil || len(items) != 1 || items[0].Kind != "unknown" {
		t.Fatal(items, err)
	}
	body := []byte("A bounded source slice")
	src := carrier.Source{Ref: "A.6.B", SourceRevision: carrier.SourceRevision{Kind: "unknown", Reason: "Original revision was not recorded"}, PublicationPath: "FPF.md", BodyDigest: carrier.Digest(body), Lines: []int{10, 11}, Extra: carrier.Extra{"complete_pattern": false}}
	_, blob, hash, err := carrier.NewSourceSnapshot(body, src)
	if err != nil {
		t.Fatal(err)
	}
	src.SnapshotRef = hash
	v := publishFixture(t, r.OutputRoot, "source-fixture", store.Output{Path: "editions/sha256/" + strings.TrimPrefix(hash, "sha256:") + ".json", Bytes: blob}, store.Output{Path: "project.yaml", Bytes: []byte("format: haft.project/1\nrepository_id: migration-fixture\n")})
	record := carrier.Record{Format: "haft/1", ID: "note-20260923-11223344", Kind: "note", Title: "Recovered bounded statement", Status: "proposed", Origin: "agent_proposal", About: "system:fixture", CreatedAt: stamp, Terms: []string{"Domain.Total"}, Sources: []carrier.Source{src}}
	record.Sources[0].Lines = []int{1, 1000}
	raw, _ := carrier.Encode(record, []byte("Current subject interpretation is a proposal.\n"))
	ar := AssistanceRequest{OutputRoot: r.OutputRoot, QueueID: items[0].ID, Proposal: raw, Reason: "Recover selected old statement with an explicit subject", RequestID: "repair-boundary", ExpectedGeneration: v.Generation}
	rejected, err := Assist(ctx, ar)
	if err == nil || rejected.Kind != "invalid" {
		t.Fatal("altered source scope admitted", rejected, err)
	}
	record.Sources[0] = src
	target := carrier.Record{Format: "haft/1", ID: "note-20260923-99887766", Kind: "note", Title: "Unpersisted target", Status: "proposed", Origin: "agent_proposal", About: "system:fixture", CreatedAt: stamp}
	targetRaw, _ := carrier.Encode(target, []byte("Only live bytes exist.\n"))
	v = publishFixture(t, r.OutputRoot, "live-only", store.Output{Path: "notes/" + target.ID + ".md", Bytes: targetRaw})
	targetDigest := v.Editions["notes/"+target.ID+".md"]
	record.Links = []carrier.Link{{Kind: "relates_to", Target: target.ID + "@" + targetDigest}}
	ar.Proposal, _ = carrier.Encode(record, []byte("Current subject interpretation is a proposal.\n"))
	ar.ExpectedGeneration = v.Generation
	rejected, err = Assist(ctx, ar)
	if rejected.Kind != "invalid" {
		t.Fatal("ephemeral current edition healed pinned dependency", rejected, err)
	}
	record.Links = nil
	ar.Proposal, _ = carrier.Encode(record, []byte("Current subject interpretation is a proposal.\n"))
	written, err := Assist(ctx, ar)
	if err != nil || written.Kind != "written" {
		t.Fatal(written, err)
	}
	items, err = Queue(ctx, r.OutputRoot)
	if err != nil || items[0].Status != "proposed_recovery" {
		t.Fatal(items, err)
	}
	v, err = (store.Store{Root: r.OutputRoot}).Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := carrier.ParseRef(items[0].ProposalRef)
	if err != nil {
		t.Fatal(err)
	}
	captured, err := carrier.ReadSnapshot(v.Snapshots[ref.Digest], ref.Digest)
	if err != nil || captured.Interpretation.Namespace != "migration-fixture" || captured.Interpretation.Terms == nil || !bytes.Equal(captured.Interpretation.Terms.Bytes, terms) {
		t.Fatal("assistance interpretation basis lost", captured, err)
	}
	if v.Coverage != "complete" {
		t.Fatal("accepted proposal degraded on immediate read", v.Diagnostics)
	}
}

func TestUnresolvedClassificationUsesActualSourceCoverage(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "good", "Note", "active", "Known source", []byte("Source"), map[string]any{"about": "system:fixture"})
	insert(t, db, "bad", "DecisionRecord", "active", "Known but incomplete", []byte("Original"), nil)
	if _, err := db.Exec(`INSERT INTO artifact_links VALUES('good','bad','based_on'),('good','absent','governs')`); err != nil {
		t.Fatal(err)
	}
	r := basicRequest(p)
	snap, err := Capture(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	plan := Build(snap, r)
	kinds := map[string]int{}
	for _, item := range plan.Report.Items {
		if item.Kind == "artifact_links" {
			kinds[item.Disposition]++
		}
	}
	if kinds["new_unresolved"] != 1 || kinds["existing_unresolved"] != 1 {
		t.Fatal("new conversion loss mislabelled as old absence", kinds)
	}
	for i := range snap.Scopes {
		if snap.Scopes[i].Source == "sqlite:artifacts" {
			snap.Scopes[i].Disposition = "source_unreadable"
		}
	}
	plan = Build(snap, r)
	kinds = map[string]int{}
	for _, item := range plan.Report.Items {
		if item.Kind == "artifact_links" {
			kinds[item.Disposition]++
		}
	}
	if kinds["new_unresolved"] != 1 || kinds["resolution_unknown"] != 1 || kinds["existing_unresolved"] != 0 {
		t.Fatal("unread scope treated as proven absent", kinds)
	}
}

func TestChangedRequestTimeConflictsAndFreshRequestCannotClobberStage(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "old", "Note", "active", "Old title", []byte("Old source"), map[string]any{"about": "system:fixture"})
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	r.RequestID = "same-request"
	first, err := Run(context.Background(), r)
	if err != nil || first.Kind != "staged" {
		t.Fatal(first, err)
	}
	r.CreatedAt = "2026-09-24T10:00:00Z"
	changed, err := Run(context.Background(), r)
	if err != nil || changed.Kind != "request_conflict" {
		t.Fatal("changed payload replayed", changed, err)
	}
	r.RequestID = "new-request"
	changed, err = Run(context.Background(), r)
	if err != nil || changed.Kind != "path_conflict" {
		t.Fatal("new request overwrote existing report", changed, err)
	}
	v, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err := json.Unmarshal(v.Files[first.ReportPath], &report); err != nil || report.CreatedAt != stamp {
		t.Fatal("report timestamp overwritten", report, err)
	}
}

func TestSourceSymlinksAndRollbackJournalRemainUnread(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "old", "Note", "active", "Title", []byte("Source"), map[string]any{"about": "system:fixture"})
	alias := filepath.Join(t.TempDir(), "alias.db")
	if err := os.Symlink(p, alias); err != nil {
		t.Fatal(err)
	}
	snap, err := Capture(context.Background(), basicRequest(alias))
	if err != nil || len(snap.Rows) != 0 || len(snap.Diagnostics) == 0 {
		t.Fatal("symlink source accepted", snap, err)
	}
	if err := os.WriteFile(p+"-journal", []byte("unfinished rollback"), 0600); err != nil {
		t.Fatal(err)
	}
	before := sourceFiles(t, p)
	snap, err = Capture(context.Background(), basicRequest(p))
	if err != nil || len(snap.Rows) != 0 || len(snap.Diagnostics) == 0 {
		t.Fatal("rollback source accepted", snap, err)
	}
	if !equalFiles(before, sourceFiles(t, p)) {
		t.Fatal("unread source mutated")
	}
	root := t.TempDir()
	if err := os.Symlink(p, filepath.Join(root, "escape.md")); err != nil {
		t.Fatal(err)
	}
	snap, err = Capture(context.Background(), Request{CarrierRoot: root})
	if err != nil || len(snap.Carriers) != 0 || len(snap.Scopes) != 1 || snap.Scopes[0].Disposition != "source_unreadable" {
		t.Fatal("carrier symlink read", snap, err)
	}
}

func TestMalformedMetadataAndMissingTermsDoNotProduceNativeCarrier(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "broken", "Note", "active", "Metadata broken", []byte("Source"), nil)
	if _, err := db.Exec(`UPDATE artifacts SET structured_data='{"about":"system:fixture"' WHERE id='broken'`); err != nil {
		t.Fatal(err)
	}
	section := map[string]any{"id": "missing-terms", "title": "Requires unavailable term map", "about": "system:fixture", "slug": "missing-terms", "receiving_use": "Read historical claims", "created_at": stamp, "terms": []string{"Domain.Absent"}, "claims": []any{map[string]any{"id": "L1", "kind": "law", "text": "Total is preserved"}}}
	raw, _ := json.Marshal(section)
	if _, err := db.Exec(`INSERT INTO spec_section_editions VALUES('p','missing-terms',?,'old',?)`, string(raw), stamp); err != nil {
		t.Fatal(err)
	}
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	result, err := Run(context.Background(), r)
	if err != nil || result.Kind != "staged" || len(result.Report.Queue) != 2 {
		t.Fatal(result, err)
	}
	v, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil || len(v.Documents) != 0 {
		t.Fatal("unknown source interpretation fabricated", v.Documents, err)
	}
}

func TestProblemPortfolioAndExplicitSelectorRolesRemainDistinct(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "problem-old", "ProblemCard", "addressed", "Cancellation signal", []byte("Original problem body"), map[string]any{"about": "system:fixture", "object": "Cancel operation", "question": "Why does total change?", "signal": "Reproduction changes 42 to 0"})
	insert(t, db, "portfolio-old", "SolutionPortfolio", "active", "Broker comparison", []byte("Original comparison body"), map[string]any{"about": "system:fixture", "question": "Which broker supports the workload?", "options": []map[string]any{{"id": "kafka", "summary": "Existing deployment"}, {"id": "nats", "summary": "Smaller deployment"}}, "comparison": map[string]any{"characteristics": []string{"operations"}, "basis": "Same workload", "comparator": "Retain tradeoffs", "non_dominated": []string{"kafka", "nats"}}, "next_use": "probe_again", "probe": "Observe restart"})
	decision := goodDecision()
	decision["governance_targets"] = []map[string]any{{"kind": "binding", "ref": "file:internal/ingest/kafka.go"}}
	insert(t, db, "decision-old", "DecisionRecord", "active", "Broker binding", []byte("Historical scope"), decision)
	if _, err := db.Exec(`INSERT INTO affected_symbols VALUES('decision-old','internal/ingest/kafka.go','Send','historical-body-hash'); INSERT INTO artifact_links VALUES('portfolio-old','problem-old','based_on')`); err != nil {
		t.Fatal(err)
	}
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	result, err := Run(context.Background(), r)
	if err != nil || result.Kind != "staged" || len(result.Report.Queue) != 0 {
		t.Fatal(result, err)
	}
	view, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil || len(view.Documents) != 3 {
		t.Fatal(view.Documents, err)
	}
	for _, entry := range view.Projection.Entries {
		if entry.State != carrier.Historical {
			t.Fatal("historical content activated", entry)
		}
		d := entry.Document.Record
		if d.Kind == "problem" && d.LegacyStatus != "addressed" {
			t.Fatal("addressed lost", d)
		}
		if d.Kind == "options" && (d.NextUse != "probe_again" || len(d.Links) != 1 || d.Links[0].Kind != "relates_to" || d.Links[0].LegacyType != "based_on") {
			t.Fatal("portfolio intent or historical relation invented", d)
		}
		if d.Kind == "decision" {
			if len(d.Constrains) != 1 || d.Constrains[0] != "file:internal/ingest/kafka.go" || len(d.Links) != 2 {
				t.Fatal("explicit governance/footprint roles collapsed", d)
			}
		}
		if _, ok := view.Snapshots[entry.Document.Edition]; !ok {
			t.Fatal("migrated exact edition is not persisted")
		}
	}
}

func TestQueueConflictingReportsFailDeterministically(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "gap", "Note", "active", "Useful old material", []byte("Original"), nil)
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	first, err := Run(context.Background(), r)
	if err != nil || first.Kind != "staged" {
		t.Fatal(first, err)
	}
	changed := first.Report
	changed.SourceDigest = carrier.Digest([]byte("different source generation"))
	changed.Queue = append([]QueueItem(nil), changed.Queue...)
	changed.Queue[0].Reason = "Conflicting interpretation of the same queued source"
	raw, _ := json.Marshal(changed)
	publishFixture(t, r.OutputRoot, "conflicting-report", store.Output{Path: reportPath(changed.SourceDigest), Bytes: raw})
	for i := 0; i < 8; i++ {
		if _, err := Queue(context.Background(), r.OutputRoot); err == nil || !strings.Contains(err.Error(), "conflicting queue item") {
			t.Fatal("report map order selected a queue winner", err)
		}
	}
}

func TestDuplicateStructuredKeysAndScalarConflictsRequireRewrite(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "duplicate", "Note", "active", "Scalar title", []byte("Source"), nil)
	if _, err := db.Exec(`UPDATE artifacts SET structured_data='{"about":"system:first","about":"system:second","title":"Different title"}'`); err != nil {
		t.Fatal(err)
	}
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	result, err := Run(context.Background(), r)
	if err != nil || result.Kind != "staged" || len(result.Report.Queue) != 1 {
		t.Fatal(result, err)
	}
	item := findItem(result.Report, "sqlite:artifacts:duplicate")
	reason := strings.Join(item.Reasons, " ")
	if item.Disposition != "needs_rewrite" || !strings.Contains(reason, "duplicate or ambiguous") || !strings.Contains(reason, "scalar source column") {
		t.Fatal("conflicting source values silently elected", item)
	}
	view, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil || len(view.Documents) != 0 {
		t.Fatal(view.Documents, err)
	}
	var original Row
	if err := json.Unmarshal(view.Files[item.Original], &original); err != nil || !bytes.Contains(original.Columns["structured_data"].Bytes, []byte(`"about":"system:first","about":"system:second"`)) {
		t.Fatal("ambiguous original bytes changed", original, err)
	}
}

func TestAssistanceRequiresPreservedOriginalBytes(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "gap", "Note", "active", "Source material", []byte("Original source body"), nil)
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	result, err := Run(context.Background(), r)
	if err != nil || len(result.Report.Queue) != 1 {
		t.Fatal(result, err)
	}
	q := result.Report.Queue[0]
	record := carrier.Record{Format: "haft/1", ID: "note-20260923-abcdef01", Kind: "note", Title: "Proposed recovery", Status: "proposed", Origin: "agent_proposal", About: "system:fixture", CreatedAt: stamp}
	raw, _ := carrier.Encode(record, []byte("Restored statement"))
	ar := AssistanceRequest{OutputRoot: r.OutputRoot, QueueID: q.ID, Proposal: raw, Reason: "Restore a selected source statement", RequestID: "original-check"}
	sourcePath := filepath.Join(r.OutputRoot, ".haft", filepath.FromSlash(q.Original))
	original, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range [][]byte{nil, []byte("foreign replacement")} {
		if replacement == nil {
			err = os.Remove(sourcePath)
		} else {
			err = os.WriteFile(sourcePath, replacement, 0644)
		}
		if err != nil {
			t.Fatal(err)
		}
		view, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		ar.ExpectedGeneration = view.Generation
		rejected, err := Assist(context.Background(), ar)
		if err == nil || rejected.Kind != "source_unavailable" {
			t.Fatal("missing or edited original accepted", rejected, err)
		}
	}
	if err := os.WriteFile(sourcePath, original, 0644); err != nil {
		t.Fatal(err)
	}
	view, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ar.ExpectedGeneration = view.Generation
	written, err := Assist(context.Background(), ar)
	if err != nil || written.Kind != "written" {
		t.Fatal("exact recovered original could not resume", written, err)
	}
	if err := os.Remove(sourcePath); err != nil {
		t.Fatal(err)
	}
	items, err := Queue(context.Background(), r.OutputRoot)
	if err != nil || len(items) != 1 || items[0].Status != "source_unavailable" {
		t.Fatal("queue still claims recovery after original disappears", items, err)
	}
}

func TestCarrierOnlyReadPreservesUnquotedTimestampAndSourceBytes(t *testing.T) {
	root := t.TempDir()
	raw := []byte("---\nid: old-short\nkind: Note\ntitle: Retained carrier\nstatus: active\nabout: system:fixture\ncreated_at: 2026-09-23T10:00:00Z\n---\nExact old body.\n")
	if err := os.WriteFile(filepath.Join(root, "note.md"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	r := Request{CarrierRoot: root, OutputRoot: t.TempDir(), CreatedAt: stamp, DryRun: true}
	result, err := Run(context.Background(), r)
	if err != nil || result.Kind != "staged" || len(result.Report.Queue) != 0 {
		t.Fatal(result, err)
	}
	v, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil || len(v.Documents) != 1 || v.Documents[0].Record.CreatedAt != stamp {
		t.Fatal(v.Documents, err)
	}
	aliases, err := LookupLegacyInView(v, "old-short")
	if err != nil || len(aliases) != 1 || !bytes.Equal(v.Files[aliases[0].Original], raw) {
		t.Fatal("carrier bytes or alias lost", aliases, err)
	}
	original, err := os.ReadFile(filepath.Join(root, "note.md"))
	if err != nil || !bytes.Equal(original, raw) {
		t.Fatal("original carrier mutated", err)
	}
}
