package cli

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func handleHaftCommission(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	action := stringArg(args, "action")

	switch action {
	case "create":
		return createWorkCommission(ctx, store, args)
	case "list_runnable":
		return listRunnableWorkCommissions(ctx, store)
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

func listRunnableWorkCommissions(ctx context.Context, store *artifact.Store) (string, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	commissions := make([]map[string]any, 0, len(records))
	for _, commission := range records {
		if workCommissionRunnable(commission, now) {
			commissions = append(commissions, commission)
		}
	}

	return commissionResponse("commissions", commissions)
}

func claimWorkCommissionForPreflight(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	commissionID := stringArg(args, "commission_id")
	if commissionID == "" {
		return "", fmt.Errorf("commission_id is required")
	}

	runnerID := stringArg(args, "runner_id")
	if runnerID == "" {
		runnerID = "haft"
	}

	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin WorkCommission claim: %w", err)
	}
	defer tx.Rollback()

	commission, err := loadWorkCommissionPayloadForUpdate(ctx, tx, commissionID)
	if err != nil {
		return "", err
	}
	if !workCommissionRunnable(commission, time.Now().UTC()) {
		return "", fmt.Errorf("commission_not_runnable")
	}
	if err := ensureWorkCommissionLocksetAvailable(ctx, tx, commission); err != nil {
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

func ensureWorkCommissionLocksetAvailable(
	ctx context.Context,
	tx *sql.Tx,
	target map[string]any,
) error {
	commissions, err := loadWorkCommissionPayloadsForClaim(ctx, tx)
	if err != nil {
		return err
	}

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
