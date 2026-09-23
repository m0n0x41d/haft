package migrate

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/store"
)

const stamp = "2026-09-23T10:00:00Z"

func fixtureDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	for _, q := range []string{
		`PRAGMA journal_mode=WAL`, `PRAGMA wal_autocheckpoint=0`,
		`CREATE TABLE artifacts(id TEXT PRIMARY KEY,kind TEXT,status TEXT,title TEXT,content TEXT,structured_data TEXT,created_at TEXT)`,
		`CREATE TABLE artifact_links(source_id TEXT,target_id TEXT,link_type TEXT)`,
		`CREATE TABLE affected_files(artifact_id TEXT,file_path TEXT,file_hash TEXT)`,
		`CREATE TABLE affected_symbols(artifact_id TEXT,file_path TEXT,symbol_name TEXT,symbol_hash TEXT)`,
		`CREATE TABLE evidence_items(id TEXT PRIMARY KEY,artifact_ref TEXT,title TEXT,content TEXT,verdict TEXT,claim_scope TEXT,claim_refs TEXT,provenance TEXT,created_at TEXT)`,
		`CREATE TABLE evidence(id TEXT PRIMARY KEY,holon_id TEXT,title TEXT,content TEXT,verdict TEXT,claim_scope TEXT,provenance TEXT,created_at TEXT)`,
		`CREATE TABLE spec_section_editions(project_id TEXT,section_id TEXT,section_json TEXT,semantic_hash TEXT,created_at TEXT)`,
		`CREATE TABLE spec_section_baselines(section_id TEXT,hash TEXT,approved_by TEXT)`,
		`CREATE TABLE future_engine(payload TEXT)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return db, p
}
func insert(t *testing.T, db *sql.DB, id, kind, status, title string, body []byte, fields map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(fields)
	if _, err := db.Exec(`INSERT INTO artifacts VALUES(?,?,?,?,CAST(? AS TEXT),?,?)`, id, kind, status, title, body, string(raw), stamp); err != nil {
		t.Fatal(err)
	}
}
func sourceFiles(t *testing.T, p string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		b, err := os.ReadFile(p + suffix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		out[suffix] = b
	}
	return out
}
func basicRequest(db string) Request {
	return Request{DatabasePath: db, OutputRoot: "", CreatedAt: stamp, DryRun: true}
}
func scope(report Report, name string) Scope {
	for _, s := range report.Scopes {
		if s.Source == name {
			return s
		}
	}
	return Scope{}
}
func findItem(report Report, source string) Item {
	for _, item := range report.Items {
		if item.Source == source {
			return item
		}
	}
	return Item{}
}
func goodDecision() map[string]any {
	return map[string]any{"about": "dir:internal/ingest", "object": "Ingestion broker", "question": "Which broker provides the required delivery contract?", "selected_title": "kafka", "why_selected": "The team operates it for this delivery contract", "no_alternative_reason": "Only one supported deployment is available", "implementation_footprint": map[string]any{"files": []string{"internal/ingest/kafka.go"}}}
}

func TestWALCaptureStagesDBOnlyWithoutChangingOriginal(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "old-long-decision-id", "DecisionRecord", "active", "Choose broker", []byte("The original decision body."), goodDecision())
	insert(t, db, "old-note", "Note", "deprecated", "Old caveat", []byte("Historical caveat."), map[string]any{"about": "system:fixture"})
	insert(t, db, "missing-why", "DecisionRecord", "active", "Incomplete decision", []byte("Original incomplete content."), map[string]any{"about": "system:fixture"})
	for _, kind := range []string{"MethodRun", "RefreshReport", "WorkCommission"} {
		insert(t, db, "excluded-"+kind, kind, "active", kind, []byte("secret-excluded-payload-"+kind), nil)
	}
	if _, err := db.Exec(`INSERT INTO affected_files VALUES('old-long-decision-id','internal/ingest/kafka.go','old-hash'); INSERT INTO artifact_links VALUES('old-note','old-long-decision-id','constrains')`); err != nil {
		t.Fatal(err)
	}
	before := sourceFiles(t, p)
	if len(before["-wal"]) == 0 {
		t.Fatal("fixture has no uncheckpointed WAL")
	}
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	result, err := Run(context.Background(), r)
	if err != nil || result.Kind != "staged" {
		t.Fatal(result, err)
	}
	after := sourceFiles(t, p)
	if !equalFiles(before, after) {
		t.Fatal("migration altered original DB/WAL/SHM bytes")
	}
	view, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Documents) != 2 {
		t.Fatalf("DB-only recognized records were lost: %d %+v", len(view.Documents), result.Report.Items)
	}
	for _, entry := range view.Projection.Entries {
		if entry.State != carrier.Historical || entry.Document.Record.OperatorConfirmed {
			t.Fatal("historical active gained authority")
		}
		if entry.Document.Record.Kind == "decision" {
			if len(entry.Document.Record.Constrains) != 0 {
				t.Fatal("footprint became constraint")
			}
			found := false
			for _, link := range entry.Document.Record.Links {
				if link.LegacyType == "implementation_footprint" && link.Kind == "relates_to" {
					found = true
				}
			}
			if !found {
				t.Fatal("footprint navigation lost")
			}
		}
	}
	for _, raw := range view.Files {
		if bytes.Contains(raw, []byte("secret-excluded-payload")) {
			t.Fatal("excluded payload archived")
		}
	}
	for _, kind := range []string{"MethodRun", "RefreshReport", "WorkCommission"} {
		if len(result.Report.Excluded[kind]) != 1 {
			t.Fatal("excluded scope disappeared")
		}
	}
	if findItem(result.Report, "sqlite:artifacts:missing-why").Disposition != "needs_rewrite" || len(result.Report.Queue) != 1 {
		t.Fatal("partial loss hidden", result.Report.Queue)
	}
	if scope(result.Report, "sqlite:future_engine").Disposition != "unsupported_source" {
		t.Fatal("unknown table claimed read")
	}
	aliases, err := LookupLegacy(context.Background(), r.OutputRoot, "old-long-decision-id")
	if err != nil || len(aliases) != 1 || !carrier.ValidID(aliases[0].NativeID) {
		t.Fatal("legacy mapping missing", aliases, err)
	}
}

func TestBothEvidenceTablesKeepScopeAndNeverInventHistoricalTargets(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "old-target", "DecisionRecord", "active", "Target", []byte("Target body"), goodDecision())
	provenance, _ := json.Marshal(map[string]any{"about": "system:fixture", "method": "Recorded property run", "source": "reports/original.txt", "basis": map[string]any{"kind": "report", "ref": "original-report-42"}})
	if _, err := db.Exec(`INSERT INTO evidence_items VALUES('shared-id','old-target','Scoped evidence','Observed cases','refutes','["claim-only"]','["L1"]',?,?); INSERT INTO evidence VALUES('shared-id','old-target','Old evidence','Partial observation','partial','["limited"]',?,?)`, string(provenance), stamp, string(provenance), stamp); err != nil {
		t.Fatal(err)
	}
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	result, err := Run(context.Background(), r)
	if err != nil || result.Kind != "staged" {
		t.Fatal(result, err)
	}
	view, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	count := 0
	for _, d := range view.Documents {
		if d.Record.Kind != "evidence" {
			continue
		}
		count++
		ids[d.Record.ID] = true
		if len(d.Record.Uses) != 0 || d.Record.Legacy["historical_edition"] != "unknown" || d.Record.Legacy["claim_scope"] == nil {
			t.Fatal("migration invented target edition or lost scope")
		}
	}
	if count != 2 || len(ids) != 2 {
		t.Fatal("evidence table ID collision overwrote a row", count, ids)
	}
	aliases, err := LookupLegacy(context.Background(), r.OutputRoot, "shared-id")
	if err != nil || len(aliases) != 2 {
		t.Fatal("table-qualified aliases lost", aliases, err)
	}
	item := findItem(result.Report, "sqlite:evidence_items:shared-id")
	if len(item.Losses) == 0 {
		t.Fatal("evidence use loss not reported")
	}
}

func TestDBCarrierConflictInvalidBytesAndSpecClaimsRemainAddressable(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "conflict", "Note", "active", "Same title", []byte("DB content"), map[string]any{"about": "system:fixture"})
	insert(t, db, "bad-utf8", "Note", "active", "Bad bytes", []byte{0xff, 0xfe, 'x'}, map[string]any{"about": "system:fixture"})
	section := map[string]any{"id": "legacy-section", "title": "Legacy section", "status": "active", "about": "system:fixture", "slug": "legacy-section", "receiving_use": "Inspect imported statements", "created_at": stamp, "claims": []any{map[string]any{"id": "L1", "kind": "law", "statement": "Total is preserved", "scope": []string{"domain"}}, map[string]any{"id": "E1", "class": "E", "statement": "A test passed", "scope": []string{"one execution"}}}}
	b, _ := json.Marshal(section)
	if _, err := db.Exec(`INSERT INTO spec_section_editions VALUES('project','legacy-section',?,'old-hash',?)`, string(b), stamp); err != nil {
		t.Fatal(err)
	}
	carriers := t.TempDir()
	if err := os.MkdirAll(filepath.Join(carriers, "notes"), 0755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte("---\nid: conflict\nkind: Note\nstatus: active\ntitle: Same title\nabout: system:fixture\ncreated_at: \"" + stamp + "\"\n---\nCarrier content\n")
	if err := os.WriteFile(filepath.Join(carriers, "notes", "conflict.md"), legacy, 0644); err != nil {
		t.Fatal(err)
	}
	r := basicRequest(p)
	r.CarrierRoot = carriers
	r.OutputRoot = t.TempDir()
	result, err := Run(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	item := findItem(result.Report, "sqlite:artifacts:conflict")
	if item.Disposition != "needs_rewrite" || !strings.Contains(strings.Join(item.Reasons, " "), "source_conflict") {
		t.Fatal("DB/carrier winner silently chosen", item)
	}
	bad := findItem(result.Report, "sqlite:artifacts:bad-utf8")
	if bad.Disposition != "needs_rewrite" || !strings.Contains(strings.Join(bad.Reasons, " "), "encoding_error") {
		t.Fatal("invalid UTF8 silently replaced", bad)
	}
	view, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var original Row
	if err := json.Unmarshal(view.Files[bad.Original], &original); err != nil || !bytes.Equal(original.Columns["content"].Bytes, []byte{0xff, 0xfe, 'x'}) {
		t.Fatal("original bytes unrecoverable")
	}
	for _, d := range view.Documents {
		if d.Record.Kind == "spec" {
			if len(d.Record.Claims) != 1 || d.Record.Claims[0].ID != "L1" || d.Record.Claims[0].Unchecked == "" {
				t.Fatal("E became a normative claim or harness invented")
			}
		}
	}
	if len(findItem(result.Report, "sqlite:spec_section_editions:project/legacy-section").Losses) < 2 {
		t.Fatal("omitted historical E claim not reported")
	}
}

func TestRetryPreservesQueueAndAssistanceIsProposed(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "old-gap", "Note", "active", "Useful fact", []byte("The original source omits a subject."), nil)
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	first, err := Run(context.Background(), r)
	if err != nil || first.Kind != "staged" {
		t.Fatal(first, err)
	}
	again, err := Run(context.Background(), r)
	if err != nil || again.Kind != "replayed" {
		t.Fatal(again, err)
	}
	items, err := Queue(context.Background(), r.OutputRoot)
	if err != nil || len(items) != 1 || items[0].Status != "pending" {
		t.Fatal(items, err)
	}
	proposal := carrier.Record{Format: "haft/1", ID: "note-20260923-aabbccdd", Kind: "note", Title: "Recovered source subject", Status: "proposed", Origin: "agent_proposal", About: "system:fixture", CreatedAt: stamp}
	raw, _ := carrier.Encode(proposal, []byte("The original source omits a subject. Current fixture context supplies system:fixture; no observation of correctness is added.\n"))
	view, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assistReq := AssistanceRequest{OutputRoot: r.OutputRoot, QueueID: items[0].ID, Proposal: raw, Reason: "Current fixture context supplies the missing subject; original statement is preserved", RequestID: "repair-one", ExpectedGeneration: view.Generation}
	res, err := Assist(context.Background(), assistReq)
	if err != nil || res.Kind != "written" {
		t.Fatal(res, err)
	}
	res, err = Assist(context.Background(), assistReq)
	if err != nil || res.Kind != "replayed" {
		t.Fatal("assistance duplicate", res, err)
	}
	items, err = Queue(context.Background(), r.OutputRoot)
	if err != nil || len(items) != 1 || items[0].Status != "proposed_recovery" || items[0].ProposalRef == "" {
		t.Fatal("queue cannot resume", items, err)
	}
	again, err = Run(context.Background(), r)
	if err != nil || again.Kind != "replayed" {
		t.Fatal("migration retry clobbered later assistance", again, err)
	}
	view, err = (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resolved := view.Resolve(items[0].ProposalRef)
	if resolved.Kind != "found" || resolved.Document.Record.OperatorConfirmed || resolved.Document.Record.Origin != "agent_proposal" {
		t.Fatal("repair gained authority", resolved)
	}
}

func TestChangedSourceConflictNeverOverwritesStageOrOriginal(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "old-note", "Note", "active", "Native note", []byte("First body"), map[string]any{"about": "system:fixture"})
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	r.RequestID = "fixed-request"
	first, err := Run(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE artifacts SET content='Second body' WHERE id='old-note'`); err != nil {
		t.Fatal(err)
	}
	before := sourceFiles(t, p)
	second, err := Run(context.Background(), r)
	if err != nil || second.Kind != "request_conflict" {
		t.Fatal("changed snapshot reused receipt", second, err)
	}
	if !equalFiles(before, sourceFiles(t, p)) {
		t.Fatal("source modified")
	}
	view, err := (store.Store{Root: r.OutputRoot}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Documents) != 1 || !bytes.Contains(view.Documents[0].Body, []byte("First body")) {
		t.Fatal("staging overwritten")
	}
	reportFile := filepath.Join(r.OutputRoot, ".haft", filepath.FromSlash(first.ReportPath))
	if err := os.WriteFile(reportFile, []byte("external edit"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := requestPayload(r, first.Report.SourceDigest)
	result, found, err := (store.Store{Root: r.OutputRoot}).Replay(context.Background(), r.RequestID, carrier.Digest(payload))
	if err != nil || !found || result.Kind != "replay_conflict" {
		t.Fatal("edited report was overwritten or trusted", result, found, err)
	}
}

func TestLimitsMissingSchemaAndRootBoundaries(t *testing.T) {
	db, p := fixtureDB(t)
	for i := 0; i < 3; i++ {
		insert(t, db, fmt.Sprintf("n%d", i), "Note", "active", "note", []byte("body"), map[string]any{"about": "system:fixture"})
	}
	r := basicRequest(p)
	r.OutputRoot = t.TempDir()
	r.MaxRows = 1
	result, err := Run(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	sc := scope(result.Report, "sqlite:artifacts")
	if sc.Disposition != "source_unreadable" || sc.Count == nil || *sc.Count != 3 {
		t.Fatal("truncation claimed full read", sc)
	}
	bad := r
	bad.OutputRoot = filepath.Dir(p)
	if _, err := Run(context.Background(), bad); err == nil {
		t.Fatal("staging overlaps source")
	}
	bad = r
	bad.DryRun = false
	if _, err := Run(context.Background(), bad); err == nil {
		t.Fatal("live activation allowed")
	}
	brokenDB := filepath.Join(t.TempDir(), "broken.db")
	if err := os.WriteFile(brokenDB, []byte("not sqlite"), 0644); err != nil {
		t.Fatal(err)
	}
	bad = basicRequest(brokenDB)
	bad.OutputRoot = t.TempDir()
	out, err := Run(context.Background(), bad)
	if err != nil {
		t.Fatal(err)
	}
	if scope(out.Report, "sqlite:"+brokenDB).Disposition != "source_unreadable" {
		t.Fatal("broken DB reported empty success")
	}
}

func TestConcurrentWALWriterYieldsConsistentOrDiagnosedCapture(t *testing.T) {
	db, p := fixtureDB(t)
	insert(t, db, "a", "Note", "active", "a", []byte("0"), map[string]any{"about": "system:fixture"})
	insert(t, db, "b", "Note", "active", "b", []byte("0"), map[string]any{"about": "system:fixture"})
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			tx, err := db.Begin()
			if err != nil {
				return
			}
			_, err = tx.Exec(`UPDATE artifacts SET content=?`, fmt.Sprint(i))
			if err != nil {
				tx.Rollback()
				return
			}
			if tx.Commit() != nil {
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	r := basicRequest(p)
	captured, err := Capture(context.Background(), r)
	close(stop)
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if len(captured.DatabaseFiles) == 0 {
		if len(captured.Diagnostics) == 0 {
			t.Fatal("moving source silently treated as absent")
		}
		return
	}
	values := []string{}
	for _, row := range captured.Rows {
		if row.Table == "artifacts" {
			values = append(values, text(row, "content"))
		}
	}
	if len(values) == 0 {
		if len(captured.Diagnostics) == 0 {
			t.Fatal("failed private snapshot had no diagnostic")
		}
		return
	}
	if len(values) != 2 || values[0] != values[1] {
		t.Fatal("captured WAL split one committed transaction", values)
	}
}
