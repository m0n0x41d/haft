package agentloop

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/session"
)

func TestComputeClosedCycleAssurance_SyncsArtifactEvidenceAndTraversesDependencies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	coord, store, rawDB := setupCoordinatorHarness(t)
	now := time.Now().UTC()

	seedCodebaseDependencyGraph(t, ctx, rawDB, now)
	createDecisionArtifact(t, ctx, store, "dec-b", now.Add(24*time.Hour), []string{"internal/shared/store.go"})
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

	createDecisionArtifact(t, ctx, store, "dec-a", now.Add(48*time.Hour), []string{"internal/api/router.go"})
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
		storedCL        int
		storedScope     string
		storedLevel     string
	)
	err = rawDB.QueryRowContext(ctx, `
		SELECT verdict, formality_level, congruence_level, claim_scope, assurance_level
		FROM evidence
		WHERE id = ?`,
		"artifact:ev-a",
	).Scan(&storedVerdict, &storedFormality, &storedCL, &storedScope, &storedLevel)
	if err != nil {
		t.Fatalf("query synced evidence: %v", err)
	}

	if storedVerdict != "accepted" {
		t.Fatalf("stored verdict = %q, want accepted", storedVerdict)
	}
	if storedFormality != 2 {
		t.Fatalf("stored formality = %d, want 2", storedFormality)
	}
	if storedCL != 3 {
		t.Fatalf("stored congruence = %d, want 3", storedCL)
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

	var projectedRelations int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM relations
		WHERE source_id = ? AND target_id = ? AND relation_type = ?`,
		"dec-a",
		"dec-b",
		projectedDependencyRelation,
	).Scan(&projectedRelations)
	if err != nil {
		t.Fatalf("count projected relations: %v", err)
	}
	if projectedRelations != 1 {
		t.Fatalf("projected dependency relations = %d, want 1", projectedRelations)
	}
}

func TestComputeClosedCycleAssurance_UsesPersistedArtifactCongruence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	coord, store, _ := setupCoordinatorHarness(t)
	now := time.Now().UTC()

	createDecisionArtifact(t, ctx, store, "dec-cl", now.Add(24*time.Hour), []string{"internal/api/self_check.go"})
	err := store.AddEvidenceItem(ctx, &artifact.EvidenceItem{
		ID:              "ev-cl",
		Type:            "measurement",
		Content:         "Self-evidence without baseline",
		Verdict:         "accepted",
		CongruenceLevel: 1,
		FormalityLevel:  2,
		ClaimScope:      []string{"criterion/self-check"},
		ValidUntil:      now.Add(24 * time.Hour).Format(time.RFC3339),
	}, "dec-cl")
	if err != nil {
		t.Fatalf("add evidence: %v", err)
	}

	assuranceTuple, _, err := coord.computeClosedCycleAssurance(ctx, "dec-cl", &agent.EvidenceChain{
		DecRef: "dec-cl",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "verdict: accepted", 1),
		},
	})
	if err != nil {
		t.Fatalf("compute closed-cycle assurance: %v", err)
	}

	if assuranceTuple.R != 0.4 {
		t.Fatalf("R = %.2f, want 0.40 with CL1 cycle evidence preserved", assuranceTuple.R)
	}
}

func TestComputeClosedCycleAssurance_PreservesDurableDependenciesWithoutInference(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	coord, store, rawDB := setupCoordinatorHarness(t)
	now := time.Now().UTC()

	createDecisionArtifact(t, ctx, store, "dec-dep", now.Add(-24*time.Hour), []string{"internal/unknown/dep.go"})
	err := store.AddEvidenceItem(ctx, &artifact.EvidenceItem{
		ID:              "ev-dep",
		Type:            "measurement",
		Content:         "Dependency evidence expired",
		Verdict:         "accepted",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ValidUntil:      now.Add(-24 * time.Hour).Format(time.RFC3339),
	}, "dec-dep")
	if err != nil {
		t.Fatalf("add dependency evidence: %v", err)
	}

	_, _, err = coord.computeClosedCycleAssurance(ctx, "dec-dep", &agent.EvidenceChain{
		DecRef: "dec-dep",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "dependency accepted", 3),
		},
	})
	if err != nil {
		t.Fatalf("sync dependency assurance: %v", err)
	}

	createDecisionArtifact(t, ctx, store, "dec-root", now.Add(24*time.Hour), []string{"internal/unknown/root.go"})
	err = store.AddEvidenceItem(ctx, &artifact.EvidenceItem{
		ID:              "ev-root",
		Type:            "measurement",
		Content:         "Primary evidence fresh",
		Verdict:         "accepted",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ValidUntil:      now.Add(24 * time.Hour).Format(time.RFC3339),
	}, "dec-root")
	if err != nil {
		t.Fatalf("add root evidence: %v", err)
	}

	_, _, err = coord.computeClosedCycleAssurance(ctx, "dec-root", &agent.EvidenceChain{
		DecRef: "dec-root",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "root accepted", 3),
		},
	})
	if err != nil {
		t.Fatalf("prime root assurance: %v", err)
	}

	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO relations (source_id, target_id, relation_type, congruence_level, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		"dec-root",
		"dec-dep",
		"dependsOn",
		3,
		now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed durable dependency relation: %v", err)
	}

	assuranceTuple, weakestLink, err := coord.computeClosedCycleAssurance(ctx, "dec-root", &agent.EvidenceChain{
		DecRef: "dec-root",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "root accepted", 3),
		},
	})
	if err != nil {
		t.Fatalf("compute assurance with durable dependency: %v", err)
	}

	if assuranceTuple.R != 0.1 {
		t.Fatalf("R = %.2f, want 0.10 when durable dependency remains active", assuranceTuple.R)
	}
	if weakestLink != "dependency dec-dep" {
		t.Fatalf("weakest link = %q, want dependency dec-dep", weakestLink)
	}

	var relationCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM relations
		WHERE source_id = ? AND target_id = ? AND relation_type = ?`,
		"dec-root",
		"dec-dep",
		"dependsOn",
	).Scan(&relationCount)
	if err != nil {
		t.Fatalf("count durable dependency relation: %v", err)
	}
	if relationCount != 1 {
		t.Fatalf("dependency relation count = %d, want 1", relationCount)
	}
}

func TestComputeClosedCycleAssurance_MergesProjectedAndManualDependencies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	coord, store, rawDB := setupCoordinatorHarness(t)
	now := time.Now().UTC()

	seedCodebaseDependencyGraph(t, ctx, rawDB, now)
	createDecisionArtifact(t, ctx, store, "dec-projected", now.Add(24*time.Hour), []string{"internal/shared/store.go"})
	err := store.AddEvidenceItem(ctx, &artifact.EvidenceItem{
		ID:              "ev-projected",
		Type:            "measurement",
		Content:         "Projected dependency remains healthy",
		Verdict:         "accepted",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ValidUntil:      now.Add(24 * time.Hour).Format(time.RFC3339),
	}, "dec-projected")
	if err != nil {
		t.Fatalf("add projected evidence: %v", err)
	}

	createDecisionArtifact(t, ctx, store, "dec-manual", now.Add(-24*time.Hour), []string{"internal/manual/dep.go"})
	err = store.AddEvidenceItem(ctx, &artifact.EvidenceItem{
		ID:              "ev-manual",
		Type:            "measurement",
		Content:         "Manual dependency has expired evidence",
		Verdict:         "accepted",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ValidUntil:      now.Add(-24 * time.Hour).Format(time.RFC3339),
	}, "dec-manual")
	if err != nil {
		t.Fatalf("add manual evidence: %v", err)
	}

	createDecisionArtifact(t, ctx, store, "dec-root-merge", now.Add(24*time.Hour), []string{"internal/api/router.go"})
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO relations (source_id, target_id, relation_type, congruence_level, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		"dec-root-merge",
		"dec-manual",
		"dependsOn",
		3,
		now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed manual dependency relation: %v", err)
	}

	assuranceTuple, weakestLink, err := coord.computeClosedCycleAssurance(ctx, "dec-root-merge", &agent.EvidenceChain{
		DecRef: "dec-root-merge",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "root accepted", 3),
		},
	})
	if err != nil {
		t.Fatalf("compute assurance with projected and manual dependencies: %v", err)
	}

	if assuranceTuple.R != 0.1 {
		t.Fatalf("R = %.2f, want 0.10 when manual dependency is preserved", assuranceTuple.R)
	}
	if weakestLink != "dependency dec-manual" {
		t.Fatalf("weakest link = %q, want dependency dec-manual", weakestLink)
	}

	var projectedRelationCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM relations
		WHERE source_id = ? AND relation_type = ?`,
		"dec-root-merge",
		projectedDependencyRelation,
	).Scan(&projectedRelationCount)
	if err != nil {
		t.Fatalf("count projected dependency relations: %v", err)
	}
	if projectedRelationCount != 1 {
		t.Fatalf("projected dependency relation count = %d, want 1", projectedRelationCount)
	}

	var manualRelationCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM relations
		WHERE source_id = ? AND relation_type = ?`,
		"dec-root-merge",
		manualDependencyRelation,
	).Scan(&manualRelationCount)
	if err != nil {
		t.Fatalf("count manual dependency relations: %v", err)
	}
	if manualRelationCount != 1 {
		t.Fatalf("manual dependency relation count = %d, want 1", manualRelationCount)
	}

	err = store.SetAffectedFiles(ctx, "dec-root-merge", []artifact.AffectedFile{{Path: "internal/shared/root.go"}})
	if err != nil {
		t.Fatalf("update root affected files: %v", err)
	}

	assuranceTuple, weakestLink, err = coord.computeClosedCycleAssurance(ctx, "dec-root-merge", &agent.EvidenceChain{
		DecRef: "dec-root-merge",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "root accepted after graph change", 3),
		},
	})
	if err != nil {
		t.Fatalf("recompute assurance after graph change: %v", err)
	}

	if assuranceTuple.R != 0.1 {
		t.Fatalf("R after graph change = %.2f, want 0.10 from preserved manual dependency", assuranceTuple.R)
	}
	if weakestLink != "dependency dec-manual" {
		t.Fatalf("weakest link after graph change = %q, want dependency dec-manual", weakestLink)
	}

	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM relations
		WHERE source_id = ? AND relation_type = ?`,
		"dec-root-merge",
		projectedDependencyRelation,
	).Scan(&projectedRelationCount)
	if err != nil {
		t.Fatalf("count stale projected dependency relations: %v", err)
	}
	if projectedRelationCount != 0 {
		t.Fatalf("projected dependency relation count after graph change = %d, want 0", projectedRelationCount)
	}

	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM relations
		WHERE source_id = ? AND relation_type = ?`,
		"dec-root-merge",
		manualDependencyRelation,
	).Scan(&manualRelationCount)
	if err != nil {
		t.Fatalf("count preserved manual dependency relations: %v", err)
	}
	if manualRelationCount != 1 {
		t.Fatalf("manual dependency relation count after graph change = %d, want 1", manualRelationCount)
	}
}

func TestComputeClosedCycleAssurance_PersistsExplicitCycleEvidence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	coord, store, rawDB := setupCoordinatorHarness(t)
	now := time.Now().UTC()

	createDecisionArtifact(t, ctx, store, "dec-cycle", now.Add(72*time.Hour), []string{"internal/runtime/check.go"})

	explicitMeasure := agent.NewEvidenceItem(agent.EvidenceMeasure, "validated internal/runtime/check.go criterion/runtime", 2)
	explicitMeasure.Formality = agent.FormalityStructuredFormal
	explicitMeasure.ClaimScope = []string{"criterion/runtime", "internal/runtime/check.go"}
	explicitMeasure.CapturedAt = now

	explicitAttached := agent.NewEvidenceItem(agent.EvidenceAttached, "attached benchmark criterion/latency", 1)
	explicitAttached.Formality = agent.FormalityStructuredInformal
	explicitAttached.ClaimScope = []string{"criterion/latency"}
	explicitAttached.CapturedAt = now.Add(time.Minute)

	assuranceTuple, _, err := coord.computeClosedCycleAssurance(ctx, "dec-cycle", &agent.EvidenceChain{
		DecRef: "dec-cycle",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.ObservationFileReview, "internal/runtime/check.go", 3),
			explicitMeasure,
			explicitAttached,
		},
	})
	if err != nil {
		t.Fatalf("compute assurance with cycle evidence: %v", err)
	}

	if assuranceTuple.F != agent.FormalityStructuredInformal {
		t.Fatalf("F = %d, want %d", assuranceTuple.F, agent.FormalityStructuredInformal)
	}
	if assuranceTuple.R != 0.3 {
		t.Fatalf("R = %.2f, want 0.30 with attached evidence as weakest link", assuranceTuple.R)
	}
	if !reflect.DeepEqual(assuranceTuple.G, []string{
		"criterion/latency",
		"criterion/runtime",
		"internal/runtime/check.go",
	}) {
		t.Fatalf("G = %#v, want union of explicit cycle scopes", assuranceTuple.G)
	}

	rows, err := rawDB.QueryContext(ctx, `
		SELECT id, verdict, formality_level, congruence_level, claim_scope, assurance_level, carrier_ref
		FROM evidence
		WHERE holon_id = ? AND assurance_level = ?
		ORDER BY id`,
		"dec-cycle",
		cycleAssuranceLevel,
	)
	if err != nil {
		t.Fatalf("query cycle assurance evidence: %v", err)
	}
	defer rows.Close()

	type storedRow struct {
		id        string
		verdict   string
		formality int
		cl        int
		scope     string
		level     string
		carrier   sql.NullString
	}

	var stored []storedRow

	for rows.Next() {
		var row storedRow
		err = rows.Scan(&row.id, &row.verdict, &row.formality, &row.cl, &row.scope, &row.level, &row.carrier)
		if err != nil {
			t.Fatalf("scan cycle assurance evidence: %v", err)
		}
		stored = append(stored, row)
	}

	err = rows.Err()
	if err != nil {
		t.Fatalf("iterate cycle assurance evidence: %v", err)
	}

	if len(stored) != 2 {
		t.Fatalf("stored cycle evidence rows = %d, want 2 explicit rows", len(stored))
	}

	if stored[0].id != "cycle:dec-cycle:001" {
		t.Fatalf("first cycle evidence id = %q, want cycle:dec-cycle:001", stored[0].id)
	}
	if stored[0].verdict != "accepted" {
		t.Fatalf("first cycle verdict = %q, want accepted", stored[0].verdict)
	}
	if stored[0].formality != agent.FormalityStructuredFormal {
		t.Fatalf("first cycle formality = %d, want %d", stored[0].formality, agent.FormalityStructuredFormal)
	}
	if stored[0].cl != 2 {
		t.Fatalf("first cycle congruence = %d, want 2", stored[0].cl)
	}
	if stored[0].scope != "[\"criterion/runtime\",\"internal/runtime/check.go\"]" {
		t.Fatalf("first cycle scope = %q, want runtime scope JSON", stored[0].scope)
	}
	if stored[0].level != cycleAssuranceLevel {
		t.Fatalf("first cycle assurance level = %q, want %q", stored[0].level, cycleAssuranceLevel)
	}
	if stored[0].carrier.String != "cycle:dec-cycle" {
		t.Fatalf("first cycle carrier = %q, want cycle:dec-cycle", stored[0].carrier.String)
	}

	if stored[1].id != "cycle:dec-cycle:002" {
		t.Fatalf("second cycle evidence id = %q, want cycle:dec-cycle:002", stored[1].id)
	}
	if stored[1].verdict != "partial" {
		t.Fatalf("second cycle verdict = %q, want partial", stored[1].verdict)
	}
	if stored[1].formality != agent.FormalityStructuredInformal {
		t.Fatalf("second cycle formality = %d, want %d", stored[1].formality, agent.FormalityStructuredInformal)
	}
	if stored[1].cl != 1 {
		t.Fatalf("second cycle congruence = %d, want 1", stored[1].cl)
	}
	if stored[1].scope != "[\"criterion/latency\"]" {
		t.Fatalf("second cycle scope = %q, want latency scope JSON", stored[1].scope)
	}
}

func TestComputeClosedCycleAssurance_PreservesDependencyCycleEvidence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	coord, store, rawDB := setupCoordinatorHarness(t)
	now := time.Now().UTC()

	createDecisionArtifact(t, ctx, store, "dec-cycle-dep", now.Add(48*time.Hour), []string{"internal/runtime/dep.go"})
	dependencyAssurance, _, err := coord.computeClosedCycleAssurance(ctx, "dec-cycle-dep", &agent.EvidenceChain{
		DecRef: "dec-cycle-dep",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "dependency accepted", 3),
		},
	})
	if err != nil {
		t.Fatalf("seed dependency cycle evidence: %v", err)
	}
	if dependencyAssurance.R != 0.8 {
		t.Fatalf("dependency R = %.2f, want 0.80", dependencyAssurance.R)
	}

	createDecisionArtifact(t, ctx, store, "dec-cycle-root", now.Add(48*time.Hour), []string{"internal/runtime/root.go"})
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO relations (source_id, target_id, relation_type, congruence_level, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		"dec-cycle-root",
		"dec-cycle-dep",
		"dependsOn",
		3,
		now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed dependency relation: %v", err)
	}

	rootAssurance, weakestLink, err := coord.computeClosedCycleAssurance(ctx, "dec-cycle-root", &agent.EvidenceChain{
		DecRef: "dec-cycle-root",
		Items: []agent.EvidenceItem{
			agent.NewEvidenceItem(agent.EvidenceMeasure, "root accepted", 3),
		},
	})
	if err != nil {
		t.Fatalf("compute root assurance: %v", err)
	}

	if rootAssurance.R != 0.8 {
		t.Fatalf("root R = %.2f, want 0.80 when dependency cycle evidence is preserved", rootAssurance.R)
	}
	if weakestLink != "dependency dec-cycle-dep" {
		t.Fatalf("weakest link = %q, want dependency dec-cycle-dep", weakestLink)
	}

	var cycleEvidenceCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM evidence
		WHERE holon_id = ? AND assurance_level = ?`,
		"dec-cycle-dep",
		cycleAssuranceLevel,
	).Scan(&cycleEvidenceCount)
	if err != nil {
		t.Fatalf("count preserved dependency cycle evidence: %v", err)
	}
	if cycleEvidenceCount != 1 {
		t.Fatalf("dependency cycle evidence count = %d, want 1", cycleEvidenceCount)
	}
}

func TestComputeClosedCycleAssurance_PreservesAttachedOnlyCycleScore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	coord, store, rawDB := setupCoordinatorHarness(t)
	now := time.Now().UTC()

	createDecisionArtifact(t, ctx, store, "dec-attached", now.Add(48*time.Hour), []string{"internal/runtime/attached.go"})
	attached := agent.NewEvidenceItem(agent.EvidenceAttached, "attached criterion/attached", 1)
	attached.ClaimScope = []string{"criterion/attached"}
	attached.CapturedAt = now

	chain := &agent.EvidenceChain{
		DecRef: "dec-attached",
		Items:  []agent.EvidenceItem{attached},
	}

	assuranceTuple, weakestLink, err := coord.computeClosedCycleAssurance(ctx, "dec-attached", chain)
	if err != nil {
		t.Fatalf("compute assurance with attached-only evidence: %v", err)
	}

	expected := agent.ComputeAssurance(chain)
	if assuranceTuple.R != expected.R {
		t.Fatalf("R = %.2f, want %.2f to match in-memory attached evidence", assuranceTuple.R, expected.R)
	}
	if weakestLink != "attached (score: 0.3)" {
		t.Fatalf("weakest link = %q, want attached (score: 0.3)", weakestLink)
	}

	var storedType string
	var storedVerdict string
	err = rawDB.QueryRowContext(ctx, `
		SELECT type, verdict
		FROM evidence
		WHERE holon_id = ? AND assurance_level = ?`,
		"dec-attached",
		cycleAssuranceLevel,
	).Scan(&storedType, &storedVerdict)
	if err != nil {
		t.Fatalf("query attached cycle evidence: %v", err)
	}

	if storedType != "attached" {
		t.Fatalf("stored type = %q, want attached", storedType)
	}
	if storedVerdict != "partial" {
		t.Fatalf("stored verdict = %q, want partial", storedVerdict)
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
	affectedFiles []string,
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

	files := make([]artifact.AffectedFile, 0, len(affectedFiles))
	for _, affectedFile := range affectedFiles {
		files = append(files, artifact.AffectedFile{Path: affectedFile})
	}

	err = store.SetAffectedFiles(ctx, id, files)
	if err != nil {
		t.Fatalf("set affected files for %s: %v", id, err)
	}
}

func seedCodebaseDependencyGraph(t *testing.T, ctx context.Context, rawDB *sql.DB, now time.Time) {
	t.Helper()

	_, err := rawDB.ExecContext(ctx, `
		INSERT INTO codebase_modules (module_id, path, name, lang, file_count, last_scanned)
		VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)`,
		"mod-internal-api",
		"internal/api",
		"api",
		"go",
		2,
		now.Format(time.RFC3339),
		"mod-internal-shared",
		"internal/shared",
		"shared",
		"go",
		1,
		now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed codebase modules: %v", err)
	}

	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO module_dependencies (source_module, target_module, dep_type, file_path, last_scanned)
		VALUES (?, ?, ?, ?, ?)`,
		"mod-internal-api",
		"mod-internal-shared",
		"import",
		"internal/api/router.go",
		now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed module dependency: %v", err)
	}
}

func TestCaptureDecisionSelection_ReturnsRepairErrorForAmbiguousComparedPortfolio(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	coord, store, rawDB := setupCoordinatorHarness(t)

	cycleStore, err := session.NewSQLiteStore(rawDB)
	if err != nil {
		t.Fatalf("new sqlite cycle store: %v", err)
	}
	coord.Cycles = cycleStore

	err = store.Create(ctx, &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "sol-ambiguous",
			Kind:  artifact.KindSolutionPortfolio,
			Title: "Ambiguous legacy portfolio",
		},
		Body: `# Ambiguous legacy portfolio

## Variants (2)

### V7. Kafka

### V7. NATS

## Comparison

Legacy comparison body.
`,
		StructuredData: `{}`,
	})
	if err != nil {
		t.Fatalf("create ambiguous portfolio: %v", err)
	}

	err = cycleStore.CreateCycle(ctx, &agent.Cycle{
		ID:                   "cyc-ambiguous",
		SessionID:            "sess-ambiguous",
		Phase:                agent.PhaseDecider,
		Status:               agent.CycleActive,
		PortfolioRef:         "sol-ambiguous",
		ComparedPortfolioRef: "sol-ambiguous",
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}

	err = coord.captureDecisionSelection(ctx, "sess-ambiguous", "pick V7")
	if err == nil {
		t.Fatal("expected ambiguous compared portfolio to surface a repair error")
	}
	if !strings.Contains(err.Error(), "Repair the portfolio identity set or re-run compare before deciding") {
		t.Fatalf("unexpected repair error: %v", err)
	}
}
