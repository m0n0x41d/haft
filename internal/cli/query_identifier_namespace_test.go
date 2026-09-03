package cli

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codeintel"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestNodeRejectsArtifactIdentifierBeforeCodeIndex(t *testing.T) {
	store := setupCLIArtifactStore(t)
	identifier := "dec-20260712-cb647a5c"
	assertSQLiteTableAbsent(t, store, "code_symbols")

	result, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		filepath.Join(t.TempDir(), ".haft"),
		map[string]any{
			"action": "node",
			"symbol": identifier,
		},
	)
	if result != "" {
		t.Fatalf("result = %q, want empty error result", result)
	}
	payload := assertWrongIdentifierNamespace(t, err)
	if payload.Identifier != identifier {
		t.Fatalf("identifier = %q, want %q", payload.Identifier, identifier)
	}
	if payload.RecoveryCall.Tool != "haft_query" {
		t.Fatalf("recovery tool = %q", payload.RecoveryCall.Tool)
	}
	if payload.RecoveryCall.Arguments.Action != "related" {
		t.Fatalf("recovery action = %q", payload.RecoveryCall.Arguments.Action)
	}
	if payload.RecoveryCall.Arguments.ArtifactRef != identifier {
		t.Fatalf("recovery artifact_ref = %q, want %q", payload.RecoveryCall.Arguments.ArtifactRef, identifier)
	}
	assertSQLiteTableAbsent(t, store, "code_symbols")
}

func TestEveryCodeSymbolActionRejectsArtifactIdentifierBeforeCodeIndex(t *testing.T) {
	actions := []string{
		"node",
		"callees",
		"callers",
		"impact",
		"explore",
		"code_context",
	}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			store := setupCLIArtifactStore(t)
			identifier := "dec-20260712-cb647a5c"
			assertSQLiteTableAbsent(t, store, "code_symbols")

			result, err := handleQuintQuery(
				context.Background(),
				store,
				nil,
				filepath.Join(t.TempDir(), ".haft"),
				map[string]any{
					"action": action,
					"symbol": identifier,
				},
			)
			if result != "" {
				t.Fatalf("result = %q, want empty error result", result)
			}
			payload := assertWrongIdentifierNamespace(t, err)
			if payload.Action != action {
				t.Fatalf("payload action = %q, want %q", payload.Action, action)
			}
			assertSQLiteTableAbsent(t, store, "code_symbols")
		})
	}
}

func TestNodeWrongNamespaceSurvivesV5MCPBoundary(t *testing.T) {
	store := setupCLIArtifactStore(t)
	identifier := "note-20260717-a1b2c3d4"
	handler := makeV5HandlerWithTaskMemoryProjection(
		store,
		nil,
		nil,
		filepath.Join(t.TempDir(), ".haft"),
		nil,
		nil,
		nil,
	)
	request := map[string]any{
		"name": "haft_query",
		"arguments": map[string]any{
			"action": "node",
			"symbol": identifier,
		},
	}
	rawRequest, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}

	result, err := handler(
		context.Background(),
		"haft_query",
		rawRequest,
	)
	if result != "" {
		t.Fatalf("result = %q, want empty error result", result)
	}
	payload := assertWrongIdentifierNamespace(t, err)
	if payload.Identifier != identifier {
		t.Fatalf("identifier = %q, want %q", payload.Identifier, identifier)
	}
}

func TestNodeMemoryNamespaceRecoverySurvivesV5MCPBoundary(t *testing.T) {
	store := setupCLIArtifactStore(t)
	identifier := "Opaque V5 Memory Alias"
	resolver := newQueryMemoryExactIdentifierResolver(
		strictMemoryRecoveryTestHandler(t, "exact_entity"),
	)
	handler := makeV5HandlerWithTaskMemoryProjectionAndCodeIntelAndIdentifierResolver(
		store,
		nil,
		nil,
		filepath.Join(t.TempDir(), ".haft"),
		nil,
		nil,
		nil,
		codeintel.NewService(store),
		resolver,
	)
	request, err := json.Marshal(map[string]any{
		"name": "haft_query",
		"arguments": map[string]any{
			"action": "node",
			"symbol": identifier,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := handler(context.Background(), "haft_query", request)
	if result != "" {
		t.Fatalf("result = %q, want empty error result", result)
	}
	payload := decodeWrongIdentifierNamespace(t, err)
	if payload.ReceivedNamespace != string(exactIdentifierNamespaceMemory) ||
		payload.RecoveryCall.Arguments.MemoryRequest == nil ||
		payload.RecoveryCall.Arguments.MemoryRequest.Query != identifier {
		t.Fatalf("V5 recovery = %#v", payload)
	}
}

func TestNodeNameAliasReportsTheActualWrongParameter(t *testing.T) {
	store := setupCLIArtifactStore(t)
	identifier := "note-20260717-a1b2c3d4"
	_, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		filepath.Join(t.TempDir(), ".haft"),
		map[string]any{
			"action": "node",
			"name":   identifier,
		},
	)
	payload := assertWrongIdentifierNamespace(t, err)
	if payload.Parameter != "name" {
		t.Fatalf("parameter = %q, want name", payload.Parameter)
	}
}

func TestRelatedRejectsExactFPFSourceIDWithExactInspectRecovery(t *testing.T) {
	dbPath := buildFPFSourceQueryTestDB(t)
	restoreOpen := stubSourceQueryDB(t, dbPath)
	defer restoreOpen()

	identifier := "A.7"
	result, err := handleQuintQuery(
		context.Background(),
		setupCLIArtifactStore(t),
		nil,
		filepath.Join(t.TempDir(), ".haft"),
		map[string]any{
			"action":       "related",
			"artifact_ref": identifier,
		},
	)
	if result != "" {
		t.Fatalf("result = %q, want empty error result", result)
	}
	payload := decodeWrongIdentifierNamespace(t, err)
	if payload.ReceivedNamespace != "fpf_source_identifier" ||
		payload.ExpectedNamespace != "haft_artifact_id" {
		t.Fatalf("namespace pair = %s -> %s", payload.ReceivedNamespace, payload.ExpectedNamespace)
	}
	recovery := payload.RecoveryCall.Arguments
	if recovery.Action != "fpf" || recovery.Mode != "inspect" || recovery.Identifier != identifier {
		t.Fatalf("recovery = %#v", recovery)
	}
	if recovery.ArtifactRef != "" || recovery.Symbol != "" ||
		recovery.AnchorID != "" || recovery.MemoryRequest != nil {
		t.Fatalf("recovery leaked another identifier namespace: %#v", recovery)
	}
}

func TestNodeEntityRecoveryCallPassesPublicMemoryDecoder(t *testing.T) {
	store := setupCLIArtifactStore(t)
	identifier := "entity:authorization-service"
	assertSQLiteTableAbsent(t, store, "code_symbols")

	result, err := handleQuintQueryWithIdentifierResolver(
		context.Background(),
		store,
		filepath.Join(t.TempDir(), ".haft"),
		map[string]any{
			"action": "node",
			"symbol": identifier,
		},
		exactMemoryIdentifierResolverFunc(func(
			context.Context,
			string,
		) (bool, error) {
			return true, nil
		}),
	)
	if result != "" {
		t.Fatalf("result = %q, want empty error result", result)
	}
	payload := decodeWrongIdentifierNamespace(t, err)
	if payload.ReceivedNamespace != "typed_memory_entity_id" ||
		payload.ExpectedNamespace != "code_symbol" {
		t.Fatalf("namespace pair = %s -> %s", payload.ReceivedNamespace, payload.ExpectedNamespace)
	}
	recovery := payload.RecoveryCall.Arguments
	if recovery.Action != "memory" || recovery.MemoryRequest == nil {
		t.Fatalf("recovery = %#v", recovery)
	}
	request := recovery.MemoryRequest
	if request.Mode != "resolve" ||
		request.ContractVersion != "haft.memory.v1" ||
		request.Query != identifier ||
		request.MaxCandidates != exactMemoryRecoveryMaxCandidates ||
		request.Basis.Kind != "project_current" {
		t.Fatalf("memory recovery = %#v", request)
	}
	if recovery.ArtifactRef != "" || recovery.Symbol != "" ||
		recovery.AnchorID != "" || recovery.Identifier != "" ||
		recovery.Mode != "" {
		t.Fatalf("recovery leaked another identifier namespace: %#v", recovery)
	}
	encoded, marshalErr := json.Marshal(recovery)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	decoded, decodeErr := typedmemorywire.DecodeQueryReadRequest(encoded)
	if decodeErr != nil {
		t.Fatalf("public decoder rejected emitted recovery: %v\n%s", decodeErr, encoded)
	}
	resolve, ok := decoded.(typedmemorywire.ResolveReadRequest)
	if !ok || resolve.Query() != identifier ||
		resolve.MaxCandidates() != exactMemoryRecoveryMaxCandidates {
		t.Fatalf("decoded recovery = %#v", decoded)
	}
	assertRecoveryCallMatchesPublishedQuerySchema(t, payload.RecoveryCall)
	assertStrictMemoryRecoveryRejectsLegacyAndUnknownFields(t, encoded)
	assertSQLiteTableAbsent(t, store, "code_symbols")
}

func TestRelatedRejectsExactIndexedCodeSymbolWithExactNodeRecovery(t *testing.T) {
	store, haftDir := setupCodeQueryStore(t)
	projectRoot := filepath.Dir(haftDir)
	sourcePath := filepath.Join(projectRoot, "sample.go")
	source := []byte("package sample\n\nfunc ExistingSymbol() {}\n")
	if err := os.WriteFile(sourcePath, source, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{
			"action": "node",
			"symbol": "ExistingSymbol",
		},
	)
	if err != nil {
		t.Fatalf("prepare exact code index: %v", err)
	}

	result, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{
			"action":       "related",
			"artifact_ref": "ExistingSymbol",
		},
	)
	if result != "" {
		t.Fatalf("result = %q, want empty error result", result)
	}
	payload := decodeWrongIdentifierNamespace(t, err)
	if payload.ReceivedNamespace != "code_symbol" ||
		payload.ExpectedNamespace != "haft_artifact_id" {
		t.Fatalf("namespace pair = %s -> %s", payload.ReceivedNamespace, payload.ExpectedNamespace)
	}
	recovery := payload.RecoveryCall.Arguments
	if recovery.Action != "node" || recovery.Symbol != "ExistingSymbol" {
		t.Fatalf("recovery = %#v", recovery)
	}
	if recovery.ArtifactRef != "" || recovery.Identifier != "" ||
		recovery.AnchorID != "" || recovery.MemoryRequest != nil {
		t.Fatalf("recovery leaked another identifier namespace: %#v", recovery)
	}
}

func TestNodeLeavesNonArtifactHyphenatedSymbolInCodeNamespace(t *testing.T) {
	store, haftDir := setupCodeQueryStore(t)
	result, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{
			"action": "node",
			"symbol": "cache-20260717-key",
		},
	)
	if err != nil {
		t.Fatalf("hyphenated code-symbol query returned error: %v", err)
	}
	if !strings.Contains(result, "not found") {
		t.Fatalf("hyphenated code-symbol query did not reach ordinary node lookup:\n%s", result)
	}
}

func TestNodeStillResolvesOrdinaryCodeSymbol(t *testing.T) {
	store, haftDir := setupCodeQueryStore(t)
	projectRoot := filepath.Dir(haftDir)
	sourcePath := filepath.Join(projectRoot, "sample.go")
	source := []byte("package sample\n\nfunc ExistingSymbol() {}\n")
	if err := os.WriteFile(sourcePath, source, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{
			"action": "node",
			"symbol": "ExistingSymbol",
		},
	)
	if err != nil {
		t.Fatalf("ordinary code-symbol query returned error: %v", err)
	}
	if !strings.Contains(result, "Node `ExistingSymbol`") {
		t.Fatalf("ordinary code symbol was not resolved:\n%s", result)
	}
}

func TestNodeEntityRecoveryExecutesThroughMCPBoundary(t *testing.T) {
	identifier := "authorization service alias"
	strictHandler := strictMemoryRecoveryTestHandler(t, "exact_entity")
	var dispatchedArguments []byte
	memoryHandler := fpf.MemoryToolHandler(func(
		ctx context.Context,
		arguments json.RawMessage,
	) (string, error) {
		dispatchedArguments = append([]byte(nil), arguments...)
		return strictHandler(ctx, arguments)
	})
	resolver := newQueryMemoryExactIdentifierResolver(memoryHandler)
	store := setupCLIArtifactStore(t)
	haftDir := filepath.Join(t.TempDir(), ".haft")

	_, err := handleQuintQueryWithIdentifierResolver(
		context.Background(),
		store,
		haftDir,
		map[string]any{
			"action": "node",
			"symbol": identifier,
		},
		resolver,
	)
	payload := decodeWrongIdentifierNamespace(t, err)
	encoded, marshalErr := json.Marshal(payload.RecoveryCall.Arguments)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	server := fpf.NewServer("test")
	server.SetV5Handler(func(context.Context, string, json.RawMessage) (string, error) {
		return "", errors.New("memory recovery reached v5 handler")
	})
	server.SetMemoryReadHandler(memoryHandler)
	result := dispatchRecoveryThroughToolsCall(t, server, payload.RecoveryCall)
	if result.IsError {
		t.Fatalf("execute emitted recovery through tools/call: %#v", result)
	}
	if len(result.Content) != 1 ||
		!strings.Contains(result.Content[0].Text, `"result_kind":"exact_entity"`) {
		t.Fatalf("unexpected memory recovery result: %#v", result)
	}
	if !bytes.Equal(dispatchedArguments, encoded) {
		t.Fatalf(
			"tools/call changed recovery arguments\n got: %s\nwant: %s",
			dispatchedArguments,
			encoded,
		)
	}
}

func TestNodeEntityRecoveryReplayDoesNotMutateOrEstablishUnknownIdentity(
	t *testing.T,
) {
	identifier := "entity:namespace-recovery-unknown"
	_, err := handleQuintQueryWithIdentifierResolver(
		context.Background(),
		setupCLIArtifactStore(t),
		filepath.Join(t.TempDir(), ".haft"),
		map[string]any{
			"action": "node",
			"symbol": identifier,
		},
		exactMemoryIdentifierResolverFunc(func(
			context.Context,
			string,
		) (bool, error) {
			return true, nil
		}),
	)
	recovery := decodeWrongIdentifierNamespace(t, err).RecoveryCall

	fixture := newReadOnlyProjectValidationFixture(t, "qnt_9eacbeef")
	configureBoundProjectMemoryAdmissionFixture(t, fixture)
	beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
	beforeFiles := readOnlyProjectValidationFiles(t, fixture.databaseDirectory)
	surface, err := openSealedProjectMemoryFullSurface(
		context.Background(),
		fixture.binding,
	)
	if err != nil {
		t.Fatalf("open read-only recovery surface: %v", err)
	}
	if err := surface.EnsureReady(context.Background()); err != nil {
		_ = surface.Close()
		t.Fatalf("prepare read-only recovery surface: %v", err)
	}
	server := fpf.NewServer("test")
	server.SetV5Handler(func(context.Context, string, json.RawMessage) (string, error) {
		return "", errors.New("memory recovery reached v5 handler")
	})
	server.SetMemoryReadHandler(surface.ReadOnlyQueryMCPHandler())
	result := dispatchRecoveryThroughToolsCall(t, server, recovery)
	if result.IsError || len(result.Content) != 1 {
		_ = surface.Close()
		t.Fatalf("read-only recovery tools/call result: %#v", result)
	}
	for _, forbidden := range []string{
		"invalid_contract",
		`"result_kind":"exact_entity"`,
		`"result_kind":"committed"`,
	} {
		if strings.Contains(result.Content[0].Text, forbidden) {
			_ = surface.Close()
			t.Fatalf("unknown identity replay produced %s: %s", forbidden, result.Content[0].Text)
		}
	}
	if !strings.Contains(result.Content[0].Text, `"performed":false`) {
		_ = surface.Close()
		t.Fatalf("unknown identity recovery lacks no-effect result: %s", result.Content[0].Text)
	}
	if err := surface.Close(); err != nil {
		t.Fatal(err)
	}
	afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
	afterFiles := readOnlyProjectValidationFiles(t, fixture.databaseDirectory)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatal("emitted memory recovery changed SQLite schema")
	}
	if !reflect.DeepEqual(afterFiles, beforeFiles) {
		t.Fatal("emitted memory recovery changed project-memory files")
	}
}

func TestExactMemoryNamespaceRequiresCurrentExactResolutionEvidence(t *testing.T) {
	identifier := "Opaque Memory Alias Ω"
	testCases := []struct {
		name    string
		payload string
		want    bool
	}{
		{
			name: "exact alias",
			payload: fmt.Sprintf(
				`{"result_kind":"exact_entity","result":{"resolution_witnesses":[{"kind":"exact_alias","matched":%q}]}}`,
				identifier,
			),
			want: true,
		},
		{
			name: "exact ambiguity candidates",
			payload: fmt.Sprintf(
				`{"result_kind":"entity_candidates","result":{"candidates":[{"resolution_witnesses":[{"kind":"exact_identifier","matched":%q}]}]}}`,
				identifier,
			),
			want: true,
		},
		{
			name: "alias conflict",
			payload: fmt.Sprintf(
				`{"result_kind":"resolution_unsettled","result":{"issues":[{"kind":"alias_conflict","alias":%q}]}}`,
				identifier,
			),
			want: true,
		},
		{
			name: "reviewed split",
			payload: fmt.Sprintf(
				`{"result_kind":"resolution_unsettled","result":{"issues":[{"kind":"reviewed_split_candidates","historical_entity_id":%q}]}}`,
				identifier,
			),
			want: true,
		},
		{
			name: "context unresolved after exact match",
			payload: fmt.Sprintf(
				`{"result_kind":"resolution_unsettled","result":{"issues":[{"kind":"context_not_resolved","query_context":%q}]}}`,
				identifier,
			),
			want: true,
		},
		{
			name: "lexical candidate is not exact",
			payload: fmt.Sprintf(
				`{"result_kind":"entity_candidates","result":{"candidates":[{"resolution_witnesses":[{"kind":"lexical_alias","matched":%q}]}]}}`,
				identifier,
			),
		},
		{
			name: "known absent",
			payload: fmt.Sprintf(
				`{"result_kind":"known_absent","result":{"resolution_witnesses":[{"kind":"exact_alias","matched":%q}]}}`,
				identifier,
			),
		},
		{
			name:    "normalized spelling is not exact",
			payload: `{"result_kind":"exact_entity","result":{"resolution_witnesses":[{"kind":"exact_alias","matched":"opaque memory alias ω"}]}}`,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			exact, err := exactMemoryResolutionResponse(
				identifier,
				[]byte(testCase.payload),
			)
			if err != nil {
				t.Fatal(err)
			}
			if exact != testCase.want {
				t.Fatalf("exact = %t, want %t", exact, testCase.want)
			}
		})
	}
}

func TestEveryEmittedQueryRecoveryCallPassesPublicSchemaAndStrictDecoder(
	t *testing.T,
) {
	identifier := "Exact Identifier Ω"
	calls := map[string]exactQueryRecoveryCall{
		"artifact": artifactRecoveryCall(identifier),
		"fpf":      fpfRecoveryCall(identifier),
		"symbol":   codeRecoveryCall(identifier, exactCodeIdentifierSymbol),
		"anchor":   codeRecoveryCall(identifier, exactCodeIdentifierAnchor),
		"memory":   memoryRecoveryCall(identifier),
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if call.Tool != "haft_query" {
				t.Fatalf("tool = %q", call.Tool)
			}
			assertRecoveryCallMatchesPublishedQuerySchema(t, call)
			if recovered := recoveryIdentifier(call.Arguments); recovered != identifier {
				t.Fatalf("recovered identifier = %q, want byte-identical %q", recovered, identifier)
			}
			server := fpf.NewServer("test")
			server.SetV5Handler(strictQueryRecoveryV5Handler(t, call.Arguments))
			server.SetMemoryReadHandler(strictMemoryRecoveryTestHandler(t, "known_absent"))
			result := dispatchRecoveryThroughToolsCall(t, server, call)
			if result.IsError {
				t.Fatalf("recovery failed exact tools/call dispatch: %#v", result)
			}

			if call.Arguments.Action == "memory" {
				encoded, err := json.Marshal(call.Arguments)
				if err != nil {
					t.Fatal(err)
				}
				request, err := typedmemorywire.DecodeQueryReadRequest(encoded)
				if err != nil {
					t.Fatalf("strict decoder rejected emitted recovery: %v", err)
				}
				resolve, ok := request.(typedmemorywire.ResolveReadRequest)
				if !ok || resolve.Query() != identifier {
					t.Fatalf("decoded recovery = %#v", request)
				}
			}
		})
	}
}

func TestExactIndexedCodeIdentifierUsesPublicIdentityAcrossPublishedEpochs(t *testing.T) {
	store, haftDir := setupCodeQueryStore(t)
	projectRoot := filepath.Dir(haftDir)
	if err := os.WriteFile(
		filepath.Join(projectRoot, "sample.go"),
		[]byte("package sample\n\nfunc ExistingSymbol() {}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{"action": "node", "symbol": "ExistingSymbol"},
	); err != nil {
		t.Fatalf("prepare exact code index: %v", err)
	}

	state, err := codeintel.NewService(store).CurrentIndexState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	legacyID := "sample.go#LegacyOnly#99"
	publicAnchor := "sym:v2:" + strings.Repeat("a", 64)
	// Incremental publication retains unchanged symbol rows at their write-time
	// epoch while advancing the current index basis. Public identifiers on such
	// rows remain current, but the internal storage ID is never a public route.
	carriedForwardEpoch := state.Epoch - 1
	if _, err := store.DB().Exec(
		`INSERT INTO code_symbols (
		 id, anchor_id, anchor_version, file_path, name, qualified_name,
		 kind, start_line, index_epoch
		) VALUES (?, ?, 2, ?, ?, ?, ?, ?, ?)`,
		legacyID,
		publicAnchor,
		"legacy.go",
		"LegacyOnly",
		"LegacyOnly",
		"function",
		99,
		carriedForwardEpoch,
	); err != nil {
		t.Fatal(err)
	}

	if route, exact := isExactIndexedCodeIdentifier(
		context.Background(),
		store,
		legacyID,
	); exact {
		t.Fatalf("legacy storage id was exposed as public %s", route)
	}
	if route, exact := isExactIndexedCodeIdentifier(
		context.Background(),
		store,
		publicAnchor,
	); !exact || route != exactCodeIdentifierAnchor {
		t.Fatalf("public anchor = (%s, %t), want (%s, true)", route, exact, exactCodeIdentifierAnchor)
	}
	if route, exact := isExactIndexedCodeIdentifier(
		context.Background(),
		store,
		"LegacyOnly",
	); !exact || route != exactCodeIdentifierSymbol {
		t.Fatalf("public symbol = (%s, %t), want (%s, true)", route, exact, exactCodeIdentifierSymbol)
	}
}

func TestCachedExactFPFIdentifierResolverMaterializesSourceIndexOnce(t *testing.T) {
	databasePath := buildFPFSourceQueryTestDB(t)
	openCount := 0
	resolver := newCachedExactFPFIdentifierResolver(func() (*sql.DB, func(), error) {
		openCount++
		database, err := sql.Open("sqlite", databasePath)
		if err != nil {
			return nil, nil, err
		}
		return database, func() { _ = database.Close() }, nil
	})

	for _, testCase := range []struct {
		identifier string
		want       bool
	}{
		{identifier: "A.7", want: true},
		{identifier: "a.7", want: true},
		{identifier: "unknown::opaque", want: false},
	} {
		exact, err := resolver.ResolveExactFPFIdentifier(
			context.Background(),
			testCase.identifier,
		)
		if err != nil {
			t.Fatalf("resolve %q: %v", testCase.identifier, err)
		}
		if exact != testCase.want {
			t.Fatalf("resolve %q = %t, want %t", testCase.identifier, exact, testCase.want)
		}
	}
	if openCount != 1 {
		t.Fatalf("source index open count = %d, want 1", openCount)
	}
}

func TestCachedExactFPFIdentifierResolverRetriesTransientLoadFailure(t *testing.T) {
	databasePath := buildFPFSourceQueryTestDB(t)
	openCount := 0
	resolver := newCachedExactFPFIdentifierResolver(func() (*sql.DB, func(), error) {
		openCount++
		if openCount == 1 {
			return nil, nil, errors.New("transient extraction failure")
		}
		database, err := sql.Open("sqlite", databasePath)
		if err != nil {
			return nil, nil, err
		}
		return database, func() { _ = database.Close() }, nil
	})

	if exact, err := resolver.ResolveExactFPFIdentifier(
		context.Background(),
		"A.7",
	); err == nil || exact {
		t.Fatalf("first resolution = (%t, %v), want unavailable", exact, err)
	}
	if exact, err := resolver.ResolveExactFPFIdentifier(
		context.Background(),
		"A.7",
	); err != nil || !exact {
		t.Fatalf("retried resolution = (%t, %v), want exact", exact, err)
	}
	if openCount != 2 {
		t.Fatalf("source index open count = %d, want 2", openCount)
	}
}

func TestExactIdentifierNamespaceMatrixRecoversAndExecutesUnchanged(t *testing.T) {
	dbPath := buildFPFSourceQueryTestDB(t)
	restoreOpen := stubSourceQueryDB(t, dbPath)
	defer restoreOpen()

	store, haftDir := setupCodeQueryStore(t)
	projectRoot := filepath.Dir(haftDir)
	if err := os.WriteFile(
		filepath.Join(projectRoot, "sample.go"),
		[]byte("package sample\n\nfunc ExistingSymbol() {}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{"action": "node", "symbol": "ExistingSymbol"},
	); err != nil {
		t.Fatalf("prepare exact code index: %v", err)
	}
	var existingAnchor string
	if err := store.DB().QueryRow(
		`SELECT anchor_id FROM code_symbols
		 WHERE name = ? ORDER BY id LIMIT 1`,
		"ExistingSymbol",
	).Scan(&existingAnchor); err != nil {
		t.Fatalf("load indexed code anchor: %v", err)
	}

	memoryIdentifiers := map[string]bool{
		"entity:authorization-service": true,
		"Authorization Service Alias":  true,
	}
	memoryResolver := exactMemoryIdentifierResolverFunc(func(
		_ context.Context,
		identifier string,
	) (bool, error) {
		return memoryIdentifiers[identifier], nil
	})
	memoryHandler := strictMemoryRecoveryTestHandler(t, "known_absent")

	testCases := []struct {
		name           string
		arguments      map[string]any
		wantAction     string
		wantIdentifier string
	}{
		{
			name: "artifact_in_code",
			arguments: map[string]any{
				"action": "node", "symbol": "dec-20260712-cb647a5c",
			},
			wantAction: "related", wantIdentifier: "dec-20260712-cb647a5c",
		},
		{
			name: "fpf_in_code",
			arguments: map[string]any{
				"action": "impact", "symbol": "A.7",
			},
			wantAction: "fpf", wantIdentifier: "A.7",
		},
		{
			name: "fpf_source_id_in_code",
			arguments: map[string]any{
				"action": "callees", "symbol": "A.7:solution",
			},
			wantAction: "fpf", wantIdentifier: "A.7:solution",
		},
		{
			name: "memory_alias_in_code",
			arguments: map[string]any{
				"action": "code_context", "symbol": "Authorization Service Alias",
			},
			wantAction: "memory", wantIdentifier: "Authorization Service Alias",
		},
		{
			name: "artifact_in_fpf",
			arguments: map[string]any{
				"action": "fpf", "mode": "inspect", "identifier": "note-20260717-a1b2c3d4",
			},
			wantAction: "related", wantIdentifier: "note-20260717-a1b2c3d4",
		},
		{
			name: "code_in_fpf_lookup",
			arguments: map[string]any{
				"action": "fpf", "mode": "lookup", "identifier": "ExistingSymbol",
			},
			wantAction: "node", wantIdentifier: "ExistingSymbol",
		},
		{
			name: "memory_id_in_fpf",
			arguments: map[string]any{
				"action": "fpf", "mode": "inspect", "identifier": "entity:authorization-service",
			},
			wantAction: "memory", wantIdentifier: "entity:authorization-service",
		},
		{
			name: "fpf_in_related",
			arguments: map[string]any{
				"action": "related", "artifact_ref": "A.7",
			},
			wantAction: "fpf", wantIdentifier: "A.7",
		},
		{
			name: "fpf_unit_id_in_related",
			arguments: map[string]any{
				"action": "related", "artifact_ref": "spec:toc-row:a-7",
			},
			wantAction: "fpf", wantIdentifier: "spec:toc-row:a-7",
		},
		{
			name: "code_in_related",
			arguments: map[string]any{
				"action": "related", "artifact_ref": "ExistingSymbol",
			},
			wantAction: "node", wantIdentifier: "ExistingSymbol",
		},
		{
			name: "code_anchor_in_related",
			arguments: map[string]any{
				"action": "related", "artifact_ref": existingAnchor,
			},
			wantAction: "node", wantIdentifier: existingAnchor,
		},
		{
			name: "memory_alias_in_related",
			arguments: map[string]any{
				"action": "related", "artifact_ref": "Authorization Service Alias",
			},
			wantAction: "memory", wantIdentifier: "Authorization Service Alias",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := handleQuintQueryWithIdentifierResolver(
				context.Background(),
				store,
				haftDir,
				testCase.arguments,
				memoryResolver,
			)
			payload := decodeWrongIdentifierNamespace(t, err)
			call := payload.RecoveryCall
			if call.Arguments.Action != testCase.wantAction {
				t.Fatalf("recovery action = %q, want %q", call.Arguments.Action, testCase.wantAction)
			}
			if recovered := recoveryIdentifier(call.Arguments); recovered != testCase.wantIdentifier {
				t.Fatalf("recovery identifier = %q, want byte-identical %q", recovered, testCase.wantIdentifier)
			}
			assertRecoveryCallMatchesPublishedQuerySchema(t, call)
			executeRecoveryUnchanged(
				t,
				store,
				haftDir,
				call,
				memoryHandler,
			)
		})
	}
}

func TestExactIdentifierNamespaceMatrixDoesNotGuess(t *testing.T) {
	dbPath := buildFPFSourceQueryTestDB(t)
	restoreOpen := stubSourceQueryDB(t, dbPath)
	defer restoreOpen()

	store, haftDir := setupCodeQueryStore(t)
	projectRoot := filepath.Dir(haftDir)
	if err := os.WriteFile(
		filepath.Join(projectRoot, "sample.go"),
		[]byte("package sample\n\nfunc ExistingSymbol() {}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{"action": "node", "symbol": "ExistingSymbol"},
	); err != nil {
		t.Fatal(err)
	}

	t.Run("correct_route_first", func(t *testing.T) {
		resolver := exactMemoryIdentifierResolverFunc(func(
			context.Context,
			string,
		) (bool, error) {
			return true, nil
		})
		result, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "node", "symbol": "ExistingSymbol"},
			resolver,
		)
		if err != nil || !strings.Contains(result, "Node `ExistingSymbol`") {
			t.Fatalf("correct code route did not win: result=%q err=%v", result, err)
		}
	})

	t.Run("correct_fpf_route_first", func(t *testing.T) {
		result, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "fpf", "mode": "inspect", "identifier": "A.7"},
			exactMemoryIdentifierResolverFunc(func(context.Context, string) (bool, error) {
				return true, nil
			}),
		)
		if err != nil || !strings.Contains(result, `"kind":"exact_hit"`) {
			t.Fatalf("correct FPF route did not win: result=%q err=%v", result, err)
		}
	})

	t.Run("correct_artifact_route_first", func(t *testing.T) {
		_, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "related", "artifact_ref": "dec-20260712-cb647a5c"},
			exactMemoryIdentifierResolverFunc(func(context.Context, string) (bool, error) {
				return true, nil
			}),
		)
		assertNotWrongIdentifierNamespace(t, err)
	})

	t.Run("concern_is_not_identifier", func(t *testing.T) {
		result, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "fpf", "mode": "concern", "query": "dec-20260712-cb647a5c"},
			exactMemoryIdentifierResolverFunc(func(context.Context, string) (bool, error) {
				return true, nil
			}),
		)
		if err != nil || strings.Contains(result, wrongIdentifierNamespaceCode) {
			t.Fatalf("concern query was namespace-classified: result=%q err=%v", result, err)
		}
	})

	t.Run("unknown_identifier", func(t *testing.T) {
		_, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "fpf", "mode": "inspect", "identifier": "unknown::opaque"},
			nil,
		)
		assertNotWrongIdentifierNamespace(t, err)
	})

	t.Run("entity_prefix_without_resolution_proves_nothing", func(t *testing.T) {
		_, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "code_context", "symbol": "entity:unproven"},
			nil,
		)
		assertNotWrongIdentifierNamespace(t, err)
	})

	t.Run("memory_basis_unavailable", func(t *testing.T) {
		_, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "fpf", "mode": "inspect", "identifier": "memory unavailable alias"},
			exactMemoryIdentifierResolverFunc(func(context.Context, string) (bool, error) {
				return false, errors.New("project memory unavailable")
			}),
		)
		assertNotWrongIdentifierNamespace(t, err)
	})

	t.Run("code_basis_unavailable", func(t *testing.T) {
		unindexedStore := setupCLIArtifactStore(t)
		_, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			unindexedStore,
			filepath.Join(t.TempDir(), ".haft"),
			map[string]any{"action": "related", "artifact_ref": "UnindexedSymbol"},
			nil,
		)
		assertNotWrongIdentifierNamespace(t, err)
	})

	t.Run("fpf_basis_unavailable", func(t *testing.T) {
		originalOpen := openFPFDBFunc
		openFPFDBFunc = func() (*sql.DB, func(), error) {
			return nil, nil, errors.New("FPF source index unavailable")
		}
		defer func() { openFPFDBFunc = originalOpen }()
		_, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "code_context", "symbol": "A.7"},
			nil,
		)
		assertNotWrongIdentifierNamespace(t, err)
	})

	t.Run("namespace_collision", func(t *testing.T) {
		_, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "related", "artifact_ref": "ExistingSymbol"},
			exactMemoryIdentifierResolverFunc(func(
				_ context.Context,
				identifier string,
			) (bool, error) {
				return identifier == "ExistingSymbol", nil
			}),
		)
		assertNotWrongIdentifierNamespace(t, err)
	})

	t.Run("artifact_like_whitespace_is_not_exact", func(t *testing.T) {
		_, err := handleQuintQueryWithIdentifierResolver(
			context.Background(),
			store,
			haftDir,
			map[string]any{"action": "fpf", "mode": "inspect", "identifier": " dec-20260712-cb647a5c "},
			nil,
		)
		assertNotWrongIdentifierNamespace(t, err)
	})
}

func handleQuintQueryWithIdentifierResolver(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	arguments map[string]any,
	resolver exactMemoryIdentifierResolver,
) (string, error) {
	return handleQuintQueryWithCodeIntelAndIdentifierResolver(
		ctx,
		store,
		nil,
		haftDir,
		arguments,
		codeintel.NewService(store),
		resolver,
	)
}

func strictMemoryRecoveryTestHandler(
	t *testing.T,
	resultKind string,
) fpf.MemoryToolHandler {
	t.Helper()
	return func(_ context.Context, arguments json.RawMessage) (string, error) {
		request, err := typedmemorywire.DecodeQueryReadRequest(arguments)
		if err != nil {
			return "", err
		}
		resolve, ok := request.(typedmemorywire.ResolveReadRequest)
		if !ok {
			return "", errors.New("expected resolve request")
		}
		witnesses := ""
		if resultKind == "exact_entity" {
			witnesses = fmt.Sprintf(
				`,"resolution_witnesses":[{"kind":"exact_alias","matched":%q}]`,
				resolve.Query(),
			)
		}
		return fmt.Sprintf(
			`{"contract_version":"haft.memory.v1","action":"resolve","result_kind":%q,"result":{%s}}`,
			resultKind,
			strings.TrimPrefix(witnesses, ","),
		), nil
	}
}

func executeRecoveryUnchanged(
	t *testing.T,
	store *artifact.Store,
	haftDir string,
	call exactQueryRecoveryCall,
	memoryHandler fpf.MemoryToolHandler,
) {
	t.Helper()
	server := fpf.NewServer("test")
	server.SetV5Handler(makeV5HandlerWithTaskMemoryProjectionAndCodeIntel(
		store,
		nil,
		nil,
		haftDir,
		nil,
		nil,
		nil,
		codeintel.NewService(store),
	))
	server.SetMemoryReadHandler(memoryHandler)
	result := dispatchRecoveryThroughToolsCall(t, server, call)
	if len(result.Content) != 1 {
		t.Fatalf("recovery returned unexpected tools/call content: %#v", result)
	}
	if strings.Contains(result.Content[0].Text, wrongIdentifierNamespaceCode) ||
		strings.Contains(result.Content[0].Text, "invalid_contract") {
		t.Fatalf("recovery did not execute unchanged: %#v", result)
	}
}

func strictQueryRecoveryV5Handler(
	t *testing.T,
	want exactQueryRecoveryArguments,
) fpf.V5ToolHandler {
	t.Helper()
	return func(
		_ context.Context,
		toolName string,
		params json.RawMessage,
	) (string, error) {
		if toolName != "haft_query" {
			return "", fmt.Errorf("tool = %q, want haft_query", toolName)
		}
		var envelope struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		decoder := json.NewDecoder(bytes.NewReader(params))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&envelope); err != nil {
			return "", fmt.Errorf("strictly decode tools/call params: %w", err)
		}
		if envelope.Name != "haft_query" {
			return "", fmt.Errorf("params name = %q, want haft_query", envelope.Name)
		}
		got := exactQueryRecoveryArguments{}
		decoder = json.NewDecoder(bytes.NewReader(envelope.Arguments))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&got); err != nil {
			return "", fmt.Errorf("strictly decode recovery arguments: %w", err)
		}
		gotBytes, err := json.Marshal(got)
		if err != nil {
			return "", err
		}
		wantBytes, err := json.Marshal(want)
		if err != nil {
			return "", err
		}
		if !bytes.Equal(gotBytes, wantBytes) {
			return "", fmt.Errorf("dispatched recovery changed: got %s want %s", gotBytes, wantBytes)
		}
		return `{"accepted":true}`, nil
	}
}

func dispatchRecoveryThroughToolsCall(
	t *testing.T,
	server *fpf.Server,
	call exactQueryRecoveryCall,
) fpf.CallToolResult {
	t.Helper()
	if server == nil {
		t.Fatal("server is nil")
	}
	params, err := json.Marshal(struct {
		Name      string                      `json:"name"`
		Arguments exactQueryRecoveryArguments `json:"arguments"`
	}{
		Name:      call.Tool,
		Arguments: call.Arguments,
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(fpf.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      "query-recovery",
		Params:  params,
	})
	if err != nil {
		t.Fatal(err)
	}

	originalStdin := os.Stdin
	originalStdout := os.Stdout
	inputReader, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outputReader, outputWriter, err := os.Pipe()
	if err != nil {
		_ = inputReader.Close()
		_ = inputWriter.Close()
		t.Fatal(err)
	}
	defer func() {
		os.Stdin = originalStdin
		os.Stdout = originalStdout
		_ = inputReader.Close()
		_ = inputWriter.Close()
		_ = outputReader.Close()
		_ = outputWriter.Close()
	}()
	os.Stdin = inputReader
	os.Stdout = outputWriter
	serverDone := make(chan struct{})
	go func() {
		server.Start()
		close(serverDone)
	}()
	if _, err := inputWriter.Write(append(request, '\n')); err != nil {
		t.Fatal(err)
	}
	type readResult struct {
		payload []byte
		err     error
	}
	responseRead := make(chan readResult, 1)
	go func() {
		payload, err := bufio.NewReader(outputReader).ReadBytes('\n')
		responseRead <- readResult{payload: payload, err: err}
	}()
	var responseBytes []byte
	select {
	case read := <-responseRead:
		if read.err != nil {
			t.Fatalf("read exact tools/call response: %v", read.err)
		}
		responseBytes = read.payload
	case <-time.After(10 * time.Second):
		_ = inputWriter.Close()
		t.Fatal("exact tools/call response timed out")
	}
	if err := inputWriter.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-serverDone:
	case <-time.After(10 * time.Second):
		t.Fatal("MCP server did not stop after request input closed")
	}
	if err := outputWriter.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = originalStdin
	os.Stdout = originalStdout
	response := struct {
		Result fpf.CallToolResult `json:"result"`
	}{}
	if err := json.Unmarshal(bytes.TrimSpace(responseBytes), &response); err != nil {
		t.Fatalf("decode exact tools/call response: %v\n%s", err, responseBytes)
	}
	return response.Result
}

func recoveryIdentifier(arguments exactQueryRecoveryArguments) string {
	switch arguments.Action {
	case "related":
		return arguments.ArtifactRef
	case "fpf":
		return arguments.Identifier
	case "node":
		if arguments.AnchorID != "" {
			return arguments.AnchorID
		}
		return arguments.Symbol
	case "memory":
		if arguments.MemoryRequest != nil {
			return arguments.MemoryRequest.Query
		}
	}
	return ""
}

func assertRecoveryCallMatchesPublishedQuerySchema(
	t *testing.T,
	call exactQueryRecoveryCall,
) {
	t.Helper()
	server := fpf.NewServer("test")
	server.SetV5Handler(func(context.Context, string, json.RawMessage) (string, error) {
		return "", nil
	})
	var schema map[string]any
	for _, tool := range server.ToolCatalog() {
		if tool.Name != "haft_query" {
			continue
		}
		encoded, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &schema); err != nil {
			t.Fatal(err)
		}
		break
	}
	if schema == nil {
		t.Fatal("published haft_query schema is missing")
	}
	encoded, err := json.Marshal(call.Arguments)
	if err != nil {
		t.Fatal(err)
	}
	arguments := map[string]any{}
	if err := json.Unmarshal(encoded, &arguments); err != nil {
		t.Fatal(err)
	}
	properties, _ := schema["properties"].(map[string]any)
	for key := range arguments {
		if _, declared := properties[key]; !declared {
			t.Fatalf("recovery field %q is absent from published haft_query schema", key)
		}
	}
	actionSchema, _ := properties["action"].(map[string]any)
	if !schemaEnumContains(actionSchema["enum"], call.Arguments.Action) {
		t.Fatalf("published action enum omits %q", call.Arguments.Action)
	}
	if call.Arguments.Action != "memory" {
		return
	}
	memorySchema, _ := properties["memory_request"].(map[string]any)
	variants, _ := memorySchema["oneOf"].([]any)
	request := arguments["memory_request"].(map[string]any)
	for _, rawVariant := range variants {
		variant, _ := rawVariant.(map[string]any)
		variantProperties, _ := variant["properties"].(map[string]any)
		mode, _ := variantProperties["mode"].(map[string]any)
		if mode["const"] != "resolve" {
			continue
		}
		if variant["additionalProperties"] != false {
			t.Fatal("published resolve schema is not closed")
		}
		allowed := map[string]bool{}
		for key := range variantProperties {
			allowed[key] = true
		}
		for key := range request {
			if !allowed[key] {
				t.Fatalf("memory recovery field %q is absent from published resolve schema", key)
			}
		}
		for _, required := range schemaStringSlice(variant["required"]) {
			if _, present := request[required]; !present {
				t.Fatalf("memory recovery omits published required field %q", required)
			}
		}
		return
	}
	t.Fatal("published memory resolve schema is missing")
}

func schemaEnumContains(raw any, want string) bool {
	for _, value := range schemaStringSlice(raw) {
		if value == want {
			return true
		}
	}
	return false
}

func schemaStringSlice(raw any) []string {
	values, _ := raw.([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func assertStrictMemoryRecoveryRejectsLegacyAndUnknownFields(
	t *testing.T,
	valid []byte,
) {
	t.Helper()
	invalid := [][]byte{
		[]byte(`{"action":"memory","mode":"resolve","contract_version":"haft.memory.v1","basis":{"kind":"project_current"},"query":"x","max_candidates":8}`),
		bytes.Replace(valid, []byte(`"query":`), []byte(`"unexpected":true,"query":`), 1),
		bytes.Replace(valid, []byte(`"memory_request":`), []byte(`"memory_request":{},"memory_request":`), 1),
	}
	for _, payload := range invalid {
		if _, err := typedmemorywire.DecodeQueryReadRequest(payload); err == nil {
			t.Fatalf("strict decoder accepted invalid recovery shape: %s", payload)
		}
	}
}

func assertNotWrongIdentifierNamespace(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	var typed wrongIdentifierNamespaceError
	if errors.As(err, &typed) || strings.Contains(err.Error(), wrongIdentifierNamespaceCode) {
		t.Fatalf("identifier received a guessed namespace recovery: %v", err)
	}
}

func assertWrongIdentifierNamespace(
	t *testing.T,
	err error,
) wrongIdentifierNamespace {
	t.Helper()
	payload := decodeWrongIdentifierNamespace(t, err)
	if payload.ReceivedNamespace != "haft_artifact_id" {
		t.Fatalf("received_namespace = %q", payload.ReceivedNamespace)
	}
	if payload.ExpectedNamespace != "code_symbol" {
		t.Fatalf("expected_namespace = %q", payload.ExpectedNamespace)
	}
	return payload
}

func decodeWrongIdentifierNamespace(
	t *testing.T,
	err error,
) wrongIdentifierNamespace {
	t.Helper()
	if err == nil {
		t.Fatal("error is nil, want wrong_identifier_namespace")
	}

	payload := wrongIdentifierNamespace{}
	if unmarshalErr := json.Unmarshal([]byte(err.Error()), &payload); unmarshalErr != nil {
		t.Fatalf("error is not structured JSON: %v\n%v", unmarshalErr, err)
	}
	if payload.Code != wrongIdentifierNamespaceCode {
		t.Fatalf("code = %q", payload.Code)
	}
	if payload.Tool != "haft_query" {
		t.Fatalf("tool = %q", payload.Tool)
	}
	if payload.Action == "" {
		t.Fatal("action is empty")
	}
	if payload.Parameter == "" {
		t.Fatal("parameter is empty")
	}
	if payload.SameCallRetryable {
		t.Fatal("same_call_retryable = true, want false")
	}
	return payload
}

func assertSQLiteTableAbsent(
	t *testing.T,
	store *artifact.Store,
	tableName string,
) {
	t.Helper()
	count := 0
	err := store.DB().QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		tableName,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("table %q exists; code index was touched", tableName)
	}
}

func setupCodeQueryStore(t *testing.T) (*artifact.Store, string) {
	t.Helper()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	if err := os.MkdirAll(haftDir, 0o700); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(haftDir, "haft.db")
	database, err := openCurrentKernelTestStore(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return artifact.NewStore(database.GetRawDB()), haftDir
}
