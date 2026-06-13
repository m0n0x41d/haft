package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintQuery_CodeContextDefaultsToIndex(t *testing.T) {
	fixture := setupCodeContextLaneFixture(t)

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(code_context) returned error: %v", err)
	}

	for _, want := range []string{"## Code context index", "Lane counts", "decisions: 1", `lane="symbols"`, `lane="decisions"`, "full=true"} {
		if !strings.Contains(result, want) {
			t.Fatalf("default code_context missing %q:\n%s", want, result)
		}
	}
	for _, notWant := range []string{"### Decisions governing this code", "### Notes", "binding context invariant"} {
		if strings.Contains(result, notWant) {
			t.Fatalf("default code_context leaked full lane content %q:\n%s", notWant, result)
		}
	}
}

func TestHandleQuintQuery_CodeContextTypedLanes(t *testing.T) {
	fixture := setupCodeContextLaneFixture(t)

	decisions, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
		"lane":   "decisions",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(code_context decisions) returned error: %v", err)
	}
	if !strings.Contains(decisions, "Code context lane decision") {
		t.Fatalf("decisions lane missing decision:\n%s", decisions)
	}
	for _, notWant := range []string{"Code context lane note", "binding context invariant"} {
		if strings.Contains(decisions, notWant) {
			t.Fatalf("decisions lane leaked %q:\n%s", notWant, decisions)
		}
	}

	invariants, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
		"lane":   "invariants",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(code_context invariants) returned error: %v", err)
	}
	if !strings.Contains(invariants, "binding context invariant") {
		t.Fatalf("invariants lane missing invariant:\n%s", invariants)
	}
	if strings.Contains(invariants, "Code context lane note") {
		t.Fatalf("invariants lane leaked note:\n%s", invariants)
	}

	notes, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
		"lane":   "notes",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(code_context notes) returned error: %v", err)
	}
	if !strings.Contains(notes, "Code context lane note") {
		t.Fatalf("notes lane missing note:\n%s", notes)
	}
	if strings.Contains(notes, "### Decisions governing this code") || strings.Contains(notes, "binding context invariant") {
		t.Fatalf("notes lane leaked other lanes:\n%s", notes)
	}
}

func TestHandleQuintQuery_CodeContextSymbolsLaneCapsByLimit(t *testing.T) {
	fixture := setupCodeContextLaneFixture(t)

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
		"lane":   "symbols",
		"limit":  float64(2),
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(code_context symbols) returned error: %v", err)
	}

	for _, want := range []string{"## Code context symbols", "Alpha", "Beta", "more omitted"} {
		if !strings.Contains(result, want) {
			t.Fatalf("symbols lane missing %q:\n%s", want, result)
		}
	}
	if strings.Contains(result, "Gamma") {
		t.Fatalf("symbols lane ignored limit:\n%s", result)
	}
}

func TestHandleQuintQuery_CodeContextRejectsUnknownLane(t *testing.T) {
	fixture := setupCodeContextLaneFixture(t)

	_, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
		"lane":   "everything",
	})
	if err == nil {
		t.Fatal("expected unknown lane error")
	}
	for _, want := range []string{"unknown code_context lane", "index", "symbols", "decisions", "all"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("unknown lane error missing %q: %v", want, err)
		}
	}
}

func TestCodeContextNormalTrace_StaysUnderBudget(t *testing.T) {
	fixture := setupCodeContextLaneFixture(t)
	ctx := context.Background()

	interfaceResult := bytes.Buffer{}
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.code_context")
	if !ok {
		t.Fatal("query.code_context capability missing")
	}
	if err := writeJSON(&interfaceResult, capability); err != nil {
		t.Fatalf("write interface JSON: %v", err)
	}

	trace := []struct {
		name string
		text string
	}{
		{name: "interface query.code_context", text: interfaceResult.String()},
	}

	status, err := handleQuintQuery(ctx, fixture.store, nil, fixture.haftDir, map[string]any{"action": "status"})
	if err != nil {
		t.Fatalf("status trace failed: %v", err)
	}
	trace = append(trace, struct {
		name string
		text string
	}{name: "status full=false", text: status})

	refresh, err := handleQuintRefresh(ctx, fixture.store, fixture.haftDir, map[string]any{"action": "scan", "verbose": false})
	if err != nil {
		t.Fatalf("refresh trace failed: %v", err)
	}
	trace = append(trace, struct {
		name string
		text string
	}{name: "refresh.scan verbose=false", text: refresh})

	for _, args := range []struct {
		name string
		args map[string]any
	}{
		{name: "code_context lane=index", args: map[string]any{"action": "code_context", "file": fixture.file}},
		{name: "code_context lane=symbols", args: map[string]any{"action": "code_context", "file": fixture.file, "lane": "symbols", "limit": float64(20)}},
		{name: "code_context lane=decisions", args: map[string]any{"action": "code_context", "file": fixture.file, "lane": "decisions"}},
	} {
		result, err := handleQuintQuery(ctx, fixture.store, nil, fixture.haftDir, args.args)
		if err != nil {
			t.Fatalf("%s trace failed: %v", args.name, err)
		}
		trace = append(trace, struct {
			name string
			text string
		}{name: args.name, text: result})
	}

	total := 0
	for _, item := range trace {
		size := len([]byte(item.text))
		total += size
		if size > 5000 {
			t.Fatalf("%s response = %d bytes, want <= 5000\n%s", item.name, size, item.text)
		}
	}
	if total > 12000 {
		t.Fatalf("normal trace total = %d bytes, want <= 12000", total)
	}

	index, err := handleQuintQuery(ctx, fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
	})
	if err != nil {
		t.Fatalf("index recovery baseline failed: %v", err)
	}
	full, err := handleQuintQuery(ctx, fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
		"full":   true,
	})
	if err != nil {
		t.Fatalf("full recovery failed: %v", err)
	}
	for _, want := range []string{"### Decisions governing this code", "### Notes", "binding context invariant"} {
		if !strings.Contains(full, want) {
			t.Fatalf("full=true should preserve audit detail %q:\nindex=%d full=%d\n%s", want, len(index), len(full), full)
		}
	}

	refreshVerbose, err := handleQuintRefresh(ctx, fixture.store, fixture.haftDir, map[string]any{"action": "scan", "verbose": true})
	if err != nil {
		t.Fatalf("refresh verbose recovery failed: %v", err)
	}
	if len(refreshVerbose) < len(refresh) {
		t.Fatalf("verbose refresh should not be smaller than compact refresh: compact=%d verbose=%d", len(refresh), len(refreshVerbose))
	}
}

type codeContextLaneFixture struct {
	store   *artifact.Store
	haftDir string
	file    string
}

func setupCodeContextLaneFixture(t *testing.T) codeContextLaneFixture {
	t.Helper()

	ctx := context.Background()
	root := t.TempDir()
	file := "internal/codecontext_lane.go"
	absFile := filepath.Join(root, file)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absFile, []byte(`package internal

func Alpha() {}

func Beta() {}

func Gamma() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	store := setupCLIArtifactStore(t)
	mustExecCodeContextLaneFixture(t, store, `CREATE TABLE IF NOT EXISTS affected_symbols (
		artifact_id TEXT NOT NULL, file_path TEXT NOT NULL, symbol_name TEXT NOT NULL,
		symbol_kind TEXT, symbol_line INTEGER, symbol_end_line INTEGER, symbol_hash TEXT,
		PRIMARY KEY (artifact_id, file_path, symbol_name))`)
	mustExecCodeContextLaneFixture(t, store, `CREATE TABLE IF NOT EXISTS audit_log (
		id TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		tool_name TEXT NOT NULL DEFAULT '',
		operation TEXT NOT NULL,
		actor TEXT NOT NULL DEFAULT '',
		target_id TEXT,
		input_hash TEXT,
		result TEXT NOT NULL DEFAULT '',
		details TEXT,
		context_id TEXT NOT NULL DEFAULT 'default')`)

	now := time.Now().UTC()
	decision := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        "dec-code-context-lane",
			Kind:      artifact.KindDecisionRecord,
			Status:    artifact.StatusActive,
			Title:     "Code context lane decision",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body:           "decision body",
		StructuredData: `{"invariants":["binding context invariant"]}`,
	}
	note := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        "note-code-context-lane",
			Kind:      artifact.KindNote,
			Status:    artifact.StatusActive,
			Title:     "Code context lane note",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body: "note body",
	}
	for _, item := range []*artifact.Artifact{decision, note} {
		if err := store.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
		mustExecCodeContextLaneFixture(t, store, `INSERT INTO affected_files (artifact_id, file_path) VALUES (?, ?)`, item.Meta.ID, file)
	}

	return codeContextLaneFixture{
		store:   store,
		haftDir: filepath.Join(root, ".haft"),
		file:    file,
	}
}

func mustExecCodeContextLaneFixture(t *testing.T, store *artifact.Store, query string, args ...any) {
	t.Helper()
	if _, err := store.DB().ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("fixture SQL failed: %v\n%s", err, query)
	}
}
