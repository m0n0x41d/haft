package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/m0n0x41d/quint-code/internal/artifact"
	"github.com/m0n0x41d/quint-code/internal/project"
	"github.com/m0n0x41d/quint-code/internal/ui"
)

var checkMode bool

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Interactive dashboard — decision health, coverage, drift, problems",
	Long: `Launch the Quint Code dashboard.

Shows decision health, module coverage, drift alerts, problem pipeline,
and evidence quality in an interactive terminal UI.

Navigation: tab/1-4 switch views, j/k navigate, enter drill in, esc back, q quit.

Use --check for CI/hooks: exits with code 1 if critical issues exist
(R_eff < 0.3, decisions expired > 30 days).`,
	RunE: runBoard,
}

func init() {
	boardCmd.Flags().BoolVar(&checkMode, "check", false, "Health check mode: print summary and exit with code 1 if critical issues")
	rootCmd.AddCommand(boardCmd)
}

func runBoard(cmd *cobra.Command, _ []string) error {
	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a quint-code project (no .quint/ directory found): %w", err)
	}

	quintDir := filepath.Join(projectRoot, ".quint")

	// Load project config
	projCfg, err := project.Load(quintDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project not initialized — run 'quint-code init' first")
	}

	// Open DB
	dbPath, err := projCfg.DBPath()
	if err != nil {
		return fmt.Errorf("get DB path: %w", err)
	}

	// Open DB with WAL mode and busy timeout.
	// MCP server may hold a write connection to the same DB —
	// WAL allows concurrent readers, busy timeout prevents instant SQLITE_BUSY.
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}
	defer sqlDB.Close()

	store := artifact.NewStore(sqlDB)
	projectName := projCfg.Name

	// Load all data
	data, err := ui.LoadBoardData(store, sqlDB, projectName, projectRoot)
	if err != nil {
		return fmt.Errorf("load board data: %w", err)
	}

	// Check mode: print summary and exit
	if checkMode {
		return runCheck(data)
	}

	// Interactive mode
	model := ui.New(data, store, sqlDB, projectName, projectRoot)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("board: %w", err)
	}

	// Interactive mode always exits 0 — user saw the dashboard.
	// Use --check for non-zero exit on critical issues.
	_ = finalModel
	return nil
}

func runCheck(data *ui.BoardData) error {
	fmt.Printf("Quint Code Health: %s\n", data.ProjectName)
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

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".quint")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .quint/ found")
		}
		dir = parent
	}
}
