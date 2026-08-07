package typedmemorystore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// CurrentProjectReadFrame binds the checker snapshot, complete entity
// directory, and active relation observation from one SQLite read
// transaction. It grants no database, mutation, Stage, head-selection, or
// authority capability.
type CurrentProjectReadFrame struct {
	snapshot  CurrentProjectSnapshot
	directory CurrentEntityDirectory
	graph     CurrentProjectGraphObservation
}

func NewCurrentProjectReadFrame(
	snapshot CurrentProjectSnapshot,
	directory CurrentEntityDirectory,
	graph CurrentProjectGraphObservation,
) (CurrentProjectReadFrame, error) {
	if err := directory.Verify(); err != nil {
		return CurrentProjectReadFrame{}, fmt.Errorf(
			"current project read frame directory: %w",
			err,
		)
	}
	if err := graph.Verify(); err != nil {
		return CurrentProjectReadFrame{}, fmt.Errorf(
			"current project read frame graph: %w",
			err,
		)
	}
	memory := snapshot.Snapshot()
	if !memorySnapshotIsPresent(memory) {
		return CurrentProjectReadFrame{}, fmt.Errorf(
			"current project read frame snapshot is missing",
		)
	}
	basis := graph.GraphSnapshotBasis()
	coordinatesMatch := snapshot.ProjectID() == directory.ProjectID() &&
		snapshot.ProjectID() == basis.Project() &&
		snapshot.Environment().Ref() == directory.ActiveTypeEnv() &&
		snapshot.Environment().Ref() == graph.ActiveTypeEnv() &&
		memory.GraphRevision() == basis.GraphRevision() &&
		memory.TypeEnvRef() == graph.ActiveTypeEnv() &&
		directory.GraphSnapshotBasis().Ref() == basis.Ref()
	if !coordinatesMatch {
		return CurrentProjectReadFrame{}, fmt.Errorf(
			"current project read frame components are uncorrelated",
		)
	}
	return CurrentProjectReadFrame{
		snapshot:  snapshot,
		directory: directory,
		graph:     graph,
	}, nil
}

func (frame CurrentProjectReadFrame) Snapshot() CurrentProjectSnapshot {
	return frame.snapshot
}

func (frame CurrentProjectReadFrame) EntityDirectory() CurrentEntityDirectory {
	return frame.directory
}

func (frame CurrentProjectReadFrame) GraphObservation() CurrentProjectGraphObservation {
	return frame.graph
}

type CurrentProjectReadFrameLoader interface {
	LoadCurrentProjectReadFrame(
		context.Context,
		projectledger.ProjectID,
	) (CurrentProjectReadFrame, error)
}

type sqliteCurrentProjectReadFrameLoader struct {
	database               *sql.DB
	typeEnvLoader          TypeEnvLoader
	selectedProjectRuntime SelectedProjectTypeEnvRuntimeResolver
}

var _ CurrentProjectReadFrameLoader = (*sqliteCurrentProjectReadFrameLoader)(nil)

func NewSQLiteCurrentProjectReadFrameLoader(
	database *sql.DB,
	loader TypeEnvLoader,
) (CurrentProjectReadFrameLoader, error) {
	if database == nil {
		return nil, ErrDatabaseRequired
	}
	if !typeEnvLoaderIsPresent(loader) {
		return nil, ErrTypeEnvLoaderRequired
	}
	return &sqliteCurrentProjectReadFrameLoader{
		database:      database,
		typeEnvLoader: loader,
	}, nil
}

func NewProjectAwareSQLiteCurrentProjectReadFrameLoader(
	database *sql.DB,
	loader TypeEnvLoader,
	selectedProjectRuntime SelectedProjectTypeEnvRuntimeResolver,
) (CurrentProjectReadFrameLoader, error) {
	if !selectedProjectTypeEnvRuntimeResolverIsPresent(
		selectedProjectRuntime,
	) {
		return nil, ErrSelectedProjectTypeEnvRuntimeResolverRequired
	}
	base, err := NewSQLiteCurrentProjectReadFrameLoader(database, loader)
	if err != nil {
		return nil, err
	}
	concrete, ok := base.(*sqliteCurrentProjectReadFrameLoader)
	if !ok {
		return nil, fmt.Errorf(
			"construct project-aware read frame: internal loader type is invalid",
		)
	}
	concrete.selectedProjectRuntime = selectedProjectRuntime
	return concrete, nil
}

func (loader *sqliteCurrentProjectReadFrameLoader) LoadCurrentProjectReadFrame(
	ctx context.Context,
	project projectledger.ProjectID,
) (CurrentProjectReadFrame, error) {
	if loader == nil || loader.database == nil {
		return CurrentProjectReadFrame{}, ErrDatabaseRequired
	}
	if !typeEnvLoaderIsPresent(loader.typeEnvLoader) {
		return CurrentProjectReadFrame{}, ErrTypeEnvLoaderRequired
	}
	canonicalProject, projectErr := projectledger.ParseProjectID(
		project.String(),
	)
	if projectErr != nil || canonicalProject != project {
		return CurrentProjectReadFrame{}, fmt.Errorf(
			"load current project read frame: project identity is required",
		)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, loader.database)
	if err != nil {
		return CurrentProjectReadFrame{}, fmt.Errorf(
			"begin current project read frame: %w",
			err,
		)
	}
	frame, err := loader.loadCurrentProjectReadFrame(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		return CurrentProjectReadFrame{}, rollbackError(
			ctx,
			transaction,
			err,
		)
	}
	if err := rollbackSuccess(ctx, transaction); err != nil {
		return CurrentProjectReadFrame{}, err
	}
	return frame, nil
}

func (loader *sqliteCurrentProjectReadFrameLoader) loadCurrentProjectReadFrame(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
) (CurrentProjectReadFrame, error) {
	adapter := &SQLiteAdapter{
		database:               loader.database,
		loader:                 loader.typeEnvLoader,
		selectedProjectRuntime: loader.selectedProjectRuntime,
	}
	graph, err := LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		return CurrentProjectReadFrame{}, err
	}
	snapshot, err := adapter.loadCurrentProjectSnapshot(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		return CurrentProjectReadFrame{}, err
	}
	head, err := loadHeadWithScanner(ctx, transaction, project)
	if err != nil {
		return CurrentProjectReadFrame{}, err
	}
	directory, err := loadCurrentEntityDirectoryTx(
		ctx,
		transaction,
		head,
		graph.GraphSnapshotBasis(),
	)
	if err != nil {
		return CurrentProjectReadFrame{}, err
	}
	return NewCurrentProjectReadFrame(snapshot, directory, graph)
}

type storedCurrentEntityDirectoryRow struct {
	EntityID         string `json:"entity_id"`
	ContextRef       string `json:"context_ref"`
	Label            string `json:"label"`
	ProvenanceRef    string `json:"provenance_ref"`
	DeclaredEventRef string `json:"declared_event_ref"`
	DeclaredRevision int64  `json:"declared_revision"`
	EventRevision    int64  `json:"event_revision"`
}

func loadCurrentEntityDirectoryTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
) (CurrentEntityDirectory, error) {
	if err := requireGenericStorageCapability(ctx, transaction); err != nil {
		return CurrentEntityDirectory{}, err
	}
	if graphBasis.Project() != head.Project() ||
		graphBasis.GraphRevision() != head.Revision() {
		return CurrentEntityDirectory{}, fmt.Errorf(
			"current entity directory basis differs from graph head",
		)
	}
	encoded, err := loadJSONAggregate(
		ctx,
		transaction,
		`SELECT COALESCE(json_group_array(json_object(
			'entity_id', entity_id,
			'context_ref', bounded_context_ref,
			'label', label,
			'provenance_ref', provenance_ref,
			'declared_event_ref', declared_event_ref,
			'declared_revision', declared_revision,
			'event_revision', event_revision
		)), '[]')
		FROM (
			SELECT entity_context.entity_id,
				entity_context.bounded_context_ref,
				entity_context.label,
				entity_context.provenance_ref,
				entity_context.declared_event_ref,
				entity_context.declared_revision,
				event.graph_revision AS event_revision
			FROM typed_memory_entity_contexts entity_context
			JOIN typed_memory_graph_events event
				ON event.project_id = entity_context.project_id
				AND event.event_ref = entity_context.declared_event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.commit_ref = event.commit_ref
			WHERE entity_context.project_id = ?
				AND event.graph_revision <= ?
			ORDER BY entity_context.entity_id,
				entity_context.bounded_context_ref
		)`,
		head,
	)
	if err != nil {
		return CurrentEntityDirectory{}, err
	}
	rows := []storedCurrentEntityDirectoryRow{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return CurrentEntityDirectory{}, storedAdmissionIntegrity(
			"decode current entity directory",
			err,
		)
	}
	entityContexts, err := directoryEntityContextSet(rows)
	if err != nil {
		return CurrentEntityDirectory{}, err
	}
	activeAliases, err := loadCurrentAliases(
		ctx,
		transaction,
		head,
		entityContexts,
	)
	if err != nil {
		return CurrentEntityDirectory{}, err
	}
	entries, err := currentEntityDirectoryEntries(rows, activeAliases)
	if err != nil {
		return CurrentEntityDirectory{}, err
	}
	return NewCurrentEntityDirectory(
		head.Project(),
		graphBasis,
		head.ActiveTypeEnv(),
		entries,
	)
}

func directoryEntityContextSet(
	rows []storedCurrentEntityDirectoryRow,
) (map[entityContextKey]struct{}, error) {
	result := make(map[entityContextKey]struct{}, len(rows))
	for _, row := range rows {
		entity, err := typedmemory.NewEntityID(row.EntityID)
		if err != nil {
			return nil, storedAdmissionIntegrity(
				"current entity directory EntityID",
				err,
			)
		}
		contextRef, err := typedmemory.NewBoundedContextRef(row.ContextRef)
		if err != nil {
			return nil, storedAdmissionIntegrity(
				"current entity directory bounded context",
				err,
			)
		}
		key := entityContextKey{entity: entity, context: contextRef}
		if _, found := result[key]; found {
			return nil, storedAdmissionIntegrity(
				"current entity directory repeats an entity/context",
				nil,
			)
		}
		result[key] = struct{}{}
	}
	return result, nil
}

func currentEntityDirectoryEntries(
	rows []storedCurrentEntityDirectoryRow,
	activeAliases map[aliasContextKey]typedmemory.EntityID,
) ([]CurrentEntityDirectoryEntry, error) {
	aliases := aliasesByEntityContext(activeAliases)
	entries := make([]CurrentEntityDirectoryEntry, 0, len(rows))
	for _, row := range rows {
		entry, err := decodeCurrentEntityDirectoryEntry(row, aliases)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func aliasesByEntityContext(
	active map[aliasContextKey]typedmemory.EntityID,
) map[entityContextKey][]typedmemory.EntityAlias {
	result := make(map[entityContextKey][]typedmemory.EntityAlias)
	for coordinate, entity := range active {
		key := entityContextKey{
			entity:  entity,
			context: coordinate.context,
		}
		result[key] = append(result[key], coordinate.alias)
	}
	return result
}

func decodeCurrentEntityDirectoryEntry(
	row storedCurrentEntityDirectoryRow,
	aliases map[entityContextKey][]typedmemory.EntityAlias,
) (CurrentEntityDirectoryEntry, error) {
	if row.DeclaredRevision <= 0 ||
		row.DeclaredRevision != row.EventRevision {
		return CurrentEntityDirectoryEntry{}, storedAdmissionIntegrity(
			"current entity directory declaration coordinates",
			nil,
		)
	}
	entity, err := typedmemory.NewEntityID(row.EntityID)
	if err != nil {
		return CurrentEntityDirectoryEntry{}, storedAdmissionIntegrity(
			"current entity directory EntityID",
			err,
		)
	}
	contextRef, err := typedmemory.NewBoundedContextRef(row.ContextRef)
	if err != nil {
		return CurrentEntityDirectoryEntry{}, storedAdmissionIntegrity(
			"current entity directory context",
			err,
		)
	}
	label, err := typedmemory.NewEntityLabel(row.Label)
	if err != nil {
		return CurrentEntityDirectoryEntry{}, storedAdmissionIntegrity(
			"current entity directory label",
			err,
		)
	}
	provenance, err := typedmemory.NewProvenanceRef(row.ProvenanceRef)
	if err != nil {
		return CurrentEntityDirectoryEntry{}, storedAdmissionIntegrity(
			"current entity directory provenance",
			err,
		)
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(
		row.DeclaredEventRef,
	)
	if err != nil {
		return CurrentEntityDirectoryEntry{}, storedAdmissionIntegrity(
			"current entity directory declaration event",
			err,
		)
	}
	revision, err := graphRevisionFromSQLite(row.DeclaredRevision)
	if err != nil {
		return CurrentEntityDirectoryEntry{}, storedAdmissionIntegrity(
			"current entity directory declaration revision",
			err,
		)
	}
	key := entityContextKey{entity: entity, context: contextRef}
	entry, err := NewCurrentEntityDirectoryEntry(
		CurrentEntityDirectoryEntryInput{
			Entity:           entity,
			Context:          contextRef,
			Label:            label,
			Provenance:       provenance,
			DeclaredEvent:    event,
			DeclaredRevision: revision,
			Aliases:          aliases[key],
		},
	)
	if err != nil {
		return CurrentEntityDirectoryEntry{}, storedAdmissionIntegrity(
			"construct current entity directory entry",
			err,
		)
	}
	return entry, nil
}
