package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/agentloop"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/config"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/hooks"
	"github.com/m0n0x41d/haft/internal/jsonrpc"
	"github.com/m0n0x41d/haft/internal/lsp"
	"github.com/m0n0x41d/haft/internal/present"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/protocol"
	"github.com/m0n0x41d/haft/internal/provider"
	"github.com/m0n0x41d/haft/internal/session"
	"github.com/m0n0x41d/haft/internal/tasks"
	"github.com/m0n0x41d/haft/internal/tools"
	"github.com/m0n0x41d/haft/internal/ui"
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

	// 3. Open DB through the canonical migrated store path.
	dbPath, err := projCfg.DBPath()
	if err != nil {
		return fmt.Errorf("get DB path: %w", err)
	}
	database, err := db.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}
	defer database.Close()
	if err := database.GetRawDB().Ping(); err != nil {
		return fmt.Errorf("ping DB: %w", err)
	}

	// 4. Create session store (runs agent-specific migrations)
	store, err := session.NewSQLiteStore(database.GetRawDB())
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
	artStore := artifact.NewStore(database.GetRawDB())
	toolRegistry.Register(tools.NewHaftProblemTool(artStore, haftDir))
	toolRegistry.Register(tools.NewHaftSolutionTool(artStore, haftDir, toolRegistry))
	toolRegistry.Register(tools.NewHaftDecisionTool(artStore, haftDir, projectRoot, toolRegistry))
	toolRegistry.Register(tools.NewHaftQueryTool(artStore, buildFPFSearchFunc()))
	toolRegistry.Register(tools.NewHaftRefreshTool(artStore, haftDir, projectRoot))
	toolRegistry.Register(tools.NewHaftNoteTool(artStore, haftDir))

	// 7. Spawn TUI process + create JSON-RPC server
	tuiCmd, tuiStdin, tuiStdout, err := spawnTUI(projectRoot)
	if err != nil {
		return fmt.Errorf("spawn TUI: %w", err)
	}

	// Disable terminal echo on the Go process's stdin while the TUI runs.
	// TUI enables mouse tracking — without disabling echo, the terminal driver
	// echoes SGR mouse sequences as garbage on screen.
	// Only touch ECHO flag — leave OPOST, ISIG, ICANON etc. intact.
	if restoreEcho, err := disableEcho(os.Stdin.Fd()); err == nil {
		defer restoreEcho()
	}

	defer func() {
		_ = tuiStdin.Close()
		_ = tuiCmd.Wait()
	}()
	rpc := jsonrpc.NewServer(tuiStdout, tuiStdin)
	bus := protocol.NewBus(rpc)

	// 7.5. Wire LSP tools
	lspManager := lsp.NewManager(projectRoot, lsp.DefaultConfigs())
	lspManager.SetCallback(func(name string, state lsp.ServerState, counts lsp.DiagnosticCounts) {
		states := lspManager.ServerStates()
		servers := make(map[string]string, len(states))
		for n, s := range states {
			servers[n] = s.String()
		}
		_ = bus.SendLSPUpdate(protocol.LSPUpdate{
			Servers:  servers,
			Errors:   counts.Error,
			Warnings: counts.Warning,
		})
	})
	toolRegistry.Register(tools.NewLSPDiagnosticsTool(lspManager, projectRoot))
	toolRegistry.Register(tools.NewLSPReferencesTool(lspManager, projectRoot))
	toolRegistry.Register(tools.NewLSPRestartTool(lspManager))

	// 8. Build system prompt
	cwd, _ := os.Getwd()
	systemPrompt := agent.BuildSystemPrompt(agent.PromptConfig{
		ProjectRoot: projectRoot,
		Cwd:         cwd,
		Lemniscate:  true,
	}) + agent.LoadProjectContext(projectRoot)

	// 9. Agent definition
	agentDef := agent.HaftAgent()

	// 10. Create session
	sess := &agent.Session{
		ID:        uuid.NewString(),
		Title:     "",
		Model:     modelToUse,
		Depth:     agent.DepthStandard,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	sess.SetExecutionMode(agent.ExecutionModeCheckpointed)
	if err := store.Create(cmd.Context(), sess); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// 10.5. Per-session logger
	_ = logger.InitSession(sess.ID)
	defer logger.Close()
	logger.Info().
		Str("component", "agent").
		Str("session_id", sess.ID).
		Str("model", modelToUse).
		Str("project", projectRoot).
		Msg("agent.session_start")

	// 11. Coordinator
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

	// 12. Subagent tool
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
		result := <-handle.Result
		if result.Error != nil {
			return "", result.Error
		}
		return result.Summary, nil
	}
	toolRegistry.Register(tools.NewSpawnAgentTool(spawnAndWaitFn))

	// 12.5. Plan mode tools (need coordinator as PlanModeController)
	toolRegistry.Register(tools.NewEnterPlanModeTool(coordinator))
	toolRegistry.Register(tools.NewExitPlanModeTool(coordinator))

	// 12.6. AskUserQuestion tool (blocks until user responds via TUI)
	toolRegistry.Register(tools.NewAskUserQuestionTool(bus))

	// 12.7. Worktree isolation tools
	worktreeState := tools.NewWorktreeState(projectRoot)
	toolRegistry.Register(tools.NewEnterWorktreeTool(worktreeState))
	toolRegistry.Register(tools.NewExitWorktreeTool(worktreeState))

	// 12.8. Web search (requires BRAVE_SEARCH_API_KEY)
	toolRegistry.Register(tools.NewWebSearchTool(os.Getenv("BRAVE_SEARCH_API_KEY")))

	// 12.9. Tool search (deferred loading — no deferred tools yet, placeholder)
	toolRegistry.Register(tools.NewToolSearchTool(toolRegistry, map[string]agent.ToolSchema{}))

	// 12.10. Task tools (background jobs)
	taskMgr := tasks.NewManager(projectRoot)
	toolRegistry.Register(tools.NewTaskCreateTool(taskMgr))
	toolRegistry.Register(tools.NewTaskGetTool(taskMgr))
	toolRegistry.Register(tools.NewTaskListTool(taskMgr))
	toolRegistry.Register(tools.NewTaskStopTool(taskMgr))
	toolRegistry.Register(tools.NewTaskUpdateTool(taskMgr))
	toolRegistry.Register(tools.NewTaskOutputTool(taskMgr))

	// 12.11. Hook executor (pre/post tool hooks from .haft/hooks.yaml)
	coordinator.Hooks = hooks.NewExecutor(projectRoot)

	// 13. Initial goal from args
	var initialGoal string
	if len(args) > 0 {
		initialGoal = strings.Join(args, " ")
	}

	// 14. Start overseer
	overseerCtx, overseerCancel := context.WithCancel(cmd.Context())
	defer overseerCancel()
	coordinator.OverseerAlerts = make(chan []string, 4)
	overseer := &agentloop.Overseer{
		ArtifactStore:   artStore,
		Cycles:          store,
		Bus:             bus,
		CoordinatorChan: coordinator.OverseerAlerts,
		SessionID:       sess.ID,
		ProjectRoot:     projectRoot,
	}
	go overseer.Run(overseerCtx)

	// 15. Run JSON-RPC server (blocks until stdin EOF)
	defer lspManager.StopAll(cmd.Context())

	agentCtx, agentCancel := context.WithCancel(cmd.Context())
	defer agentCancel()

	// Handle incoming messages from the TUI
	rpc.SetHandler(func(msg jsonrpc.Message) {
		switch msg.Method {
		case protocol.MethodSubmit:
			var sub protocol.Submit
			if err := msg.UnmarshalParams(&sub); err != nil {
				return
			}
			parts := []agent.Part{agent.TextPart{Text: sub.Text}}
			// Read attachments and include as ImagePart or TextPart
			for _, att := range sub.Attachments {
				if att.IsImage {
					data, err := os.ReadFile(att.Path)
					if err != nil {
						logger.Warn().Str("component", "agent").Str("path", att.Path).Err(err).Msg("agent.attachment_read_error")
						continue
					}
					mime := att.MIMEType
					if mime == "" {
						mime = "image/png"
					}
					parts = append(parts, agent.ImagePart{
						Filename: att.Name,
						MIMEType: mime,
						Data:     data,
					})
				} else if att.Content != "" {
					parts = append(parts, agent.TextPart{Text: persistedFileAttachmentText(att.Name, att.Content)})
				} else if att.Path != "" {
					data, err := os.ReadFile(att.Path)
					if err == nil {
						parts = append(parts, agent.TextPart{Text: persistedFileAttachmentText(att.Name, string(data))})
					}
				}
			}
			go coordinator.Run(agentCtx, sess, parts)

		case protocol.MethodCancel:
			agentCancel()
			agentCtx, agentCancel = context.WithCancel(cmd.Context())

		case protocol.MethodCompact:
			if msg.ID != nil {
				before, after, err := coordinator.ForceCompact(agentCtx, sess)
				if err != nil {
					_ = rpc.RespondError(*msg.ID, -1, err.Error())
				} else {
					_ = rpc.Respond(*msg.ID, protocol.CompactResponse{Before: before, After: after})
				}
			}

		case protocol.MethodBoard:
			if msg.ID != nil {
				var params struct {
					View string `json:"view"`
				}
				_ = msg.UnmarshalParams(&params)
				boardData, err := ui.LoadBoardData(artStore, database.GetRawDB(), projCfg.Name, projectRoot)
				if err != nil {
					_ = rpc.RespondError(*msg.ID, -1, err.Error())
				} else {
					var text string
					switch params.View {
					case "decisions":
						text = present.BoardDecisionsW(boardData, 0)
					case "problems":
						text = present.BoardProblemsW(boardData, 0)
					case "coverage":
						text = present.BoardCoverageW(boardData, 0)
					case "evidence":
						text = present.BoardEvidenceW(boardData, 0)
					default:
						text = present.BoardOverviewW(boardData, 0)
					}
					_ = rpc.Respond(*msg.ID, map[string]string{"text": text})
				}
			}

		case protocol.MethodAutonomyToggle:
			var toggle protocol.ModeUpdate
			if err := msg.UnmarshalParams(&toggle); err != nil {
				return
			}
			if applyModeUpdate(sess, toggle) {
				_ = store.Update(cmd.Context(), sess)
			}

		case protocol.MethodYoloToggle:
			var toggle protocol.ModeUpdate
			if err := msg.UnmarshalParams(&toggle); err != nil {
				return
			}
			sess.Yolo = toggle.Yolo
			_ = store.Update(cmd.Context(), sess)

		case protocol.MethodSessionList:
			if msg.ID != nil {
				go func() {
					sessions, err := store.ListRecent(cmd.Context(), 20)
					if err != nil {
						_ = rpc.RespondError(*msg.ID, -1, err.Error())
						return
					}
					var infos []protocol.SessionInfo
					for _, s := range sessions {
						if s.ID == sess.ID {
							continue
						}
						infos = append(infos, sessionInfo(&s))
					}
					_ = rpc.Respond(*msg.ID, protocol.SessionListResponse{Sessions: infos})
				}()
			}

		case protocol.MethodSessionResume:
			if msg.ID != nil {
				var req protocol.SessionResumeRequest
				if err := msg.UnmarshalParams(&req); err != nil {
					_ = rpc.RespondError(*msg.ID, -1, err.Error())
					return
				}
				go func() {
					oldSess, err := store.Get(cmd.Context(), req.SessionID)
					if err != nil {
						_ = rpc.RespondError(*msg.ID, -1, err.Error())
						return
					}
					msgs, err := store.ListBySession(cmd.Context(), req.SessionID)
					if err != nil {
						_ = rpc.RespondError(*msg.ID, -1, err.Error())
						return
					}
					sess = oldSess
					_ = rpc.Respond(*msg.ID, protocol.SessionResumeResponse{
						Session:  sessionInfo(sess),
						Messages: msgsToMsgInfos(msgs),
					})
				}()
			}

		case protocol.MethodModelList:
			if msg.ID != nil {
				registry := provider.DefaultRegistry()
				var models []protocol.ModelInfo
				for _, p := range registry.Providers() {
					for _, m := range p.Models {
						models = append(models, protocol.ModelInfo{
							ID: m.ID, Name: m.Name, Provider: p.ID,
							ContextWindow: m.ContextWindow, CanReason: m.CanReason,
						})
					}
				}
				_ = rpc.Respond(*msg.ID, protocol.ModelListResponse{Models: models})
			}

		case protocol.MethodModelSwitch:
			if msg.ID != nil {
				var req protocol.ModelSwitchRequest
				if err := msg.UnmarshalParams(&req); err != nil {
					_ = rpc.RespondError(*msg.ID, -1, err.Error())
					return
				}
				go func() {
					providerID := req.Provider
					if providerID == "" {
						providerID = config.ProviderForModel(req.Model)
					}
					apiKey := req.APIKey
					if apiKey == "" {
						if c, err := config.Load(); err == nil {
							apiKey = c.GetAuth(providerID).APIKey
						}
					}
					newLLM, err := provider.NewProvider(providerID, req.Model, apiKey)
					if err != nil {
						_ = rpc.RespondError(*msg.ID, -1, err.Error())
						return
					}
					coordinator.Provider = newLLM
					sess.Model = req.Model
					_ = rpc.Respond(*msg.ID, protocol.ModelSwitchResponse{OK: true})
				}()
			}

		case protocol.MethodFileList:
			if msg.ID != nil {
				go func() {
					var req protocol.FileListRequest
					_ = msg.UnmarshalParams(&req)
					limit := req.Limit
					if limit <= 0 {
						limit = 200
					}
					files := scanProjectFiles(projectRoot, limit)
					_ = rpc.Respond(*msg.ID, protocol.FileListResponse{Files: files})
				}()
			}

		case protocol.MethodResize:
			// Backend receives terminal size — used for prompt width hints
			var r protocol.Resize
			_ = msg.UnmarshalParams(&r)
			// Store for future use (prompt formatting)
			_ = r
		}
	})

	// Send init event
	_ = bus.SendInit(protocol.Init{
		Session:     sessionInfo(sess),
		ProjectRoot: projectRoot,
	})

	// If initial goal provided, run it immediately
	if initialGoal != "" {
		go coordinator.Run(agentCtx, sess, []agent.Part{agent.TextPart{Text: initialGoal}})
	}

	// Block on the read loop (exits on stdin close / EOF)
	if err := rpc.ReadLoop(); err != nil {
		return fmt.Errorf("rpc read loop: %w", err)
	}
	return nil
}

// buildFPFSearchFunc creates a callback for HaftQueryTool's "fpf" action.
// Uses the embedded FPF spec database (same as `haft fpf search`).
func buildFPFSearchFunc() tools.FPFSearchFunc {
	return func(request tools.FPFSearchRequest) (string, error) {
		retrieval, err := retrieveEmbeddedFPF(fpf.SpecRetrievalRequest{
			Query: request.Query,
			Limit: request.Limit,
			Full:  request.Full,
			Mode:  request.Mode,
		})
		if err != nil {
			return "", err
		}

		return formatAgentFPFSearchWithExplain(retrieval.Query, presentFPFRetrieval(retrieval.Results), request.Explain), nil
	}
}
