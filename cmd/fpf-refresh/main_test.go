package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpfrefresh"
)

func TestJoinCleanupResult(t *testing.T) {
	t.Parallel()

	primaryErr := errors.New("primary failure")
	cleanupErr := errors.New("cleanup failure")
	tests := []struct {
		name            string
		exitCode        int
		resultErr       error
		cleanupErr      error
		wantExitCode    int
		wantPrimaryErr  bool
		wantCleanupText bool
	}{
		{
			name:         "successful cleanup preserves result",
			exitCode:     0,
			wantExitCode: 0,
		},
		{
			name:            "cleanup failure turns success into failure",
			exitCode:        0,
			cleanupErr:      cleanupErr,
			wantExitCode:    1,
			wantCleanupText: true,
		},
		{
			name:            "cleanup failure joins primary failure",
			exitCode:        3,
			resultErr:       primaryErr,
			cleanupErr:      cleanupErr,
			wantExitCode:    3,
			wantPrimaryErr:  true,
			wantCleanupText: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			exitCode, resultErr := joinCleanupResult(
				test.exitCode,
				test.resultErr,
				"clean checked candidate artifact",
				test.cleanupErr,
			)
			if exitCode != test.wantExitCode {
				t.Fatalf("exit code = %d, want %d", exitCode, test.wantExitCode)
			}
			if errors.Is(resultErr, primaryErr) != test.wantPrimaryErr {
				t.Fatalf("errors.Is(primary) = %t, want %t", errors.Is(resultErr, primaryErr), test.wantPrimaryErr)
			}
			hasCleanupText := resultErr != nil && strings.Contains(
				resultErr.Error(),
				"clean checked candidate artifact: cleanup failure",
			)
			if hasCleanupText != test.wantCleanupText {
				t.Fatalf("cleanup error visibility = %t, want %t; error = %v", hasCleanupText, test.wantCleanupText, resultErr)
			}
		})
	}
}

func TestBuildIndexerExecutablePropagatesCleanupFailures(t *testing.T) {
	t.Parallel()

	buildFailure := errors.New("build failure")
	cleanupFailure := errors.New("cleanup failure")
	directory := filepath.Join(t.TempDir(), refreshToolTemporaryPrefix+"fixture")
	operations := indexerBuildOperations{
		mkdirTemp: func(string, string) (string, error) {
			return directory, nil
		},
		removeAll: func(string) error {
			return cleanupFailure
		},
	}

	t.Run("successful build exposes cleanup failure", func(t *testing.T) {
		operations := operations
		operations.build = func(context.Context, string, string) ([]byte, error) {
			return nil, nil
		}
		path, cleanup, err := buildIndexerExecutableWithOperations(
			context.Background(),
			"/repository",
			operations,
		)
		if err != nil {
			t.Fatalf("buildIndexerExecutableWithOperations() error = %v", err)
		}
		if path != filepath.Join(directory, "haft-fpf-indexer") {
			t.Fatalf("indexer path = %q", path)
		}
		if err := cleanup(); !errors.Is(err, cleanupFailure) {
			t.Fatalf("cleanup error = %v, want cleanup failure", err)
		}
	})

	t.Run("failed build joins cleanup failure", func(t *testing.T) {
		operations := operations
		cleanupCalls := 0
		operations.removeAll = func(string) error {
			cleanupCalls++
			return cleanupFailure
		}
		operations.build = func(context.Context, string, string) ([]byte, error) {
			return []byte("compiler detail"), buildFailure
		}
		path, cleanup, err := buildIndexerExecutableWithOperations(
			context.Background(),
			"/repository",
			operations,
		)
		if path != "" || cleanup != nil {
			t.Fatalf("failed build returned path=%q cleanup=%v", path, cleanup != nil)
		}
		if !errors.Is(err, buildFailure) || !errors.Is(err, cleanupFailure) {
			t.Fatalf("failed build error = %v, want joined build and cleanup failures", err)
		}
		if !strings.Contains(err.Error(), "compiler detail") ||
			!strings.Contains(err.Error(), "clean failed temporary indexer build") {
			t.Fatalf("failed build diagnostics = %v", err)
		}
		if cleanupCalls != 1 {
			t.Fatalf("cleanup calls = %d, want 1", cleanupCalls)
		}
	})
}

func TestWriteNextCommandKeepsReviewVisibleWithoutBlockingRefresh(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := writeNextCommand(
		&output,
		"/repository",
		fpfrefresh.StateReviewReady,
		strings.Repeat("a", 40),
	)
	if err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "fpf-refresh apply") ||
		!strings.Contains(text, "semantic deltas remain visible") ||
		strings.Contains(text, "accept-review-ready") ||
		strings.Contains(text, "after human assessment") {
		t.Fatalf("review-ready next action still blocks source refresh: %q", text)
	}
}

func TestWriteReviewWarningMakesNonBlockingFindingsProminent(t *testing.T) {
	relativeRoot := filepath.Join("..", "..")
	root, err := filepath.Abs(relativeRoot)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := fpfrefresh.ResolveRepositoryLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	lockPayload, err := os.ReadFile(layout.IntegrationLock)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := fpfrefresh.ParseIntegrationLock(lockPayload)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := fpfrefresh.NewDiagnostic(
		fpfrefresh.DiagnosticTokenGateFailed,
		"candidate query behavior",
		"the frozen query expectation changed on the fresh source",
		fpfrefresh.DefaultTokenGateFixtureRelativePath,
		"bash scripts/fpf_query_token_gate.sh",
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := fpfrefresh.BuildCompatibilityReport(
		fpfrefresh.CompatibilityReportInput{
			ToolRevision:                 lock.GeneratedBy,
			Predecessor:                  lock.Coordinates,
			Candidate:                    lock.Coordinates,
			PredecessorDatabasePath:      layout.Database,
			CandidateDatabasePath:        layout.Database,
			LatestLocalPracticeCandidate: layout.LatestLocalPracticeCandidate,
			AdditionalDiagnostics:        []fpfrefresh.Diagnostic{diagnostic},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeReviewWarning(&output, report, true); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	required := []string{
		reviewWarningBorder,
		"FPF REFRESH REVIEW WARNING",
		"fresh source, index, and integration lock will now be applied",
		"token_gate_failed: candidate query behavior",
		"detail: the frozen query expectation changed on the fresh source",
		"source: " + fpfrefresh.DefaultTokenGateFixtureRelativePath,
		"may block release or quality claims",
		"do not block FPF source freshness",
		"bash scripts/fpf_query_token_gate.sh",
	}
	for _, fragment := range required {
		if !strings.Contains(text, fragment) {
			t.Fatalf("review warning omits %q:\n%s", fragment, text)
		}
	}
	borderCount := strings.Count(text, reviewWarningBorder)
	if borderCount != 2 {
		t.Fatalf("review warning border count = %d:\n%s", borderCount, text)
	}
}

func TestWriteRejectionWarningMakesHardStructureBoundaryProminent(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	layout, err := fpfrefresh.ResolveRepositoryLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	lockPayload, err := os.ReadFile(layout.IntegrationLock)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := fpfrefresh.ParseIntegrationLock(lockPayload)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := fpfrefresh.NewDiagnostic(
		fpfrefresh.DiagnosticSourceStructureCollapse,
		"candidate source-unit projection",
		"source-unit count collapsed from 8000 to 100",
		"candidate:{Readme.md,FPF-Spec.md}",
		"go run ./cmd/fpf-refresh check --candidate-ref candidate --no-fetch",
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := fpfrefresh.BuildCompatibilityReport(
		fpfrefresh.CompatibilityReportInput{
			ToolRevision:                 lock.GeneratedBy,
			Predecessor:                  lock.Coordinates,
			Candidate:                    lock.Coordinates,
			PredecessorDatabasePath:      layout.Database,
			CandidateDatabasePath:        layout.Database,
			LatestLocalPracticeCandidate: layout.LatestLocalPracticeCandidate,
			AdditionalDiagnostics:        []fpfrefresh.Diagnostic{diagnostic},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeRejectionWarning(&output, report); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, fragment := range []string{
		"FPF REFRESH STRUCTURE BLOCKED",
		"fresh candidate was NOT applied",
		"source_structure_collapse: candidate source-unit projection",
		"extreme structural collapse",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("rejection warning omits %q:\n%s", fragment, text)
		}
	}
	if strings.Count(text, reviewWarningBorder) != 2 {
		t.Fatalf("rejection warning border is not prominent:\n%s", text)
	}
}

func TestRunCheckEmitsReportBeforeReturningCleanupFailure(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	layout, err := fpfrefresh.ResolveRepositoryLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	lockPayload, err := os.ReadFile(layout.IntegrationLock)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := fpfrefresh.ParseIntegrationLock(lockPayload)
	if err != nil {
		t.Fatal(err)
	}
	report, err := fpfrefresh.BuildCompatibilityReport(
		fpfrefresh.CompatibilityReportInput{
			ToolRevision:                 lock.GeneratedBy,
			Predecessor:                  lock.Coordinates,
			Candidate:                    lock.Coordinates,
			PredecessorDatabasePath:      layout.Database,
			CandidateDatabasePath:        layout.Database,
			LatestLocalPracticeCandidate: layout.LatestLocalPracticeCandidate,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	cleanupFailure := errors.New("injected candidate cleanup failure")
	originalCheckCandidate := checkCandidate
	checkCandidate = func(
		context.Context,
		fpfrefresh.CandidateCheckRequest,
	) (fpfrefresh.CandidateCheckResult, error) {
		return fpfrefresh.CandidateCheckResult{Report: report}, cleanupFailure
	}
	t.Cleanup(func() { checkCandidate = originalCheckCandidate })

	reportPath := filepath.Join(t.TempDir(), "cleanup-failure-report.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode, runErr := runCheck(
		context.Background(),
		[]string{
			"--repo", root,
			"--candidate-ref", lock.Coordinates.SourceRevision,
			"--no-fetch",
			"--report", reportPath,
		},
		&stdout,
		&stderr,
		false,
	)
	if exitCode != 1 || !errors.Is(runErr, cleanupFailure) {
		t.Fatalf("runCheck() = %d, %v; want exit 1 with cleanup failure", exitCode, runErr)
	}
	if !strings.Contains(stdout.String(), "schema: "+fpfrefresh.ReportSchemaV1) ||
		!strings.Contains(stdout.String(), "report: "+reportPath) {
		t.Fatalf("cleanup failure erased readable report:\n%s", stdout.String())
	}
	written, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("cleanup failure erased canonical report: %v", err)
	}
	if !bytes.Equal(written, append(report.CanonicalBytes(), '\n')) {
		t.Fatalf("written report differs from canonical report: %s", written)
	}
}
