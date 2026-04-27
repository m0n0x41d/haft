package autonomyenvelope

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/scopeauth"
)

type State string

const (
	StateDraft     State = "draft"
	StateApproved  State = "approved"
	StateActive    State = "active"
	StateExhausted State = "exhausted"
	StateRevoked   State = "revoked"
	StateExpired   State = "expired"
)

type FailureStrategy string

const (
	FailureBlockPlan           FailureStrategy = "block_plan"
	FailureBlockNode           FailureStrategy = "block_node"
	FailureContinueIndependent FailureStrategy = "continue_independent"
)

type Decision string

const (
	DecisionAllowed            Decision = "allowed"
	DecisionBlocked            Decision = "blocked"
	DecisionCheckpointRequired Decision = "checkpoint_required"
)

type Finding struct {
	Code   string
	Field  string
	Detail string
}

type Report struct {
	Decision Decision
	Findings []Finding
}

type Snapshot struct {
	Ref                        string
	Revision                   string
	State                      State
	AllowedRepos               []string
	AllowedPaths               []string
	ForbiddenPaths             []string
	AllowedActions             []string
	AllowedModules             []string
	ForbiddenActions           []string
	ForbiddenOneWayDoorActions []string
	MaxConcurrency             int
	CommissionBudget           int
	ActiveConcurrency          int
	ConsumedCommissions        int
	OnFailure                  FailureStrategy
	OnStale                    FailureStrategy
	ValidUntil                 time.Time
	RevokedAt                  *time.Time
	RequiredGates              []string
	Hash                       string
}

type CommissionRequest struct {
	RepoRef             string
	AllowedPaths        []string
	ForbiddenPaths      []string
	AllowedActions      []string
	AllowedModules      []string
	ActiveConcurrency   int
	ConsumedCommissions int
}

var RequiredGates = []string{
	"freshness",
	"scope",
	"evidence",
}

func NoEnvelopeReport() Report {
	return Report{
		Decision: DecisionCheckpointRequired,
		Findings: []Finding{{
			Code:   "autonomy_envelope_missing",
			Field:  "autonomy_envelope_snapshot",
			Detail: "autonomous continuation requires an explicit human-approved envelope",
		}},
	}
}

func NormalizeSnapshot(payload map[string]any) (map[string]any, Snapshot, error) {
	snapshot, err := SnapshotFromMap(payload)
	if err != nil {
		return nil, Snapshot{}, err
	}

	normalized := snapshot.Map()
	return normalized, snapshot, nil
}

func SnapshotFromMap(payload map[string]any) (Snapshot, error) {
	snapshot := Snapshot{
		Ref:                        firstString(payload, "ref", "id"),
		Revision:                   requiredString(payload, "revision"),
		State:                      State(requiredString(payload, "state")),
		AllowedRepos:               requiredStringSlice(payload, "allowed_repos"),
		AllowedPaths:               requiredStringSlice(payload, "allowed_paths"),
		ForbiddenPaths:             optionalStringSlice(payload, "forbidden_paths"),
		AllowedActions:             requiredStringSlice(payload, "allowed_actions"),
		AllowedModules:             optionalStringSlice(payload, "allowed_modules"),
		ForbiddenActions:           optionalStringSlice(payload, "forbidden_actions"),
		ForbiddenOneWayDoorActions: firstStringSlice(payload, "forbidden_one_way_door_actions", "one_way_door_exclusions"),
		MaxConcurrency:             requiredPositiveInt(payload, "max_concurrency"),
		CommissionBudget:           firstPositiveInt(payload, "commission_budget", "max_commissions"),
		ActiveConcurrency:          optionalNonNegativeInt(payload, "active_concurrency"),
		ConsumedCommissions:        optionalNonNegativeInt(payload, "consumed_commissions"),
		OnFailure:                  FailureStrategy(requiredString(payload, "on_failure")),
		OnStale:                    FailureStrategy(optionalString(payload, "on_stale")),
		RequiredGates:              firstStringSlice(payload, "required_gates"),
	}

	if len(firstStringSlice(payload, "skip_gates", "disabled_gates")) > 0 {
		return Snapshot{}, fmt.Errorf("autonomy_envelope_gate_skip_forbidden")
	}

	validUntil, err := requiredTime(payload, "valid_until", "expires_at")
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.ValidUntil = validUntil

	revokedAt, err := optionalTime(payload, "revoked_at")
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.RevokedAt = revokedAt

	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}

	hash, err := snapshot.CanonicalHash()
	if err != nil {
		return Snapshot{}, err
	}
	if suppliedHash := optionalString(payload, "hash"); suppliedHash != "" && suppliedHash != hash {
		return Snapshot{}, fmt.Errorf("autonomy_envelope_hash_mismatch")
	}

	snapshot.Hash = hash
	return snapshot, nil
}

func (snapshot Snapshot) Validate() error {
	validations := []func(Snapshot) error{
		validateIdentity,
		validateState,
		validateBounds,
		validateFailureStrategies,
		validateGatePolicy,
	}

	for _, validate := range validations {
		if err := validate(snapshot); err != nil {
			return err
		}
	}

	return nil
}

func (snapshot Snapshot) Evaluate(request CommissionRequest, now time.Time) Report {
	findings := make([]Finding, 0)
	findings = append(findings, lifecycleFindings(snapshot, now)...)
	findings = append(findings, repoFindings(snapshot, request)...)
	findings = append(findings, pathFindings(snapshot, request)...)
	findings = append(findings, actionFindings(snapshot, request)...)
	findings = append(findings, moduleFindings(snapshot, request)...)
	findings = append(findings, budgetFindings(snapshot, request)...)

	if len(findings) > 0 {
		return Report{
			Decision: DecisionBlocked,
			Findings: findings,
		}
	}

	return Report{
		Decision: DecisionAllowed,
		Findings: []Finding{},
	}
}

func (snapshot Snapshot) Map() map[string]any {
	payload := map[string]any{
		"ref":                            snapshot.Ref,
		"revision":                       snapshot.Revision,
		"state":                          string(snapshot.State),
		"allowed_repos":                  stringSliceToAny(snapshot.AllowedRepos),
		"allowed_paths":                  stringSliceToAny(snapshot.AllowedPaths),
		"forbidden_paths":                stringSliceToAny(snapshot.ForbiddenPaths),
		"allowed_actions":                stringSliceToAny(snapshot.AllowedActions),
		"allowed_modules":                stringSliceToAny(snapshot.AllowedModules),
		"forbidden_actions":              stringSliceToAny(snapshot.ForbiddenActions),
		"forbidden_one_way_door_actions": stringSliceToAny(snapshot.ForbiddenOneWayDoorActions),
		"max_concurrency":                snapshot.MaxConcurrency,
		"commission_budget":              snapshot.CommissionBudget,
		"active_concurrency":             snapshot.ActiveConcurrency,
		"consumed_commissions":           snapshot.ConsumedCommissions,
		"on_failure":                     string(snapshot.OnFailure),
		"valid_until":                    snapshot.ValidUntil.Format(time.RFC3339),
		"required_gates":                 stringSliceToAny(snapshot.RequiredGates),
		"hash":                           snapshot.Hash,
	}

	if snapshot.OnStale != "" {
		payload["on_stale"] = string(snapshot.OnStale)
	}
	if snapshot.RevokedAt != nil {
		payload["revoked_at"] = snapshot.RevokedAt.Format(time.RFC3339)
	}

	return payload
}

func (snapshot Snapshot) CanonicalHash() (string, error) {
	payload := map[string]any{
		"active_concurrency":             snapshot.ActiveConcurrency,
		"allowed_actions":                sortedUniqueStrings(snapshot.AllowedActions),
		"allowed_modules":                sortedUniqueStrings(snapshot.AllowedModules),
		"allowed_paths":                  sortedUniqueStrings(snapshot.AllowedPaths),
		"allowed_repos":                  sortedUniqueStrings(snapshot.AllowedRepos),
		"commission_budget":              snapshot.CommissionBudget,
		"consumed_commissions":           snapshot.ConsumedCommissions,
		"forbidden_actions":              sortedUniqueStrings(snapshot.ForbiddenActions),
		"forbidden_one_way_door_actions": sortedUniqueStrings(snapshot.ForbiddenOneWayDoorActions),
		"forbidden_paths":                sortedUniqueStrings(snapshot.ForbiddenPaths),
		"max_concurrency":                snapshot.MaxConcurrency,
		"on_failure":                     string(snapshot.OnFailure),
		"on_stale":                       string(snapshot.OnStale),
		"ref":                            snapshot.Ref,
		"required_gates":                 sortedUniqueStrings(snapshot.RequiredGates),
		"revision":                       snapshot.Revision,
		"revoked_at":                     optionalTimeString(snapshot.RevokedAt),
		"state":                          string(snapshot.State),
		"valid_until":                    snapshot.ValidUntil.Format(time.RFC3339),
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validateIdentity(snapshot Snapshot) error {
	switch {
	case snapshot.Ref == "":
		return fmt.Errorf("autonomy_envelope_ref_required")
	case snapshot.Revision == "":
		return fmt.Errorf("autonomy_envelope_revision_required")
	default:
		return nil
	}
}

func validateState(snapshot Snapshot) error {
	switch snapshot.State {
	case StateDraft, StateApproved, StateActive, StateExhausted, StateRevoked, StateExpired:
		return nil
	default:
		return fmt.Errorf("invalid_autonomy_envelope_state: %s", snapshot.State)
	}
}

func validateBounds(snapshot Snapshot) error {
	switch {
	case len(snapshot.AllowedRepos) == 0:
		return fmt.Errorf("autonomy_envelope_allowed_repos_required")
	case len(snapshot.AllowedPaths) == 0:
		return fmt.Errorf("autonomy_envelope_allowed_paths_required")
	case len(snapshot.AllowedActions) == 0:
		return fmt.Errorf("autonomy_envelope_allowed_actions_required")
	case snapshot.MaxConcurrency <= 0:
		return fmt.Errorf("autonomy_envelope_max_concurrency_required")
	case snapshot.CommissionBudget <= 0:
		return fmt.Errorf("autonomy_envelope_commission_budget_required")
	case snapshot.ActiveConcurrency < 0:
		return fmt.Errorf("autonomy_envelope_active_concurrency_invalid")
	case snapshot.ConsumedCommissions < 0:
		return fmt.Errorf("autonomy_envelope_consumed_commissions_invalid")
	default:
		return nil
	}
}

func validateFailureStrategies(snapshot Snapshot) error {
	if !validFailureStrategy(snapshot.OnFailure) {
		return fmt.Errorf("invalid_autonomy_envelope_on_failure: %s", snapshot.OnFailure)
	}
	if snapshot.OnStale == "" {
		return nil
	}
	if !validFailureStrategy(snapshot.OnStale) {
		return fmt.Errorf("invalid_autonomy_envelope_on_stale: %s", snapshot.OnStale)
	}
	return nil
}

func validateGatePolicy(snapshot Snapshot) error {
	skipGates := snapshot.RequiredGates
	if len(skipGates) == 0 {
		return nil
	}

	required := stringSet(RequiredGates)
	actual := stringSet(skipGates)
	for gate := range required {
		if !actual[gate] {
			return fmt.Errorf("autonomy_envelope_required_gate_missing: %s", gate)
		}
	}

	return nil
}

func validFailureStrategy(strategy FailureStrategy) bool {
	switch strategy {
	case FailureBlockPlan, FailureBlockNode, FailureContinueIndependent:
		return true
	default:
		return false
	}
}

func lifecycleFindings(snapshot Snapshot, now time.Time) []Finding {
	findings := make([]Finding, 0)
	findings = append(findings, stateFindings(snapshot)...)
	findings = append(findings, expiryFindings(snapshot, now)...)
	findings = append(findings, revocationFindings(snapshot, now)...)
	return findings
}

func stateFindings(snapshot Snapshot) []Finding {
	switch snapshot.State {
	case StateApproved, StateActive:
		return nil
	case StateRevoked:
		return []Finding{{Code: "autonomy_envelope_revoked", Field: "state", Detail: string(snapshot.State)}}
	case StateExpired:
		return []Finding{{Code: "autonomy_envelope_expired", Field: "state", Detail: string(snapshot.State)}}
	case StateExhausted:
		return []Finding{{Code: "autonomy_envelope_exhausted", Field: "state", Detail: string(snapshot.State)}}
	default:
		return []Finding{{Code: "autonomy_envelope_not_active", Field: "state", Detail: string(snapshot.State)}}
	}
}

func expiryFindings(snapshot Snapshot, now time.Time) []Finding {
	if snapshot.ValidUntil.After(now) {
		return nil
	}

	return []Finding{{
		Code:   "autonomy_envelope_expired",
		Field:  "valid_until",
		Detail: snapshot.ValidUntil.Format(time.RFC3339),
	}}
}

func revocationFindings(snapshot Snapshot, now time.Time) []Finding {
	if snapshot.RevokedAt == nil {
		return nil
	}
	if snapshot.RevokedAt.After(now) {
		return nil
	}

	return []Finding{{
		Code:   "autonomy_envelope_revoked",
		Field:  "revoked_at",
		Detail: snapshot.RevokedAt.Format(time.RFC3339),
	}}
}

func repoFindings(snapshot Snapshot, request CommissionRequest) []Finding {
	if stringAllowed(snapshot.AllowedRepos, request.RepoRef) {
		return nil
	}

	return []Finding{{
		Code:   "repo_outside_autonomy_envelope",
		Field:  "scope.repo_ref",
		Detail: request.RepoRef,
	}}
}

func pathFindings(snapshot Snapshot, request CommissionRequest) []Finding {
	authorization := scopeauth.AuthorizeWorkspaceDiff(
		scopeauth.CommissionScope{
			AllowedPaths:   snapshot.AllowedPaths,
			ForbiddenPaths: snapshot.ForbiddenPaths,
		},
		request.AllowedPaths,
		scopeauth.PathFacts{},
	)
	block := authorization.BlockingReason()
	if block.Code == "" {
		return nil
	}

	return []Finding{{
		Code:   "path_outside_autonomy_envelope",
		Field:  "scope.allowed_paths",
		Detail: strings.Join(block.Paths, ","),
	}}
}

func actionFindings(snapshot Snapshot, request CommissionRequest) []Finding {
	findings := make([]Finding, 0)
	allowed := stringSet(snapshot.AllowedActions)
	forbidden := stringSet(append(snapshot.ForbiddenActions, snapshot.ForbiddenOneWayDoorActions...))
	oneWayDoors := stringSet(snapshot.ForbiddenOneWayDoorActions)

	for _, action := range sortedUniqueStrings(request.AllowedActions) {
		if forbidden[action] {
			findings = append(findings, forbiddenActionFinding(action, oneWayDoors[action]))
			continue
		}
		if !allowed[action] {
			findings = append(findings, Finding{
				Code:   "action_outside_autonomy_envelope",
				Field:  "scope.allowed_actions",
				Detail: action,
			})
		}
	}

	return findings
}

func forbiddenActionFinding(action string, oneWayDoor bool) Finding {
	if oneWayDoor {
		return Finding{
			Code:   "one_way_door_action_forbidden",
			Field:  "scope.allowed_actions",
			Detail: action,
		}
	}

	return Finding{
		Code:   "action_forbidden_by_autonomy_envelope",
		Field:  "scope.allowed_actions",
		Detail: action,
	}
}

func moduleFindings(snapshot Snapshot, request CommissionRequest) []Finding {
	if len(snapshot.AllowedModules) == 0 {
		return nil
	}

	allowed := stringSet(snapshot.AllowedModules)
	findings := make([]Finding, 0)
	for _, module := range sortedUniqueStrings(request.AllowedModules) {
		if allowed[module] {
			continue
		}
		findings = append(findings, Finding{
			Code:   "module_outside_autonomy_envelope",
			Field:  "scope.allowed_modules",
			Detail: module,
		})
	}
	return findings
}

func budgetFindings(snapshot Snapshot, request CommissionRequest) []Finding {
	findings := make([]Finding, 0)
	activeConcurrency := maxInt(snapshot.ActiveConcurrency, request.ActiveConcurrency)
	consumedCommissions := maxInt(snapshot.ConsumedCommissions, request.ConsumedCommissions)

	if activeConcurrency >= snapshot.MaxConcurrency {
		findings = append(findings, Finding{
			Code:   "autonomy_envelope_concurrency_exhausted",
			Field:  "max_concurrency",
			Detail: fmt.Sprintf("%d/%d", activeConcurrency, snapshot.MaxConcurrency),
		})
	}
	if consumedCommissions >= snapshot.CommissionBudget {
		findings = append(findings, Finding{
			Code:   "autonomy_envelope_commission_budget_exhausted",
			Field:  "commission_budget",
			Detail: fmt.Sprintf("%d/%d", consumedCommissions, snapshot.CommissionBudget),
		})
	}

	return findings
}

func requiredString(payload map[string]any, key string) string {
	value := optionalString(payload, key)
	if value == "" {
		return ""
	}
	return value
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value := optionalString(payload, key)
		if value != "" {
			return value
		}
	}
	return ""
}

func optionalString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}

	text, ok := value.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(text)
}

func requiredStringSlice(payload map[string]any, key string) []string {
	return optionalStringSlice(payload, key)
}

func firstStringSlice(payload map[string]any, keys ...string) []string {
	for _, key := range keys {
		values := optionalStringSlice(payload, key)
		if len(values) > 0 {
			return values
		}
	}
	return nil
}

func optionalStringSlice(payload map[string]any, key string) []string {
	value, ok := payload[key]
	if !ok {
		return nil
	}

	switch typed := value.(type) {
	case []string:
		return sortedUniqueStrings(typed)
	case []any:
		return stringSliceFromAny(typed)
	default:
		return nil
	}
}

func stringSliceFromAny(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		result = append(result, text)
	}
	return sortedUniqueStrings(result)
}

func requiredPositiveInt(payload map[string]any, key string) int {
	return positiveInt(payload[key])
}

func firstPositiveInt(payload map[string]any, keys ...string) int {
	for _, key := range keys {
		value := positiveInt(payload[key])
		if value > 0 {
			return value
		}
	}
	return 0
}

func optionalNonNegativeInt(payload map[string]any, key string) int {
	value := positiveInt(payload[key])
	if value > 0 {
		return value
	}
	return 0
}

func positiveInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0
		}
		return int(parsed)
	default:
		return 0
	}
}

func requiredTime(payload map[string]any, keys ...string) (time.Time, error) {
	for _, key := range keys {
		value := optionalString(payload, key)
		if value == "" {
			continue
		}
		return time.Parse(time.RFC3339, value)
	}
	return time.Time{}, fmt.Errorf("autonomy_envelope_valid_until_required")
}

func optionalTime(payload map[string]any, key string) (*time.Time, error) {
	value := optionalString(payload, key)
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func optionalTimeString(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}

func stringAllowed(allowed []string, value string) bool {
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	return false
}

func sortedUniqueStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" {
			continue
		}
		cleaned = append(cleaned, text)
	}

	sort.Strings(cleaned)
	result := make([]string, 0, len(cleaned))
	for _, value := range cleaned {
		if len(result) > 0 && result[len(result)-1] == value {
			continue
		}
		result = append(result, value)
	}
	return result
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func stringSliceToAny(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
