package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestNodeRejectsArtifactIdentifierBeforeCodeIndex(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func TestNodeNameAliasReportsTheActualWrongParameter(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	if recovery.ArtifactRef != "" || recovery.Symbol != "" || recovery.Query != "" {
		t.Fatalf("recovery leaked another identifier namespace: %#v", recovery)
	}
}

func TestNodeRejectsEntityIDWithExactMemoryResolveRecovery(t *testing.T) {
	t.Parallel()

	store := setupCLIArtifactStore(t)
	identifier := "entity:authorization-service"
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
	payload := decodeWrongIdentifierNamespace(t, err)
	if payload.ReceivedNamespace != "typed_memory_entity_id" ||
		payload.ExpectedNamespace != "code_symbol" {
		t.Fatalf("namespace pair = %s -> %s", payload.ReceivedNamespace, payload.ExpectedNamespace)
	}
	recovery := payload.RecoveryCall.Arguments
	if recovery.Action != "memory" || recovery.Mode != "resolve" ||
		recovery.ContractVersion != "haft.memory.v1" || recovery.Query != identifier {
		t.Fatalf("recovery = %#v", recovery)
	}
	if recovery.Basis == nil || recovery.Basis.Kind != "project_current" {
		t.Fatalf("recovery basis = %#v", recovery.Basis)
	}
	if recovery.ArtifactRef != "" || recovery.Symbol != "" || recovery.Identifier != "" {
		t.Fatalf("recovery leaked another identifier namespace: %#v", recovery)
	}
	assertSQLiteTableAbsent(t, store, "code_symbols")
}

func TestRelatedRejectsExactIndexedCodeSymbolWithExactNodeRecovery(t *testing.T) {
	t.Parallel()

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
	if recovery.ArtifactRef != "" || recovery.Identifier != "" || recovery.Query != "" {
		t.Fatalf("recovery leaked another identifier namespace: %#v", recovery)
	}
}

func TestNodeLeavesNonArtifactHyphenatedSymbolInCodeNamespace(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
