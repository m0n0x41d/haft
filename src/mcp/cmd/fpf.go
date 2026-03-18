package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/m0n0x41d/quint-code/internal/fpf"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var fpfCmd = &cobra.Command{
	Use:   "fpf",
	Short: "Search the FPF (First Principles Framework) specification",
	Long: `Search the embedded FPF specification using full-text search.

The FPF spec is indexed and embedded in the binary — no external files needed.

Examples:
  quint-code fpf search "WLNK weak link"
  quint-code fpf search "ADI cycle" --limit 5
  quint-code fpf search "characterization" --full
  quint-code fpf section "3.1. WLNK"
  quint-code fpf info`,
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
	Short: "Show FPF index version and upstream commit",
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
	tmpDir, err := os.MkdirTemp("", "quint-fpf-*")
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
	// Support legacy-style invocation: quint-code fpf search "term1" "term2" --limit 5
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
		fmt.Println("No results found.")
		return nil
	}

	for i, r := range results {
		fmt.Printf("### %d. %s\n\n", i+1, r.Heading)
		if fpfSearchFull {
			body, err := fpf.GetSpecSection(db, r.Heading)
			if err == nil {
				fmt.Println(body)
			} else {
				fmt.Println(r.Snippet)
			}
		} else {
			fmt.Println(r.Snippet)
		}
		fmt.Println()
	}
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

	fmt.Printf("## %s\n\n%s\n", heading, body)
	return nil
}

func runFPFInfo(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openFPFDB()
	if err != nil {
		return err
	}
	defer cleanup()

	fmt.Printf("quint-code fpf version: %s\n", Version)

	commit, err := fpf.GetSpecMeta(db, "fpf_commit")
	if err == nil {
		fmt.Printf("FPF upstream commit: %s\n", commit)
		fmt.Printf("FPF source: https://github.com/ailev/FPF/commit/%s\n", commit)
	}
	return nil
}
