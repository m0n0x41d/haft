package main

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf"
	_ "modernc.org/sqlite"
)

func TestResolveSpecCommit(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "FPF-Spec.md")

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
		got := resolveSpecCommit(tt.explicitCommit, specPath)
		if got != tt.want {
			t.Fatalf("%s: resolveSpecCommit(%q) = %q, want %q", tt.name, tt.explicitCommit, got, tt.want)
		}
	}
}

func TestResolveSpecCommit_DetectsGitCommitFromSpecPath(t *testing.T) {
	repoDir := t.TempDir()
	specDir := filepath.Join(repoDir, "data", "FPF")
	specPath := filepath.Join(specDir, "FPF-Spec.md")

	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(specPath, []byte("# spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	want := strings.TrimSpace(runGit(t, repoDir, "rev-parse", "HEAD"))
	got := resolveSpecCommit("", specPath)

	if got != want {
		t.Fatalf("resolveSpecCommit() = %q, want %q", got, want)
	}
}

func TestBuildSpecIndexMetadata_LeavesCommitEmptyOutsideGit(t *testing.T) {
	buildTime := time.Date(2026, time.March, 26, 12, 34, 56, 0, time.UTC)
	specPath := filepath.Join(t.TempDir(), "FPF-Spec.md")
	metadata := buildSpecIndexMetadata(specPath, 42, "", buildTime)

	if metadata["fpf_commit"] != "" {
		t.Fatalf("expected empty fpf_commit outside git, got %q", metadata["fpf_commit"])
	}
	if metadata["indexed_sections"] != "42" {
		t.Fatalf("unexpected indexed_sections %q", metadata["indexed_sections"])
	}
}

func TestVerifyIndexRejectsSchemaVersionMismatch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fpf.db")
	expectedCommit := "expected-sha"

	writeVerifyIndexFixture(t, dbPath, verifyIndexFixture{
		commit:          expectedCommit,
		schemaVersion:   "3",
		indexedSections: 1,
		rows:            rowsForContract(shippedFPFEmbeddingContract, 1),
		routeRows:       routeRowsForContract(shippedFPFEmbeddingContract),
	})

	err := verifyIndex([]string{dbPath, expectedCommit})
	if err == nil {
		t.Fatal("expected schema mismatch error")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("expected schema_version error, got %v", err)
	}
}

func TestVerifyIndexAcceptsShippedEmbeddingContract(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fpf.db")
	expectedCommit := "expected-sha"

	writeVerifyIndexFixture(t, dbPath, verifyIndexFixture{
		commit:          expectedCommit,
		schemaVersion:   fpf.SpecIndexSchemaVersion,
		indexedSections: 2,
		rows:            rowsForContract(shippedFPFEmbeddingContract, 1, 2),
		routeRows:       routeRowsForContract(shippedFPFEmbeddingContract),
	})

	err := verifyIndex([]string{dbPath, expectedCommit})
	if err != nil {
		t.Fatalf("verifyIndex() error: %v", err)
	}
}

func TestVerifyIndexRejectsWrongEmbeddingContract(t *testing.T) {
	tests := []struct {
		name     string
		contract specEmbeddingContract
	}{
		{
			name: "provider",
			contract: specEmbeddingContract{
				provider: "openai",
				model:    shippedFPFEmbeddingContract.model,
				dim:      shippedFPFEmbeddingContract.dim,
			},
		},
		{
			name: "model",
			contract: specEmbeddingContract{
				provider: shippedFPFEmbeddingContract.provider,
				model:    "embeddinggemma-300m",
				dim:      shippedFPFEmbeddingContract.dim,
			},
		},
		{
			name: "dim",
			contract: specEmbeddingContract{
				provider: shippedFPFEmbeddingContract.provider,
				model:    shippedFPFEmbeddingContract.model,
				dim:      0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			dbPath := filepath.Join(tempDir, "fpf.db")
			expectedCommit := "expected-sha"

			writeVerifyIndexFixture(t, dbPath, verifyIndexFixture{
				commit:          expectedCommit,
				schemaVersion:   fpf.SpecIndexSchemaVersion,
				indexedSections: 2,
				rows:            rowsForContract(tt.contract, 1, 2),
				routeRows:       routeRowsForContract(shippedFPFEmbeddingContract),
			})

			err := verifyIndex([]string{dbPath, expectedCommit})
			if err == nil {
				t.Fatal("expected wrong-contract error")
			}
			if !strings.Contains(err.Error(), "vector contract mismatch") {
				t.Fatalf("expected vector contract error, got %v", err)
			}
			if !strings.Contains(err.Error(), "found 0") {
				t.Fatalf("expected zero shipped-contract vectors, got %v", err)
			}
		})
	}
}

func TestVerifyIndexRejectsPartialEmbeddingBake(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fpf.db")
	expectedCommit := "expected-sha"

	writeVerifyIndexFixture(t, dbPath, verifyIndexFixture{
		commit:          expectedCommit,
		schemaVersion:   fpf.SpecIndexSchemaVersion,
		indexedSections: 2,
		rows:            rowsForContract(shippedFPFEmbeddingContract, 1),
		routeRows:       routeRowsForContract(shippedFPFEmbeddingContract),
	})

	err := verifyIndex([]string{dbPath, expectedCommit})
	if err == nil {
		t.Fatal("expected partial-bake error")
	}
	if !strings.Contains(err.Error(), "found 1") {
		t.Fatalf("expected partial vector count error, got %v", err)
	}
}

func TestBuildIndexRejectsVectorlessBake(t *testing.T) {
	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "FPF-Spec.md")
	dbPath := filepath.Join(tempDir, "fpf.db")
	routePath := filepath.Join(tempDir, "routes.json")

	writeIndexerFixture(t, specPath, routePath)
	stubBakeSpecEmbeddings(t, 0, nil)

	err := buildIndex(specPath, dbPath, "", routePath)
	if err == nil {
		t.Fatal("expected vectorless bake to fail")
	}
	if !strings.Contains(err.Error(), "no section vectors baked") {
		t.Fatalf("expected no-vectors error, got %v", err)
	}
}

func TestVerifyIndexRejectsMissingPatternUseRouteEmbeddings(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fpf.db")
	expectedCommit := "expected-sha"

	writeVerifyIndexFixture(t, dbPath, verifyIndexFixture{
		commit:          expectedCommit,
		schemaVersion:   fpf.SpecIndexSchemaVersion,
		indexedSections: 1,
		rows:            rowsForContract(shippedFPFEmbeddingContract, 1),
	})

	err := verifyIndex([]string{dbPath, expectedCommit})
	if err == nil {
		t.Fatal("expected missing PatternUse route vectors error")
	}
	if !strings.Contains(err.Error(), "PatternUse route vector contract mismatch") {
		t.Fatalf("expected PatternUse route vector error, got %v", err)
	}
}

func TestVerifyIndexRejectsStalePatternUseRouteEmbeddingHash(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fpf.db")
	expectedCommit := "expected-sha"

	routeRows := routeRowsForContract(shippedFPFEmbeddingContract)
	routeRows[0].contentHash = "stale"
	writeVerifyIndexFixture(t, dbPath, verifyIndexFixture{
		commit:          expectedCommit,
		schemaVersion:   fpf.SpecIndexSchemaVersion,
		indexedSections: 1,
		rows:            rowsForContract(shippedFPFEmbeddingContract, 1),
		routeRows:       routeRows,
	})

	err := verifyIndex([]string{dbPath, expectedCommit})
	if err == nil {
		t.Fatal("expected stale PatternUse route vector hash error")
	}
	if !strings.Contains(err.Error(), "PatternUse route vectors are STALE") {
		t.Fatalf("expected stale PatternUse route hash error, got %v", err)
	}
}

func TestVerifyIndexRejectsMissingPatternAtlasRows(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fpf.db")
	expectedCommit := "expected-sha"

	writeVerifyIndexFixture(t, dbPath, verifyIndexFixture{
		commit:          expectedCommit,
		schemaVersion:   fpf.SpecIndexSchemaVersion,
		indexedSections: 1,
		rows:            rowsForContract(shippedFPFEmbeddingContract, 1),
		routeRows:       routeRowsForContract(shippedFPFEmbeddingContract),
		omitAtlasRows:   true,
	})

	err := verifyIndex([]string{dbPath, expectedCommit})
	if err == nil {
		t.Fatal("expected missing PatternAtlas rows error")
	}
	if !strings.Contains(err.Error(), "PatternAtlas contract mismatch") {
		t.Fatalf("expected PatternAtlas contract error, got %v", err)
	}
}

func TestVerifyIndexRejectsStalePatternAtlasHash(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fpf.db")
	expectedCommit := "expected-sha"

	writeVerifyIndexFixture(t, dbPath, verifyIndexFixture{
		commit:          expectedCommit,
		schemaVersion:   fpf.SpecIndexSchemaVersion,
		indexedSections: 1,
		rows:            rowsForContract(shippedFPFEmbeddingContract, 1),
		routeRows:       routeRowsForContract(shippedFPFEmbeddingContract),
	})

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`UPDATE pattern_atlas_cards SET content_hash='stale' WHERE pattern_id='F.18'`); err != nil {
		t.Fatalf("stale atlas card hash: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	err = verifyIndex([]string{dbPath, expectedCommit})
	if err == nil {
		t.Fatal("expected stale PatternAtlas hash error")
	}
	if !strings.Contains(err.Error(), "PatternAtlas hash integrity failed") {
		t.Fatalf("expected PatternAtlas hash error, got %v", err)
	}
}

func TestCleanSpecCommitRef(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "empty defaults to HEAD", in: "", want: "HEAD", ok: true},
		{name: "lowercase sha", in: "0123456789abcdef0123456789abcdef01234567", want: "0123456789abcdef0123456789abcdef01234567", ok: true},
		{name: "uppercase sha normalizes", in: "ABCDEF0123456789ABCDEF0123456789ABCDEF01", want: "abcdef0123456789abcdef0123456789abcdef01", ok: true},
		{name: "option injection rejected", in: "--format=%H", ok: false},
		{name: "short ref rejected", in: "abc123", ok: false},
		{name: "pathspec rejected", in: "HEAD:cmd/indexer/main.go", ok: false},
	}

	for _, tt := range tests {
		got, ok := cleanSpecCommitRef(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("%s: cleanSpecCommitRef(%q) = %q, %v; want %q, %v", tt.name, tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}

	return string(output)
}

func TestBuildIndex_PreservesHeadingOnlyRootPatternShells(t *testing.T) {
	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "FPF-Spec.md")
	dbPath := filepath.Join(tempDir, "fpf.db")
	routePath := filepath.Join(tempDir, "routes.json")

	writeIndexerFixture(t, specPath, routePath)
	stubBakeSpecEmbeddings(t, 1, nil)
	stubBakePatternUseRouteEmbeddings(t, 1, nil)

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

func writeIndexerFixture(t *testing.T, specPath, routePath string) {
	t.Helper()

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
}

func stubBakeSpecEmbeddings(t *testing.T, baked int, err error) {
	t.Helper()

	original := bakeSpecEmbeddingsFunc
	bakeSpecEmbeddingsFunc = func(string) (int, error) {
		return baked, err
	}
	t.Cleanup(func() {
		bakeSpecEmbeddingsFunc = original
	})
}

func stubBakePatternUseRouteEmbeddings(t *testing.T, baked int, err error) {
	t.Helper()

	original := bakePatternUseRouteEmbeddingsFunc
	bakePatternUseRouteEmbeddingsFunc = func(string) (int, error) {
		return baked, err
	}
	t.Cleanup(func() {
		bakePatternUseRouteEmbeddingsFunc = original
	})
}

type verifyIndexFixture struct {
	commit          string
	schemaVersion   string
	indexedSections int
	rows            []verifyEmbeddingRow
	routeRows       []verifyRouteEmbeddingRow
	intentRows      []verifyIntentEmbeddingRow
	omitAtlasRows   bool
}

type verifyEmbeddingRow struct {
	sectionID int
	contract  specEmbeddingContract
}

type verifyRouteEmbeddingRow struct {
	routeID      string
	documentID   string
	documentKind string
	contentHash  string
	contract     specEmbeddingContract
}

type verifyIntentEmbeddingRow struct {
	laneID       fpf.PatternUseIntentLane
	documentID   string
	documentKind string
	contentHash  string
	contract     specEmbeddingContract
}

func rowsForContract(contract specEmbeddingContract, sectionIDs ...int) []verifyEmbeddingRow {
	rows := make([]verifyEmbeddingRow, 0, len(sectionIDs))
	for _, sectionID := range sectionIDs {
		rows = append(rows, verifyEmbeddingRow{sectionID: sectionID, contract: contract})
	}
	return rows
}

func routeRowsForContract(contract specEmbeddingContract) []verifyRouteEmbeddingRow {
	documents := fpf.PatternUseRouteEmbeddingDocuments(fpf.DefaultPatternUseRouteCards())
	rows := make([]verifyRouteEmbeddingRow, 0, len(documents))
	for _, document := range documents {
		rows = append(rows, verifyRouteEmbeddingRow{
			routeID:      document.RouteID,
			documentID:   document.DocumentID,
			documentKind: document.DocumentKind,
			contentHash:  document.ContentHash,
			contract:     contract,
		})
	}
	return rows
}

func intentRowsForContract(contract specEmbeddingContract) []verifyIntentEmbeddingRow {
	documents := fpf.PatternUseIntentEmbeddingDocuments(fpf.DefaultPatternUseIntentLaneCards())
	rows := make([]verifyIntentEmbeddingRow, 0, len(documents))
	for _, document := range documents {
		rows = append(rows, verifyIntentEmbeddingRow{
			laneID:       document.LaneID,
			documentID:   document.DocumentID,
			documentKind: document.DocumentKind,
			contentHash:  document.ContentHash,
			contract:     contract,
		})
	}
	return rows
}

func writeVerifyIndexFixture(t *testing.T, dbPath string, fixture verifyIndexFixture) {
	t.Helper()
	if fixture.schemaVersion == fpf.SpecIndexSchemaVersion && fixture.intentRows == nil {
		fixture.intentRows = intentRowsForContract(shippedFPFEmbeddingContract)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT)`,
		`CREATE TABLE fpf_embeddings (
			section_id INTEGER NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			dim INTEGER NOT NULL,
			content_hash TEXT NOT NULL,
			vector BLOB NOT NULL,
			PRIMARY KEY (section_id, provider, model, dim)
		)`,
		`CREATE TABLE pattern_use_route_embeddings (
			route_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			document_kind TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			dim INTEGER NOT NULL,
			content_hash TEXT NOT NULL,
			vector BLOB NOT NULL,
			PRIMARY KEY (route_id, document_id, provider, model, dim)
		)`,
		`CREATE TABLE pattern_use_intent_embeddings (
			lane_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			document_kind TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			dim INTEGER NOT NULL,
			content_hash TEXT NOT NULL,
			vector BLOB NOT NULL,
			PRIMARY KEY (lane_id, document_id, provider, model, dim)
		)`,
		`CREATE TABLE pattern_atlas_nodes (
			node_id TEXT PRIMARY KEY,
			pattern_id TEXT,
			heading TEXT NOT NULL,
			level INTEGER NOT NULL,
			start_line INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			own_end_line INTEGER NOT NULL,
			parent_node_id TEXT,
			path TEXT NOT NULL,
			body TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			source_ref TEXT NOT NULL,
			fpf_commit TEXT NOT NULL
		)`,
		`CREATE TABLE pattern_atlas_cards (
			pattern_id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			card_start_line INTEGER NOT NULL,
			card_end_line INTEGER NOT NULL,
			root_node_id TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			source_ref TEXT NOT NULL,
			fpf_commit TEXT NOT NULL
		)`,
		`CREATE TABLE pattern_atlas_lints (
			line_number INTEGER NOT NULL,
			lint_kind TEXT NOT NULL,
			message TEXT NOT NULL,
			raw_line TEXT NOT NULL,
			source_ref TEXT NOT NULL,
			fpf_commit TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	metadata := map[string]string{
		"fpf_commit":       fixture.commit,
		"schema_version":   fixture.schemaVersion,
		"indexed_sections": strconv.Itoa(fixture.indexedSections),
	}
	for key, value := range metadata {
		_, err := db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)`, key, value)
		if err != nil {
			t.Fatalf("insert meta %s: %v", key, err)
		}
	}

	for _, row := range fixture.rows {
		_, err := db.Exec(
			`INSERT INTO fpf_embeddings (section_id, provider, model, dim, content_hash, vector) VALUES (?, ?, ?, ?, ?, ?)`,
			row.sectionID,
			row.contract.provider,
			row.contract.model,
			row.contract.dim,
			"hash",
			[]byte{0, 1, 2, 3},
		)
		if err != nil {
			t.Fatalf("insert embedding row %+v: %v", row, err)
		}
	}

	for _, row := range fixture.routeRows {
		_, err := db.Exec(
			`INSERT INTO pattern_use_route_embeddings (route_id, document_id, document_kind, provider, model, dim, content_hash, vector) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			row.routeID,
			row.documentID,
			row.documentKind,
			row.contract.provider,
			row.contract.model,
			row.contract.dim,
			row.contentHash,
			[]byte{0, 1, 2, 3},
		)
		if err != nil {
			t.Fatalf("insert route embedding row %+v: %v", row, err)
		}
	}

	for _, row := range fixture.intentRows {
		_, err := db.Exec(
			`INSERT INTO pattern_use_intent_embeddings (lane_id, document_id, document_kind, provider, model, dim, content_hash, vector) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			row.laneID,
			row.documentID,
			row.documentKind,
			row.contract.provider,
			row.contract.model,
			row.contract.dim,
			row.contentHash,
			[]byte{0, 1, 2, 3},
		)
		if err != nil {
			t.Fatalf("insert intent embedding row %+v: %v", row, err)
		}
	}

	if !fixture.omitAtlasRows {
		atlas, err := fpf.BuildPatternAtlas([]byte(verifyIndexAtlasMarkdown()), "fixture.md", fixture.commit)
		if err != nil {
			t.Fatalf("build atlas fixture: %v", err)
		}
		if err := fpf.StorePatternAtlasDB(db, atlas); err != nil {
			t.Fatalf("store atlas fixture: %v", err)
		}
	}
}

func verifyIndexAtlasMarkdown() string {
	return `# Fixture

## F.18 - Local-First Unification Naming Protocol
Intro.

### F.18:1 - Context
NameCard:

## C.30 - Grounded Architecture and Selected-Structure Adequacy
Intro.

### C.30:1 - Problem frame
ArchitectureQuestionCard:

## A.10 - Evidence Graph Referring: Claim-Bound Evidence and Provenance Graph
Intro.

### A.10:1 - Evidence relation
EvidenceRelation:

## A.7 - Strict Distinction (Clarity Lattice)
Intro.

### A.7:1 - Strict table
ObjectDescriptionCarrierEvidence:

## B.3 - Evidence Congruence and Decay
Intro.

### B.3:1 - Congruence
CongruenceLevel:
`
}
