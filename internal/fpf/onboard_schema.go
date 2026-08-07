package fpf

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/onboarding"
)

const (
	haftOnboardStatusAction               = "status"
	haftOnboardProfilePrepareAction       = "profile_prepare"
	haftOnboardProfileChangePrepareAction = "profile_change_prepare"
)

func haftOnboardTool() Tool {
	schema := onboardRequestSchema()
	return Tool{
		Name: "haft_onboard",
		Description: "Inspect readable Haft project-setup status, prepare an initial profile review, or prepare one bounded predecessor-pinned scope entity relation change. " +
			"Project memory is initialized automatically by haft init, which may also admit only a complete supported singleton as origin=detector_default. Status and detection are read-only; preparation may materialize or reuse only a non-binding review carrier and never applies it. Initial profile_prepare may supersede only detector_default. profile_change_prepare can express only one existing scope entity_ref successor and its apply remains human-gated and CAS-pinned.",
		InputSchema: schema,
	}
}

func onboardRequestSchema() map[string]interface{} {
	action := stringEnumSchema(
		haftOnboardStatusAction,
		haftOnboardProfilePrepareAction,
		haftOnboardProfileChangePrepareAction,
	)
	action["description"] = "status never writes and accepts only action. " +
		"profile_prepare accepts action alone for advisory repository detection, " +
		"or action plus both basis and non-empty scopes for an explicit fallback. " +
		"It may materialize or reuse only a non-binding review carrier. " +
		"profile_change_prepare requires scope_id and entity_ref and prepares a CAS-pinned non-binding successor review; it never applies the change."
	scope := onboardScopeSchema()
	scopes := map[string]interface{}{
		"type":        "array",
		"minItems":    1,
		"maxItems":    onboarding.MaximumProfileScopes,
		"items":       scope,
		"uniqueItems": true,
		"description": "Optional explicit fallback scopes for profile_prepare. " +
			"When present, basis is also required and scope_id values must be " +
			"unique. The server rejects scopes for status.",
	}
	basisPurpose := "Readable human basis for explicit profile_prepare scopes. " +
		"When present, a non-empty scopes array is also required. The server " +
		"rejects basis for status."
	basis := onboardExactTextSchema(
		onboarding.MaximumProfileBasisBytes,
		basisPurpose,
	)
	changeScopeID := onboardExactTextSchema(
		onboarding.MaximumScopeIDBytes,
		"Exact existing scope_id for profile_change_prepare. The server rejects it for every other action.",
	)
	entityRef := onboardExactTextSchema(
		onboarding.MaximumEntityRefBytes,
		"Selected successor entity_ref for profile_change_prepare. The server rejects it for every other action.",
	)
	properties := map[string]interface{}{
		"action":     action,
		"scopes":     scopes,
		"basis":      basis,
		"scope_id":   changeScopeID,
		"entity_ref": entityRef,
	}
	schema := objectMCPSchemaWithRequired(
		properties,
		[]string{"action"},
	)
	schema["description"] = "Read project-setup status or prepare a non-binding project-profile review. " +
		"Project memory is initialized automatically by haft init; an eligible complete supported singleton is admitted as origin=detector_default, while ambiguous or reviewed bases remain operator-mediated profile-review work. profile_prepare " +
		"never applies a profile or grants authority. profile_change_prepare is a separate bounded successor-review action for an already-declared profile. Closed status values include " +
		"needs_init, needs_profile, profile_review_ready, profile_change_review_ready, and ready. Closed result values include " +
		"onboarding_required, needs_profile, needs_scope_review, " +
		"profile_review_ready, profile_review_prepared, profile_review_reused, " +
		"profile_change_review_prepared, profile_change_review_reused, restart_required, ready, and blocked. Every response reports repository_inspected, " +
		"review_carrier_created, review_carrier_reused, " +
		"canonical_profile_changed, structured_memory_enabled, and " +
		"authority_granted effects plus state_domain, ready_for, and does_not_establish. ready means only canonical profile plus structured project memory; it never establishes specification applicability, health, lifecycle, or release readiness. Status also reports profile_origin when declared, profile_override_eligible only for detector_default, and automatic_bootstrap_eligible when haft init --core-only is the recovery."
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
