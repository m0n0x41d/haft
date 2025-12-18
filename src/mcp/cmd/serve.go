package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"quint-mcp/db"
	"quint-mcp/internal/fpf"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server for AI tool integration.

The server communicates via stdio and provides FPF tools to AI assistants
like Claude Code, Cursor, Gemini CLI, and Codex CLI.

The project root is determined by:
  1. QUINT_PROJECT_ROOT environment variable (if set)
  2. Current working directory (default)`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cwd := os.Getenv("QUINT_PROJECT_ROOT")
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	quintDir := filepath.Join(cwd, ".quint")
	stateFile := filepath.Join(quintDir, "state.json")
	dbPath := filepath.Join(quintDir, "quint.db")

	var database *db.DB
	if _, err := os.Stat(dbPath); err == nil {
		database, _ = db.New(dbPath)
	}

	var rawDB *sql.DB
	if database != nil {
		rawDB = database.GetRawDB()
	}

	fsm, err := fpf.LoadState(stateFile, rawDB)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	tools := fpf.NewTools(fsm, cwd, database)
	server := fpf.NewServer(tools)
	server.Start()

	return nil
}
