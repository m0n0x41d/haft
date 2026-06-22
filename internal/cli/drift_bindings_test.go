package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

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

func TestDriftEventResolutionLedgerRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drift-event-resolutions.json")
	ledger := artifact.NewDriftEventResolutionLedger([]artifact.DriftEventResolution{{
		EventID:      "drift-event-abc",
		Status:       artifact.DriftEventResolutionResolved,
		Reason:       "verified additive-only",
		EvidenceRefs: []string{"go test ./..."},
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
