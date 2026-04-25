package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	_ "modernc.org/sqlite"
)

// rpcEnv holds the initialized environment for a desktop-rpc invocation.
// Created once per command, closed after the handler returns.
type rpcEnv struct {
	ctx         context.Context
	store       *artifact.Store
	rawDB       *sql.DB
	dbStore     *db.Store
	projectRoot string
	haftDir     string
}

func (e *rpcEnv) close() {
	if e.dbStore != nil {
		_ = e.dbStore.Close()
	}
}

// initRPCEnv resolves the active project and opens the database.
// The caller must defer env.close().
func initRPCEnv() (*rpcEnv, error) {
	projectRoot := os.Getenv("HAFT_PROJECT_ROOT")
	if projectRoot == "" {
		var err error
		projectRoot, err = findProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("not a haft project: %w", err)
		}
	}

	haftDir := filepath.Join(projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return nil, fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return nil, fmt.Errorf("resolve DB path: %w", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	rawDB := database.GetRawDB()
	_, _ = rawDB.Exec("PRAGMA journal_mode=WAL")
	_, _ = rawDB.Exec("PRAGMA busy_timeout=5000")

	return &rpcEnv{
		ctx:         context.Background(),
		store:       artifact.NewStore(rawDB),
		rawDB:       rawDB,
		dbStore:     database,
		projectRoot: projectRoot,
		haftDir:     haftDir,
	}, nil
}

// rpcResult is the envelope written to stdout for every desktop-rpc call.
type rpcResult struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// writeResult marshals data as a successful rpcResult to stdout.
func writeResult(w io.Writer, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return writeError(w, fmt.Errorf("marshal result: %w", err))
	}
	return json.NewEncoder(w).Encode(rpcResult{OK: true, Data: raw})
}

// writeError writes an error rpcResult to stdout.
func writeError(w io.Writer, err error) error {
	return json.NewEncoder(w).Encode(rpcResult{OK: false, Error: err.Error()})
}

// readInput decodes JSON from stdin into the target.
func readInput(target any) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return nil // no input is valid for parameterless commands
	}
	return json.Unmarshal(data, target)
}

// makeRPCCommand creates a cobra subcommand that reads JSON from stdin,
// calls the handler, and writes JSON to stdout.
func makeRPCCommand(use, short string, handler func(*rpcEnv, io.Writer) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := initRPCEnv()
			if err != nil {
				return writeError(cmd.OutOrStdout(), err)
			}
			defer env.close()

			if err := handler(env, cmd.OutOrStdout()); err != nil {
				return writeError(cmd.OutOrStdout(), err)
			}
			return nil
		},
	}
}

// makeRawRPCCommand creates a desktop-rpc subcommand that does not initialize
// a project database. Use it for project discovery/readiness calls that must
// also classify missing or uninitialized paths.
func makeRawRPCCommand(use, short string, handler func(io.Writer) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := handler(cmd.OutOrStdout()); err != nil {
				return writeError(cmd.OutOrStdout(), err)
			}
			return nil
		},
	}
}

var desktopRPCCmd = &cobra.Command{
	Use:    "desktop-rpc",
	Short:  "Desktop RPC bridge — JSON stdin/stdout dispatch to Go domain functions",
	Hidden: true,
}

func init() {
	desktopRPCCmd.AddCommand(
		// Artifact authoring
		makeRPCCommand("create-problem", "Frame a new problem", handleCreateProblem),
		makeRPCCommand("create-decision", "Create a decision record", handleCreateDecision),
		makeRPCCommand("create-portfolio", "Explore solution variants", handleCreatePortfolio),
		makeRPCCommand("characterize", "Add comparison dimensions to a problem", handleCharacterize),
		makeRPCCommand("compare-portfolio", "Run fair comparison of variants", handleComparePortfolio),

		// Decision lifecycle
		makeRPCCommand("implement-decision", "Generate implementation brief", handleImplementDecision),
		makeRPCCommand("verify-decision", "Verify decision invariants", handleVerifyDecision),
		makeRPCCommand("baseline", "Snapshot affected files for drift detection", handleBaseline),
		makeRPCCommand("measure", "Record impact measurement", handleMeasure),

		// Artifact lifecycle
		makeRPCCommand("waive", "Extend artifact validity", handleWaive),
		makeRPCCommand("deprecate", "Archive artifact as no longer relevant", handleDeprecate),
		makeRPCCommand("reopen", "Reopen a failed decision as a new problem", handleReopen),

		// Problem candidates
		makeRPCCommand("adopt-candidate", "Adopt a governance problem candidate", handleAdoptCandidate),
		makeRPCCommand("dismiss-candidate", "Dismiss a governance problem candidate", handleDismissCandidate),

		// Flow management
		makeRPCCommand("create-flow", "Create an automated flow", handleCreateFlow),
		makeRPCCommand("update-flow", "Update an existing flow", handleUpdateFlow),
		makeRPCCommand("toggle-flow", "Enable or disable a flow", handleToggleFlow),
		makeRPCCommand("delete-flow", "Delete a flow", handleDeleteFlow),
		makeRPCCommand("run-flow-now", "Trigger a flow immediately", handleRunFlowNow),

		// Harness operator
		makeRPCCommand("list-commissions", "List WorkCommissions for the harness operator", handleListCommissions),
		makeRPCCommand("show-commission", "Show one WorkCommission", handleShowCommission),
		makeRPCCommand("requeue-commission", "Return a WorkCommission to the queue", handleRequeueCommission),
		makeRPCCommand("cancel-commission", "Cancel an unfinished WorkCommission", handleCancelCommission),
		makeRPCCommand("harness-result", "Inspect a harness run result", handleHarnessResult),
		makeRPCCommand("harness-apply", "Apply a completed harness workspace diff", handleHarnessApply),

		// Project management
		makeRawRPCCommand("project-readiness", "Inspect project readiness without opening the DB", handleProjectReadiness),
		makeRawRPCCommand("spec-check", "Run deterministic project spec checks", handleSpecCheck),
		makeRPCCommand("switch-project", "Switch active project", handleSwitchProject),
		makeRPCCommand("add-project", "Register a project by path", handleAddProject),
		makeRPCCommand("add-project-smart", "Register or initialize a project by path", handleAddProjectSmart),
		makeRPCCommand("init-project", "Initialize a new haft project", handleInitProject),

		// Governance & analysis
		makeRPCCommand("refresh-governance", "Scan for stale artifacts and drift", handleRefreshGovernance),
		makeRPCCommand("get-governance-overview", "Get governance overview", handleGetGovernanceOverview),
		makeRPCCommand("get-coverage", "Get module decision coverage", handleGetCoverage),
		makeRPCCommand("assess-readiness", "Assess portfolio readiness", handleAssessReadiness),

		// Agents & external
		makeRPCCommand("detect-agents", "Detect installed coding agents", handleDetectAgents),
		makeRPCCommand("create-pull-request", "Create a pull request from a decision branch", handleCreatePullRequest),
		makeRPCCommand("persist-task", "Persist a desktop agent task snapshot", handlePersistTask),
	)

	rootCmd.AddCommand(desktopRPCCmd)
}
