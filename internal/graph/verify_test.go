package graph

import (
	"context"
	"testing"
)

func TestVerifyInvariants_NoDependencyHolds(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-api", "internal/api", "api")
	seedModule(t, db, "mod-db", "internal/db", "db")
	// No dependency from api to db
	seedDecision(t, db, "dec-001", "Layer separation",
		[]string{"no dependency from api to db"},
		[]string{"internal/api/handler.go"})

	results, err := VerifyInvariants(ctx, store, db, "dec-001")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != InvariantHolds {
		t.Fatalf("expected holds, got %s: %s", results[0].Status, results[0].Reason)
	}
}

func TestVerifyInvariants_NoDependencyViolated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-api", "internal/api", "api")
	seedModule(t, db, "mod-db", "internal/db", "db")
	// Add forbidden dependency
	seedDep(t, db, "mod-api", "mod-db")

	seedDecision(t, db, "dec-001", "Layer separation",
		[]string{"no dependency from api to db"},
		[]string{"internal/api/handler.go"})

	results, err := VerifyInvariants(ctx, store, db, "dec-001")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != InvariantViolated {
		t.Fatalf("expected violated, got %s: %s", results[0].Status, results[0].Reason)
	}
	if results[0].Reason == "" {
		t.Fatal("expected violation reason")
	}
}

func TestVerifyInvariants_NoCyclesHolds(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-a", "pkg/a", "a")
	seedModule(t, db, "mod-b", "pkg/b", "b")
	seedDep(t, db, "mod-a", "mod-b") // a→b only, no cycle

	seedDecision(t, db, "dec-001", "Architecture",
		[]string{"no circular dependencies in module graph"},
		[]string{"pkg/a/main.go"})

	results, err := VerifyInvariants(ctx, store, db, "dec-001")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != InvariantHolds {
		t.Fatalf("expected holds, got %s: %s", results[0].Status, results[0].Reason)
	}
}

func TestVerifyInvariants_NoCyclesViolated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-a", "pkg/a", "a")
	seedModule(t, db, "mod-b", "pkg/b", "b")
	seedDep(t, db, "mod-a", "mod-b")
	seedDep(t, db, "mod-b", "mod-a") // cycle!

	seedDecision(t, db, "dec-001", "Architecture",
		[]string{"no circular dependencies"},
		[]string{"pkg/a/main.go"})

	results, err := VerifyInvariants(ctx, store, db, "dec-001")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != InvariantViolated {
		t.Fatalf("expected violated, got %s: %s", results[0].Status, results[0].Reason)
	}
}

func TestVerifyInvariants_UnknownPattern(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedDecision(t, db, "dec-001", "Quality",
		[]string{"All public functions must have documentation"},
		[]string{"pkg/api/handler.go"})

	results, err := VerifyInvariants(ctx, store, db, "dec-001")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != InvariantUnknown {
		t.Fatalf("expected unknown, got %s", results[0].Status)
	}
}

func TestVerifyInvariants_MultipleInvariants(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-api", "internal/api", "api")
	seedModule(t, db, "mod-db", "internal/db", "db")
	seedModule(t, db, "mod-cache", "internal/cache", "cache")
	seedDep(t, db, "mod-api", "mod-cache") // allowed
	// no api→db dependency (good)

	seedDecision(t, db, "dec-001", "Architecture rules",
		[]string{
			"no dependency from api to db",
			"no circular dependencies",
			"All handlers must validate input",
		},
		[]string{"internal/api/handler.go"})

	results, err := VerifyInvariants(ctx, store, db, "dec-001")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// First: no dependency from api to db → holds (no such dep)
	if results[0].Status != InvariantHolds {
		t.Fatalf("invariant 0: expected holds, got %s", results[0].Status)
	}
	// Second: no circular dependencies → holds
	if results[1].Status != InvariantHolds {
		t.Fatalf("invariant 1: expected holds, got %s", results[1].Status)
	}
	// Third: unrecognized pattern → unknown
	if results[2].Status != InvariantUnknown {
		t.Fatalf("invariant 2: expected unknown, got %s", results[2].Status)
	}
}
