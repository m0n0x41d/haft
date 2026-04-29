package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/autonomyenvelope"
	"github.com/m0n0x41d/haft/internal/implementationplan"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/workcommission"
)

const defaultCommissionValidFor = 168 * time.Hour

const defaultDeliveryPolicy = "workspace_patch_manual"

const defaultCommissionOpenAttentionAfter = 24 * time.Hour

const defaultCommissionLeaseAttentionAfter = 2 * time.Hour

const defaultCommissionLeaseAgeCap = 24 * time.Hour

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
	DeliveryPolicy       string
	State                string
	ValidUntil           string
	// SliceDescription names the specific slice of the parent DecisionRecord
	// that THIS commission implements. Required when the same decision_ref
	// already has at least one non-cancelled commission, to prevent the
	// scope-leak anti-pattern documented in
	// `.context/multi-commission-anti-pattern-retrospective.md` (each codex
	// inheriting full decision text and independently implementing the union
	// of slices that intersects its writeable surface).
	SliceDescription string
}

type implementationPlanCommission struct {
	DecisionRef    string
	DependencyRefs []string
	Commission     map[string]any
}

type workCommissionTransition struct {
	ErrorCode        string
	TargetState      string
	AllowedStates    []string
	RequiresReason   bool
	RejectsExpired   bool
	RefreshFetchedAt bool
}

type commissionFreshnessIssue struct {
	Code     string
	Field    string
	Ref      string
	Expected string
	Actual   string
}

type commissionFreshnessGap struct {
	Code   string
	Field  string
	Ref    string
	Reason string
}

type commissionFreshnessReport struct {
	Issues []commissionFreshnessIssue
	Gaps   []commissionFreshnessGap
}

type commissionOperatorAttention struct {
	Code   string
	Reason string
}

var requeueWorkCommissionTransition = workCommissionTransition{
	ErrorCode:        "commission_not_requeueable",
	TargetState:      "queued",
	AllowedStates:    workcommission.RecoverableStateValues(),
	RequiresReason:   true,
	RejectsExpired:   true,
	RefreshFetchedAt: true,
}

var cancelWorkCommissionTransition = workCommissionTransition{
	ErrorCode:      "commission_not_cancellable",
	TargetState:    "cancelled",
	AllowedStates:  workcommission.CancellableStateValues(),
	RequiresReason: true,
}

var workCommissionLifecycleAllowedStatesByAction = map[string][]string{
	"record_preflight":      {"preflighting"},
	"start_after_preflight": {"preflighting"},
	"record_run_event":      {"preflighting", "running"},
	"complete_or_block":     {"running"},
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
	case "list":
		return listWorkCommissions(ctx, store, args)
	case "list_runnable":
		return listRunnableWorkCommissions(ctx, store, args)
	case "drain_status":
		return drainWorkCommissionStatus(ctx, store, args)
	case "show":
		return showWorkCommission(ctx, store, args)
	case "claim_for_preflight":
		return claimWorkCommissionForPreflight(ctx, store, args)
	case "requeue":
		return requeueWorkCommission(ctx, store, args)
	case "cancel":
		return cancelWorkCommission(ctx, store, args)
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

	if err := guardMultiCommissionDecision(ctx, store, args); err != nil {
		return "", err
	}

	commission, err := buildWorkCommissionFromDecision(ctx, store, args, now)
	if err != nil {
		return "", err
	}

	return persistWorkCommission(ctx, store, commission, now)
}

// guardMultiCommissionDecision enforces that the second-or-later commission
// against the same DecisionRecord must carry a non-empty `slice_description`.
// Inheriting the parent decision body across multiple commissions without
// per-slice scope text leaks scope between codex sessions: each agent reads
// the full decision text and independently implements every slice whose
// scope intersects its allowed_paths. See
// `.context/multi-commission-anti-pattern-retrospective.md`.
func guardMultiCommissionDecision(
	ctx context.Context,
	store *artifact.Store,
	args map[string]any,
) error {
	decisionRef := strings.TrimSpace(stringArg(args, "decision_ref"))
	if decisionRef == "" {
		return nil
	}
	sliceDescription := strings.TrimSpace(stringArg(args, "slice_description"))

	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return err
	}

	existing := liveCommissionsForDecision(records, decisionRef)
	if len(existing) == 0 {
		return nil
	}

	if sliceDescription == "" {
		existingIDs := make([]string, 0, len(existing))
		for _, commission := range existing {
			existingIDs = append(existingIDs, stringField(commission, "id"))
		}
		return fmt.Errorf(
			"multi_commission_requires_slice_description: decision %q already has %d non-terminal commission(s) (%s); subsequent commissions for the same decision must declare `slice_description` to scope which slice of the decision THIS commission implements (see .context/multi-commission-anti-pattern-retrospective.md)",
			decisionRef,
			len(existing),
			strings.Join(existingIDs, ", "),
		)
	}

	for _, commission := range existing {
		other := strings.TrimSpace(stringField(commission, "slice_description"))
		if other == "" {
			otherID := stringField(commission, "id")
			return fmt.Errorf(
				"multi_commission_existing_lacks_slice_description: decision %q already has commission %q without a slice_description; either cancel that commission or update it before creating slice %q",
				decisionRef,
				otherID,
				sliceDescription,
			)
		}
	}

	return nil
}

// liveCommissionsForDecision returns commissions linked to the decision_ref
// whose state is not terminal (cancelled / expired / completed_with_projection_debt
// / completed are excluded — those are audit records, not active claims).
func liveCommissionsForDecision(records []map[string]any, decisionRef string) []map[string]any {
	live := make([]map[string]any, 0)
	for _, commission := range records {
		if stringField(commission, "decision_ref") != decisionRef {
			continue
		}
		state := stringField(commission, "state")
		if commissionStateIsTerminal(state) {
			continue
		}
		live = append(live, commission)
	}
	return live
}

func commissionStateIsTerminal(state string) bool {
	switch strings.TrimSpace(state) {
	case "cancelled", "expired", "completed", "completed_with_projection_debt", "failed", "blocked_policy":
		return true
	}
	return false
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

	if _, err := implementationplan.ParsePayload(plan); err != nil {
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
	next["delivery_policy"] = firstStringField("delivery_policy", entry, defaults, plan, args)
	next["state"] = firstStringField("state", entry, defaults, plan, args)
	next["valid_for"] = firstStringField("valid_for", entry, defaults, plan, args)
	next["valid_until"] = firstStringField("valid_until", entry, defaults, plan, args)
	next["queue"] = firstStringField("queue", entry, defaults, plan, args)
	if envelope := firstMapField("autonomy_envelope_snapshot", entry, defaults, plan, args); envelope != nil {
		next["autonomy_envelope_snapshot"] = envelope
	}
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

func firstMapField(key string, maps ...map[string]any) map[string]any {
	for _, values := range maps {
		if values == nil {
			continue
		}
		if value, ok := mapArg(values, key); ok {
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
	putOptionalString(summary, "delivery_policy", stringField(plan, "delivery_policy"))
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
	specSectionRefs := decisionSpecSectionRefs(decision, fields)

	problemRef, problemHash, err := primaryProblemRefAndHash(ctx, store, decision, fields)
	if err != nil {
		return nil, err
	}

	decisionHash, err := decisionRevisionHash(ctx, store, decision)
	if err != nil {
		return nil, err
	}

	specSnapshot, specRevisionHashes, specEvidenceRequirements, err := commissionSpecSnapshot(args, specSectionRefs, now)
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
	evidence = append(evidence, specEvidenceRequirements...)

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
		"delivery_policy":        input.DeliveryPolicy,
		"state":                  input.State,
		"valid_until":            input.ValidUntil,
		"fetched_at":             now.Format(time.RFC3339),
	}
	if input.SliceDescription != "" {
		commission["slice_description"] = input.SliceDescription
	}
	if len(specSectionRefs) > 0 {
		commission["spec_section_refs"] = stringSliceToAny(specSectionRefs)
		commission["spec_revision_hashes"] = specRevisionHashes
		commission["spec_snapshot"] = specSnapshot
	}

	putOptionalString(commission, "implementation_plan_ref", stringArg(args, "implementation_plan_ref"))
	putOptionalString(commission, "implementation_plan_revision", stringArg(args, "implementation_plan_revision"))
	putOptionalString(commission, "autonomy_envelope_ref", stringArg(args, "autonomy_envelope_ref"))
	putOptionalString(commission, "autonomy_envelope_revision", stringArg(args, "autonomy_envelope_revision"))
	putOptionalString(commission, "queue", stringArg(args, "queue"))
	if envelopeSnapshot, ok, err := autonomyEnvelopeSnapshotFromArgs(args); err != nil {
		return nil, err
	} else if ok {
		commission, err = withCommissionAutonomyEnvelope(commission, envelopeSnapshot, now)
		if err != nil {
			return nil, err
		}
	}
	if override, ok := mapArg(args, "spec_readiness_override"); ok {
		commission = withCommissionSpecReadinessOverride(commission, override, now)
	}

	return commission, nil
}

func listRunnableWorkCommissions(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return "", err
	}

	leaseAgeCap, err := commissionLeaseAgeCap(args)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	commissions, skipped := runnableAndSkippedWorkCommissions(records, args, now, leaseAgeCap)

	return commissionResponseMap(map[string]any{
		"commissions": commissions,
		"skipped":     skipped,
	})
}

func runnableAndSkippedWorkCommissions(
	records []map[string]any,
	args map[string]any,
	now time.Time,
	leaseAgeCap time.Duration,
) ([]map[string]any, []map[string]any) {
	commissions := make([]map[string]any, 0, len(records))
	skipped := make([]map[string]any, 0)

	for _, commission := range records {
		if !workCommissionMatchesRequest(commission, args) {
			continue
		}
		if skip := workCommissionIntakeSkip(commission, now, leaseAgeCap); skip != nil {
			skipped = append(skipped, skip)
			continue
		}
		if workCommissionRunnableForRequestWithLeaseCap(commission, records, args, now, leaseAgeCap) {
			commissions = append(commissions, commission)
		}
	}

	return commissions, skipped
}

func listWorkCommissions(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return "", err
	}

	selector := stringArg(args, "selector")
	if selector == "" {
		selector = "open"
	}
	if !validWorkCommissionListSelector(selector) {
		return "", fmt.Errorf("invalid commission selector: %s", selector)
	}

	olderThan, err := commissionAttentionDuration(args)
	if err != nil {
		return "", err
	}

	leaseAgeCap, err := commissionLeaseAgeCap(args)
	if err != nil {
		return "", err
	}

	stateFilter := stringArg(args, "state")
	now := time.Now().UTC()
	commissions := make([]map[string]any, 0, len(records))
	for _, commission := range records {
		if !workCommissionListSelectorMatchesWithLeaseCap(commission, records, selector, args, now, olderThan, leaseAgeCap) {
			continue
		}
		if stateFilter != "" && stringField(commission, "state") != stateFilter {
			continue
		}
		commissions = append(
			commissions,
			workCommissionWithOperatorFieldsAndLeaseCap(commission, now, olderThan, leaseAgeCap),
		)
	}

	return commissionResponse("commissions", commissions)
}

func drainWorkCommissionStatus(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return "", err
	}

	leaseAgeCap, err := commissionLeaseAgeCap(args)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	commissions, skipped := runnableAndSkippedWorkCommissions(records, args, now, leaseAgeCap)
	drain := map[string]any{
		"empty":          len(commissions) == 0,
		"runnable_count": len(commissions),
		"skipped_count":  len(skipped),
		"lease_age_cap":  leaseAgeCap.String(),
	}

	return commissionResponseMap(map[string]any{
		"drain":       drain,
		"commissions": commissions,
		"skipped":     skipped,
	})
}

func showWorkCommission(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	commissionID := stringArg(args, "commission_id")
	if commissionID == "" {
		return "", fmt.Errorf("commission_id is required")
	}

	olderThan, err := commissionAttentionDuration(args)
	if err != nil {
		return "", err
	}

	leaseAgeCap, err := commissionLeaseAgeCap(args)
	if err != nil {
		return "", err
	}

	commission, err := loadWorkCommissionPayload(ctx, store, commissionID)
	if err != nil {
		return "", err
	}

	commission = workCommissionWithOperatorFieldsAndLeaseCap(
		commission,
		time.Now().UTC(),
		olderThan,
		leaseAgeCap,
	)
	return commissionResponse("commission", commission)
}

func cancelWorkCommission(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	commissionID := stringArg(args, "commission_id")
	if commissionID == "" {
		return "", fmt.Errorf("commission_id is required")
	}

	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin WorkCommission cancel: %w", err)
	}
	defer tx.Rollback()

	commission, err := loadWorkCommissionPayloadForUpdate(ctx, tx, commissionID)
	if err != nil {
		return "", err
	}
	if err := ensureWorkCommissionTransition(commission, args, cancelWorkCommissionTransition, time.Now().UTC()); err != nil {
		return "", err
	}

	commission = appendCancelLifecycleEvent(commission, args)
	commission["state"] = cancelWorkCommissionTransition.TargetState
	delete(commission, "lease")

	if err := updateWorkCommissionPayload(ctx, tx, commission); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit WorkCommission cancel: %w", err)
	}

	return commissionResponse("commission", commission)
}

func claimWorkCommissionForPreflight(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	runnerID := stringArg(args, "runner_id")
	if runnerID == "" {
		runnerID = "haft"
	}

	leaseAgeCap, err := commissionLeaseAgeCap(args)
	if err != nil {
		return "", err
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

	now := time.Now().UTC()
	commission, err := selectWorkCommissionForClaimWithLeaseCap(commissions, args, now, leaseAgeCap)
	if err != nil {
		return "", err
	}
	if err := ensureWorkCommissionLocksetAvailable(commissions, commission); err != nil {
		return "", err
	}

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

func requeueWorkCommission(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	commissionID := stringArg(args, "commission_id")
	if commissionID == "" {
		return "", fmt.Errorf("commission_id is required")
	}

	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin WorkCommission requeue: %w", err)
	}
	defer tx.Rollback()

	commission, err := loadWorkCommissionPayloadForUpdate(ctx, tx, commissionID)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	if err := ensureWorkCommissionTransition(commission, args, requeueWorkCommissionTransition, now); err != nil {
		return "", err
	}

	commission = appendRequeueLifecycleEvent(commission, args)
	commission["state"] = requeueWorkCommissionTransition.TargetState
	if requeueWorkCommissionTransition.RefreshFetchedAt {
		commission["fetched_at"] = now.Format(time.RFC3339)
	}
	delete(commission, "lease")

	if err := updateWorkCommissionPayload(ctx, tx, commission); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit WorkCommission requeue: %w", err)
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

	if err := ensureWorkCommissionLifecycleTransition(commission, args); err != nil {
		return "", err
	}

	freshnessReport, err := workCommissionFreshnessReport(ctx, store, commission, args)
	if err != nil {
		return "", err
	}
	if commissionFreshnessBlocked(freshnessReport) {
		commission = appendFreshnessBlockLifecycleEvent(commission, args, freshnessReport)
		commission["state"] = "blocked_stale"
		if err := updateWorkCommissionPayload(ctx, tx, commission); err != nil {
			return "", err
		}
		if err := tx.Commit(); err != nil {
			return "", fmt.Errorf("commit WorkCommission freshness block: %w", err)
		}
		return "", commissionFreshnessBlockError(freshnessReport)
	}

	args = workCommissionArgsWithFreshnessGaps(args, freshnessReport)
	if stringArg(args, "action") == "start_after_preflight" {
		envelopeReport, envelopePresent, err := workCommissionAutonomyEnvelopeReport(commission, time.Now().UTC())
		if err != nil {
			return "", err
		}
		if envelopePresent && commissionAutonomyEnvelopeBlocked(envelopeReport) {
			commission = appendAutonomyEnvelopeBlockLifecycleEvent(commission, args, envelopeReport)
			commission["state"] = autonomyEnvelopeBlockState(envelopeReport)
			if err := updateWorkCommissionPayload(ctx, tx, commission); err != nil {
				return "", err
			}
			if err := tx.Commit(); err != nil {
				return "", fmt.Errorf("commit WorkCommission autonomy envelope block: %w", err)
			}
			return "", commissionAutonomyEnvelopeBlockError(envelopeReport)
		}
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
		if key == "action" || key == "config_hash" || key == "project_root" {
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
		DeliveryPolicy:       stringArg(args, "delivery_policy"),
		State:                stringArg(args, "state"),
		ValidUntil:           validUntil,
		SliceDescription:     strings.TrimSpace(stringArg(args, "slice_description")),
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
	input.DeliveryPolicy = strings.TrimSpace(input.DeliveryPolicy)
	input.State = strings.TrimSpace(input.State)
	if input.ProjectionPolicy == "" {
		input.ProjectionPolicy = "local_only"
	}
	if input.DeliveryPolicy == "" {
		input.DeliveryPolicy = defaultDeliveryPolicy
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
	if !validDeliveryPolicy(input.DeliveryPolicy) {
		return commissionFromDecisionInput{}, fmt.Errorf("invalid delivery_policy: %s", input.DeliveryPolicy)
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

func decisionSpecSectionRefs(
	decision *artifact.Artifact,
	fields artifact.DecisionFields,
) []string {
	refs := append([]string(nil), fields.SectionRefs...)

	for _, link := range decision.Meta.Links {
		if link.Type != "governs" {
			continue
		}

		refs = append(refs, link.Ref)
	}

	return sortedUniqueStrings(refs)
}

func commissionSpecSnapshot(
	args map[string]any,
	sectionRefs []string,
	now time.Time,
) (map[string]any, map[string]any, []any, error) {
	refs := sortedUniqueStrings(sectionRefs)
	if len(refs) == 0 {
		return nil, nil, nil, nil
	}

	projectRoot := stringArg(args, "project_root")
	sectionsByID, err := commissionSpecSectionsByID(projectRoot)
	if err != nil {
		return nil, nil, nil, err
	}

	snapshotSections, revisionHashes, unresolvedRefs, err := commissionSpecSectionSnapshots(refs, sectionsByID)
	if err != nil {
		return nil, nil, nil, err
	}
	if projectRoot != "" && len(unresolvedRefs) > 0 {
		return nil, nil, nil, fmt.Errorf("spec_section_refs not found in ProjectSpecificationSet: %s", strings.Join(unresolvedRefs, ", "))
	}

	snapshot := map[string]any{
		"captured_at":      now.Format(time.RFC3339),
		"section_refs":     stringSliceToAny(refs),
		"revision_hashes":  revisionHashes,
		"snapshot_source":  commissionSpecSnapshotSource(projectRoot),
		"snapshot_state":   commissionSpecSnapshotState(unresolvedRefs),
		"spec_section_set": snapshotSections,
	}
	if len(unresolvedRefs) > 0 {
		snapshot["unresolved_refs"] = stringSliceToAny(unresolvedRefs)
	}

	requirements := commissionSpecEvidenceRequirements(refs, sectionsByID)
	return snapshot, revisionHashes, requirements, nil
}

func commissionSpecSectionsByID(projectRoot string) (map[string]project.SpecSection, error) {
	sectionsByID := map[string]project.SpecSection{}
	if strings.TrimSpace(projectRoot) == "" {
		return sectionsByID, nil
	}

	sections, err := project.LoadSpecSections(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load ProjectSpecificationSet sections: %w", err)
	}

	for _, section := range sections {
		if strings.TrimSpace(section.ID) == "" {
			continue
		}

		sectionsByID[section.ID] = section
	}

	return sectionsByID, nil
}

func commissionSpecSectionSnapshots(
	refs []string,
	sectionsByID map[string]project.SpecSection,
) ([]any, map[string]any, []string, error) {
	snapshots := make([]any, 0, len(refs))
	revisionHashes := make(map[string]any, len(refs))
	unresolvedRefs := make([]string, 0)

	for _, ref := range refs {
		section, ok := sectionsByID[ref]
		if !ok {
			snapshot, revisionHash, err := commissionRefOnlySpecSectionSnapshot(ref)
			if err != nil {
				return nil, nil, nil, err
			}

			snapshots = append(snapshots, snapshot)
			revisionHashes[ref] = revisionHash
			unresolvedRefs = append(unresolvedRefs, ref)
			continue
		}

		snapshot, revisionHash, err := commissionResolvedSpecSectionSnapshot(section)
		if err != nil {
			return nil, nil, nil, err
		}

		snapshots = append(snapshots, snapshot)
		revisionHashes[section.ID] = revisionHash
	}

	return snapshots, revisionHashes, sortedUniqueStrings(unresolvedRefs), nil
}

func commissionResolvedSpecSectionSnapshot(
	section project.SpecSection,
) (map[string]any, string, error) {
	snapshot := map[string]any{
		"claim_layer":       section.ClaimLayer,
		"document_kind":     section.DocumentKind,
		"evidence_required": specEvidenceRequirementsToAny(section.EvidenceRequired),
		"id":                section.ID,
		"kind":              section.Kind,
		"line":              section.Line,
		"owner":             section.Owner,
		"path":              section.Path,
		"snapshot_state":    "resolved",
		"status":            section.Status,
		"statement_type":    section.StatementType,
		"title":             section.Title,
		"valid_until":       section.ValidUntil,
	}

	revisionHash, err := canonicalJSONHash(snapshot)
	if err != nil {
		return nil, "", err
	}

	snapshot["revision_hash"] = revisionHash
	return snapshot, revisionHash, nil
}

func commissionRefOnlySpecSectionSnapshot(ref string) (map[string]any, string, error) {
	snapshot := map[string]any{
		"id":             ref,
		"snapshot_state": "ref_only",
	}

	revisionHash, err := canonicalJSONHash(snapshot)
	if err != nil {
		return nil, "", err
	}

	snapshot["revision_hash"] = revisionHash
	return snapshot, revisionHash, nil
}

func commissionSpecSnapshotSource(projectRoot string) string {
	if strings.TrimSpace(projectRoot) != "" {
		return "project_specification_set"
	}

	return "decision_record"
}

func commissionSpecSnapshotState(unresolvedRefs []string) string {
	if len(unresolvedRefs) == 0 {
		return "resolved"
	}

	return "ref_only"
}

func commissionSpecEvidenceRequirements(
	refs []string,
	sectionsByID map[string]project.SpecSection,
) []any {
	requirements := make([]any, 0)

	for _, ref := range refs {
		section, ok := sectionsByID[ref]
		if !ok {
			continue
		}

		for _, requirement := range section.EvidenceRequired {
			entry := specEvidenceRequirementToMap(requirement)
			if len(entry) == 0 {
				continue
			}

			entry["spec_section_ref"] = section.ID
			requirements = append(requirements, entry)
		}
	}

	return requirements
}

func specEvidenceRequirementsToAny(requirements []project.SpecEvidenceRequirement) []any {
	result := make([]any, 0, len(requirements))

	for _, requirement := range requirements {
		entry := specEvidenceRequirementToMap(requirement)
		if len(entry) == 0 {
			continue
		}

		result = append(result, entry)
	}

	return result
}

func specEvidenceRequirementToMap(requirement project.SpecEvidenceRequirement) map[string]any {
	entry := map[string]any{}
	if strings.TrimSpace(requirement.Kind) != "" {
		entry["kind"] = strings.TrimSpace(requirement.Kind)
	}
	if strings.TrimSpace(requirement.Description) != "" {
		entry["description"] = strings.TrimSpace(requirement.Description)
	}

	return entry
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
	if stringField(commission, "delivery_policy") == "" {
		commission["delivery_policy"] = defaultDeliveryPolicy
	}
	if !validProjectionPolicy(stringField(commission, "projection_policy")) {
		return fmt.Errorf("invalid projection_policy: %s", stringField(commission, "projection_policy"))
	}
	if !validDeliveryPolicy(stringField(commission, "delivery_policy")) {
		return fmt.Errorf("invalid delivery_policy: %s", stringField(commission, "delivery_policy"))
	}
	if !validWorkCommissionState(stringField(commission, "state")) {
		return fmt.Errorf("invalid WorkCommission state: %s", stringField(commission, "state"))
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
		"delivery_policy",
		"state",
		"valid_until",
		"fetched_at",
	}
	for _, key := range required {
		if stringField(commission, key) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	scope, ok := mapArg(commission, "scope")
	if !ok {
		return fmt.Errorf("scope is required")
	}
	if err := ensureWorkCommissionScopeSnapshot(commission, scope); err != nil {
		return err
	}
	if err := ensureWorkCommissionSpecAuthority(commission, now); err != nil {
		return err
	}
	if err := ensureWorkCommissionAutonomyEnvelope(commission, now); err != nil {
		return err
	}
	return nil
}

func ensureWorkCommissionScopeSnapshot(commission map[string]any, scope map[string]any) error {
	scopeHash, err := workCommissionScopeHash(scope)
	if err != nil {
		return err
	}

	if stringField(commission, "scope_hash") == "" {
		commission["scope_hash"] = scopeHash
	}
	if stringField(scope, "hash") == "" {
		scope["hash"] = scopeHash
	}
	if stringField(commission, "scope_hash") != scopeHash {
		return fmt.Errorf("scope_hash does not match canonical scope")
	}
	if stringField(scope, "hash") != scopeHash {
		return fmt.Errorf("scope.hash does not match canonical scope")
	}

	scopeBaseSHA := stringField(scope, "base_sha")
	if stringField(commission, "base_sha") == "" {
		commission["base_sha"] = scopeBaseSHA
	}
	if stringField(commission, "base_sha") != scopeBaseSHA {
		return fmt.Errorf("base_sha does not match scope.base_sha")
	}

	return nil
}

func ensureWorkCommissionSpecAuthority(commission map[string]any, now time.Time) error {
	refs := workCommissionSpecSectionRefs(commission)
	if len(refs) > 0 {
		commission["spec_section_refs"] = stringSliceToAny(refs)
		ensureWorkCommissionSpecSnapshot(commission, refs, now)
		return nil
	}

	if commissionHasTacticalSpecReadinessOverride(commission) {
		return nil
	}

	return fmt.Errorf("spec_section_refs is required unless spec_readiness_override records explicit out-of-spec tactical work")
}

func ensureWorkCommissionSpecSnapshot(
	commission map[string]any,
	refs []string,
	now time.Time,
) {
	if _, ok := mapArg(commission, "spec_snapshot"); ok {
		return
	}

	snapshot, revisionHashes, _, err := commissionSpecSnapshot(map[string]any{}, refs, now)
	if err != nil {
		return
	}

	commission["spec_snapshot"] = snapshot
	commission["spec_revision_hashes"] = revisionHashes
}

func workCommissionSpecSectionRefs(commission map[string]any) []string {
	refs := make([]string, 0)
	refs = append(refs, stringSliceField(commission, "spec_section_refs")...)
	refs = append(refs, stringSliceField(commission, "section_refs")...)

	snapshot, ok := mapArg(commission, "spec_snapshot")
	if ok {
		refs = append(refs, stringSliceField(snapshot, "section_refs")...)
	}

	return sortedUniqueStrings(refs)
}

func commissionHasTacticalSpecReadinessOverride(commission map[string]any) bool {
	override, ok := mapArg(commission, "spec_readiness_override")
	if !ok {
		return false
	}
	if stringField(override, "kind") != "tactical" {
		return false
	}
	if !boolField(override, "out_of_spec") {
		return false
	}
	if stringField(override, "reason") == "" {
		return false
	}

	return true
}

func autonomyEnvelopeSnapshotFromArgs(args map[string]any) (map[string]any, bool, error) {
	if snapshot, ok := mapArg(args, "autonomy_envelope_snapshot"); ok {
		return copyStringAnyMap(snapshot), true, nil
	}
	if snapshot, ok := mapArg(args, "autonomy_envelope"); ok {
		return copyStringAnyMap(snapshot), true, nil
	}
	return nil, false, nil
}

func withCommissionAutonomyEnvelope(
	commission map[string]any,
	envelopeSnapshot map[string]any,
	now time.Time,
) (map[string]any, error) {
	normalized, snapshot, err := autonomyenvelope.NormalizeSnapshot(envelopeSnapshot)
	if err != nil {
		return nil, err
	}

	commission["autonomy_envelope_snapshot"] = normalized
	if err := ensureAutonomyEnvelopeFieldMatch(commission, "autonomy_envelope_ref", snapshot.Ref); err != nil {
		return nil, err
	}
	if err := ensureAutonomyEnvelopeFieldMatch(commission, "autonomy_envelope_revision", snapshot.Revision); err != nil {
		return nil, err
	}

	report := snapshot.Evaluate(autonomyEnvelopeCommissionRequest(commission), now)
	if report.Decision == autonomyenvelope.DecisionBlocked {
		return nil, commissionAutonomyEnvelopeBlockError(report)
	}

	return commission, nil
}

func ensureWorkCommissionAutonomyEnvelope(commission map[string]any, now time.Time) error {
	envelopeSnapshot, ok := mapArg(commission, "autonomy_envelope_snapshot")
	if !ok {
		return nil
	}

	_, err := withCommissionAutonomyEnvelope(commission, envelopeSnapshot, now)
	return err
}

func ensureAutonomyEnvelopeFieldMatch(
	commission map[string]any,
	field string,
	value string,
) error {
	current := stringField(commission, field)
	if current == "" {
		commission[field] = value
		return nil
	}
	if current == value {
		return nil
	}

	return fmt.Errorf("%s does not match autonomy_envelope_snapshot", field)
}

func autonomyEnvelopeCommissionRequest(commission map[string]any) autonomyenvelope.CommissionRequest {
	scope, _ := mapArg(commission, "scope")

	return autonomyenvelope.CommissionRequest{
		RepoRef:        stringField(scope, "repo_ref"),
		AllowedPaths:   scopeStringSlice(scope, "allowed_paths"),
		ForbiddenPaths: scopeStringSlice(scope, "forbidden_paths"),
		AllowedActions: scopeStringSlice(scope, "allowed_actions"),
		AllowedModules: scopeStringSlice(scope, "allowed_modules"),
	}
}

func workCommissionAutonomyEnvelopeReport(
	commission map[string]any,
	now time.Time,
) (autonomyenvelope.Report, bool, error) {
	envelopeSnapshot, ok := mapArg(commission, "autonomy_envelope_snapshot")
	if !ok {
		return autonomyenvelope.Report{}, false, nil
	}

	_, snapshot, err := autonomyenvelope.NormalizeSnapshot(envelopeSnapshot)
	if err != nil {
		return autonomyenvelope.Report{}, true, err
	}

	return snapshot.Evaluate(autonomyEnvelopeCommissionRequest(commission), now), true, nil
}

func workCommissionAutonomyEnvelopeRunnable(
	commission map[string]any,
	now time.Time,
) bool {
	report, ok, err := workCommissionAutonomyEnvelopeReport(commission, now)
	if err != nil {
		return false
	}
	if !ok {
		return true
	}

	return report.Decision == autonomyenvelope.DecisionAllowed
}

func commissionAutonomyEnvelopeBlocked(report autonomyenvelope.Report) bool {
	return report.Decision == autonomyenvelope.DecisionBlocked
}

func commissionAutonomyEnvelopeBlockError(report autonomyenvelope.Report) error {
	return fmt.Errorf(
		"commission_autonomy_envelope_blocked: %s",
		strings.Join(commissionAutonomyEnvelopeFindingCodes(report), ","),
	)
}

func commissionAutonomyEnvelopeFindingCodes(report autonomyenvelope.Report) []string {
	codes := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		codes = append(codes, finding.Code)
	}
	return sortedUniqueStrings(codes)
}

func appendAutonomyEnvelopeBlockLifecycleEvent(
	commission map[string]any,
	args map[string]any,
	report autonomyenvelope.Report,
) map[string]any {
	eventArgs := copyStringAnyMap(args)
	eventArgs["event"] = "autonomy_envelope_blocked"
	eventArgs["verdict"] = "blocked"
	eventArgs["reason"] = commissionAutonomyEnvelopeBlockError(report).Error()

	payload, _ := mapArg(eventArgs, "payload")
	if payload == nil {
		payload = map[string]any{}
	} else {
		payload = copyStringAnyMap(payload)
	}
	payload["autonomy_envelope_findings"] = autonomyEnvelopeFindingMaps(report.Findings)
	eventArgs["payload"] = payload

	return appendLifecycleEvent(commission, eventArgs)
}

func autonomyEnvelopeFindingMaps(findings []autonomyenvelope.Finding) []any {
	result := make([]any, 0, len(findings))
	for _, finding := range findings {
		result = append(result, map[string]any{
			"code":   finding.Code,
			"field":  finding.Field,
			"detail": finding.Detail,
		})
	}
	return result
}

func autonomyEnvelopeBlockState(report autonomyenvelope.Report) string {
	for _, finding := range report.Findings {
		switch finding.Code {
		case "repo_outside_autonomy_envelope",
			"path_outside_autonomy_envelope",
			"action_outside_autonomy_envelope",
			"module_outside_autonomy_envelope",
			"action_forbidden_by_autonomy_envelope",
			"one_way_door_action_forbidden":
			return "needs_human_review"
		}
	}
	return "blocked_policy"
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

func loadWorkCommissionPayload(
	ctx context.Context,
	store *artifact.Store,
	commissionID string,
) (map[string]any, error) {
	item, err := store.Get(ctx, commissionID)
	if err != nil {
		return nil, err
	}
	if item.Meta.Kind != artifact.KindWorkCommission {
		return nil, fmt.Errorf("%s is %s, not WorkCommission", commissionID, item.Meta.Kind)
	}

	return decodeWorkCommissionPayload(commissionID, item.StructuredData)
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
	if !workcommission.IsRunnableState(state) {
		return false
	}

	validUntil, err := time.Parse(time.RFC3339, stringField(commission, "valid_until"))
	if err != nil {
		return false
	}
	return validUntil.After(now)
}

func workCommissionIntakeSkip(
	commission map[string]any,
	now time.Time,
	leaseAgeCap time.Duration,
) map[string]any {
	attention := workCommissionLeaseAgeCapAttention(commission, now, leaseAgeCap)
	if attention.Code == "" {
		return nil
	}

	lease, _ := mapArg(commission, "lease")
	claimedAt := stringField(lease, "claimed_at")
	return map[string]any{
		"id":         stringField(commission, "id"),
		"state":      stringField(commission, "state"),
		"reason":     attention.Code,
		"message":    attention.Reason,
		"claimed_at": claimedAt,
		"age_cap":    leaseAgeCap.String(),
	}
}

func commissionIntakeSkipError(skip map[string]any) error {
	return fmt.Errorf(
		"%s: commission_id=%s age_cap=%s",
		stringField(skip, "reason"),
		stringField(skip, "id"),
		stringField(skip, "age_cap"),
	)
}

func workCommissionRunnableForRequest(
	commission map[string]any,
	commissions []map[string]any,
	args map[string]any,
	now time.Time,
) bool {
	return workCommissionRunnableForRequestWithLeaseCap(
		commission,
		commissions,
		args,
		now,
		defaultCommissionLeaseAgeCap,
	)
}

func workCommissionRunnableForRequestWithLeaseCap(
	commission map[string]any,
	commissions []map[string]any,
	args map[string]any,
	now time.Time,
	leaseAgeCap time.Duration,
) bool {
	return workCommissionMatchesRequest(commission, args) &&
		workCommissionIntakeSkip(commission, now, leaseAgeCap) == nil &&
		workCommissionRunnable(commission, now) &&
		workCommissionDependenciesSatisfied(commission, commissions) &&
		workCommissionAutonomyEnvelopeRunnable(commission, now)
}

func workCommissionMatchesRequest(commission map[string]any, args map[string]any) bool {
	planRef := stringArg(args, "plan_ref")
	if planRef != "" && stringField(commission, "implementation_plan_ref") != planRef {
		return false
	}

	planRevision := stringArg(args, "plan_revision")
	if planRevision != "" && stringField(commission, "implementation_plan_revision") != planRevision {
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

	satisfiedByID := make(map[string]bool, len(commissions))
	for _, candidate := range commissions {
		satisfiedByID[stringField(candidate, "id")] = workCommissionDependencySatisfied(candidate)
	}

	return implementationplan.DependenciesSatisfied(dependencyIDs, satisfiedByID)
}

func workCommissionDependencySatisfied(commission map[string]any) bool {
	state := stringField(commission, "state")
	return workcommission.SatisfiesDependencyState(state)
}

func selectWorkCommissionForClaimWithLeaseCap(
	commissions []map[string]any,
	args map[string]any,
	now time.Time,
	leaseAgeCap time.Duration,
) (map[string]any, error) {
	commissionID := stringArg(args, "commission_id")
	var firstSkip map[string]any

	for _, commission := range commissions {
		if commissionID != "" && stringField(commission, "id") != commissionID {
			continue
		}
		if skip := workCommissionIntakeSkip(commission, now, leaseAgeCap); skip != nil {
			if commissionID != "" {
				return nil, commissionIntakeSkipError(skip)
			}
			if firstSkip == nil {
				firstSkip = skip
			}
			continue
		}
		if workCommissionRunnableForRequestWithLeaseCap(commission, commissions, args, now, leaseAgeCap) {
			return commission, nil
		}
		if commissionID != "" {
			return nil, fmt.Errorf("commission_not_runnable")
		}
	}

	if commissionID != "" {
		return nil, fmt.Errorf("commission_not_found")
	}
	if firstSkip != nil {
		return nil, commissionIntakeSkipError(firstSkip)
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
	return workcommission.IsExecutingState(state)
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
	return implementationplan.LocksetsOverlap(left, right)
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

func validDeliveryPolicy(value string) bool {
	switch value {
	case "workspace_patch_manual", "workspace_patch_auto_on_pass":
		return true
	default:
		return false
	}
}

func validWorkCommissionListSelector(value string) bool {
	switch value {
	case "all", "open", "stale", "terminal", "runnable":
		return true
	default:
		return false
	}
}

func validWorkCommissionState(value string) bool {
	return workcommission.IsKnownState(value)
}

func workCommissionListSelectorMatchesWithLeaseCap(
	commission map[string]any,
	commissions []map[string]any,
	selector string,
	args map[string]any,
	now time.Time,
	openAttentionAfter time.Duration,
	leaseAgeCap time.Duration,
) bool {
	switch selector {
	case "all":
		return true
	case "open":
		return !workCommissionTerminal(commission)
	case "terminal":
		return workCommissionTerminal(commission)
	case "stale":
		return workCommissionAttentionWithLeaseCap(commission, now, openAttentionAfter, leaseAgeCap).Reason != ""
	case "runnable":
		return workCommissionRunnableForRequestWithLeaseCap(commission, commissions, args, now, leaseAgeCap)
	default:
		return false
	}
}

func workCommissionWithOperatorFieldsAndLeaseCap(
	commission map[string]any,
	now time.Time,
	openAttentionAfter time.Duration,
	leaseAgeCap time.Duration,
) map[string]any {
	enriched := copyStringAnyMap(commission)
	attention := workCommissionAttentionWithLeaseCap(commission, now, openAttentionAfter, leaseAgeCap)
	enriched["operator"] = map[string]any{
		"terminal":          workCommissionTerminal(commission),
		"expired":           workCommissionExpired(commission, now),
		"attention":         attention.Reason != "",
		"attention_code":    attention.Code,
		"attention_reason":  attention.Reason,
		"suggested_actions": workCommissionSuggestedActions(commission, attention.Reason, now),
	}
	return enriched
}

func workCommissionSuggestedActions(commission map[string]any, reason string, now time.Time) []any {
	if reason == "" {
		return []any{}
	}
	if workCommissionExpired(commission, now) {
		return []any{"inspect", "cancel"}
	}

	state := stringField(commission, "state")
	if state == "blocked_stale" {
		return []any{"refresh_decision", "requeue", "cancel"}
	}
	if workcommission.IsRecoverableState(state) {
		return []any{"inspect", "requeue", "cancel"}
	}

	return []any{"inspect", "cancel"}
}

func workCommissionAttentionWithLeaseCap(
	commission map[string]any,
	now time.Time,
	openAttentionAfter time.Duration,
	leaseAgeCap time.Duration,
) commissionOperatorAttention {
	state := stringField(commission, "state")
	if workcommission.IsTerminalState(state) {
		return commissionOperatorAttention{}
	}
	if workCommissionExpired(commission, now) {
		return commissionOperatorAttention{
			Code:   "expired_before_terminal",
			Reason: "expired before terminal state",
		}
	}

	if attention := workCommissionLeaseAgeCapAttention(commission, now, leaseAgeCap); attention.Code != "" {
		return attention
	}

	if workcommission.RequiresOperatorDecisionState(state) {
		return commissionOperatorAttention{
			Code:   "operator_decision_required",
			Reason: "requires operator decision: " + state,
		}
	}
	if workcommission.IsExecutingState(state) {
		return activeLeaseAttention(commission, now)
	}

	return openCommissionAttention(commission, now, openAttentionAfter)
}

func activeLeaseAttention(commission map[string]any, now time.Time) commissionOperatorAttention {
	lease, ok := mapArg(commission, "lease")
	if !ok {
		return commissionOperatorAttention{
			Code:   "active_lease_missing",
			Reason: "active state has no lease",
		}
	}

	claimedAt, ok := parseRFC3339Field(lease, "claimed_at")
	if !ok {
		return commissionOperatorAttention{
			Code:   "active_lease_missing_claimed_at",
			Reason: "active lease has no claimed_at",
		}
	}
	if now.Sub(claimedAt) >= defaultCommissionLeaseAttentionAfter {
		return commissionOperatorAttention{
			Code:   "active_lease_old",
			Reason: "active lease older than " + defaultCommissionLeaseAttentionAfter.String(),
		}
	}
	return commissionOperatorAttention{}
}

func openCommissionAttention(
	commission map[string]any,
	now time.Time,
	openAttentionAfter time.Duration,
) commissionOperatorAttention {
	fetchedAt, ok := parseRFC3339Field(commission, "fetched_at")
	if !ok {
		return commissionOperatorAttention{
			Code:   "open_missing_fetched_at",
			Reason: "open commission has no fetched_at",
		}
	}
	if now.Sub(fetchedAt) >= openAttentionAfter {
		return commissionOperatorAttention{
			Code:   "open_too_long",
			Reason: "open longer than " + openAttentionAfter.String(),
		}
	}
	return commissionOperatorAttention{}
}

func workCommissionLeaseAgeCapAttention(
	commission map[string]any,
	now time.Time,
	leaseAgeCap time.Duration,
) commissionOperatorAttention {
	age, ok := workCommissionLeaseAge(commission, now)
	if !ok {
		return commissionOperatorAttention{}
	}
	if age < leaseAgeCap {
		return commissionOperatorAttention{}
	}

	return commissionOperatorAttention{
		Code:   "lease_too_old",
		Reason: "lease older than " + leaseAgeCap.String(),
	}
}

func workCommissionLeaseAge(commission map[string]any, now time.Time) (time.Duration, bool) {
	lease, ok := mapArg(commission, "lease")
	if !ok {
		return 0, false
	}

	claimedAt, ok := parseRFC3339Field(lease, "claimed_at")
	if !ok {
		return 0, false
	}

	return now.Sub(claimedAt), true
}

func commissionAttentionDuration(args map[string]any) (time.Duration, error) {
	value := stringArg(args, "older_than")
	if value == "" {
		return defaultCommissionOpenAttentionAfter, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("older_than must be a Go duration like 24h: %w", err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("older_than must be positive")
	}
	return duration, nil
}

func commissionLeaseAgeCap(args map[string]any) (time.Duration, error) {
	value := firstStringField("lease_age_cap", args)
	if value == "" {
		value = firstStringField("stale_lease_age_cap", args)
	}
	if value == "" {
		value = firstStringField("max_lease_age", args)
	}
	if value == "" {
		return defaultCommissionLeaseAgeCap, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("lease_age_cap must be a Go duration like 24h: %w", err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("lease_age_cap must be positive")
	}
	return duration, nil
}

func workCommissionExpired(commission map[string]any, now time.Time) bool {
	validUntil, ok := parseRFC3339Field(commission, "valid_until")
	return ok && !validUntil.After(now)
}

func parseRFC3339Field(payload map[string]any, key string) (time.Time, bool) {
	value := stringField(payload, key)
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func workCommissionTerminal(commission map[string]any) bool {
	state := stringField(commission, "state")
	return workcommission.IsTerminalState(state)
}

func ensureWorkCommissionTransition(
	commission map[string]any,
	args map[string]any,
	transition workCommissionTransition,
	now time.Time,
) error {
	state := stringField(commission, "state")
	if !workCommissionStateAllowed(transition.AllowedStates, state) {
		return fmt.Errorf(
			"%s: state=%s allowed_states=%s",
			transition.ErrorCode,
			state,
			strings.Join(transition.AllowedStates, ","),
		)
	}
	if transition.RejectsExpired && workCommissionExpired(commission, now) {
		return fmt.Errorf(
			"%s: state=%s valid_until expired; cancel or create a fresh WorkCommission",
			transition.ErrorCode,
			state,
		)
	}
	if transition.RequiresReason && stringArg(args, "reason") == "" {
		return fmt.Errorf("%s: reason is required", transition.ErrorCode)
	}
	return nil
}

func ensureWorkCommissionLifecycleTransition(commission map[string]any, args map[string]any) error {
	action := stringArg(args, "action")
	state := stringField(commission, "state")
	allowedStates := workCommissionLifecycleAllowedStatesByAction[action]

	if workCommissionStateAllowed(allowedStates, state) {
		return nil
	}

	return fmt.Errorf(
		"commission_lifecycle_forbidden: action=%s state=%s allowed_states=%s",
		action,
		state,
		strings.Join(allowedStates, ","),
	)
}

func workCommissionFreshnessReport(
	ctx context.Context,
	store *artifact.Store,
	commission map[string]any,
	args map[string]any,
) (commissionFreshnessReport, error) {
	if stringArg(args, "action") != "start_after_preflight" {
		return commissionFreshnessReport{}, nil
	}

	report := commissionFreshnessReport{}
	decisionIssues, err := decisionFreshnessIssues(ctx, store, commission)
	if err != nil {
		return report, err
	}
	problemIssues, err := problemFreshnessIssues(ctx, store, commission)
	if err != nil {
		return report, err
	}

	report.Issues = append(report.Issues, decisionIssues...)
	report.Issues = append(report.Issues, problemIssues...)
	report.Issues = append(report.Issues, scopeFreshnessIssues(commission, args)...)
	report.Issues = append(report.Issues, specFreshnessIssues(commission, args)...)
	report.Issues = append(report.Issues, autonomyEnvelopeFreshnessIssues(commission, args)...)
	report.Gaps = append(report.Gaps, specFreshnessGaps(commission, args)...)
	report.Gaps = append(report.Gaps, deferredFreshnessGaps(commission)...)
	return report, nil
}

func decisionFreshnessIssues(
	ctx context.Context,
	store *artifact.Store,
	commission map[string]any,
) ([]commissionFreshnessIssue, error) {
	decisionRef := stringField(commission, "decision_ref")
	expectedHash := stringField(commission, "decision_revision_hash")

	decision, err := store.Get(ctx, decisionRef)
	if err != nil {
		return []commissionFreshnessIssue{{
			Code:  "decision_missing",
			Field: "decision_ref",
			Ref:   decisionRef,
		}}, nil
	}
	if decision.Meta.Kind != artifact.KindDecisionRecord {
		return []commissionFreshnessIssue{{
			Code:   "decision_kind_changed",
			Field:  "decision_ref",
			Ref:    decisionRef,
			Actual: string(decision.Meta.Kind),
		}}, nil
	}
	if decision.Meta.Status != artifact.StatusActive {
		return []commissionFreshnessIssue{{
			Code:   "decision_not_active",
			Field:  "decision.status",
			Ref:    decisionRef,
			Actual: string(decision.Meta.Status),
		}}, nil
	}

	currentHash, err := decisionRevisionHash(ctx, store, decision)
	if err != nil {
		return nil, err
	}

	return hashFreshnessIssues(
		"decision_revision_hash_changed",
		"decision_revision_hash",
		decisionRef,
		expectedHash,
		currentHash,
	), nil
}

func problemFreshnessIssues(
	ctx context.Context,
	store *artifact.Store,
	commission map[string]any,
) ([]commissionFreshnessIssue, error) {
	problemRef := stringField(commission, "problem_card_ref")
	expectedHash := stringField(commission, "problem_revision_hash")
	if expectedHash == "" {
		return []commissionFreshnessIssue{{
			Code:  "problem_revision_hash_missing",
			Field: "problem_revision_hash",
			Ref:   problemRef,
		}}, nil
	}

	problem, err := store.Get(ctx, problemRef)
	if err != nil {
		return []commissionFreshnessIssue{{
			Code:  "problem_missing",
			Field: "problem_card_ref",
			Ref:   problemRef,
		}}, nil
	}
	if problem.Meta.Kind != artifact.KindProblemCard {
		return []commissionFreshnessIssue{{
			Code:   "problem_kind_changed",
			Field:  "problem_card_ref",
			Ref:    problemRef,
			Actual: string(problem.Meta.Kind),
		}}, nil
	}
	if problemBlockedStatus(problem.Meta.Status) {
		return []commissionFreshnessIssue{{
			Code:   "problem_not_fresh",
			Field:  "problem.status",
			Ref:    problemRef,
			Actual: string(problem.Meta.Status),
		}}, nil
	}

	currentHash, err := artifactRevisionHash(problem, nil)
	if err != nil {
		return nil, err
	}

	return hashFreshnessIssues(
		"problem_revision_hash_changed",
		"problem_revision_hash",
		problemRef,
		expectedHash,
		currentHash,
	), nil
}

func problemBlockedStatus(status artifact.Status) bool {
	switch status {
	case artifact.StatusSuperseded, artifact.StatusDeprecated, artifact.StatusRefreshDue:
		return true
	default:
		return false
	}
}

func scopeFreshnessIssues(
	commission map[string]any,
	args map[string]any,
) []commissionFreshnessIssue {
	scope, ok := mapArg(commission, "scope")
	if !ok {
		return []commissionFreshnessIssue{{
			Code:  "scope_missing",
			Field: "scope",
		}}
	}

	scopeHash, err := workCommissionScopeHash(scope)
	if err != nil {
		return []commissionFreshnessIssue{{
			Code:   "scope_invalid",
			Field:  "scope",
			Actual: err.Error(),
		}}
	}

	issues := make([]commissionFreshnessIssue, 0)
	issues = append(issues, hashFreshnessIssues(
		"scope_hash_changed",
		"scope_hash",
		stringField(commission, "id"),
		stringField(commission, "scope_hash"),
		scopeHash,
	)...)
	issues = append(issues, hashFreshnessIssues(
		"scope_embedded_hash_changed",
		"scope.hash",
		stringField(commission, "id"),
		stringField(scope, "hash"),
		scopeHash,
	)...)
	issues = append(issues, hashFreshnessIssues(
		"base_sha_changed",
		"base_sha",
		stringField(commission, "id"),
		stringField(commission, "base_sha"),
		stringField(scope, "base_sha"),
	)...)
	issues = append(issues, admittedBaseFreshnessIssues(commission, args)...)
	return issues
}

func admittedBaseFreshnessIssues(
	commission map[string]any,
	args map[string]any,
) []commissionFreshnessIssue {
	baseSHA := startAdmittedBaseSHA(args)
	if baseSHA == "" {
		return nil
	}

	return hashFreshnessIssues(
		"admitted_base_sha_changed",
		"base_sha",
		stringField(commission, "id"),
		stringField(commission, "base_sha"),
		baseSHA,
	)
}

func startAdmittedBaseSHA(args map[string]any) string {
	if baseSHA := stringArg(args, "base_sha"); baseSHA != "" {
		return baseSHA
	}

	payload, ok := mapArg(args, "payload")
	if !ok {
		return ""
	}
	if baseSHA := stringField(payload, "base_sha"); baseSHA != "" {
		return baseSHA
	}
	return stringField(payload, "current_base_sha")
}

func specFreshnessIssues(
	commission map[string]any,
	args map[string]any,
) []commissionFreshnessIssue {
	refs := workCommissionSpecSectionRefs(commission)
	if len(refs) == 0 {
		return nil
	}

	if projectRoot := stringArg(args, "project_root"); projectRoot != "" {
		return resolvedSpecFreshnessIssues(commission, refs, projectRoot)
	}

	return nil
}

func specFreshnessGaps(
	commission map[string]any,
	args map[string]any,
) []commissionFreshnessGap {
	if len(workCommissionSpecSectionRefs(commission)) == 0 {
		return nil
	}
	if stringArg(args, "project_root") != "" {
		return nil
	}

	return []commissionFreshnessGap{{
		Code:   "spec_revision_gate_deferred",
		Field:  "spec_revision_hashes",
		Reason: "ProjectSpecificationSet root was not supplied to the lifecycle call",
	}}
}

func autonomyEnvelopeFreshnessIssues(
	commission map[string]any,
	args map[string]any,
) []commissionFreshnessIssue {
	expectedRevision := stringField(commission, "autonomy_envelope_revision")
	if expectedRevision == "" {
		return nil
	}

	currentRevision := startAutonomyEnvelopeRevision(args)
	if currentRevision == "" {
		return nil
	}

	return hashFreshnessIssues(
		"autonomy_envelope_revision_changed",
		"autonomy_envelope_revision",
		stringField(commission, "autonomy_envelope_ref"),
		expectedRevision,
		currentRevision,
	)
}

func startAutonomyEnvelopeRevision(args map[string]any) string {
	if revision := stringArg(args, "autonomy_envelope_revision"); revision != "" {
		return revision
	}

	payload, ok := mapArg(args, "payload")
	if ok {
		if revision := stringField(payload, "autonomy_envelope_revision"); revision != "" {
			return revision
		}
	}

	for _, key := range []string{"autonomy_envelope_snapshot", "autonomy_envelope"} {
		snapshot, ok := mapArg(args, key)
		if !ok {
			continue
		}
		if revision := stringField(snapshot, "revision"); revision != "" {
			return revision
		}
	}

	return ""
}

func resolvedSpecFreshnessIssues(
	commission map[string]any,
	refs []string,
	projectRoot string,
) []commissionFreshnessIssue {
	expectedHashes := stringMapField(commission, "spec_revision_hashes")
	_, currentHashes, _, err := commissionSpecSnapshot(
		map[string]any{"project_root": projectRoot},
		refs,
		time.Now().UTC(),
	)
	if err != nil {
		return []commissionFreshnessIssue{{
			Code:   "spec_revision_unavailable",
			Field:  "spec_revision_hashes",
			Actual: err.Error(),
		}}
	}

	issues := make([]commissionFreshnessIssue, 0)
	for _, ref := range refs {
		issues = append(issues, hashFreshnessIssues(
			"spec_revision_hash_changed",
			"spec_revision_hashes."+ref,
			ref,
			expectedHashes[ref],
			stringField(currentHashes, ref),
		)...)
	}
	return issues
}

func deferredFreshnessGaps(commission map[string]any) []commissionFreshnessGap {
	gaps := make([]commissionFreshnessGap, 0)
	if stringField(commission, "implementation_plan_revision") != "" {
		gaps = append(gaps, commissionFreshnessGap{
			Code:   "implementation_plan_gate_deferred",
			Field:  "implementation_plan_revision",
			Ref:    stringField(commission, "implementation_plan_ref"),
			Reason: "ImplementationPlan is not a persisted artifact kind yet",
		})
	}
	if stringField(commission, "autonomy_envelope_revision") != "" && !workCommissionHasAutonomyEnvelopeSnapshot(commission) {
		gaps = append(gaps, commissionFreshnessGap{
			Code:   "autonomy_envelope_gate_deferred",
			Field:  "autonomy_envelope_revision",
			Ref:    stringField(commission, "autonomy_envelope_ref"),
			Reason: "AutonomyEnvelope is not a persisted artifact kind yet",
		})
	}
	return gaps
}

func workCommissionHasAutonomyEnvelopeSnapshot(commission map[string]any) bool {
	_, ok := mapArg(commission, "autonomy_envelope_snapshot")
	return ok
}

func hashFreshnessIssues(
	code string,
	field string,
	ref string,
	expected string,
	actual string,
) []commissionFreshnessIssue {
	if expected != "" && expected == actual {
		return nil
	}

	return []commissionFreshnessIssue{{
		Code:     code,
		Field:    field,
		Ref:      ref,
		Expected: expected,
		Actual:   actual,
	}}
}

func commissionFreshnessBlocked(report commissionFreshnessReport) bool {
	return len(report.Issues) > 0
}

func commissionFreshnessBlockError(report commissionFreshnessReport) error {
	return fmt.Errorf("commission_freshness_blocked: %s", strings.Join(commissionFreshnessIssueCodes(report), ","))
}

func commissionFreshnessIssueCodes(report commissionFreshnessReport) []string {
	codes := make([]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		codes = append(codes, issue.Code)
	}
	return sortedUniqueStrings(codes)
}

func appendFreshnessBlockLifecycleEvent(
	commission map[string]any,
	args map[string]any,
	report commissionFreshnessReport,
) map[string]any {
	eventArgs := workCommissionArgsWithFreshnessGaps(args, report)
	eventArgs["event"] = "freshness_blocked"
	eventArgs["verdict"] = "blocked"
	eventArgs["reason"] = commissionFreshnessBlockError(report).Error()

	payload, _ := mapArg(eventArgs, "payload")
	if payload == nil {
		payload = map[string]any{}
	}
	payload["freshness_mismatches"] = commissionFreshnessIssueMaps(report.Issues)
	eventArgs["payload"] = payload

	return appendLifecycleEvent(commission, eventArgs)
}

func workCommissionArgsWithFreshnessGaps(
	args map[string]any,
	report commissionFreshnessReport,
) map[string]any {
	if len(report.Gaps) == 0 {
		return args
	}

	next := copyStringAnyMap(args)
	payload, _ := mapArg(next, "payload")
	if payload == nil {
		payload = map[string]any{}
	} else {
		payload = copyStringAnyMap(payload)
	}
	payload["freshness_gaps"] = commissionFreshnessGapMaps(report.Gaps)
	next["payload"] = payload
	return next
}

func commissionFreshnessIssueMaps(issues []commissionFreshnessIssue) []any {
	result := make([]any, 0, len(issues))
	for _, issue := range issues {
		result = append(result, commissionFreshnessIssueMap(issue))
	}
	return result
}

func commissionFreshnessIssueMap(issue commissionFreshnessIssue) map[string]any {
	payload := map[string]any{
		"code":  issue.Code,
		"field": issue.Field,
	}
	putOptionalString(payload, "ref", issue.Ref)
	putOptionalString(payload, "expected", issue.Expected)
	putOptionalString(payload, "actual", issue.Actual)
	return payload
}

func commissionFreshnessGapMaps(gaps []commissionFreshnessGap) []any {
	result := make([]any, 0, len(gaps))
	for _, gap := range gaps {
		result = append(result, commissionFreshnessGapMap(gap))
	}
	return result
}

func commissionFreshnessGapMap(gap commissionFreshnessGap) map[string]any {
	payload := map[string]any{
		"code":   gap.Code,
		"field":  gap.Field,
		"reason": gap.Reason,
	}
	putOptionalString(payload, "ref", gap.Ref)
	return payload
}

func workCommissionStateAllowed(allowed []string, state string) bool {
	for _, candidate := range allowed {
		if candidate == state {
			return true
		}
	}
	return false
}

func appendRequeueLifecycleEvent(commission map[string]any, args map[string]any) map[string]any {
	payload := map[string]any{
		"previous_state": stringField(commission, "state"),
	}
	if lease, ok := mapArg(commission, "lease"); ok {
		payload["previous_lease"] = lease
	}

	eventArgs := map[string]any{
		"action":    "requeue",
		"runner_id": stringArg(args, "runner_id"),
		"event":     "commission_requeued",
		"reason":    stringArg(args, "reason"),
		"payload":   payload,
	}

	return appendLifecycleEvent(commission, eventArgs)
}

func appendCancelLifecycleEvent(commission map[string]any, args map[string]any) map[string]any {
	payload := map[string]any{
		"previous_state": stringField(commission, "state"),
	}
	if lease, ok := mapArg(commission, "lease"); ok {
		payload["previous_lease"] = lease
	}

	eventArgs := map[string]any{
		"action":    "cancel",
		"runner_id": stringArg(args, "runner_id"),
		"event":     "commission_cancelled",
		"reason":    stringArg(args, "reason"),
		"payload":   payload,
	}

	return appendLifecycleEvent(commission, eventArgs)
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
	if runtimeRunID := workCommissionLifecycleRuntimeRunID(commission, args); runtimeRunID != "" {
		event["runtime_run_id"] = runtimeRunID
	}

	events, _ := commission["events"].([]any)
	commission["events"] = append(events, event)
	return commission
}

func workCommissionLifecycleRuntimeRunID(
	commission map[string]any,
	args map[string]any,
) string {
	if !workCommissionLifecycleActionCreatesRuntimeRun(stringArg(args, "action")) {
		return ""
	}

	payload, _ := mapArg(args, "payload")
	if id := firstStringField("runtime_run_id", args, payload); id != "" {
		return id
	}
	if id := firstStringField("run_id", args, payload); id != "" {
		return id
	}
	if id := firstStringField("carrier_ref", args, payload); id != "" {
		return id
	}

	return workCommissionLifecycleOrdinalRuntimeRunID(commission)
}

func workCommissionLifecycleActionCreatesRuntimeRun(action string) bool {
	runtimeActions := map[string]bool{
		"record_preflight":      true,
		"start_after_preflight": true,
		"record_run_event":      true,
		"complete_or_block":     true,
	}

	return runtimeActions[strings.TrimSpace(action)]
}

func workCommissionLifecycleOrdinalRuntimeRunID(commission map[string]any) string {
	return fmt.Sprintf(
		"%s#runtime-run-%03d",
		stringField(commission, "id"),
		workCommissionLifecycleRuntimeRunOrdinal(commission),
	)
}

func workCommissionLifecycleRuntimeRunOrdinal(commission map[string]any) int {
	ordinal := 1

	for _, event := range mapSliceField(commission, "events") {
		if !workCommissionLifecycleEventClosesRuntimeRun(event) {
			continue
		}

		ordinal++
	}

	return ordinal
}

func workCommissionLifecycleEventClosesRuntimeRun(event map[string]any) bool {
	if stringField(event, "action") == "complete_or_block" {
		return true
	}
	if stringField(event, "event") == "freshness_blocked" {
		return true
	}
	if stringField(event, "action") == "record_preflight" && stringField(event, "verdict") == "blocked" {
		return true
	}

	return false
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
		commission = completeWorkCommissionAfterLocalEvidence(commission, args)
		commission = withWorkCommissionDeliveryDecision(commission, args, time.Now().UTC())
	case action == "complete_or_block" && verdict == "failed":
		commission["state"] = "failed"
	case action == "complete_or_block" && verdict == "blocked":
		commission["state"] = "blocked_policy"
	}

	return commission
}

func completeWorkCommissionAfterLocalEvidence(
	commission map[string]any,
	args map[string]any,
) map[string]any {
	policy := stringField(commission, "projection_policy")
	projectionPolicy := workcommission.NormalizeProjectionPolicy(policy)
	publication := workCommissionProjectionPublication(args)
	completion := workcommission.CompletionAfterLocalEvidence(projectionPolicy, publication)

	commission["state"] = string(completion.State)
	commission["local_execution"] = workCommissionLocalExecution(commission)

	if completion.Debt == nil {
		delete(commission, "projection_debt")
		return commission
	}

	commission["projection_debt"] = projectionDebtPayload(*completion.Debt)
	return commission
}

func withWorkCommissionDeliveryDecision(
	commission map[string]any,
	args map[string]any,
	now time.Time,
) map[string]any {
	policy := workcommission.NormalizeDeliveryPolicy(stringField(commission, "delivery_policy"))
	verdict := workcommission.NormalizeDeliveryVerdict(stringArg(args, "verdict"))
	gate, findings := workCommissionDeliveryGate(commission, now)
	decision := workcommission.DeliveryAfterLocalEvidence(policy, verdict, gate)
	payload := workCommissionDeliveryDecisionPayload(policy, verdict, gate, decision, findings, now)

	commission["delivery_decision"] = payload
	commission["auto_apply"] = workCommissionAutoApplyPayload(payload)
	return commission
}

func workCommissionDeliveryGate(
	commission map[string]any,
	now time.Time,
) (workcommission.DeliveryGate, []any) {
	report, ok, err := workCommissionAutonomyEnvelopeReport(commission, now)
	if err != nil {
		return workcommission.DeliveryGateBlocked, []any{
			map[string]any{
				"code":   "autonomy_envelope_invalid",
				"field":  "autonomy_envelope_snapshot",
				"detail": err.Error(),
			},
		}
	}
	if !ok {
		return workcommission.DeliveryGateMissing, nil
	}
	if report.Decision == autonomyenvelope.DecisionAllowed {
		return workcommission.DeliveryGateAllowed, nil
	}

	return workcommission.DeliveryGateBlocked, autonomyEnvelopeFindingMaps(report.Findings)
}

func workCommissionDeliveryDecisionPayload(
	policy workcommission.DeliveryPolicy,
	verdict workcommission.DeliveryVerdict,
	gate workcommission.DeliveryGate,
	decision workcommission.DeliveryDecision,
	findings []any,
	now time.Time,
) map[string]any {
	payload := map[string]any{
		"policy":                       string(policy),
		"verdict":                      string(verdict),
		"action":                       string(decision.Action),
		"mode":                         string(decision.Action),
		"auto_apply":                   decision.AutoApply,
		"reason":                       decision.Reason,
		"autonomy_envelope_decision":   string(gate),
		"workspace_commit_granularity": "per_commission",
		"recorded_at":                  now.Format(time.RFC3339),
	}
	if len(findings) > 0 {
		payload["autonomy_envelope_findings"] = findings
	}
	return payload
}

func workCommissionAutoApplyPayload(delivery map[string]any) map[string]any {
	return map[string]any{
		"allowed":                      delivery["auto_apply"],
		"decision":                     delivery["action"],
		"reason":                       delivery["reason"],
		"delivery_policy":              delivery["policy"],
		"verdict":                      delivery["verdict"],
		"autonomy_envelope_decision":   delivery["autonomy_envelope_decision"],
		"workspace_commit_granularity": delivery["workspace_commit_granularity"],
	}
}

func workCommissionProjectionPublication(args map[string]any) workcommission.ProjectionPublication {
	payload, _ := mapArg(args, "payload")
	publication, _ := mapArg(payload, "external_publication")

	state := firstStringField("state", publication)
	if state == "" {
		state = firstStringField("external_publication_state", args, payload)
	}

	return workcommission.ProjectionPublication{
		State:       workcommission.NormalizeProjectionPublicationState(state),
		Carrier:     firstStringField("carrier", publication),
		Target:      firstStringField("target", publication),
		LastError:   firstStringField("last_error", publication),
		RetryPolicy: firstStringField("retry_policy", publication),
	}
}

func workCommissionLocalExecution(commission map[string]any) map[string]any {
	event := lastWorkCommissionLifecycleEvent(commission)

	return map[string]any{
		"state":          "evidence_passed",
		"verdict":        "pass",
		"runtime_run_id": stringField(event, "runtime_run_id"),
		"recorded_at":    stringField(event, "recorded_at"),
	}
}

func lastWorkCommissionLifecycleEvent(commission map[string]any) map[string]any {
	events := mapSliceField(commission, "events")
	if len(events) == 0 {
		return map[string]any{}
	}

	return events[len(events)-1]
}

func projectionDebtPayload(debt workcommission.ProjectionDebt) map[string]any {
	return map[string]any{
		"carrier":      debt.Carrier,
		"target":       debt.Target,
		"last_error":   debt.LastError,
		"retry_policy": debt.RetryPolicy,
	}
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
		"- Delivery policy: " + stringField(commission, "delivery_policy"),
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

func stringMapField(payload map[string]any, key string) map[string]string {
	switch value := payload[key].(type) {
	case map[string]string:
		return cleanStringMap(value)
	case map[string]any:
		return anyMapToStringMap(value)
	default:
		return map[string]string{}
	}
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

func cleanStringMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		cleanKey := strings.TrimSpace(key)
		cleanValue := strings.TrimSpace(value)
		if cleanKey != "" && cleanValue != "" {
			result[cleanKey] = cleanValue
		}
	}
	return result
}

func anyMapToStringMap(values map[string]any) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}

		cleanKey := strings.TrimSpace(key)
		cleanValue := strings.TrimSpace(text)
		if cleanKey != "" && cleanValue != "" {
			result[cleanKey] = cleanValue
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
