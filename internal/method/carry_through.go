package method

import (
	"fmt"
	"strings"
)

const (
	CarryThroughGateID = "carry_through_disposition_recorded"

	CarryDispositionPending    = "pending"
	CarryDispositionApplied    = "applied"
	CarryDispositionRejected   = "rejected"
	CarryDispositionDeferred   = "deferred"
	CarryDispositionSuperseded = "superseded"
)

const (
	CarryAcceptanceKindOperatorMessage    = "operator_message"
	CarryAcceptanceKindReviewDisposition  = "review_disposition"
	CarryAcceptanceKindDecisionRecord     = "decision_record"
	CarryAcceptanceKindManualCLIReceipt   = "manual_cli_receipt"
	CarryAcceptanceKindExternalUnverified = "external_unverified"
	CarryAcceptanceKindUnknown            = "unknown"
)

const (
	CarryAcceptanceStatusVerified           = "verified"
	CarryAcceptanceStatusExternallyAsserted = "externally_asserted"
	CarryAcceptanceStatusMissing            = "missing"
	CarryAcceptanceStatusMalformed          = "malformed"
)

type CarryThroughAcceptancePosture struct {
	Kind   string `json:"acceptance_ref_kind"`
	Status string `json:"acceptance_ref_status"`
}

func normalizeCarryThroughItems(items []CarryThroughItem, defaultPending bool) []CarryThroughItem {
	normalized := make([]CarryThroughItem, 0, len(items))
	for _, item := range items {
		item.SourceRef = strings.TrimSpace(item.SourceRef)
		item.SourceItemRef = strings.TrimSpace(item.SourceItemRef)
		item.AcceptanceRef = strings.TrimSpace(item.AcceptanceRef)
		item.AcceptanceRefKind = normalizeToken(item.AcceptanceRefKind)
		item.AcceptanceRefStatus = normalizeToken(item.AcceptanceRefStatus)
		posture := InferCarryThroughAcceptancePosture(item.AcceptanceRef)
		if item.AcceptanceRefKind == "" {
			item.AcceptanceRefKind = posture.Kind
		}
		if item.AcceptanceRefStatus == "" {
			item.AcceptanceRefStatus = posture.Status
		}
		item.Disposition = normalizeCloseStatus(item.Disposition)
		item.TargetRefs = dedupeStrings(item.TargetRefs)
		item.Reason = strings.TrimSpace(item.Reason)
		item.EvidenceRefs = dedupeStrings(item.EvidenceRefs)
		item.UpdatedAt = strings.TrimSpace(item.UpdatedAt)
		if defaultPending && item.Disposition == "" {
			item.Disposition = CarryDispositionPending
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func NormalizeCarryThroughItem(item CarryThroughItem) CarryThroughItem {
	items := normalizeCarryThroughItems([]CarryThroughItem{item}, false)
	if len(items) == 0 {
		return CarryThroughItem{}
	}
	return items[0]
}

func InferCarryThroughAcceptancePosture(acceptanceRef string) CarryThroughAcceptancePosture {
	ref := strings.TrimSpace(acceptanceRef)
	if ref == "" {
		return CarryThroughAcceptancePosture{
			Kind:   CarryAcceptanceKindUnknown,
			Status: CarryAcceptanceStatusMissing,
		}
	}
	normalized := strings.ToLower(ref)
	normalized = strings.ReplaceAll(normalized, "-", "_")

	switch {
	case strings.HasPrefix(normalized, "operator_message:"):
		return CarryThroughAcceptancePosture{Kind: CarryAcceptanceKindOperatorMessage, Status: CarryAcceptanceStatusVerified}
	case strings.HasPrefix(normalized, "operator:"):
		return CarryThroughAcceptancePosture{Kind: CarryAcceptanceKindOperatorMessage, Status: CarryAcceptanceStatusExternallyAsserted}
	case strings.HasPrefix(normalized, "review_disposition:"):
		return CarryThroughAcceptancePosture{Kind: CarryAcceptanceKindReviewDisposition, Status: CarryAcceptanceStatusVerified}
	case strings.HasPrefix(normalized, "review:"):
		return CarryThroughAcceptancePosture{Kind: CarryAcceptanceKindReviewDisposition, Status: CarryAcceptanceStatusExternallyAsserted}
	case strings.HasPrefix(normalized, "external_review:"):
		return CarryThroughAcceptancePosture{Kind: CarryAcceptanceKindReviewDisposition, Status: CarryAcceptanceStatusExternallyAsserted}
	case strings.HasPrefix(normalized, "decision:"):
		return CarryThroughAcceptancePosture{Kind: CarryAcceptanceKindDecisionRecord, Status: CarryAcceptanceStatusVerified}
	case strings.HasPrefix(normalized, "dec_"):
		return CarryThroughAcceptancePosture{Kind: CarryAcceptanceKindDecisionRecord, Status: CarryAcceptanceStatusVerified}
	case strings.HasPrefix(normalized, "manual_cli:"):
		return CarryThroughAcceptancePosture{Kind: CarryAcceptanceKindManualCLIReceipt, Status: CarryAcceptanceStatusVerified}
	case strings.HasPrefix(normalized, "external:"):
		return CarryThroughAcceptancePosture{Kind: CarryAcceptanceKindExternalUnverified, Status: CarryAcceptanceStatusExternallyAsserted}
	default:
		return CarryThroughAcceptancePosture{
			Kind:   CarryAcceptanceKindUnknown,
			Status: CarryAcceptanceStatusMalformed,
		}
	}
}

func validatePullCarryThrough(items []CarryThroughItem) error {
	var problems []string
	for index, item := range items {
		problems = append(problems, validateCarryThroughIdentity(index, item)...)
		problems = append(problems, validateCarryThroughAcceptancePosture(index, item)...)
		if !carryThroughDispositionSupported(item.Disposition) {
			problems = append(problems, fmt.Sprintf("carry_through[%d] disposition must be pending, applied, rejected, deferred, or superseded", index))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("method pull carry_through invalid: %s", strings.Join(problems, "; "))
	}
	return nil
}

func validateCloseCarryThrough(run MethodRun, input CloseInput) []string {
	if len(run.CarryThrough) == 0 {
		return nil
	}
	if closeInputHasWaiver(input, CarryThroughGateID) {
		return nil
	}

	closed := map[string]CarryThroughItem{}
	for _, item := range normalizeCarryThroughItems(input.CarryThrough, false) {
		closed[carryThroughKey(item)] = item
	}

	var problems []string
	for index, item := range run.CarryThrough {
		problems = append(problems, validateCarryThroughIdentity(index, item)...)
		problems = append(problems, validateCarryThroughAcceptancePosture(index, item)...)
		disposition, ok := closed[carryThroughKey(item)]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s needs carry_through close disposition", carryThroughKey(item)))
			continue
		}
		problems = append(problems, validateCarryThroughAcceptancePosture(index, disposition)...)
		problems = append(problems, validateCarryThroughDisposition(disposition)...)
	}
	return problems
}

func closeoutCarryThroughItems(items []CarryThroughItem, closedAt string) []CarryThroughItem {
	closed := normalizeCarryThroughItems(items, false)
	for index := range closed {
		if closed[index].UpdatedAt == "" {
			closed[index].UpdatedAt = closedAt
		}
	}
	return closed
}

func validateCarryThroughIdentity(index int, item CarryThroughItem) []string {
	var problems []string
	if item.SourceRef == "" {
		problems = append(problems, fmt.Sprintf("carry_through[%d] missing source_ref", index))
	}
	if item.SourceItemRef == "" {
		problems = append(problems, fmt.Sprintf("carry_through[%d] missing source_item_ref", index))
	}
	if item.AcceptanceRef == "" {
		problems = append(problems, fmt.Sprintf("carry_through[%d] missing acceptance_ref", index))
	}
	return problems
}

func validateCarryThroughAcceptancePosture(index int, item CarryThroughItem) []string {
	item = NormalizeCarryThroughItem(item)
	var problems []string
	if !carryThroughAcceptanceKindSupported(item.AcceptanceRefKind) {
		problems = append(problems, fmt.Sprintf("carry_through[%d] unsupported acceptance_ref_kind %q", index, item.AcceptanceRefKind))
	}
	if !carryThroughAcceptanceStatusSupported(item.AcceptanceRefStatus) {
		problems = append(problems, fmt.Sprintf("carry_through[%d] unsupported acceptance_ref_status %q", index, item.AcceptanceRefStatus))
	}
	switch item.AcceptanceRefStatus {
	case CarryAcceptanceStatusMissing:
		problems = append(problems, fmt.Sprintf("carry_through[%d] missing acceptance_ref", index))
	case CarryAcceptanceStatusMalformed:
		problems = append(problems, fmt.Sprintf("carry_through[%d] malformed acceptance_ref %q; use operator_message:, review_disposition:, decision:, dec-, manual_cli:, external:, review:, or operator: prefix", index, item.AcceptanceRef))
	}
	if item.AcceptanceRefKind == CarryAcceptanceKindExternalUnverified && item.AcceptanceRefStatus == CarryAcceptanceStatusVerified {
		problems = append(problems, fmt.Sprintf("carry_through[%d] external_unverified acceptance_ref cannot be marked verified", index))
	}
	return problems
}

func validateCarryThroughDisposition(item CarryThroughItem) []string {
	var problems []string
	if !carryThroughDispositionSupported(item.Disposition) {
		problems = append(problems, carryThroughKey(item)+" disposition must be applied, rejected, deferred, superseded, or waived")
		return problems
	}
	switch item.Disposition {
	case CarryDispositionApplied:
		if len(item.TargetRefs) == 0 {
			problems = append(problems, carryThroughKey(item)+" applied disposition needs target_refs")
		}
	case CarryDispositionRejected, CarryDispositionDeferred, CarryDispositionSuperseded:
		if item.Reason == "" {
			problems = append(problems, carryThroughKey(item)+" "+item.Disposition+" disposition needs reason")
		}
	case CarryDispositionPending, "":
		problems = append(problems, carryThroughKey(item)+" remains pending")
	}
	return problems
}

func carryThroughDispositionSupported(disposition string) bool {
	switch disposition {
	case "", CarryDispositionPending, CarryDispositionApplied, CarryDispositionRejected, CarryDispositionDeferred, CarryDispositionSuperseded:
		return true
	default:
		return false
	}
}

func carryThroughAcceptanceKindSupported(kind string) bool {
	switch kind {
	case CarryAcceptanceKindOperatorMessage,
		CarryAcceptanceKindReviewDisposition,
		CarryAcceptanceKindDecisionRecord,
		CarryAcceptanceKindManualCLIReceipt,
		CarryAcceptanceKindExternalUnverified,
		CarryAcceptanceKindUnknown:
		return true
	default:
		return false
	}
}

func carryThroughAcceptanceStatusSupported(status string) bool {
	switch status {
	case CarryAcceptanceStatusVerified,
		CarryAcceptanceStatusExternallyAsserted,
		CarryAcceptanceStatusMissing,
		CarryAcceptanceStatusMalformed:
		return true
	default:
		return false
	}
}

func carryThroughKey(item CarryThroughItem) string {
	return item.SourceRef + "#" + item.SourceItemRef
}
