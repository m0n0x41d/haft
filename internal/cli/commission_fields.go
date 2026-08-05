package cli

import "time"

// boolField reads one strict boolean from a decoded commission payload.
func boolField(payload map[string]any, key string) bool {
	value, ok := payload[key].(bool)
	return ok && value
}

// mapSliceField reads only object elements from a decoded JSON array.
func mapSliceField(payload map[string]any, key string) []map[string]any {
	values, ok := payload[key].([]any)
	if !ok {
		return nil
	}

	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			result = append(result, item)
		}
	}
	return result
}

// withCommissionSpecReadinessOverride records the already-authorized tactical
// exception without coupling WorkCommission creation to any executor.
func withCommissionSpecReadinessOverride(
	commission map[string]any,
	override map[string]any,
	now time.Time,
) map[string]any {
	record := copyStringAnyMap(override)
	if stringField(record, "recorded_at") == "" {
		record["recorded_at"] = now.Format(time.RFC3339)
	}

	commission["spec_readiness_override"] = record
	return commission
}
