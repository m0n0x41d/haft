package session

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/agent"
	_ "modernc.org/sqlite"
)

func TestSQLiteStore_PersistsCycleAssuranceTuple(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "session.db")
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	store, err := NewSQLiteStore(sqlDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	sess := &agent.Session{
		ID:        "sess-001",
		Title:     "assurance test",
		Model:     "test-model",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	cycle := &agent.Cycle{
		ID:                   "cyc-001",
		SessionID:            sess.ID,
		Phase:                agent.PhaseMeasure,
		Depth:                agent.DepthStandard,
		Status:               agent.CycleComplete,
		ComparedPortfolioRef: "port-001",
		SelectedPortfolioRef: "port-001",
		SelectedVariantRef:   "V2",
		Assurance: agent.AssuranceTuple{
			F: 2,
			G: []string{"criterion/latency", "criterion/throughput"},
			R: 0.72,
		},
		CLMin:     2,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateCycle(ctx, cycle); err != nil {
		t.Fatalf("CreateCycle: %v", err)
	}

	stored, err := store.GetCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("GetCycle: %v", err)
	}
	if stored.Assurance.F != 2 {
		t.Errorf("Assurance.F = %d, want 2", stored.Assurance.F)
	}
	if stored.Assurance.R != 0.72 {
		t.Errorf("Assurance.R = %.2f, want 0.72", stored.Assurance.R)
	}
	if !reflect.DeepEqual(stored.Assurance.G, []string{"criterion/latency", "criterion/throughput"}) {
		t.Errorf("Assurance.G = %#v", stored.Assurance.G)
	}
	if stored.REff != 0.72 {
		t.Errorf("REff = %.2f, want 0.72", stored.REff)
	}
	if stored.ComparedPortfolioRef != "port-001" {
		t.Errorf("ComparedPortfolioRef = %q, want port-001", stored.ComparedPortfolioRef)
	}
	if stored.SelectedPortfolioRef != "port-001" {
		t.Errorf("SelectedPortfolioRef = %q, want port-001", stored.SelectedPortfolioRef)
	}
	if stored.SelectedVariantRef != "V2" {
		t.Errorf("SelectedVariantRef = %q, want V2", stored.SelectedVariantRef)
	}

	cycle.ComparedPortfolioRef = "port-002"
	cycle.SelectedPortfolioRef = "port-002"
	cycle.SelectedVariantRef = "V1"
	cycle.Assurance = agent.AssuranceTuple{
		F: 1,
		G: []string{"criterion/latency"},
		R: 0.41,
	}
	if err := store.UpdateCycle(ctx, cycle); err != nil {
		t.Fatalf("UpdateCycle: %v", err)
	}

	updated, err := store.GetCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("GetCycle after update: %v", err)
	}
	if updated.Assurance.F != 1 {
		t.Errorf("updated Assurance.F = %d, want 1", updated.Assurance.F)
	}
	if updated.Assurance.R != 0.41 {
		t.Errorf("updated Assurance.R = %.2f, want 0.41", updated.Assurance.R)
	}
	if !reflect.DeepEqual(updated.Assurance.G, []string{"criterion/latency"}) {
		t.Errorf("updated Assurance.G = %#v", updated.Assurance.G)
	}
	if updated.REff != 0.41 {
		t.Errorf("updated REff = %.2f, want 0.41", updated.REff)
	}
	if updated.ComparedPortfolioRef != "port-002" {
		t.Errorf("updated ComparedPortfolioRef = %q, want port-002", updated.ComparedPortfolioRef)
	}
	if updated.SelectedPortfolioRef != "port-002" {
		t.Errorf("updated SelectedPortfolioRef = %q, want port-002", updated.SelectedPortfolioRef)
	}
	if updated.SelectedVariantRef != "V1" {
		t.Errorf("updated SelectedVariantRef = %q, want V1", updated.SelectedVariantRef)
	}
}

func TestNewSQLiteStore_RepairsMissingCyclesTableForMigration12(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "session.db")
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	_, err = sqlDB.Exec(`
		CREATE TABLE agent_schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO agent_schema_version (version) VALUES
			(1), (2), (3), (4), (5), (6), (7), (8), (9), (10);
		CREATE TABLE agent_sessions (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			current_phase TEXT DEFAULT '',
			depth TEXT DEFAULT 'standard',
			interaction TEXT DEFAULT 'symbiotic',
			parent_id TEXT DEFAULT '',
			active_cycle_id TEXT DEFAULT '',
			yolo INTEGER DEFAULT 0
		);
		CREATE TABLE agent_messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			parts_json TEXT NOT NULL,
			model TEXT DEFAULT '',
			tokens INTEGER DEFAULT 0,
			created_at TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("seed partial agent schema: %v", err)
	}

	store, err := NewSQLiteStore(sqlDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	var applied int
	err = sqlDB.QueryRow(`SELECT COUNT(*) FROM agent_schema_version WHERE version = 12`).Scan(&applied)
	if err != nil {
		t.Fatalf("read migration 12 marker: %v", err)
	}
	if applied != 1 {
		t.Fatalf("migration 12 marker count = %d, want 1", applied)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	sess := &agent.Session{
		ID:        "sess-restore",
		Title:     "repair test",
		Model:     "test-model",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	cycle := &agent.Cycle{
		ID:                   "cyc-restore",
		SessionID:            sess.ID,
		Phase:                agent.PhaseExplorer,
		Status:               agent.CycleActive,
		PortfolioRef:         "portfolio-compare",
		ComparedPortfolioRef: "portfolio-compare",
		SelectedPortfolioRef: "portfolio-compare",
		SelectedVariantRef:   "V2",
		Assurance: agent.AssuranceTuple{
			F: 1,
			G: []string{"criterion/speed"},
			R: 0.55,
		},
		CLMin:     2,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateCycle(ctx, cycle); err != nil {
		t.Fatalf("CreateCycle: %v", err)
	}

	stored, err := store.GetCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("GetCycle: %v", err)
	}
	if stored.ComparedPortfolioRef != "portfolio-compare" {
		t.Fatalf("ComparedPortfolioRef = %q, want portfolio-compare", stored.ComparedPortfolioRef)
	}
	if stored.SelectedPortfolioRef != "portfolio-compare" {
		t.Fatalf("SelectedPortfolioRef = %q, want portfolio-compare", stored.SelectedPortfolioRef)
	}
	if stored.SelectedVariantRef != "V2" {
		t.Fatalf("SelectedVariantRef = %q, want V2", stored.SelectedVariantRef)
	}
	if stored.Assurance.F != 1 {
		t.Fatalf("Assurance.F = %d, want 1", stored.Assurance.F)
	}
	if stored.REff != 0.55 {
		t.Fatalf("REff = %.2f, want 0.55", stored.REff)
	}
}

func TestSQLiteStore_CreateCycleCanonicalizesActiveCycleState(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "session.db")
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	store, err := NewSQLiteStore(sqlDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	sess := &agent.Session{
		ID:        "sess-canonical",
		Title:     "canonical cycle test",
		Model:     "test-model",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	cycle := &agent.Cycle{
		ID:                   "cyc-canonical",
		SessionID:            sess.ID,
		Phase:                agent.PhaseFramer,
		Status:               agent.CycleActive,
		ProblemRef:           "prob-001",
		PortfolioRef:         "sol-001",
		ComparedPortfolioRef: "sol-stale",
		SelectedPortfolioRef: "sol-stale",
		SelectedVariantRef:   "V2",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := store.CreateCycle(ctx, cycle); err != nil {
		t.Fatalf("CreateCycle: %v", err)
	}

	stored, err := store.GetCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("GetCycle: %v", err)
	}
	if stored.Phase != agent.PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", stored.Phase, agent.PhaseExplorer)
	}
	if stored.SelectedPortfolioRef != "" || stored.SelectedVariantRef != "" {
		t.Fatalf("selection = (%q, %q), want cleared", stored.SelectedPortfolioRef, stored.SelectedVariantRef)
	}
}

func TestSQLiteStore_CreateCanonicalizesSessionExecutionMode(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "session.db")
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	store, err := NewSQLiteStore(sqlDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	sess := &agent.Session{
		ID:          "sess-invalid-mode",
		Title:       "mode canonicalization",
		Model:       "test-model",
		Interaction: agent.ExecutionMode("invalid"),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.ExecutionMode() != agent.ExecutionModeSymbiotic {
		t.Fatalf("ExecutionMode = %q, want symbiotic", sess.ExecutionMode())
	}

	stored, err := store.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.ExecutionMode() != agent.ExecutionModeSymbiotic {
		t.Fatalf("stored ExecutionMode = %q, want symbiotic", stored.ExecutionMode())
	}
}

func TestSQLiteStore_GetNormalizesPersistedLegacyInteraction(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "session.db")
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	store, err := NewSQLiteStore(sqlDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	_, err = sqlDB.Exec(`
		INSERT INTO agent_sessions (
			id, parent_id, title, model, current_phase, depth, interaction, yolo, active_cycle_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sess-legacy-mode",
		"",
		"legacy mode",
		"test-model",
		"",
		"standard",
		"legacy",
		0,
		"",
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed session: %v", err)
	}

	stored, err := store.Get(context.Background(), "sess-legacy-mode")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.ExecutionMode() != agent.ExecutionModeSymbiotic {
		t.Fatalf("ExecutionMode = %q, want symbiotic", stored.ExecutionMode())
	}
}
