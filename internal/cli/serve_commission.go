package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

const defaultCommissionValidFor = 168 * time.Hour

type commissionFromDecisionInput struct {
	DecisionRef          string
	RepoRef              string
	BaseSHA              string
	TargetBranch         string
	AllowedPaths         []string
	ForbiddenPaths       []string
	AllowedActions       []string
	AffectedFiles        []string
	AllowedModules       []string
	Lockset              []string
	EvidenceRequirements []any
	ProjectionPolicy     string
	State                string
	ValidUntil           string
}

type implementationPlanCommission struct {
	DecisionRef    string
	DependencyRefs []string
	Commission     map[string]any
}

func handleHaftCommission(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	action := stringArg(args, "action")

	switch action {
	case "create":
		return createWorkCommission(ctx, store, args)
	case "create_from_decision":
		return createWorkCommissionFromDecision(ctx, store, args)
	case "create_from_plan":
		return createWorkCommissionsFromPlan(ctx, store, args)
	case "create_batch_from_decisions":
		return createWorkCommissionBatchFromDecisions(ctx, store, args)
	case "list_runnable":
		return listRunnableWorkCommissions(ctx, store, args)
	case "claim_for_preflight":
		return claimWorkCommissionForPreflight(ctx, store, args)
	case "record_preflight", "start_after_preflight", "record_run_event", "complete_or_block":
		return appendWorkCommissionLifecycle(ctx, store, args)
	default:
		return "", fmt.Errorf("unknown haft_commission action: %s", action)
	}
}

func createWorkCommission(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	now := time.Now().UTC()

	commission, err := commissionPayload(args)
	if err != nil {
		return "", err
	}

	return persistWorkCommission(ctx, store, commission, now)
}

func createWorkCommissionFromDecision(
	ctx context.Context,
	store *artifact.Store,
	args map[string]any,
) (string, error) {
	now := time.Now().UTC()

	commission, err := buildWorkCommissionFromDecision(ctx, store, args, now)
	if err != nil {
		return "", err
	}

	return persistWorkCommission(ctx, store, commission, now)
}

func createWorkCommissionsFromPlan(
	ctx context.Context,
	store *artifact.Store,
	args map[string]any,
) (string, error) {
	now := time.Now().UTC()

	plan, err := implementationPlanPayload(args)
	if err != nil {
		return "", err
	}

	commissions, err := buildWorkCommissionsFromPlan(ctx, store, args, plan, now)
	if err != nil {
		return "", err
	}

	for _, commission := range commissions {
		if _, err := persistWorkCommission(ctx, store, commission, now); err != nil {
			return "", fmt.Errorf("persist commission for %s: %w", stringField(commission, "decision_ref"), err)
		}
	}

	return commissionResponseMap(map[string]any{
		"implementation_plan": implementationPlanSummary(plan),
		"commissions":         commissions,
	})
}

func createWorkCommissionBatchFromDecisions(
	ctx context.Context,
	store *artifact.Store,
	args map[string]any,
) (string, error) {
	now := time.Now().UTC()

	decisionRefs, err := parseStrictStringArrayFromArgs(args, "decision_refs")
	if err != nil {
		return "", err
	}
	if len(decisionRefs) == 0 {
		return "", fmt.Errorf("decision_refs is required")
	}

	commissions := make([]map[string]any, 0, len(decisionRefs))
	for _, decisionRef := range sortedUniqueStrings(decisionRefs) {
		commission, err := buildWorkCommissionFromDecision(
			ctx,
			store,
			commissionArgsForDecision(args, decisionRef),
			now,
		)
		if err != nil {
			return "", fmt.Errorf("build commission for %s: %w", decisionRef, err)
		}
		if err := normalizeNewWorkCommission(commission, now); err != nil {
			return "", fmt.Errorf("normalize commission for %s: %w", decisionRef, err)
		}
		commissions = append(commissions, commission)
	}

	for _, commission := range commissions {
		if _, err := persistWorkCommission(ctx, store, commission, now); err != nil {
			return "", fmt.Errorf("persist commission for %s: %w", stringField(commission, "decision_ref"), err)
		}
	}

	return commissionResponse("commissions", commissions)
}

func buildWorkCommissionsFromPlan(
	ctx context.Context,
	store *artifact.Store,
	args map[string]any,
	plan map[string]any,
	now time.Time,
) ([]map[string]any, error) {
	entries, err := implementationPlanDecisionEntries(plan)
	if err != nil {
		return nil, err
	}

	drafts := make([]implementationPlanCommission, 0, len(entries))
	for _, entry := range entries {
		decisionRef := implementationPlanDecisionRef(entry)
		commissionArgs := commissionArgsForPlanDecision(args, plan, entry, decisionRef)

		commission, err := buildWorkCommissionFromDecision(ctx, store, commissionArgs, now)
		if err != nil {
			return nil, fmt.Errorf("build commission for %s: %w", decisionRef, err)
		}

		putOptionalString(commission, "implementation_plan_ref", stringField(plan, "id"))
		putOptionalString(commission, "implementation_plan_revision", stringField(plan, "revision"))
		putOptionalAnySlice(commission, "plan_tags", planDecisionAnySlice(entry, "tags"))

		if err := normalizeNewWorkCommission(commission, now); err != nil {
			return nil, fmt.Errorf("normalize commission for %s: %w", decisionRef, err)
		}

		drafts = append(drafts, implementationPlanCommission{
			DecisionRef:    decisionRef,
			DependencyRefs: planDecisionStringSlice(entry, "depends_on"),
			Commission:     commission,
		})
	}

	if err := attachImplementationPlanDependencies(drafts); err != nil {
		return nil, err
	}

	return planDraftCommissions(drafts), nil
}

func persistWorkCommission(
	ctx context.Context,
	store *artifact.Store,
	commission map[string]any,
	now time.Time,
) (string, error) {
	if err := normalizeNewWorkCommission(commission, now); err != nil {
		return "", err
	}

	encoded, err := json.Marshal(commission)
	if err != nil {
		return "", fmt.Errorf("encode WorkCommission: %w", err)
	}

	item := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         stringField(commission, "id"),
			Kind:       artifact.KindWorkCommission,
			Status:     artifact.StatusActive,
			Title:      workCommissionTitle(commission),
			ValidUntil: stringField(commission, "valid_until"),
		},
		Body:           renderWorkCommissionBody(commission),
		StructuredData: string(encoded),
	}

	if err := store.Create(ctx, item); err != nil {
		return "", err
	}

	return commissionResponse("commission", commission)
}

func commissionArgsForDecision(args map[string]any, decisionRef string) map[string]any {
	next := copyStringAnyMap(args)
	next["decision_ref"] = decisionRef
	delete(next, "decision_refs")
	return next
}

func implementationPlanPayload(args map[string]any) (map[string]any, error) {
	plan, ok := mapArg(args, "plan")
	if !ok {
		return nil, fmt.Errorf("plan is required")
	}
	if stringField(plan, "id") == "" {
		return nil, fmt.Errorf("plan.id is required")
	}
	if stringField(plan, "revision") == "" {
		return nil, fmt.Errorf("plan.revision is required")
	}
	return plan, nil
}

func implementationPlanDecisionEntries(plan map[string]any) ([]map[string]any, error) {
	raw, ok := plan["decisions"].([]any)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("plan.decisions is required")
	}

	entries := make([]map[string]any, 0, len(raw))
	for index, value := range raw {
		entry, err := implementationPlanDecisionEntry(value, index)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err := validateImplementationPlanDecisionGraph(entries); err != nil {
		return nil, err
	}

	return entries, nil
}

func implementationPlanDecisionEntry(value any, index int) (map[string]any, error) {
	switch entry := value.(type) {
	case string:
		if strings.TrimSpace(entry) == "" {
			return nil, fmt.Errorf("plan.decisions[%d] is empty", index)
		}
		return map[string]any{"ref": entry}, nil
	case map[string]any:
		if implementationPlanDecisionRef(entry) == "" {
			return nil, fmt.Errorf("plan.decisions[%d].ref is required", index)
		}
		return entry, nil
	default:
		return nil, fmt.Errorf("plan.decisions[%d] must be a decision ref string or object", index)
	}
}

func implementationPlanDecisionRef(entry map[string]any) string {
	if ref := stringField(entry, "ref"); ref != "" {
		return ref
	}
	return stringField(entry, "decision_ref")
}

func commissionArgsForPlanDecision(
	args map[string]any,
	plan map[string]any,
	entry map[string]any,
	decisionRef string,
) map[string]any {
	defaults, _ := mapArg(plan, "defaults")

	next := copyStringAnyMap(args)
	next["decision_ref"] = decisionRef
	next["repo_ref"] = firstStringField("repo_ref", entry, defaults, plan, args)
	next["base_sha"] = firstStringField("base_sha", entry, defaults, plan, args)
	next["target_branch"] = firstStringField("target_branch", entry, defaults, plan, args)
	next["projection_policy"] = firstStringField("projection_policy", entry, defaults, plan, args)
	next["state"] = firstStringField("state", entry, defaults, plan, args)
	next["valid_for"] = firstStringField("valid_for", entry, defaults, plan, args)
	next["valid_until"] = firstStringField("valid_until", entry, defaults, plan, args)
	next["queue"] = firstStringField("queue", entry, defaults, plan, args)
	next["allowed_paths"] = stringSliceToAny(firstStringSliceField("allowed_paths", entry, defaults, plan, args))
	next["forbidden_paths"] = stringSliceToAny(firstStringSliceField("forbidden_paths", entry, defaults, plan, args))
	next["allowed_actions"] = stringSliceToAny(firstStringSliceField("allowed_actions", entry, defaults, plan, args))
	next["affected_files"] = stringSliceToAny(firstStringSliceField("affected_files", entry, defaults, plan, args))
	next["allowed_modules"] = stringSliceToAny(firstStringSliceField("allowed_modules", entry, defaults, plan, args))
	next["lockset"] = stringSliceToAny(firstStringSliceField("lockset", entry, defaults, plan, args))
	next["evidence_requirements"] = firstEvidenceRequirements(entry, defaults, plan, args)
	next["implementation_plan_ref"] = stringField(plan, "id")
	next["implementation_plan_revision"] = stringField(plan, "revision")

	delete(next, "plan")
	return next
}

func firstStringField(key string, maps ...map[string]any) string {
	for _, values := range maps {
		if values == nil {
			continue
		}
		if value := stringField(values, key); value != "" {
			return value
		}
	}
	return ""
}

func firstStringSliceField(key string, maps ...map[string]any) []string {
	for _, values := range maps {
		if values == nil {
			continue
		}
		if value := planDecisionStringSlice(values, key); len(value) > 0 {
			return value
		}
	}
	return nil
}

func firstEvidenceRequirements(maps ...map[string]any) []any {
	for _, values := range maps {
		if values == nil {
			continue
		}
		requirements, err := evidenceRequirementsFromArgs(values)
		if err != nil || len(requirements) == 0 {
			continue
		}
		return requirements
	}
	return nil
}

func validateImplementationPlanDecisionGraph(entries []map[string]any) error {
	dependenciesByRef := make(map[string][]string, len(entries))
	for _, entry := range entries {
		ref := implementationPlanDecisionRef(entry)
		if _, exists := dependenciesByRef[ref]; exists {
			return fmt.Errorf("plan decision %s is duplicated", ref)
		}
		dependenciesByRef[ref] = sortedUniqueStrings(planDecisionStringSlice(entry, "depends_on"))
	}

	for ref, dependencies := range dependenciesByRef {
		for _, dependency := range dependencies {
			if dependency == ref {
				return fmt.Errorf("plan decision %s depends on itself", ref)
			}
			if _, exists := dependenciesByRef[dependency]; !exists {
				return fmt.Errorf("plan decision %s depends on unknown decision %s", ref, dependency)
			}
		}
	}

	visiting := map[string]bool{}
	visited := map[string]bool{}
	for ref := range dependenciesByRef {
		if err := detectImplementationPlanCycle(ref, dependenciesByRef, visiting, visited); err != nil {
			return err
		}
	}

	return nil
}

func detectImplementationPlanCycle(
	ref string,
	dependenciesByRef map[string][]string,
	visiting map[string]bool,
	visited map[string]bool,
) error {
	if visited[ref] {
		return nil
	}
	if visiting[ref] {
		return fmt.Errorf("plan decision dependency cycle includes %s", ref)
	}

	visiting[ref] = true
	for _, dependency := range dependenciesByRef[ref] {
		if err := detectImplementationPlanCycle(dependency, dependenciesByRef, visiting, visited); err != nil {
			return err
		}
	}
	delete(visiting, ref)
	visited[ref] = true

	return nil
}

func attachImplementationPlanDependencies(drafts []implementationPlanCommission) error {
	commissionIDsByDecisionRef := make(map[string]string, len(drafts))
	for _, draft := range drafts {
		commissionIDsByDecisionRef[draft.DecisionRef] = stringField(draft.Commission, "id")
	}

	for _, draft := range drafts {
		decisionRefs := sortedUniqueStrings(draft.DependencyRefs)
		if len(decisionRefs) == 0 {
			continue
		}

		commissionIDs := make([]string, 0, len(decisionRefs))
		for _, decisionRef := range decisionRefs {
			commissionID := commissionIDsByDecisionRef[decisionRef]
			if commissionID == "" {
				return fmt.Errorf("plan decision %s depends on unknown decision %s", draft.DecisionRef, decisionRef)
			}
			commissionIDs = append(commissionIDs, commissionID)
		}

		draft.Commission["depends_on"] = stringSliceToAny(sortedUniqueStrings(commissionIDs))
		draft.Commission["depends_on_decisions"] = stringSliceToAny(decisionRefs)
	}

	return nil
}

func planDraftCommissions(drafts []implementationPlanCommission) []map[string]any {
	commissions := make([]map[string]any, 0, len(drafts))
	for _, draft := range drafts {
		commissions = append(commissions, draft.Commission)
	}
	return commissions
}

func planDecisionStringSlice(payload map[string]any, key string) []string {
	return stringSliceField(payload, key)
}

func planDecisionAnySlice(payload map[string]any, key string) []any {
	values, ok := payload[key].([]any)
	if !ok {
		return nil
	}
	return values
}

func implementationPlanSummary(plan map[string]any) map[string]any {
	summary := map[string]any{
		"id":       stringField(plan, "id"),
		"revision": stringField(plan, "revision"),
	}

	putOptionalString(summary, "title", stringField(plan, "title"))
	putOptionalString(summary, "failure_policy", stringField(plan, "failure_policy"))
	putOptionalString(summary, "projection_policy", stringField(plan, "projection_policy"))
	putOptionalString(summary, "queue", stringField(plan, "queue"))

	return summary
}

func buildWorkCommissionFromDecision(
	ctx context.Context,
	store *artifact.Store,
	args map[string]any,
	now time.Time,
) (map[string]any, error) {
	input, err := parseCommissionFromDecisionInput(args, now)
	if err != nil {
		return nil, err
	}

	decision, err := loadActiveDecisionRecord(ctx, store, input.DecisionRef)
	if err != nil {
		return nil, err
	}

	fields := decision.UnmarshalDecisionFields()

	problemRef, problemHash, err := primaryProblemRefAndHash(ctx, store, decision, fields)
	if err != nil {
		return nil, err
	}

	decisionHash, err := decisionRevisionHash(ctx, store, decision)
	if err != nil {
		return nil, err
	}

	scope, scopeHash, err := workCommissionScopeFromDecision(ctx, store, decision, input)
	if err != nil {
		return nil, err
	}

	evidence := input.EvidenceRequirements
	if len(evidence) == 0 {
		evidence = evidenceRequirementsFromStrings(fields.EvidenceRequirements)
	}

	commission := map[string]any{
		"decision_ref":           decision.Meta.ID,
		"decision_revision_hash": decisionHash,
		"problem_card_ref":       problemRef,
		"problem_revision_hash":  problemHash,
		"scope":                  scope,
		"scope_hash":             scopeHash,
		"base_sha":               input.BaseSHA,
		"lockset":                stringSliceToAny(scopeStringSlice(scope, "lockset")),
		"evidence_requirements":  evidence,
		"projection_policy":      input.ProjectionPolicy,
		"state":                  input.State,
		"valid_until":            input.ValidUntil,
		"fetched_at":             now.Format(time.RFC3339),
	}

	putOptionalString(commission, "implementation_plan_ref", stringArg(args, "implementation_plan_ref"))
	putOptionalString(commission, "implementation_plan_revision", stringArg(args, "implementation_plan_revision"))
	putOptionalString(commission, "autonomy_envelope_ref", stringArg(args, "autonomy_envelope_ref"))
	putOptionalString(commission, "autonomy_envelope_revision", stringArg(args, "autonomy_envelope_revision"))
	putOptionalString(commission, "queue", stringArg(args, "queue"))

	return commission, nil
}

func listRunnableWorkCommissions(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	commissions := make([]map[string]any, 0, len(records))
	for _, commission := range records {
		if workCommissionRunnableForRequest(commission, records, args, now) {
			commissions = append(commissions, commission)
		}
	}

	return commissionResponse("commissions", commissions)
}

func claimWorkCommissionForPreflight(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	runnerID := stringArg(args, "runner_id")
	if runnerID == "" {
		runnerID = "haft"
	}

	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin WorkCommission claim: %w", err)
	}
	defer tx.Rollback()

	commissions, err := loadWorkCommissionPayloadsForClaim(ctx, tx)
	if err != nil {
		return "", err
	}

	commission, err := selectWorkCommissionForClaim(commissions, args, time.Now().UTC())
	if err != nil {
		return "", err
	}
	if err := ensureWorkCommissionLocksetAvailable(commissions, commission); err != nil {
		return "", err
	}

	now := time.Now().UTC()
	commission["state"] = "preflighting"
	commission["lease"] = map[string]any{
		"runner_id":  runnerID,
		"state":      "claimed_for_preflight",
		"claimed_at": now.Format(time.RFC3339),
	}

	if err := updateWorkCommissionPayload(ctx, tx, commission); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit WorkCommission claim: %w", err)
	}

	return commissionResponse("commission", commission)
}

func appendWorkCommissionLifecycle(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	commissionID := stringArg(args, "commission_id")
	if commissionID == "" {
		return "", fmt.Errorf("commission_id is required")
	}

	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin WorkCommission lifecycle update: %w", err)
	}
	defer tx.Rollback()

	commission, err := loadWorkCommissionPayloadForUpdate(ctx, tx, commissionID)
	if err != nil {
		return "", err
	}

	commission = appendLifecycleEvent(commission, args)
	commission = applyLifecycleState(commission, args)

	if err := updateWorkCommissionPayload(ctx, tx, commission); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit WorkCommission lifecycle update: %w", err)
	}

	return commissionResponse("commission", commission)
}

func commissionPayload(args map[string]any) (map[string]any, error) {
	if payload, ok := mapArg(args, "commission"); ok {
		return copyStringAnyMap(payload), nil
	}

	payload := map[string]any{}
	for key, value := range args {
		if key == "action" || key == "config_hash" {
			continue
		}
		payload[key] = value
	}
	return payload, nil
}

func parseCommissionFromDecisionInput(
	args map[string]any,
	now time.Time,
) (commissionFromDecisionInput, error) {
	validUntil, err := commissionValidUntil(args, now)
	if err != nil {
		return commissionFromDecisionInput{}, err
	}

	evidence, err := evidenceRequirementsFromArgs(args)
	if err != nil {
		return commissionFromDecisionInput{}, err
	}

	input := commissionFromDecisionInput{
		DecisionRef:          stringArg(args, "decision_ref"),
		RepoRef:              stringArg(args, "repo_ref"),
		BaseSHA:              stringArg(args, "base_sha"),
		TargetBranch:         stringArg(args, "target_branch"),
		AllowedActions:       []string{"edit_files", "run_tests"},
		EvidenceRequirements: evidence,
		ProjectionPolicy:     stringArg(args, "projection_policy"),
		State:                stringArg(args, "state"),
		ValidUntil:           validUntil,
	}

	var parseErr error
	if input.AllowedPaths, parseErr = parseStrictStringArrayFromArgs(args, "allowed_paths"); parseErr != nil {
		return commissionFromDecisionInput{}, parseErr
	}
	if input.ForbiddenPaths, parseErr = parseStrictStringArrayFromArgs(args, "forbidden_paths"); parseErr != nil {
		return commissionFromDecisionInput{}, parseErr
	}
	if input.AffectedFiles, parseErr = parseStrictStringArrayFromArgs(args, "affected_files"); parseErr != nil {
		return commissionFromDecisionInput{}, parseErr
	}
	if input.AllowedModules, parseErr = parseStrictStringArrayFromArgs(args, "allowed_modules"); parseErr != nil {
		return commissionFromDecisionInput{}, parseErr
	}
	if input.Lockset, parseErr = parseStrictStringArrayFromArgs(args, "lockset"); parseErr != nil {
		return commissionFromDecisionInput{}, parseErr
	}
	if actions, parseErr := parseStrictStringArrayFromArgs(args, "allowed_actions"); parseErr != nil {
		return commissionFromDecisionInput{}, parseErr
	} else if len(actions) > 0 {
		input.AllowedActions = actions
	}

	return validateCommissionFromDecisionInput(input)
}

func validateCommissionFromDecisionInput(
	input commissionFromDecisionInput,
) (commissionFromDecisionInput, error) {
	input.DecisionRef = strings.TrimSpace(input.DecisionRef)
	input.RepoRef = strings.TrimSpace(input.RepoRef)
	input.BaseSHA = strings.TrimSpace(input.BaseSHA)
	input.TargetBranch = strings.TrimSpace(input.TargetBranch)
	input.ProjectionPolicy = strings.TrimSpace(input.ProjectionPolicy)
	input.State = strings.TrimSpace(input.State)
	if input.ProjectionPolicy == "" {
		input.ProjectionPolicy = "local_only"
	}
	if input.State == "" {
		input.State = "queued"
	}

	if input.DecisionRef == "" {
		return commissionFromDecisionInput{}, fmt.Errorf("decision_ref is required")
	}
	if input.RepoRef == "" {
		return commissionFromDecisionInput{}, fmt.Errorf("repo_ref is required")
	}
	if input.BaseSHA == "" {
		return commissionFromDecisionInput{}, fmt.Errorf("base_sha is required")
	}
	if input.TargetBranch == "" {
		return commissionFromDecisionInput{}, fmt.Errorf("target_branch is required")
	}
	if !validProjectionPolicy(input.ProjectionPolicy) {
		return commissionFromDecisionInput{}, fmt.Errorf("invalid projection_policy: %s", input.ProjectionPolicy)
	}
	if !validWorkCommissionState(input.State) {
		return commissionFromDecisionInput{}, fmt.Errorf("invalid WorkCommission state: %s", input.State)
	}

	return input, nil
}

func commissionValidUntil(args map[string]any, now time.Time) (string, error) {
	if value := stringArg(args, "valid_until"); value != "" {
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return "", fmt.Errorf("valid_until must be RFC3339: %w", err)
		}
		return value, nil
	}

	duration := defaultCommissionValidFor
	if value := stringArg(args, "valid_for"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return "", fmt.Errorf("valid_for must be a Go duration like 168h: %w", err)
		}
		duration = parsed
	}
	if duration <= 0 {
		return "", fmt.Errorf("valid_for must be positive")
	}

	return now.Add(duration).Format(time.RFC3339), nil
}

func loadActiveDecisionRecord(
	ctx context.Context,
	store *artifact.Store,
	decisionRef string,
) (*artifact.Artifact, error) {
	decision, err := store.Get(ctx, decisionRef)
	if err != nil {
		return nil, fmt.Errorf("load decision %s: %w", decisionRef, err)
	}
	if decision.Meta.Kind != artifact.KindDecisionRecord {
		return nil, fmt.Errorf("%s is %s, not DecisionRecord", decisionRef, decision.Meta.Kind)
	}
	if decision.Meta.Status != artifact.StatusActive {
		return nil, fmt.Errorf("decision_not_active: %s is %s", decisionRef, decision.Meta.Status)
	}

	return decision, nil
}

func primaryProblemRefAndHash(
	ctx context.Context,
	store *artifact.Store,
	decision *artifact.Artifact,
	fields artifact.DecisionFields,
) (string, string, error) {
	problemRefs := decisionProblemRefs(ctx, store, decision, fields)
	if len(problemRefs) == 0 {
		return "", "", fmt.Errorf("decision %s has no problem_card_ref", decision.Meta.ID)
	}

	problem, err := store.Get(ctx, problemRefs[0])
	if err != nil {
		return problemRefs[0], "", nil
	}

	hash, err := artifactRevisionHash(problem, nil)
	if err != nil {
		return "", "", err
	}
	return problemRefs[0], hash, nil
}

func decisionProblemRefs(
	ctx context.Context,
	store *artifact.Store,
	decision *artifact.Artifact,
	fields artifact.DecisionFields,
) []string {
	refs := appendStringSet(nil, fields.ProblemRefs...)

	for _, link := range decision.Meta.Links {
		if link.Type != "based_on" {
			continue
		}
		if strings.HasPrefix(link.Ref, artifact.KindProblemCard.IDPrefix()+"-") {
			refs = appendStringSet(refs, link.Ref)
			continue
		}

		portfolio, err := store.Get(ctx, link.Ref)
		if err != nil || portfolio.Meta.Kind != artifact.KindSolutionPortfolio {
			continue
		}

		refs = appendStringSet(refs, artifact.ResolvePortfolioProblemRefs(portfolio)...)
	}

	return refs
}

func decisionRevisionHash(
	ctx context.Context,
	store *artifact.Store,
	decision *artifact.Artifact,
) (string, error) {
	files, err := store.GetAffectedFiles(ctx, decision.Meta.ID)
	if err != nil {
		return "", fmt.Errorf("load decision affected files: %w", err)
	}

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}

	return artifactRevisionHash(decision, paths)
}

func artifactRevisionHash(item *artifact.Artifact, affectedFiles []string) (string, error) {
	links := append([]artifact.Link(nil), item.Meta.Links...)
	sort.Slice(links, func(i, j int) bool {
		left := links[i].Type + "\x00" + links[i].Ref
		right := links[j].Type + "\x00" + links[j].Ref
		return left < right
	})

	payload := struct {
		ID             string          `json:"id"`
		Kind           string          `json:"kind"`
		Version        int             `json:"version"`
		Status         string          `json:"status"`
		Title          string          `json:"title"`
		ValidUntil     string          `json:"valid_until,omitempty"`
		Body           string          `json:"body"`
		StructuredData string          `json:"structured_data"`
		Links          []artifact.Link `json:"links,omitempty"`
		AffectedFiles  []string        `json:"affected_files,omitempty"`
	}{
		ID:             item.Meta.ID,
		Kind:           string(item.Meta.Kind),
		Version:        item.Meta.Version,
		Status:         string(item.Meta.Status),
		Title:          item.Meta.Title,
		ValidUntil:     item.Meta.ValidUntil,
		Body:           item.Body,
		StructuredData: item.StructuredData,
		Links:          links,
		AffectedFiles:  sortedUniqueStrings(affectedFiles),
	}

	return canonicalJSONHash(payload)
}

func workCommissionScopeFromDecision(
	ctx context.Context,
	store *artifact.Store,
	decision *artifact.Artifact,
	input commissionFromDecisionInput,
) (map[string]any, string, error) {
	decisionAffectedFiles, err := decisionAffectedFilePaths(ctx, store, decision)
	if err != nil {
		return nil, "", err
	}

	allowedPaths := sortedUniqueStrings(input.AllowedPaths)
	if len(allowedPaths) == 0 {
		allowedPaths = decisionAffectedFiles
	}
	if len(allowedPaths) == 0 {
		return nil, "", fmt.Errorf("allowed_paths is required when decision has no affected_files")
	}

	affectedFiles := sortedUniqueStrings(input.AffectedFiles)
	if len(affectedFiles) == 0 {
		affectedFiles = allowedPaths
	}

	lockset := sortedUniqueStrings(input.Lockset)
	if len(lockset) == 0 {
		lockset = affectedFiles
	}

	scope := map[string]any{
		"repo_ref":        input.RepoRef,
		"base_sha":        input.BaseSHA,
		"target_branch":   input.TargetBranch,
		"allowed_paths":   stringSliceToAny(allowedPaths),
		"forbidden_paths": stringSliceToAny(sortedUniqueStrings(input.ForbiddenPaths)),
		"allowed_actions": stringSliceToAny(sortedUniqueStrings(input.AllowedActions)),
		"affected_files":  stringSliceToAny(affectedFiles),
		"allowed_modules": stringSliceToAny(sortedUniqueStrings(input.AllowedModules)),
		"lockset":         stringSliceToAny(lockset),
	}

	scopeHash, err := workCommissionScopeHash(scope)
	if err != nil {
		return nil, "", err
	}

	scope["hash"] = scopeHash
	return scope, scopeHash, nil
}

func decisionAffectedFilePaths(
	ctx context.Context,
	store *artifact.Store,
	decision *artifact.Artifact,
) ([]string, error) {
	files, err := store.GetAffectedFiles(ctx, decision.Meta.ID)
	if err != nil {
		return nil, fmt.Errorf("load decision affected files: %w", err)
	}

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}

	return sortedUniqueStrings(paths), nil
}

func evidenceRequirementsFromArgs(args map[string]any) ([]any, error) {
	raw, ok := args["evidence_requirements"]
	if !ok {
		return nil, nil
	}

	switch value := raw.(type) {
	case []string:
		return evidenceRequirementsFromStrings(value), nil
	case []any:
		return evidenceRequirementsFromAny(value), nil
	case string:
		return evidenceRequirementsFromString(value)
	default:
		return nil, fmt.Errorf("evidence_requirements must be an array of strings or objects")
	}
}

func evidenceRequirementsFromAny(values []any) []any {
	requirements := make([]any, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if ok {
			requirements = append(requirements, evidenceRequirementFromString(text))
			continue
		}

		entry, ok := value.(map[string]any)
		if ok {
			requirements = append(requirements, entry)
		}
	}
	return requirements
}

func evidenceRequirementsFromString(value string) ([]any, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		values := []any{}
		if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
			return nil, fmt.Errorf("parse evidence_requirements JSON: %w", err)
		}
		return evidenceRequirementsFromAny(values), nil
	}

	return evidenceRequirementsFromStrings([]string{trimmed}), nil
}

func evidenceRequirementsFromStrings(values []string) []any {
	requirements := make([]any, 0, len(values))
	for _, value := range values {
		requirement := evidenceRequirementFromString(value)
		if requirement != nil {
			requirements = append(requirements, requirement)
		}
	}
	return requirements
}

func evidenceRequirementFromString(value string) map[string]any {
	command := strings.TrimSpace(value)
	if command == "" {
		return nil
	}

	return map[string]any{
		"kind":    "command",
		"command": command,
	}
}

func normalizeNewWorkCommission(commission map[string]any, now time.Time) error {
	if stringField(commission, "id") == "" {
		commission["id"] = generateWorkCommissionID(now)
	}
	if stringField(commission, "state") == "" {
		commission["state"] = "queued"
	}
	if stringField(commission, "projection_policy") == "" {
		commission["projection_policy"] = "local_only"
	}
	if stringField(commission, "fetched_at") == "" {
		commission["fetched_at"] = now.Format(time.RFC3339)
	}
	if _, ok := commission["evidence_requirements"]; !ok {
		commission["evidence_requirements"] = []any{}
	}

	required := []string{
		"id",
		"decision_ref",
		"decision_revision_hash",
		"problem_card_ref",
		"projection_policy",
		"state",
		"valid_until",
		"fetched_at",
	}
	for _, key := range required {
		if stringField(commission, key) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if _, ok := mapArg(commission, "scope"); !ok {
		return fmt.Errorf("scope is required")
	}
	return nil
}

func loadWorkCommissionPayloads(ctx context.Context, store *artifact.Store) ([]map[string]any, error) {
	rows, err := store.DB().QueryContext(ctx, `
		SELECT id, COALESCE(structured_data, '')
		FROM artifacts
		WHERE kind = ?
		ORDER BY created_at ASC`, string(artifact.KindWorkCommission))
	if err != nil {
		return nil, fmt.Errorf("list WorkCommissions: %w", err)
	}
	defer rows.Close()

	commissions := []map[string]any{}
	for rows.Next() {
		var id string
		var structuredData string
		if err := rows.Scan(&id, &structuredData); err != nil {
			return nil, err
		}

		commission, err := decodeWorkCommissionPayload(id, structuredData)
		if err != nil {
			return nil, err
		}
		commissions = append(commissions, commission)
	}
	return commissions, rows.Err()
}

func loadWorkCommissionPayloadForUpdate(ctx context.Context, tx *sql.Tx, commissionID string) (map[string]any, error) {
	var structuredData string
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(structured_data, '')
		FROM artifacts
		WHERE id = ? AND kind = ?`, commissionID, string(artifact.KindWorkCommission)).
		Scan(&structuredData)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("commission_not_found")
	}
	if err != nil {
		return nil, fmt.Errorf("load WorkCommission %s: %w", commissionID, err)
	}

	return decodeWorkCommissionPayload(commissionID, structuredData)
}

func loadWorkCommissionPayloadsForClaim(ctx context.Context, tx *sql.Tx) ([]map[string]any, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, COALESCE(structured_data, '')
		FROM artifacts
		WHERE kind = ?
		ORDER BY created_at ASC`, string(artifact.KindWorkCommission))
	if err != nil {
		return nil, fmt.Errorf("list WorkCommissions for claim: %w", err)
	}
	defer rows.Close()

	commissions := []map[string]any{}
	for rows.Next() {
		var id string
		var structuredData string
		if err := rows.Scan(&id, &structuredData); err != nil {
			return nil, err
		}

		commission, err := decodeWorkCommissionPayload(id, structuredData)
		if err != nil {
			return nil, err
		}
		commissions = append(commissions, commission)
	}
	return commissions, rows.Err()
}

func updateWorkCommissionPayload(ctx context.Context, tx *sql.Tx, commission map[string]any) error {
	encoded, err := json.Marshal(commission)
	if err != nil {
		return fmt.Errorf("encode WorkCommission: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE artifacts
		SET version = version + 1,
		    title = ?,
		    content = ?,
		    valid_until = ?,
		    updated_at = ?,
		    structured_data = ?
		WHERE id = ? AND kind = ?`,
		workCommissionTitle(commission),
		renderWorkCommissionBody(commission),
		stringField(commission, "valid_until"),
		time.Now().UTC().Format(time.RFC3339),
		string(encoded),
		stringField(commission, "id"),
		string(artifact.KindWorkCommission),
	)
	if err != nil {
		return fmt.Errorf("update WorkCommission %s: %w", stringField(commission, "id"), err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("commission_not_found")
	}
	return nil
}

func decodeWorkCommissionPayload(id string, structuredData string) (map[string]any, error) {
	if strings.TrimSpace(structuredData) == "" {
		return nil, fmt.Errorf("WorkCommission %s has no structured_data", id)
	}

	payload := map[string]any{}
	if err := json.Unmarshal([]byte(structuredData), &payload); err != nil {
		return nil, fmt.Errorf("decode WorkCommission %s: %w", id, err)
	}
	if stringField(payload, "id") == "" {
		payload["id"] = id
	}
	return payload, nil
}

func workCommissionRunnable(commission map[string]any, now time.Time) bool {
	state := stringField(commission, "state")
	if state != "queued" && state != "ready" {
		return false
	}

	validUntil, err := time.Parse(time.RFC3339, stringField(commission, "valid_until"))
	if err != nil {
		return false
	}
	return validUntil.After(now)
}

func workCommissionRunnableForRequest(
	commission map[string]any,
	commissions []map[string]any,
	args map[string]any,
	now time.Time,
) bool {
	return workCommissionMatchesRequest(commission, args) &&
		workCommissionRunnable(commission, now) &&
		workCommissionDependenciesSatisfied(commission, commissions)
}

func workCommissionMatchesRequest(commission map[string]any, args map[string]any) bool {
	planRef := stringArg(args, "plan_ref")
	if planRef != "" && stringField(commission, "implementation_plan_ref") != planRef {
		return false
	}

	queue := stringArg(args, "queue")
	if queue != "" && stringField(commission, "queue") != queue {
		return false
	}

	return true
}

func workCommissionDependenciesSatisfied(
	commission map[string]any,
	commissions []map[string]any,
) bool {
	dependencyIDs := stringSliceField(commission, "depends_on")
	if len(dependencyIDs) == 0 {
		return true
	}

	commissionsByID := make(map[string]map[string]any, len(commissions))
	for _, candidate := range commissions {
		commissionsByID[stringField(candidate, "id")] = candidate
	}

	for _, dependencyID := range dependencyIDs {
		dependency := commissionsByID[dependencyID]
		if dependency == nil {
			return false
		}
		if !workCommissionDependencySatisfied(dependency) {
			return false
		}
	}

	return true
}

func workCommissionDependencySatisfied(commission map[string]any) bool {
	switch stringField(commission, "state") {
	case "completed", "completed_with_projection_debt":
		return true
	default:
		return false
	}
}

func selectWorkCommissionForClaim(
	commissions []map[string]any,
	args map[string]any,
	now time.Time,
) (map[string]any, error) {
	commissionID := stringArg(args, "commission_id")
	for _, commission := range commissions {
		if commissionID != "" && stringField(commission, "id") != commissionID {
			continue
		}
		if workCommissionRunnableForRequest(commission, commissions, args, now) {
			return commission, nil
		}
		if commissionID != "" {
			return nil, fmt.Errorf("commission_not_runnable")
		}
	}

	if commissionID != "" {
		return nil, fmt.Errorf("commission_not_found")
	}
	return nil, fmt.Errorf("commission_not_runnable")
}

func ensureWorkCommissionLocksetAvailable(
	commissions []map[string]any,
	target map[string]any,
) error {
	for _, active := range commissions {
		if workCommissionLocksetConflicts(target, active) {
			return fmt.Errorf("commission_lock_conflict")
		}
	}
	return nil
}

func workCommissionLocksetConflicts(target map[string]any, active map[string]any) bool {
	if stringField(target, "id") == stringField(active, "id") {
		return false
	}
	if !workCommissionActiveLeaseState(active) {
		return false
	}
	if commissionRepoRef(target) != commissionRepoRef(active) {
		return false
	}

	targetLockset := commissionLockset(target)
	activeLockset := commissionLockset(active)

	return locksetsOverlap(targetLockset, activeLockset)
}

func workCommissionActiveLeaseState(commission map[string]any) bool {
	state := stringField(commission, "state")
	return state == "preflighting" || state == "running"
}

func commissionRepoRef(commission map[string]any) string {
	scope, ok := mapArg(commission, "scope")
	if !ok {
		return ""
	}
	return stringField(scope, "repo_ref")
}

func commissionLockset(commission map[string]any) []string {
	topLevel := stringSliceField(commission, "lockset")
	if len(topLevel) > 0 {
		return topLevel
	}

	scope, ok := mapArg(commission, "scope")
	if !ok {
		return nil
	}
	return stringSliceField(scope, "lockset")
}

func locksetsOverlap(left []string, right []string) bool {
	for _, leftPath := range left {
		for _, rightPath := range right {
			if lockPathsOverlap(leftPath, rightPath) {
				return true
			}
		}
	}
	return false
}

func lockPathsOverlap(left string, right string) bool {
	left = normalizeLockPath(left)
	right = normalizeLockPath(right)

	switch {
	case left == "" || right == "":
		return false
	case left == "**/*" || right == "**/*":
		return true
	case left == "*" || right == "*":
		return true
	case left == right:
		return true
	case lockPrefixContains(left, right):
		return true
	case lockPrefixContains(right, left):
		return true
	}

	leftMatchesRight, _ := path.Match(left, right)
	rightMatchesLeft, _ := path.Match(right, left)
	return leftMatchesRight || rightMatchesLeft
}

func lockPrefixContains(pattern string, candidate string) bool {
	prefix, ok := lockPrefix(pattern)
	if !ok {
		return false
	}
	return candidate == prefix || strings.HasPrefix(candidate, prefix+"/")
}

func lockPrefix(pattern string) (string, bool) {
	switch {
	case strings.HasSuffix(pattern, "/**/*"):
		return strings.TrimSuffix(pattern, "/**/*"), true
	case strings.HasSuffix(pattern, "/**"):
		return strings.TrimSuffix(pattern, "/**"), true
	default:
		return "", false
	}
}

func normalizeLockPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "./")
	if value == "" {
		return ""
	}
	return path.Clean(value)
}

func workCommissionScopeHash(scope map[string]any) (string, error) {
	fields, err := canonicalScopeFields(scope)
	if err != nil {
		return "", err
	}

	return canonicalJSONHash(fields)
}

func canonicalScopeFields(scope map[string]any) (map[string]any, error) {
	fields := map[string]any{
		"affected_files":  sortedUniqueStrings(scopeStringSlice(scope, "affected_files")),
		"allowed_actions": sortedUniqueStrings(scopeStringSlice(scope, "allowed_actions")),
		"allowed_modules": sortedUniqueStrings(scopeStringSlice(scope, "allowed_modules")),
		"allowed_paths":   sortedUniqueStrings(scopeStringSlice(scope, "allowed_paths")),
		"base_sha":        stringField(scope, "base_sha"),
		"forbidden_paths": sortedUniqueStrings(scopeStringSlice(scope, "forbidden_paths")),
		"lockset":         sortedUniqueStrings(scopeStringSlice(scope, "lockset")),
		"repo_ref":        stringField(scope, "repo_ref"),
		"target_branch":   stringField(scope, "target_branch"),
	}

	if err := validateCanonicalScopeFields(fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func validateCanonicalScopeFields(fields map[string]any) error {
	requiredStrings := []string{"base_sha", "repo_ref", "target_branch"}
	for _, key := range requiredStrings {
		if stringField(fields, key) == "" {
			return fmt.Errorf("scope.%s is required", key)
		}
	}

	requiredLists := []string{"affected_files", "allowed_actions", "allowed_paths", "lockset"}
	for _, key := range requiredLists {
		if len(scopeStringSlice(fields, key)) == 0 {
			return fmt.Errorf("scope.%s is required", key)
		}
	}

	return nil
}

func canonicalJSONHash(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validProjectionPolicy(value string) bool {
	switch value {
	case "local_only", "external_optional", "external_required":
		return true
	default:
		return false
	}
}

func validWorkCommissionState(value string) bool {
	switch value {
	case "draft", "queued", "ready", "preflighting", "running", "blocked_stale",
		"blocked_policy", "blocked_conflict", "needs_human_review", "completed",
		"completed_with_projection_debt", "failed", "cancelled", "expired":
		return true
	default:
		return false
	}
}

func appendLifecycleEvent(commission map[string]any, args map[string]any) map[string]any {
	event := map[string]any{
		"action":      stringArg(args, "action"),
		"runner_id":   stringArg(args, "runner_id"),
		"event":       stringArg(args, "event"),
		"verdict":     stringArg(args, "verdict"),
		"reason":      stringArg(args, "reason"),
		"recorded_at": time.Now().UTC().Format(time.RFC3339),
	}
	if payload, ok := mapArg(args, "payload"); ok {
		event["payload"] = payload
	}

	events, _ := commission["events"].([]any)
	commission["events"] = append(events, event)
	return commission
}

func applyLifecycleState(commission map[string]any, args map[string]any) map[string]any {
	action := stringArg(args, "action")
	verdict := stringArg(args, "verdict")

	switch {
	case action == "start_after_preflight":
		commission["state"] = "running"
	case action == "record_preflight" && verdict == "blocked":
		commission["state"] = "blocked_stale"
	case action == "complete_or_block" && (verdict == "pass" || verdict == "completed"):
		commission["state"] = "completed"
	case action == "complete_or_block" && verdict == "failed":
		commission["state"] = "failed"
	case action == "complete_or_block" && verdict == "blocked":
		commission["state"] = "blocked_policy"
	}

	return commission
}

func commissionResponse(key string, value any) (string, error) {
	encoded, err := json.Marshal(map[string]any{key: value})
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func commissionResponseMap(value map[string]any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func workCommissionTitle(commission map[string]any) string {
	id := stringField(commission, "id")
	decisionRef := stringField(commission, "decision_ref")
	if decisionRef == "" {
		return "WorkCommission " + id
	}
	return "WorkCommission " + id + " for " + decisionRef
}

func renderWorkCommissionBody(commission map[string]any) string {
	lines := []string{
		"# WorkCommission " + stringField(commission, "id"),
		"",
		"- State: " + stringField(commission, "state"),
		"- Decision: " + stringField(commission, "decision_ref"),
		"- ProblemCard: " + stringField(commission, "problem_card_ref"),
		"- Projection policy: " + stringField(commission, "projection_policy"),
		"- Valid until: " + stringField(commission, "valid_until"),
	}
	return strings.Join(lines, "\n")
}

func generateWorkCommissionID(now time.Time) string {
	var bytes [4]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("wc-%s-%09d", now.Format("20060102"), now.UnixNano()%1_000_000_000)
	}
	return "wc-" + now.Format("20060102") + "-" + hex.EncodeToString(bytes[:])
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func putOptionalString(payload map[string]any, key string, value string) {
	cleaned := strings.TrimSpace(value)
	if cleaned != "" {
		payload[key] = cleaned
	}
}

func putOptionalAnySlice(payload map[string]any, key string, value []any) {
	if len(value) > 0 {
		payload[key] = value
	}
}

func stringSliceField(payload map[string]any, key string) []string {
	switch value := payload[key].(type) {
	case []string:
		return cleanStringSlice(value)
	case []any:
		return anySliceToStrings(value)
	default:
		return nil
	}
}

func scopeStringSlice(payload map[string]any, key string) []string {
	return stringSliceField(payload, key)
}

func stringSliceToAny(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func sortedUniqueStrings(values []string) []string {
	result := cleanStringSlice(values)
	sort.Strings(result)

	if len(result) < 2 {
		return result
	}

	unique := result[:1]
	for _, value := range result[1:] {
		if value != unique[len(unique)-1] {
			unique = append(unique, value)
		}
	}
	return unique
}

func appendStringSet(target []string, values ...string) []string {
	result := append([]string(nil), target...)
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" {
			continue
		}
		if stringSliceContains(result, cleaned) {
			continue
		}
		result = append(result, cleaned)
	}
	return result
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cleanStringSlice(values []string) []string {
	result := []string{}
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

func anySliceToStrings(values []any) []string {
	result := []string{}
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}

		cleaned := strings.TrimSpace(text)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

func mapArg(args map[string]any, key string) (map[string]any, bool) {
	value, ok := args[key]
	if !ok {
		return nil, false
	}
	payload, ok := value.(map[string]any)
	return payload, ok
}

func copyStringAnyMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
