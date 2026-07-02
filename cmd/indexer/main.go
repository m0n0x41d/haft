package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/embedding"
	"github.com/m0n0x41d/haft/internal/fpf"
	_ "modernc.org/sqlite"
)

// verifyIndex is the CI guard: the FPF index is baked locally (heavy CPU
// inference unfit for runners) and committed, so CI must check the committed
// fpf.db is fresh (matches the submodule SHA) and carries baked vectors — never
// re-bake. Fails loudly so a maintainer who forgot `task fpf-refresh` cannot
// ship a stale or vectorless index.
func verifyIndex(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: indexer -verify <fpf.db> <expected-fpf-commit-sha>")
	}
	dbPath, expectedSHA := args[0], strings.TrimSpace(args[1])

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer func() { _ = db.Close() }()

	commit, err := fpf.GetSpecMeta(db, "fpf_commit")
	if err != nil {
		return fmt.Errorf("read fpf_commit meta: %w", err)
	}
	if strings.TrimSpace(commit) != expectedSHA {
		return fmt.Errorf("fpf.db is STALE: meta fpf_commit=%q but submodule HEAD=%q — run `task fpf-refresh` and commit the result", commit, expectedSHA)
	}

	schemaVersion, err := fpf.GetSpecMeta(db, "schema_version")
	if err != nil {
		return fmt.Errorf("read schema_version meta: %w", err)
	}
	if strings.TrimSpace(schemaVersion) != fpf.SpecIndexSchemaVersion {
		return fmt.Errorf("fpf.db schema_version=%q but code expects %q — run `task fpf-refresh` and commit the result", schemaVersion, fpf.SpecIndexSchemaVersion)
	}

	indexedSections, err := readIndexedSectionCount(db)
	if err != nil {
		return err
	}

	count, err := countSpecEmbeddingsForContract(db, shippedFPFEmbeddingContract)
	if err != nil {
		return fmt.Errorf("read embedding contract: %w", err)
	}
	if count != indexedSections {
		return fmt.Errorf("fpf.db vector contract mismatch: expected %d vectors for %s, found %d (%s) — run `task fpf-refresh` without HAFT_FPF_BAKE_SCOPE/HAFT_FPF_BAKE_DIM/HAFT_EMBED_MODEL overrides and commit the result",
			indexedSections, shippedFPFEmbeddingContract, count, describeDominantSpecEmbeddingContract(db))
	}

	routeDocuments := fpf.PatternUseRouteEmbeddingDocuments(fpf.DefaultPatternUseRouteCards())
	routeCount, err := fpf.CountPatternUseRouteEmbeddingsForContract(
		db,
		shippedFPFEmbeddingContract.provider,
		shippedFPFEmbeddingContract.model,
		shippedFPFEmbeddingContract.dim,
	)
	if err != nil {
		return fmt.Errorf("read pattern-use route embedding contract: %w", err)
	}
	if routeCount != len(routeDocuments) {
		return fmt.Errorf("fpf.db PatternUse route vector contract mismatch: expected %d route document vectors for %s, found %d (%s) — run `task fpf-index` without HAFT_FPF_BAKE_SCOPE/HAFT_FPF_BAKE_DIM/HAFT_EMBED_MODEL overrides and commit the result",
			len(routeDocuments), shippedFPFEmbeddingContract, routeCount, describeDominantPatternUseRouteEmbeddingContract(db))
	}

	missingRouteDocuments, err := fpf.MissingPatternUseRouteEmbeddingDocuments(
		db,
		shippedFPFEmbeddingContract.provider,
		shippedFPFEmbeddingContract.model,
		shippedFPFEmbeddingContract.dim,
		routeDocuments,
	)
	if err != nil {
		return fmt.Errorf("verify pattern-use route embedding hashes: %w", err)
	}
	if len(missingRouteDocuments) > 0 {
		return fmt.Errorf("fpf.db PatternUse route vectors are STALE: %d current route document hash(es) missing for %s (first missing: %s) — run `task fpf-index` and commit the result",
			len(missingRouteDocuments), shippedFPFEmbeddingContract, missingRouteDocuments[0])
	}

	intentDocuments := fpf.PatternUseIntentEmbeddingDocuments(fpf.DefaultPatternUseIntentLaneCards())
	intentCount, err := fpf.CountPatternUseIntentEmbeddingsForContract(
		db,
		shippedFPFEmbeddingContract.provider,
		shippedFPFEmbeddingContract.model,
		shippedFPFEmbeddingContract.dim,
	)
	if err != nil {
		return fmt.Errorf("read pattern-use intent embedding contract: %w", err)
	}
	if intentCount != len(intentDocuments) {
		return fmt.Errorf("fpf.db PatternUse intent vector contract mismatch: expected %d intent document vectors for %s, found %d (%s) — run `task fpf-index` without HAFT_FPF_BAKE_SCOPE/HAFT_FPF_BAKE_DIM/HAFT_EMBED_MODEL overrides and commit the result",
			len(intentDocuments), shippedFPFEmbeddingContract, intentCount, describeDominantPatternUseIntentEmbeddingContract(db))
	}

	missingIntentDocuments, err := fpf.MissingPatternUseIntentEmbeddingDocuments(
		db,
		shippedFPFEmbeddingContract.provider,
		shippedFPFEmbeddingContract.model,
		shippedFPFEmbeddingContract.dim,
		intentDocuments,
	)
	if err != nil {
		return fmt.Errorf("verify pattern-use intent embedding hashes: %w", err)
	}
	if len(missingIntentDocuments) > 0 {
		return fmt.Errorf("fpf.db PatternUse intent vectors are STALE: %d current intent document hash(es) missing for %s (first missing: %s) — run `task fpf-index` and commit the result",
			len(missingIntentDocuments), shippedFPFEmbeddingContract, missingIntentDocuments[0])
	}

	atlasNodes, atlasCards, atlasLints, err := fpf.PatternAtlasCounts(db)
	if err != nil {
		return fmt.Errorf("read PatternAtlas contract: %w", err)
	}
	if atlasNodes == 0 || atlasCards == 0 {
		return fmt.Errorf("fpf.db PatternAtlas contract mismatch: expected baked atlas nodes/cards, found %d nodes and %d cards — run `task fpf-index` and commit the result", atlasNodes, atlasCards)
	}
	if err := verifyPatternAtlasCommit(db, expectedSHA); err != nil {
		return err
	}
	if err := verifyPatternAtlasRequiredCards(db); err != nil {
		return err
	}
	if err := verifyPatternAtlasIntegrity(db); err != nil {
		return err
	}

	fmt.Printf("fpf.db OK: commit %s, %d baked section vectors, %d PatternUse route vectors, %d PatternUse intent vectors, %d PatternAtlas nodes, %d PatternAtlas cards, %d PatternAtlas lints for %s\n",
		commit[:min(8, len(commit))],
		count,
		routeCount,
		intentCount,
		atlasNodes,
		atlasCards,
		atlasLints,
		shippedFPFEmbeddingContract,
	)
	return nil
}

const shippedFPFEmbeddingDim = 256

type specEmbeddingContract struct {
	provider string
	model    string
	dim      int
}

func (c specEmbeddingContract) String() string {
	return fmt.Sprintf("%s/%s/%d", c.provider, c.model, c.dim)
}

var shippedFPFEmbeddingContract = specEmbeddingContract{
	provider: embedding.ProviderLocal,
	model:    embedding.DefaultLocalModel,
	dim:      shippedFPFEmbeddingDim,
}

func readIndexedSectionCount(db *sql.DB) (int, error) {
	value, err := fpf.GetSpecMeta(db, "indexed_sections")
	if err != nil {
		return 0, fmt.Errorf("read indexed_sections meta: %w", err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("indexed_sections=%q is not an integer", value)
	}
	if count <= 0 {
		return 0, fmt.Errorf("indexed_sections=%d must be positive", count)
	}
	return count, nil
}

func countSpecEmbeddingsForContract(db *sql.DB, contract specEmbeddingContract) (int, error) {
	var count int
	err := db.
		QueryRow(
			`SELECT COUNT(*) FROM fpf_embeddings WHERE provider=? AND model=? AND dim=?`,
			contract.provider,
			contract.model,
			contract.dim,
		).
		Scan(&count)
	return count, err
}

func describeDominantSpecEmbeddingContract(db *sql.DB) string {
	provider, model, dim, count, err := fpf.SpecEmbeddingContract(db)
	if err != nil {
		return fmt.Sprintf("dominant contract unavailable: %v", err)
	}
	if count == 0 {
		return "no baked vectors"
	}
	return fmt.Sprintf("dominant contract %s/%s/%d has %d vectors", provider, model, dim, count)
}

func describeDominantPatternUseRouteEmbeddingContract(db *sql.DB) string {
	provider, model, dim, count, err := fpf.PatternUseRouteEmbeddingContract(db)
	if err != nil {
		return fmt.Sprintf("dominant contract unavailable: %v", err)
	}
	if count == 0 {
		return "no baked PatternUse route vectors"
	}
	return fmt.Sprintf("dominant contract %s/%s/%d has %d vectors", provider, model, dim, count)
}

func describeDominantPatternUseIntentEmbeddingContract(db *sql.DB) string {
	provider, model, dim, count, err := fpf.PatternUseIntentEmbeddingContract(db)
	if err != nil {
		return fmt.Sprintf("dominant contract unavailable: %v", err)
	}
	if count == 0 {
		return "no baked PatternUse intent vectors"
	}
	return fmt.Sprintf("dominant contract %s/%s/%d has %d vectors", provider, model, dim, count)
}

var requiredPatternAtlasCards = []string{"F.18", "C.30", "A.10", "A.7", "B.3"}

func verifyPatternAtlasCommit(db *sql.DB, expectedSHA string) error {
	var mismatches int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM (
			SELECT fpf_commit FROM pattern_atlas_nodes
			UNION ALL
			SELECT fpf_commit FROM pattern_atlas_cards
			UNION ALL
			SELECT fpf_commit FROM pattern_atlas_lints
		)
		WHERE fpf_commit <> ?`, expectedSHA).Scan(&mismatches)
	if err != nil {
		return fmt.Errorf("read PatternAtlas commit contract: %w", err)
	}
	if mismatches > 0 {
		return fmt.Errorf("fpf.db PatternAtlas is STALE: %d atlas row(s) do not match submodule HEAD %q — run `task fpf-index` and commit the result", mismatches, expectedSHA)
	}
	return nil
}

func verifyPatternAtlasRequiredCards(db *sql.DB) error {
	missing, err := fpf.MissingPatternAtlasCards(db, requiredPatternAtlasCards)
	if err != nil {
		return fmt.Errorf("verify PatternAtlas required cards: %w", err)
	}
	if len(missing) > 0 {
		return fmt.Errorf("fpf.db PatternAtlas missing full card(s): %s — run `task fpf-index` and inspect markdown heading extraction", strings.Join(missing, ", "))
	}
	return nil
}

func verifyPatternAtlasIntegrity(db *sql.DB) error {
	rangeErrors, err := fpf.PatternAtlasRangeIntegrityErrors(db)
	if err != nil {
		return fmt.Errorf("verify PatternAtlas ranges: %w", err)
	}
	if len(rangeErrors) > 0 {
		return fmt.Errorf("fpf.db PatternAtlas range integrity failed: %s", rangeErrors[0])
	}

	hashErrors, err := fpf.PatternAtlasHashIntegrityErrors(db)
	if err != nil {
		return fmt.Errorf("verify PatternAtlas hashes: %w", err)
	}
	if len(hashErrors) > 0 {
		return fmt.Errorf("fpf.db PatternAtlas hash integrity failed: %s", hashErrors[0])
	}
	return nil
}

const routeArtifactPath = "internal/fpf/fpf-routes.json"
const patternsDir = "internal/fpf/patterns"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: indexer <FPF-Spec.md> [output.db] [fpf-commit-sha]  |  indexer -verify <fpf.db> <expected-sha>")
	}
	if os.Args[1] == "-verify" {
		return verifyIndex(os.Args[2:])
	}

	specPath := os.Args[1]
	dbPath := filepath.Join("internal", "cli", "fpf.db")
	if len(os.Args) >= 3 {
		dbPath = os.Args[2]
	}
	commitSHA := ""
	if len(os.Args) >= 4 {
		commitSHA = os.Args[3]
	}

	return buildIndex(specPath, dbPath, commitSHA, routeArtifactPath)
}

func buildIndex(specPath, dbPath, commitSHA, routePath string) error {
	resolvedCommit := resolveSpecCommit(commitSHA, specPath)
	corpus, err := fpf.LoadSpecIndexCorpus(specPath)
	if err != nil {
		return fmt.Errorf("load production spec corpus: %w", err)
	}

	routes, err := fpf.LoadRoutes(routePath)
	if err != nil {
		return fmt.Errorf("loading routes: %w", err)
	}

	// Load compiled pattern files and merge into the corpus
	patternChunks, err := fpf.LoadPatternChunks(patternsDir)
	if err != nil {
		return fmt.Errorf("loading patterns: %w", err)
	}

	allChunks := make([]fpf.SpecChunk, 0, len(corpus.Indexed)+len(patternChunks))
	allChunks = append(allChunks, corpus.Indexed...)
	allChunks = append(allChunks, patternChunks...)

	if err := fpf.BuildSpecIndex(dbPath, allChunks, routes); err != nil {
		return fmt.Errorf("building index: %w", err)
	}

	atlas, err := fpf.LoadPatternAtlas(specPath, resolvedCommit)
	if err != nil {
		return fmt.Errorf("build PatternAtlas: %w", err)
	}
	if err := fpf.StorePatternAtlas(dbPath, atlas); err != nil {
		return fmt.Errorf("store PatternAtlas: %w", err)
	}

	// build_time is the FPF spec COMMIT's date, not wall-clock time, so the
	// index is byte-reproducible: a given submodule SHA always yields the same
	// fpf.db (committed == rebuild; every release-matrix platform ships identical
	// bytes). Wall-clock time.Now() would drift every build.
	metadata := buildSpecIndexMetadata(specPath, len(allChunks), resolvedCommit, resolveSpecBuildTime(resolvedCommit, specPath))
	if err := fpf.SetSpecMetaEntries(dbPath, metadata); err != nil {
		return fmt.Errorf("setting meta: %w", err)
	}

	baked, err := bakeSpecEmbeddingsFunc(dbPath)
	if err != nil {
		return fmt.Errorf("bake embeddings: %w", err)
	}
	if baked == 0 {
		return fmt.Errorf("bake embeddings: no section vectors baked — install/build haft-embed before running indexer")
	}

	routeBaked, err := bakePatternUseRouteEmbeddingsFunc(dbPath)
	if err != nil {
		return fmt.Errorf("bake PatternUse route embeddings: %w", err)
	}
	if routeBaked == 0 {
		return fmt.Errorf("bake PatternUse route embeddings: no route document vectors baked")
	}

	intentBaked, err := bakePatternUseIntentEmbeddingsFunc(dbPath)
	if err != nil {
		return fmt.Errorf("bake PatternUse intent embeddings: %w", err)
	}
	if intentBaked == 0 {
		return fmt.Errorf("bake PatternUse intent embeddings: no intent document vectors baked")
	}

	fmt.Printf("Indexed %d chunks (%d spec + %d patterns) and %d PatternAtlas cards into %s; baked %d section vectors, %d PatternUse route vectors, and %d PatternUse intent vectors\n",
		len(allChunks), len(corpus.Indexed), len(patternChunks), len(atlas.Cards), dbPath, baked, routeBaked, intentBaked)
	return nil
}

var bakeSpecEmbeddingsFunc = bakeSpecEmbeddings
var bakePatternUseRouteEmbeddingsFunc = bakePatternUseRouteEmbeddings
var bakePatternUseIntentEmbeddingsFunc = bakePatternUseIntentEmbeddings

// bakeSpecEmbeddings embeds every section into fpf_embeddings via the local
// sidecar (MRL-256). Index refresh is allowed to degrade at runtime, but the
// committed fpf.db must not be vectorless.
func bakeSpecEmbeddings(dbPath string) (int, error) {
	emb, err := embedding.New(embedding.Config{Provider: embedding.ProviderLocal, Model: os.Getenv("HAFT_EMBED_MODEL"), Dim: specEmbeddingBakeDim()})
	if err != nil {
		if embedding.Degraded(err) {
			return 0, fmt.Errorf("haft-embed unavailable; cannot build committed FPF index without baked vectors: %w", err)
		}
		return 0, fmt.Errorf("start embedder: %w", err)
	}
	defer func() { _ = emb.Close() }()

	ctx := context.Background()
	return fpf.BakeSpecEmbeddings(ctx, dbPath, indexEmbedderAdapter{embedder: emb}, bakeScopeFromEnv())
}

func bakePatternUseRouteEmbeddings(dbPath string) (int, error) {
	emb, err := embedding.New(embedding.Config{Provider: embedding.ProviderLocal, Model: os.Getenv("HAFT_EMBED_MODEL"), Dim: specEmbeddingBakeDim()})
	if err != nil {
		if embedding.Degraded(err) {
			return 0, fmt.Errorf("haft-embed unavailable; cannot build committed PatternUse route index without baked vectors: %w", err)
		}
		return 0, fmt.Errorf("start embedder: %w", err)
	}
	defer func() { _ = emb.Close() }()

	ctx := context.Background()
	routes := fpf.DefaultPatternUseRouteCards()
	return fpf.BakePatternUseRouteEmbeddings(ctx, dbPath, indexEmbedderAdapter{embedder: emb}, routes)
}

func bakePatternUseIntentEmbeddings(dbPath string) (int, error) {
	emb, err := embedding.New(embedding.Config{Provider: embedding.ProviderLocal, Model: os.Getenv("HAFT_EMBED_MODEL"), Dim: specEmbeddingBakeDim()})
	if err != nil {
		if embedding.Degraded(err) {
			return 0, fmt.Errorf("haft-embed unavailable; cannot build committed PatternUse intent index without baked vectors: %w", err)
		}
		return 0, fmt.Errorf("start embedder: %w", err)
	}
	defer func() { _ = emb.Close() }()

	ctx := context.Background()
	cards := fpf.DefaultPatternUseIntentLaneCards()
	return fpf.BakePatternUseIntentEmbeddings(ctx, dbPath, indexEmbedderAdapter{embedder: emb}, cards)
}

// specEmbeddingBakeDim is the MRL truncation target for the bake. Default 256
// (the shipped contract); HAFT_FPF_BAKE_DIM overrides — 0 means the model's
// native width (use for non-MRL models like bge where truncation hurts).
func specEmbeddingBakeDim() int {
	if v := strings.TrimSpace(os.Getenv("HAFT_FPF_BAKE_DIM")); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			return d
		}
	}
	return shippedFPFEmbeddingDim
}

// bakeScopeFromEnv selects the embedding scope. Default is the full corpus;
// HAFT_FPF_BAKE_SCOPE=patterns restricts the bake to the 66 compiled pattern
// cards (seconds vs ~tens of minutes) — used to measure whether prose sections
// earn their place before committing the scope.
func bakeScopeFromEnv() fpf.SpecEmbeddingScope {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("HAFT_FPF_BAKE_SCOPE")), "patterns") {
		return fpf.ScopePatternCards
	}
	return fpf.ScopeAllSections
}

// indexEmbedderAdapter bridges the local embedding.Embedder to the provider-free
// fpf.SemanticEmbedder port (mirror of internal/embedding's openAIAdapter). It
// embeds sections in the document role.
type indexEmbedderAdapter struct {
	embedder embedding.Embedder
}

func (a indexEmbedderAdapter) Descriptor() fpf.SemanticEmbedderDescriptor {
	d := a.embedder.Descriptor()
	return fpf.SemanticEmbedderDescriptor{Provider: d.Provider, Model: d.Model, Dimensions: d.Dimensions}
}

func (a indexEmbedderAdapter) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	return a.embedder.Embed(ctx, embedding.RoleDocument, texts)
}

func buildSpecIndexMetadata(specPath string, indexedSections int, explicitCommit string, buildTime time.Time) map[string]string {
	return map[string]string{
		"fpf_commit":       resolveSpecCommit(explicitCommit, specPath),
		"indexed_sections": fmt.Sprintf("%d", indexedSections),
		"build_time":       buildTime.UTC().Format(time.RFC3339),
		"spec_path":        filepath.Clean(specPath),
		"schema_version":   fpf.SpecIndexSchemaVersion,
	}
}

func resolveSpecCommit(explicitCommit, specPath string) string {
	commit := strings.TrimSpace(explicitCommit)
	if commit != "" {
		return commit
	}

	return detectSpecCommit(specPath)
}

// resolveSpecBuildTime returns the committer date of the FPF spec commit, so the
// index build is deterministic. Falls back to the Unix epoch when git/the commit
// is unavailable — still deterministic (never wall-clock).
func resolveSpecBuildTime(commitSHA, specPath string) time.Time {
	epoch := time.Unix(0, 0).UTC()
	gitDir, err := specGitLookupDir(specPath)
	if err != nil {
		return epoch
	}
	ref, ok := cleanSpecCommitRef(commitSHA)
	if !ok {
		return epoch
	}
	cmd := exec.Command("git", "show", "-s", "--format=%cI", ref)
	cmd.Dir = gitDir
	output, err := cmd.Output()
	if err != nil {
		return epoch
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(string(output)))
	if err != nil {
		return epoch
	}
	return parsed.UTC()
}

func cleanSpecCommitRef(commitSHA string) (string, bool) {
	ref := strings.TrimSpace(commitSHA)
	if ref == "" {
		return "HEAD", true
	}
	if len(ref) != 40 {
		return "", false
	}
	for _, r := range ref {
		if !isHexCommitRune(r) {
			return "", false
		}
	}
	return strings.ToLower(ref), true
}

func isHexCommitRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func detectSpecCommit(specPath string) string {
	gitDir, err := specGitLookupDir(specPath)
	if err != nil {
		return ""
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = gitDir

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func specGitLookupDir(specPath string) (string, error) {
	absPath, err := filepath.Abs(specPath)
	if err != nil {
		return "", err
	}

	return filepath.Dir(absPath), nil
}
