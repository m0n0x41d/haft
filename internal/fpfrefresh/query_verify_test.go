package fpfrefresh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
)

func TestVerifyCandidateQueryContractAgainstCurrentProductionSource(t *testing.T) {
	lockPath := filepath.Join(
		"..",
		"..",
		"data",
		"haft",
		"fpf-integration.lock.json",
	)
	lockBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read generated integration lock: %v", err)
	}
	lock, err := ParseIntegrationLock(lockBytes)
	if err != nil {
		t.Fatalf("ParseIntegrationLock() error: %v", err)
	}
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	units, err := fpf.LoadSourceUnits(
		readmePath,
		specPath,
		lock.Coordinates.SourceRevision,
	)
	if err != nil {
		t.Fatalf("LoadSourceUnits() error: %v", err)
	}
	databasePath := filepath.Join(t.TempDir(), "candidate.db")
	if err := fpf.StoreSourceUnits(databasePath, units); err != nil {
		t.Fatalf("StoreSourceUnits() error: %v", err)
	}

	results, err := VerifyCandidateQueryContract(databasePath)
	if err != nil {
		t.Fatalf("VerifyCandidateQueryContract() error: %v", err)
	}
	if len(results) != 12 {
		t.Fatalf("query smoke count = %d, want 12: %#v", len(results), results)
	}
	for _, result := range results {
		if result.CaseID == "" || result.ResultKind == "" {
			t.Fatalf("query smoke lost identity or result kind: %#v", result)
		}
	}
}
