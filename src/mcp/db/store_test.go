package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_HolonCRUD(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	err = store.CreateHolon(ctx, "h1", "hypothesis", "system", "L0", "Test Hypothesis", "Content here", "ctx1", "scope1", "")
	if err != nil {
		t.Fatalf("CreateHolon failed: %v", err)
	}

	holon, err := store.GetHolon(ctx, "h1")
	if err != nil {
		t.Fatalf("GetHolon failed: %v", err)
	}

	if holon.ID != "h1" {
		t.Errorf("Expected ID 'h1', got '%s'", holon.ID)
	}
	if holon.Kind.String != "system" {
		t.Errorf("Expected Kind 'system', got '%s'", holon.Kind.String)
	}
	if holon.Layer != "L0" {
		t.Errorf("Expected Layer 'L0', got '%s'", holon.Layer)
	}

	err = store.UpdateHolonLayer(ctx, "h1", "L1")
	if err != nil {
		t.Fatalf("UpdateHolonLayer failed: %v", err)
	}

	holon, _ = store.GetHolon(ctx, "h1")
	if holon.Layer != "L1" {
		t.Errorf("Expected Layer 'L1' after update, got '%s'", holon.Layer)
	}

	title, err := store.GetHolonTitle(ctx, "h1")
	if err != nil {
		t.Fatalf("GetHolonTitle failed: %v", err)
	}
	if title != "Test Hypothesis" {
		t.Errorf("Expected title 'Test Hypothesis', got '%s'", title)
	}

	ids, err := store.ListAllHolonIDs(ctx)
	if err != nil {
		t.Fatalf("ListAllHolonIDs failed: %v", err)
	}
	if len(ids) != 1 || ids[0] != "h1" {
		t.Errorf("Expected ['h1'], got %v", ids)
	}
}

func TestStore_EvidenceCRUD(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	_ = store.CreateHolon(ctx, "h1", "hypothesis", "system", "L0", "Test", "Content", "ctx", "", "")

	err = store.AddEvidence(ctx, "e1", "h1", "test_result", "All tests pass", "pass", "L1", "internal-logic", "")
	if err != nil {
		t.Fatalf("AddEvidence failed: %v", err)
	}

	evidence, err := store.GetEvidence(ctx, "h1")
	if err != nil {
		t.Fatalf("GetEvidence failed: %v", err)
	}
	if len(evidence) != 1 {
		t.Fatalf("Expected 1 evidence, got %d", len(evidence))
	}
	if evidence[0].Verdict != "pass" {
		t.Errorf("Expected verdict 'pass', got '%s'", evidence[0].Verdict)
	}

	withCarrier, err := store.GetEvidenceWithCarrier(ctx)
	if err != nil {
		t.Fatalf("GetEvidenceWithCarrier failed: %v", err)
	}
	if len(withCarrier) != 1 {
		t.Errorf("Expected 1 evidence with carrier, got %d", len(withCarrier))
	}
}

func TestStore_RelationsCRUD(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	_ = store.CreateHolon(ctx, "parent", "hypothesis", "system", "L1", "Parent", "Content", "ctx", "", "")
	_ = store.CreateHolon(ctx, "child", "hypothesis", "system", "L0", "Child", "Content", "ctx", "", "")

	err = store.Link(ctx, "child", "parent", "componentOf")
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}

	components, err := store.GetComponentsOf(ctx, "parent")
	if err != nil {
		t.Fatalf("GetComponentsOf failed: %v", err)
	}
	if len(components) != 1 {
		t.Fatalf("Expected 1 component, got %d", len(components))
	}
	if components[0].SourceID != "child" {
		t.Errorf("Expected source 'child', got '%s'", components[0].SourceID)
	}
}

func TestStore_WorkRecords(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	start := time.Now()
	end := start.Add(time.Second)

	err = store.RecordWork(ctx, "w1", "TestMethod", "Agent", start, end, `{"duration_ms": 1000}`)
	if err != nil {
		t.Fatalf("RecordWork failed: %v", err)
	}
}

func TestStore_ParentChild(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	_ = store.CreateHolon(ctx, "l0-hypo", "hypothesis", "system", "L0", "L0 Hypothesis", "Content", "ctx", "", "")
	_ = store.CreateHolon(ctx, "l1-hypo", "hypothesis", "system", "L1", "L1 Verified", "Content", "ctx", "", "l0-hypo")
	_ = store.CreateHolon(ctx, "l2-hypo", "hypothesis", "system", "L2", "L2 Validated", "Content", "ctx", "", "l1-hypo")

	children, err := store.GetHolonsByParent(ctx, "l0-hypo")
	if err != nil {
		t.Fatalf("GetHolonsByParent failed: %v", err)
	}
	if len(children) != 1 || children[0].ID != "l1-hypo" {
		t.Errorf("Expected ['l1-hypo'], got %v", children)
	}

	lineage, err := store.GetHolonLineage(ctx, "l2-hypo")
	if err != nil {
		t.Fatalf("GetHolonLineage failed: %v", err)
	}
	if len(lineage) != 3 {
		t.Fatalf("Expected 3 holons in lineage, got %d", len(lineage))
	}
	if lineage[0].ID != "l0-hypo" || lineage[1].ID != "l1-hypo" || lineage[2].ID != "l2-hypo" {
		t.Errorf("Expected lineage [l0-hypo, l1-hypo, l2-hypo], got [%s, %s, %s]",
			lineage[0].ID, lineage[1].ID, lineage[2].ID)
	}
}

func TestStore_AuditLog(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	err = store.InsertAuditLog(ctx, "log-1", "quint_propose", "create_hypothesis", "agent", "hypo-1", "abc123", "SUCCESS", "", "default")
	if err != nil {
		t.Fatalf("InsertAuditLog failed: %v", err)
	}

	err = store.InsertAuditLog(ctx, "log-2", "quint_verify", "verify_hypothesis", "agent", "hypo-1", "def456", "SUCCESS", `{"verdict":"PASS"}`, "default")
	if err != nil {
		t.Fatalf("InsertAuditLog failed: %v", err)
	}

	logs, err := store.GetAuditLogByContext(ctx, "default")
	if err != nil {
		t.Fatalf("GetAuditLogByContext failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("Expected 2 logs, got %d", len(logs))
	}

	targetLogs, err := store.GetAuditLogByTarget(ctx, "hypo-1")
	if err != nil {
		t.Fatalf("GetAuditLogByTarget failed: %v", err)
	}
	if len(targetLogs) != 2 {
		t.Errorf("Expected 2 logs for hypo-1, got %d", len(targetLogs))
	}

	recentLogs, err := store.GetRecentAuditLog(ctx, 1)
	if err != nil {
		t.Fatalf("GetRecentAuditLog failed: %v", err)
	}
	if len(recentLogs) != 1 {
		t.Errorf("Expected 1 recent log, got %d", len(recentLogs))
	}
}

func TestStore_FileCleanup(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	store.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file should exist after close")
	}
}
