package initfs

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

const publicationJournalSchema = "haft.host-publication-journal/v1"

type PublicationJournalPhase string

const (
	PublicationJournalPrepared PublicationJournalPhase = "prepared"
	PublicationJournalApplying PublicationJournalPhase = "applying"
	PublicationJournalManifest PublicationJournalPhase = "manifest"
)

type publicationJournalWire struct {
	Schema                    string                               `json:"schema"`
	BatchDigest               string                               `json:"batch_digest"`
	ManifestPath              string                               `json:"manifest_path"`
	DesiredManifestDigest     string                               `json:"desired_manifest_digest"`
	ManifestPredecessorKind   initplanning.ManifestPredecessorKind `json:"manifest_predecessor_kind"`
	ManifestPredecessorRef    string                               `json:"manifest_predecessor_ref,omitempty"`
	ManifestPredecessorDigest string                               `json:"manifest_predecessor_digest,omitempty"`
	RecoveryArgv              []string                             `json:"recovery_argv"`
	Phase                     PublicationJournalPhase              `json:"phase"`
	ActivePath                string                               `json:"active_path,omitempty"`
	CompletedPaths            []string                             `json:"completed_paths"`
}

type PublicationJournal struct {
	wire      publicationJournalWire
	canonical []byte
	digest    string
}

func NewPublicationJournal(
	batch initplanning.HostPublicationBatch,
	manifestPath string,
) (PublicationJournal, error) {
	if batch.Digest() == "" {
		return PublicationJournal{}, fmt.Errorf("publication batch identity is invalid")
	}
	if manifestPath == "" ||
		!filepath.IsAbs(manifestPath) ||
		filepath.Clean(manifestPath) != manifestPath {
		return PublicationJournal{}, fmt.Errorf("publication journal manifest path is invalid")
	}
	predecessor := batch.ManifestPredecessor()
	wire := publicationJournalWire{
		Schema:                    publicationJournalSchema,
		BatchDigest:               batch.Digest(),
		ManifestPath:              manifestPath,
		DesiredManifestDigest:     batch.Manifest().Digest(),
		ManifestPredecessorKind:   predecessor.Kind(),
		ManifestPredecessorRef:    predecessor.Ref(),
		ManifestPredecessorDigest: predecessor.Digest(),
		RecoveryArgv:              batch.Recovery().Argv(),
		Phase:                     PublicationJournalPrepared,
		CompletedPaths:            []string{},
	}
	journal, err := newPublicationJournal(wire)
	if err != nil {
		return PublicationJournal{}, err
	}
	if err := journal.ValidateAgainst(batch, manifestPath); err != nil {
		return PublicationJournal{}, err
	}
	return journal, nil
}

func ParsePublicationJournal(raw []byte) (PublicationJournal, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var wire publicationJournalWire
	if err := decoder.Decode(&wire); err != nil {
		return PublicationJournal{}, fmt.Errorf("decode publication journal: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return PublicationJournal{}, fmt.Errorf("publication journal has trailing JSON")
	}
	journal, err := newPublicationJournal(wire)
	if err != nil {
		return PublicationJournal{}, err
	}
	if !bytes.Equal(raw, journal.canonical) {
		return PublicationJournal{}, fmt.Errorf("publication journal is not canonical JSON")
	}
	return journal, nil
}

func newPublicationJournal(
	wire publicationJournalWire,
) (PublicationJournal, error) {
	if err := validatePublicationJournalWire(wire); err != nil {
		return PublicationJournal{}, err
	}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return PublicationJournal{}, fmt.Errorf("encode publication journal: %w", err)
	}
	return PublicationJournal{
		wire:      clonePublicationJournalWire(wire),
		canonical: canonical,
		digest:    digestForCarrier(canonical),
	}, nil
}

func validatePublicationJournalWire(wire publicationJournalWire) error {
	if wire.Schema != publicationJournalSchema {
		return fmt.Errorf("publication journal schema is invalid")
	}
	if !validSHA256Digest(wire.BatchDigest) ||
		!validSHA256Digest(wire.DesiredManifestDigest) {
		return fmt.Errorf("publication journal digest identity is invalid")
	}
	if wire.ManifestPath == "" ||
		!filepath.IsAbs(wire.ManifestPath) ||
		filepath.Clean(wire.ManifestPath) != wire.ManifestPath {
		return fmt.Errorf("publication journal manifest path is invalid")
	}
	if err := validateJournalManifestPredecessor(wire); err != nil {
		return err
	}
	if len(wire.RecoveryArgv) == 0 {
		return fmt.Errorf("publication journal recovery operation is empty")
	}
	for _, argument := range wire.RecoveryArgv {
		if argument == "" || argument != strings.TrimSpace(argument) {
			return fmt.Errorf("publication journal recovery argument is invalid")
		}
	}
	if wire.CompletedPaths == nil {
		return fmt.Errorf("publication journal completed path set is absent")
	}
	previous := ""
	for _, path := range wire.CompletedPaths {
		if path <= previous ||
			!filepath.IsAbs(path) ||
			filepath.Clean(path) != path {
			return fmt.Errorf("publication journal completed paths are not canonical")
		}
		previous = path
	}
	switch wire.Phase {
	case PublicationJournalPrepared:
		if wire.ActivePath != "" {
			return fmt.Errorf("prepared publication journal has an active path")
		}
	case PublicationJournalApplying:
		if wire.ActivePath == "" ||
			!filepath.IsAbs(wire.ActivePath) ||
			filepath.Clean(wire.ActivePath) != wire.ActivePath {
			return fmt.Errorf("applying publication journal active path is invalid")
		}
	case PublicationJournalManifest:
		if wire.ActivePath != "" {
			return fmt.Errorf("manifest publication journal has an active path")
		}
	default:
		return fmt.Errorf("publication journal phase is invalid")
	}
	return nil
}

func validateJournalManifestPredecessor(
	wire publicationJournalWire,
) error {
	switch wire.ManifestPredecessorKind {
	case initplanning.ManifestPredecessorMissing:
		if wire.ManifestPredecessorRef != "" ||
			wire.ManifestPredecessorDigest != "" {
			return fmt.Errorf("missing publication manifest predecessor carries identity")
		}
		return nil
	case initplanning.ManifestPredecessorExact:
		if wire.ManifestPredecessorRef == "" ||
			!validSHA256Digest(wire.ManifestPredecessorDigest) {
			return fmt.Errorf("exact publication manifest predecessor is invalid")
		}
		return nil
	default:
		return fmt.Errorf("publication manifest predecessor kind is invalid")
	}
}

func clonePublicationJournalWire(
	wire publicationJournalWire,
) publicationJournalWire {
	return publicationJournalWire{
		Schema:                    wire.Schema,
		BatchDigest:               wire.BatchDigest,
		ManifestPath:              wire.ManifestPath,
		DesiredManifestDigest:     wire.DesiredManifestDigest,
		ManifestPredecessorKind:   wire.ManifestPredecessorKind,
		ManifestPredecessorRef:    wire.ManifestPredecessorRef,
		ManifestPredecessorDigest: wire.ManifestPredecessorDigest,
		RecoveryArgv:              slices.Clone(wire.RecoveryArgv),
		Phase:                     wire.Phase,
		ActivePath:                wire.ActivePath,
		CompletedPaths:            slices.Clone(wire.CompletedPaths),
	}
}

func (journal PublicationJournal) CanonicalBytes() []byte {
	return slices.Clone(journal.canonical)
}

func (journal PublicationJournal) Digest() string {
	return journal.digest
}

func (journal PublicationJournal) BatchDigest() string {
	return journal.wire.BatchDigest
}

func (journal PublicationJournal) ManifestPath() string {
	return journal.wire.ManifestPath
}

func (journal PublicationJournal) DesiredManifestDigest() string {
	return journal.wire.DesiredManifestDigest
}

func (journal PublicationJournal) Phase() PublicationJournalPhase {
	return journal.wire.Phase
}

func (journal PublicationJournal) ActivePath() string {
	return journal.wire.ActivePath
}

func (journal PublicationJournal) CompletedPaths() []string {
	return slices.Clone(journal.wire.CompletedPaths)
}

func (journal PublicationJournal) RecoveryArgv() []string {
	return slices.Clone(journal.wire.RecoveryArgv)
}

func (journal PublicationJournal) ValidateAgainst(
	batch initplanning.HostPublicationBatch,
	manifestPath string,
) error {
	predecessor := batch.ManifestPredecessor()
	if journal.wire.BatchDigest != batch.Digest() ||
		journal.wire.ManifestPath != manifestPath ||
		journal.wire.DesiredManifestDigest != batch.Manifest().Digest() ||
		journal.wire.ManifestPredecessorKind != predecessor.Kind() ||
		journal.wire.ManifestPredecessorRef != predecessor.Ref() ||
		journal.wire.ManifestPredecessorDigest != predecessor.Digest() ||
		!slices.Equal(journal.wire.RecoveryArgv, batch.Recovery().Argv()) {
		return fmt.Errorf("publication journal belongs to another exact batch")
	}
	mutations := mutationStepSet(batch)
	completed := make(map[string]struct{}, len(journal.wire.CompletedPaths))
	for _, path := range journal.wire.CompletedPaths {
		if _, exists := mutations[path]; !exists {
			return fmt.Errorf("publication journal completes a non-mutating path")
		}
		completed[path] = struct{}{}
	}
	if journal.wire.ActivePath != "" {
		if _, exists := mutations[journal.wire.ActivePath]; !exists {
			return fmt.Errorf("publication journal activates a non-mutating path")
		}
		if _, exists := completed[journal.wire.ActivePath]; exists {
			return fmt.Errorf("publication journal active path is already complete")
		}
	}
	if journal.wire.Phase == PublicationJournalManifest &&
		len(completed) != len(mutations) {
		return fmt.Errorf("publication journal reached manifest before all path effects")
	}
	return nil
}

func BeginPublicationStep(
	journal PublicationJournal,
	batch initplanning.HostPublicationBatch,
	path string,
) (PublicationJournal, error) {
	if err := journal.ValidateAgainst(batch, journal.wire.ManifestPath); err != nil {
		return PublicationJournal{}, err
	}
	if journal.wire.Phase != PublicationJournalPrepared {
		return PublicationJournal{}, fmt.Errorf("publication journal is not ready for a path effect")
	}
	mutations := mutationStepSet(batch)
	if _, exists := mutations[path]; !exists {
		return PublicationJournal{}, fmt.Errorf("publication path is not a mutating step")
	}
	if slices.Contains(journal.wire.CompletedPaths, path) {
		return PublicationJournal{}, fmt.Errorf("publication path is already complete")
	}
	wire := clonePublicationJournalWire(journal.wire)
	wire.Phase = PublicationJournalApplying
	wire.ActivePath = path
	return newPublicationJournal(wire)
}

func CompletePublicationStep(
	journal PublicationJournal,
	batch initplanning.HostPublicationBatch,
	path string,
) (PublicationJournal, error) {
	if err := journal.ValidateAgainst(batch, journal.wire.ManifestPath); err != nil {
		return PublicationJournal{}, err
	}
	if journal.wire.Phase != PublicationJournalApplying ||
		journal.wire.ActivePath != path {
		return PublicationJournal{}, fmt.Errorf("publication journal active path differs")
	}
	wire := clonePublicationJournalWire(journal.wire)
	wire.Phase = PublicationJournalPrepared
	wire.ActivePath = ""
	wire.CompletedPaths = append(wire.CompletedPaths, path)
	sort.Strings(wire.CompletedPaths)
	return newPublicationJournal(wire)
}

func BeginManifestPublication(
	journal PublicationJournal,
	batch initplanning.HostPublicationBatch,
) (PublicationJournal, error) {
	if err := journal.ValidateAgainst(batch, journal.wire.ManifestPath); err != nil {
		return PublicationJournal{}, err
	}
	if journal.wire.Phase != PublicationJournalPrepared {
		return PublicationJournal{}, fmt.Errorf("publication journal is not ready for the manifest")
	}
	if len(journal.wire.CompletedPaths) != len(mutationStepSet(batch)) {
		return PublicationJournal{}, fmt.Errorf("publication path effects are incomplete")
	}
	wire := clonePublicationJournalWire(journal.wire)
	wire.Phase = PublicationJournalManifest
	return newPublicationJournal(wire)
}

func mutationStepSet(
	batch initplanning.HostPublicationBatch,
) map[string]struct{} {
	result := make(map[string]struct{})
	for _, step := range batch.Steps() {
		switch step.Kind() {
		case initplanning.PublicationCreate,
			initplanning.PublicationReplace,
			initplanning.PublicationRemove:
			result[step.Path()] = struct{}{}
		}
	}
	return result
}

func digestForCarrier(content []byte) string {
	digest := sha256.Sum256(content)
	return fmt.Sprintf("sha256:%x", digest)
}

type publicationJournalReadKind string

const (
	publicationJournalMissing publicationJournalReadKind = "missing"
	publicationJournalPresent publicationJournalReadKind = "present"
)

type publicationJournalRead struct {
	kind    publicationJournalReadKind
	journal PublicationJournal
}

type publicationJournalStore struct {
	root     string
	path     string
	observer FileObserver
}

func newPublicationJournalStore(
	manifestStore ManifestStore,
) (publicationJournalStore, error) {
	if err := manifestStore.valid(); err != nil {
		return publicationJournalStore{}, err
	}
	path := manifestStore.JournalPath()
	selectedRoot, err := containingManagedRoot(path, []string{manifestStore.root})
	if err != nil || selectedRoot != manifestStore.root {
		return publicationJournalStore{}, fmt.Errorf("publication journal path is outside manifest root")
	}
	return publicationJournalStore{
		root:     manifestStore.root,
		path:     path,
		observer: manifestStore.observer,
	}, nil
}

func (store publicationJournalStore) read() (publicationJournalRead, error) {
	info, missing, err := lstatWithoutManagedRootSymlinks(store.root, store.path)
	if err != nil {
		return publicationJournalRead{}, err
	}
	if missing {
		return publicationJournalRead{kind: publicationJournalMissing}, nil
	}
	content, _, err := store.observer.readStableRegularFile(store.path, info)
	if err != nil {
		return publicationJournalRead{}, err
	}
	journal, err := ParsePublicationJournal(content)
	if err != nil {
		return publicationJournalRead{}, fmt.Errorf("parse publication journal %s: %w", store.path, err)
	}
	return publicationJournalRead{
		kind:    publicationJournalPresent,
		journal: journal,
	}, nil
}

func (store publicationJournalStore) create(
	journal PublicationJournal,
) error {
	current, err := store.read()
	if err != nil {
		return err
	}
	if current.kind == publicationJournalPresent {
		if current.journal.Digest() == journal.Digest() {
			return nil
		}
		return fmt.Errorf("another publication journal already exists")
	}
	stagePath, err := stageCanonicalCarrier(store.path, journal.CanonicalBytes())
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(stagePath)
	}()
	if err := os.Link(stagePath, store.path); err != nil {
		return fmt.Errorf("publish initial publication journal: %w", err)
	}
	if err := os.Remove(stagePath); err != nil {
		return fmt.Errorf("remove linked publication journal stage: %w", err)
	}
	if err := syncDirectory(filepath.Dir(store.path)); err != nil {
		return err
	}
	return store.requireExact(journal.Digest())
}

func (store publicationJournalStore) replace(
	journal PublicationJournal,
	expectedDigest string,
) error {
	current, err := store.read()
	if err != nil {
		return err
	}
	if current.kind != publicationJournalPresent ||
		current.journal.Digest() != expectedDigest {
		return fmt.Errorf("publication journal changed before replacement")
	}
	if current.journal.Digest() == journal.Digest() {
		return nil
	}
	stagePath, err := stageCanonicalCarrier(store.path, journal.CanonicalBytes())
	if err != nil {
		return err
	}
	removeStage := true
	defer func() {
		if removeStage {
			_ = os.Remove(stagePath)
		}
	}()
	current, err = store.read()
	if err != nil {
		return err
	}
	if current.kind != publicationJournalPresent ||
		current.journal.Digest() != expectedDigest {
		return fmt.Errorf("publication journal changed while staged")
	}
	if err := os.Rename(stagePath, store.path); err != nil {
		return fmt.Errorf("replace publication journal: %w", err)
	}
	removeStage = false
	if err := syncDirectory(filepath.Dir(store.path)); err != nil {
		return err
	}
	return store.requireExact(journal.Digest())
}

func (store publicationJournalStore) remove(expectedDigest string) error {
	current, err := store.read()
	if err != nil {
		return err
	}
	if current.kind != publicationJournalPresent ||
		current.journal.Digest() != expectedDigest {
		return fmt.Errorf("publication journal changed before removal")
	}
	if err := os.Remove(store.path); err != nil {
		return fmt.Errorf("remove completed publication journal: %w", err)
	}
	return syncDirectory(filepath.Dir(store.path))
}

func (store publicationJournalStore) requireExact(digest string) error {
	current, err := store.read()
	if err != nil {
		return err
	}
	if current.kind != publicationJournalPresent ||
		current.journal.Digest() != digest {
		return fmt.Errorf("publication journal failed exact reread")
	}
	return nil
}
