package codebase

import "testing"

func TestBuildIndexEpochCandidateCompleteBasis(t *testing.T) {
	indexed := mustIndexedDisposition(t, 2)
	empty := NewEmptyFileDisposition()
	admittedA := mustTestAdmittedSource(t, "a.go", "package a")
	admittedB := mustTestAdmittedSource(t, "b.go", "package b")
	states := map[string]CodeFileState{
		"a.go": {
			FilePath:    "a.go",
			ContentHash: admittedA.Digest(),
			Language:    "go",
		},
		"b.go": {
			FilePath:    "b.go",
			ContentHash: admittedB.Digest(),
			Language:    "go",
		},
	}
	admissions := map[string]AdmittedSource{
		"a.go": admittedA,
		"b.go": admittedB,
	}
	dispositions := map[string]FileIndexDisposition{
		"a.go": indexed,
		"b.go": empty,
	}
	candidate, err := BuildIndexEpochCandidate(
		7,
		states,
		admissions,
		dispositions,
		map[string]SourceSkipInfo{},
		DefaultIndexBudget(),
	)
	if err != nil {
		t.Fatal(err)
	}
	basis := candidate.Basis()
	if basis.Epoch != 7 ||
		basis.Coverage.Posture != IndexCoverageComplete ||
		!basis.SupportsKnownAbsence() {
		t.Fatalf("complete basis = %+v", basis)
	}
	if basis.Coverage.IndexedFiles != 1 ||
		basis.Coverage.EmptyFiles != 1 ||
		basis.Coverage.SkippedFiles != 0 {
		t.Fatalf("complete coverage = %+v", basis.Coverage)
	}
	if basis.CorpusDigest == "" || basis.BasisDigest == "" {
		t.Fatalf("candidate identity is incomplete: %+v", basis)
	}
}

func TestBuildIndexEpochCandidateBindsExclusions(t *testing.T) {
	reason, err := ParseSourceSkipReason("oversized")
	if err != nil {
		t.Fatal(err)
	}
	skipped, err := NewSkippedFileDisposition(reason)
	if err != nil {
		t.Fatal(err)
	}
	info := SourceSkipInfo{
		Path:          "large.go",
		Reason:        "oversized",
		ObservedBytes: 500001,
		LimitBytes:    500000,
		Detail:        "observed source exceeds the per-file byte budget",
	}
	states := map[string]CodeFileState{
		"large.go": {
			FilePath:    "large.go",
			ContentHash: skippedSourceStateHash(info),
			Language:    "go",
		},
	}
	candidate, err := BuildIndexEpochCandidate(
		3,
		states,
		map[string]AdmittedSource{},
		map[string]FileIndexDisposition{"large.go": skipped},
		map[string]SourceSkipInfo{"large.go": info},
		DefaultIndexBudget(),
	)
	if err != nil {
		t.Fatal(err)
	}
	basis := candidate.Basis()
	if basis.Coverage.Posture !=
		IndexCoverageBoundedWithExclusions {
		t.Fatalf("bounded basis = %+v", basis)
	}
	if basis.SupportsKnownAbsence() {
		t.Fatal("excluded corpus must not support a known-absence claim")
	}
	if len(basis.Exclusions) != 1 ||
		basis.Exclusions[0].Reason != "oversized" {
		t.Fatalf("exclusions = %+v", basis.Exclusions)
	}
}

func TestBuildIndexEpochCandidateRejectsInvalidPartitions(t *testing.T) {
	state := CodeFileState{
		FilePath:    "a.go",
		ContentHash: "digest",
		Language:    "go",
	}
	indexed := mustIndexedDisposition(t, 1)
	_, err := BuildIndexEpochCandidate(
		1,
		map[string]CodeFileState{"a.go": state},
		map[string]AdmittedSource{},
		map[string]FileIndexDisposition{"a.go": indexed},
		map[string]SourceSkipInfo{},
		DefaultIndexBudget(),
	)
	if err == nil {
		t.Fatal("indexed file without admission must be rejected")
	}
	degraded, err := NewDegradedFileDisposition("syntax error")
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildIndexEpochCandidate(
		1,
		map[string]CodeFileState{"a.go": state},
		map[string]AdmittedSource{
			"a.go": mustTestAdmittedSource(t, "a.go", "package a"),
		},
		map[string]FileIndexDisposition{"a.go": degraded},
		map[string]SourceSkipInfo{},
		DefaultIndexBudget(),
	)
	if err == nil {
		t.Fatal("degraded file must not enter a published epoch")
	}
	admitted := mustTestAdmittedSource(t, "a.go", "package a")
	_, err = BuildIndexEpochCandidate(
		1,
		map[string]CodeFileState{
			"a.go": {
				FilePath:    "a.go",
				ContentHash: "wrong-digest",
				Language:    "go",
			},
		},
		map[string]AdmittedSource{"a.go": admitted},
		map[string]FileIndexDisposition{"a.go": indexed},
		map[string]SourceSkipInfo{},
		DefaultIndexBudget(),
	)
	if err == nil {
		t.Fatal("candidate state must match exact admitted bytes")
	}
}

func mustIndexedDisposition(
	t *testing.T,
	symbols int64,
) FileIndexDisposition {
	t.Helper()
	count, err := NewFileCount(symbols)
	if err != nil {
		t.Fatal(err)
	}
	disposition, err := NewIndexedFileDisposition(count)
	if err != nil {
		t.Fatal(err)
	}
	return disposition
}

func mustTestAdmittedSource(
	t *testing.T,
	path string,
	content string,
) AdmittedSource {
	t.Helper()
	projectPath, err := NewProjectPath(path)
	if err != nil {
		t.Fatal(err)
	}
	language, err := NewSourceLanguage("go")
	if err != nil {
		t.Fatal(err)
	}
	class, err := ParseSourceClass("supported")
	if err != nil {
		t.Fatal(err)
	}
	observation, err := NewContentObservation(
		projectPath,
		language,
		class,
		[]byte(content),
	)
	if err != nil {
		t.Fatal(err)
	}
	admission, _, err := AdmitSource(
		observation,
		DefaultIndexBudget(),
		EmptyAdmissionUsage(),
	)
	if err != nil {
		t.Fatal(err)
	}
	source, err := AdmittedSourceFrom(admission)
	if err != nil {
		t.Fatal(err)
	}
	return source
}
