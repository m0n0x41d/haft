package projectmemory

import (
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation"
	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

var (
	ErrCurrentResolutionFrameInvalid = errors.New(
		"current project resolution frame is invalid",
	)
	ErrEntityReferenceKindMissing = errors.New(
		"active TypeEnv does not define U.EntityRef",
	)
	ErrCurrentIdentityReconciliationBasis = errors.New(
		"current identity-reconciliation state differs from the read frame",
	)
	ErrCurrentIdentityReconciliationIntegrity = errors.New(
		"current identity-reconciliation state cannot be projected through the entity directory",
	)
)

const currentResolutionIndexSchemaV1 = "haft.current-resolution-index/v1"

// BuildCurrentResolutionIndex conservatively projects the complete entity
// directory of one correlated current-project read frame. It adds no relation,
// applicability, ranking, truth, lifecycle, or authority claim.
func BuildCurrentResolutionIndex(
	frame typedmemorystore.CurrentProjectReadFrame,
) (memoryresolve.ResolutionIndex, error) {
	correlated, err := typedmemorystore.NewCurrentProjectReadFrame(
		frame.Snapshot(),
		frame.EntityDirectory(),
		frame.GraphObservation(),
	)
	if err != nil {
		return memoryresolve.ResolutionIndex{}, fmt.Errorf(
			"%w: %v",
			ErrCurrentResolutionFrameInvalid,
			err,
		)
	}
	environment := correlated.Snapshot().Environment()
	entityRefKind, err := currentEntityReferenceKind(environment)
	if err != nil {
		return memoryresolve.ResolutionIndex{}, err
	}
	directory := correlated.EntityDirectory()
	snapshot, err := currentResolutionSnapshotBasis(directory)
	if err != nil {
		return memoryresolve.ResolutionIndex{}, err
	}
	units, err := currentResolutionUnits(
		directory,
		entityRefKind,
	)
	if err != nil {
		return memoryresolve.ResolutionIndex{}, err
	}
	indexRef, err := memoryresolve.NewResolutionIndexRef(
		"current-entity-directory:" + directory.ProjectID().String(),
	)
	if err != nil {
		return memoryresolve.ResolutionIndex{}, err
	}
	indexVersion, err := memoryresolve.NewResolutionIndexVersion(
		currentResolutionIndexSchemaV1 + "@" + directory.Digest().String(),
	)
	if err != nil {
		return memoryresolve.ResolutionIndex{}, err
	}
	completenessRef, err :=
		memoryresolve.NewResolutionCompletenessBasisRef(
			"current-entity-directory-complete:" +
				directory.GraphSnapshotBasis().Ref().String() +
				"@" +
				directory.Digest().String(),
		)
	if err != nil {
		return memoryresolve.ResolutionIndex{}, err
	}
	completeness, err := memoryresolve.NewCompleteAllContexts(
		completenessRef,
	)
	if err != nil {
		return memoryresolve.ResolutionIndex{}, err
	}
	index, err := memoryresolve.NewResolutionIndex(
		indexRef,
		indexVersion,
		snapshot,
		completeness,
		units,
	)
	if err != nil {
		return memoryresolve.ResolutionIndex{}, fmt.Errorf(
			"build current project resolution index: %w",
			err,
		)
	}
	return index, nil
}

// BuildCurrentReviewedIdentityIndex projects only verified committed
// reconciliation state at the exact same project, graph revision, and TypeEnv
// as the current read frame. Directory declarations remain historical facts;
// this index changes only how exact historical references resolve.
func BuildCurrentReviewedIdentityIndex(
	frame typedmemorystore.CurrentProjectReadFrame,
	state identityreconciliation.CommittedResolutionState,
) (memoryresolve.ReviewedIdentityIndex, error) {
	correlated, err := typedmemorystore.NewCurrentProjectReadFrame(
		frame.Snapshot(),
		frame.EntityDirectory(),
		frame.GraphObservation(),
	)
	if err != nil {
		return memoryresolve.ReviewedIdentityIndex{}, fmt.Errorf(
			"%w: %v",
			ErrCurrentResolutionFrameInvalid,
			err,
		)
	}
	if err := state.Verify(); err != nil {
		return memoryresolve.ReviewedIdentityIndex{}, fmt.Errorf(
			"%w: %v",
			ErrCurrentIdentityReconciliationIntegrity,
			err,
		)
	}
	directory := correlated.EntityDirectory()
	basis := state.Basis()
	basisMatches := basis.Project() == directory.ProjectID() &&
		basis.GraphRevision() == directory.GraphSnapshotBasis().GraphRevision() &&
		basis.TypeEnv() == directory.ActiveTypeEnv()
	if !basisMatches {
		return memoryresolve.ReviewedIdentityIndex{},
			ErrCurrentIdentityReconciliationBasis
	}
	snapshot, err := currentResolutionSnapshotBasis(directory)
	if err != nil {
		return memoryresolve.ReviewedIdentityIndex{}, err
	}
	entries, err := currentReviewedIdentityEntries(directory, state)
	if err != nil {
		return memoryresolve.ReviewedIdentityIndex{}, err
	}
	index, err := memoryresolve.NewReviewedIdentityIndex(
		snapshot,
		state.Digest(),
		entries,
	)
	if err != nil {
		return memoryresolve.ReviewedIdentityIndex{}, fmt.Errorf(
			"build current reviewed identity index: %w",
			err,
		)
	}
	return index, nil
}

func currentReviewedIdentityEntries(
	directory typedmemorystore.CurrentEntityDirectory,
	state identityreconciliation.CommittedResolutionState,
) ([]memoryresolve.ReviewedIdentityResolution, error) {
	directoryEntries := directory.Entries()
	queries := make(
		[]identityreconciliation.HistoricalResolutionQuery,
		0,
		len(directoryEntries),
	)
	for _, directoryEntry := range directoryEntries {
		query, err := identityreconciliation.NewHistoricalResolutionQuery(
			directoryEntry.Entity(),
			directoryEntry.Context(),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: %v",
				ErrCurrentIdentityReconciliationIntegrity,
				err,
			)
		}
		queries = append(queries, query)
	}
	resolutions, err := state.ResolveBatch(queries)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %v",
			ErrCurrentIdentityReconciliationIntegrity,
			err,
		)
	}
	entries := make([]memoryresolve.ReviewedIdentityResolution, 0)
	for _, resolution := range resolutions {
		projected, found, err := currentReviewedIdentityEntry(
			directory,
			resolution,
		)
		if err != nil {
			return nil, err
		}
		if found {
			entries = append(entries, projected)
		}
	}
	return entries, nil
}

func currentReviewedIdentityEntry(
	directory typedmemorystore.CurrentEntityDirectory,
	resolution identityreconciliation.HistoricalResolution,
) (memoryresolve.ReviewedIdentityResolution, bool, error) {
	switch value := resolution.(type) {
	case identityreconciliation.CurrentIdentity:
		return nil, false, nil
	case identityreconciliation.MergedIdentity:
		if !currentDirectoryContains(
			directory,
			value.Current(),
			value.Context(),
		) {
			return nil, false, fmt.Errorf(
				"%w: merge successor %s is absent",
				ErrCurrentIdentityReconciliationIntegrity,
				value.Current().String(),
			)
		}
		history, err := currentReconciliationHistory(
			value.ReconciliationHistory(),
		)
		if err != nil {
			return nil, false, err
		}
		projected, err := memoryresolve.NewReviewedMergedIdentity(
			value.Entity(),
			value.Current(),
			value.Context(),
			history,
		)
		return projected, err == nil, err
	case identityreconciliation.SplitIdentityCandidates:
		for _, candidate := range value.Candidates() {
			if !currentDirectoryContains(
				directory,
				candidate,
				value.Context(),
			) {
				return nil, false, fmt.Errorf(
					"%w: split candidate %s is absent",
					ErrCurrentIdentityReconciliationIntegrity,
					candidate.String(),
				)
			}
		}
		history, err := currentReconciliationHistory(
			value.ReconciliationHistory(),
		)
		if err != nil {
			return nil, false, fmt.Errorf(
				"%w: split reconciliation history: %v",
				ErrCurrentIdentityReconciliationIntegrity,
				err,
			)
		}
		projected, err := memoryresolve.NewReviewedSplitIdentity(
			value.Entity(),
			value.Candidates(),
			value.Context(),
			history,
		)
		return projected, err == nil, err
	case identityreconciliation.IdentityAbsent:
		return nil, false, fmt.Errorf(
			"%w: directory entity %s is absent from reconciliation state",
			ErrCurrentIdentityReconciliationIntegrity,
			value.Entity().String(),
		)
	default:
		return nil, false, fmt.Errorf(
			"%w: unsupported resolution %T",
			ErrCurrentIdentityReconciliationIntegrity,
			resolution,
		)
	}
}

func currentDirectoryContains(
	directory typedmemorystore.CurrentEntityDirectory,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) bool {
	for _, entry := range directory.Entries() {
		if entry.Entity() == entity && entry.Context() == contextRef {
			return true
		}
	}
	return false
}

func currentReconciliationHistory(
	values []string,
) ([]typedmemory.ResolutionBasisRef, error) {
	result := make([]typedmemory.ResolutionBasisRef, 0, len(values))
	for _, value := range values {
		basis, err := typedmemory.NewResolutionBasisRef(value)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: merge reconciliation reference: %v",
				ErrCurrentIdentityReconciliationIntegrity,
				err,
			)
		}
		result = append(result, basis)
	}
	return result, nil
}

func currentEntityReferenceKind(
	environment typedmemory.TypeEnv,
) (typedmemory.RefKindRef, error) {
	id, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		return typedmemory.RefKindRef{}, err
	}
	reference, err := typedmemory.NewRefKindRef(environment.Ref(), id)
	if err != nil {
		return typedmemory.RefKindRef{}, err
	}
	if _, found := environment.RefKindDefinition(reference); !found {
		return typedmemory.RefKindRef{}, ErrEntityReferenceKindMissing
	}
	return reference, nil
}

func currentResolutionSnapshotBasis(
	directory typedmemorystore.CurrentEntityDirectory,
) (neighborhood.SnapshotBasis, error) {
	basis := directory.GraphSnapshotBasis()
	snapshot, err := neighborhood.NewSnapshotBasis(
		basis.GraphRevision(),
		directory.ActiveTypeEnv(),
		directory.ActiveTypeEnv().Digest(),
	)
	if err != nil {
		return neighborhood.SnapshotBasis{}, fmt.Errorf(
			"build current project resolution snapshot basis: %w",
			err,
		)
	}
	return snapshot, nil
}

func currentResolutionUnits(
	directory typedmemorystore.CurrentEntityDirectory,
	entityRefKind typedmemory.RefKindRef,
) ([]memoryresolve.ResolutionUnit, error) {
	entries := directory.Entries()
	units := make([]memoryresolve.ResolutionUnit, 0, len(entries))
	for _, entry := range entries {
		unit, err := currentResolutionUnit(
			directory,
			entityRefKind,
			entry,
		)
		if err != nil {
			return nil, err
		}
		units = append(units, unit)
	}
	return units, nil
}

func currentResolutionUnit(
	directory typedmemorystore.CurrentEntityDirectory,
	entityRefKind typedmemory.RefKindRef,
	entry typedmemorystore.CurrentEntityDirectoryEntry,
) (memoryresolve.ResolutionUnit, error) {
	referenceID, err := typedmemory.NewReferenceID(entry.Entity().String())
	if err != nil {
		return memoryresolve.ResolutionUnit{}, fmt.Errorf(
			"build current project entity reference: %w",
			err,
		)
	}
	reference, err := typedmemory.NewPersistedRef(
		entityRefKind,
		referenceID,
	)
	if err != nil {
		return memoryresolve.ResolutionUnit{}, err
	}
	label, err := neighborhood.NewReadableItemText(entry.Label().String())
	if err != nil {
		return memoryresolve.ResolutionUnit{}, err
	}
	basis, err := typedmemory.NewResolutionBasisRef(
		"current-entity-directory-entry:" +
			directory.Digest().String() +
			":" +
			entry.Entity().String() +
			":" +
			entry.Context().String(),
	)
	if err != nil {
		return memoryresolve.ResolutionUnit{}, err
	}
	unit, err := memoryresolve.NewResolutionUnit(
		reference,
		entry.Context(),
		label,
		entry.Aliases(),
		entry.Provenance(),
		basis,
	)
	if err != nil {
		return memoryresolve.ResolutionUnit{}, fmt.Errorf(
			"build current project resolution unit %q: %w",
			entry.Entity().String(),
			err,
		)
	}
	return unit, nil
}
