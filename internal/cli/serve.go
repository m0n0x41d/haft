package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/agenthostrestart"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/ceremony"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/codeintel"
	"github.com/m0n0x41d/haft/internal/config"
	"github.com/m0n0x41d/haft/internal/contextgraph"
	"github.com/m0n0x41d/haft/internal/embedding"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/graph"
	"github.com/m0n0x41d/haft/internal/present"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/recall"
	"github.com/m0n0x41d/haft/internal/ui"
	"github.com/m0n0x41d/haft/logger"

	"github.com/spf13/cobra"
)

var (
	serveProcessStartedAt  = time.Now().UTC()
	serveProjectRoot       string
	serveExpectedProjectID string
	serveScopeID           string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server for AI tool integration.

The server communicates via stdio and provides Haft tools to embedded host
agents. Product support targets Claude Code and Codex; other MCP clients may
remain protocol-compatible experimental integrations.

The project root is determined by:
  1. --project-root flag (if set)
  2. HAFT_PROJECT_ROOT environment variable (if set)
  3. QUINT_PROJECT_ROOT legacy environment variable (if set)
  4. Current working directory (default)`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveProjectRoot, "project-root", "", "Explicit Haft project root for host-level MCP configs")
	serveCmd.Flags().StringVar(&serveExpectedProjectID, "expected-project-id", "", "Expected Haft project_id guard for explicit host-level MCP configs")
	serveCmd.Flags().StringVar(&serveScopeID, "scope-id", "", "Exact canonical project ScopeID for a mixed admitted profile")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// Ensure global ~/.haft/ exists (migrates from ~/.quint-code/ if needed)
	_ = project.EnsureDir()

	rootInput, err := projectRootInputFromExplicitOrEnv(serveProjectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root input: %w", err)
	}

	if err := logger.Init(rootInput.Path); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize logger: %v\n", err)
	}
	defer logger.Close()

	// HAFT_SERVER_ORIGIN: "local" (default) or URL for future remote server
	serverOrigin := os.Getenv("HAFT_SERVER_ORIGIN")
	if serverOrigin != "" && serverOrigin != "local" {
		logger.Info().Str("origin", serverOrigin).Msg("HAFT_SERVER_ORIGIN set to remote — not implemented yet, using local storage")
	}

	server := fpf.NewServer(Version)
	if err := configureMemoryValidation(cmd.Context(), server); err != nil {
		return fmt.Errorf("configure read-only typed-memory validation: %w", err)
	}
	onboardFallback, err := newOnboardingRequiredMCPHandler()
	if err != nil {
		return fmt.Errorf(
			"configure pre-initialization onboarding recovery: %w",
			err,
		)
	}
	server.SetOnboardHandler(onboardFallback)
	entityFallback, err := newEntityOnboardingRequiredMCPHandler(
		"Haft is not initialized for this MCP process; no entity was written. Run haft init, complete readable onboarding, and reconnect the host.",
	)
	if err != nil {
		return fmt.Errorf(
			"configure pre-onboarding EntityOfConcern recovery: %w",
			err,
		)
	}
	server.SetEntityHandler(entityFallback)
	binding, bindingErr := resolveProjectBindingFromInput(rootInput, serveExpectedProjectIDForRun())
	if bindingErr != nil {
		instructions := composeServerInstructionParts(nil, nil)
		server.SetInstructions(instructions)
		server.SetV5Handler(func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
			return "", projectBindingError(binding, bindingErr)
		})
		server.Start()
		return nil
	}

	logger.Info().
		Str("project_root", binding.ProjectRoot).
		Str("project_id", binding.ProjectID).
		Str("db_path", binding.DBPath).
		Str("db_state", binding.DBState).
		Msg("haft project binding resolved")

	workflow, workflowErr := project.LoadWorkflow(binding.ProjectRoot)
	if workflowErr != nil {
		logger.Warn().Err(workflowErr).Msg("failed to load workflow policy")
	}
	instructions := composeServerInstructionParts(workflow, nil)
	server.SetInstructions(instructions)

	if _, err := os.Stat(binding.DBPath); err != nil {
		server.SetV5Handler(func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
			return "", fmt.Errorf("haft project database is missing; run `haft init` in %s to create %s; %s", binding.ProjectRoot, binding.DBPath, formatProjectBindingDiagnostic(binding))
		})
		server.Start()
		return nil
	}

	onboardSurface, onboardSurfaceErr := openServeProjectOnboardSurface(
		cmd.Context(),
		binding,
	)
	if onboardSurfaceErr != nil {
		logger.Warn().
			Err(onboardSurfaceErr).
			Msg("project onboarding surface remains in initialization recovery")
	} else {
		server.SetOnboardHandler(onboardSurface.Handler())
		defer func() {
			if closeErr := onboardSurface.Close(); closeErr != nil {
				logger.Warn().
					Err(closeErr).
					Msg("failed to close project onboarding surface")
			}
		}()
	}

	entitySurface, entitySurfaceErr := openServeProjectEntitySurface(
		cmd.Context(),
		binding,
	)
	if entitySurfaceErr != nil {
		logger.Warn().
			Err(entitySurfaceErr).
			Msg("task-level EntityOfConcern establishment remains in onboarding recovery")
	} else {
		server.SetEntityHandler(entitySurface.Handler())
		defer func() {
			if closeErr := entitySurface.Close(); closeErr != nil {
				logger.Warn().
					Err(closeErr).
					Msg("failed to close task-level EntityOfConcern surface")
			}
		}()
	}

	scopeRequest, err := projectSpecificationScopeRequestFromFlag(serveScopeID)
	if err != nil {
		return err
	}
	profileResolution, profileErr := resolveCanonicalProjectSpecificationApplicability(
		cmd.Context(),
		binding.ProjectRoot,
		scopeRequest,
	)
	if profileErr != nil {
		logger.Warn().
			Err(profileErr).
			Msg("canonical profile applicability unavailable; SWE host doctrines omitted")
		instructions = composeServerInstructionsForUnavailableProfile(workflow)
	}
	if profileErr == nil {
		instructions, err = composeServerInstructionsForProfileResolution(
			workflow,
			profileResolution,
		)
		if err != nil {
			return fmt.Errorf(
				"compose profile-aware server instructions: %w",
				err,
			)
		}
	}
	server.SetInstructions(instructions)

	ledger, err := openServeProjectLedger(cmd.Context(), binding)
	if err != nil {
		server.SetV5Handler(func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
			return "", serveProjectLedgerError(binding, err)
		})
		server.Start()
		return nil
	}
	defer ledger.Close()

	rawDatabase := ledger.Database()
	artStore := artifact.NewStore(rawDatabase)
	codeIndexCoordinator, err := codeintel.NewProjectIndexCoordinator(
		codeintel.ProjectIndexCoordinates{
			ProjectID:   ledger.ProjectID().String(),
			ProjectRoot: ledger.ProjectRoot().String(),
			LedgerPath:  ledger.DatabasePath(),
		},
	)
	if err != nil {
		return fmt.Errorf(
			"configure checked project code-index coordinator: %w",
			err,
		)
	}
	codeIntelService := codeintel.NewServiceWithIndexCoordinator(
		artStore,
		codeIndexCoordinator,
	)

	memorySurface, memorySurfaceErr := configureServeProjectMemoryFullSurface(
		cmd.Context(),
		server,
		binding,
	)
	if memorySurfaceErr != nil {
		logger.Warn().
			Err(memorySurfaceErr).
			Msg("typed project-memory surface remains unavailable; validate-only surface retained")
	} else {
		defer memorySurface.Close()
	}

	indexStore, indexErr := project.OpenIndex()
	if indexErr != nil {
		logger.Warn().Err(indexErr).Msg("failed to open cross-project index")
	}

	_ = project.PopulateContextFacts(context.Background(), rawDatabase, binding.ProjectName)

	go func() {
		if _, err := codeIntelService.EnsureIndexForStartup(
			context.Background(),
			binding.ProjectRoot,
		); err != nil {
			logger.Warn().Err(err).Msg("code-graph startup refresh failed")
		}
	}()

	searcher := buildHybridSearcher(artStore, rawDatabase)
	crossHybrid := buildCrossProjectHybrid(indexStore)
	projCfg := &project.Config{
		ID:   binding.ProjectID,
		Name: binding.ProjectName,
	}

	taskProjector, taskProjectionErr := newTaskMemoryProjectionRuntime(
		cmd.Context(),
		ledger.ProjectID(),
		rawDatabase,
		artStore,
	)
	var taskMemorySurface taskMemoryProjector = taskProjector
	if taskProjectionErr != nil {
		logger.Warn().
			Err(taskProjectionErr).
			Msg("task-memory projection remains unavailable; legacy carriers remain writable with explicit unsettled posture")
		taskMemorySurface = newUnavailableTaskMemoryProjector(
			taskProjectionErr,
		)
	}

	v5Handler := makeV5HandlerWithTaskMemoryProjectionAndCodeIntel(
		artStore,
		searcher,
		crossHybrid,
		binding.HaftDir,
		projCfg,
		indexStore,
		taskMemorySurface,
		codeIntelService,
	)
	server.SetV5Handler(makeRevalidatedServeV5Handler(
		binding,
		ledger,
		v5Handler,
	))
	server.Start()
	return nil
}

func openServeProjectLedger(
	ctx context.Context,
	binding ProjectBinding,
) (*projectledger.Handle, error) {
	ledger, err := projectledger.OpenExisting(
		ctx,
		binding.ProjectRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		return nil, err
	}
	closeOnFailure := func(cause error) (*projectledger.Handle, error) {
		return nil, errors.Join(cause, ledger.Close())
	}
	if ledger.ProjectID().String() != binding.ProjectID {
		return closeOnFailure(fmt.Errorf(
			"checked project ledger id %q does not match resolved project id %q",
			ledger.ProjectID().String(),
			binding.ProjectID,
		))
	}
	if err := db.RequireCurrentSchemaReadOnly(ctx, ledger.Database()); err != nil {
		return closeOnFailure(err)
	}
	return ledger, nil
}

func serveProjectLedgerError(binding ProjectBinding, cause error) error {
	if errors.Is(
		cause,
		projectledger.ErrSQLiteSidecarGenerationChanged,
	) {
		return fmt.Errorf(
			"haft project database connection is stale: %w; restart the long-lived Haft MCP process so it reopens the current SQLite WAL/SHM generation; do not run `haft project migrate` for this condition; no migration or repair was attempted; %s",
			cause,
			formatProjectBindingDiagnostic(binding),
		)
	}
	migrationCommand := fmt.Sprintf(
		"haft project migrate --project-root %q --project-id %s",
		binding.ProjectRoot,
		binding.ProjectID,
	)
	return fmt.Errorf(
		"haft project database is not ready for this Haft binary: %w; run `%s` to apply the explicit host-free database upgrade, then restart the MCP server; no startup migration was attempted; %s",
		cause,
		migrationCommand,
		formatProjectBindingDiagnostic(binding),
	)
}

type serveProjectLedgerRevalidator interface {
	Revalidate(context.Context) error
}

func makeRevalidatedServeV5Handler(
	binding ProjectBinding,
	ledger serveProjectLedgerRevalidator,
	next fpf.V5ToolHandler,
) fpf.V5ToolHandler {
	return func(
		ctx context.Context,
		toolName string,
		rawParams json.RawMessage,
	) (string, error) {
		if ledger == nil || next == nil {
			return "", fmt.Errorf(
				"checked Haft MCP handler dependencies are incomplete",
			)
		}
		if err := ledger.Revalidate(ctx); err != nil {
			cause := fmt.Errorf(
				"revalidate checked project ledger before MCP tool %q: %w",
				toolName,
				err,
			)
			return "", serveProjectLedgerError(binding, cause)
		}
		result, operationErr := next(ctx, toolName, rawParams)
		revalidationErr := ledger.Revalidate(ctx)
		if revalidationErr == nil {
			return result, operationErr
		}
		cause := fmt.Errorf(
			"revalidate checked project ledger after MCP tool %q; discard any untrusted read result and treat a mutation outcome as unknown until idempotent replay: %w",
			toolName,
			revalidationErr,
		)
		presented := serveProjectLedgerError(binding, cause)
		return result, errors.Join(operationErr, presented)
	}
}

func serveExpectedProjectIDForRun() string {
	flagValue := strings.TrimSpace(serveExpectedProjectID)
	if flagValue != "" {
		return flagValue
	}

	return strings.TrimSpace(os.Getenv(envExpectedProjectID))
}

func haftServeRuntimeStatusLine() string {
	executable := "unknown"
	executableMTime := "unknown"

	if path, err := os.Executable(); err == nil {
		executable = filepath.Clean(path)
		if info, statErr := os.Stat(path); statErr == nil {
			executableMTime = info.ModTime().UTC().Format(time.RFC3339)
		}
	}

	return fmt.Sprintf(
		"### Runtime\n\n- `haft serve`: pid=%d started=%s executable=`%s` executable_mtime=%s\n",
		os.Getpid(),
		serveProcessStartedAt.Format(time.RFC3339),
		executable,
		executableMTime,
	)
}

// buildHybridSearcher wires the optional embedding layer over the artifact
// store. The embedder is resolved lazily on first search, so startup never
// blocks on a first-run model download or spawns an embedding sidecar just
// because an MCP client connected. A nil result (provider disabled or any setup
// fault) means search runs FTS5+PPR only — recall never hard-fails on the
// optional layer (dec-20260605-fe77b358).
func buildHybridSearcher(store *artifact.Store, rawDB *sql.DB) recall.Searcher {
	embCfg := embeddingConfigFromFile()
	if strings.EqualFold(strings.TrimSpace(embCfg.Provider), embedding.ProviderNone) {
		logger.Info().Msg("hybrid recall disabled by config — using FTS5+PPR")
		return nil
	}

	newEmbedder := func() (embedding.Embedder, error) {
		embedder, err := embedding.New(embCfg)
		if err != nil {
			if embedding.Degraded(err) {
				logger.Info().Err(err).Msg("embedding layer unavailable — using FTS5+PPR")
			} else {
				logger.Warn().Err(err).Msg("embedding layer setup failed — using FTS5+PPR")
			}
			return nil, err
		}
		descriptor := embedder.Descriptor()
		logger.Info().
			Str("provider", descriptor.Provider).
			Str("model", descriptor.Model).
			Int("dim", descriptor.Dimensions).
			Msg("hybrid recall enabled")
		return embedder, nil
	}

	hybrid := recall.NewHybrid(store, newEmbedder, rawDB)
	return hybrid
}

// embeddingConfigFromFile resolves the embedding provider/model/dim from
// ~/.haft/config.yaml, defaulting to the local sidecar.
func embeddingConfigFromFile() embedding.Config {
	embCfg := embedding.Config{Provider: embedding.ProviderLocal}
	if cfg, err := config.Load(); err == nil && cfg != nil {
		if cfg.Embedding.Provider != "" {
			embCfg.Provider = cfg.Embedding.Provider
		}
		embCfg.Model = cfg.Embedding.Model
		embCfg.Dim = cfg.Embedding.Dim
	}
	// Env override (wins over the config file) for artifact/cross-project recall
	// experiments. FPF spec search pins its own baked-vector contract.
	if m := strings.TrimSpace(os.Getenv("HAFT_EMBED_MODEL")); m != "" {
		embCfg.Model = m
	}
	return embCfg
}

// buildCrossProjectHybrid wires the optional embedding layer over the
// cross-project index, mirroring buildHybridSearcher. nil (provider disabled /
// no index) means cross-project recall runs the FTS path (now AND+OR) only.
// The embedder is intentionally lazy so a single MCP server does not load an
// additional ONNX model before the operator asks for cross-project recall.
func buildCrossProjectHybrid(indexStore *project.IndexStore) *project.CrossHybrid {
	if indexStore == nil {
		return nil
	}
	embCfg := embeddingConfigFromFile()
	if strings.EqualFold(strings.TrimSpace(embCfg.Provider), embedding.ProviderNone) {
		return nil
	}
	newEmbedder := func() (embedding.Embedder, error) {
		embedder, err := embedding.New(embCfg)
		if err != nil {
			if embedding.Degraded(err) {
				logger.Info().Err(err).Msg("embedding layer unavailable — cross-project recall uses FTS5")
			} else {
				logger.Warn().Err(err).Msg("embedding layer setup failed — cross-project recall uses FTS5")
			}
			return nil, err
		}
		return embedder, nil
	}
	hybrid := project.NewCrossHybrid(indexStore, newEmbedder)
	return hybrid
}

// invalidateRecall tells the hybrid searcher to rebuild its semantic index when
// a decision/note is created or updated, so it becomes searchable the same
// session. No-op for the plain FTS searcher or non-corpus artifacts.
func invalidateRecall(searcher recall.Searcher, createdRef string) {
	if createdRef == "" {
		return
	}
	if !strings.HasPrefix(createdRef, "dec-") && !strings.HasPrefix(createdRef, "note-") {
		return
	}
	if invalidator, ok := searcher.(interface{ Invalidate() }); ok {
		invalidator.Invalidate()
	}
}

// searchArtifacts routes a query through the hybrid searcher when one is wired,
// and falls back to the store's FTS ranking otherwise. Same contract either way.
func searchArtifacts(ctx context.Context, store *artifact.Store, searcher recall.Searcher, query string, limit int) ([]*artifact.Artifact, error) {
	if artifact.IsArtifactID(query) {
		return store.Search(ctx, query, 1)
	}
	if searcher == nil {
		return artifact.FetchSearchResults(ctx, store, query, limit)
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 20
	}
	return searcher.Search(ctx, query, limit)
}

func makeV5HandlerWithTaskMemoryProjectionAndCodeIntel(
	store *artifact.Store,
	searcher recall.Searcher,
	crossHybrid *project.CrossHybrid,
	haftDir string,
	projCfg *project.Config,
	indexStore *project.IndexStore,
	taskProjector taskMemoryProjector,
	codeIntelService *codeintel.Service,
) fpf.V5ToolHandler {
	return func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		action, _ := params.Arguments["action"].(string)
		logToolEntry(params.Name, action, params.Arguments)
		start := time.Now()

		// Dispatch
		result, createdRef, toolErr := dispatchToolWithCodeIntel(
			ctx,
			store,
			searcher,
			haftDir,
			params.Name,
			params.Arguments,
			codeIntelService,
		)

		// Post-dispatch hooks
		logger.ToolResult(params.Name, action, time.Since(start).Milliseconds(), toolErr)

		if toolErr == nil {
			result = applyTaskMemoryProjection(
				ctx,
				result,
				taskMemoryProjectionRequest{
					ToolName:    params.Name,
					Action:      action,
					ArtifactRef: createdRef,
					Arguments:   params.Arguments,
					Mode:        taskMemoryProjectionApply,
				},
				taskProjector,
			)
			result = applyCrossProjectRecall(ctx, result, params.Name, action, params.Arguments, store, projCfg, indexStore, crossHybrid)
			result = applyGraphSeededRecallWithCodeIntel(
				ctx,
				result,
				params.Name,
				action,
				params.Arguments,
				store,
				haftDir,
				codeIntelService,
			)
			applyCrossProjectIndex(ctx, params.Name, action, params.Arguments, createdRef, store, projCfg, indexStore, crossHybrid)
			invalidateRecall(searcher, createdRef)
		}

		logAudit(ctx, store.DB(), params.Name, action, params.Arguments, toolErr)

		if toolErr == nil {
			result = applyRefreshReminder(ctx, result, params.Name, store)
			result = applyProfileAwareReadinessReminder(
				ctx,
				result,
				params.Name,
				haftDir,
				params.Arguments,
			)
		}

		return result, toolErr
	}
}

// dispatchTool routes a tool call to its handler. Pure dispatch, no hooks.
// Returns (result, createdArtifactID, err). createdArtifactID is the canonical
// ID of the artifact created by this call (e.g. "dec-20260418-a3f7c1d2"); empty
// string when the action does not create a primary artifact (e.g. read-only
// queries, mutations of existing artifacts).
func dispatchToolWithCodeIntel(
	ctx context.Context,
	store *artifact.Store,
	searcher recall.Searcher,
	haftDir string,
	name string,
	args map[string]any,
	codeIntelService *codeintel.Service,
) (string, string, error) {
	if err := rejectMCPBindingAction(name, args); err != nil {
		return "", "", err
	}

	switch name {
	case "haft_note":
		return handleQuintNote(ctx, store, haftDir, args)
	case "haft_problem":
		return handleQuintProblemWithCreatedRef(
			ctx,
			store,
			haftDir,
			args,
		)
	case "haft_solution":
		return handleQuintSolutionWithCreatedRef(
			ctx,
			store,
			haftDir,
			args,
		)
	case "haft_decision":
		return handleQuintDecision(ctx, store, haftDir, args)
	case "haft_refresh":
		result, err := handleQuintRefreshWithCodeIntel(
			ctx,
			store,
			haftDir,
			args,
			codeIntelService,
		)
		return result, "", err
	case "haft_query":
		result, err := handleQuintQueryWithCodeIntel(
			ctx,
			store,
			searcher,
			haftDir,
			args,
			codeIntelService,
		)
		return result, "", err
	case "haft_commission":
		args = commissionArgsWithProjectRoot(args, filepath.Dir(haftDir))
		result, err := handleHaftCommissionForProject(ctx, store, args)
		return result, "", err
	case "haft_method":
		return handleHaftMethodForProject(
			ctx,
			store,
			haftDir,
			args,
		)
	case "haft_spec_section":
		return handleHaftSpecSectionWithProjectionRef(
			ctx,
			haftDir,
			args,
		)
	default:
		return "", "", fmt.Errorf("unknown tool: %s", name)
	}
}

func commissionArgsWithProjectRoot(args map[string]any, projectRoot string) map[string]any {
	next := copyStringAnyMap(args)
	delete(next, "project_root")
	if strings.TrimSpace(projectRoot) != "" {
		next["project_root"] = projectRoot
	}
	return next
}

// logToolEntry logs the tool call entry with extracted refs.
func logToolEntry(name, action string, args map[string]any) {
	logParams := map[string]string{}
	if action != "" {
		logParams["action"] = action
	}
	for _, key := range []string{"decision_ref", "artifact_ref", "problem_ref"} {
		if ref, ok := args[key].(string); ok {
			logParams[key] = ref
		}
	}
	logger.ToolCall(name, action, logParams)
}

// applyCrossProjectRecall appends cross-project history to frame results. When
// the cross-project hybrid is wired it fuses semantic recall over FTS; otherwise
// it falls back to the FTS path (now AND+OR) directly.
func applyCrossProjectRecall(ctx context.Context, result, name, action string, args map[string]any, store *artifact.Store, projCfg *project.Config, indexStore *project.IndexStore, crossHybrid *project.CrossHybrid) string {
	if name != "haft_problem" || action != "frame" || indexStore == nil || projCfg == nil {
		return result
	}
	signal, _ := args["signal"].(string)
	title, _ := args["title"].(string)
	query := title + " " + signal
	primaryLang := project.DetectPrimaryLanguage(store.DB())

	var recalls []project.IndexRecall
	var err error
	if crossHybrid != nil {
		recalls, err = crossHybrid.Search(ctx, query, projCfg.ID, primaryLang, 3)
	} else {
		recalls, err = indexStore.Search(ctx, query, projCfg.ID, primaryLang, 3)
	}
	if err != nil || len(recalls) == 0 {
		return result
	}
	result += "\n## Cross-Project History\n\n"
	for _, r := range recalls {
		clLabel := fmt.Sprintf("CL%d", r.CL)
		if r.CL == 2 {
			clLabel += " (similar context)"
		} else {
			clLabel += " (different context)"
		}
		result += fmt.Sprintf("- [%s] **%s** — %s (%s, from %s)\n",
			r.DecisionID, r.Title, truncateStr(r.WhySelected, 120), clLabel, r.ProjectName)
	}
	return result + "\n"
}

// applyGraphSeededRecall appends, on a frame that names a seed_file, the artifacts
// the FUSED code+reasoning graph ranks NEAREST that file — closing the gap where
// the keyword-based recall (recallRelated FTS5) misses a decision governing the
// exact file but phrased differently. Best-effort: any error or empty result
// leaves the frame response unchanged. Lives in the shell because the artifact
// core cannot import the code-graph (codeintel).
func applyGraphSeededRecallWithCodeIntel(
	ctx context.Context,
	result string,
	name string,
	action string,
	args map[string]any,
	store *artifact.Store,
	haftDir string,
	codeIntelService *codeintel.Service,
) string {
	if name != "haft_problem" || action != "frame" {
		return result
	}
	seedFileRaw, _ := args["seed_file"].(string)
	seedFile := strings.TrimSpace(seedFileRaw)
	if seedFile == "" {
		return result
	}
	projectRoot := filepath.Dir(haftDir)
	view, err := codeIntelService.RelatedView(
		ctx,
		projectRoot,
		seedFile,
		6,
	)
	if err != nil || len(view.Results) == 0 {
		return result
	}
	lines := make([]string, 0, len(view.Results))
	for _, r := range view.Results {
		if r.Kind != codeintel.RelatedArtifact {
			continue // symbols are not recall — only governing/related artifacts
		}
		lines = append(lines, fmt.Sprintf("- **%s** `%s`", r.Title, r.ID))
	}
	if len(lines) == 0 {
		return result
	}
	result += fmt.Sprintf("\n## Governed nearby (graph recall for %s)\n\n", seedFile)
	result += strings.Join(lines, "\n") + "\n"
	result += "\n_Nearest artifacts in the fused graph — surfaced because keyword recall can miss a decision phrased differently. Check before re-deciding._\n"
	return result
}

// applyCrossProjectIndex writes decision summaries to the global index on
// decide. The cross-project index is keyed by (project_id, decision_id) where
// decision_id MUST be the canonical artifact ID (e.g. "dec-20260418-a3f7c1d2"),
// not the user-supplied selected_title — two decisions in one project can
// legitimately share the same selected option label without colliding.
func applyCrossProjectIndex(ctx context.Context, name, action string, args map[string]any, createdRef string, store *artifact.Store, projCfg *project.Config, indexStore *project.IndexStore, crossHybrid *project.CrossHybrid) {
	if name != "haft_decision" || action != "decide" || indexStore == nil || projCfg == nil {
		return
	}
	if createdRef == "" {
		// Defensive: handler did not return an artifact ID. Skip indexing
		// rather than fall back to selected_title (would collide on duplicate
		// titled options within the same project).
		return
	}
	selectedTitle, _ := args["selected_title"].(string)
	whySelected, _ := args["why_selected"].(string)
	weakestLink, _ := args["weakest_link"].(string)
	primaryLang := project.DetectPrimaryLanguage(store.DB())
	_ = indexStore.WriteDecision(ctx, project.IndexEntry{
		ProjectID:     projCfg.ID,
		ProjectName:   projCfg.Name,
		DecisionID:    createdRef,
		Title:         selectedTitle,
		SelectedTitle: selectedTitle,
		WhySelected:   whySelected,
		WeakestLink:   weakestLink,
		PrimaryLang:   primaryLang,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	logger.Debug().Str("project", projCfg.ID).Str("decision", createdRef).Str("title", selectedTitle).Msg("index.write")

	// A freshly-decided cross-project decision becomes semantically searchable
	// same-session: rebuild the cross-project index in the background (cached
	// vectors reused, only the new decision embeds).
	if crossHybrid != nil {
		crossHybrid.Invalidate()
	}
}

func decisionTitlesForRefs(ctx context.Context, store *artifact.Store, refs []string) map[string]string {
	titles := make(map[string]string, len(refs))
	for _, ref := range refs {
		trimmedRef := strings.TrimSpace(ref)
		if trimmedRef == "" {
			continue
		}
		decision, err := store.Get(ctx, trimmedRef)
		if err != nil {
			continue
		}
		titles[trimmedRef] = decision.Meta.Title
	}
	return titles
}

// applyRefreshReminder appends a reminder if >5 days since last stale scan.
func applyRefreshReminder(ctx context.Context, result, name string, store *artifact.Store) string {
	if refreshReminderDisabled(name) {
		return result
	}
	if machineJSONResponse(result) {
		return result
	}
	lastScan := store.LastRefreshScan(ctx)
	if lastScan.IsZero() {
		return result
	}
	daysSince := int(time.Since(lastScan).Hours() / 24)
	if daysSince >= 5 {
		result += fmt.Sprintf("\n\n--- Refresh reminder: %d days since last stale scan. Run haft_refresh(action=\"scan\") to check for stale decisions and evidence decay. ---\n", daysSince)
	}
	return result
}

func refreshReminderDisabled(name string) bool {
	switch name {
	case "haft_refresh":
		return true
	case "haft_commission":
		return true
	case "haft_method":
		return true
	default:
		return false
	}
}

func machineJSONResponse(result string) bool {
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return false
	}
	return json.Valid([]byte(trimmed))
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// logAudit writes an audit_log row for every tool call. Errors are logged, never propagated.
func logAudit(ctx context.Context, rawDB *sql.DB, toolName, action string, args map[string]any, toolErr error) {
	operation := toolName
	if action != "" {
		operation = toolName + ":" + action
	}

	resultStr := "ok"
	if toolErr != nil {
		resultStr = "error: " + toolErr.Error()
	}

	// Extract target ID from common arg patterns
	targetID := ""
	for _, key := range []string{"artifact_ref", "decision_ref", "problem_ref", "portfolio_ref"} {
		if v, ok := args[key].(string); ok && v != "" {
			targetID = v
			break
		}
	}

	contextID := ""
	if v, ok := args["context"].(string); ok {
		contextID = v
	}

	id := fmt.Sprintf("audit-%s-%09d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000000)

	_, err := rawDB.ExecContext(ctx,
		`INSERT INTO audit_log (id, tool_name, operation, actor, target_id, result, context_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, toolName, operation, "agent", targetID, resultStr, contextID,
	)
	if err != nil {
		logger.Warn().Err(err).Str("tool", toolName).Msg("audit log write failed")
	}
}

func truncateMeasure(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func evidenceRelianceBoundaryLine() string {
	return "Authority boundary: evidence/WLNK display is not approval, not gate passage, not claim truth, not global truth, and not publication; use EvidencePath for attempted-use reliance.\n"
}

// --- Tool handlers ---

// parseNoteAnchors decodes the haft_note `anchors` arg ([{type, ref}, ...]) into
// typed NoteAnchors, skipping malformed entries and ones with an empty ref.
func parseNoteAnchors(raw any) []artifact.NoteAnchor {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []artifact.NoteAnchor
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ref, _ := m["ref"].(string)
		if strings.TrimSpace(ref) == "" {
			continue
		}
		typ, _ := m["type"].(string)
		out = append(out, artifact.NoteAnchor{Type: typ, Ref: ref})
	}
	return out
}

// isArtifactRef reports whether an anchor ref is an artifact ID (a lower-case
// prefix + '-' + a date digit, e.g. dec-20260604-…) as opposed to a code-symbol
// name. Symbol names have no such shape.
func isArtifactRef(ref string) bool {
	return artifact.IsArtifactID(ref)
}

// resolveSymbolAnchor resolves a symbol-anchor ref ("Name" or "Name@file") to a
// single indexed symbol, returning the AffectedSymbol to persist. A ref that
// resolves to no symbol — or to several without a @file qualifier — is an error
// (a dead/ambiguous anchor is never silently kept).
func resolveSymbolAnchor(ctx context.Context, syms *codebase.SymbolStore, ref string) (artifact.AffectedSymbol, error) {
	name, file := ref, ""
	if i := strings.LastIndexByte(ref, '@'); i > 0 {
		name, file = ref[:i], ref[i+1:]
	}
	cands, err := syms.GetByName(ctx, name)
	if err != nil {
		return artifact.AffectedSymbol{}, err
	}
	if file != "" {
		var filtered []codebase.CodeSymbol
		for _, c := range cands {
			if c.FilePath == file {
				filtered = append(filtered, c)
			}
		}
		cands = filtered
	}
	switch len(cands) {
	case 0:
		return artifact.AffectedSymbol{}, fmt.Errorf("symbol anchor %q resolves to no indexed symbol — check the name or qualify with @<file>", ref)
	case 1:
		c := cands[0]
		return artifact.AffectedSymbol{
			FilePath:   c.FilePath,
			SymbolName: c.Name,
			SymbolKind: c.Kind,
			Line:       c.StartLine,
			EndLine:    c.EndLine,
			Hash:       c.Hash,
		}, nil
	default:
		return artifact.AffectedSymbol{}, fmt.Errorf("symbol anchor %q is ambiguous (%d matches) — qualify with @<file>", ref, len(cands))
	}
}

func handleQuintNote(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, string, error) {
	input := artifact.NoteInput{}
	if v, ok := args["title"].(string); ok {
		input.Title = v
	}
	if v, ok := args["task_context"].(string); ok {
		input.TaskContext = v
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
	if v, ok := args["valid_until"].(string); ok {
		input.ValidUntil = v
	}
	input.AffectedFiles = parseStringArrayFromArgs(args, "affected_files")
	input.Observations = parseStringArrayFromArgs(args, "observations")
	// Classify each anchor: an artifact ID (dec-/prob-/...) becomes a typed link;
	// anything else is a code-symbol anchor, resolved against the symbol store
	// (a dead/ambiguous symbol anchor rejects the note — no dead edges).
	symStore := codebase.NewSymbolStore(store.DB())
	for _, an := range parseNoteAnchors(args["anchors"]) {
		if isArtifactRef(an.Ref) {
			input.Anchors = append(input.Anchors, an)
			continue
		}
		as, err := resolveSymbolAnchor(ctx, symStore, an.Ref)
		if err != nil {
			return "", "", err
		}
		input.AffectedSymbols = append(input.AffectedSymbols, as)
	}
	if v, ok := args["search_keywords"].(string); ok {
		input.SearchKeywords = v
	}

	validation := artifact.ValidateNote(ctx, store, input)
	navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, input.Context))

	if !validation.OK {
		return present.NoteRejection(validation, navStrip), "", nil
	}

	a, filePath, err := artifact.CreateNote(ctx, store, haftDir, input)
	if err != nil {
		// WriteWarning is non-fatal — surface warnings in response
		var ww *artifact.WriteWarning
		if errors.As(err, &ww) {
			validation.Warnings = append(validation.Warnings, ww.Warnings...)
		} else {
			return "", "", err
		}
	}
	return present.NoteResponse(a, filePath, validation, navStrip), a.Meta.ID, nil
}

func handleQuintProblemWithCreatedRef(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	args map[string]any,
) (string, string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)

	switch action {
	case "frame":
		input := artifact.ProblemFrameInput{Context: contextName}
		if v, ok := args["task_context"].(string); ok {
			input.TaskContext = v
		}
		if v, ok := args["title"].(string); ok {
			input.Title = v
		}
		if v, ok := args["problem_type"].(string); ok {
			input.ProblemType = v
		}
		if v, ok := args["problem_profile"].(string); ok {
			input.ProblemProfile = v
		}
		if v, ok := args["source_kind"].(string); ok {
			input.SourceKind = v
		}
		if v, ok := args["signal"].(string); ok {
			input.Signal = v
		}
		if v, ok := args["why_now"].(string); ok {
			input.WhyNow = v
		}
		if v, ok := args["scope"].(string); ok {
			input.Scope = v
		}
		if v, ok := args["acceptance_probe"].(string); ok {
			input.AcceptanceProbe = v
		}
		if v, ok := args["freshness_disposition"].(string); ok {
			input.FreshnessDisposition = v
		}
		if v, ok := args["acceptance"].(string); ok {
			input.Acceptance = v
		}
		if v, ok := args["blast_radius"].(string); ok {
			input.BlastRadius = v
		}
		if v, ok := args["reversibility"].(string); ok {
			input.Reversibility = v
		}
		if v, ok := args["mode"].(string); ok {
			input.Mode = v
		}
		input.Constraints = parseStringArrayFromArgs(args, "constraints")
		input.OptimizationTargets = parseStringArrayFromArgs(args, "optimization_targets")
		input.ObservationIndicators = parseStringArrayFromArgs(args, "observation_indicators")
		input = applyProblemSpecFit(ctx, store, haftDir, input)

		a, filePath, err := artifact.FrameProblem(ctx, store, haftDir, input)
		if err != nil {
			return "", "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		resp := present.ProblemResponse("frame", a, filePath, navStrip)
		if warn := artifact.UmbrellaWarning(input.Title, input.Signal, input.Acceptance); warn != "" {
			resp += "\n" + warn
		}
		return resp, a.Meta.ID, nil

	case "characterize":
		input := artifact.CharacterizeInput{}
		if v, ok := args["problem_ref"].(string); ok {
			input.ProblemRef = v
		}
		if v, ok := args["parity_rules"].(string); ok {
			input.ParityRules = v
		}
		parityPlan, err := parseStrictParityPlanFromArgs(args, "parity_plan")
		if err != nil {
			return "", "", err
		}
		input.ParityPlan = parityPlan
		// Log all args keys and types for debugging
		for k, v := range args {
			logger.Debug().Str("key", k).Str("type", fmt.Sprintf("%T", v)).Msg("characterize arg")
		}
		input.Dimensions = parseDimensions(args["dimensions"])
		if input.ProblemRef == "" {
			prob, err := artifact.FindActiveProblem(ctx, store, contextName)
			if err != nil || prob == nil {
				return "No active problem found.\nUse /h-frame to create one first.\n" +
					present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)), "", nil
			}
			input.ProblemRef = prob.Meta.ID
		}

		a, filePath, err := artifact.CharacterizeProblem(ctx, store, haftDir, input)
		if err != nil {
			return "", "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		resp := present.ProblemResponse("characterize", a, filePath, navStrip)
		if warn := artifact.ValueBeforeProxyWarning(input.Dimensions); warn != "" {
			resp += "\n" + warn + "\n"
		}
		return resp, "", nil

	case "select":
		problems, err := artifact.SelectProblems(ctx, store, contextName, 20)
		if err != nil {
			return "", "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		items := artifact.EnrichProblemsForList(ctx, store, problems)
		return present.ProblemsListResponse(items, navStrip), "", nil

	case "close":
		problemRef, _ := args["problem_ref"].(string)
		if problemRef == "" {
			return "", "", fmt.Errorf("problem_ref is required for close action")
		}
		a, err := store.Get(ctx, problemRef)
		if err != nil {
			return "", "", fmt.Errorf("problem %s not found: %w", problemRef, err)
		}
		if a.Meta.Kind != artifact.KindProblemCard {
			return "", "", fmt.Errorf("%s is %s, not a ProblemCard", problemRef, a.Meta.Kind)
		}
		a.Meta.Status = artifact.StatusAddressed
		if err := store.Update(ctx, a); err != nil {
			return "", "", fmt.Errorf("update problem status: %w", err)
		}
		if _, err := artifact.WriteFile(haftDir, a); err != nil {
			logger.Warn().Err(err).Str("problem_ref", problemRef).Msg("problem.close.file_write_failed")
		}
		return fmt.Sprintf("Problem %s marked as addressed.\n", problemRef), "", nil

	default:
		return "", "", fmt.Errorf("unknown action %q — use 'frame', 'characterize', 'select', or 'close'", action)
	}
}

// handleQuintSolutionWithCreatedRef returns the exact persisted portfolio
// carrier touched by explore or compare. The post-dispatch shell uses this
// source coordinate for indexing and typed project-memory projection.
func handleQuintSolutionWithCreatedRef(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	args map[string]any,
) (string, string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)

	switch action {
	case "explore":
		input := artifact.ExploreInput{Context: contextName}
		if v, ok := args["task_context"].(string); ok {
			input.TaskContext = v
		}
		if v, ok := args["problem_ref"].(string); ok {
			input.ProblemRef = v
		}
		if v, ok := args["mode"].(string); ok {
			input.Mode = v
		}
		input.Variants = parseVariants(args)
		if v, ok := args["no_stepping_stone_rationale"].(string); ok {
			input.NoSteppingStoneRationale = v
		}
		if input.ProblemRef == "" {
			prob, _ := artifact.FindActiveProblem(ctx, store, contextName)
			if prob != nil {
				input.ProblemRef = prob.Meta.ID
			}
		}
		input = applyExploreSpecFit(ctx, store, haftDir, input)

		a, filePath, err := artifact.ExploreSolutions(ctx, store, haftDir, input)
		if err != nil {
			return "", "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.SolutionResponse("explore", a, filePath, navStrip),
			a.Meta.ID,
			nil

	case "compare":
		input := artifact.CompareInput{}
		if v, ok := args["portfolio_ref"].(string); ok {
			input.PortfolioRef = v
		}
		input.Results.Dimensions = parseStringArrayFromArgs(args, "dimensions")
		scores, _, err := parseNestedStringMapArg(args, "scores")
		if err != nil {
			return "", "", err
		}
		if scores != nil {
			input.Results.Scores = scores
		}
		input.Results.NonDominatedSet = parseStringArrayFromArgs(args, "non_dominated_set")
		if v, ok := args["policy_applied"].(string); ok {
			input.Results.PolicyApplied = v
		}
		if v, ok := args["selected_ref"].(string); ok {
			input.Results.SelectedRef = v
		}
		if v, ok := args["legacy_recommendation_ref"].(string); ok {
			input.Results.LegacyRecommendationRef = v
		}
		if v, ok := args["recommendation_rationale"].(string); ok {
			input.Results.RecommendationRationale = v
		}
		if _, err := parseJSONArg(args, "dominated_variants", &input.Results.DominatedVariants); err != nil {
			return "", "", err
		}
		if _, err := parseJSONArg(args, "pareto_tradeoffs", &input.Results.ParetoTradeoffs); err != nil {
			return "", "", err
		}
		if _, err := parseJSONArg(args, "incomparable", &input.Results.Incomparable); err != nil {
			return "", "", err
		}
		parityPlan, err := parseStrictParityPlanFromArgs(args, "parity_plan")
		if err != nil {
			return "", "", err
		}
		input.Results.ParityPlan = parityPlan
		if input.PortfolioRef == "" {
			p, _ := artifact.FindActivePortfolio(ctx, store, contextName)
			if p != nil {
				input.PortfolioRef = p.Meta.ID
			} else {
				return "No active solution portfolio found.\nUse /h-explore to create variants first.\n" +
					present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)), "", nil
			}
		}
		input = applyCompareSpecFit(ctx, store, haftDir, input)

		a, filePath, err := artifact.CompareSolutions(ctx, store, haftDir, input)
		if err != nil {
			return "", "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return compareToolResponse(a, filePath, navStrip), a.Meta.ID, nil

	case "similar":
		query, _ := args["query"].(string)
		if query == "" {
			return "", "", fmt.Errorf("query required for similar search")
		}
		results, err := artifact.FetchSearchResults(ctx, store, query, 10)
		if err != nil {
			return "", "", err
		}
		var matches []string
		for _, r := range results {
			if r.Meta.Kind == artifact.KindSolutionPortfolio {
				matches = append(matches, fmt.Sprintf("- [%s] %s (problem: %s)",
					r.Meta.ID, r.Meta.Title, r.Meta.Context))
			}
		}
		if len(matches) == 0 {
			return "No similar past solutions found. This is a novel problem.", "", nil
		}
		return fmt.Sprintf("Past solution portfolios matching \"%s\":\n%s\n\nUse haft_query(search) for details on any portfolio.",
			query, strings.Join(matches, "\n")), "", nil

	default:
		return "", "", fmt.Errorf(
			"unknown action %q — use 'explore', 'compare', or 'similar'",
			action,
		)
	}
}

// handleQuintDecision returns (result, createdArtifactID, err). createdArtifactID
// is the canonical ID of the newly created DecisionRecord on the "decide"
// action, empty for all other actions (which mutate or read existing artifacts).
func handleQuintDecision(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)

	switch action {
	case "decide":
		// MCP arguments are proposal content, never conversational provenance.
		// Until MCP receives a verifiable host receipt, decision binding stays
		// fail-closed and the supported host routes the operator request to CLI.
		if err := rejectUnverifiedMCPDecisionBinding(); err != nil {
			return "", "", err
		}

		input := artifact.DecideInput{Context: contextName}
		var err error
		if v, ok := args["selected_title"].(string); ok {
			input.SelectedTitle = v
		}
		if v, ok := args["why_selected"].(string); ok {
			input.WhySelected = v
		}
		if v, ok := args["selection_policy"].(string); ok {
			input.SelectionPolicy = v
		}
		if v, ok := args["counterargument"].(string); ok {
			input.CounterArgument = v
		}
		if v, ok := args["weakest_link"].(string); ok {
			input.WeakestLink = v
		}
		if v, ok := args["problem_ref"].(string); ok {
			input.ProblemRef = v
		}
		if v, ok := args["problem_statement"].(string); ok {
			input.ProblemStatement = v
		}
		if input.ProblemRefs, err = parseStrictStringArrayFromArgs(args, "problem_refs"); err != nil {
			return "", "", err
		}
		if v, ok := args["portfolio_ref"].(string); ok {
			input.PortfolioRef = v
		}
		if v, ok := args["valid_until"].(string); ok {
			input.ValidUntil = v
		}
		if v, ok := args["task_context"].(string); ok {
			input.TaskContext = v
		}
		if v, ok := args["mode"].(string); ok {
			input.Mode = v
		}
		if v, ok := args["governance_mode"].(string); ok {
			input.GovernanceMode = v
		}
		if input.Invariants, err = parseStrictStringArrayFromArgs(args, "invariants"); err != nil {
			return "", "", err
		}
		if input.PreConditions, err = parseStrictStringArrayFromArgs(args, "pre_conditions"); err != nil {
			return "", "", err
		}
		if input.PostConditions, err = parseStrictStringArrayFromArgs(args, "post_conditions"); err != nil {
			return "", "", err
		}
		if input.Admissibility, err = parseStrictStringArrayFromArgs(args, "admissibility"); err != nil {
			return "", "", err
		}
		if input.EvidenceReqs, err = parseStrictStringArrayFromArgs(args, "evidence_requirements"); err != nil {
			return "", "", err
		}
		if input.RefreshTriggers, err = parseStrictStringArrayFromArgs(args, "refresh_triggers"); err != nil {
			return "", "", err
		}
		if input.SectionRefs, err = parseStrictStringArrayFromArgs(args, "section_refs"); err != nil {
			return "", "", err
		}
		if _, err := parseJSONArg(args, "spec_binding_preflight", &input.SpecBindingPreflight); err != nil {
			return "", "", err
		}
		if v, ok := args["spec_binding_preflight_required"].(bool); ok {
			input.SpecBindingRequired = v
		}
		if input.AffectedFiles, err = parseStrictStringArrayFromArgs(args, "affected_files"); err != nil {
			return "", "", err
		}
		if input.BindingHints, err = parseStrictStringArrayFromArgs(args, "binding_hints"); err != nil {
			return "", "", err
		}
		if v, ok := args["binding_scope"].(string); ok {
			input.BindingScope = v
		}
		if v, ok := args["binding_fallback_reason"].(string); ok {
			input.BindingFallbackReason = v
		}
		if _, err := parseJSONArg(args, "binding_targets", &input.BindingTargets); err != nil {
			return "", "", err
		}
		if v, ok := args["search_keywords"].(string); ok {
			input.SearchKeywords = v
		}
		if input.Rollback, err = parseStrictRollbackSpecFromArgs(args, "rollback"); err != nil {
			return "", "", err
		}
		if input.WhyNotOthers, err = parseStrictRejectionReasonsFromArgs(args, "why_not_others"); err != nil {
			return "", "", err
		}
		if input.Predictions, err = parsePredictionInputsFromArgs(args, "predictions"); err != nil {
			return "", "", err
		}
		if _, err := parseJSONArg(args, "choice_result", &input.ChoiceResult); err != nil {
			return "", "", err
		}
		if _, err := parseJSONArg(args, "transformation_record", &input.TransformationRecord); err != nil {
			return "", "", err
		}
		if input.ProblemRef == "" && len(input.ProblemRefs) == 0 && strings.TrimSpace(input.ProblemStatement) == "" {
			p, _ := artifact.FindActiveProblem(ctx, store, contextName)
			if p != nil {
				input.ProblemRef = p.Meta.ID
			}
		}
		// Auto-detect portfolio ONLY when it's linked to the same problem.
		// Load full artifact to get links (ListByKind returns lightweight entries).
		if input.PortfolioRef == "" && input.ProblemRef != "" {
			p, _ := artifact.FindActivePortfolio(ctx, store, contextName)
			if p != nil {
				fullPortfolio, _ := store.Get(ctx, p.Meta.ID)
				if fullPortfolio != nil {
					for _, ref := range artifact.ResolvePortfolioProblemRefs(fullPortfolio) {
						if ref == input.ProblemRef {
							input.PortfolioRef = p.Meta.ID
							break
						}
					}
				}
			}
		}

		input, err = applyDecisionSpecBindingPreflight(ctx, store, haftDir, input)
		if err != nil {
			return "", "", err
		}

		a, filePath, err := (*artifact.Artifact)(nil), "", rejectUnverifiedMCPDecisionBinding()
		if err != nil {
			return "", "", err
		}

		// Auto-baseline when affected_files are present
		var baselineNote string
		if len(input.AffectedFiles) > 0 {
			projectRoot := filepath.Dir(haftDir)
			fields := a.UnmarshalDecisionFields()
			if fields.IsImplementationFootprintOnly() && !decideInputRequestsExplicitBaselineAuthority(input) {
				baselineNote = "\n\nAffected files recorded as implementation footprint only; no drift baseline created. Add governance_targets, drift_watch_targets, binding_targets, or binding_scope with binding_fallback_reason if this decision should govern drift."
			} else {
				baselined, blErr := artifact.Baseline(ctx, store, projectRoot, artifact.BaselineInput{
					DecisionRef:           a.Meta.ID,
					BindingTargets:        input.BindingTargets,
					BindingHints:          input.BindingHints,
					BindingScope:          input.BindingScope,
					BindingFallbackReason: input.BindingFallbackReason,
				})
				if blErr != nil {
					baselineNote = fmt.Sprintf("\n\n⚠ Auto-baseline failed: %v\nRun manually: haft_decision(action=\"baseline\", decision_ref=\"%s\")", blErr, a.Meta.ID)
				} else {
					baselineNote = fmt.Sprintf("\n\nBaseline established for %d file(s).", len(baselined))
				}
			}
		}

		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		resp := present.DecisionResponse("decide", a, filePath, "", navStrip) + baselineNote
		if warn := artifact.ReputationWarning(input.WhySelected, input.SelectionPolicy, input.CounterArgument, input.WeakestLink); warn != "" {
			resp += "\n" + warn
		}
		return resp, a.Meta.ID, nil

	case "apply":
		decisionRef, _ := args["decision_ref"].(string)
		if decisionRef == "" {
			decisionRef, _ = args["artifact_ref"].(string)
		}
		if decisionRef == "" {
			decisions, _ := store.ListByKind(ctx, artifact.KindDecisionRecord, 1)
			if len(decisions) > 0 {
				decisionRef = decisions[0].Meta.ID
			} else {
				return "No decision found.\nUse /h-decide to finalize a decision first.\n" +
					present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)), "", nil
			}
		}

		brief, err := artifact.Apply(ctx, store, decisionRef)
		if err != nil {
			return "", "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.DecisionResponse("apply", nil, "", brief, navStrip), "", nil

	case "measure":
		input := artifact.MeasureInput{}
		var err error
		if v, ok := args["decision_ref"].(string); ok {
			input.DecisionRef = v
		}
		if input.DecisionRef == "" {
			if v, ok := args["artifact_ref"].(string); ok {
				input.DecisionRef = v
			}
		}
		if v, ok := args["findings"].(string); ok {
			input.Findings = v
		}
		if v, ok := args["verdict"].(string); ok {
			input.Verdict = v
		}
		if input.CriteriaMet, err = parseStrictStringArrayFromArgs(args, "criteria_met"); err != nil {
			return "", "", err
		}
		if input.CriteriaNotMet, err = parseStrictStringArrayFromArgs(args, "criteria_not_met"); err != nil {
			return "", "", err
		}
		if input.Measurements, err = parseStrictStringArrayFromArgs(args, "measurements"); err != nil {
			return "", "", err
		}
		if input.DecisionRef == "" {
			return "haft_decision(measure) requires decision_ref (or artifact_ref) — the DecisionRecord ID to record measurement against. Run haft_query(action=\"status\") to find the intended decision ID.\n" +
				present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)), "", nil
		}

		a, err := artifact.Measure(ctx, store, haftDir, input)
		// Surface baseline gate warnings (not errors — measurement still recorded)
		var measureWarning string
		if ww, ok := err.(*artifact.WriteWarning); ok {
			for _, w := range ww.Warnings {
				measureWarning += w + "\n"
			}
			err = nil // warnings, not errors
		}
		if err != nil {
			return "", "", err
		}
		// Show WLNK summary after measurement
		wlnk := artifact.ComputeWLNKSummary(ctx, store, a.Meta.ID)
		extra := ""
		if measureWarning != "" {
			extra += measureWarning + "\n"
		}
		extra += fmt.Sprintf("WLNK: %s\n", wlnk.Summary)
		extra += evidenceRelianceBoundaryLine()

		// Lemniscate feedback: failed/partial measurement → suggest reopen
		if input.Verdict == "failed" || input.Verdict == "partial" {
			extra += fmt.Sprintf("\nThis decision's measurement %s. Consider re-evaluating:\n", input.Verdict)
			extra += fmt.Sprintf("  haft_refresh(action=\"reopen\", artifact_ref=\"%s\", reason=\"measurement %s: %s\")\n",
				input.DecisionRef, input.Verdict, truncateMeasure(input.Findings, 80))
		}

		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.DecisionResponse("measure", a, "", extra, navStrip), "", nil

	case "evidence":
		input := artifact.EvidenceInput{
			CongruenceLevel: -1, // sentinel: "not provided", will default to 3
			FormalityLevel:  -1, // sentinel: "not provided", will default to 5
		}
		var err error
		if v, ok := args["artifact_ref"].(string); ok {
			input.ArtifactRef = v
		}
		if v, ok := args["evidence_content"].(string); ok {
			input.Content = v
		}
		if v, ok := args["evidence_type"].(string); ok {
			input.Type = v
		}
		if v, ok := args["evidence_verdict"].(string); ok {
			input.Verdict = v
		}
		if v, ok := args["carrier_ref"].(string); ok {
			input.CarrierRef = v
		}
		if v, ok := args["valid_until"].(string); ok {
			input.ValidUntil = v
		}
		if v, ok := args["causal_support_basis"].(string); ok {
			input.CausalSupportBasis = v
		}
		if cl, ok := args["congruence_level"].(float64); ok {
			input.CongruenceLevel = int(cl)
		}
		if fl, ok := args["formality_level"].(float64); ok {
			input.FormalityLevel = int(fl)
		}
		if v, ok := args["formality_scale_id"].(string); ok {
			input.FormalityScaleID = v
		}
		if input.ClaimRefs, err = parseStrictStringArrayFromArgs(args, "claim_refs"); err != nil {
			return "", "", err
		}
		if input.ClaimScope, err = parseStrictStringArrayFromArgs(args, "claim_scope"); err != nil {
			return "", "", err
		}

		item, err := artifact.AttachEvidence(ctx, store, input)
		if err != nil {
			return "", "", err
		}

		wlnk := artifact.ComputeWLNKSummary(ctx, store, input.ArtifactRef)
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		extra := fmt.Sprintf("Evidence attached: %s [%s]\nVerdict: %s\nWLNK: %s\n%s", item.ID, item.Type, item.Verdict, wlnk.Summary, evidenceRelianceBoundaryLine())
		if warning := evidenceCausalUseWarning(input, item); warning != "" {
			extra += "\n" + warning + "\n"
		}
		return present.DecisionResponse("evidence", nil, "", extra, navStrip), "", nil

	case "baseline":
		input := artifact.BaselineInput{}
		if v, ok := args["decision_ref"].(string); ok {
			input.DecisionRef = v
		}
		if input.DecisionRef == "" {
			if v, ok := args["artifact_ref"].(string); ok {
				input.DecisionRef = v
			}
		}
		if input.DecisionRef == "" {
			return "haft_decision(baseline) requires decision_ref (or artifact_ref) — the DecisionRecord ID to snapshot files for. Run haft_query(action=\"status\") to find the intended decision ID.\n" +
				present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)), "", nil
		}
		input.AffectedFiles = parseStringArrayFromArgs(args, "affected_files")
		input.BindingHints = parseStringArrayFromArgs(args, "binding_hints")
		if v, ok := args["binding_scope"].(string); ok {
			input.BindingScope = v
		}
		if v, ok := args["binding_fallback_reason"].(string); ok {
			input.BindingFallbackReason = v
		}
		if _, err := parseJSONArg(args, "binding_targets", &input.BindingTargets); err != nil {
			return "", "", err
		}

		var baselineWarnings []string
		if len(input.AffectedFiles) > 0 {
			baselineWarnings = artifact.WarnSharedFiles(input.AffectedFiles)
		}

		files, err := artifact.Baseline(ctx, store, filepath.Dir(haftDir), input)
		if err != nil {
			return "", "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		decisionTitle := ""
		if decision, err := store.Get(ctx, input.DecisionRef); err == nil {
			decisionTitle = decision.Meta.Title
		}
		result := present.BaselineResponse(decisionTitle, input.DecisionRef, files, navStrip)
		for _, w := range baselineWarnings {
			result = "⚠ " + w + "\n" + result
		}
		return result, "", nil

	default:
		return "", "", fmt.Errorf("unknown action %q — use 'decide', 'apply', 'measure', 'evidence', or 'baseline'", action)
	}
}

func handleQuintRefreshWithCodeIntel(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	args map[string]any,
	codeIntelService *codeintel.Service,
) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)
	reason, _ := args["reason"].(string)
	taskContext, _ := args["task_context"].(string)
	navStrip := navStripForStatusStaleLane(ctx, store, contextName)

	// Support both artifact_ref (new) and decision_ref (backward compat)
	artifactRef, _ := args["artifact_ref"].(string)
	if artifactRef == "" {
		artifactRef, _ = args["decision_ref"].(string)
	}

	switch artifact.RefreshAction(action) {
	case artifact.RefreshScan:
		projectRoot := filepath.Dir(haftDir)

		// The shared coordinator owns source parsing and module publication.
		// The legacy dependency projection remains a secondary best-effort scan.
		indexRefresh, err := codeIntelService.EnsureIndex(ctx, projectRoot)
		if err != nil {
			logger.Warn().Err(err).Msg("refresh: code-index coordination failed (non-fatal)")
		} else if indexRefresh.Outcome == codeintel.IndexNoCompleteEpoch ||
			indexRefresh.Outcome == codeintel.IndexRetainedAfterFailure {
			logger.Info().
				Str("outcome", string(indexRefresh.Outcome)).
				Str("reason", indexRefresh.Reason).
				Msg("refresh: code index retained its prior publication")
		}
		if _, err := codebase.NewScanner(store.DB()).ScanDependencies(
			ctx,
			projectRoot,
		); err != nil {
			logger.Warn().Err(err).Msg(
				"refresh: dependency rescan failed (non-fatal)",
			)
		}

		items, err := artifact.ScanStale(ctx, store, projectRoot)
		if err != nil {
			return "", err
		}
		verbose, _ := args["verbose"].(bool)
		result := present.ScanResponseSummary(items, "")
		if verbose {
			result = present.ScanResponse(items, "")
		}
		governanceAttention := scanGovernanceAttention(ctx, store)
		result += present.GovernanceAttentionResponse(governanceAttention)

		// Level C: enrich drift reports with dependency impact
		driftReports, _ := artifact.CheckDrift(ctx, store, projectRoot)
		for i, r := range driftReports {
			if !r.HasBaseline {
				continue
			}
			hasDrift := false
			var driftedFiles []string
			for _, f := range r.Files {
				if f.Status == artifact.DriftModified || f.Status == artifact.DriftAdded || f.Status == artifact.DriftMissing {
					hasDrift = true
					driftedFiles = append(driftedFiles, f.Path)
				}
			}
			if hasDrift && len(driftedFiles) > 0 {
				impacts, _ := codebase.EnrichDriftWithImpact(ctx, store.DB(), driftedFiles)
				if len(impacts) > 0 {
					for _, imp := range impacts {
						driftReports[i].ImpactedModules = append(driftReports[i].ImpactedModules, artifact.ModuleImpact{
							ModuleID:       imp.ModuleID,
							ModulePath:     imp.ModulePath,
							DecisionIDs:    imp.DecisionIDs,
							DecisionTitles: decisionTitlesForRefs(ctx, store, imp.DecisionIDs),
							IsBlind:        imp.IsBlind,
						})
					}
				}
			}
		}
		// If any drift has impact propagation, append the detailed report
		hasImpact := false
		for _, r := range driftReports {
			if len(r.ImpactedModules) > 0 {
				hasImpact = true
				break
			}
		}
		if hasImpact {
			if verbose {
				result += "\n" + present.DriftResponse(driftReports, "")
			} else {
				result += "\n" + present.DriftResponseSummary(driftReports, "")
			}
		}

		return result + navStripWithStaleSnapshot(ctx, store, contextName, items), nil

	case artifact.RefreshPlan:
		projectRoot := filepath.Dir(haftDir)
		plan, err := artifact.BuildMaintenancePlan(ctx, store, projectRoot)
		if err != nil {
			return "", err
		}
		verbose, _ := args["verbose"].(bool)
		if verbose {
			return present.MaintenancePlanResponse(plan, navStrip), nil
		}
		return present.CompactMaintenancePlanResponse(plan, navStrip), nil

	case artifact.RefreshReview:
		projectRoot := filepath.Dir(haftDir)
		plan, err := artifact.BuildMaintenancePlan(ctx, store, projectRoot)
		if err != nil {
			return "", err
		}
		review := artifact.BuildMaintenanceJudgmentReview(plan)
		return present.MaintenanceJudgmentReviewResponse(review, navStrip), nil

	case artifact.RefreshDrain:
		projectRoot := filepath.Dir(haftDir)
		dryRun := true
		if rawDryRun, ok := args["dry_run"]; ok {
			var dryRunOK bool
			dryRun, dryRunOK = rawDryRun.(bool)
			if !dryRunOK {
				return "", fmt.Errorf("dry_run must be a boolean for refresh drain")
			}
		}
		if !dryRun {
			return "", fmt.Errorf("haft_refresh(action=\"drain\") is MCP-safe preview only; use dry_run=true here and `haft overseer drain` for explicit non-dry maintenance")
		}
		report, err := buildMaintenanceDrainReport(ctx, store, projectRoot, dryRun)
		if err != nil {
			return "", err
		}
		navStrip = navStripForStatusStaleLane(ctx, store, contextName)
		return present.MaintenanceDrainResponse(report, navStrip), nil

	case artifact.RefreshWaive:
		if artifactRef == "" {
			return "artifact_ref is required for waive.\n" + navStrip, nil
		}
		newValidUntil, _ := args["new_valid_until"].(string)
		evidence, _ := args["evidence"].(string)
		a, err := artifact.WaiveArtifact(ctx, store, haftDir, artifactRef, reason, newValidUntil, evidence)
		if err != nil {
			return "", err
		}
		_, _ = artifact.CreateRefreshReportWithTaskContext(ctx, store, haftDir, artifactRef, "waive", reason, fmt.Sprintf("Extended to %s", a.Meta.ValidUntil), taskContext)
		return present.RefreshActionResponse(artifact.RefreshWaive, a, nil, navStrip), nil

	case artifact.RefreshReopen:
		if artifactRef == "" {
			return "artifact_ref is required for reopen. Note: reopen only works on decisions.\n" + navStrip, nil
		}
		dec, newProb, err := artifact.ReopenDecisionWithTaskContext(ctx, store, haftDir, artifactRef, reason, taskContext)
		if err != nil {
			return "", err
		}
		_, _ = artifact.CreateRefreshReportWithTaskContext(ctx, store, haftDir, artifactRef, "reopen", reason, fmt.Sprintf("New problem: %s", newProb.Meta.ID), taskContext)
		return present.RefreshActionResponse(artifact.RefreshReopen, dec, newProb, navStrip), nil

	case artifact.RefreshSupersede:
		if artifactRef == "" {
			return "artifact_ref is required for supersede.\n" + navStrip, nil
		}
		newRef, _ := args["new_decision_ref"].(string)
		if newRef == "" {
			newRef, _ = args["new_artifact_ref"].(string)
		}
		a, err := artifact.SupersedeArtifact(ctx, store, haftDir, artifactRef, newRef, reason)
		if err != nil {
			return "", err
		}
		_, _ = artifact.CreateRefreshReportWithTaskContext(ctx, store, haftDir, artifactRef, "supersede", reason, fmt.Sprintf("Replaced by %s", newRef), taskContext)
		return present.RefreshActionResponse(artifact.RefreshSupersede, a, nil, navStrip), nil

	case artifact.RefreshDeprecate:
		if artifactRef == "" {
			return "artifact_ref is required for deprecate.\n" + navStrip, nil
		}
		a, err := artifact.DeprecateArtifact(ctx, store, haftDir, artifactRef, reason)
		if err != nil {
			return "", err
		}
		_, _ = artifact.CreateRefreshReportWithTaskContext(ctx, store, haftDir, artifactRef, "deprecate", reason, "Artifact deprecated", taskContext)
		return present.RefreshActionResponse(artifact.RefreshDeprecate, a, nil, navStrip), nil

	case artifact.RefreshReconcile:
		overlaps, err := artifact.Reconcile(ctx, store)
		if err != nil {
			return "", err
		}
		return present.ReconcileResponse(overlaps, navStrip), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'scan', 'waive', 'reopen', 'supersede', 'deprecate', or 'reconcile'", action)
	}
}

const codeContextBatchLimit = 8

func handleCodeContextQueryWithCodeIntel(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	args map[string]any,
	navStrip string,
	codeIntelService *codeintel.Service,
) (string, error) {
	projectRoot := filepath.Dir(haftDir)
	files := codeContextFiles(args)
	anchorID, _ := args["anchor_id"].(string)
	symbol, _ := args["symbol"].(string)
	line := intArg(args, "line", 0)
	if anchorID != "" && len(files) == 0 {
		resolved, err := resolveCurrentSymbolAnchorWithCodeIntel(
			ctx,
			store,
			projectRoot,
			anchorID,
			codeIntelService,
		)
		if err != nil {
			return "", err
		}
		files = []string{resolved.FilePath}
		symbol = resolved.Name
		line = resolved.StartLine
	}
	if len(files) == 0 {
		return "", fmt.Errorf("file or files is required for code_context action")
	}
	if len(files) > codeContextBatchLimit {
		return "", fmt.Errorf("code_context files accepts at most %d unique paths, got %d", codeContextBatchLimit, len(files))
	}
	if len(files) > 1 && (anchorID != "" || symbol != "" || line > 0) {
		return "", fmt.Errorf("symbol, anchor_id, and line are single-target fields; use them only with one code_context file")
	}
	full, _ := args["full"].(bool)
	if len(files) > 1 && full {
		return "", fmt.Errorf("full=true is single-target only; use a typed lane for bounded code_context batches")
	}
	lane, err := codeContextLane(args)
	if err != nil {
		return "", err
	}
	limit, hasLimit := codeContextLaneLimit(args)
	if len(files) > 1 && !hasLimit {
		limit = 5
	}
	usesCodeIndex := !full &&
		(lane == present.CodeContextLaneIndex ||
			lane == present.CodeContextLaneSymbols)
	attemptLimit := 1
	if usesCodeIndex {
		attemptLimit = 2
	}
	var lastErr error
	for range attemptLimit {
		refreshed := false
		var indexState codebase.IndexState
		var publishedIndexState codebase.IndexState
		var indexRefresh codeintel.IndexCoordinationResult
		if usesCodeIndex {
			var refreshErr error
			indexRefresh, refreshErr = codeIntelService.EnsureIndex(
				ctx,
				projectRoot,
			)
			err = refreshErr
			if err != nil {
				return "", err
			}
			refreshed = indexRefresh.Rebuilt()
			publishedIndexState, err = codeIntelService.CurrentIndexState(ctx)
			if err != nil {
				return "", err
			}
			indexState = indexRefresh.EffectiveIndexState(
				publishedIndexState,
			)
		}
		parts, err := renderCodeContextTargets(
			ctx,
			store,
			files,
			symbol,
			anchorID,
			line,
			lane,
			limit,
			hasLimit,
			full,
			refreshed,
			indexState,
		)
		if err != nil {
			return "", err
		}
		if !usesCodeIndex {
			return strings.Join(parts, "\n---\n\n") + navStrip, nil
		}
		if err := codeIntelService.ConfirmIndexState(
			ctx,
			publishedIndexState,
		); err != nil {
			var changed *codeintel.IndexBasisChangedError
			if !errors.As(err, &changed) {
				return "", err
			}
			lastErr = err
			continue
		}
		response := present.IndexStateResponse(indexState)
		response += present.IndexCoordinationResponse(indexRefresh)
		response += strings.Join(parts, "\n---\n\n")
		return response + navStrip, nil
	}
	return "", lastErr
}

func renderCodeContextTargets(
	ctx context.Context,
	store *artifact.Store,
	files []string,
	symbol string,
	anchorID string,
	line int,
	lane present.CodeContextLane,
	limit int,
	hasLimit bool,
	full bool,
	refreshed bool,
	indexState codebase.IndexState,
) ([]string, error) {
	parts := make([]string, 0, len(files))
	for index, file := range files {
		target := contextgraph.Target{
			File:     file,
			Symbol:   symbol,
			AnchorID: anchorID,
			Line:     line,
		}
		body, err := renderCodeContextTarget(
			ctx,
			store,
			target,
			lane,
			limit,
			hasLimit,
			full,
			refreshed,
			indexState,
		)
		if err != nil {
			return nil, err
		}
		if len(files) > 1 {
			body = fmt.Sprintf(
				"# Code context batch %d/%d — `%s`\n\n%s",
				index+1,
				len(files),
				file,
				body,
			)
		}
		parts = append(parts, body)
	}
	return parts, nil
}

func codeContextFiles(args map[string]any) []string {
	values := make([]string, 0)
	if file, _ := args["file"].(string); strings.TrimSpace(file) != "" {
		values = append(values, file)
	}
	switch files := args["files"].(type) {
	case []string:
		values = append(values, files...)
	case []any:
		for _, value := range files {
			if file, ok := value.(string); ok {
				values = append(values, file)
			}
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		file := filepath.ToSlash(strings.TrimSpace(value))
		if file == "" || seen[file] {
			continue
		}
		seen[file] = true
		out = append(out, file)
	}
	return out
}

func codeContextLane(args map[string]any) (present.CodeContextLane, error) {
	rawLane, _ := args["lane"].(string)
	if strings.TrimSpace(rawLane) == "" {
		return present.CodeContextLaneIndex, nil
	}
	lane, valid := present.ParseCodeContextLane(rawLane)
	if !valid {
		return "", fmt.Errorf("unknown code_context lane %q — valid lanes: %s", rawLane, strings.Join(present.ValidCodeContextLaneNames(), ", "))
	}
	return lane, nil
}

func renderCodeContextTarget(
	ctx context.Context,
	store *artifact.Store,
	target contextgraph.Target,
	lane present.CodeContextLane,
	limit int,
	hasLimit bool,
	full bool,
	refreshed bool,
	indexState codebase.IndexState,
) (string, error) {
	exclusion, excluded := codeContextIndexExclusion(
		indexState,
		target.File,
	)
	if !full && lane == present.CodeContextLaneSymbols {
		if excluded {
			return present.CodeContextSymbolsUnavailableResponse(
				target,
				fmt.Errorf(
					"source excluded from epoch %d: %s",
					indexState.Epoch,
					exclusion.Reason,
				),
			), nil
		}
		symbols, err := codeContextSymbolsForFile(ctx, store, target.File)
		if err != nil {
			return present.CodeContextSymbolsUnavailableResponse(target, err), nil
		}
		return present.CodeContextSymbolsResponse(target, symbols, limit, refreshed), nil
	}
	cc, err := contextgraph.FetchCodeContext(ctx, store, graph.NewStore(store.DB()), target)
	if err != nil {
		return "", err
	}
	if full {
		return present.CodeContextResponseFull(cc), nil
	}
	options := present.CodeContextRenderOptions{Lane: lane, ArtifactLimit: limit}
	if hasLimit {
		options.InvariantLimit = limit
		options.ContextInvariantLimit = limit
	}
	if lane == present.CodeContextLaneIndex {
		if excluded {
			options.SymbolUnavailable = fmt.Sprintf(
				"source excluded from epoch %d: %s",
				indexState.Epoch,
				exclusion.Reason,
			)
		} else {
			symbols, err := codeContextSymbolsForFile(
				ctx,
				store,
				target.File,
			)
			options.SymbolCountKnown = err == nil
			options.SymbolCount = len(symbols)
			if err != nil {
				options.SymbolUnavailable = err.Error()
			}
		}
	}
	return present.CodeContextResponseWithOptions(cc, options), nil
}

func codeContextIndexExclusion(
	state codebase.IndexState,
	file string,
) (codebase.IndexExclusionSnapshot, bool) {
	canonical := filepath.ToSlash(filepath.Clean(file))
	for _, exclusion := range state.Basis.Exclusions {
		if exclusion.Path == canonical {
			return exclusion, true
		}
	}
	return codebase.IndexExclusionSnapshot{}, false
}

func codeContextLaneLimit(args map[string]any) (int, bool) {
	if _, ok := args["limit"]; !ok {
		return 20, false
	}
	limit := intArg(args, "limit", 20)
	if limit < 1 {
		return 20, true
	}
	if limit > 100 {
		return 100, true
	}
	return limit, true
}

func compactProjectionLimit(args map[string]any) int {
	limit := intArg(args, "limit", driftEventsSummaryEventLimit)
	if limit < 1 {
		return driftEventsSummaryEventLimit
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func codeContextSymbolsForFile(ctx context.Context, store *artifact.Store, file string) ([]present.CodeContextSymbolItem, error) {
	symbolStore := codebase.NewSymbolStore(store.DB())
	symbols, err := symbolStore.GetByFile(ctx, file)
	if err != nil {
		return nil, err
	}

	items := make([]present.CodeContextSymbolItem, 0, len(symbols))
	for _, symbol := range symbols {
		endLine := symbol.EndLine
		if endLine == 0 {
			endLine = symbol.StartLine
		}
		items = append(items, present.CodeContextSymbolItem{
			Name:      codeContextSymbolName(symbol),
			Kind:      symbol.Kind,
			StartLine: symbol.StartLine,
			EndLine:   endLine,
		})
	}
	return items, nil
}

func statusDataStaleNavItems(items []artifact.StaleItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID+": "+item.Title+" ("+item.Reason+")")
	}
	return out
}

func navStripWithStaleSnapshot(ctx context.Context, store artifact.ArtifactStore, contextName string, staleItems []artifact.StaleItem) string {
	nav := artifact.ComputeNavState(ctx, store, contextName)
	nav.StaleCount = len(staleItems)
	nav.StaleItems = statusDataStaleNavItems(staleItems)
	return present.NavStrip(nav)
}

func navStripWithoutStaleSnapshot(ctx context.Context, store artifact.ArtifactStore, contextName string) string {
	nav := artifact.ComputeNavState(ctx, store, contextName)
	nav.StaleCount = 0
	nav.StaleItems = nil
	return present.NavStrip(nav)
}

func navStripForStatusStaleLane(ctx context.Context, store artifact.ArtifactStore, contextName string) string {
	data, err := artifact.FetchStatusData(ctx, store, contextName, "")
	if err != nil {
		return navStripWithoutStaleSnapshot(ctx, store, contextName)
	}
	return navStripWithStaleSnapshot(ctx, store, contextName, data.StaleItems)
}

func codeContextSymbolName(symbol codebase.CodeSymbol) string {
	if symbol.Receiver == "" {
		return symbol.Name
	}
	return symbol.Receiver + "." + symbol.Name
}

func handleQuintQuery(ctx context.Context, store *artifact.Store, searcher recall.Searcher, haftDir string, args map[string]any) (string, error) {
	return handleQuintQueryWithCodeIntel(
		ctx,
		store,
		searcher,
		haftDir,
		args,
		codeintel.NewService(store),
	)
}

func handleQuintQueryWithCodeIntel(
	ctx context.Context,
	store *artifact.Store,
	searcher recall.Searcher,
	haftDir string,
	args map[string]any,
	codeIntelService *codeintel.Service,
) (string, error) {
	action, _ := args["action"].(string)
	if err := rejectWrongIdentifierNamespaceForQueryAction(ctx, store, action, args); err != nil {
		return "", err
	}
	contextName, _ := args["context"].(string)
	// Status already owns one StatusData snapshot and renders its navigation
	// strip from that same snapshot below. Computing the generic strip here
	// would perform the full status aggregation twice for every status call.
	var navStrip string
	if action != "status" {
		navStrip = navStripForStatusStaleLane(ctx, store, contextName)
	}

	switch action {
	case "search":
		query, _ := args["query"].(string)
		full, _ := args["full"].(bool)
		if full && !artifact.IsArtifactID(query) {
			return "", fmt.Errorf("full artifact search requires an exact artifact ID; run haft_query(action=\"search\", query=\"<terms>\") first, then haft_query(action=\"related\", artifact_ref=\"<ref>\")")
		}
		if full {
			return artifactQueryContractResponse(ctx, store, strings.TrimSpace(query))
		}
		limit := 20
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		results, err := searchArtifacts(ctx, store, searcher, query, limit)
		if err != nil {
			return "", err
		}
		if artifact.IsArtifactID(query) && len(results) == 0 {
			return "", exactArtifactMissError(strings.TrimSpace(query))
		}
		response := present.SearchResponse(results, query)
		if artifact.IsArtifactID(query) {
			return response, nil
		}
		return response + navStrip, nil

	case "status":
		// H1 (dec-20260526-9fdd33ed): pass projectRoot so /h-status
		// surfaces drift via FetchStatusData → CheckDrift → StatusData.Drift.
		projectRoot := filepath.Dir(haftDir)
		scopeRequest, err := projectSpecificationScopeRequestFromFlag(
			stringArg(args, "scope_id"),
		)
		if err != nil {
			return "", err
		}
		readiness, err := inspectCanonicalProjectReadiness(
			ctx,
			projectRoot,
			scopeRequest,
		)
		if err != nil {
			return "", err
		}
		if view, _ := args["view"].(string); view == "governor" {
			// Prompt-budgeted projection for host-side prompt governors;
			// deliberately skips coverage and the navigation strip.
			response, responseErr := governorStatusResponse(
				ctx,
				store,
				codeIntelService,
				contextName,
				projectRoot,
				readiness,
			)
			if responseErr != nil {
				return "", responseErr
			}
			return statusWithLiveMCPReceipt(projectRoot, response)
		}
		profileInspection, err := inspectStatusProjectProfile(
			ctx,
			projectRoot,
			readiness,
		)
		if err != nil {
			return "", err
		}
		full, _ := args["full"].(bool)
		data, err := fetchBoundedProjectStatusData(
			ctx,
			store,
			codeIntelService,
			contextName,
			projectRoot,
		)
		if err != nil {
			return "", err
		}
		data.SpecBindingDebt = specBindingDebtReportForCanonicalStatus(
			ctx,
			store,
			readiness,
		)
		data = applyDefaultDriftEventResolutionLedgerToStatusData(ctx, store, projectRoot, data)
		statusBody := present.CockpitStatusResponse(data)
		if full {
			statusBody = present.StatusResponse(data)
		}
		result := overseerStatusPrefix(
			ctx,
			store,
			projectRoot,
			readiness,
		) +
			statusProjectProfilePrefix(
				readiness,
				profileInspection,
				full,
			) +
			statusBody
		coverageRequired, err := profileCodeCoverageRequired(readiness)
		if err != nil {
			return "", err
		}
		if coverageRequired {
			scanner := codebase.NewScanner(store.DB())
			if !scanner.ModulesLastScanned(ctx).IsZero() {
				if report, err := codebase.ComputeCoverage(ctx, store.DB()); err == nil && report.TotalModules > 0 {
					if full {
						result += "\n" + codebase.FormatCoverageResponse(report)
					} else {
						result += "\n" + codebase.FormatCoverageCockpitSummary(report)
					}
				}
			}
		}
		result += "\n" + haftServeRuntimeStatusLine()
		return statusWithLiveMCPReceipt(
			projectRoot,
			result+navStripWithStaleSnapshot(ctx, store, contextName, data.StaleItems),
		)

	case "board":
		boardView, _ := args["view"].(string)
		boardData, err := ui.LoadBoardData(store, store.DB(), haftDir, "")
		if err != nil {
			return "", fmt.Errorf("load board data: %w", err)
		}
		switch boardView {
		case "decisions":
			return present.BoardDecisions(boardData), nil
		case "problems":
			return present.BoardProblems(boardData), nil
		case "coverage":
			return present.BoardCoverage(boardData), nil
		case "evidence":
			return present.BoardEvidence(boardData), nil
		case "full":
			return present.BoardFull(boardData), nil
		default:
			return present.BoardOverview(boardData), nil
		}

	case "related":
		artifactRef := firstNonEmptyQueryArg(args, "artifact_ref", "ref", "artifact_id")
		if artifactRef != "" {
			return artifactQueryContractResponse(ctx, store, artifactRef)
		}

		file, _ := args["file"].(string)
		results, err := artifact.FetchRelatedArtifacts(ctx, store, file)
		if err != nil {
			return "", err
		}
		resp := present.RelatedResponse(results, file)
		if file != "" {
			svc := codeIntelService
			projectRoot := filepath.Dir(haftDir)
			// Phase-2 graph-proximity recall (dec-20260604-3aaad199): FTS5-seeded
			// PPR over the fused graph, additive to the exact affected-file list.
			// Best-effort — a failure never breaks the related response.
			if view, perr := svc.RelatedView(ctx, projectRoot, file, 12); perr == nil && len(view.Results) > 0 {
				// Dedup: drop anything already shown in the exact affected-file
				// section, so a decision is not listed twice.
				shown := make(map[string]bool, len(results))
				for _, a := range results {
					shown[a.Meta.ID] = true
				}
				items := make([]present.RelatedProximityItem, 0, len(view.Results))
				for _, r := range view.Results {
					if shown[r.ID] {
						continue
					}
					label := "reasoning"
					if r.Kind == codeintel.RelatedSymbol {
						label = "symbol"
					}
					items = append(items, present.RelatedProximityItem{Title: r.Title, Label: label, Ref: r.ID})
				}
				resp += present.IndexStateResponse(view.Index)
				resp += present.IndexCoordinationResponse(view.IndexRefresh)
				resp += present.RelatedProximityResponse(items)
			}
			// Structural test-coverage lane (dec-20260604-ef966a11): which tests
			// exercise this file's symbols — 'exercised by', never 'verified'.
			if view, cerr := svc.TestCoverageView(ctx, projectRoot, file); cerr == nil && len(view.Symbols) > 0 {
				items := make([]present.TestedByItem, 0, len(view.Symbols))
				for _, c := range view.Symbols {
					items = append(items, present.TestedByItem{Symbol: c.Symbol, Exported: c.Exported, TestedBy: c.TestedBy})
				}
				resp += present.IndexStateResponse(view.Index)
				resp += present.IndexCoordinationResponse(view.IndexRefresh)
				resp += present.TestedByResponse(items)
			}
		}
		return resp + navStrip, nil

	case "code_context":
		return handleCodeContextQueryWithCodeIntel(
			ctx,
			store,
			haftDir,
			args,
			navStrip,
			codeIntelService,
		)

	case "callees", "callers", "impact":
		name := firstNonEmptyQueryArg(args, "symbol", "name")
		file, _ := args["file"].(string)
		line := 0
		if l, ok := args["line"].(float64); ok {
			line = int(l)
		}
		projectRoot := filepath.Dir(haftDir)
		if anchorID, _ := args["anchor_id"].(string); anchorID != "" && name == "" {
			resolved, err := resolveCurrentSymbolAnchorWithCodeIntel(
				ctx,
				store,
				projectRoot,
				anchorID,
				codeIntelService,
			)
			if err != nil {
				return "", err
			}
			name = resolved.Name
			file = resolved.FilePath
			line = resolved.StartLine
		}
		if name == "" {
			return "", fmt.Errorf("symbol is required for %s action", action)
		}
		depth := 0
		if d, ok := args["depth"].(float64); ok {
			depth = int(d)
		}
		dir := codeintel.Callees
		if action != "callees" {
			dir = codeintel.Callers // callers + impact both walk inbound edges
		}
		profileRaw, _ := args["profile"].(string)
		profile, validProfile := codeintel.ParseTraversalProfile(profileRaw)
		if !validProfile {
			return "", fmt.Errorf("unknown traversal profile %q; valid profiles: call_flow, type_impact, reference_impact, all_code", profileRaw)
		}
		res, err := codeIntelService.FlowWithProfile(
			ctx,
			projectRoot,
			name,
			file,
			line,
			depth,
			dir,
			profile,
		)
		if err != nil {
			return "", err
		}
		return present.FlowResponse(res, action, name) + navStrip, nil

	case "node":
		name := firstNonEmptyQueryArg(args, "symbol", "name")
		file, _ := args["file"].(string)
		line := 0
		if l, ok := args["line"].(float64); ok {
			line = int(l)
		}
		projectRoot := filepath.Dir(haftDir)
		if anchorID, _ := args["anchor_id"].(string); anchorID != "" && name == "" {
			resolved, err := resolveCurrentSymbolAnchorWithCodeIntel(
				ctx,
				store,
				projectRoot,
				anchorID,
				codeIntelService,
			)
			if err != nil {
				return "", err
			}
			name = resolved.Name
			file = resolved.FilePath
			line = resolved.StartLine
		}
		if name == "" {
			return "", fmt.Errorf("symbol is required for node action")
		}
		view, err := codeIntelService.Node(
			ctx,
			projectRoot,
			name,
			file,
			line,
		)
		if err != nil {
			return "", err
		}
		return present.NodeResponse(view, nodeLang(file, view)) + navStrip, nil

	case "explore":
		concern, _ := args["query"].(string)
		name := firstNonEmptyQueryArg(args, "symbol", "name")
		file, _ := args["file"].(string)
		line := 0
		if l, ok := args["line"].(float64); ok {
			line = int(l)
		}
		projectRoot := filepath.Dir(haftDir)
		if anchorID, _ := args["anchor_id"].(string); anchorID != "" && name == "" {
			if strings.TrimSpace(concern) != "" {
				return "", fmt.Errorf(
					"explore accepts a concern query or anchor_id, not both",
				)
			}
			resolved, err := resolveCurrentSymbolAnchorWithCodeIntel(
				ctx,
				store,
				projectRoot,
				anchorID,
				codeIntelService,
			)
			if err != nil {
				return "", err
			}
			name = resolved.Name
			file = resolved.FilePath
			line = resolved.StartLine
		}
		maxCandidates := codeintel.DefaultConcernCandidateBudget
		if _, present := args["max_candidates"]; present {
			parsedCandidates, budgetErr := nonNegativeIntArg(
				args,
				"max_candidates",
			)
			if budgetErr != nil {
				return "", budgetErr
			}
			maxCandidates = parsedCandidates
		}
		wire, err := publishCodeExploreWithService(
			ctx,
			store,
			codeIntelService,
			projectRoot,
			name,
			file,
			line,
			concern,
			maxCandidates,
			stringExploreView(args),
			stringExploreTraceRef(args),
		)
		if err != nil {
			return "", err
		}
		return string(wire), nil

	case "ceremony":
		files := ceremonyFiles(args)
		if len(files) == 0 {
			return "", fmt.Errorf("files (array) or a space/comma-separated file is required for ceremony action")
		}
		projectRoot := filepath.Dir(haftDir)
		gov := func(f string) ceremony.GovFacts {
			arts, err := store.SearchByAffectedFile(ctx, f)
			if err != nil {
				return ceremony.GovFacts{}
			}
			for _, a := range arts {
				// A file governed by an active decision is higher-stakes → at
				// least standard. (Conservative + precise: governed-presence
				// only; recorded low-reversibility → High is a follow-up that
				// needs the decision→problem body parse, deferred to avoid a
				// coarse body-scan false-High.)
				if a.Meta.Kind == artifact.KindDecisionRecord && a.Meta.Status == artifact.StatusActive {
					return ceremony.GovFacts{Reversibility: "medium"}
				}
			}
			return ceremony.GovFacts{}
		}
		rec := ceremony.Recommend(projectRoot, files, gov)
		return present.CeremonyResponse(rec, files) + navStrip, nil

	case "projection":
		viewName, _ := args["view"].(string)
		view, err := artifact.ParseProjectionView(viewName)
		if err != nil {
			return "", err
		}
		graph, err := artifact.FetchProjectionGraph(ctx, store, contextName)
		if err != nil {
			return "", err
		}
		return present.ProjectionResponse(graph, view) + navStrip, nil

	case "list":
		kind, _ := args["kind"].(string)
		limit := 50
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		data, err := artifact.FetchListData(ctx, store, kind, limit)
		if err != nil {
			return "", err
		}
		return present.ListResponse(data) + navStrip, nil

	case "coverage":
		projectRoot := filepath.Dir(haftDir)
		scopeRequest, err := projectSpecificationScopeRequestFromFlag(
			stringArg(args, "scope_id"),
		)
		if err != nil {
			return "", err
		}
		readiness, err := inspectCanonicalProjectReadiness(
			ctx,
			projectRoot,
			scopeRequest,
		)
		if err != nil {
			return "", err
		}
		response, err := profileAwareCoverageResponse(
			ctx,
			store,
			projectRoot,
			readiness,
			intArg(args, "limit", 0),
		)
		if err != nil {
			return "", err
		}
		return response + navStrip, nil

	case "fpf":
		request, err := fpfQueryRequestFromArgs(args)
		if err != nil {
			return "", err
		}
		publicationRequest, err := fpfQueryPublicationRequestFromArgs(args)
		if err != nil {
			return "", err
		}
		payload, err := encodeEmbeddedFPFQuery(
			request,
			publicationRequest,
			fpf.PublishedQueryJSONCompact,
		)
		if err != nil {
			return "", fmt.Errorf("FPF query: %w", err)
		}
		return string(payload), nil

	case "check":
		projectRoot := filepath.Dir(haftDir)
		report, err := buildCheckReport(ctx, store, projectRoot)
		if err != nil {
			return "", fmt.Errorf("build check report: %w", err)
		}
		report = normalizeCheckReport(report)
		payload, err := json.Marshal(report)
		if err != nil {
			return "", fmt.Errorf("marshal check report: %w", err)
		}
		return string(payload), nil

	case "carrier_manifest":
		manifest := project.DefaultCarrierAuthorityManifest()
		findings := project.ValidateCarrierAuthorityManifest(manifest)
		if len(findings) > 0 {
			return "", fmt.Errorf("carrier manifest invalid: %v", findings)
		}
		payload, err := project.CarrierAuthorityManifestJSON(manifest)
		if err != nil {
			return "", fmt.Errorf("marshal carrier manifest: %w", err)
		}
		return string(payload), nil

	case "carrier_check":
		projectRoot := filepath.Dir(haftDir)
		result, err := project.CheckCarrierSemioWithVirtualTexts(projectRoot, carrierCheckGeneratedSurfaces())
		if err != nil {
			return "", fmt.Errorf("check carrier semio: %w", err)
		}
		payload, err := project.CarrierSemioCheckResultJSON(result)
		if err != nil {
			return "", fmt.Errorf("marshal carrier semio check: %w", err)
		}
		return string(payload), nil

	case "contract_audit":
		record := buildInterfaceContractAuditReport(haftInterfaceCatalog())
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal interface contract audit: %w", err)
		}
		return string(payload), nil

	case "contract_generation":
		record := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal interface contract generation manifest: %w", err)
		}
		return string(payload), nil

	case "spec_review":
		projectRoot := filepath.Dir(haftDir)
		packet, err := buildSpecReviewPacket(projectRoot)
		if err != nil {
			return "", fmt.Errorf("build spec review packet: %w", err)
		}
		payload, err := json.Marshal(packet)
		if err != nil {
			return "", fmt.Errorf("marshal spec review packet: %w", err)
		}
		return string(payload), nil

	case "spec_validate":
		projectRoot := filepath.Dir(haftDir)
		report, err := buildSpecValidationReport(projectRoot)
		if err != nil {
			return "", fmt.Errorf("build spec validation report: %w", err)
		}
		payload, err := json.Marshal(report)
		if err != nil {
			return "", fmt.Errorf("marshal spec validation report: %w", err)
		}
		return string(payload), nil

	case "spec_use":
		projectRoot := filepath.Dir(haftDir)
		var gate specflow.OperationalGateProfile
		gatePresent, err := decodeStrictArgFromArgs(args, "operational_gate", &gate)
		if err != nil {
			return "", fmt.Errorf("parse operational_gate: %w", err)
		}
		var gatePtr *specflow.OperationalGateProfile
		if gatePresent {
			gatePtr = &gate
		}
		record, err := buildSpecUseRecord(
			ctx,
			projectRoot,
			stringArg(args, "section_id"),
			stringArg(args, "use_context"),
			stringArg(args, "policy"),
			stringArg(args, "waiver_expires_at"),
			gatePtr,
			time.Now().UTC(),
			store,
			nil,
		)
		if err != nil {
			return "", fmt.Errorf("build spec use record: %w", err)
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal spec use record: %w", err)
		}
		return string(payload), nil

	case "spec_trace":
		record, err := buildSpecTraceRecord(ctx, filepath.Dir(haftDir), store, stringArg(args, "section_id"))
		if err != nil {
			return "", fmt.Errorf("build spec trace record: %w", err)
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal spec trace record: %w", err)
		}
		return string(payload), nil

	case "spec_binding_preflight":
		return handleQuintQuerySpecBindingPreflight(ctx, store, haftDir, args)

	case "spec_fit_probe":
		return handleQuintQuerySpecFitProbe(haftDir, args)

	case "change_case":
		decisionRef := stringArg(args, "artifact_ref")
		if decisionRef == "" {
			decisionRef = stringArg(args, "decision_ref")
		}
		record, err := buildEngineeringChangeCase(
			ctx,
			store,
			artifact.EngineeringChangeCaseInput{
				DecisionRef:  decisionRef,
				AttemptedUse: stringArg(args, "attempted_use"),
				ProducerRef:  stringArg(args, "producer_ref"),
				MethodRef:    stringArg(args, "method_ref"),
				WorkRef:      stringArg(args, "work_ref"),
			},
			time.Now().UTC(),
		)
		if err != nil {
			return "", fmt.Errorf("build engineering change case: %w", err)
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal engineering change case: %w", err)
		}
		return string(payload), nil

	case "correspondence_graph":
		decisionRef := stringArg(args, "artifact_ref")
		if decisionRef == "" {
			decisionRef = stringArg(args, "decision_ref")
		}
		record, err := buildQualifiedCorrespondenceGraph(
			ctx,
			store,
			artifact.CorrespondenceGraphInput{DecisionRef: decisionRef},
			time.Now().UTC(),
		)
		if err != nil {
			return "", fmt.Errorf("build correspondence graph: %w", err)
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal correspondence graph: %w", err)
		}
		return string(payload), nil

	case "drift_route":
		record := artifact.BuildSemanticDriftRoute(artifact.DriftRouteInput{
			DriftKind:  stringArg(args, "drift_kind"),
			BearerRef:  stringArg(args, "bearer_ref"),
			UseContext: stringArg(args, "use_context"),
		})
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal drift route: %w", err)
		}
		return string(payload), nil

	case "drift_events":
		projectRoot := filepath.Dir(haftDir)
		reports, err := artifact.CheckDrift(ctx, store, projectRoot)
		if err != nil {
			return "", fmt.Errorf("scan drift: %w", err)
		}
		ledger, err := readDriftEventResolutionLedger(driftEventResolutionLedgerPath(projectRoot, ""))
		if err != nil {
			return "", err
		}
		record := buildDriftEventReportWithResolutionLedger(reports, ledger, timeNow())
		full, _ := args["full"].(bool)
		if !full {
			record = artifact.CompactDriftEventReport(record, compactProjectionLimit(args))
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal drift event report: %w", err)
		}
		return string(payload), nil

	case "decision_reconcile":
		record, err := artifact.BuildDecisionReconciliationPlan(ctx, store)
		if err != nil {
			return "", fmt.Errorf("build decision reconciliation plan: %w", err)
		}
		full, _ := args["full"].(bool)
		if !full {
			record = artifact.CompactDecisionReconciliationPlan(record, compactProjectionLimit(args))
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal decision reconciliation plan: %w", err)
		}
		return string(payload), nil

	case "governing_set":
		record, err := artifact.BuildCurrentGoverningSetReportFiltered(ctx, store, governingSetFilterFromArgs(args))
		if err != nil {
			return "", fmt.Errorf("build current governing set: %w", err)
		}
		full, _ := args["full"].(bool)
		if !full {
			record = artifact.CompactCurrentGoverningSetReport(record, compactProjectionLimit(args))
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal current governing set: %w", err)
		}
		return string(payload), nil

	case "blocked_use":
		nextActions := parseStringArrayFromArgs(args, "next_actions")
		if len(nextActions) == 0 {
			nextActions = parseStringArrayFromArgs(args, "next_admissible_actions")
		}
		label := stringArg(args, "label")
		if label == "" {
			label = stringArg(args, "entity_or_subject_label")
		}
		roleRef := stringArg(args, "role_ref")
		if roleRef == "" {
			roleRef = stringArg(args, "required_role_assignment_ref")
		}
		record := artifact.BuildBlockedUseAttentionItem(artifact.BlockedUseAttentionInput{
			BearerRef:                 stringArg(args, "bearer_ref"),
			EntityOrSubjectLabel:      label,
			FindingKind:               stringArg(args, "finding_kind"),
			BlockedUse:                stringArg(args, "blocked_use"),
			SourceRefs:                parseStringArrayFromArgs(args, "source_refs"),
			ExactRecordNeeded:         stringArg(args, "exact_record_needed"),
			NextAdmissibleActions:     nextActions,
			RequiredRoleAssignmentRef: roleRef,
			ValidUntil:                stringArg(args, "valid_until"),
		})
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal blocked-use attention item: %w", err)
		}
		return string(payload), nil

	case "value_space":
		bearerRef := stringArg(args, "bearer_ref")
		if bearerRef == "" {
			bearerRef = stringArg(args, "artifact_ref")
		}
		evidenceRefs := parseStringArrayFromArgs(args, "source_refs")
		if len(evidenceRefs) == 0 {
			evidenceRefs = parseStringArrayFromArgs(args, "evidence_refs")
		}
		window := stringArg(args, "context")
		if window == "" {
			window = stringArg(args, "window")
		}
		record := artifact.BuildEngineeringValueSpace(artifact.EngineeringValueSpaceInput{
			BearerRef:    bearerRef,
			Window:       window,
			MethodRef:    stringArg(args, "method_ref"),
			EvidenceRefs: evidenceRefs,
		})
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal engineering value space: %w", err)
		}
		return string(payload), nil

	case "evidence_path":
		requiresCurrentFormality, _ := args["requires_current_formality"].(bool)
		record, err := buildEvidencePathRecord(
			ctx,
			store,
			artifact.EvidencePathInput{
				ArtifactRef:              stringArg(args, "artifact_ref"),
				EvidenceRef:              stringArg(args, "evidence_ref"),
				ClaimRef:                 stringArg(args, "claim_ref"),
				AttemptedUse:             stringArg(args, "attempted_use"),
				RequiresCurrentFormality: requiresCurrentFormality,
				ProducerRef:              stringArg(args, "producer_ref"),
				MethodRef:                stringArg(args, "method_ref"),
				WorkRef:                  stringArg(args, "work_ref"),
			},
			time.Now().UTC(),
		)
		if err != nil {
			return "", fmt.Errorf("build evidence path record: %w", err)
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal evidence path record: %w", err)
		}
		return string(payload), nil

	case "resolve_term":
		return handleQuintQueryResolveTerm(ctx, store, haftDir, args)

	default:
		return "", fmt.Errorf("unknown action %q — use 'search', 'status', 'related', 'code_context', 'callees', 'callers', 'impact', 'node', 'explore', 'ceremony', 'projection', 'list', 'coverage', 'fpf', 'check', 'carrier_manifest', 'carrier_check', 'contract_audit', 'contract_generation', 'spec_review', 'spec_validate', 'spec_use', 'spec_trace', 'spec_binding_preflight', 'spec_fit_probe', 'change_case', 'correspondence_graph', 'drift_route', 'drift_events', 'decision_reconcile', 'governing_set', 'blocked_use', 'value_space', 'evidence_path', or 'resolve_term'", action)
	}
}

func statusWithLiveMCPReceipt(projectRoot string, response string) (string, error) {
	posture, err := agenthostrestart.FulfillLiveMCPChallengeForStatus(projectRoot)
	if err != nil {
		return "", fmt.Errorf("fulfill live MCP restart challenge: %w", err)
	}
	if posture == agenthostrestart.LiveMCPStatusPredecessorIgnored {
		response += "\nRestart acceptance: stale goal-coupled checkpoint ignored; it grants no current restart authority."
	}
	return response, nil
}

func handleQuintQuerySpecFitProbe(
	haftDir string,
	args map[string]any,
) (string, error) {
	projectRoot := filepath.Dir(haftDir)
	specSet, err := loadProjectSpecificationSetSQLFirst(projectRoot)
	if err != nil {
		return "", fmt.Errorf("load ProjectSpecificationSet for spec_fit_probe: %w", err)
	}

	input, err := specFitProbeInputFromArgs(args)
	if err != nil {
		return "", err
	}
	result := specflow.BuildSpecFitProbe(specSet, input)
	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal spec_fit_probe result: %w", err)
	}
	return string(payload), nil
}

func specFitProbeInputFromArgs(args map[string]any) (specflow.SpecFitProbeInput, error) {
	var input specflow.SpecFitProbeInput
	if _, err := parseJSONArg(args, "probe", &input); err != nil {
		return specflow.SpecFitProbeInput{}, err
	}
	if input.ProblemSignal == "" {
		input.ProblemSignal = stringArg(args, "problem_signal")
	}
	if input.Scope == "" {
		input.Scope = stringArg(args, "scope")
	}
	if input.Mode == "" {
		input.Mode = stringArg(args, "mode")
	}
	if input.DeclaredRelation == "" {
		input.DeclaredRelation = stringArg(args, "declared_relation")
	}
	if len(input.SectionRefs) == 0 {
		input.SectionRefs = parseStringArrayFromArgs(args, "section_refs")
	}
	if len(input.AffectedFiles) == 0 {
		input.AffectedFiles = parseStringArrayFromArgs(args, "affected_files")
	}
	if len(input.TargetRefs) == 0 {
		input.TargetRefs = parseStringArrayFromArgs(args, "target_refs")
	}
	if len(input.ConflictRefs) == 0 {
		input.ConflictRefs = parseStringArrayFromArgs(args, "conflict_refs")
	}
	if len(input.Variants) == 0 {
		if _, err := parseJSONArg(args, "variants", &input.Variants); err != nil {
			return specflow.SpecFitProbeInput{}, err
		}
	}
	return input, nil
}

func handleQuintQuerySpecBindingPreflight(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	args map[string]any,
) (string, error) {
	projectRoot := filepath.Dir(haftDir)
	specSet, err := loadProjectSpecificationSetSQLFirst(projectRoot)
	if err != nil {
		return "", fmt.Errorf("load ProjectSpecificationSet for spec_binding_preflight: %w", err)
	}

	input, err := specBindingPreflightInputFromArgs(args)
	if err != nil {
		return "", err
	}
	input.DecisionDraft = enrichSpecBindingDecisionDraft(ctx, store, input.DecisionDraft)
	result := specflow.BuildSpecBindingPreflight(specSet, input)
	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal spec_binding_preflight result: %w", err)
	}
	return string(payload), nil
}

func specBindingPreflightInputFromArgs(
	args map[string]any,
) (specflow.SpecBindingPreflightInput, error) {
	var draft specflow.SpecBindingDecisionDraft
	if _, err := parseJSONArg(args, "decision_draft", &draft); err != nil {
		return specflow.SpecBindingPreflightInput{}, err
	}

	if draft.SelectedTitle == "" {
		draft.SelectedTitle = stringArg(args, "selected_title")
	}
	if draft.WhySelected == "" {
		draft.WhySelected = stringArg(args, "why_selected")
	}
	if draft.CounterArgument == "" {
		draft.CounterArgument = stringArg(args, "counterargument")
	}
	if draft.WeakestLink == "" {
		draft.WeakestLink = stringArg(args, "weakest_link")
	}
	if draft.Mode == "" {
		draft.Mode = stringArg(args, "mode")
	}
	if draft.LoadBearingLevel == "" {
		draft.LoadBearingLevel = stringArg(args, "load_bearing_level")
	}
	if draft.DecisionSubjectRef == "" {
		draft.DecisionSubjectRef = stringArg(args, "decision_subject_ref")
	}
	if draft.PortfolioRef == "" {
		draft.PortfolioRef = stringArg(args, "portfolio_ref")
	}
	if len(draft.ProblemRefs) == 0 {
		if problemRef := stringArg(args, "problem_ref"); problemRef != "" {
			draft.ProblemRefs = append(draft.ProblemRefs, problemRef)
		}
		draft.ProblemRefs = append(draft.ProblemRefs, parseStringArrayFromArgs(args, "problem_refs")...)
	}
	if len(draft.ActiveDecisionRefs) == 0 {
		draft.ActiveDecisionRefs = parseStringArrayFromArgs(args, "active_decision_refs")
	}
	if draft.SearchKeywords == "" {
		draft.SearchKeywords = stringArg(args, "search_keywords")
	}
	if draft.BindingScope == "" {
		draft.BindingScope = stringArg(args, "binding_scope")
	}
	if draft.BindingFallbackReason == "" {
		draft.BindingFallbackReason = stringArg(args, "binding_fallback_reason")
	}
	if draft.DeclaredRelation == "" {
		draft.DeclaredRelation = stringArg(args, "declared_relation")
	}
	if len(draft.SectionRefs) == 0 {
		draft.SectionRefs = parseStringArrayFromArgs(args, "section_refs")
	}
	if len(draft.LinkedSectionRefs) == 0 {
		draft.LinkedSectionRefs = parseStringArrayFromArgs(args, "linked_section_refs")
	}
	if len(draft.LineageSectionRefs) == 0 {
		draft.LineageSectionRefs = parseStringArrayFromArgs(args, "active_decision_lineage_section_refs")
	}
	if len(draft.AffectedFiles) == 0 {
		draft.AffectedFiles = parseStringArrayFromArgs(args, "affected_files")
	}
	if len(draft.BindingHints) == 0 {
		draft.BindingHints = parseStringArrayFromArgs(args, "binding_hints")
	}
	if len(draft.BindingTargetRefs) == 0 {
		draft.BindingTargetRefs = parseStringArrayFromArgs(args, "binding_target_refs")
	}
	if len(draft.GovernanceTargetRefs) == 0 {
		draft.GovernanceTargetRefs = parseStringArrayFromArgs(args, "governance_target_refs")
	}
	if len(draft.ConflictRefs) == 0 {
		draft.ConflictRefs = parseStringArrayFromArgs(args, "conflict_refs")
	}

	return specflow.SpecBindingPreflightInput{DecisionDraft: draft}, nil
}

// nodeLang derives the markdown code-fence language for a node view from the
// file argument, falling back to the first overload's file extension.
func nodeLang(file string, view codeintel.NodeView) string {
	ext := filepath.Ext(file)
	if ext == "" && len(view.Overloads) > 0 {
		ext = filepath.Ext(view.Overloads[0].Symbol.FilePath)
	}
	return strings.TrimPrefix(ext, ".")
}

// ceremonyFiles extracts the touched-file set for the ceremony action: a
// `files` array if given, else a space/comma-separated `file` string.
func ceremonyFiles(args map[string]any) []string {
	if raw, ok := args["files"].([]any); ok {
		out := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if f, _ := args["file"].(string); strings.TrimSpace(f) != "" {
		return splitSeedBag(f)
	}
	return nil
}

// splitSeedBag splits an explore symbol argument into a bag of seed names on
// whitespace or commas (so "FrameProblem Create" or "FrameProblem, Create"
// both work), dropping empties. A single token yields a one-element slice.
func splitSeedBag(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ','
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func resolveCurrentSymbolAnchorWithCodeIntel(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	anchorID string,
	codeIntelService *codeintel.Service,
) (codebase.CodeSymbol, error) {
	var lastErr error
	for range 2 {
		if _, err := codeIntelService.EnsureIndex(ctx, projectRoot); err != nil {
			return codebase.CodeSymbol{}, fmt.Errorf(
				"refresh code index for anchor lookup: %w",
				err,
			)
		}
		indexState, err := codeIntelService.CurrentIndexState(ctx)
		if err != nil {
			return codebase.CodeSymbol{}, err
		}
		resolved, ok, err := codebase.NewSymbolStore(
			store.DB(),
		).GetByID(ctx, anchorID)
		if err != nil {
			return codebase.CodeSymbol{}, fmt.Errorf(
				"resolve symbol anchor: %w",
				err,
			)
		}
		if err := codeIntelService.ConfirmIndexState(ctx, indexState); err != nil {
			var changed *codeintel.IndexBasisChangedError
			if !errors.As(err, &changed) {
				return codebase.CodeSymbol{}, err
			}
			lastErr = err
			continue
		}
		if ok {
			return resolved, nil
		}
		if !indexState.SupportsKnownAbsence() {
			return codebase.CodeSymbol{}, fmt.Errorf(
				"symbol_anchor_unavailable: %q was not resolved under incomplete index basis %s",
				anchorID,
				indexState.Basis.CoverageRef(),
			)
		}
		return codebase.CodeSymbol{}, fmt.Errorf(
			"symbol anchor %q not found in the current code index basis %s",
			anchorID,
			indexState.Basis.CoverageRef(),
		)
	}
	return codebase.CodeSymbol{}, lastErr
}

func firstNonEmptyQueryArg(args map[string]any, keys ...string) string {
	for _, key := range keys {
		value, _ := args[key].(string)
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func artifactQueryContractResponse(ctx context.Context, store *artifact.Store, artifactRef string) (string, error) {
	item, err := store.Get(ctx, artifactRef)
	if errors.Is(err, sql.ErrNoRows) {
		return "", exactArtifactMissError(artifactRef)
	}
	if err != nil {
		return "", err
	}

	key := "artifact"
	if item.Meta.Kind == artifact.KindProblemCard {
		key = "problem_card"
	}

	encoded, err := json.Marshal(map[string]any{
		key: artifactQueryContractPayload(item),
	})
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func exactArtifactMissError(artifactRef string) error {
	return fmt.Errorf("artifact_not_found: %s (exact lookup; semantic fallback was not used)", artifactRef)
}

func artifactQueryContractPayload(item *artifact.Artifact) map[string]any {
	meta := map[string]any{
		"id":          item.Meta.ID,
		"kind":        string(item.Meta.Kind),
		"version":     item.Meta.Version,
		"status":      string(item.Meta.Status),
		"context":     item.Meta.Context,
		"mode":        string(item.Meta.Mode),
		"title":       item.Meta.Title,
		"valid_until": item.Meta.ValidUntil,
		"created_at":  item.Meta.CreatedAt.Format(time.RFC3339),
		"updated_at":  item.Meta.UpdatedAt.Format(time.RFC3339),
		"links":       item.Meta.Links,
	}

	payload := map[string]any{
		"id":              item.Meta.ID,
		"kind":            string(item.Meta.Kind),
		"version":         item.Meta.Version,
		"status":          string(item.Meta.Status),
		"context":         item.Meta.Context,
		"mode":            string(item.Meta.Mode),
		"title":           item.Meta.Title,
		"valid_until":     item.Meta.ValidUntil,
		"created_at":      item.Meta.CreatedAt.Format(time.RFC3339),
		"updated_at":      item.Meta.UpdatedAt.Format(time.RFC3339),
		"body":            item.Body,
		"content":         item.Body,
		"description":     item.Body,
		"search_keywords": item.SearchKeywords,
		"structured_data": item.StructuredData,
		"meta":            meta,
	}
	if item.Meta.Kind == artifact.KindProblemCard {
		payload["semantic"] = artifact.ProblemSemanticEnvelopeForArtifact(item)
		payload["views"] = artifactProblemSemanticViews(item)
	}

	return payload
}

func artifactProblemSemanticViews(item *artifact.Artifact) map[string]any {
	fields := item.UnmarshalProblemFields()
	semantic := artifact.ProblemSemanticEnvelopeForArtifact(item)
	carrierBytes := map[string]any{
		"carrier_kind": semantic.CarrierBinding.CarrierKind,
		"carrier_ref":  semantic.CarrierBinding.CarrierRef,
		"hash":         semantic.PublicationUnit.CarrierHash,
	}

	return map[string]any{
		"working": map[string]any{
			"id":                  item.Meta.ID,
			"title":               item.Meta.Title,
			"semantic_status":     string(semantic.Status),
			"profile_id":          semantic.Profile.ID,
			"problem_profile":     problemProfileLevel(fields),
			"p2w_readiness":       problemReadinessLabel(fields),
			"source_edition_hash": semantic.SemanticEdition.Hash,
			"publication_hash":    semantic.PublicationUnit.PublicationHash,
			"signal":              fields.Signal,
			"acceptance":          fields.Acceptance,
		},
		"exact": map[string]any{
			"id":                     item.Meta.ID,
			"semantic":               semantic,
			"problem_fields":         fields,
			"source_episteme":        semantic.SemanticEdition,
			"publication_projection": semantic.PublicationProjection,
			"publication_unit":       semantic.PublicationUnit,
			"carrier_bytes":          carrierBytes,
			"carrier_binding":        semantic.CarrierBinding,
			"reference_scheme":       semantic.ReferenceScheme,
		},
		"audit": map[string]any{
			"id":                     item.Meta.ID,
			"semantic_status":        string(semantic.Status),
			"profile":                semantic.Profile,
			"source_episteme":        semantic.SemanticEdition,
			"semantic_edition":       semantic.SemanticEdition,
			"publication_projection": semantic.PublicationProjection,
			"publication_unit":       semantic.PublicationUnit,
			"carrier_bytes":          carrierBytes,
			"carrier_binding":        semantic.CarrierBinding,
			"reference_scheme":       semantic.ReferenceScheme,
			"warnings":               semantic.Warnings,
		},
	}
}

func problemProfileLevel(fields artifact.ProblemFields) string {
	if fields.Profile == nil {
		return ""
	}

	return fields.Profile.Level
}

func problemReadinessLabel(fields artifact.ProblemFields) string {
	if fields.Profile == nil {
		return ""
	}

	return fields.Profile.Readiness
}

func governingSetFilterFromArgs(args map[string]any) artifact.CurrentGoverningSetFilter {
	targetRefs := parseStringArrayFromArgs(args, "source_refs")
	targetRef := ""
	if len(targetRefs) > 0 {
		targetRef = targetRefs[0]
	}
	return artifact.CurrentGoverningSetFilter{
		Query:      stringArg(args, "query"),
		SubjectRef: stringArg(args, "bearer_ref"),
		TargetRef:  targetRef,
	}
}

// parseStringArrayFromArgs handles MCP client serialization differences.
// Some clients send JSON arrays as parsed []any, others as raw JSON strings.
func parseStringArrayFromArgs(args map[string]any, key string) []string {
	if items, ok := args[key].([]any); ok {
		var result []string
		for _, item := range items {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if s, ok := args[key].(string); ok && len(s) > 0 && s[0] == '[' {
		logger.Debug().Str("key", key).Str("raw_type", "string").Msg("parseStringArrayFromArgs: JSON string fallback")
		var parsed []string
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			return parsed
		}
	}
	return nil
}

func parseStrictStringArrayFromArgs(args map[string]any, key string) ([]string, error) {
	var values []string

	present, err := decodeStrictArgFromArgs(args, key, &values)
	if err != nil {
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
	if !present {
		return nil, nil
	}

	return values, nil
}

func fpfQueryRequestFromArgs(args map[string]any) (fpf.QueryRequest, error) {
	mode := strings.TrimSpace(stringArg(args, "mode"))
	if mode == "" {
		return nil, fmt.Errorf("mode is required for FPF query; expected concern, lookup, or inspect")
	}

	switch fpf.QueryMode(mode) {
	case fpf.QueryModeConcern:
		if _, hasRoles := args["roles"]; hasRoles {
			return nil, fmt.Errorf("roles are not accepted for concern mode; use lookup or inspect to hydrate a selected source role")
		}
		knownContext, err := parseStrictStringArrayFromArgs(args, "known_context")
		if err != nil {
			return nil, err
		}
		budget, err := fpfResponseBudgetFromArgs(args)
		if err != nil {
			return nil, err
		}
		return fpf.ConcernQuery{
			Text:            stringArg(args, "query"),
			EntityOfConcern: stringArg(args, "entity_of_concern"),
			KnownContext:    knownContext,
			IntendedUse:     stringArg(args, "intended_use"),
			ResponseBudget:  budget,
		}, nil

	case fpf.QueryModeLookup:
		roleValues, err := parseStrictStringArrayFromArgs(args, "roles")
		if err != nil {
			return nil, err
		}
		budget, err := fpfResponseBudgetFromArgs(args)
		if err != nil {
			return nil, err
		}
		return fpf.LookupQuery{
			Identifier:     stringArg(args, "identifier"),
			Roles:          sourceUnitRoles(roleValues),
			ResponseBudget: budget,
		}, nil

	case fpf.QueryModeInspect:
		roleValues, err := parseStrictStringArrayFromArgs(args, "roles")
		if err != nil {
			return nil, err
		}
		return fpf.InspectQuery{
			Identifier: stringArg(args, "identifier"),
			Roles:      sourceUnitRoles(roleValues),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported FPF query mode %q; expected concern, lookup, or inspect", mode)
	}
}

func fpfQueryPublicationRequestFromArgs(
	args map[string]any,
) (fpf.QueryPublicationRequest, error) {
	view, _, err := strictPresentStringArg(args, "view")
	if err != nil {
		return fpf.QueryPublicationRequest{}, err
	}
	traceRef, _, err := strictPresentStringArg(args, "trace_ref")
	if err != nil {
		return fpf.QueryPublicationRequest{}, err
	}
	return fpf.NewQueryPublicationRequest(
		view,
		traceRef,
	)
}

func strictPresentStringArg(
	args map[string]any,
	key string,
) (string, bool, error) {
	raw, present := args[key]
	if !present {
		return "", false, nil
	}
	value, valid := raw.(string)
	if !valid {
		return "", true, fmt.Errorf("%s must be a string", key)
	}
	return strings.TrimSpace(value), true, nil
}

func fpfResponseBudgetFromArgs(args map[string]any) (fpf.ResponseBudget, error) {
	perRole, err := nonNegativeIntArg(args, "max_candidates_per_role")
	if err != nil {
		return fpf.ResponseBudget{}, err
	}
	total, err := nonNegativeIntArg(args, "max_total_candidates")
	if err != nil {
		return fpf.ResponseBudget{}, err
	}
	excerpt, err := nonNegativeIntArg(args, "max_excerpt_characters")
	if err != nil {
		return fpf.ResponseBudget{}, err
	}
	relations, err := nonNegativeIntArg(args, "max_relations_per_candidate")
	if err != nil {
		return fpf.ResponseBudget{}, err
	}
	return fpf.ResponseBudget{
		MaxCandidatesPerRole:     perRole,
		MaxTotalCandidates:       total,
		MaxExcerptCharacters:     excerpt,
		MaxRelationsPerCandidate: relations,
	}, nil
}

func nonNegativeIntArg(args map[string]any, key string) (int, error) {
	raw, ok := args[key]
	if !ok {
		return 0, nil
	}

	var value int64
	switch typed := raw.(type) {
	case int:
		value = int64(typed)
	case int64:
		value = typed
	case float64:
		value = int64(typed)
		if float64(value) != typed {
			return 0, fmt.Errorf("%s must be a non-negative integer", key)
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, fmt.Errorf("%s must be a non-negative integer", key)
		}
		value = parsed
	default:
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}

	converted := int(value)
	if value < 0 || int64(converted) != value {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return converted, nil
}

func decideInputRequestsExplicitBaselineAuthority(input artifact.DecideInput) bool {
	return len(input.BindingTargets) > 0 ||
		len(input.BindingHints) > 0 ||
		strings.TrimSpace(input.BindingScope) != "" ||
		strings.TrimSpace(input.BindingFallbackReason) != ""
}

func parseStrictRejectionReasonsFromArgs(args map[string]any, key string) ([]artifact.RejectionReason, error) {
	var values []artifact.RejectionReason

	present, err := decodeStrictArgFromArgs(args, key, &values)
	if err != nil {
		return nil, fmt.Errorf("%s must be an array of rejection reasons", key)
	}
	if !present {
		return nil, nil
	}

	return values, nil
}

func parseStrictRollbackSpecFromArgs(args map[string]any, key string) (*artifact.RollbackSpec, error) {
	var value artifact.RollbackSpec

	present, err := decodeStrictArgFromArgs(args, key, &value)
	if err != nil {
		return nil, fmt.Errorf("%s must be an object with rollback fields", key)
	}
	if !present {
		return nil, nil
	}

	return &value, nil
}

func parseStrictParityPlanFromArgs(args map[string]any, key string) (*artifact.ParityPlan, error) {
	var value artifact.ParityPlan

	present, err := decodeStrictArgFromArgs(args, key, &value)
	if err != nil {
		return nil, fmt.Errorf("%s must be an object with parity plan fields", key)
	}
	if !present {
		return nil, nil
	}

	return &value, nil
}

// evidenceCausalUseWarning surfaces the C.28 / CC-B3.9 advisory when the
// attached evidence content reads like a causal-use claim but the caller did
// not declare a CausalSupportBasis. Warning, not reject — legacy ingest
// continues unchanged.
func evidenceCausalUseWarning(input artifact.EvidenceInput, item *artifact.EvidenceItem) string {
	if item == nil || item.CausalSupportBasis != "" {
		return ""
	}
	lower := strings.ToLower(input.Content)
	triggers := []string{
		"causal",
		"caused",
		"intervention",
		"counterfactual",
		"uplift",
		"treatment effect",
		"causal effect",
		" effect of ",
	}
	for _, trigger := range triggers {
		if strings.Contains(lower, trigger) {
			return "Warning (C.28): evidence content suggests a causal-use claim but causal_support_basis is not declared. " +
				"Per CC-B3.9, undeclared basis cannot raise R for causal-ladder climbs. " +
				"Consider re-attaching with causal_support_basis in {observational|interventional|realized_counterfactual|identified_estimate|simulation_only}."
		}
	}
	return ""
}

func parsePredictionInputsFromArgs(args map[string]any, key string) ([]artifact.PredictionInput, error) {
	var predictions []artifact.PredictionInput

	present, err := decodeStrictArgFromArgs(args, key, &predictions)
	if err != nil {
		return nil, fmt.Errorf("%s must be an array of prediction objects", key)
	}
	if !present {
		return nil, nil
	}

	for index, value := range predictions {
		claim := strings.TrimSpace(value.Claim)
		observable := strings.TrimSpace(value.Observable)
		threshold := strings.TrimSpace(value.Threshold)

		if claim == "" && observable == "" && threshold == "" {
			return nil, fmt.Errorf("%s[%d] must declare at least one non-empty field", key, index)
		}
		if claim == "" || observable == "" || threshold == "" {
			return nil, fmt.Errorf("%s[%d] must include claim, observable, and threshold", key, index)
		}
	}

	return predictions, nil
}

func decodeStrictArgFromArgs(args map[string]any, key string, target any) (bool, error) {
	raw, ok := args[key]
	if !ok {
		return false, nil
	}

	data, err := strictArgBytes(raw)
	if err != nil {
		return true, err
	}

	if err := json.Unmarshal(data, target); err != nil {
		return true, err
	}

	return true, nil
}

func strictArgBytes(value any) ([]byte, error) {
	text, ok := value.(string)
	if ok {
		trimmed := strings.TrimSpace(text)
		if trimmed != "" && (trimmed[0] == '[' || trimmed[0] == '{') {
			return []byte(trimmed), nil
		}
	}

	return json.Marshal(value)
}

// parseDimensions handles MCP client serialization of comparison dimensions.
// Some MCP clients may send the array as:
//   - []any (parsed JSON array) — standard case
//   - string (JSON-encoded string) — when the client serializes nested arrays as strings
func parseDimensions(raw any) []artifact.ComparisonDimension {
	var items []map[string]any

	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				items = append(items, m)
			}
		}
	case string:
		// JSON string fallback: "[{...}, {...}]"
		if len(v) > 0 && v[0] == '[' {
			var parsed []map[string]any
			if err := json.Unmarshal([]byte(v), &parsed); err == nil {
				items = parsed
			}
		}
	case nil:
		return nil
	default:
		// Try JSON marshal/unmarshal roundtrip as last resort
		data, err := json.Marshal(raw)
		if err == nil {
			var parsed []map[string]any
			if json.Unmarshal(data, &parsed) == nil {
				items = parsed
			}
		}
	}

	var dims []artifact.ComparisonDimension
	for _, dm := range items {
		dim := artifact.ComparisonDimension{}
		if v, ok := dm["name"].(string); ok {
			dim.Name = v
		}
		if v, ok := dm["scale_type"].(string); ok {
			dim.ScaleType = v
		}
		if v, ok := dm["unit"].(string); ok {
			dim.Unit = v
		}
		if v, ok := dm["polarity"].(string); ok {
			dim.Polarity = v
		}
		if v, ok := dm["how_to_measure"].(string); ok {
			dim.HowToMeasure = v
		}
		if v, ok := dm["role"].(string); ok {
			dim.Role = v
		}
		if v, ok := dm["proxy_for"].(string); ok {
			dim.ProxyFor = v
		}
		if v, ok := dm["valid_until"].(string); ok {
			dim.ValidUntil = v
		}
		if dim.Name != "" {
			dims = append(dims, dim)
		}
	}
	return dims
}

const nestedStringMapShapeHint = `{"V1":{"latency":"10ms","cost":"$5"}} (variant_id -> dimension_name -> string score)`

func parseNestedStringMapArg(args map[string]any, key string) (map[string]map[string]string, bool, error) {
	value, present := args[key]
	if !present {
		return nil, false, nil
	}

	var raw map[string]any
	if m, ok := value.(map[string]any); ok {
		raw = m
	} else if s, ok := value.(string); ok && len(s) > 0 && s[0] == '{' {
		logger.Debug().Str("key", key).Str("raw_type", "string").Msg("parseNestedStringMapFromArgs: JSON string fallback")
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			return nil, true, fmt.Errorf("argument %q must match shape %s: %w", key, nestedStringMapShapeHint, err)
		}
	} else {
		return nil, true, fmt.Errorf("argument %q must match shape %s", key, nestedStringMapShapeHint)
	}
	if len(raw) == 0 {
		return nil, true, nil
	}

	result := make(map[string]map[string]string, len(raw))
	for outerKey, innerVal := range raw {
		inner, ok := innerVal.(map[string]any)
		if !ok {
			return nil, true, fmt.Errorf("argument %q entry %q must be an object; expected shape %s", key, outerKey, nestedStringMapShapeHint)
		}

		result[outerKey] = make(map[string]string, len(inner))
		for innerKey, innerValue := range inner {
			stringValue, ok := innerValue.(string)
			if !ok {
				return nil, true, fmt.Errorf("argument %q entry %q.%q must be a string score; expected shape %s", key, outerKey, innerKey, nestedStringMapShapeHint)
			}
			result[outerKey][innerKey] = stringValue
		}
	}

	return result, true, nil
}

// parseJSONArg decodes a JSON-shaped argument from the MCP args map into
// the typed target. Returns (present, error). Callers MUST propagate the
// error: an earlier version returned only `bool` and the call sites
// silently discarded parse failures, which produced empty payloads that
// downstream validators reported as "missing variant" coverage errors —
// the original symptom in github issue #71. Surfacing the parse error
// keeps the caller's intent visible.
func parseJSONArg(args map[string]any, key string, target any) (bool, error) {
	value, ok := args[key]
	if !ok {
		return false, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return true, fmt.Errorf("argument %q is not JSON-encodable: %w", key, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return true, fmt.Errorf("argument %q does not match the expected JSON shape: %w", key, err)
	}

	return true, nil
}

func compareToolResponse(a *artifact.Artifact, filePath string, navStrip string) string {
	response := present.SolutionResponse("compare", a, filePath, "")
	warnings := artifact.ExtractComparisonWarnings(a.Body)
	if len(warnings) == 0 {
		return response + navStrip
	}

	var builder strings.Builder
	builder.WriteString(response)
	builder.WriteString("Comparison warnings:\n")
	for _, warning := range warnings {
		builder.WriteString("- ")
		builder.WriteString(warning)
		builder.WriteString("\n")
	}
	builder.WriteString(navStrip)

	return builder.String()
}

// parseVariants handles MCP client serialization of the variants array.
// Accepts both parsed []any and raw JSON string formats.
func parseVariants(args map[string]any) []artifact.Variant {
	var raw []any

	if items, ok := args["variants"].([]any); ok {
		raw = items
	} else if s, ok := args["variants"].(string); ok && len(s) > 0 && s[0] == '[' {
		logger.Debug().Str("key", "variants").Str("raw_type", "string").Msg("parseVariants: JSON string fallback")
		// Try direct unmarshal into []Variant first
		var parsed []artifact.Variant
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			return parsed
		}
		// Fall back to generic unmarshal
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			logger.Warn().Str("key", "variants").Err(err).Msg("parseVariants: failed to parse JSON string")
			return nil
		}
	}

	if len(raw) == 0 {
		return nil
	}

	var variants []artifact.Variant
	for _, vRaw := range raw {
		vm, ok := vRaw.(map[string]any)
		if !ok {
			continue
		}
		v := artifact.Variant{}
		if s, ok := vm["title"].(string); ok {
			v.Title = s
		}
		if s, ok := vm["id"].(string); ok {
			v.ID = s
		}
		if s, ok := vm["description"].(string); ok {
			v.Description = s
		}
		if s, ok := vm["weakest_link"].(string); ok {
			v.WeakestLink = s
		}
		if s, ok := vm["rollback_notes"].(string); ok {
			v.RollbackNotes = s
		}
		if s, ok := vm["novelty_marker"].(string); ok {
			v.NoveltyMarker = s
		}
		if b, ok := vm["stepping_stone"].(bool); ok {
			v.SteppingStone = b
		}
		if s, ok := vm["stepping_stone_basis"].(string); ok {
			v.SteppingStoneBasis = s
		}
		if s, ok := vm["diversity_role"].(string); ok {
			v.DiversityRole = s
		}
		if s, ok := vm["assumption_notes"].(string); ok {
			v.AssumptionNotes = s
		}
		if items, ok := vm["strengths"].([]any); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					v.Strengths = append(v.Strengths, s)
				}
			}
		}
		if items, ok := vm["risks"].([]any); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					v.Risks = append(v.Risks, s)
				}
			}
		}
		if items, ok := vm["evidence_refs"].([]any); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					v.EvidenceRefs = append(v.EvidenceRefs, s)
				}
			}
		}
		if reference, ok := parseVariantProjectRecordRef(
			vm["project_record_ref"],
		); ok {
			v.ProjectRecordRef = &reference
		}
		variants = append(variants, v)
	}
	return variants
}

func parseVariantProjectRecordRef(
	raw any,
) (artifact.VariantProjectRecordRef, bool) {
	value, ok := raw.(map[string]any)
	if !ok {
		return artifact.VariantProjectRecordRef{}, false
	}
	refKindID, refKindPresent := value["ref_kind_id"].(string)
	referenceID, referencePresent := value["reference_id"].(string)
	if !refKindPresent || !referencePresent {
		return artifact.VariantProjectRecordRef{}, false
	}
	return artifact.VariantProjectRecordRef{
		RefKindID:   refKindID,
		ReferenceID: referenceID,
	}, true
}
