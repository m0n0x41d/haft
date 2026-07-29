package fpf

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestHaftEntitySchemaIsClosedIntentWithoutMemoryInternals(t *testing.T) {
	t.Parallel()

	tool := haftEntityTool()
	if tool.Name != "haft_entity" {
		t.Fatalf("tool name = %q", tool.Name)
	}
	schema, ok := tool.InputSchema.(map[string]interface{})
	if !ok ||
		schema["type"] != "object" ||
		schema["additionalProperties"] != false {
		t.Fatalf("haft_entity schema = %#v", tool.InputSchema)
	}
	required := schemaStringSet(t, schema["required"], "haft_entity.required")
	wantRequired := []string{
		"action",
		"aliases",
		"bounded_context_ref",
		"entity_id",
		"idempotency_key",
		"label",
		"persistence_reason",
		"request_provenance_ref",
	}
	if !slices.Equal(required, wantRequired) {
		t.Fatalf("required = %#v, want %#v", required, wantRequired)
	}
	properties := mustSchemaProperties(t, schema, "haft_entity")
	propertyNames := slices.Sorted(maps.Keys(properties))
	if !slices.Equal(propertyNames, wantRequired) {
		t.Fatalf(
			"properties = %#v, want exact task-level set %#v",
			propertyNames,
			wantRequired,
		)
	}
	if mustStringLiteral(t, properties["action"], "action") != "establish" {
		t.Fatalf("action schema = %#v", properties["action"])
	}
	reasons := mustStringEnum(
		t,
		properties["persistence_reason"],
		"persistence_reason",
	)
	if len(reasons) != 2 ||
		!reasons["explicit_operator_request"] ||
		!reasons["named_receiving_use"] {
		t.Fatalf("persistence reasons = %#v", reasons)
	}
	aliases := properties["aliases"].(map[string]interface{})
	if aliases["uniqueItems"] != true {
		t.Fatalf("aliases schema does not require unique canonical aliases: %#v", aliases)
	}
	aliasItems := aliases["items"].(map[string]interface{})
	if aliasItems["maxLength"] != typedmemorywire.MaximumIdentifierBytes {
		t.Fatalf("alias bounds = %#v", aliasItems)
	}
	for _, fragment := range []string{
		"strictly increasing bytewise order",
		"commit atomically",
		"conflict writes nothing",
	} {
		description, _ := aliases["description"].(string)
		if !strings.Contains(description, fragment) {
			t.Fatalf("aliases description missing %q: %q", fragment, description)
		}
	}
	for name, maximum := range map[string]int{
		"entity_id":              typedmemorywire.MaximumIdentifierBytes,
		"label":                  typedmemorywire.MaximumTextBytes,
		"bounded_context_ref":    typedmemorywire.MaximumIdentifierBytes,
		"request_provenance_ref": typedmemorywire.MaximumIdentifierBytes,
		"idempotency_key":        typedmemorywire.MaximumAdmissionIdempotencyKeyBytes,
	} {
		field := properties[name].(map[string]interface{})
		description, _ := field["description"].(string)
		if field["maxLength"] != maximum ||
			!strings.Contains(description, "without surrounding whitespace") ||
			!strings.Contains(description, "UTF-8 bytes") {
			t.Fatalf(
				"%s schema does not match strict decoder: %#v",
				name,
				field,
			)
		}
	}
	aliasDescription, _ := aliasItems["description"].(string)
	if !strings.Contains(aliasDescription, "single-line text") ||
		!strings.Contains(aliasDescription, "UTF-8 bytes") {
		t.Fatalf(
			"alias schema does not match strict decoder: %#v",
			aliasItems,
		)
	}
	idempotency := properties["idempotency_key"].(map[string]interface{})
	idempotencyDescription, _ := idempotency["description"].(string)
	for _, fragment := range []string{"unknown commit outcome", "unchanged request", "same key"} {
		if !strings.Contains(idempotencyDescription, fragment) {
			t.Fatalf(
				"idempotency description missing %q: %q",
				fragment,
				idempotencyDescription,
			)
		}
	}
	schemaDescription, _ := schema["description"].(string)
	for _, fragment := range []string{
		"onboarding_required",
		"enablement_choice_required",
		"restart_required",
		"established",
		"identity_conflict",
		"alias_conflict",
		"idempotency_conflict",
		"rejected",
		"commit_outcome_unknown",
		"U.EntityRef",
		"persistence.performed",
		"persistence.authority_granted",
		"never grants decision",
	} {
		if !strings.Contains(schemaDescription, fragment) {
			t.Fatalf(
				"entity schema recovery description missing %q: %q",
				fragment,
				schemaDescription,
			)
		}
	}

	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(encoded))
	for _, forbidden := range []string{
		"type_env",
		"graph_revision",
		"change_set",
		"authority_class",
		"basis",
		"local_ref",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("haft_entity schema leaked %q: %s", forbidden, encoded)
		}
	}
	description := strings.ToLower(tool.Description)
	for _, forbidden := range []string{
		"typeenv",
		"memorychangeset",
		"graphrevision",
	} {
		if strings.Contains(description, forbidden) {
			t.Fatalf("haft_entity description leaked %q: %s", forbidden, tool.Description)
		}
	}
}

func TestHaftEntityCatalogPreservesTaskLevelPreconditionsAndRecovery(
	t *testing.T,
) {
	schema := mustListToolInputSchema(t, "haft_entity")
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"strictly increasing bytewise order",
		"commit_outcome_unknown",
		"unchanged request and the same key",
		"persistence.performed",
		"persistence.authority_granted",
		"UTF-8 bytes",
	} {
		if !strings.Contains(string(encoded), fragment) {
			t.Fatalf(
				"catalog entity schema lost %q:\n%s",
				fragment,
				encoded,
			)
		}
	}
}
