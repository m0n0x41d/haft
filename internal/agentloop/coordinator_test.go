package agentloop

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestComputeClosedCycleAssurance_SyncsArtifactEvidenceAndTraversesDependencies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	coord, store, rawDB := setupCoordinatorHarness(t)
	now := time.Now().UTC()

	createDecisionArtifact(t, ctx, store, "dec-b", now.Add(24*time.Hour))
	err := store.AddEvidenceItem(ctx, &artifact.EvidenceItem{
		ID:              "ev-b",
		Type:            "measurement",
		Content:         "Dependency evidence expired",
		Verdict:         "accepted",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ClaimScope:      []string{"criterion/dependency"},
		ValidUntil:      now.Add(-24 * time.Hour).Format(time.RFC3339),
	}, "dec-b")
	if err != nil {
		t.Fatalf("add dependency evidence: %v", err)
	}

	_, _, err = coord.computeClosedCycleAssurance(ctx, "dec-b", &agent.EvidenceChain{
		DecRef: "dec-b",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "verdict: accepted", 3),
		},
	})
	if err != nil {
		t.Fatalf("sync dependency assurance: %v", err)
	}

	createDecisionArtifact(t, ctx, store, "dec-a", now.Add(48*time.Hour))
	err = store.AddEvidenceItem(ctx, &artifact.EvidenceItem{
		ID:              "ev-a",
		Type:            "measurement",
		Content:         "Primary decision validated",
		Verdict:         "accepted",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ClaimScope:      []string{"criterion/latency"},
		ValidUntil:      now.Add(48 * time.Hour).Format(time.RFC3339),
	}, "dec-a")
	if err != nil {
		t.Fatalf("add primary evidence: %v", err)
	}

	_, _, err = coord.computeClosedCycleAssurance(ctx, "dec-a", &agent.EvidenceChain{
		DecRef: "dec-a",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "verdict: accepted", 3),
		},
	})
	if err != nil {
		t.Fatalf("prime primary assurance: %v", err)
	}

	_, err = rawDB.ExecContext(ctx,
		`INSERT INTO relations (source_id, target_id, relation_type, congruence_level, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		"dec-a",
		"dec-b",
		"dependsOn",
		3,
		now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert dependency relation: %v", err)
	}

	assuranceTuple, weakestLink, err := coord.computeClosedCycleAssurance(ctx, "dec-a", &agent.EvidenceChain{
		DecRef: "dec-a",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "verdict: accepted", 3),
		},
	})
	if err != nil {
		t.Fatalf("compute closed-cycle assurance: %v", err)
	}

	if assuranceTuple.R != 0.1 {
		t.Fatalf("R = %.2f, want 0.10 after dependency decay", assuranceTuple.R)
	}
	if assuranceTuple.F != 2 {
		t.Fatalf("F = %d, want 2", assuranceTuple.F)
	}
	if !reflect.DeepEqual(assuranceTuple.G, []string{"criterion/latency"}) {
		t.Fatalf("G = %#v, want latency scope", assuranceTuple.G)
	}
	if weakestLink != "dependency dec-b" {
		t.Fatalf("weakest link = %q, want dependency dec-b", weakestLink)
	}

	var (
		storedVerdict   string
		storedFormality int
		storedScope     string
		storedLevel     string
	)
	err = rawDB.QueryRowContext(ctx, `
		SELECT verdict, formality_level, claim_scope, assurance_level
		FROM evidence
		WHERE id = ?`,
		"artifact:ev-a",
	).Scan(&storedVerdict, &storedFormality, &storedScope, &storedLevel)
	if err != nil {
		t.Fatalf("query synced evidence: %v", err)
	}

	if storedVerdict != "accepted" {
		t.Fatalf("stored verdict = %q, want accepted", storedVerdict)
	}
	if storedFormality != 2 {
		t.Fatalf("stored formality = %d, want 2", storedFormality)
	}
	if storedScope != "[\"criterion/latency\"]" {
		t.Fatalf("stored scope = %q, want latency claim scope", storedScope)
	}
	if storedLevel != assuranceAdapterLevel {
		t.Fatalf("assurance level = %q, want %q", storedLevel, assuranceAdapterLevel)
	}

	var syncedCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM evidence
		WHERE holon_id = ? AND assurance_level = ?`,
		"dec-a",
		assuranceAdapterLevel,
	).Scan(&syncedCount)
	if err != nil {
		t.Fatalf("count synced evidence: %v", err)
	}
	if syncedCount != 1 {
		t.Fatalf("synced evidence count = %d, want 1", syncedCount)
	}
}

func setupCoordinatorHarness(t *testing.T) (*Coordinator, *artifact.Store, *sql.DB) {
	t.Helper()

	dbPath := t.TempDir() + "/coordinator.db"
	kernelStore, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("new kernel store: %v", err)
	}
	t.Cleanup(func() { _ = kernelStore.Close() })

	artStore := artifact.NewStore(kernelStore.GetRawDB())
	coord := &Coordinator{ArtifactStore: artStore}

	return coord, artStore, kernelStore.GetRawDB()
}

func createDecisionArtifact(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
	id string,
	validUntil time.Time,
) {
	t.Helper()

	now := time.Now().UTC()
	err := store.Create(ctx, &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         id,
			Kind:       artifact.KindDecisionRecord,
			Status:     artifact.StatusActive,
			Title:      id,
			ValidUntil: validUntil.Format(time.RFC3339),
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		Body: "# Decision\n\nBody",
	})
	if err != nil {
		t.Fatalf("create decision %s: %v", id, err)
	}
}
