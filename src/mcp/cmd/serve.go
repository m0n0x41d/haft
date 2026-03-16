package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/quint-code/db"
	"github.com/m0n0x41d/quint-code/internal/artifact"
	"github.com/m0n0x41d/quint-code/internal/fpf"
	"github.com/m0n0x41d/quint-code/logger"

	"github.com/spf13/cobra"
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

	if err := logger.Init(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize logger: %v\n", err)
	}
	defer logger.Close()

	quintDir := filepath.Join(cwd, ".quint")
	dbPath := filepath.Join(quintDir, "quint.db")

	var database *db.Store
	if _, err := os.Stat(dbPath); err == nil {
		database, err = db.NewStore(dbPath)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to open database")
		}
	}

	var rawDB *sql.DB
	if database != nil {
		rawDB = database.GetRawDB()
	}

	fsm, err := fpf.LoadState("default", rawDB)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	tools := fpf.NewTools(fsm, cwd, database)
	server := fpf.NewServer(tools)

	// Wire v5 artifact handler
	if rawDB != nil {
		artStore := artifact.NewStore(rawDB)
		server.SetV5Handler(makeV5Handler(artStore, quintDir))
	}

	server.Start()

	return nil
}

func makeV5Handler(store *artifact.Store, quintDir string) fpf.V5ToolHandler {
	return func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		switch params.Name {
		case "quint_note":
			return handleQuintNote(ctx, store, quintDir, params.Arguments)
		default:
			return "", fmt.Errorf("unknown tool: %s", params.Name)
		}
	}
}

func handleQuintNote(ctx context.Context, store *artifact.Store, quintDir string, args map[string]interface{}) (string, error) {
	input := artifact.NoteInput{}

	if v, ok := args["title"].(string); ok {
		input.Title = v
	}
	if v, ok := args["rationale"].(string); ok {
		input.Rationale = v
	}
	if v, ok := args["evidence"].(string); ok {
		input.Evidence = v
	}
	if v, ok := args["context"].(string); ok {
		input.Context = v
	}
	if files, ok := args["affected_files"].([]interface{}); ok {
		for _, f := range files {
			if s, ok := f.(string); ok {
				input.AffectedFiles = append(input.AffectedFiles, s)
			}
		}
	}

	// Validate
	validation := artifact.ValidateNote(ctx, store, input)

	navStrip := artifact.BuildNavStrip(ctx, store, input.Context)

	if !validation.OK {
		return artifact.FormatNoteRejection(validation, navStrip), nil
	}

	// Create
	a, filePath, err := artifact.CreateNote(ctx, store, quintDir, input)
	if err != nil {
		return "", err
	}

	return artifact.FormatNoteResponse(a, filePath, validation, navStrip), nil
}
