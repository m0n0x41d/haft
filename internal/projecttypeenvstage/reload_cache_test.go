package projecttypeenvstage

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSelectionReadyStageCacheCoordinatesConcurrentExactInput(t *testing.T) {
	t.Parallel()

	cache := selectionReadyStageCache{}
	rowsKey := selectionReadyStageRowsCacheKeyForTest("concurrent-rows")
	key := selectionReadyStageCacheKeyForTest("concurrent-full")
	capability := &selectionReadyStageCapability{}
	const callerCount = 32
	var calls atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	builder := func() (SelectionReadyStage, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return SelectionReadyStage{capability: capability}, nil
	}

	start := make(chan struct{})
	results := make(chan SelectionReadyStage, callerCount)
	errs := make(chan error, callerCount)
	var wait sync.WaitGroup
	wait.Add(callerCount)
	for range callerCount {
		go func() {
			defer wait.Done()
			<-start
			result, err := cache.load(rowsKey, key, builder)
			results <- result
			errs <- err
		}()
	}
	close(start)
	<-entered
	close(release)
	wait.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent cache load: %v", err)
		}
	}
	for result := range results {
		if result.capability != capability {
			t.Fatal("concurrent cache load returned a different reconstruction")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("builder calls = %d, want 1", got)
	}
	cachedKey, cached, found := cache.lookupRows(rowsKey)
	if !found || cachedKey != key || cached.capability != capability {
		t.Fatal("row lookup did not recover the completed exact reconstruction")
	}
}

func TestSelectionReadyStageCacheDoesNotCacheFailures(t *testing.T) {
	t.Parallel()

	cache := selectionReadyStageCache{}
	rowsKey := selectionReadyStageRowsCacheKeyForTest("retry-rows")
	key := selectionReadyStageCacheKeyForTest("retry-full")
	transient := errors.New("transient reconstruction failure")
	var calls atomic.Int64
	builder := func() (SelectionReadyStage, error) {
		if calls.Add(1) == 1 {
			return SelectionReadyStage{}, transient
		}
		return SelectionReadyStage{capability: &selectionReadyStageCapability{}}, nil
	}

	if _, err := cache.load(rowsKey, key, builder); !errors.Is(err, transient) {
		t.Fatalf("first cache load error = %v, want %v", err, transient)
	}
	if _, _, found := cache.lookupRows(rowsKey); found {
		t.Fatal("failed reconstruction remained addressable by row key")
	}
	if _, err := cache.load(rowsKey, key, builder); err != nil {
		t.Fatalf("retry cache load: %v", err)
	}
	if _, err := cache.load(rowsKey, key, builder); err != nil {
		t.Fatalf("cached successful retry: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("builder calls = %d, want 2", got)
	}
}

func TestSelectionReadyStageCacheBoundsCompletedExactInputs(t *testing.T) {
	t.Parallel()

	cache := selectionReadyStageCache{}
	const overflow = 5
	for index := range selectionReadyStageCacheMaxCompletedEntries + overflow {
		rowsKey := selectionReadyStageRowsCacheKeyForTest(
			fmt.Sprintf("bounded-rows-%02d", index),
		)
		key := selectionReadyStageCacheKeyForTest(
			fmt.Sprintf("bounded-full-%02d", index),
		)
		if _, err := cache.load(rowsKey, key, func() (SelectionReadyStage, error) {
			return SelectionReadyStage{capability: &selectionReadyStageCapability{}}, nil
		}); err != nil {
			t.Fatalf("load input %d: %v", index, err)
		}
	}
	if got := len(cache.entries); got != selectionReadyStageCacheMaxCompletedEntries {
		t.Fatalf("retained entries = %d, want %d", got, selectionReadyStageCacheMaxCompletedEntries)
	}
	if got := len(cache.rows); got != selectionReadyStageCacheMaxCompletedEntries {
		t.Fatalf("retained row keys = %d, want %d", got, selectionReadyStageCacheMaxCompletedEntries)
	}
	if got := len(cache.completedOrder); got != selectionReadyStageCacheMaxCompletedEntries {
		t.Fatalf("completed order = %d, want %d", got, selectionReadyStageCacheMaxCompletedEntries)
	}
	if _, _, found := cache.lookupRows(selectionReadyStageRowsCacheKeyForTest("bounded-rows-00")); found {
		t.Fatal("oldest completed reconstruction was not evicted")
	}
}

func selectionReadyStageRowsCacheKeyForTest(label string) selectionReadyStageRowsCacheKey {
	return selectionReadyStageRowsCacheKey(sha256.Sum256([]byte(label)))
}

func selectionReadyStageCacheKeyForTest(label string) selectionReadyStageCacheKey {
	return selectionReadyStageCacheKey(sha256.Sum256([]byte(label)))
}
