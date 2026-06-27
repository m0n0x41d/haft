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

func normalizeCarryThroughItems(items []CarryThroughItem, defaultPending bool) []CarryThroughItem {
	normalized := make([]CarryThroughItem, 0, len(items))
	for _, item := range items {
		item.SourceRef = strings.TrimSpace(item.SourceRef)
		item.SourceItemRef = strings.TrimSpace(item.SourceItemRef)
		item.AcceptanceRef = strings.TrimSpace(item.AcceptanceRef)
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

func validatePullCarryThrough(items []CarryThroughItem) error {
	var problems []string
	for index, item := range items {
		problems = append(problems, validateCarryThroughIdentity(index, item)...)
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
		disposition, ok := closed[carryThroughKey(item)]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s needs carry_through close disposition", carryThroughKey(item)))
			continue
		}
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

func carryThroughKey(item CarryThroughItem) string {
	return item.SourceRef + "#" + item.SourceItemRef
}
