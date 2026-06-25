package specflow

import (
	"errors"
	"path/filepath"
	"strings"
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
	b.Line = 42                                // line is excluded
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

func TestHashSectionChangesWhenSystemFrameChanges(t *testing.T) {
	a := project.SpecSection{ID: "tgt-env-1"}
	b := a
	b.SystemFrame = project.SystemReferenceFrame{
		ID:     "target_system",
		Kind:   "target_system",
		Source: "system_frame",
	}

	if HashSection(a) == HashSection(b) {
		t.Fatalf("hash unchanged when system_frame changed: %s", HashSection(a))
	}
}

func TestHashSectionChangesWhenClaimChanges(t *testing.T) {
	a := project.SpecSection{ID: "tgt-env-1"}
	b := a
	b.Claims = []project.SpecClaim{{
		ID:                   "claim-1",
		Class:                "A",
		Statement:            "Claim-scoped authority is load-bearing.",
		Scope:                []string{"tgt-env-1"},
		SupportRefs:          []string{"dec-1"},
		EvidenceRefs:         []string{"ev-1"},
		ValidUntil:           "2026-08-01",
		GoverningPatternRefs: []string{"A.7"},
	}}

	if HashSection(a) == HashSection(b) {
		t.Fatalf("hash unchanged when claims changed: %s", HashSection(a))
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
	if got.Kind != BaselineKindSpecSectionApproval {
		t.Fatalf("kind = %q, want %q", got.Kind, BaselineKindSpecSectionApproval)
	}
	if got.ApprovedBy != "human" {
		t.Fatalf("approved_by = %q, want %q", got.ApprovedBy, "human")
	}
	if got.CapturedAt.IsZero() {
		t.Fatalf("captured_at should be set on Put")
	}
}

func TestSpecSectionApprovalBaselineConstructorCreatesTypedProjection(t *testing.T) {
	section := activeEnvironmentSection()
	captured := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

	snapshot := NewSpecSectionApprovalBaseline("proj-1", section, "human", captured)
	projection := snapshot.SectionBaseline()

	if projection.Kind != BaselineKindSpecSectionApproval {
		t.Fatalf("kind = %q, want %q", projection.Kind, BaselineKindSpecSectionApproval)
	}
	if projection.Hash != HashSection(section) {
		t.Fatalf("hash = %q, want HashSection(section)", projection.Hash)
	}
	if projection.CapturedAt != captured {
		t.Fatalf("captured_at = %s, want %s", projection.CapturedAt, captured)
	}
}

func TestBaselineKindProfilesKeepSnapshotMeaningsDistinct(t *testing.T) {
	cases := []struct {
		kind      BaselineKind
		object    string
		authority string
	}{
		{
			kind:      BaselineKindSpecSectionApproval,
			object:    "SpecSectionApprovalBaseline",
			authority: "spec_lifecycle_approval_baseline",
		},
		{
			kind:      BaselineKindPreWorkReference,
			object:    "PreWorkReferenceSnapshot",
			authority: "work_reference_only",
		},
		{
			kind:      BaselineKindVerifiedState,
			object:    "VerifiedStateSnapshot",
			authority: "evidence_measurement_only",
		},
		{
			kind:      BaselineKindUnknownLegacy,
			object:    "UnknownLegacyBaseline",
			authority: "unknown_legacy_do_not_strengthen",
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			profile := DescribeBaselineKind(tc.kind)
			if profile.Object != tc.object {
				t.Fatalf("object = %q, want %q", profile.Object, tc.object)
			}
			if profile.AuthorityBoundary != tc.authority {
				t.Fatalf("authority = %q, want %q", profile.AuthorityBoundary, tc.authority)
			}
		})
	}
}

func TestBaselineStoreRejectsNonSpecApprovalSnapshotKind(t *testing.T) {
	store := NewMemoryBaselineStore()

	err := store.Put(SectionBaseline{
		Kind:      BaselineKindVerifiedState,
		ProjectID: "proj-1",
		SectionID: "tgt-env-1",
		Hash:      "abc123",
	})
	if err == nil {
		t.Fatal("Put accepted verified_state_snapshot as a spec section approval baseline")
	}
	if !strings.Contains(err.Error(), string(BaselineKindSpecSectionApproval)) {
		t.Fatalf("error = %v, want spec-section approval boundary", err)
	}
}

func TestBaselineStoreRejectsNonSpecApprovalRewrite(t *testing.T) {
	store := NewMemoryBaselineStore()
	approval := SectionBaseline{
		Kind:       BaselineKindSpecSectionApproval,
		ProjectID:  "proj-1",
		SectionID:  "tgt-env-1",
		Hash:       "approval-hash",
		ApprovedBy: "human",
	}
	if err := store.Put(approval); err != nil {
		t.Fatalf("Put approval baseline: %v", err)
	}

	rewriteAttempts := []SectionBaseline{
		{
			Kind:      BaselineKindPreWorkReference,
			ProjectID: "proj-1",
			SectionID: "tgt-env-1",
			Hash:      "planned-hash",
		},
		{
			Kind:      BaselineKindVerifiedState,
			ProjectID: "proj-1",
			SectionID: "tgt-env-1",
			Hash:      "verified-hash",
		},
	}
	for _, attempt := range rewriteAttempts {
		if err := store.Put(attempt); err == nil {
			t.Fatalf("Put accepted %s rewrite of spec approval baseline", attempt.Kind)
		}
	}

	got, err := store.Get("proj-1", "tgt-env-1")
	if err != nil {
		t.Fatalf("Get approval baseline: %v", err)
	}
	if got.Hash != approval.Hash {
		t.Fatalf("hash = %q, want preserved approval hash %q", got.Hash, approval.Hash)
	}
	if got.ApprovedBy != approval.ApprovedBy {
		t.Fatalf("approved_by = %q, want preserved approval actor %q", got.ApprovedBy, approval.ApprovedBy)
	}
	if got.Kind != BaselineKindSpecSectionApproval {
		t.Fatalf("kind = %q, want %q", got.Kind, BaselineKindSpecSectionApproval)
	}
}

func TestParseBaselineKindPreservesUnknownLegacyPosture(t *testing.T) {
	for _, raw := range []string{"", "  ", "legacy", "pre_work_reference"} {
		if got := ParseBaselineKind(raw); got != BaselineKindUnknownLegacy {
			t.Fatalf("ParseBaselineKind(%q) = %q, want %q", raw, got, BaselineKindUnknownLegacy)
		}
	}

	if got := ParseBaselineKind(string(BaselineKindSpecSectionApproval)); got != BaselineKindSpecSectionApproval {
		t.Fatalf("ParseBaselineKind(spec) = %q", got)
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
	if got.Kind != BaselineKindSpecSectionApproval {
		t.Fatalf("kind = %q, want %q", got.Kind, BaselineKindSpecSectionApproval)
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
