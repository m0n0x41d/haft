package fpf

import (
	"encoding/base64"

	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const maximumUint32 = uint64(1<<32 - 1)
const maximumUint64 = ^uint64(0)

type memorySurfaceMode uint8

const (
	memorySurfaceUnavailable memorySurfaceMode = iota
	memorySurfaceValidateOnly
	memorySurfaceFull
)

const memoryQueryAction = typedmemorywire.QueryActionMemory

func haftMemoryFullTool() Tool {
	return Tool{
		Name:        "haft_memory",
		Description: "Expert validate/admit. Use haft_entity. No automatic admission.",
		InputSchema: memorySurfaceRequestSchema(memorySurfaceFull),
	}
}

func memorySurfaceRequestSchema(
	mode memorySurfaceMode,
) map[string]interface{} {
	variants := []interface{}{
		memoryValidationRequestSchema(),
	}
	if mode == memorySurfaceFull {
		variants = append(
			variants,
			memoryAdmissionRequestSchema(),
		)
	}
	request := map[string]interface{}{
		"description": "Required MCP envelope for the strict typed-memory decoder. " +
			"Choose exactly one closed validate or admit request; the adapter " +
			"unwraps that request without changing the internal flat wire.",
		"oneOf": variants,
	}
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"request": request,
		},
		[]string{"request"},
	)
}

func memorySurfaceActions(mode memorySurfaceMode) []string {
	switch mode {
	case memorySurfaceValidateOnly:
		return []string{typedmemorywire.ActionValidate}
	case memorySurfaceFull:
		return []string{
			typedmemorywire.ActionValidate,
			typedmemorywire.ActionAdmit,
		}
	default:
		return []string{}
	}
}

func installMemoryQuerySchema(tools []Tool) []Tool {
	for index := range tools {
		if tools[index].Name != "haft_query" {
			continue
		}
		tools[index] = augmentMemoryQueryTool(tools[index])
		return tools
	}
	return append(tools, haftMemoryQueryTool())
}

func haftMemoryQueryTool() Tool {
	return Tool{
		Name:        "haft_query",
		Description: "Read the exact current EntityOfConcern memory scope.",
		InputSchema: memoryQueryRequestSchema(),
	}
}

func augmentMemoryQueryTool(tool Tool) Tool {
	schema, schemaOK := tool.InputSchema.(map[string]interface{})
	if !schemaOK {
		return haftMemoryQueryTool()
	}
	properties, propertiesOK :=
		schema["properties"].(map[string]interface{})
	if !propertiesOK {
		return haftMemoryQueryTool()
	}
	properties["action"] = appendStringEnumValue(
		properties["action"],
		memoryQueryAction,
	)
	properties["memory_request"] = memoryQueryEnvelopeSchema()
	// Do not add memory_request to the shared tool's unconditional required
	// list. Every non-memory haft_query action would then become invalid, while
	// a top-level action-conditional allOf/oneOf is rejected by Anthropic MCP
	// hosts. The nested branches publish their exact required fields and the
	// strict public decoder requires memory_request whenever action=memory.
	tool.InputSchema = schema
	return tool
}

func memoryQueryRequestSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"action":         stringEnumSchema(memoryQueryAction),
			"memory_request": memoryQueryEnvelopeSchema(),
		},
		[]string{"action", "memory_request"},
	)
}

func memoryQueryEnvelopeSchema() map[string]interface{} {
	return map[string]interface{}{
		"description": "Required when action=memory. Choose exactly one closed " +
			"resolve, neighborhood, or recall request. The MCP adapter adds the " +
			"outer memory action and passes the resulting flat bytes unchanged " +
			"to the strict typed-memory decoder. Legacy flat memory fields are " +
			"rejected.",
		"oneOf": []interface{}{
			memoryResolveQueryVariantSchema(),
			memoryNeighborhoodQueryVariantSchema(),
			memoryRecallQueryVariantSchema(),
		},
	}
}

func memoryResolveQueryVariantSchema() map[string]interface{} {
	schema := objectMCPSchemaWithRequired(
		map[string]interface{}{
			"mode":                stringLiteralSchema(typedmemorywire.ActionResolve),
			"contract_version":    stringLiteralSchema(typedmemorywire.ContractVersion),
			"basis":               memoryProjectReadBasisSchema(),
			"query":               boundedReadTextSchema(),
			"bounded_context_ref": boundedIdentifierSchema(),
			"max_candidates":      positiveUint32Schema(),
		},
		[]string{
			"mode",
			"contract_version",
			"basis",
			"query",
			"max_candidates",
		},
	)
	schema["description"] = "resolve branch: read-only identity resolution " +
		"against one project snapshot. Returns exact_entity, entity_candidates, " +
		"known_absent, resolution_unsettled, retry_required, or typed setup " +
		"recovery. known_absent performs no write and grants no persistence " +
		"authority. The strict typed-memory decoder rejects cross-branch fields."
	return schema
}

func memoryNeighborhoodQueryVariantSchema() map[string]interface{} {
	schema := objectMCPSchemaWithRequired(
		map[string]interface{}{
			"mode":                stringLiteralSchema(typedmemorywire.ActionNeighborhood),
			"contract_version":    stringLiteralSchema(typedmemorywire.ContractVersion),
			"basis":               memoryProjectReadBasisSchema(),
			"entity_ref":          memoryEntityReferenceSchema(),
			"bounded_context_ref": boundedIdentifierSchema(),
			"view":                memoryNeighborhoodViewSchema(),
			"read_budget":         memoryReadBudgetSchema(),
		},
		[]string{
			"mode",
			"contract_version",
			"basis",
			"entity_ref",
			"bounded_context_ref",
			"view",
			"read_budget",
		},
	)
	schema["description"] = "neighborhood branch: read-only hydration of one " +
		"exact EntityOfConcern and bounded context under a closed projection " +
		"profile and dimensioned budget. Returns exact_neighborhood, abstained, " +
		"retry_required, or typed setup recovery. The strict typed-memory " +
		"decoder never widens identity, context, or snapshot."
	return schema
}

func memoryRecallQueryVariantSchema() map[string]interface{} {
	schema := objectMCPSchemaWithRequired(
		map[string]interface{}{
			"mode":                stringLiteralSchema(typedmemorywire.ActionRecall),
			"contract_version":    stringLiteralSchema(typedmemorywire.ContractVersion),
			"basis":               memoryProjectReadBasisSchema(),
			"entity_ref":          memoryEntityReferenceSchema(),
			"bounded_context_ref": boundedIdentifierSchema(),
			"view":                memoryNeighborhoodViewSchema(),
			"read_budget":         memoryReadBudgetSchema(),
			"query":               boundedReadTextSchema(),
			"candidate_budget": objectMCPSchemaWithRequired(
				map[string]interface{}{
					"max_candidates": positiveUint32Schema(),
				},
				[]string{"max_candidates"},
			),
		},
		[]string{
			"mode",
			"contract_version",
			"basis",
			"entity_ref",
			"bounded_context_ref",
			"view",
			"read_budget",
			"query",
			"candidate_budget",
		},
	)
	schema["description"] = "recall branch: read-only lexical recall inside one " +
		"exact EntityOfConcern, bounded context, projection profile, and project " +
		"snapshot. Returns scoped candidates, abstained, retry_required, or " +
		"typed setup recovery. The strict typed-memory decoder never widens " +
		"scope and candidate rank grants no authority."
	return schema
}

func appendStringEnumValue(
	raw interface{},
	value string,
) map[string]interface{} {
	return appendStringEnumValues(raw, value)
}

func appendStringEnumValues(
	raw interface{},
	values ...string,
) map[string]interface{} {
	schema, ok := raw.(map[string]interface{})
	if !ok {
		return stringEnumSchema(values...)
	}
	rawValues, ok := schema["enum"].([]interface{})
	if !ok {
		return stringEnumSchema(values...)
	}
	seen := make(map[string]struct{}, len(rawValues)+len(values))
	combined := make([]interface{}, 0, len(rawValues)+len(values))
	for _, rawValue := range rawValues {
		value, valueOK := rawValue.(string)
		if !valueOK {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		combined = append(combined, value)
	}
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		combined = append(combined, value)
	}
	return map[string]interface{}{
		"type": "string",
		"enum": combined,
	}
}

func (mode memorySurfaceMode) allowsAction(action string) bool {
	for _, allowed := range memorySurfaceActions(mode) {
		if action == allowed {
			return true
		}
	}
	return false
}

func memoryValidationRequestSchema() map[string]interface{} {
	schema := memoryRequestVariantSchemaForVersion(
		typedmemorywire.ContractVersionV2,
		typedmemorywire.ActionValidate,
		map[string]interface{}{
			"basis":      memoryBasisSchema(),
			"change_set": memoryChangeSetSchema(),
		},
		[]string{
			"contract_version",
			"action",
			"basis",
			"change_set",
		},
	)
	schema["description"] = "Validate branch: checks one change_set against the " +
		"requested basis and returns valid, invalid, or underdetermined. The " +
		"strict typed-memory decoder performs no admission and writes zero rows; " +
		"a valid verdict grants no persistence authority."
	return schema
}

func memoryAdmissionRequestSchema() map[string]interface{} {
	schema := memoryRequestVariantSchemaForVersion(
		typedmemorywire.ContractVersionV2,
		typedmemorywire.ActionAdmit,
		map[string]interface{}{
			"basis": memoryExactProjectBasisSchema(0),
			"authority_class": stringEnumSchema(
				typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
			),
			"idempotency_key":        admissionIdempotencyKeySchema(),
			"request_provenance_ref": boundedIdentifierSchema(),
			"change_set":             memoryChangeSetSchema(),
		},
		[]string{
			"contract_version",
			"action",
			"basis",
			"authority_class",
			"idempotency_key",
			"request_provenance_ref",
			"change_set",
		},
	)
	schema["description"] = "Admit branch: requires an exact_project basis, " +
		"non_binding_semantic_assertion authority, request provenance, and an " +
		"idempotency key. The strict typed-memory decoder may atomically persist " +
		"the change_set; invalid or underdetermined returns not_admitted with zero " +
		"writes. committed returns a receipt. commit_outcome_unknown requires " +
		"replaying the unchanged request with the same idempotency key and never " +
		"establishes rollback or success. Admission cannot bind a decision, " +
		"commission, specification lifecycle, or evidence truth."
	return schema
}

func memoryRequestVariantSchemaForVersion(
	contractVersion string,
	action string,
	properties map[string]interface{},
	required []string,
) map[string]interface{} {
	actionSchema := stringEnumSchema(action)
	actionSchema["description"] = memoryActionSchemaDescription(action)
	allProperties := map[string]interface{}{
		"contract_version": stringEnumSchema(contractVersion),
		"action":           actionSchema,
	}
	for name, schema := range properties {
		allProperties[name] = schema
	}
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           allProperties,
		"required":             required,
	}
}

func memoryActionSchemaDescription(action string) string {
	switch action {
	case typedmemorywire.ActionValidate:
		return "validate uses the strict typed-memory decoder for a no-write " +
			"valid, invalid, or underdetermined result; it never admits automatically."
	case typedmemorywire.ActionAdmit:
		return "admit uses the strict typed-memory decoder for non-binding atomic " +
			"admission; retry commit_outcome_unknown with the unchanged request " +
			"and the same idempotency key."
	default:
		return "Unsupported strict typed-memory decoder action."
	}
}

func memoryBasisSchema() map[string]interface{} {
	return map[string]interface{}{
		"oneOf": []interface{}{
			memoryBasisVariantSchema(
				string(typedmemorywire.BasisBundledCandidateOpenWorld),
				map[string]interface{}{},
				[]string{"kind"},
			),
			memoryBasisVariantSchema(
				string(typedmemorywire.BasisProjectCurrent),
				map[string]interface{}{},
				[]string{"kind"},
			),
			memoryBasisVariantSchema(
				string(typedmemorywire.BasisExactProject),
				map[string]interface{}{
					"type_env_digest": sha256DigestSchema(),
					"graph_revision": map[string]interface{}{
						"type":    "integer",
						"minimum": 0,
						"maximum": maximumUint64,
					},
				},
				[]string{"kind", "type_env_digest", "graph_revision"},
			),
		},
	}
}

func memoryProjectReadBasisSchema() map[string]interface{} {
	return map[string]interface{}{
		"oneOf": []interface{}{
			memoryBasisVariantSchema(
				string(typedmemorywire.BasisProjectCurrent),
				map[string]interface{}{},
				[]string{"kind"},
			),
			memoryExactProjectBasisSchema(1),
		},
	}
}

func memoryExactProjectBasisSchema(
	minimumGraphRevision int,
) map[string]interface{} {
	return memoryBasisVariantSchema(
		string(typedmemorywire.BasisExactProject),
		map[string]interface{}{
			"type_env_digest": sha256DigestSchema(),
			"graph_revision": map[string]interface{}{
				"type":    "integer",
				"minimum": minimumGraphRevision,
				"maximum": maximumUint64,
			},
		},
		[]string{
			"kind",
			"type_env_digest",
			"graph_revision",
		},
	)
}

func memoryEntityReferenceSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"ref_kind_id":  boundedIdentifierSchema(),
			"reference_id": boundedIdentifierSchema(),
		},
		[]string{"ref_kind_id", "reference_id"},
	)
}

func memoryProjectRecordReferenceSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"ref_kind_id": stringEnumSchema(
				"Haft.ProjectRecordRef",
			),
			"reference_id": boundedIdentifierSchema(),
		},
		[]string{"ref_kind_id", "reference_id"},
	)
}

func memoryNeighborhoodViewSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"projection_profile_ref": stringEnumSchema(
				"agent_orientation.v2",
				"agent_orientation.v1",
				"decision_rationale.v1",
				"spec_impact.v1",
				"evidence_currentness.v1",
				"implementation_trace.v1",
			),
			"requested_facets": map[string]interface{}{
				"type":     "array",
				"minItems": 1,
				"maxItems": typedmemorywire.MaximumArrayItems,
				"items": stringEnumSchema(
					"epistemes",
					"problems",
					"alternatives",
					"decisions",
					"specifications",
					"evidence",
					"work",
					"implementation",
					"unresolved",
				),
			},
			"detail": stringEnumSchema(
				"overview",
				"standard",
				"evidence",
			),
			"include_history": map[string]interface{}{"type": "boolean"},
		},
		[]string{
			"projection_profile_ref",
			"requested_facets",
			"detail",
			"include_history",
		},
	)
}

func memoryReadBudgetSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"max_facets":                     positiveUint32Schema(),
			"max_items_per_facet":            positiveUint32Schema(),
			"max_relation_paths_per_item":    positiveUint32Schema(),
			"max_carrier_excerpt_characters": positiveUint32Schema(),
			"max_provenance_depth":           positiveUint32Schema(),
		},
		[]string{
			"max_facets",
			"max_items_per_facet",
			"max_relation_paths_per_item",
			"max_carrier_excerpt_characters",
			"max_provenance_depth",
		},
	)
}

func objectMCPSchemaWithRequired(
	properties map[string]interface{},
	required []string,
) map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
		"required":             required,
	}
}

func boundedIdentifierSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":      "string",
		"minLength": 1,
		"maxLength": typedmemorywire.MaximumIdentifierBytes,
		"description": "The strict decoder additionally enforces this " +
			"limit in UTF-8 bytes.",
	}
}

func boundedReadTextSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":      "string",
		"minLength": 1,
		"maxLength": typedmemorywire.MaximumTextBytes,
		"description": "The strict decoder additionally enforces this " +
			"limit in UTF-8 bytes and rejects surrounding whitespace.",
	}
}

func admissionIdempotencyKeySchema() map[string]interface{} {
	return map[string]interface{}{
		"type":      "string",
		"minLength": 1,
		"maxLength": typedmemorywire.MaximumAdmissionIdempotencyKeyBytes,
		"description": "Canonical non-empty admission replay key. The strict " +
			"decoder additionally enforces this limit in UTF-8 bytes and " +
			"rejects surrounding whitespace.",
	}
}

func positiveUint32Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":    "integer",
		"minimum": 1,
		"maximum": maximumUint32,
	}
}

func memoryBasisVariantSchema(
	kind string,
	variantProperties map[string]interface{},
	required []string,
) map[string]interface{} {
	properties := map[string]interface{}{
		"kind": stringLiteralSchema(kind),
	}
	for name, schema := range variantProperties {
		properties[name] = schema
	}
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
		"required":             required,
	}
}

func memoryChangeSetSchema() map[string]interface{} {
	return memoryChangeSetSchemaWithRelation(memoryAssertRelationChangeSchema())
}

func memoryChangeSetSchemaWithRelation(
	relation map[string]interface{},
) map[string]interface{} {
	variants := []interface{}{
		memoryDeclareEntityChangeSchema(),
		memoryIdentityChangeSchema(),
	}
	if relation != nil {
		variants = append(variants, relation)
	}
	variants = append(variants, memoryRetractAssertionChangeSchema())
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"changes": map[string]interface{}{
				"type":     "array",
				"minItems": 1,
				"maxItems": typedmemorywire.MaximumChanges,
				"items": map[string]interface{}{
					"oneOf": variants,
				},
			},
		},
		[]string{"changes"},
	)
}

func memoryDeclareEntityChangeSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":       stringLiteralSchema("declare_entity"),
			"entity_id":  boundedIdentifierSchema(),
			"local_ref":  boundedIdentifierSchema(),
			"context":    boundedIdentifierSchema(),
			"label":      boundedTextSchema(),
			"provenance": boundedIdentifierSchema(),
		},
		[]string{
			"kind",
			"entity_id",
			"local_ref",
			"context",
			"label",
			"provenance",
		},
	)
}

func memoryIdentityChangeSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind": stringLiteralSchema("identity_change"),
			"change": map[string]interface{}{
				"oneOf": []interface{}{
					memoryAdmitAliasSchema(),
					memorySupersedeAliasSchema(),
				},
			},
		},
		[]string{"kind", "change"},
	)
}

func memoryAdmitAliasSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":       stringLiteralSchema("admit_alias"),
			"entity_id":  boundedIdentifierSchema(),
			"alias":      boundedIdentifierSchema(),
			"context":    boundedIdentifierSchema(),
			"provenance": boundedIdentifierSchema(),
		},
		[]string{
			"kind",
			"entity_id",
			"alias",
			"context",
			"provenance",
		},
	)
}

func memorySupersedeAliasSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":        stringLiteralSchema("supersede_alias"),
			"entity_id":   boundedIdentifierSchema(),
			"old_alias":   boundedIdentifierSchema(),
			"replacement": boundedIdentifierSchema(),
			"context":     boundedIdentifierSchema(),
			"provenance":  boundedIdentifierSchema(),
		},
		[]string{
			"kind",
			"entity_id",
			"old_alias",
			"replacement",
			"context",
			"provenance",
		},
	)
}

func memoryAssertRelationChangeSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":          stringLiteralSchema("assert_relation"),
			"assertion_id":  boundedIdentifierSchema(),
			"signature_id":  boundedIdentifierSchema(),
			"context_slice": memoryContextSliceSchema(),
			"modality":      memoryAssertionModalitySchema(),
			"bindings": map[string]interface{}{
				"type":     "array",
				"minItems": 1,
				"maxItems": typedmemorywire.MaximumSlotBindings,
				"items":    memorySlotBindingSchema(),
			},
			"provenance": boundedIdentifierSchema(),
		},
		[]string{
			"kind",
			"assertion_id",
			"signature_id",
			"context_slice",
			"modality",
			"bindings",
			"provenance",
		},
	)
}

func memoryAssertionModalitySchema() map[string]interface{} {
	return map[string]interface{}{
		"oneOf": []interface{}{
			memoryAssertionModalityVariantSchema("affirms_obtaining"),
			memoryAssertionModalityVariantSchema("denies_obtaining"),
			memoryAssertionModalityVariantSchema("obtaining_unknown"),
		},
	}
}

func memoryAssertionModalityVariantSchema(kind string) map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind": stringLiteralSchema(kind),
		},
		[]string{"kind"},
	)
}

func memoryContextSliceSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"context": boundedIdentifierSchema(),
			"standard_pins": boundedObjectArraySchema(
				memoryVersionedPinSchema(),
				typedmemorywire.MaximumContextPins,
			),
			"environment_selectors": boundedObjectArraySchema(
				memoryEnvironmentSelectorSchema(),
				typedmemorywire.MaximumEnvironmentSelectors,
			),
			"vocabulary_pins": boundedObjectArraySchema(
				memoryVersionedPinSchema(),
				typedmemorywire.MaximumContextPins,
			),
			"role_set_pins": boundedObjectArraySchema(
				memoryVersionedPinSchema(),
				typedmemorywire.MaximumContextPins,
			),
			"gamma_time": memoryGammaTimeSchema(),
		},
		[]string{
			"context",
			"standard_pins",
			"environment_selectors",
			"vocabulary_pins",
			"role_set_pins",
			"gamma_time",
		},
	)
}

func memoryVersionedPinSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"reference": boundedIdentifierSchema(),
			"edition":   boundedIdentifierSchema(),
			"digest":    sha256DigestSchema(),
		},
		[]string{"reference", "edition", "digest"},
	)
}

func memoryEnvironmentSelectorSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"key":           boundedIdentifierSchema(),
			"value":         boundedIdentifierSchema(),
			"source_digest": sha256DigestSchema(),
		},
		[]string{"key", "value", "source_digest"},
	)
}

func memoryGammaTimeSchema() map[string]interface{} {
	return map[string]interface{}{
		"oneOf": []interface{}{
			memoryGammaPointSchema(),
			memoryGammaWindowSchema(),
			memoryGammaPolicyApplicationSchema(),
		},
	}
}

func memoryResolvedGammaTimeSchema() map[string]interface{} {
	return map[string]interface{}{
		"oneOf": []interface{}{
			memoryGammaPointSchema(),
			memoryGammaWindowSchema(),
		},
	}
}

func memoryGammaPointSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind": stringLiteralSchema("point"),
			"at":   boundedIdentifierSchema(),
		},
		[]string{"kind", "at"},
	)
}

func memoryGammaWindowSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":           stringLiteralSchema("window"),
			"start":          boundedIdentifierSchema(),
			"end":            boundedIdentifierSchema(),
			"start_boundary": stringEnumSchema("inclusive", "exclusive"),
			"end_boundary":   stringEnumSchema("inclusive", "exclusive"),
		},
		[]string{
			"kind",
			"start",
			"end",
			"start_boundary",
			"end_boundary",
		},
	)
}

func memoryGammaPolicyApplicationSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":              stringLiteralSchema("policy_application"),
			"policy_ref":        boundedIdentifierSchema(),
			"policy_edition":    boundedIdentifierSchema(),
			"policy_digest":     sha256DigestSchema(),
			"evaluation_anchor": memoryGammaPointSchema(),
			"resolved":          memoryResolvedGammaTimeSchema(),
		},
		[]string{
			"kind",
			"policy_ref",
			"policy_edition",
			"policy_digest",
			"evaluation_anchor",
			"resolved",
		},
	)
}

func memorySlotBindingSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"slot_kind": boundedIdentifierSchema(),
			"fillers": map[string]interface{}{
				"type":     "array",
				"minItems": 1,
				"maxItems": typedmemorywire.MaximumFillersPerSlot,
				"items": map[string]interface{}{
					"oneOf": []interface{}{
						memoryReferenceFillerSchema(),
						memoryValueFillerSchema(),
					},
				},
			},
		},
		[]string{"slot_kind", "fillers"},
	)
}

func memoryReferenceFillerSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind": stringLiteralSchema("by_reference"),
			"reference": map[string]interface{}{
				"oneOf": []interface{}{
					memoryPersistedReferenceSchema(),
					memoryLocalReferenceSchema(),
				},
			},
		},
		[]string{"kind", "reference"},
	)
}

func memoryPersistedReferenceSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":     stringLiteralSchema("persisted"),
			"ref_kind": boundedIdentifierSchema(),
			"id":       boundedIdentifierSchema(),
		},
		[]string{"kind", "ref_kind", "id"},
	)
}

func memoryLocalReferenceSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":      stringLiteralSchema("local"),
			"ref_kind":  boundedIdentifierSchema(),
			"local_ref": boundedIdentifierSchema(),
		},
		[]string{"kind", "ref_kind", "local_ref"},
	)
}

func memoryValueFillerSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":  stringLiteralSchema("by_value"),
			"value": memoryTypedValueCandidateSchema(),
		},
		[]string{"kind", "value"},
	)
}

func memoryTypedValueCandidateSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"value_kind":  boundedIdentifierSchema(),
			"value_shape": memoryValueShapeSchema(),
			"codec":       memoryCodecSchema(),
			"input_base64": map[string]interface{}{
				"type":      "string",
				"minLength": 1,
				"maxLength": base64.StdEncoding.EncodedLen(
					typedmemorywire.MaximumTypedValueBytes,
				),
			},
			"asserted_digest": memoryAssertedDigestSchema(),
		},
		[]string{
			"value_kind",
			"value_shape",
			"codec",
			"input_base64",
			"asserted_digest",
		},
	)
}

func memoryValueShapeSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"id":     boundedIdentifierSchema(),
			"digest": sha256DigestSchema(),
		},
		[]string{"id", "digest"},
	)
}

func memoryCodecSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"id":                   boundedIdentifierSchema(),
			"version":              boundedIdentifierSchema(),
			"specification_digest": sha256DigestSchema(),
		},
		[]string{"id", "version", "specification_digest"},
	)
}

func memoryAssertedDigestSchema() map[string]interface{} {
	return map[string]interface{}{
		"oneOf": []interface{}{
			objectMCPSchemaWithRequired(
				map[string]interface{}{
					"kind": stringLiteralSchema("none"),
				},
				[]string{"kind"},
			),
			objectMCPSchemaWithRequired(
				map[string]interface{}{
					"kind":   stringLiteralSchema("exact"),
					"digest": sha256DigestSchema(),
				},
				[]string{"kind", "digest"},
			),
		},
	}
}

func memoryRetractAssertionChangeSchema() map[string]interface{} {
	return objectMCPSchemaWithRequired(
		map[string]interface{}{
			"kind":         stringLiteralSchema("retract_assertion"),
			"assertion_id": boundedIdentifierSchema(),
			"reason":       boundedTextSchema(),
			"provenance":   boundedIdentifierSchema(),
		},
		[]string{"kind", "assertion_id", "reason", "provenance"},
	)
}

func boundedObjectArraySchema(
	itemSchema map[string]interface{},
	maximum int,
) map[string]interface{} {
	return map[string]interface{}{
		"type":     "array",
		"maxItems": maximum,
		"items":    itemSchema,
	}
}

func boundedTextSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":      "string",
		"minLength": 1,
		"maxLength": typedmemorywire.MaximumTextBytes,
	}
}

func stringLiteralSchema(value string) map[string]interface{} {
	return map[string]interface{}{
		"const": value,
	}
}

func stringEnumSchema(values ...string) map[string]interface{} {
	enum := make([]interface{}, 0, len(values))
	for _, value := range values {
		enum = append(enum, value)
	}
	return map[string]interface{}{
		"type": "string",
		"enum": enum,
	}
}

func sha256DigestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":    "string",
		"pattern": `^sha256:[0-9a-f]{64}$`,
	}
}
