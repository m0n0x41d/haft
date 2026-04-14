package graph

import (
	"context"
	"testing"
)

func TestComputeImpactSet_DirectOnly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-cache", "internal/cache", "cache")
	seedDecision(t, db, "dec-001", "Cache strategy",
		[]string{"Use Redis"}, []string{"internal/cache/redis.go"})

	items, err := store.ComputeImpactSet(ctx, "mod-cache")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 impact item, got %d", len(items))
	}
	if !items[0].IsDirect {
		t.Fatal("expected direct impact")
	}
	if items[0].DecisionID != "dec-001" {
		t.Fatalf("expected dec-001, got %s", items[0].DecisionID)
	}
}

func TestComputeImpactSet_Transitive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-core", "internal/core", "core")
	seedModule(t, db, "mod-api", "internal/api", "api")

	// api imports core
	seedDep(t, db, "mod-api", "mod-core")

	// Decision governs API module
	seedDecision(t, db, "dec-api", "API design",
		[]string{"REST only"}, []string{"internal/api/handler.go"})

	// When core changes, API decision should be in impact set
	items, err := store.ComputeImpactSet(ctx, "mod-core")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 transitive impact, got %d", len(items))
	}
	if items[0].IsDirect {
		t.Fatal("expected transitive (indirect) impact")
	}
	if items[0].DecisionID != "dec-api" {
		t.Fatalf("expected dec-api, got %s", items[0].DecisionID)
	}
}

func TestComputeImpactForFile(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-core", "internal/core", "core")
	seedModule(t, db, "mod-api", "internal/api", "api")
	seedDep(t, db, "mod-api", "mod-core")

	seedDecision(t, db, "dec-core", "Core invariants",
		[]string{"Pure functions only"}, []string{"internal/core/calc.go"})
	seedDecision(t, db, "dec-api", "API design",
		[]string{"REST only"}, []string{"internal/api/handler.go"})

	// Changing a core file should impact both core and API decisions
	items, err := store.ComputeImpactForFile(ctx, "internal/core/calc.go")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) < 2 {
		t.Fatalf("expected at least 2 impact items (direct + transitive), got %d", len(items))
	}

	decIDs := map[string]bool{}
	for _, item := range items {
		decIDs[item.DecisionID] = true
	}
	if !decIDs["dec-core"] {
		t.Fatal("expected dec-core in impact set")
	}
	if !decIDs["dec-api"] {
		t.Fatal("expected dec-api in impact set (transitive)")
	}
}

func TestComputeImpactForFile_NoModule(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	// File with direct decision but no module
	seedDecision(t, db, "dec-001", "Root config",
		nil, []string{"config.yaml"})

	items, err := store.ComputeImpactForFile(ctx, "config.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].DecisionID != "dec-001" {
		t.Fatalf("expected dec-001, got %s", items[0].DecisionID)
	}
}
