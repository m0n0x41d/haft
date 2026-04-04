package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestResolveSpecCommit_UsesOnlyExplicitCommit(t *testing.T) {
	tests := []struct {
		name           string
		explicitCommit string
		want           string
	}{
		{
			name:           "empty",
			explicitCommit: "",
			want:           "",
		},
		{
			name:           "trimmed",
			explicitCommit: "  abc123  ",
			want:           "abc123",
		},
	}

	for _, tt := range tests {
		got := resolveSpecCommit(tt.explicitCommit)
		if got != tt.want {
			t.Fatalf("%s: resolveSpecCommit(%q) = %q, want %q", tt.name, tt.explicitCommit, got, tt.want)
		}
	}
}

func TestBuildSpecIndexMetadata_DoesNotInventCommitProvenance(t *testing.T) {
	buildTime := time.Date(2026, time.March, 26, 12, 34, 56, 0, time.UTC)
	metadata := buildSpecIndexMetadata("data/FPF/FPF-Spec.md", 42, "", buildTime)

	if metadata["fpf_commit"] != "" {
		t.Fatalf("expected empty fpf_commit without explicit upstream revision, got %q", metadata["fpf_commit"])
	}
	if metadata["indexed_sections"] != "42" {
		t.Fatalf("unexpected indexed_sections %q", metadata["indexed_sections"])
	}
}

func TestBuildIndex_PreservesHeadingOnlyRootPatternShells(t *testing.T) {
	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "FPF-Spec.md")
	dbPath := filepath.Join(tempDir, "fpf.db")
	routePath := filepath.Join(tempDir, "routes.json")

	spec := `## A.17 - Canonical “Characteristic” (A.CHR-NORM)

### A.17:1 - Context

To have reproducibility and explainability there is a need to measure various aspects of systems or knowledge artifacts.
`
	routes := `{"routes":[]}`

	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := os.WriteFile(routePath, []byte(routes), 0o644); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	if err := buildIndex(specPath, dbPath, "", routePath); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow(`SELECT count(*) FROM sections WHERE pattern_id = ?`, "A.17").Scan(&count)
	if err != nil {
		t.Fatalf("count A.17: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected A.17 root shell in built index, got count %d", count)
	}

	var aliasesJSON string
	err = db.QueryRow(`SELECT aliases_json FROM sections WHERE pattern_id = ?`, "A.17").Scan(&aliasesJSON)
	if err != nil {
		t.Fatalf("read aliases_json: %v", err)
	}
	if !strings.Contains(aliasesJSON, "A.CHR-NORM") {
		t.Fatalf("expected technical alias in aliases_json, got %q", aliasesJSON)
	}
}
