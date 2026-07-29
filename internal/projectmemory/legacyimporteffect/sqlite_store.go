package legacyimporteffect

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var _ ImportApplyStore = (*SQLiteStore)(nil)
var _ ImportApplyTransaction = (*sqliteImportTransaction)(nil)

// SQLiteStore is the dormant kernel adapter for one already-migrated v50
// ledger. Opening it never creates, migrates, repairs, selects, or activates
// project state.
type SQLiteStore struct {
	database *sql.DB
	heads    *projecttypeenvheadstore.Store
	closures CommittedClosureReader
}

// CommittedClosureReader is the narrow read capability already implemented by
// projecttypeenvselectioneffect/sqlite.CurrentCommittedClosureLoader. The
// store independently correlates the returned value with the exact rows in its
// own transaction, so an ad-hoc implementation cannot mint selection proof.
type CommittedClosureReader interface {
	LoadCommittedClosureForCurrentHeadTx(
		context.Context,
		*sqlitetransaction.Transaction,
		projectidentity.ProjectID,
		typedmemory.GraphRevision,
		projecttypeenvselection.ProjectTypeEnvHeadRef,
		projecttypeenvselection.HeadRevision,
		typedmemory.TypeEnvRef,
	) (
		projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
		error,
	)
}

func NewSQLiteStore(
	ctx context.Context,
	database *sql.DB,
	closures CommittedClosureReader,
) (*SQLiteStore, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open legacy import store: context is required")
	}
	if database == nil {
		return nil, fmt.Errorf("open legacy import store: database is required")
	}
	if closures == nil {
		return nil, fmt.Errorf(
			"open legacy import store: committed closure reader is required",
		)
	}
	if err := db.RequireCurrentSchemaReadOnly(ctx, database); err != nil {
		return nil, fmt.Errorf("open legacy import store: %w", err)
	}
	heads, err := projecttypeenvheadstore.OpenReadOnly(ctx, database)
	if err != nil {
		return nil, fmt.Errorf("open legacy import head reader: %w", err)
	}
	return &SQLiteStore{
		database: database,
		heads:    heads,
		closures: closures,
	}, nil
}

func (*SQLiteStore) legacyImportApplyStore() {}

func (store *SQLiteStore) RunImportTransaction(
	ctx context.Context,
	operation func(ImportApplyTransaction) error,
) error {
	if ctx == nil {
		return fmt.Errorf("run legacy import transaction: context is required")
	}
	if store == nil ||
		store.database == nil ||
		store.heads == nil ||
		store.closures == nil {
		return fmt.Errorf("run legacy import transaction: store is unavailable")
	}
	if operation == nil {
		return fmt.Errorf("run legacy import transaction: operation is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, store.database)
	if err != nil {
		return fmt.Errorf("begin legacy import transaction: %w", err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = transaction.Rollback(context.Background()).Err()
		}
	}()
	adapter := &sqliteImportTransaction{
		transaction: transaction,
		heads:       store.heads,
		closures:    store.closures,
	}
	if err := operation(adapter); err != nil {
		finished = true
		rollback := transaction.Rollback(context.Background())
		return errors.Join(err, rollback.Err())
	}
	if err := ctx.Err(); err != nil {
		finished = true
		rollback := transaction.Rollback(context.Background())
		return errors.Join(err, rollback.Err())
	}
	finished = true
	commit := transaction.Commit(context.Background())
	if commit.StatementError() != nil {
		return fmt.Errorf(
			"commit legacy import transaction: %w",
			commit.Err(),
		)
	}
	// COMMIT succeeded. A connection-close failure cannot turn the durable
	// effect back into a rollback, so the effect result remains successful.
	return nil
}

type sqliteImportTransaction struct {
	transaction *sqlitetransaction.Transaction
	heads       *projecttypeenvheadstore.Store
	closures    CommittedClosureReader
}

func (*sqliteImportTransaction) legacyImportApplyTransaction() {}

func (adapter *sqliteImportTransaction) ProbeImportReplay(
	ctx context.Context,
	coordinate ImportApplyCoordinate,
) (ImportReplayProbe, error) {
	if err := adapter.requireImmediate(); err != nil {
		return nil, err
	}
	row := storedImportRun{}
	err := adapter.transaction.ScanOne(
		ctx,
		loadImportRunByKeySQL,
		[]any{
			coordinate.ProjectID().String(),
			coordinate.IdempotencyKey().String(),
		},
		row.destinations(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ImportReplayAbsent{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load legacy import replay: %w", err)
	}
	existingDigest, err := typedmemory.NewSHA256Digest(row.planDigest)
	if err != nil {
		return nil, replayCorrupt("stored plan digest", err)
	}
	if existingDigest != coordinate.PlanDigest() {
		return newImportReplayConflict(existingDigest), nil
	}
	if err := row.verifyCanonicalDigests(); err != nil {
		return nil, err
	}
	receipt, err := DecodeImportReceipt(row.receiptCanonical)
	if err != nil {
		return nil, replayCorrupt("stored receipt bytes", err)
	}
	if err := row.verifyReceipt(receipt); err != nil {
		return nil, err
	}
	if err := adapter.verifyReplayFootprint(ctx, row, receipt); err != nil {
		return nil, err
	}
	trustedRef, err := ParseImportReceiptRef(row.receiptRef)
	if err != nil {
		return nil, replayCorrupt("stored receipt reference", err)
	}
	replay, err := newImportReplayExact(trustedRef, receipt)
	if err != nil {
		return nil, err
	}
	return replay, nil
}

func (adapter *sqliteImportTransaction) verifyReplayFootprint(
	ctx context.Context,
	row storedImportRun,
	receipt ImportReceipt,
) error {
	var carriers int64
	var dispositions int64
	err := adapter.transaction.ScanOne(
		ctx,
		`SELECT
			(SELECT COUNT(*)
			 FROM legacy_import_run_carriers
			 WHERE project_id = ? AND import_receipt_ref = ?),
			(SELECT COUNT(*)
			 FROM legacy_import_dispositions
			 WHERE project_id = ? AND import_receipt_ref = ?)`,
		[]any{
			row.projectID,
			row.receiptRef,
			row.projectID,
			row.receiptRef,
		},
		[]any{&carriers, &dispositions},
	)
	if err != nil {
		return replayCorrupt("load stored import footprint", err)
	}
	expectedCarriers, expectedDispositions, err :=
		legacyImportSQLiteReceiptCounts(receipt)
	if err != nil {
		return replayCorrupt("canonical receipt footprint", err)
	}
	if carriers != expectedCarriers ||
		dispositions != expectedDispositions {
		return replayCorrupt(
			"stored import footprint differs from canonical receipt counts",
			nil,
		)
	}
	return nil
}

func (adapter *sqliteImportTransaction) ResolveSelectedProjectTypeEnv(
	ctx context.Context,
	project projectidentity.ProjectID,
) (SelectedProjectTypeEnvBasis, error) {
	if err := adapter.requireImmediate(); err != nil {
		return SelectedProjectTypeEnvBasis{}, err
	}
	observation, err := adapter.heads.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		adapter.transaction,
		project,
	)
	if err != nil {
		return SelectedProjectTypeEnvBasis{}, err
	}
	observed, present :=
		observation.(projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead)
	if !present {
		return SelectedProjectTypeEnvBasis{}, fmt.Errorf(
			"project %s has no selected ProjectTypeEnvHead",
			project.String(),
		)
	}
	state := observed.State()
	var revisionValue int64
	var activeTypeEnv string
	err = adapter.transaction.ScanOne(
		ctx,
		`SELECT graph_revision, active_type_env_ref
		 FROM typed_memory_graph_heads
		 WHERE project_id = ?`,
		[]any{project.String()},
		[]any{&revisionValue, &activeTypeEnv},
	)
	if err != nil {
		return SelectedProjectTypeEnvBasis{}, fmt.Errorf(
			"load current typed-memory graph head: %w",
			err,
		)
	}
	if revisionValue <= 0 {
		return SelectedProjectTypeEnvBasis{}, fmt.Errorf(
			"current typed-memory graph revision must be positive",
		)
	}
	graphRevision := typedmemory.NewGraphRevision(uint64(revisionValue))
	graphTypeEnv, err := typedmemory.ParseTypeEnvRef(activeTypeEnv)
	if err != nil {
		return SelectedProjectTypeEnvBasis{}, fmt.Errorf(
			"decode current typed-memory TypeEnv: %w",
			err,
		)
	}
	if graphTypeEnv != state.SelectedComposite() {
		return SelectedProjectTypeEnvBasis{}, fmt.Errorf(
			"current typed-memory TypeEnv differs from selected ProjectTypeEnvHead",
		)
	}
	closure, err := adapter.closures.LoadCommittedClosureForCurrentHeadTx(
		ctx,
		adapter.transaction,
		project,
		graphRevision,
		state.Ref(),
		state.Revision(),
		state.SelectedComposite(),
	)
	if err != nil {
		return SelectedProjectTypeEnvBasis{}, err
	}
	if err := verifyLegacyImportCurrentClosure(
		ctx,
		adapter.transaction,
		project,
		graphRevision,
		state.Ref(),
		state.Revision(),
		state.SelectedComposite(),
		closure,
	); err != nil {
		return SelectedProjectTypeEnvBasis{}, err
	}
	return newSelectedProjectTypeEnvBasis(
		selectedProjectTypeEnvBasisInput{
			project:             project,
			headRef:             state.Ref(),
			headRevision:        state.Revision(),
			typeEnvRef:          state.SelectedComposite(),
			graphRevision:       graphRevision,
			selectionReceiptRef: closure.ReceiptRef(),
			selectionClosureRef: closure.Ref(),
		},
	)
}

func verifyLegacyImportCurrentClosure(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectidentity.ProjectID,
	graphRevision typedmemory.GraphRevision,
	headRef projecttypeenvselection.ProjectTypeEnvHeadRef,
	headRevision projecttypeenvselection.HeadRevision,
	selected typedmemory.TypeEnvRef,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) error {
	storedGraphRevision, err := legacyImportSQLiteInteger(
		"current graph revision",
		graphRevision.Value(),
	)
	if err != nil {
		return err
	}
	storedHeadRevision, err := legacyImportSQLiteInteger(
		"current head revision",
		headRevision.Value(),
	)
	if err != nil {
		return err
	}
	var closureRef string
	var closureDigest string
	var closureCanonical []byte
	var receiptRef string
	var receiptDigest string
	var receiptCanonical []byte
	err = transaction.ScanOne(
		ctx,
		`SELECT
			closure.closure_ref,
			closure.closure_digest,
			closure.canonical_bytes,
			receipt.receipt_ref,
			receipt.receipt_digest,
			receipt.canonical_bytes
		 FROM project_typeenv_head_selection_closures closure
		 JOIN project_typeenv_head_selection_receipts receipt
			ON receipt.receipt_ref = closure.receipt_ref
			AND receipt.receipt_digest = closure.receipt_digest
		 JOIN project_typeenv_head_history history
			ON history.project_id = closure.project_id
			AND history.head_ref = closure.head_ref
			AND history.head_revision = closure.head_revision
			AND history.head_state_digest = closure.head_state_digest
			AND history.graph_revision = closure.graph_revision
			AND history.graph_event_ref = closure.graph_event_ref
			AND history.graph_commit_ref = closure.graph_commit_ref
		 WHERE closure.closure_ref = ?
			AND closure.project_id = ?
			AND closure.graph_revision <= ?
			AND closure.head_ref = ?
			AND closure.head_revision = ?
			AND history.selected_composite_ref = ?
		 ORDER BY closure.graph_revision DESC
		LIMIT 1`,
		[]any{
			closure.Ref().String(),
			project.String(),
			storedGraphRevision,
			headRef.String(),
			storedHeadRevision,
			selected.String(),
		},
		[]any{
			&closureRef,
			&closureDigest,
			&closureCanonical,
			&receiptRef,
			&receiptDigest,
			&receiptCanonical,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"correlate current committed TypeEnv selection closure: %w",
			err,
		)
	}
	if err := closure.Verify(); err != nil {
		return fmt.Errorf("verify current TypeEnv selection closure: %w", err)
	}
	receipt, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvHeadSelectionReceiptV1(
			receiptCanonical,
		)
	if err != nil {
		return fmt.Errorf("decode current TypeEnv selection receipt: %w", err)
	}
	if err := receipt.Verify(); err != nil {
		return fmt.Errorf("verify current TypeEnv selection receipt: %w", err)
	}
	successor := closure.SuccessorHead()
	correlated := closure.Ref().String() == closureRef &&
		closure.Digest().String() == closureDigest &&
		bytes.Equal(closure.CanonicalBytes(), closureCanonical) &&
		closure.Project() == project &&
		closure.CommittedGraphRevision().Value() <= graphRevision.Value() &&
		successor.Ref().String() == headRef.String() &&
		successor.Revision().Value() == headRevision.Value() &&
		successor.SelectedComposite() == selected &&
		closure.ReceiptRef().String() == receiptRef &&
		closure.ReceiptDigest().String() == receiptDigest &&
		receipt.Ref() == closure.ReceiptRef() &&
		receipt.Digest() == closure.ReceiptDigest() &&
		receipt.Project() == project &&
		receipt.SuccessorHeadDigest() == closure.SuccessorHeadDigest() &&
		receipt.CommittedGraphRevision() == closure.CommittedGraphRevision()
	if !correlated {
		return fmt.Errorf(
			"current TypeEnv selection closure and receipt are uncorrelated",
		)
	}
	return nil
}

func (adapter *sqliteImportTransaction) AppendOpaqueImport(
	ctx context.Context,
	batch OpaqueImportBatch,
) error {
	if err := adapter.requireImmediate(); err != nil {
		return err
	}
	if err := batch.Plan().Verify(); err != nil {
		return fmt.Errorf("append opaque import plan: %w", err)
	}
	if err := batch.Receipt().verifyCanonicalFields(); err != nil {
		return fmt.Errorf("append opaque import receipt: %w", err)
	}
	if batch.Plan().ProjectID() != batch.Receipt().ProjectID() ||
		batch.SelectedProjectTypeEnv() !=
			batch.Receipt().SelectedProjectTypeEnv() {
		return fmt.Errorf("append opaque import batch is uncorrelated")
	}
	if err := adapter.insertRun(ctx, batch); err != nil {
		return err
	}
	if err := adapter.insertCarriers(ctx, batch); err != nil {
		return err
	}
	if err := adapter.insertDispositions(ctx, batch); err != nil {
		return err
	}
	return adapter.verifyBatchCounts(ctx, batch)
}

func (adapter *sqliteImportTransaction) requireImmediate() error {
	if adapter == nil ||
		adapter.transaction == nil ||
		adapter.heads == nil ||
		adapter.closures == nil {
		return fmt.Errorf("legacy import SQLite transaction is unavailable")
	}
	return adapter.transaction.RequireImmediate()
}

func (adapter *sqliteImportTransaction) insertRun(
	ctx context.Context,
	batch OpaqueImportBatch,
) error {
	plan := batch.Plan()
	receipt := batch.Receipt()
	basis := batch.SelectedProjectTypeEnv()
	headRevision, graphRevision, err :=
		legacyImportSQLiteBasisCoordinates(basis)
	if err != nil {
		return err
	}
	carrierCount, dispositionCount, err :=
		legacyImportSQLiteReceiptCounts(receipt)
	if err != nil {
		return err
	}
	_, err = adapter.transaction.Execute(
		ctx,
		`INSERT INTO legacy_import_runs (
			project_id,
			idempotency_key,
			import_plan_digest,
			dry_run_report_digest,
			source_snapshot_digest,
			selected_head_ref,
			selected_head_revision,
			selected_type_env_ref,
			selected_graph_revision,
			selection_receipt_ref,
			selection_closure_ref,
			import_receipt_ref,
			import_receipt_digest,
			import_plan_canonical,
			dry_run_report_canonical,
			import_receipt_canonical,
			opaque_carrier_count,
			subject_disposition_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			plan.ProjectID().String(),
			receipt.IdempotencyKey().String(),
			plan.Digest().String(),
			plan.DryRunReportDigest().String(),
			plan.SourceSnapshotDigest().String(),
			basis.HeadRef().String(),
			headRevision,
			basis.TypeEnvRef().String(),
			graphRevision,
			basis.SelectionReceiptRef().String(),
			basis.SelectionClosureRef().String(),
			receipt.Ref().String(),
			receipt.Ref().Digest().String(),
			plan.CanonicalBytes(),
			plan.DryRunReportCanonicalBytes(),
			receipt.CanonicalBytes(),
			carrierCount,
			dispositionCount,
		},
	)
	if err != nil {
		return fmt.Errorf("insert legacy import run: %w", err)
	}
	return nil
}

func (adapter *sqliteImportTransaction) insertCarriers(
	ctx context.Context,
	batch OpaqueImportBatch,
) error {
	project := batch.Plan().ProjectID().String()
	receiptRef := batch.Receipt().Ref().String()
	for index, carrier := range batch.Plan().CarrierHistories() {
		identity := ""
		identified, present :=
			carrier.LegacyIdentity().(legacyimport.IdentifiedLegacyCarrier)
		if present {
			identity = identified.Ref().String()
		}
		_, err := adapter.transaction.Execute(
			ctx,
			`INSERT INTO legacy_import_carriers (
				project_id,
				carrier_ref,
				carrier_edition,
				carrier_digest,
				source_coordinate,
				carrier_format,
				exact_bytes,
				legacy_identity_ref
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(project_id, carrier_ref, carrier_edition) DO NOTHING`,
			[]any{
				project,
				carrier.CarrierRef().String(),
				carrier.CarrierEdition().String(),
				carrier.CarrierDigest().String(),
				carrier.SourceCoordinate().String(),
				carrier.CarrierFormat().String(),
				carrier.ExactBytes(),
				nullableText(identity),
			},
		)
		if err != nil {
			return fmt.Errorf("insert legacy carrier %d: %w", index, err)
		}
		if err := adapter.verifyCarrier(ctx, project, carrier, identity); err != nil {
			return fmt.Errorf("verify legacy carrier %d: %w", index, err)
		}
		_, err = adapter.transaction.Execute(
			ctx,
			`INSERT INTO legacy_import_run_carriers (
				project_id,
				import_receipt_ref,
				carrier_ref,
				carrier_edition,
				carrier_digest
			) VALUES (?, ?, ?, ?, ?)`,
			[]any{
				project,
				receiptRef,
				carrier.CarrierRef().String(),
				carrier.CarrierEdition().String(),
				carrier.CarrierDigest().String(),
			},
		)
		if err != nil {
			return fmt.Errorf("link legacy carrier %d to import run: %w", index, err)
		}
	}
	return nil
}

func (adapter *sqliteImportTransaction) verifyCarrier(
	ctx context.Context,
	project string,
	carrier legacyimport.OpaqueCarrierHistory,
	identity string,
) error {
	var digest string
	var coordinate string
	var format string
	var exact []byte
	var storedIdentity sql.NullString
	err := adapter.transaction.ScanOne(
		ctx,
		`SELECT
			carrier_digest,
			source_coordinate,
			carrier_format,
			exact_bytes,
			legacy_identity_ref
		 FROM legacy_import_carriers
		 WHERE project_id = ?
			AND carrier_ref = ?
			AND carrier_edition = ?`,
		[]any{
			project,
			carrier.CarrierRef().String(),
			carrier.CarrierEdition().String(),
		},
		[]any{
			&digest,
			&coordinate,
			&format,
			&exact,
			&storedIdentity,
		},
	)
	if err != nil {
		return err
	}
	expectedIdentity := nullableText(identity)
	matches := digest == carrier.CarrierDigest().String() &&
		coordinate == carrier.SourceCoordinate().String() &&
		format == carrier.CarrierFormat().String() &&
		bytes.Equal(exact, carrier.ExactBytes()) &&
		storedIdentity.Valid == expectedIdentity.Valid &&
		storedIdentity.String == expectedIdentity.String
	if !matches {
		return fmt.Errorf("stored legacy carrier differs from exact history")
	}
	return nil
}

func (adapter *sqliteImportTransaction) insertDispositions(
	ctx context.Context,
	batch OpaqueImportBatch,
) error {
	project := batch.Plan().ProjectID().String()
	receiptRef := batch.Receipt().Ref().String()
	for index, disposition := range batch.Plan().SubjectDispositions() {
		canonical, reason, err := encodeStoredDisposition(disposition)
		if err != nil {
			return fmt.Errorf("encode legacy disposition %d: %w", index, err)
		}
		_, err = adapter.transaction.Execute(
			ctx,
			`INSERT INTO legacy_import_dispositions (
				project_id,
				import_receipt_ref,
				subject_ref,
				classification_kind,
				unresolved_reason,
				canonical_bytes
			) VALUES (?, ?, ?, ?, ?, ?)`,
			[]any{
				project,
				receiptRef,
				disposition.Subject().String(),
				string(disposition.Kind()),
				reason,
				canonical,
			},
		)
		if err != nil {
			return fmt.Errorf("insert legacy disposition %d: %w", index, err)
		}
	}
	return nil
}

func (adapter *sqliteImportTransaction) verifyBatchCounts(
	ctx context.Context,
	batch OpaqueImportBatch,
) error {
	var carrierCount int64
	var dispositionCount int64
	err := adapter.transaction.ScanOne(
		ctx,
		`SELECT
			(SELECT COUNT(*)
			 FROM legacy_import_run_carriers
			 WHERE project_id = ? AND import_receipt_ref = ?),
			(SELECT COUNT(*)
			 FROM legacy_import_dispositions
			 WHERE project_id = ? AND import_receipt_ref = ?)`,
		[]any{
			batch.Plan().ProjectID().String(),
			batch.Receipt().Ref().String(),
			batch.Plan().ProjectID().String(),
			batch.Receipt().Ref().String(),
		},
		[]any{&carrierCount, &dispositionCount},
	)
	if err != nil {
		return fmt.Errorf("verify legacy import batch counts: %w", err)
	}
	expectedCarriers, expectedDispositions, err :=
		legacyImportSQLiteReceiptCounts(batch.Receipt())
	if err != nil {
		return err
	}
	if carrierCount != expectedCarriers ||
		dispositionCount != expectedDispositions {
		return fmt.Errorf("legacy import batch counts differ from receipt")
	}
	return nil
}

type storedImportRun struct {
	projectID           string
	idempotencyKey      string
	planDigest          string
	reportDigest        string
	sourceDigest        string
	headRef             string
	headRevision        int64
	typeEnvRef          string
	graphRevision       int64
	selectionReceiptRef string
	selectionClosureRef string
	receiptRef          string
	receiptDigest       string
	planCanonical       []byte
	reportCanonical     []byte
	receiptCanonical    []byte
	carrierCount        int64
	dispositionCount    int64
}

func (row *storedImportRun) destinations() []any {
	return []any{
		&row.projectID,
		&row.idempotencyKey,
		&row.planDigest,
		&row.reportDigest,
		&row.sourceDigest,
		&row.headRef,
		&row.headRevision,
		&row.typeEnvRef,
		&row.graphRevision,
		&row.selectionReceiptRef,
		&row.selectionClosureRef,
		&row.receiptRef,
		&row.receiptDigest,
		&row.planCanonical,
		&row.reportCanonical,
		&row.receiptCanonical,
		&row.carrierCount,
		&row.dispositionCount,
	}
}

func (row storedImportRun) verifyCanonicalDigests() error {
	planDigest := digestSQLiteBytes(row.planCanonical)
	reportDigest := digestSQLiteBytes(row.reportCanonical)
	receiptDigest := digestSQLiteBytes(row.receiptCanonical)
	matches := planDigest.String() == row.planDigest &&
		reportDigest.String() == row.reportDigest &&
		receiptDigest.String() == row.receiptDigest
	if !matches {
		return replayCorrupt(
			"stored canonical bytes differ from their digests",
			nil,
		)
	}
	return nil
}

func (row storedImportRun) verifyReceipt(receipt ImportReceipt) error {
	basis := receipt.SelectedProjectTypeEnv()
	headRevision, graphRevision, err :=
		legacyImportSQLiteBasisCoordinates(basis)
	if err != nil {
		return replayCorrupt("canonical receipt TypeEnv basis", err)
	}
	carrierCount, dispositionCount, err :=
		legacyImportSQLiteReceiptCounts(receipt)
	if err != nil {
		return replayCorrupt("canonical receipt counts", err)
	}
	matches := row.projectID == receipt.ProjectID().String() &&
		row.idempotencyKey == receipt.IdempotencyKey().String() &&
		row.planDigest == receipt.ImportPlanDigest().String() &&
		row.reportDigest == receipt.DryRunReportDigest().String() &&
		row.sourceDigest == receipt.SourceSnapshotDigest().String() &&
		row.headRef == basis.HeadRef().String() &&
		row.headRevision == headRevision &&
		row.typeEnvRef == basis.TypeEnvRef().String() &&
		row.graphRevision == graphRevision &&
		row.selectionReceiptRef == basis.SelectionReceiptRef().String() &&
		row.selectionClosureRef == basis.SelectionClosureRef().String() &&
		row.receiptRef == receipt.Ref().String() &&
		row.receiptDigest == receipt.Ref().Digest().String() &&
		row.carrierCount == carrierCount &&
		row.dispositionCount == dispositionCount
	if !matches {
		return replayCorrupt("stored row differs from canonical receipt", nil)
	}
	return nil
}

func legacyImportSQLiteBasisCoordinates(
	basis SelectedProjectTypeEnvBasis,
) (int64, int64, error) {
	headRevision, err := legacyImportSQLiteInteger(
		"selected head revision",
		basis.HeadRevision().Value(),
	)
	if err != nil {
		return 0, 0, err
	}
	graphRevision, err := legacyImportSQLiteInteger(
		"selected graph revision",
		basis.GraphRevision().Value(),
	)
	if err != nil {
		return 0, 0, err
	}
	return headRevision, graphRevision, nil
}

func legacyImportSQLiteReceiptCounts(
	receipt ImportReceipt,
) (int64, int64, error) {
	carrierCount, err := legacyImportSQLiteInteger(
		"opaque carrier count",
		receipt.OpaqueCarrierCount(),
	)
	if err != nil {
		return 0, 0, err
	}
	dispositionCount, err := legacyImportSQLiteInteger(
		"subject disposition count",
		receipt.SubjectDispositionCount(),
	)
	if err != nil {
		return 0, 0, err
	}
	return carrierCount, dispositionCount, nil
}

func legacyImportSQLiteInteger(label string, value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("%s exceeds SQLite integer range", label)
	}
	return int64(value), nil // #nosec G115 -- value is bounded by math.MaxInt64 above.
}

const loadImportRunByKeySQL = `
SELECT
	project_id,
	idempotency_key,
	import_plan_digest,
	dry_run_report_digest,
	source_snapshot_digest,
	selected_head_ref,
	selected_head_revision,
	selected_type_env_ref,
	selected_graph_revision,
	selection_receipt_ref,
	selection_closure_ref,
	import_receipt_ref,
	import_receipt_digest,
	import_plan_canonical,
	dry_run_report_canonical,
	import_receipt_canonical,
	opaque_carrier_count,
	subject_disposition_count
FROM legacy_import_runs
WHERE project_id = ? AND idempotency_key = ?
`

type storedDispositionDTO struct {
	Subject      string                 `json:"subject"`
	Kind         string                 `json:"kind"`
	Reason       string                 `json:"unresolved_reason,omitempty"`
	Observations []storedObservationDTO `json:"observations"`
}

type storedObservationDTO struct {
	Kind       string `json:"kind"`
	Subject    string `json:"subject"`
	CarrierRef string `json:"carrier_ref"`
	Edition    string `json:"edition"`
	Digest     string `json:"digest"`
	Source     string `json:"source,omitempty"`
	Target     string `json:"target,omitempty"`
	Label      string `json:"label,omitempty"`
}

func encodeStoredDisposition(
	disposition legacyimport.OpaqueSubjectDisposition,
) ([]byte, string, error) {
	reason := ""
	unresolved, present := disposition.UnresolvedReason()
	if present {
		reason = unresolved.String()
	}
	observations := make(
		[]storedObservationDTO,
		0,
		len(disposition.Observations()),
	)
	for _, observation := range disposition.Observations() {
		dto := storedObservationDTO{
			Subject:    observation.Subject().String(),
			CarrierRef: observation.Carrier().Ref().String(),
			Edition:    observation.Carrier().Edition().String(),
			Digest:     observation.Carrier().Digest().String(),
		}
		switch value := observation.(type) {
		case legacyimport.CarrierObservation:
			dto.Kind = "carrier"
		case legacyimport.AssociationObservation:
			dto.Kind = "association"
			dto.Source = value.Source().String()
			dto.Target = value.Target().String()
			dto.Label = value.Label().String()
		default:
			return nil, "", fmt.Errorf(
				"unsupported legacy observation %T",
				observation,
			)
		}
		observations = append(observations, dto)
	}
	sort.Slice(
		observations,
		func(left, right int) bool {
			leftBytes, _ := json.Marshal(observations[left])
			rightBytes, _ := json.Marshal(observations[right])
			return bytes.Compare(leftBytes, rightBytes) < 0
		},
	)
	canonical, err := json.Marshal(
		storedDispositionDTO{
			Subject:      disposition.Subject().String(),
			Kind:         string(disposition.Kind()),
			Reason:       reason,
			Observations: observations,
		},
	)
	if err != nil {
		return nil, "", err
	}
	return canonical, reason, nil
}

func nullableText(value string) sql.NullString {
	return sql.NullString{
		String: value,
		Valid:  value != "",
	}
}

func digestSQLiteBytes(value []byte) typedmemory.SHA256Digest {
	sum := sha256.Sum256(value)
	digest, _ := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	return digest
}

func replayCorrupt(subject string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", ErrImportReplayCorrupt, subject)
	}
	return fmt.Errorf("%w: %s: %v", ErrImportReplayCorrupt, subject, cause)
}
