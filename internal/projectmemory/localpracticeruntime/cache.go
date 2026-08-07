package localpracticeruntime

import (
	"sync"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
)

var processTargetBuildCache targetBuildCache

const targetBuildCacheMaxCompletedEntries = 16

type targetBuilder func(
	typeenv.BaseTypeEnvArtifact,
	[]byte,
) (Target, error)

// targetBuildCache owns successful immutable Build results for one process.
// The key retains the exact canonical base and source bytes rather than a
// project identity or activation state. Build is pure with respect to project
// state, and Target exposes immutable values through owned-value accessors, so
// one successful result can be shared without sharing a mutable project store.
//
// An in-flight entry coordinates only callers for the same exact input pair.
// Different inputs may compile concurrently. Failed or panicking builds remove
// their entry so a later unchanged call can retry. Completed entries use a
// small FIFO bound so a long-lived process cannot retain arbitrary project
// source carriers forever; the shipped runtime currently needs six.
type targetBuildCache struct {
	mu             sync.Mutex
	entries        map[targetBuildCacheKey]*targetBuildCacheEntry
	completedOrder []targetBuildCacheKey
}

type targetBuildCacheKey struct {
	baseCanonical string
	source        string
}

type targetBuildCacheEntry struct {
	ready      chan struct{}
	target     Target
	err        error
	panicked   bool
	panicValue any
}

func (cache *targetBuildCache) load(
	base typeenv.BaseTypeEnvArtifact,
	source []byte,
	builder targetBuilder,
) (Target, error) {
	sourceBytes := append([]byte(nil), source...)
	key := targetBuildCacheKey{
		baseCanonical: string(base.CanonicalBytes()),
		source:        string(sourceBytes),
	}

	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = make(map[targetBuildCacheKey]*targetBuildCacheEntry)
	}
	if entry, ok := cache.entries[key]; ok {
		cache.mu.Unlock()
		return awaitTargetBuildCacheEntry(entry)
	}
	entry := &targetBuildCacheEntry{ready: make(chan struct{})}
	cache.entries[key] = entry
	cache.mu.Unlock()

	cache.build(entry, key, base, sourceBytes, builder)
	return awaitTargetBuildCacheEntry(entry)
}

func (cache *targetBuildCache) build(
	entry *targetBuildCacheEntry,
	key targetBuildCacheKey,
	base typeenv.BaseTypeEnvArtifact,
	source []byte,
	builder targetBuilder,
) {
	defer func() {
		if panicValue := recover(); panicValue != nil {
			cache.mu.Lock()
			delete(cache.entries, key)
			entry.panicked = true
			entry.panicValue = panicValue
			close(entry.ready)
			cache.mu.Unlock()
		}
	}()

	target, err := builder(base, source)

	cache.mu.Lock()
	entry.target = target
	entry.err = err
	if err != nil {
		delete(cache.entries, key)
	} else {
		cache.completedOrder = append(cache.completedOrder, key)
		cache.evictCompleted()
	}
	close(entry.ready)
	cache.mu.Unlock()
}

func (cache *targetBuildCache) evictCompleted() {
	for len(cache.completedOrder) > targetBuildCacheMaxCompletedEntries {
		oldest := cache.completedOrder[0]
		cache.completedOrder = cache.completedOrder[1:]
		delete(cache.entries, oldest)
	}
}

func awaitTargetBuildCacheEntry(
	entry *targetBuildCacheEntry,
) (Target, error) {
	<-entry.ready
	if entry.panicked {
		panic(entry.panicValue)
	}
	return entry.target, entry.err
}
