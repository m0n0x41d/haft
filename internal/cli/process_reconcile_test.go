package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestBuildProcessReconcileReportFindsDuplicateHistoryAndCarrierIssues(t *testing.T) {
	t.Parallel()

	report := buildProcessReconcileReport(processAuthorityReport{
		Authority: processAuthorityAuthority,
		Entries: []ProcessAuthorityEntry{
			{
				AuthorityKey:    "method:swe-core:review-a:step",
				ClaimKind:       "method_step",
				BoundedContext:  "methodpack:swe-core",
				TargetRef:       "method:review",
				SourceRef:       "swe-core@1.0.0:review-a",
				LifecycleStatus: "current",
				CarrierRefs:     []string{"internal/method/catalog.go"},
			},
			{
				AuthorityKey:    "method:swe-core:review-b:step",
				ClaimKind:       "method_step",
				BoundedContext:  "methodpack:swe-core",
				TargetRef:       "method:review",
				SourceRef:       "swe-core@1.0.0:review-b",
				LifecycleStatus: "current",
				CarrierRefs:     []string{"internal/method/catalog.go"},
			},
			{
				AuthorityKey:    "method:swe-core:old:step",
				ClaimKind:       "method_step",
				BoundedContext:  "methodpack:swe-core",
				TargetRef:       "method:old",
				SourceRef:       "swe-core@0.9.0:old",
				LifecycleStatus: "deprecated",
				CarrierRefs:     []string{"internal/method/catalog.go"},
			},
			{
				AuthorityKey:    "interface:missing-carrier",
				ClaimKind:       "interface_contract",
				BoundedContext:  "kernel_interface_catalog",
				TargetRef:       "interface:missing-carrier",
				SourceRef:       "kernel_interface_catalog",
				LifecycleStatus: "current",
			},
		},
	})

	if report.Kind != processReconcileKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.Authority != processReconcileAuthority {
		t.Fatalf("authority = %q", report.Authority)
	}
	if report.Summary.Findings != 3 {
		t.Fatalf("summary = %+v, findings = %+v", report.Summary, report.Findings)
	}
	if report.Summary.DuplicateCurrentTargets != 1 {
		t.Fatalf("duplicate current targets = %d, want 1", report.Summary.DuplicateCurrentTargets)
	}
	if report.Summary.NonCurrentHistoryEntries != 1 {
		t.Fatalf("non-current history = %d, want 1", report.Summary.NonCurrentHistoryEntries)
	}
	if report.Summary.MissingCarrierRefs != 1 {
		t.Fatalf("missing carriers = %d, want 1", report.Summary.MissingCarrierRefs)
	}
	if report.Summary.ApplyReadyMutations != 0 {
		t.Fatalf("apply-ready mutations = %d, want 0", report.Summary.ApplyReadyMutations)
	}
	for _, finding := range report.Findings {
		if finding.NextAction == "" || !strings.Contains(finding.AuthorityBoundary, "not_apply_authority") {
			t.Fatalf("finding lacks review boundary: %+v", finding)
		}
	}
}

func TestRunProcessReconcileJSON(t *testing.T) {
	t.Parallel()

	oldJSON := processReconcileJSON
	t.Cleanup(func() {
		processReconcileJSON = oldJSON
	})
	processReconcileJSON = true

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runProcessReconcile(cmd, nil); err != nil {
		t.Fatal(err)
	}

	var report processReconcileReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode process reconcile report: %v\n%s", err, output.String())
	}
	if report.Kind != processReconcileKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.Summary.ApplyReadyMutations != 0 {
		t.Fatalf("process reconcile must stay report-only: %+v", report.Summary)
	}
}

func TestRunProcessReconcileTextStaysCompact(t *testing.T) {
	t.Parallel()

	oldJSON := processReconcileJSON
	t.Cleanup(func() {
		processReconcileJSON = oldJSON
	})
	processReconcileJSON = false

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runProcessReconcile(cmd, nil); err != nil {
		t.Fatal(err)
	}

	text := output.String()
	for _, want := range []string{"Haft process reconciliation report", "findings=", processReconcileAuthority} {
		if !strings.Contains(text, want) {
			t.Fatalf("process reconcile text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, `"findings"`) || strings.Contains(text, `"authority_keys"`) {
		t.Fatalf("compact process reconcile text should not inline JSON bodies:\n%s", text)
	}
}

func TestProcessReconcileDoesNotInlineIntoDefaultStatus(t *testing.T) {
	fixture := newCheckTestProject(t)
	status, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{"action": "status"})
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{
		processReconcileKind,
		"ProcessReconcileFinding",
		processReconcileAuthority,
	} {
		if strings.Contains(status, absent) {
			t.Fatalf("default status should not inline process reconcile fragment %q:\n%s", absent, status)
		}
	}
}
