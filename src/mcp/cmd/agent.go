package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/m0n0x41d/quint-code/internal/agent"
	"github.com/m0n0x41d/quint-code/internal/agentloop"
	"github.com/m0n0x41d/quint-code/internal/artifact"
	"github.com/m0n0x41d/quint-code/internal/codebase"
	"github.com/m0n0x41d/quint-code/internal/project"
	"github.com/m0n0x41d/quint-code/internal/provider"
	"github.com/m0n0x41d/quint-code/internal/session"
	"github.com/m0n0x41d/quint-code/internal/tools"
	"github.com/m0n0x41d/quint-code/internal/tui"
	"github.com/m0n0x41d/quint-code/logger"
)

var agentModel string

var agentCmd = &cobra.Command{
	Use:   "agent [goal]",
	Short: "Interactive AI coding agent",
	Long: `Launch the Quint Code interactive agent.

The agent connects to an LLM (OpenAI by default), streams responses,
and executes tools (bash, read, write, edit, glob, grep) with permission prompts.

Set OPENAI_API_KEY or login via Codex CLI for authentication.

Examples:
  haft agent
  haft agent "fix the failing tests in src/auth"
  haft agent --model gpt-4o-mini "list files"`,
	RunE: runAgent,
}

func init() {
	agentCmd.Flags().StringVar(&agentModel, "model", "gpt-5.4", "LLM model to use")
	rootCmd.AddCommand(agentCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	// 1. Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project (no .quint/ directory found): %w", err)
	}

	quintDir := filepath.Join(projectRoot, ".quint")

	// Logger initialized after session creation (per-session log file)

	// 2. Load project config
	projCfg, err := project.Load(quintDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project not initialized — run 'haft init' first")
	}

	// 3. Open DB (WAL + busy timeout, same as board.go)
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

	// 4. Create session store (runs migrations)
	store, err := session.NewSQLiteStore(sqlDB)
	if err != nil {
		return fmt.Errorf("init session store: %w", err)
	}

	// 5. Create LLM provider
	llm, err := provider.NewOpenAI(agentModel)
	if err != nil {
		return fmt.Errorf("init provider: %w", err)
	}

	// 6. Create tool registry with builtin + quint kernel tools
	toolRegistry := tools.NewRegistry(projectRoot)

	// Wire quint kernel tools — same core as MCP server, different transport
	artStore := artifact.NewStore(sqlDB)
	toolRegistry.Register(tools.NewQuintProblemTool(artStore, quintDir))
	toolRegistry.Register(tools.NewQuintSolutionTool(artStore, quintDir))
	toolRegistry.Register(tools.NewQuintDecisionTool(artStore, quintDir))
	toolRegistry.Register(tools.NewQuintQueryTool(artStore))
	toolRegistry.Register(tools.NewQuintRefreshTool(artStore, quintDir, projectRoot))
	toolRegistry.Register(tools.NewQuintNoteTool(artStore, quintDir))

	// 7. Create event bus
	bus := tui.NewBus(256)

	// 8. Build system prompt with project context + repo map
	cwd, _ := os.Getwd()
	systemPrompt := agent.BuildSystemPrompt(projectRoot, cwd) + agent.LoadProjectContext(projectRoot)

	// Build tree-sitter repo map (symbol extraction for all supported languages)
	repoMap, err := codebase.BuildRepoMap(projectRoot, 500)
	if err == nil && repoMap != nil && repoMap.TotalFiles > 0 {
		systemPrompt += "\n\n" + codebase.RenderRepoMap(repoMap, 2000)
	}

	// 9. Create coordinator with lemniscate agent
	agentDef := agent.HaftAgent()

	// 10. Create session (before coordinator — spawn callbacks need sess)
	sess := &agent.Session{
		ID:          uuid.NewString(),
		Title:       "",
		Model:       agentModel,
		Depth:       agent.DepthStandard, // FPF B.5.1: all phases mandatory
		Interaction: agent.InteractionSymbiotic,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store.Create(cmd.Context(), sess); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// 10.5. Initialize per-session logger
	_ = logger.InitSession(sess.ID)
	defer logger.Close()
	logger.Info().
		Str("component", "agent").
		Str("session_id", sess.ID).
		Str("model", agentModel).
		Str("project", projectRoot).
		Msg("agent.session_start")

	// 11. Create coordinator with lemniscate agent
	coordinator := &agentloop.Coordinator{
		Provider:       llm,
		Tools:          toolRegistry,
		Sessions:       store,
		Messages:       store,
		Cycles:         store,
		ArtifactStore:  artStore,
		Bus:            bus,
		SystemPrompt:   systemPrompt,
		AgentDef:       agentDef,
		Subagents:      agentloop.NewSubagentTracker(),
		SessionContext: "", // project-wide NavState — artifacts scoped by problem, not session
	}

	// Register subagent tool — callback spawns AND waits (blocking, like Crush/Claude Code)
	spawnAndWaitFn := func(ctx context.Context, agentType, task, model string) (string, error) {
		def, ok := agent.SubagentDefByName(agentType)
		if !ok {
			return "", fmt.Errorf("unknown subagent type: %s", agentType)
		}
		if model != "" {
			def.Model = model
		}
		handle, err := coordinator.SpawnSubagent(ctx, sess, def, task)
		if err != nil {
			return "", err
		}
		// Block until subagent completes
		result := <-handle.Result
		if result.Error != nil {
			return "", result.Error
		}
		return result.Summary, nil
	}
	toolRegistry.Register(tools.NewSpawnAgentTool(spawnAndWaitFn))

	// 12. Get initial goal from args
	var initialGoal string
	if len(args) > 0 {
		initialGoal = strings.Join(args, " ")
	}

	// 13. Create TUI model
	runFn := func(ctx context.Context, s *agent.Session, text string) {
		coordinator.Run(ctx, s, text)
	}
	compactFn := func(ctx context.Context, s *agent.Session) (int, int, error) {
		return coordinator.ForceCompact(ctx, s)
	}
	model := tui.New(sess, runFn, bus, initialGoal, store, store, compactFn, store)

	// 14. Run TUI
	p := tea.NewProgram(model)
	_, err = p.Run()
	if err != nil {
		return fmt.Errorf("agent TUI: %w", err)
	}

	return nil
}
