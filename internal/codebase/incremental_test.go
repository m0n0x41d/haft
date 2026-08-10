package codebase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRefreshIncrementalPublishesOnlyChangedClosure(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "app/main.ts", `import { value } from "../lib/value"
export function run(): number { return value() }
`)
	writeTestFile(t, root, "lib/value.ts", `export function value(): number { return 1 }
`)
	writeTestFile(t, root, "other/alone.ts", `export function alone(): number { return 1 }
`)

	scanner := NewScanner(store.db)
	first, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Published || !first.FullRebuild || first.Epoch != 1 || first.ChangedFiles != 3 {
		t.Fatalf("first refresh = %+v", first)
	}
	assertRowCount(t, store, "code_files", 3)
	state, err := scanner.CurrentIndexState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Basis.Coverage.Posture != IndexCoverageComplete ||
		state.Basis.Coverage.DiscoveredFiles != 3 ||
		!state.SupportsKnownAbsence() {
		t.Fatalf("published complete basis = %+v", state)
	}

	warm, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if warm.Published || warm.Epoch != 1 {
		t.Fatalf("unchanged refresh = %+v", warm)
	}

	writeTestFile(t, root, "lib/value.ts", `export function value(): number { return 2 }
export function added(): number { return 3 }
`)
	changed, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !changed.Published || changed.FullRebuild || changed.Epoch != 2 {
		t.Fatalf("incremental refresh = %+v", changed)
	}
	if changed.ChangedFiles >= first.ChangedFiles {
		t.Fatalf("incremental closure scanned %d files; full corpus has %d", changed.ChangedFiles, first.ChangedFiles)
	}
	if symbols, err := store.GetByName(ctx, "added"); err != nil || len(symbols) != 1 {
		t.Fatalf("added symbol missing: symbols=%+v err=%v", symbols, err)
	}

	if err := os.Remove(filepath.Join(root, "lib", "value.ts")); err != nil {
		t.Fatal(err)
	}
	deleted, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.Published || deleted.Epoch != 3 {
		t.Fatalf("delete refresh = %+v", deleted)
	}
	if symbols, err := store.GetByName(ctx, "value"); err != nil || len(symbols) != 0 {
		t.Fatalf("deleted symbol survived: symbols=%+v err=%v", symbols, err)
	}
}

func TestIncrementalClosureReachesBarrelImporters(t *testing.T) {
	current := map[string]CodeFileState{
		"app/main.ts":    {},
		"lib/index.ts":   {},
		"lib/value.ts":   {},
		"other/alone.ts": {},
	}
	imports := []CodeImport{
		{SourceFile: "app/main.ts", TargetBase: "lib/index"},
		{SourceFile: "lib/index.ts", TargetBase: "lib/value"},
	}
	got := incrementalReindexClosure(current, imports, []string{"lib/value.ts"}, nil)
	want := map[string]bool{"app/main.ts": true, "lib/index.ts": true, "lib/value.ts": true}
	if len(got) != len(want) {
		t.Fatalf("closure = %v", got)
	}
	for _, path := range got {
		if !want[path] {
			t.Fatalf("unexpected closure member %q in %v", path, got)
		}
	}
}

func TestIncrementalClosureDoesNotFanOutJSTSSiblings(t *testing.T) {
	current := map[string]CodeFileState{
		"app/main.ts": {
			Language: "typescript",
		},
		"feature/changed.ts": {
			Language: "typescript",
		},
		"feature/unrelated-a.ts": {
			Language: "typescript",
		},
		"feature/unrelated-b.vue": {
			Language: "vue",
		},
	}
	imports := []CodeImport{
		{SourceFile: "app/main.ts", TargetBase: "feature/changed"},
	}

	got := incrementalReindexClosure(
		current,
		imports,
		[]string{"feature/changed.ts"},
		nil,
	)
	want := []string{"app/main.ts", "feature/changed.ts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSTS closure = %v, want %v", got, want)
	}
}

func TestIncrementalClosureRetainsDirectoryScopedResolvers(t *testing.T) {
	tests := []struct {
		name     string
		changed  string
		deleted  bool
		current  map[string]CodeFileState
		expected []string
	}{
		{
			name:    "Go package",
			changed: "pkg/changed.go",
			current: map[string]CodeFileState{
				"pkg/changed.go": {Language: "go"},
				"pkg/caller.go":  {Language: "go"},
				"pkg/unrelated.ts": {
					Language: "typescript",
				},
			},
			expected: []string{"pkg/caller.go", "pkg/changed.go"},
		},
		{
			name:    "deleted Python module",
			changed: "pkg/changed.py",
			deleted: true,
			current: map[string]CodeFileState{
				"pkg/caller.py": {Language: "python"},
				"pkg/unrelated.js": {
					Language: "javascript",
				},
			},
			expected: []string{"pkg/caller.py"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := []string{test.changed}
			var deleted []string
			if test.deleted {
				changed = nil
				deleted = []string{test.changed}
			}
			got := incrementalReindexClosure(
				test.current,
				nil,
				changed,
				deleted,
			)
			if !reflect.DeepEqual(got, test.expected) {
				t.Fatalf("closure = %v, want %v", got, test.expected)
			}
		})
	}
}

func TestRefreshIncrementalProfilesExactJSTSClosure(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "feature/changed.ts", `export function changed(): number { return 1 }
`)
	writeTestFile(t, root, "feature/unrelated-a.ts", `export function unrelatedA(): number { return 1 }
`)
	writeTestFile(t, root, "feature/unrelated-b.ts", `export function unrelatedB(): number { return 1 }
`)

	scanner := NewScanner(store.db)
	if _, err := scanner.RefreshIncremental(ctx, root); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "feature/changed.ts", `export function changed(): number { return 2 }
`)

	result, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published || result.FullRebuild || result.ChangedFiles != 1 {
		t.Fatalf("incremental refresh = %+v", result)
	}
	metrics := result.Metrics
	if metrics.DiscoveredFiles != 3 ||
		metrics.AdmittedFiles != 3 ||
		metrics.ObservedBytes < 1 ||
		metrics.SeedFiles != 1 ||
		metrics.ReindexFiles != 1 ||
		metrics.ResolveFiles != 1 ||
		metrics.ResolveWorkers != 1 ||
		metrics.TotalDuration <= 0 {
		t.Fatalf("refresh metrics = %+v", metrics)
	}
}

func TestScannerResolveWorkerLimitIsThermalSafe(t *testing.T) {
	scanner := NewScanner(nil)
	limit := scanner.resolveWorkerLimit()
	if limit.Value() != defaultMaxResolveWorkers {
		t.Fatalf(
			"resolve worker limit = %d, want %d",
			limit.Value(),
			defaultMaxResolveWorkers,
		)
	}
	if workers := indexResolveWorkerCount(100, limit); workers > 2 {
		t.Fatalf("active resolve workers = %d, want at most 2", workers)
	}
	if DefaultIndexBudget().MaxParseWorkers().Value() == limit.Value() {
		t.Fatal("runtime thermal limit must remain outside persisted index budget")
	}
}

func TestRefreshIncrementalUpdatesPreparedBarrelModel(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "app/main.ts", `import { selected } from "../lib"
export function run(): number { return selected() }
`)
	writeTestFile(t, root, "lib/index.ts", `export { value as selected } from "./value"
`)
	writeTestFile(t, root, "lib/value.ts", `export function value(): number { return 1 }
`)
	writeTestFile(t, root, "lib/other.ts", `export function other(): number { return 2 }
`)

	scanner := NewScanner(store.db)
	if _, err := scanner.RefreshIncremental(ctx, root); err != nil {
		t.Fatal(err)
	}
	assertIncrementalCallTargets(t, store, "run", []string{"value"})

	writeTestFile(t, root, "lib/index.ts", `export { other as selected } from "./other"
`)
	changed, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !changed.Published || changed.FullRebuild {
		t.Fatalf("barrel refresh = %+v", changed)
	}
	assertIncrementalCallTargets(t, store, "run", []string{"other"})

	if err := os.Remove(filepath.Join(root, "lib", "other.ts")); err != nil {
		t.Fatal(err)
	}
	deleted, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.Published || deleted.FullRebuild {
		t.Fatalf("barrel target delete = %+v", deleted)
	}
	assertIncrementalCallTargets(t, store, "run", nil)
}

func TestRefreshIncrementalConfigChangeForcesFullRebuild(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "src/a.ts", `export function a(): number { return 1 }
`)
	writeTestFile(t, root, "src/b.ts", `export function b(): number { return 2 }
`)
	writeTestFile(t, root, "package.json", `{"name":"fixture","exports":"./src/a.ts"}`)
	scanner := NewScanner(store.db)
	if _, err := scanner.RefreshIncremental(ctx, root); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "package.json", `{"name":"fixture","exports":"./src/b.ts"}`)
	result, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published || !result.FullRebuild || result.ChangedFiles != 2 {
		t.Fatalf("config refresh = %+v", result)
	}
}

func TestRefreshIncrementalBuildsOneProjectSnapshotAndSkipsNodeModules(t *testing.T) {
	store, root := newSymbolStore(t)
	tsResolutionCache.Delete(root)
	t.Cleanup(func() { tsResolutionCache.Delete(root) })
	ctx := context.Background()
	writeTestFile(t, root, "src/a.ts", `export function a(): number { return 1 }
`)
	writeTestFile(t, root, "src/b.ts", `import { a } from "./a"
export function b(): number { return a() }
`)
	for index := 0; index < 50; index++ {
		path := filepath.Join("node_modules", "dependency", "src", fmt.Sprintf("dep-%d.ts", index))
		writeTestFile(t, root, path, `export function dependency(): number { return 1 }
`)
	}
	writeTestFile(t, root, "node_modules/dependency/package.json", `{"name":"dependency"}`)

	scanner := NewScanner(store.db)
	snapshotBuilds := 0
	var snapshotFiles []string
	scanner.projectSnapshotFactory = func(
		sources map[string]AdmittedSource,
		resolution tsProjectResolution,
	) *projectIndexSnapshot {
		snapshotBuilds++
		snapshotFiles = make([]string, 0, len(sources))
		for path := range sources {
			snapshotFiles = append(snapshotFiles, path)
		}
		sort.Strings(snapshotFiles)
		return newProjectIndexSnapshot(sources, resolution)
	}
	first, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Published || first.ChangedFiles != 2 {
		t.Fatalf("first refresh = %+v", first)
	}
	if snapshotBuilds != 1 {
		t.Fatalf("project snapshots = %d, want one per published refresh", snapshotBuilds)
	}
	if _, cached := tsResolutionCache.Load(root); cached {
		t.Fatal("snapshot-backed resolution populated the per-file TypeScript resolution cache")
	}
	for _, path := range snapshotFiles {
		if strings.HasPrefix(path, "node_modules/") {
			t.Fatalf("dependency source entered project snapshot: %q", path)
		}
	}
	assertRowCount(t, store, "code_files", 2)

	writeTestFile(t, root, "node_modules/dependency/src/dep-0.ts", `export function changedDependency(): number { return 2 }
`)
	writeTestFile(t, root, "node_modules/dependency/package.json", `{"name":"dependency","version":"2.0.0"}`)
	warm, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if warm.Published || warm.Epoch != first.Epoch {
		t.Fatalf("dependency-only edit changed project epoch: %+v", warm)
	}
	if snapshotBuilds != 1 {
		t.Fatalf("dependency-only edit rebuilt project snapshot: %d", snapshotBuilds)
	}
}

func TestRefreshIncrementalKeepsCompleteEpochAcrossParseFailure(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "Example.vue", `<script setup lang="ts">
function stable(): number { return 1 }
</script>
`)
	scanner := NewScanner(store.db)
	first, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "Example.vue", `<script setup lang="ts">
function broken(: number { return 2 }
`)
	failed, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !failed.Degraded || failed.Published || failed.Epoch != first.Epoch {
		t.Fatalf("failed refresh = %+v after %+v", failed, first)
	}
	state, err := scanner.CurrentIndexState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Degraded || state.Epoch != first.Epoch {
		t.Fatalf("degraded state = %+v", state)
	}
	if state.Basis.Epoch != first.Epoch ||
		state.Basis.BasisDigest == "" ||
		state.SupportsKnownAbsence() {
		t.Fatalf("degraded current basis = %+v", state)
	}
	if symbols, err := store.GetByName(ctx, "stable"); err != nil || len(symbols) != 1 {
		t.Fatalf("last complete epoch was not retained: symbols=%+v err=%v", symbols, err)
	}

	writeTestFile(t, root, "Example.vue", `<script setup lang="ts">
function recovered(): number { return 3 }
</script>
`)
	recovered, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !recovered.Published || recovered.Degraded || recovered.Epoch != first.Epoch+1 {
		t.Fatalf("recovery refresh = %+v", recovered)
	}
	state, err = scanner.CurrentIndexState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Degraded || state.Epoch != recovered.Epoch {
		t.Fatalf("recovered state = %+v", state)
	}
}

func TestRefreshIncrementalPersistsOversizedAsSkippedNotEmpty(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "stable.go", "package sample\nfunc Stable() {}\n")
	writeTestFile(
		t,
		root,
		"large.go",
		strings.Repeat("x", int(defaultMaxFileBytes)+1),
	)

	scanner := NewScanner(store.db)
	result, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published {
		t.Fatalf("refresh = %+v, want published partial corpus", result)
	}
	var status string
	err = store.db.QueryRowContext(
		ctx,
		`SELECT parse_status FROM code_files WHERE file_path = 'large.go'`,
	).Scan(&status)
	if err != nil {
		t.Fatal(err)
	}
	if status != "skipped:oversized" {
		t.Fatalf("large.go status = %q, want skipped:oversized", status)
	}
	state, err := scanner.CurrentIndexState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Basis.Coverage.Posture !=
		IndexCoverageBoundedWithExclusions ||
		state.Basis.Coverage.SkippedFiles != 1 ||
		len(state.Basis.Exclusions) != 1 ||
		state.SupportsKnownAbsence() {
		t.Fatalf("oversized coverage basis = %+v", state)
	}
	if symbols, err := store.GetByFile(ctx, "large.go"); err != nil ||
		len(symbols) != 0 {
		t.Fatalf("oversized symbols = %#v err=%v", symbols, err)
	}

	writeTestFile(t, root, "large.go", "package sample\nfunc Recovered() {}\n")
	recovered, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !recovered.Published || recovered.Epoch != result.Epoch+1 {
		t.Fatalf("recovery refresh = %+v after %+v", recovered, result)
	}
	if symbols, err := store.GetByName(ctx, "Recovered"); err != nil ||
		len(symbols) != 1 {
		t.Fatalf("recovered symbols = %#v err=%v", symbols, err)
	}
}

func TestRefreshIncrementalPersistsInvalidEncodingAsSkipped(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	path := filepath.Join(root, "invalid.go")
	if err := os.WriteFile(path, []byte{0xff, 0xfe}, 0o644); err != nil {
		t.Fatal(err)
	}
	scanner := NewScanner(store.db)
	result, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published {
		t.Fatalf("refresh = %+v, want published exclusion record", result)
	}
	var status string
	err = store.db.QueryRowContext(
		ctx,
		`SELECT parse_status FROM code_files WHERE file_path = 'invalid.go'`,
	).Scan(&status)
	if err != nil {
		t.Fatal(err)
	}
	if status != "skipped:invalid_encoding" {
		t.Fatalf(
			"invalid.go status = %q, want skipped:invalid_encoding",
			status,
		)
	}
	if symbols, err := store.GetByFile(ctx, "invalid.go"); err != nil ||
		len(symbols) != 0 {
		t.Fatalf("invalid-encoding symbols = %#v err=%v", symbols, err)
	}
}

func TestRefreshIncrementalRetainsUnavailableBasisForMissingRoot(t *testing.T) {
	store, root := newSymbolStore(t)
	missing := filepath.Join(root, "does-not-exist")
	scanner := NewScanner(store.db)
	result, err := scanner.RefreshIncremental(
		context.Background(),
		missing,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Degraded ||
		result.Published ||
		result.Epoch != 0 {
		t.Fatalf("missing-root result = %+v", result)
	}
	state, err := scanner.CurrentIndexState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !state.Degraded ||
		state.Epoch != 0 ||
		state.Basis.Coverage.Posture != IndexCoverageUnavailable ||
		state.SupportsKnownAbsence() {
		t.Fatalf("missing-root state = %+v", state)
	}
}

func TestRefreshIncrementalRetainsPriorEpochForMissingRoot(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "stable.go", "package sample\nfunc Stable() {}\n")
	scanner := NewScanner(store.db)
	first, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := scanner.CurrentIndexState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := scanner.RefreshIncremental(
		ctx,
		filepath.Join(root, "does-not-exist"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Degraded ||
		result.Published ||
		result.Epoch != first.Epoch {
		t.Fatalf("missing-root refresh = %+v", result)
	}
	after, err := scanner.CurrentIndexState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.Epoch != before.Epoch ||
		after.Basis.BasisDigest != before.Basis.BasisDigest ||
		!after.Degraded ||
		after.SupportsKnownAbsence() {
		t.Fatalf("prior basis was not retained: before=%+v after=%+v", before, after)
	}
	if symbols, err := store.GetByName(
		ctx,
		"Stable",
	); err != nil || len(symbols) != 1 {
		t.Fatalf("prior graph is not usable: symbols=%+v err=%v", symbols, err)
	}
}

func TestRootAdmissionBudgetRetainsPriorEpoch(t *testing.T) {
	tests := map[string]struct {
		firstSource  string
		secondSource string
		budget       func(*testing.T) IndexBudget
		reason       string
	}{
		"file count": {
			firstSource:  "package sample\nfunc A() {}\n",
			secondSource: "package sample\nfunc B() {}\n",
			budget: func(t *testing.T) IndexBudget {
				return incrementalTestBudget(t, 100, 1, 200)
			},
			reason: "root admitted-file budget is exhausted",
		},
		"observed bytes": {
			firstSource: "package sample\n// " +
				strings.Repeat("a", 40) +
				"\nfunc A() {}\n",
			secondSource: "package sample\n// " +
				strings.Repeat("b", 40) +
				"\nfunc B() {}\n",
			budget: func(t *testing.T) IndexBudget {
				return incrementalTestBudget(t, 100, 4, 100)
			},
			reason: "root admitted-byte budget is exhausted",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			store, root := newSymbolStore(t)
			ctx := context.Background()
			writeTestFile(t, root, "a.go", test.firstSource)
			scanner := NewScanner(store.db)
			first, err := scanner.RefreshIncremental(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			before, err := scanner.CurrentIndexState(ctx)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, root, "b.go", test.secondSource)
			scanner.indexBudget = test.budget(t)
			result, err := scanner.RefreshIncremental(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Degraded ||
				result.Published ||
				result.Epoch != first.Epoch ||
				!strings.Contains(result.Reason, test.reason) {
				t.Fatalf("budget result = %+v", result)
			}
			after, err := scanner.CurrentIndexState(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if after.Epoch != before.Epoch ||
				after.Basis.BasisDigest != before.Basis.BasisDigest ||
				after.SupportsKnownAbsence() {
				t.Fatalf(
					"budget failure changed current basis: before=%+v after=%+v",
					before,
					after,
				)
			}
		})
	}
}

func incrementalTestBudget(
	t *testing.T,
	maxFileBytes int64,
	maxFiles int64,
	maxObservedBytes int64,
) IndexBudget {
	t.Helper()
	fileBytes, err := NewByteCount(maxFileBytes)
	if err != nil {
		t.Fatal(err)
	}
	files, err := NewFileCount(maxFiles)
	if err != nil {
		t.Fatal(err)
	}
	observedBytes, err := NewByteCount(maxObservedBytes)
	if err != nil {
		t.Fatal(err)
	}
	workers, err := NewWorkerCount(1)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := ParseGeneratedSourcePolicy("include_generated")
	if err != nil {
		t.Fatal(err)
	}
	budget, err := NewIndexBudget(IndexBudgetSpec{
		MaxFileBytes:     fileBytes,
		MaxFiles:         files,
		MaxObservedBytes: observedBytes,
		MaxParseWorkers:  workers,
		GeneratedSources: generated,
	})
	if err != nil {
		t.Fatal(err)
	}
	return budget
}

func TestRefreshIncrementalDoesNotFollowSymlinkCycle(t *testing.T) {
	store, root := newSymbolStore(t)
	writeTestFile(t, root, "only.go", "package sample\nfunc Only() {}\n")
	if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	result, err := NewScanner(store.db).RefreshIncremental(
		context.Background(),
		root,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published || result.ChangedFiles != 1 {
		t.Fatalf("symlink-cycle refresh = %+v", result)
	}
	assertRowCount(t, store, "code_files", 1)
}

func TestRefreshIncrementalCancellationKeepsCurrentEpoch(t *testing.T) {
	store, root := newSymbolStore(t)
	writeTestFile(t, root, "stable.go", "package sample\nfunc Stable() {}\n")
	scanner := NewScanner(store.db)
	first, err := scanner.RefreshIncremental(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "stable.go", "package sample\nfunc Changed() {}\n")
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := scanner.RefreshIncremental(cancelled, root); err == nil {
		t.Fatal("cancelled refresh must not publish")
	}
	state, err := scanner.CurrentIndexState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Epoch != first.Epoch {
		t.Fatalf("cancelled refresh changed epoch: %+v after %+v", state, first)
	}
	if symbols, err := store.GetByName(
		context.Background(),
		"Stable",
	); err != nil || len(symbols) != 1 {
		t.Fatalf(
			"last complete symbols lost: symbols=%+v err=%v",
			symbols,
			err,
		)
	}
}

func TestIndexPublicationFailureRollsBackCandidateAndBasis(t *testing.T) {
	for _, stage := range []string{
		"after_symbols",
		"after_search_rows",
		"after_edges",
		"after_candidate_rows",
		"after_basis_rows",
		"after_current_pointer",
	} {
		t.Run(stage, func(t *testing.T) {
			store, root := newSymbolStore(t)
			ctx := context.Background()
			writeTestFile(
				t,
				root,
				"stable.go",
				"package sample\nfunc Stable() {}\n",
			)
			scanner := NewScanner(store.db)
			first, err := scanner.RefreshIncremental(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			before, err := scanner.CurrentIndexState(ctx)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(
				t,
				root,
				"stable.go",
				"package sample\nfunc Changed() {}\n",
			)
			wantErr := errors.New(
				"simulated publication failure at " + stage,
			)
			scanner.publicationCheckpoint = func(
				_ context.Context,
				_ *sql.Tx,
				reached indexPublicationStage,
			) error {
				if reached.String() == stage {
					return wantErr
				}
				return nil
			}
			if _, err := scanner.RefreshIncremental(
				ctx,
				root,
			); !errors.Is(err, wantErr) {
				t.Fatalf(
					"publication error = %v, want %v",
					err,
					wantErr,
				)
			}
			after, err := scanner.CurrentIndexState(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if after.Epoch != first.Epoch ||
				after.Basis.BasisDigest != before.Basis.BasisDigest {
				t.Fatalf(
					"failed publication changed basis: before=%+v after=%+v",
					before,
					after,
				)
			}
			if symbols, err := store.GetByName(
				ctx,
				"Stable",
			); err != nil || len(symbols) != 1 {
				t.Fatalf(
					"published symbol lost: symbols=%+v err=%v",
					symbols,
					err,
				)
			}
			if symbols, err := store.GetByName(
				ctx,
				"Changed",
			); err != nil || len(symbols) != 0 {
				t.Fatalf(
					"candidate symbol leaked: symbols=%+v err=%v",
					symbols,
					err,
				)
			}
			query, err := NewConcernQuery("Changed")
			if err != nil {
				t.Fatal(err)
			}
			budget, err := NewDiscoveryBudget(5)
			if err != nil {
				t.Fatal(err)
			}
			discovery, err := store.DiscoverSymbols(
				ctx,
				query,
				budget,
				first.Epoch,
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(discovery.Candidates()) != 0 {
				t.Fatalf(
					"candidate search row leaked: %+v",
					discovery.Candidates(),
				)
			}
			assertRowCount(t, store, "code_index_epochs", 1)
		})
	}
}

func TestIndexPublicationCancellationRollsBackCandidate(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "stable.go", "package sample\nfunc Stable() {}\n")
	scanner := NewScanner(store.db)
	first, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "stable.go", "package sample\nfunc Changed() {}\n")
	scanner.publicationCheckpoint = func(
		_ context.Context,
		_ *sql.Tx,
		stage indexPublicationStage,
	) error {
		if stage.String() != "after_edges" {
			return nil
		}
		return context.Canceled
	}
	if _, err := scanner.RefreshIncremental(
		ctx,
		root,
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("publication cancellation = %v", err)
	}
	state, err := scanner.CurrentIndexState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Epoch != first.Epoch {
		t.Fatalf("cancelled publication changed epoch: %+v", state)
	}
	assertRowCount(t, store, "code_index_epochs", 1)
}

func TestIndexBasisSurvivesProcessReopen(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "code-index.db")
	ctx := context.Background()
	writeTestFile(t, root, "stable.go", "package sample\nfunc Stable() {}\n")

	firstDatabase, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	firstStore := NewSymbolStore(firstDatabase)
	if err := firstStore.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	firstScanner := NewScanner(firstDatabase)
	if _, err := firstScanner.RefreshIncremental(ctx, root); err != nil {
		t.Fatal(err)
	}
	before, err := firstScanner.CurrentIndexState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := firstDatabase.Close(); err != nil {
		t.Fatal(err)
	}

	reopenedDatabase, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopenedDatabase.Close() })
	reopenedScanner := NewScanner(reopenedDatabase)
	if err := reopenedScanner.EnsureIncrementalSchema(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := reopenedScanner.CurrentIndexState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.Epoch != before.Epoch ||
		after.Basis.BasisDigest != before.Basis.BasisDigest ||
		after.Basis.CorpusDigest != before.Basis.CorpusDigest ||
		!after.SupportsKnownAbsence() {
		t.Fatalf("reopened basis: before=%+v after=%+v", before, after)
	}
	warm, err := reopenedScanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if warm.Published || warm.Epoch != before.Epoch {
		t.Fatalf("reopened warm refresh = %+v", warm)
	}
}

func TestConcurrentReaderObservesOnlyWholeEpochBasis(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	if _, err := store.db.ExecContext(
		ctx,
		"PRAGMA journal_mode=WAL",
	); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "a.go", "package sample\nfunc A() {}\n")
	writer := NewScanner(store.db)
	first, err := writer.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "b.go", "package sample\nfunc B() {}\n")
	entered := make(chan struct{})
	release := make(chan struct{})
	writer.publicationCheckpoint = func(
		ctx context.Context,
		_ *sql.Tx,
		stage indexPublicationStage,
	) error {
		if stage.String() != "after_current_pointer" {
			return nil
		}
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	refreshResult := make(chan IndexRefreshResult, 1)
	refreshError := make(chan error, 1)
	go func() {
		result, err := writer.RefreshIncremental(ctx, root)
		refreshResult <- result
		refreshError <- err
	}()
	<-entered
	readState := make(chan IndexState, 1)
	readError := make(chan error, 1)
	go func() {
		state, err := NewScanner(store.db).CurrentIndexState(ctx)
		readState <- state
		readError <- err
	}()
	close(release)
	if err := <-refreshError; err != nil {
		t.Fatal(err)
	}
	published := <-refreshResult
	if !published.Published || published.Epoch != first.Epoch+1 {
		t.Fatalf("concurrent publish = %+v", published)
	}
	if err := <-readError; err != nil {
		t.Fatal(err)
	}
	observed := <-readState
	switch observed.Epoch {
	case first.Epoch:
		if observed.Basis.Coverage.DiscoveredFiles != 1 {
			t.Fatalf("mixed old epoch basis = %+v", observed)
		}
	case published.Epoch:
		if observed.Basis.Coverage.DiscoveredFiles != 2 {
			t.Fatalf("mixed new epoch basis = %+v", observed)
		}
	default:
		t.Fatalf("reader observed unknown epoch = %+v", observed)
	}
	if observed.Basis.BasisDigest == "" ||
		observed.Basis.CorpusDigest == "" {
		t.Fatalf("reader observed incomplete basis = %+v", observed)
	}
}

func TestSchemaV4ShapeMigratesIdempotentlyToV5(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	statements := []string{
		`CREATE TABLE code_index_meta (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			fingerprint TEXT NOT NULL
		)`,
		`CREATE TABLE code_files (
			file_path TEXT PRIMARY KEY,
			content_hash TEXT NOT NULL,
			language TEXT NOT NULL DEFAULT '',
			parse_status TEXT NOT NULL,
			index_epoch INTEGER NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE code_imports (
			source_file TEXT NOT NULL,
			target_base TEXT NOT NULL,
			import_kind TEXT NOT NULL DEFAULT 'import',
			index_epoch INTEGER NOT NULL,
			PRIMARY KEY (source_file, target_base, import_kind)
		)`,
		`CREATE TABLE code_index_epochs (
			epoch INTEGER PRIMARY KEY,
			status TEXT NOT NULL,
			full_rebuild INTEGER NOT NULL DEFAULT 0,
			changed_files INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, statement := range statements {
		if _, err := store.db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	scanner := NewScanner(store.db)
	if err := scanner.EnsureIncrementalSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := scanner.EnsureIncrementalSchema(ctx); err != nil {
		t.Fatal(err)
	}
	requiredColumns := map[string][]string{
		"code_index_meta": {
			"current_epoch",
			"config_hash",
			"schema_version",
			"degraded",
			"degraded_reason",
		},
		"code_files": {
			"symbol_count",
		},
		"code_index_epochs": {
			"corpus_digest",
			"basis_digest",
			"coverage_posture",
			"discovered_files",
			"admitted_files",
			"indexed_files",
			"empty_files",
			"skipped_files",
		},
	}
	for table, columns := range requiredColumns {
		assertTableColumns(t, store.db, table, columns)
	}
	writeTestFile(t, root, "stable.go", "package sample\nfunc Stable() {}\n")
	result, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published || result.Epoch != 1 {
		t.Fatalf("migrated initial publish = %+v", result)
	}
}

func TestLegacyEpochWithoutBasisRebuildsAndPreservesStableAnchor(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "stable.go", "package sample\nfunc Stable() {}\n")
	scanner := NewScanner(store.db)
	first, err := scanner.RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := store.GetByName(ctx, "Stable")
	if err != nil || len(symbols) != 1 {
		t.Fatalf("initial Stable = %+v err=%v", symbols, err)
	}
	stableAnchor := symbols[0].ID
	if _, err := store.db.ExecContext(ctx, `
		UPDATE code_index_epochs
		SET corpus_digest = '',
		    basis_digest = '',
		    coverage_posture = 'legacy_unknown'
		WHERE epoch = ?;
		UPDATE code_index_meta
		SET schema_version = 4
		WHERE id = 1`,
		first.Epoch,
	); err != nil {
		t.Fatal(err)
	}
	recovered, err := NewScanner(store.db).RefreshIncremental(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !recovered.Published ||
		!recovered.FullRebuild ||
		recovered.Epoch != first.Epoch+1 {
		t.Fatalf("legacy basis recovery = %+v", recovered)
	}
	symbols, err = store.GetByName(ctx, "Stable")
	if err != nil || len(symbols) != 1 {
		t.Fatalf("recovered Stable = %+v err=%v", symbols, err)
	}
	if symbols[0].ID != stableAnchor {
		t.Fatalf(
			"stable anchor changed across migration: %s -> %s",
			stableAnchor,
			symbols[0].ID,
		)
	}
}

func TestRefreshIncrementalPublishesEmptyCompleteCorpus(t *testing.T) {
	store, root := newSymbolStore(t)
	scanner := NewScanner(store.db)
	result, err := scanner.RefreshIncremental(
		context.Background(),
		root,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published || !result.FullRebuild || result.Epoch != 1 {
		t.Fatalf("empty refresh = %+v", result)
	}
	state, err := scanner.CurrentIndexState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !state.SupportsKnownAbsence() ||
		state.Basis.Coverage.DiscoveredFiles != 0 ||
		state.Basis.CorpusDigest == "" {
		t.Fatalf("empty complete basis = %+v", state)
	}
}

func TestFileIndexDispositionRejectsContradictoryStates(t *testing.T) {
	zero, err := NewFileCount(0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewIndexedFileDisposition(zero); err == nil {
		t.Fatal("indexed with zero symbols must be inexpressible")
	}
	if _, err := NewSkippedFileDisposition(SourceSkipReason{}); err == nil {
		t.Fatal("skipped without a reason must be inexpressible")
	}
	if _, err := NewDegradedFileDisposition(""); err == nil {
		t.Fatal("degraded without a reason must be inexpressible")
	}
	failure, err := NewFileIndexFailure("broken.go", "syntax error")
	if err != nil {
		t.Fatal(err)
	}
	if failure.Disposition.Kind().String() != CodeFileDegraded ||
		failure.Disposition.DetailCode() != "syntax error" {
		t.Fatalf("typed parse failure = %+v", failure)
	}
	empty := NewEmptyFileDisposition()
	if empty.Kind().String() != CodeFileEmpty ||
		empty.StatusCode() != CodeFileEmpty {
		t.Fatalf(
			"empty disposition = %s/%s",
			empty.Kind().String(),
			empty.StatusCode(),
		)
	}
}

func TestParsePersistedFileIndexDispositionNeedsExactSymbolCount(t *testing.T) {
	if _, err := ParsePersistedFileIndexDisposition(
		CodeFileIndexed,
		0,
	); err == nil {
		t.Fatal("persisted indexed file with zero symbols must be rejected")
	}
	indexed, err := ParsePersistedFileIndexDisposition(
		CodeFileIndexed,
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if indexed.Kind().String() != CodeFileIndexed ||
		indexed.DetailCode() != "2" {
		t.Fatalf("persisted indexed disposition = %+v", indexed)
	}
}

func writeTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertRowCount(t *testing.T, store *SymbolStore, table string, want int) {
	t.Helper()
	var got int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}

func assertTableColumns(
	t *testing.T,
	database *sql.DB,
	table string,
	required []string,
) {
	t.Helper()
	rows, err := database.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(
			&cid,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			t.Fatal(err)
		}
		found[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, name := range required {
		if !found[name] {
			t.Fatalf("%s is missing migrated column %s", table, name)
		}
	}
}

func assertIncrementalCallTargets(t *testing.T, store *SymbolStore, sourceName string, want []string) {
	t.Helper()
	ctx := context.Background()
	sources, err := store.GetByName(ctx, sourceName)
	if err != nil || len(sources) != 1 {
		t.Fatalf("source %q = %+v err=%v", sourceName, sources, err)
	}
	edges, err := NewEdgeStore(store.db).OutEdges(ctx, sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	targets := make([]string, 0)
	for _, edge := range edges {
		if edge.Kind != EdgeCall {
			continue
		}
		target, found, err := store.GetByID(ctx, edge.DstID)
		if err != nil {
			t.Fatal(err)
		}
		if found {
			targets = append(targets, target.Name)
		}
	}
	sort.Strings(targets)
	sortedWant := append([]string{}, want...)
	sort.Strings(sortedWant)
	if !reflect.DeepEqual(targets, sortedWant) {
		t.Fatalf("call targets for %q = %v, want %v", sourceName, targets, sortedWant)
	}
}
