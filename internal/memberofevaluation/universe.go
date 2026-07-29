package memberofevaluation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const persistedEntityUniverseSchemaV1 = "haft.typed-memory.persisted-entity-universe/v1"

const persistedEntityUniverseObservablePrefix = "persisted-entity-universe:"

// PersistedEntityUniverse is the closed posture for the exact persisted entity
// universe visible at one graph revision and bounded context. It is not itself
// U.EntitySet: the selected enumeration and candidate-visibility rules still
// have to produce that semantic result.
type PersistedEntityUniverse interface {
	persistedEntityUniverseVariant()
}

type persistedEntityUniverseCanonicalV1 struct {
	SchemaVersion  string   `json:"schema_version"`
	ProjectID      string   `json:"project_id"`
	BoundedContext string   `json:"bounded_context"`
	GraphRevision  uint64   `json:"graph_revision"`
	EntityIDs      []string `json:"entity_ids"`
}

type ExactPersistedEntityUniverse struct {
	project        projectledger.ProjectID
	context        typedmemory.BoundedContextRef
	revision       typedmemory.GraphRevision
	members        []typedmemory.EntityID
	canonicalBytes []byte
	digest         typedmemory.SHA256Digest
}

// NewExactPersistedEntityUniverse constructs a canonical exact-universe value.
// Construction validates bytes and coordinates; only an outer store shell can
// establish that the supplied members came from one correlated transaction.
func NewExactPersistedEntityUniverse(
	project projectledger.ProjectID,
	contextRef typedmemory.BoundedContextRef,
	revision typedmemory.GraphRevision,
	members []typedmemory.EntityID,
) (ExactPersistedEntityUniverse, error) {
	parsedProject, err := projectledger.ParseProjectID(project.String())
	if err != nil || parsedProject != project {
		return ExactPersistedEntityUniverse{}, fmt.Errorf(
			"persisted entity universe requires an exact project identity",
		)
	}
	parsedContext, err := typedmemory.NewBoundedContextRef(contextRef.String())
	if err != nil || parsedContext != contextRef {
		return ExactPersistedEntityUniverse{}, fmt.Errorf(
			"persisted entity universe requires an exact bounded context",
		)
	}
	normalized, err := normalizePersistedEntityUniverseMembers(members)
	if err != nil {
		return ExactPersistedEntityUniverse{}, err
	}
	canonical, err := encodePersistedEntityUniverse(
		project,
		contextRef,
		revision,
		normalized,
	)
	if err != nil {
		return ExactPersistedEntityUniverse{}, err
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return ExactPersistedEntityUniverse{}, err
	}
	return ExactPersistedEntityUniverse{
		project:        project,
		context:        contextRef,
		revision:       revision,
		members:        normalized,
		canonicalBytes: canonical,
		digest:         digest,
	}, nil
}

func (universe ExactPersistedEntityUniverse) ProjectID() projectledger.ProjectID {
	return universe.project
}

func (universe ExactPersistedEntityUniverse) BoundedContext() typedmemory.BoundedContextRef {
	return universe.context
}

func (universe ExactPersistedEntityUniverse) GraphRevision() typedmemory.GraphRevision {
	return universe.revision
}

func (universe ExactPersistedEntityUniverse) Members() []typedmemory.EntityID {
	return append([]typedmemory.EntityID(nil), universe.members...)
}

func (universe ExactPersistedEntityUniverse) Contains(entity typedmemory.EntityID) bool {
	_, found := slices.BinarySearchFunc(
		universe.members,
		entity,
		func(left, right typedmemory.EntityID) int {
			return compareText(left.String(), right.String())
		},
	)
	return found
}

func (universe ExactPersistedEntityUniverse) CanonicalBytes() []byte {
	return append([]byte(nil), universe.canonicalBytes...)
}

func (universe ExactPersistedEntityUniverse) Digest() typedmemory.SHA256Digest {
	return universe.digest
}

func (universe ExactPersistedEntityUniverse) ObservableInput() (
	typedmemory.MemberOfObservableInput,
	error,
) {
	if !universe.Valid() {
		return typedmemory.MemberOfObservableInput{}, fmt.Errorf(
			"persisted entity universe is invalid",
		)
	}
	reference, err := typedmemory.NewObservableInputRef(
		persistedEntityUniverseObservablePrefix + universe.digest.String(),
	)
	if err != nil {
		return typedmemory.MemberOfObservableInput{}, err
	}
	return typedmemory.NewMemberOfObservableInput(reference, universe.digest)
}

func (universe ExactPersistedEntityUniverse) ObservableBlob() (
	ObservableInputBlob,
	error,
) {
	input, err := universe.ObservableInput()
	if err != nil {
		return ObservableInputBlob{}, err
	}
	return NewObservableInputBlob(
		input.Reference(),
		input.Digest(),
		universe.CanonicalBytes(),
	)
}

func (ExactPersistedEntityUniverse) persistedEntityUniverseVariant() {}

func (universe ExactPersistedEntityUniverse) Valid() bool {
	parsedProject, err := projectledger.ParseProjectID(universe.project.String())
	if err != nil || parsedProject != universe.project {
		return false
	}
	parsedContext, err := typedmemory.NewBoundedContextRef(universe.context.String())
	if err != nil || parsedContext != universe.context {
		return false
	}
	normalized, err := normalizePersistedEntityUniverseMembers(universe.members)
	if err != nil || !slices.Equal(normalized, universe.members) {
		return false
	}
	canonical, err := encodePersistedEntityUniverse(
		universe.project,
		universe.context,
		universe.revision,
		normalized,
	)
	if err != nil {
		return false
	}
	digest, err := digestBytes(canonical)
	return err == nil &&
		digest == universe.digest &&
		bytes.Equal(canonical, universe.canonicalBytes)
}

// PersistedEntityUniverseUnavailable is explicit and cannot be interpreted as
// an exact empty universe.
type PersistedEntityUniverseUnavailable struct{}

func NewPersistedEntityUniverseUnavailable() PersistedEntityUniverseUnavailable {
	return PersistedEntityUniverseUnavailable{}
}

func (PersistedEntityUniverseUnavailable) Valid() bool { return true }

func (PersistedEntityUniverseUnavailable) persistedEntityUniverseVariant() {}

func normalizePersistedEntityUniverseMembers(
	members []typedmemory.EntityID,
) ([]typedmemory.EntityID, error) {
	normalized := append([]typedmemory.EntityID(nil), members...)
	for _, entity := range normalized {
		parsed, err := typedmemory.NewEntityID(entity.String())
		if err != nil || parsed != entity {
			return nil, fmt.Errorf(
				"persisted entity universe contains an invalid EntityID",
			)
		}
	}
	slices.SortFunc(normalized, func(left, right typedmemory.EntityID) int {
		return compareText(left.String(), right.String())
	})
	for index := 1; index < len(normalized); index++ {
		if normalized[index-1] == normalized[index] {
			return nil, fmt.Errorf(
				"persisted entity universe contains duplicate EntityID %q",
				normalized[index].String(),
			)
		}
	}
	return normalized, nil
}

func encodePersistedEntityUniverse(
	project projectledger.ProjectID,
	contextRef typedmemory.BoundedContextRef,
	revision typedmemory.GraphRevision,
	members []typedmemory.EntityID,
) ([]byte, error) {
	entityIDs := make([]string, 0, len(members))
	for _, entity := range members {
		entityIDs = append(entityIDs, entity.String())
	}
	encoded := persistedEntityUniverseCanonicalV1{
		SchemaVersion:  persistedEntityUniverseSchemaV1,
		ProjectID:      project.String(),
		BoundedContext: contextRef.String(),
		GraphRevision:  revision.Value(),
		EntityIDs:      entityIDs,
	}
	canonical, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("encode persisted entity universe: %w", err)
	}
	return canonical, nil
}

func compareText(left, right string) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}
