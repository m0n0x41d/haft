//go:build query_token_gate

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpfrefresh"
)

const (
	embeddedTokenGateSchema            = "haft.fpf-query-token-gate/v1"
	embeddedTokenGateResultSchema      = "haft.fpf-query-token-gate-result/v1"
	embeddedTokenGateEncoding          = "o200k_base"
	embeddedTokenGateTokenizerVersion  = "0.9.0"
	embeddedTokenGateEncodingAssetHash = "446a9538cb6c348e3516120d7c08b09f57c36495e2acfffe59a5bf8b0cfb1a2d"
	embeddedTokenGateCalibrationTokens = 21
	embeddedTokenGateMinimumReduction  = 0.30
)

var embeddedTokenGateDefaultBudget = fpf.ResponseBudget{
	MaxCandidatesPerRole:     5,
	MaxTotalCandidates:       10,
	MaxExcerptCharacters:     480,
	MaxRelationsPerCandidate: 12,
}

type embeddedTokenGateCase struct {
	CaseID                     string              `json:"case_id"`
	Request                    fpf.ConcernQuery    `json:"request"`
	ExpectedKind               fpf.QueryResultKind `json:"expected_kind"`
	ExpectedCandidateCount     int                 `json:"expected_candidate_count"`
	ExpectedCandidateIDs       []string            `json:"expected_candidate_ids"`
	ExpectedCandidateSourceIDs []string            `json:"expected_candidate_source_ids,omitempty"`
	ExpectedTruncationApplied  bool                `json:"expected_truncation_applied"`
	ExpectedOmittedAtLeast     int                 `json:"expected_omitted_at_least"`
	ExpectedTruncationBasis    []string            `json:"expected_truncation_basis"`
}

type embeddedTokenGateFixture struct {
	SchemaVersion   string                  `json:"schema_version"`
	FixtureRevision string                  `json:"fixture_revision"`
	Cases           []embeddedTokenGateCase `json:"cases"`
}

type embeddedTokenGateBasis struct {
	DatabaseImage []byte
	Fixture       embeddedTokenGateFixture
	Lock          fpfrefresh.IntegrationLock
}

type embeddedTokenGateArtifactPaths struct {
	DatabasePath string
	FixturePath  string
	LockPath     string
}

type embeddedTokenGateInput struct {
	Schema string                       `json:"schema"`
	Cases  []embeddedTokenGateInputCase `json:"cases"`
}

type embeddedTokenGateInputCase struct {
	CaseID        string `json:"case_id"`
	CanonicalJSON string `json:"canonical_json"`
	WorkingJSON   string `json:"working_json"`
}

type embeddedTokenGateOutput struct {
	Schema                string                        `json:"schema"`
	Encoding              string                        `json:"encoding"`
	TokenizerDistribution string                        `json:"tokenizer_distribution"`
	TokenizerVersion      string                        `json:"tokenizer_version"`
	EncodingAssetSHA256   string                        `json:"encoding_asset_sha256"`
	CalibrationTokens     int                           `json:"calibration_tokens"`
	PythonImplementation  string                        `json:"python_implementation"`
	PythonVersion         string                        `json:"python_version"`
	Counts                []embeddedTokenGateOutputCase `json:"counts"`
}

type embeddedTokenGateOutputCase struct {
	CaseID          string `json:"case_id"`
	CanonicalTokens int    `json:"canonical_tokens"`
	WorkingTokens   int    `json:"working_tokens"`
}

type embeddedWorkingCandidateSet struct {
	View     fpf.QueryPublicationView `json:"view"`
	TraceRef fpf.QueryTraceRef        `json:"trace_ref"`
	fpf.PublishedCandidateSetFields
}

func TestFPFQueryWorkingViewEmbeddedO200kAcceptance(t *testing.T) {
	basis := loadEmbeddedTokenGateBasis(t)
	assertEmbeddedTokenGateDatabaseDigest(
		t,
		basis.DatabaseImage,
		basis.Lock.Coordinates.DatabaseDigest,
	)
	corpus := basis.Fixture.Cases

	db, cleanup, err := openFPFDBImage(context.Background(), basis.DatabaseImage)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := fpf.VerifySourceQueryIndexReadOnlyDB(db); err != nil {
		t.Fatalf("verify embedded FPF Query index: %v", err)
	}
	snapshot, err := fpf.LoadQuerySourceSnapshot(db)
	if err != nil {
		t.Fatalf("load embedded FPF Query source snapshot: %v", err)
	}
	assertEmbeddedTokenGateSnapshot(t, snapshot, basis.Lock.Coordinates)

	publicationRequest, err := fpf.NewQueryPublicationRequest("working", "")
	if err != nil {
		t.Fatal(err)
	}
	index := fpf.NewSQLiteQueryIndex(db)
	inputCases := make([]embeddedTokenGateInputCase, 0, len(corpus))
	allCasesMatched := true
	for _, testCase := range corpus {
		caseMatched := t.Run(testCase.CaseID, func(t *testing.T) {
			inputCase := evaluateEmbeddedTokenGateCase(
				t,
				index,
				snapshot,
				publicationRequest,
				testCase,
			)
			inputCases = append(inputCases, inputCase)
		})
		allCasesMatched = allCasesMatched && caseMatched
	}
	if !allCasesMatched {
		t.FailNow()
	}

	measurements := runEmbeddedTokenGateCounter(t, inputCases)
	assertEmbeddedTokenGateReduction(t, measurements)
}

func evaluateEmbeddedTokenGateCase(
	t *testing.T,
	index fpf.QueryIndex,
	snapshot fpf.QuerySourceSnapshot,
	publicationRequest fpf.QueryPublicationRequest,
	testCase embeddedTokenGateCase,
) embeddedTokenGateInputCase {
	t.Helper()
	evaluation, err := fpf.EvaluateQuery(index, testCase.Request)
	if err != nil {
		t.Fatalf("evaluate real embedded concern: %v", err)
	}
	canonical := evaluation.Result()
	candidateSet, ok := canonical.(fpf.CandidateSet)
	if !ok {
		t.Fatalf("canonical result = %T, want CandidateSet", canonical)
	}
	assertEmbeddedTokenGateCanonicalResult(t, testCase, candidateSet)

	execution, err := fpf.NewCanonicalQueryExecution(
		testCase.Request,
		evaluation,
		snapshot,
	)
	if err != nil {
		t.Fatalf("canonical execution: %v", err)
	}
	published, err := fpf.ProjectQueryResult(execution, publicationRequest)
	if err != nil {
		t.Fatalf("working projection: %v", err)
	}
	workingJSON, err := fpf.EncodePublishedQuery(published, fpf.PublishedQueryJSONCompact)
	if err != nil {
		t.Fatalf("compact working JSON: %v", err)
	}
	assertEmbeddedTokenGateWorkingSemantics(t, candidateSet, workingJSON)

	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		t.Fatalf("compact canonical JSON: %v", err)
	}
	return embeddedTokenGateInputCase{
		CaseID:        testCase.CaseID,
		CanonicalJSON: string(canonicalJSON),
		WorkingJSON:   string(workingJSON),
	}
}

func loadEmbeddedTokenGateBasis(t *testing.T) embeddedTokenGateBasis {
	t.Helper()
	paths, err := resolveEmbeddedTokenGateArtifactPaths(os.Getenv)
	if err != nil {
		t.Fatalf("resolve token-gate artifact paths: %v", err)
	}
	fixtureCoordinates, err := fpfrefresh.ReadTokenGateCoordinates(paths.FixturePath)
	if err != nil {
		t.Fatalf("read token-gate fixture coordinates: %v", err)
	}
	payload, err := os.ReadFile(paths.FixturePath)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	fixture := embeddedTokenGateFixture{}
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatalf("decode token-gate behavior fixture: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("decode token-gate behavior fixture trailing data: %v", err)
	}
	if fixture.SchemaVersion != fpfrefresh.TokenGateFixtureSchemaVersion ||
		fixture.FixtureRevision != fixtureCoordinates.FixtureRevision ||
		len(fixture.Cases) == 0 {
		t.Fatalf(
			"token-gate behavior fixture identity = schema %q revision %q cases %d",
			fixture.SchemaVersion,
			fixture.FixtureRevision,
			len(fixture.Cases),
		)
	}

	lockPayload, err := os.ReadFile(paths.LockPath)
	if err != nil {
		t.Fatalf("read generated FPF integration lock: %v", err)
	}
	lock, err := fpfrefresh.ParseIntegrationLock(lockPayload)
	if err != nil {
		t.Fatalf("parse generated FPF integration lock: %v", err)
	}
	if lock.TokenGate == nil || *lock.TokenGate != fixtureCoordinates {
		t.Fatalf(
			"generated lock token fixture = %#v, exact fixture coordinates = %#v",
			lock.TokenGate,
			fixtureCoordinates,
		)
	}
	databaseImage := embeddedFPFDB
	if paths.DatabasePath != "" {
		info, statErr := os.Stat(paths.DatabasePath)
		if statErr != nil {
			t.Fatalf("inspect candidate FPF database: %v", statErr)
		}
		if !info.Mode().IsRegular() || info.Size() <= 0 {
			t.Fatalf(
				"candidate FPF database %q is not a non-empty regular file",
				paths.DatabasePath,
			)
		}
		databaseImage, err = os.ReadFile(paths.DatabasePath)
		if err != nil {
			t.Fatalf("read candidate FPF database: %v", err)
		}
	}
	return embeddedTokenGateBasis{
		DatabaseImage: databaseImage,
		Fixture:       fixture,
		Lock:          lock,
	}
}

func resolveEmbeddedTokenGateArtifactPaths(
	getenv func(string) string,
) (embeddedTokenGateArtifactPaths, error) {
	if getenv == nil {
		return embeddedTokenGateArtifactPaths{}, fmt.Errorf(
			"token-gate environment reader is required",
		)
	}
	paths := embeddedTokenGateArtifactPaths{
		DatabasePath: getenv(fpfrefresh.CandidateTokenGateDatabasePathEnvironment),
		FixturePath:  getenv(fpfrefresh.CandidateTokenGateFixturePathEnvironment),
		LockPath:     getenv(fpfrefresh.CandidateTokenGateLockPathEnvironment),
	}
	supplied := 0
	for _, path := range []string{
		paths.DatabasePath,
		paths.FixturePath,
		paths.LockPath,
	} {
		if path != "" {
			supplied++
		}
	}
	if supplied == 0 {
		return embeddedTokenGateArtifactPaths{
			FixturePath: filepath.Join("testdata", "fpf_query_token_gate_corpus.json"),
			LockPath: filepath.Join(
				"..",
				"..",
				fpfrefresh.DefaultIntegrationLockRelativePath,
			),
		}, nil
	}
	if supplied != 3 {
		return embeddedTokenGateArtifactPaths{}, fmt.Errorf(
			"candidate database, integration lock, and fixture overrides must be supplied together",
		)
	}
	for label, path := range map[string]string{
		"database":         paths.DatabasePath,
		"integration lock": paths.LockPath,
		"fixture":          paths.FixturePath,
	} {
		if path != strings.TrimSpace(path) ||
			!filepath.IsAbs(path) ||
			filepath.Clean(path) != path {
			return embeddedTokenGateArtifactPaths{}, fmt.Errorf(
				"candidate %s override must be a trimmed canonical absolute path",
				label,
			)
		}
	}
	return paths, nil
}

func assertEmbeddedTokenGateDatabaseDigest(
	t *testing.T,
	databaseImage []byte,
	expected string,
) {
	t.Helper()
	digest := sha256.Sum256(databaseImage)
	got := "sha256:" + hex.EncodeToString(digest[:])
	if got != expected {
		t.Fatalf("selected FPF database digest = %s, generated lock = %s", got, expected)
	}
}

func assertEmbeddedTokenGateSnapshot(
	t *testing.T,
	snapshot fpf.QuerySourceSnapshot,
	coordinates fpfrefresh.IntegrationCoordinates,
) {
	t.Helper()
	if fpf.SpecIndexSchemaVersion != coordinates.IndexSchemaVersion ||
		snapshot.IndexSchemaVersion() != coordinates.IndexSchemaVersion ||
		snapshot.Revision() != coordinates.SourceRevision ||
		snapshot.ReadmeDigest() != coordinates.ReadmeDocumentDigest ||
		snapshot.SpecDigest() != coordinates.SpecDocumentDigest {
		t.Fatalf(
			"embedded token-gate source snapshot = %#v, generated lock = %#v",
			snapshot,
			coordinates,
		)
	}
}

func assertEmbeddedTokenGateCanonicalResult(
	t *testing.T,
	testCase embeddedTokenGateCase,
	result fpf.CandidateSet,
) {
	t.Helper()
	if result.Kind != testCase.ExpectedKind {
		t.Fatalf("%s result kind = %q, want %q", testCase.CaseID, result.Kind, testCase.ExpectedKind)
	}
	if result.Truncation.Budget != embeddedTokenGateDefaultBudget {
		t.Fatalf(
			"%s default response budget = %#v, want frozen %#v",
			testCase.CaseID,
			result.Truncation.Budget,
			embeddedTokenGateDefaultBudget,
		)
	}
	ids := embeddedTokenGateCanonicalCandidateIDs(result)
	if len(ids) != testCase.ExpectedCandidateCount || !reflect.DeepEqual(ids, testCase.ExpectedCandidateIDs) {
		t.Fatalf(
			"%s candidate identity = %#v (%d), want %#v (%d)",
			testCase.CaseID,
			ids,
			len(ids),
			testCase.ExpectedCandidateIDs,
			testCase.ExpectedCandidateCount,
		)
	}
	if testCase.ExpectedCandidateSourceIDs != nil {
		sourceIDs := embeddedTokenGateCanonicalCandidateSourceIDs(result)
		if !reflect.DeepEqual(sourceIDs, testCase.ExpectedCandidateSourceIDs) {
			t.Fatalf(
				"%s candidate source IDs = %#v, want %#v",
				testCase.CaseID,
				sourceIDs,
				testCase.ExpectedCandidateSourceIDs,
			)
		}
	}
	if result.Truncation.Applied != testCase.ExpectedTruncationApplied ||
		result.Truncation.IncludedCandidates != testCase.ExpectedCandidateCount ||
		result.Truncation.OmittedAtLeast != testCase.ExpectedOmittedAtLeast ||
		!reflect.DeepEqual(result.Truncation.Basis, testCase.ExpectedTruncationBasis) {
		t.Fatalf("%s truncation = %#v, want frozen posture from %#v", testCase.CaseID, result.Truncation, testCase)
	}
}

func embeddedTokenGateCanonicalCandidateIDs(result fpf.CandidateSet) []string {
	ids := make([]string, 0, result.Truncation.IncludedCandidates)
	for _, group := range result.Groups {
		for _, candidate := range group.Candidates {
			ids = append(ids, candidate.Source.UnitID)
		}
	}
	return ids
}

func embeddedTokenGateCanonicalCandidateSourceIDs(result fpf.CandidateSet) []string {
	ids := make([]string, 0, result.Truncation.IncludedCandidates)
	for _, group := range result.Groups {
		for _, candidate := range group.Candidates {
			ids = append(ids, candidate.Source.SourceID)
		}
	}
	return ids
}

func assertEmbeddedTokenGateWorkingSemantics(
	t *testing.T,
	canonical fpf.CandidateSet,
	encoded []byte,
) {
	t.Helper()
	working := embeddedWorkingCandidateSet{}
	if err := json.Unmarshal(encoded, &working); err != nil {
		t.Fatalf("decode working token-gate carrier: %v\n%s", err, encoded)
	}
	if working.View != fpf.QueryPublicationViewWorking || working.TraceRef == "" || working.Kind != canonical.Kind {
		t.Fatalf("working carrier identity = %#v", working)
	}
	wantBudget := fpf.PublishedResponseBudget{
		MaxCandidatesPerRole:     canonical.Truncation.Budget.MaxCandidatesPerRole,
		MaxTotalCandidates:       canonical.Truncation.Budget.MaxTotalCandidates,
		MaxExcerptCharacters:     canonical.Truncation.Budget.MaxExcerptCharacters,
		MaxRelationsPerCandidate: canonical.Truncation.Budget.MaxRelationsPerCandidate,
	}
	if working.Truncation.Budget != wantBudget ||
		working.Truncation.Applied != canonical.Truncation.Applied ||
		working.Truncation.IncludedCandidates != canonical.Truncation.IncludedCandidates ||
		working.Truncation.OmittedAtLeast != canonical.Truncation.OmittedAtLeast {
		t.Fatalf("working truncation changed canonical posture: got %#v, canonical %#v", working.Truncation, canonical.Truncation)
	}
	if len(working.Groups) != len(canonical.Groups) {
		t.Fatalf("working group count = %d, want %d", len(working.Groups), len(canonical.Groups))
	}
	for groupIndex, canonicalGroup := range canonical.Groups {
		workingGroup := working.Groups[groupIndex]
		if workingGroup.Role != canonicalGroup.Role || len(workingGroup.Candidates) != len(canonicalGroup.Candidates) {
			t.Fatalf("working group %d = %#v, canonical %#v", groupIndex, workingGroup, canonicalGroup)
		}
		for candidateIndex, canonicalCandidate := range canonicalGroup.Candidates {
			workingCandidate := workingGroup.Candidates[candidateIndex].Source
			assertEmbeddedTokenGateCandidateSemantics(t, canonicalCandidate.Source, workingCandidate)
		}
	}

	payload := make(map[string]any)
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	assertQueryIntegrationWorkingDenylist(t, payload)
}

func assertEmbeddedTokenGateCandidateSemantics(
	t *testing.T,
	canonical fpf.CandidateSourceUnit,
	working fpf.PublishedCandidateSourceUnit,
) {
	t.Helper()
	canonicalScalars := []any{
		canonical.UnitID,
		canonical.SourceID,
		canonical.SourceRole,
		canonical.Title,
		canonical.Excerpt,
		canonical.ExcerptTruncated,
		canonical.UseCuesTruncated,
		canonical.PatternID,
		canonical.ParentPatternID,
		canonical.PublicationStatus,
	}
	workingScalars := []any{
		working.UnitID,
		working.SourceID,
		working.SourceRole,
		working.Title,
		working.Excerpt,
		working.ExcerptTruncated,
		working.UseCuesTruncated,
		working.PatternID,
		working.ParentPatternID,
		working.PublicationStatus,
	}
	if !reflect.DeepEqual(canonicalScalars, workingScalars) {
		t.Fatalf("working candidate scalar semantics changed: got %#v, canonical %#v", workingScalars, canonicalScalars)
	}
	if !reflect.DeepEqual(canonical.DirectRefs, working.DirectRefs) ||
		working.DirectRefsTruncated ||
		working.DirectRefsOmittedAtLeast != 0 {
		t.Fatalf("working candidate direct refs changed: got %#v, canonical %#v", working, canonical)
	}
	assertEmbeddedTokenGateUseCues(t, canonical.UseCues, working.UseCues)
	assertEmbeddedTokenGateRelations(t, canonical.RelationProjection, working.RelationProjection)
}

func assertEmbeddedTokenGateUseCues(
	t *testing.T,
	canonical *fpf.SourceUseCues,
	working *fpf.PublishedSourceUseCues,
) {
	t.Helper()
	if canonical == nil && working == nil {
		return
	}
	if canonical == nil || working == nil ||
		canonical.ConditionText != working.ConditionText ||
		canonical.FirstResultText != working.FirstResultText ||
		canonical.StopReturnText != working.StopReturnText {
		t.Fatalf("working use cues = %#v, canonical %#v", working, canonical)
	}
}

func assertEmbeddedTokenGateRelations(
	t *testing.T,
	canonical *fpf.CandidateRelationProjection,
	working *fpf.PublishedCandidateRelationProjection,
) {
	t.Helper()
	if canonical == nil && working == nil {
		return
	}
	if canonical == nil || working == nil ||
		canonical.Truncated != working.Truncated ||
		canonical.OmittedAtLeast != working.OmittedAtLeast ||
		len(canonical.Relations) != len(working.Relations) {
		t.Fatalf("working relation projection = %#v, canonical %#v", working, canonical)
	}
	for relationIndex, canonicalRelation := range canonical.Relations {
		workingRelation := working.Relations[relationIndex]
		if canonicalRelation.Kind != workingRelation.Kind ||
			canonicalRelation.TargetPatternID != workingRelation.TargetPatternID {
			t.Fatalf("working relation %d = %#v, canonical %#v", relationIndex, workingRelation, canonicalRelation)
		}
	}
}

func runEmbeddedTokenGateCounter(
	t *testing.T,
	cases []embeddedTokenGateInputCase,
) embeddedTokenGateOutput {
	t.Helper()
	request := embeddedTokenGateInput{Schema: embeddedTokenGateSchema, Cases: cases}
	encodedRequest, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	python := strings.TrimSpace(os.Getenv("HAFT_QUERY_TOKEN_GATE_PYTHON"))
	if python == "" {
		python = "python3"
	}
	helperPath := filepath.Join("..", "..", "scripts", "fpf_query_token_count.py")
	command := exec.Command(python, helperPath)
	command.Stdin = bytes.NewReader(encodedRequest)
	stderr := bytes.Buffer{}
	command.Stderr = &stderr
	encodedOutput, err := command.Output()
	if err != nil {
		t.Fatalf("pinned o200k_base counter failed: %v\n%s", err, stderr.String())
	}

	output := embeddedTokenGateOutput{}
	if err := json.Unmarshal(encodedOutput, &output); err != nil {
		t.Fatalf("decode pinned token counter output: %v\n%s", err, encodedOutput)
	}
	if output.Schema != embeddedTokenGateResultSchema ||
		output.Encoding != embeddedTokenGateEncoding ||
		output.TokenizerDistribution != "tiktoken" ||
		output.TokenizerVersion != embeddedTokenGateTokenizerVersion ||
		output.EncodingAssetSHA256 != embeddedTokenGateEncodingAssetHash ||
		output.CalibrationTokens != embeddedTokenGateCalibrationTokens ||
		output.PythonImplementation != "CPython" ||
		!embeddedTokenGateSupportedPython(output.PythonVersion) {
		t.Fatalf("unexpected tokenizer/runtime identity: %#v", output)
	}
	if len(output.Counts) != len(cases) {
		t.Fatalf("token counter returned %d cases, want %d", len(output.Counts), len(cases))
	}
	for index, count := range output.Counts {
		if count.CaseID != cases[index].CaseID {
			t.Fatalf("token counter case %d = %q, want %q", index, count.CaseID, cases[index].CaseID)
		}
		if count.CanonicalTokens <= 0 || count.WorkingTokens <= 0 {
			t.Fatalf("token counter returned non-positive counts: %#v", count)
		}
	}
	t.Logf(
		"tokenizer=%s@%s asset_sha256=%s python=%s@%s",
		output.TokenizerDistribution,
		output.TokenizerVersion,
		output.EncodingAssetSHA256,
		output.PythonImplementation,
		output.PythonVersion,
	)
	return output
}

func embeddedTokenGateSupportedPython(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	return majorErr == nil && minorErr == nil && major == 3 && minor >= 10 && minor <= 13
}

func assertEmbeddedTokenGateReduction(t *testing.T, measurements embeddedTokenGateOutput) {
	t.Helper()
	canonicalCounts := make([]int, 0, len(measurements.Counts))
	workingCounts := make([]int, 0, len(measurements.Counts))
	perCaseReductions := make([]float64, 0, len(measurements.Counts))
	for _, measurement := range measurements.Counts {
		reduction := 1 - float64(measurement.WorkingTokens)/float64(measurement.CanonicalTokens)
		canonicalCounts = append(canonicalCounts, measurement.CanonicalTokens)
		workingCounts = append(workingCounts, measurement.WorkingTokens)
		perCaseReductions = append(perCaseReductions, reduction)
		t.Logf(
			"case=%s canonical=%d working=%d reduction=%.1f%%",
			measurement.CaseID,
			measurement.CanonicalTokens,
			measurement.WorkingTokens,
			reduction*100,
		)
	}

	medianPerCaseReduction := embeddedTokenGateMedianFloat64(perCaseReductions)
	medianCanonical := embeddedTokenGateMedianInt(canonicalCounts)
	medianWorking := embeddedTokenGateMedianInt(workingCounts)
	ratioOfMediansReduction := 1 - medianWorking/medianCanonical
	t.Logf(
		"real_embedded_median_per_case_reduction=%.1f%% median_canonical=%.1f median_working=%.1f ratio_of_medians_reduction=%.1f%%",
		medianPerCaseReduction*100,
		medianCanonical,
		medianWorking,
		ratioOfMediansReduction*100,
	)
	if medianPerCaseReduction < embeddedTokenGateMinimumReduction {
		t.Fatalf(
			"real embedded median per-case o200k_base reduction = %.2f%%, require at least %.2f%%",
			medianPerCaseReduction*100,
			embeddedTokenGateMinimumReduction*100,
		)
	}
	if ratioOfMediansReduction < embeddedTokenGateMinimumReduction {
		t.Fatalf(
			"real embedded o200k_base ratio-of-medians reduction = %.2f%%, require at least %.2f%%",
			ratioOfMediansReduction*100,
			embeddedTokenGateMinimumReduction*100,
		)
	}
}

func embeddedTokenGateMedianInt(values []int) float64 {
	ordered := append([]int(nil), values...)
	sort.Ints(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return float64(ordered[middle])
	}
	return float64(ordered[middle-1]+ordered[middle]) / 2
}

func embeddedTokenGateMedianFloat64(values []float64) float64 {
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}

func TestEmbeddedTokenGateSupportedPython(t *testing.T) {
	for _, version := range []string{"3.10.0", "3.12.7", "3.13.1"} {
		if !embeddedTokenGateSupportedPython(version) {
			t.Fatalf("supported Python rejected: %s", version)
		}
	}
	for _, version := range []string{"", "3", "3.9.20", "3.14.0", "PyPy-3.12"} {
		if embeddedTokenGateSupportedPython(version) {
			t.Fatalf("unsupported Python accepted: %s", version)
		}
	}
}

func TestEmbeddedTokenGateExpectedCountsMatchIDs(t *testing.T) {
	basis := loadEmbeddedTokenGateBasis(t)
	seen := make(map[string]struct{}, len(basis.Fixture.Cases))
	for _, testCase := range basis.Fixture.Cases {
		if _, duplicate := seen[testCase.CaseID]; duplicate || strings.TrimSpace(testCase.CaseID) == "" {
			t.Fatalf("duplicate or empty token-gate case ID %q", testCase.CaseID)
		}
		seen[testCase.CaseID] = struct{}{}
		if testCase.ExpectedCandidateCount != len(testCase.ExpectedCandidateIDs) {
			t.Fatalf(
				"%s expected candidate count = %d, IDs = %d",
				testCase.CaseID,
				testCase.ExpectedCandidateCount,
				len(testCase.ExpectedCandidateIDs),
			)
		}
		if testCase.ExpectedCandidateSourceIDs != nil &&
			testCase.ExpectedCandidateCount != len(testCase.ExpectedCandidateSourceIDs) {
			t.Fatalf(
				"%s expected candidate count = %d, source IDs = %d",
				testCase.CaseID,
				testCase.ExpectedCandidateCount,
				len(testCase.ExpectedCandidateSourceIDs),
			)
		}
		if testCase.ExpectedKind != fpf.QueryResultKindCandidateSet {
			t.Fatalf("%s expected kind = %q", testCase.CaseID, testCase.ExpectedKind)
		}
	}
}

func TestEmbeddedTokenGateGeneratedDatabaseDigestFormat(t *testing.T) {
	basis := loadEmbeddedTokenGateBasis(t)
	digest := strings.TrimPrefix(basis.Lock.Coordinates.DatabaseDigest, "sha256:")
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256.Size {
		t.Fatalf("generated token-gate database digest is invalid: %q", digest)
	}
}

func TestEmbeddedTokenGateArtifactOverridesAreAllOrNoneAndAbsolute(t *testing.T) {
	root := t.TempDir()
	complete := map[string]string{
		fpfrefresh.CandidateTokenGateDatabasePathEnvironment: filepath.Join(root, "candidate.db"),
		fpfrefresh.CandidateTokenGateLockPathEnvironment: filepath.Join(
			root,
			"candidate.lock.json",
		),
		fpfrefresh.CandidateTokenGateFixturePathEnvironment: filepath.Join(root, "corpus.json"),
	}
	getenv := func(values map[string]string) func(string) string {
		return func(key string) string {
			return values[key]
		}
	}

	defaults, err := resolveEmbeddedTokenGateArtifactPaths(getenv(nil))
	if err != nil {
		t.Fatalf("default artifact paths error = %v", err)
	}
	if defaults.DatabasePath != "" ||
		defaults.FixturePath != filepath.Join("testdata", "fpf_query_token_gate_corpus.json") ||
		defaults.LockPath != filepath.Join(
			"..",
			"..",
			fpfrefresh.DefaultIntegrationLockRelativePath,
		) {
		t.Fatalf("default artifact paths = %#v", defaults)
	}

	overridden, err := resolveEmbeddedTokenGateArtifactPaths(getenv(complete))
	if err != nil {
		t.Fatalf("complete artifact overrides error = %v", err)
	}
	if overridden.DatabasePath != complete[fpfrefresh.CandidateTokenGateDatabasePathEnvironment] ||
		overridden.LockPath != complete[fpfrefresh.CandidateTokenGateLockPathEnvironment] ||
		overridden.FixturePath != complete[fpfrefresh.CandidateTokenGateFixturePathEnvironment] {
		t.Fatalf("complete artifact overrides = %#v", overridden)
	}

	incomplete := map[string]string{
		fpfrefresh.CandidateTokenGateDatabasePathEnvironment: complete[fpfrefresh.CandidateTokenGateDatabasePathEnvironment],
	}
	if _, err := resolveEmbeddedTokenGateArtifactPaths(getenv(incomplete)); err == nil {
		t.Fatal("incomplete artifact overrides error = nil")
	}

	relative := make(map[string]string, len(complete))
	for key, value := range complete {
		relative[key] = value
	}
	relative[fpfrefresh.CandidateTokenGateFixturePathEnvironment] = "relative/corpus.json"
	if _, err := resolveEmbeddedTokenGateArtifactPaths(getenv(relative)); err == nil {
		t.Fatal("relative artifact override error = nil")
	}
}
