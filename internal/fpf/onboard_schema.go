package fpf

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/onboarding"
)

const (
	haftOnboardStatusAction         = "status"
	haftOnboardProfilePrepareAction = "profile_prepare"
	haftOnboardMemoryPrepareAction  = "memory_prepare"
)

func haftOnboardTool() Tool {
	schema := onboardRequestSchema()
	return Tool{
		Name: "haft_onboard",
		Description: "Inspect readable Haft setup status or prepare a review. " +
			"Status and detection are read-only. Prepare actions may materialize or reuse only a non-binding review carrier; they never apply a project profile or enable structured project memory.",
		InputSchema: schema,
	}
}

func onboardRequestSchema() map[string]interface{} {
	action := stringEnumSchema(
		haftOnboardStatusAction,
		haftOnboardProfilePrepareAction,
		haftOnboardMemoryPrepareAction,
	)
	action["description"] = "status never writes and accepts only action. " +
		"profile_prepare accepts action alone for advisory repository detection, " +
		"or action plus both basis and non-empty scopes for an explicit fallback. " +
		"memory_prepare accepts only action. Prepare actions may materialize or " +
		"reuse only a non-binding review carrier."
	scope := onboardScopeSchema()
	scopes := map[string]interface{}{
		"type":        "array",
		"minItems":    1,
		"maxItems":    onboarding.MaximumProfileScopes,
		"items":       scope,
		"uniqueItems": true,
		"description": "Optional explicit fallback scopes for profile_prepare. " +
			"When present, basis is also required and scope_id values must be " +
			"unique. The server rejects scopes for status and memory_prepare.",
	}
	basisPurpose := "Readable human basis for explicit profile_prepare scopes. " +
		"When present, a non-empty scopes array is also required. The server " +
		"rejects basis for status and memory_prepare."
	basis := onboardExactTextSchema(
		onboarding.MaximumProfileBasisBytes,
		basisPurpose,
	)
	properties := map[string]interface{}{
		"action": action,
		"scopes": scopes,
		"basis":  basis,
	}
	schema := objectMCPSchemaWithRequired(
		properties,
		[]string{"action"},
	)
	schema["description"] = "Read setup status or prepare a non-binding review. " +
		"Prepare actions never apply a profile, enable structured memory, or " +
		"grant authority. Closed status values are needs_init, needs_profile, " +
		"profile_review_ready, needs_memory, memory_review_ready, " +
		"memory_deferred, and ready. Closed result values are " +
		"onboarding_required, needs_profile, needs_scope_review, " +
		"profile_review_ready, profile_review_prepared, profile_review_reused, " +
		"needs_memory, memory_review_ready, memory_review_prepared, " +
		"memory_review_reused, memory_deferred, restart_required, ready, and " +
		"blocked. The only enablement choices are Enable structured project " +
		"memory and Not now. Every response reports repository_inspected, " +
		"review_carrier_created, review_carrier_reused, " +
		"canonical_profile_changed, structured_memory_enabled, and " +
		"authority_granted effects."
	return schema
}

func onboardScopeSchema() map[string]interface{} {
	scopeID := onboardExactTextSchema(
		onboarding.MaximumScopeIDBytes,
		"Stable readable scope identity.",
	)
	label := onboardExactTextSchema(
		onboarding.MaximumScopeLabelBytes,
		"Readable label for the project scope.",
	)
	kind := stringEnumSchema(
		"software",
		"non_software",
	)
	evidencePath := onboardExactTextSchema(
		onboarding.MaximumEvidencePathBytes,
		"Repository-relative path supporting the scope classification.",
	)
	evidencePaths := map[string]interface{}{
		"type":        "array",
		"maxItems":    onboarding.MaximumEvidencePaths,
		"items":       evidencePath,
		"uniqueItems": true,
		"description": "Repository paths supporting this scope. May be empty " +
			"when the readable top-level basis explains an empty repository. " +
			"Duplicate paths are rejected.",
	}
	properties := map[string]interface{}{
		"scope_id":         scopeID,
		"label":            label,
		"realization_kind": kind,
		"evidence_paths":   evidencePaths,
	}
	return objectMCPSchemaWithRequired(
		properties,
		[]string{
			"scope_id",
			"label",
			"realization_kind",
			"evidence_paths",
		},
	)
}

func onboardExactTextSchema(
	maximumBytes int,
	purpose string,
) map[string]interface{} {
	limit := fmt.Sprintf(
		" Exact non-empty text without surrounding whitespace or NUL; "+
			"the server enforces at most %d UTF-8 bytes.",
		maximumBytes,
	)
	return map[string]interface{}{
		"type":        "string",
		"minLength":   1,
		"maxLength":   maximumBytes,
		"description": purpose + limit,
	}
}
