package localpracticeruntime

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
)

func TestTargetBuildCacheReusesOneSuccessfulExactInput(t *testing.T) {
	t.Parallel()

	cache := targetBuildCache{}
	base := loadCurrentBaseArtifact(t)
	source := []byte("exact-source")
	var calls atomic.Int64
	builder := func(
		typeenv.BaseTypeEnvArtifact,
		[]byte,
	) (Target, error) {
		calls.Add(1)
		return targetBuildCacheTestTarget(1), nil
	}

	first, err := cache.load(base, source, builder)
	if err != nil {
		t.Fatalf("first cache load: %v", err)
	}
	second, err := cache.load(base, source, builder)
	if err != nil {
		t.Fatalf("second cache load: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("builder calls = %d, want 1", got)
	}
	assertTargetBuildCacheTestMarker(t, first, 1)
	assertTargetBuildCacheTestMarker(t, second, 1)
}

func TestTargetBuildCacheKeepsDifferentBaseAndSourceInputsDistinct(
	t *testing.T,
) {
	t.Parallel()

	cache := targetBuildCache{}
	current := loadCurrentBaseArtifact(t)
	historical := loadHistoricalV1_2BaseArtifact(t)
	var calls atomic.Int64
	builder := func(
		typeenv.BaseTypeEnvArtifact,
		[]byte,
	) (Target, error) {
		marker := int(calls.Add(1))
		return targetBuildCacheTestTarget(marker), nil
	}
	inputs := []struct {
		base   typeenv.BaseTypeEnvArtifact
		source []byte
		marker int
	}{
		{base: current, source: []byte("source-a"), marker: 1},
		{base: current, source: []byte("source-b"), marker: 2},
		{base: historical, source: []byte("source-a"), marker: 3},
	}

	for _, input := range inputs {
		target, err := cache.load(input.base, input.source, builder)
		if err != nil {
			t.Fatalf("load distinct input: %v", err)
		}
		assertTargetBuildCacheTestMarker(t, target, input.marker)
	}
	for _, input := range inputs {
		target, err := cache.load(input.base, input.source, builder)
		if err != nil {
			t.Fatalf("reload distinct input: %v", err)
		}
		assertTargetBuildCacheTestMarker(t, target, input.marker)
	}
	if got := calls.Load(); got != int64(len(inputs)) {
		t.Fatalf("builder calls = %d, want %d", got, len(inputs))
	}
}

func TestTargetBuildCacheDoesNotCacheFailures(t *testing.T) {
	t.Parallel()

	cache := targetBuildCache{}
	base := loadCurrentBaseArtifact(t)
	source := []byte("retry-source")
	transient := errors.New("transient build failure")
	var calls atomic.Int64
	builder := func(
		typeenv.BaseTypeEnvArtifact,
		[]byte,
	) (Target, error) {
		if calls.Add(1) == 1 {
			return Target{}, transient
		}
		return targetBuildCacheTestTarget(2), nil
	}

	if _, err := cache.load(base, source, builder); !errors.Is(err, transient) {
		t.Fatalf("first cache load error = %v, want %v", err, transient)
	}
	target, err := cache.load(base, source, builder)
	if err != nil {
		t.Fatalf("retry cache load: %v", err)
	}
	assertTargetBuildCacheTestMarker(t, target, 2)
	if _, err := cache.load(base, source, builder); err != nil {
		t.Fatalf("cached successful retry: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("builder calls = %d, want 2", got)
	}
}

func TestTargetBuildCacheCoordinatesConcurrentExactInput(t *testing.T) {
	t.Parallel()

	cache := targetBuildCache{}
	base := loadCurrentBaseArtifact(t)
	source := []byte("concurrent-source")
	const callerCount = 32
	var calls atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	builder := func(
		typeenv.BaseTypeEnvArtifact,
		[]byte,
	) (Target, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return targetBuildCacheTestTarget(1), nil
	}

	start := make(chan struct{})
	results := make(chan Target, callerCount)
	errs := make(chan error, callerCount)
	var wait sync.WaitGroup
	wait.Add(callerCount)
	for range callerCount {
		go func() {
			defer wait.Done()
			<-start
			target, err := cache.load(base, source, builder)
			results <- target
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
	for target := range results {
		assertTargetBuildCacheTestMarker(t, target, 1)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("builder calls = %d, want 1", got)
	}
}

func TestTargetBuildCacheBoundsCompletedExactInputs(t *testing.T) {
	t.Parallel()

	cache := targetBuildCache{}
	base := loadCurrentBaseArtifact(t)
	var calls atomic.Int64
	builder := func(
		typeenv.BaseTypeEnvArtifact,
		[]byte,
	) (Target, error) {
		return targetBuildCacheTestTarget(int(calls.Add(1))), nil
	}
	const overflow = 5
	for index := range targetBuildCacheMaxCompletedEntries + overflow {
		source := []byte(fmt.Sprintf("bounded-source-%02d", index))
		if _, err := cache.load(base, source, builder); err != nil {
			t.Fatalf("load input %d: %v", index, err)
		}
	}
	if got := len(cache.entries); got != targetBuildCacheMaxCompletedEntries {
		t.Fatalf(
			"retained entries = %d, want bound %d",
			got,
			targetBuildCacheMaxCompletedEntries,
		)
	}
	if got := len(cache.completedOrder); got !=
		targetBuildCacheMaxCompletedEntries {
		t.Fatalf(
			"completed order = %d, want bound %d",
			got,
			targetBuildCacheMaxCompletedEntries,
		)
	}

	before := calls.Load()
	latest := []byte(fmt.Sprintf(
		"bounded-source-%02d",
		targetBuildCacheMaxCompletedEntries+overflow-1,
	))
	if _, err := cache.load(base, latest, builder); err != nil {
		t.Fatalf("reload retained latest input: %v", err)
	}
	if got := calls.Load(); got != before {
		t.Fatalf("retained latest input rebuilt; calls = %d, want %d", got, before)
	}
	if _, err := cache.load(base, []byte("bounded-source-00"), builder); err != nil {
		t.Fatalf("reload evicted oldest input: %v", err)
	}
	if got := calls.Load(); got != before+1 {
		t.Fatalf("evicted oldest input calls = %d, want %d", got, before+1)
	}
	if got := len(cache.entries); got != targetBuildCacheMaxCompletedEntries {
		t.Fatalf("post-reload retained entries = %d", got)
	}
}

func targetBuildCacheTestTarget(marker int) Target {
	return Target{
		policies: make(
			[]recordmembershipregistration.RegistrationArtifactV1,
			marker,
		),
	}
}

func assertTargetBuildCacheTestMarker(
	t *testing.T,
	target Target,
	want int,
) {
	t.Helper()
	if got := len(target.RegistrationPolicies()); got != want {
		t.Fatalf("target marker = %d, want %d", got, want)
	}
}
