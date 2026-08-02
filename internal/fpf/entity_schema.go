package fpf

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemory/entitycontract"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func haftEntityTool() Tool {
	return Tool{
		Name: "haft_entity",
		Description: "Establish one non-binding EntityOfConcern and its aliases. " +
			"The server derives and validates all internal memory coordinates; " +
			"the caller supplies only task-level identity and persistence provenance. " +
			"A concrete receiving use may be operator-named or agent-inferred from current Work.",
		InputSchema: entityEstablishmentRequestSchema(),
	}
}

func entityEstablishmentRequestSchema() map[string]interface{} {
	action := stringLiteralSchema(
		entitycontract.ActionEstablish,
	)
	action["description"] = "Establish identity after an explicit save request or an agent-inferred receiving use makes stable identity necessary. known_absent alone never invokes this action."
	entityID := entityExactTextSchema(
		typedmemorywire.MaximumIdentifierBytes,
		"Stable identity for the existing EntityOfConcern.",
	)
	label := entityExactTextSchema(
		typedmemorywire.MaximumTextBytes,
		"Readable label for the same EntityOfConcern.",
	)
	boundedContext := entityExactTextSchema(
		typedmemorywire.MaximumIdentifierBytes,
		"Exact bounded context in which this identity and its aliases resolve.",
	)
	alias := entityExactTextSchema(
		typedmemorywire.MaximumIdentifierBytes,
		"Canonical alias for the same EntityOfConcern in the exact bounded context.",
	)
	aliases := map[string]interface{}{
		"type":        "array",
		"maxItems":    entitycontract.MaximumAliases,
		"items":       alias,
		"uniqueItems": true,
		"description": "Canonical aliases in strictly increasing bytewise order. Identity and every alias commit atomically; any conflict writes nothing.",
	}
	persistenceReason := stringEnumSchema(
		entitycontract.ExplicitOperatorRequest,
		entitycontract.NamedReceivingUse,
	)
	persistenceReason["description"] = "Why persistence is justified: an explicit operator save request or an exact concrete receiving use, including an agent-inferred receiving use from current Work."
	requestProvenance := entityExactTextSchema(
		typedmemorywire.MaximumIdentifierBytes,
		"Addressable provenance for the operator request or the concrete operator-named or agent-inferred receiving use.",
	)
	idempotencyPurpose := "Stable key for this exact request. On retry or an " +
		"unknown commit outcome, resend the unchanged request and the same key."
	idempotencyKey := entityExactTextSchema(
		typedmemorywire.MaximumAdmissionIdempotencyKeyBytes,
		idempotencyPurpose,
	)
	schema := objectMCPSchemaWithRequired(
		map[string]interface{}{
			"action":                 action,
			"entity_id":              entityID,
			"label":                  label,
			"bounded_context_ref":    boundedContext,
			"aliases":                aliases,
			"persistence_reason":     persistenceReason,
			"request_provenance_ref": requestProvenance,
			"idempotency_key":        idempotencyKey,
		},
		[]string{
			"action",
			"entity_id",
			"label",
			"bounded_context_ref",
			"aliases",
			"persistence_reason",
			"request_provenance_ref",
			"idempotency_key",
		},
	)
	schema["description"] = "Atomically establish one EntityOfConcern identity " +
		"and its aliases. Closed results are onboarding_required, " +
		"restart_required, established, " +
		"identity_conflict, alias_conflict, idempotency_conflict, rejected, " +
		"and commit_outcome_unknown. Every response reports persistence.performed " +
		"and persistence.authority_granted. An agent-inferred receiving use may justify " +
		"this minimal non-binding persistence without a separate permission prompt; " +
		"establishment never grants decision, " +
		"specification, commission, or evidence authority. established returns " +
		"an exact U.EntityRef payload accepted verbatim by project-memory reads."
	return schema
}

func entityExactTextSchema(
	maximumBytes int,
	purpose string,
) map[string]interface{} {
	limit := fmt.Sprintf(
		" Exact non-empty single-line text without surrounding whitespace; "+
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
