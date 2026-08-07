package codebase

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

// BenchmarkComputeCoverage_RealProject locks the upper bound declared in
// dec-20260527-e4b86938 (V1 in-place coverage ranking, claim-003): the
// coverage section must add <100ms on the haft project at current size.
// Skipped unless HAFT_COVERAGE_BENCH_DB points at a real project database
// (e.g. ~/.haft/projects/<id>/haft.db) — file mode 0444 access is read-only.
func BenchmarkComputeCoverage_RealProject(b *testing.B) {
	dbPath := os.Getenv("HAFT_COVERAGE_BENCH_DB")
	if dbPath == "" {
		b.Skip("HAFT_COVERAGE_BENCH_DB not set — skipping real-project coverage benchmark")
	}

	conn, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		b.Fatalf("open %s: %v", dbPath, err)
	}
	defer conn.Close()

	ctx := context.Background()
	report, err := ComputeCoverage(ctx, conn)
	if err != nil {
		b.Fatalf("ComputeCoverage: %v", err)
	}
	if report.TotalModules == 0 {
		b.Fatal("benchmark DB has no modules — wrong database?")
	}
	b.Logf("modules: %d", report.TotalModules)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		report, err := ComputeCoverage(ctx, conn)
		if err != nil {
			b.Fatalf("ComputeCoverage: %v", err)
		}
		_ = FormatCoverageResponse(report)
	}
}
