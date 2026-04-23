package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/m0n0x41d/haft/internal/artifact"
)

type harnessPlanFile struct {
	ID               string                `yaml:"id"`
	Revision         string                `yaml:"revision"`
	Title            string                `yaml:"title,omitempty"`
	RepoRef          string                `yaml:"repo_ref"`
	BaseSHA          string                `yaml:"base_sha"`
	TargetBranch     string                `yaml:"target_branch"`
	ProjectionPolicy string                `yaml:"projection_policy"`
	ValidFor         string                `yaml:"valid_for,omitempty"`
	ValidUntil       string                `yaml:"valid_until,omitempty"`
	Queue            string                `yaml:"queue,omitempty"`
	Defaults         harnessPlanDefaults   `yaml:"defaults"`
	Decisions        []harnessPlanDecision `yaml:"decisions"`
}

type harnessPlanDefaults struct {
	AllowedActions       []string `yaml:"allowed_actions"`
	EvidenceRequirements []string `yaml:"evidence_requirements,omitempty"`
}

type harnessPlanDecision struct {
	Ref       string   `yaml:"ref"`
	DependsOn []string `yaml:"depends_on,omitempty"`
}

type harnessRunOptions struct {
	PlanPath            string
	GeneratedPlanPath   string
	PrepareOnly         bool
	ForceCreate         bool
	Once                bool
	OnceTimeoutMS       int
	Mock                bool
	MockAgent           bool
	MockJudge           bool
	Concurrency         int
	MaxClaims           int
	PollIntervalMS      int
	StatusPath          string
	LogPath             string
	WorkspaceRoot       string
	RuntimePath         string
	RepoURL             string
	AgentMaxTurns       int
	AgentTurnTimeoutMS  int
	AgentWallClockS     int
	JudgeWallClockS     int
	ApprovalPolicy      string
	ThreadSandbox       string
	ProjectionMode      string
	ProjectionProfile   string
	ExternalApprover    string
	StatusHTTPEnabled   bool
	StatusHTTPPort      int
	CommissionRunnerID  string
	CommissionPlanRef   string
	CommissionQueueName string
}

var (
	harnessPlanOut          string
	harnessPlanID           string
	harnessPlanTitle        string
	harnessPlanRevision     string
	harnessPlanSequential   bool
	harnessPlanDependencies []string
	harnessPlanProblems     []string
	harnessPlanContext      string
	harnessPlanAllActive    bool

	harnessRunPlanPath          string
	harnessRunGeneratedPlanPath string
	harnessRunPrepareOnly       bool
	harnessRunForceCreate       bool
	harnessRunOnce              bool
	harnessRunOnceTimeoutMS     int
	harnessRunMock              bool
	harnessRunMockAgent         bool
	harnessRunMockJudge         bool
	harnessRunConcurrency       int
	harnessRunMaxClaims         int
	harnessRunPollIntervalMS    int
	harnessRunStatusPath        string
	harnessRunLogPath           string
	harnessRunWorkspaceRoot     string
	harnessRunRuntimePath       string
	harnessRunRepoURL           string
)

var harnessCmd = &cobra.Command{
	Use:   "harness",
	Short: "Run Open-Sleigh from Haft decisions",
	Long: `Run Open-Sleigh from Haft DecisionRecords.

This is the operator path: author ProblemCards and DecisionRecords in Haft,
then run the harness. By default Haft selects active DecisionRecords that do
not already have WorkCommissions, creates bounded WorkCommissions, and starts
Open-Sleigh. Open-Sleigh claims runnable work, and dependencies are enforced
by Haft.`,
}

var harnessPlanCmd = &cobra.Command{
	Use:   "plan [decision-id]...",
	Short: "Create an ImplementationPlan file from DecisionRecords",
	Args:  cobra.ArbitraryArgs,
	RunE:  runHarnessPlan,
}

var harnessRunCmd = &cobra.Command{
	Use:   "run [decision-id]...",
	Short: "Create commissions and start the Open-Sleigh harness",
	Long: `Create commissions and start the Open-Sleigh harness.

Run the normal active backlog path:
  haft harness run

Pass decision ids directly for an explicit override:
  haft harness run dec-a dec-b --sequential

Or select decisions from Haft:
  haft harness run --problem prob-...
  haft harness run --context mvp-harness
  haft harness run --all-active-decisions

Or pass an existing plan:
  haft harness run --plan .haft/plans/mvp.yaml`,
	Args: cobra.ArbitraryArgs,
	RunE: runHarnessRun,
}

func init() {
	registerCommissionFromDecisionFlags(harnessPlanCmd)
	registerHarnessPlanFlags(harnessPlanCmd)

	registerCommissionFromDecisionFlags(harnessRunCmd)
	registerHarnessPlanFlags(harnessRunCmd)
	registerHarnessRunFlags(harnessRunCmd)

	harnessCmd.AddCommand(harnessPlanCmd)
	harnessCmd.AddCommand(harnessRunCmd)
	rootCmd.AddCommand(harnessCmd)
}

func registerHarnessPlanFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessPlanOut, "out", "", "Plan file path (default: .haft/plans/<id>.yaml)")
	cmd.Flags().StringVar(&harnessPlanID, "id", "", "Plan id (default: generated from current UTC time)")
	cmd.Flags().StringVar(&harnessPlanTitle, "title", "", "Plan title")
	cmd.Flags().StringVar(&harnessPlanRevision, "revision", "p1", "Plan revision")
	cmd.Flags().BoolVar(&harnessPlanSequential, "sequential", false, "Make each decision depend on the previous command-line decision")
	cmd.Flags().StringArrayVar(&harnessPlanDependencies, "depend", nil, "Dependency edge target:source[,source]. Example: dec-b:dec-a")
	cmd.Flags().StringArrayVar(&harnessPlanProblems, "problem", nil, "Select active decisions linked to this ProblemCard; repeatable")
	cmd.Flags().StringVar(&harnessPlanContext, "context", "", "Select uncommissioned active decisions in this optional Haft context")
	cmd.Flags().BoolVar(&harnessPlanAllActive, "all-active-decisions", false, "Select all active DecisionRecords, including already commissioned decisions")
}

func registerHarnessRunFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessRunPlanPath, "plan", "", "Existing ImplementationPlan YAML/JSON file")
	cmd.Flags().StringVar(&harnessRunGeneratedPlanPath, "generated-plan", "", "Path for generated plan when decision ids are passed")
	cmd.Flags().BoolVar(&harnessRunPrepareOnly, "prepare-only", false, "Create/reuse plan and commissions, then stop before starting Open-Sleigh")
	cmd.Flags().BoolVar(&harnessRunForceCreate, "force-create-commissions", false, "Create another commission set even when this plan already has commissions")
	cmd.Flags().BoolVar(&harnessRunOnce, "once", false, "Run one Open-Sleigh polling pass")
	cmd.Flags().IntVar(&harnessRunOnceTimeoutMS, "once-timeout-ms", 8000, "Open-Sleigh --once timeout in milliseconds")
	cmd.Flags().BoolVar(&harnessRunMock, "mock", false, "Use mock agent and mock judge")
	cmd.Flags().BoolVar(&harnessRunMockAgent, "mock-agent", false, "Use mock agent")
	cmd.Flags().BoolVar(&harnessRunMockJudge, "mock-judge", false, "Use mock judge")
	cmd.Flags().IntVar(&harnessRunConcurrency, "concurrency", 2, "Open-Sleigh engine concurrency")
	cmd.Flags().IntVar(&harnessRunMaxClaims, "max-claims", 50, "Maximum commission claims per poll")
	cmd.Flags().IntVar(&harnessRunPollIntervalMS, "poll-interval-ms", 30000, "Open-Sleigh poll interval")
	cmd.Flags().StringVar(&harnessRunStatusPath, "status-path", "", "Status JSON path")
	cmd.Flags().StringVar(&harnessRunLogPath, "log-path", "", "Runtime JSONL log path")
	cmd.Flags().StringVar(&harnessRunWorkspaceRoot, "workspace-root", "", "Open-Sleigh workspace root")
	cmd.Flags().StringVar(&harnessRunRuntimePath, "runtime", "", "Open-Sleigh runtime directory (default: project open-sleigh or installed ~/.haft runtime)")
	cmd.Flags().StringVar(&harnessRunRepoURL, "repo-url", "", "Repository URL/path cloned into workspaces (default: project root)")
}

func runHarnessPlan(cmd *cobra.Command, decisionRefs []string) error {
	return withCommissionProject(func(ctx context.Context, store *artifact.Store, projectRoot string) error {
		selectedRefs, err := resolveHarnessDecisionRefs(ctx, store, decisionRefs)
		if err != nil {
			return err
		}

		plan, err := buildHarnessPlan(projectRoot, selectedRefs)
		if err != nil {
			return err
		}

		path, err := writeHarnessPlan(projectRoot, plan, harnessPlanOut)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Plan: %s\n", path)
		return err
	})
}

func runHarnessRun(cmd *cobra.Command, decisionRefs []string) error {
	if harnessRunPlanPath != "" && harnessDecisionSelectorsPresent(decisionRefs) {
		return fmt.Errorf("use either decision selectors or --plan, not both")
	}

	return withCommissionProject(func(ctx context.Context, store *artifact.Store, projectRoot string) error {
		selectedRefs, err := harnessRunDecisionRefs(ctx, store, decisionRefs)
		if err != nil {
			return err
		}

		planPath, plan, err := harnessPlanForRun(projectRoot, selectedRefs)
		if err != nil {
			return err
		}

		created, result, err := ensureHarnessCommissions(ctx, store, projectRoot, plan)
		if err != nil {
			return err
		}

		opts := defaultHarnessRunOptions(projectRoot, planPath, plan)
		configPath, err := writeHarnessRuntimeConfig(projectRoot, opts)
		if err != nil {
			return err
		}

		if err := printHarnessRunSummary(cmd, planPath, configPath, created, result, opts); err != nil {
			return err
		}
		if opts.PrepareOnly {
			return nil
		}

		return startOpenSleigh(cmd, opts, configPath)
	})
}

func harnessRunDecisionRefs(
	ctx context.Context,
	store *artifact.Store,
	decisionRefs []string,
) ([]string, error) {
	if harnessRunPlanPath != "" {
		return nil, nil
	}
	return resolveHarnessDecisionRefs(ctx, store, decisionRefs)
}

func harnessDecisionSelectorsPresent(decisionRefs []string) bool {
	return len(decisionRefs) > 0 ||
		len(harnessPlanProblems) > 0 ||
		strings.TrimSpace(harnessPlanContext) != "" ||
		harnessPlanAllActive
}

func harnessPlanForRun(
	projectRoot string,
	decisionRefs []string,
) (string, map[string]any, error) {
	if harnessRunPlanPath != "" {
		plan, err := readCommissionPlanPayload(bytes.NewReader(nil), harnessRunPlanPath)
		return harnessRunPlanPath, plan, err
	}

	plan, err := buildHarnessPlan(projectRoot, decisionRefs)
	if err != nil {
		return "", nil, err
	}

	path, err := writeHarnessPlan(projectRoot, plan, harnessRunGeneratedPlanPath)
	if err != nil {
		return "", nil, err
	}

	mapped, err := harnessPlanFileMap(plan)
	if err != nil {
		return "", nil, err
	}
	return path, mapped, nil
}

func resolveHarnessDecisionRefs(
	ctx context.Context,
	store *artifact.Store,
	explicitRefs []string,
) ([]string, error) {
	selected := cleanStringSlice(explicitRefs)
	if len(selected) == 0 && !harnessPlanSelectorFlagsPresent() {
		refs, err := uncommissionedActiveDecisionRefs(ctx, store)
		if err != nil {
			return nil, err
		}
		if len(refs) == 0 {
			return nil, fmt.Errorf("no active decisions without WorkCommissions")
		}
		return refs, nil
	}

	if harnessPlanAllActive {
		refs, err := activeDecisionRefs(ctx, store)
		if err != nil {
			return nil, err
		}
		selected = append(selected, refs...)
	}

	if strings.TrimSpace(harnessPlanContext) != "" {
		refs, err := decisionRefsForContext(ctx, store, harnessPlanContext)
		if err != nil {
			return nil, err
		}
		selected = append(selected, refs...)
	}

	for _, problemRef := range cleanStringSlice(harnessPlanProblems) {
		refs, err := decisionRefsForProblem(ctx, store, problemRef)
		if err != nil {
			return nil, err
		}
		selected = append(selected, refs...)
	}

	selected = uniqueStringsPreserveOrder(selected)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no active decisions matched the harness selectors")
	}

	if len(explicitRefs) > 0 || harnessPlanAllActive {
		return selected, nil
	}

	filtered, err := filterUncommissionedDecisionRefs(ctx, store, selected)
	if err != nil {
		return nil, err
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no selected active decisions without WorkCommissions")
	}
	return filtered, nil
}

func harnessPlanSelectorFlagsPresent() bool {
	return len(harnessPlanProblems) > 0 ||
		strings.TrimSpace(harnessPlanContext) != "" ||
		harnessPlanAllActive
}

func activeDecisionRefs(ctx context.Context, store *artifact.Store) ([]string, error) {
	decisions, err := store.ListActiveByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return nil, fmt.Errorf("list active decisions: %w", err)
	}

	return decisionRefsInCreationOrder(decisions), nil
}

func uncommissionedActiveDecisionRefs(
	ctx context.Context,
	store *artifact.Store,
) ([]string, error) {
	decisions, err := store.ListActiveByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return nil, fmt.Errorf("list active decisions: %w", err)
	}

	refs := decisionRefsInCreationOrder(decisions)
	return filterUncommissionedDecisionRefs(ctx, store, refs)
}

func filterUncommissionedDecisionRefs(
	ctx context.Context,
	store *artifact.Store,
	decisionRefs []string,
) ([]string, error) {
	commissioned, err := commissionedDecisionRefSet(ctx, store)
	if err != nil {
		return nil, err
	}

	filtered := make([]string, 0, len(decisionRefs))
	for _, ref := range decisionRefs {
		if _, exists := commissioned[ref]; exists {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered, nil
}

func commissionedDecisionRefSet(
	ctx context.Context,
	store *artifact.Store,
) (map[string]struct{}, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return nil, err
	}

	commissioned := make(map[string]struct{}, len(records))
	for _, commission := range records {
		ref := stringField(commission, "decision_ref")
		if ref == "" {
			continue
		}
		commissioned[ref] = struct{}{}
	}
	return commissioned, nil
}

func decisionRefsForContext(
	ctx context.Context,
	store *artifact.Store,
	contextName string,
) ([]string, error) {
	decisions, err := store.ListActiveByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return nil, fmt.Errorf("list decisions for context %s: %w", contextName, err)
	}

	matching := make([]*artifact.Artifact, 0, len(decisions))
	for _, decision := range decisions {
		if decision.Meta.Context == strings.TrimSpace(contextName) {
			matching = append(matching, decision)
		}
	}

	refs := decisionRefsInCreationOrder(matching)
	if len(refs) == 0 {
		return nil, fmt.Errorf("no active decisions in context %s", contextName)
	}
	return refs, nil
}

func decisionRefsForProblem(
	ctx context.Context,
	store *artifact.Store,
	problemRef string,
) ([]string, error) {
	decisions, err := store.ListActiveByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return nil, fmt.Errorf("list decisions for problem %s: %w", problemRef, err)
	}

	matching := make([]*artifact.Artifact, 0, len(decisions))
	for _, decision := range decisions {
		fullDecision, err := store.Get(ctx, decision.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf("load decision %s: %w", decision.Meta.ID, err)
		}
		if decisionReferencesProblem(ctx, store, fullDecision, problemRef) {
			matching = append(matching, fullDecision)
		}
	}

	refs := decisionRefsInCreationOrder(matching)
	if len(refs) == 0 {
		return nil, fmt.Errorf("no active decisions linked to problem %s", problemRef)
	}
	return refs, nil
}

func decisionReferencesProblem(
	ctx context.Context,
	store *artifact.Store,
	decision *artifact.Artifact,
	problemRef string,
) bool {
	fields := decision.UnmarshalDecisionFields()
	refs := decisionProblemRefs(ctx, store, decision, fields)
	return stringSliceContains(refs, problemRef)
}

func decisionRefsInCreationOrder(decisions []*artifact.Artifact) []string {
	sorted := append([]*artifact.Artifact(nil), decisions...)
	sortArtifactsByCreatedAt(sorted)

	refs := make([]string, 0, len(sorted))
	for _, decision := range sorted {
		refs = append(refs, decision.Meta.ID)
	}
	return refs
}

func sortArtifactsByCreatedAt(items []*artifact.Artifact) {
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].Meta.CreatedAt
		right := items[j].Meta.CreatedAt
		if left.Equal(right) {
			return items[i].Meta.ID < items[j].Meta.ID
		}
		return left.Before(right)
	})
}

func uniqueStringsPreserveOrder(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" {
			continue
		}
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func buildHarnessPlan(projectRoot string, decisionRefs []string) (harnessPlanFile, error) {
	cleanRefs := sortedUniqueDecisionRefs(decisionRefs)
	if len(cleanRefs) == 0 {
		return harnessPlanFile{}, fmt.Errorf("at least one decision id is required")
	}

	base, err := commissionPlanBaseParams(projectRoot, map[string]any{})
	if err != nil {
		return harnessPlanFile{}, err
	}

	planID := harnessPlanID
	if strings.TrimSpace(planID) == "" {
		planID = generatedHarnessPlanID(time.Now().UTC())
	}

	plan := harnessPlanFile{
		ID:               strings.TrimSpace(planID),
		Revision:         strings.TrimSpace(harnessPlanRevision),
		Title:            strings.TrimSpace(harnessPlanTitle),
		RepoRef:          stringField(base, "repo_ref"),
		BaseSHA:          stringField(base, "base_sha"),
		TargetBranch:     stringField(base, "target_branch"),
		ProjectionPolicy: stringField(base, "projection_policy"),
		ValidFor:         stringField(base, "valid_for"),
		ValidUntil:       stringField(base, "valid_until"),
		Defaults: harnessPlanDefaults{
			AllowedActions:       anyStrings(base["allowed_actions"]),
			EvidenceRequirements: anyStrings(base["evidence_requirements"]),
		},
		Decisions: harnessPlanDecisions(cleanRefs),
	}

	if plan.Title == "" {
		plan.Title = "Harness run " + plan.ID
	}

	return applyHarnessPlanDependencies(plan, harnessPlanDependencies, harnessPlanSequential)
}

func sortedUniqueDecisionRefs(decisionRefs []string) []string {
	refs := cleanStringSlice(decisionRefs)
	if harnessPlanSequential {
		return refs
	}
	return sortedUniqueStrings(refs)
}

func harnessPlanDecisions(decisionRefs []string) []harnessPlanDecision {
	decisions := make([]harnessPlanDecision, 0, len(decisionRefs))
	for _, ref := range decisionRefs {
		decisions = append(decisions, harnessPlanDecision{Ref: ref})
	}
	return decisions
}

func applyHarnessPlanDependencies(
	plan harnessPlanFile,
	edges []string,
	sequential bool,
) (harnessPlanFile, error) {
	indexByRef := make(map[string]int, len(plan.Decisions))
	for index, decision := range plan.Decisions {
		indexByRef[decision.Ref] = index
	}

	if sequential {
		for index := 1; index < len(plan.Decisions); index++ {
			previous := plan.Decisions[index-1].Ref
			plan.Decisions[index].DependsOn = append(plan.Decisions[index].DependsOn, previous)
		}
	}

	for _, edge := range edges {
		target, dependencies, err := parseHarnessDependency(edge)
		if err != nil {
			return harnessPlanFile{}, err
		}

		index, exists := indexByRef[target]
		if !exists {
			return harnessPlanFile{}, fmt.Errorf("dependency target %s is not in decisions", target)
		}
		for _, dependency := range dependencies {
			if _, exists := indexByRef[dependency]; !exists {
				return harnessPlanFile{}, fmt.Errorf("dependency source %s is not in decisions", dependency)
			}
		}

		plan.Decisions[index].DependsOn = append(plan.Decisions[index].DependsOn, dependencies...)
	}

	for index, decision := range plan.Decisions {
		plan.Decisions[index].DependsOn = sortedUniqueStrings(decision.DependsOn)
	}

	return plan, nil
}

func parseHarnessDependency(edge string) (string, []string, error) {
	parts := strings.SplitN(edge, ":", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("--depend must look like target:source[,source]")
	}

	target := strings.TrimSpace(parts[0])
	dependencies := strings.Split(parts[1], ",")
	dependencies = cleanStringSlice(dependencies)
	if target == "" || len(dependencies) == 0 {
		return "", nil, fmt.Errorf("--depend must include target and at least one source")
	}

	return target, dependencies, nil
}

func writeHarnessPlan(projectRoot string, plan harnessPlanFile, out string) (string, error) {
	path := strings.TrimSpace(out)
	if path == "" {
		path = filepath.Join(projectRoot, ".haft", "plans", plan.ID+".yaml")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}

	data, err := yaml.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("encode plan YAML: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create plan directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write plan: %w", err)
	}

	return path, nil
}

func harnessPlanFileMap(plan harnessPlanFile) (map[string]any, error) {
	encoded, err := yaml.Marshal(plan)
	if err != nil {
		return nil, err
	}

	decoded := map[string]any{}
	if err := yaml.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}

	normalized, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}

	result := map[string]any{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func ensureHarnessCommissions(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	plan map[string]any,
) (bool, string, error) {
	count, err := countExistingHarnessPlanCommissions(ctx, store, plan)
	if err != nil {
		return false, "", err
	}
	if count > 0 && !harnessRunForceCreate {
		return false, fmt.Sprintf("reused %d existing commission(s)", count), nil
	}

	args, err := commissionFromPlanCLIParams(projectRoot, plan)
	if err != nil {
		return false, "", err
	}

	result, err := handleHaftCommission(ctx, store, args)
	if err != nil {
		return false, "", err
	}
	return true, result, nil
}

func countExistingHarnessPlanCommissions(
	ctx context.Context,
	store *artifact.Store,
	plan map[string]any,
) (int, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return 0, err
	}

	count := 0
	planRef := stringField(plan, "id")
	planRevision := stringField(plan, "revision")
	for _, commission := range records {
		if stringField(commission, "implementation_plan_ref") != planRef {
			continue
		}
		if stringField(commission, "implementation_plan_revision") != planRevision {
			continue
		}
		count++
	}
	return count, nil
}

func defaultHarnessRunOptions(
	projectRoot string,
	planPath string,
	plan map[string]any,
) harnessRunOptions {
	statusPath := harnessRunStatusPath
	if strings.TrimSpace(statusPath) == "" {
		statusPath = filepath.Join(os.Getenv("HOME"), ".open-sleigh", "status.json")
	}

	logPath := harnessRunLogPath
	if strings.TrimSpace(logPath) == "" {
		logPath = filepath.Join(os.Getenv("HOME"), ".open-sleigh", "runtime.jsonl")
	}

	workspaceRoot := harnessRunWorkspaceRoot
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = filepath.Join(os.Getenv("HOME"), ".open-sleigh", "workspaces")
	}

	repoURL := harnessRunRepoURL
	if strings.TrimSpace(repoURL) == "" {
		repoURL = projectRoot
	}

	runtimePath := harnessRunRuntimePath
	if strings.TrimSpace(runtimePath) == "" {
		runtimePath = os.Getenv("HAFT_OPEN_SLEIGH_RUNTIME")
	}
	if strings.TrimSpace(runtimePath) == "" {
		runtimePath = os.Getenv("OPEN_SLEIGH_RUNTIME")
	}
	if strings.TrimSpace(runtimePath) == "" {
		runtimePath = defaultOpenSleighRuntimePath(projectRoot)
	}

	return harnessRunOptions{
		PlanPath:           planPath,
		PrepareOnly:        harnessRunPrepareOnly,
		ForceCreate:        harnessRunForceCreate,
		Once:               harnessRunOnce,
		OnceTimeoutMS:      harnessRunOnceTimeoutMS,
		Mock:               harnessRunMock,
		MockAgent:          harnessRunMockAgent || harnessRunMock,
		MockJudge:          harnessRunMockJudge || harnessRunMock,
		Concurrency:        positiveOrDefault(harnessRunConcurrency, 2),
		MaxClaims:          positiveOrDefault(harnessRunMaxClaims, 50),
		PollIntervalMS:     positiveOrDefault(harnessRunPollIntervalMS, 30000),
		StatusPath:         statusPath,
		LogPath:            logPath,
		WorkspaceRoot:      workspaceRoot,
		RuntimePath:        runtimePath,
		RepoURL:            repoURL,
		AgentMaxTurns:      20,
		AgentTurnTimeoutMS: 3600000,
		AgentWallClockS:    600,
		JudgeWallClockS:    120,
		ApprovalPolicy:     "never",
		ThreadSandbox:      "workspace-write",
		ProjectionMode:     "local_only",
		ProjectionProfile:  "manager_plain",
		ExternalApprover:   "ivan@weareocta.com",
		StatusHTTPPort:     4767,
		CommissionPlanRef:  stringField(plan, "id"),
	}
}

func writeHarnessRuntimeConfig(
	projectRoot string,
	opts harnessRunOptions,
) (string, error) {
	if err := validateOpenSleighRuntime(opts.RuntimePath); err != nil {
		return "", err
	}

	frontmatter, err := yaml.Marshal(harnessRuntimeConfig(projectRoot, opts))
	if err != nil {
		return "", fmt.Errorf("encode Open-Sleigh config: %w", err)
	}

	file, err := os.CreateTemp("", "sleigh.harness-*.md")
	if err != nil {
		return "", fmt.Errorf("create Open-Sleigh config: %w", err)
	}
	defer file.Close()

	content := "---\n" + string(frontmatter) + "---\n\n" + harnessPromptTemplates()
	if _, err := file.WriteString(content); err != nil {
		return "", fmt.Errorf("write Open-Sleigh config: %w", err)
	}

	return file.Name(), nil
}

func defaultOpenSleighRuntimePath(projectRoot string) string {
	projectRuntime := filepath.Join(projectRoot, "open-sleigh")
	if openSleighRuntimeExists(projectRuntime) {
		return projectRuntime
	}

	installedRuntime := installedOpenSleighRuntimePath()
	if openSleighRuntimeExists(installedRuntime) {
		return installedRuntime
	}

	return projectRuntime
}

func installedOpenSleighRuntimePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return ""
	}
	return filepath.Join(homeDir, ".haft", "runtimes", "open-sleigh", "current")
}

func openSleighRuntimeExists(runtimePath string) bool {
	_, err := openSleighRuntimeKind(runtimePath)
	return err == nil
}

func openSleighRuntimeKind(runtimePath string) (string, error) {
	if strings.TrimSpace(runtimePath) == "" {
		return "", fmt.Errorf("open-sleigh runtime path is empty")
	}

	info, err := os.Stat(runtimePath)
	if err != nil {
		return "", fmt.Errorf(
			"open-sleigh runtime not found at %s; run from the Haft monorepo, pass --runtime, or set HAFT_OPEN_SLEIGH_RUNTIME",
			runtimePath,
		)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("open-sleigh runtime path is not a directory: %s", runtimePath)
	}

	if _, err := os.Stat(openSleighReleaseExecutable(runtimePath)); err == nil {
		return "release", nil
	}
	if _, err := os.Stat(filepath.Join(runtimePath, "mix.exs")); err == nil {
		return "source", nil
	}

	return "", fmt.Errorf("open-sleigh runtime at %s is missing bin/open_sleigh or mix.exs", runtimePath)
}

func validateOpenSleighRuntime(runtimePath string) error {
	kind, err := openSleighRuntimeKind(runtimePath)
	if err != nil {
		return err
	}

	if kind == "source" {
		return validateOpenSleighSourceRuntime()
	}
	return nil
}

func validateOpenSleighSourceRuntime() error {
	if _, err := exec.LookPath("mix"); err != nil {
		return fmt.Errorf("mix is required to run Open-Sleigh; install Elixir 1.18+ or use --prepare-only")
	}
	return nil
}

func harnessRuntimeConfig(projectRoot string, opts harnessRunOptions) map[string]any {
	haftCommand := harnessHaftCommand(projectRoot)

	return map[string]any{
		"engine": map[string]any{
			"poll_interval_ms":   opts.PollIntervalMS,
			"status_path":        opts.StatusPath,
			"status_interval_ms": 5000,
			"log_path":           opts.LogPath,
			"concurrency":        opts.Concurrency,
			"status_http": map[string]any{
				"enabled": opts.StatusHTTPEnabled,
				"host":    "127.0.0.1",
				"port":    opts.StatusHTTPPort,
			},
		},
		"commission_source": map[string]any{
			"kind":            "haft",
			"selector":        "runnable",
			"max_claims":      opts.MaxClaims,
			"lease_timeout_s": 300,
			"plan_ref":        opts.CommissionPlanRef,
			"queue":           nil,
		},
		"projection": map[string]any{
			"mode":           opts.ProjectionMode,
			"targets":        []any{},
			"writer_profile": opts.ProjectionProfile,
		},
		"agent": map[string]any{
			"kind":                  "codex",
			"version_pin":           "local",
			"command":               "codex app-server",
			"max_turns":             opts.AgentMaxTurns,
			"max_tokens_per_turn":   80000,
			"wall_clock_timeout_s":  opts.AgentWallClockS,
			"max_retry_backoff_ms":  300000,
			"max_concurrent_agents": opts.Concurrency,
		},
		"codex": map[string]any{
			"approval_policy": opts.ApprovalPolicy,
			"thread_sandbox":  opts.ThreadSandbox,
			"turn_sandbox_policy": map[string]any{
				"type": "workspaceWrite",
			},
			"read_timeout_ms":  5000,
			"turn_timeout_ms":  opts.AgentTurnTimeoutMS,
			"stall_timeout_ms": 300000,
		},
		"judge": map[string]any{
			"kind":                 "codex",
			"adapter_version":      "mvp1-judge",
			"max_tokens_per_turn":  4000,
			"wall_clock_timeout_s": opts.JudgeWallClockS,
		},
		"workspace": map[string]any{
			"root":           opts.WorkspaceRoot,
			"cleanup_policy": "keep",
		},
		"hooks": map[string]any{
			"timeout_ms": 60000,
			"failure_policy": map[string]any{
				"after_create": "blocking",
				"before_run":   "blocking",
				"after_run":    "warning",
			},
			"after_create":  "git clone --depth 1 " + shellEscape(opts.RepoURL) + " .\nmix deps.get || true",
			"before_run":    nil,
			"after_run":     nil,
			"before_remove": nil,
		},
		"haft": map[string]any{
			"command": haftCommand,
			"version": "local",
		},
		"external_publication": map[string]any{
			"branch_regex":          "^(main|master|release/.*)$",
			"tracker_transition_to": []any{},
			"approvers":             []string{opts.ExternalApprover},
			"timeout_h":             24,
		},
		"phases": harnessPhaseConfig(),
	}
}

func harnessPhaseConfig() map[string]any {
	return map[string]any{
		"preflight": map[string]any{
			"agent_role": "preflight_checker",
			"tools":      []string{"haft_query", "read", "grep", "bash"},
			"gates": map[string]any{
				"structural": []string{"commission_runnable", "decision_fresh", "scope_snapshot_fresh"},
				"semantic":   []string{},
			},
		},
		"frame": map[string]any{
			"agent_role": "frame_verifier",
			"tools":      []string{"haft_query", "read", "grep"},
			"gates": map[string]any{
				"structural": []string{"problem_card_ref_present", "described_entity_field_present", "valid_until_field_present"},
				"semantic":   []string{"object_of_talk_is_specific"},
			},
		},
		"execute": map[string]any{
			"agent_role": "executor",
			"tools":      []string{"read", "write", "edit", "bash", "haft_note"},
			"gates": map[string]any{
				"structural": []string{"design_runtime_split_ok"},
				"semantic":   []string{"lade_quadrants_split_ok"},
			},
		},
		"measure": map[string]any{
			"agent_role": "measurer",
			"tools":      []string{"haft_decision", "haft_refresh"},
			"gates": map[string]any{
				"structural": []string{"evidence_ref_not_self", "valid_until_field_present"},
				"semantic":   []string{"no_self_evidence_semantic"},
			},
		},
	}
}

func harnessPromptTemplates() string {
	return strings.Join([]string{
		"# Prompt templates",
		"",
		"## Preflight",
		"You are the Commission Preflight checker. Given WorkCommission {{commission.id}}, read the linked ProblemCard, DecisionRecord, scope, base branch, and lockset. Report whether current context materially changed. You do not authorize execution; Haft decides after validating deterministic preflight facts.",
		"",
		"## Frame",
		"You are the Frame verifier. Given WorkCommission {{commission.id}} and linked ProblemCardRef {{commission.problem_card_ref}}, verify that the upstream ProblemCard is present, fresh, and sufficiently specific.",
		"",
		"## Execute",
		"You are the Executor. Given DecisionRecord {{decision.id}} and WorkCommission {{commission.id}}, implement only the bounded scope and produce external evidence.",
		"",
		"## Measure",
		"You are the Measurer. Given WorkCommission {{commission.id}}, assemble external evidence and decide whether the measured outcome passes.",
		"",
	}, "\n")
}

func printHarnessRunSummary(
	cmd *cobra.Command,
	planPath string,
	configPath string,
	created bool,
	commissionResult string,
	opts harnessRunOptions,
) error {
	mode := "long-running"
	if opts.Once {
		mode = "once"
	}
	if opts.MockAgent && opts.MockJudge {
		mode += " mock"
	}

	lines := []string{
		"Plan: " + planPath,
		"Open-Sleigh config: " + configPath,
		"Status: " + opts.StatusPath,
		"Runtime log: " + opts.LogPath,
		"Mode: " + mode,
	}

	if created {
		lines = append(lines, "Commissions: created")
	} else {
		lines = append(lines, "Commissions: "+commissionResult)
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}
	return nil
}

func startOpenSleigh(cmd *cobra.Command, opts harnessRunOptions, configPath string) error {
	args := openSleighStartArgs(opts, configPath)
	kind, err := openSleighRuntimeKind(opts.RuntimePath)
	if err != nil {
		return err
	}
	if kind == "release" {
		return startOpenSleighRelease(cmd, opts, args)
	}
	return startOpenSleighSource(cmd, opts, args)
}

func openSleighStartArgs(opts harnessRunOptions, configPath string) []string {
	args := []string{"--path", configPath}
	if opts.Once {
		args = append(args, "--once", "--once-timeout-ms="+strconv.Itoa(opts.OnceTimeoutMS))
	}
	if opts.MockAgent {
		args = append(args, "--mock-agent")
	}
	if opts.MockJudge {
		args = append(args, "--mock-judge")
	}
	return args
}

func startOpenSleighSource(cmd *cobra.Command, opts harnessRunOptions, args []string) error {
	mixArgs := append([]string{"open_sleigh.start"}, args...)
	process := exec.Command("mix", mixArgs...)
	return runOpenSleighProcess(cmd, opts, process)
}

func startOpenSleighRelease(cmd *cobra.Command, opts harnessRunOptions, args []string) error {
	expression := "Application.ensure_all_started(:mix); Mix.Tasks.OpenSleigh.Start.run(" +
		elixirStringListLiteral(args) +
		")"

	process := exec.Command(openSleighReleaseExecutable(opts.RuntimePath), "eval", expression)
	return runOpenSleighProcess(cmd, opts, process)
}

func openSleighReleaseExecutable(runtimePath string) string {
	return filepath.Join(runtimePath, "bin", "open_sleigh")
}

func runOpenSleighProcess(cmd *cobra.Command, opts harnessRunOptions, process *exec.Cmd) error {
	process.Dir = opts.RuntimePath
	process.Env = append(os.Environ(), "REPO_URL="+opts.RepoURL)
	process.Stdout = cmd.OutOrStdout()
	process.Stderr = cmd.ErrOrStderr()
	process.Stdin = cmd.InOrStdin()

	return process.Run()
}

func elixirStringListLiteral(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, elixirStringLiteral(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func elixirStringLiteral(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
	)
	return `"` + replacer.Replace(value) + `"`
}

func harnessHaftCommand(projectRoot string) string {
	executable, err := os.Executable()
	if err != nil || strings.TrimSpace(executable) == "" {
		executable = "haft"
	}

	return "HAFT_PROJECT_ROOT=" + shellEscape(projectRoot) + " " + shellEscape(executable) + " serve"
}

func generatedHarnessPlanID(now time.Time) string {
	return "plan-" + now.Format("20060102-150405")
}

func shellEscape(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func positiveOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func anyStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		return cleanStringSlice(typed)
	case []any:
		return anySliceToStrings(typed)
	default:
		return nil
	}
}
