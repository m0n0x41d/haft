package identityreconciliation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// CommittedResolutionBasis is the exact project-head coordinate against which
// a read-only identity-reconciliation state is observed. A caller-supplied
// basis never becomes current: SQLiteService revalidates it against the graph
// head inside the same read transaction that loads every reviewed redirect.
type CommittedResolutionBasis struct {
	project  projectledger.ProjectID
	revision typedmemory.GraphRevision
	typeEnv  typedmemory.TypeEnvRef
}

func NewCommittedResolutionBasis(
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	typeEnv typedmemory.TypeEnvRef,
) (CommittedResolutionBasis, error) {
	canonicalProject, projectErr := projectledger.ParseProjectID(project.String())
	canonicalTypeEnv, typeEnvErr := typedmemory.ParseTypeEnvRef(typeEnv.String())
	if projectErr != nil ||
		typeEnvErr != nil ||
		canonicalProject != project ||
		canonicalTypeEnv != typeEnv ||
		revision.Value() > math.MaxInt64 {
		return CommittedResolutionBasis{}, fmt.Errorf(
			"committed identity-resolution basis is invalid",
		)
	}
	return CommittedResolutionBasis{
		project:  canonicalProject,
		revision: revision,
		typeEnv:  canonicalTypeEnv,
	}, nil
}

func (basis CommittedResolutionBasis) Project() projectledger.ProjectID {
	return basis.project
}

func (basis CommittedResolutionBasis) GraphRevision() typedmemory.GraphRevision {
	return basis.revision
}

func (basis CommittedResolutionBasis) TypeEnv() typedmemory.TypeEnvRef {
	return basis.typeEnv
}

// CommittedResolutionState is an immutable, verified projection of all
// reviewed identity redirects committed at one exact current project basis.
// It preserves durable reconciliation references and contains no write,
// admission, head-selection, or authority capability.
type CommittedResolutionState struct {
	basis          CommittedResolutionBasis
	redirects      []committedRedirect
	canonicalBytes []byte
	digest         typedmemory.SHA256Digest
}

type HistoricalResolutionQuery struct {
	entity  typedmemory.EntityID
	context typedmemory.BoundedContextRef
}

func NewHistoricalResolutionQuery(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) (HistoricalResolutionQuery, error) {
	canonicalEntity, entityErr := typedmemory.NewEntityID(entity.String())
	canonicalContext, contextErr := typedmemory.NewBoundedContextRef(
		contextRef.String(),
	)
	if entityErr != nil ||
		contextErr != nil ||
		canonicalEntity != entity ||
		canonicalContext != contextRef {
		return HistoricalResolutionQuery{}, fmt.Errorf(
			"historical identity-resolution query is invalid",
		)
	}
	return HistoricalResolutionQuery{
		entity:  canonicalEntity,
		context: canonicalContext,
	}, nil
}

func (query HistoricalResolutionQuery) Entity() typedmemory.EntityID {
	return query.entity
}

func (query HistoricalResolutionQuery) Context() typedmemory.BoundedContextRef {
	return query.context
}

func (state CommittedResolutionState) Basis() CommittedResolutionBasis {
	return state.basis
}

func (state CommittedResolutionState) CanonicalBytes() []byte {
	return append([]byte(nil), state.canonicalBytes...)
}

func (state CommittedResolutionState) Digest() typedmemory.SHA256Digest {
	return state.digest
}

func (state CommittedResolutionState) Verify() error {
	rebuilt, err := newCommittedResolutionState(state.basis, state.redirects)
	if err != nil {
		return err
	}
	if rebuilt.digest != state.digest ||
		string(rebuilt.canonicalBytes) != string(state.canonicalBytes) {
		return fmt.Errorf("%w: committed identity-resolution state is not canonical", ErrStoredIntegrity)
	}
	return nil
}

// Resolve preserves the requested historical EntityID. A merge follows its
// exact reviewed history; a split returns its complete candidate set and
// cannot be converted into a guessed successor.
func (state CommittedResolutionState) Resolve(
	requested typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) (HistoricalResolution, error) {
	query, err := NewHistoricalResolutionQuery(requested, contextRef)
	if err != nil {
		return nil, err
	}
	resolved, err := state.ResolveBatch([]HistoricalResolutionQuery{query})
	if err != nil {
		return nil, err
	}
	return resolved[0], nil
}

// ResolveBatch verifies the immutable state once and preserves query order.
// It is intended for complete current-directory projection, where one state
// may be consulted for many historical EntityID/context coordinates.
func (state CommittedResolutionState) ResolveBatch(
	queries []HistoricalResolutionQuery,
) ([]HistoricalResolution, error) {
	if err := state.Verify(); err != nil {
		return nil, err
	}
	result := make([]HistoricalResolution, 0, len(queries))
	for _, query := range queries {
		canonical, err := NewHistoricalResolutionQuery(
			query.Entity(),
			query.Context(),
		)
		if err != nil || canonical != query {
			return nil, fmt.Errorf(
				"committed identity-resolution query is invalid",
			)
		}
		resolution, err := resolveCommittedRedirects(
			state.redirects,
			query.Entity(),
			query.Context(),
		)
		if err != nil {
			return nil, err
		}
		result = append(result, resolution)
	}
	return result, nil
}

// CommittedResolutionStateSource is the read-only port consumed by the
// project-memory resolution shell. Implementations must return the complete
// committed state at exactly the requested basis or fail closed.
type CommittedResolutionStateSource interface {
	LoadCommittedResolutionState(
		context.Context,
		CommittedResolutionBasis,
	) (CommittedResolutionState, error)
}

var _ CommittedResolutionStateSource = (*SQLiteService)(nil)

// SQLiteCommittedResolutionStateSource is the capability-narrow production
// adapter for public reads. Unlike SQLiteService it exposes no Commit or
// projection-debt effect and requires no clock.
type SQLiteCommittedResolutionStateSource struct {
	database *sql.DB
	schema   schemaGate
}

var _ CommittedResolutionStateSource = (*SQLiteCommittedResolutionStateSource)(nil)

func NewSQLiteCommittedResolutionStateSource(
	database *sql.DB,
) (*SQLiteCommittedResolutionStateSource, error) {
	if database == nil {
		return nil, ErrDatabaseRequired
	}
	return &SQLiteCommittedResolutionStateSource{
		database: database,
		schema:   newSQLiteSchemaGate(database),
	}, nil
}

type storedCommittedRedirect struct {
	EventRef              string `json:"event_ref"`
	CommitRef             string `json:"commit_ref"`
	ReconciliationRef     string `json:"reconciliation_ref"`
	Operation             string `json:"operation"`
	ResolutionKind        string `json:"resolution_kind"`
	Context               string `json:"bounded_context_ref"`
	Source                string `json:"source_entity_id"`
	Target                string `json:"target_entity_id"`
	GraphRevision         int64  `json:"graph_revision"`
	RedirectOrdinal       int64  `json:"redirect_ordinal"`
	MaterializationDigest string `json:"materialization_digest"`
}

type committedRedirect struct {
	eventRef          string
	commitRef         string
	reconciliationRef string
	operation         typedmemory.IdentityReconciliationOperation
	kind              string
	context           typedmemory.BoundedContextRef
	source            typedmemory.EntityID
	target            typedmemory.EntityID
	revision          typedmemory.GraphRevision
	ordinal           uint64
	materialization   typedmemory.SHA256Digest
}

// LoadCommittedResolutionState loads and verifies every reviewed
// reconciliation whose event is committed at the requested current graph
// basis. It never falls back to a different graph revision or TypeEnv.
func (service *SQLiteService) LoadCommittedResolutionState(
	ctx context.Context,
	basis CommittedResolutionBasis,
) (CommittedResolutionState, error) {
	if service == nil {
		return CommittedResolutionState{}, ErrDatabaseRequired
	}
	return loadCommittedResolutionState(
		ctx,
		service.database,
		service.schema,
		basis,
	)
}

func (source *SQLiteCommittedResolutionStateSource) LoadCommittedResolutionState(
	ctx context.Context,
	basis CommittedResolutionBasis,
) (CommittedResolutionState, error) {
	if source == nil {
		return CommittedResolutionState{}, ErrDatabaseRequired
	}
	return loadCommittedResolutionState(
		ctx,
		source.database,
		source.schema,
		basis,
	)
}

func loadCommittedResolutionState(
	ctx context.Context,
	database *sql.DB,
	schema schemaGate,
	basis CommittedResolutionBasis,
) (CommittedResolutionState, error) {
	if ctx == nil {
		return CommittedResolutionState{}, fmt.Errorf(
			"load committed identity-resolution state: context is required",
		)
	}
	if database == nil || schema == nil {
		return CommittedResolutionState{}, ErrDatabaseRequired
	}
	canonical, err := NewCommittedResolutionBasis(
		basis.Project(),
		basis.GraphRevision(),
		basis.TypeEnv(),
	)
	if err != nil || canonical != basis {
		return CommittedResolutionState{}, fmt.Errorf(
			"load committed identity-resolution state: basis is invalid",
		)
	}
	if err := schema.RequireCompatible(ctx); err != nil {
		return CommittedResolutionState{}, fmt.Errorf("%w: %v", ErrSchemaUnavailable, err)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, database)
	if err != nil {
		return CommittedResolutionState{}, fmt.Errorf(
			"begin committed identity-resolution read: %w",
			err,
		)
	}
	state, loadErr := loadCommittedResolutionStateTx(ctx, transaction, basis)
	finish := transaction.Rollback(ctx)
	if loadErr != nil {
		return CommittedResolutionState{}, errors.Join(loadErr, finish.Err())
	}
	if !finish.Succeeded() {
		return CommittedResolutionState{}, finish.Err()
	}
	return state, nil
}

func loadCommittedResolutionStateTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis CommittedResolutionBasis,
) (CommittedResolutionState, error) {
	if err := requireExactCurrentResolutionBasis(ctx, transaction, basis); err != nil {
		return CommittedResolutionState{}, err
	}
	rows, err := loadCommittedRedirectRows(ctx, transaction, basis)
	if err != nil {
		return CommittedResolutionState{}, err
	}
	eventCount, err := loadCommittedReconciliationEventCount(
		ctx,
		transaction,
		basis,
	)
	if err != nil {
		return CommittedResolutionState{}, err
	}
	if committedRedirectEventCount(rows) != eventCount {
		return CommittedResolutionState{}, fmt.Errorf(
			"%w: committed reconciliation event/redirect closure differs",
			ErrStoredIntegrity,
		)
	}
	redirects, err := verifyAndDecodeCommittedRedirects(
		ctx,
		transaction,
		basis,
		rows,
	)
	if err != nil {
		return CommittedResolutionState{}, err
	}
	return newCommittedResolutionState(basis, redirects)
}

func loadCommittedReconciliationEventCount(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis CommittedResolutionBasis,
) (int, error) {
	basisRevision, err := committedBasisRevisionSQLiteValue(basis.GraphRevision())
	if err != nil {
		return 0, err
	}
	var count int64
	err = transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
		FROM typed_memory_identity_reconciliations reconciliation
		JOIN typed_memory_graph_events event
			ON event.project_id = reconciliation.project_id
			AND event.event_ref = reconciliation.event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		WHERE reconciliation.project_id = ?
			AND event.graph_revision <= ?`,
		[]any{basis.Project().String(), basisRevision},
		[]any{&count},
	)
	if err != nil {
		return 0, fmt.Errorf("count committed identity reconciliations: %w", err)
	}
	if count < 0 || count > math.MaxInt {
		return 0, fmt.Errorf(
			"%w: committed reconciliation count is invalid",
			ErrStoredIntegrity,
		)
	}
	return int(count), nil
}

func committedRedirectEventCount(rows []storedCommittedRedirect) int {
	events := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		events[row.EventRef] = struct{}{}
	}
	return len(events)
}

func requireExactCurrentResolutionBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis CommittedResolutionBasis,
) error {
	basisRevision, err := committedBasisRevisionSQLiteValue(basis.GraphRevision())
	if err != nil {
		return err
	}
	var revision int64
	var typeEnvText string
	err = transaction.ScanOne(
		ctx,
		`SELECT graph_revision, active_type_env_ref
		FROM typed_memory_graph_heads WHERE project_id = ?`,
		[]any{basis.Project().String()},
		[]any{&revision, &typeEnvText},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEntityBasisMissing
	}
	if err != nil {
		return fmt.Errorf("load identity-resolution graph head: %w", err)
	}
	if revision != basisRevision {
		return ErrStaleGraphRevision
	}
	if typeEnvText != basis.TypeEnv().String() {
		return ErrActiveTypeEnvChanged
	}
	return nil
}

func loadCommittedRedirectRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis CommittedResolutionBasis,
) ([]storedCommittedRedirect, error) {
	basisRevision, err := committedBasisRevisionSQLiteValue(basis.GraphRevision())
	if err != nil {
		return nil, err
	}
	var encoded string
	err = transaction.ScanOne(
		ctx,
		`SELECT COALESCE(json_group_array(json_object(
			'event_ref', event_ref,
			'commit_ref', commit_ref,
			'reconciliation_ref', reconciliation_ref,
			'operation', operation,
			'resolution_kind', resolution_kind,
			'bounded_context_ref', bounded_context_ref,
			'source_entity_id', source_entity_id,
			'target_entity_id', target_entity_id,
			'graph_revision', graph_revision,
			'redirect_ordinal', redirect_ordinal,
			'materialization_digest', materialization_digest
		)), '[]')
		FROM (
			SELECT reconciliation.event_ref, reconciliation.commit_ref,
				reconciliation.reconciliation_ref, reconciliation.operation,
				redirect.resolution_kind, redirect.bounded_context_ref,
				redirect.source_entity_id, redirect.target_entity_id,
				event.graph_revision, redirect.redirect_ordinal,
				closure.materialization_digest
			FROM typed_memory_identity_reconciliations reconciliation
			JOIN typed_memory_identity_redirects redirect
				ON redirect.project_id = reconciliation.project_id
				AND redirect.event_ref = reconciliation.event_ref
			JOIN typed_memory_identity_reconciliation_closures closure
				ON closure.project_id = reconciliation.project_id
				AND closure.event_ref = reconciliation.event_ref
			JOIN typed_memory_graph_events event
				ON event.project_id = reconciliation.project_id
				AND event.event_ref = reconciliation.event_ref
			JOIN typed_memory_graph_commits commit_record
				ON commit_record.project_id = event.project_id
				AND commit_record.event_ref = event.event_ref
			WHERE reconciliation.project_id = ?
				AND event.graph_revision <= ?
			ORDER BY redirect.bounded_context_ref,
				redirect.source_entity_id,
				event.graph_revision,
				redirect.redirect_ordinal
		)`,
		[]any{basis.Project().String(), basisRevision},
		[]any{&encoded},
	)
	if err != nil {
		return nil, fmt.Errorf("load committed identity redirects: %w", err)
	}
	rows := []storedCommittedRedirect{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return nil, fmt.Errorf("%w: decode committed identity redirects: %v", ErrStoredIntegrity, err)
	}
	return rows, nil
}

func verifyAndDecodeCommittedRedirects(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis CommittedResolutionBasis,
	rows []storedCommittedRedirect,
) ([]committedRedirect, error) {
	verified := make(map[string]VerifiedClosure)
	redirects := make([]committedRedirect, 0, len(rows))
	for _, row := range rows {
		closure, found := verified[row.EventRef]
		if !found {
			loaded, closureFound, err := VerifyCommittedClosure(
				ctx,
				transaction,
				basis.Project(),
				row.EventRef,
			)
			if err != nil {
				return nil, err
			}
			if !closureFound {
				return nil, fmt.Errorf(
					"%w: identity redirect has no reviewed closure",
					ErrStoredIntegrity,
				)
			}
			closure = loaded
			verified[row.EventRef] = loaded
		}
		decoded, err := decodeCommittedRedirect(row, closure, basis)
		if err != nil {
			return nil, err
		}
		redirects = append(redirects, decoded)
	}
	return normalizeCommittedRedirects(redirects)
}

func decodeCommittedRedirect(
	row storedCommittedRedirect,
	closure VerifiedClosure,
	basis CommittedResolutionBasis,
) (committedRedirect, error) {
	contextRef, contextErr := typedmemory.NewBoundedContextRef(row.Context)
	source, sourceErr := typedmemory.NewEntityID(row.Source)
	target, targetErr := typedmemory.NewEntityID(row.Target)
	materialization, materializationErr := typedmemory.NewSHA256Digest(
		row.MaterializationDigest,
	)
	operation := typedmemory.IdentityReconciliationOperation(row.Operation)
	revision, revisionErr := committedRedirectGraphRevision(
		row.GraphRevision,
		basis.GraphRevision(),
	)
	ordinalValid := row.RedirectOrdinal >= 0
	closureMatches := closure.EventRef() == row.EventRef &&
		closure.CommitRef() == row.CommitRef &&
		closure.MaterializationDigest() == materialization
	operationMatches := (operation == typedmemory.ReconciliationMergeEntities &&
		row.ResolutionKind == "merge_redirect") ||
		(operation == typedmemory.ReconciliationSplitEntity &&
			row.ResolutionKind == "split_candidate")
	if contextErr != nil ||
		sourceErr != nil ||
		targetErr != nil ||
		materializationErr != nil ||
		revisionErr != nil ||
		!ordinalValid ||
		!closureMatches ||
		!operationMatches ||
		row.ReconciliationRef == "" {
		return committedRedirect{}, fmt.Errorf(
			"%w: committed identity redirect is invalid",
			ErrStoredIntegrity,
		)
	}
	return committedRedirect{
		eventRef:          row.EventRef,
		commitRef:         row.CommitRef,
		reconciliationRef: row.ReconciliationRef,
		operation:         operation,
		kind:              row.ResolutionKind,
		context:           contextRef,
		source:            source,
		target:            target,
		revision:          revision,
		ordinal:           uint64(row.RedirectOrdinal),
		materialization:   materialization,
	}, nil
}

func committedBasisRevisionSQLiteValue(
	revision typedmemory.GraphRevision,
) (int64, error) {
	value := revision.Value()
	if value > math.MaxInt64 {
		return 0, fmt.Errorf(
			"%w: committed identity-resolution basis revision exceeds SQLite range",
			ErrStoredIntegrity,
		)
	}
	return int64(value), nil
}

func committedRedirectGraphRevision(
	stored int64,
	basis typedmemory.GraphRevision,
) (typedmemory.GraphRevision, error) {
	if stored <= 0 {
		return typedmemory.GraphRevision{}, fmt.Errorf(
			"%w: committed identity redirect revision is invalid",
			ErrStoredIntegrity,
		)
	}
	revision := typedmemory.NewGraphRevision(uint64(stored))
	if revision.Value() > basis.Value() {
		return typedmemory.GraphRevision{}, fmt.Errorf(
			"%w: committed identity redirect revision exceeds the read basis",
			ErrStoredIntegrity,
		)
	}
	return revision, nil
}

func normalizeCommittedRedirects(
	values []committedRedirect,
) ([]committedRedirect, error) {
	result := append([]committedRedirect(nil), values...)
	sort.Slice(result, func(left int, right int) bool {
		return committedRedirectLess(result[left], result[right])
	})
	for index, redirect := range result {
		if index > 0 && committedRedirectKey(result[index-1]) == committedRedirectKey(redirect) {
			return nil, fmt.Errorf("%w: repeated committed identity redirect", ErrStoredIntegrity)
		}
	}
	if err := validateCommittedRedirectGroups(result); err != nil {
		return nil, err
	}
	return result, nil
}

func validateCommittedRedirectGroups(redirects []committedRedirect) error {
	for start := 0; start < len(redirects); {
		end := committedRedirectGroupEnd(redirects, start)
		group := redirects[start:end]
		first := group[0]
		for index, redirect := range group {
			coordinatesMatch := redirect.eventRef == first.eventRef &&
				redirect.commitRef == first.commitRef &&
				redirect.reconciliationRef == first.reconciliationRef &&
				redirect.operation == first.operation &&
				redirect.kind == first.kind &&
				redirect.revision == first.revision &&
				redirect.materialization == first.materialization
			if first.kind == "split_candidate" {
				coordinatesMatch = coordinatesMatch &&
					redirect.ordinal == uint64(index)
			}
			if !coordinatesMatch {
				return fmt.Errorf(
					"%w: one historical identity has conflicting redirect events",
					ErrStoredIntegrity,
				)
			}
		}
		if first.kind == "merge_redirect" && len(group) != 1 {
			return fmt.Errorf("%w: merge redirect has %d targets", ErrStoredIntegrity, len(group))
		}
		if first.kind == "split_candidate" && len(group) < 2 {
			return fmt.Errorf("%w: reviewed split has fewer than two candidates", ErrStoredIntegrity)
		}
		start = end
	}
	for _, redirect := range redirects {
		_, err := resolveCommittedRedirects(redirects, redirect.source, redirect.context)
		if err != nil {
			return err
		}
	}
	return nil
}

func committedRedirectGroupEnd(redirects []committedRedirect, start int) int {
	end := start + 1
	for end < len(redirects) &&
		redirects[end].context == redirects[start].context &&
		redirects[end].source == redirects[start].source {
		end++
	}
	return end
}

func resolveCommittedRedirects(
	redirects []committedRedirect,
	requested typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) (HistoricalResolution, error) {
	current := requested
	history := make([]string, 0)
	visited := map[string]struct{}{requested.String(): {}}
	for {
		group := committedRedirectGroup(redirects, current, contextRef)
		if len(group) == 0 {
			if current == requested {
				return CurrentIdentity{entity: requested, context: contextRef}, nil
			}
			return MergedIdentity{
				requested: requested,
				current:   current,
				context:   contextRef,
				history:   append([]string(nil), history...),
			}, nil
		}
		first := group[0]
		if first.kind == "split_candidate" {
			candidates := make([]typedmemory.EntityID, 0, len(group))
			for _, redirect := range group {
				candidates = append(candidates, redirect.target)
			}
			return SplitIdentityCandidates{
				source:     requested,
				candidates: candidates,
				context:    contextRef,
				history: append(
					append([]string(nil), history...),
					first.reconciliationRef,
				),
			}, nil
		}
		target := first.target
		if _, found := visited[target.String()]; found {
			return nil, fmt.Errorf("%w: identity redirect cycle", ErrStoredIntegrity)
		}
		visited[target.String()] = struct{}{}
		history = append(history, first.reconciliationRef)
		current = target
	}
}

func committedRedirectGroup(
	redirects []committedRedirect,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) []committedRedirect {
	start := sort.Search(len(redirects), func(index int) bool {
		key := redirects[index].context.String() + "\x1f" + redirects[index].source.String()
		wanted := contextRef.String() + "\x1f" + entity.String()
		return key >= wanted
	})
	if start == len(redirects) ||
		redirects[start].context != contextRef ||
		redirects[start].source != entity {
		return nil
	}
	end := committedRedirectGroupEnd(redirects, start)
	return redirects[start:end]
}

func newCommittedResolutionState(
	basis CommittedResolutionBasis,
	redirects []committedRedirect,
) (CommittedResolutionState, error) {
	canonicalBasis, err := NewCommittedResolutionBasis(
		basis.Project(),
		basis.GraphRevision(),
		basis.TypeEnv(),
	)
	if err != nil || canonicalBasis != basis {
		return CommittedResolutionState{}, fmt.Errorf(
			"committed identity-resolution state basis is invalid",
		)
	}
	normalized, err := normalizeCommittedRedirects(redirects)
	if err != nil {
		return CommittedResolutionState{}, err
	}
	fields := []string{
		basis.Project().String(),
		strconv.FormatUint(basis.GraphRevision().Value(), 10),
		basis.TypeEnv().String(),
		strconv.Itoa(len(normalized)),
	}
	for _, redirect := range normalized {
		fields = append(
			fields,
			redirect.eventRef,
			redirect.commitRef,
			redirect.reconciliationRef,
			string(redirect.operation),
			redirect.kind,
			redirect.context.String(),
			redirect.source.String(),
			redirect.target.String(),
			strconv.FormatUint(redirect.revision.Value(), 10),
			strconv.FormatUint(redirect.ordinal, 10),
			redirect.materialization.String(),
		)
	}
	canonicalBytes := canonicalFields(
		"haft.committed-identity-resolution-state.v1",
		fields...,
	)
	return CommittedResolutionState{
		basis:          basis,
		redirects:      append([]committedRedirect(nil), normalized...),
		canonicalBytes: canonicalBytes,
		digest:         digestBytes(canonicalBytes),
	}, nil
}

func committedRedirectKey(redirect committedRedirect) string {
	return redirect.context.String() +
		"\x1f" +
		redirect.source.String() +
		"\x1f" +
		strconv.FormatUint(redirect.ordinal, 10) +
		"\x1f" +
		redirect.target.String()
}

func committedRedirectLess(left committedRedirect, right committedRedirect) bool {
	if left.context != right.context {
		return left.context.String() < right.context.String()
	}
	if left.source != right.source {
		return left.source.String() < right.source.String()
	}
	if left.ordinal != right.ordinal {
		return left.ordinal < right.ordinal
	}
	return left.target.String() < right.target.String()
}
