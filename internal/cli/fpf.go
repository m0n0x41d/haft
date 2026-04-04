package cli

import (
	"database/sql"
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
  haft fpf section "3.1. WLNK"
  haft fpf info`,
}

var fpfSearchCmd = &cobra.Command{
	Use:   "search <query> [--limit N] [--full]",
	Short: "Search FPF spec by keyword",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runFPFSearch,
}

var fpfSectionCmd = &cobra.Command{
	Use:   "section <heading>",
	Short: "Get full content of a section by heading",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runFPFSection,
}

var fpfInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show FPF index version and provenance metadata",
	RunE:  runFPFInfo,
}

var (
	fpfSearchLimit int
	fpfSearchFull  bool
)

func init() {
	fpfSearchCmd.Flags().IntVar(&fpfSearchLimit, "limit", 10, "Maximum number of results")
	fpfSearchCmd.Flags().BoolVar(&fpfSearchFull, "full", false, "Show full section content instead of snippets")

	fpfCmd.AddCommand(fpfSearchCmd)
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

	db, cleanup, err := openFPFDB()
	if err != nil {
		return err
	}
	defer cleanup()

	results, err := fpf.SearchSpec(db, query, fpfSearchLimit)
	if err != nil {
		return fmt.Errorf("search error: %w", err)
	}

	if len(results) == 0 {
		fmt.Print(formatCLIFPFSearch(nil))
		return nil
	}

	formattedResults := make([]present.FPFSearchResult, 0, len(results))
	for _, r := range results {
		content := r.Snippet
		if fpfSearchFull {
			body, err := fpf.GetSpecSection(db, firstNonEmpty(r.PatternID, r.Heading))
			if err == nil {
				content = body
			}
		}

		formattedResults = append(formattedResults, present.FPFSearchResult{
			PatternID: r.PatternID,
			Heading:   r.Heading,
			Tier:      r.Tier,
			Reason:    r.Reason,
			Content:   content,
		})
	}

	fmt.Print(formatCLIFPFSearch(formattedResults))
	return nil
}

func runFPFSection(cmd *cobra.Command, args []string) error {
	heading := strings.Join(args, " ")

	db, cleanup, err := openFPFDB()
	if err != nil {
		return err
	}
	defer cleanup()

	body, err := fpf.GetSpecSection(db, heading)
	if err != nil {
		return fmt.Errorf("section not found: %s", heading)
	}

	fmt.Print(present.FormatFPFSection(heading, body))
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func runFPFInfo(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openFPFDB()
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
	if strings.TrimSpace(indexInfo.Commit) != "" {
		info.Source = fmt.Sprintf("https://github.com/ailev/FPF/commit/%s", indexInfo.Commit)
	}

	fmt.Print(present.FormatFPFInfo(info))
	return nil
}
