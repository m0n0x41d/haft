package fpfrefresh

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	basetypeenvartifacts "github.com/m0n0x41d/haft/data/haft/base-typeenv/artifacts"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestAnalyzeSnapshotCompatibilityClassifiesSQLiteFixtureChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	predecessorPath := filepath.Join(root, "predecessor.db")
	candidatePath := filepath.Join(root, "candidate.db")

	predecessor := analysisFixture{
		revision:      strings.Repeat("a", 40),
		readmeDigest:  analysisDigest("1"),
		specDigest:    analysisDigest("2"),
		typeEnvDigest: analysisDigest("3"),
		units: []analysisFixtureUnit{
			analysisUnit(
				"card-removed",
				"LEGACY-ROUTE",
				fpf.SourceUnitRolePracticalUseCard,
				"Legacy route",
				"",
				`["A.OLD"]`,
				analysisDigest("4"),
				`{"recognizable_situation":"legacy"}`,
			),
			analysisUnit(
				"card-shared",
				"SYSTEM-RECOGNITION",
				fpf.SourceUnitRolePracticalUseCard,
				"Recognize the system",
				"",
				`["A.1"]`,
				analysisDigest("5"),
				`{"recognizable_situation":"old cue"}`,
			),
			analysisUnit(
				"pattern-a1",
				"A.1",
				fpf.SourceUnitRolePatternBody,
				"System recognition",
				"A.1",
				`[]`,
				analysisDigest("6"),
				`{}`,
			),
			analysisUnit(
				"pattern-removed",
				"A.OLD",
				fpf.SourceUnitRolePatternBody,
				"Retired pattern",
				"A.OLD",
				`[]`,
				analysisDigest("7"),
				`{}`,
			),
			analysisUnit(
				"role-common",
				"",
				fpf.SourceUnitRolePreface,
				"Shared source unit",
				"",
				`[]`,
				analysisDigest("8"),
				`{}`,
			),
			analysisUnit(
				"structure-common",
				"",
				fpf.SourceUnitRoleTOCRow,
				"Shared ToC row",
				"A.2",
				`[]`,
				analysisDigest("9"),
				`{}`,
			),
		},
	}
	candidate := analysisFixture{
		revision:      strings.Repeat("b", 40),
		readmeDigest:  analysisDigest("a"),
		specDigest:    analysisDigest("b"),
		typeEnvDigest: analysisDigest("3"),
		units: []analysisFixtureUnit{
			analysisUnit(
				"card-added",
				"NEW-ROUTE",
				fpf.SourceUnitRolePracticalUseCard,
				"New route",
				"",
				`["A.NEW"]`,
				analysisDigest("d"),
				`{"recognizable_situation":"new"}`,
			),
			analysisUnit(
				"card-shared",
				"SYSTEM-RECOGNITION",
				fpf.SourceUnitRolePracticalUseCard,
				"Recognize the current system",
				"",
				`["A.1","A.2"]`,
				analysisDigest("5"),
				`{"recognizable_situation":"current cue"}`,
			),
			analysisUnit(
				"extra-source-unit",
				"",
				fpf.SourceUnitRolePreface,
				"Additional source material",
				"",
				`[]`,
				analysisDigest("e"),
				`{}`,
			),
			analysisUnit(
				"pattern-a1",
				"A.1",
				fpf.SourceUnitRolePatternBody,
				"System recognition",
				"A.1",
				`[]`,
				analysisDigest("6"),
				`{}`,
			),
			analysisUnit(
				"pattern-added",
				"A.NEW",
				fpf.SourceUnitRolePatternBody,
				"New pattern",
				"A.NEW",
				`[]`,
				analysisDigest("f"),
				`{}`,
			),
			analysisUnit(
				"role-common",
				"",
				fpf.SourceUnitRolePatternSection,
				"Shared source unit",
				"",
				`[]`,
				analysisDigest("8"),
				`{}`,
			),
			analysisUnit(
				"structure-common",
				"",
				fpf.SourceUnitRoleTOCRow,
				"Shared ToC row",
				"A.2",
				`[]`,
				analysisDigest("0"),
				`{}`,
			),
		},
	}
	writeAnalysisDatabase(t, predecessorPath, predecessor)
	writeAnalysisDatabase(t, candidatePath, candidate)

	input := SnapshotAnalysisInput{
		PredecessorDatabasePath: predecessorPath,
		CandidateDatabasePath:   candidatePath,
	}
	first, err := AnalyzeSnapshotCompatibility(input)
	if err != nil {
		t.Fatalf("AnalyzeSnapshotCompatibility(first) error = %v", err)
	}
	second, err := AnalyzeSnapshotCompatibility(input)
	if err != nil {
		t.Fatalf("AnalyzeSnapshotCompatibility(second) error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("analysis differs across identical reads:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	if len(first.Deltas) != 16 {
		t.Fatalf("delta count = %d, want 16:\n%s", len(first.Deltas), describeAnalysisDeltas(first.Deltas))
	}
	if len(first.Diagnostics) != 0 {
		t.Fatalf("diagnostic count = %d, want 0", len(first.Diagnostics))
	}

	assertAnalysisDelta(
		t,
		first.Deltas,
		DeltaSourceIdentity,
		DeltaSourceIdentityChanged,
		"FPF source revision",
		predecessor.revision,
		candidate.revision,
	)
	assertAnalysisDelta(
		t,
		first.Deltas,
		DeltaSourceContent,
		DeltaContentOnlyCompatible,
		"Readme.md",
		predecessor.readmeDigest,
		candidate.readmeDigest,
	)
	assertAnalysisDelta(
		t,
		first.Deltas,
		DeltaSourceContent,
		DeltaContentOnlyCompatible,
		"FPF-Spec.md",
		predecessor.specDigest,
		candidate.specDigest,
	)
	assertAnalysisDelta(
		t,
		first.Deltas,
		DeltaSourceContent,
		DeltaContentOnlyCompatible,
		"source-unit count",
		"6",
		"7",
	)

	assertAnalysisDeltaKind(
		t,
		first.Deltas,
		DeltaPracticalUseCards,
		DeltaPracticalUseCardAdded,
		"NEW-ROUTE",
	)
	assertAnalysisDeltaKind(
		t,
		first.Deltas,
		DeltaPracticalUseCards,
		DeltaPracticalUseCardRemoved,
		"LEGACY-ROUTE",
	)
	assertAnalysisDeltaKind(
		t,
		first.Deltas,
		DeltaPracticalUseCards,
		DeltaPracticalUseCardChanged,
		"SYSTEM-RECOGNITION",
	)
	assertAnalysisDelta(
		t,
		first.Deltas,
		DeltaPracticalCardDirectRefs,
		DeltaPracticalCardDirectRefsChanged,
		"SYSTEM-RECOGNITION",
		`["A.1"]`,
		`["A.1","A.2"]`,
	)
	assertAnalysisDeltaKind(
		t,
		first.Deltas,
		DeltaQueryBehavior,
		DeltaQueryExpectationChanged,
		"exact practical-use identifier NEW-ROUTE",
	)
	assertAnalysisDeltaKind(
		t,
		first.Deltas,
		DeltaQueryBehavior,
		DeltaQueryExpectationChanged,
		"exact practical-use identifier LEGACY-ROUTE",
	)

	assertAnalysisDeltaKind(
		t,
		first.Deltas,
		DeltaPatternIDs,
		DeltaPatternIDAdded,
		"A.NEW",
	)
	assertAnalysisDeltaKind(
		t,
		first.Deltas,
		DeltaPatternIDs,
		DeltaPatternIDRemoved,
		"A.OLD",
	)
	assertAnalysisDeltaKind(
		t,
		first.Deltas,
		DeltaQueryBehavior,
		DeltaQueryExpectationChanged,
		"exact PatternID A.NEW",
	)
	assertAnalysisDeltaKind(
		t,
		first.Deltas,
		DeltaQueryBehavior,
		DeltaQueryExpectationChanged,
		"exact PatternID A.OLD",
	)
	assertAnalysisDelta(
		t,
		first.Deltas,
		DeltaToCRelations,
		DeltaToCRelationChanged,
		"structure-common",
		analysisDigest("9"),
		analysisDigest("0"),
	)
	assertAnalysisDelta(
		t,
		first.Deltas,
		DeltaSourceRoles,
		DeltaSourceRoleChanged,
		"role-common",
		string(fpf.SourceUnitRolePreface),
		string(fpf.SourceUnitRolePatternSection),
	)

	predecessorCoordinates := fixtureIntegrationCoordinates(predecessor, analysisDigest("d"))
	candidateCoordinates := fixtureIntegrationCoordinates(candidate, analysisDigest("e"))
	predecessorSnapshot, candidateSnapshot, err := ReportSnapshotsFromIntegrationCoordinates(
		predecessorCoordinates,
		candidateCoordinates,
	)
	if err != nil {
		t.Fatalf("ReportSnapshotsFromIntegrationCoordinates() error = %v", err)
	}
	report, err := newTestReport(t,
		testRevision(t, "f"),
		predecessorSnapshot,
		candidateSnapshot,
		first.Deltas,
		first.Diagnostics,
		nil,
	)
	if err != nil {
		t.Fatalf("NewReport() error = %v", err)
	}
	for _, rendered := range [][]byte{report.CanonicalBytes(), []byte(report.Readable())} {
		if bytes.Contains(rendered, []byte(root)) {
			t.Fatalf("report leaked temporary workspace path %q:\n%s", root, rendered)
		}
	}
}

func TestCandidateTypeEnvDeltasKeepsDeclarationChangeNeutral(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "candidate.db")
	writeAnalysisDatabase(t, path, analysisFixture{
		revision:      strings.Repeat("a", 40),
		readmeDigest:  analysisDigest("1"),
		specDigest:    analysisDigest("2"),
		typeEnvDigest: analysisDigest("3"),
		typeEnvChanges: []analysisTypeEnvChange{
			{ordinal: 2, symbol: "U.Removed", kind: "removed", rationale: "symbol is absent"},
			{ordinal: 0, symbol: "U.Added", kind: "added", rationale: "new declaration"},
			{ordinal: 1, symbol: "U.Changed", kind: "changed", rationale: "declaration digest changed"},
		},
	})

	deltas, err := candidateTypeEnvDeltas(path)
	if err != nil {
		t.Fatalf("candidateTypeEnvDeltas() error = %v", err)
	}
	assertAnalysisDeltaKind(t, deltas, DeltaBaseTypeEnv, DeltaTypeEnvAdditive, "U.Added")
	assertAnalysisDeltaKind(t, deltas, DeltaBaseTypeEnv, DeltaTypeEnvChanged, "U.Changed")
	assertAnalysisDeltaKind(t, deltas, DeltaBaseTypeEnv, DeltaTypeEnvRemoved, "U.Removed")
	for _, delta := range deltas {
		if delta.Kind() == DeltaTypeEnvNarrowed {
			t.Fatalf("declaration change was overclassified as narrowing: %#v", delta)
		}
	}
}

func TestTypeEnvSuccessorDeltasClassifyOnlyProvenSemanticOrder(t *testing.T) {
	t.Parallel()

	rules := []analysisSuccessorRule{
		analysisRule(
			t,
			projecttypeenvcompatibility.RelationSlotFamily,
			"removed",
			projecttypeenvcompatibility.SuccessorRemoved,
			projecttypeenvcompatibility.GroundSemanticCoordinateRemoved,
			"1",
			"",
		),
		analysisRule(
			t,
			projecttypeenvcompatibility.RelationSlotFamily,
			"narrowed",
			projecttypeenvcompatibility.SuccessorNarrowed,
			projecttypeenvcompatibility.GroundCardinalityDomainRestricted,
			"2",
			"3",
		),
		analysisRule(
			t,
			projecttypeenvcompatibility.RelationSlotFamily,
			"widened",
			projecttypeenvcompatibility.SuccessorWidened,
			projecttypeenvcompatibility.GroundCardinalityDomainExpanded,
			"4",
			"5",
		),
		analysisRule(
			t,
			projecttypeenvcompatibility.RelationSlotFamily,
			"added",
			projecttypeenvcompatibility.SuccessorAdditive,
			projecttypeenvcompatibility.GroundSemanticCoordinateAdded,
			"",
			"6",
		),
		analysisRule(
			t,
			projecttypeenvcompatibility.RelationSlotFamily,
			"compiler-gap",
			projecttypeenvcompatibility.SuccessorCompilerGap,
			projecttypeenvcompatibility.GroundSemanticOrderNotImplemented,
			"7",
			"8",
		),
		analysisRule(
			t,
			projecttypeenvcompatibility.RelationSlotFamily,
			"unchanged",
			projecttypeenvcompatibility.SuccessorUnchanged,
			projecttypeenvcompatibility.GroundExactSemanticMatch,
			"9",
			"9",
		),
	}

	firstDeltas, firstDiagnostics, err := typeEnvSuccessorDeltas(rules)
	if err != nil {
		t.Fatalf("typeEnvSuccessorDeltas(first) error = %v", err)
	}
	slices.Reverse(rules)
	secondDeltas, secondDiagnostics, err := typeEnvSuccessorDeltas(rules)
	if err != nil {
		t.Fatalf("typeEnvSuccessorDeltas(second) error = %v", err)
	}
	if !reflect.DeepEqual(firstDeltas, secondDeltas) ||
		!reflect.DeepEqual(firstDiagnostics, secondDiagnostics) {
		t.Fatal("successor delta projection depends on input order")
	}
	if len(firstDeltas) != 5 || len(firstDiagnostics) != 1 {
		t.Fatalf(
			"successor projection = %d deltas/%d diagnostics, want 5/1",
			len(firstDeltas),
			len(firstDiagnostics),
		)
	}
	for subject, want := range map[string]DeltaKind{
		"added":        DeltaTypeEnvAdditive,
		"widened":      DeltaTypeEnvChanged,
		"narrowed":     DeltaTypeEnvNarrowed,
		"removed":      DeltaTypeEnvRemoved,
		"compiler-gap": DeltaTypeEnvCompilerGap,
	} {
		assertAnalysisDeltaKind(
			t,
			firstDeltas,
			DeltaBaseTypeEnv,
			want,
			"Base TypeEnv semantic relation_slot/"+subject,
		)
	}
	if firstDiagnostics[0].Code() != DiagnosticTypeEnvCompilerGap ||
		!strings.Contains(firstDiagnostics[0].Message(), "semantic_order_not_implemented") {
		t.Fatalf("compiler-gap diagnostic = %#v", firstDiagnostics[0])
	}
}

func TestCompareExecutableTypeEnvCompatibilityIgnoresSourceIdentityOnlyChange(
	t *testing.T,
) {
	t.Parallel()

	fixture := newRefreshEffectsFixture(t)

	deltas, diagnostics, err := compareExecutableTypeEnvCompatibility(
		fixture.predecessorDatabaseBackup,
		fixture.candidateDatabase,
	)
	if err != nil {
		t.Fatalf("compareExecutableTypeEnvCompatibility() error = %v", err)
	}
	if len(deltas) != 0 || len(diagnostics) != 0 {
		t.Fatalf(
			"source-identity-only comparison = %d deltas/%d diagnostics:\n%s",
			len(deltas),
			len(diagnostics),
			describeAnalysisDeltas(deltas),
		)
	}
}

func TestSourceAuthoredE11SplitContinuityUsesFixtureAndExactLine(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(
		"..",
		"fpf",
		"testdata",
		"practical_card_split_continuity.md",
	)
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read split-continuity fixture: %v", err)
	}
	candidate := analysisIndex{units: map[string]analysisSourceUnit{
		"e11": {
			UnitID:     "e11",
			Role:       string(fpf.SourceUnitRolePatternBody),
			Body:       string(body),
			PatternID:  "E.11",
			SourcePath: "data/FPF/FPF-Spec.md",
			StartLine:  100,
			EndLine:    104,
		},
	}}
	for _, identity := range []string{
		"SYSTEM-RECOGNITION",
		"SYSTEM-DELIMITATION",
		"WORDING",
		"ARCHITECTURE",
	} {
		candidate.units["card:"+identity] = analysisSourceUnit{
			UnitID:   "card:" + identity,
			SourceID: identity,
			Role:     string(fpf.SourceUnitRolePracticalUseCard),
		}
	}
	predecessor := analysisIndex{units: map[string]analysisSourceUnit{
		"card:SYSTEM-IN-CONTEXT": {
			UnitID:   "card:SYSTEM-IN-CONTEXT",
			SourceID: "SYSTEM-IN-CONTEXT",
			Role:     string(fpf.SourceUnitRolePracticalUseCard),
		},
	}}
	deltas, err := compareSourceAuthoredPracticalUseContinuity(
		candidate,
		sourceUnitsByIdentity(
			predecessor.units,
			string(fpf.SourceUnitRolePracticalUseCard),
		),
		sourceUnitsByIdentity(
			candidate.units,
			string(fpf.SourceUnitRolePracticalUseCard),
		),
	)
	if err != nil {
		t.Fatalf("compareSourceAuthoredPracticalUseContinuity() error = %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("continuity deltas = %d, want 1: %#v", len(deltas), deltas)
	}
	delta := deltas[0]
	if delta.Family() != DeltaPracticalUseCards ||
		delta.Kind() != DeltaPracticalUseCardSplit ||
		delta.Before() != "SYSTEM-IN-CONTEXT" ||
		delta.After() != "SYSTEM-RECOGNITION,SYSTEM-DELIMITATION,WORDING,ARCHITECTURE" ||
		delta.SourceRef() != "data/FPF/FPF-Spec.md:104-104" {
		t.Fatalf("source-authored continuity delta = %#v", delta)
	}
}

func TestSourceAuthoredE11SplitContinuityDoesNotInferFromSimilarNames(t *testing.T) {
	t.Parallel()

	index := analysisIndex{units: map[string]analysisSourceUnit{
		"similar-prose": {
			UnitID:     "similar-prose",
			Role:       string(fpf.SourceUnitRolePatternBody),
			Body:       "SYSTEM-IN-CONTEXT became SYSTEM-RECOGNITION and SYSTEM-DELIMITATION.",
			PatternID:  "E.11",
			SourcePath: "data/FPF/FPF-Spec.md",
			StartLine:  50,
			EndLine:    50,
		},
		"wrong-pattern": {
			UnitID: "wrong-pattern",
			Role:   string(fpf.SourceUnitRolePatternBody),
			Body: e11HistoricalReadPathPrefix +
				"`splits(SYSTEM-IN-CONTEXT -> {SYSTEM-RECOGNITION, SYSTEM-DELIMITATION, WORDING, ARCHITECTURE})`.",
			PatternID:  "E.10",
			SourcePath: "data/FPF/FPF-Spec.md",
			StartLine:  60,
			EndLine:    60,
		},
	}}
	splits, err := sourceAuthoredPracticalUseSplits(index)
	if err != nil {
		t.Fatalf("sourceAuthoredPracticalUseSplits(similar prose) error = %v", err)
	}
	if len(splits) != 0 {
		t.Fatalf("similar-name prose emitted continuity: %#v", splits)
	}

	index.units["malformed"] = analysisSourceUnit{
		UnitID: "malformed",
		Role:   string(fpf.SourceUnitRolePatternBody),
		Body: e11HistoricalReadPathPrefix +
			"`splits(SYSTEM-IN-CONTEXT -> {SYSTEM-RECOGNITION, SYSTEM-DELIMITATION, WORDING, ARCHITECTURE})`.junk",
		PatternID:  "E.11",
		SourcePath: "data/FPF/FPF-Spec.md",
		StartLine:  70,
		EndLine:    70,
	}
	if _, err := sourceAuthoredPracticalUseSplits(index); err == nil ||
		!strings.Contains(err.Error(), "unsupported splits(...) grammar") {
		t.Fatalf("malformed direct suffix error = %v", err)
	}
}

func TestProductionCorpusCarriesExactE11SplitContinuitySpan(t *testing.T) {
	t.Parallel()

	index, err := loadAnalysisIndex(filepath.Join("..", "cli", "fpf.db"))
	if err != nil {
		t.Fatalf("load production FPF index: %v", err)
	}
	splits, err := sourceAuthoredPracticalUseSplits(index)
	if err != nil {
		t.Fatalf("sourceAuthoredPracticalUseSplits(production) error = %v", err)
	}
	if len(splits) != 1 {
		t.Fatalf("production E.11 splits = %d, want 1: %#v", len(splits), splits)
	}
	if splits[0].predecessor != "SYSTEM-IN-CONTEXT" ||
		!reflect.DeepEqual(splits[0].successors, []string{
			"SYSTEM-RECOGNITION",
			"SYSTEM-DELIMITATION",
			"WORDING",
			"ARCHITECTURE",
		}) || splits[0].sourceRef != "data/FPF/FPF-Spec.md:76915-76915" {
		t.Fatalf("production E.11 split = %#v", splits[0])
	}
}

func TestClassifyLocalPracticeCompatibilityClosedResults(t *testing.T) {
	t.Parallel()

	predecessor := mustAnalysisTypeEnvRef(t, "typeenv:"+analysisDigest("1"))
	candidate := mustAnalysisTypeEnvRef(t, "typeenv:"+analysisDigest("2"))
	unavailable := mustAnalysisTypeEnvRef(t, "typeenv:"+analysisDigest("3"))
	tests := []struct {
		name    string
		carrier typedmemory.TypeEnvRef
		rules   []analysisSuccessorRule
		want    LocalPracticeCompatibilityResult
	}{
		{
			name:    "exact",
			carrier: candidate,
			want:    LocalPracticeExact,
		},
		{
			name:    "compatible successor candidate possible",
			carrier: predecessor,
			rules: []analysisSuccessorRule{analysisRule(
				t,
				projecttypeenvcompatibility.RelationSlotFamily,
				"added",
				projecttypeenvcompatibility.SuccessorAdditive,
				projecttypeenvcompatibility.GroundSemanticCoordinateAdded,
				"",
				"4",
			)},
			want: LocalPracticeCompatibleSuccessorCandidatePossible,
		},
		{
			name:    "semantic review required",
			carrier: predecessor,
			rules: []analysisSuccessorRule{analysisRule(
				t,
				projecttypeenvcompatibility.RelationSlotFamily,
				"narrowed",
				projecttypeenvcompatibility.SuccessorNarrowed,
				projecttypeenvcompatibility.GroundCardinalityDomainRestricted,
				"5",
				"6",
			)},
			want: LocalPracticeSemanticReviewRequired,
		},
		{
			name:    "compiler gap",
			carrier: predecessor,
			rules: []analysisSuccessorRule{analysisRule(
				t,
				projecttypeenvcompatibility.RelationSlotFamily,
				"compiler-gap",
				projecttypeenvcompatibility.SuccessorCompilerGap,
				projecttypeenvcompatibility.GroundSemanticOrderNotImplemented,
				"7",
				"8",
			)},
			want: LocalPracticeCompilerGap,
		},
		{
			name:    "unavailable carrier basis is compiler gap",
			carrier: unavailable,
			want:    LocalPracticeCompilerGap,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := classifyLocalPracticeCompatibility(
				test.carrier,
				predecessor,
				candidate,
				test.rules,
			)
			if err != nil {
				t.Fatalf("classifyLocalPracticeCompatibility() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("result = %s, want %s", got.String(), test.want.String())
			}
		})
	}
}

func TestProductionLocalPracticeCompatibilityUsesExecutableSuccessor(t *testing.T) {
	t.Parallel()

	historicalRef := mustAnalysisTypeEnvRef(
		t,
		basetypeenvartifacts.HistoricalV5Ref,
	)
	historicalArtifact, err := basetypeenvartifacts.LoadExact(historicalRef)
	if err != nil {
		t.Fatalf("load historical Base TypeEnv: %v", err)
	}
	historicalEnvironment, err := typeenv.LowerBaseTypeEnvArtifact(historicalArtifact)
	if err != nil {
		t.Fatalf("lower historical Base TypeEnv: %v", err)
	}
	currentDatabase := filepath.Join("..", "cli", "fpf.db")
	currentEnvironment, err := loadExecutableBaseTypeEnv(currentDatabase)
	if err != nil {
		t.Fatalf("load current executable Base TypeEnv: %v", err)
	}
	currentIndex, err := loadAnalysisIndex(currentDatabase)
	if err != nil {
		t.Fatalf("load current FPF index: %v", err)
	}
	baseDiff, err := projecttypeenvcompatibility.CompareSuccessor(
		historicalEnvironment,
		currentEnvironment,
	)
	if err != nil {
		t.Fatalf("compare production Base TypeEnvs: %v", err)
	}
	classCounts := make(map[projecttypeenvcompatibility.SuccessorRuleClass]int)
	for _, rule := range baseDiff.Rules() {
		classCounts[rule.Class()]++
	}
	t.Logf(
		"production Base successor rules: unchanged=%d additive=%d widened=%d narrowed=%d removed=%d compiler_gap=%d",
		classCounts[projecttypeenvcompatibility.SuccessorUnchanged],
		classCounts[projecttypeenvcompatibility.SuccessorAdditive],
		classCounts[projecttypeenvcompatibility.SuccessorWidened],
		classCounts[projecttypeenvcompatibility.SuccessorNarrowed],
		classCounts[projecttypeenvcompatibility.SuccessorRemoved],
		classCounts[projecttypeenvcompatibility.SuccessorCompilerGap],
	)
	assessment, deltas, diagnostics, err := compareLocalPracticeCandidateAgainstEnvironments(
		filepath.Join(
			"..",
			"..",
			"data",
			"haft",
			"local-practice",
			"typed-memory",
			"candidates",
			"1.4.0.yaml",
		),
		historicalEnvironment,
		currentEnvironment,
		currentIndex.meta["fpf_commit"],
		currentIndex.meta["spec_document_digest"],
	)
	if err != nil {
		t.Fatalf("compare production Local-Practice successor: %v", err)
	}
	wantResult := LocalPracticeCompatibleSuccessorCandidatePossible
	if classCounts[projecttypeenvcompatibility.SuccessorCompilerGap] > 0 {
		wantResult = LocalPracticeCompilerGap
	} else if classCounts[projecttypeenvcompatibility.SuccessorNarrowed] > 0 ||
		classCounts[projecttypeenvcompatibility.SuccessorRemoved] > 0 {
		wantResult = LocalPracticeSemanticReviewRequired
	}
	if assessment.Result() != wantResult {
		t.Fatalf(
			"production historical/current result = %s, want %s from executable rules",
			assessment.Result().String(),
			wantResult.String(),
		)
	}
	if assessment.CarrierBase() != historicalRef ||
		assessment.PredecessorBase() != historicalRef ||
		assessment.CandidateBase() != currentEnvironment.Ref() {
		t.Fatalf("production assessment coordinates = %#v", assessment)
	}
	if len(deltas) == 0 || len(diagnostics) != 0 {
		t.Fatalf("production successor emitted %d deltas/%d diagnostics", len(deltas), len(diagnostics))
	}

	currentAssessment, currentDeltas, currentDiagnostics, err := compareLocalPracticeCandidate(
		filepath.Join(
			"..",
			"..",
			"data",
			"haft",
			"local-practice",
			"typed-memory",
			"candidates",
			"1.5.0.yaml",
		),
		currentDatabase,
		currentDatabase,
		currentIndex.meta["fpf_commit"],
		currentIndex.meta["spec_document_digest"],
	)
	if err != nil {
		t.Fatalf("compare current production Local-Practice basis: %v", err)
	}
	if currentAssessment.Result() != LocalPracticeExact ||
		len(currentDeltas) != 0 || len(currentDiagnostics) != 0 {
		t.Fatalf(
			"current production assessment = %s with %d deltas/%d diagnostics",
			currentAssessment.Result().String(),
			len(currentDeltas),
			len(currentDiagnostics),
		)
	}
}

func TestBuildCompatibilityReportCarriesExactProductionLocalPracticeAssessment(
	t *testing.T,
) {
	t.Parallel()

	lockBytes, err := os.ReadFile(filepath.Join(
		"..",
		"..",
		"data",
		"haft",
		"fpf-integration.lock.json",
	))
	if err != nil {
		t.Fatalf("read production integration lock: %v", err)
	}
	lock, err := ParseIntegrationLock(lockBytes)
	if err != nil {
		t.Fatalf("parse production integration lock: %v", err)
	}
	databasePath := filepath.Join("..", "cli", "fpf.db")
	report, err := BuildCompatibilityReport(CompatibilityReportInput{
		ToolRevision:                 strings.Repeat("f", 40),
		Predecessor:                  lock.Coordinates,
		Candidate:                    lock.Coordinates,
		PredecessorDatabasePath:      databasePath,
		CandidateDatabasePath:        databasePath,
		LatestLocalPracticeCandidate: filepath.Join("..", "..", DefaultLocalPracticeCandidateRelative),
	})
	if err != nil {
		t.Fatalf("BuildCompatibilityReport() error = %v", err)
	}
	assessment, exists := report.LocalPracticeCompatibility()
	if !exists || assessment.Result() != LocalPracticeExact {
		t.Fatalf("production report Local-Practice assessment = %#v/%t", assessment, exists)
	}
	if report.Outcome().State() != StateNoChange {
		t.Fatalf("same-snapshot result = %s, want no_change", report.Outcome().State().String())
	}
	if !strings.Contains(
		string(report.CanonicalBytes()),
		`"local_practice_compatibility":{"result":"exact"`,
	) {
		t.Fatalf("canonical report omits exact assessment: %s", report.CanonicalBytes())
	}
}

func TestBuildCompatibilityReportRequiresLocalPracticeAssessment(t *testing.T) {
	t.Parallel()

	_, err := BuildCompatibilityReport(CompatibilityReportInput{})
	if err == nil || !strings.Contains(
		err.Error(),
		"requires the latest repo-owned Local-Practice candidate",
	) {
		t.Fatalf("missing Local-Practice candidate error = %v", err)
	}
}

func TestCompareLocalPracticeCandidateInspectsTypedFPFSourcePins(t *testing.T) {
	t.Parallel()

	const pinCount = 12
	candidateBase := "typeenv:" + analysisDigest("a")
	candidateTypeEnvRef := mustAnalysisTypeEnvRef(t, candidateBase)
	candidateRevision := strings.Repeat("b", 40)
	candidateSpecDigest := analysisDigest("c")
	compareExact := func(path string) (
		LocalPracticeCompatibilityAssessment,
		[]Delta,
		[]Diagnostic,
		error,
	) {
		return compareLocalPracticeCandidateAgainstSuccessor(
			path,
			candidateTypeEnvRef,
			candidateTypeEnvRef,
			[]analysisSuccessorRule(nil),
			candidateRevision,
			candidateSpecDigest,
		)
	}

	t.Run("matching", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "candidate.yaml")
		writeLocalPracticeCandidate(
			t,
			path,
			candidateBase,
			candidateRevision,
			candidateSpecDigest,
		)
		assessment, deltas, diagnostics, err := compareExact(path)
		if err != nil {
			t.Fatalf("compareLocalPracticeCandidate() error = %v", err)
		}
		if assessment.Result() != LocalPracticeExact {
			t.Fatalf("matching carrier assessment = %s, want exact", assessment.Result().String())
		}
		if len(deltas) != 0 || len(diagnostics) != 0 {
			t.Fatalf("matching carrier = %d deltas/%d diagnostics", len(deltas), len(diagnostics))
		}
	})

	for _, test := range []struct {
		name       string
		edition    string
		digest     string
		wantBefore string
	}{
		{
			name:       "stale edition",
			edition:    strings.Repeat("d", 40),
			digest:     candidateSpecDigest,
			wantBefore: strings.Repeat("d", 40) + "@" + candidateSpecDigest,
		},
		{
			name:       "stale digest",
			edition:    candidateRevision,
			digest:     analysisDigest("e"),
			wantBefore: candidateRevision + "@" + analysisDigest("e"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "candidate.yaml")
			writeLocalPracticeCandidate(t, path, candidateBase, test.edition, test.digest)
			firstAssessment, firstDeltas, firstDiagnostics, err := compareExact(path)
			if err != nil {
				t.Fatalf("compareLocalPracticeCandidate(first) error = %v", err)
			}
			secondAssessment, secondDeltas, secondDiagnostics, err := compareExact(path)
			if err != nil {
				t.Fatalf("compareLocalPracticeCandidate(second) error = %v", err)
			}
			if firstAssessment != secondAssessment ||
				!reflect.DeepEqual(firstDeltas, secondDeltas) ||
				!reflect.DeepEqual(firstDiagnostics, secondDiagnostics) {
				t.Fatal("Local-Practice source-pin comparison is not deterministic")
			}
			if len(firstDeltas) != pinCount || len(firstDiagnostics) != 0 {
				t.Fatalf(
					"stale carrier = %d deltas/%d diagnostics, want %d/0",
					len(firstDeltas),
					len(firstDiagnostics),
					pinCount,
				)
			}
			seen := make(map[string]struct{}, pinCount)
			for _, delta := range firstDeltas {
				if delta.Family() != DeltaSpecCarrierReferences ||
					delta.Kind() != DeltaSpecSemanticReviewRequired ||
					delta.Before() != test.wantBefore ||
					delta.After() != candidateRevision+"@"+candidateSpecDigest {
					t.Fatalf("unexpected spec-carrier delta: %#v", delta)
				}
				if !strings.HasPrefix(
					delta.SourceRef(),
					DefaultLocalPracticeCandidateRelative+":",
				) || strings.Contains(delta.SourceRef(), filepath.Dir(path)) {
					t.Fatalf("unstable spec-carrier source ref %q", delta.SourceRef())
				}
				if _, duplicate := seen[delta.Subject()]; duplicate {
					t.Fatalf("duplicate spec-carrier subject %q", delta.Subject())
				}
				seen[delta.Subject()] = struct{}{}
			}
		})
	}

	t.Run("base mismatch remains separate", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "candidate.yaml")
		writeLocalPracticeCandidate(
			t,
			path,
			"typeenv:"+analysisDigest("f"),
			candidateRevision,
			candidateSpecDigest,
		)
		assessment, deltas, diagnostics, err := compareLocalPracticeCandidateAgainstSuccessor(
			path,
			candidateTypeEnvRef,
			candidateTypeEnvRef,
			[]analysisSuccessorRule(nil),
			candidateRevision,
			candidateSpecDigest,
		)
		if err != nil {
			t.Fatalf("compareLocalPracticeCandidate() error = %v", err)
		}
		if len(deltas) != 1 || len(diagnostics) != 0 ||
			deltas[0].Family() != DeltaLocalPracticeBasis {
			t.Fatalf("base mismatch projection = %#v / %#v", deltas, diagnostics)
		}
		if assessment.Result() != LocalPracticeCompilerGap {
			t.Fatalf("unavailable carrier basis = %s, want compiler_gap", assessment.Result().String())
		}
	})

	t.Run("malformed pin fails closed", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "candidate.yaml")
		content := localPracticeCandidateBytes(
			t,
			candidateBase,
			candidateRevision,
			candidateSpecDigest,
		)
		content = []byte(strings.Replace(
			string(content),
			"digest: "+candidateSpecDigest,
			"digest: malformed",
			1,
		))
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, _, err := compareExact(path)
		if err == nil || !strings.Contains(err.Error(), "reference_scheme.digest") {
			t.Fatalf("malformed pin error = %v", err)
		}
	})

	t.Run("contradictory pins fail closed", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "candidate.yaml")
		content := localPracticeCandidateBytes(
			t,
			candidateBase,
			candidateRevision,
			candidateSpecDigest,
		)
		content = []byte(strings.Replace(
			string(content),
			"edition: "+candidateRevision,
			"edition: "+strings.Repeat("f", 40),
			1,
		))
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, _, err := compareExact(path)
		if err == nil || !strings.Contains(err.Error(), "contradictory FPF-Spec pins") {
			t.Fatalf("contradictory pin error = %v", err)
		}
	})
}

func TestReportSnapshotsFromIntegrationCoordinatesIsDeterministic(t *testing.T) {
	t.Parallel()

	predecessorCoordinates := IntegrationCoordinates{
		SourceRevision:         strings.Repeat("1", 40),
		ReadmeDocumentDigest:   analysisDigest("2"),
		SpecDocumentDigest:     analysisDigest("3"),
		DatabaseDigest:         analysisDigest("4"),
		SourceUnitCount:        12,
		IndexSchemaVersion:     "11",
		BaseTypeEnvRef:         "typeenv:" + analysisDigest("5"),
		BaseTypeEnvDigest:      analysisDigest("5"),
		TypeEnvCompilerEdition: "fpf-base-typeenv.cov2.v4",
	}
	candidateCoordinates := IntegrationCoordinates{
		SourceRevision:         strings.Repeat("6", 40),
		ReadmeDocumentDigest:   analysisDigest("7"),
		SpecDocumentDigest:     analysisDigest("8"),
		DatabaseDigest:         analysisDigest("9"),
		SourceUnitCount:        13,
		IndexSchemaVersion:     "12",
		BaseTypeEnvRef:         "typeenv:" + analysisDigest("a"),
		BaseTypeEnvDigest:      analysisDigest("a"),
		TypeEnvCompilerEdition: "fpf-base-typeenv.cov2.v5",
	}

	firstPredecessor, firstCandidate, err := ReportSnapshotsFromIntegrationCoordinates(
		predecessorCoordinates,
		candidateCoordinates,
	)
	if err != nil {
		t.Fatalf("ReportSnapshotsFromIntegrationCoordinates(first) error = %v", err)
	}
	secondPredecessor, secondCandidate, err := ReportSnapshotsFromIntegrationCoordinates(
		predecessorCoordinates,
		candidateCoordinates,
	)
	if err != nil {
		t.Fatalf("ReportSnapshotsFromIntegrationCoordinates(second) error = %v", err)
	}
	if !reflect.DeepEqual(firstPredecessor, secondPredecessor) ||
		!reflect.DeepEqual(firstCandidate, secondCandidate) {
		t.Fatal("coordinate conversion is not deterministic")
	}
	if firstPredecessor.Source().Revision().String() != predecessorCoordinates.SourceRevision {
		t.Fatalf("predecessor revision = %q", firstPredecessor.Source().Revision().String())
	}
	if firstPredecessor.Derived().SourceUnitCount() != uint64(predecessorCoordinates.SourceUnitCount) {
		t.Fatalf("predecessor source-unit count = %d", firstPredecessor.Derived().SourceUnitCount())
	}
	candidateDerived, exists := firstCandidate.Derived()
	if !exists {
		t.Fatal("candidate derived coordinates are absent")
	}
	if candidateDerived.BaseTypeEnvRef().String() != candidateCoordinates.BaseTypeEnvRef ||
		candidateDerived.DatabaseDigest().String() != candidateCoordinates.DatabaseDigest ||
		candidateDerived.IndexSchemaVersion() != 12 {
		t.Fatalf("candidate derived coordinates = %#v", candidateDerived)
	}

	// Conversion owns its strong values and does not mutate the public lock
	// coordinates supplied by callers.
	if predecessorCoordinates.SourceUnitCount != 12 ||
		candidateCoordinates.TypeEnvCompilerEdition != "fpf-base-typeenv.cov2.v5" {
		t.Fatal("coordinate conversion mutated its input")
	}

	invalid := predecessorCoordinates
	invalid.SourceUnitCount = -1
	if _, _, err := ReportSnapshotsFromIntegrationCoordinates(
		invalid,
		candidateCoordinates,
	); err == nil || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("negative source-unit count error = %v, want positive-count rejection", err)
	}
}

type analysisFixture struct {
	revision       string
	readmeDigest   string
	specDigest     string
	typeEnvDigest  string
	units          []analysisFixtureUnit
	typeEnvChanges []analysisTypeEnvChange
}

type analysisFixtureUnit struct {
	unitID          string
	sourceID        string
	role            string
	title           string
	patternID       string
	directRefsJSON  string
	relationsDigest string
	useCuesJSON     string
}

type analysisTypeEnvChange struct {
	ordinal   int
	symbol    string
	kind      string
	rationale string
}

type analysisSuccessorRule struct {
	family    projecttypeenvcompatibility.Family
	key       string
	class     projecttypeenvcompatibility.SuccessorRuleClass
	ground    projecttypeenvcompatibility.SuccessorRuleGround
	before    typedmemory.SHA256Digest
	after     typedmemory.SHA256Digest
	hasBefore bool
	hasAfter  bool
}

func (rule analysisSuccessorRule) Family() projecttypeenvcompatibility.Family {
	return rule.family
}

func (rule analysisSuccessorRule) Key() string {
	return rule.key
}

func (rule analysisSuccessorRule) Class() projecttypeenvcompatibility.SuccessorRuleClass {
	return rule.class
}

func (rule analysisSuccessorRule) Ground() projecttypeenvcompatibility.SuccessorRuleGround {
	return rule.ground
}

func (rule analysisSuccessorRule) BeforeDigest() (typedmemory.SHA256Digest, bool) {
	return rule.before, rule.hasBefore
}

func (rule analysisSuccessorRule) AfterDigest() (typedmemory.SHA256Digest, bool) {
	return rule.after, rule.hasAfter
}

func analysisRule(
	t *testing.T,
	family projecttypeenvcompatibility.Family,
	key string,
	class projecttypeenvcompatibility.SuccessorRuleClass,
	ground projecttypeenvcompatibility.SuccessorRuleGround,
	beforeSeed string,
	afterSeed string,
) analysisSuccessorRule {
	t.Helper()
	rule := analysisSuccessorRule{
		family: family,
		key:    key,
		class:  class,
		ground: ground,
	}
	if beforeSeed != "" {
		before, err := typedmemory.NewSHA256Digest(analysisDigest(beforeSeed))
		if err != nil {
			t.Fatalf("construct predecessor rule digest: %v", err)
		}
		rule.before = before
		rule.hasBefore = true
	}
	if afterSeed != "" {
		after, err := typedmemory.NewSHA256Digest(analysisDigest(afterSeed))
		if err != nil {
			t.Fatalf("construct candidate rule digest: %v", err)
		}
		rule.after = after
		rule.hasAfter = true
	}
	return rule
}

func mustAnalysisTypeEnvRef(t *testing.T, raw string) typedmemory.TypeEnvRef {
	t.Helper()
	reference, err := typedmemory.ParseTypeEnvRef(raw)
	if err != nil {
		t.Fatalf("parse TypeEnv reference %q: %v", raw, err)
	}
	return reference
}

func analysisUnit(
	unitID string,
	sourceID string,
	role fpf.SourceUnitRole,
	title string,
	patternID string,
	directRefsJSON string,
	relationsDigest string,
	useCuesJSON string,
) analysisFixtureUnit {
	return analysisFixtureUnit{
		unitID:          unitID,
		sourceID:        sourceID,
		role:            string(role),
		title:           title,
		patternID:       patternID,
		directRefsJSON:  directRefsJSON,
		relationsDigest: relationsDigest,
		useCuesJSON:     useCuesJSON,
	}
}

func writeAnalysisDatabase(t *testing.T, path string, fixture analysisFixture) {
	t.Helper()
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open(%q) error = %v", path, err)
	}
	t.Cleanup(func() { _ = database.Close() })
	for _, statement := range []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE source_units (
			unit_id TEXT PRIMARY KEY,
			source_id TEXT NOT NULL,
			source_role TEXT NOT NULL,
			title TEXT NOT NULL,
			pattern_id TEXT NOT NULL,
			direct_refs_json TEXT NOT NULL,
			relations_digest TEXT NOT NULL,
			use_cues_json TEXT NOT NULL,
			source_path TEXT NOT NULL,
			start_line INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			content_hash TEXT NOT NULL,
			source_revision TEXT NOT NULL
		)`,
		`CREATE TABLE fpf_typeenv_compatibility_changes (
			change_ordinal INTEGER PRIMARY KEY,
			symbol TEXT NOT NULL UNIQUE,
			change_kind TEXT NOT NULL,
			rationale TEXT NOT NULL
		)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("create analysis fixture schema: %v", err)
		}
	}
	meta := map[string]string{
		"fpf_commit":                      fixture.revision,
		"indexed_source_units":            fmt.Sprintf("%d", len(fixture.units)),
		"readme_document_digest":          fixture.readmeDigest,
		"schema_version":                  "11",
		"spec_document_digest":            fixture.specDigest,
		"typeenv_artifact_digest":         fixture.typeEnvDigest,
		"typeenv_compiler_schema_version": "fpf-base-typeenv.cov2.v4",
		"typeenv_ref":                     "typeenv:" + fixture.typeEnvDigest,
		"typeenv_source_revision":         fixture.revision,
	}
	metaKeys := make([]string, 0, len(meta))
	for key := range meta {
		metaKeys = append(metaKeys, key)
	}
	slices.Sort(metaKeys)
	for _, key := range metaKeys {
		if _, err := database.Exec(
			`INSERT INTO meta(key, value) VALUES (?, ?)`,
			key,
			meta[key],
		); err != nil {
			t.Fatalf("insert meta %q: %v", key, err)
		}
	}
	for _, unit := range fixture.units {
		if _, err := database.Exec(
			`INSERT INTO source_units (
				unit_id, source_id, source_role, title, pattern_id,
				direct_refs_json, relations_digest, use_cues_json,
				source_path, start_line, end_line, content_hash, source_revision
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			unit.unitID,
			unit.sourceID,
			unit.role,
			unit.title,
			unit.patternID,
			unit.directRefsJSON,
			unit.relationsDigest,
			unit.useCuesJSON,
			"data/FPF/FPF-Spec.md",
			10,
			20,
			analysisDigest("f"),
			fixture.revision,
		); err != nil {
			t.Fatalf("insert source unit %q: %v", unit.unitID, err)
		}
	}
	for _, change := range fixture.typeEnvChanges {
		if _, err := database.Exec(
			`INSERT INTO fpf_typeenv_compatibility_changes (
				change_ordinal, symbol, change_kind, rationale
			) VALUES (?, ?, ?, ?)`,
			change.ordinal,
			change.symbol,
			change.kind,
			change.rationale,
		); err != nil {
			t.Fatalf("insert TypeEnv change %q: %v", change.symbol, err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close analysis fixture database: %v", err)
	}
}

const (
	historicalLocalPracticeBaseTypeEnvRef = "typeenv:sha256:effff65cae9eaf1aba287245df79c460fbeaee5f666dcaa7992bfeb251c1e35e"
	historicalLocalPracticeFPFRevision    = "2ada413629b846ef308222d16489a82cb5b40a71"
	historicalLocalPracticeFPFDigest      = "sha256:00e8213ed4f2ab548ea16118b0559d72c1fc9c9baedd025891eeed160d5143af"
)

func writeLocalPracticeCandidate(
	t *testing.T,
	path string,
	baseTypeEnvRef string,
	sourceRevision string,
	specDigest string,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create Local-Practice fixture directory: %v", err)
	}
	content := localPracticeCandidateBytes(
		t,
		baseTypeEnvRef,
		sourceRevision,
		specDigest,
	)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write Local-Practice fixture: %v", err)
	}
}

func localPracticeCandidateBytes(
	t *testing.T,
	baseTypeEnvRef string,
	sourceRevision string,
	specDigest string,
) []byte {
	t.Helper()
	path := filepath.Join(
		"..",
		"..",
		"data",
		"haft",
		"local-practice",
		"typed-memory",
		"candidates",
		"1.4.0.yaml",
	)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read typed Local-Practice fixture %q: %v", path, err)
	}
	raw := string(content)
	if count := strings.Count(raw, historicalLocalPracticeBaseTypeEnvRef); count != 1 {
		t.Fatalf("historical Local-Practice Base TypeEnv ref count = %d, want 1", count)
	}
	const pinCount = 12
	if count := strings.Count(raw, historicalLocalPracticeFPFRevision); count != pinCount {
		t.Fatalf("historical Local-Practice FPF edition count = %d, want %d", count, pinCount)
	}
	if count := strings.Count(raw, historicalLocalPracticeFPFDigest); count != pinCount {
		t.Fatalf("historical Local-Practice FPF digest count = %d, want %d", count, pinCount)
	}
	raw = strings.Replace(raw, historicalLocalPracticeBaseTypeEnvRef, baseTypeEnvRef, 1)
	raw = strings.ReplaceAll(raw, historicalLocalPracticeFPFRevision, sourceRevision)
	raw = strings.ReplaceAll(raw, historicalLocalPracticeFPFDigest, specDigest)
	return []byte(raw)
}

func fixtureIntegrationCoordinates(
	fixture analysisFixture,
	databaseDigest string,
) IntegrationCoordinates {
	return IntegrationCoordinates{
		SourceRevision:         fixture.revision,
		ReadmeDocumentDigest:   fixture.readmeDigest,
		SpecDocumentDigest:     fixture.specDigest,
		DatabaseDigest:         databaseDigest,
		SourceUnitCount:        len(fixture.units),
		IndexSchemaVersion:     "11",
		BaseTypeEnvRef:         "typeenv:" + fixture.typeEnvDigest,
		BaseTypeEnvDigest:      fixture.typeEnvDigest,
		TypeEnvCompilerEdition: "fpf-base-typeenv.cov2.v4",
	}
}

func assertAnalysisDelta(
	t *testing.T,
	deltas []Delta,
	family DeltaFamily,
	kind DeltaKind,
	subject string,
	before string,
	after string,
) Delta {
	t.Helper()
	for _, delta := range deltas {
		if delta.Family() != family ||
			delta.Kind() != kind ||
			delta.Subject() != subject {
			continue
		}
		if delta.Before() != before || delta.After() != after {
			t.Fatalf(
				"%s/%s %q = %q -> %q, want %q -> %q",
				family,
				kind,
				subject,
				delta.Before(),
				delta.After(),
				before,
				after,
			)
		}
		return delta
	}
	t.Fatalf(
		"missing delta %s/%s %q:\n%s",
		family,
		kind,
		subject,
		describeAnalysisDeltas(deltas),
	)
	return Delta{}
}

func assertAnalysisDeltaKind(
	t *testing.T,
	deltas []Delta,
	family DeltaFamily,
	kind DeltaKind,
	subject string,
) Delta {
	t.Helper()
	for _, delta := range deltas {
		if delta.Family() == family &&
			delta.Kind() == kind &&
			delta.Subject() == subject {
			return delta
		}
	}
	t.Fatalf(
		"missing delta %s/%s %q:\n%s",
		family,
		kind,
		subject,
		describeAnalysisDeltas(deltas),
	)
	return Delta{}
}

func describeAnalysisDeltas(deltas []Delta) string {
	var builder strings.Builder
	for _, delta := range deltas {
		fmt.Fprintf(
			&builder,
			"%s/%s %s: %q -> %q\n",
			delta.Family(),
			delta.Kind(),
			delta.Subject(),
			delta.Before(),
			delta.After(),
		)
	}
	return builder.String()
}

func analysisDigest(character string) string {
	return "sha256:" + strings.Repeat(character, 64)
}
