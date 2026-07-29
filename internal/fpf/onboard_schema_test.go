package fpf

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/onboarding"
)

func TestHaftOnboardSchemaIsClosedReadableActionUnion(t *testing.T) {
	t.Parallel()

	tool := haftOnboardTool()
	if tool.Name != "haft_onboard" {
		t.Fatalf("tool name = %q", tool.Name)
	}
	schema, ok := tool.InputSchema.(map[string]interface{})
	if !ok ||
		schema["type"] != "object" ||
		schema["additionalProperties"] != false {
		t.Fatalf("haft_onboard schema = %#v", tool.InputSchema)
	}
	for _, forbidden := range []string{"allOf", "oneOf", "anyOf"} {
		if _, present := schema[forbidden]; present {
			t.Fatalf("haft_onboard declares unsupported top-level %s", forbidden)
		}
	}
	required := schemaStringSet(t, schema["required"], "haft_onboard.required")
	if !slices.Equal(required, []string{"action"}) {
		t.Fatalf("haft_onboard required = %#v", required)
	}
	properties := mustSchemaProperties(t, schema, "haft_onboard")
	actions := mustStringEnum(t, properties["action"], "haft_onboard.action")
	for _, action := range []string{
		haftOnboardStatusAction,
		haftOnboardProfilePrepareAction,
		haftOnboardMemoryPrepareAction,
	} {
		if !actions[action] {
			t.Fatalf("haft_onboard action %q is missing", action)
		}
	}
	if len(actions) != 3 {
		t.Fatalf("haft_onboard actions = %#v", actions)
	}
}

func TestHaftOnboardProfilePrepareAcceptsOnlyReadableScopes(t *testing.T) {
	schema := mustListToolInputSchema(t, "haft_onboard")
	properties := mustSchemaProperties(t, schema, "haft_onboard")
	scopes, ok := properties["scopes"].(map[string]interface{})
	if !ok || scopes["type"] != "array" {
		t.Fatalf("profile_prepare scopes = %#v", properties["scopes"])
	}
	if !schemaIntegerEquals(scopes["minItems"], 1) ||
		!schemaIntegerEquals(
			scopes["maxItems"],
			onboarding.MaximumProfileScopes,
		) ||
		scopes["uniqueItems"] != true {
		t.Fatalf(
			"profile_prepare scope bounds = %#v",
			scopes,
		)
	}
	scope, ok := scopes["items"].(map[string]interface{})
	if !ok ||
		scope["type"] != "object" ||
		scope["additionalProperties"] != false {
		t.Fatalf("profile_prepare scope item = %#v", scopes["items"])
	}
	scopeRequired := schemaStringSet(
		t,
		scope["required"],
		"profile_prepare.scope.required",
	)
	wantRequired := []string{
		"evidence_paths",
		"label",
		"realization_kind",
		"scope_id",
	}
	if !slices.Equal(scopeRequired, wantRequired) {
		t.Fatalf("scope required = %#v, want %#v", scopeRequired, wantRequired)
	}
	scopeProperties := mustSchemaProperties(t, scope, "profile_prepare.scope")
	kinds := mustStringEnum(
		t,
		scopeProperties["realization_kind"],
		"realization_kind",
	)
	if len(kinds) != 2 ||
		!kinds["software"] ||
		!kinds["non_software"] {
		t.Fatalf("realization kinds = %#v", kinds)
	}
	basis, ok := properties["basis"].(map[string]interface{})
	if !ok {
		t.Fatalf("profile_prepare basis = %#v", properties["basis"])
	}
	basisDescription, _ := basis["description"].(string)
	if !strings.Contains(
		basisDescription,
		"explicit profile_prepare scopes",
	) {
		t.Fatalf(
			"profile_prepare basis description = %q",
			basisDescription,
		)
	}
	if !schemaIntegerEquals(
		basis["maxLength"],
		onboarding.MaximumProfileBasisBytes,
	) {
		t.Fatalf("profile_prepare basis bounds = %#v", basis)
	}
	for name, maximum := range map[string]int{
		"scope_id": onboarding.MaximumScopeIDBytes,
		"label":    onboarding.MaximumScopeLabelBytes,
	} {
		field := scopeProperties[name].(map[string]interface{})
		if !schemaIntegerEquals(field["maxLength"], maximum) {
			t.Fatalf("%s bounds = %#v", name, field)
		}
	}
	evidencePaths := scopeProperties["evidence_paths"].(map[string]interface{})
	if !schemaIntegerEquals(
		evidencePaths["maxItems"],
		onboarding.MaximumEvidencePaths,
	) ||
		evidencePaths["uniqueItems"] != true {
		t.Fatalf("evidence_paths bounds = %#v", evidencePaths)
	}
	evidencePath := evidencePaths["items"].(map[string]interface{})
	if !schemaIntegerEquals(
		evidencePath["maxLength"],
		onboarding.MaximumEvidencePathBytes,
	) {
		t.Fatalf("evidence path bounds = %#v", evidencePath)
	}
	for _, exactText := range []map[string]interface{}{
		basis,
		scopeProperties["scope_id"].(map[string]interface{}),
		scopeProperties["label"].(map[string]interface{}),
		evidencePath,
	} {
		description, _ := exactText["description"].(string)
		for _, fragment := range []string{
			"without surrounding whitespace or NUL",
			"UTF-8 bytes",
		} {
			if !strings.Contains(description, fragment) {
				t.Fatalf(
					"exact-text schema omits %q: %#v",
					fragment,
					exactText,
				)
			}
		}
	}
}

func TestHaftOnboardSchemaHidesImplementationAndBindingEffects(t *testing.T) {
	t.Parallel()

	tool := haftOnboardTool()
	encoded, err := json.Marshal(tool)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(encoded))
	for _, forbidden := range []string{
		"typeenv",
		"type_env",
		"projecttypeenvhead",
		"graph_revision",
		"change_set",
		"authority_class",
		"contract_version",
		"profile_apply",
		`"memory_enable"`,
		"preparation is read-only",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("haft_onboard schema leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestHaftOnboardCatalogPreservesEffectsAndClosedRecovery(t *testing.T) {
	schema := mustListToolInputSchema(t, "haft_onboard")
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"status never writes",
		"never apply a profile",
		"grant authority",
		"onboarding_required",
		"needs_scope_review",
		"profile_review_ready",
		"profile_review_prepared",
		"memory_review_prepared",
		"memory_deferred",
		"Enable structured project memory",
		"Not now",
		"restart_required",
		"repository_inspected",
		"authority_granted",
	} {
		if !strings.Contains(string(encoded), fragment) {
			t.Fatalf(
				"catalog onboarding schema lost %q:\n%s",
				fragment,
				encoded,
			)
		}
	}
}

func schemaIntegerEquals(raw interface{}, expected int) bool {
	switch value := raw.(type) {
	case int:
		return value == expected
	case float64:
		return value == float64(expected)
	default:
		return false
	}
}
