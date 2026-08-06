package projecttypeenvstage

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"sync"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
)

const selectionReadyStageCacheMaxCompletedEntries = 8

type selectionReadyStageRowsCacheKey [sha256.Size]byte

type selectionReadyStageCacheKey [sha256.Size]byte

type selectionReadyStageCache struct {
	mu             sync.Mutex
	entries        map[selectionReadyStageCacheKey]*selectionReadyStageCacheEntry
	rows           map[selectionReadyStageRowsCacheKey]selectionReadyStageCacheKey
	completedOrder []selectionReadyStageCacheKey
}

type selectionReadyStageCacheEntry struct {
	ready      chan struct{}
	result     SelectionReadyStage
	err        error
	panicked   bool
	panicValue any
}

type selectionReadyStageReloadInputs struct {
	base                         typeenv.BaseTypeEnvArtifact
	extensions                   []projecttypeenv.ProjectTypeEnvExtensionArtifact
	runtimeBasis                 projecttypeenv.RuntimeEvaluationBasisArtifact
	runtimeMechanismCanonicals   [][]byte
	registrationPolicyCanonicals [][]byte
	composite                    projecttypeenv.ProjectTypeEnvCompositeArtifact
}

func (cache *selectionReadyStageCache) lookupRows(
	key selectionReadyStageRowsCacheKey,
) (selectionReadyStageCacheKey, SelectionReadyStage, bool) {
	cache.mu.Lock()
	fullKey, ok := cache.rows[key]
	entry := cache.entries[fullKey]
	cache.mu.Unlock()
	if !ok || entry == nil {
		return selectionReadyStageCacheKey{}, SelectionReadyStage{}, false
	}
	result, err := awaitSelectionReadyStageCacheEntry(entry)
	if err != nil {
		return selectionReadyStageCacheKey{}, SelectionReadyStage{}, false
	}
	return fullKey, result, true
}

func (cache *selectionReadyStageCache) load(
	rowsKey selectionReadyStageRowsCacheKey,
	key selectionReadyStageCacheKey,
	builder func() (SelectionReadyStage, error),
) (SelectionReadyStage, error) {
	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = make(map[selectionReadyStageCacheKey]*selectionReadyStageCacheEntry)
	}
	if cache.rows == nil {
		cache.rows = make(map[selectionReadyStageRowsCacheKey]selectionReadyStageCacheKey)
	}
	if entry, ok := cache.entries[key]; ok {
		cache.mu.Unlock()
		return awaitSelectionReadyStageCacheEntry(entry)
	}
	entry := &selectionReadyStageCacheEntry{ready: make(chan struct{})}
	cache.entries[key] = entry
	cache.rows[rowsKey] = key
	cache.mu.Unlock()

	cache.build(entry, rowsKey, key, builder)
	return awaitSelectionReadyStageCacheEntry(entry)
}

func (cache *selectionReadyStageCache) build(
	entry *selectionReadyStageCacheEntry,
	rowsKey selectionReadyStageRowsCacheKey,
	key selectionReadyStageCacheKey,
	builder func() (SelectionReadyStage, error),
) {
	defer func() {
		if panicValue := recover(); panicValue != nil {
			cache.mu.Lock()
			delete(cache.entries, key)
			if cache.rows[rowsKey] == key {
				delete(cache.rows, rowsKey)
			}
			entry.panicked = true
			entry.panicValue = panicValue
			close(entry.ready)
			cache.mu.Unlock()
		}
	}()

	result, err := builder()

	cache.mu.Lock()
	entry.result = result
	entry.err = err
	if err != nil {
		delete(cache.entries, key)
		if cache.rows[rowsKey] == key {
			delete(cache.rows, rowsKey)
		}
	} else {
		cache.completedOrder = append(cache.completedOrder, key)
		cache.evictCompleted()
	}
	close(entry.ready)
	cache.mu.Unlock()
}

func (cache *selectionReadyStageCache) evictCompleted() {
	for len(cache.completedOrder) > selectionReadyStageCacheMaxCompletedEntries {
		oldest := cache.completedOrder[0]
		cache.completedOrder = cache.completedOrder[1:]
		delete(cache.entries, oldest)
		for rowsKey, fullKey := range cache.rows {
			if fullKey == oldest {
				delete(cache.rows, rowsKey)
			}
		}
	}
}

func awaitSelectionReadyStageCacheEntry(
	entry *selectionReadyStageCacheEntry,
) (SelectionReadyStage, error) {
	<-entry.ready
	if entry.panicked {
		panic(entry.panicValue)
	}
	return entry.result, entry.err
}

func newSelectionReadyStageRowsCacheKey(
	stage stageRecord,
	verification verificationRecord,
	snapshot executableSnapshotRecord,
) selectionReadyStageRowsCacheKey {
	writer := newSelectionReadyStageCacheKeyWriter(
		"haft.project-typeenv-stage.reload-rows/v1",
	)
	writer.addRows(stage, verification, snapshot)
	return selectionReadyStageRowsCacheKey(writer.sum())
}

func newSelectionReadyStageCacheKey(
	stage stageRecord,
	verification verificationRecord,
	snapshot executableSnapshotRecord,
	inputs selectionReadyStageReloadInputs,
) selectionReadyStageCacheKey {
	writer := newSelectionReadyStageCacheKeyWriter(
		"haft.project-typeenv-stage.reload/v1",
	)
	writer.addRows(stage, verification, snapshot)
	writer.addBytes("base", inputs.base.CanonicalBytes())
	writer.addUint64("extension_count", uint64(len(inputs.extensions)))
	for _, extension := range inputs.extensions {
		writer.addBytes("extension", extension.CanonicalBytes())
	}
	writer.addBytes("runtime_basis", inputs.runtimeBasis.CanonicalBytes())
	writer.addUint64(
		"runtime_mechanism_count",
		uint64(len(inputs.runtimeMechanismCanonicals)),
	)
	for _, canonical := range inputs.runtimeMechanismCanonicals {
		writer.addBytes("runtime_mechanism", canonical)
	}
	writer.addUint64(
		"registration_policy_count",
		uint64(len(inputs.registrationPolicyCanonicals)),
	)
	for _, canonical := range inputs.registrationPolicyCanonicals {
		writer.addBytes("registration_policy", canonical)
	}
	writer.addBytes("composite", inputs.composite.CanonicalBytes())
	return selectionReadyStageCacheKey(writer.sum())
}

type selectionReadyStageCacheKeyWriter struct {
	hash hash.Hash
}

func newSelectionReadyStageCacheKeyWriter(
	domain string,
) *selectionReadyStageCacheKeyWriter {
	writer := &selectionReadyStageCacheKeyWriter{hash: sha256.New()}
	writer.addString("domain", domain)
	return writer
}

func (writer *selectionReadyStageCacheKeyWriter) addRows(
	stage stageRecord,
	verification verificationRecord,
	snapshot executableSnapshotRecord,
) {
	writer.addString("stage.ref", stage.ref)
	writer.addString("stage.digest", stage.digest)
	writer.addString("stage.project", stage.project)
	writer.addString("stage.verification_ref", stage.verificationRef)
	writer.addString("stage.executable_ref", stage.executableRef)
	writer.addString("stage.canonical_schema", stage.canonicalSchema)
	writer.addBytes("stage.canonical", stage.canonical)

	writer.addString("verification.ref", verification.ref)
	writer.addString("verification.digest", verification.digest)
	writer.addString("verification.lowerer_schema", verification.lowererSchema)
	writer.addString("verification.canonical_schema", verification.canonicalSchema)
	writer.addBytes("verification.canonical", verification.canonical)

	writer.addString("snapshot.type_env_ref", snapshot.typeEnvRef)
	writer.addString("snapshot.digest", snapshot.snapshotDigest)
	writer.addString("snapshot.lowered_digest", snapshot.loweredDigest)
	writer.addString("snapshot.source_revision", snapshot.sourceRevision)
	writer.addString("snapshot.compiler_schema", snapshot.compilerSchema)
	writer.addString("snapshot.lowerer_schema", snapshot.lowererSchema)
	writer.addString("snapshot.verification_ref", snapshot.verificationRef)
	writer.addString("snapshot.canonical_schema", snapshot.canonicalSchema)
	writer.addBytes("snapshot.canonical", snapshot.canonical)
}

func (writer *selectionReadyStageCacheKeyWriter) addString(label, value string) {
	writer.addBytes(label, []byte(value))
}

func (writer *selectionReadyStageCacheKeyWriter) addUint64(label string, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.addBytes(label, encoded[:])
}

func (writer *selectionReadyStageCacheKeyWriter) addBytes(label string, value []byte) {
	writeSelectionReadyStageCacheKeyPart(writer.hash, []byte(label))
	writeSelectionReadyStageCacheKeyPart(writer.hash, value)
}

func (writer *selectionReadyStageCacheKeyWriter) sum() [sha256.Size]byte {
	var result [sha256.Size]byte
	copy(result[:], writer.hash.Sum(nil))
	return result
}

func writeSelectionReadyStageCacheKeyPart(destination hash.Hash, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = destination.Write(length[:])
	_, _ = destination.Write(value)
}
