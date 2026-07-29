package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	profileauthoritysqlite "github.com/m0n0x41d/haft/internal/profileauthority/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type transactionStarter interface {
	BeginImmediate(context.Context, *sql.DB) (*sqlitetransaction.Transaction, error)
	BeginRead(context.Context, *sql.DB) (*sqlitetransaction.Transaction, error)
}

type canonicalTransactionStarter struct{}

func (canonicalTransactionStarter) BeginImmediate(
	ctx context.Context,
	database *sql.DB,
) (*sqlitetransaction.Transaction, error) {
	return sqlitetransaction.BeginImmediate(ctx, database)
}

func (canonicalTransactionStarter) BeginRead(
	ctx context.Context,
	database *sql.DB,
) (*sqlitetransaction.Transaction, error) {
	return sqlitetransaction.BeginRead(ctx, database)
}

type transactionFinishEvidence struct {
	statementErr error
	cleanupErr   error
	closeErr     error
}

type transactionFinisher interface {
	Commit(context.Context, *sqlitetransaction.Transaction) transactionFinishEvidence
	Rollback(context.Context, *sqlitetransaction.Transaction) transactionFinishEvidence
}

type canonicalTransactionFinisher struct{}

func (canonicalTransactionFinisher) Commit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) transactionFinishEvidence {
	result := transaction.Commit(ctx)
	return transactionFinishEvidence{
		statementErr: result.StatementError(),
		cleanupErr:   result.CleanupError(),
		closeErr:     result.CloseError(),
	}
}

func (canonicalTransactionFinisher) Rollback(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) transactionFinishEvidence {
	result := transaction.Rollback(ctx)
	return transactionFinishEvidence{
		statementErr: result.StatementError(),
		cleanupErr:   result.CleanupError(),
		closeErr:     result.CloseError(),
	}
}

// adapter owns the canonical SQLite effect for one profile admission. The
// value retains only the database handle; every Admit call acquires and closes
// its own dedicated connection.
type adapter struct {
	database      *sql.DB
	starter       transactionStarter
	finisher      transactionFinisher
	authorityGate profileAuthorityGate
	now           func() time.Time
}

// newAdapter verifies the canonical schema. The caller continues to own
// database lifetime; adapter owns each per-call connection and transaction.
func newAdapter(database *sql.DB) (adapter, error) {
	if database == nil {
		return adapter{}, fmt.Errorf("profile-admission SQLite database is required")
	}
	err := database.Ping()
	if err != nil {
		return adapter{}, fmt.Errorf("ping profile-admission SQLite database: %w", err)
	}
	err = verifyCanonicalSchema(database)
	if err != nil {
		return adapter{}, err
	}
	authorityStore, err := profileauthoritysqlite.Open(database)
	if err != nil {
		return adapter{}, fmt.Errorf("open canonical profile authority store: %w", err)
	}
	return adapter{
		database:      database,
		starter:       canonicalTransactionStarter{},
		finisher:      canonicalTransactionFinisher{},
		authorityGate: authorityStore,
		now:           time.Now,
	}, nil
}

func verifyCanonicalSchema(database *sql.DB) error {
	err := kerneldb.RequireCurrentSchemaReadOnly(context.Background(), database)
	if err != nil {
		return fmt.Errorf("profile-admission requires the current kernel schema: %w", err)
	}
	tables := canonicalTransactionTables()
	err = verifyTableSchemas(database, tables, 0)
	if err != nil {
		return err
	}
	err = verifyRequiredTriggers(database, canonicalTransactionTriggers(), 0)
	if err != nil {
		return err
	}
	statements := []string{
		insertAdmissionV3SQL,
		insertAuthorityUseV3SQL,
		insertRevisionV3SQL,
		selectDurableAdmissionSQL,
	}
	return verifyStatements(database, statements, 0)
}

func canonicalTransactionTriggers() []string {
	return []string{
		"project_profile_admissions_v3_revision_cas",
		"project_profile_admissions_v3_exact_sources",
		"project_profile_admissions_v3_no_cross_generation_collision",
		"profile_declaration_authority_uses_v3_exact_sources",
		"profile_declaration_authority_uses_v3_no_cross_generation_collision",
		"project_profile_revisions_v3_exact_admission",
		"project_profile_revisions_v3_no_cross_generation_collision",
		"project_profile_admissions_v3_no_update",
		"project_profile_admissions_v3_no_delete",
		"profile_declaration_authority_uses_v3_no_update",
		"profile_declaration_authority_uses_v3_no_delete",
		"project_profile_revisions_v3_no_update",
		"project_profile_revisions_v3_no_delete",
	}
}

func verifyRequiredTriggers(
	database *sql.DB,
	triggers []string,
	index int,
) error {
	if index >= len(triggers) {
		return nil
	}
	name := triggers[index]
	var count int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?",
		name,
	).Scan(&count)
	if err != nil || count != 1 {
		return fmt.Errorf("profile-admission trigger %s is unavailable", name)
	}
	return verifyRequiredTriggers(database, triggers, index+1)
}

type requiredTableSchema struct {
	name    string
	columns []string
}

func canonicalTransactionTables() []requiredTableSchema {
	return []requiredTableSchema{
		{
			name: "project_profile_admissions_v3",
			columns: []string{
				"admission_id", "action_kind", "project_root", "authority_mode", "resolution_kind",
				"project_binding_digest", "work_input_ref", "work_input_digest",
				"profile_payload_json", "candidate_provenance_json", "candidate_provenance_digest",
				"profile_author_role_assignment_ref", "profile_author_role_assignment_digest",
				"profile_payload_digest", "observed_project_basis_ref", "observed_project_basis_digest",
				"work_record_ref", "work_record_digest", "outcome_assessment_ref", "outcome_assessment_digest",
				"authority_basis_ref", "authority_basis_digest", "authority_resolution_ref",
				"authority_resolution_digest", "receipt_json", "receipt_digest",
				"expected_ledger_revision", "ledger_revision", "single_use_key",
				"admission_request_digest", "admission_json", "admission_digest", "recorded_at",
			},
		},
		{
			name: "profile_declaration_authority_uses_v3",
			columns: []string{
				"use_ref", "use_digest", "project_root", "action_kind", "authority_mode", "resolution_kind",
				"project_binding_digest",
				"authority_resolution_ref", "authority_resolution_digest",
				"authority_basis_ref", "authority_basis_digest",
				"work_input_ref", "work_input_digest", "single_use_key",
				"admission_request_digest", "committed_admission_ref", "committed_admission_digest",
				"canonical_json", "consumed_at", "recorded_at",
			},
		},
		{
			name: "project_profile_revisions_v3",
			columns: []string{
				"project_root", "ledger_revision", "configured_profile_kind", "profile_payload_json",
				"profile_payload_digest", "receipt_json", "receipt_digest", "admission_id",
				"admission_digest", "recorded_at",
			},
		},
	}
}

func verifyTableSchemas(
	database *sql.DB,
	tables []requiredTableSchema,
	index int,
) error {
	if index >= len(tables) {
		return nil
	}
	table := tables[index]
	query := "SELECT name FROM pragma_table_info(?) ORDER BY cid"
	rows, err := database.Query(query, table.name)
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table.name, err)
	}
	columns, readErr := readColumnNames(rows, []string{})
	closeErr := rows.Close()
	if readErr != nil {
		return fmt.Errorf("inspect %s columns: %w", table.name, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s column inspection: %w", table.name, closeErr)
	}
	if !slices.Equal(columns, table.columns) {
		return fmt.Errorf("profile-admission table %s has an unexpected exact column schema", table.name)
	}
	return verifyTableSchemas(database, tables, index+1)
}

func readColumnNames(rows *sql.Rows, values []string) ([]string, error) {
	if !rows.Next() {
		return values, rows.Err()
	}
	var value string
	err := rows.Scan(&value)
	if err != nil {
		return nil, err
	}
	values = append(values, value)
	return readColumnNames(rows, values)
}

func verifyStatements(database *sql.DB, statements []string, index int) error {
	if index >= len(statements) {
		return nil
	}
	statement, err := database.Prepare(statements[index])
	if err != nil {
		return fmt.Errorf("prepare canonical profile-admission SQL %d: %w", index, err)
	}
	closeErr := statement.Close()
	if closeErr != nil {
		return fmt.Errorf("close canonical profile-admission SQL %d: %w", index, closeErr)
	}
	return verifyStatements(database, statements, index+1)
}

func writeFailure(
	posture AdmissionCommitPosture,
	stage effectFailureStage,
) adapterOutcome {
	return newAdapterFailed(posture, stage)
}

func denied(
	code string,
	detail string,
) adapterOutcome {
	if code == "" || detail == "" {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			failureStageDenialContract,
		)
	}
	return newAdapterDenied([]AdmissionDenial{{code: code, detail: detail}})
}

func admitted(
	value canonicalAdmissionMaterial,
	posture CanonicalAdmissionDelivery,
) adapterOutcome {
	return newAdapterAdmitted(value, posture)
}
