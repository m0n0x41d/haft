package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestBuildProcessAuthorityReportDerivesMethodAndInterfaceEntries(t *testing.T) {
	t.Parallel()

	report, err := buildProcessAuthorityReport()
	if err != nil {
		t.Fatal(err)
	}
	if report.Kind != processAuthorityKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.Authority != processAuthorityAuthority {
		t.Fatalf("authority = %q", report.Authority)
	}
	if report.Summary.Total == 0 {
		t.Fatal("process authority report returned no entries")
	}

	for _, want := range []struct {
		kind   string
		target string
	}{
		{kind: "method_step", target: "method:verification-before-completion"},
		{kind: "hard_gate", target: "method:verification-before-completion#gate:fresh_verification_before_completion"},
		{kind: "authority_boundary", target: "method:verification-before-completion#source_posture"},
		{kind: "interface_contract", target: "interface:method.catalog"},
	} {
		if !processAuthorityReportHasEntry(report, want.kind, want.target) {
			t.Fatalf("report missing %s %s in %#v", want.kind, want.target, report.Entries)
		}
	}
	if report.Summary.ByClaimKind["interface_contract"] == 0 {
		t.Fatalf("summary missing interface contracts: %+v", report.Summary)
	}
	if !strings.Contains(report.AuthorityBoundary, "not_processpattern") {
		t.Fatalf("authority boundary = %q", report.AuthorityBoundary)
	}
}

func TestRunProcessAuthorityJSON(t *testing.T) {
	oldJSON := processAuthorityJSON
	t.Cleanup(func() {
		processAuthorityJSON = oldJSON
	})
	processAuthorityJSON = true

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runProcessAuthority(cmd, nil); err != nil {
		t.Fatal(err)
	}

	var report processAuthorityReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode process authority report: %v\n%s", err, output.String())
	}
	if report.Kind != processAuthorityKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if !processAuthorityReportHasEntry(report, "interface_contract", "interface:method.catalog") {
		t.Fatalf("JSON report missing method.catalog interface contract")
	}
}

func TestRunProcessAuthorityTextStaysCompact(t *testing.T) {
	oldJSON := processAuthorityJSON
	t.Cleanup(func() {
		processAuthorityJSON = oldJSON
	})
	processAuthorityJSON = false

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runProcessAuthority(cmd, nil); err != nil {
		t.Fatal(err)
	}

	text := output.String()
	for _, want := range []string{"Haft process authority index", "entries=", "interface:method.catalog", "haft process authority --json"} {
		if !strings.Contains(text, want) {
			t.Fatalf("process authority text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, `"entries"`) || strings.Contains(text, `"source_digest"`) {
		t.Fatalf("compact process authority text should not inline JSON bodies:\n%s", text)
	}
}

func TestProcessAuthorityDoesNotInlineIntoDefaultStatus(t *testing.T) {
	fixture := newCheckTestProject(t)
	status, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{"action": "status"})
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{
		processAuthorityKind,
		"ProcessAuthorityEntry",
		"read_only_process_authority_index_not_source_of_truth_not_processpattern",
	} {
		if strings.Contains(status, absent) {
			t.Fatalf("default status should not inline process authority fragment %q:\n%s", absent, status)
		}
	}
}

func processAuthorityReportHasEntry(report processAuthorityReport, kind string, target string) bool {
	for _, entry := range report.Entries {
		if entry.ClaimKind == kind && entry.TargetRef == target {
			return true
		}
	}
	return false
}
