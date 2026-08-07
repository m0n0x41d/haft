package codebase

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type tsParityManifest struct {
	SchemaVersion               int                 `json:"schema_version"`
	FixtureKind                 string              `json:"fixture_kind"`
	SourceBoundary              string              `json:"source_boundary"`
	RawCountsRole               string              `json:"raw_counts_role"`
	RequiredSeeds               []tsParitySymbolRef `json:"required_seeds"`
	RequiredRelations           []tsParityRelation  `json:"required_relations"`
	KnownMissingRelations       []tsParityRelation  `json:"known_missing_relations"`
	KnownFalsePositiveRelations []tsParityRelation  `json:"known_false_positive_relations"`
	ForbiddenRelations          []tsParityRelation  `json:"forbidden_relations"`
}

type tsParitySymbolRef struct {
	File     string `json:"file"`
	Name     string `json:"name"`
	Receiver string `json:"receiver,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

type tsParityRelation struct {
	ID     string            `json:"id"`
	Kind   EdgeKind          `json:"kind"`
	Source tsParitySymbolRef `json:"source"`
	Target tsParitySymbolRef `json:"target"`
}

type tsParityMetrics struct {
	Symbols                     int     `json:"symbols"`
	FilesWithSymbols            int     `json:"files_with_symbols"`
	Edges                       int     `json:"edges"`
	RequiredSeeds               int     `json:"required_seeds"`
	RequiredRelations           int     `json:"required_relations"`
	RequiredRelationsResolved   int     `json:"required_relations_resolved"`
	KnownMissingRelations       int     `json:"known_missing_relations"`
	KnownFalsePositiveRelations int     `json:"known_false_positive_relations"`
	ForbiddenRelationsObserved  int     `json:"forbidden_relations_observed"`
	RequiredRecall              float64 `json:"required_recall"`
	LabeledPrecision            float64 `json:"labeled_precision"`
}

type codeGraphCandidate struct {
	id        string
	qualified string
}

type codeGraphQualificationCarrier struct {
	Schema          string                            `json:"schema"`
	Posture         string                            `json:"posture"`
	ResultSemantics string                            `json:"result_semantics"`
	Source          codeGraphQualificationSource      `json:"source"`
	FixtureDigest   string                            `json:"fixture_digest"`
	ManifestDigest  string                            `json:"manifest_digest"`
	Observed        codeGraphQualificationObservation `json:"observed"`
	DeclaredFixture codeGraphQualificationObservation `json:"declared_fixture"`
}

type codeGraphQualificationSource struct {
	Package  string `json:"package"`
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

type codeGraphQualificationObservation struct {
	RequiredSeeds      int `json:"required_seeds"`
	RequiredRelations  int `json:"required_relations"`
	ForbiddenRelations int `json:"forbidden_relations"`
}

func TestTypeScriptParityManifest(t *testing.T) {
	manifest := loadTypeScriptParityManifest(t)
	if manifest.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", manifest.SchemaVersion)
	}
	if manifest.FixtureKind != "synthetic_typescript_parity_corpus" {
		t.Fatalf("fixture_kind = %q", manifest.FixtureKind)
	}
	if manifest.SourceBoundary != "synthetic_fixture_no_private_project_source" {
		t.Fatalf("source_boundary = %q", manifest.SourceBoundary)
	}
	if manifest.RawCountsRole != "observation_not_target" {
		t.Fatalf("raw_counts_role = %q", manifest.RawCountsRole)
	}
	if len(manifest.RequiredSeeds) != 12 {
		t.Fatalf("required seeds = %d, want 12", len(manifest.RequiredSeeds))
	}

	root := typeScriptParityRoot()
	symbols, edges := buildTypeScriptParityGraph(t, root)
	allEdges, err := edges.AllEdges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range manifest.RequiredSeeds {
		mustResolveParitySymbol(t, symbols, seed)
	}
	validateParityRelationEndpoints(t, symbols, manifest)

	metrics := measureTypeScriptParity(t, symbols, allEdges, manifest)
	if metrics.RequiredRelationsResolved != metrics.RequiredRelations {
		t.Fatalf("required relation recall = %.3f, metrics=%+v", metrics.RequiredRecall, metrics)
	}
	if metrics.ForbiddenRelationsObserved != 0 {
		t.Fatalf("forbidden relation(s) observed: metrics=%+v", metrics)
	}

	encoded, err := json.Marshal(metrics)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("typescript parity metrics: %s", encoded)
}

func TestCodeGraphTypeScriptParityQualificationCarrierIsCurrent(t *testing.T) {
	carrier := loadCodeGraphQualificationCarrier(t)
	manifest := loadTypeScriptParityManifest(t)
	manifestPath := filepath.Join(typeScriptParityRoot(), "manifest.json")
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifestRaw)
	expectedManifestDigest := fmt.Sprintf("sha256:%x", manifestDigest)
	if carrier.Schema != "haft.codegraph-reference-qualification/v1" ||
		carrier.Posture != "frozen_external_observation_not_product_dependency" ||
		carrier.ResultSemantics == "" {
		t.Fatal("CodeGraph qualification carrier semantics are incomplete")
	}
	if carrier.Source.Package != "@colbymchenry/codegraph" ||
		carrier.Source.Version != "1.3.1" ||
		carrier.Source.Revision != "e76a355df5a3489c4cd6ee26cb8bd967893b4348" {
		t.Fatalf("CodeGraph qualification source = %#v", carrier.Source)
	}
	if carrier.ManifestDigest != expectedManifestDigest {
		t.Fatalf(
			"CodeGraph qualification manifest digest = %s, want %s",
			carrier.ManifestDigest,
			expectedManifestDigest,
		)
	}
	fixtureDigest := typeScriptParityFixtureDigest(t)
	if carrier.FixtureDigest != fixtureDigest {
		t.Fatalf(
			"CodeGraph qualification fixture digest = %s, want %s",
			carrier.FixtureDigest,
			fixtureDigest,
		)
	}
	declared := codeGraphQualificationObservation{
		RequiredSeeds:      len(manifest.RequiredSeeds),
		RequiredRelations:  len(manifest.RequiredRelations),
		ForbiddenRelations: len(manifest.ForbiddenRelations),
	}
	if carrier.DeclaredFixture != declared {
		t.Fatalf("CodeGraph declared fixture = %#v, want %#v", carrier.DeclaredFixture, declared)
	}
	expectedObservation := codeGraphQualificationObservation{
		RequiredSeeds:      12,
		RequiredRelations:  24,
		ForbiddenRelations: 1,
	}
	if carrier.Observed != expectedObservation {
		t.Fatalf("CodeGraph frozen observation = %#v", carrier.Observed)
	}
}

func TestCodeGraphTypeScriptParityManifest(t *testing.T) {
	databasePath := os.Getenv("CODEGRAPH_TS_PARITY_DB")
	if databasePath == "" {
		t.Skip("set CODEGRAPH_TS_PARITY_DB to evaluate the pinned CodeGraph reference")
	}
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	manifest := loadTypeScriptParityManifest(t)
	carrier := loadCodeGraphQualificationCarrier(t)
	seedCount := 0
	for _, seed := range manifest.RequiredSeeds {
		if len(codeGraphNodeIDs(t, database, seed)) > 0 {
			seedCount++
		}
	}
	requiredCount := 0
	for _, relation := range manifest.RequiredRelations {
		if codeGraphHasRelation(t, database, relation) {
			requiredCount++
		}
	}
	forbiddenCount := 0
	for _, relation := range manifest.ForbiddenRelations {
		if codeGraphHasRelation(t, database, relation) {
			forbiddenCount++
		}
	}
	t.Logf(
		"CodeGraph parity seeds=%d/%d required=%d/%d forbidden=%d/%d",
		seedCount,
		len(manifest.RequiredSeeds),
		requiredCount,
		len(manifest.RequiredRelations),
		forbiddenCount,
		len(manifest.ForbiddenRelations),
	)
	observed := codeGraphQualificationObservation{
		RequiredSeeds:      seedCount,
		RequiredRelations:  requiredCount,
		ForbiddenRelations: forbiddenCount,
	}
	if observed != carrier.Observed {
		t.Fatalf("unexpected pinned CodeGraph 1.3.1 parity result")
	}
}

func codeGraphNodeIDs(t *testing.T, database *sql.DB, ref tsParitySymbolRef) []string {
	t.Helper()
	rows, err := database.Query(`
		SELECT id, qualified_name
		FROM nodes
		WHERE file_path = ? AND name = ?
		ORDER BY start_line`, ref.File, ref.Name)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	candidates := make([]codeGraphCandidate, 0)
	for rows.Next() {
		var item codeGraphCandidate
		if err := rows.Scan(&item.id, &item.qualified); err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if ref.Receiver == "" || len(candidates) <= 1 {
		return codeGraphCandidateIDs(candidates)
	}
	qualified := make([]codeGraphCandidate, 0)
	for _, item := range candidates {
		prefixes := []string{ref.Receiver + "::", ref.Receiver + "."}
		if strings.HasPrefix(item.qualified, prefixes[0]) || strings.HasPrefix(item.qualified, prefixes[1]) {
			qualified = append(qualified, item)
		}
	}
	return codeGraphCandidateIDs(qualified)
}

func codeGraphCandidateIDs(candidates []codeGraphCandidate) []string {
	ids := make([]string, 0, len(candidates))
	for _, item := range candidates {
		ids = append(ids, item.id)
	}
	return ids
}

func codeGraphHasRelation(t *testing.T, database *sql.DB, relation tsParityRelation) bool {
	t.Helper()
	sourceIDs := codeGraphNodeIDs(t, database, relation.Source)
	targetIDs := codeGraphNodeIDs(t, database, relation.Target)
	kind := codeGraphRelationKind(relation.Kind)
	for _, sourceID := range sourceIDs {
		for _, targetID := range targetIDs {
			var count int
			err := database.QueryRow(`
				SELECT COUNT(*) FROM edges
				WHERE source = ? AND target = ? AND kind = ?`, sourceID, targetID, kind).Scan(&count)
			if err != nil {
				t.Fatal(err)
			}
			if count > 0 {
				return true
			}
		}
	}
	return false
}

func codeGraphRelationKind(kind EdgeKind) string {
	if kind == EdgeCall {
		return "calls"
	}
	if kind == EdgeTemplateUse {
		return "calls"
	}
	return "references"
}

func TestTypeScriptTargetIndexMetrics(t *testing.T) {
	root := os.Getenv("HAFT_TS_TARGET_ROOT")
	if root == "" {
		t.Skip("set HAFT_TS_TARGET_ROOT to run the optional real-project TypeScript index")
	}
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "target.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	scanner := NewScanner(database)
	startedAt := time.Now()
	result, err := scanner.RefreshIncremental(context.Background(), root)
	indexDuration := time.Since(startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published || result.Degraded {
		t.Fatalf("target refresh = %+v", result)
	}
	metrics := map[string]int{}
	queries := map[string]string{
		"indexed_files":  `SELECT COUNT(*) FROM code_files WHERE parse_status = 'indexed'`,
		"empty_files":    `SELECT COUNT(*) FROM code_files WHERE parse_status = 'empty'`,
		"degraded_files": `SELECT COUNT(*) FROM code_files WHERE parse_status = 'degraded'`,
		"symbols":        `SELECT COUNT(*) FROM code_symbols`,
		"edges":          `SELECT COUNT(*) FROM code_edges`,
		"ambiguous":      `SELECT COUNT(*) FROM code_resolution_diagnostics WHERE status = 'ambiguous'`,
		"unresolved":     `SELECT COUNT(*) FROM code_resolution_diagnostics WHERE status = 'unresolved'`,
	}
	for name, query := range queries {
		var count int
		if err := database.QueryRow(query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		metrics[name] = count
	}
	if metrics["degraded_files"] != 0 || metrics["symbols"] == 0 {
		t.Fatalf("target metrics = %+v", metrics)
	}
	t.Logf("TypeScript target duration=%s epoch=%d full=%t changed=%d metrics=%v", indexDuration, result.Epoch, result.FullRebuild, result.ChangedFiles, metrics)
}

func TestTypeScriptTargetIncrementalMetrics(t *testing.T) {
	sourceRoot := os.Getenv("HAFT_TS_TARGET_ROOT")
	if sourceRoot == "" {
		t.Skip("set HAFT_TS_TARGET_ROOT to run the optional real-project TypeScript index")
	}
	root := copyTypeScriptQualificationTarget(t, sourceRoot)
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "incremental-target.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	scanner := NewScanner(database)
	first, err := scanner.RefreshIncremental(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Published || first.Degraded {
		t.Fatalf("initial target refresh = %+v", first)
	}
	current, err := scanner.scanCurrentCodeFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	target := typeScriptQualificationEditTarget(codeFilePaths(current))
	if target == "" {
		t.Fatal("qualification target has no editable TypeScript source")
	}
	path := filepath.Join(root, filepath.FromSlash(target))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, []byte("\n// haft incremental qualification\n")...)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now()
	result, err := scanner.RefreshIncremental(context.Background(), root)
	duration := time.Since(startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published || result.FullRebuild || result.Degraded {
		t.Fatalf("incremental target refresh = %+v", result)
	}
	t.Logf("TypeScript incremental duration=%s target=%s changed=%d total=%d", duration, target, result.ChangedFiles, first.ChangedFiles)
}

func copyTypeScriptQualificationTarget(tb testing.TB, sourceRoot string) string {
	tb.Helper()
	targetRoot := tb.TempDir()
	registry := NewRegistry()
	err := walkProjectFiles(sourceRoot, func(path string, relPath string, entry os.DirEntry) error {
		if !registry.SupportsSymbols(path) && !typeScriptQualificationConfig(entry.Name()) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(targetRoot, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		tb.Fatal(err)
	}
	return targetRoot
}

func typeScriptQualificationConfig(name string) bool {
	if name == "package.json" || name == "pnpm-workspace.yaml" || name == "pnpm-lock.yaml" {
		return true
	}
	return strings.HasPrefix(name, "tsconfig") && filepath.Ext(name) == ".json"
}

func typeScriptQualificationEditTarget(paths []string) string {
	for index := len(paths) - 1; index >= 0; index-- {
		extension := strings.ToLower(filepath.Ext(paths[index]))
		if extension == ".ts" || extension == ".tsx" || extension == ".mts" || extension == ".cts" {
			return paths[index]
		}
	}
	return ""
}

func measureTypeScriptParity(
	t *testing.T,
	symbols *SymbolStore,
	edges []CodeEdge,
	manifest tsParityManifest,
) tsParityMetrics {
	t.Helper()
	metrics := tsParityMetrics{
		RequiredSeeds:               len(manifest.RequiredSeeds),
		RequiredRelations:           len(manifest.RequiredRelations),
		KnownMissingRelations:       len(manifest.KnownMissingRelations),
		KnownFalsePositiveRelations: len(manifest.KnownFalsePositiveRelations),
		Edges:                       len(edges),
	}
	refs, err := symbols.AllSymbolRefs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	metrics.Symbols = len(refs)
	files := map[string]bool{}
	for _, ref := range refs {
		files[ref.FilePath] = true
	}
	metrics.FilesWithSymbols = len(files)

	for _, relation := range manifest.RequiredRelations {
		if parityRelationExists(t, symbols, edges, relation) {
			metrics.RequiredRelationsResolved++
			continue
		}
		t.Errorf("required relation %q is missing; source out-edges: %v", relation.ID, paritySourceOutEdges(t, symbols, edges, relation.Source))
	}
	for _, relation := range manifest.KnownMissingRelations {
		if parityRelationExists(t, symbols, edges, relation) {
			t.Errorf("known-missing relation %q now resolves; promote it to required_relations", relation.ID)
		}
	}
	for _, relation := range manifest.KnownFalsePositiveRelations {
		if !parityRelationExists(t, symbols, edges, relation) {
			t.Errorf("known false-positive relation %q disappeared; remove it from known_false_positive_relations", relation.ID)
		}
	}
	for _, relation := range manifest.ForbiddenRelations {
		if parityRelationExists(t, symbols, edges, relation) {
			metrics.ForbiddenRelationsObserved++
			t.Errorf("forbidden relation %q was emitted", relation.ID)
		}
	}

	metrics.RequiredRecall = ratio(metrics.RequiredRelationsResolved, metrics.RequiredRelations)
	labeledObserved := metrics.RequiredRelationsResolved + metrics.KnownFalsePositiveRelations + metrics.ForbiddenRelationsObserved
	metrics.LabeledPrecision = ratio(metrics.RequiredRelationsResolved, labeledObserved)
	return metrics
}

func paritySourceOutEdges(t testing.TB, symbols *SymbolStore, edges []CodeEdge, sourceRef tsParitySymbolRef) []string {
	t.Helper()
	source := mustResolveParitySymbol(t, symbols, sourceRef)
	out := make([]string, 0)
	for _, edge := range edges {
		if edge.SrcID != source.ID {
			continue
		}
		target, ok, err := symbols.GetByID(context.Background(), edge.DstID)
		if err != nil || !ok {
			out = append(out, string(edge.Kind)+"-><missing:"+edge.DstID+">")
			continue
		}
		out = append(out, string(edge.Kind)+"->"+target.FilePath+"::"+target.QualifiedName)
	}
	return out
}

func validateParityRelationEndpoints(t *testing.T, symbols *SymbolStore, manifest tsParityManifest) {
	t.Helper()
	groups := [][]tsParityRelation{
		manifest.RequiredRelations,
		manifest.KnownMissingRelations,
		manifest.KnownFalsePositiveRelations,
		manifest.ForbiddenRelations,
	}
	for _, relations := range groups {
		for _, relation := range relations {
			mustResolveParitySymbol(t, symbols, relation.Source)
			mustResolveParitySymbol(t, symbols, relation.Target)
		}
	}
}

func parityRelationExists(
	t *testing.T,
	symbols *SymbolStore,
	edges []CodeEdge,
	relation tsParityRelation,
) bool {
	t.Helper()
	source := mustResolveParitySymbol(t, symbols, relation.Source)
	target := mustResolveParitySymbol(t, symbols, relation.Target)
	for _, edge := range edges {
		if edge.SrcID == source.ID && edge.DstID == target.ID && edge.Kind == relation.Kind {
			return true
		}
	}
	return false
}

func mustResolveParitySymbol(t testing.TB, symbols *SymbolStore, ref tsParitySymbolRef) CodeSymbol {
	t.Helper()
	candidates, err := symbols.GetByFile(context.Background(), filepath.FromSlash(ref.File))
	if err != nil {
		t.Fatal(err)
	}
	matches := make([]CodeSymbol, 0, 1)
	for _, candidate := range candidates {
		if candidate.Name != ref.Name || candidate.Receiver != ref.Receiver {
			continue
		}
		if ref.Kind != "" && candidate.Kind != ref.Kind {
			continue
		}
		matches = append(matches, candidate)
	}
	if len(matches) != 1 {
		t.Fatalf("symbol %+v resolved to %d candidate(s): %+v", ref, len(matches), matches)
	}
	return matches[0]
}

func loadTypeScriptParityManifest(t testing.TB) tsParityManifest {
	t.Helper()
	path := filepath.Join(typeScriptParityRoot(), "manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest tsParityManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func typeScriptParityRoot() string {
	return filepath.Join("testdata", "typescript_parity")
}

func loadCodeGraphQualificationCarrier(t testing.TB) codeGraphQualificationCarrier {
	t.Helper()
	path := filepath.Join(typeScriptParityRoot(), "codegraph_reference.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var carrier codeGraphQualificationCarrier
	if err := json.Unmarshal(raw, &carrier); err != nil {
		t.Fatal(err)
	}
	return carrier
}

func typeScriptParityFixtureDigest(t testing.TB) string {
	t.Helper()
	root := typeScriptParityRoot()
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(
		path string,
		entry fs.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relativePath == "codegraph_reference.json" {
			return nil
		}
		paths = append(paths, filepath.ToSlash(relativePath))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	digest := sha256.New()
	for _, relativePath := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintf(digest, "%d:%s\n%d:", len(relativePath), relativePath, len(content))
		_, _ = digest.Write(content)
	}
	return fmt.Sprintf("sha256:%x", digest.Sum(nil))
}

func buildTypeScriptParityGraph(t testing.TB, root string) (*SymbolStore, *EdgeStore) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "parity.db")
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	scanner := NewScanner(database)
	if _, err := scanner.ScanSymbols(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if _, err := scanner.ScanEdges(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	return NewSymbolStore(database), NewEdgeStore(database)
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 1
	}
	return float64(numerator) / float64(denominator)
}

func BenchmarkTypeScriptParityColdIndex(b *testing.B) {
	root := typeScriptParityRoot()
	databaseRoot := b.TempDir()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		path := filepath.Join(databaseRoot, fmt.Sprintf("cold-%d.db", iteration))
		database, err := sql.Open("sqlite", path)
		if err != nil {
			b.Fatal(err)
		}
		scanner := NewScanner(database)
		if _, err := scanner.ScanSymbols(context.Background(), root); err != nil {
			b.Fatal(err)
		}
		if _, err := scanner.ScanEdges(context.Background(), root); err != nil {
			b.Fatal(err)
		}
		if err := database.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTypeScriptParityWarmLookup(b *testing.B) {
	symbols, _ := buildTypeScriptParityGraph(b, typeScriptParityRoot())
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		matches, err := symbols.GetByName(context.Background(), "createApplication")
		if err != nil {
			b.Fatal(err)
		}
		if len(matches) != 1 {
			b.Fatalf("matches = %d, want 1", len(matches))
		}
	}
}

func BenchmarkTypeScriptParityOneFileRefresh(b *testing.B) {
	root := typeScriptParityRoot()
	symbols, edges := buildTypeScriptParityGraph(b, root)
	registry := NewRegistry()
	relPath := filepath.FromSlash("src/app.ts")
	resolver := registry.ResolverForFile(relPath)
	if resolver == nil {
		b.Fatal("TypeScript resolver is unavailable")
	}
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if err := symbols.IndexFileSymbolsWithRegistry(context.Background(), root, relPath, registry); err != nil {
			b.Fatal(err)
		}
		resolved, err := resolver.ResolveFileEdges(context.Background(), root, relPath, symbols)
		if err != nil {
			b.Fatal(err)
		}
		if err := edges.ReplaceFileEdges(context.Background(), relPath, resolved); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTypeScriptParityAtomicIncrementalRefresh(b *testing.B) {
	root := filepath.Join(b.TempDir(), "corpus")
	copyTypeScriptParityCorpus(b, typeScriptParityRoot(), root)
	database, err := sql.Open("sqlite", filepath.Join(b.TempDir(), "incremental.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = database.Close() })
	scanner := NewScanner(database)
	if _, err := scanner.RefreshIncremental(context.Background(), root); err != nil {
		b.Fatal(err)
	}
	target := filepath.Join(root, "src", "helpers.ts")
	original, err := os.ReadFile(target)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		content := append([]byte{}, original...)
		marker := fmt.Sprintf("\n// refresh-%d\n", iteration)
		content = append(content, []byte(marker)...)
		if err := os.WriteFile(target, content, 0o644); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		result, err := scanner.RefreshIncremental(context.Background(), root)
		if err != nil {
			b.Fatal(err)
		}
		if !result.Published || result.FullRebuild {
			b.Fatalf("incremental result = %+v", result)
		}
	}
}

func BenchmarkTypeScriptTargetAtomicIncrementalRefresh(b *testing.B) {
	sourceRoot := os.Getenv("HAFT_TS_TARGET_ROOT")
	if sourceRoot == "" {
		b.Skip("set HAFT_TS_TARGET_ROOT to run the optional real-project TypeScript index")
	}
	root := copyTypeScriptQualificationTarget(b, sourceRoot)
	database, err := sql.Open("sqlite", filepath.Join(b.TempDir(), "target-incremental.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = database.Close() })
	scanner := NewScanner(database)
	first, err := scanner.RefreshIncremental(context.Background(), root)
	if err != nil || !first.Published {
		b.Fatalf("initial refresh = %+v err=%v", first, err)
	}
	current, err := scanner.scanCurrentCodeFiles(root)
	if err != nil {
		b.Fatal(err)
	}
	target := typeScriptQualificationEditTarget(codeFilePaths(current))
	path := filepath.Join(root, filepath.FromSlash(target))
	original, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		content := append([]byte{}, original...)
		marker := fmt.Sprintf("\n// target-refresh-%d\n", iteration)
		content = append(content, []byte(marker)...)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		result, err := scanner.RefreshIncremental(context.Background(), root)
		if err != nil {
			b.Fatal(err)
		}
		if !result.Published || result.FullRebuild {
			b.Fatalf("incremental result = %+v", result)
		}
	}
}

func copyTypeScriptParityCorpus(tb testing.TB, sourceRoot, targetRoot string) {
	tb.Helper()
	err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(targetRoot, relPath)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		tb.Fatal(err)
	}
}
