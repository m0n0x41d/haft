package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	methodpkg "github.com/m0n0x41d/haft/internal/method"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestRunMethodCatalogJSON(t *testing.T) {
	oldJSON := methodCatalogJSON
	oldStatus := methodCatalogStatus
	oldScopeID := methodCatalogScopeID
	t.Cleanup(func() {
		methodCatalogJSON = oldJSON
		methodCatalogStatus = oldStatus
		methodCatalogScopeID = oldScopeID
	})
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "method-catalog-json")
	restore := enterTestProjectRoot(t, root)
	defer restore()

	methodCatalogJSON = true
	methodCatalogStatus = methodpkg.LifecycleCurrent
	methodCatalogScopeID = ""

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runMethodCatalog(cmd, nil); err != nil {
		t.Fatal(err)
	}

	var report methodpkg.CatalogReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode catalog report: %v\n%s", err, output.String())
	}
	if report.Kind != methodpkg.CatalogReportKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.FilterStatus != methodpkg.LifecycleCurrent {
		t.Fatalf("filter_status = %q", report.FilterStatus)
	}
	if report.Summary.Returned == 0 {
		t.Fatalf("returned = %d, want current methods", report.Summary.Returned)
	}
	if len(report.Methods[0].SourcePatternRefs) == 0 {
		t.Fatalf("first catalog method missing source_pattern_refs: %+v", report.Methods[0])
	}
	if !strings.Contains(report.AuthorityBoundary, "not_processpattern") {
		t.Fatalf("authority boundary = %q, want not_processpattern", report.AuthorityBoundary)
	}
}

func TestRunMethodCatalogTextStaysCompact(t *testing.T) {
	oldJSON := methodCatalogJSON
	oldStatus := methodCatalogStatus
	oldScopeID := methodCatalogScopeID
	t.Cleanup(func() {
		methodCatalogJSON = oldJSON
		methodCatalogStatus = oldStatus
		methodCatalogScopeID = oldScopeID
	})
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "method-catalog-text")
	restore := enterTestProjectRoot(t, root)
	defer restore()

	methodCatalogJSON = false
	methodCatalogStatus = methodpkg.LifecycleCurrent
	methodCatalogScopeID = ""

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runMethodCatalog(cmd, nil); err != nil {
		t.Fatal(err)
	}

	text := output.String()
	for _, want := range []string{"Haft MethodPack catalog", "status=current", "lifecycle=current", "source_pattern_refs", "documentary context only"} {
		if !strings.Contains(text, want) {
			t.Fatalf("catalog text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, `"hard_gates"`) || strings.Contains(text, `"procedure"`) {
		t.Fatalf("catalog text inlined full method definitions:\n%s", text)
	}
}

func TestRunMethodCatalogOmitsSWEForNonSoftware(t *testing.T) {
	oldJSON := methodCatalogJSON
	oldStatus := methodCatalogStatus
	oldScopeID := methodCatalogScopeID
	t.Cleanup(func() {
		methodCatalogJSON = oldJSON
		methodCatalogStatus = oldStatus
		methodCatalogScopeID = oldScopeID
	})
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "method-catalog-nonsoftware")
	restore := enterTestProjectRoot(t, root)
	defer restore()

	methodCatalogJSON = true
	methodCatalogStatus = methodpkg.LifecycleCurrent
	methodCatalogScopeID = ""

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runMethodCatalog(cmd, nil); err != nil {
		t.Fatal(err)
	}
	response := methodProfileApplicabilityResponse{}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf(
			"decode profile-aware catalog response: %v\n%s",
			err,
			output.String(),
		)
	}
	if response.Applicability != "not_applicable" ||
		response.ArtifactCreated {
		t.Fatalf("non-software catalog response = %#v", response)
	}
}

func TestRunMethodCheckpointOpenCloseTraceJSON(t *testing.T) {
	fixture := newCheckTestProject(t)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	_, _, run, err := methodpkg.CreateRun(context.Background(), fixture.store, fixture.haftDir, methodpkg.PullInput{
		Task:             "CLI checkpoint test",
		DeclaredTaskKind: "feature",
		ChangeIntent:     "test checkpoint CLI behavior",
	})
	if err != nil {
		t.Fatal(err)
	}

	oldOpenJSON := methodCheckpointOpenJSON
	oldTargetRef := methodCheckpointOpenTargetRef
	oldCheckRef := methodCheckpointOpenCheckRef
	oldDigest := methodCheckpointOpenDigest
	oldTTL := methodCheckpointOpenTTLMinutes
	oldCloseJSON := methodCheckpointCloseJSON
	oldOutcome := methodCheckpointCloseOutcome
	oldObsRefs := methodCheckpointCloseObsRefs
	oldCloseDigest := methodCheckpointCloseDigest
	oldNextTarget := methodCheckpointCloseNextTarget
	oldTraceJSON := methodCheckpointTraceJSON
	t.Cleanup(func() {
		methodCheckpointOpenJSON = oldOpenJSON
		methodCheckpointOpenTargetRef = oldTargetRef
		methodCheckpointOpenCheckRef = oldCheckRef
		methodCheckpointOpenDigest = oldDigest
		methodCheckpointOpenTTLMinutes = oldTTL
		methodCheckpointCloseJSON = oldCloseJSON
		methodCheckpointCloseOutcome = oldOutcome
		methodCheckpointCloseObsRefs = oldObsRefs
		methodCheckpointCloseDigest = oldCloseDigest
		methodCheckpointCloseNextTarget = oldNextTarget
		methodCheckpointTraceJSON = oldTraceJSON
	})

	methodCheckpointOpenJSON = true
	methodCheckpointOpenTargetRef = "internal/cli/method_checkpoint.go::runMethodCheckpointOpen"
	methodCheckpointOpenCheckRef = "method:checkpoint-cli"
	methodCheckpointOpenDigest = "sha256:open"
	methodCheckpointOpenTTLMinutes = int(time.Hour.Minutes())

	var openOutput bytes.Buffer
	openCmd := &cobra.Command{}
	openCmd.SetOut(&openOutput)
	if err := runMethodCheckpointOpen(openCmd, []string{run.ID}); err != nil {
		t.Fatal(err)
	}
	var opened methodpkg.CheckpointResult
	if err := json.Unmarshal(openOutput.Bytes(), &opened); err != nil {
		t.Fatalf("decode open result: %v\n%s", err, openOutput.String())
	}
	if opened.CloseToken == "" || opened.CheckpointID == "" {
		t.Fatalf("open result missing token/id: %+v", opened)
	}

	methodCheckpointCloseJSON = true
	methodCheckpointCloseOutcome = "reviewed"
	methodCheckpointCloseObsRefs = []string{"go test ./internal/cli"}
	methodCheckpointCloseDigest = "sha256:closed"
	methodCheckpointCloseNextTarget = "internal/cli/method_test.go"

	var closeOutput bytes.Buffer
	closeCmd := &cobra.Command{}
	closeCmd.SetOut(&closeOutput)
	if err := runMethodCheckpointClose(closeCmd, []string{opened.CloseToken}); err != nil {
		t.Fatal(err)
	}
	var closed methodpkg.CheckpointResult
	if err := json.Unmarshal(closeOutput.Bytes(), &closed); err != nil {
		t.Fatalf("decode close result: %v\n%s", err, closeOutput.String())
	}
	if closed.CheckpointID != opened.CheckpointID || closed.Record.Outcome != "reviewed" {
		t.Fatalf("close result = %+v, opened = %+v", closed, opened)
	}

	methodCheckpointTraceJSON = true
	var traceOutput bytes.Buffer
	traceCmd := &cobra.Command{}
	traceCmd.SetOut(&traceOutput)
	if err := runMethodCheckpointTrace(traceCmd, []string{run.ID}); err != nil {
		t.Fatal(err)
	}
	var trace methodpkg.CheckpointTraceReport
	if err := json.Unmarshal(traceOutput.Bytes(), &trace); err != nil {
		t.Fatalf("decode trace report: %v\n%s", err, traceOutput.String())
	}
	if trace.Summary.Records != 2 || trace.Summary.Closed != 1 {
		t.Fatalf("trace summary = %+v", trace.Summary)
	}
	if !strings.Contains(trace.AuthorityBoundary, "not_evidence") {
		t.Fatalf("trace authority boundary = %q", trace.AuthorityBoundary)
	}
}
