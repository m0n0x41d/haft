package fpf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestWorkingProjectionRecursivelyExcludesInternalQueryFields(t *testing.T) {
	result := publicProjectionCandidateSet(4)
	execution := mustCanonicalQueryExecution(
		t,
		ConcernQuery{Text: strings.Repeat("unbounded concern ", 1024)},
		result,
		publicProjectionSnapshot(t, "revision-a"),
	)
	request := mustQueryPublicationRequest(t, "", "")
	published, err := ProjectQueryResult(execution, request)
	if err != nil {
		t.Fatal(err)
	}
	encoded := mustEncodePublishedQuery(t, published)
	for _, key := range []string{
		"provenance",
		"source_path",
		"start_line",
		"end_line",
		"content_hash",
		"source_revision",
		"match_grounds",
		"tier",
		"probe_field",
		"source_field",
		"matched_value",
		"phrase_kind",
		"evidence",
		"projection_relation",
		"authored_phrases",
		"keywords",
		"target_class",
		"origin",
		"canonical_unit_id",
		"subject_pattern_id",
		"basis",
		"producer_ids",
	} {
		if recursiveJSONKeyExists(t, encoded, key) {
			t.Fatalf("working response contains forbidden key %q:\n%s", key, encoded)
		}
	}

	payload := decodePublishedObject(t, encoded)
	if payload["view"] != string(QueryPublicationViewWorking) {
		t.Fatalf("view = %#v", payload["view"])
	}
	if payload["kind"] != string(QueryResultKindCandidateSet) {
		t.Fatalf("kind = %#v", payload["kind"])
	}
	if _, exists := payload["concern"]; exists {
		t.Fatal("working candidate set echoes the unbounded concern")
	}
	groups := payload["groups"].([]any)
	candidate := groups[0].(map[string]any)["candidates"].([]any)[0].(map[string]any)
	source := candidate["source"].(map[string]any)
	for key, want := range map[string]any{
		"unit_id":            "toc:a-1",
		"source_role":        string(SourceUnitRoleTOCRow),
		"title":              "Pattern A.1",
		"pattern_id":         "A.1",
		"publication_status": "Stable",
	} {
		if source[key] != want {
			t.Fatalf("source.%s = %#v, want %#v", key, source[key], want)
		}
	}
	projection := source["relation_projection"].(map[string]any)
	relations := projection["relations"].([]any)
	relation := relations[0].(map[string]any)
	if !reflect.DeepEqual(relation, map[string]any{
		"kind":              string(SourceRelationKindBuildsOn),
		"target_pattern_id": "A.0",
	}) {
		t.Fatalf("working relation = %#v", relation)
	}
}

func TestWorkingExactProjectionKeepsInspectBodyAndSeparatesLookupIdentity(t *testing.T) {
	body := strings.Join([]string{
		"Problem frame",
		"Problem",
		"Forces",
		"Solution",
		"Ordinary boundary",
		"Worked slice",
		"Checklist",
	}, "\n")
	result := publicProjectionExactHit(body)
	snapshot := publicProjectionSnapshot(t, "revision-a")

	tests := []struct {
		name     string
		request  QueryRequest
		wantBody bool
	}{
		{name: "inspect", request: InspectQuery{Identifier: "A.1"}, wantBody: true},
		{name: "lookup", request: LookupQuery{Identifier: "A.1"}, wantBody: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execution := mustCanonicalQueryExecution(t, test.request, result, snapshot)
			published, err := ProjectQueryResult(
				execution,
				mustQueryPublicationRequest(t, "working", ""),
			)
			if err != nil {
				t.Fatal(err)
			}
			payload := decodePublishedObject(t, mustEncodePublishedQuery(t, published))
			unit := payload["unit"].(map[string]any)
			gotBody, bodyExists := unit["body"]
			if test.wantBody && (!bodyExists || gotBody != body) {
				t.Fatalf("inspect body = %#v, exists=%v", gotBody, bodyExists)
			}
			if !test.wantBody && bodyExists {
				t.Fatalf("working lookup leaked full body: %#v", gotBody)
			}
		})
	}
}

func TestTraceProjectionDeduplicatesAndReconstructsCanonicalProvenance(t *testing.T) {
	canonical := publicProjectionCandidateSet(3)
	execution := mustCanonicalQueryExecution(
		t,
		ConcernQuery{Text: "bounded concern"},
		canonical,
		publicProjectionSnapshot(t, "revision-a"),
	)
	published, err := ProjectQueryResult(
		execution,
		mustQueryPublicationRequest(t, "trace", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded := mustEncodePublishedQuery(t, published)
	if got := recursiveJSONKeyCount(t, encoded, "source_revision"); got != 1 {
		t.Fatalf("source_revision key count = %d, want one response-wide value", got)
	}
	if recursiveJSONKeyExists(t, encoded, "match_grounds") {
		t.Fatal("trace response includes raw match grounds")
	}

	traceResult, ok := published.(traceCandidateSet)
	if !ok {
		t.Fatalf("published = %T", published)
	}
	trace := traceResult.Trace
	if len(trace.Provenance) != 3 {
		t.Fatalf("deduplicated provenance entries = %d, want 3: %#v", len(trace.Provenance), trace.Provenance)
	}
	byRef := make(map[string]TraceProvenanceEntry)
	for _, entry := range trace.Provenance {
		byRef[entry.Ref] = entry
	}
	candidate := canonical.Groups[0].Candidates[0]
	unitBinding := trace.UnitBindings[0]
	if got := reconstructSourceProvenance(trace.SourceSnapshot, byRef[unitBinding.ProvenanceRef]); got != candidate.Source.Provenance {
		t.Fatalf("unit provenance = %#v, want %#v", got, candidate.Source.Provenance)
	}
	relationBinding := trace.RelationBindings[0]
	relation := candidate.Source.RelationProjection.Relations[0]
	if got := reconstructSourceProvenance(trace.SourceSnapshot, byRef[relationBinding.ProvenanceRef]); got != relation.Provenance {
		t.Fatalf("relation provenance = %#v, want %#v", got, relation.Provenance)
	}
	evidenceBinding := trace.RetrievalEvidenceBindings[0]
	evidence := candidate.MatchGrounds[0].Evidence
	if got := reconstructSourceProvenance(trace.SourceSnapshot, byRef[evidenceBinding.ProvenanceRef]); got != evidence.Provenance {
		t.Fatalf("evidence provenance = %#v, want %#v", got, evidence.Provenance)
	}
}

func TestDiagnosticProjectionAloneExposesRetrievalInternals(t *testing.T) {
	execution := mustCanonicalQueryExecution(
		t,
		ConcernQuery{Text: "bounded concern"},
		publicProjectionCandidateSet(2),
		publicProjectionSnapshot(t, "revision-a"),
	)
	published, err := ProjectQueryResult(
		execution,
		mustQueryPublicationRequest(t, "diagnostic", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded := mustEncodePublishedQuery(t, published)
	for _, key := range []string{"provenance", "match_grounds", "tier", "probe_field", "producer_ids", "basis"} {
		if !recursiveJSONKeyExists(t, encoded, key) {
			t.Fatalf("diagnostic response lacks %q:\n%s", key, encoded)
		}
	}
}

func TestTraceReplayFailsClosedBeforeRetrievalOnSnapshotOrRequestDrift(t *testing.T) {
	requestA := ConcernQuery{Text: "bounded concern"}
	snapshotA := publicProjectionSnapshot(t, "revision-a")
	executionA := mustCanonicalQueryExecution(
		t,
		requestA,
		publicProjectionCandidateSet(1),
		snapshotA,
	)
	workingA, err := ProjectQueryResult(
		executionA,
		mustQueryPublicationRequest(t, "working", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	traceRef := workingA.(workingCandidateSet).TraceRef.String()
	traceRequest := mustQueryPublicationRequest(t, "trace", traceRef)

	tests := []struct {
		name     string
		request  QueryRequest
		snapshot QuerySourceSnapshot
		wantCode QueryReplayMismatchCode
	}{
		{
			name:     "source snapshot",
			request:  requestA,
			snapshot: publicProjectionSnapshot(t, "revision-b"),
			wantCode: QueryReplayMismatchSourceSnapshot,
		},
		{
			name:     "typed request",
			request:  ConcernQuery{Text: "different concern"},
			snapshot: snapshotA,
			wantCode: QueryReplayMismatchRequest,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preflight, err := NewQueryReplayPreflight(test.request, test.snapshot)
			if err != nil {
				t.Fatal(err)
			}
			mismatch, proceed, err := preflight.Check(traceRequest)
			if err != nil {
				t.Fatal(err)
			}
			if proceed {
				t.Fatal("replay preflight allowed retrieval after drift")
			}
			typed := mismatch.(queryReplayMismatch)
			if typed.Code != test.wantCode {
				t.Fatalf("mismatch code = %q, want %q", typed.Code, test.wantCode)
			}
		})
	}
}

func TestTraceReplayDetectsCanonicalResultDriftAfterPreflight(t *testing.T) {
	request := ConcernQuery{Text: "bounded concern"}
	snapshot := publicProjectionSnapshot(t, "revision-a")
	executionA := mustCanonicalQueryExecution(t, request, publicProjectionCandidateSet(1), snapshot)
	workingA, err := ProjectQueryResult(
		executionA,
		mustQueryPublicationRequest(t, "working", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	traceRef := workingA.(workingCandidateSet).TraceRef.String()

	changed := publicProjectionCandidateSet(1)
	changed.Groups[0].Candidates[0].Source.Title = "changed title under same source snapshot"
	executionB := mustCanonicalQueryExecution(t, request, changed, snapshot)
	published, err := ProjectQueryResult(
		executionB,
		mustQueryPublicationRequest(t, "trace", traceRef),
	)
	if err != nil {
		t.Fatal(err)
	}
	mismatch := published.(queryReplayMismatch)
	if mismatch.Code != QueryReplayMismatchResult {
		t.Fatalf("mismatch = %#v", mismatch)
	}
}

func TestWorkingPayloadDoesNotGrowWithConcernOrRawGroundCount(t *testing.T) {
	base := publicProjectionCandidateSet(1)
	large := publicProjectionCandidateSet(1)
	ground := large.Groups[0].Candidates[0].MatchGrounds[0]
	large.Groups[0].Candidates[0].MatchGrounds = make([]MatchGround, 100_000)
	for index := range large.Groups[0].Candidates[0].MatchGrounds {
		large.Groups[0].Candidates[0].MatchGrounds[index] = ground
	}
	snapshot := publicProjectionSnapshot(t, "revision-a")
	baseExecution := mustCanonicalQueryExecution(
		t,
		ConcernQuery{Text: "short concern"},
		base,
		snapshot,
	)
	largeExecution := mustCanonicalQueryExecution(
		t,
		ConcernQuery{Text: strings.Repeat("q", 1<<20)},
		large,
		snapshot,
	)
	basePublished, err := ProjectQueryResult(
		baseExecution,
		mustQueryPublicationRequest(t, "working", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	largePublished, err := ProjectQueryResult(
		largeExecution,
		mustQueryPublicationRequest(t, "working", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	baseBytes := mustEncodePublishedQuery(t, basePublished)
	largeBytes := mustEncodePublishedQuery(t, largePublished)
	if len(baseBytes) != len(largeBytes) {
		t.Fatalf("working payload grew from %d to %d bytes", len(baseBytes), len(largeBytes))
	}
}

func TestWorkingPayloadBoundsOversizedCandidateDirectRefsAndAbstentionBasis(t *testing.T) {
	t.Run("candidate direct refs", func(t *testing.T) {
		result := publicProjectionCandidateSet(1)
		result.Groups[0].Candidates[0].Source.DirectRefs = make([]string, 100_000)
		for index := range result.Groups[0].Candidates[0].Source.DirectRefs {
			result.Groups[0].Candidates[0].Source.DirectRefs[index] = fmt.Sprintf("A.%06d", index)
		}
		execution := mustCanonicalQueryExecution(
			t,
			ConcernQuery{Text: "bounded concern"},
			result,
			publicProjectionSnapshot(t, "revision-a"),
		)
		published, err := ProjectQueryResult(
			execution,
			mustQueryPublicationRequest(t, "working", ""),
		)
		if err != nil {
			t.Fatal(err)
		}
		encoded := mustEncodePublishedQuery(t, published)
		payload := decodePublishedObject(t, encoded)
		groups := payload["groups"].([]any)
		candidate := groups[0].(map[string]any)["candidates"].([]any)[0].(map[string]any)
		source := candidate["source"].(map[string]any)
		directRefs := source["direct_refs"].([]any)
		if len(directRefs) != workingDirectReferenceMax {
			t.Fatalf("working candidate direct refs = %d, want %d", len(directRefs), workingDirectReferenceMax)
		}
		if source["direct_refs_truncated"] != true {
			t.Fatalf("working candidate direct-ref posture = %#v", source["direct_refs_truncated"])
		}
		wantOmitted := float64(len(result.Groups[0].Candidates[0].Source.DirectRefs) - workingDirectReferenceMax)
		if source["direct_refs_omitted_at_least"] != wantOmitted {
			t.Fatalf("working candidate omitted direct refs = %#v, want %#v", source["direct_refs_omitted_at_least"], wantOmitted)
		}
		if len(encoded) > 4<<10 {
			t.Fatalf("working candidate payload = %d bytes, want at most 4 KiB", len(encoded))
		}
	})

	t.Run("abstention missing basis", func(t *testing.T) {
		oversizedBasis := strings.Repeat("missing basis ", 320)
		missingBasis := make([]string, 100_000)
		for index := range missingBasis {
			missingBasis[index] = "missing basis"
		}
		for index := range 9 {
			missingBasis[index] = oversizedBasis
		}
		result := Abstained{
			Kind:         QueryResultKindAbstained,
			Query:        strings.Repeat("unbounded query ", 10_000),
			Reason:       "insufficient exact source basis",
			MissingBasis: missingBasis,
		}
		execution := mustCanonicalQueryExecution(
			t,
			ConcernQuery{Text: "bounded concern"},
			result,
			publicProjectionSnapshot(t, "revision-a"),
		)
		published, err := ProjectQueryResult(
			execution,
			mustQueryPublicationRequest(t, "working", ""),
		)
		if err != nil {
			t.Fatal(err)
		}
		encoded := mustEncodePublishedQuery(t, published)
		payload := decodePublishedObject(t, encoded)
		projectedBasis := payload["missing_basis"].([]any)
		if len(projectedBasis) != workingMissingBasisMax {
			t.Fatalf("working missing basis = %d items, want %d", len(projectedBasis), workingMissingBasisMax)
		}
		for index, item := range projectedBasis {
			if got := len([]rune(item.(string))); got > workingMissingBasisRunes {
				t.Fatalf("working missing basis item %d = %d runes, want at most %d", index, got, workingMissingBasisRunes)
			}
		}
		if payload["missing_basis_truncated"] != true {
			t.Fatalf("working missing-basis posture = %#v", payload["missing_basis_truncated"])
		}
		if len(encoded) > 4<<10 {
			t.Fatalf("working abstention payload = %d bytes, want at most 4 KiB", len(encoded))
		}
	})
}

func TestCanonicalProvenanceValidationPrecedesProjection(t *testing.T) {
	snapshot := publicProjectionSnapshot(t, "revision-a")
	request := InspectQuery{Identifier: "A.1"}
	tests := []struct {
		name   string
		mutate func(ExactHit) ExactHit
	}{
		{
			name: "content hash",
			mutate: func(result ExactHit) ExactHit {
				result.Unit.Provenance.ContentHash = "tampered"
				return result
			},
		},
		{
			name: "source revision",
			mutate: func(result ExactHit) ExactHit {
				result.Unit.Provenance.SourceRevision = "other-revision"
				return result
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.mutate(publicProjectionExactHit("full body"))
			evaluation := testQueryEvaluation(request, result)
			if _, err := NewCanonicalQueryExecution(request, evaluation, snapshot); err == nil {
				t.Fatal("invalid canonical provenance reached public projector")
			}
		})
	}
}

func TestCanonicalProvenanceRejectsEmptyPathsAndInvalidLineRanges(t *testing.T) {
	type provenanceMutation func(*SourceProvenance)
	type canonicalCarrier struct {
		name  string
		build func(provenanceMutation) (QueryRequest, QueryResult)
	}
	mutations := []struct {
		name  string
		apply provenanceMutation
	}{
		{
			name: "empty path",
			apply: func(provenance *SourceProvenance) {
				provenance.SourcePath = ""
			},
		},
		{
			name: "non-positive start line",
			apply: func(provenance *SourceProvenance) {
				provenance.StartLine = 0
			},
		},
		{
			name: "end line before start line",
			apply: func(provenance *SourceProvenance) {
				provenance.EndLine = provenance.StartLine - 1
			},
		},
	}
	carriers := []canonicalCarrier{
		{
			name: "unit provenance",
			build: func(mutate provenanceMutation) (QueryRequest, QueryResult) {
				result := publicProjectionExactHit("full body")
				mutate(&result.Unit.Provenance)
				return InspectQuery{Identifier: "A.1"}, result
			},
		},
		{
			name: "relation provenance",
			build: func(mutate provenanceMutation) (QueryRequest, QueryResult) {
				result := publicProjectionExactHit("full body")
				mutate(&result.Unit.Relations[0].Provenance)
				return InspectQuery{Identifier: "A.1"}, result
			},
		},
		{
			name: "evidence provenance",
			build: func(mutate provenanceMutation) (QueryRequest, QueryResult) {
				result := publicProjectionCandidateSet(1)
				mutate(&result.Groups[0].Candidates[0].MatchGrounds[0].Evidence.Provenance)
				return ConcernQuery{Text: "bounded concern"}, result
			},
		},
	}
	snapshot := publicProjectionSnapshot(t, "revision-a")
	for _, carrier := range carriers {
		for _, mutation := range mutations {
			t.Run(carrier.name+"/"+mutation.name, func(t *testing.T) {
				request, result := carrier.build(mutation.apply)
				evaluation := testQueryEvaluation(request, result)
				if _, err := NewCanonicalQueryExecution(request, evaluation, snapshot); err == nil {
					t.Fatal("invalid canonical provenance reached public projector")
				}
			})
		}
	}
}

func TestQueryEvaluationAndCanonicalExecutionOwnSnapshots(t *testing.T) {
	request := ConcernQuery{
		Text:         "bounded concern",
		KnownContext: []string{"original context"},
	}
	canonical := publicProjectionCandidateSet(1)
	evaluation := newQueryEvaluation(
		canonical,
		queryEvaluationProducerIDs(request, canonical, nil),
	)

	canonical.Groups[0].Candidates[0].Source.Title = "forged input title"
	canonical.Groups[0].Candidates[0].Source.Provenance.SourcePath = "forged/input.md"
	exposedEvaluation := evaluation.Result().(CandidateSet)
	exposedEvaluation.Groups[0].Candidates[0].Source.Title = "forged evaluation title"
	exposedEvaluation.Groups[0].Candidates[0].Source.Provenance.SourcePath = "forged/evaluation.md"

	execution, err := NewCanonicalQueryExecution(
		request,
		evaluation,
		publicProjectionSnapshot(t, "revision-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.KnownContext[0] = "forged caller request"
	ownedEvaluation := evaluation.canonicalResult().(CandidateSet)
	ownedEvaluation.Groups[0].Candidates[0].Source.Title = "forged post-construction evaluation"
	ownedEvaluation.Groups[0].Candidates[0].Source.Provenance.SourcePath = "forged/post-construction.md"

	exposedExecution := execution.Result().(CandidateSet)
	exposedCandidate := &exposedExecution.Groups[0].Candidates[0]
	exposedCandidate.Source.Title = "forged execution title"
	exposedCandidate.Source.Provenance.SourcePath = "forged/execution.md"
	exposedCandidate.Source.RelationProjection.Relations[0].Provenance.SourcePath = "forged/relation.md"
	exposedCandidate.MatchGrounds[0].Evidence.Provenance.SourcePath = "forged/evidence.md"
	exposedRequest := execution.Request().(ConcernQuery)
	exposedRequest.KnownContext[0] = "forged exposed request"

	published, err := ProjectQueryResult(
		execution,
		mustQueryPublicationRequest(t, "trace", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded := mustEncodePublishedQuery(t, published)
	if strings.Contains(string(encoded), "forged") {
		t.Fatalf("snapshot-owned execution exposed caller mutation:\n%s", encoded)
	}
	trace := published.(traceCandidateSet)
	gotTitle := trace.Groups[0].Candidates[0].Source.Title
	if gotTitle != "Pattern A.1" {
		t.Fatalf("snapshot-owned title = %q", gotTitle)
	}
	gotRequest := execution.Request().(ConcernQuery)
	if !reflect.DeepEqual(gotRequest.KnownContext, []string{"original context"}) {
		t.Fatalf("snapshot-owned request = %#v", gotRequest)
	}
}

func TestQueryPublicationRequestUsesWorkingDefaultAndClosedViews(t *testing.T) {
	request, err := NewQueryPublicationRequest("", "")
	if err != nil {
		t.Fatal(err)
	}
	if request.View() != QueryPublicationViewWorking {
		t.Fatalf("default view = %q", request.View())
	}
	for _, view := range []string{"working", "trace", "diagnostic"} {
		if _, err := NewQueryPublicationRequest(view, ""); err != nil {
			t.Fatalf("view %q: %v", view, err)
		}
	}
	if _, err := NewQueryPublicationRequest("audit-ish", ""); err == nil {
		t.Fatal("unknown public view accepted")
	}
	if _, err := NewQueryPublicationRequest("working", "not-a-trace-ref"); err == nil {
		t.Fatal("working view accepted replay coordinates")
	}
}

func TestPublishedEncoderRejectsForgedWorkingLabelsOnInternalCarriers(t *testing.T) {
	execution := mustCanonicalQueryExecution(
		t,
		ConcernQuery{Text: "bounded concern"},
		publicProjectionCandidateSet(1),
		publicProjectionSnapshot(t, "revision-a"),
	)
	published, err := ProjectQueryResult(
		execution,
		mustQueryPublicationRequest(t, "working", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	traceRef := published.TraceReference()
	for name, forged := range map[string]PublishedQueryResult{
		"trace exact": traceExactHit{
			View:     QueryPublicationViewWorking,
			TraceRef: traceRef,
		},
		"trace candidates": traceCandidateSet{
			View:     QueryPublicationViewWorking,
			TraceRef: traceRef,
		},
		"trace abstained": traceAbstained{
			View:     QueryPublicationViewWorking,
			TraceRef: traceRef,
		},
		"diagnostic exact": diagnosticExactHit{
			View:     QueryPublicationViewWorking,
			TraceRef: traceRef,
		},
		"diagnostic candidates": diagnosticCandidateSet{
			View:     QueryPublicationViewWorking,
			TraceRef: traceRef,
		},
		"diagnostic abstained": diagnosticAbstained{
			View:     QueryPublicationViewWorking,
			TraceRef: traceRef,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := EncodePublishedQuery(forged, PublishedQueryJSONCompact); err == nil {
				t.Fatal("forged working label reached JSON encoder")
			}
		})
	}
}

func TestWorkingExactLookupBudgetsCuesRelationsAndReferencesButInspectDoesNot(t *testing.T) {
	result := publicProjectionExactHit("complete authoritative body")
	result.Unit.UseCues = SourceUseCues{
		ConditionText:   strings.Repeat("condition ", 100),
		FirstResultText: strings.Repeat("first result ", 100),
		StopReturnText:  strings.Repeat("stop return ", 100),
	}
	result.Unit.DirectRefs = make([]string, 100)
	for index := range result.Unit.DirectRefs {
		result.Unit.DirectRefs[index] = fmt.Sprintf("A.%d", index)
	}
	relation := result.Unit.Relations[0]
	result.Unit.Relations = make([]SourceRelation, 200)
	for index := range result.Unit.Relations {
		result.Unit.Relations[index] = relation
		result.Unit.Relations[index].TargetPatternID = fmt.Sprintf("B.%d", index)
	}
	snapshot := publicProjectionSnapshot(t, "revision-a")
	lookupRequest := LookupQuery{
		Identifier: "A.1",
		ResponseBudget: ResponseBudget{
			MaxExcerptCharacters:     30,
			MaxRelationsPerCandidate: 2,
		},
	}
	lookup := mustCanonicalQueryExecution(t, lookupRequest, result, snapshot)
	lookupPublished, err := ProjectQueryResult(
		lookup,
		mustQueryPublicationRequest(t, "working", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	lookupPayload := decodePublishedObject(t, mustEncodePublishedQuery(t, lookupPublished))
	lookupUnit := lookupPayload["unit"].(map[string]any)
	if _, exists := lookupUnit["body"]; exists {
		t.Fatal("working lookup exposed the authoritative body")
	}
	if got := len(lookupUnit["direct_refs"].([]any)); got != workingDirectReferenceMax {
		t.Fatalf("working lookup direct refs = %d, want %d", got, workingDirectReferenceMax)
	}
	if lookupUnit["direct_refs_truncated"] != true {
		t.Fatalf("working lookup direct-ref posture = %#v", lookupUnit["direct_refs_truncated"])
	}
	useCues := lookupUnit["use_cues"].(map[string]any)
	cueRunes := 0
	for _, value := range useCues {
		cueRunes += len([]rune(value.(string)))
	}
	if cueRunes > 30 || lookupUnit["use_cues_truncated"] != true {
		t.Fatalf("working lookup cue budget = %d runes, posture=%#v", cueRunes, lookupUnit["use_cues_truncated"])
	}
	relationProjection := lookupUnit["relation_projection"].(map[string]any)
	if got := len(relationProjection["relations"].([]any)); got != 2 {
		t.Fatalf("working lookup relations = %d, want 2", got)
	}
	if relationProjection["truncated"] != true || relationProjection["omitted_at_least"] != float64(198) {
		t.Fatalf("working lookup relation posture = %#v", relationProjection)
	}

	inspectRequest := InspectQuery{Identifier: "A.1"}
	inspect := mustCanonicalQueryExecution(t, inspectRequest, result, snapshot)
	inspectPublished, err := ProjectQueryResult(
		inspect,
		mustQueryPublicationRequest(t, "working", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	inspectPayload := decodePublishedObject(t, mustEncodePublishedQuery(t, inspectPublished))
	inspectUnit := inspectPayload["unit"].(map[string]any)
	if inspectUnit["body"] != result.Unit.Body {
		t.Fatalf("inspect body = %#v", inspectUnit["body"])
	}
	if got := len(inspectUnit["direct_refs"].([]any)); got != len(result.Unit.DirectRefs) {
		t.Fatalf("inspect direct refs = %d, want %d", got, len(result.Unit.DirectRefs))
	}
	if got := len(inspectUnit["relations"].([]any)); got != len(result.Unit.Relations) {
		t.Fatalf("inspect relations = %d, want %d", got, len(result.Unit.Relations))
	}
}

func TestQuerySourceSnapshotRejectsMalformedOrWrongCoordinates(t *testing.T) {
	validRevision := strings.Repeat("a", 40)
	validDigest := "sha256:" + strings.Repeat("1", 64)
	tests := []struct {
		name     string
		schema   string
		revision string
		readme   string
		spec     string
	}{
		{name: "wrong schema", schema: "10", revision: validRevision, readme: validDigest, spec: validDigest},
		{name: "malformed revision", schema: SpecIndexSchemaVersion, revision: "revision", readme: validDigest, spec: validDigest},
		{name: "malformed README digest", schema: SpecIndexSchemaVersion, revision: validRevision, readme: "sha256:short", spec: validDigest},
		{name: "malformed spec digest", schema: SpecIndexSchemaVersion, revision: validRevision, readme: validDigest, spec: "not-a-digest"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewQuerySourceSnapshot(
				test.schema,
				test.revision,
				test.readme,
				test.spec,
			); err == nil {
				t.Fatal("invalid source snapshot accepted")
			}
		})
	}
}

func publicProjectionSnapshot(t *testing.T, revision string) QuerySourceSnapshot {
	t.Helper()
	snapshot, err := NewQuerySourceSnapshot(
		SpecIndexSchemaVersion,
		publicProjectionRevision(revision),
		"sha256:"+strings.Repeat("1", 64),
		"sha256:"+strings.Repeat("2", 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func publicProjectionExactHit(body string) ExactHit {
	unitProvenance := publicProjectionProvenance("revision-a", "data/FPF/FPF-Spec.md", 10, body)
	relationBody := "ToC authored relation"
	relationProvenance := publicProjectionProvenance("revision-a", "data/FPF/FPF-Spec.md", 2, relationBody)
	return ExactHit{
		Kind:       QueryResultKindExactHit,
		Identifier: "A.1",
		Unit: SourceUnit{
			UnitID:            "spec:pattern_body:a-1",
			SourceID:          "A.1",
			Role:              SourceUnitRolePatternBody,
			Title:             "Pattern A.1",
			Body:              body,
			PatternID:         "A.1",
			PublicationStatus: "Stable",
			DirectRefs:        []string{"A.0"},
			Relations: []SourceRelation{{
				Kind:            SourceRelationKindBuildsOn,
				TargetPatternID: "A.0",
				TargetClass:     SourceRelationTargetClassLocalPattern,
				Origin:          SourceRelationOriginTOCExplicit,
				Provenance:      relationProvenance,
			}},
			AuthoredPhrases: []string{"How to apply A.1?"},
			Keywords:        []string{"pattern"},
			Provenance:      unitProvenance,
		},
	}
}

func publicProjectionCandidateSet(groundCount int) CandidateSet {
	unitBody := "Pattern A.1 source body"
	unitProvenance := publicProjectionProvenance("revision-a", "data/FPF/FPF-Spec.md", 10, unitBody)
	relationBody := "ToC authored relation"
	relationProvenance := publicProjectionProvenance("revision-a", "data/FPF/FPF-Spec.md", 2, relationBody)
	evidenceBody := "Navigation source witness"
	evidenceProvenance := publicProjectionProvenance("revision-a", "data/FPF/Readme.md", 4, evidenceBody)
	grounds := make([]MatchGround, groundCount)
	for index := range grounds {
		grounds[index] = MatchGround{
			Tier:         RetrievalTierHeadingKeyword,
			ProbeField:   "text",
			SourceField:  "heading",
			MatchedValue: "pattern",
			PhraseKind:   SourcePhraseKindExactProbeSpan,
			Evidence: &MatchGroundEvidence{
				UnitID:             "readme:practical_use_card:pattern",
				PatternID:          "A.1",
				SourceRole:         SourceUnitRolePracticalUseCard,
				Provenance:         evidenceProvenance,
				ProjectionRelation: "navigation_for_pattern",
			},
		}
	}
	candidate := SourceCandidate{
		Source: CandidateSourceUnit{
			UnitID:            "toc:a-1",
			SourceID:          "A.1",
			SourceRole:        SourceUnitRoleTOCRow,
			Title:             "Pattern A.1",
			Excerpt:           "bounded source excerpt",
			ExcerptTruncated:  true,
			PatternID:         "A.1",
			PublicationStatus: "Stable",
			DirectRefs:        []string{"A.0"},
			RelationProjection: &CandidateRelationProjection{
				SubjectPatternID: "A.1",
				CanonicalUnitID:  "spec:pattern_body:a-1",
				Relations: []SourceRelation{
					{
						Kind:            SourceRelationKindBuildsOn,
						TargetPatternID: "A.0",
						TargetClass:     SourceRelationTargetClassLocalPattern,
						Origin:          SourceRelationOriginTOCExplicit,
						Provenance:      relationProvenance,
					},
					{
						Kind:            SourceRelationKindCoordinatesWith,
						TargetPatternID: "A.2",
						TargetClass:     SourceRelationTargetClassLocalPattern,
						Origin:          SourceRelationOriginTOCExplicit,
						Provenance:      relationProvenance,
					},
				},
				Truncated:      true,
				OmittedAtLeast: 1,
			},
			Provenance: unitProvenance,
		},
		MatchGrounds: grounds,
	}
	return CandidateSet{
		Kind:    QueryResultKindCandidateSet,
		Concern: "bounded concern",
		Groups: []SourceCandidateGroup{{
			Role:       SourceUnitRoleTOCRow,
			Candidates: []SourceCandidate{candidate},
		}},
		Truncation: CandidateTruncation{
			Applied: true,
			Budget: ResponseBudget{
				MaxCandidatesPerRole:     5,
				MaxTotalCandidates:       10,
				MaxExcerptCharacters:     480,
				MaxRelationsPerCandidate: 12,
			},
			IncludedCandidates: 1,
			OmittedAtLeast:     2,
			Basis:              []string{"response_budget", "role_local_fts_producer_limit"},
		},
	}
}

func publicProjectionProvenance(
	revision string,
	path string,
	line int,
	body string,
) SourceProvenance {
	return SourceProvenance{
		SourcePath:     path,
		StartLine:      line,
		EndLine:        line,
		ContentHash:    sourceContentHash(body),
		SourceRevision: publicProjectionRevision(revision),
	}
}

func publicProjectionRevision(label string) string {
	digest := sha256.Sum256([]byte(label))
	return hex.EncodeToString(digest[:20])
}

func mustCanonicalQueryExecution(
	t *testing.T,
	request QueryRequest,
	result QueryResult,
	snapshot QuerySourceSnapshot,
) CanonicalQueryExecution {
	t.Helper()
	execution, err := NewCanonicalQueryExecution(
		request,
		testQueryEvaluation(request, result),
		snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	return execution
}

func testQueryEvaluation(request QueryRequest, result QueryResult) QueryEvaluation {
	producerIDs := queryEvaluationProducerIDs(request, result, nil)
	return newQueryEvaluation(result, producerIDs)
}

func mustQueryPublicationRequest(t *testing.T, view, traceRef string) QueryPublicationRequest {
	t.Helper()
	request, err := NewQueryPublicationRequest(view, traceRef)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func mustEncodePublishedQuery(t *testing.T, result PublishedQueryResult) []byte {
	t.Helper()
	encoded, err := EncodePublishedQuery(result, PublishedQueryJSONCompact)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func decodePublishedObject(t *testing.T, encoded []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func recursiveJSONKeyExists(t *testing.T, encoded []byte, key string) bool {
	t.Helper()
	return recursiveJSONKeyCount(t, encoded, key) > 0
}

func recursiveJSONKeyCount(t *testing.T, encoded []byte, key string) int {
	t.Helper()
	var payload any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	return countRecursiveJSONKey(payload, key)
}

func countRecursiveJSONKey(value any, key string) int {
	switch typed := value.(type) {
	case map[string]any:
		count := 0
		for childKey, child := range typed {
			if childKey == key {
				count++
			}
			count += countRecursiveJSONKey(child, key)
		}
		return count
	case []any:
		count := 0
		for _, child := range typed {
			count += countRecursiveJSONKey(child, key)
		}
		return count
	default:
		return 0
	}
}

func reconstructSourceProvenance(
	snapshot TraceSourceSnapshot,
	entry TraceProvenanceEntry,
) SourceProvenance {
	return SourceProvenance{
		SourcePath:     entry.SourcePath,
		StartLine:      entry.StartLine,
		EndLine:        entry.EndLine,
		ContentHash:    entry.ContentHash,
		SourceRevision: snapshot.SourceRevision,
	}
}
