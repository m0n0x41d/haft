package codeintel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHGExploreProjectionContract(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	const filePath = "internal/cli/memory_read_runtime.go"
	writeCurrentnessSource(
		t,
		root,
		filePath,
		"package cli\nfunc NeighborhoodRead() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	service := NewService(store)
	if _, err := service.EnsureIndex(ctx, root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(
		ctx,
		`DELETE FROM codebase_modules`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(
		ctx,
		`INSERT INTO codebase_modules
			(module_id, path, name, lang, file_count, last_scanned)
		 VALUES ('mod-cli', 'internal/cli', 'cli', 'go', 1, ?)`,
		time.Now().UTC(),
	); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	createDecision := func(id string, structuredData string) {
		t.Helper()
		item := &artifact.Artifact{
			Meta: artifact.Meta{
				ID:        id,
				Kind:      artifact.KindDecisionRecord,
				Version:   1,
				Status:    artifact.StatusActive,
				Title:     id,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Body:           "fixture",
			StructuredData: structuredData,
		}
		if err := store.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	createDecision(
		"dec-explore-exact",
		`{"binding_targets":[{"kind":"whole_file_fallback","file_path":"internal/cli/memory_read_runtime.go"}]}`,
	)
	createDecision(
		"dec-explore-context",
		`{"implementation_footprint":{"files":["internal/cli/memory_read_runtime.go"]}}`,
	)
	if err := store.SetAffectedFiles(
		ctx,
		"dec-explore-context",
		[]artifact.AffectedFile{{Path: filePath}},
	); err != nil {
		t.Fatal(err)
	}
	createDecision(
		"dec-explore-module",
		`{"binding_targets":[{"kind":"module","module_path":"internal/cli"}]}`,
	)

	request, err := NewExploreExecutionRequest(
		"NeighborhoodRead",
		filePath,
		0,
		"",
		DefaultConcernCandidateBudget,
	)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := NewExplorePublicationRequest("working", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := PublishExplore(
		ctx,
		service,
		root,
		request,
		publication,
	)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := EncodePublishedExplore(
		result,
		PublishedExploreJSONCompact,
	)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Kind           PublishedExploreKind `json:"kind"`
		SeedResolution struct {
			Kind string `json:"kind"`
		} `json:"seed_resolution"`
		ReasoningContext []struct {
			SymbolAnchor                    string   `json:"symbol_anchor"`
			ExactBindingDecisionRefs        []string `json:"exact_binding_decision_refs"`
			AffectedPathContextDecisionRefs []string `json:"affected_path_context_decision_refs"`
			ModuleDecisionRefs              []string `json:"module_decision_refs"`
		} `json:"reasoning_context"`
	}
	if err := json.Unmarshal(wire, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Kind != PublishedExploreKindResolved ||
		payload.SeedResolution.Kind != "resolved_seed" {
		t.Fatalf("explore outcome = %+v; payload=%s", payload, wire)
	}
	if len(payload.ReasoningContext) == 0 ||
		payload.ReasoningContext[0].SymbolAnchor == "" {
		t.Fatalf("reasoning context lacks symbol identity: %s", wire)
	}
	context := payload.ReasoningContext[0]
	assertExploreStringSet(
		t,
		context.ExactBindingDecisionRefs,
		[]string{"dec-explore-exact"},
	)
	assertExploreStringSet(
		t,
		context.AffectedPathContextDecisionRefs,
		[]string{"dec-explore-context"},
	)
	assertExploreStringSet(
		t,
		context.ModuleDecisionRefs,
		[]string{"dec-explore-module"},
	)
}

func assertExploreStringSet(
	t *testing.T,
	actual []string,
	expected []string,
) {
	t.Helper()
	actualSet := make(map[string]bool, len(actual))
	for _, item := range actual {
		actualSet[item] = true
	}
	if len(actualSet) != len(expected) {
		t.Fatalf("actual=%v expected=%v", actual, expected)
	}
	for _, item := range expected {
		if !actualSet[item] {
			t.Fatalf("actual=%v expected=%v", actual, expected)
		}
	}
}

func TestExploreExecutionRequestIsExactOneOf(t *testing.T) {
	testCases := []struct {
		name    string
		symbol  string
		query   string
		wantErr string
	}{
		{
			name:    "neither",
			wantErr: "exactly one",
		},
		{
			name:    "both",
			symbol:  "Publish",
			query:   "where is publication",
			wantErr: "exactly one",
		},
		{
			name:    "blank concern",
			query:   " ",
			wantErr: "concern query must not be blank",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewExploreExecutionRequest(
				testCase.symbol,
				"",
				0,
				testCase.query,
				DefaultConcernCandidateBudget,
			)
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error = %v, want %q", err, testCase.wantErr)
			}
		})
	}
}

func TestPublishedExploreWorkingConcernIsBoundedAndScoreFree(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"service.go",
		`package sample
func PublishAlpha() {}
func PublishBeta() {}
func PublishGamma() {}
func PublishDelta() {}
func PublishEpsilon() {}
func PublishZeta() {}
`,
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()

	request, err := NewExploreExecutionRequest(
		"",
		"",
		0,
		"publish",
		12,
	)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := NewExplorePublicationRequest("working", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := PublishExplore(
		ctx,
		NewService(store),
		root,
		request,
		publication,
	)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := EncodePublishedExplore(result, PublishedExploreJSONCompact)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(wire, &payload); err != nil {
		t.Fatal(err)
	}
	candidates, ok := payload["candidates"].([]any)
	if !ok || len(candidates) != workingExploreCandidateMax {
		t.Fatalf("working candidates = %#v", payload["candidates"])
	}
	for _, forbidden := range []string{
		`"combined":`,
		`"code_lexical":`,
		`"reasoning":`,
		`"typed_memory":`,
		`"ppr":`,
		`"selected"`,
		`"winner"`,
		`"reasoning_artifacts"`,
		`"source_lane"`,
		`"direct_bridge"`,
		`"origin_lanes"`,
		`"provenance"`,
		`"trace_basis"`,
	} {
		if strings.Contains(string(wire), forbidden) {
			t.Fatalf("working projection leaked %s: %s", forbidden, wire)
		}
	}
	if result.PublicationView() != ExplorePublicationViewWorking ||
		result.PublishedKind() != PublishedExploreKindCandidateSet {
		t.Fatalf(
			"publication view=%s kind=%s",
			result.PublicationView(),
			result.PublishedKind(),
		)
	}
}

func TestPublishedExploreTraceIsDeterministicAndDiagnosticIsExplicit(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"service.go",
		"package sample\nfunc PublishEpoch() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	service := NewService(store)

	request, err := NewExploreExecutionRequest(
		"PublishEpoch",
		"",
		0,
		"",
		DefaultConcernCandidateBudget,
	)
	if err != nil {
		t.Fatal(err)
	}
	traceRequest, err := NewExplorePublicationRequest("trace", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := PublishExplore(
		ctx,
		service,
		root,
		request,
		traceRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PublishExplore(
		ctx,
		service,
		root,
		request,
		traceRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	firstWire, err := EncodePublishedExplore(
		first,
		PublishedExploreJSONCompact,
	)
	if err != nil {
		t.Fatal(err)
	}
	secondWire, err := EncodePublishedExplore(
		second,
		PublishedExploreJSONCompact,
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstWire) != string(secondWire) {
		t.Fatalf("trace projection is not deterministic:\n%s\n%s", firstWire, secondWire)
	}
	if first.TraceReference().String() == "" ||
		!strings.Contains(string(firstWire), `"trace_basis"`) {
		t.Fatalf("trace basis missing: %s", firstWire)
	}

	diagnosticRequest, err := NewExplorePublicationRequest(
		"diagnostic",
		first.TraceReference().String(),
	)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := PublishExplore(
		ctx,
		service,
		root,
		request,
		diagnosticRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	diagnosticWire, err := EncodePublishedExplore(
		diagnostic,
		PublishedExploreJSONCompact,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`"resolution_diagnostics"`,
		`"retrieval_diagnostics"`,
		`"trace_basis"`,
	} {
		if !strings.Contains(string(diagnosticWire), required) {
			t.Fatalf("diagnostic projection lacks %s: %s", required, diagnosticWire)
		}
	}
}

func TestPublishedExploreReplayMismatchNamesChangedBasis(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"service.go",
		"package sample\nfunc PublishEpoch() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	service := NewService(store)

	request, err := NewExploreExecutionRequest(
		"PublishEpoch",
		"",
		0,
		"",
		DefaultConcernCandidateBudget,
	)
	if err != nil {
		t.Fatal(err)
	}
	traceRequest, err := NewExplorePublicationRequest("trace", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := PublishExplore(
		ctx,
		service,
		root,
		request,
		traceRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	writeCurrentnessSource(
		t,
		root,
		"added.go",
		"package sample\nfunc AddedAfterTrace() {}\n",
	)

	replayRequest, err := NewExplorePublicationRequest(
		"trace",
		first.TraceReference().String(),
	)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := PublishExplore(
		ctx,
		service,
		root,
		request,
		replayRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.PublishedKind() != PublishedExploreKindReplayMismatch {
		t.Fatalf("replay kind = %s", replayed.PublishedKind())
	}
	wire, err := EncodePublishedExplore(
		replayed,
		PublishedExploreJSONCompact,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wire), `"mismatch":"index_snapshot"`) {
		t.Fatalf("replay mismatch = %s", wire)
	}
}

func TestPublishedExploreWorkingSourceTruncationIsVisible(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	hostileLine := "\t// " + strings.Repeat(`\"`, 40) + "\n"
	hostileLineCount := workingExploreSourceByteMax/len(hostileLine) + 20
	body := "package sample\nfunc LargeSource() {\n" +
		strings.Repeat(hostileLine, hostileLineCount) +
		"}\n"
	writeCurrentnessSource(t, root, "large.go", body)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()

	request, err := NewExploreExecutionRequest(
		"LargeSource",
		"",
		0,
		"",
		DefaultConcernCandidateBudget,
	)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := NewExplorePublicationRequest("working", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := PublishExplore(
		ctx,
		NewService(store),
		root,
		request,
		publication,
	)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := EncodePublishedExplore(result, PublishedExploreJSONCompact)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wire), `"truncated":true`) ||
		!strings.Contains(string(wire), `"return_view":"trace"`) ||
		!strings.Contains(
			string(wire),
			`"truncation_rule":"utf8_prefix_fit_12000_byte_payload"`,
		) {
		t.Fatalf("source truncation is not explicit: %s", wire)
	}
	if len(wire) > workingExplorePayloadByteMax {
		t.Fatalf(
			"working payload = %d bytes, budget %d",
			len(wire),
			workingExplorePayloadByteMax,
		)
	}
}

func TestPublishedExploreDiagnosticUnresolvedDoesNotFabricateTraversal(
	t *testing.T,
) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"service.go",
		"package sample\nfunc ExistingSymbol() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()

	request, err := NewExploreExecutionRequest(
		"MissingSymbol",
		"",
		0,
		"",
		DefaultConcernCandidateBudget,
	)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := NewExplorePublicationRequest("diagnostic", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := PublishExplore(
		ctx,
		NewService(store),
		root,
		request,
		publication,
	)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := EncodePublishedExplore(
		result,
		PublishedExploreJSONCompact,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.PublishedKind() != PublishedExploreKindUnresolved {
		t.Fatalf("unresolved kind = %s", result.PublishedKind())
	}
	if strings.Contains(string(wire), `"traversal_outcome"`) ||
		strings.Contains(string(wire), `"chain_outcome"`) {
		t.Fatalf("unresolved projection fabricated traversal: %s", wire)
	}
}

func TestPublishedExploreWorkingHostileConcernIsEscapedDeterministicAndBounded(
	t *testing.T,
) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"internal/service/hostile.go",
		"package service\nfunc HostileBoundary() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	item := createConcernArtifact(
		t,
		ctx,
		store,
		"dec-20260726-hostile-working-payload",
		artifact.KindDecisionRecord,
		"Nebula quoted λ "+strings.Repeat("oversized-title-", 1200),
		"Nebula quoted λ boundary policy.",
	)
	if err := store.SetAffectedFiles(
		ctx,
		item.Meta.ID,
		[]artifact.AffectedFile{
			{Path: "internal/service/hostile.go"},
		},
	); err != nil {
		t.Fatal(err)
	}
	rawQuery := "Nebula\n\"quoted\" λ path:internal/service"
	request, err := NewExploreExecutionRequest(
		"",
		"",
		0,
		rawQuery,
		DefaultConcernCandidateBudget,
	)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := NewExplorePublicationRequest("working", "")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	first, err := PublishExplore(
		ctx,
		service,
		root,
		request,
		publication,
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PublishExplore(
		ctx,
		service,
		root,
		request,
		publication,
	)
	if err != nil {
		t.Fatal(err)
	}
	firstWire, err := EncodePublishedExplore(
		first,
		PublishedExploreJSONCompact,
	)
	if err != nil {
		t.Fatal(err)
	}
	secondWire, err := EncodePublishedExplore(
		second,
		PublishedExploreJSONCompact,
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstWire) != string(secondWire) {
		t.Fatalf(
			"hostile working projection is not deterministic:\n%s\n%s",
			firstWire,
			secondWire,
		)
	}
	if len(firstWire) > workingExplorePayloadByteMax {
		t.Fatalf(
			"working payload = %d bytes, budget %d",
			len(firstWire),
			workingExplorePayloadByteMax,
		)
	}
	var payload struct {
		RequestBasis struct {
			Query string `json:"query"`
		} `json:"request_basis"`
		TraceRef string `json:"trace_ref"`
	}
	if err := json.Unmarshal(firstWire, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RequestBasis.Query != rawQuery {
		t.Fatalf(
			"query round trip = %q, want %q",
			payload.RequestBasis.Query,
			rawQuery,
		)
	}
	if payload.TraceRef == "" {
		t.Fatalf("working projection lacks opaque trace ref: %+v", payload)
	}
	for _, forbidden := range []string{
		`"reasoning_artifacts"`,
		`"source_lane"`,
		`"direct_bridge"`,
		`"origin_lanes"`,
		`"provenance"`,
		"oversized-title-",
	} {
		if strings.Contains(string(firstWire), forbidden) {
			t.Fatalf("working projection leaked %q: %s", forbidden, firstWire)
		}
	}
	traceRequest, err := NewExplorePublicationRequest(
		"trace",
		payload.TraceRef,
	)
	if err != nil {
		t.Fatal(err)
	}
	trace, err := PublishExplore(
		ctx,
		service,
		root,
		request,
		traceRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	traceWire, err := EncodePublishedExplore(
		trace,
		PublishedExploreJSONCompact,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(traceWire), `"reasoning_artifacts"`) ||
		!strings.Contains(string(traceWire), "oversized-title-") {
		t.Fatalf("trace projection lacks omitted provenance: %s", traceWire)
	}
}
