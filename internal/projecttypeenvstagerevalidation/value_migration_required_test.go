package projecttypeenvstagerevalidation

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestValueMigrationRequiredRemainsUnderdeterminedAndBlocksSelection(
	t *testing.T,
) {
	digest := mustStageRevalidationDigest(t, "a")
	target, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef(): %v", err)
	}
	graphRef, err := projecttypeenvassertionreport.ParseGraphSnapshotRef(
		"project-graph-snapshot-basis:" + digest.String(),
	)
	if err != nil {
		t.Fatalf("ParseGraphSnapshotRef(): %v", err)
	}
	graph, err := projecttypeenvassertionreport.NewGraphSnapshotCoordinate(
		graphRef,
		typedmemory.NewGraphRevision(7),
		digest,
	)
	if err != nil {
		t.Fatalf("NewGraphSnapshotCoordinate(): %v", err)
	}
	runtime, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(
		"runtime-evaluation-basis:" + digest.String(),
	)
	if err != nil {
		t.Fatalf("ParseRuntimeEvaluationBasisRef(): %v", err)
	}
	assertion, err := typedmemory.NewAssertionID("assertion:value-migration-required")
	if err != nil {
		t.Fatalf("NewAssertionID(): %v", err)
	}
	ground, err := projecttypeenvassertionreport.NewMissingBasisGround(
		projecttypeenvassertionreport.CodeValueMigrationRequired,
		"relation.slot[value]",
		"stored canonical bytes belong to another codec or value shape",
		nil,
		"append-a-new-value-and-assertion-under-the-target-binding",
	)
	if err != nil {
		t.Fatalf("NewMissingBasisGround(): %v", err)
	}
	outcome, err := projecttypeenvassertionreport.NewAssertionOutcome(
		assertion,
		mustStageRevalidationDigest(t, "b"),
		[]projecttypeenvassertionreport.Ground{ground},
	)
	if err != nil {
		t.Fatalf("NewAssertionOutcome(): %v", err)
	}
	report, err := projecttypeenvassertionreport.NewReport(
		target,
		graph,
		runtime,
		digest,
		[]projecttypeenvassertionreport.AssertionOutcome{outcome},
	)
	if err != nil {
		t.Fatalf("NewReport(): %v", err)
	}
	if report.Posture() != typedmemory.RevalidationUnderdetermined ||
		report.Outcomes()[0].Grounds()[0].Code() !=
			projecttypeenvassertionreport.CodeValueMigrationRequired {
		t.Fatalf("migration report = %s %#v", report.Posture(), report.Outcomes())
	}
	issues := assertionReadinessIssues(report)
	if len(issues) != 1 ||
		issues[0].Code() != IssueAssertionRevalidationUnderdetermined {
		t.Fatalf("selection readiness issues = %#v", issues)
	}
}

func mustStageRevalidationDigest(
	t *testing.T,
	seed string,
) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(seed, 64),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}
