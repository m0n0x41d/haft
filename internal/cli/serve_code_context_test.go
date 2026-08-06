package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/present"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestHandleQuintQuery_CodeContextDefaultsToIndex(t *testing.T) {
	t.Parallel()

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
	for _, notWant := range []string{"### `exact_binding`", "### Notes", "binding context invariant"} {
		if strings.Contains(result, notWant) {
			t.Fatalf("default code_context leaked full lane content %q:\n%s", notWant, result)
		}
	}
	assertNoContractGenerationManifestInline(t, "default code_context", result)
}

func TestHandleQuintQuery_CodeContextBatchesFilesWithBoundedTypedLane(t *testing.T) {
	t.Parallel()

	fixture := setupCodeContextLaneFixture(t)
	second := "internal/second.go"
	root := filepath.Dir(fixture.haftDir)
	if err := os.WriteFile(filepath.Join(root, second), []byte("package internal\nfunc Second() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"files":  []any{fixture.file, second, fixture.file},
		"lane":   "decisions",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Code context batch 1/2",
		"Code context batch 2/2",
		fixture.file,
		second,
		"Code context lane decision",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("batch code_context missing %q:\n%s", want, result)
		}
	}
	if strings.Count(result, "── Haft") != 1 {
		t.Fatalf("batch must append one navigation strip, got:\n%s", result)
	}
}

func TestHandleQuintQuery_CodeContextBatchRejectsUnboundedOrAmbiguousInputs(t *testing.T) {
	t.Parallel()

	fixture := setupCodeContextLaneFixture(t)
	files := make([]any, codeContextBatchLimit+1)
	for index := range files {
		files[index] = fmt.Sprintf("src/file-%d.go", index)
	}
	_, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"files":  files,
	})
	if err == nil || !strings.Contains(err.Error(), "at most 8") {
		t.Fatalf("unbounded batch error = %v", err)
	}

	_, err = handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"files":  []any{fixture.file, "internal/second.go"},
		"symbol": "Alpha",
	})
	if err == nil || !strings.Contains(err.Error(), "single-target fields") {
		t.Fatalf("ambiguous batch error = %v", err)
	}
}

func TestHandleQuintQuery_CodeContextFooterUsesTypedStaleSnapshot(t *testing.T) {
	t.Parallel()

	fixture := setupCodeContextLaneFixture(t)
	checkFixture := checkTestProject{
		root:    filepath.Dir(fixture.haftDir),
		haftDir: fixture.haftDir,
		store:   fixture.store,
	}
	decision := mustCreateDecision(t, checkFixture, artifact.DecideInput{
		SelectedTitle:   "Deprecated expired decision",
		WhySelected:     "Need a terminal decision that raw nav would count as stale.",
		SelectionPolicy: "Prefer a single terminal decision with no active operator work.",
		CounterArgument: "Terminal decisions should not surface as current stale debt.",
		WeakestLink:     "Read-only query footers must follow the same stale lane as status.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Active expired decision",
			Reason:  "Would be legitimate stale debt and would not prove footer filtering.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Deprecated decisions resurface in code_context footers."},
		},
	})
	mustSetValidUntil(t, checkFixture, decision.Meta.ID, time.Now().Add(-72*time.Hour).Format("2006-01-02"))
	mustSetArtifactStatus(t, checkFixture, decision.Meta.ID, artifact.StatusDeprecated)

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(code_context) returned error: %v", err)
	}
	if strings.Contains(result, "Evidence pressure: 1 decision(s) need refresh") {
		t.Fatalf("code_context footer used raw stale decision count instead of typed snapshot:\n%s", result)
	}
}

func TestNavStripWithoutStaleSnapshotSuppressesRawStaleDebt(t *testing.T) {
	t.Parallel()

	fixture := setupCodeContextLaneFixture(t)
	checkFixture := checkTestProject{
		root:    filepath.Dir(fixture.haftDir),
		haftDir: fixture.haftDir,
		store:   fixture.store,
	}
	decision := mustCreateDecision(t, checkFixture, artifact.DecideInput{
		SelectedTitle:   "Expired active decision",
		WhySelected:     "Need raw nav stale debt for fallback regression coverage.",
		SelectionPolicy: "Exercise fallback without a typed stale snapshot.",
		CounterArgument: "Typed status data should be preferred whenever available.",
		WeakestLink:     "Fallback must not reintroduce raw stale count noise.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Deprecated decision",
			Reason:  "Terminal stale filtering is covered separately.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Raw stale debt appears in fallback query footers."},
		},
	})
	mustSetValidUntil(t, checkFixture, decision.Meta.ID, time.Now().Add(-72*time.Hour).Format("2006-01-02"))

	raw := present.NavStrip(artifact.ComputeNavState(context.Background(), fixture.store, ""))
	if !strings.Contains(raw, "Evidence pressure:") {
		t.Fatalf("test setup did not produce raw stale nav debt:\n%s", raw)
	}

	result := navStripWithoutStaleSnapshot(context.Background(), fixture.store, "")
	if strings.Contains(result, "Evidence pressure:") {
		t.Fatalf("fallback nav leaked raw stale debt:\n%s", result)
	}
}

func assertNoContractGenerationManifestInline(t *testing.T, surface string, text string) {
	t.Helper()

	for _, forbidden := range []string{
		"haft_interface_contract_generation_manifest",
		"read_only_generation_manifest_not_host_materialization",
		"source_digest",
		"generator_target_surfaces",
		"generator_target_fields",
		"generated_preview_fragments",
		"generated_schema_fragments",
		"runtime_schema_audit",
		"runtime_schema_drift",
		"generated_fragments",
		"generated/contract-generation/preview/",
		"generated/contract-generation/schema/",
		"surface_policy",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%s inlined contract generation manifest fragment %q:\n%s", surface, forbidden, text)
		}
	}
	assertNoContractAuditInline(t, surface, text)
	assertNoInterfaceOutputShapeInline(t, surface, text)
}

func assertNoContractAuditInline(t *testing.T, surface string, text string) {
	t.Helper()

	for _, forbidden := range []string{
		"haft_interface_contract_audit",
		"read_only_contract_inventory_not_schema_generation",
		"schema_required_covered_surfaces",
		"schema_required_missing_surfaces",
		"schema_missing_required_fields",
		"required_coverage=",
		"mcp_required_fields",
		"missing_required_fields",
		"action_required_fields",
		"required_posture",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%s inlined contract audit fragment %q:\n%s", surface, forbidden, text)
		}
	}
}

func assertNoInterfaceOutputShapeInline(t *testing.T, surface string, text string) {
	t.Helper()

	for _, forbidden := range []string{
		"bounded_reliance|advisory_only|blocked",
		"legacy_formality_projection_lossy|unversioned_formality_source_scale_missing|current_f0_f9_formality",
		"not_claim_truth",
		"not_publication",
		"planned_edition",
		"markdown_sync_back",
		"semantic_field_update",
		"relationship_update",
		"sql_edition_update_not_approval_rebaseline_evidence_gate_claim_truth_global_truth_or_prose_authority",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%s inlined interface output-shape fragment %q:\n%s", surface, forbidden, text)
		}
	}
}

func TestHandleQuintQuery_CodeContextTypedLanes(t *testing.T) {
	t.Parallel()

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
	if strings.Contains(notes, "### `exact_binding`") || strings.Contains(notes, "binding context invariant") {
		t.Fatalf("notes lane leaked other lanes:\n%s", notes)
	}
}

func TestHandleQuintQuery_CodeContextShowsSpecSectionLinkedDecision(t *testing.T) {
	t.Parallel()

	fixture := setupCodeContextLaneFixture(t)
	mustExecCodeContextLaneFixture(t, fixture.store, `CREATE TABLE IF NOT EXISTS spec_section_editions (
		project_id TEXT NOT NULL,
		section_id TEXT NOT NULL,
		semantic_hash TEXT NOT NULL,
		section_json TEXT NOT NULL,
		source_kind TEXT NOT NULL DEFAULT '',
		carrier_path TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (project_id, section_id)
	)`)
	mustExecCodeContextLaneFixture(t, fixture.store, `CREATE TABLE IF NOT EXISTS spec_section_baselines (
		project_id TEXT NOT NULL,
		section_id TEXT NOT NULL,
		hash TEXT NOT NULL,
		captured_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		approved_by TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, section_id)
	)`)
	section := project.SpecSection{
		ID:            "TS.environment.001",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "Harnessable repository environment",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "human",
		Status:        "active",
		ValidUntil:    "2026-12-31",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	sectionJSON, err := json.Marshal(section)
	if err != nil {
		t.Fatal(err)
	}
	sectionHash := specflow.HashSection(section)
	mustExecCodeContextLaneFixture(
		t,
		fixture.store,
		`INSERT INTO spec_section_editions
		 (project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"project-cli",
		section.ID,
		sectionHash,
		string(sectionJSON),
		"carrier_import",
		section.Path,
		time.Now().UTC(),
	)
	mustExecCodeContextLaneFixture(
		t,
		fixture.store,
		`INSERT INTO spec_section_baselines (project_id, section_id, hash, captured_at, approved_by)
		 VALUES (?, ?, ?, ?, ?)`,
		"project-cli",
		section.ID,
		sectionHash,
		time.Now().UTC(),
		"operator",
	)
	decision := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        "dec-code-context-section-ref",
			Kind:      artifact.KindDecisionRecord,
			Status:    artifact.StatusActive,
			Title:     "Spec section linked code decision",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		Body:           "decision body",
		StructuredData: `{"section_refs":["TS.environment.001"]}`,
	}
	addExactFileBindingToCodeContextFixture(t, decision, fixture.file)
	if err := fixture.store.Create(context.Background(), decision); err != nil {
		t.Fatal(err)
	}
	mustExecCodeContextLaneFixture(t, fixture.store, `INSERT INTO affected_files (artifact_id, file_path) VALUES (?, ?)`, decision.Meta.ID, fixture.file)

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
		"lane":   "decisions",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(code_context decisions) returned error: %v", err)
	}

	for _, want := range []string{
		"### `exact_binding` — authority-bearing target/anchor bindings",
		"Spec section linked code decision",
		"dec-code-context-section-ref",
		"SpecSections: `TS.environment.001`",
		"### Referenced SpecSections",
		"Harnessable repository environment",
		"resolution=resolved",
		"baseline=current",
		"lifecycle=active",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("code_context decisions lane missing %q:\n%s", want, result)
		}
	}
}

func TestHandleQuintQuery_CodeContextInvariantsLaneSummarizesHighFanout(t *testing.T) {
	t.Parallel()

	fixture := setupCodeContextLaneFixture(t)

	invariants := make([]string, 0, 50)
	for i := 1; i <= 50; i++ {
		invariants = append(invariants, fmt.Sprintf("%q", fmt.Sprintf("fanout invariant %02d", i)))
	}
	decision := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        "dec-code-context-fanout",
			Kind:      artifact.KindDecisionRecord,
			Status:    artifact.StatusActive,
			Title:     "Code context fanout decision",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		Body:           "decision body",
		StructuredData: `{"invariants":[` + strings.Join(invariants, ",") + `]}`,
	}
	addExactFileBindingToCodeContextFixture(t, decision, fixture.file)
	if err := fixture.store.Create(context.Background(), decision); err != nil {
		t.Fatal(err)
	}
	mustExecCodeContextLaneFixture(t, fixture.store, `INSERT INTO affected_files (artifact_id, file_path) VALUES (?, ?)`, decision.Meta.ID, fixture.file)

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
		"lane":   "invariants",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(code_context invariants) returned error: %v", err)
	}
	for _, want := range []string{
		"High fanout:",
		"Default lane shows source groups",
		"Code context fanout decision",
		"full=true",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("high-fanout invariant lane missing %q:\n%s", want, result)
		}
	}
	if strings.Contains(result, "fanout invariant 49") {
		t.Fatalf("high-fanout invariant lane should not inline every invariant sentence:\n%s", result)
	}

	full, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "code_context",
		"file":   fixture.file,
		"lane":   "invariants",
		"full":   true,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(code_context invariants full) returned error: %v", err)
	}
	if !strings.Contains(full, "fanout invariant 49") {
		t.Fatalf("full invariant lane should restore audit detail:\n%s", full)
	}
}

func TestHandleQuintQuery_CodeContextSymbolsLaneCapsByLimit(t *testing.T) {
	t.Parallel()

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

func TestHandleQuintQuery_CodeContextExcludedSourceIsUnavailable(t *testing.T) {
	t.Parallel()

	fixture := setupCodeContextLaneFixture(t)
	root := filepath.Dir(fixture.haftDir)
	absoluteFile := filepath.Join(root, fixture.file)
	if err := os.WriteFile(
		absoluteFile,
		[]byte("package internal\n"+strings.Repeat(" ", 500_001)),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(
		context.Background(),
		fixture.store,
		nil,
		fixture.haftDir,
		map[string]any{
			"action": "code_context",
			"file":   fixture.file,
			"lane":   "symbols",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"coverage: bounded_with_exclusions",
		"Index exclusions:",
		"oversized",
		"Symbol lane unavailable",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("excluded code_context missing %q:\n%s", want, result)
		}
	}
	if strings.Contains(result, "No symbols indexed for this file") {
		t.Fatalf("excluded source masqueraded as indexed absence:\n%s", result)
	}
}

func TestHandleQuintQuery_CodeContextRejectsUnknownLane(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	for _, want := range []string{"### `exact_binding` — authority-bearing target/anchor bindings", "### Notes", "binding context invariant"} {
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

func TestCoverageReadsCurrentFileGapIndexWithoutMutatingIt(t *testing.T) {
	fixture := setupAffectedPathCodeContextLaneFixture(t)
	ctx := context.Background()
	root := filepath.Dir(fixture.haftDir)
	const profileSuffix = "coverage-file-gaps"
	profileHarness := profileadmissionfixture.New(t, root)
	profileHarness.AdmitSoftwareRevision(t, profileSuffix)
	scopeID := "software-" + profileSuffix
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/coverage\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unlinkedFile := filepath.Join(filepath.Dir(fixture.file), "unlinked.go")
	unlinkedAbs := filepath.Join(root, unlinkedFile)
	if err := os.WriteFile(unlinkedAbs, []byte("package internal\n\nfunc Unlinked() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := handleQuintRefresh(ctx, fixture.store, fixture.haftDir, map[string]any{
		"action":  "scan",
		"verbose": false,
	}); err != nil {
		t.Fatalf("explicit refresh failed: %v", err)
	}

	before := coverageDerivedStateSnapshot(t, fixture.store)
	result, err := handleQuintQuery(ctx, fixture.store, nil, fixture.haftDir, map[string]any{
		"action":   "coverage",
		"limit":    float64(1),
		"scope_id": scopeID,
	})
	if err != nil {
		t.Fatalf("read-only coverage failed: %v", err)
	}
	for _, want := range []string{
		"Exact File Decision-Link Gaps",
		"Current code index epoch",
		unlinkedFile,
		"not proof that a file is undocumented",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("coverage response missing %q:\n%s", want, result)
		}
	}
	after := coverageDerivedStateSnapshot(t, fixture.store)
	if after != before {
		t.Fatalf("read-only coverage mutated derived index:\nbefore=%s\nafter=%s", before, after)
	}

	if err := os.WriteFile(unlinkedAbs, []byte("package internal\n\nfunc UnlinkedChanged() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleBefore := coverageDerivedStateSnapshot(t, fixture.store)
	stale, err := handleQuintQuery(ctx, fixture.store, nil, fixture.haftDir, map[string]any{
		"action":   "coverage",
		"scope_id": scopeID,
	})
	if err != nil {
		t.Fatalf("stale read-only coverage failed: %v", err)
	}
	for _, want := range []string{
		"derived code index is `stale`",
		"not an empty or clean result",
		`haft_refresh(action="scan")`,
	} {
		if !strings.Contains(stale, want) {
			t.Fatalf("stale coverage response missing %q:\n%s", want, stale)
		}
	}
	if strings.Contains(stale, unlinkedFile) {
		t.Fatalf("stale coverage emitted a file-gap claim:\n%s", stale)
	}
	staleAfter := coverageDerivedStateSnapshot(t, fixture.store)
	if staleAfter != staleBefore {
		t.Fatalf("stale coverage mutated derived index:\nbefore=%s\nafter=%s", staleBefore, staleAfter)
	}
}

func coverageDerivedStateSnapshot(t *testing.T, store *artifact.Store) string {
	t.Helper()
	queries := []string{
		`SELECT module_id, path, name, lang, file_count, last_scanned FROM codebase_modules ORDER BY module_id`,
		`SELECT source_module, target_module, dep_type, file_path, last_scanned FROM module_dependencies ORDER BY source_module, target_module, dep_type`,
		`SELECT file_path, content_hash, language, parse_status, index_epoch, updated_at FROM code_files ORDER BY file_path`,
		`SELECT id, fingerprint, current_epoch, config_hash, schema_version, degraded, degraded_reason FROM code_index_meta ORDER BY id`,
	}

	var snapshot strings.Builder
	for _, query := range queries {
		rows, err := store.DB().QueryContext(context.Background(), query)
		if err != nil {
			t.Fatalf("snapshot derived state: %v\n%s", err, query)
		}
		columns, err := rows.Columns()
		if err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			targets := make([]any, len(columns))
			for index := range values {
				targets[index] = &values[index]
			}
			if err := rows.Scan(targets...); err != nil {
				_ = rows.Close()
				t.Fatal(err)
			}
			for _, value := range values {
				if bytes, ok := value.([]byte); ok {
					value = string(bytes)
				}
				fmt.Fprintf(&snapshot, "%v\x00", value)
			}
			snapshot.WriteByte('\n')
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		_ = rows.Close()
		snapshot.WriteString("---\n")
	}
	return snapshot.String()
}

type codeContextLaneFixture struct {
	store   *artifact.Store
	haftDir string
	file    string
}

func setupCodeContextLaneFixture(t *testing.T) codeContextLaneFixture {
	t.Helper()
	return setupCodeContextLaneFixtureWithExactBinding(t, true)
}

func setupAffectedPathCodeContextLaneFixture(t *testing.T) codeContextLaneFixture {
	t.Helper()
	return setupCodeContextLaneFixtureWithExactBinding(t, false)
}

func setupCodeContextLaneFixtureWithExactBinding(
	t *testing.T,
	exactBinding bool,
) codeContextLaneFixture {
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
	if exactBinding {
		addExactFileBindingToCodeContextFixture(t, decision, file)
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

func addExactFileBindingToCodeContextFixture(
	t *testing.T,
	item *artifact.Artifact,
	file string,
) {
	t.Helper()
	fields := map[string]any{}
	if err := json.Unmarshal([]byte(item.StructuredData), &fields); err != nil {
		t.Fatalf("decode fixture decision fields: %v", err)
	}
	fields["binding_targets"] = []artifact.BindingTarget{{
		Kind:     artifact.BindingTargetWholeFileFallback,
		FilePath: file,
	}}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("encode fixture decision fields: %v", err)
	}
	item.StructuredData = string(encoded)
}

func mustExecCodeContextLaneFixture(t *testing.T, store *artifact.Store, query string, args ...any) {
	t.Helper()
	if _, err := store.DB().ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("fixture SQL failed: %v\n%s", err, query)
	}
}
