package artifact

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type DriftEventReport struct {
	SchemaVersion               int               `json:"schema_version"`
	View                        string            `json:"view,omitempty"`
	Summary                     DriftEventSummary `json:"summary"`
	Events                      []DriftEvent      `json:"events"`
	OmittedEvents               int               `json:"omitted_events,omitempty"`
	OmittedCompatibilityReports int               `json:"omitted_compatibility_reports,omitempty"`
	FullAuditCommand            string            `json:"full_audit_command,omitempty"`
	Compatibility               []DriftReport     `json:"compatibility_reports,omitempty"`
}

type DriftEventSummary struct {
	UniqueEvents                 int `json:"unique_events"`
	ImpactedDecisions            int `json:"impacted_decisions"`
	MaterialEvents               int `json:"material_events"`
	AuditOnlyEvents              int `json:"audit_only_events"`
	NeedsBindingResolutionEvents int `json:"needs_binding_resolution_events"`
	SemanticTargetEvents         int `json:"semantic_target_events"`
	FileFallbackEvents           int `json:"file_fallback_events"`
	UnknownHighRiskEvents        int `json:"unknown_high_risk_events"`
	ResolvedByLedgerEvents       int `json:"resolved_by_ledger_events"`
	WaivedByLedgerEvents         int `json:"waived_by_ledger_events"`
	MaxFanout                    int `json:"max_fanout"`
}

type DriftEvent struct {
	EventID                  string                 `json:"event_id"`
	ChangeRef                string                 `json:"change_ref"`
	ChangedTargetRef         string                 `json:"changed_target_ref"`
	TargetKind               string                 `json:"target_kind"`
	FilePath                 string                 `json:"file_path"`
	SymbolName               string                 `json:"symbol_name,omitempty"`
	SymbolKind               string                 `json:"symbol_kind,omitempty"`
	DriftStatus              DriftStatus            `json:"drift_status"`
	SymbolStatus             string                 `json:"symbol_status,omitempty"`
	TargetStatus             string                 `json:"target_status,omitempty"`
	TriggerKind              DriftTriggerKind       `json:"trigger_kind,omitempty"`
	Materiality              DriftMateriality       `json:"materiality,omitempty"`
	FallbackKind             string                 `json:"fallback_kind,omitempty"`
	FallbackReason           string                 `json:"fallback_reason,omitempty"`
	AuditOnly                bool                   `json:"audit_only"`
	Fanout                   int                    `json:"fanout"`
	ImpactedDecisions        []DriftEventDecision   `json:"impacted_decisions"`
	OmittedImpactedDecisions int                    `json:"omitted_impacted_decisions,omitempty"`
	RootCause                string                 `json:"root_cause"`
	RootCauseDetail          string                 `json:"root_cause_detail,omitempty"`
	ResolutionStatus         string                 `json:"resolution_status"`
	ResolutionRecordPosture  string                 `json:"resolution_record_posture,omitempty"`
	ResolutionRecord         *DriftEventResolution  `json:"resolution_record,omitempty"`
	SuggestedNextCommand     string                 `json:"suggested_next_command,omitempty"`
	SourceItems              []DriftEventSourceItem `json:"source_items,omitempty"`
	OmittedSourceItems       int                    `json:"omitted_source_items,omitempty"`
}

type DriftEventDecision struct {
	DecisionID    string `json:"decision_id"`
	DecisionTitle string `json:"decision_title,omitempty"`
}

type DriftEventSourceItem struct {
	DecisionID       string            `json:"decision_id"`
	Path             string            `json:"path"`
	ChangedTargetRef string            `json:"changed_target_ref,omitempty"`
	TargetKind       string            `json:"target_kind,omitempty"`
	TargetStatus     string            `json:"target_status,omitempty"`
	Status           DriftStatus       `json:"status"`
	TriggerKind      DriftTriggerKind  `json:"trigger_kind,omitempty"`
	Materiality      DriftMateriality  `json:"materiality,omitempty"`
	FallbackKind     string            `json:"fallback_kind,omitempty"`
	FallbackReason   string            `json:"fallback_reason,omitempty"`
	AuditOnly        bool              `json:"audit_only,omitempty"`
	SuppressedReason string            `json:"suppressed_reason,omitempty"`
	LinesChanged     string            `json:"lines_changed,omitempty"`
	Invariants       []string          `json:"invariants,omitempty"`
	ClaimRefs        []string          `json:"claim_refs,omitempty"`
	EvidenceRefs     []string          `json:"evidence_refs,omitempty"`
	Symbols          []SymbolDriftItem `json:"symbols,omitempty"`
}

const (
	DriftEventRootCauseSemanticTargetChanged    = "semantic_target_changed"
	DriftEventRootCauseCarrierOnlyChanged       = "carrier_only_changed"
	DriftEventRootCauseGeneratedArtifactChanged = "generated_artifact_changed"
	DriftEventRootCauseBindingTargetMissing     = "binding_target_missing"
	DriftEventRootCauseTargetDeleted            = "target_deleted"
	DriftEventRootCauseTargetRenamed            = "target_renamed"
	DriftEventRootCauseRetargetCandidate        = "retarget_candidate"
	DriftEventRootCauseImplementationFootprint  = "implementation_footprint_churn"
	DriftEventRootCauseSchemaChanged            = "schema_changed"
	DriftEventRootCauseUnknownHighRisk          = "unknown_high_risk"
)

const (
	DriftEventResolutionOpen                  = "open"
	DriftEventResolutionNeedsScopeEnrichment  = "needs_scope_enrichment"
	DriftEventResolutionNeedsRebaseline       = "needs_rebaseline"
	DriftEventResolutionNeedsReconcile        = "needs_reconcile"
	DriftEventResolutionNeedsOperatorJudgment = "needs_operator_judgment"
	DriftEventResolutionResolved              = "resolved"
	DriftEventResolutionWaivedUntil           = "waived_until"
)

const (
	DriftEventResolutionRecordPostureApplied           = "applied"
	DriftEventResolutionRecordPostureStaleEventBinding = "stale_event_binding"
	DriftEventResolutionRecordPostureInactiveWaiver    = "inactive_waiver"
	DriftEventResolutionRecordPostureUnsupportedStatus = "unsupported_status"
)

const DriftEventResolutionLedgerAuthority = "drift_event_resolution_metadata_not_decision_authority"

const legacyFileScopeFallbackReason = "legacy file-scope decision binding has no precise symbol, section, or contract target"

type DriftEventResolutionLedger struct {
	SchemaVersion int                    `json:"schema_version"`
	Authority     string                 `json:"authority"`
	Records       []DriftEventResolution `json:"records"`
}

type DriftEventResolution struct {
	EventID          string   `json:"event_id"`
	Status           string   `json:"status"`
	Reason           string   `json:"reason"`
	EvidenceRefs     []string `json:"evidence_refs,omitempty"`
	WaiverExpiresAt  string   `json:"waiver_expires_at,omitempty"`
	RecordedAt       string   `json:"recorded_at,omitempty"`
	RecordedBy       string   `json:"recorded_by,omitempty"`
	ChangedTargetRef string   `json:"changed_target_ref,omitempty"`
	TargetKind       string   `json:"target_kind,omitempty"`
	TargetStatus     string   `json:"target_status,omitempty"`
	Materiality      string   `json:"materiality,omitempty"`
	AuditOnly        *bool    `json:"audit_only,omitempty"`
	RootCause        string   `json:"root_cause,omitempty"`
}

type driftEventBuilder struct {
	event     DriftEvent
	decisions map[string]DriftEventDecision
}

func NewDriftEventResolutionLedger(records []DriftEventResolution) DriftEventResolutionLedger {
	return DriftEventResolutionLedger{
		SchemaVersion: 1,
		Authority:     DriftEventResolutionLedgerAuthority,
		Records:       append([]DriftEventResolution(nil), records...),
	}
}

func UpsertDriftEventResolution(
	ledger DriftEventResolutionLedger,
	record DriftEventResolution,
	now time.Time,
) (DriftEventResolutionLedger, error) {
	if ledger.SchemaVersion == 0 {
		ledger.SchemaVersion = 1
	}
	if strings.TrimSpace(ledger.Authority) == "" {
		ledger.Authority = DriftEventResolutionLedgerAuthority
	}
	if err := ValidateDriftEventResolution(record, now); err != nil {
		return DriftEventResolutionLedger{}, err
	}

	updated := NewDriftEventResolutionLedger(ledger.Records)
	for index, existing := range updated.Records {
		if existing.EventID == record.EventID {
			updated.Records[index] = record
			return updated, nil
		}
	}
	updated.Records = append(updated.Records, record)
	sort.Slice(updated.Records, func(i, j int) bool {
		return updated.Records[i].EventID < updated.Records[j].EventID
	})
	return updated, nil
}

func ValidateDriftEventResolution(record DriftEventResolution, now time.Time) error {
	if strings.TrimSpace(record.EventID) == "" {
		return errors.New("event_id is required")
	}
	switch strings.TrimSpace(record.Status) {
	case DriftEventResolutionResolved:
	case DriftEventResolutionWaivedUntil:
		if _, err := ParseDriftEventResolutionTime(record.WaiverExpiresAt, now); err != nil {
			return fmt.Errorf("waiver_expires_at is required for waived_until: %w", err)
		}
	default:
		return fmt.Errorf("status must be %s or %s", DriftEventResolutionResolved, DriftEventResolutionWaivedUntil)
	}
	if strings.TrimSpace(record.Reason) == "" {
		return errors.New("reason is required")
	}
	return nil
}

func ApplyDriftEventResolutionLedger(
	report DriftEventReport,
	ledger DriftEventResolutionLedger,
	now time.Time,
) DriftEventReport {
	report.Events = cloneDriftEvents(report.Events)
	records := map[string]DriftEventResolution{}
	for _, record := range ledger.Records {
		records[record.EventID] = record
	}

	for index, event := range report.Events {
		record, ok := records[event.EventID]
		if !ok {
			continue
		}
		report.Events[index].ResolutionRecord = &record
		posture := driftEventResolutionRecordPosture(record, event, now)
		report.Events[index].ResolutionRecordPosture = posture
		if posture != DriftEventResolutionRecordPostureApplied {
			continue
		}
		report.Events[index].ResolutionStatus = record.Status
		report.Events[index].SuggestedNextCommand = ""
	}
	report.Summary = summarizeDriftEvents(report.Events)
	return report
}

func CompactDriftEventReport(report DriftEventReport, eventLimit int) DriftEventReport {
	compact := report
	compact.View = "compact"
	compact.FullAuditCommand = `haft_query(action="drift_events", full=true)`
	compact.OmittedCompatibilityReports = len(report.Compatibility)
	compact.Compatibility = nil

	events := cloneDriftEvents(report.Events)
	if eventLimit > 0 && len(events) > eventLimit {
		compact.OmittedEvents = len(events) - eventLimit
		events = events[:eventLimit]
	}
	for index, event := range events {
		if eventLimit > 0 && len(event.ImpactedDecisions) > eventLimit {
			events[index].OmittedImpactedDecisions = len(event.ImpactedDecisions) - eventLimit
			events[index].ImpactedDecisions = append([]DriftEventDecision(nil), event.ImpactedDecisions[:eventLimit]...)
		}
		if len(event.SourceItems) == 0 {
			continue
		}
		events[index].OmittedSourceItems = len(event.SourceItems)
		events[index].SourceItems = nil
	}
	compact.Events = events

	return compact
}

func cloneDriftEvents(events []DriftEvent) []DriftEvent {
	cloned := make([]DriftEvent, 0, len(events))
	for _, event := range events {
		event.ImpactedDecisions = append([]DriftEventDecision(nil), event.ImpactedDecisions...)
		event.SourceItems = append([]DriftEventSourceItem(nil), event.SourceItems...)
		if event.ResolutionRecord != nil {
			record := *event.ResolutionRecord
			event.ResolutionRecord = &record
		}
		cloned = append(cloned, event)
	}
	return cloned
}

func BuildDriftEventReport(reports []DriftReport) DriftEventReport {
	builders := map[string]*driftEventBuilder{}
	for _, report := range reports {
		for _, item := range report.Files {
			for _, candidate := range driftEventCandidates(report, item) {
				builder := builders[candidate.key]
				if builder == nil {
					event := candidate.event
					event.EventID = driftEventID(candidate.key)
					event.ChangeRef = "drift-change:" + candidate.key
					event.RootCause = driftEventRootCause(event)
					event.ResolutionStatus = driftEventResolutionStatus(event)
					event.SuggestedNextCommand = driftEventSuggestedNextCommand(event)
					builder = &driftEventBuilder{
						event:     event,
						decisions: map[string]DriftEventDecision{},
					}
					builders[candidate.key] = builder
				}
				builder.decisions[report.DecisionID] = DriftEventDecision{
					DecisionID:    report.DecisionID,
					DecisionTitle: report.DecisionTitle,
				}
				builder.event.AuditOnly = builder.event.AuditOnly && item.AuditOnly
				if builder.event.FallbackKind == "" {
					builder.event.FallbackKind = candidate.event.FallbackKind
				}
				if builder.event.FallbackReason == "" {
					builder.event.FallbackReason = candidate.event.FallbackReason
				}
				builder.event.RootCause = driftEventRootCause(builder.event)
				builder.event.RootCauseDetail = driftEventRootCauseDetail(builder.event)
				builder.event.SourceItems = append(builder.event.SourceItems, driftEventSourceItem(report.DecisionID, item, candidate.event))
				builder.event.ResolutionStatus = driftEventResolutionStatus(builder.event)
				builder.event.SuggestedNextCommand = driftEventSuggestedNextCommand(builder.event)
			}
		}
	}

	events := make([]DriftEvent, 0, len(builders))
	impacted := map[string]struct{}{}
	for _, builder := range builders {
		for _, decision := range builder.decisions {
			impacted[decision.DecisionID] = struct{}{}
			builder.event.ImpactedDecisions = append(builder.event.ImpactedDecisions, decision)
		}
		sort.Slice(builder.event.ImpactedDecisions, func(i, j int) bool {
			return builder.event.ImpactedDecisions[i].DecisionID < builder.event.ImpactedDecisions[j].DecisionID
		})
		sort.Slice(builder.event.SourceItems, func(i, j int) bool {
			left := builder.event.SourceItems[i]
			right := builder.event.SourceItems[j]
			if left.DecisionID != right.DecisionID {
				return left.DecisionID < right.DecisionID
			}
			return left.Path < right.Path
		})
		builder.event.Fanout = len(builder.event.ImpactedDecisions)
		events = append(events, builder.event)
	}
	sort.Slice(events, func(i, j int) bool {
		return driftEventLess(events[i], events[j])
	})

	summary := summarizeDriftEvents(events)
	summary.ImpactedDecisions = len(impacted)

	return DriftEventReport{
		SchemaVersion: 2,
		Summary:       summary,
		Events:        events,
		Compatibility: append([]DriftReport(nil), reports...),
	}
}

func summarizeDriftEvents(events []DriftEvent) DriftEventSummary {
	impacted := map[string]struct{}{}
	summary := DriftEventSummary{UniqueEvents: len(events)}
	for _, event := range events {
		for _, decision := range event.ImpactedDecisions {
			impacted[decision.DecisionID] = struct{}{}
		}
		if event.AuditOnly {
			summary.AuditOnlyEvents++
		} else {
			summary.MaterialEvents++
		}
		if driftEventNeedsBindingResolution(event) {
			summary.NeedsBindingResolutionEvents++
		}
		if event.TargetKind != "file" {
			summary.SemanticTargetEvents++
		}
		if driftEventUsesFileFallback(event) {
			summary.FileFallbackEvents++
		}
		if event.RootCause == DriftEventRootCauseUnknownHighRisk {
			summary.UnknownHighRiskEvents++
		}
		if event.ResolutionRecord != nil {
			switch event.ResolutionStatus {
			case DriftEventResolutionResolved:
				summary.ResolvedByLedgerEvents++
			case DriftEventResolutionWaivedUntil:
				summary.WaivedByLedgerEvents++
			}
		}
		if event.Fanout > summary.MaxFanout {
			summary.MaxFanout = event.Fanout
		}
	}
	summary.ImpactedDecisions = len(impacted)
	return summary
}

func BindDriftEventResolutionToEvent(
	record DriftEventResolution,
	event DriftEvent,
) DriftEventResolution {
	record.ChangedTargetRef = event.ChangedTargetRef
	record.TargetKind = event.TargetKind
	record.TargetStatus = event.TargetStatus
	record.Materiality = string(event.Materiality)
	record.AuditOnly = boolPtr(event.AuditOnly)
	record.RootCause = event.RootCause
	return record
}

func driftEventResolutionRecordPosture(
	record DriftEventResolution,
	event DriftEvent,
	now time.Time,
) string {
	if !driftEventResolutionMatchesEvent(record, event) {
		return DriftEventResolutionRecordPostureStaleEventBinding
	}
	switch record.Status {
	case DriftEventResolutionResolved:
		return DriftEventResolutionRecordPostureApplied
	case DriftEventResolutionWaivedUntil:
		expiresAt, err := ParseDriftEventResolutionTime(record.WaiverExpiresAt, now)
		if err != nil {
			return DriftEventResolutionRecordPostureInactiveWaiver
		}
		if now.After(expiresAt) {
			return DriftEventResolutionRecordPostureInactiveWaiver
		}
		return DriftEventResolutionRecordPostureApplied
	default:
		return DriftEventResolutionRecordPostureUnsupportedStatus
	}
}

func driftEventResolutionMatchesEvent(record DriftEventResolution, event DriftEvent) bool {
	return driftEventResolutionFieldMatches(record.ChangedTargetRef, event.ChangedTargetRef) &&
		driftEventResolutionFieldMatches(record.TargetKind, event.TargetKind) &&
		driftEventResolutionFieldMatches(record.TargetStatus, event.TargetStatus) &&
		driftEventResolutionFieldMatches(record.Materiality, string(event.Materiality)) &&
		driftEventResolutionBoolMatches(record.AuditOnly, event.AuditOnly) &&
		driftEventResolutionFieldMatches(record.RootCause, event.RootCause)
}

func driftEventResolutionFieldMatches(recordValue string, eventValue string) bool {
	recordValue = strings.TrimSpace(recordValue)
	if recordValue == "" {
		return true
	}
	return recordValue == strings.TrimSpace(eventValue)
}

func driftEventResolutionBoolMatches(recordValue *bool, eventValue bool) bool {
	if recordValue == nil {
		return true
	}
	return *recordValue == eventValue
}

func boolPtr(value bool) *bool {
	return &value
}

func ParseDriftEventResolutionTime(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("time value is empty")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, now.Location()), nil
}

type driftEventCandidate struct {
	key   string
	event DriftEvent
}

func driftEventCandidates(report DriftReport, item DriftItem) []driftEventCandidate {
	materiality := item.EffectiveMateriality()
	trigger := item.TriggerKind
	if trigger == "" {
		trigger = driftEventDefaultTrigger(item.Status)
	}
	fallbackKind, fallbackReason := driftEventFallbackMetadata(item, materiality)
	event := DriftEvent{
		ChangedTargetRef: driftEventFileTarget(item.Path),
		TargetKind:       "file",
		FilePath:         item.Path,
		DriftStatus:      item.Status,
		TriggerKind:      trigger,
		Materiality:      materiality,
		FallbackKind:     fallbackKind,
		FallbackReason:   fallbackReason,
		AuditOnly:        item.AuditOnly,
	}
	candidates := driftEventSymbolCandidates(event, item)
	if len(candidates) > 0 {
		return candidates
	}
	candidates = driftEventExplicitTargetCandidates(event, item)
	if len(candidates) > 0 {
		return candidates
	}
	_ = report
	return []driftEventCandidate{driftEventCandidateFromEvent(event)}
}

func driftEventExplicitTargetCandidates(fileEvent DriftEvent, item DriftItem) []driftEventCandidate {
	changedTargetRef := strings.TrimSpace(item.ChangedTargetRef)
	if changedTargetRef == "" {
		return nil
	}
	targetKind := strings.TrimSpace(item.TargetKind)
	if targetKind == "" {
		return nil
	}
	event := fileEvent
	event.ChangedTargetRef = changedTargetRef
	event.TargetKind = targetKind
	event.TargetStatus = strings.TrimSpace(item.TargetStatus)
	return []driftEventCandidate{driftEventCandidateFromEvent(event)}
}

func driftEventSymbolCandidates(fileEvent DriftEvent, item DriftItem) []driftEventCandidate {
	if item.EffectiveMateriality() != DriftMaterialityMaterialSymbol {
		return nil
	}
	candidates := make([]driftEventCandidate, 0, len(item.Symbols))
	for _, symbol := range item.Symbols {
		if strings.TrimSpace(symbol.SymbolName) == "" {
			continue
		}
		if symbol.Status == "added" {
			continue
		}
		event := fileEvent
		event.ChangedTargetRef = driftEventSymbolTarget(item.Path, symbol)
		event.TargetKind = "symbol"
		event.SymbolName = symbol.SymbolName
		event.SymbolKind = symbol.SymbolKind
		event.SymbolStatus = symbol.Status
		candidates = append(candidates, driftEventCandidateFromEvent(event))
	}
	return candidates
}

func driftEventCandidateFromEvent(event DriftEvent) driftEventCandidate {
	event.RootCause = driftEventRootCause(event)
	event.RootCauseDetail = driftEventRootCauseDetail(event)
	event.ResolutionStatus = driftEventResolutionStatus(event)
	event.SuggestedNextCommand = driftEventSuggestedNextCommand(event)
	return driftEventCandidate{
		key:   driftEventKey(event),
		event: event,
	}
}

func driftEventSourceItem(decisionID string, item DriftItem, event DriftEvent) DriftEventSourceItem {
	claimRefs, evidenceRefs := driftEventSourceItemRefs(item, event)
	fallbackKind, fallbackReason := driftEventSourceItemFallbackMetadata(item, event)
	return DriftEventSourceItem{
		DecisionID:       decisionID,
		Path:             item.Path,
		ChangedTargetRef: event.ChangedTargetRef,
		TargetKind:       event.TargetKind,
		TargetStatus:     event.TargetStatus,
		Status:           item.Status,
		TriggerKind:      item.TriggerKind,
		Materiality:      item.EffectiveMateriality(),
		FallbackKind:     fallbackKind,
		FallbackReason:   fallbackReason,
		AuditOnly:        item.AuditOnly,
		SuppressedReason: item.SuppressedReason,
		LinesChanged:     item.LinesChanged,
		Invariants:       append([]string(nil), item.Invariants...),
		ClaimRefs:        claimRefs,
		EvidenceRefs:     evidenceRefs,
		Symbols:          append([]SymbolDriftItem(nil), item.Symbols...),
	}
}

func driftEventSourceItemRefs(item DriftItem, event DriftEvent) ([]string, []string) {
	for _, symbol := range item.Symbols {
		if driftEventSymbolTarget(item.Path, symbol) != event.ChangedTargetRef {
			continue
		}
		return append([]string(nil), symbol.ClaimRefs...), append([]string(nil), symbol.EvidenceRefs...)
	}
	return append([]string(nil), item.ClaimRefs...), append([]string(nil), item.EvidenceRefs...)
}

func driftEventKey(event DriftEvent) string {
	parts := []string{
		event.TargetKind,
		event.ChangedTargetRef,
		string(event.DriftStatus),
		event.SymbolStatus,
		event.TargetStatus,
		string(event.TriggerKind),
		string(event.Materiality),
	}
	return strings.Join(parts, "|")
}

func driftEventID(key string) string {
	sum := sha1.Sum([]byte(key))
	return fmt.Sprintf("drift-event-%x", sum[:6])
}

func driftEventFileTarget(path string) string {
	return "file:" + path
}

func driftEventSymbolTarget(path string, symbol SymbolDriftItem) string {
	return fmt.Sprintf("symbol:%s::%s:%s", path, symbol.SymbolKind, symbol.SymbolName)
}

func driftEventDefaultTrigger(status DriftStatus) DriftTriggerKind {
	switch status {
	case DriftMissing:
		return DriftTriggerMissingFile
	case DriftNoBaseline:
		return DriftTriggerMissingBaseline
	case DriftAdded:
		return DriftTriggerScopeManifest
	case DriftModified:
		return DriftTriggerFileHash
	default:
		return ""
	}
}

func driftEventFallbackMetadata(
	item DriftItem,
	materiality DriftMateriality,
) (string, string) {
	fallbackKind := strings.TrimSpace(item.FallbackKind)
	fallbackReason := strings.TrimSpace(item.FallbackReason)
	if materiality != DriftMaterialityUnknownLegacyFileScope || fallbackKind != "" {
		return fallbackKind, fallbackReason
	}
	return BindingTargetWholeFileFallback, legacyFileScopeFallbackReason
}

func driftEventSourceItemFallbackMetadata(item DriftItem, event DriftEvent) (string, string) {
	fallbackKind := strings.TrimSpace(item.FallbackKind)
	fallbackReason := strings.TrimSpace(item.FallbackReason)
	if fallbackKind != "" {
		return fallbackKind, fallbackReason
	}
	return event.FallbackKind, event.FallbackReason
}

func driftEventRootCause(event DriftEvent) string {
	if event.TargetStatus == "retarget_candidate" {
		return DriftEventRootCauseRetargetCandidate
	}
	if event.FallbackKind != "" || event.Materiality == DriftMaterialityNeedsBindingResolution {
		return DriftEventRootCauseBindingTargetMissing
	}
	if event.TargetStatus == "renamed" {
		return DriftEventRootCauseTargetRenamed
	}
	if event.DriftStatus == DriftMissing || event.SymbolStatus == "removed" || event.TargetStatus == "removed" {
		return DriftEventRootCauseTargetDeleted
	}
	if event.TriggerKind == DriftTriggerScopeManifest && driftEventMaterialityImpliesSchemaChange(event.Materiality) {
		return DriftEventRootCauseSchemaChanged
	}
	switch event.Materiality {
	case DriftMaterialityCarrierOnly:
		return DriftEventRootCauseCarrierOnlyChanged
	case DriftMaterialityGeneratedOrIgnored:
		return DriftEventRootCauseGeneratedArtifactChanged
	case DriftMaterialityMaterialSymbol, DriftMaterialityMaterialSemanticTarget:
		if event.TargetKind != "file" {
			return DriftEventRootCauseSemanticTargetChanged
		}
		if event.Materiality == DriftMaterialityMaterialSemanticTarget {
			return DriftEventRootCauseSemanticTargetChanged
		}
		return DriftEventRootCauseUnknownHighRisk
	case DriftMaterialityAdjacentFileChurn:
		return DriftEventRootCauseImplementationFootprint
	case DriftMaterialityUnknownLegacyFileScope:
		return DriftEventRootCauseUnknownHighRisk
	}
	return DriftEventRootCauseUnknownHighRisk
}

func driftEventMaterialityImpliesSchemaChange(materiality DriftMateriality) bool {
	switch materiality {
	case DriftMaterialityMaterialSymbol, DriftMaterialityMaterialSemanticTarget, DriftMaterialityUnknownLegacyFileScope:
		return true
	default:
		return false
	}
}

func driftEventRootCauseDetail(event DriftEvent) string {
	if event.Materiality == DriftMaterialityUnknownLegacyFileScope && event.FallbackKind == BindingTargetWholeFileFallback {
		return fmt.Sprintf("%s drift on %s via %s; %s", event.Materiality, event.ChangedTargetRef, event.TriggerKind, event.FallbackReason)
	}
	return fmt.Sprintf("%s drift on %s via %s", event.Materiality, event.ChangedTargetRef, event.TriggerKind)
}

func driftEventResolutionStatus(event DriftEvent) string {
	if event.AuditOnly {
		return DriftEventResolutionResolved
	}
	if event.TargetStatus == "retarget_candidate" {
		return DriftEventResolutionNeedsOperatorJudgment
	}
	if event.DriftStatus == DriftNoBaseline {
		return DriftEventResolutionNeedsRebaseline
	}
	switch event.Materiality {
	case DriftMaterialityNeedsBindingResolution:
		return DriftEventResolutionNeedsScopeEnrichment
	case DriftMaterialityCarrierOnly, DriftMaterialityGeneratedOrIgnored, DriftMaterialityAdjacentFileChurn:
		return DriftEventResolutionResolved
	case DriftMaterialityUnknownLegacyFileScope:
		return DriftEventResolutionNeedsScopeEnrichment
	case DriftMaterialityMaterialSymbol, DriftMaterialityMaterialSemanticTarget:
		return DriftEventResolutionNeedsOperatorJudgment
	default:
		return DriftEventResolutionOpen
	}
}

func driftEventSuggestedNextCommand(event DriftEvent) string {
	switch event.ResolutionStatus {
	case DriftEventResolutionNeedsScopeEnrichment, DriftEventResolutionNeedsReconcile:
		return "haft decision reconcile --json"
	case DriftEventResolutionNeedsRebaseline:
		return `haft_refresh(action="scan", verbose=true)`
	case DriftEventResolutionNeedsOperatorJudgment:
		return `haft_refresh(action="review")`
	case DriftEventResolutionOpen:
		return "haft drift events --json"
	default:
		return ""
	}
}

func driftEventNeedsBindingResolution(event DriftEvent) bool {
	if event.Materiality == DriftMaterialityNeedsBindingResolution {
		return true
	}
	return event.Materiality == DriftMaterialityUnknownLegacyFileScope &&
		event.FallbackKind == BindingTargetWholeFileFallback
}

func driftEventUsesFileFallback(event DriftEvent) bool {
	if event.TargetKind != "file" {
		return false
	}
	if event.FallbackKind != "" {
		return true
	}
	switch event.Materiality {
	case DriftMaterialityNeedsBindingResolution, DriftMaterialityUnknownLegacyFileScope:
		return true
	default:
		return false
	}
}

func driftEventLess(left DriftEvent, right DriftEvent) bool {
	if left.AuditOnly != right.AuditOnly {
		return !left.AuditOnly
	}
	if left.Fanout != right.Fanout {
		return left.Fanout > right.Fanout
	}
	if left.Materiality != right.Materiality {
		return left.Materiality < right.Materiality
	}
	if left.ChangedTargetRef != right.ChangedTargetRef {
		return left.ChangedTargetRef < right.ChangedTargetRef
	}
	return left.EventID < right.EventID
}
