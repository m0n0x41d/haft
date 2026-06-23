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
	"github.com/m0n0x41d/haft/internal/recall"
	"github.com/m0n0x41d/haft/internal/ui"
	"github.com/m0n0x41d/haft/logger"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server for AI tool integration.

The server communicates via stdio and provides Haft tools to embedded host
agents. v7 product support targets Claude Code and Codex; other MCP clients
may remain protocol-compatible experimental integrations.

The project root is determined by:
  1. HAFT_PROJECT_ROOT environment variable (if set)
  2. QUINT_PROJECT_ROOT legacy environment variable (if set)
  3. Current working directory (default)`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// Ensure global ~/.haft/ exists (migrates from ~/.quint-code/ if needed)
	_ = project.EnsureDir()

	rootInput, err := projectRootInputFromEnv()
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

	server := fpf.NewServer()
	binding, bindingErr := resolveProjectBindingFromInput(rootInput, strings.TrimSpace(os.Getenv(envExpectedProjectID)))
	if bindingErr != nil {
		server.SetInstructions(composeServerInstructions(nil))
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
	// Always emit server instructions: the workflow policy (when present) plus
	// the always-on code-graph doctrine, so every session's system prompt tells
	// the agent the fused code graph exists and to consult it before editing.
	server.SetInstructions(composeServerInstructions(workflow))

	if _, err := os.Stat(binding.DBPath); err != nil {
		server.SetV5Handler(func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
			return "", fmt.Errorf("haft project database is missing; run `haft init` in %s to create %s; %s", binding.ProjectRoot, binding.DBPath, formatProjectBindingDiagnostic(binding))
		})
		server.Start()
		return nil
	}

	database, err := db.NewStore(binding.DBPath)
	if err != nil {
		server.SetV5Handler(func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
			return "", fmt.Errorf("failed to open haft project database: %w; %s", err, formatProjectBindingDiagnostic(binding))
		})
		server.Start()
		return nil
	}

	artStore := artifact.NewStore(database.GetRawDB())

	indexStore, indexErr := project.OpenIndex()
	if indexErr != nil {
		logger.Warn().Err(indexErr).Msg("failed to open cross-project index")
	}

	_ = project.PopulateContextFacts(context.Background(), database.GetRawDB(), binding.ProjectName)

	go func() {
		if built, err := codeintel.NewService(artStore).EnsureIndex(context.Background(), binding.ProjectRoot); err != nil {
			logger.Warn().Err(err).Msg("code-graph startup refresh failed")
		} else if built {
			logger.Info().Msg("code-graph index rebuilt on startup (source changed)")
		}
	}()

	searcher := buildHybridSearcher(artStore, database.GetRawDB())
	crossHybrid := buildCrossProjectHybrid(indexStore)
	projCfg := &project.Config{
		ID:   binding.ProjectID,
		Name: binding.ProjectName,
	}

	server.SetV5Handler(makeV5Handler(artStore, searcher, crossHybrid, binding.HaftDir, projCfg, indexStore))
	server.Start()
	return nil
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

func makeV5Handler(store *artifact.Store, searcher recall.Searcher, crossHybrid *project.CrossHybrid, haftDir string, projCfg *project.Config, indexStore *project.IndexStore) fpf.V5ToolHandler {
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
		result, createdRef, toolErr := dispatchTool(ctx, store, searcher, haftDir, params.Name, params.Arguments)

		// Post-dispatch hooks
		logger.ToolResult(params.Name, action, time.Since(start).Milliseconds(), toolErr)

		if toolErr == nil {
			result = applyCrossProjectRecall(ctx, result, params.Name, action, params.Arguments, store, projCfg, indexStore, crossHybrid)
			result = applyGraphSeededRecall(ctx, result, params.Name, action, params.Arguments, store, haftDir)
			applyCrossProjectIndex(ctx, params.Name, action, params.Arguments, createdRef, store, projCfg, indexStore, crossHybrid)
			invalidateRecall(searcher, createdRef)
		}

		logAudit(ctx, store.DB(), params.Name, action, params.Arguments, toolErr)

		if toolErr == nil {
			result = applyRefreshReminder(ctx, result, params.Name, store)
			result = applyReadinessReminder(result, params.Name, haftDir)
		}

		return result, toolErr
	}
}

// dispatchTool routes a tool call to its handler. Pure dispatch, no hooks.
// Returns (result, createdArtifactID, err). createdArtifactID is the canonical
// ID of the artifact created by this call (e.g. "dec-20260418-a3f7c1d2"); empty
// string when the action does not create a primary artifact (e.g. read-only
// queries, mutations of existing artifacts).
func dispatchTool(ctx context.Context, store *artifact.Store, searcher recall.Searcher, haftDir string, name string, args map[string]any) (string, string, error) {
	if err := rejectMCPBindingAction(name, args); err != nil {
		return "", "", err
	}

	switch name {
	case "haft_note":
		return handleQuintNote(ctx, store, haftDir, args)
	case "haft_problem":
		result, err := handleQuintProblem(ctx, store, haftDir, args)
		return result, "", err
	case "haft_solution":
		result, err := handleQuintSolution(ctx, store, haftDir, args)
		return result, "", err
	case "haft_decision":
		return handleQuintDecision(ctx, store, haftDir, args)
	case "haft_refresh":
		result, err := handleQuintRefresh(ctx, store, haftDir, args)
		return result, "", err
	case "haft_query":
		result, err := handleQuintQuery(ctx, store, searcher, haftDir, args)
		return result, "", err
	case "haft_commission":
		args = commissionArgsWithProjectRoot(args, filepath.Dir(haftDir))
		result, err := handleHaftCommission(ctx, store, args)
		return result, "", err
	case "haft_method":
		return handleHaftMethod(ctx, store, haftDir, args)
	case "haft_spec_section":
		result, err := handleHaftSpecSection(ctx, store, haftDir, args)
		return result, "", err
	default:
		return "", "", fmt.Errorf("unknown tool: %s", name)
	}
}

func commissionArgsWithProjectRoot(args map[string]any, projectRoot string) map[string]any {
	if stringArg(args, "project_root") != "" {
		return args
	}
	if strings.TrimSpace(projectRoot) == "" {
		return args
	}

	next := copyStringAnyMap(args)
	next["project_root"] = projectRoot
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
func applyGraphSeededRecall(ctx context.Context, result, name, action string, args map[string]any, store *artifact.Store, haftDir string) string {
	if name != "haft_problem" || action != "frame" {
		return result
	}
	seedFileRaw, _ := args["seed_file"].(string)
	seedFile := strings.TrimSpace(seedFileRaw)
	if seedFile == "" {
		return result
	}
	projectRoot := filepath.Dir(haftDir)
	ranked, err := codeintel.NewService(store).RelatedToFile(ctx, projectRoot, seedFile, 6)
	if err != nil || len(ranked) == 0 {
		return result
	}
	lines := make([]string, 0, len(ranked))
	for _, r := range ranked {
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
	return "Authority boundary: evidence/WLNK display is not approval, not gate passage, and not global truth; use EvidencePath for attempted-use reliance.\n"
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
	ref = strings.TrimSpace(ref)
	dash := strings.IndexByte(ref, '-')
	if dash <= 0 || dash+1 >= len(ref) {
		return false
	}
	for i := 0; i < dash; i++ {
		if c := ref[i]; c < 'a' || c > 'z' {
			return false
		}
	}
	return ref[dash+1] >= '0' && ref[dash+1] <= '9'
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

func handleQuintProblem(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
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

		a, filePath, err := artifact.FrameProblem(ctx, store, haftDir, input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		resp := present.ProblemResponse("frame", a, filePath, navStrip) + present.FPFPhaseHint("frame")
		if warn := artifact.UmbrellaWarning(input.Title, input.Signal, input.Acceptance); warn != "" {
			resp += "\n" + warn
		}
		return resp, nil

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
			return "", err
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
					present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)), nil
			}
			input.ProblemRef = prob.Meta.ID
		}

		a, filePath, err := artifact.CharacterizeProblem(ctx, store, haftDir, input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		resp := present.ProblemResponse("characterize", a, filePath, navStrip) + present.FPFPhaseHint("characterize")
		if warn := artifact.ValueBeforeProxyWarning(input.Dimensions); warn != "" {
			resp += "\n" + warn + "\n"
		}
		return resp, nil

	case "select":
		problems, err := artifact.SelectProblems(ctx, store, contextName, 20)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		items := artifact.EnrichProblemsForList(ctx, store, problems)
		return present.ProblemsListResponse(items, navStrip), nil

	case "close":
		problemRef, _ := args["problem_ref"].(string)
		if problemRef == "" {
			return "", fmt.Errorf("problem_ref is required for close action")
		}
		a, err := store.Get(ctx, problemRef)
		if err != nil {
			return "", fmt.Errorf("problem %s not found: %w", problemRef, err)
		}
		if a.Meta.Kind != artifact.KindProblemCard {
			return "", fmt.Errorf("%s is %s, not a ProblemCard", problemRef, a.Meta.Kind)
		}
		a.Meta.Status = artifact.StatusAddressed
		if err := store.Update(ctx, a); err != nil {
			return "", fmt.Errorf("update problem status: %w", err)
		}
		if _, err := artifact.WriteFile(haftDir, a); err != nil {
			logger.Warn().Err(err).Str("problem_ref", problemRef).Msg("problem.close.file_write_failed")
		}
		return fmt.Sprintf("Problem %s marked as addressed.\n", problemRef), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'frame', 'characterize', 'select', or 'close'", action)
	}
}

func handleQuintSolution(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
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

		a, filePath, err := artifact.ExploreSolutions(ctx, store, haftDir, input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.SolutionResponse("explore", a, filePath, navStrip) + present.FPFPhaseHint("explore"), nil

	case "compare":
		input := artifact.CompareInput{}
		if v, ok := args["portfolio_ref"].(string); ok {
			input.PortfolioRef = v
		}
		input.Results.Dimensions = parseStringArrayFromArgs(args, "dimensions")
		scores, _, err := parseNestedStringMapArg(args, "scores")
		if err != nil {
			return "", err
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
			return "", err
		}
		if _, err := parseJSONArg(args, "pareto_tradeoffs", &input.Results.ParetoTradeoffs); err != nil {
			return "", err
		}
		if _, err := parseJSONArg(args, "incomparable", &input.Results.Incomparable); err != nil {
			return "", err
		}
		parityPlan, err := parseStrictParityPlanFromArgs(args, "parity_plan")
		if err != nil {
			return "", err
		}
		input.Results.ParityPlan = parityPlan
		if input.PortfolioRef == "" {
			p, _ := artifact.FindActivePortfolio(ctx, store, contextName)
			if p != nil {
				input.PortfolioRef = p.Meta.ID
			} else {
				return "No active solution portfolio found.\nUse /h-explore to create variants first.\n" +
					present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)), nil
			}
		}

		a, filePath, err := artifact.CompareSolutions(ctx, store, haftDir, input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return compareToolResponse(a, filePath, navStrip), nil

	case "similar":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("query required for similar search")
		}
		results, err := artifact.FetchSearchResults(ctx, store, query, 10)
		if err != nil {
			return "", err
		}
		var matches []string
		for _, r := range results {
			if r.Meta.Kind == artifact.KindSolutionPortfolio {
				matches = append(matches, fmt.Sprintf("- [%s] %s (problem: %s)",
					r.Meta.ID, r.Meta.Title, r.Meta.Context))
			}
		}
		if len(matches) == 0 {
			return "No similar past solutions found. This is a novel problem.", nil
		}
		return fmt.Sprintf("Past solution portfolios matching \"%s\":\n%s\n\nUse haft_query(search) for details on any portfolio.",
			query, strings.Join(matches, "\n")), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'explore', 'compare', or 'similar'", action)
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
		if input.ProblemRef == "" {
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

		a, filePath, err := artifact.Decide(ctx, store, haftDir, input)
		if err != nil {
			return "", "", err
		}

		// Auto-baseline when affected_files are present
		var baselineNote string
		if len(input.AffectedFiles) > 0 {
			projectRoot := filepath.Dir(haftDir)
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

		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		resp := present.DecisionResponse("decide", a, filePath, "", navStrip) + baselineNote + present.FPFPhaseHint("decide")
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
		return present.DecisionResponse("measure", a, "", extra, navStrip) + present.FPFPhaseHint("verify"), "", nil

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

func handleQuintRefresh(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
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

		// Rescan modules before drift detection — keeps dependency graph fresh
		scanner := codebase.NewScanner(store.DB())
		if _, err := scanner.ScanModules(ctx, projectRoot); err != nil {
			logger.Warn().Err(err).Msg("refresh: module rescan failed (non-fatal)")
		}
		if _, err := scanner.ScanDependencies(ctx, projectRoot); err != nil {
			_ = err // non-fatal
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
		return present.MaintenancePlanResponse(plan, navStrip), nil

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
		dryRun, _ := args["dry_run"].(bool)
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

func codeContextSymbolsForFile(ctx context.Context, store *artifact.Store, projectRoot string, file string) ([]present.CodeContextSymbolItem, bool, error) {
	symbolStore := codebase.NewSymbolStore(store.DB())
	if err := symbolStore.EnsureSchema(ctx); err != nil {
		return nil, false, err
	}

	stale, err := symbolStore.FileSymbolsStale(ctx, projectRoot, file)
	if err != nil {
		return nil, false, err
	}

	refreshed := false
	if stale {
		if err := symbolStore.IndexFileSymbols(ctx, projectRoot, file); err != nil {
			return nil, false, err
		}
		refreshed = true
	}

	symbols, err := symbolStore.GetByFile(ctx, file)
	if err != nil {
		return nil, false, err
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
	return items, refreshed, nil
}

func statusDataStaleNavItems(items []artifact.StaleItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID+": "+item.Title+" ("+item.Reason+")")
	}
	return out
}

func navStripWithStaleSnapshot(ctx context.Context, store *artifact.Store, contextName string, staleItems []artifact.StaleItem) string {
	nav := artifact.ComputeNavState(ctx, store, contextName)
	nav.StaleCount = len(staleItems)
	nav.StaleItems = statusDataStaleNavItems(staleItems)
	return present.NavStrip(nav)
}

func navStripForStatusStaleLane(ctx context.Context, store *artifact.Store, contextName string) string {
	data, err := artifact.FetchStatusData(ctx, store, contextName, "")
	if err != nil {
		return present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
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
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)
	navStrip := navStripForStatusStaleLane(ctx, store, contextName)

	switch action {
	case "search":
		query, _ := args["query"].(string)
		limit := 20
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		results, err := searchArtifacts(ctx, store, searcher, query, limit)
		if err != nil {
			return "", err
		}
		return present.SearchResponse(results, query) + navStrip, nil

	case "status":
		// H1 (dec-20260526-9fdd33ed): pass projectRoot so /h-status
		// surfaces drift via FetchStatusData → CheckDrift → StatusData.Drift.
		projectRoot := filepath.Dir(haftDir)
		if view, _ := args["view"].(string); view == "governor" {
			// Prompt-budgeted projection for host-side prompt governors;
			// deliberately skips coverage and the navigation strip.
			return governorStatusResponse(ctx, store, contextName, projectRoot)
		}
		full, _ := args["full"].(bool)
		data, err := artifact.FetchStatusData(ctx, store, contextName, projectRoot)
		if err != nil {
			return "", err
		}
		statusBody := present.CockpitStatusResponse(data)
		if full {
			statusBody = present.StatusResponse(data)
		}
		result := overseerStatusPrefix(projectRoot) + statusBody
		// Append module coverage if modules are scanned
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
		return result + navStripWithStaleSnapshot(ctx, store, contextName, data.StaleItems), nil

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
		artifactRef := firstNonEmptyQueryArg(args, "artifact_id", "ref")
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
			svc := codeintel.NewService(store)
			projectRoot := filepath.Dir(haftDir)
			// Phase-2 graph-proximity recall (dec-20260604-3aaad199): FTS5-seeded
			// PPR over the fused graph, additive to the exact affected-file list.
			// Best-effort — a failure never breaks the related response.
			if ranked, perr := svc.RelatedToFile(ctx, projectRoot, file, 12); perr == nil && len(ranked) > 0 {
				// Dedup: drop anything already shown in the exact affected-file
				// section, so a decision is not listed twice.
				shown := make(map[string]bool, len(results))
				for _, a := range results {
					shown[a.Meta.ID] = true
				}
				items := make([]present.RelatedProximityItem, 0, len(ranked))
				for _, r := range ranked {
					if shown[r.ID] {
						continue
					}
					label := "reasoning"
					if r.Kind == codeintel.RelatedSymbol {
						label = "symbol"
					}
					items = append(items, present.RelatedProximityItem{Title: r.Title, Label: label, Ref: r.ID})
				}
				resp += present.RelatedProximityResponse(items)
			}
			// Structural test-coverage lane (dec-20260604-ef966a11): which tests
			// exercise this file's symbols — 'exercised by', never 'verified'.
			if cov, cerr := svc.TestedBy(ctx, projectRoot, file); cerr == nil && len(cov) > 0 {
				items := make([]present.TestedByItem, 0, len(cov))
				for _, c := range cov {
					items = append(items, present.TestedByItem{Symbol: c.Symbol, Exported: c.Exported, TestedBy: c.TestedBy})
				}
				resp += present.TestedByResponse(items)
			}
		}
		return resp + navStrip, nil

	case "code_context":
		file, _ := args["file"].(string)
		if file == "" {
			return "", fmt.Errorf("file is required for code_context action")
		}
		symbol, _ := args["symbol"].(string)
		line := 0
		if l, ok := args["line"].(float64); ok {
			line = int(l)
		}
		target := contextgraph.Target{File: file, Symbol: symbol, Line: line}
		lane := present.CodeContextLaneIndex
		if rawLane, ok := args["lane"].(string); ok && strings.TrimSpace(rawLane) != "" {
			parsed, valid := present.ParseCodeContextLane(rawLane)
			if !valid {
				return "", fmt.Errorf("unknown code_context lane %q — valid lanes: %s", rawLane, strings.Join(present.ValidCodeContextLaneNames(), ", "))
			}
			lane = parsed
		}
		limit, hasLimit := codeContextLaneLimit(args)
		projectRoot := filepath.Dir(haftDir)
		full, _ := args["full"].(bool)
		if !full && lane == present.CodeContextLaneSymbols {
			symbols, refreshed, err := codeContextSymbolsForFile(ctx, store, projectRoot, file)
			if err != nil {
				return present.CodeContextSymbolsUnavailableResponse(target, err) + navStrip, nil
			}
			return present.CodeContextSymbolsResponse(target, symbols, limit, refreshed) + navStrip, nil
		}
		cc, err := contextgraph.FetchCodeContext(ctx, store, graph.NewStore(store.DB()), target)
		if err != nil {
			return "", err
		}
		if full {
			return present.CodeContextResponseFull(cc) + navStrip, nil
		}
		options := present.CodeContextRenderOptions{
			Lane:          lane,
			ArtifactLimit: limit,
		}
		if hasLimit {
			options.InvariantLimit = limit
			options.ContextInvariantLimit = limit
		}
		if lane == present.CodeContextLaneIndex {
			symbols, _, err := codeContextSymbolsForFile(ctx, store, projectRoot, file)
			options.SymbolCountKnown = err == nil
			options.SymbolCount = len(symbols)
			if err != nil {
				options.SymbolUnavailable = err.Error()
			}
		}
		return present.CodeContextResponseWithOptions(cc, options) + navStrip, nil

	case "callees", "callers", "impact":
		name := firstNonEmptyQueryArg(args, "symbol", "name")
		if name == "" {
			return "", fmt.Errorf("symbol is required for %s action", action)
		}
		file, _ := args["file"].(string)
		line := 0
		if l, ok := args["line"].(float64); ok {
			line = int(l)
		}
		depth := 0
		if d, ok := args["depth"].(float64); ok {
			depth = int(d)
		}
		dir := codeintel.Callees
		if action != "callees" {
			dir = codeintel.Callers // callers + impact both walk inbound edges
		}
		projectRoot := filepath.Dir(haftDir)
		res, err := codeintel.NewService(store).Flow(ctx, projectRoot, name, file, line, depth, dir)
		if err != nil {
			return "", err
		}
		return present.FlowResponse(res, action, name) + navStrip, nil

	case "node":
		name := firstNonEmptyQueryArg(args, "symbol", "name")
		if name == "" {
			return "", fmt.Errorf("symbol is required for node action")
		}
		file, _ := args["file"].(string)
		line := 0
		if l, ok := args["line"].(float64); ok {
			line = int(l)
		}
		projectRoot := filepath.Dir(haftDir)
		view, err := codeintel.NewService(store).Node(ctx, projectRoot, name, file, line)
		if err != nil {
			return "", err
		}
		return present.NodeResponse(view, nodeLang(file, view)) + navStrip, nil

	case "explore":
		name := firstNonEmptyQueryArg(args, "symbol", "name")
		if name == "" {
			return "", fmt.Errorf("symbol is required for explore action")
		}
		file, _ := args["file"].(string)
		line := 0
		if l, ok := args["line"].(float64); ok {
			line = int(l)
		}
		projectRoot := filepath.Dir(haftDir)
		svc := codeintel.NewService(store)
		// A bag of >=2 names (space/comma-separated) → multi-seed: connect them.
		// A single name → the single-seed flow. Same arg, no new tool.
		if seeds := splitSeedBag(name); len(seeds) >= 2 {
			res, err := svc.ExploreBag(ctx, projectRoot, seeds)
			if err != nil {
				return "", err
			}
			return present.ExploreBagResponse(res) + navStrip, nil
		}
		res, err := svc.Explore(ctx, projectRoot, name, file, line)
		if err != nil {
			return "", err
		}
		return present.ExploreResponse(res, name, exploreLang(file, res)) + navStrip, nil

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
		scanner := codebase.NewScanner(store.DB())

		// Always rescan — module detection is fast (<100ms)
		if _, err := scanner.ScanModules(ctx, projectRoot); err != nil {
			return "", fmt.Errorf("module scan: %w", err)
		}
		if _, err := scanner.ScanDependencies(ctx, projectRoot); err != nil {
			_ = err // non-fatal
		}

		report, err := codebase.ComputeCoverage(ctx, store.DB())
		if err != nil {
			return "", fmt.Errorf("compute coverage: %w", err)
		}
		return codebase.FormatCoverageResponse(report) + navStrip, nil

	case "fpf":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("query is required for fpf search")
		}
		limit := fpf.DefaultSpecSearchLimit
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		full, _ := args["full"].(bool)
		explain, _ := args["explain"].(bool)
		mode, _ := args["mode"].(string)
		retrieval, err := retrieveEmbeddedFPF(fpf.SpecRetrievalRequest{
			Query: query,
			Limit: limit,
			Full:  full,
			Mode:  mode,
		})
		if err != nil {
			return "", fmt.Errorf("fpf search: %w", err)
		}
		if len(retrieval.Results) == 0 {
			return formatMCPFPFSearchWithExplain(nil, explain) + navStrip, nil
		}

		return formatMCPFPFSearchWithExplain(presentFPFRetrieval(retrieval.Results), explain) + navStrip, nil

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
			projectRoot,
			stringArg(args, "section_id"),
			stringArg(args, "use_context"),
			stringArg(args, "policy"),
			stringArg(args, "waiver_expires_at"),
			gatePtr,
			time.Now().UTC(),
		)
		if err != nil {
			return "", fmt.Errorf("build spec use record: %w", err)
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return "", fmt.Errorf("marshal spec use record: %w", err)
		}
		return string(payload), nil

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
		record := artifact.BuildDriftEventReport(reports)
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
		return "", fmt.Errorf("unknown action %q — use 'search', 'status', 'related', 'code_context', 'callees', 'callers', 'impact', 'node', 'explore', 'ceremony', 'projection', 'list', 'coverage', 'fpf', 'check', 'carrier_manifest', 'carrier_check', 'contract_audit', 'contract_generation', 'spec_review', 'spec_use', 'change_case', 'correspondence_graph', 'drift_route', 'drift_events', 'decision_reconcile', 'governing_set', 'blocked_use', 'value_space', 'evidence_path', or 'resolve_term'", action)
	}
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

// exploreLang derives the code-fence language for an explore view from the file
// argument, falling back to the seed's file extension.
func exploreLang(file string, res codeintel.ExploreResult) string {
	ext := filepath.Ext(file)
	if ext == "" {
		ext = filepath.Ext(res.Seed.FilePath)
	}
	return strings.TrimPrefix(ext, ".")
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

// parseNestedStringMapFromArgs handles MCP client serialization of map[string]map[string]string.
// Some clients send JSON objects as parsed map[string]any, others as raw JSON strings.
func parseNestedStringMapFromArgs(args map[string]any, key string) map[string]map[string]string {
	result, _, err := parseNestedStringMapArg(args, key)
	if err != nil {
		return nil
	}
	return result
}

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
		return response + navStrip + present.FPFPhaseHint("compare")
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
	builder.WriteString(present.FPFPhaseHint("compare"))

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
		variants = append(variants, v)
	}
	return variants
}
