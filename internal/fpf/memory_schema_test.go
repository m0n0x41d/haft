package fpf

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestHaftMemorySchemaExposesStableExpertContract(t *testing.T) {
	schema := mustListToolInputSchema(t, "haft_memory")
	if schema["type"] != "object" {
		t.Fatalf("haft_memory schema type = %#v, want object", schema["type"])
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("haft_memory additionalProperties = %#v, want false", schema["additionalProperties"])
	}

	required := schemaStringSet(t, schema["required"], "haft_memory.required")
	wantRequired := []string{"request"}
	if !slices.Equal(required, wantRequired) {
		t.Fatalf("haft_memory required = %#v, want %#v", required, wantRequired)
	}

	properties := mustSchemaProperties(t, schema, "haft_memory")
	if len(properties) != 1 {
		t.Fatalf(
			"haft_memory properties = %#v, want only request",
			slices.Sorted(maps.Keys(properties)),
		)
	}
	request, ok := properties["request"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_memory.request schema = %#v", properties["request"])
	}
	requestDescription, _ := request["description"].(string)
	if !strings.Contains(requestDescription, "strict typed-memory decoder") ||
		!strings.Contains(requestDescription, "exactly one closed") {
		t.Fatalf(
			"haft_memory.request description lost routing contract: %q",
			requestDescription,
		)
	}

	variants := memoryRequestVariantsByAction(t, schema, "haft_memory")
	wantRequiredByAction := map[string][]string{
		typedmemorywire.ActionValidate: {
			"action",
			"basis",
			"change_set",
			"contract_version",
		},
		typedmemorywire.ActionAdmit: {
			"action",
			"authority_class",
			"basis",
			"change_set",
			"contract_version",
			"idempotency_key",
			"request_provenance_ref",
		},
	}
	if len(variants) != len(wantRequiredByAction) {
		t.Fatalf(
			"haft_memory request variants = %#v, want validate/admit",
			slices.Sorted(maps.Keys(variants)),
		)
	}
	for action, wantBranchRequired := range wantRequiredByAction {
		variant := variants[action]
		branchRequired := schemaStringSet(
			t,
			variant["required"],
			"haft_memory."+action+".required",
		)
		if !slices.Equal(branchRequired, wantBranchRequired) {
			t.Fatalf(
				"haft_memory %s required = %#v, want %#v",
				action,
				branchRequired,
				wantBranchRequired,
			)
		}
		branchProperties := mustSchemaProperties(
			t,
			variant,
			"haft_memory."+action,
		)
		if len(branchProperties) != len(wantBranchRequired) {
			t.Fatalf(
				"haft_memory %s properties = %#v, want exactly required fields",
				action,
				slices.Sorted(maps.Keys(branchProperties)),
			)
		}
		contractVersions := mustStringEnum(
			t,
			branchProperties["contract_version"],
			"haft_memory."+action+".contract_version",
		)
		if len(contractVersions) != 1 ||
			!contractVersions[typedmemorywire.ContractVersionV2] {
			t.Fatalf(
				"haft_memory %s contract versions = %#v",
				action,
				contractVersions,
			)
		}
		branchDescription, _ := variant["description"].(string)
		actionSchema, _ :=
			branchProperties["action"].(map[string]interface{})
		actionDescription, _ := actionSchema["description"].(string)
		if !strings.Contains(branchDescription, "strict typed-memory decoder") ||
			!strings.Contains(actionDescription, "strict typed-memory decoder") {
			t.Fatalf(
				"haft_memory %s descriptions were compacted: branch=%q action=%q",
				action,
				branchDescription,
				actionDescription,
			)
		}
	}
	validateDescription, _ :=
		variants[typedmemorywire.ActionValidate]["description"].(string)
	for _, fragment := range []string{
		"writes zero rows",
		"grants no persistence authority",
	} {
		if !strings.Contains(validateDescription, fragment) {
			t.Fatalf(
				"validate description omits %q: %q",
				fragment,
				validateDescription,
			)
		}
	}
	admitDescription, _ :=
		variants[typedmemorywire.ActionAdmit]["description"].(string)
	for _, fragment := range []string{
		"exact_project",
		"not_admitted with zero writes",
		"commit_outcome_unknown",
		"unchanged request",
		"cannot bind a decision",
	} {
		if !strings.Contains(admitDescription, fragment) {
			t.Fatalf(
				"admit description omits %q: %q",
				fragment,
				admitDescription,
			)
		}
	}

	validateProperties := mustSchemaProperties(
		t,
		variants[typedmemorywire.ActionValidate],
		"haft_memory.validate",
	)
	basis, ok := validateProperties["basis"].(map[string]interface{})
	if !ok {
		t.Fatalf("basis schema = %#v", validateProperties["basis"])
	}
	basisVariants, ok := basis["oneOf"].([]interface{})
	if !ok || len(basisVariants) != 3 {
		t.Fatalf("basis oneOf = %#v, want three closed variants", basis["oneOf"])
	}
	requiredByKind := map[string][]string{}
	for index, rawVariant := range basisVariants {
		variant, ok := rawVariant.(map[string]interface{})
		if !ok {
			t.Fatalf("basis oneOf[%d] = %#v", index, rawVariant)
		}
		if variant["type"] != "object" ||
			variant["additionalProperties"] != false {
			t.Fatalf("basis oneOf[%d] is not a closed object: %#v", index, variant)
		}
		variantProperties := mustSchemaProperties(
			t,
			variant,
			"basis variant",
		)
		kind := mustStringLiteral(
			t,
			variantProperties["kind"],
			"basis.kind",
		)
		requiredByKind[kind] = schemaStringSet(
			t,
			variant["required"],
			"basis variant.required",
		)
	}
	wantRequiredByKind := map[string][]string{
		string(typedmemorywire.BasisBundledCandidateOpenWorld): {"kind"},
		string(typedmemorywire.BasisProjectCurrent):            {"kind"},
		string(typedmemorywire.BasisExactProject): {
			"graph_revision",
			"kind",
			"type_env_digest",
		},
	}
	if !maps.EqualFunc(
		requiredByKind,
		wantRequiredByKind,
		slices.Equal,
	) {
		t.Fatalf(
			"basis required fields = %#v, want %#v",
			requiredByKind,
			wantRequiredByKind,
		)
	}

	changeSet, ok := validateProperties["change_set"].(map[string]interface{})
	if !ok {
		t.Fatalf("change_set schema = %#v", validateProperties["change_set"])
	}
	if changeSet["type"] != "object" {
		t.Fatalf("change_set type = %#v, want object", changeSet["type"])
	}
	if changeSet["additionalProperties"] != false {
		t.Fatalf("change_set is not closed: %#v", changeSet)
	}
	if !slices.Equal(
		schemaStringSet(t, changeSet["required"], "change_set.required"),
		[]string{"changes"},
	) {
		t.Fatalf("change_set.required = %#v", changeSet["required"])
	}
}

func TestHaftMemorySchemaDoesNotInlineLiveFPFCatalogIdentifiers(t *testing.T) {
	schemaBytes, err := json.Marshal(memoryValidationRequestSchema())
	if err != nil {
		t.Fatal(err)
	}
	body := string(schemaBytes)
	for _, forbidden := range []string{
		`"A.6.5"`,
		`"C.2.1"`,
		`"U.System"`,
		`"U.Kind"`,
		`"U.Episteme"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("haft_memory schema inlined live FPF identifier %s", forbidden)
		}
	}
}

func TestMemoryChangeSetSchemaFullyDescribesClosedBoundedWireAlgebra(
	t *testing.T,
) {
	schema := memoryChangeSetSchema()
	assertClosedMemorySchemaTree(t, schema, "change_set")

	properties := mustSchemaProperties(t, schema, "change_set")
	changes, ok := properties["changes"].(map[string]interface{})
	if !ok {
		t.Fatalf("change_set.changes schema = %#v", properties["changes"])
	}
	if changes["type"] != "array" ||
		changes["minItems"] != 1 ||
		changes["maxItems"] != typedmemorywire.MaximumChanges {
		t.Fatalf("change_set.changes bounds = %#v", changes)
	}
	items, ok := changes["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("change_set.changes.items = %#v", changes["items"])
	}
	variants, ok := items["oneOf"].([]interface{})
	if !ok || len(variants) != 4 {
		t.Fatalf(
			"change_set stable variants = %#v, want four",
			items["oneOf"],
		)
	}
	kinds := make([]string, 0, len(variants))
	for index, raw := range variants {
		variant, variantOK := raw.(map[string]interface{})
		if !variantOK {
			t.Fatalf("change variant %d = %#v", index, raw)
		}
		variantProperties := mustSchemaProperties(
			t,
			variant,
			"change variant",
		)
		kindSchema, kindOK :=
			variantProperties["kind"].(map[string]interface{})
		if !kindOK {
			t.Fatalf("change variant %d kind = %#v", index, variantProperties["kind"])
		}
		kind, literalOK := kindSchema["const"].(string)
		if !literalOK {
			t.Fatalf("change variant %d lacks const discriminator", index)
		}
		kinds = append(kinds, kind)
	}
	slices.Sort(kinds)
	wantKinds := []string{
		"assert_relation",
		"declare_entity",
		"identity_change",
		"retract_assertion",
	}
	if !slices.Equal(kinds, wantKinds) {
		t.Fatalf("change kinds = %#v, want %#v", kinds, wantKinds)
	}

	body, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, nestedKind := range []string{
		"admit_alias",
		"supersede_alias",
		"affirms_obtaining",
		"denies_obtaining",
		"obtaining_unknown",
		"point",
		"window",
		"policy_application",
		"by_reference",
		"by_value",
		"persisted",
		"local",
		"none",
		"exact",
	} {
		needle := `"const":"` + nestedKind + `"`
		if !strings.Contains(string(body), needle) {
			t.Fatalf("change_set schema omits nested variant %q", nestedKind)
		}
	}
	if !strings.Contains(
		string(body),
		fmt.Sprintf(
			`"maxLength":%d`,
			base64.StdEncoding.EncodedLen(
				typedmemorywire.MaximumTypedValueBytes,
			),
		),
	) {
		t.Fatalf(
			"typed-value base64 bound does not mirror %d decoded bytes",
			typedmemorywire.MaximumTypedValueBytes,
		)
	}
}

func TestHaftMemoryPublicSchemaOmitsManualIdentityReconciliation(t *testing.T) {
	t.Parallel()

	for _, schema := range []map[string]interface{}{
		memoryValidationRequestSchema(),
		haftMemoryFullTool().InputSchema.(map[string]interface{}),
	} {
		encoded, err := json.Marshal(schema)
		if err != nil {
			t.Fatal(err)
		}
		body := string(encoded)
		for _, forbidden := range []string{
			`"merge_entities"`,
			`"split_entity"`,
		} {
			if strings.Contains(body, forbidden) {
				t.Fatalf(
					"haft_memory publicly advertises manual identity operation %s",
					forbidden,
				)
			}
		}
	}
	schema := haftMemoryFullTool().InputSchema.(map[string]interface{})
	variants := memoryRequestVariantsByAction(t, schema, "haft_memory")
	actions := slices.Sorted(maps.Keys(variants))
	if !slices.Equal(actions, []string{"admit", "validate"}) {
		t.Fatalf("haft_memory actions = %#v, want validate/admit only", actions)
	}
	tool := haftMemoryFullTool()
	if !strings.Contains(tool.Description, "haft_entity") ||
		!strings.Contains(tool.Description, "automatic admission") {
		t.Fatalf("haft_memory routing description = %q", tool.Description)
	}
	validateProperties := mustSchemaProperties(
		t,
		variants[typedmemorywire.ActionValidate],
		"haft_memory.validate",
	)
	actionSchema, _ :=
		validateProperties["action"].(map[string]interface{})
	actionDescription, _ := actionSchema["description"].(string)
	if !strings.Contains(actionDescription, "never admits automatically") {
		t.Fatalf("haft_memory validate action guidance = %q", actionDescription)
	}
}

func assertClosedMemorySchemaTree(
	t *testing.T,
	raw interface{},
	path string,
) {
	t.Helper()
	switch value := raw.(type) {
	case map[string]interface{}:
		if value["type"] == "object" {
			if value["additionalProperties"] != false {
				t.Fatalf("%s object is not closed: %#v", path, value)
			}
			properties := mustSchemaProperties(t, value, path)
			required := schemaStringSet(t, value["required"], path+".required")
			propertyNames := slices.Sorted(maps.Keys(properties))
			if !slices.Equal(required, propertyNames) {
				t.Fatalf(
					"%s required = %#v, want every property %#v",
					path,
					required,
					propertyNames,
				)
			}
		}
		if value["type"] == "array" {
			if _, bounded := value["maxItems"]; !bounded {
				t.Fatalf("%s array has no decoder-mirroring maxItems", path)
			}
		}
		for key, nested := range value {
			assertClosedMemorySchemaTree(t, nested, path+"."+key)
		}
	case []interface{}:
		for index, nested := range value {
			assertClosedMemorySchemaTree(
				t,
				nested,
				fmt.Sprintf("%s[%d]", path, index),
			)
		}
	}
}

func TestHaftMemoryQuerySchemaPublishesClosedModeEnvelope(t *testing.T) {
	schema, ok := haftMemoryQueryTool().InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf(
			"haft_query memory schema = %T",
			haftMemoryQueryTool().InputSchema,
		)
	}
	assertMemorySchemaHasNoTopLevelCompositors(t, schema, "query read")
	if schema["type"] != "object" ||
		schema["additionalProperties"] != false {
		t.Fatalf("haft_query memory schema is not a closed object: %#v", schema)
	}
	required := schemaStringSet(t, schema["required"], "memory query.required")
	wantRequired := []string{"action", "memory_request"}
	if !slices.Equal(required, wantRequired) {
		t.Fatalf("memory query required = %#v, want %#v", required, wantRequired)
	}

	properties := mustSchemaProperties(t, schema, "memory query")
	actions := mustStringEnum(t, properties["action"], "memory query.action")
	if len(actions) != 1 || !actions[memoryQueryAction] {
		t.Fatalf("memory query actions = %#v", actions)
	}
	if len(properties) != 2 {
		t.Fatalf(
			"memory query properties = %#v, want only action/memory_request",
			slices.Sorted(maps.Keys(properties)),
		)
	}
	variants := memoryQueryVariantsByMode(t, schema, "memory query")
	wantRequiredByMode := map[string][]string{
		typedmemorywire.ActionResolve: {
			"basis",
			"contract_version",
			"max_candidates",
			"mode",
			"query",
		},
		typedmemorywire.ActionNeighborhood: {
			"basis",
			"bounded_context_ref",
			"contract_version",
			"entity_ref",
			"mode",
			"read_budget",
			"view",
		},
		typedmemorywire.ActionRecall: {
			"basis",
			"bounded_context_ref",
			"candidate_budget",
			"contract_version",
			"entity_ref",
			"mode",
			"query",
			"read_budget",
			"view",
		},
	}
	if len(variants) != len(wantRequiredByMode) {
		t.Fatalf("memory query modes = %#v", slices.Sorted(maps.Keys(variants)))
	}
	for mode, wantBranchRequired := range wantRequiredByMode {
		variant := variants[mode]
		gotBranchRequired := schemaStringSet(
			t,
			variant["required"],
			"memory query."+mode+".required",
		)
		if !slices.Equal(gotBranchRequired, wantBranchRequired) {
			t.Fatalf(
				"memory query %s required = %#v, want %#v",
				mode,
				gotBranchRequired,
				wantBranchRequired,
			)
		}
		branchProperties := mustSchemaProperties(
			t,
			variant,
			"memory query."+mode,
		)
		wantPropertyCount := len(wantBranchRequired)
		if mode == typedmemorywire.ActionResolve {
			wantPropertyCount++
			if _, present := branchProperties["bounded_context_ref"]; !present {
				t.Fatal("resolve branch omits optional bounded_context_ref")
			}
		}
		if len(branchProperties) != wantPropertyCount {
			t.Fatalf(
				"memory query %s fields = %#v",
				mode,
				slices.Sorted(maps.Keys(branchProperties)),
			)
		}
		description, _ := variant["description"].(string)
		if !strings.Contains(description, "strict typed-memory decoder") {
			t.Fatalf(
				"memory query %s lost strict runtime contract: %q",
				mode,
				description,
			)
		}
	}
	resolveProperties := mustSchemaProperties(
		t,
		variants[typedmemorywire.ActionResolve],
		"memory query.resolve",
	)
	for _, forbidden := range []string{
		"authority_class",
		"idempotency_key",
		"request_provenance_ref",
	} {
		for mode, variant := range variants {
			branchProperties := mustSchemaProperties(
				t,
				variant,
				"memory query."+mode,
			)
			if _, present := branchProperties[forbidden]; present {
				t.Fatalf(
					"read-only memory %s exposes admission field %q",
					mode,
					forbidden,
				)
			}
		}
	}

	querySchema, ok := resolveProperties["query"].(map[string]interface{})
	if !ok ||
		querySchema["maxLength"] != typedmemorywire.MaximumTextBytes {
		t.Fatalf("resolve query schema = %#v", resolveProperties["query"])
	}
	contextSchema, ok :=
		resolveProperties["bounded_context_ref"].(map[string]interface{})
	if !ok ||
		contextSchema["maxLength"] !=
			typedmemorywire.MaximumIdentifierBytes {
		t.Fatalf(
			"resolve context schema = %#v",
			resolveProperties["bounded_context_ref"],
		)
	}
	neighborhoodProperties := mustSchemaProperties(
		t,
		variants[typedmemorywire.ActionNeighborhood],
		"memory query.neighborhood",
	)
	entitySchema, ok :=
		neighborhoodProperties["entity_ref"].(map[string]interface{})
	if !ok {
		t.Fatalf(
			"neighborhood entity schema = %#v",
			neighborhoodProperties["entity_ref"],
		)
	}
	entityProperties := mustSchemaProperties(
		t,
		entitySchema,
		"neighborhood entity",
	)
	for _, field := range []string{"ref_kind_id", "reference_id"} {
		fieldSchema, fieldOK :=
			entityProperties[field].(map[string]interface{})
		if !fieldOK ||
			fieldSchema["maxLength"] !=
				typedmemorywire.MaximumIdentifierBytes {
			t.Fatalf("%s schema = %#v", field, entityProperties[field])
		}
	}
}

func TestMemoryNeighborhoodViewSchemaRetainsLegacyAndCurrentOrientationEditions(
	t *testing.T,
) {
	schema := memoryNeighborhoodViewSchema()
	properties := mustSchemaProperties(t, schema, "memory neighborhood view")
	profiles := mustStringEnum(
		t,
		properties["projection_profile_ref"],
		"memory neighborhood view.projection_profile_ref",
	)
	for _, expected := range []string{
		"agent_orientation.v1",
		"agent_orientation.v2",
	} {
		if !profiles[expected] {
			t.Fatalf("projection profile enum omitted %q", expected)
		}
	}

	requested, ok := properties["requested_facets"].(map[string]interface{})
	if !ok {
		t.Fatalf("requested_facets schema = %#v", properties["requested_facets"])
	}
	facets := mustStringEnum(
		t,
		requested["items"],
		"memory neighborhood view.requested_facets.items",
	)
	if !facets["epistemes"] {
		t.Fatal("requested facet enum omitted epistemes")
	}
}

func TestHaftQueryMemorySchemaFirstCallsReachStrictDecoderOnFirstAttempt(
	t *testing.T,
) {
	server := NewServer("test")
	decodedModes := make([]string, 0, 3)
	v5Actions := make([]string, 0, 1)
	server.SetV5Handler(func(
		_ context.Context,
		action string,
		_ json.RawMessage,
	) (string, error) {
		v5Actions = append(v5Actions, action)
		return `{"result_kind":"non_memory_ok"}`, nil
	})
	server.SetMemoryReadHandler(func(
		_ context.Context,
		arguments json.RawMessage,
	) (string, error) {
		request, err := typedmemorywire.DecodeQueryReadRequest(arguments)
		if err != nil {
			return "", err
		}
		decodedModes = append(decodedModes, request.Action())
		return `{"result_kind":"schema_first_ok"}`, nil
	})

	schema := actualToolsListInputSchema(t, server, "haft_query")
	if additional, present := schema["additionalProperties"]; !present ||
		additional != false {
		t.Fatalf(
			"actual tools/list haft_query outer schema is open: %#v",
			additional,
		)
	}
	required := schemaStringSet(
		t,
		schema["required"],
		"actual tools/list haft_query.required",
	)
	if !slices.Equal(required, []string{"action"}) {
		t.Fatalf(
			"shared haft_query required = %#v, want host-safe [action]",
			required,
		)
	}
	variants := memoryQueryVariantsByMode(
		t,
		schema,
		"actual tools/list haft_query",
	)
	assertMemoryQueryArraysStayTypedAfterCompaction(t, variants)

	for _, mode := range []string{
		typedmemorywire.ActionResolve,
		typedmemorywire.ActionNeighborhood,
		typedmemorywire.ActionRecall,
	} {
		arguments := schemaFirstMemoryQueryArguments(
			t,
			variants[mode],
		)
		encodedArguments, err := json.Marshal(arguments)
		if err != nil {
			t.Fatal(err)
		}
		params, err := json.Marshal(map[string]any{
			"name":      "haft_query",
			"arguments": json.RawMessage(encodedArguments),
		})
		if err != nil {
			t.Fatal(err)
		}
		result := captureToolsCallResult(t, server, JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "tools/call",
			ID:      "schema-first-" + mode,
			Params:  params,
		})
		if result.IsError ||
			len(result.Content) != 1 ||
			result.Content[0].Text !=
				`{"result_kind":"schema_first_ok"}` {
			t.Fatalf(
				"schema-first %s call failed: %#v\narguments=%s",
				mode,
				result,
				encodedArguments,
			)
		}
	}
	if !slices.Equal(
		decodedModes,
		[]string{
			typedmemorywire.ActionResolve,
			typedmemorywire.ActionNeighborhood,
			typedmemorywire.ActionRecall,
		},
	) {
		t.Fatalf("strict decoder modes = %#v", decodedModes)
	}

	nonMemoryResult := captureToolsCallResult(t, server, JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      "schema-first-status",
		Params: json.RawMessage(
			`{"name":"haft_query","arguments":` +
				`{"action":"status","full":true}}`,
		),
	})
	if nonMemoryResult.IsError ||
		len(nonMemoryResult.Content) != 1 ||
		nonMemoryResult.Content[0].Text !=
			`{"result_kind":"non_memory_ok"}` ||
		!slices.Equal(v5Actions, []string{"haft_query"}) {
		t.Fatalf(
			"closed outer schema broke non-memory query: result=%#v actions=%#v",
			nonMemoryResult,
			v5Actions,
		)
	}

	for name, arguments := range map[string]string{
		"missing_memory_request": `{"action":"memory"}`,
		"legacy_flat": `{"action":"memory","mode":"resolve",` +
			`"contract_version":"haft.memory.v1",` +
			`"basis":{"kind":"project_current"},` +
			`"query":"authorization service","max_candidates":5}`,
		"unknown_outer": `{"action":"memory","unexpected":true,` +
			`"memory_request":{"mode":"resolve",` +
			`"contract_version":"haft.memory.v1",` +
			`"basis":{"kind":"project_current"},` +
			`"query":"authorization service","max_candidates":5}}`,
	} {
		t.Run(name, func(t *testing.T) {
			params := `{"name":"haft_query","arguments":` +
				arguments +
				`}`
			result := captureToolsCallResult(t, server, JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "tools/call",
				ID:      "schema-first-reject-" + name,
				Params:  json.RawMessage(params),
			})
			if !result.IsError || len(result.Content) != 1 {
				t.Fatalf(
					"invalid public memory query was accepted: %#v",
					result,
				)
			}
		})
	}
}

func actualToolsListInputSchema(
	t *testing.T,
	server *Server,
	toolName string,
) map[string]interface{} {
	t.Helper()

	pages := mustToolsListResponsePagesForServer(t, server)
	for _, page := range pages {
		for _, tool := range page.tools {
			if tool["name"] != toolName {
				continue
			}
			schema, ok := tool["inputSchema"].(map[string]interface{})
			if !ok {
				t.Fatalf(
					"actual tools/list %s inputSchema = %#v",
					toolName,
					tool["inputSchema"],
				)
			}
			return schema
		}
	}
	t.Fatalf("actual tools/list omitted %s", toolName)
	return nil
}

func schemaFirstMemoryQueryArguments(
	t *testing.T,
	variant map[string]interface{},
) map[string]any {
	t.Helper()
	properties := mustSchemaProperties(t, variant, "schema-first branch")
	mode := mustStringLiteral(
		t,
		properties["mode"],
		"schema-first mode",
	)
	contractVersion := mustStringLiteral(
		t,
		properties["contract_version"],
		"schema-first contract_version",
	)
	memoryRequest := map[string]any{
		"mode":             mode,
		"contract_version": contractVersion,
		"basis": map[string]any{
			"kind": string(typedmemorywire.BasisProjectCurrent),
		},
	}
	switch mode {
	case typedmemorywire.ActionResolve:
		memoryRequest["query"] = "authorization service"
		memoryRequest["max_candidates"] = 5
	case typedmemorywire.ActionNeighborhood:
		addSchemaFirstNeighborhoodFields(memoryRequest)
	case typedmemorywire.ActionRecall:
		addSchemaFirstNeighborhoodFields(memoryRequest)
		memoryRequest["query"] = "authorization decisions"
		memoryRequest["candidate_budget"] = map[string]any{
			"max_candidates": 5,
		}
	default:
		t.Fatalf("unsupported schema-first memory mode %q", mode)
	}
	required := schemaStringSet(
		t,
		variant["required"],
		"schema-first branch.required",
	)
	for _, field := range required {
		if _, present := memoryRequest[field]; !present {
			t.Fatalf(
				"schema-first %s request omits required field %q",
				mode,
				field,
			)
		}
	}
	return map[string]any{
		"action":         memoryQueryAction,
		"memory_request": memoryRequest,
	}
}

func addSchemaFirstNeighborhoodFields(request map[string]any) {
	request["entity_ref"] = map[string]any{
		"ref_kind_id":  "U.EntityRef",
		"reference_id": "service:authorization",
	}
	request["bounded_context_ref"] = "context:authorization"
	request["view"] = map[string]any{
		"projection_profile_ref": "agent_orientation.v2",
		"requested_facets":       []string{"problems", "decisions"},
		"detail":                 "standard",
		"include_history":        false,
	}
	request["read_budget"] = map[string]any{
		"max_facets":                     2,
		"max_items_per_facet":            8,
		"max_relation_paths_per_item":    4,
		"max_carrier_excerpt_characters": 2048,
		"max_provenance_depth":           3,
	}
}

func assertMemoryQueryArraysStayTypedAfterCompaction(
	t *testing.T,
	variants map[string]map[string]interface{},
) {
	t.Helper()
	for mode, variant := range variants {
		body, err := json.Marshal(variant)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), `"items":{}`) {
			t.Fatalf(
				"compacted %s branch contains unconstrained array items: %s",
				mode,
				body,
			)
		}
	}
	neighborhood := mustSchemaProperties(
		t,
		variants[typedmemorywire.ActionNeighborhood],
		"compacted neighborhood",
	)
	view := mustSchemaProperties(
		t,
		neighborhood["view"],
		"compacted neighborhood.view",
	)
	requested, ok := view["requested_facets"].(map[string]interface{})
	if !ok || requested["type"] != "array" {
		t.Fatalf(
			"compacted requested_facets = %#v",
			view["requested_facets"],
		)
	}
	items, ok := requested["items"].(map[string]interface{})
	if !ok || items["type"] != "string" {
		t.Fatalf("compacted requested_facets.items = %#v", requested["items"])
	}
}

func TestHaftMemoryFullSchemaIsHostCompatibleClosedEnvelope(t *testing.T) {
	schema, ok := haftMemoryFullTool().InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf(
			"haft_memory full schema = %T",
			haftMemoryFullTool().InputSchema,
		)
	}
	assertMemorySchemaHasNoTopLevelCompositors(t, schema, "full")
	if schema["type"] != "object" ||
		schema["additionalProperties"] != false {
		t.Fatalf("haft_memory full schema is not a closed object: %#v", schema)
	}
	required := schemaStringSet(t, schema["required"], "memory full.required")
	wantSurfaceRequired := []string{"request"}
	if !slices.Equal(required, wantSurfaceRequired) {
		t.Fatalf(
			"memory full required = %#v, want %#v",
			required,
			wantSurfaceRequired,
		)
	}

	properties := mustSchemaProperties(t, schema, "memory full")
	if len(properties) != 1 {
		t.Fatalf(
			"memory full properties = %#v, want only request",
			slices.Sorted(maps.Keys(properties)),
		)
	}
	variants := memoryRequestVariantsByAction(t, schema, "memory full")
	wantActions := []string{
		typedmemorywire.ActionAdmit,
		typedmemorywire.ActionValidate,
	}
	gotActions := slices.Sorted(maps.Keys(variants))
	if !slices.Equal(gotActions, wantActions) {
		t.Fatalf("memory full actions = %#v, want %#v", gotActions, wantActions)
	}
	validate := mustSchemaProperties(
		t,
		variants[typedmemorywire.ActionValidate],
		"memory full.validate",
	)
	if _, present := validate["authority_class"]; present {
		t.Fatal("validate branch was broadened with admission authority")
	}
	admit := mustSchemaProperties(
		t,
		variants[typedmemorywire.ActionAdmit],
		"memory full.admit",
	)
	for _, field := range []string{
		"basis",
		"change_set",
		"authority_class",
		"idempotency_key",
		"request_provenance_ref",
	} {
		if _, present := admit[field]; !present {
			t.Fatalf("memory full admit branch omits field %q", field)
		}
	}
	body, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"instantiate_relation"`) {
		t.Fatal("memory full schema exposes frozen v1 instantiate_relation")
	}
	for _, required := range []string{
		`"assert_relation"`,
		`"affirms_obtaining"`,
		`"denies_obtaining"`,
		`"obtaining_unknown"`,
	} {
		if !strings.Contains(string(body), required) {
			t.Fatalf("memory full schema omits v2 assertion member %s", required)
		}
	}
}

func TestHaftMemoryAdmissionVariantIsStrictAndNonBinding(t *testing.T) {
	admit := memoryAdmissionRequestSchema()
	wantRequired := []string{
		"action",
		"authority_class",
		"basis",
		"change_set",
		"contract_version",
		"idempotency_key",
		"request_provenance_ref",
	}
	required := schemaStringSet(t, admit["required"], "admit.required")
	if !slices.Equal(required, wantRequired) {
		t.Fatalf("admit required = %#v, want %#v", required, wantRequired)
	}

	properties := mustSchemaProperties(t, admit, "admit")
	if len(properties) != len(wantRequired) {
		t.Fatalf(
			"admit properties = %#v, want exactly required fields",
			slices.Sorted(maps.Keys(properties)),
		)
	}
	contractVersions := mustStringEnum(
		t,
		properties["contract_version"],
		"admit.contract_version",
	)
	if len(contractVersions) != 1 ||
		!contractVersions[typedmemorywire.ContractVersionV2] {
		t.Fatalf("admit contract versions = %#v, want v2 only", contractVersions)
	}
	authority := mustStringEnum(
		t,
		properties["authority_class"],
		"admit.authority_class",
	)
	_, nonBinding := authority[typedmemorywire.AuthorityClassNonBindingSemanticAssertion]
	if len(authority) != 1 || !nonBinding {
		t.Fatalf("admit authority class = %#v", authority)
	}
	idempotency, ok :=
		properties["idempotency_key"].(map[string]interface{})
	if !ok ||
		idempotency["minLength"] != 1 ||
		idempotency["maxLength"] !=
			typedmemorywire.MaximumAdmissionIdempotencyKeyBytes {
		t.Fatalf("admit idempotency schema = %#v", properties["idempotency_key"])
	}

	basis, ok := properties["basis"].(map[string]interface{})
	if !ok ||
		basis["type"] != "object" ||
		basis["additionalProperties"] != false {
		t.Fatalf("admit basis is not a closed exact selector: %#v", properties["basis"])
	}
	basisRequired := schemaStringSet(t, basis["required"], "admit.basis.required")
	wantBasisRequired := []string{"graph_revision", "kind", "type_env_digest"}
	if !slices.Equal(basisRequired, wantBasisRequired) {
		t.Fatalf(
			"admit basis required = %#v, want %#v",
			basisRequired,
			wantBasisRequired,
		)
	}
	basisProperties := mustSchemaProperties(t, basis, "admit.basis")
	kind := mustStringLiteral(
		t,
		basisProperties["kind"],
		"admit.basis.kind",
	)
	if kind != string(typedmemorywire.BasisExactProject) {
		t.Fatalf("admit basis kind = %q", kind)
	}
	revision, ok :=
		basisProperties["graph_revision"].(map[string]interface{})
	if !ok || revision["minimum"] != 0 {
		t.Fatalf(
			"admit graph revision schema = %#v",
			basisProperties["graph_revision"],
		)
	}

	admitBody, err := json.Marshal(admit)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`"instantiate_relation"`,
		`"operator_confirmed"`,
		`"decision_binding"`,
		`"project_current"`,
		`"bundled_candidate_open_world"`,
	} {
		if strings.Contains(string(admitBody), forbidden) {
			t.Fatalf("admit schema contains forbidden broadening %s", forbidden)
		}
	}
	for _, required := range []string{
		`"assert_relation"`,
		`"affirms_obtaining"`,
		`"denies_obtaining"`,
		`"obtaining_unknown"`,
	} {
		if !strings.Contains(string(admitBody), required) {
			t.Fatalf("admit schema omits v2 assertion member %s", required)
		}
	}
}

func TestHaftMemoryCatalogDescriptionDoesNotDescribeAdmissionAsReadOnly(
	t *testing.T,
) {
	server := NewServer("test")
	server.SetMemoryFullHandler(
		func(context.Context, json.RawMessage) (string, error) {
			return "", nil
		},
	)

	tool := catalogTool(t, server.ToolCatalog(), "haft_memory")
	if !strings.Contains(tool.Description, "admit") {
		t.Fatalf(
			"full haft_memory description = %q, want admission disclosed",
			tool.Description,
		)
	}
	if strings.Contains(tool.Description, "without persisting") {
		t.Fatalf(
			"full haft_memory description falsely claims no persistence: %q",
			tool.Description,
		)
	}
}

func TestHaftMemoryCatalogEntryIsStableAcrossHandlerAvailability(t *testing.T) {
	server := NewServer("test")
	for _, name := range []string{
		"haft_query",
		"haft_onboard",
		"haft_entity",
		"haft_memory",
	} {
		if !catalogContainsTool(server.ToolCatalog(), name) {
			t.Fatalf("%s disappeared without a dedicated handler", name)
		}
	}

	server.SetMemoryHandler(func(_ context.Context, _ json.RawMessage) (string, error) {
		return "", nil
	})
	catalog := server.ToolCatalog()
	if !catalogContainsTool(catalog, "haft_memory") {
		t.Fatal("haft_memory not advertised with a dedicated handler")
	}

	server.SetMemoryReadHandler(
		func(_ context.Context, _ json.RawMessage) (string, error) {
			return "", nil
		},
	)
	readCatalog := server.ToolCatalog()
	readTool := catalogTool(t, readCatalog, "haft_query")
	readSchema, ok := readTool.InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("memory query schema = %T", readTool.InputSchema)
	}
	readProperties := mustSchemaProperties(t, readSchema, "catalog read")
	readActions := mustStringEnum(t, readProperties["action"], "catalog read.action")
	if !readActions[memoryQueryAction] {
		t.Fatalf("memory query actions = %#v", readActions)
	}
	validateTool := catalogTool(t, readCatalog, "haft_memory")
	validateSchema, ok := validateTool.InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("validate memory schema = %T", validateTool.InputSchema)
	}
	validateVariants := memoryRequestVariantsByAction(
		t,
		validateSchema,
		"catalog validate",
	)
	validateActions := slices.Sorted(maps.Keys(validateVariants))
	if !slices.Equal(
		validateActions,
		[]string{
			typedmemorywire.ActionAdmit,
			typedmemorywire.ActionValidate,
		},
	) {
		t.Fatalf("stable expert memory actions = %#v", validateActions)
	}

	server.SetMemoryFullHandler(
		func(_ context.Context, _ json.RawMessage) (string, error) {
			return "", nil
		},
	)
	fullTool := catalogTool(t, server.ToolCatalog(), "haft_memory")
	fullSchema, ok := fullTool.InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("full memory schema = %T", fullTool.InputSchema)
	}
	assertMemorySchemaHasNoTopLevelCompositors(t, fullSchema, "catalog full")
	fullVariants := memoryRequestVariantsByAction(
		t,
		fullSchema,
		"catalog full",
	)
	fullActions := slices.Sorted(maps.Keys(fullVariants))
	if !slices.Equal(
		fullActions,
		[]string{
			typedmemorywire.ActionAdmit,
			typedmemorywire.ActionValidate,
		},
	) {
		t.Fatalf("full memory actions = %#v", fullActions)
	}

	server.SetMemoryFullHandler(nil)
	if !catalogContainsTool(server.ToolCatalog(), "haft_memory") {
		t.Fatal("handler removal made haft_memory disappear")
	}
	if server.memorySurface != memorySurfaceUnavailable {
		t.Fatalf(
			"memory surface after nil handler = %d, want unavailable",
			server.memorySurface,
		)
	}
	if !catalogContainsTool(server.ToolCatalog(), "haft_query") {
		t.Fatal("removing mutation handler also removed memory reads")
	}
}

func TestHaftMemoryReadAndFullToolsMarshalInAtomicCatalog(t *testing.T) {
	handler := func(
		context.Context,
		json.RawMessage,
	) (string, error) {
		return "", nil
	}
	for _, fixture := range []struct {
		name      string
		configure func(*Server)
	}{
		{
			name: "read",
			configure: func(server *Server) {
				server.SetMemoryReadHandler(handler)
			},
		},
		{
			name: "full",
			configure: func(server *Server) {
				server.SetMemoryFullHandler(handler)
			},
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			server := NewServer("test")
			fixture.configure(server)
			catalog := server.ToolCatalog()
			for _, name := range []string{
				"haft_query",
				"haft_onboard",
				"haft_entity",
				"haft_memory",
			} {
				if !catalogContainsTool(catalog, name) {
					t.Fatalf("%s absent from memory recovery catalog", name)
				}
			}
			encoded, err := json.Marshal(map[string]interface{}{
				"tools": catalog,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s compacted catalog bytes=%d", fixture.name, len(encoded))
		})
	}
}

func TestAugmentedHaftQueryMarshalsInAtomicCatalog(t *testing.T) {
	server := NewServer("test")
	server.SetV5Handler(func(
		context.Context,
		string,
		json.RawMessage,
	) (string, error) {
		return "", nil
	})
	server.SetMemoryReadHandler(func(
		context.Context,
		json.RawMessage,
	) (string, error) {
		return "", nil
	})

	catalog := server.ToolCatalog()
	query := catalogTool(t, catalog, "haft_query")
	schema, ok := query.InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("augmented query schema = %T", query.InputSchema)
	}
	properties := mustSchemaProperties(t, schema, "augmented query")
	view, ok := properties["view"].(map[string]interface{})
	if !ok {
		t.Fatalf("augmented query view schema = %#v", properties["view"])
	}
	if view["type"] != "string" {
		t.Fatalf("augmented query changed non-memory view: %#v", view)
	}
	viewDescription, _ := view["description"].(string)
	for _, fragment := range []string{
		"action=fpf",
		"working (default), trace, or diagnostic",
	} {
		if !strings.Contains(viewDescription, fragment) {
			t.Fatalf("augmented query view description missing %q: %q", fragment, viewDescription)
		}
	}
	memoryRequest, ok :=
		properties["memory_request"].(map[string]interface{})
	if !ok {
		t.Fatalf(
			"augmented query memory_request = %#v",
			properties["memory_request"],
		)
	}
	rawVariants, ok := memoryRequest["oneOf"].([]interface{})
	if !ok || len(rawVariants) != 3 {
		t.Fatalf(
			"augmented query memory_request.oneOf = %#v",
			memoryRequest["oneOf"],
		)
	}
	traceRef, ok := properties["trace_ref"].(map[string]interface{})
	if !ok || traceRef["type"] != "string" {
		t.Fatalf("augmented query lost FPF trace_ref: %#v", properties["trace_ref"])
	}
	encoded, err := json.Marshal(map[string]interface{}{
		"tools": catalog,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 || query.Name != "haft_query" {
		t.Fatalf("atomic catalog did not marshal augmented haft_query")
	}
}

func memoryQueryVariantsByMode(
	t *testing.T,
	schema map[string]interface{},
	label string,
) map[string]map[string]interface{} {
	t.Helper()
	properties := mustSchemaProperties(t, schema, label)
	request, ok := properties["memory_request"].(map[string]interface{})
	if !ok {
		t.Fatalf(
			"%s.memory_request schema = %#v",
			label,
			properties["memory_request"],
		)
	}
	rawVariants, ok := request["oneOf"].([]interface{})
	if !ok || len(rawVariants) == 0 {
		t.Fatalf(
			"%s.memory_request.oneOf = %#v",
			label,
			request["oneOf"],
		)
	}
	variants := make(map[string]map[string]interface{}, len(rawVariants))
	for index, rawVariant := range rawVariants {
		variant, variantOK := rawVariant.(map[string]interface{})
		if !variantOK ||
			variant["type"] != "object" ||
			variant["additionalProperties"] != false {
			t.Fatalf(
				"%s.memory_request.oneOf[%d] is not closed: %#v",
				label,
				index,
				rawVariant,
			)
		}
		variantProperties := mustSchemaProperties(
			t,
			variant,
			fmt.Sprintf(
				"%s.memory_request.oneOf[%d]",
				label,
				index,
			),
		)
		mode := mustStringLiteral(
			t,
			variantProperties["mode"],
			fmt.Sprintf(
				"%s.memory_request.oneOf[%d].mode",
				label,
				index,
			),
		)
		if _, duplicate := variants[mode]; duplicate {
			t.Fatalf("%s memory mode %q is duplicated", label, mode)
		}
		variants[mode] = variant
	}
	return variants
}

func assertMemorySchemaHasNoTopLevelCompositors(
	t *testing.T,
	schema map[string]interface{},
	label string,
) {
	t.Helper()
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if _, present := schema[key]; present {
			t.Fatalf(
				"%s memory schema declares host-rejected top-level %q",
				label,
				key,
			)
		}
	}
}

func memoryRequestVariantsByAction(
	t *testing.T,
	schema map[string]interface{},
	label string,
) map[string]map[string]interface{} {
	t.Helper()
	properties := mustSchemaProperties(t, schema, label)
	request, ok := properties["request"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s.request schema = %#v", label, properties["request"])
	}
	rawVariants, ok := request["oneOf"].([]interface{})
	if !ok || len(rawVariants) == 0 {
		t.Fatalf("%s.request.oneOf = %#v", label, request["oneOf"])
	}
	variants := make(map[string]map[string]interface{}, len(rawVariants))
	for index, rawVariant := range rawVariants {
		variant, variantOK := rawVariant.(map[string]interface{})
		if !variantOK {
			t.Fatalf(
				"%s.request.oneOf[%d] = %#v, want object schema",
				label,
				index,
				rawVariant,
			)
		}
		if variant["type"] != "object" ||
			variant["additionalProperties"] != false {
			t.Fatalf(
				"%s.request.oneOf[%d] is not closed: %#v",
				label,
				index,
				variant,
			)
		}
		variantProperties := mustSchemaProperties(
			t,
			variant,
			fmt.Sprintf("%s.request.oneOf[%d]", label, index),
		)
		actions := mustStringEnum(
			t,
			variantProperties["action"],
			fmt.Sprintf("%s.request.oneOf[%d].action", label, index),
		)
		actionNames := slices.Sorted(maps.Keys(actions))
		if len(actionNames) != 1 {
			t.Fatalf(
				"%s.request.oneOf[%d] action enum = %#v, want one discriminator",
				label,
				index,
				actionNames,
			)
		}
		action := actionNames[0]
		if _, duplicate := variants[action]; duplicate {
			t.Fatalf("%s request action %q is duplicated", label, action)
		}
		variants[action] = variant
	}
	return variants
}

func TestToolCatalogKeepsConciseSelectionDescriptions(t *testing.T) {
	server := NewServer("test")
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})
	server.SetMemoryHandler(func(_ context.Context, _ json.RawMessage) (string, error) {
		return "", nil
	})

	for _, tool := range server.ToolCatalog() {
		if strings.TrimSpace(tool.Description) == "" {
			t.Fatalf("%s has no selection description", tool.Name)
		}
		if len(tool.Description) > 72 {
			t.Fatalf("%s description is not concise: %q", tool.Name, tool.Description)
		}
	}
	for _, name := range []string{"haft_decision", "haft_commission", "haft_spec_section"} {
		description := catalogToolDescription(t, server.ToolCatalog(), name)
		if !strings.Contains(description, "human-gated") {
			t.Fatalf("%s description omits human authority boundary: %q", name, description)
		}
		if !strings.Contains(description, "brief") {
			t.Fatalf("%s description omits self-contained gate briefing cue: %q", name, description)
		}
	}
	queryDescription := catalogToolDescription(t, server.ToolCatalog(), "haft_query")
	if !strings.Contains(queryDescription, "human gates") ||
		!strings.Contains(queryDescription, "briefs") {
		t.Fatalf("haft_query description omits gate expansion cue: %q", queryDescription)
	}
}

func schemaStringSet(t *testing.T, raw interface{}, label string) []string {
	t.Helper()

	direct, ok := raw.([]string)
	if ok {
		result := append([]string{}, direct...)
		slices.Sort(result)
		return result
	}
	values, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("%s = %#v, want string array", label, raw)
	}
	result := make([]string, 0, len(values))
	for _, rawValue := range values {
		value, ok := rawValue.(string)
		if !ok {
			t.Fatalf("%s contains non-string %#v", label, rawValue)
		}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func mustStringLiteral(
	t *testing.T,
	raw interface{},
	label string,
) string {
	t.Helper()
	schema, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s = %#v, want schema object", label, raw)
	}
	value, ok := schema["const"].(string)
	if !ok || value == "" {
		t.Fatalf("%s const = %#v, want non-empty string", label, schema["const"])
	}
	return value
}

func catalogContainsTool(catalog []Tool, name string) bool {
	for _, tool := range catalog {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func catalogToolDescription(t *testing.T, catalog []Tool, name string) string {
	t.Helper()
	for _, tool := range catalog {
		if tool.Name == name {
			return tool.Description
		}
	}
	t.Fatalf("tool %q not found", name)
	return ""
}

func catalogTool(t *testing.T, catalog []Tool, name string) Tool {
	t.Helper()
	for _, tool := range catalog {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return Tool{}
}
