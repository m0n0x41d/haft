package fpfrefresh

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/fpf"
)

func TestSanitizeCandidateDiagnosticBoundsCanonicalMessage(t *testing.T) {
	t.Parallel()

	workspace := "/tmp/" + candidateTemporaryPrefix + "owned/FPF-Spec.md"
	message := workspace + ": " + strings.Repeat("ошибка ", maxMessageText)

	got := sanitizeCandidateDiagnostic(message)

	if len(got) > maxMessageText {
		t.Fatalf("sanitized diagnostic length = %d, want <= %d", len(got), maxMessageText)
	}
	if !utf8.ValidString(got) {
		t.Fatal("sanitized diagnostic truncated through a UTF-8 code point")
	}
	if strings.Contains(got, workspace) {
		t.Fatal("sanitized diagnostic retained a private candidate workspace")
	}
	if !strings.HasSuffix(got, " [truncated]") {
		t.Fatalf("sanitized diagnostic suffix = %q, want truncation marker", got[len(got)-32:])
	}
}

func TestCanReuseVerifiedPredecessorRequiresNoRebuildDelta(t *testing.T) {
	t.Parallel()

	const revision = "308edacfa2bdb2c60d07e4e10c0deb1f260a6a31"
	if !canReuseVerifiedPredecessor(revision, revision, nil) {
		t.Fatal("identical source and token-gate coordinates should reuse the predecessor")
	}
	delta, err := NewDelta(
		DeltaTokenBudgetCorpus,
		DeltaTokenBudgetCorpusChanged,
		"token-gate behavior fixture identity",
		"fixture-v1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"fixture-v2@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		DefaultTokenGateFixtureRelativePath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if canReuseVerifiedPredecessor(revision, revision, []Delta{delta}) {
		t.Fatal("a rebuild delta requires a prepared apply artifact even at the same source")
	}
	if canReuseVerifiedPredecessor(revision, strings.Repeat("b", 40), nil) {
		t.Fatal("a changed source requires a prepared candidate artifact")
	}
}

func TestDerivationToolCompatibilityDeltasExposeGeneratorDrift(t *testing.T) {
	t.Parallel()

	predecessor := &IntegrationLock{GeneratedBy: "fpf-refresh-inputs/v1:sha256:" + strings.Repeat("a", 64)}
	candidate := "fpf-refresh-inputs/v1:sha256:" + strings.Repeat("b", 64)

	deltas, err := derivationToolCompatibilityDeltas(predecessor, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 ||
		deltas[0].Family() != DeltaDerivationToolchain ||
		deltas[0].Kind() != DeltaDerivationToolChanged ||
		deltas[0].Before() != predecessor.GeneratedBy ||
		deltas[0].After() != candidate ||
		deltas[0].SourceRef() != refreshToolInputGraphSourceRef {
		t.Fatalf("toolchain deltas = %#v", deltas)
	}
	deltas, err = derivationToolCompatibilityDeltas(
		&IntegrationLock{GeneratedBy: candidate},
		candidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 0 {
		t.Fatalf("unchanged toolchain deltas = %#v, want none", deltas)
	}
}

func TestShouldVerifyPredecessorProjectionSeparatesCorruptionFromToolMigration(t *testing.T) {
	t.Parallel()

	const current = "fpf-refresh-inputs/v1:sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if !shouldVerifyPredecessorProjection(nil, current) {
		t.Fatal("predecessor without a verified integration lock must reproduce under the current tool")
	}
	if !shouldVerifyPredecessorProjection(&IntegrationLock{GeneratedBy: current}, current) {
		t.Fatal("current-tool predecessor must reproduce under that exact tool")
	}
	predecessor := &IntegrationLock{
		GeneratedBy: "fpf-refresh-inputs/v1:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if shouldVerifyPredecessorProjection(predecessor, current) {
		t.Fatal("verified older-tool predecessor cannot be rederived with the successor parser")
	}
}

func TestCleanupCandidateArtifactPropagatesFailure(t *testing.T) {
	t.Parallel()

	primaryErr := errors.New("comparison failure")
	artifact := &CandidateArtifact{ownedRootPath: "not-an-owned-absolute-root"}

	err := cleanupCandidateArtifact(artifact, primaryErr)
	if !errors.Is(err, primaryErr) {
		t.Fatalf("cleanup result = %v, want preserved primary error", err)
	}
	if !strings.Contains(err.Error(), "clean checked candidate artifact") ||
		!strings.Contains(err.Error(), "not an owned private root") {
		t.Fatalf("cleanup result omitted cleanup failure: %v", err)
	}

	cleanupOnly := cleanupCandidateArtifact(
		&CandidateArtifact{ownedRootPath: "also-not-an-owned-root"},
		nil,
	)
	if cleanupOnly == nil ||
		!strings.Contains(cleanupOnly.Error(), "clean checked candidate artifact") {
		t.Fatalf("cleanup-only result = %v, want cleanup failure", cleanupOnly)
	}

	if got := cleanupCandidateArtifact(nil, primaryErr); !errors.Is(got, primaryErr) {
		t.Fatalf("nil artifact result = %v, want primary error", got)
	}
}

func TestClassifyCandidateBuildDiagnosticRejectsUnbuildableCompilerLineageGap(t *testing.T) {
	t.Parallel()

	err := errors.New(
		`candidate compiler version "fpf-base-typeenv.cov2.v99" is neither current "fpf-base-typeenv.cov2.v5" nor a known predecessor`,
	)
	if got := classifyCandidateBuildDiagnostic(err); got != DiagnosticCandidateVerificationFailed {
		t.Fatalf(
			"classifyCandidateBuildDiagnostic() = %s, want %s",
			got.String(),
			DiagnosticCandidateVerificationFailed.String(),
		)
	}
}

func TestCandidateSourceGrammarDriftBecomesReviewDiagnostic(t *testing.T) {
	t.Parallel()

	revision := strings.Repeat("a", 40)
	observed := []fpf.SourceGrammarDiagnostic{{
		Class:                   fpf.SourceGrammarUnsupported,
		SourceID:                "SYSTEM-RECOGNITION",
		Title:                   "Recognize a system",
		SourcePath:              "data/FPF/FPF-Spec.md",
		SourceRevision:          revision,
		StartLine:               10,
		EndLine:                 20,
		LabelsDiscovered:        []string{"Fresh outcome route"},
		LabelsRecognized:        []string{"Situation and question", "Boundaries"},
		MissingSemanticCategory: "admitted_result_block",
		Detail:                  "new recognizable result label",
	}}

	diagnostics, err := candidateSourceGrammarReviewDiagnostics(
		revision,
		observed,
	)
	if err != nil {
		t.Fatalf("candidateSourceGrammarReviewDiagnostics() error = %v", err)
	}
	if len(diagnostics) != 1 ||
		diagnostics[0].Code() != DiagnosticAdapterGrammarUnsupported {
		t.Fatalf("review diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].SourceRef() != "data/FPF/FPF-Spec.md:10-20" {
		t.Fatalf("source reference = %q", diagnostics[0].SourceRef())
	}
}

func TestCandidateIncompleteCardBecomesDegradedProjectionDiagnostic(t *testing.T) {
	t.Parallel()

	revision := strings.Repeat("b", 40)
	observed := []fpf.SourceGrammarDiagnostic{{
		Class:                   fpf.SourceGrammarMalformed,
		SourceID:                "NEW-CARD",
		SourcePath:              "data/FPF/FPF-Spec.md",
		SourceRevision:          revision,
		StartLine:               30,
		EndLine:                 40,
		MissingSemanticCategory: "first_result",
		Detail:                  "card lacks one projected cue category",
	}}

	diagnostics, err := candidateSourceGrammarReviewDiagnostics(
		revision,
		observed,
	)
	if err != nil {
		t.Fatalf("candidateSourceGrammarReviewDiagnostics() error = %v", err)
	}
	if len(diagnostics) != 1 ||
		diagnostics[0].Code() != DiagnosticSourceProjectionDegraded {
		t.Fatalf("degraded projection diagnostics = %#v", diagnostics)
	}
}

func TestCandidateSourceStructureCollapseHasExplicitHardBoundary(t *testing.T) {
	t.Parallel()

	revision := strings.Repeat("c", 40)
	withinBoundary, collapsed, err := candidateSourceStructureDiagnostic(
		100,
		50,
		revision,
	)
	if err != nil || collapsed || withinBoundary.Code() != 0 {
		t.Fatalf(
			"50%% retention = %#v, %v, %v; want no collapse",
			withinBoundary,
			collapsed,
			err,
		)
	}
	diagnostic, collapsed, err := candidateSourceStructureDiagnostic(
		100,
		49,
		revision,
	)
	if err != nil {
		t.Fatalf("candidateSourceStructureDiagnostic() error = %v", err)
	}
	if !collapsed || diagnostic.Code() != DiagnosticSourceStructureCollapse {
		t.Fatalf("49%% retention = %#v, collapsed=%v", diagnostic, collapsed)
	}
	if !strings.Contains(diagnostic.Message(), "less than 50%") {
		t.Fatalf("collapse diagnostic message = %q", diagnostic.Message())
	}
}
