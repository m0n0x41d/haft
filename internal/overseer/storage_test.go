package overseer

import (
	"strings"
	"testing"
)

func TestStoreLoadLatestRunAndReminder(t *testing.T) {
	root := t.TempDir()
	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: RepoState{GitRoot: ".", Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	run := NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z")
	if err := StoreRun(root, packet, run); err != nil {
		t.Fatalf("StoreRun returned error: %v", err)
	}

	loaded, err := LoadLatestRun(root)
	if err != nil {
		t.Fatalf("LoadLatestRun returned error: %v", err)
	}
	if loaded.Run.ReviewRunID != run.ReviewRunID {
		t.Fatalf("run id = %q, want %q", loaded.Run.ReviewRunID, run.ReviewRunID)
	}
	if loaded.Packet.PacketID != packet.PacketID {
		t.Fatalf("packet id = %q, want %q", loaded.Packet.PacketID, packet.PacketID)
	}

	reminder := BuildReminder(loaded)
	if !reminder.HasReminder {
		t.Fatalf("expected reminder for high risk packet, got %+v", reminder)
	}
	if reminder.Command != "haft overseer show "+run.ReviewRunID {
		t.Fatalf("command = %q, want show command", reminder.Command)
	}
}

func TestMaintenanceRunSuppressesAutoResolvableDriftAndSurfacesRisk(t *testing.T) {
	run, err := BuildMaintenanceRun(MaintenanceInput{
		CreatedAt: "2026-06-09T00:00:00Z",
		Drift: []MaintenanceDriftFinding{
			{
				ID:      "dec-additive",
				Title:   "Additive drift",
				Summary: "code drift - 1 added",
				Action:  "auto_resolve_silent",
				Reason:  "every drift is provably additive (new symbols only)",
			},
			{
				ID:      "dec-risk",
				Title:   "Governed body drift",
				Summary: "code drift - 1 modified",
				Action:  "stage_for_confirm",
				Reason:  "a governed symbol body was modified/removed or a file was deleted",
			},
		},
		Stale: []FindingSummary{{
			ID:     "dec-stale",
			Title:  "Expired evidence",
			Reason: "expired 3 day(s) ago",
		}},
	})
	if err != nil {
		t.Fatalf("BuildMaintenanceRun returned error: %v", err)
	}

	if run.Summary.SuppressedCount != 1 {
		t.Fatalf("suppressed count = %d, want 1", run.Summary.SuppressedCount)
	}
	if run.Summary.SignalCount != 2 {
		t.Fatalf("signal count = %d, want 2", run.Summary.SignalCount)
	}
	if run.Authority.Status != "advisory_only" {
		t.Fatalf("authority = %q, want advisory_only", run.Authority.Status)
	}

	summary := BuildStatusSummary(StoredRun{}, false, run, true)
	output := FormatStatusSignals(summary)
	for _, want := range []string{
		"## Overseer Signals",
		"Drift requires confirmation",
		"Stale governance artifact",
		"low-signal maintenance item",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status signal output missing %q:\n%s", want, output)
		}
	}
}

func TestMaintenanceRunCarriesReconciliationProposalsAndAfterAction(t *testing.T) {
	run, err := BuildMaintenanceRun(MaintenanceInput{
		CreatedAt: "2026-06-09T00:00:00Z",
		Drift: []MaintenanceDriftFinding{{
			ID:      "dec-risk",
			Title:   "Governed body drift",
			Summary: "code drift - 1 modified",
			Action:  "stage_for_confirm",
			Reason:  "a governed symbol body was modified/removed or a file was deleted",
		}},
		Executed: []MaintenanceAction{
			{
				ID:          "act-001",
				Kind:        "auto_rebaseline",
				DecisionRef: "dec-safe",
				Title:       "Safe additive drift",
				Outcome:     "applied",
				PriorState:  `{"files":[]}`,
			},
			{
				ID:           "act-002",
				Kind:         "observable_run",
				DecisionRef:  "dec-machine",
				Title:        "Machine check",
				Detail:       "go test ./internal/cli",
				Outcome:      "evidence_attached",
				EvidenceRefs: []string{"evid-1"},
			},
		},
		ReconciliationProposals: []MaintenanceReconciliationProposal{{
			ID:               "reconcile-high-fanout-1",
			Kind:             "high_fanout_reconciliation_review",
			GroupID:          "decision-reconcile-1",
			Reason:           "fanout needs review",
			DecisionRefs:     []string{"dec-a", "dec-b"},
			Fanout:           7,
			SuggestedCommand: "haft decision reconcile --json",
		}},
	})
	if err != nil {
		t.Fatalf("BuildMaintenanceRun returned error: %v", err)
	}

	if run.Summary.ReconciliationProposalCount != 1 {
		t.Fatalf("reconciliation proposal count = %d, want 1", run.Summary.ReconciliationProposalCount)
	}
	if len(run.ReconciliationProposals) != 1 {
		t.Fatalf("reconciliation proposals = %#v", run.ReconciliationProposals)
	}
	if run.ReconciliationProposals[0].AuthorityBoundary != "read_only_reconciliation_proposal_not_binding_authority" {
		t.Fatalf("proposal authority = %q", run.ReconciliationProposals[0].AuthorityBoundary)
	}
	if len(run.AfterAction.AutoClosedItems) != 1 {
		t.Fatalf("auto closed items = %#v", run.AfterAction.AutoClosedItems)
	}
	if len(run.AfterAction.EvidenceChecked) != 1 || len(run.AfterAction.EvidenceChecked[0].EvidenceRefs) != 1 {
		t.Fatalf("evidence checked = %#v", run.AfterAction.EvidenceChecked)
	}
	if len(run.AfterAction.RemainingOperatorJudgment) != 1 {
		t.Fatalf("remaining operator judgment = %#v", run.AfterAction.RemainingOperatorJudgment)
	}
	if len(run.AfterAction.UndoCommands) != 1 || !strings.Contains(run.AfterAction.UndoCommands[0], "haft overseer undo "+run.MaintenanceID+" act-001") {
		t.Fatalf("undo commands = %#v", run.AfterAction.UndoCommands)
	}
}

func TestFormatStatusSignalsSuppressionsOnlyStaysSilent(t *testing.T) {
	run, err := BuildMaintenanceRun(MaintenanceInput{
		CreatedAt: "2026-06-09T00:00:00Z",
		Drift: []MaintenanceDriftFinding{{
			ID:      "dec-additive",
			Title:   "Additive drift",
			Summary: "code drift - 1 added",
			Action:  "auto_resolve_silent",
			Reason:  "every drift is provably additive (new symbols only)",
		}},
	})
	if err != nil {
		t.Fatalf("BuildMaintenanceRun returned error: %v", err)
	}

	summary := BuildStatusSummary(StoredRun{}, false, run, true)
	if output := FormatStatusSignals(summary); output != "" {
		t.Fatalf("suppression-only status should stay silent, got:\n%s", output)
	}
}

func TestFormatStatusSignalsDedupesScopedAndGeneralStale(t *testing.T) {
	summary := StatusSummary{
		HasSignals: true,
		Signals: []StatusSignal{
			{
				Severity: "medium",
				Source:   "scoped_stale",
				Title:    "Scoped stale governance debt: Same decision",
				Detail:   "expired 6 day(s) ago",
				Command:  "haft overseer show old",
			},
			{
				Severity: "high",
				Source:   "stale",
				Title:    "Stale governance artifact: Same decision",
				Detail:   "expired 6 day(s) ago",
				Command:  "haft_refresh(action=\"scan\")",
			},
		},
	}

	summary.Signals = normalizeStatusSignals(summary.Signals)
	output := FormatStatusSignals(summary)

	if strings.Count(output, "Same decision") != 1 {
		t.Fatalf("expected one deduped signal, got:\n%s", output)
	}
	if !strings.Contains(output, "**HIGH**") {
		t.Fatalf("dedupe should preserve highest severity:\n%s", output)
	}
}

func TestLoadStatusSummaryCombinesLatestRunAndMaintenance(t *testing.T) {
	root := t.TempDir()
	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: RepoState{GitRoot: ".", Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	reviewRun := NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z")
	reviewRun.Findings = []ReviewFinding{AdvisoryFindingDefaults(ReviewFinding{
		ID:           "ofind-1",
		Severity:     "high",
		Claim:        "Important invariant can be violated",
		ConcreteHarm: "Agents may miss an unresolved review finding.",
	})}
	if err := StoreRun(root, packet, reviewRun); err != nil {
		t.Fatalf("StoreRun returned error: %v", err)
	}

	maintenance, err := BuildMaintenanceRun(MaintenanceInput{
		CreatedAt: "2026-06-09T00:00:00Z",
		Drift: []MaintenanceDriftFinding{{
			ID:     "dec-additive",
			Title:  "Additive drift",
			Action: "auto_resolve_silent",
			Reason: "every drift is provably additive (new symbols only)",
		}},
	})
	if err != nil {
		t.Fatalf("BuildMaintenanceRun returned error: %v", err)
	}
	if err := StoreMaintenanceRun(root, maintenance); err != nil {
		t.Fatalf("StoreMaintenanceRun returned error: %v", err)
	}

	summary, err := LoadStatusSummary(root)
	if err != nil {
		t.Fatalf("LoadStatusSummary returned error: %v", err)
	}
	if !summary.HasSignals {
		t.Fatalf("expected status signals, got %+v", summary)
	}
	if summary.SuppressedCount != 1 {
		t.Fatalf("suppressed count = %d, want 1", summary.SuppressedCount)
	}

	output := FormatStatusSignals(summary)
	if !strings.Contains(output, "Important invariant can be violated") {
		t.Fatalf("review finding not rendered:\n%s", output)
	}
}

func TestFormatStatusSignalsGroupsDriftSignals(t *testing.T) {
	summary := StatusSummary{
		HasSignals: true,
		Signals: []StatusSignal{
			{
				Severity: "high",
				Source:   maintenanceSourceDrift,
				Title:    "Drift requires confirmation: First decision `dec-001`",
				Detail:   "code drift — 1 modified",
			},
			{
				Severity: "high",
				Source:   maintenanceSourceDrift,
				Title:    "Drift requires confirmation: Second decision `dec-002`",
				Detail:   "code drift — 2 modified",
			},
			{
				Severity: "medium",
				Source:   "staleness",
				Title:    "Stale governance artifact: Third decision `dec-003`",
				Detail:   "expired",
			},
		},
	}

	output := FormatStatusSignals(summary)
	for _, want := range []string{
		"Drift requires confirmation: 2 item(s) grouped",
		"haft overseer maintain --json",
		"haft_refresh(action=\"scan\", verbose=true)",
		"Stale governance artifact",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status signal output missing %q:\n%s", want, output)
		}
	}
	for _, absent := range []string{"First decision", "Second decision"} {
		if strings.Contains(output, absent) {
			t.Fatalf("status signal output should hide per-decision drift %q:\n%s", absent, output)
		}
	}
}

func TestFormatStatusSignalsGroupsStaleSignals(t *testing.T) {
	summary := StatusSummary{
		HasSignals: true,
		Signals: []StatusSignal{
			{
				Severity: "high",
				Source:   maintenanceSourceStale,
				Title:    "Stale governance artifact: First decision `dec-001`",
				Detail:   "AT RISK — evidence degraded",
			},
			{
				Severity: "medium",
				Source:   "scoped_stale",
				Title:    "Scoped stale governance debt: Second decision `dec-002`",
				Detail:   "expired 6 day(s) ago",
			},
			{
				Severity: "medium",
				Source:   maintenanceSourceSpecHealth,
				Title:    "Spec health finding: Needs owner",
				Detail:   "missing owner",
			},
		},
	}

	output := FormatStatusSignals(summary)
	for _, want := range []string{
		"Stale governance artifacts: 2 item(s) grouped, 1 at risk",
		"haft_refresh(action=\"scan\", verbose=true)",
		"haft_refresh(action=\"review\")",
		"Spec health finding",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status signal output missing %q:\n%s", want, output)
		}
	}
	for _, absent := range []string{"First decision", "Second decision"} {
		if strings.Contains(output, absent) {
			t.Fatalf("status signal output should hide per-decision stale %q:\n%s", absent, output)
		}
	}
}

func TestIngestReviewResultNormalizesAdvisoryFindingsAndDispositionCloses(t *testing.T) {
	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: RepoState{GitRoot: ".", Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	stored := StoredRun{
		Packet: packet,
		Run:    NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z"),
	}
	stored, err = IngestReviewResult(stored, ReviewResultInput{
		Reviewer: Reviewer{Agent: "codex-reviewer"},
		Findings: []ReviewFinding{{
			ID:             "ofind-1",
			Severity:       "critical",
			Confidence:     "high",
			Claim:          "The hook can hide a failed reviewer result.",
			ConcreteHarm:   "Agents may claim clean review without reading the failure.",
			SupportPosture: "accepted_by_input",
			CountsForREff:  true,
		}},
	}, "2026-06-09T01:00:00Z")
	if err != nil {
		t.Fatalf("IngestReviewResult returned error: %v", err)
	}

	if stored.Run.Verdict != "findings_recorded" {
		t.Fatalf("verdict = %q, want findings_recorded", stored.Run.Verdict)
	}
	if got := stored.Run.Findings[0].SupportPosture; got != "advisory_unverified" {
		t.Fatalf("support posture = %q, want advisory_unverified", got)
	}
	if stored.Run.Findings[0].CountsForREff {
		t.Fatalf("ingested review finding must not count for R_eff")
	}

	summary := BuildStatusSummary(stored, true, MaintenanceRun{}, false)
	output := FormatStatusSignals(summary)
	if !strings.Contains(output, "The hook can hide a failed reviewer result") {
		t.Fatalf("ingested finding missing from status:\n%s", output)
	}

	stored, err = ApplyDisposition(stored, ReviewDisposition{
		FindingID: "ofind-1",
		Status:    "fixed_by_commit",
		Actor:     "agent",
		Reason:    "fixed in follow-up change",
	})
	if err != nil {
		t.Fatalf("ApplyDisposition returned error: %v", err)
	}

	summary = BuildStatusSummary(stored, true, MaintenanceRun{}, false)
	if output := FormatStatusSignals(summary); output != "" {
		t.Fatalf("closed finding should not stay in status:\n%s", output)
	}
}
