package typedmemorystore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const currentEntityDirectorySchemaV1 = "haft.current-entity-directory/v1"

// CurrentEntityDirectoryEntry is one exact admitted EntityID/context label and
// its active aliases at the observed graph snapshot. It contains no inferred
// reference kind, project relation, applicability, or authority.
type CurrentEntityDirectoryEntry struct {
	entity           typedmemory.EntityID
	context          typedmemory.BoundedContextRef
	label            typedmemory.EntityLabel
	provenance       typedmemory.ProvenanceRef
	declaredEvent    projecttypeenvselection.GraphEventRef
	declaredRevision typedmemory.GraphRevision
	aliases          []typedmemory.EntityAlias
}

type CurrentEntityDirectoryEntryInput struct {
	Entity           typedmemory.EntityID
	Context          typedmemory.BoundedContextRef
	Label            typedmemory.EntityLabel
	Provenance       typedmemory.ProvenanceRef
	DeclaredEvent    projecttypeenvselection.GraphEventRef
	DeclaredRevision typedmemory.GraphRevision
	Aliases          []typedmemory.EntityAlias
}

func NewCurrentEntityDirectoryEntry(
	input CurrentEntityDirectoryEntryInput,
) (CurrentEntityDirectoryEntry, error) {
	aliases := canonicalDirectoryAliases(input.Aliases)
	entry := CurrentEntityDirectoryEntry{
		entity:           input.Entity,
		context:          input.Context,
		label:            input.Label,
		provenance:       input.Provenance,
		declaredEvent:    input.DeclaredEvent,
		declaredRevision: input.DeclaredRevision,
		aliases:          aliases,
	}
	if len(aliases) != len(input.Aliases) || !entry.Valid() {
		return CurrentEntityDirectoryEntry{}, fmt.Errorf(
			"current entity directory entry is invalid",
		)
	}
	return entry, nil
}

func (entry CurrentEntityDirectoryEntry) Entity() typedmemory.EntityID {
	return entry.entity
}

func (entry CurrentEntityDirectoryEntry) Context() typedmemory.BoundedContextRef {
	return entry.context
}

func (entry CurrentEntityDirectoryEntry) Label() typedmemory.EntityLabel {
	return entry.label
}

func (entry CurrentEntityDirectoryEntry) Provenance() typedmemory.ProvenanceRef {
	return entry.provenance
}

func (entry CurrentEntityDirectoryEntry) DeclaredEvent() projecttypeenvselection.GraphEventRef {
	return entry.declaredEvent
}

func (entry CurrentEntityDirectoryEntry) DeclaredRevision() typedmemory.GraphRevision {
	return entry.declaredRevision
}

func (entry CurrentEntityDirectoryEntry) Aliases() []typedmemory.EntityAlias {
	return append([]typedmemory.EntityAlias(nil), entry.aliases...)
}

func (entry CurrentEntityDirectoryEntry) Valid() bool {
	entity, entityErr := typedmemory.NewEntityID(entry.entity.String())
	contextRef, contextErr := typedmemory.NewBoundedContextRef(
		entry.context.String(),
	)
	label, labelErr := typedmemory.NewEntityLabel(entry.label.String())
	provenance, provenanceErr := typedmemory.NewProvenanceRef(
		entry.provenance.String(),
	)
	event, eventErr := projecttypeenvselection.ParseGraphEventRef(
		entry.declaredEvent.String(),
	)
	return entityErr == nil &&
		contextErr == nil &&
		labelErr == nil &&
		provenanceErr == nil &&
		eventErr == nil &&
		entry.declaredRevision.Value() > 0 &&
		entity == entry.entity &&
		contextRef == entry.context &&
		label == entry.label &&
		provenance == entry.provenance &&
		event == entry.declaredEvent &&
		slices.Equal(entry.aliases, canonicalDirectoryAliases(entry.aliases))
}

func (entry CurrentEntityDirectoryEntry) key() string {
	return entry.entity.String() + "|" + entry.context.String()
}

type CurrentEntityDirectory struct {
	project        projectledger.ProjectID
	graphBasis     projecttypeenvselection.ProjectGraphSnapshotBasis
	activeTypeEnv  typedmemory.TypeEnvRef
	entries        []CurrentEntityDirectoryEntry
	canonicalBytes []byte
	digest         typedmemory.SHA256Digest
}

func NewCurrentEntityDirectory(
	project projectledger.ProjectID,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	activeTypeEnv typedmemory.TypeEnvRef,
	entries []CurrentEntityDirectoryEntry,
) (CurrentEntityDirectory, error) {
	canonicalProject, projectErr := projectledger.ParseProjectID(
		project.String(),
	)
	typeEnv, typeEnvErr := typedmemory.ParseTypeEnvRef(activeTypeEnv.String())
	if projectErr != nil ||
		typeEnvErr != nil ||
		canonicalProject != project ||
		typeEnv != activeTypeEnv ||
		graphBasis.Verify() != nil ||
		graphBasis.Project() != project {
		return CurrentEntityDirectory{}, fmt.Errorf(
			"current entity directory snapshot coordinates are invalid",
		)
	}
	owned := append([]CurrentEntityDirectoryEntry(nil), entries...)
	sort.Slice(owned, func(left int, right int) bool {
		return owned[left].key() < owned[right].key()
	})
	if err := validateCurrentEntityDirectoryEntries(
		graphBasis.GraphRevision(),
		owned,
	); err != nil {
		return CurrentEntityDirectory{}, err
	}
	canonical, err := encodeCurrentEntityDirectory(
		project,
		graphBasis,
		activeTypeEnv,
		owned,
	)
	if err != nil {
		return CurrentEntityDirectory{}, err
	}
	digest, err := digestCurrentEntityDirectory(canonical)
	if err != nil {
		return CurrentEntityDirectory{}, err
	}
	return CurrentEntityDirectory{
		project:        canonicalProject,
		graphBasis:     graphBasis,
		activeTypeEnv:  typeEnv,
		entries:        owned,
		canonicalBytes: canonical,
		digest:         digest,
	}, nil
}

func (directory CurrentEntityDirectory) ProjectID() projectledger.ProjectID {
	return directory.project
}

func (directory CurrentEntityDirectory) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis {
	return directory.graphBasis
}

func (directory CurrentEntityDirectory) ActiveTypeEnv() typedmemory.TypeEnvRef {
	return directory.activeTypeEnv
}

func (directory CurrentEntityDirectory) Entries() []CurrentEntityDirectoryEntry {
	return append([]CurrentEntityDirectoryEntry(nil), directory.entries...)
}

func (directory CurrentEntityDirectory) CanonicalBytes() []byte {
	return append([]byte(nil), directory.canonicalBytes...)
}

func (directory CurrentEntityDirectory) Digest() typedmemory.SHA256Digest {
	return directory.digest
}

func (directory CurrentEntityDirectory) Verify() error {
	rebuilt, err := NewCurrentEntityDirectory(
		directory.project,
		directory.graphBasis,
		directory.activeTypeEnv,
		directory.entries,
	)
	if err != nil {
		return err
	}
	if rebuilt.digest != directory.digest ||
		!bytes.Equal(rebuilt.canonicalBytes, directory.canonicalBytes) {
		return fmt.Errorf(
			"current entity directory differs from its canonical snapshot",
		)
	}
	return nil
}

func validateCurrentEntityDirectoryEntries(
	graphRevision typedmemory.GraphRevision,
	entries []CurrentEntityDirectoryEntry,
) error {
	aliases := make(map[string]typedmemory.EntityID)
	for index, entry := range entries {
		if !entry.Valid() ||
			entry.DeclaredRevision().Value() > graphRevision.Value() {
			return fmt.Errorf(
				"current entity directory entry %d is invalid",
				index,
			)
		}
		if index > 0 && entries[index-1].key() == entry.key() {
			return fmt.Errorf(
				"current entity directory repeats an entity/context",
			)
		}
		for _, alias := range entry.Aliases() {
			key := entry.Context().String() + "|" + alias.String()
			if previous, found := aliases[key]; found &&
				previous != entry.Entity() {
				return fmt.Errorf(
					"current entity directory contains an ambiguous active alias",
				)
			}
			aliases[key] = entry.Entity()
		}
	}
	if graphRevision.Value() == 0 && len(entries) != 0 {
		return fmt.Errorf(
			"revision-zero current entity directory must be empty",
		)
	}
	return nil
}

func canonicalDirectoryAliases(
	values []typedmemory.EntityAlias,
) []typedmemory.EntityAlias {
	result := append([]typedmemory.EntityAlias(nil), values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].String() < result[right].String()
	})
	return slices.Compact(result)
}

type currentEntityDirectoryCanonicalV1 struct {
	Schema        string                          `json:"schema"`
	Project       string                          `json:"project_id"`
	GraphBasis    string                          `json:"graph_snapshot_basis_ref"`
	GraphRevision uint64                          `json:"graph_revision"`
	ActiveTypeEnv string                          `json:"active_type_env_ref"`
	Entries       []currentEntityEntryCanonicalV1 `json:"entries"`
}

type currentEntityEntryCanonicalV1 struct {
	Entity           string   `json:"entity_id"`
	Context          string   `json:"bounded_context_ref"`
	Label            string   `json:"label"`
	Provenance       string   `json:"provenance_ref"`
	DeclaredEvent    string   `json:"declared_event_ref"`
	DeclaredRevision uint64   `json:"declared_revision"`
	Aliases          []string `json:"active_aliases"`
}

func encodeCurrentEntityDirectory(
	project projectledger.ProjectID,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	activeTypeEnv typedmemory.TypeEnvRef,
	entries []CurrentEntityDirectoryEntry,
) ([]byte, error) {
	encodedEntries := make(
		[]currentEntityEntryCanonicalV1,
		0,
		len(entries),
	)
	for _, entry := range entries {
		aliases := make([]string, 0, len(entry.Aliases()))
		for _, alias := range entry.Aliases() {
			aliases = append(aliases, alias.String())
		}
		encodedEntries = append(
			encodedEntries,
			currentEntityEntryCanonicalV1{
				Entity:           entry.Entity().String(),
				Context:          entry.Context().String(),
				Label:            entry.Label().String(),
				Provenance:       entry.Provenance().String(),
				DeclaredEvent:    entry.DeclaredEvent().String(),
				DeclaredRevision: entry.DeclaredRevision().Value(),
				Aliases:          aliases,
			},
		)
	}
	canonical, err := json.Marshal(currentEntityDirectoryCanonicalV1{
		Schema:        currentEntityDirectorySchemaV1,
		Project:       project.String(),
		GraphBasis:    graphBasis.Ref().String(),
		GraphRevision: graphBasis.GraphRevision().Value(),
		ActiveTypeEnv: activeTypeEnv.String(),
		Entries:       encodedEntries,
	})
	if err != nil {
		return nil, fmt.Errorf("encode current entity directory: %w", err)
	}
	return canonical, nil
}

func digestCurrentEntityDirectory(
	canonical []byte,
) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	return typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
}
