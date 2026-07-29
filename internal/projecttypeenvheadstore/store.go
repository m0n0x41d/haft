package projecttypeenvheadstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const maximumSQLiteHeadRevision = uint64(math.MaxInt64)

type Store struct {
	database *sql.DB
}

func New(ctx context.Context, database *sql.DB) (*Store, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if database == nil {
		return nil, ErrStoreRequired
	}
	if err := ensureSchema(ctx, database); err != nil {
		return nil, err
	}
	return &Store{database: database}, nil
}

// LoadCurrentProjectTypeEnvHeadTx reconstructs the exact current head through
// one caller-owned transaction. Exact absence and exact current state are
// returned as the revalidator's non-authorizing observation sum. Any positive
// but partial, non-canonical, or internally inconsistent footprint is an
// integrity failure and never degrades to absence.
func (store *Store) LoadCurrentProjectTypeEnvHeadTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectidentity.ProjectID,
) (
	projecttypeenvstagerevalidation.CurrentProjectTypeEnvHeadObservation,
	error,
) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if err := store.verify(); err != nil {
		return nil, err
	}
	if err := transaction.RequireActive(); err != nil {
		return nil, err
	}
	canonicalProject, err := normalizeProject(project)
	if err != nil {
		return nil, err
	}
	row, err := loadCurrentHeadRow(ctx, transaction, canonicalProject)
	if errors.Is(err, sql.ErrNoRows) {
		return store.loadExactAbsence(ctx, transaction, canonicalProject)
	}
	if err != nil {
		return nil, fmt.Errorf("load current project TypeEnv head: %w", err)
	}
	state, err := decodeStoredHeadRow(row)
	if err != nil {
		return nil, storedHeadIntegrityError(err)
	}
	if state.Project() != canonicalProject {
		return nil, storedHeadIntegrityError(
			fmt.Errorf("current head project differs from requested project"),
		)
	}
	if err := verifyExactCurrentHistory(ctx, transaction, row, state); err != nil {
		return nil, err
	}
	observation, err := projecttypeenvstagerevalidation.NewObservedProjectTypeEnvHead(
		state,
	)
	if err != nil {
		return nil, storedHeadIntegrityError(err)
	}
	return observation, nil
}

// CompareAndSwapGenesisProjectTypeEnvHeadTx writes only the dedicated head
// projection after an exact same-transaction absence reread. It is not a
// selection service, authority use, Work record, receipt, graph commit, or
// transaction finish. The later P8G effect shell must surround this primitive
// with those separately required facts in the same BEGIN IMMEDIATE.
func (store *Store) CompareAndSwapGenesisProjectTypeEnvHeadTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	successor projecttypeenvselection.ProjectTypeEnvHeadState,
) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if err := store.verify(); err != nil {
		return err
	}
	if err := transaction.RequireImmediate(); err != nil {
		return err
	}
	row, err := prepareStoredHeadRow(successor)
	if err != nil {
		return err
	}
	if successor.Revision().Value() != 1 {
		return fmt.Errorf("genesis project TypeEnv head successor must have HeadRevision 1")
	}
	current, err := store.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		transaction,
		successor.Project(),
	)
	if err != nil {
		return err
	}
	switch current.(type) {
	case projecttypeenvstagerevalidation.ObservedNoProjectTypeEnvHead:
	case projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead:
		return fmt.Errorf(
			"%w: Genesis requires exact current head absence",
			ErrProjectTypeEnvHeadCASConflict,
		)
	default:
		return storedHeadIntegrityError(
			fmt.Errorf("current project TypeEnv head observation variant is invalid"),
		)
	}
	result, err := transaction.Execute(
		ctx,
		`INSERT INTO project_typeenv_heads (
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			state_digest,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?)`,
		row.arguments(),
	)
	if err != nil {
		return fmt.Errorf("insert Genesis project TypeEnv head projection: %w", err)
	}
	if err := requireOneHeadRow(result); err != nil {
		return err
	}
	return store.verifyWrittenState(ctx, transaction, successor)
}

// CompareAndSwapTransitionProjectTypeEnvHeadTx advances only the dedicated
// head projection from one exact prior state to its exact structural
// successor. It leaves transaction commit and every authority/effect closure
// to its caller.
func (store *Store) CompareAndSwapTransitionProjectTypeEnvHeadTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	expected projecttypeenvselection.ProjectTypeEnvHeadState,
	successor projecttypeenvselection.ProjectTypeEnvHeadState,
) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if err := store.verify(); err != nil {
		return err
	}
	if err := transaction.RequireImmediate(); err != nil {
		return err
	}
	expectedRow, err := prepareStoredHeadRow(expected)
	if err != nil {
		return err
	}
	successorRow, err := prepareStoredHeadRow(successor)
	if err != nil {
		return err
	}
	if err := verifyTransitionCoordinates(expected, successor); err != nil {
		return err
	}
	current, err := store.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		transaction,
		expected.Project(),
	)
	if err != nil {
		return err
	}
	observed, ok := current.(projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead)
	if !ok {
		return fmt.Errorf(
			"%w: Transition requires an exact current head",
			ErrProjectTypeEnvHeadCASConflict,
		)
	}
	if !sameHeadState(observed.State(), expected) {
		return fmt.Errorf(
			"%w: Transition prior head differs from current storage",
			ErrProjectTypeEnvHeadCASConflict,
		)
	}
	arguments := append(
		successorRow.arguments(),
		expectedRow.project,
		expectedRow.head,
		expectedRow.revision,
		expectedRow.selectedComposite,
		expectedRow.digest,
		expectedRow.canonical,
	)
	result, err := transaction.Execute(
		ctx,
		`UPDATE project_typeenv_heads
		SET project_id = ?,
			head_ref = ?,
			head_revision = ?,
			selected_composite_ref = ?,
			state_digest = ?,
			canonical_bytes = ?
		WHERE project_id = ?
			AND head_ref = ?
			AND head_revision = ?
			AND selected_composite_ref = ?
			AND state_digest = ?
			AND canonical_bytes = ?`,
		arguments,
	)
	if err != nil {
		return fmt.Errorf("advance project TypeEnv head projection: %w", err)
	}
	if err := requireOneHeadRow(result); err != nil {
		return err
	}
	return store.verifyWrittenState(ctx, transaction, successor)
}

func (store *Store) verify() error {
	if store == nil || store.database == nil {
		return ErrStoreRequired
	}
	return nil
}

func (store *Store) loadExactAbsence(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectidentity.ProjectID,
) (
	projecttypeenvstagerevalidation.CurrentProjectTypeEnvHeadObservation,
	error,
) {
	var historyCount int64
	err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
		FROM project_typeenv_head_states
		WHERE project_id = ?`,
		[]any{project.String()},
		[]any{&historyCount},
	)
	if err != nil {
		return nil, fmt.Errorf("inspect project TypeEnv head-state absence: %w", err)
	}
	if historyCount != 0 {
		return nil, storedHeadIntegrityError(
			fmt.Errorf(
				"current head is absent but %d history rows remain",
				historyCount,
			),
		)
	}
	observation, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		return nil, err
	}
	return observation, nil
}

func (store *Store) verifyWrittenState(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	expected projecttypeenvselection.ProjectTypeEnvHeadState,
) error {
	observation, err := store.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		transaction,
		expected.Project(),
	)
	if err != nil {
		return err
	}
	current, ok := observation.(projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead)
	if !ok || !sameHeadState(current.State(), expected) {
		return storedHeadIntegrityError(
			fmt.Errorf("written head state does not reread exactly"),
		)
	}
	return nil
}

type storedHeadRow struct {
	project           string
	head              string
	revision          int64
	selectedComposite string
	digest            string
	canonical         []byte
}

func (row storedHeadRow) arguments() []any {
	return []any{
		row.project,
		row.head,
		row.revision,
		row.selectedComposite,
		row.digest,
		append([]byte(nil), row.canonical...),
	}
}

func prepareStoredHeadRow(
	state projecttypeenvselection.ProjectTypeEnvHeadState,
) (storedHeadRow, error) {
	if err := state.Verify(); err != nil {
		return storedHeadRow{}, fmt.Errorf("verify project TypeEnv head state: %w", err)
	}
	revision, err := sqliteHeadRevision(state.Revision())
	if err != nil {
		return storedHeadRow{}, err
	}
	digest, err := digestHeadState(state.CanonicalBytes())
	if err != nil {
		return storedHeadRow{}, err
	}
	return storedHeadRow{
		project:           state.Project().String(),
		head:              state.Ref().String(),
		revision:          revision,
		selectedComposite: state.SelectedComposite().String(),
		digest:            digest.String(),
		canonical:         state.CanonicalBytes(),
	}, nil
}

func loadCurrentHeadRow(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectidentity.ProjectID,
) (storedHeadRow, error) {
	row := storedHeadRow{}
	err := transaction.ScanOne(
		ctx,
		`SELECT project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			state_digest,
			canonical_bytes
		FROM project_typeenv_heads
		WHERE project_id = ?`,
		[]any{project.String()},
		[]any{
			&row.project,
			&row.head,
			&row.revision,
			&row.selectedComposite,
			&row.digest,
			&row.canonical,
		},
	)
	return row, err
}

func loadHistoryHeadRow(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectidentity.ProjectID,
	revision projecttypeenvselection.HeadRevision,
) (storedHeadRow, error) {
	row := storedHeadRow{}
	storedRevision, err := sqliteHeadRevision(revision)
	if err != nil {
		return row, err
	}
	err = transaction.ScanOne(
		ctx,
		`SELECT project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			state_digest,
			canonical_bytes
		FROM project_typeenv_head_states
		WHERE project_id = ? AND head_revision = ?`,
		[]any{project.String(), storedRevision},
		[]any{
			&row.project,
			&row.head,
			&row.revision,
			&row.selectedComposite,
			&row.digest,
			&row.canonical,
		},
	)
	return row, err
}

func decodeStoredHeadRow(
	row storedHeadRow,
) (projecttypeenvselection.ProjectTypeEnvHeadState, error) {
	project, err := projectidentity.ParseProjectID(row.project)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, err
	}
	head, err := projecttypeenvselection.ParseProjectTypeEnvHeadRef(row.head)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, err
	}
	if row.revision <= 0 {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, fmt.Errorf(
			"stored head revision must be positive",
		)
	}
	revision, err := projecttypeenvselection.NewHeadRevision(
		uint64(row.revision),
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, err
	}
	composite, err := typedmemory.ParseTypeEnvRef(row.selectedComposite)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(row.digest)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, err
	}
	state, err := projecttypeenvselection.DecodeProjectTypeEnvHeadState(
		row.canonical,
	)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, err
	}
	actualDigest, err := digestHeadState(row.canonical)
	if err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, err
	}
	checks := []struct {
		matches bool
		label   string
	}{
		{state.Project() == project, "project"},
		{state.Ref() == head, "head reference"},
		{state.Revision() == revision, "HeadRevision"},
		{state.SelectedComposite() == composite, "selected composite"},
		{digest == actualDigest, "state digest"},
	}
	for _, check := range checks {
		if !check.matches {
			return projecttypeenvselection.ProjectTypeEnvHeadState{}, fmt.Errorf(
				"stored head %s differs from canonical bytes",
				check.label,
			)
		}
	}
	return state, nil
}

func verifyExactCurrentHistory(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	currentRow storedHeadRow,
	currentState projecttypeenvselection.ProjectTypeEnvHeadState,
) error {
	historyRow, err := loadHistoryHeadRow(
		ctx,
		transaction,
		currentState.Project(),
		currentState.Revision(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storedHeadIntegrityError(
			fmt.Errorf("current head has no exact history row"),
		)
	}
	if err != nil {
		return fmt.Errorf("load exact immutable project TypeEnv head state: %w", err)
	}
	historyState, err := decodeStoredHeadRow(historyRow)
	if err != nil {
		return storedHeadIntegrityError(err)
	}
	if !sameStoredHeadRow(currentRow, historyRow) ||
		!sameHeadState(currentState, historyState) {
		return storedHeadIntegrityError(
			fmt.Errorf("current head and exact history row differ"),
		)
	}
	var historyCount int64
	var minimumRevision int64
	var maximumRevision int64
	err = transaction.ScanOne(
		ctx,
		`SELECT COUNT(*),
			COALESCE(MIN(head_revision), 0),
			COALESCE(MAX(head_revision), 0)
		FROM project_typeenv_head_states
		WHERE project_id = ?`,
		[]any{currentState.Project().String()},
		[]any{&historyCount, &minimumRevision, &maximumRevision},
	)
	if err != nil {
		return fmt.Errorf("inspect immutable project TypeEnv head-state spine: %w", err)
	}
	expectedRevision, err := sqliteHeadRevision(currentState.Revision())
	if err != nil {
		return storedHeadIntegrityError(err)
	}
	if historyCount != expectedRevision ||
		minimumRevision != 1 ||
		maximumRevision != expectedRevision {
		return storedHeadIntegrityError(
			fmt.Errorf(
				"immutable head-state spine is not contiguous through revision %d",
				expectedRevision,
			),
		)
	}
	return nil
}

func verifyTransitionCoordinates(
	expected projecttypeenvselection.ProjectTypeEnvHeadState,
	successor projecttypeenvselection.ProjectTypeEnvHeadState,
) error {
	if expected.Project() != successor.Project() ||
		expected.Ref() != successor.Ref() {
		return fmt.Errorf("transition project TypeEnv head coordinate mismatch")
	}
	if expected.Revision().Value() == math.MaxUint64 ||
		successor.Revision().Value() != expected.Revision().Value()+1 {
		return fmt.Errorf("transition project TypeEnv head is not the exact successor")
	}
	return nil
}

func sqliteHeadRevision(
	revision projecttypeenvselection.HeadRevision,
) (int64, error) {
	value := revision.Value()
	if value > maximumSQLiteHeadRevision {
		return 0, ErrHeadRevisionOutOfSQLiteRange
	}
	return int64(value), nil
}

func requireOneHeadRow(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read project TypeEnv head CAS row count: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf(
			"%w: storage changed %d current-head rows",
			ErrProjectTypeEnvHeadCASConflict,
			affected,
		)
	}
	return nil
}

func normalizeProject(
	project projectidentity.ProjectID,
) (projectidentity.ProjectID, error) {
	canonical, err := projectidentity.ParseProjectID(project.String())
	if err != nil || canonical != project {
		return projectidentity.ProjectID{}, fmt.Errorf(
			"project TypeEnv head project is required",
		)
	}
	return canonical, nil
}

func digestHeadState(canonical []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	return typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
}

func sameStoredHeadRow(left storedHeadRow, right storedHeadRow) bool {
	return left.project == right.project &&
		left.head == right.head &&
		left.revision == right.revision &&
		left.selectedComposite == right.selectedComposite &&
		left.digest == right.digest &&
		bytes.Equal(left.canonical, right.canonical)
}

func sameHeadState(
	left projecttypeenvselection.ProjectTypeEnvHeadState,
	right projecttypeenvselection.ProjectTypeEnvHeadState,
) bool {
	return left.Project() == right.Project() &&
		left.Ref() == right.Ref() &&
		left.Revision() == right.Revision() &&
		left.SelectedComposite() == right.SelectedComposite() &&
		bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}

func storedHeadIntegrityError(cause error) error {
	return fmt.Errorf("%w: %v", ErrStoredHeadIntegrity, cause)
}
