package artifact

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/codebase"
)

func TestResolveBindingTargetsUsesSymbolsForSupportedLanguages(t *testing.T) {
	projectRoot := t.TempDir()
	fixtures := map[string]string{
		"app.go":  "package main\nfunc Run() string { return \"go\" }\n",
		"app.py":  "def run():\n    return 'python'\n",
		"app.js":  "function run() { return 'javascript'; }\n",
		"app.jsx": "function Run() { return null; }\n",
		"app.ts":  "function run(): string { return 'typescript'; }\n",
		"app.tsx": "function Run() { return null; }\n",
		"app.rs":  "fn run() -> &'static str { \"rust\" }\n",
		"app.c":   "int run() { return 1; }\n",
		"app.h":   "int run() { return 1; }\n",
		"app.cpp": "int run() { return 1; }\n",
	}

	for path, body := range fixtures {
		t.Run(path, func(t *testing.T) {
			writeTestFile(t, projectRoot, path, body)

			resolution, err := ResolveBindingTargets(
				projectRoot,
				[]AffectedFile{{Path: path}},
				BindingResolutionOptions{},
			)
			if err != nil {
				t.Fatalf("ResolveBindingTargets: %v", err)
			}
			if len(resolution.Targets) != 1 {
				t.Fatalf("targets = %+v, want one", resolution.Targets)
			}
			if resolution.Targets[0].Kind != BindingTargetSymbol {
				t.Fatalf("kind = %q, want %q", resolution.Targets[0].Kind, BindingTargetSymbol)
			}
			if len(resolution.Symbols) != 1 {
				t.Fatalf("symbols = %+v, want one affected symbol", resolution.Symbols)
			}
		})
	}
}

func TestResolveBindingTargetsBlocksAmbiguousMultiSymbolFileWithoutHint(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string { return "run" }
func Stop() string { return "stop" }
`)

	_, err := ResolveBindingTargets(
		projectRoot,
		[]AffectedFile{{Path: "app.go"}},
		BindingResolutionOptions{},
	)
	if err == nil {
		t.Fatal("expected ambiguous file to require binding resolution")
	}
	if !strings.Contains(err.Error(), "needs_binding_resolution") && !strings.Contains(err.Error(), "multiple parseable symbols") {
		t.Fatalf("error = %q, want binding resolution hint", err.Error())
	}
}

func TestResolveBindingTargetsUsesHintBeforeWholeFileFallback(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string { return "run" }
func Stop() string { return "stop" }
`)

	resolution, err := ResolveBindingTargets(
		projectRoot,
		[]AffectedFile{{Path: "app.go"}},
		BindingResolutionOptions{Hints: []string{"Stop"}},
	)
	if err != nil {
		t.Fatalf("ResolveBindingTargets: %v", err)
	}
	if len(resolution.Targets) != 1 {
		t.Fatalf("targets = %+v, want one", resolution.Targets)
	}
	if resolution.Targets[0].Kind != BindingTargetSymbol || resolution.Targets[0].SymbolName != "Stop" {
		t.Fatalf("target = %+v, want Stop symbol target", resolution.Targets[0])
	}
}

func TestResolveBindingTargetsRequiresReasonForExplicitWholeFileScope(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, projectRoot, "app.go", "package main\nfunc Run() {}\n")

	_, err := ResolveBindingTargets(
		projectRoot,
		[]AffectedFile{{Path: "app.go"}},
		BindingResolutionOptions{Scope: BindingScopeWholeFile},
	)
	if err == nil {
		t.Fatal("expected whole-file scope without reason to fail")
	}
}

func TestBindingResolutionStrategyOrderMakesWholeFileFallbackLast(t *testing.T) {
	order := BindingResolutionStrategyOrder()
	if len(order) == 0 {
		t.Fatal("strategy order should not be empty")
	}
	if order[len(order)-1] != BindingResolutionSourceWholeFileFallback {
		t.Fatalf("last strategy = %q, want %q", order[len(order)-1], BindingResolutionSourceWholeFileFallback)
	}
	for _, want := range []string{
		BindingResolutionSourceSingleSymbolFile,
		BindingResolutionSourceOperatorDecisionHint,
		BindingResolutionSourceLanguageAdapterRange,
		BindingResolutionSourceWholeFileFallback,
	} {
		if !containsString(order, want) {
			t.Fatalf("strategy order missing %q: %#v", want, order)
		}
	}
}

func TestSupportedBindingLanguagesMatrixCoversResolverLanguages(t *testing.T) {
	matrix := codebase.SupportedBindingLanguages()
	for _, want := range []string{"go", "python", "javascript", "typescript", "rust", "c", "cpp"} {
		if !bindingLanguageMatrixHas(matrix, want) {
			t.Fatalf("supported binding language matrix missing %q: %#v", want, matrix)
		}
	}
	for _, entry := range matrix {
		if len(entry.Extensions) == 0 {
			t.Fatalf("language %q has no extensions: %#v", entry.Language, entry)
		}
		if !entry.SymbolExtraction || !entry.RangeFallback {
			t.Fatalf("language %q should declare symbol extraction and range fallback: %#v", entry.Language, entry)
		}
	}
}

func TestNormalizeBindingTargetsPreservesExplicitSemanticRefsWithoutFilePath(t *testing.T) {
	targets := normalizeBindingTargets([]BindingTarget{
		{
			Kind:      BindingTargetAPIContract,
			TargetRef: " api_contract:haft/status ",
		},
		{
			Kind:      BindingTargetInvariant,
			TargetRef: " invariant:decision-terminal-status ",
		},
		{
			Kind:      BindingTargetSpecSection,
			TargetRef: " spec_section:target-system#authority ",
		},
	})

	if len(targets) != 3 {
		t.Fatalf("targets = %#v, want three semantic targets", targets)
	}
	for _, target := range targets {
		if target.FilePath != "" {
			t.Fatalf("semantic target should not require file_path: %#v", target)
		}
		if target.TargetRef == "" {
			t.Fatalf("semantic target should keep target_ref: %#v", target)
		}
		if target.ResolutionSource != BindingResolutionSourceExplicitTargets {
			t.Fatalf("resolution_source = %q, want %q", target.ResolutionSource, BindingResolutionSourceExplicitTargets)
		}
	}
}

func TestNormalizeBindingTargetsKeepsReceiverOverloadsDistinct(t *testing.T) {
	targets := normalizeBindingTargets([]BindingTarget{
		{
			Kind:       BindingTargetSymbol,
			FilePath:   "store.go",
			SymbolKind: "method",
			SymbolName: "Get",
			Receiver:   "SQLiteBaselineStore",
		},
		{
			Kind:       BindingTargetSymbol,
			FilePath:   "store.go",
			SymbolKind: "method",
			SymbolName: "Get",
			Receiver:   "MemoryBaselineStore",
		},
	})

	if len(targets) != 2 {
		t.Fatalf("targets = %#v, want receiver overloads kept distinct", targets)
	}
}

func TestSemanticBindingTargetsParticipateInReconciliationTargetKeys(t *testing.T) {
	targets := decisionReconciliationGovernanceTargets([]BindingTarget{{
		Kind:      BindingTargetAPIContract,
		TargetRef: "api_contract:haft/status",
	}})

	if len(targets) != 1 || targets[0] != "api_contract:haft/status" {
		t.Fatalf("governance targets = %#v, want api contract target ref", targets)
	}
}

func TestBaselinePersistsBindingTargetsAndPreciseSymbols(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string { return "run" }
func Stop() string { return "stop" }
`)

	dec := createTestDecision(t, store, "dec-binding-001", "Use Stop")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "app.go"}}); err != nil {
		t.Fatal(err)
	}

	_, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:  dec.Meta.ID,
		BindingHints: []string{"Stop"},
	})
	if err != nil {
		t.Fatalf("Baseline: %v", err)
	}

	updated, err := store.Get(ctx, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields := updated.UnmarshalDecisionFields()
	if len(fields.BindingTargets) != 1 || fields.BindingTargets[0].SymbolName != "Stop" {
		t.Fatalf("binding targets = %+v, want Stop", fields.BindingTargets)
	}

	symbols, err := store.GetAffectedSymbols(ctx, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 || symbols[0].SymbolName != "Stop" {
		t.Fatalf("affected symbols = %+v, want Stop only", symbols)
	}
}

func TestWholeFileFallbackDriftNeedsBindingResolution(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "notes.txt", "first\n")

	dec := createTestDecision(t, store, "dec-binding-002", "Unsupported text")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "notes.txt"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "notes.txt", "second\n")

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drifted file", reports)
	}
	if reports[0].Files[0].Materiality != DriftMaterialityNeedsBindingResolution {
		t.Fatalf("materiality = %q, want %q", reports[0].Files[0].Materiality, DriftMaterialityNeedsBindingResolution)
	}
	if got := reports[0].SymbolVerdict(); got != SymbolVerdictNeedsReview {
		t.Fatalf("SymbolVerdict = %q, want %q", got, SymbolVerdictNeedsReview)
	}
}

func TestUnsupportedLanguageFallbackMatrixNeedsBindingResolution(t *testing.T) {
	for _, tc := range []struct {
		path    string
		initial string
		changed string
	}{
		{
			path:    "service.rb",
			initial: "def call\n  :ok\nend\n",
			changed: "def call\n  :changed\nend\n",
		},
		{
			path:    "Controller.java",
			initial: "class Controller { String call() { return \"ok\"; } }\n",
			changed: "class Controller { String call() { return \"changed\"; } }\n",
		},
		{
			path:    "handler.php",
			initial: "<?php function call() { return 'ok'; }\n",
			changed: "<?php function call() { return 'changed'; }\n",
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			store := setupTestDB(t)
			ctx := context.Background()
			projectRoot := t.TempDir()

			writeTestFile(t, projectRoot, tc.path, tc.initial)
			resolution, err := ResolveBindingTargets(
				projectRoot,
				[]AffectedFile{{Path: tc.path}},
				BindingResolutionOptions{},
			)
			if err != nil {
				t.Fatalf("ResolveBindingTargets: %v", err)
			}
			if len(resolution.Targets) != 1 {
				t.Fatalf("targets = %+v, want one fallback target", resolution.Targets)
			}
			target := resolution.Targets[0]
			if target.Kind != BindingTargetWholeFileFallback {
				t.Fatalf("target kind = %q, want %q", target.Kind, BindingTargetWholeFileFallback)
			}
			if target.ResolutionSource != BindingResolutionSourceWholeFileFallback {
				t.Fatalf("resolution_source = %q, want whole_file_fallback", target.ResolutionSource)
			}
			if target.LanguageSupport != codebase.BindingSupportUnsupportedLanguage {
				t.Fatalf("language_support = %q, want unsupported_language", target.LanguageSupport)
			}
			if target.Confidence != "low" || target.Reason == "" {
				t.Fatalf("fallback target should be low-confidence with reason: %+v", target)
			}

			dec := createTestDecision(t, store, "dec-unsupported-"+strings.ReplaceAll(tc.path, ".", "-"), "Unsupported fallback")
			if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: tc.path}}); err != nil {
				t.Fatal(err)
			}
			if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
				DecisionRef:    dec.Meta.ID,
				BindingTargets: resolution.Targets,
			}); err != nil {
				t.Fatal(err)
			}

			writeTestFile(t, projectRoot, tc.path, tc.changed)
			reports, err := CheckDrift(ctx, store, projectRoot)
			if err != nil {
				t.Fatal(err)
			}
			if len(reports) != 1 || len(reports[0].Files) != 1 {
				t.Fatalf("reports = %+v, want one drift report", reports)
			}
			file := reports[0].Files[0]
			if file.Materiality != DriftMaterialityNeedsBindingResolution {
				t.Fatalf("materiality = %q, want %q", file.Materiality, DriftMaterialityNeedsBindingResolution)
			}
			if file.FallbackKind != BindingTargetWholeFileFallback {
				t.Fatalf("fallback_kind = %q, want %q", file.FallbackKind, BindingTargetWholeFileFallback)
			}
			if reports[0].SymbolVerdict() != SymbolVerdictNeedsReview {
				t.Fatalf("SymbolVerdict = %q, want %q", reports[0].SymbolVerdict(), SymbolVerdictNeedsReview)
			}
		})
	}
}

func TestSemanticBindingTargetIgnoresCarrierOnlyHashChurnWhenTextHashUnchanged(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "contracts.md", "status contract\n")
	target := semanticTargetForStableFile(t, projectRoot, "contracts.md", BindingTargetAPIContract, "api_contract:haft/status")
	dec := createTestDecision(t, store, "dec-semantic-target-audit", "Semantic target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "contracts.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:    dec.Meta.ID,
		BindingTargets: []BindingTarget{target},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "contracts.md", "status contract   \n")
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn || !file.AuditOnly {
		t.Fatalf("file = %+v, want audit-only unchanged semantic target", file)
	}
}

func TestSemanticBindingTargetWithoutEvaluatorNeedsBindingResolution(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "contracts.md", "status contract\n")
	target := BindingTarget{
		Kind:      BindingTargetAPIContract,
		TargetRef: "api_contract:haft/status",
		FilePath:  "contracts.md",
	}
	dec := createTestDecision(t, store, "dec-semantic-target-missing-evaluator", "Semantic target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "contracts.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:    dec.Meta.ID,
		BindingTargets: []BindingTarget{target},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "contracts.md", "status contract changed\n")
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityNeedsBindingResolution {
		t.Fatalf("materiality = %q, want %q", file.Materiality, DriftMaterialityNeedsBindingResolution)
	}
	if file.FallbackKind != "semantic_target_missing_evaluator" {
		t.Fatalf("fallback_kind = %q, want semantic_target_missing_evaluator", file.FallbackKind)
	}
}

func TestSemanticBindingTargetTextHashChangeIsMaterialSemanticTarget(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "contracts.md", "status contract\n")
	target := semanticTargetForStableFile(t, projectRoot, "contracts.md", BindingTargetAPIContract, "api_contract:haft/status")
	dec := createTestDecision(t, store, "dec-semantic-target-changed", "Semantic target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "contracts.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:    dec.Meta.ID,
		BindingTargets: []BindingTarget{target},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "contracts.md", "status contract changed\n")
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityMaterialSemanticTarget {
		t.Fatalf("materiality = %q, want %q", file.Materiality, DriftMaterialityMaterialSemanticTarget)
	}
	if got := reports[0].SymbolVerdict(); got != SymbolVerdictGovernedModified {
		t.Fatalf("SymbolVerdict = %q, want %q", got, SymbolVerdictGovernedModified)
	}
}

func TestResolveBindingTargetsAttachesSpecSectionEvaluatorFromMarkdownCarrier(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, projectRoot, "spec.md", `# Spec

## TS.boundary.001 Governance boundary

`+"```yaml spec-section\n"+
		`id: TS.boundary.001
title: Governance boundary
status: active
`+"```\n\nBody text.\n")

	resolution, err := ResolveBindingTargets(
		projectRoot,
		[]AffectedFile{{Path: "spec.md"}},
		BindingResolutionOptions{ExplicitTargets: []BindingTarget{{
			Kind:      BindingTargetSpecSection,
			TargetRef: "spec_section:TS.boundary.001",
			FilePath:  "spec.md",
		}}},
	)
	if err != nil {
		t.Fatalf("ResolveBindingTargets: %v", err)
	}
	if len(resolution.Targets) != 1 {
		t.Fatalf("targets = %#v, want one", resolution.Targets)
	}
	target := resolution.Targets[0]
	if target.TextHash == "" || target.AnchorHash == "" {
		t.Fatalf("target did not receive evaluator hashes: %#v", target)
	}
	if target.Line != 3 || target.EndLine == 0 {
		t.Fatalf("target range = %d-%d, want markdown section range from heading", target.Line, target.EndLine)
	}
	if target.ResolutionSource != BindingResolutionSourceMarkdownSection {
		t.Fatalf("resolution_source = %q, want %q", target.ResolutionSource, BindingResolutionSourceMarkdownSection)
	}
}

func TestResolveBindingTargetsAttachesSemanticEvaluatorFromMarkdownHeading(t *testing.T) {
	for _, tc := range []struct {
		name      string
		kind      string
		targetRef string
		heading   string
	}{
		{
			name:      "api contract",
			kind:      BindingTargetAPIContract,
			targetRef: "api_contract:haft/status",
			heading:   "haft/status API contract",
		},
		{
			name:      "invariant",
			kind:      BindingTargetInvariant,
			targetRef: "invariant:decision-terminal-status",
			heading:   "decision-terminal-status invariant",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			writeTestFile(t, projectRoot, "contracts.md", "# Contracts\n\n## "+tc.heading+"\n\nGoverned body.\n\n## Other\n\nOther body.\n")

			resolution, err := ResolveBindingTargets(
				projectRoot,
				[]AffectedFile{{Path: "contracts.md"}},
				BindingResolutionOptions{ExplicitTargets: []BindingTarget{{
					Kind:      tc.kind,
					TargetRef: tc.targetRef,
					FilePath:  "contracts.md",
				}}},
			)
			if err != nil {
				t.Fatalf("ResolveBindingTargets: %v", err)
			}
			if len(resolution.Targets) != 1 {
				t.Fatalf("targets = %#v, want one", resolution.Targets)
			}
			target := resolution.Targets[0]
			if target.TextHash == "" || target.AnchorHash == "" {
				t.Fatalf("target did not receive evaluator hashes: %#v", target)
			}
			if target.Line != 3 || target.EndLine == 0 {
				t.Fatalf("target range = %d-%d, want markdown heading range", target.Line, target.EndLine)
			}
			if target.ResolutionSource != BindingResolutionSourceMarkdownSection {
				t.Fatalf("resolution_source = %q, want %q", target.ResolutionSource, BindingResolutionSourceMarkdownSection)
			}
		})
	}
}

func TestResolveBindingTargetsAttachesSemanticEvaluatorFromMarkdownFencedTarget(t *testing.T) {
	for _, tc := range []struct {
		name      string
		kind      string
		targetRef string
		fenceInfo string
		body      string
	}{
		{
			name:      "api contract",
			kind:      BindingTargetAPIContract,
			targetRef: "api_contract:haft/status",
			fenceInfo: "yaml api-contract",
			body: "target_ref: api_contract:haft/status\n" +
				"method: haft_query.status\n",
		},
		{
			name:      "invariant",
			kind:      BindingTargetInvariant,
			targetRef: "invariant:decision-terminal-status",
			fenceInfo: "yaml invariant",
			body: "id: decision-terminal-status\n" +
				"statement: terminal decisions do not reopen silently\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			writeTestFile(t, projectRoot, "contracts.md",
				"# Contracts\n\n"+
					"## Governed target\n\n"+
					"```"+tc.fenceInfo+"\n"+
					tc.body+
					"```\n\n"+
					"## Other target\n\n"+
					"```yaml api-contract\n"+
					"target_ref: api_contract:haft/other\n"+
					"method: haft_query.other\n"+
					"```\n")

			resolution, err := ResolveBindingTargets(
				projectRoot,
				[]AffectedFile{{Path: "contracts.md"}},
				BindingResolutionOptions{ExplicitTargets: []BindingTarget{{
					Kind:      tc.kind,
					TargetRef: tc.targetRef,
					FilePath:  "contracts.md",
				}}},
			)
			if err != nil {
				t.Fatalf("ResolveBindingTargets: %v", err)
			}
			if len(resolution.Targets) != 1 {
				t.Fatalf("targets = %#v, want one", resolution.Targets)
			}
			target := resolution.Targets[0]
			if target.TextHash == "" || target.AnchorHash == "" {
				t.Fatalf("target did not receive evaluator hashes: %#v", target)
			}
			if target.Line != 3 || target.EndLine == 0 {
				t.Fatalf("target range = %d-%d, want fenced markdown section", target.Line, target.EndLine)
			}
			if target.ResolutionSource != BindingResolutionSourceMarkdownSection {
				t.Fatalf("resolution_source = %q, want %q", target.ResolutionSource, BindingResolutionSourceMarkdownSection)
			}
		})
	}
}

func TestResolveBindingTargetsAttachesSemanticEvaluatorFromTargetMarker(t *testing.T) {
	for _, tc := range []struct {
		name      string
		kind      string
		targetRef string
		path      string
		body      string
	}{
		{
			name:      "api contract in go source",
			kind:      BindingTargetAPIContract,
			targetRef: "api_contract:haft/status",
			path:      "contracts.go",
			body:      "package contracts\n\n// haft-target: api_contract:haft/status\n// status contract body\nfunc StatusContract() {}\n\n// haft-target: api_contract:haft/other\nfunc OtherContract() {}\n",
		},
		{
			name:      "invariant in python source",
			kind:      BindingTargetInvariant,
			targetRef: "invariant:decision-terminal-status",
			path:      "contracts.py",
			body:      "# haft-target: invariant:decision-terminal-status\nTERMINAL = {'superseded', 'deprecated'}\n\n# haft-target: invariant:other\nOTHER = True\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			writeTestFile(t, projectRoot, tc.path, tc.body)

			resolution, err := ResolveBindingTargets(
				projectRoot,
				[]AffectedFile{{Path: tc.path}},
				BindingResolutionOptions{ExplicitTargets: []BindingTarget{{
					Kind:      tc.kind,
					TargetRef: tc.targetRef,
					FilePath:  tc.path,
				}}},
			)
			if err != nil {
				t.Fatalf("ResolveBindingTargets: %v", err)
			}
			if len(resolution.Targets) != 1 {
				t.Fatalf("targets = %#v, want one", resolution.Targets)
			}
			target := resolution.Targets[0]
			if target.TextHash == "" || target.AnchorHash == "" {
				t.Fatalf("target did not receive evaluator hashes: %#v", target)
			}
			if target.ResolutionSource != BindingResolutionSourceTargetMarker {
				t.Fatalf("resolution_source = %q, want %q", target.ResolutionSource, BindingResolutionSourceTargetMarker)
			}
			if target.Language == "" {
				t.Fatalf("target language should be filled from source extension: %#v", target)
			}
		})
	}
}

func TestResolveBindingTargetsAttachesSemanticEvaluatorFromYAMLTarget(t *testing.T) {
	for _, tc := range []struct {
		name      string
		kind      string
		targetRef string
		body      string
		endLine   int
	}{
		{
			name:      "spec section",
			kind:      BindingTargetSpecSection,
			targetRef: "spec_section:TS.boundary.001",
			endLine:   4,
			body: "sections:\n" +
				"  - id: TS.boundary.001\n" +
				"    title: Governance boundary\n" +
				"    status: active\n" +
				"  - id: TS.other.001\n" +
				"    title: Other\n",
		},
		{
			name:      "spec section section_id",
			kind:      BindingTargetSpecSection,
			targetRef: "spec_section:TS.boundary.001",
			endLine:   4,
			body: "sections:\n" +
				"  - section_id: TS.boundary.001\n" +
				"    title: Governance boundary\n" +
				"    status: active\n" +
				"  - section_id: TS.other.001\n" +
				"    title: Other\n",
		},
		{
			name:      "api contract",
			kind:      BindingTargetAPIContract,
			targetRef: "api_contract:haft/status",
			endLine:   3,
			body: "contracts:\n" +
				"  - target_ref: api_contract:haft/status\n" +
				"    method: haft_query.status\n" +
				"  - target_ref: api_contract:haft/other\n" +
				"    method: haft_query.other\n",
		},
		{
			name:      "invariant",
			kind:      BindingTargetInvariant,
			targetRef: "invariant:decision-terminal-status",
			endLine:   3,
			body: "invariants:\n" +
				"  - target_ref: invariant:decision-terminal-status\n" +
				"    statement: terminal decisions do not reopen silently\n" +
				"  - target_ref: invariant:other\n" +
				"    statement: other\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			writeTestFile(t, projectRoot, "targets.yaml", tc.body)

			resolution, err := ResolveBindingTargets(
				projectRoot,
				[]AffectedFile{{Path: "targets.yaml"}},
				BindingResolutionOptions{ExplicitTargets: []BindingTarget{{
					Kind:      tc.kind,
					TargetRef: tc.targetRef,
					FilePath:  "targets.yaml",
				}}},
			)
			if err != nil {
				t.Fatalf("ResolveBindingTargets: %v", err)
			}
			if len(resolution.Targets) != 1 {
				t.Fatalf("targets = %#v, want one", resolution.Targets)
			}
			target := resolution.Targets[0]
			if target.TextHash == "" || target.AnchorHash == "" {
				t.Fatalf("target did not receive evaluator hashes: %#v", target)
			}
			if target.ResolutionSource != BindingResolutionSourceYAMLTarget {
				t.Fatalf("resolution_source = %q, want %q", target.ResolutionSource, BindingResolutionSourceYAMLTarget)
			}
			if target.Line != 2 || target.EndLine != tc.endLine {
				t.Fatalf("target range = %d-%d, want first yaml object only", target.Line, target.EndLine)
			}
		})
	}
}

func TestResolveBindingTargetsAttachesSemanticEvaluatorFromJSONTarget(t *testing.T) {
	for _, tc := range []struct {
		name      string
		kind      string
		targetRef string
		body      string
		endLine   int
	}{
		{
			name:      "spec section",
			kind:      BindingTargetSpecSection,
			targetRef: "spec_section:TS.boundary.001",
			endLine:   6,
			body: "{\n" +
				"  \"sections\": [\n" +
				"    {\n" +
				"      \"id\": \"TS.boundary.001\",\n" +
				"      \"title\": \"Governance boundary\"\n" +
				"    },\n" +
				"    {\"id\": \"TS.other.001\", \"title\": \"Other\"}\n" +
				"  ]\n" +
				"}\n",
		},
		{
			name:      "spec section section_id",
			kind:      BindingTargetSpecSection,
			targetRef: "spec_section:TS.boundary.001",
			endLine:   6,
			body: "{\n" +
				"  \"sections\": [\n" +
				"    {\n" +
				"      \"section_id\": \"TS.boundary.001\",\n" +
				"      \"title\": \"Governance boundary\"\n" +
				"    },\n" +
				"    {\"section_id\": \"TS.other.001\", \"title\": \"Other\"}\n" +
				"  ]\n" +
				"}\n",
		},
		{
			name:      "api contract",
			kind:      BindingTargetAPIContract,
			targetRef: "api_contract:haft/status",
			endLine:   6,
			body: "{\n" +
				"  \"contracts\": [\n" +
				"    {\n" +
				"      \"target_ref\": \"api_contract:haft/status\",\n" +
				"      \"method\": \"haft_query.status\"\n" +
				"    },\n" +
				"    {\"target_ref\": \"api_contract:haft/other\", \"method\": \"haft_query.other\"}\n" +
				"  ]\n" +
				"}\n",
		},
		{
			name:      "invariant",
			kind:      BindingTargetInvariant,
			targetRef: "invariant:decision-terminal-status",
			endLine:   6,
			body: "{\n" +
				"  \"invariants\": [\n" +
				"    {\n" +
				"      \"target_ref\": \"invariant:decision-terminal-status\",\n" +
				"      \"statement\": \"terminal decisions do not reopen silently\"\n" +
				"    },\n" +
				"    {\"target_ref\": \"invariant:other\", \"statement\": \"other\"}\n" +
				"  ]\n" +
				"}\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			writeTestFile(t, projectRoot, "targets.json", tc.body)

			resolution, err := ResolveBindingTargets(
				projectRoot,
				[]AffectedFile{{Path: "targets.json"}},
				BindingResolutionOptions{ExplicitTargets: []BindingTarget{{
					Kind:      tc.kind,
					TargetRef: tc.targetRef,
					FilePath:  "targets.json",
				}}},
			)
			if err != nil {
				t.Fatalf("ResolveBindingTargets: %v", err)
			}
			if len(resolution.Targets) != 1 {
				t.Fatalf("targets = %#v, want one", resolution.Targets)
			}
			target := resolution.Targets[0]
			if target.TextHash == "" || target.AnchorHash == "" {
				t.Fatalf("target did not receive evaluator hashes: %#v", target)
			}
			if target.ResolutionSource != BindingResolutionSourceJSONTarget {
				t.Fatalf("resolution_source = %q, want %q", target.ResolutionSource, BindingResolutionSourceJSONTarget)
			}
			if target.Line != 3 || target.EndLine != tc.endLine {
				t.Fatalf("target range = %d-%d, want first json object only", target.Line, target.EndLine)
			}
		})
	}
}

func TestTargetMarkerSemanticTargetDriftUsesBoundedHash(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	initial := `package contracts

// haft-target: api_contract:haft/status
// status contract body
func StatusContract() {}

// haft-target: api_contract:haft/other
func OtherContract() {}
`
	writeTestFile(t, projectRoot, "contracts.go", initial)
	dec := createTestDecision(t, store, "dec-api-contract-marker-evaluator", "API contract marker target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "contracts.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
		BindingTargets: []BindingTarget{{
			Kind:      BindingTargetAPIContract,
			TargetRef: "api_contract:haft/status",
			FilePath:  "contracts.go",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "contracts.go", strings.Replace(initial, "func OtherContract() {}", "func OtherContract() { println(\"changed\") }", 1))
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn || !file.AuditOnly {
		t.Fatalf("file = %+v, want audit-only drift outside target marker block", file)
	}

	writeTestFile(t, projectRoot, "contracts.go", strings.Replace(initial, "status contract body", "status contract body changed", 1))
	reports, err = CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	file = reports[0].Files[0]
	if file.Materiality != DriftMaterialityMaterialSemanticTarget {
		t.Fatalf("file = %+v, want material semantic target after marker block changes", file)
	}
	if file.ChangedTargetRef != "api_contract:haft/status" || file.TargetKind != BindingTargetAPIContract {
		t.Fatalf("target = %q/%q, want api_contract target", file.ChangedTargetRef, file.TargetKind)
	}
}

func TestJSONSemanticTargetDriftUsesBoundedHash(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	initial := "{\n" +
		"  \"contracts\": [\n" +
		"    {\n" +
		"      \"target_ref\": \"api_contract:haft/status\",\n" +
		"      \"method\": \"haft_query.status\"\n" +
		"    },\n" +
		"    {\"target_ref\": \"api_contract:haft/other\", \"method\": \"haft_query.other\"}\n" +
		"  ]\n" +
		"}\n"
	writeTestFile(t, projectRoot, "contracts.json", initial)
	dec := createTestDecision(t, store, "dec-api-contract-json-evaluator", "API contract json target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "contracts.json"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
		BindingTargets: []BindingTarget{{
			Kind:      BindingTargetAPIContract,
			TargetRef: "api_contract:haft/status",
			FilePath:  "contracts.json",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "contracts.json", strings.Replace(initial, "haft_query.other", "haft_query.other_changed", 1))
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn || !file.AuditOnly {
		t.Fatalf("file = %+v, want audit-only drift outside json target object", file)
	}

	writeTestFile(t, projectRoot, "contracts.json", strings.Replace(initial, "haft_query.status", "haft_query.status_changed", 1))
	reports, err = CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	file = reports[0].Files[0]
	if file.Materiality != DriftMaterialityMaterialSemanticTarget {
		t.Fatalf("file = %+v, want material semantic target after json object changes", file)
	}
	if file.ChangedTargetRef != "api_contract:haft/status" || file.TargetKind != BindingTargetAPIContract {
		t.Fatalf("target = %q/%q, want api_contract target", file.ChangedTargetRef, file.TargetKind)
	}
}

func TestYAMLSemanticTargetDriftUsesBoundedHash(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	initial := "contracts:\n" +
		"  - target_ref: api_contract:haft/status\n" +
		"    method: haft_query.status\n" +
		"  - target_ref: api_contract:haft/other\n" +
		"    method: haft_query.other\n"
	writeTestFile(t, projectRoot, "contracts.yaml", initial)
	dec := createTestDecision(t, store, "dec-api-contract-yaml-evaluator", "API contract yaml target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "contracts.yaml"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
		BindingTargets: []BindingTarget{{
			Kind:      BindingTargetAPIContract,
			TargetRef: "api_contract:haft/status",
			FilePath:  "contracts.yaml",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "contracts.yaml", strings.Replace(initial, "haft_query.other", "haft_query.other_changed", 1))
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn || !file.AuditOnly {
		t.Fatalf("file = %+v, want audit-only drift outside yaml target block", file)
	}

	writeTestFile(t, projectRoot, "contracts.yaml", strings.Replace(initial, "haft_query.status", "haft_query.status_changed", 1))
	reports, err = CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	file = reports[0].Files[0]
	if file.Materiality != DriftMaterialityMaterialSemanticTarget {
		t.Fatalf("file = %+v, want material semantic target after yaml block changes", file)
	}
	if file.ChangedTargetRef != "api_contract:haft/status" || file.TargetKind != BindingTargetAPIContract {
		t.Fatalf("target = %q/%q, want api_contract target", file.ChangedTargetRef, file.TargetKind)
	}
}

func TestMarkdownSemanticTargetHeadingDriftUsesBoundedHash(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	initial := "# Contracts\n\n## haft/status API contract\n\nGoverned body.\n\n## Other\n\nOther body.\n"
	writeTestFile(t, projectRoot, "contracts.md", initial)
	dec := createTestDecision(t, store, "dec-api-contract-heading-evaluator", "API contract target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "contracts.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
		BindingTargets: []BindingTarget{{
			Kind:      BindingTargetAPIContract,
			TargetRef: "api_contract:haft/status",
			FilePath:  "contracts.md",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "contracts.md", strings.Replace(initial, "Other body.", "Other body changed.", 1))
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn || !file.AuditOnly {
		t.Fatalf("file = %+v, want audit-only drift outside bounded API contract", file)
	}

	writeTestFile(t, projectRoot, "contracts.md", strings.Replace(initial, "Governed body.", "Governed body changed.", 1))
	reports, err = CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	file = reports[0].Files[0]
	if file.Materiality != DriftMaterialityMaterialSemanticTarget {
		t.Fatalf("file = %+v, want material semantic target after API contract body changes", file)
	}
	if file.ChangedTargetRef != "api_contract:haft/status" || file.TargetKind != BindingTargetAPIContract {
		t.Fatalf("target = %q/%q, want api_contract target", file.ChangedTargetRef, file.TargetKind)
	}
}

func TestMarkdownSemanticTargetFencedDriftUsesBoundedHash(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	initial := "# Contracts\n\n" +
		"## Status contract\n\n" +
		"```yaml api-contract\n" +
		"target_ref: api_contract:haft/status\n" +
		"method: haft_query.status\n" +
		"```\n\n" +
		"## Other contract\n\n" +
		"```yaml api-contract\n" +
		"target_ref: api_contract:haft/other\n" +
		"method: haft_query.other\n" +
		"```\n"
	writeTestFile(t, projectRoot, "contracts.md", initial)
	dec := createTestDecision(t, store, "dec-api-contract-fenced-evaluator", "API contract fenced target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "contracts.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
		BindingTargets: []BindingTarget{{
			Kind:      BindingTargetAPIContract,
			TargetRef: "api_contract:haft/status",
			FilePath:  "contracts.md",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "contracts.md", strings.Replace(initial, "haft_query.other", "haft_query.other_changed", 1))
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn || !file.AuditOnly {
		t.Fatalf("file = %+v, want audit-only drift outside fenced target block", file)
	}

	writeTestFile(t, projectRoot, "contracts.md", strings.Replace(initial, "haft_query.status", "haft_query.status_changed", 1))
	reports, err = CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	file = reports[0].Files[0]
	if file.Materiality != DriftMaterialityMaterialSemanticTarget {
		t.Fatalf("file = %+v, want material semantic target after fenced block changes", file)
	}
	if file.ChangedTargetRef != "api_contract:haft/status" || file.TargetKind != BindingTargetAPIContract {
		t.Fatalf("target = %q/%q, want api_contract target", file.ChangedTargetRef, file.TargetKind)
	}
}

func TestAutoAttachedSpecSectionTargetDriftUsesBoundedSectionHash(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	initial := `# Spec

## TS.boundary.001 Governance boundary

` + "```yaml spec-section\n" +
		`id: TS.boundary.001
title: Governance boundary
status: active
` + "```\n\nBody text.\n\n## TS.other.001 Other\n\nOther text.\n"
	writeTestFile(t, projectRoot, "spec.md", initial)
	dec := createTestDecision(t, store, "dec-spec-section-auto-evaluator", "Spec section target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "spec.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
		BindingTargets: []BindingTarget{{
			Kind:      BindingTargetSpecSection,
			TargetRef: "spec_section:TS.boundary.001",
			FilePath:  "spec.md",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "spec.md", strings.Replace(initial, "Other text.", "Other text changed.", 1))
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn || !file.AuditOnly {
		t.Fatalf("file = %+v, want audit-only drift outside bounded spec section", file)
	}

	writeTestFile(t, projectRoot, "spec.md", strings.Replace(initial, "Body text.", "Body text changed.", 1))
	reports, err = CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	file = reports[0].Files[0]
	if file.Materiality != DriftMaterialityMaterialSemanticTarget {
		t.Fatalf("file = %+v, want material semantic target after section body changes", file)
	}
}

func TestAutoAttachedSpecSectionDeletionIsTargetDeleted(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	initial := `# Spec

## TS.boundary.001 Governance boundary

` + "```yaml spec-section\n" +
		`id: TS.boundary.001
title: Governance boundary
status: active
` + "```\n\nBody text.\n\n## TS.other.001 Other\n\nOther text.\n"
	writeTestFile(t, projectRoot, "spec.md", initial)
	dec := createTestDecision(t, store, "dec-spec-section-deleted", "Spec section target deleted")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "spec.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
		BindingTargets: []BindingTarget{{
			Kind:      BindingTargetSpecSection,
			TargetRef: "spec_section:TS.boundary.001",
			FilePath:  "spec.md",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "spec.md", "# Spec\n\n## TS.other.001 Other\n\nOther text.\n")
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityMaterialSemanticTarget {
		t.Fatalf("file = %+v, want material semantic target after section deletion", file)
	}
	if file.ChangedTargetRef != "spec_section:TS.boundary.001" {
		t.Fatalf("changed_target_ref = %q, want spec_section:TS.boundary.001", file.ChangedTargetRef)
	}
	if file.TargetKind != BindingTargetSpecSection || file.TargetStatus != "removed" {
		t.Fatalf("target = kind %q status %q, want spec_section removed", file.TargetKind, file.TargetStatus)
	}

	eventReport := BuildDriftEventReport(reports)
	if len(eventReport.Events) != 1 {
		t.Fatalf("events = %+v, want one target-level event", eventReport.Events)
	}
	event := eventReport.Events[0]
	if event.ChangedTargetRef != "spec_section:TS.boundary.001" || event.TargetKind != BindingTargetSpecSection {
		t.Fatalf("event target = %q/%q, want deleted spec section", event.ChangedTargetRef, event.TargetKind)
	}
	if event.RootCause != DriftEventRootCauseTargetDeleted {
		t.Fatalf("root_cause = %q, want %q", event.RootCause, DriftEventRootCauseTargetDeleted)
	}
	if event.ResolutionStatus != DriftEventResolutionNeedsOperatorJudgment {
		t.Fatalf("resolution_status = %q, want %q", event.ResolutionStatus, DriftEventResolutionNeedsOperatorJudgment)
	}
}

func TestReceiverBindingTargetIgnoresAdjacentMethodChange(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "store.go", `package main

type SQLiteBaselineStore struct{}
func (s SQLiteBaselineStore) Get() string { return "sqlite" }

type MemoryBaselineStore struct{}
func (s MemoryBaselineStore) Get() string { return "memory" }
`)

	target := bindingTargetForSnapshot(t, projectRoot, "store.go", "Get", "MemoryBaselineStore")
	dec := createTestDecision(t, store, "dec-binding-receiver-adjacent", "Receiver target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "store.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:    dec.Meta.ID,
		BindingTargets: []BindingTarget{target},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "store.go", `package main

type SQLiteBaselineStore struct{}
func (s SQLiteBaselineStore) Get() string { return "sqlite changed" }

type MemoryBaselineStore struct{}
func (s MemoryBaselineStore) Get() string { return "memory" }
`)

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn || !file.AuditOnly {
		t.Fatalf("file = %+v, want audit-only adjacent churn", file)
	}
}

func TestReceiverBindingTargetDetectsSelectedMethodChange(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "store.go", `package main

type SQLiteBaselineStore struct{}
func (s SQLiteBaselineStore) Get() string { return "sqlite" }

type MemoryBaselineStore struct{}
func (s MemoryBaselineStore) Get() string { return "memory" }
`)

	target := bindingTargetForSnapshot(t, projectRoot, "store.go", "Get", "MemoryBaselineStore")
	dec := createTestDecision(t, store, "dec-binding-receiver-selected", "Receiver target selected")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "store.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:    dec.Meta.ID,
		BindingTargets: []BindingTarget{target},
	}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "store.go", `package main

type SQLiteBaselineStore struct{}
func (s SQLiteBaselineStore) Get() string { return "sqlite" }

type MemoryBaselineStore struct{}
func (s MemoryBaselineStore) Get() string { return "memory changed" }
`)

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityMaterialSymbol || file.AuditOnly {
		t.Fatalf("file = %+v, want material symbol drift", file)
	}
	if len(file.Symbols) != 1 || file.Symbols[0].SymbolName != "Get" || file.Symbols[0].Status != "modified" {
		t.Fatalf("symbols = %+v, want modified Get", file.Symbols)
	}
}

func TestMovedSymbolTargetReportsTargetRenamed(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	body := "package main\n\nfunc Run() string { return \"ok\" }\n"
	writeTestFile(t, projectRoot, "old_worker.go", body)
	target := bindingTargetForSnapshot(t, projectRoot, "old_worker.go", "Run", "")
	dec := createTestDecision(t, store, "dec-binding-moved-symbol", "Moved symbol target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "old_worker.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:    dec.Meta.ID,
		BindingTargets: []BindingTarget{target},
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(projectRoot, "old_worker.go")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, projectRoot, "new_worker.go", body)

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	var file DriftItem
	for _, candidate := range reports[0].Files {
		if candidate.Path == "old_worker.go" {
			file = candidate
			break
		}
	}
	if file.Path == "" {
		t.Fatalf("files = %+v, want missing old_worker.go item", reports[0].Files)
	}
	if file.Status != DriftMissing {
		t.Fatalf("status = %q, want missing original file", file.Status)
	}
	if file.Materiality != DriftMaterialityMaterialSymbol {
		t.Fatalf("materiality = %q, want %q", file.Materiality, DriftMaterialityMaterialSymbol)
	}
	if file.TargetStatus != "renamed" {
		t.Fatalf("target_status = %q, want renamed", file.TargetStatus)
	}
	if file.ChangedTargetRef != "symbol:new_worker.go::func:Run" {
		t.Fatalf("changed_target_ref = %q, want moved symbol target", file.ChangedTargetRef)
	}

	eventReport := BuildDriftEventReport(reports)
	if len(eventReport.Events) == 0 {
		t.Fatalf("events = %+v, want moved-target event", eventReport.Events)
	}
	var event DriftEvent
	for _, candidate := range eventReport.Events {
		if candidate.ChangedTargetRef == "symbol:new_worker.go::func:Run" {
			event = candidate
			break
		}
	}
	if event.ChangedTargetRef == "" {
		t.Fatalf("events = %+v, want moved target event", eventReport.Events)
	}
	if event.RootCause != DriftEventRootCauseTargetRenamed {
		t.Fatalf("root_cause = %q, want %q", event.RootCause, DriftEventRootCauseTargetRenamed)
	}
	if event.TargetStatus != "renamed" {
		t.Fatalf("event target_status = %q, want renamed", event.TargetStatus)
	}
	if event.ResolutionStatus != DriftEventResolutionNeedsOperatorJudgment {
		t.Fatalf("resolution_status = %q, want %q", event.ResolutionStatus, DriftEventResolutionNeedsOperatorJudgment)
	}
}

func TestEditedMovedSymbolTargetReportsRetargetCandidate(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	oldBody := "package main\n\nfunc Run() string { return \"old\" }\n"
	newBody := "package main\n\nfunc Run() string { return \"new\" }\n"
	writeTestFile(t, projectRoot, "old_worker.go", oldBody)
	target := bindingTargetForSnapshot(t, projectRoot, "old_worker.go", "Run", "")
	dec := createTestDecision(t, store, "dec-binding-edited-moved-symbol", "Edited moved symbol target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "old_worker.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:    dec.Meta.ID,
		BindingTargets: []BindingTarget{target},
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(projectRoot, "old_worker.go")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, projectRoot, "new_worker.go", newBody)

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	var file DriftItem
	for _, candidate := range reports[0].Files {
		if candidate.Path == "old_worker.go" {
			file = candidate
			break
		}
	}
	if file.Path == "" {
		t.Fatalf("files = %+v, want missing old_worker.go item", reports[0].Files)
	}
	if file.Materiality != DriftMaterialityNeedsBindingResolution {
		t.Fatalf("materiality = %q, want %q", file.Materiality, DriftMaterialityNeedsBindingResolution)
	}
	if file.TargetStatus != "retarget_candidate" {
		t.Fatalf("target_status = %q, want retarget_candidate", file.TargetStatus)
	}
	if file.FallbackKind != "edited_symbol_move_candidate" {
		t.Fatalf("fallback_kind = %q, want edited_symbol_move_candidate", file.FallbackKind)
	}
	if file.ChangedTargetRef != "symbol:new_worker.go::func:Run" {
		t.Fatalf("changed_target_ref = %q, want edited moved symbol target", file.ChangedTargetRef)
	}

	eventReport := BuildDriftEventReport(reports)
	var event DriftEvent
	for _, candidate := range eventReport.Events {
		if candidate.ChangedTargetRef == "symbol:new_worker.go::func:Run" {
			event = candidate
			break
		}
	}
	if event.ChangedTargetRef == "" {
		t.Fatalf("events = %+v, want retarget candidate event", eventReport.Events)
	}
	if event.RootCause != DriftEventRootCauseRetargetCandidate {
		t.Fatalf("root_cause = %q, want %q", event.RootCause, DriftEventRootCauseRetargetCandidate)
	}
	if event.ResolutionStatus != DriftEventResolutionNeedsOperatorJudgment {
		t.Fatalf("resolution_status = %q, want %q", event.ResolutionStatus, DriftEventResolutionNeedsOperatorJudgment)
	}
	if event.TargetStatus != "retarget_candidate" {
		t.Fatalf("event target_status = %q, want retarget_candidate", event.TargetStatus)
	}
}

func TestFuzzyEditedMovedSymbolTargetReportsRetargetCandidate(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	oldBody := "package main\n\ntype OldWorker struct{}\nfunc (w OldWorker) Run() string { return \"old\" }\n"
	newBody := "package main\n\nfunc Run() string { return \"new\" }\n"
	writeTestFile(t, projectRoot, "old_worker.go", oldBody)
	target := bindingTargetForSnapshot(t, projectRoot, "old_worker.go", "Run", "OldWorker")
	dec := createTestDecision(t, store, "dec-binding-fuzzy-edited-moved-symbol", "Fuzzy edited moved symbol target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "old_worker.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:    dec.Meta.ID,
		BindingTargets: []BindingTarget{target},
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(projectRoot, "old_worker.go")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, projectRoot, "new_worker.go", newBody)

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityNeedsBindingResolution {
		t.Fatalf("materiality = %q, want %q", file.Materiality, DriftMaterialityNeedsBindingResolution)
	}
	if file.FallbackKind != "fuzzy_edited_symbol_move_candidate" {
		t.Fatalf("fallback_kind = %q, want fuzzy_edited_symbol_move_candidate", file.FallbackKind)
	}
	if file.ChangedTargetRef != "symbol:new_worker.go::func:Run" {
		t.Fatalf("changed_target_ref = %q, want fuzzy moved symbol target", file.ChangedTargetRef)
	}
	if file.TargetStatus != "retarget_candidate" {
		t.Fatalf("target_status = %q, want retarget_candidate", file.TargetStatus)
	}
}

func TestFuzzyEditedMovedSymbolTargetRejectsAmbiguousCandidates(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	oldBody := "package main\n\ntype OldWorker struct{}\nfunc (w OldWorker) Run() string { return \"old\" }\n"
	writeTestFile(t, projectRoot, "old_worker.go", oldBody)
	target := bindingTargetForSnapshot(t, projectRoot, "old_worker.go", "Run", "OldWorker")
	dec := createTestDecision(t, store, "dec-binding-ambiguous-fuzzy-edited-moved-symbol", "Ambiguous fuzzy edited moved symbol target")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "old_worker.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:    dec.Meta.ID,
		BindingTargets: []BindingTarget{target},
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(projectRoot, "old_worker.go")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, projectRoot, "new_worker.go", "package main\n\nfunc Run() string { return \"new\" }\n")
	writeTestFile(t, projectRoot, "other_worker.go", "package main\n\ntype OtherWorker struct{}\nfunc (w OtherWorker) Run() string { return \"other\" }\n")

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("reports = %+v, want one drift report", reports)
	}
	file := reports[0].Files[0]
	if file.FallbackKind == "fuzzy_edited_symbol_move_candidate" {
		t.Fatalf("ambiguous candidates must not produce fuzzy retarget: %+v", file)
	}
	if file.ChangedTargetRef != "" {
		t.Fatalf("changed_target_ref = %q, want empty when fuzzy candidates are ambiguous", file.ChangedTargetRef)
	}
}

func TestImplementationFootprintOnlyDoesNotCreateDecisionDrift(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "build.go", "package main\nfunc Build() string { return \"old\" }\n")
	hash, err := hashFile(projectRoot + "/build.go")
	if err != nil {
		t.Fatal(err)
	}

	dec := createTestDecision(t, store, "dec-footprint-only", "Footprint only")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "build.go", Hash: hash}}); err != nil {
		t.Fatal(err)
	}
	fields := dec.UnmarshalDecisionFields()
	fields.ImplementationFootprint = ImplementationFootprint{Files: []string{"build.go"}}
	if err := persistDecisionFields(ctx, store, dec, fields); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "build.go", "package main\nfunc Build() string { return \"new\" }\n")

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("reports = %+v, want no drift for implementation-footprint-only file", reports)
	}
}

func TestDriftWatchTargetsTakePrecedenceOverLegacyWholeFileFallback(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "store.go", `package main

type SQLiteBaselineStore struct{}
func (s SQLiteBaselineStore) Get() string { return "sqlite" }

type MemoryBaselineStore struct{}
func (s MemoryBaselineStore) Get() string { return "memory" }
`)

	target := bindingTargetForSnapshot(t, projectRoot, "store.go", "Get", "MemoryBaselineStore")
	hash, err := hashFile(projectRoot + "/store.go")
	if err != nil {
		t.Fatal(err)
	}
	dec := createTestDecision(t, store, "dec-watch-target-precedence", "Watch target precedence")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "store.go", Hash: hash}}); err != nil {
		t.Fatal(err)
	}
	fields := dec.UnmarshalDecisionFields()
	fields.BindingTargets = []BindingTarget{{
		Kind:            BindingTargetWholeFileFallback,
		FilePath:        "store.go",
		WhySymbolFailed: "legacy file-scope binding",
	}}
	fields.DriftWatchTargets = []DriftWatchTarget{{
		TargetRef:     "symbol:store.go::MemoryBaselineStore.Get",
		Trigger:       "symbol_body_changed",
		BindingTarget: &target,
	}}
	if err := persistDecisionFields(ctx, store, dec, fields); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "store.go", `package main

type SQLiteBaselineStore struct{}
func (s SQLiteBaselineStore) Get() string { return "sqlite changed" }

type MemoryBaselineStore struct{}
func (s MemoryBaselineStore) Get() string { return "memory" }
`)

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Files) != 1 {
		t.Fatalf("reports = %+v, want one audit-only drift report", reports)
	}
	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn || !file.AuditOnly {
		t.Fatalf("file = %+v, want drift_watch_targets to override legacy whole-file fallback", file)
	}
}

func bindingTargetForSnapshot(t *testing.T, projectRoot, filePath, symbolName, receiver string) BindingTarget {
	t.Helper()

	snapshots, err := codebase.ExtractSymbolSnapshots(projectRoot, filePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range snapshots {
		if snapshot.SymbolName != symbolName || snapshot.Receiver != receiver {
			continue
		}
		return BindingTarget{
			Kind:             BindingTargetSymbol,
			FilePath:         snapshot.FilePath,
			Language:         "go",
			SymbolName:       snapshot.SymbolName,
			SymbolKind:       snapshot.SymbolKind,
			Receiver:         snapshot.Receiver,
			Line:             snapshot.Line,
			EndLine:          snapshot.EndLine,
			BodyHash:         snapshot.Hash,
			Confidence:       "high",
			ResolutionSource: "test_selection",
		}
	}
	t.Fatalf("missing symbol %s receiver %s in %s", symbolName, receiver, filePath)
	return BindingTarget{}
}

func semanticTargetForStableFile(t *testing.T, projectRoot, filePath, kind, targetRef string) BindingTarget {
	t.Helper()

	snapshot, err := codebase.ExtractStableFileRange(projectRoot, filePath)
	if err != nil {
		t.Fatal(err)
	}
	return BindingTarget{
		Kind:       kind,
		TargetRef:  targetRef,
		FilePath:   filePath,
		Language:   snapshot.Language,
		Line:       snapshot.StartLine,
		EndLine:    snapshot.EndLine,
		AnchorHash: snapshot.AnchorHash,
		TextHash:   snapshot.TextHash,
		Confidence: "medium",
	}
}

func bindingLanguageMatrixHas(matrix []codebase.BindingLanguageSupport, language string) bool {
	for _, entry := range matrix {
		if entry.Language == language {
			return true
		}
	}
	return false
}
