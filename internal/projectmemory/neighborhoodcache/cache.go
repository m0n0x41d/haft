// Package neighborhoodcache owns the replaceable cache effect around the pure
// EntityOfConcern neighborhood assembler. Cache state is derived and
// disposable; it cannot mutate typed memory, graph revision, TypeEnv head,
// evidence currentness, authority, or Work order.
package neighborhoodcache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const KeySchemaV1 = "haft.neighborhood-cache-key/v1"

type ProjectionSchemaVersion struct{ value string }

func NewProjectionSchemaVersion(
	raw string,
) (ProjectionSchemaVersion, error) {
	value, err := exactOneLine("projection schema version", raw)
	if err != nil {
		return ProjectionSchemaVersion{}, err
	}
	return ProjectionSchemaVersion{value: value}, nil
}

func (version ProjectionSchemaVersion) String() string {
	return version.value
}

type Key struct {
	project          projectledger.ProjectID
	request          neighborhood.NeighborhoodRequest
	directoryDigest  typedmemory.SHA256Digest
	graphBasisRef    projecttypeenvselection.ProjectGraphSnapshotBasisRef
	projectionSchema ProjectionSchemaVersion
	canonical        []byte
	digest           typedmemory.SHA256Digest
}

type keyCanonicalV1 struct {
	Schema    string `json:"schema"`
	ProjectID string `json:"project_id"`
	// TypeEnvRef is the content-addressed semantic environment selected for
	// this projection. HeadRevision is deliberately absent: selecting another
	// C changes this coordinate, while selecting the same C again does not
	// change projection meaning. GraphRevision separately pins project facts.
	TypeEnvRef        string           `json:"type_env_ref"`
	TypeEnvDigest     string           `json:"type_env_digest"`
	GraphRevision     uint64           `json:"graph_revision"`
	EntityRefKind     string           `json:"entity_ref_kind"`
	EntityReference   string           `json:"entity_reference"`
	BoundedContextRef string           `json:"bounded_context_ref"`
	ProfileRef        string           `json:"profile_ref"`
	ProfileDigest     string           `json:"profile_digest"`
	ProjectionSchema  string           `json:"projection_schema"`
	RequestedFacets   []string         `json:"requested_facets"`
	Detail            string           `json:"detail"`
	IncludeHistory    bool             `json:"include_history"`
	Budget            keyBudgetV1      `json:"budget"`
	SourceBasis       keySourceBasisV1 `json:"source_basis"`
}

type keyBudgetV1 struct {
	MaxFacets                   uint32 `json:"max_facets"`
	MaxItemsPerFacet            uint32 `json:"max_items_per_facet"`
	MaxRelationPathsPerItem     uint32 `json:"max_relation_paths_per_item"`
	MaxCarrierExcerptCharacters uint32 `json:"max_carrier_excerpt_characters"`
	MaxProvenanceDepth          uint32 `json:"max_provenance_depth"`
}

type keySourceBasisV1 struct {
	EntityDirectoryDigest string `json:"entity_directory_digest"`
	GraphSnapshotBasisRef string `json:"graph_snapshot_basis_ref"`
}

func NewKey(
	project projectledger.ProjectID,
	request neighborhood.NeighborhoodRequest,
	directoryDigest typedmemory.SHA256Digest,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	projectionSchema ProjectionSchemaVersion,
) (Key, error) {
	canonicalProject, err := projectledger.ParseProjectID(project.String())
	if err != nil || canonicalProject != project {
		return Key{}, fmt.Errorf(
			"neighborhood cache key requires exact project identity",
		)
	}
	if !request.Valid() {
		return Key{}, fmt.Errorf(
			"neighborhood cache key requires an exact request",
		)
	}
	canonicalDigest, err := typedmemory.NewSHA256Digest(
		directoryDigest.String(),
	)
	if err != nil || canonicalDigest != directoryDigest {
		return Key{}, fmt.Errorf(
			"neighborhood cache key requires entity-directory digest",
		)
	}
	if err := graphBasis.Verify(); err != nil {
		return Key{}, fmt.Errorf(
			"neighborhood cache key requires exact graph snapshot basis: %w",
			err,
		)
	}
	basisProject, err := projectledger.ParseProjectID(
		graphBasis.Project().String(),
	)
	if err != nil ||
		basisProject != project ||
		graphBasis.GraphRevision() != request.GraphRevision() {
		return Key{}, fmt.Errorf(
			"neighborhood cache key graph basis is uncorrelated",
		)
	}
	if projectionSchema.String() == "" {
		return Key{}, fmt.Errorf(
			"neighborhood cache key requires projection schema",
		)
	}
	key := Key{
		project:          project,
		request:          request,
		directoryDigest:  directoryDigest,
		graphBasisRef:    graphBasis.Ref(),
		projectionSchema: projectionSchema,
	}
	canonical, digest, err := canonicalizeKey(key)
	if err != nil {
		return Key{}, err
	}
	key.canonical = canonical
	key.digest = digest
	if !key.Valid() {
		return Key{}, fmt.Errorf("built neighborhood cache key is invalid")
	}
	return key, nil
}

func (key Key) Project() projectledger.ProjectID {
	return key.project
}

func (key Key) SnapshotBasis() neighborhood.SnapshotBasis {
	request := key.request
	basis, err := neighborhood.NewSnapshotBasis(
		request.GraphRevision(),
		request.TypeEnv(),
		request.TypeEnv().Digest(),
	)
	if err != nil {
		return neighborhood.SnapshotBasis{}
	}
	return basis
}

func (key Key) Request() neighborhood.NeighborhoodRequest {
	return key.request
}

func (key Key) DirectoryDigest() typedmemory.SHA256Digest {
	return key.directoryDigest
}

func (key Key) GraphSnapshotBasisRef() projecttypeenvselection.ProjectGraphSnapshotBasisRef {
	return key.graphBasisRef
}

func (key Key) ProjectionSchema() ProjectionSchemaVersion {
	return key.projectionSchema
}

func (key Key) CanonicalBytes() []byte {
	return append([]byte{}, key.canonical...)
}

func (key Key) Digest() typedmemory.SHA256Digest {
	return key.digest
}

func (key Key) Valid() bool {
	canonicalProject, projectErr :=
		projectledger.ParseProjectID(key.project.String())
	directoryDigest, directoryErr := typedmemory.NewSHA256Digest(
		key.directoryDigest.String(),
	)
	graphBasisRef, graphBasisErr :=
		projecttypeenvselection.ParseProjectGraphSnapshotBasisRef(
			key.graphBasisRef.String(),
		)
	canonical, digest, canonicalErr := canonicalizeKey(key)
	return projectErr == nil &&
		canonicalProject == key.project &&
		key.request.Valid() &&
		directoryErr == nil &&
		directoryDigest == key.directoryDigest &&
		graphBasisErr == nil &&
		graphBasisRef == key.graphBasisRef &&
		key.projectionSchema.String() != "" &&
		canonicalErr == nil &&
		bytes.Equal(canonical, key.canonical) &&
		digest == key.digest
}

func canonicalizeKey(
	key Key,
) ([]byte, typedmemory.SHA256Digest, error) {
	if !key.request.Valid() {
		return nil, typedmemory.SHA256Digest{},
			fmt.Errorf("cache-key request is invalid")
	}
	view := key.request.View()
	budget := key.request.Budget()
	facets := view.RequestedFacets()
	presentedFacets := make([]string, 0, len(facets))
	for _, facet := range facets {
		presentedFacets = append(presentedFacets, string(facet))
	}
	canonical := keyCanonicalV1{
		Schema:            KeySchemaV1,
		ProjectID:         key.project.String(),
		TypeEnvRef:        key.request.TypeEnv().String(),
		TypeEnvDigest:     key.request.TypeEnv().Digest().String(),
		GraphRevision:     key.request.GraphRevision().Value(),
		EntityRefKind:     key.request.Entity().RefKind().String(),
		EntityReference:   key.request.Entity().ReferenceID().String(),
		BoundedContextRef: key.request.Context().String(),
		ProfileRef:        view.ProfileRef().String(),
		ProfileDigest:     view.ProfileDigest().String(),
		ProjectionSchema:  key.projectionSchema.String(),
		RequestedFacets:   presentedFacets,
		Detail:            string(view.Detail()),
		IncludeHistory:    view.IncludeHistory(),
		Budget: keyBudgetV1{
			MaxFacets:                   budget.MaxFacets(),
			MaxItemsPerFacet:            budget.MaxItemsPerFacet(),
			MaxRelationPathsPerItem:     budget.MaxRelationPathsPerItem(),
			MaxCarrierExcerptCharacters: budget.MaxCarrierExcerptCharacters(),
			MaxProvenanceDepth:          budget.MaxProvenanceDepth(),
		},
		SourceBasis: keySourceBasisV1{
			EntityDirectoryDigest: key.directoryDigest.String(),
			GraphSnapshotBasisRef: key.graphBasisRef.String(),
		},
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, typedmemory.SHA256Digest{}, fmt.Errorf(
			"encode neighborhood cache key: %w",
			err,
		)
	}
	sum := sha256.Sum256(encoded)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		return nil, typedmemory.SHA256Digest{}, err
	}
	return encoded, digest, nil
}

type Entry struct {
	key       Key
	result    neighborhood.ExactNeighborhood
	canonical []byte
	digest    typedmemory.SHA256Digest
}

func NewEntry(
	key Key,
	result neighborhood.ExactNeighborhood,
) (Entry, error) {
	entry := Entry{
		key:       key,
		result:    result,
		canonical: result.CanonicalBytes(),
		digest:    result.Digest(),
	}
	if !entry.ValidFor(key) {
		return Entry{}, fmt.Errorf(
			"exact neighborhood does not match cache key",
		)
	}
	return entry, nil
}

func (entry Entry) Key() Key {
	return entry.key
}

func (entry Entry) Result() neighborhood.ExactNeighborhood {
	return entry.result
}

func (entry Entry) CanonicalBytes() []byte {
	return append([]byte{}, entry.canonical...)
}

func (entry Entry) Digest() typedmemory.SHA256Digest {
	return entry.digest
}

func (entry Entry) ValidFor(key Key) bool {
	if !key.Valid() ||
		entry.key.Digest() != key.Digest() ||
		!bytes.Equal(entry.key.CanonicalBytes(), key.CanonicalBytes()) ||
		!entry.result.Valid() ||
		!bytes.Equal(entry.canonical, entry.result.CanonicalBytes()) ||
		entry.digest != entry.result.Digest() {
		return false
	}
	return exactResultMatchesKey(entry.result, key)
}

func exactResultMatchesKey(
	result neighborhood.ExactNeighborhood,
	key Key,
) bool {
	request := key.Request()
	view := result.ViewContext()
	basis := result.ProjectionBasis()
	facets := result.Facets()
	requestedFacets := request.View().RequestedFacets()
	if result.SnapshotBasis() != key.SnapshotBasis() ||
		view.Entity() != request.Entity() ||
		view.Context() != request.Context() ||
		view.ProfileRef() != request.View().ProfileRef() ||
		basis.ProfileRef() != request.View().ProfileRef() ||
		basis.ProfileDigest() != request.View().ProfileDigest() ||
		basis.ProjectionSchemaVersion() != key.ProjectionSchema().String() ||
		result.AppliedBudget().RequestedLimits() != request.Budget() ||
		len(facets) != len(requestedFacets) {
		return false
	}
	for index := range facets {
		if facets[index].Kind() != requestedFacets[index] {
			return false
		}
	}
	return true
}

type LookupKind string

const (
	LookupHit         LookupKind = "hit"
	LookupMiss        LookupKind = "miss"
	LookupCorrupt     LookupKind = "corrupt"
	LookupUnavailable LookupKind = "unavailable"
)

type LookupResult interface {
	Kind() LookupKind
	lookupResultVariant()
}

type Hit struct{ entry Entry }
type Miss struct{}
type Corrupt struct{}
type Unavailable struct{}

func (Hit) Kind() LookupKind         { return LookupHit }
func (Miss) Kind() LookupKind        { return LookupMiss }
func (Corrupt) Kind() LookupKind     { return LookupCorrupt }
func (Unavailable) Kind() LookupKind { return LookupUnavailable }

func (Hit) lookupResultVariant()         {}
func (Miss) lookupResultVariant()        {}
func (Corrupt) lookupResultVariant()     {}
func (Unavailable) lookupResultVariant() {}

func (hit Hit) Entry() Entry {
	return hit.entry
}

type StoreResult interface {
	Stored() bool
	storeResultVariant()
}

type Stored struct{}
type StoreUnavailable struct{}

func (Stored) Stored() bool           { return true }
func (StoreUnavailable) Stored() bool { return false }

func (Stored) storeResultVariant()           {}
func (StoreUnavailable) storeResultVariant() {}

type Snapshot struct {
	entries map[string]Entry
}

func NewSnapshot() Snapshot {
	return Snapshot{entries: map[string]Entry{}}
}

func (snapshot Snapshot) Lookup(key Key) LookupResult {
	if !key.Valid() {
		return Corrupt{}
	}
	entry, found := snapshot.entries[key.Digest().String()]
	if !found {
		return Miss{}
	}
	if !entry.ValidFor(key) {
		return Corrupt{}
	}
	return Hit{entry: entry}
}

func (snapshot Snapshot) With(entry Entry) (Snapshot, error) {
	if !entry.ValidFor(entry.Key()) {
		return Snapshot{}, fmt.Errorf(
			"cannot store invalid neighborhood cache entry",
		)
	}
	entries := make(map[string]Entry, len(snapshot.entries)+1)
	for key, value := range snapshot.entries {
		entries[key] = value
	}
	entries[entry.Key().Digest().String()] = entry
	return Snapshot{entries: entries}, nil
}

type Store interface {
	Lookup(context.Context, Key) LookupResult
	Put(context.Context, Entry) StoreResult
}

type AtomicStore struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

func NewAtomicStore() *AtomicStore {
	return &AtomicStore{snapshot: NewSnapshot()}
}

func (store *AtomicStore) Lookup(
	ctx context.Context,
	key Key,
) LookupResult {
	if store == nil || ctx == nil || ctx.Err() != nil {
		return Unavailable{}
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.snapshot.Lookup(key)
}

func (store *AtomicStore) Put(
	ctx context.Context,
	entry Entry,
) StoreResult {
	if store == nil || ctx == nil || ctx.Err() != nil {
		return StoreUnavailable{}
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	next, err := store.snapshot.With(entry)
	if err != nil {
		return StoreUnavailable{}
	}
	store.snapshot = next
	return Stored{}
}

type UnavailableStore struct{}

func NewUnavailableStore() UnavailableStore {
	return UnavailableStore{}
}

func (UnavailableStore) Lookup(
	context.Context,
	Key,
) LookupResult {
	return Unavailable{}
}

func (UnavailableStore) Put(
	context.Context,
	Entry,
) StoreResult {
	return StoreUnavailable{}
}

type UseKind string

const (
	UseHit               UseKind = "hit"
	UseMissStored        UseKind = "miss_stored"
	UseMissUnstored      UseKind = "miss_unstored"
	UseCorruptReplaced   UseKind = "corrupt_replaced"
	UseCorruptUnstored   UseKind = "corrupt_unstored"
	UseUnavailableBypass UseKind = "unavailable_bypass"
)

type UseObservation interface {
	Kind() UseKind
	useObservationVariant()
}

type HitUse struct{}
type MissStoredUse struct{}
type MissUnstoredUse struct{}
type CorruptReplacedUse struct{}
type CorruptUnstoredUse struct{}
type UnavailableBypassUse struct{}

func (HitUse) Kind() UseKind               { return UseHit }
func (MissStoredUse) Kind() UseKind        { return UseMissStored }
func (MissUnstoredUse) Kind() UseKind      { return UseMissUnstored }
func (CorruptReplacedUse) Kind() UseKind   { return UseCorruptReplaced }
func (CorruptUnstoredUse) Kind() UseKind   { return UseCorruptUnstored }
func (UnavailableBypassUse) Kind() UseKind { return UseUnavailableBypass }

func (HitUse) useObservationVariant()               {}
func (MissStoredUse) useObservationVariant()        {}
func (MissUnstoredUse) useObservationVariant()      {}
func (CorruptReplacedUse) useObservationVariant()   {}
func (CorruptUnstoredUse) useObservationVariant()   {}
func (UnavailableBypassUse) useObservationVariant() {}

type Read struct {
	result      neighborhood.ExactNeighborhood
	canonical   []byte
	observation UseObservation
}

func (read Read) Result() neighborhood.ExactNeighborhood {
	return read.result
}

func (read Read) CanonicalBytes() []byte {
	return append([]byte{}, read.canonical...)
}

func (read Read) CacheUse() UseObservation {
	return read.observation
}

type Compute func(context.Context) (
	neighborhood.ExactNeighborhood,
	error,
)

type Shell struct {
	store Store
}

func NewShell(store Store) (Shell, error) {
	if store == nil {
		return Shell{}, fmt.Errorf(
			"neighborhood cache shell requires a store",
		)
	}
	return Shell{store: store}, nil
}

func NewUnavailableShell() Shell {
	shell, err := NewShell(NewUnavailableStore())
	if err != nil {
		panic("static unavailable neighborhood cache is invalid")
	}
	return shell
}

func (shell Shell) ReadThrough(
	ctx context.Context,
	key Key,
	compute Compute,
) (Read, error) {
	if ctx == nil || compute == nil || !key.Valid() {
		return Read{}, fmt.Errorf(
			"neighborhood cache read-through request is invalid",
		)
	}
	lookup := shell.lookup(ctx, key)
	switch exact := lookup.(type) {
	case Hit:
		return newRead(exact.Entry(), HitUse{})
	case Miss:
		return shell.computeAndStore(
			ctx,
			key,
			compute,
			MissStoredUse{},
			MissUnstoredUse{},
		)
	case Corrupt:
		return shell.computeAndStore(
			ctx,
			key,
			compute,
			CorruptReplacedUse{},
			CorruptUnstoredUse{},
		)
	case Unavailable:
		result, err := compute(ctx)
		if err != nil {
			return Read{}, err
		}
		entry, err := NewEntry(key, result)
		if err != nil {
			return Read{}, err
		}
		return newRead(entry, UnavailableBypassUse{})
	default:
		return Read{}, fmt.Errorf(
			"unsupported neighborhood cache lookup %T",
			lookup,
		)
	}
}

func (shell Shell) lookup(
	ctx context.Context,
	key Key,
) LookupResult {
	if shell.store == nil {
		return Unavailable{}
	}
	return shell.store.Lookup(ctx, key)
}

func (shell Shell) computeAndStore(
	ctx context.Context,
	key Key,
	compute Compute,
	storedObservation UseObservation,
	unstoredObservation UseObservation,
) (Read, error) {
	result, err := compute(ctx)
	if err != nil {
		return Read{}, err
	}
	entry, err := NewEntry(key, result)
	if err != nil {
		return Read{}, err
	}
	storeResult := shell.store.Put(ctx, entry)
	if storeResult.Stored() {
		return newRead(entry, storedObservation)
	}
	return newRead(entry, unstoredObservation)
}

func newRead(
	entry Entry,
	observation UseObservation,
) (Read, error) {
	if observation == nil || !entry.ValidFor(entry.Key()) {
		return Read{}, fmt.Errorf("cached neighborhood read is invalid")
	}
	return Read{
		result:      entry.Result(),
		canonical:   entry.CanonicalBytes(),
		observation: observation,
	}, nil
}

func exactOneLine(label string, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" ||
		value != raw ||
		strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s must be exact non-empty one-line text", label)
	}
	return value, nil
}
