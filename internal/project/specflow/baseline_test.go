package specflow

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
)

func newTestBaselineDB(t *testing.T) *db.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "haft.db")
	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestHashSectionIsDeterministicForSameLoadBearingFields(t *testing.T) {
	a := project.SpecSection{
		ID:            "tgt-env-1",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "Environment change",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "human",
		Status:        "active",
		ValidUntil:    "2026-10-28",
		Terms:         []string{"Harnessability"},
	}
	b := a
	b.Path = "different/path/target-system.md" // path is excluded from hash
	b.Line = 42                                 // line is excluded
	b.Malformed = false

	if HashSection(a) != HashSection(b) {
		t.Fatalf("hash differs on excluded fields:\n  a=%s\n  b=%s", HashSection(a), HashSection(b))
	}
}

func TestHashSectionChangesWhenLoadBearingFieldChanges(t *testing.T) {
	a := project.SpecSection{
		ID:            "tgt-env-1",
		StatementType: "definition",
		ClaimLayer:    "object",
		ValidUntil:    "2026-10-28",
	}

	b := a
	b.ValidUntil = "2026-11-01" // valid_until is load-bearing

	if HashSection(a) == HashSection(b) {
		t.Fatalf("hash unchanged when valid_until changed: %s", HashSection(a))
	}
}

func TestHashSectionTreatsTrimmedWhitespaceAsEqual(t *testing.T) {
	a := project.SpecSection{ID: "tgt-1", Status: "active"}
	b := project.SpecSection{ID: " tgt-1 ", Status: "active "}

	if HashSection(a) != HashSection(b) {
		t.Fatalf("hash should ignore leading/trailing whitespace")
	}
}

func TestMemoryBaselineStoreGetReturnsNotFoundWhenAbsent(t *testing.T) {
	store := NewMemoryBaselineStore()

	_, err := store.Get("proj-1", "tgt-env-1")
	if !errors.Is(err, ErrBaselineNotFound) {
		t.Fatalf("err = %v, want ErrBaselineNotFound", err)
	}
}

func TestMemoryBaselineStoreRoundTrip(t *testing.T) {
	store := NewMemoryBaselineStore()

	baseline := SectionBaseline{
		ProjectID:  "proj-1",
		SectionID:  "tgt-env-1",
		Hash:       "abc123",
		ApprovedBy: "human",
	}
	if err := store.Put(baseline); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get("proj-1", "tgt-env-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Hash != "abc123" {
		t.Fatalf("hash = %q, want %q", got.Hash, "abc123")
	}
	if got.ApprovedBy != "human" {
		t.Fatalf("approved_by = %q, want %q", got.ApprovedBy, "human")
	}
	if got.CapturedAt.IsZero() {
		t.Fatalf("captured_at should be set on Put")
	}
}

func TestMemoryBaselineStoreUpsertReplacesExisting(t *testing.T) {
	store := NewMemoryBaselineStore()
	store.Put(SectionBaseline{ProjectID: "p", SectionID: "s", Hash: "v1"})
	store.Put(SectionBaseline{ProjectID: "p", SectionID: "s", Hash: "v2"})

	got, _ := store.Get("p", "s")
	if got.Hash != "v2" {
		t.Fatalf("hash = %q, want %q (upsert should replace)", got.Hash, "v2")
	}
}

func TestSQLiteBaselineStoreRoundTripWithMigration(t *testing.T) {
	dbStore := newTestBaselineDB(t)
	store := NewSQLiteBaselineStore(dbStore.GetRawDB())
	now := time.Now().UTC().Truncate(time.Second)

	baseline := SectionBaseline{
		ProjectID:  "qnt_test",
		SectionID:  "tgt-role-1",
		Hash:       "deadbeef",
		CapturedAt: now,
		ApprovedBy: "human",
	}
	if err := store.Put(baseline); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get("qnt_test", "tgt-role-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Hash != "deadbeef" {
		t.Fatalf("hash = %q, want %q", got.Hash, "deadbeef")
	}
	if got.ApprovedBy != "human" {
		t.Fatalf("approved_by = %q, want human", got.ApprovedBy)
	}

	// Delete + Get -> not found.
	if err := store.Delete("qnt_test", "tgt-role-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get("qnt_test", "tgt-role-1"); !errors.Is(err, ErrBaselineNotFound) {
		t.Fatalf("err = %v, want ErrBaselineNotFound after Delete", err)
	}
}

func TestSQLiteBaselineStoreListForProjectScopesByProject(t *testing.T) {
	dbStore := newTestBaselineDB(t)
	store := NewSQLiteBaselineStore(dbStore.GetRawDB())
	store.Put(SectionBaseline{ProjectID: "p1", SectionID: "s1", Hash: "h1"})
	store.Put(SectionBaseline{ProjectID: "p1", SectionID: "s2", Hash: "h2"})
	store.Put(SectionBaseline{ProjectID: "p2", SectionID: "s3", Hash: "h3"})

	rows, err := store.ListForProject("p1")
	if err != nil {
		t.Fatalf("ListForProject: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}

	otherRows, _ := store.ListForProject("p2")
	if len(otherRows) != 1 {
		t.Fatalf("len(otherRows) = %d, want 1", len(otherRows))
	}
}
