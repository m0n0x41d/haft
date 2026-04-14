package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/present"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/ui"
)

var checkMode bool

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Interactive health dashboard",
	Long: `Full-screen interactive project health dashboard.

Shows trust status, decisions, problems, coverage, and evidence health.
Switch views with 1-5, refresh with r, quit with q.

Use --check for CI/hooks: prints compact summary and exits with code 1 if critical.`,
	RunE: runBoard,
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "CI governance check — exit 1 if findings",
	Long: `Scan for governance debt: stale artifacts, drifted decisions, unassessed decisions, coverage gaps.

Exit 0 = clean. Exit 1 = findings. Use --json for machine-readable output.

This is equivalent to "haft board --check".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkMode = true
		return runBoard(cmd, args)
	},
}

var checkJSON bool

func init() {
	boardCmd.Flags().BoolVar(&checkMode, "check", false, "CI mode: compact summary, exit 1 if critical")
	checkCmd.Flags().BoolVar(&checkJSON, "json", false, "Output findings as JSON")
	rootCmd.AddCommand(boardCmd)
	rootCmd.AddCommand(checkCmd)
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

	store := artifact.NewStore(sqlDB)

	loadData := func() (*ui.BoardData, error) {
		return ui.LoadBoardData(store, sqlDB, projCfg.Name, projectRoot)
	}

	data, err := loadData()
	if err != nil {
		return fmt.Errorf("load board data: %w", err)
	}

	// --check mode: print and exit
	if checkMode {
		fmt.Print(present.BoardCheck(data))
		if closeErr := sqlDB.Close(); closeErr != nil {
			return fmt.Errorf("close DB: %w", closeErr)
		}
		if data.CriticalCount > 0 {
			os.Exit(1)
		}
		return nil
	}
	defer sqlDB.Close()

	// Interactive mode: full-screen TUI
	currentData := data

	renderView := func(viewIndex int, width int) string {
		switch viewIndex {
		case 0:
			return present.BoardOverviewW(currentData, width)
		case 1:
			return present.BoardDecisionsW(currentData, width)
		case 2:
			return present.BoardProblemsW(currentData, width)
		case 3:
			return present.BoardCoverageW(currentData, width)
		case 4:
			return present.BoardEvidenceW(currentData, width)
		default:
			return present.BoardOverviewW(currentData, width)
		}
	}

	refresh := func() error {
		newData, err := loadData()
		if err != nil {
			return err
		}
		currentData = newData
		return nil
	}

	return ui.RunInteractive(renderView, refresh)
}
