package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintQueryExploreConcernReturnsEvidenceBearingCandidates(t *testing.T) {
	store := setupCLIArtifactStore(t)
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package codebase
type Scanner struct{}
func (s *Scanner) publishIndexEpoch() {}
`
	if err := os.WriteFile(
		filepath.Join(root, "incremental.go"),
		[]byte(source),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{
			"action":         "explore",
			"query":          "Where is the index epoch published atomically?",
			"max_candidates": float64(5),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{
		`"contract_version":"haft.code_explore.v1"`,
		`"view":"working"`,
		`"kind":"candidate_set"`,
		`"query":"Where is the index epoch published atomically?"`,
		`"max_candidates":5`,
		`"name":"publishIndexEpoch"`,
		`"symbol_kind":"method"`,
		`"anchor_id":"sym:v2:`,
		`"ranking_is_advisory":true`,
		`"identity_auto_selected":false`,
	} {
		if !strings.Contains(result, wanted) {
			t.Fatalf("concern response missing %q:\n%s", wanted, result)
		}
	}
	for _, forbidden := range []string{
		`"combined":`,
		`"ppr":`,
		`"selected":`,
		`"winner":`,
		`"source_lane":`,
		`"direct_bridge":`,
		`"origin_lanes":`,
		`"reasoning_artifacts":`,
		`"provenance":`,
	} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("working concern response leaked %q:\n%s", forbidden, result)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("working concern response is not canonical JSON: %v", err)
	}
	traceRef, ok := payload["trace_ref"].(string)
	if !ok || traceRef == "" {
		t.Fatalf("working concern response lacks trace_ref: %s", result)
	}
	trace, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{
			"action":         "explore",
			"query":          "Where is the index epoch published atomically?",
			"max_candidates": float64(5),
			"view":           "trace",
			"trace_ref":      traceRef,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(trace, `"source_lane":"production"`) {
		t.Fatalf("trace concern response lacks retrieval provenance: %s", trace)
	}

	direct, err := publishCodeExplore(
		context.Background(),
		store,
		root,
		"",
		"",
		0,
		"Where is the index epoch published atomically?",
		5,
		"working",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != string(direct) {
		t.Fatalf("MCP and CLI encoder path differ:\nMCP=%s\nCLI=%s", result, direct)
	}
}

func TestHandleQuintQueryExploreRejectsFractionalConcernBudget(t *testing.T) {
	store := setupCLIArtifactStore(t)
	_, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		filepath.Join(t.TempDir(), ".haft"),
		map[string]any{
			"action":         "explore",
			"query":          "find the index publisher",
			"max_candidates": 1.5,
		},
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"non-negative integer",
	) {
		t.Fatalf("fractional concern budget error = %v", err)
	}
}

func TestHandleQuintQueryExploreModuleDecisionRefsMatchCLIProjection(
	t *testing.T,
) {
	store := setupCLIArtifactStore(t)
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	sourceDir := filepath.Join(root, "internal", "sample")
	for _, directory := range []string{haftDir, sourceDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.test/module-context\n\ngo 1.25\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "sample.go"),
		[]byte("package sample\nfunc NeighborhoodRead() {}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	fields := artifact.DecisionFields{
		BindingTargets: []artifact.BindingTarget{{
			Kind:       artifact.BindingTargetModule,
			ModulePath: "internal/sample",
		}},
	}
	structured, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	decision := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        "dec-module-context",
			Kind:      artifact.KindDecisionRecord,
			Status:    artifact.StatusActive,
			Title:     "Sample module context",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body:           "fixture",
		StructuredData: string(structured),
	}
	if err := store.Create(context.Background(), decision); err != nil {
		t.Fatal(err)
	}

	mcpResult, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{
			"action": "explore",
			"symbol": "NeighborhoodRead",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{
		`"module_decision_refs":["dec-module-context"]`,
		`"exact_binding_decision_refs":[]`,
		`"affected_path_context_decision_refs":[]`,
	} {
		if !strings.Contains(mcpResult, wanted) {
			t.Fatalf(
				"MCP Explore response missing %q:\n%s",
				wanted,
				mcpResult,
			)
		}
	}
	if strings.Contains(
		mcpResult,
		"legacy file+name context (not an exact binding)",
	) {
		t.Fatalf(
			"empty legacy backlink lane claimed match granularity:\n%s",
			mcpResult,
		)
	}
	cliResult, err := publishCodeExplore(
		context.Background(),
		store,
		root,
		"NeighborhoodRead",
		"",
		0,
		"",
		5,
		"working",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if mcpResult != string(cliResult) {
		t.Fatalf(
			"MCP and CLI module-context projections differ:\nMCP=%s\nCLI=%s",
			mcpResult,
			cliResult,
		)
	}
}

func TestHandleQuintQueryExploreRejectsMixedConcernAndSymbol(t *testing.T) {
	store := setupCLIArtifactStore(t)
	_, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		filepath.Join(t.TempDir(), ".haft"),
		map[string]any{
			"action": "explore",
			"query":  "find the index publisher",
			"symbol": "publishIndexEpoch",
		},
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"exactly one of symbol or query",
	) {
		t.Fatalf("mixed explore error = %v", err)
	}
}

func TestHandleQuintQueryExploreExactKeepsSourceAndPathSemantics(t *testing.T) {
	store := setupCLIArtifactStore(t)
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package sample\nfunc ExactExploreSeed() {}\n"
	if err := os.WriteFile(
		filepath.Join(root, "sample.go"),
		[]byte(source),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(
		context.Background(),
		store,
		nil,
		haftDir,
		map[string]any{
			"action": "explore",
			"symbol": "ExactExploreSeed",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{
		`"kind":"resolved"`,
		`"seed_resolution":{"kind":"resolved_seed"`,
		`"source_hops":[`,
		`"name":"ExactExploreSeed"`,
		`"source":{"available":true`,
		`func ExactExploreSeed() {}`,
	} {
		if !strings.Contains(result, wanted) {
			t.Fatalf("exact response missing %q:\n%s", wanted, result)
		}
	}
}
