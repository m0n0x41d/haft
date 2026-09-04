package projectledger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

const rootRelocationSchemaV1 = "haft.project-ledger-root-relocation/v1"

// PersistedRootState is the verified location lineage for one project ledger.
// GenesisRoot is immutable historical identity evidence; CurrentRoot is the
// only physical root accepted for an ordinary runtime attachment.
type PersistedRootState struct {
	ProjectID       string
	GenesisRoot     string
	CurrentRoot     string
	BindingDigest   string
	CurrentDigest   string
	RelocationCount int
	BoundAt         time.Time
	LastRelocatedAt time.Time
}

// RootRelocation is the canonical append-only record committed by RelocateRoot.
type RootRelocation struct {
	Sequence          int
	ProjectID         string
	FromRoot          string
	ToRoot            string
	PredecessorDigest string
	Digest            string
	CanonicalJSON     []byte
	RelocatedAt       time.Time
}

type rootRelocationDTO struct {
	Schema             string `json:"schema"`
	RelocationSequence int    `json:"relocation_sequence"`
	ProjectID          string `json:"project_id"`
	FromRoot           string `json:"from_root"`
	ToRoot             string `json:"to_root"`
	PredecessorDigest  string `json:"predecessor_digest"`
	RelocatedAt        string `json:"relocated_at"`
}

type rootRelocationRow struct {
	sequence          int
	projectID         string
	fromRoot          string
	toRoot            string
	predecessorDigest string
	relocationDigest  string
	relocationJSON    string
	relocatedAt       string
}

// InspectPersistedRootStateDatabase validates a ledger's binding lineage on a
// caller-owned database handle. It is intended for verified backup inspection;
// it does not establish filesystem attachment or mutation authority.
func InspectPersistedRootStateDatabase(
	ctx context.Context,
	database *sql.DB,
) (PersistedRootState, error) {
	if ctx == nil {
		return PersistedRootState{}, fmt.Errorf(
			"inspect persisted project-root state: context is required",
		)
	}
	if database == nil {
		return PersistedRootState{}, fmt.Errorf(
			"inspect persisted project-root state: database is required",
		)
	}
	return readPersistedRootState(ctx, database)
}

// InspectPersistedRootState validates the immutable genesis binding and every
// available relocation record without requiring the historical directories to
// remain on disk. It performs no mutation and does not establish attachment.
func (handle *Handle) InspectPersistedRootState(
	ctx context.Context,
) (PersistedRootState, error) {
	if handle == nil ||
		handle.database == nil ||
		len(handle.anchors) == 0 ||
		handle.databaseAnchor.file == nil {
		return PersistedRootState{}, fmt.Errorf(
			"project ledger handle is closed",
		)
	}
	if err := validateContext(ctx); err != nil {
		return PersistedRootState{}, err
	}
	if err := verifyAnchoredTopology(handle.anchors); err != nil {
		return PersistedRootState{}, err
	}
	if err := handle.sidecarGeneration.Revalidate(); err != nil {
		return PersistedRootState{}, err
	}
	connection, err := handle.database.Conn(ctx)
	if err != nil {
		return PersistedRootState{}, fmt.Errorf(
			"reserve checked project ledger root-state connection: %w",
			err,
		)
	}
	defer connection.Close()
	if err := verifySQLiteMainDatabaseIdentity(
		ctx,
		connection,
		handle.databaseAnchor,
	); err != nil {
		return PersistedRootState{}, err
	}
	state, err := readPersistedRootState(ctx, connection)
	if err != nil {
		return PersistedRootState{}, err
	}
	if err := handle.sidecarGeneration.Revalidate(); err != nil {
		return PersistedRootState{}, err
	}
	if err := verifyAnchoredTopology(handle.anchors); err != nil {
		return PersistedRootState{}, fmt.Errorf(
			"revalidate project ledger topology after root-state read: %w",
			err,
		)
	}
	return state, nil
}

// RelocateRoot appends one exact successor root to a schema-60 ledger. The
// caller must have opened the handle from that successor root and must name the
// current predecessor root. The predecessor must already be absent so a copied
// project cannot silently replace a still-live attachment.
func (handle *Handle) RelocateRoot(
	ctx context.Context,
	fromRootRaw string,
	at time.Time,
) (RootRelocation, error) {
	if handle == nil || handle.database == nil {
		return RootRelocation{}, fmt.Errorf("project ledger handle is closed")
	}
	if err := validateContext(ctx); err != nil {
		return RootRelocation{}, err
	}
	if at.IsZero() {
		return RootRelocation{}, fmt.Errorf(
			"project-root relocation time is required",
		)
	}
	fromRoot, err := parseRecordedProjectRoot(fromRootRaw)
	if err != nil {
		return RootRelocation{}, fmt.Errorf(
			"parse predecessor project root: %w",
			err,
		)
	}
	if fromRoot.String() == handle.identity.root.String() {
		return RootRelocation{}, fmt.Errorf(
			"project-root relocation requires distinct predecessor and successor roots",
		)
	}
	if _, err := os.Lstat(fromRoot.String()); err == nil {
		return RootRelocation{}, fmt.Errorf(
			"predecessor project root still exists at %s; refusing to select between two live roots",
			fromRoot.String(),
		)
	} else if !os.IsNotExist(err) {
		return RootRelocation{}, fmt.Errorf(
			"inspect predecessor project root: %w",
			err,
		)
	}

	connection, err := handle.database.Conn(ctx)
	if err != nil {
		return RootRelocation{}, fmt.Errorf(
			"reserve project-root relocation connection: %w",
			err,
		)
	}
	defer connection.Close()
	if err := verifyAnchoredTopology(handle.anchors); err != nil {
		return RootRelocation{}, err
	}
	if err := handle.sidecarGeneration.Revalidate(); err != nil {
		return RootRelocation{}, err
	}
	if err := verifySQLiteMainDatabaseIdentity(
		ctx,
		connection,
		handle.databaseAnchor,
	); err != nil {
		return RootRelocation{}, err
	}
	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return RootRelocation{}, fmt.Errorf(
			"begin atomic project-root relocation: %w",
			err,
		)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = connection.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	state, err := readPersistedRootState(ctx, connection)
	if err != nil {
		return RootRelocation{}, err
	}
	if state.ProjectID != handle.identity.id.String() {
		return RootRelocation{}, fmt.Errorf(
			"project ledger is durably bound to id %q, not requested id %q",
			state.ProjectID,
			handle.identity.id.String(),
		)
	}
	if state.CurrentRoot != fromRoot.String() {
		return RootRelocation{}, &BindingRootMismatchError{
			StoredProjectID:    state.ProjectID,
			StoredRoot:         state.CurrentRoot,
			RequestedProjectID: handle.identity.id.String(),
			RequestedRoot:      fromRoot.String(),
		}
	}
	record, err := newRootRelocation(
		state.RelocationCount+1,
		handle.identity.id,
		fromRoot,
		handle.identity.root,
		state.CurrentDigest,
		at,
	)
	if err != nil {
		return RootRelocation{}, err
	}
	_, err = connection.ExecContext(
		ctx,
		`INSERT INTO project_ledger_root_relocations (
			relocation_sequence, project_id, from_root, to_root,
			predecessor_digest, relocation_digest, relocation_json,
			relocated_at
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Sequence,
		record.ProjectID,
		record.FromRoot,
		record.ToRoot,
		record.PredecessorDigest,
		record.Digest,
		string(record.CanonicalJSON),
		record.RelocatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return RootRelocation{}, fmt.Errorf(
			"append project-root relocation: %w",
			err,
		)
	}
	verified, err := readPersistedRootState(ctx, connection)
	if err != nil {
		return RootRelocation{}, fmt.Errorf(
			"verify appended project-root relocation: %w",
			err,
		)
	}
	if verified.CurrentRoot != handle.identity.root.String() ||
		verified.CurrentDigest != record.Digest ||
		verified.RelocationCount != record.Sequence {
		return RootRelocation{}, fmt.Errorf(
			"appended project-root relocation did not become the exact current root",
		)
	}
	if err := verifyAnchoredTopology(handle.anchors); err != nil {
		return RootRelocation{}, fmt.Errorf(
			"revalidate project topology before relocation commit: %w",
			err,
		)
	}
	if err := verifySQLiteMainDatabaseIdentity(
		ctx,
		connection,
		handle.databaseAnchor,
	); err != nil {
		return RootRelocation{}, fmt.Errorf(
			"revalidate project database before relocation commit: %w",
			err,
		)
	}
	if _, err := connection.ExecContext(ctx, "COMMIT"); err != nil {
		return RootRelocation{}, fmt.Errorf(
			"commit project-root relocation: %w",
			err,
		)
	}
	committed = true
	if err := connection.Close(); err != nil {
		return record, errors.Join(
			ErrBindingCommittedTopologyChanged,
			fmt.Errorf("release committed relocation connection: %w", err),
		)
	}
	if err := handle.Revalidate(ctx); err != nil {
		return record, errors.Join(
			ErrBindingCommittedTopologyChanged,
			err,
		)
	}
	return record, nil
}

func readPersistedRootState(
	ctx context.Context,
	database bindingReader,
) (PersistedRootState, error) {
	row := ledgerBindingRow{}
	err := database.QueryRowContext(
		ctx,
		`SELECT binding_slot, project_id, project_root,
			binding_digest, binding_json, bound_at
		 FROM project_ledger_binding WHERE binding_slot = 1`,
	).Scan(
		&row.slot,
		&row.projectID,
		&row.projectRoot,
		&row.bindingDigest,
		&row.bindingJSON,
		&row.boundAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PersistedRootState{}, ErrBindingMissing
	}
	if err != nil {
		return PersistedRootState{}, fmt.Errorf(
			"read durable project ledger binding: %w",
			err,
		)
	}
	projectID, genesisRoot, err := validateLedgerBindingRowCanonical(row)
	if err != nil {
		return PersistedRootState{}, err
	}
	boundAt, _ := time.Parse(time.RFC3339Nano, row.boundAt)
	state := PersistedRootState{
		ProjectID:     projectID.String(),
		GenesisRoot:   genesisRoot.String(),
		CurrentRoot:   genesisRoot.String(),
		BindingDigest: row.bindingDigest,
		CurrentDigest: row.bindingDigest,
		BoundAt:       boundAt,
	}

	var relocationTableCount int
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_schema
		 WHERE type = 'table' AND name = 'project_ledger_root_relocations'`,
	).Scan(&relocationTableCount); err != nil {
		return PersistedRootState{}, fmt.Errorf(
			"inspect project-root relocation schema: %w",
			err,
		)
	}
	if relocationTableCount == 0 {
		return state, nil
	}
	rows, err := database.QueryContext(
		ctx,
		`SELECT relocation_sequence, project_id, from_root, to_root,
			predecessor_digest, relocation_digest, relocation_json,
			relocated_at
		 FROM project_ledger_root_relocations
		 ORDER BY relocation_sequence`,
	)
	if err != nil {
		return PersistedRootState{}, fmt.Errorf(
			"read project-root relocation lineage: %w",
			err,
		)
	}
	defer rows.Close()
	seenRoots := map[string]struct{}{state.GenesisRoot: {}}
	for rows.Next() {
		relocationRow := rootRelocationRow{}
		if err := rows.Scan(
			&relocationRow.sequence,
			&relocationRow.projectID,
			&relocationRow.fromRoot,
			&relocationRow.toRoot,
			&relocationRow.predecessorDigest,
			&relocationRow.relocationDigest,
			&relocationRow.relocationJSON,
			&relocationRow.relocatedAt,
		); err != nil {
			return PersistedRootState{}, fmt.Errorf(
				"scan project-root relocation lineage: %w",
				err,
			)
		}
		record, err := validateRootRelocationRow(relocationRow)
		if err != nil {
			return PersistedRootState{}, fmt.Errorf(
				"verify project-root relocation %d: %w",
				relocationRow.sequence,
				err,
			)
		}
		if record.Sequence != state.RelocationCount+1 ||
			record.ProjectID != state.ProjectID ||
			record.FromRoot != state.CurrentRoot ||
			record.PredecessorDigest != state.CurrentDigest {
			return PersistedRootState{}, fmt.Errorf(
				"project-root relocation %d does not extend the exact predecessor",
				record.Sequence,
			)
		}
		if _, exists := seenRoots[record.ToRoot]; exists {
			return PersistedRootState{}, fmt.Errorf(
				"project-root relocation %d reuses root %q",
				record.Sequence,
				record.ToRoot,
			)
		}
		if record.RelocatedAt.Before(state.BoundAt) ||
			(!state.LastRelocatedAt.IsZero() &&
				record.RelocatedAt.Before(state.LastRelocatedAt)) {
			return PersistedRootState{}, fmt.Errorf(
				"project-root relocation %d time precedes its lineage",
				record.Sequence,
			)
		}
		seenRoots[record.ToRoot] = struct{}{}
		state.CurrentRoot = record.ToRoot
		state.CurrentDigest = record.Digest
		state.RelocationCount = record.Sequence
		state.LastRelocatedAt = record.RelocatedAt
	}
	if err := rows.Err(); err != nil {
		return PersistedRootState{}, fmt.Errorf(
			"read project-root relocation lineage: %w",
			err,
		)
	}
	return state, nil
}

func newRootRelocation(
	sequence int,
	projectID ProjectID,
	fromRoot ProjectRoot,
	toRoot ProjectRoot,
	predecessorDigest string,
	at time.Time,
) (RootRelocation, error) {
	if sequence <= 0 {
		return RootRelocation{}, fmt.Errorf(
			"project-root relocation sequence must be positive",
		)
	}
	if fromRoot.String() == toRoot.String() {
		return RootRelocation{}, fmt.Errorf(
			"project-root relocation roots must be distinct",
		)
	}
	if !validSHA256Ref(predecessorDigest) {
		return RootRelocation{}, fmt.Errorf(
			"project-root relocation predecessor digest is invalid",
		)
	}
	canonicalTime := at.Round(0).UTC()
	dto := rootRelocationDTO{
		Schema:             rootRelocationSchemaV1,
		RelocationSequence: sequence,
		ProjectID:          projectID.String(),
		FromRoot:           fromRoot.String(),
		ToRoot:             toRoot.String(),
		PredecessorDigest:  predecessorDigest,
		RelocatedAt:        canonicalTime.Format(time.RFC3339Nano),
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return RootRelocation{}, fmt.Errorf(
			"encode canonical project-root relocation: %w",
			err,
		)
	}
	digest := sha256.Sum256(canonical)
	return RootRelocation{
		Sequence:          sequence,
		ProjectID:         dto.ProjectID,
		FromRoot:          dto.FromRoot,
		ToRoot:            dto.ToRoot,
		PredecessorDigest: dto.PredecessorDigest,
		Digest:            "sha256:" + hex.EncodeToString(digest[:]),
		CanonicalJSON:     canonical,
		RelocatedAt:       canonicalTime,
	}, nil
}

func validateRootRelocationRow(
	row rootRelocationRow,
) (RootRelocation, error) {
	projectID, err := ParseProjectID(row.projectID)
	if err != nil {
		return RootRelocation{}, err
	}
	fromRoot, err := parseRecordedProjectRoot(row.fromRoot)
	if err != nil {
		return RootRelocation{}, fmt.Errorf("parse predecessor root: %w", err)
	}
	toRoot, err := parseRecordedProjectRoot(row.toRoot)
	if err != nil {
		return RootRelocation{}, fmt.Errorf("parse successor root: %w", err)
	}
	relocatedAt, err := time.Parse(time.RFC3339Nano, row.relocatedAt)
	if err != nil || relocatedAt.Location() != time.UTC {
		return RootRelocation{}, fmt.Errorf(
			"project-root relocation time is not canonical UTC RFC3339Nano",
		)
	}
	record, err := newRootRelocation(
		row.sequence,
		projectID,
		fromRoot,
		toRoot,
		row.predecessorDigest,
		relocatedAt,
	)
	if err != nil {
		return RootRelocation{}, err
	}
	if !bytes.Equal(record.CanonicalJSON, []byte(row.relocationJSON)) ||
		record.Digest != row.relocationDigest {
		return RootRelocation{}, fmt.Errorf(
			"project-root relocation canonical bytes or digest are invalid",
		)
	}
	return record, nil
}

func validSHA256Ref(value string) bool {
	if len(value) != 71 || value[:7] != "sha256:" {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil
}
