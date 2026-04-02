package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/ui"
)

var checkMode bool

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Decision health dashboard",
	Long: `Shows decision health, coverage, drift, and problems.

Use --check for CI/hooks: exits with code 1 if critical issues exist.
Interactive dashboard is being migrated to tview.`,
	RunE: runBoard,
}

func init() {
	boardCmd.Flags().BoolVar(&checkMode, "check", false, "Health check mode: print summary and exit with code 1 if critical issues")
	rootCmd.AddCommand(boardCmd)
}

func runBoard(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	haftDir := filepath.Join(projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return fmt.Errorf("get DB path: %w", err)
	}

	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}
	defer sqlDB.Close()

	store := artifact.NewStore(sqlDB)
	projectName := projCfg.Name

	data, err := ui.LoadBoardData(store, sqlDB, projectName, projectRoot)
	if err != nil {
		return fmt.Errorf("load board data: %w", err)
	}

	if checkMode {
		return runCheck(data)
	}

	// Interactive dashboard not yet available — use --check mode.
	return runCheck(data)
}

func runCheck(data *ui.BoardData) error {
	fmt.Printf("Haft Health: %s\n", data.ProjectName)
	fmt.Printf("  Decisions: %d shipped, %d pending\n", data.ShippedCount, data.PendingCount)
	fmt.Printf("  Problems:  %d backlog, %d addressed\n", len(data.BacklogProblems), data.AddressedCount)
	fmt.Printf("  Stale:     %d items\n", len(data.StaleItems))

	if data.CoverageReport != nil {
		cr := data.CoverageReport
		pct := 0
		if cr.TotalModules > 0 {
			pct = (cr.CoveredCount + cr.PartialCount) * 100 / cr.TotalModules
		}
		fmt.Printf("  Coverage:  %d%% (%d/%d modules)\n", pct, cr.CoveredCount+cr.PartialCount, cr.TotalModules)
	}

	if data.CriticalCount > 0 {
		fmt.Printf("\n  CRITICAL: %d issue(s) require attention\n", data.CriticalCount)
		os.Exit(1)
	}

	fmt.Println("\n  OK: no critical issues")
	return nil
}
