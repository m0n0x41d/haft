package fpfrefresh

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestNewReportDerivesClosedCheckPolicy(t *testing.T) {
	t.Parallel()

	predecessor := testPredecessor(t)
	unchanged := testCandidateFromPredecessor(t, predecessor)
	changed := testCompleteCandidate(t)

	t.Run("no change", func(t *testing.T) {
		report, err := newTestReport(t, testRevision(t, "f"), predecessor, unchanged, nil, nil, nil)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[NoChange](t, report, StateNoChange)
	})

	t.Run("apply ready", func(t *testing.T) {
		delta := testDelta(
			t,
			DeltaSourceContent,
			DeltaContentOnlyCompatible,
			"FPF-Spec.md",
			"sha256:before",
			"sha256:after",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			[]Delta{delta},
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[ApplyReady](t, report, StateApplyReady)
	})

	t.Run("review ready", func(t *testing.T) {
		delta := testDelta(
			t,
			DeltaPublicationGrammar,
			DeltaPublicationGrammarExtended,
			"practical-use card labels",
			"legacy labels",
			"legacy plus route labels",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			[]Delta{delta},
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[ReviewReady](t, report, StateReviewReady)
	})

	t.Run("token-gate drift is review ready", func(t *testing.T) {
		diagnostic := testDiagnostic(
			t,
			DiagnosticTokenGateFailed,
			"candidate query behavior",
			"the frozen query expectation changed on the fresh source",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			nil,
			[]Diagnostic{diagnostic},
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[ReviewReady](t, report, StateReviewReady)
	})

	t.Run("source-specific query drift is review ready", func(t *testing.T) {
		diagnostic := testDiagnostic(
			t,
			DiagnosticQueryContractRegression,
			"candidate source-specific Query expectations",
			"one exact PatternID from the previous FPF source was renamed",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			nil,
			[]Diagnostic{diagnostic},
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[ReviewReady](t, report, StateReviewReady)
	})

	t.Run("new recognizable source label is review ready", func(t *testing.T) {
		diagnostic := testDiagnostic(
			t,
			DiagnosticAdapterGrammarUnsupported,
			"candidate practical-use card",
			"a result-like block uses a new source-owned label family",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			nil,
			[]Diagnostic{diagnostic},
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[ReviewReady](t, report, StateReviewReady)
	})

	t.Run("incomplete card projection is review ready", func(t *testing.T) {
		diagnostic := testDiagnostic(
			t,
			DiagnosticSourceProjectionDegraded,
			"candidate practical-use card",
			"raw source is indexed but one projected cue is absent",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			nil,
			[]Diagnostic{diagnostic},
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[ReviewReady](t, report, StateReviewReady)
	})

	t.Run("semantic compiler comparison gap is review ready", func(t *testing.T) {
		delta := testDelta(
			t,
			DeltaBaseTypeEnv,
			DeltaTypeEnvCompilerGap,
			"compiled semantic relation",
			"sha256:before",
			"sha256:after",
		)
		diagnostic := testDiagnostic(
			t,
			DiagnosticTypeEnvCompilerGap,
			"compiled semantic relation",
			"the compatibility order is not yet implemented",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			[]Delta{delta},
			[]Diagnostic{diagnostic},
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[ReviewReady](t, report, StateReviewReady)
	})

	t.Run("candidate rejected before derived build", func(t *testing.T) {
		sourceOnly, err := NewCandidateSourceSnapshot(changed.Source())
		if err != nil {
			t.Fatalf("NewCandidateSourceSnapshot() error = %v", err)
		}
		diagnostic := testDiagnostic(
			t,
			DiagnosticCandidateVerificationFailed,
			"candidate derived publication",
			"no coherent deterministic source/index projection could be built",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			sourceOnly,
			nil,
			[]Diagnostic{diagnostic},
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[CandidateRejected](t, report, StateCandidateRejected)
		if _, exists := report.Candidate().Derived(); exists {
			t.Fatal("rejected source-only candidate unexpectedly has derived coordinates")
		}
	})

	t.Run("candidate rejected after extreme source structure collapse", func(t *testing.T) {
		diagnostic := testDiagnostic(
			t,
			DiagnosticSourceStructureCollapse,
			"candidate source-unit projection",
			"candidate retains less than 50% of the predecessor projection",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			nil,
			[]Diagnostic{diagnostic},
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[CandidateRejected](t, report, StateCandidateRejected)
	})

	t.Run("candidate rejected before required publications are complete", func(t *testing.T) {
		candidate, err := NewCandidateRevisionSnapshot(testRevision(t, "d"))
		if err != nil {
			t.Fatalf("NewCandidateRevisionSnapshot() error = %v", err)
		}
		diagnostic := testDiagnostic(
			t,
			DiagnosticSourcePublicationMalformed,
			"candidate source publication",
			"FPF-Spec.md is missing from the resolved commit",
		)
		report, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			candidate,
			nil,
			[]Diagnostic{diagnostic},
			nil,
		)
		if err != nil {
			t.Fatalf("NewReport() error = %v", err)
		}
		assertOutcome[CandidateRejected](t, report, StateCandidateRejected)
		if report.Candidate().SourceComplete() {
			t.Fatal("missing publication candidate reports complete source coordinates")
		}
		payload := report.CanonicalBytes()
		var encoded reportDTO
		if err := json.Unmarshal(payload, &encoded); err != nil {
			t.Fatalf("decode report: %v", err)
		}
		if encoded.Candidate.Source.ReadmeDigest != "" ||
			encoded.Candidate.Source.SpecDigest != "" {
			t.Fatalf("missing publication report synthesized source digests: %s", payload)
		}
	})
}

func TestReportCarriesExactLocalPracticeCompatibilityObservation(t *testing.T) {
	t.Parallel()

	predecessor := testPredecessor(t)
	candidate := testCandidateFromPredecessor(t, predecessor)
	base := predecessor.Derived().BaseTypeEnvRef()
	assessment, err := NewLocalPracticeCompatibilityAssessment(
		base,
		base,
		base,
		LocalPracticeExact,
	)
	if err != nil {
		t.Fatalf("NewLocalPracticeCompatibilityAssessment() error = %v", err)
	}
	report, err := newReport(
		testRevision(t, "f"),
		predecessor,
		candidate,
		&assessment,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("newReport() error = %v", err)
	}
	observed, exists := report.LocalPracticeCompatibility()
	if !exists || observed != assessment || observed.Result() != LocalPracticeExact {
		t.Fatalf("LocalPracticeCompatibility() = %#v/%t", observed, exists)
	}
	var encoded reportDTO
	if err := json.Unmarshal(report.CanonicalBytes(), &encoded); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if encoded.LocalPracticeCompatibility == nil ||
		encoded.LocalPracticeCompatibility.Result != "exact" ||
		encoded.LocalPracticeCompatibility.CarrierBaseRef != base.String() {
		t.Fatalf("encoded Local-Practice assessment = %#v", encoded.LocalPracticeCompatibility)
	}
	if !strings.Contains(report.Readable(), "local_practice_compatibility: exact") {
		t.Fatalf("readable report omits exact Local-Practice assessment:\n%s", report.Readable())
	}
}

func TestReportRejectsSnapshotDetachedLocalPracticeAssessment(t *testing.T) {
	t.Parallel()

	predecessor := testPredecessor(t)
	candidate := testCompleteCandidate(t)
	candidateDerived, exists := candidate.Derived()
	if !exists {
		t.Fatal("test candidate has no derived coordinates")
	}
	predecessorBase := predecessor.Derived().BaseTypeEnvRef()
	candidateBase := candidateDerived.BaseTypeEnvRef()
	delta := testDelta(
		t,
		DeltaSourceIdentity,
		DeltaSourceIdentityChanged,
		"FPF revision",
		predecessor.Source().Revision().String(),
		candidate.Source().Revision().String(),
	)

	t.Run("predecessor coordinate", func(t *testing.T) {
		assessment, err := NewLocalPracticeCompatibilityAssessment(
			candidateBase,
			candidateBase,
			candidateBase,
			LocalPracticeExact,
		)
		if err != nil {
			t.Fatal(err)
		}
		_, err = newReport(
			testRevision(t, "f"),
			predecessor,
			candidate,
			&assessment,
			[]Delta{delta},
			nil,
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "report predecessor") {
			t.Fatalf("detached predecessor assessment error = %v", err)
		}
	})

	t.Run("candidate coordinate", func(t *testing.T) {
		assessment, err := NewLocalPracticeCompatibilityAssessment(
			predecessorBase,
			predecessorBase,
			predecessorBase,
			LocalPracticeExact,
		)
		if err != nil {
			t.Fatal(err)
		}
		_, err = newReport(
			testRevision(t, "f"),
			predecessor,
			candidate,
			&assessment,
			[]Delta{delta},
			nil,
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "report candidate") {
			t.Fatalf("detached candidate assessment error = %v", err)
		}
	})
}

func TestReportEncodingAndRenderingAreDeterministic(t *testing.T) {
	t.Parallel()

	predecessor := testPredecessor(t)
	candidate := testCompleteCandidate(t)
	deltas := []Delta{
		testDelta(
			t,
			DeltaPatternIDs,
			DeltaPatternIDAdded,
			"A.1.SCR",
			"",
			"published",
		),
		testDelta(
			t,
			DeltaSourceIdentity,
			DeltaSourceIdentityChanged,
			"FPF revision",
			predecessor.Source().Revision().String(),
			candidate.Source().Revision().String(),
		),
	}
	diagnostics := []Diagnostic{
		testDiagnostic(
			t,
			DiagnosticLocalPracticeRebaseRequired,
			"typed-memory candidate 1.6.0",
			"candidate Base TypeEnv differs from the pinned base",
		),
		testDiagnostic(
			t,
			DiagnosticSnapshotPinStale,
			"embedded query fixture",
			"the predecessor digest remains pinned",
		),
	}
	timings := []StageTiming{
		testTiming(t, StageSQLiteBuild, 1500*time.Millisecond),
		testTiming(t, StageFetch, 12*time.Millisecond),
		testTiming(t, StageCompatibilityComparison, 35*time.Millisecond),
	}
	first, err := newTestReport(t,
		testRevision(t, "f"),
		predecessor,
		candidate,
		deltas,
		diagnostics,
		timings,
	)
	if err != nil {
		t.Fatalf("NewReport(first) error = %v", err)
	}
	second, err := newTestReport(t,
		testRevision(t, "f"),
		predecessor,
		candidate,
		reversed(deltas),
		reversed(diagnostics),
		reversed(timings),
	)
	if err != nil {
		t.Fatalf("NewReport(second) error = %v", err)
	}
	if !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatalf(
			"canonical bytes differ by input order:\nfirst:  %s\nsecond: %s",
			first.CanonicalBytes(),
			second.CanonicalBytes(),
		)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf("digest differs by input order: %s != %s", first.Digest().String(), second.Digest().String())
	}
	if first.Readable() != second.Readable() {
		t.Fatalf("readable output differs by input order:\n%s\n---\n%s", first.Readable(), second.Readable())
	}
	if got := first.Timings(); len(got) != 3 ||
		got[0].Stage() != StageFetch ||
		got[1].Stage() != StageCompatibilityComparison ||
		got[2].Stage() != StageSQLiteBuild {
		t.Fatalf("timings are not in fixed stage order: %#v", got)
	}
	if err := first.Verify(); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !bytes.Equal(encoded, first.CanonicalBytes()) {
		t.Fatalf("MarshalJSON bytes differ:\nmarshal:   %s\ncanonical: %s", encoded, first.CanonicalBytes())
	}
}

func TestReportSchemaCannotCarryAuthorityOrApplyRecoveryState(t *testing.T) {
	t.Parallel()

	delta := testDelta(
		t,
		DeltaSourceContent,
		DeltaContentOnlyCompatible,
		"Readme.md",
		"old digest",
		"new digest",
	)
	report, err := newTestReport(t,
		testRevision(t, "f"),
		testPredecessor(t),
		testCompleteCandidate(t),
		[]Delta{delta},
		nil,
		[]StageTiming{testTiming(t, StageSourceObjectRead, 8*time.Millisecond)},
	)
	if err != nil {
		t.Fatalf("NewReport() error = %v", err)
	}
	var document any
	if err := json.Unmarshal(report.CanonicalBytes(), &document); err != nil {
		t.Fatalf("decode canonical report: %v", err)
	}
	forbidden := map[string]struct{}{
		"applicability":     {},
		"applicable":        {},
		"approval":          {},
		"approved":          {},
		"authority":         {},
		"authorization":     {},
		"authorized":        {},
		"release":           {},
		"recovery_required": {},
	}
	assertNoForbiddenKeys(t, document, forbidden)
	if strings.Contains(string(report.CanonicalBytes()), "RecoveryRequired") ||
		strings.Contains(string(report.CanonicalBytes()), "recovery_required") {
		t.Fatalf("compatibility report exposes apply/re-entry state: %s", report.CanonicalBytes())
	}
	if exported := exportedFieldNames(reflect.TypeOf(Report{})); len(exported) != 0 {
		t.Fatalf("Report exposes mutable fields: %v", exported)
	}
}

func TestNewReportRejectsInconsistentInputs(t *testing.T) {
	t.Parallel()

	predecessor := testPredecessor(t)
	changed := testCompleteCandidate(t)
	sourceOnly, err := NewCandidateSourceSnapshot(changed.Source())
	if err != nil {
		t.Fatalf("NewCandidateSourceSnapshot() error = %v", err)
	}

	t.Run("incomplete non-rejected candidate", func(t *testing.T) {
		_, err := NewReport(testRevision(t, "f"), predecessor, sourceOnly, nil, nil, nil)
		if !errors.Is(err, ErrIncompleteCandidate) {
			t.Fatalf("error = %v, want ErrIncompleteCandidate", err)
		}
	})

	t.Run("complete candidate requires Local-Practice assessment", func(t *testing.T) {
		_, err := NewReport(testRevision(t, "f"), predecessor, changed, nil, nil, nil)
		if !errors.Is(err, ErrMissingLocalPracticeAssessment) {
			t.Fatalf("error = %v, want ErrMissingLocalPracticeAssessment", err)
		}
	})

	t.Run("changed candidate without classification", func(t *testing.T) {
		_, err := newTestReport(t, testRevision(t, "f"), predecessor, changed, nil, nil, nil)
		if !errors.Is(err, ErrUnexplainedCandidateChange) {
			t.Fatalf("error = %v, want ErrUnexplainedCandidateChange", err)
		}
	})

	t.Run("duplicate delta subject", func(t *testing.T) {
		delta := testDelta(
			t,
			DeltaPracticalUseCards,
			DeltaPracticalUseCardAdded,
			"SYSTEM-RECOGNITION",
			"",
			"present",
		)
		_, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			[]Delta{delta, delta},
			nil,
			nil,
		)
		if !errors.Is(err, ErrDuplicateDelta) {
			t.Fatalf("error = %v, want ErrDuplicateDelta", err)
		}
	})

	t.Run("duplicate stage timing", func(t *testing.T) {
		timing := testTiming(t, StageFetch, time.Second)
		delta := testDelta(
			t,
			DeltaSourceIdentity,
			DeltaSourceIdentityChanged,
			"FPF revision",
			"old",
			"new",
		)
		_, err := newTestReport(t,
			testRevision(t, "f"),
			predecessor,
			changed,
			[]Delta{delta},
			nil,
			[]StageTiming{timing, timing},
		)
		if !errors.Is(err, ErrDuplicateStageTiming) {
			t.Fatalf("error = %v, want ErrDuplicateStageTiming", err)
		}
	})
}

func TestClosedDeltaPolicyRejectsUnknownFamilyKindPair(t *testing.T) {
	t.Parallel()

	neutral, err := NewDelta(
		DeltaBaseTypeEnv,
		DeltaTypeEnvChanged,
		"source declaration",
		"previous declaration digest",
		"candidate declaration digest",
		"",
	)
	if err != nil {
		t.Fatalf("NewDelta(neutral TypeEnv change) error = %v", err)
	}
	if neutral.Kind().String() != "typeenv_changed" {
		t.Fatalf("neutral TypeEnv kind = %q, want typeenv_changed", neutral.Kind().String())
	}

	_, err = NewDelta(
		DeltaPatternIDs,
		DeltaTypeEnvAdditive,
		"A.1.SCR",
		"absent",
		"present",
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported refresh delta") {
		t.Fatalf("NewDelta() error = %v, want unsupported pair", err)
	}
	if _, err := NewStageTiming(Stage(255), time.Second); err == nil {
		t.Fatal("NewStageTiming() accepted an unknown stage")
	}
	if _, err := NewStageTiming(StageFetch, -time.Nanosecond); err == nil {
		t.Fatal("NewStageTiming() accepted a negative duration")
	}
}

func assertOutcome[T CheckOutcome](t *testing.T, report Report, want CheckState) {
	t.Helper()
	if report.Outcome().State() != want {
		t.Fatalf("state = %s, want %s", report.Outcome().State().String(), want.String())
	}
	if _, ok := report.Outcome().(T); !ok {
		t.Fatalf("outcome = %T, want %T", report.Outcome(), *new(T))
	}
}

func testPredecessor(t *testing.T) PredecessorSnapshot {
	t.Helper()
	source := testSourceCoordinates(t, "a", "1", "2")
	derived := testDerivedCoordinates(t, 15, "3", "4", 54)
	snapshot, err := NewPredecessorSnapshot(source, derived)
	if err != nil {
		t.Fatalf("NewPredecessorSnapshot() error = %v", err)
	}
	return snapshot
}

func testCandidateFromPredecessor(
	t *testing.T,
	predecessor PredecessorSnapshot,
) CandidateSnapshot {
	t.Helper()
	snapshot, err := NewCandidateSnapshot(predecessor.Source(), predecessor.Derived())
	if err != nil {
		t.Fatalf("NewCandidateSnapshot() error = %v", err)
	}
	return snapshot
}

func testCompleteCandidate(t *testing.T) CandidateSnapshot {
	t.Helper()
	source := testSourceCoordinates(t, "b", "5", "6")
	derived := testDerivedCoordinates(t, 16, "7", "8", 55)
	snapshot, err := NewCandidateSnapshot(source, derived)
	if err != nil {
		t.Fatalf("NewCandidateSnapshot() error = %v", err)
	}
	return snapshot
}

func testSourceCoordinates(
	t *testing.T,
	revisionCharacter string,
	readmeCharacter string,
	specCharacter string,
) SourceCoordinates {
	t.Helper()
	coordinates, err := NewSourceCoordinates(
		testRevision(t, revisionCharacter),
		testDigest(t, readmeCharacter),
		testDigest(t, specCharacter),
	)
	if err != nil {
		t.Fatalf("NewSourceCoordinates() error = %v", err)
	}
	return coordinates
}

func testDerivedCoordinates(
	t *testing.T,
	sourceUnitCount uint64,
	typeEnvCharacter string,
	databaseCharacter string,
	indexSchemaVersion uint64,
) DerivedCoordinates {
	t.Helper()
	typeEnvDigest := testDigest(t, typeEnvCharacter)
	typeEnvRef, err := typedmemory.NewTypeEnvRef(typeEnvDigest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef() error = %v", err)
	}
	compiler, err := typedmemory.NewCompilerSchemaVersion("fpf-base-typeenv.cov2.v4")
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion() error = %v", err)
	}
	coordinates, err := NewDerivedCoordinates(
		sourceUnitCount,
		typeEnvRef,
		typeEnvDigest,
		compiler,
		testDigest(t, databaseCharacter),
		indexSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewDerivedCoordinates() error = %v", err)
	}
	return coordinates
}

func testRevision(t *testing.T, character string) typedmemory.SourceRevision {
	t.Helper()
	revision, err := typedmemory.NewSourceRevision(strings.Repeat(character, 40))
	if err != nil {
		t.Fatalf("NewSourceRevision() error = %v", err)
	}
	return revision
}

func testDigest(t *testing.T, character string) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(character, 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	return digest
}

func testDelta(
	t *testing.T,
	family DeltaFamily,
	kind DeltaKind,
	subject string,
	before string,
	after string,
) Delta {
	t.Helper()
	delta, err := NewDelta(
		family,
		kind,
		subject,
		before,
		after,
		"data/FPF/FPF-Spec.md:1",
	)
	if err != nil {
		t.Fatalf("NewDelta() error = %v", err)
	}
	return delta
}

func newTestReport(
	t *testing.T,
	toolRevision typedmemory.SourceRevision,
	predecessor PredecessorSnapshot,
	candidate CandidateSnapshot,
	deltas []Delta,
	diagnostics []Diagnostic,
	timings []StageTiming,
) (Report, error) {
	t.Helper()
	candidateDerived, complete := candidate.Derived()
	if !complete {
		return NewReport(
			toolRevision,
			predecessor,
			candidate,
			deltas,
			diagnostics,
			timings,
		)
	}
	predecessorBase := predecessor.Derived().BaseTypeEnvRef()
	candidateBase := candidateDerived.BaseTypeEnvRef()
	carrierBase := predecessorBase
	result := LocalPracticeCompatibleSuccessorCandidatePossible
	if predecessorBase == candidateBase {
		carrierBase = candidateBase
		result = LocalPracticeExact
	}
	assessment, err := NewLocalPracticeCompatibilityAssessment(
		carrierBase,
		predecessorBase,
		candidateBase,
		result,
	)
	if err != nil {
		t.Fatalf("construct test Local-Practice assessment: %v", err)
	}
	return newReport(
		toolRevision,
		predecessor,
		candidate,
		&assessment,
		deltas,
		diagnostics,
		timings,
	)
}

func testDiagnostic(
	t *testing.T,
	code DiagnosticCode,
	subject string,
	message string,
) Diagnostic {
	t.Helper()
	diagnostic, err := NewDiagnostic(
		code,
		subject,
		message,
		"data/FPF/FPF-Spec.md:1",
		"go test ./internal/fpf -count=1",
	)
	if err != nil {
		t.Fatalf("NewDiagnostic() error = %v", err)
	}
	return diagnostic
}

func testTiming(t *testing.T, stage Stage, duration time.Duration) StageTiming {
	t.Helper()
	timing, err := NewStageTiming(stage, duration)
	if err != nil {
		t.Fatalf("NewStageTiming() error = %v", err)
	}
	return timing
}

func reversed[T any](values []T) []T {
	result := append([]T(nil), values...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func assertNoForbiddenKeys(
	t *testing.T,
	value any,
	forbidden map[string]struct{},
) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, blocked := forbidden[key]; blocked {
				t.Fatalf("report contains forbidden field %q", key)
			}
			assertNoForbiddenKeys(t, child, forbidden)
		}
	case []any:
		for _, child := range typed {
			assertNoForbiddenKeys(t, child, forbidden)
		}
	}
}

func exportedFieldNames(value reflect.Type) []string {
	var result []string
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		if field.IsExported() {
			result = append(result, field.Name)
		}
	}
	return result
}
