package cli

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

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/agentloop"
	"github.com/m0n0x41d/haft/internal/config"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/lsp"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/provider"
	"github.com/m0n0x41d/haft/internal/session"
	"github.com/m0n0x41d/haft/internal/tools"
	"github.com/m0n0x41d/haft/internal/tui"
	"github.com/m0n0x41d/haft/logger"
)

var agentModel string

var agentCmd = &cobra.Command{
	Use:   "agent [goal]",
	Short: "Interactive AI coding agent",
	Long: `Launch the Haft interactive agent.

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
	agentCmd.Flags().StringVar(&agentModel, "model", "", "LLM model to use (overrides config)")
	rootCmd.AddCommand(agentCmd)
}

// knownSubcommands lists registered subcommand names to prevent misrouting.
// If user types "haft status" and status isn't a subcommand, don't silently
// run the agent with "status" as a goal — show a helpful error instead.
var knownSubcommands = map[string]bool{
	"status": true, "search": true, "problems": true, "refresh": true,
	"frame": true, "explore": true, "decide": true, "measure": true,
	"note": true, "compare": true, "char": true, "reason": true,
}

func runAgent(cmd *cobra.Command, args []string) error {
	// Guard: catch single-word args that look like mistyped subcommands
	if len(args) == 1 {
		if knownSubcommands[args[0]] {
			return fmt.Errorf("%q is a slash command, not a CLI subcommand. Run 'haft' and type /%s inside the agent", args[0], args[0])
		}
	}

	// 0. Ensure configured (run setup on first launch)
	cfg, err := ensureConfigured()
	if err != nil {
		return err
	}

	// CLI flag overrides config
	modelToUse := cfg.Model
	if agentModel != "" {
		modelToUse = agentModel
	}
	if modelToUse == "" {
		modelToUse = "gpt-5.4" // ultimate fallback
	}

	// 1. Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project (no .haft/ directory found): %w", err)
	}

	haftDir := filepath.Join(projectRoot, ".haft")

	// 2. Load project config
	projCfg, err := project.Load(haftDir)
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
	// Use config's configured providers first, fall back to model-name guessing
	providerID := ""
	for _, pid := range cfg.ConfiguredProviders() {
		if config.ProviderForModel(modelToUse) == pid {
			providerID = pid
			break
		}
	}
	if providerID == "" {
		providerID = config.ProviderForModel(modelToUse)
	}
	if providerID == "" {
		providerID = "openai" // ultimate fallback
	}
	auth := cfg.GetAuth(providerID)
	llm, err := provider.NewProvider(providerID, modelToUse, auth.APIKey)
	if err != nil {
		return fmt.Errorf("init provider: %w", err)
	}

	// 6. Create tool registry with builtin + haft kernel tools
	toolRegistry := tools.NewRegistry(projectRoot)

	// Wire haft kernel tools — same core as MCP server, different transport
	artStore := artifact.NewStore(sqlDB)
	toolRegistry.Register(tools.NewHaftProblemTool(artStore, haftDir))
	toolRegistry.Register(tools.NewHaftSolutionTool(artStore, haftDir, toolRegistry))
	toolRegistry.Register(tools.NewHaftDecisionTool(artStore, haftDir, projectRoot, toolRegistry))
	toolRegistry.Register(tools.NewHaftQueryTool(artStore, buildFPFSearchFunc()))
	toolRegistry.Register(tools.NewHaftRefreshTool(artStore, haftDir, projectRoot))
	toolRegistry.Register(tools.NewHaftNoteTool(artStore, haftDir))

	// 7. Create event bus (before LSP — callback needs bus)
	bus := tui.NewBus(256)

	// 6.5. Wire LSP tools (language server diagnostics + references)
	lspManager := lsp.NewManager(projectRoot, lsp.DefaultConfigs())
	lspManager.SetCallback(func(name string, state lsp.ServerState, counts lsp.DiagnosticCounts) {
		states := lspManager.ServerStates()
		servers := make(map[string]string, len(states))
		for n, s := range states {
			servers[n] = s.String()
		}
		bus.Send(tui.LSPUpdateMsg{
			Servers:  servers,
			Errors:   counts.Error,
			Warnings: counts.Warning,
		})
	})
	toolRegistry.Register(tools.NewLSPDiagnosticsTool(lspManager, projectRoot))
	toolRegistry.Register(tools.NewLSPReferencesTool(lspManager, projectRoot))
	toolRegistry.Register(tools.NewLSPRestartTool(lspManager))

	// 8. Build system prompt with project context (repo map injected lazily by coordinator)
	cwd, _ := os.Getwd()
	systemPrompt := agent.BuildSystemPrompt(agent.PromptConfig{
		ProjectRoot: projectRoot,
		Cwd:         cwd,
		Lemniscate:  true,
	}) + agent.LoadProjectContext(projectRoot)

	// 9. Create coordinator with lemniscate agent
	agentDef := agent.HaftAgent()

	// 10. Create session (before coordinator — spawn callbacks need sess)
	sess := &agent.Session{
		ID:          uuid.NewString(),
		Title:       "",
		Model:       modelToUse,
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
		Str("model", modelToUse).
		Str("project", projectRoot).
		Msg("agent.session_start")

	// 11. Create coordinator with lemniscate agent
	coordinator := &agentloop.Coordinator{
		Provider:      llm,
		Tools:         toolRegistry,
		Sessions:      store,
		Messages:      store,
		Cycles:        store,
		ArtifactStore: artStore,
		Bus:           bus,
		SystemPrompt:  systemPrompt,
		AgentDef:      agentDef,
		Subagents:     agentloop.NewSubagentTracker(),
		ProjectRoot:   projectRoot,
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
	runFn := func(ctx context.Context, s *agent.Session, parts []agent.Part) {
		coordinator.Run(ctx, s, parts)
	}
	compactFn := func(ctx context.Context, s *agent.Session) (int, int, error) {
		return coordinator.ForceCompact(ctx, s)
	}
	tuiModel := tui.New(sess, runFn, bus, initialGoal, store, store, compactFn, store, projectRoot)

	// Wire model switch callback — swaps provider mid-session on Ctrl+M
	tuiModel.SetModelSwitchFn(func(msg tui.ModelSwitchMsg) {
		switchAPIKey := msg.APIKey
		if switchAPIKey == "" {
			switchAPIKey = cfg.GetAuth(msg.Provider).APIKey
		}
		newProvider, err := provider.NewProvider(msg.Provider, msg.Model, switchAPIKey)
		if err != nil {
			bus.Send(tui.ErrorMsg{Err: fmt.Errorf("switch model: %w", err)})
			return
		}
		coordinator.Provider = newProvider
		sess.Model = msg.Model

		// Save to config if new API key was entered
		if msg.APIKey != "" {
			cfg, _ := config.Load()
			cfg.SetAuth(msg.Provider, config.ProviderAuth{
				AuthType: "api_key",
				APIKey:   msg.APIKey,
			})
			cfg.Model = msg.Model
			_ = config.Save(cfg)
		}

		logger.Info().Str("component", "agent").
			Str("model", msg.Model).
			Str("provider", msg.Provider).
			Msg("agent.model_switched")
	})

	// 14. Start overseer (background health monitor)
	overseerCtx, overseerCancel := context.WithCancel(cmd.Context())
	defer overseerCancel()
	coordinator.OverseerAlerts = make(chan []string, 4) // buffered, non-blocking writes
	overseer := &agentloop.Overseer{
		ArtifactStore:   artStore,
		Cycles:          store,
		Bus:             bus,
		CoordinatorChan: coordinator.OverseerAlerts,
		SessionID:       sess.ID,
		ProjectRoot:     projectRoot,
	}
	go overseer.Run(overseerCtx)

	// 15. Run TUI
	defer lspManager.StopAll(cmd.Context())
	p := tea.NewProgram(tuiModel, tea.WithEnvironment(bubbleTeaEnv()))
	_, err = p.Run()
	if err != nil {
		return fmt.Errorf("agent TUI: %w", err)
	}

	return nil
}

// buildFPFSearchFunc creates a callback for HaftQueryTool's "fpf" action.
// Uses the embedded FPF spec database (same as `haft fpf search`).
func buildFPFSearchFunc() tools.FPFSearchFunc {
	return func(query string, limit int) (string, error) {
		db, cleanup, err := openFPFDB()
		if err != nil {
			return "", fmt.Errorf("open fpf db: %w", err)
		}
		defer cleanup()

		results, err := fpf.SearchSpec(db, query, limit)
		if err != nil {
			return "", err
		}
		if len(results) == 0 {
			return "No FPF spec matches for: " + query, nil
		}

		var b strings.Builder
		for _, r := range results {
			fmt.Fprintf(&b, "### %s\n%s\n\n", r.Heading, r.Snippet)
		}
		return b.String(), nil
	}
}
