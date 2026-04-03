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
		ID:        "cyc-001",
		SessionID: sess.ID,
		Phase:     agent.PhaseMeasure,
		Depth:     agent.DepthStandard,
		Status:    agent.CycleComplete,
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
}
