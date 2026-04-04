package agentloop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/jsonrpc"
	"github.com/m0n0x41d/haft/internal/protocol"

	_ "modernc.org/sqlite"
)

func TestOverseerCheckEmitsStructuredFindings(t *testing.T) {
	t.Parallel()

	store := setupOverseerArtifactStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	projectRoot := t.TempDir()
	writer := &lockedBuffer{}
	coordinatorAlerts := make(chan []string, 1)

	filePath := filepath.Join(projectRoot, "pkg", "foo.go")
	err := ensureFile(filePath, "package pkg\n\nfunc Foo() string { return \"new\" }\n")
	if err != nil {
		t.Fatalf("ensureFile: %v", err)
	}

	err = store.Create(ctx, &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         "dec-001",
			Kind:       artifact.KindDecisionRecord,
			Status:     artifact.StatusActive,
			Title:      "Decision with drift and debt",
			ValidUntil: now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
		},
		Body: "# Decision\n\nKeep this module stable.",
	})
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}

	err = store.SetAffectedFiles(ctx, "dec-001", []artifact.AffectedFile{
		{Path: "pkg/foo.go", Hash: "outdated-baseline"},
	})
	if err != nil {
		t.Fatalf("set affected files: %v", err)
	}

	err = store.AddEvidenceItem(ctx, &artifact.EvidenceItem{
		ID:              "ev-001",
		Type:            "measure",
		Content:         "Load check expired",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ValidUntil:      now.Add(-2 * 24 * time.Hour).Format(time.RFC3339),
	}, "dec-001")
	if err != nil {
		t.Fatalf("add expired evidence: %v", err)
	}

	err = store.AddEvidenceItem(ctx, &artifact.EvidenceItem{
		ID:              "ev-002",
		Type:            "measure",
		Content:         "Runtime regression",
		Verdict:         "refutes",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ValidUntil:      now.Add(7 * 24 * time.Hour).Format(time.RFC3339),
	}, "dec-001")
	if err != nil {
		t.Fatalf("add refuting evidence: %v", err)
	}

	_, err = store.DB().ExecContext(ctx, `
		INSERT INTO fpf_state (context_id, active_role, epistemic_debt_budget, updated_at)
		VALUES (?, ?, ?, ?)`,
		"default", "decide", 1.0, now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert fpf_state: %v", err)
	}

	overseer := &Overseer{
		ArtifactStore:   store,
		Bus:             newProtocolBusWithWriter(writer),
		CoordinatorChan: coordinatorAlerts,
		ProjectRoot:     projectRoot,
	}

	overseer.check(ctx)

	event := findOverseerAlertEvent(t, writer.String())
	if len(event.Alerts) == 0 {
		t.Fatal("expected overseer alerts")
	}
	if !containsString(event.Alerts, "⚑ 1 drifted") {
		t.Fatalf("expected drift summary, got %#v", event.Alerts)
	}
	if !containsString(event.Alerts, "⚠ 1 weak evidence") {
		t.Fatalf("expected weak evidence summary, got %#v", event.Alerts)
	}
	if !containsString(event.Alerts, "⚠ ED 2.0/1.0") {
		t.Fatalf("expected ED summary, got %#v", event.Alerts)
	}

	findings := findingsByType(event.Findings)
	if _, ok := findings["decision_stale"]; !ok {
		t.Fatalf("missing decision_stale finding: %#v", event.Findings)
	}
	if _, ok := findings["reff_degraded"]; !ok {
		t.Fatalf("missing reff_degraded finding: %#v", event.Findings)
	}
	debtFinding, ok := findings["ed_budget_exceeded"]
	if !ok {
		t.Fatalf("missing ed_budget_exceeded finding: %#v", event.Findings)
	}
	if math.Abs(debtFinding.TotalED-2.0) > 0.05 || math.Abs(debtFinding.Budget-1.0) > 0.0001 {
		t.Fatalf("unexpected debt finding totals: %#v", debtFinding)
	}
	if len(debtFinding.DebtBreakdown) != 1 {
		t.Fatalf("unexpected debt breakdown: %#v", debtFinding.DebtBreakdown)
	}

	driftFinding := findings["decision_stale"]
	if len(driftFinding.DriftItems) != 1 {
		t.Fatalf("unexpected drift items: %#v", driftFinding.DriftItems)
	}
	if driftFinding.DriftItems[0].Status != "modified" {
		t.Fatalf("drift status = %q, want modified", driftFinding.DriftItems[0].Status)
	}

	select {
	case queued := <-coordinatorAlerts:
		if len(queued) != len(event.Alerts) {
			t.Fatalf("coordinator alert count = %d, want %d", len(queued), len(event.Alerts))
		}
	case <-time.After(time.Second):
		t.Fatal("expected coordinator alerts")
	}
}

func TestOverseerRun_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	overseer := &Overseer{
		Bus:      newProtocolBus(),
		Interval: 10 * time.Millisecond,
	}

	go func() {
		overseer.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("overseer did not stop after context cancellation")
	}
}

func TestOverseerCheckEmitsScanFailureFinding(t *testing.T) {
	t.Parallel()

	store := setupOverseerArtifactStore(t)
	ctx := context.Background()
	writer := &lockedBuffer{}
	coordinatorAlerts := make(chan []string, 1)

	overseer := &Overseer{
		ArtifactStore: failingOverseerArtifactStore{
			ArtifactStore:      store,
			findStaleArtifacts: errors.New("stale artifact query failed"),
		},
		Bus:             newProtocolBusWithWriter(writer),
		CoordinatorChan: coordinatorAlerts,
	}

	overseer.check(ctx)

	event := findOverseerAlertEvent(t, writer.String())
	if !containsString(event.Alerts, "⚠ 1 scan failures") {
		t.Fatalf("expected scan failure summary, got %#v", event.Alerts)
	}

	findings := findingsByType(event.Findings)
	finding, ok := findings["scan_failed"]
	if !ok {
		t.Fatalf("missing scan_failed finding: %#v", event.Findings)
	}
	if !strings.Contains(finding.Reason, "stale artifact scan failed") {
		t.Fatalf("unexpected scan failure reason: %q", finding.Reason)
	}

	select {
	case queued := <-coordinatorAlerts:
		if !containsString(queued, "⚠ 1 scan failures") {
			t.Fatalf("expected coordinator scan failure alert, got %#v", queued)
		}
	case <-time.After(time.Second):
		t.Fatal("expected coordinator alerts")
	}
}

func setupOverseerArtifactStore(t *testing.T) *artifact.Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "overseer.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	stmts := []string{
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY, kind TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active', context TEXT, mode TEXT,
			title TEXT NOT NULL, content TEXT NOT NULL, file_path TEXT,
			valid_until TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			search_keywords TEXT DEFAULT '', structured_data TEXT DEFAULT '')`,
		`CREATE TABLE artifact_links (
			source_id TEXT NOT NULL, target_id TEXT NOT NULL, link_type TEXT NOT NULL,
			created_at TEXT NOT NULL, PRIMARY KEY (source_id, target_id, link_type))`,
		`CREATE TABLE evidence_items (
			id TEXT PRIMARY KEY, artifact_ref TEXT NOT NULL, type TEXT NOT NULL,
			content TEXT NOT NULL, verdict TEXT, carrier_ref TEXT,
			congruence_level INTEGER DEFAULT 3, formality_level INTEGER DEFAULT 5,
			claim_scope TEXT DEFAULT '[]',
			valid_until TEXT, created_at TEXT NOT NULL)`,
		`CREATE TABLE affected_files (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL, file_hash TEXT,
			PRIMARY KEY (artifact_id, file_path))`,
		`CREATE TABLE affected_symbols (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL,
			symbol_name TEXT NOT NULL, symbol_kind TEXT NOT NULL,
			symbol_line INTEGER, symbol_end_line INTEGER, symbol_hash TEXT,
			PRIMARY KEY (artifact_id, file_path, symbol_name))`,
		`CREATE TABLE fpf_state (
			context_id TEXT PRIMARY KEY,
			active_role TEXT,
			epistemic_debt_budget REAL DEFAULT 30.0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE audit_log (
			id TEXT PRIMARY KEY,
			timestamp TEXT NOT NULL,
			operation TEXT NOT NULL)`,
	}

	for _, stmt := range stmts {
		_, err := db.Exec(stmt)
		if err != nil {
			t.Fatalf("setup overseer db: %v\nSQL: %s", err, stmt)
		}
	}

	return artifact.NewStore(db)
}

func ensureFile(path, content string) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), 0o644)
}

func findOverseerAlertEvent(t *testing.T, output string) protocol.OverseerAlert {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var message jsonrpc.Message
		err := json.Unmarshal([]byte(line), &message)
		if err != nil {
			t.Fatalf("unmarshal jsonrpc message: %v", err)
		}

		if message.Method != protocol.MethodOverseerAlert {
			continue
		}

		var event protocol.OverseerAlert
		err = json.Unmarshal(message.Params, &event)
		if err != nil {
			t.Fatalf("unmarshal overseer.alert params: %v", err)
		}
		return event
	}

	t.Fatalf("overseer.alert event not found in %q", output)
	return protocol.OverseerAlert{}
}

func findingsByType(findings []protocol.OverseerFinding) map[string]protocol.OverseerFinding {
	result := make(map[string]protocol.OverseerFinding, len(findings))
	for _, finding := range findings {
		result[finding.Type] = finding
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type failingOverseerArtifactStore struct {
	artifact.ArtifactStore
	findStaleArtifacts error
}

func (s failingOverseerArtifactStore) FindStaleArtifacts(ctx context.Context) ([]*artifact.Artifact, error) {
	if s.findStaleArtifacts != nil {
		return nil, s.findStaleArtifacts
	}
	return s.ArtifactStore.FindStaleArtifacts(ctx)
}
