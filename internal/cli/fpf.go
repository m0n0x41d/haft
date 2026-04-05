package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/present"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var fpfCmd = &cobra.Command{
	Use:   "fpf",
	Short: "Search the FPF (First Principles Framework) specification",
	Long: `Search the embedded FPF specification using full-text search.

The FPF spec is indexed and embedded in the binary — no external files needed.

Examples:
  haft fpf search "WLNK weak link"
  haft fpf search "ADI cycle" --limit 5
  haft fpf search "characterization" --full
  haft fpf search "boundary routing" --tier route --explain
  haft fpf search "compare options with normalization" --mode tree --explain
  haft fpf semantic-index
  haft fpf semantic-search "boundary contract unpacking" --explain
  haft fpf section "A.6"
  haft fpf section "A.6 - Signature Stack & Boundary Discipline"
  haft fpf info`,
}

var fpfSearchCmd = &cobra.Command{
	Use:   "search <query> [--limit N] [--full] [--explain] [--tier TIER] [--mode MODE]",
	Short: "Search FPF spec by keyword",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runFPFSearch,
}

var fpfSemanticIndexCmd = &cobra.Command{
	Use:   "semantic-index",
	Short: "Build the experimental FPF semantic embedding artifact",
	Long: `Build the explicit embedding artifact used by "haft fpf semantic-search".

This stays outside the supported deterministic retriever. The artifact is
stored under ~/.cache/haft by default, so it remains optional and ignored by
git. A direct OpenAI API key is required because the artifact is built against
the platform embeddings API, not the ChatGPT/Codex responses backend.`,
	RunE: runFPFSemanticIndex,
}

var fpfSemanticSearchCmd = &cobra.Command{
	Use:   "semantic-search <query> [--limit N] [--full] [--explain]",
	Short: "Run the experimental embedding-backed FPF search prototype",
	Long: `Run the explicit semantic-search experiment for the embedded FPF spec.

This command is intentionally opt-in. The standard "haft fpf search" path
remains the authoritative deterministic retriever. "semantic-search" uses an
optional embedding artifact built by "haft fpf semantic-index": exact
pattern-id preservation, deterministic route seeds when classification is
clear, and embedding cosine ranking over the full indexed FPF corpus. The
full-corpus noisy-query harness gives a deterministic wiring/proxy check
before broader rollout; live embedding quality still needs explicit
measurement with the built artifact.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runFPFSemanticSearch,
}

var fpfSectionCmd = &cobra.Command{
	Use:   "section <heading-or-pattern-id>",
	Short: "Get full content of a section by heading or pattern id",
	Long: `Look up one exact FPF section by either its indexed heading or its pattern id.

Pattern id input uses the same normalization as search, so common formatting
variants such as "a.6" and "A.6:" still resolve to the canonical section.`,
	Example: `  haft fpf section "A.6"
  haft fpf section "A.6 - Signature Stack & Boundary Discipline"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runFPFSection,
}

var fpfInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show FPF index version and provenance metadata",
	RunE:  runFPFInfo,
}

var (
	fpfSearchLimit           int
	fpfSearchFull            bool
	fpfSearchExplain         bool
	fpfSearchTier            string
	fpfSearchMode            string
	fpfSemanticArtifactPath  string
	fpfSemanticDimensions    int
	fpfSemanticModel         string
	fpfSemanticSearchLimit   int
	fpfSemanticSearchFull    bool
	fpfSemanticSearchExplain bool
)

var openFPFDBFunc = openFPFDB
var newFPFSemanticEmbedder = fpf.NewOpenAISemanticEmbedder

func init() {
	fpfSearchCmd.Flags().IntVar(&fpfSearchLimit, "limit", 10, "Maximum number of results")
	fpfSearchCmd.Flags().BoolVar(&fpfSearchFull, "full", false, "Show full section content instead of snippets")
	fpfSearchCmd.Flags().BoolVar(&fpfSearchExplain, "explain", false, "Show why each result matched")
	fpfSearchCmd.Flags().StringVar(&fpfSearchTier, "tier", "", "Restrict results to one tier: pattern, drilldown, route, related, or fts")
	fpfSearchCmd.Flags().StringVar(&fpfSearchMode, "mode", "", "Experimental retrieval mode; currently supports tree")
	fpfSemanticIndexCmd.Flags().StringVar(&fpfSemanticArtifactPath, "artifact", "", "Path for the semantic artifact (defaults to ~/.cache/haft/...)")
	fpfSemanticIndexCmd.Flags().StringVar(&fpfSemanticModel, "model", fpf.DefaultSemanticEmbeddingModel, "Embedding model for the semantic artifact")
	fpfSemanticIndexCmd.Flags().IntVar(&fpfSemanticDimensions, "dimensions", fpf.DefaultSemanticEmbeddingDimensions, "Embedding dimensions for the semantic artifact")
	fpfSemanticSearchCmd.Flags().IntVar(&fpfSemanticSearchLimit, "limit", 10, "Maximum number of results")
	fpfSemanticSearchCmd.Flags().BoolVar(&fpfSemanticSearchFull, "full", false, "Show full section content instead of snippets")
	fpfSemanticSearchCmd.Flags().BoolVar(&fpfSemanticSearchExplain, "explain", false, "Show why each result matched")
	fpfSemanticSearchCmd.Flags().StringVar(&fpfSemanticArtifactPath, "artifact", "", "Path to the semantic artifact (defaults to ~/.cache/haft/...)")
	fpfSemanticSearchCmd.Flags().StringVar(&fpfSemanticModel, "model", fpf.DefaultSemanticEmbeddingModel, "Embedding model that matches the semantic artifact")
	fpfSemanticSearchCmd.Flags().IntVar(&fpfSemanticDimensions, "dimensions", fpf.DefaultSemanticEmbeddingDimensions, "Embedding dimensions that match the semantic artifact")

	fpfCmd.AddCommand(fpfSearchCmd)
	fpfCmd.AddCommand(fpfSemanticIndexCmd)
	fpfCmd.AddCommand(fpfSemanticSearchCmd)
	fpfCmd.AddCommand(fpfSectionCmd)
	fpfCmd.AddCommand(fpfInfoCmd)
	rootCmd.AddCommand(fpfCmd)
}

func openFPFDB() (*sql.DB, func(), error) {
	tmpDir, err := os.MkdirTemp("", "haft-fpf-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir: %w", err)
	}

	dbPath := filepath.Join(tmpDir, "fpf.db")
	if err := os.WriteFile(dbPath, embeddedFPFDB, 0644); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, nil, fmt.Errorf("write temp db: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, nil, fmt.Errorf("open db: %w", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
	}
	return db, cleanup, nil
}

func runFPFSearch(cmd *cobra.Command, args []string) error {
	// Support legacy-style invocation: haft fpf search "term1" "term2" --limit 5
	var queryParts []string
	for _, arg := range args {
		if _, err := strconv.Atoi(arg); err == nil {
			continue // skip stray numbers (parsed by flags)
		}
		queryParts = append(queryParts, arg)
	}
	query := strings.Join(queryParts, " ")
	if query == "" {
		return fmt.Errorf("empty query")
	}

	normalizedTier, err := fpf.NormalizeSpecSearchTier(fpfSearchTier)
	if err != nil {
		return fmt.Errorf("invalid --tier: %w", err)
	}
	normalizedMode, err := fpf.NormalizeSpecSearchMode(fpfSearchMode)
	if err != nil {
		return fmt.Errorf("invalid --mode: %w", err)
	}
	retrieval, err := retrieveEmbeddedFPF(fpf.SpecRetrievalRequest{
		Query: query,
		Limit: fpfSearchLimit,
		Tier:  normalizedTier,
		Full:  fpfSearchFull,
		Mode:  normalizedMode,
	})
	if err != nil {
		return fmt.Errorf("search error: %w", err)
	}

	if len(retrieval.Results) == 0 {
		fmt.Print(formatCLIFPFSearchWithExplain(nil, fpfSearchExplain))
		return nil
	}

	fmt.Print(formatCLIFPFSearchWithExplain(presentFPFRetrieval(retrieval.Results), fpfSearchExplain))
	return nil
}

func runFPFSemanticIndex(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		return err
	}
	defer cleanup()

	artifactPath, err := resolveFPFSemanticArtifactPath()
	if err != nil {
		return err
	}

	embedder, err := newFPFSemanticEmbedder(fpfSemanticModel, fpfSemanticDimensions)
	if err != nil {
		return fmt.Errorf("semantic index embedder: %w", err)
	}

	if err := fpf.BuildSemanticArtifact(context.Background(), db, embedder, artifactPath); err != nil {
		return fmt.Errorf("build semantic artifact: %w", err)
	}

	fmt.Printf("built semantic artifact: %s\n", artifactPath)
	return nil
}

func runFPFSemanticSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("empty query")
	}

	artifactPath, err := resolveFPFSemanticArtifactPath()
	if err != nil {
		return err
	}

	retrieval, err := retrieveEmbeddedFPF(fpf.SpecRetrievalRequest{
		Query:                query,
		Limit:                fpfSemanticSearchLimit,
		Full:                 fpfSemanticSearchFull,
		Mode:                 fpf.SpecRetrievalModeSemantic,
		SemanticArtifactPath: artifactPath,
		SemanticEmbedderFactory: func() (fpf.SemanticEmbedder, error) {
			return newFPFSemanticEmbedder(fpfSemanticModel, fpfSemanticDimensions)
		},
	})
	if err != nil {
		return fmt.Errorf("semantic search error: %w", err)
	}

	if len(retrieval.Results) == 0 {
		fmt.Print(formatCLIFPFSearchWithExplain(nil, fpfSemanticSearchExplain))
		return nil
	}

	fmt.Print(formatCLIFPFSearchWithExplain(presentFPFRetrieval(retrieval.Results), fpfSemanticSearchExplain))
	return nil
}

func resolveFPFSemanticArtifactPath() (string, error) {
	if strings.TrimSpace(fpfSemanticArtifactPath) != "" {
		return fpfSemanticArtifactPath, nil
	}

	return fpf.DefaultSemanticArtifactPath(fpfSemanticModel, fpfSemanticDimensions)
}

func runFPFSection(cmd *cobra.Command, args []string) error {
	lookup := strings.Join(args, " ")

	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		return err
	}
	defer cleanup()

	body, err := fpf.GetSpecSection(db, lookup)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("section not found by heading or pattern id: %q", lookup)
		}
		return fmt.Errorf("get FPF section: %w", err)
	}

	fmt.Print(present.FormatFPFSection(lookup, body))
	return nil
}

func runFPFInfo(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		return err
	}
	defer cleanup()

	info := present.FPFInfo{
		Version: Version,
	}
	indexInfo, err := fpf.GetSpecIndexInfo(db)
	if err != nil {
		return err
	}

	info.Commit = indexInfo.Commit
	info.IndexedSections = indexInfo.IndexedSections
	info.BuildTime = indexInfo.BuildTime
	info.SpecPath = indexInfo.SpecPath
	info.SchemaVersion = indexInfo.SchemaVersion

	fmt.Print(present.FormatFPFInfo(info))
	return nil
}
