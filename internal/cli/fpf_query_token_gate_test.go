//go:build query_token_gate

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
)

const (
	embeddedTokenGateSchema            = "haft.fpf-query-token-gate/v1"
	embeddedTokenGateResultSchema      = "haft.fpf-query-token-gate-result/v1"
	embeddedTokenGateEncoding          = "o200k_base"
	embeddedTokenGateTokenizerVersion  = "0.9.0"
	embeddedTokenGateEncodingAssetHash = "446a9538cb6c348e3516120d7c08b09f57c36495e2acfffe59a5bf8b0cfb1a2d"
	embeddedTokenGateCalibrationTokens = 21
	embeddedTokenGateMinimumReduction  = 0.30
	embeddedTokenGateDatabaseDigest    = "6794aa7fb613a2a3013057a7e9aa5edaebc738fe7711fc664beda2e391f4b954"
	embeddedTokenGateCorpusDigest      = "4e6eedff7c26fdb3940d55aa5cc52ca739dc4a3f441873d544e571b4145e3171"
	embeddedTokenGateIndexSchema       = "11"
	embeddedTokenGateSourceRevision    = "0990ff1d1ccee4587b8f7e16e7a725a8edbe66b4"
	embeddedTokenGateReadmeDigest      = "sha256:6c8d87a641f36d34a9d84aa0ab8e7565dcca2a691482a0cee31bd28a743eb3fd"
	embeddedTokenGateSpecDigest        = "sha256:1093a25640c61a2674f56443bffb8e27f33ac2cdf95f09af2c0cf67c68913eac"
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
	assertEmbeddedTokenGateDatabaseDigest(t)
	corpus := embeddedTokenGateCorpus()
	assertEmbeddedTokenGateCorpusDigest(t, corpus)

	db, cleanup, err := openFPFDBImage(context.Background(), embeddedFPFDB)
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
	assertEmbeddedTokenGateSnapshot(t, snapshot)

	publicationRequest, err := fpf.NewQueryPublicationRequest("working", "")
	if err != nil {
		t.Fatal(err)
	}
	index := fpf.NewSQLiteQueryIndex(db)
	inputCases := make([]embeddedTokenGateInputCase, 0, len(corpus))
	for _, testCase := range corpus {
		evaluation, err := fpf.EvaluateQuery(index, testCase.Request)
		if err != nil {
			t.Fatalf("%s evaluate real embedded concern: %v", testCase.CaseID, err)
		}
		canonical := evaluation.Result()
		candidateSet, ok := canonical.(fpf.CandidateSet)
		if !ok {
			t.Fatalf("%s canonical result = %T, want CandidateSet", testCase.CaseID, canonical)
		}
		assertEmbeddedTokenGateCanonicalResult(t, testCase, candidateSet)

		execution, err := fpf.NewCanonicalQueryExecution(
			testCase.Request,
			evaluation,
			snapshot,
		)
		if err != nil {
			t.Fatalf("%s canonical execution: %v", testCase.CaseID, err)
		}
		published, err := fpf.ProjectQueryResult(execution, publicationRequest)
		if err != nil {
			t.Fatalf("%s working projection: %v", testCase.CaseID, err)
		}
		workingJSON, err := fpf.EncodePublishedQuery(published, fpf.PublishedQueryJSONCompact)
		if err != nil {
			t.Fatalf("%s compact working JSON: %v", testCase.CaseID, err)
		}
		assertEmbeddedTokenGateWorkingSemantics(t, candidateSet, workingJSON)

		canonicalJSON, err := json.Marshal(canonical)
		if err != nil {
			t.Fatalf("%s compact canonical JSON: %v", testCase.CaseID, err)
		}
		inputCases = append(inputCases, embeddedTokenGateInputCase{
			CaseID:        testCase.CaseID,
			CanonicalJSON: string(canonicalJSON),
			WorkingJSON:   string(workingJSON),
		})
	}

	measurements := runEmbeddedTokenGateCounter(t, inputCases)
	assertEmbeddedTokenGateReduction(t, measurements)
}

func embeddedTokenGateCorpus() []embeddedTokenGateCase {
	return []embeddedTokenGateCase{
		{
			CaseID:                 "relation-occurrence-assertion",
			Request:                fpf.ConcernQuery{Text: "relation occurrence assertion distinction"},
			ExpectedKind:           fpf.QueryResultKindCandidateSet,
			ExpectedCandidateCount: 7,
			ExpectedCandidateIDs: []string{
				"readme:practical_use_card:wording",
				"readme:practical_use_card:system-in-context",
				"spec:toc_row:a-6-rel",
				"spec:toc_row:c-22-pfr",
				"spec:toc_row:c-3-1",
				"spec:toc_row:c-3-3",
				"spec:toc_row:a-15-1",
			},
			ExpectedTruncationApplied: true,
			ExpectedOmittedAtLeast:    5,
			ExpectedTruncationBasis:   []string{"response_budget"},
		},
		{
			CaseID:                 "pattern-use-working-situation",
			Request:                fpf.ConcernQuery{Text: "pattern use working situation"},
			ExpectedKind:           fpf.QueryResultKindCandidateSet,
			ExpectedCandidateCount: 4,
			ExpectedCandidateIDs: []string{
				"readme:practical_use_card:working-documents",
				"readme:practical_use_card:architecture",
				"spec:toc_row:e-11-pua",
				"spec:toc_row:e-8-ecspf",
			},
		},
		{
			CaseID:                 "evidence-decay-decision-verification",
			Request:                fpf.ConcernQuery{Text: "evidence decay decision verification"},
			ExpectedKind:           fpf.QueryResultKindCandidateSet,
			ExpectedCandidateCount: 3,
			ExpectedCandidateIDs: []string{
				"readme:practical_use_card:costly-action",
				"readme:practical_use_card:time",
				"spec:toc_row:b-3-4",
			},
		},
		{
			CaseID:                 "work-plan-performed-work-evidence",
			Request:                fpf.ConcernQuery{Text: "work plan performed work evidence"},
			ExpectedKind:           fpf.QueryResultKindCandidateSet,
			ExpectedCandidateCount: 8,
			ExpectedCandidateIDs: []string{
				"readme:practical_use_card:system-in-context",
				"readme:practical_use_card:costly-action",
				"readme:practical_use_card:working-documents",
				"spec:toc_row:a-15",
				"spec:toc_row:a-15-1",
				"spec:toc_row:a-15-5",
				"spec:toc_row:a-3-4-p",
				"spec:toc_row:d-1",
			},
			ExpectedTruncationApplied: true,
			ExpectedOmittedAtLeast:    2,
			ExpectedTruncationBasis:   []string{"response_budget"},
		},
		{
			CaseID:                 "entity-reference-claim-graph",
			Request:                fpf.ConcernQuery{Text: "entity of concern reference scheme claim graph"},
			ExpectedKind:           fpf.QueryResultKindCandidateSet,
			ExpectedCandidateCount: 10,
			ExpectedCandidateIDs: []string{
				"readme:practical_use_card:system-in-context",
				"readme:practical_use_card:wording",
				"readme:practical_use_card:dpf-authoring",
				"readme:practical_use_card:naming",
				"readme:practical_use_card:description-use",
				"spec:toc_row:a-6-3",
				"spec:toc_row:a-6-3-cr",
				"spec:toc_row:a-6-3-rt",
				"spec:toc_row:a-6-4",
				"spec:toc_row:e-10-d2",
			},
			ExpectedTruncationApplied: true,
			ExpectedOmittedAtLeast:    8,
			ExpectedTruncationBasis:   []string{"response_budget"},
		},
		{
			CaseID:                 "authority-speech-act-permission",
			Request:                fpf.ConcernQuery{Text: "authority speech act permission"},
			ExpectedKind:           fpf.QueryResultKindCandidateSet,
			ExpectedCandidateCount: 5,
			ExpectedCandidateIDs: []string{
				"spec:toc_row:a-10",
				"spec:toc_row:a-2-9",
				"spec:toc_row:e-16",
				"spec:toc_row:a-6-c",
				"spec:toc_row:e-10",
			},
			ExpectedTruncationApplied: true,
			ExpectedOmittedAtLeast:    1,
			ExpectedTruncationBasis:   []string{"response_budget"},
		},
		{
			CaseID: "russian-plan-work-evidence",
			Request: fpf.ConcernQuery{
				Text:            "Как различить план, выполненную работу и свидетельство результата?",
				EntityOfConcern: "план и выполненная работа",
				KnownContext:    []string{"work plan performed work evidence"},
				IntendedUse:     "выбрать прямой FPF pattern для проверки",
			},
			ExpectedKind:           fpf.QueryResultKindCandidateSet,
			ExpectedCandidateCount: 8,
			ExpectedCandidateIDs: []string{
				"readme:practical_use_card:system-in-context",
				"readme:practical_use_card:costly-action",
				"readme:practical_use_card:working-documents",
				"spec:toc_row:a-15",
				"spec:toc_row:a-15-1",
				"spec:toc_row:a-15-5",
				"spec:toc_row:a-3-4-p",
				"spec:toc_row:d-1",
			},
			ExpectedCandidateSourceIDs: []string{
				"SYSTEM-IN-CONTEXT",
				"COSTLY-ACTION",
				"WORKING-DOCUMENTS",
				"",
				"",
				"",
				"",
				"",
			},
			ExpectedTruncationApplied: true,
			ExpectedOmittedAtLeast:    40,
			ExpectedTruncationBasis: []string{
				"role_local_fts_producer_limit",
				"role_local_fts:toc_row",
				"response_budget",
			},
		},
	}
}

func assertEmbeddedTokenGateDatabaseDigest(t *testing.T) {
	t.Helper()
	digest := sha256.Sum256(embeddedFPFDB)
	got := hex.EncodeToString(digest[:])
	if got != embeddedTokenGateDatabaseDigest {
		t.Fatalf("embedded FPF database digest = %s, want %s", got, embeddedTokenGateDatabaseDigest)
	}
}

func assertEmbeddedTokenGateCorpusDigest(t *testing.T, corpus []embeddedTokenGateCase) {
	t.Helper()
	encoded, err := json.Marshal(corpus)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	got := hex.EncodeToString(digest[:])
	if got != embeddedTokenGateCorpusDigest {
		t.Fatalf(
			"embedded concern corpus digest = %s, want %s; requests or expected retrieval facts require an explicit quantitative rebaseline",
			got,
			embeddedTokenGateCorpusDigest,
		)
	}
}

func assertEmbeddedTokenGateSnapshot(t *testing.T, snapshot fpf.QuerySourceSnapshot) {
	t.Helper()
	if fpf.SpecIndexSchemaVersion != embeddedTokenGateIndexSchema ||
		snapshot.IndexSchemaVersion() != embeddedTokenGateIndexSchema ||
		snapshot.Revision() != embeddedTokenGateSourceRevision ||
		snapshot.ReadmeDigest() != embeddedTokenGateReadmeDigest ||
		snapshot.SpecDigest() != embeddedTokenGateSpecDigest {
		t.Fatalf("embedded token-gate source snapshot = %#v", snapshot)
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
	for _, testCase := range embeddedTokenGateCorpus() {
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

func TestEmbeddedTokenGatePinnedDatabaseDigestFormat(t *testing.T) {
	decoded, err := hex.DecodeString(embeddedTokenGateDatabaseDigest)
	if err != nil || len(decoded) != sha256.Size {
		t.Fatalf("embedded token-gate database digest is invalid: %q", embeddedTokenGateDatabaseDigest)
	}
}

func Example_embeddedTokenGateIdentity() {
	fmt.Printf("db=sha256:%s source=%s\n", embeddedTokenGateDatabaseDigest, embeddedTokenGateSourceRevision)
	// Output:
	// db=sha256:6794aa7fb613a2a3013057a7e9aa5edaebc738fe7711fc664beda2e391f4b954 source=0990ff1d1ccee4587b8f7e16e7a725a8edbe66b4
}
