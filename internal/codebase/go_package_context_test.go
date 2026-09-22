package codebase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestPreparedGoPackagePreservesPointEdgesAndAdmittedBytes(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	writeTestFile(t, root, "types.go", `package sample
type Runner interface { Run() }
type Worker struct{}
func (Worker) Run() {}
func NewWorker() Worker { return Worker{} }
`)
	writeTestFile(t, root, "calls.go", `package sample
func Call() { worker := NewWorker(); worker.Run() }
func Dispatch(runner Runner) { runner.Run() }
`)
	scanner := NewScanner(store.db)
	refreshed, err := scanner.RefreshIncremental(ctx, root)
	if err != nil || !refreshed.Published {
		t.Fatalf("initial refresh = %#v, %v", refreshed, err)
	}
	corpus, err := scanner.scanCurrentCodeCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	files := []string{"calls.go", "types.go"}
	snapshot := newProjectIndexSnapshot(corpus.admissions, buildTSProjectResolution(root))
	prepared, err := prepareGoPackageContexts(ctx, root, files, store, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &GoLang{}
	source := corpus.admissions["calls.go"]
	expected, err := resolver.ResolveAdmittedFileEdges(ctx, root, source, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) == 0 {
		t.Fatal("fixture resolved no edges")
	}
	// Prepared resolution must use the admitted package, even if its carrier
	// changes before the per-file worker consumes it.
	if err := os.Remove(filepath.Join(root, "types.go")); err != nil {
		t.Fatal(err)
	}
	actual, err := resolver.ResolveAdmittedFileEdgeOutcomesWithProjectSnapshot(ctx, root, source, store, prepared)
	if err != nil {
		t.Fatal(err)
	}
	expectedKeys := goPackageEdgeKeys(expected)
	actualEdges := make([]CodeEdge, 0, len(actual))
	for _, outcome := range actual {
		resolved, ok := outcome.(ResolvedEdge)
		if !ok {
			t.Fatalf("unexpected outcome: %#v", outcome)
		}
		actualEdges = append(actualEdges, resolved.Edge)
	}
	actualKeys := goPackageEdgeKeys(actualEdges)
	if !reflect.DeepEqual(expectedKeys, actualKeys) {
		t.Fatalf("prepared edges = %#v, point edges = %#v", actualKeys, expectedKeys)
	}
	// A later batch must discard the previous package facts after deletion.
	changed, err := scanner.RefreshIncremental(ctx, root)
	if err != nil || !changed.Published || changed.Epoch != refreshed.Epoch+1 {
		t.Fatalf("deleted package source refresh = %#v, %v", changed, err)
	}
	remaining, err := store.GetByName(ctx, "Worker")
	if err != nil || len(remaining) != 0 {
		t.Fatalf("deleted declaration survived: %#v, %v", remaining, err)
	}
}

func goPackageEdgeKeys(edges []CodeEdge) []string {
	keys := make([]string, 0, len(edges))
	for _, edge := range edges {
		key := edge.SrcID + "\x00" + edge.DstID + "\x00" + string(edge.Kind)
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestResolveIndexFileHonorsCancellationBeforeParsing(t *testing.T) {
	store, root := newSymbolStore(t)
	writeTestFile(t, root, "sample.go", "package sample\nfunc Call() {}\n")
	registry := NewRegistry()
	source, err := registry.ReadAdmittedSource(root, "sample.go")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scanner := NewScanner(store.db)
	_, err = scanner.resolveIndexFile(ctx, root, source, store, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled resolver = %v", err)
	}
}
