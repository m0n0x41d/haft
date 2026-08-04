package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/spf13/cobra"
)

func TestSourceNativeFPFQueryIntegration(t *testing.T) {
	dbPath := buildFPFSourceQueryTestDB(t)
	restoreOpen := stubSourceQueryDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubFPFQueryFlags(t)
	defer restoreFlags()

	store := setupCLIArtifactStore(t)
	beforeArtifacts := countQueryIntegrationArtifacts(t, store)

	t.Run("omitted view is working and all result paths obey its denylist", func(t *testing.T) {
		paths := map[string]map[string]any{
			"concern candidate": {
				"action": "fpf",
				"mode":   "concern",
				"query":  "architecture structural view",
			},
			"concern abstained": {
				"action": "fpf",
				"mode":   "concern",
				"query":  "outofvocabularytoken",
			},
			"lookup exact": {
				"action":     "fpf",
				"mode":       "lookup",
				"identifier": "A.7",
			},
			"lookup candidate fallback": {
				"action":     "fpf",
				"mode":       "lookup",
				"identifier": "architecture structural view",
			},
			"lookup abstained fallback": {
				"action":     "fpf",
				"mode":       "lookup",
				"identifier": "outofvocabularytoken",
			},
			"inspect exact": {
				"action":     "fpf",
				"mode":       "inspect",
				"identifier": "A.7",
			},
			"inspect abstained": {
				"action":     "fpf",
				"mode":       "inspect",
				"identifier": "outofvocabularytoken",
			},
		}

		for name, args := range paths {
			t.Run(name, func(t *testing.T) {
				payload := runMCPFPFQueryJSON(t, store, args)
				if payload["view"] != string(fpf.QueryPublicationViewWorking) {
					t.Fatalf("omitted view = %#v, want working", payload["view"])
				}
				assertQueryIntegrationWorkingDenylist(t, payload)
				assertQueryIntegrationHasNoHiddenWinner(t, payload)
			})
		}
	})

	t.Run("three views have byte-identical compact CLI and MCP carriers", func(t *testing.T) {
		query := "architecture structural view"
		fpfQueryJSON = true

		for _, view := range []string{"working", "trace", "diagnostic"} {
			t.Run(view, func(t *testing.T) {
				fpfPublicationView = view
				cli := runFPFQueryCompactBytes(t, query)
				mcp := runMCPFPFQueryBytes(t, store, map[string]any{
					"action": "fpf",
					"mode":   "concern",
					"query":  query,
					"view":   view,
				})
				if !bytes.Equal(cli, mcp) {
					t.Fatalf("%s CLI/MCP bytes differ:\nCLI=%s\nMCP=%s", view, cli, mcp)
				}
				payload := decodeQueryIntegrationJSON(t, cli)
				if payload["view"] != view {
					t.Fatalf("published view = %#v, want %q", payload["view"], view)
				}
			})
		}
	})

	t.Run("working lookup is a summary while inspect returns the exact full body", func(t *testing.T) {
		lookup := runMCPFPFQueryJSON(t, store, map[string]any{
			"action":     "fpf",
			"mode":       "lookup",
			"identifier": "A.7",
		})
		inspect := runMCPFPFQueryJSON(t, store, map[string]any{
			"action":     "fpf",
			"mode":       "inspect",
			"identifier": "A.7",
		})

		assertQueryIntegrationKind(t, lookup, fpf.QueryResultKindExactHit)
		assertQueryIntegrationKind(t, inspect, fpf.QueryResultKindExactHit)
		lookupUnit := queryIntegrationExactUnit(t, lookup)
		if _, exists := lookupUnit["body"]; exists {
			t.Fatalf("working lookup leaked exact body: %#v", lookupUnit)
		}
		inspectUnit := queryIntegrationExactUnit(t, inspect)
		wantBody := "A.7 exact body sentinel\nProblem frame\nProblem\nForces\nSolution\nOrdinary boundary\nWorked slice\nChecklist"
		if inspectUnit["body"] != wantBody {
			t.Fatalf("working inspect body = %#v, want exact source body %q", inspectUnit["body"], wantBody)
		}
		assertQueryIntegrationWorkingDenylist(t, lookup)
		assertQueryIntegrationWorkingDenylist(t, inspect)
	})

	t.Run("trace carries replayable provenance and fails closed on drift", func(t *testing.T) {
		args := map[string]any{
			"action": "fpf",
			"mode":   "concern",
			"query":  "architecture structural view",
			"view":   "trace",
		}
		first := runMCPFPFQueryBytes(t, store, args)
		payload := decodeQueryIntegrationJSON(t, first)
		traceRef, ok := payload["trace_ref"].(string)
		if !ok || traceRef == "" {
			t.Fatalf("trace_ref = %#v", payload["trace_ref"])
		}
		assertQueryIntegrationTrace(
			t,
			payload,
			syntheticFPFSourceRevision("cli-query-test-revision"),
		)
		if count := strings.Count(string(first), `"source_revision"`); count != 1 {
			t.Fatalf("trace source_revision count = %d, want one response-wide coordinate: %s", count, first)
		}

		replayArgs := copyQueryIntegrationArgs(args)
		replayArgs["trace_ref"] = traceRef
		replayed := runMCPFPFQueryBytes(t, store, replayArgs)
		if !bytes.Equal(first, replayed) {
			t.Fatalf("same-snapshot replay changed bytes:\nfirst=%s\nreplay=%s", first, replayed)
		}

		requestDriftArgs := copyQueryIntegrationArgs(replayArgs)
		requestDriftArgs["query"] = ""
		requestMismatch := runMCPFPFQueryJSON(t, store, requestDriftArgs)
		assertQueryIntegrationReplayMismatch(
			t,
			requestMismatch,
			fpf.QueryReplayMismatchRequest,
			traceRef,
		)

		changedDB := buildFPFSourceQueryTestDBAtRevision(t, "cli-query-changed-revision")
		restoreChanged := stubSourceQueryDB(t, changedDB)
		snapshotDriftArgs := copyQueryIntegrationArgs(replayArgs)
		snapshotDriftArgs["query"] = ""
		snapshotMismatch := runMCPFPFQueryJSON(t, store, snapshotDriftArgs)
		restoreChanged()
		assertQueryIntegrationReplayMismatch(
			t,
			snapshotMismatch,
			fpf.QueryReplayMismatchSourceSnapshot,
			traceRef,
		)
	})

	t.Run("diagnostic alone exposes raw retrieval grounds", func(t *testing.T) {
		payload := runMCPFPFQueryJSON(t, store, map[string]any{
			"action": "fpf",
			"mode":   "concern",
			"query":  "architecture structural view",
			"view":   "diagnostic",
		})
		assertQueryIntegrationKind(t, payload, fpf.QueryResultKindCandidateSet)
		keys := make(map[string]bool)
		collectQueryIntegrationKeys(payload, keys)
		for _, required := range []string{
			"match_grounds",
			"tier",
			"probe_field",
			"source_field",
			"matched_value",
			"provenance",
			"producer_ids",
		} {
			if !keys[required] {
				t.Fatalf("diagnostic response lacks %q: %s", required, mustMarshalQueryIntegrationJSON(t, payload))
			}
		}
		diagnostic, ok := payload["diagnostic"].(map[string]any)
		if !ok || diagnostic["retrieval_mode"] != string(fpf.QueryModeConcern) {
			t.Fatalf("diagnostic coordinates = %#v", payload["diagnostic"])
		}
	})

	afterArtifacts := countQueryIntegrationArtifacts(t, store)
	if afterArtifacts != beforeArtifacts {
		t.Fatalf("read-only FPF Query changed artifact count from %d to %d", beforeArtifacts, afterArtifacts)
	}
}

func TestEmbeddedFPFQueryWorksFromEmptyDownstreamProject(t *testing.T) {
	restoreFlags := stubFPFQueryFlags(t)
	defer restoreFlags()
	originalOpen := openFPFDBFunc
	openFPFDBFunc = openFPFDB
	defer func() { openFPFDBFunc = originalOpen }()

	downstream := t.TempDir()
	t.Chdir(downstream)
	if _, err := os.Stat("data/FPF/FPF-Spec.md"); !os.IsNotExist(err) {
		t.Fatalf("downstream fixture unexpectedly contains FPF source path: %v", err)
	}
	publicationRequest, err := fpf.NewQueryPublicationRequest("working", "")
	if err != nil {
		t.Fatal(err)
	}
	requests := map[string]fpf.QueryRequest{
		"concern": fpf.ConcernQuery{Text: "relation occurrence assertion distinction"},
		"lookup":  fpf.LookupQuery{Identifier: "A.6.REL"},
		"inspect": fpf.InspectQuery{Identifier: "A.6.REL"},
	}
	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			published, err := publishEmbeddedFPFQuery(request, publicationRequest)
			if err != nil {
				t.Fatalf("embedded downstream %s query: %v", name, err)
			}
			encoded, err := fpf.EncodePublishedQuery(published, fpf.PublishedQueryJSONCompact)
			if err != nil {
				t.Fatal(err)
			}
			payload := decodeQueryIntegrationJSON(t, encoded)
			if payload["view"] != string(fpf.QueryPublicationViewWorking) {
				t.Fatalf("embedded downstream view = %#v", payload["view"])
			}
			assertQueryIntegrationWorkingDenylist(t, payload)
			if name != "inspect" {
				return
			}
			unit := queryIntegrationExactUnit(t, payload)
			body, ok := unit["body"].(string)
			if !ok || !strings.Contains(body, "Problem") ||
				!strings.Contains(body, "Forces") ||
				!strings.Contains(body, "Solution") {
				t.Fatalf("embedded exact inspect lacks authoritative pattern body: %#v", unit["body"])
			}
		})
	}
	t.Run("target-system concern keeps stronger phrase navigation", func(t *testing.T) {
		request := fpf.ConcernQuery{Text: "What is the target system here?"}
		published, err := publishEmbeddedFPFQuery(request, publicationRequest)
		if err != nil {
			t.Fatalf("embedded target-system query: %v", err)
		}
		encoded, err := fpf.EncodePublishedQuery(published, fpf.PublishedQueryJSONCompact)
		if err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{
			`"unit_id":"readme:practical_use_card:system-recognition"`,
			`"unit_id":"readme:practical_use_card:system-delimitation"`,
			`"pattern_id":"C.26"`,
		} {
			if !bytes.Contains(encoded, []byte(required)) {
				t.Fatalf(
					"embedded target-system query omits %s: %s",
					required,
					encoded,
				)
			}
		}
		// The current FPF source contains the exact "target system" span in
		// C.26. C.32.PAD remains an authored navigation edge from the
		// SYSTEM-DELIMITATION card; it must not be promoted into a fabricated
		// phrase witness merely to preserve an older-source result set.
		assertQueryIntegrationCandidateDirectRef(
			t,
			decodeQueryIntegrationJSON(t, encoded),
			"readme:practical_use_card:system-delimitation",
			"C.32.PAD",
		)
	})
	entries, err := os.ReadDir(downstream)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("embedded Query wrote into downstream project: %#v", entries)
	}
}

func TestFPFQueryIndexVerificationCacheRunsExactVerificationOnce(t *testing.T) {
	calls := 0
	cache := newFPFQueryIndexVerificationCache(func(*sql.DB) error {
		calls++
		return nil
	})

	if err := cache.Verify(nil); err != nil {
		t.Fatalf("first cached verification: %v", err)
	}
	if err := cache.Verify(nil); err != nil {
		t.Fatalf("replayed cached verification: %v", err)
	}
	if calls != 1 {
		t.Fatalf("verification calls = %d, want 1", calls)
	}
}

func runFPFQueryJSON(t *testing.T, query string) map[string]any {
	t.Helper()
	return decodeQueryIntegrationJSON(t, runFPFQueryCompactBytes(t, query))
}

func runFPFQueryCompactBytes(t *testing.T, query string) []byte {
	t.Helper()

	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&output)
	args := strings.Fields(query)
	if err := runFPFQuery(command, args); err != nil {
		t.Fatalf("run haft fpf query: %v", err)
	}
	return bytes.TrimSuffix(output.Bytes(), []byte("\n"))
}

func runFPFLookupJSON(t *testing.T, identifier string) map[string]any {
	t.Helper()

	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&output)
	if err := runFPFLookup(command, []string{identifier}); err != nil {
		t.Fatalf("run haft fpf lookup: %v", err)
	}
	return decodeQueryIntegrationJSON(t, output.Bytes())
}

func runMCPFPFQueryJSON(t *testing.T, store *artifact.Store, args map[string]any) map[string]any {
	t.Helper()
	return decodeQueryIntegrationJSON(t, runMCPFPFQueryBytes(t, store, args))
}

func runMCPFPFQueryBytes(t *testing.T, store *artifact.Store, args map[string]any) []byte {
	t.Helper()

	ctx := context.Background()
	haftDir := t.TempDir()
	result, err := handleQuintQuery(ctx, store, nil, haftDir, args)
	if err != nil {
		t.Fatalf("run haft_query(action=fpf): %v", err)
	}
	return []byte(result)
}

func decodeQueryIntegrationJSON(t *testing.T, encoded []byte) map[string]any {
	t.Helper()

	payload := make(map[string]any)
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode FPF Query JSON: %v\n%s", err, encoded)
	}
	return payload
}

func mustMarshalQueryIntegrationJSON(t *testing.T, payload map[string]any) string {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal FPF Query payload: %v", err)
	}
	return string(encoded)
}

func assertQueryIntegrationKind(t *testing.T, payload map[string]any, want fpf.QueryResultKind) {
	t.Helper()

	if payload["kind"] != string(want) {
		t.Fatalf("FPF Query kind = %#v, want %s", payload["kind"], want)
	}
}

func queryIntegrationCandidateCount(t *testing.T, payload map[string]any) int {
	t.Helper()

	groups, ok := payload["groups"].([]any)
	if !ok {
		t.Fatalf("CandidateSet groups = %#v", payload["groups"])
	}

	total := 0
	for _, groupValue := range groups {
		group, ok := groupValue.(map[string]any)
		if !ok {
			t.Fatalf("candidate group = %#v", groupValue)
		}
		candidates, ok := group["candidates"].([]any)
		if !ok {
			t.Fatalf("candidate list = %#v", group["candidates"])
		}
		total += len(candidates)
	}
	return total
}

func assertQueryIntegrationCandidateDirectRef(
	t *testing.T,
	payload map[string]any,
	unitID string,
	wantRef string,
) {
	t.Helper()

	groups, ok := payload["groups"].([]any)
	if !ok {
		t.Fatalf("CandidateSet groups = %#v", payload["groups"])
	}
	for _, groupValue := range groups {
		group, ok := groupValue.(map[string]any)
		if !ok {
			t.Fatalf("candidate group = %#v", groupValue)
		}
		candidates, ok := group["candidates"].([]any)
		if !ok {
			t.Fatalf("candidate list = %#v", group["candidates"])
		}
		for _, candidateValue := range candidates {
			candidate, ok := candidateValue.(map[string]any)
			if !ok {
				t.Fatalf("candidate = %#v", candidateValue)
			}
			source, ok := candidate["source"].(map[string]any)
			if !ok || source["unit_id"] != unitID {
				continue
			}
			refs, ok := source["direct_refs"].([]any)
			if !ok {
				t.Fatalf("candidate %s direct_refs = %#v", unitID, source["direct_refs"])
			}
			for _, ref := range refs {
				if ref == wantRef {
					return
				}
			}
			t.Fatalf("candidate %s omits authored direct ref %s: %#v", unitID, wantRef, refs)
		}
	}
	t.Fatalf("CandidateSet omits source unit %s", unitID)
}

func queryIntegrationExactUnit(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()

	unit, ok := payload["unit"].(map[string]any)
	if !ok {
		t.Fatalf("ExactHit unit = %#v", payload["unit"])
	}
	return unit
}

func assertQueryIntegrationWorkingDenylist(t *testing.T, payload map[string]any) {
	t.Helper()

	keys := make(map[string]bool)
	collectQueryIntegrationKeys(payload, keys)
	for _, forbidden := range []string{
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
		"concern",
		"query",
		"trace",
		"diagnostic",
	} {
		if keys[forbidden] {
			t.Fatalf("working response leaked forbidden key %q: %s", forbidden, mustMarshalQueryIntegrationJSON(t, payload))
		}
	}
}

func assertQueryIntegrationTrace(t *testing.T, payload map[string]any, wantRevision string) {
	t.Helper()

	trace, ok := payload["trace"].(map[string]any)
	if !ok {
		t.Fatalf("trace = %#v", payload["trace"])
	}
	snapshot, ok := trace["source_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("trace.source_snapshot = %#v", trace["source_snapshot"])
	}
	if snapshot["source_revision"] != wantRevision {
		t.Fatalf("trace source revision = %#v, want %q", snapshot["source_revision"], wantRevision)
	}
	if snapshot["index_schema_version"] != fpf.SpecIndexSchemaVersion {
		t.Fatalf("trace schema version = %#v, want %q", snapshot["index_schema_version"], fpf.SpecIndexSchemaVersion)
	}
	for _, key := range []string{"readme_document_digest", "specification_document_digest"} {
		value, ok := snapshot[key].(string)
		if !ok || !strings.HasPrefix(value, "sha256:") {
			t.Fatalf("trace snapshot %s = %#v", key, snapshot[key])
		}
	}
	provenance, ok := trace["provenance"].([]any)
	if !ok || len(provenance) == 0 {
		t.Fatalf("trace provenance catalog = %#v", trace["provenance"])
	}
	provenanceRefs := make(map[string]bool, len(provenance))
	for _, value := range provenance {
		entry, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("trace provenance entry = %#v", value)
		}
		ref, refOK := entry["ref"].(string)
		path, pathOK := entry["source_path"].(string)
		contentHash, hashOK := entry["content_hash"].(string)
		startLine, startOK := entry["start_line"].(float64)
		endLine, endOK := entry["end_line"].(float64)
		if !refOK || ref == "" || !pathOK || path == "" || !hashOK || len(contentHash) != 64 || !startOK || !endOK || startLine <= 0 || endLine < startLine {
			t.Fatalf("incomplete trace provenance entry = %#v", entry)
		}
		if _, duplicate := provenanceRefs[ref]; duplicate {
			t.Fatalf("duplicate trace provenance ref %q", ref)
		}
		provenanceRefs[ref] = true
	}
	unitBindings, ok := trace["unit_bindings"].([]any)
	if !ok || len(unitBindings) == 0 {
		t.Fatalf("trace unit bindings = %#v", trace["unit_bindings"])
	}
	for _, value := range unitBindings {
		binding, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("trace unit binding = %#v", value)
		}
		provenanceRef, refOK := binding["provenance_ref"].(string)
		if !refOK || !provenanceRefs[provenanceRef] {
			t.Fatalf("trace unit binding does not resolve through provenance catalog: %#v", value)
		}
	}
}

func assertQueryIntegrationReplayMismatch(
	t *testing.T,
	payload map[string]any,
	wantCode fpf.QueryReplayMismatchCode,
	wantTraceRef string,
) {
	t.Helper()

	if payload["kind"] != string(fpf.PublishedQueryResultKindReplayMismatch) {
		t.Fatalf("replay kind = %#v, want replay_mismatch", payload["kind"])
	}
	if payload["code"] != string(wantCode) {
		t.Fatalf("replay code = %#v, want %q", payload["code"], wantCode)
	}
	if payload["expected_trace_ref"] != wantTraceRef {
		t.Fatalf("expected trace ref = %#v, want %q", payload["expected_trace_ref"], wantTraceRef)
	}
}

func copyQueryIntegrationArgs(args map[string]any) map[string]any {
	copy := make(map[string]any, len(args))
	for key, value := range args {
		copy[key] = value
	}
	return copy
}

func assertQueryIntegrationHasNoHiddenWinner(t *testing.T, payload map[string]any) {
	t.Helper()

	keys := make(map[string]bool)
	collectQueryIntegrationKeys(payload, keys)
	for _, forbidden := range []string{
		"applicability",
		"applicability_score",
		"matched_route_id",
		"recommended_pattern",
		"recommended_pattern_use",
		"required_next_action",
		"route_cards",
		"score",
		"selected_pattern",
		"selected_query_view",
		"should_use_pattern",
		"suggested_haft_surface",
		"winner",
	} {
		if keys[forbidden] {
			t.Fatalf("FPF Query leaked hidden winner/router field %q", forbidden)
		}
	}
}

func collectQueryIntegrationKeys(value any, keys map[string]bool) {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			keys[key] = true
			collectQueryIntegrationKeys(child, keys)
		}
	case []any:
		for _, child := range node {
			collectQueryIntegrationKeys(child, keys)
		}
	}
}

func countQueryIntegrationArtifacts(t *testing.T, store *artifact.Store) int {
	t.Helper()

	row := store.DB().QueryRow(`SELECT COUNT(*) FROM artifacts`)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count query-integration artifacts: %v", err)
	}
	return count
}
