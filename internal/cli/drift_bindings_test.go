package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestDriftBindingsDryRunFlagIsAdvertised(t *testing.T) {
	flag := driftBindingsCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("drift bindings --dry-run flag is not registered")
	}
	if flag.DefValue != "false" {
		t.Fatalf("dry-run default = %q, want false", flag.DefValue)
	}
}

func TestDriftBindingsLimitFlagIsAdvertised(t *testing.T) {
	flag := driftBindingsCmd.Flags().Lookup("limit")
	if flag == nil {
		t.Fatal("drift bindings --limit flag is not registered")
	}
	if flag.DefValue != "-1" {
		t.Fatalf("limit default = %q, want -1", flag.DefValue)
	}
}

func TestValidateDriftBindingsModeRejectsDryRunWithMutation(t *testing.T) {
	restore := setDriftBindingsModeForTest(t, true, true, "")
	defer restore()

	err := validateDriftBindingsMode()
	if err == nil {
		t.Fatal("validateDriftBindingsMode accepted --dry-run with --apply-high-confidence")
	}
	if !strings.Contains(err.Error(), "--dry-run is read-only") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDriftBindingsModeRejectsDryRunWithSelection(t *testing.T) {
	restore := setDriftBindingsModeForTest(t, true, false, "selection.json")
	defer restore()

	err := validateDriftBindingsMode()
	if err == nil {
		t.Fatal("validateDriftBindingsMode accepted --dry-run with --apply-selection")
	}
	if !strings.Contains(err.Error(), "--dry-run is read-only") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDriftBindingsModeAllowsDryRunPreview(t *testing.T) {
	restore := setDriftBindingsModeForTest(t, true, false, "")
	defer restore()

	if err := validateDriftBindingsMode(); err != nil {
		t.Fatalf("validateDriftBindingsMode rejected read-only dry-run preview: %v", err)
	}
}

func TestDriftBindingsJSONPayloadDryRunDefaultsCompact(t *testing.T) {
	report := driftBindingsReportWithItems(7)

	payload, ok := driftBindingsJSONPayload(report, true, -1).(driftBindingsProjectedReport)
	if !ok {
		t.Fatalf("payload type = %T, want driftBindingsProjectedReport", driftBindingsJSONPayload(report, true, -1))
	}
	if payload.View != "compact" {
		t.Fatalf("view = %q, want compact", payload.View)
	}
	if len(payload.Items) != driftBindingsDryRunJSONLimit {
		t.Fatalf("items = %d, want %d", len(payload.Items), driftBindingsDryRunJSONLimit)
	}
	if payload.OmittedItems != 2 {
		t.Fatalf("omitted_items = %d, want 2", payload.OmittedItems)
	}
	if payload.FullAuditCommand != "haft drift bindings --json" {
		t.Fatalf("full_audit_command = %q", payload.FullAuditCommand)
	}
}

func TestDriftBindingsJSONPayloadKeepsFullAuditWithoutDryRun(t *testing.T) {
	report := driftBindingsReportWithItems(7)

	payload, ok := driftBindingsJSONPayload(report, false, -1).(artifact.LegacyBindingReport)
	if !ok {
		t.Fatalf("payload type = %T, want artifact.LegacyBindingReport", driftBindingsJSONPayload(report, false, -1))
	}
	if len(payload.Items) != 7 {
		t.Fatalf("items = %d, want full 7", len(payload.Items))
	}
}

func TestDriftBindingsJSONPayloadHonorsExplicitLimit(t *testing.T) {
	report := driftBindingsReportWithItems(3)

	payload, ok := driftBindingsJSONPayload(report, false, 1).(driftBindingsProjectedReport)
	if !ok {
		t.Fatalf("payload type = %T, want driftBindingsProjectedReport", driftBindingsJSONPayload(report, false, 1))
	}
	if len(payload.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(payload.Items))
	}
	if payload.OmittedItems != 2 {
		t.Fatalf("omitted_items = %d, want 2", payload.OmittedItems)
	}
}

func driftBindingsReportWithItems(count int) artifact.LegacyBindingReport {
	report := artifact.LegacyBindingReport{
		SchemaVersion: artifact.LegacyBindingSchemaVersion,
		Authority:     artifact.LegacyBindingAuthority,
		Summary:       artifact.LegacyBindingSummary{TotalDecisions: count},
		Items:         make([]artifact.LegacyBindingItem, 0, count),
	}
	for i := 0; i < count; i++ {
		report.Items = append(report.Items, artifact.LegacyBindingItem{
			DecisionID:    fmt.Sprintf("dec-%d", i),
			DecisionTitle: fmt.Sprintf("Decision %d", i),
		})
	}
	return report
}

func setDriftBindingsModeForTest(t *testing.T, dryRun bool, apply bool, selection string) func() {
	t.Helper()

	prevDryRun := driftBindingsDryRun
	prevApply := driftBindingsApply
	prevSelection := driftBindingsSelect

	driftBindingsDryRun = dryRun
	driftBindingsApply = apply
	driftBindingsSelect = selection

	return func() {
		driftBindingsDryRun = prevDryRun
		driftBindingsApply = prevApply
		driftBindingsSelect = prevSelection
	}
}

func TestWriteDriftBindingsSummaryNamesActions(t *testing.T) {
	report := artifact.LegacyBindingReport{
		Authority: artifact.LegacyBindingAuthority,
		Summary: artifact.LegacyBindingSummary{
			TotalDecisions:          2,
			MissingBindingTargets:   1,
			HighConfidenceProposals: 1,
			NeedsOperatorSelection:  1,
			AmbiguousFileScope:      1,
			AlreadyPrecise:          0,
		},
		Items: []artifact.LegacyBindingItem{
			{
				DecisionID:           "dec-one",
				DecisionTitle:        "One symbol",
				Posture:              artifact.LegacyBindingPostureMissingSymbolBaseline,
				RecommendedAction:    artifact.LegacyBindingActionProposeRebaseline,
				CandidateSymbolCount: 1,
			},
		},
	}

	var buf bytes.Buffer
	if err := writeDriftBindingsSummary(&buf, report); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	for _, want := range []string{
		"haft drift bindings",
		"binding_target_review_proposal",
		"high_confidence=1",
		"missing_bindings=1",
		"applied=0",
		"One symbol `dec-one`",
		"action=propose_rebaseline_with_binding_targets",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("summary missing %q:\n%s", want, output)
		}
	}
}

func TestReadDriftEventResolutionLedgerTreatsEmptyFileAsEmptyLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty-ledger.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty ledger: %v", err)
	}

	ledger, err := readDriftEventResolutionLedger(path)
	if err != nil {
		t.Fatalf("readDriftEventResolutionLedger: %v", err)
	}
	if ledger.Authority != artifact.DriftEventResolutionLedgerAuthority {
		t.Fatalf("authority = %q", ledger.Authority)
	}
	if len(ledger.Records) != 0 {
		t.Fatalf("records = %#v", ledger.Records)
	}
}

func TestWriteDriftEventsSummaryNamesFanout(t *testing.T) {
	report := artifact.DriftEventReport{
		Summary: artifact.DriftEventSummary{
			UniqueEvents:                 1,
			ImpactedDecisions:            2,
			MaterialEvents:               1,
			NeedsBindingResolutionEvents: 1,
			MaxFanout:                    2,
		},
		Events: []artifact.DriftEvent{{
			EventID:          "drift-event-abc",
			ChangedTargetRef: "file:shared.go",
			Fanout:           2,
			Materiality:      artifact.DriftMaterialityMaterialSymbol,
			FallbackKind:     artifact.BindingTargetWholeFileFallback,
			RootCause:        artifact.DriftEventRootCauseBindingTargetMissing,
			ResolutionStatus: artifact.DriftEventResolutionNeedsScopeEnrichment,
		}},
	}

	var buf bytes.Buffer
	if err := writeDriftEventsSummary(&buf, report); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	for _, want := range []string{
		"haft drift events",
		"unique=1",
		"impacted_decisions=2",
		"needs_binding=1",
		"resolved=0",
		"waived=0",
		"max_fanout=2",
		"target=file:shared.go",
		"fanout=2",
		"fallback=whole_file_fallback",
		"root_cause=binding_target_missing",
		"resolution=needs_scope_enrichment",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("summary missing %q:\n%s", want, output)
		}
	}
}

func TestWriteDriftEventsSummaryCapsDefaultEventList(t *testing.T) {
	events := make([]artifact.DriftEvent, 0, driftEventsSummaryEventLimit+2)
	for i := 1; i <= driftEventsSummaryEventLimit+2; i++ {
		events = append(events, artifact.DriftEvent{
			EventID:          fmt.Sprintf("drift-event-%02d", i),
			ChangedTargetRef: fmt.Sprintf("symbol:file.go::func:Event%02d", i),
			Fanout:           i,
			Materiality:      artifact.DriftMaterialityMaterialSymbol,
			RootCause:        artifact.DriftEventRootCauseSemanticTargetChanged,
			ResolutionStatus: artifact.DriftEventResolutionNeedsOperatorJudgment,
		})
	}
	report := artifact.DriftEventReport{
		Summary: artifact.DriftEventSummary{UniqueEvents: len(events)},
		Events:  events,
	}

	var buf bytes.Buffer
	if err := writeDriftEventsSummary(&buf, report); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "drift-event-20") {
		t.Fatalf("summary should include the capped prefix:\n%s", output)
	}
	if strings.Contains(output, "drift-event-21") {
		t.Fatalf("summary should omit events beyond the compact cap:\n%s", output)
	}
	for _, want := range []string{
		"... and 2 more DriftEvent(s)",
		"haft drift events --json",
		"full audit detail",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("summary missing %q:\n%s", want, output)
		}
	}
}

func TestDriftEventResolutionLedgerRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drift-event-resolutions.json")
	ledger := artifact.NewDriftEventResolutionLedger([]artifact.DriftEventResolution{{
		EventID:          "drift-event-abc",
		Status:           artifact.DriftEventResolutionResolved,
		Reason:           "verified additive-only",
		EvidenceRefs:     []string{"go test ./..."},
		ChangedTargetRef: "symbol:shared.go::func:Run",
		TargetKind:       artifact.BindingTargetSymbol,
		TargetStatus:     "modified",
		RootCause:        artifact.DriftEventRootCauseSemanticTargetChanged,
	}})

	if err := writeDriftEventResolutionLedger(path, ledger); err != nil {
		t.Fatalf("writeDriftEventResolutionLedger: %v", err)
	}
	loaded, err := readDriftEventResolutionLedger(path)
	if err != nil {
		t.Fatalf("readDriftEventResolutionLedger: %v", err)
	}

	if loaded.Authority != artifact.DriftEventResolutionLedgerAuthority {
		t.Fatalf("authority = %q", loaded.Authority)
	}
	if len(loaded.Records) != 1 || loaded.Records[0].EventID != "drift-event-abc" {
		t.Fatalf("records = %#v", loaded.Records)
	}
	if loaded.Records[0].ChangedTargetRef != "symbol:shared.go::func:Run" {
		t.Fatalf("changed_target_ref = %q", loaded.Records[0].ChangedTargetRef)
	}
	if loaded.Records[0].RootCause != artifact.DriftEventRootCauseSemanticTargetChanged {
		t.Fatalf("root_cause = %q", loaded.Records[0].RootCause)
	}
}

func TestWriteDriftEventsResolutionSummaryNamesAuthority(t *testing.T) {
	var buf bytes.Buffer
	record := artifact.DriftEventResolution{
		EventID: "drift-event-abc",
		Status:  artifact.DriftEventResolutionWaivedUntil,
		Reason:  "temporary migration waiver",
	}

	if err := writeDriftEventsResolutionSummary(&buf, ".haft/drift-event-resolutions.json", record); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	for _, want := range []string{
		"haft drift events resolve",
		"event=drift-event-abc",
		"status=waived_until",
		"drift_event_resolution_metadata_not_decision_authority",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("summary missing %q:\n%s", want, output)
		}
	}
}
