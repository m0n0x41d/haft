package contextgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"

	_ "modernc.org/sqlite"
)

func TestFetchGoverningSpecSections_TypedLifecycleBaselineAndDedup(t *testing.T) {
	database := openSpecContextDB(t, true, true)
	ctx := context.Background()
	now := time.Date(2026, time.July, 11, 12, 0, 0, 0, time.UTC)

	current := specContextTestSection("TS.current.001", "Current contract", "active", "2026-12-31")
	current.Claims = []project.SpecClaim{{
		ID:         "TS.current.claim",
		Class:      "requirement",
		Statement:  "Current behavior remains observable.",
		ValidUntil: "2026-12-31",
	}}
	stale := specContextTestSection("TS.stale.001", "Expired contract", "active", "2026-01-01")
	insertSpecEdition(t, database, "project-a", current, "")
	insertSpecEdition(t, database, "project-a", stale, "")
	insertSpecBaseline(t, database, "project-a", current.ID, specflow.HashSection(current))
	insertSpecBaseline(t, database, "project-a", stale.ID, "outdated-baseline")

	decisions := []*artifact.Artifact{
		specContextTestDecision(t, "dec-one", current.ID, stale.ID, "TS.missing.001"),
		specContextTestDecision(t, "dec-two", current.ID),
	}
	sections := fetchGoverningSpecSections(ctx, database, decisions, now)
	if len(sections) != 3 {
		t.Fatalf("sections = %d, want 3: %+v", len(sections), sections)
	}

	currentContext := sections[0]
	if currentContext.ID != current.ID || currentContext.Resolution != SpecResolutionResolved {
		t.Fatalf("current resolution = %+v", currentContext)
	}
	if currentContext.LifecycleState != project.SpecSectionStateActive || currentContext.BaselineState != SpecBaselineCurrent {
		t.Fatalf("current lifecycle/baseline = %s/%s", currentContext.LifecycleState, currentContext.BaselineState)
	}
	if len(currentContext.DecisionRefs) != 2 || currentContext.DecisionRefs[0] != "dec-one" || currentContext.DecisionRefs[1] != "dec-two" {
		t.Fatalf("current decision refs = %#v, want stable deduped refs", currentContext.DecisionRefs)
	}
	if len(currentContext.Claims) != 1 || currentContext.Claims[0].ID != "TS.current.claim" {
		t.Fatalf("current claims = %+v", currentContext.Claims)
	}

	staleContext := sections[1]
	if staleContext.LifecycleState != project.SpecSectionStateStale || staleContext.BaselineState != SpecBaselineDrifted {
		t.Fatalf("stale lifecycle/baseline = %s/%s", staleContext.LifecycleState, staleContext.BaselineState)
	}

	missingContext := sections[2]
	if missingContext.Resolution != SpecResolutionMissing || missingContext.BaselineState != SpecBaselineMissing {
		t.Fatalf("missing resolution/baseline = %s/%s", missingContext.Resolution, missingContext.BaselineState)
	}
}

func TestFetchGoverningSpecSections_DegradesPerSection(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.July, 11, 12, 0, 0, 0, time.UTC)

	t.Run("corrupt hash and ambiguous project stay explicit", func(t *testing.T) {
		database := openSpecContextDB(t, true, true)
		corrupt := specContextTestSection("TS.corrupt.001", "Corrupt", "active", "2026-12-31")
		ambiguous := specContextTestSection("TS.ambiguous.001", "Ambiguous", "active", "2026-12-31")
		insertSpecEdition(t, database, "project-a", corrupt, "wrong-hash")
		insertSpecEdition(t, database, "project-a", ambiguous, "")
		insertSpecEdition(t, database, "project-b", ambiguous, "")

		decision := specContextTestDecision(t, "dec-degraded", corrupt.ID, ambiguous.ID)
		sections := fetchGoverningSpecSections(ctx, database, []*artifact.Artifact{decision}, now)
		if sections[0].Resolution != SpecResolutionCorrupt || !strings.Contains(sections[0].ResolutionDetail, "stored semantic hash") {
			t.Fatalf("corrupt section = %+v", sections[0])
		}
		if sections[1].Resolution != SpecResolutionAmbiguous || !strings.Contains(sections[1].ResolutionDetail, "2 current editions") {
			t.Fatalf("ambiguous section = %+v", sections[1])
		}
	})

	t.Run("missing baseline table preserves resolved edition", func(t *testing.T) {
		database := openSpecContextDB(t, true, false)
		section := specContextTestSection("TS.legacy.001", "Legacy", "draft", "")
		insertSpecEdition(t, database, "project-a", section, "")

		decision := specContextTestDecision(t, "dec-legacy", section.ID)
		sections := fetchGoverningSpecSections(ctx, database, []*artifact.Artifact{decision}, now)
		if sections[0].Resolution != SpecResolutionResolved || sections[0].BaselineState != SpecBaselineUnavailable {
			t.Fatalf("legacy section = %+v", sections[0])
		}
		if sections[0].LifecycleState != project.SpecSectionStateDraft || !strings.Contains(sections[0].BaselineDetail, "spec_section_baselines") {
			t.Fatalf("legacy lifecycle/detail = %+v", sections[0])
		}
	})

	t.Run("missing edition table is unavailable not empty", func(t *testing.T) {
		database := openSpecContextDB(t, false, false)
		decision := specContextTestDecision(t, "dec-unavailable", "TS.unavailable.001")
		sections := fetchGoverningSpecSections(ctx, database, []*artifact.Artifact{decision}, now)
		if sections[0].Resolution != SpecResolutionUnavailable || sections[0].BaselineState != SpecBaselineUnavailable {
			t.Fatalf("unavailable section = %+v", sections[0])
		}
	})
}

func TestFetchCodeContext_PopulatesTypedSpecsFromGoverningDecisions(t *testing.T) {
	store, graphStore, database := setupContextDB(t)
	createSpecContextTables(t, database, true, true)
	ctx := context.Background()
	section := specContextTestSection("TS.behavior.001", "Typed behavior", "active", "2026-12-31")
	insertSpecEdition(t, database, "project-a", section, "")
	insertSpecBaseline(t, database, "project-a", section.ID, specflow.HashSection(section))

	decision := specContextTestDecision(t, "dec-typed-spec", section.ID)
	fields := decision.UnmarshalDecisionFields()
	fields.BindingTargets = []artifact.BindingTarget{{
		Kind:     artifact.BindingTargetWholeFileFallback,
		FilePath: "internal/typed/spec.go",
		Reason:   "explicit test fixture",
	}}
	structured, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	decision.StructuredData = string(structured)
	decision.Meta.CreatedAt = time.Now().UTC()
	decision.Meta.UpdatedAt = decision.Meta.CreatedAt
	decision.Body = "typed spec decision"
	if err := store.Create(ctx, decision); err != nil {
		t.Fatal(err)
	}
	file := "internal/typed/spec.go"
	if _, err := database.ExecContext(ctx, `INSERT INTO affected_files (artifact_id, file_path) VALUES (?, ?)`, decision.Meta.ID, file); err != nil {
		t.Fatal(err)
	}

	codeContext, err := FetchCodeContext(ctx, store, graphStore, Target{File: file})
	if err != nil {
		t.Fatal(err)
	}
	if len(codeContext.Specs) != 1 {
		t.Fatalf("typed specs = %+v, want one", codeContext.Specs)
	}
	if codeContext.Specs[0].ID != section.ID || codeContext.Specs[0].BaselineState != SpecBaselineCurrent {
		t.Fatalf("typed spec context = %+v", codeContext.Specs[0])
	}
}

func openSpecContextDB(t *testing.T, editions, baselines bool) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", t.TempDir()+"/spec-context.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	createSpecContextTables(t, database, editions, baselines)
	return database
}

func createSpecContextTables(t *testing.T, database *sql.DB, editions, baselines bool) {
	t.Helper()
	if editions {
		_, err := database.Exec(`CREATE TABLE spec_section_editions (
			project_id TEXT NOT NULL,
			section_id TEXT NOT NULL,
			semantic_hash TEXT NOT NULL,
			section_json TEXT NOT NULL,
			source_kind TEXT NOT NULL DEFAULT '',
			carrier_path TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (project_id, section_id)
		)`)
		if err != nil {
			t.Fatal(err)
		}
	}
	if baselines {
		_, err := database.Exec(`CREATE TABLE spec_section_baselines (
			project_id TEXT NOT NULL,
			section_id TEXT NOT NULL,
			hash TEXT NOT NULL,
			captured_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			approved_by TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (project_id, section_id)
		)`)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func specContextTestSection(id, title, status, validUntil string) project.SpecSection {
	return project.SpecSection{
		ID:            id,
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         title,
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "human",
		Status:        status,
		ValidUntil:    validUntil,
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
}

func specContextTestDecision(t *testing.T, id string, sectionRefs ...string) *artifact.Artifact {
	t.Helper()
	structured, err := json.Marshal(artifact.DecisionFields{SectionRefs: sectionRefs})
	if err != nil {
		t.Fatal(err)
	}
	return &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     id,
			Kind:   artifact.KindDecisionRecord,
			Status: artifact.StatusActive,
			Title:  id,
		},
		StructuredData: string(structured),
	}
}

func insertSpecEdition(t *testing.T, database *sql.DB, projectID string, section project.SpecSection, storedHash string) {
	t.Helper()
	sectionJSON, err := json.Marshal(section)
	if err != nil {
		t.Fatal(err)
	}
	if storedHash == "" {
		storedHash = specflow.HashSection(section)
	}
	_, err = database.Exec(
		`INSERT INTO spec_section_editions
		 (project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		projectID,
		section.ID,
		storedHash,
		string(sectionJSON),
		"carrier_import",
		section.Path,
		time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func insertSpecBaseline(t *testing.T, database *sql.DB, projectID, sectionID, hash string) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO spec_section_baselines (project_id, section_id, hash, captured_at, approved_by)
		 VALUES (?, ?, ?, ?, ?)`,
		projectID,
		sectionID,
		hash,
		time.Date(2026, time.July, 10, 13, 0, 0, 0, time.UTC),
		"operator",
	)
	if err != nil {
		t.Fatal(err)
	}
}
