package profileprojection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"strconv"
	"time"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const (
	selectExactOpenDebtBody = `
SELECT opened.storage_generation, opened.profile_revision_generation,
       opened.event_id, opened.debt_id,
       opened.admission_id, opened.admission_digest,
       opened.project_root, opened.ledger_revision,
       opened.profile_payload_digest, opened.projection_path,
       opened.reason_code, opened.detail,
       opened.expected_projection_digest, opened.observed_projection_digest,
       opened.recorded_at
FROM scoped_debt_events opened
WHERE opened.event_kind = 'opened'
  AND opened.project_root = ?
  AND opened.ledger_revision = ?
  AND opened.admission_id = ?
  AND opened.admission_digest = ?
  AND opened.profile_payload_digest = ?
  AND opened.projection_path = ?
  AND opened.expected_projection_digest = ?
  AND NOT EXISTS (
      SELECT 1
      FROM scoped_debt_events resolved
      WHERE resolved.debt_id = opened.debt_id
        AND resolved.event_kind = 'resolved'
        AND resolved.supersedes_event_generation = opened.storage_generation
        AND resolved.supersedes_event_id = opened.event_id
  )
ORDER BY opened.recorded_at, opened.event_id`

	countExactOpenDebtBody = `
SELECT COUNT(*)
FROM scoped_debt_events opened
WHERE opened.event_kind = 'opened'
  AND opened.project_root = ?
  AND opened.ledger_revision = ?
  AND opened.admission_id = ?
  AND opened.admission_digest = ?
  AND opened.profile_payload_digest = ?
  AND opened.projection_path = ?
  AND opened.expected_projection_digest = ?
  AND NOT EXISTS (
      SELECT 1
      FROM scoped_debt_events resolved
      WHERE resolved.debt_id = opened.debt_id
        AND resolved.event_kind = 'resolved'
        AND resolved.supersedes_event_generation = opened.storage_generation
        AND resolved.supersedes_event_id = opened.event_id
  )`

	countExactDebtCyclesBody = `
SELECT COUNT(*)
FROM scoped_debt_events
WHERE event_kind = 'opened'
  AND project_root = ?
  AND ledger_revision = ?
  AND admission_id = ?
  AND admission_digest = ?
  AND profile_payload_digest = ?
  AND projection_path = ?
  AND expected_projection_digest = ?`
)

type debtRecord struct {
	storageGeneration         debtEventStorageGeneration
	profileRevisionGeneration admissionStorageGeneration
	eventID                   string
	debtID                    string
	admissionID               string
	admissionDigest           string
	projectRoot               string
	ledgerRevision            uint64
	profilePayloadDigest      string
	projectionPath            string
	reasonCode                string
	detail                    string
	expectedProjectionDigest  string
	observedProjectionDigest  string
	recordedAt                string
}

func (service Service) openDebt(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	source exactAdmissionSource,
	expected projection,
	path string,
	observation projectionObservation,
) (debtRecord, error) {
	existing, found, err := scanExactOpenDebt(
		transaction,
		ctx,
		admission,
		source,
		path,
		expected.digest,
	)
	if err != nil {
		return debtRecord{}, err
	}
	if found {
		return existing, nil
	}
	cycle, err := countExactDebtCycles(
		transaction,
		ctx,
		admission,
		source,
		path,
		expected.digest,
	)
	if err != nil {
		return debtRecord{}, err
	}
	debtID := deterministicDebtIdentifier(
		"project-profile-projection-debt",
		admission,
		path,
		expected.digest,
		cycle+1,
	)
	eventID := deterministicEventIdentifier(debtID, "opened")
	record := debtRecordFromAdmission(
		admission,
		source,
		expected,
		path,
		observation,
		debtID,
		eventID,
		service.now(),
	)
	err = insertOpenedDebtEvent(transaction, ctx, record)
	if err != nil {
		return debtRecord{}, err
	}
	reread, found, err := scanExactOpenDebt(
		transaction,
		ctx,
		admission,
		source,
		path,
		expected.digest,
	)
	if err != nil {
		return debtRecord{}, err
	}
	if !found || reread.eventID != eventID || reread.debtID != debtID {
		return debtRecord{}, fmt.Errorf(
			"projection-debt open event failed exact transactional reread",
		)
	}
	return reread, nil
}

func insertOpenedDebtEvent(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	record debtRecord,
) error {
	_, err := transaction.Execute(
		ctx,
		insertDebtEventV3SQL,
		debtOpenArguments(record),
	)
	return err
}

func scanExactOpenDebt(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	source exactAdmissionSource,
	path string,
	expectedDigest projectprofile.ContentDigest,
) (debtRecord, bool, error) {
	scope, err := newDebtEventScope(source)
	if err != nil {
		return debtRecord{}, false, err
	}
	arguments := exactDebtArguments(admission, path, expectedDigest)
	err = validateExactDebtChain(transaction, ctx, scope, arguments)
	if err != nil {
		return debtRecord{}, false, err
	}
	cycle, err := countExactDebtCycles(
		transaction,
		ctx,
		admission,
		source,
		path,
		expectedDigest,
	)
	if err != nil {
		return debtRecord{}, false, err
	}
	err = validateCanonicalDebtIdentities(
		transaction,
		ctx,
		admission,
		scope,
		path,
		expectedDigest,
		cycle,
	)
	if err != nil {
		return debtRecord{}, false, err
	}
	countStatement, err := scope.statement(countExactOpenDebtBody)
	if err != nil {
		return debtRecord{}, false, err
	}
	var count int
	err = transaction.ScanOne(
		ctx,
		countStatement,
		arguments,
		[]any{&count},
	)
	if err != nil {
		return debtRecord{}, false, err
	}
	if count == 0 {
		return debtRecord{}, false, nil
	}
	if count != 1 {
		return debtRecord{}, false, fmt.Errorf(
			"projection-debt chain has %d exact open events",
			count,
		)
	}
	selectStatement, err := scope.statement(selectExactOpenDebtBody)
	if err != nil {
		return debtRecord{}, false, err
	}
	var record debtRecord
	err = transaction.ScanOne(
		ctx,
		selectStatement,
		arguments,
		debtDestinations(&record),
	)
	if err != nil {
		return debtRecord{}, false, fmt.Errorf(
			"projection-debt count/reread contradiction: %w",
			err,
		)
	}
	if !record.validFor(scope) {
		return debtRecord{}, false, fmt.Errorf(
			"projection-debt open event has invalid generation binding",
		)
	}
	expectedDebtID := deterministicDebtIdentifier(
		"project-profile-projection-debt",
		admission,
		path,
		expectedDigest,
		cycle,
	)
	expectedEventID := deterministicEventIdentifier(expectedDebtID, "opened")
	if record.debtID != expectedDebtID || record.eventID != expectedEventID {
		return debtRecord{}, false, fmt.Errorf(
			"projection-debt open event has a non-canonical identity",
		)
	}
	return record, true, nil
}

func (record debtRecord) validFor(scope debtEventScope) bool {
	return record.profileRevisionGeneration == scope.admissionGeneration &&
		scope.acceptsOpenedStorage(record.storageGeneration)
}

func validateCanonicalDebtIdentities(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	scope debtEventScope,
	path string,
	expectedDigest projectprofile.ContentDigest,
	cycleCount uint64,
) error {
	return validateCanonicalDebtCycle(
		transaction,
		ctx,
		admission,
		scope,
		path,
		expectedDigest,
		cycleCount,
		1,
	)
}

func validateCanonicalDebtCycle(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	scope debtEventScope,
	path string,
	expectedDigest projectprofile.ContentDigest,
	cycleCount uint64,
	cycle uint64,
) error {
	if cycle > cycleCount {
		return nil
	}
	debtID := deterministicDebtIdentifier(
		"project-profile-projection-debt",
		admission,
		path,
		expectedDigest,
		cycle,
	)
	openedEventID := deterministicEventIdentifier(debtID, "opened")
	resolvedEventID := deterministicEventIdentifier(debtID, "resolved")
	openedCount, err := countCanonicalOpenedEvent(
		transaction,
		ctx,
		admission,
		scope,
		path,
		expectedDigest,
		debtID,
		openedEventID,
	)
	if err != nil {
		return err
	}
	if openedCount != 1 {
		return fmt.Errorf(
			"projection-debt cycle %d has non-canonical open identity",
			cycle,
		)
	}
	canonicalResolved, totalResolved, err := countCanonicalResolvedEvent(
		transaction,
		ctx,
		admission,
		scope,
		path,
		expectedDigest,
		debtID,
		openedEventID,
		resolvedEventID,
	)
	if err != nil {
		return err
	}
	if totalResolved != canonicalResolved || totalResolved > 1 {
		return fmt.Errorf(
			"projection-debt cycle %d has non-canonical resolution identity",
			cycle,
		)
	}
	if cycle < cycleCount && canonicalResolved != 1 {
		return fmt.Errorf(
			"projection-debt cycle %d was not resolved before the next cycle",
			cycle,
		)
	}
	return validateCanonicalDebtCycle(
		transaction,
		ctx,
		admission,
		scope,
		path,
		expectedDigest,
		cycleCount,
		cycle+1,
	)
}

func countCanonicalOpenedEvent(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	scope debtEventScope,
	path string,
	expectedDigest projectprofile.ContentDigest,
	debtID string,
	eventID string,
) (int, error) {
	body := `SELECT COUNT(*)
FROM scoped_debt_events
WHERE event_kind = 'opened'
  AND project_root = ?
  AND ledger_revision = ?
  AND admission_id = ?
  AND admission_digest = ?
  AND profile_payload_digest = ?
  AND projection_path = ?
  AND expected_projection_digest = ?
  AND debt_id = ?
  AND event_id = ?
  AND supersedes_event_generation IS NULL
  AND supersedes_event_id IS NULL`
	statement, err := scope.statement(body)
	if err != nil {
		return 0, err
	}
	arguments := exactDebtArguments(admission, path, expectedDigest)
	arguments = append(arguments, debtID, eventID)
	var count int
	err = transaction.ScanOne(ctx, statement, arguments, []any{&count})
	return count, err
}

func countCanonicalResolvedEvent(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	scope debtEventScope,
	path string,
	expectedDigest projectprofile.ContentDigest,
	debtID string,
	openedEventID string,
	resolvedEventID string,
) (int, int, error) {
	canonicalBody := `SELECT COUNT(*)
FROM scoped_debt_events opened
JOIN scoped_debt_events resolved
  ON resolved.debt_id = opened.debt_id
 AND resolved.event_kind = 'resolved'
 AND resolved.supersedes_event_generation = opened.storage_generation
 AND resolved.supersedes_event_id = opened.event_id
WHERE opened.event_kind = 'opened'
  AND opened.project_root = ?
  AND opened.ledger_revision = ?
  AND opened.admission_id = ?
  AND opened.admission_digest = ?
  AND opened.profile_payload_digest = ?
  AND opened.projection_path = ?
  AND opened.expected_projection_digest = ?
  AND opened.debt_id = ?
  AND opened.event_id = ?
  AND resolved.event_id = ?
  AND resolved.project_root = opened.project_root
  AND resolved.ledger_revision = opened.ledger_revision
  AND resolved.admission_id = opened.admission_id
  AND resolved.admission_digest = opened.admission_digest
  AND resolved.profile_payload_digest = opened.profile_payload_digest
  AND resolved.projection_path = opened.projection_path
  AND resolved.expected_projection_digest = opened.expected_projection_digest
  AND resolved.observed_projection_digest != ''`
	canonicalStatement, err := scope.statement(canonicalBody)
	if err != nil {
		return 0, 0, err
	}
	arguments := exactDebtArguments(admission, path, expectedDigest)
	arguments = append(arguments, debtID, openedEventID, resolvedEventID)
	var canonicalCount int
	err = transaction.ScanOne(
		ctx,
		canonicalStatement,
		arguments,
		[]any{&canonicalCount},
	)
	if err != nil {
		return 0, 0, err
	}
	totalStatement, err := scope.statement(`SELECT COUNT(*)
FROM scoped_debt_events
WHERE event_kind = 'resolved'
  AND debt_id = ?`)
	if err != nil {
		return 0, 0, err
	}
	var totalCount int
	err = transaction.ScanOne(
		ctx,
		totalStatement,
		[]any{debtID},
		[]any{&totalCount},
	)
	return canonicalCount, totalCount, err
}

type debtChainCheck struct {
	name string
	body string
}

func validateExactDebtChain(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	scope debtEventScope,
	exactArguments []any,
) error {
	err := validateDebtScopeOwnership(
		transaction,
		ctx,
		scope,
		exactArguments,
	)
	if err != nil {
		return err
	}
	checks := []debtChainCheck{
		{
			name: "duplicate open debt identities",
			body: `SELECT COUNT(*)
FROM (
    SELECT opened.debt_id
    FROM scoped_debt_events opened
    WHERE opened.event_kind = 'opened'
      AND opened.project_root = ?
      AND opened.ledger_revision = ?
      AND opened.admission_id = ?
      AND opened.admission_digest = ?
      AND opened.profile_payload_digest = ?
      AND opened.projection_path = ?
      AND opened.expected_projection_digest = ?
    GROUP BY opened.debt_id
    HAVING COUNT(*) != 1
) invalid`,
		},
		{
			name: "multiple resolutions for one open event",
			body: `SELECT COUNT(*)
FROM (
    SELECT opened.event_id
    FROM scoped_debt_events opened
    LEFT JOIN scoped_debt_events resolved
      ON resolved.debt_id = opened.debt_id
     AND resolved.event_kind = 'resolved'
     AND resolved.supersedes_event_generation = opened.storage_generation
     AND resolved.supersedes_event_id = opened.event_id
    WHERE opened.event_kind = 'opened'
      AND opened.project_root = ?
      AND opened.ledger_revision = ?
      AND opened.admission_id = ?
      AND opened.admission_digest = ?
      AND opened.profile_payload_digest = ?
      AND opened.projection_path = ?
      AND opened.expected_projection_digest = ?
    GROUP BY opened.event_id
    HAVING COUNT(resolved.event_id) > 1
) invalid`,
		},
		{
			name: "mismatched resolution lineage",
			body: `SELECT COUNT(*)
FROM scoped_debt_events opened
JOIN scoped_debt_events resolved
  ON resolved.debt_id = opened.debt_id
 AND resolved.event_kind = 'resolved'
WHERE opened.event_kind = 'opened'
  AND opened.project_root = ?
  AND opened.ledger_revision = ?
  AND opened.admission_id = ?
  AND opened.admission_digest = ?
  AND opened.profile_payload_digest = ?
  AND opened.projection_path = ?
  AND opened.expected_projection_digest = ?
  AND (
      COALESCE(resolved.supersedes_event_generation, '') != opened.storage_generation
      OR COALESCE(resolved.supersedes_event_id, '') != opened.event_id
      OR resolved.admission_id != opened.admission_id
      OR resolved.admission_digest != opened.admission_digest
      OR resolved.project_root != opened.project_root
      OR resolved.ledger_revision != opened.ledger_revision
      OR resolved.profile_payload_digest != opened.profile_payload_digest
      OR resolved.projection_path != opened.projection_path
      OR resolved.expected_projection_digest != opened.expected_projection_digest
      OR resolved.observed_projection_digest = ''
  )`,
		},
		{
			name: "opened event with supersedes lineage",
			body: `SELECT COUNT(*)
FROM scoped_debt_events opened
WHERE opened.event_kind = 'opened'
  AND opened.project_root = ?
  AND opened.ledger_revision = ?
  AND opened.admission_id = ?
  AND opened.admission_digest = ?
  AND opened.profile_payload_digest = ?
  AND opened.projection_path = ?
  AND opened.expected_projection_digest = ?
  AND (
      opened.supersedes_event_generation IS NOT NULL
      OR opened.supersedes_event_id IS NOT NULL
  )`,
		},
		{
			name: "orphan resolution event",
			body: `SELECT COUNT(*)
FROM scoped_debt_events resolved
WHERE resolved.event_kind = 'resolved'
  AND resolved.project_root = ?
  AND resolved.ledger_revision = ?
  AND resolved.admission_id = ?
  AND resolved.admission_digest = ?
  AND resolved.profile_payload_digest = ?
  AND resolved.projection_path = ?
  AND resolved.expected_projection_digest = ?
  AND NOT EXISTS (
      SELECT 1
      FROM scoped_debt_events opened
      WHERE opened.event_kind = 'opened'
        AND opened.debt_id = resolved.debt_id
        AND opened.storage_generation = resolved.supersedes_event_generation
        AND opened.event_id = resolved.supersedes_event_id
  )`,
		},
	}
	return runDebtChainChecks(
		transaction,
		ctx,
		scope,
		exactArguments,
		checks,
		0,
	)
}

func validateDebtScopeOwnership(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	scope debtEventScope,
	exactArguments []any,
) error {
	statement, mode, err := scope.invalidOwnershipStatement()
	if err != nil {
		return err
	}
	arguments := debtOwnershipArguments(exactArguments, mode)
	var count int
	err = transaction.ScanOne(ctx, statement, arguments, []any{&count})
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf(
			"projection-debt chain has %d event(s) outside its exact admission generation",
			count,
		)
	}
	return nil
}

func runDebtChainChecks(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	scope debtEventScope,
	exactArguments []any,
	checks []debtChainCheck,
	index int,
) error {
	if index >= len(checks) {
		return nil
	}
	check := checks[index]
	statement, err := scope.statement(check.body)
	if err != nil {
		return err
	}
	var count int
	err = transaction.ScanOne(
		ctx,
		statement,
		exactArguments,
		[]any{&count},
	)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("projection-debt chain has %s", check.name)
	}
	return runDebtChainChecks(
		transaction,
		ctx,
		scope,
		exactArguments,
		checks,
		index+1,
	)
}

func (service Service) resolveExactDebt(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	source exactAdmissionSource,
	opened debtRecord,
	observedDigest projectprofile.ContentDigest,
) error {
	scope, err := newDebtEventScope(source)
	if err != nil {
		return err
	}
	if !opened.validFor(scope) {
		return fmt.Errorf("projection-debt resolution has invalid generation binding")
	}
	eventID := deterministicEventIdentifier(opened.debtID, "resolved")
	resolved := resolvedDebtRecord(opened, observedDigest, eventID, service.now())
	_, err = transaction.Execute(
		ctx,
		insertDebtEventV3SQL,
		debtResolvedArguments(resolved, opened),
	)
	if err != nil {
		return err
	}
	var count int
	err = transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
         FROM project_profile_projection_debt_v3
         WHERE event_id = ?
           AND debt_id = ?
           AND profile_revision_generation = ?
           AND event_kind = 'resolved'
           AND supersedes_event_generation = ?
           AND supersedes_event_id = ?
           AND expected_projection_digest = ?
           AND observed_projection_digest = ?`,
		[]any{
			eventID,
			opened.debtID,
			string(source.generation),
			string(opened.storageGeneration),
			opened.eventID,
			opened.expectedProjectionDigest,
			observedDigest.String(),
		},
		[]any{&count},
	)
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf(
			"projection-debt resolution failed exact transactional reread",
		)
	}
	return nil
}

func countExactDebtCycles(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	source exactAdmissionSource,
	path string,
	expectedDigest projectprofile.ContentDigest,
) (uint64, error) {
	scope, err := newDebtEventScope(source)
	if err != nil {
		return 0, err
	}
	statement, err := scope.statement(countExactDebtCyclesBody)
	if err != nil {
		return 0, err
	}
	var count uint64
	err = transaction.ScanOne(
		ctx,
		statement,
		exactDebtArguments(admission, path, expectedDigest),
		[]any{&count},
	)
	return count, err
}

func exactDebtArguments(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	path string,
	expectedDigest projectprofile.ContentDigest,
) []any {
	return []any{
		admission.ProjectRoot().String(),
		admission.LedgerRevision().Value(),
		admission.AdmissionRecordRef().String(),
		admission.AdmissionRecordDigest().String(),
		admission.PayloadDigest().String(),
		path,
		expectedDigest.String(),
	}
}

func debtDestinations(record *debtRecord) []any {
	return []any{
		&record.storageGeneration,
		&record.profileRevisionGeneration,
		&record.eventID,
		&record.debtID,
		&record.admissionID,
		&record.admissionDigest,
		&record.projectRoot,
		&record.ledgerRevision,
		&record.profilePayloadDigest,
		&record.projectionPath,
		&record.reasonCode,
		&record.detail,
		&record.expectedProjectionDigest,
		&record.observedProjectionDigest,
		&record.recordedAt,
	}
}

func debtRecordFromAdmission(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	source exactAdmissionSource,
	expected projection,
	path string,
	observation projectionObservation,
	debtID string,
	eventID string,
	recordedAt time.Time,
) debtRecord {
	return debtRecord{
		storageGeneration:         debtEventStorageV3,
		profileRevisionGeneration: source.generation,
		eventID:                   eventID,
		debtID:                    debtID,
		admissionID:               admission.AdmissionRecordRef().String(),
		admissionDigest:           admission.AdmissionRecordDigest().String(),
		projectRoot:               admission.ProjectRoot().String(),
		ledgerRevision:            admission.LedgerRevision().Value(),
		profilePayloadDigest:      admission.PayloadDigest().String(),
		projectionPath:            path,
		reasonCode:                debtReason(observation),
		detail:                    observation.detail,
		expectedProjectionDigest:  expected.digest.String(),
		observedProjectionDigest:  observation.digest.String(),
		recordedAt:                recordedAt.UTC().Format(time.RFC3339Nano),
	}
}

func resolvedDebtRecord(
	opened debtRecord,
	observedDigest projectprofile.ContentDigest,
	eventID string,
	recordedAt time.Time,
) debtRecord {
	return debtRecord{
		storageGeneration:         debtEventStorageV3,
		profileRevisionGeneration: opened.profileRevisionGeneration,
		eventID:                   eventID,
		debtID:                    opened.debtID,
		admissionID:               opened.admissionID,
		admissionDigest:           opened.admissionDigest,
		projectRoot:               opened.projectRoot,
		ledgerRevision:            opened.ledgerRevision,
		profilePayloadDigest:      opened.profilePayloadDigest,
		projectionPath:            opened.projectionPath,
		reasonCode:                "projection_verified",
		detail:                    "projection reread exactly matched the corresponding canonical ledger revision",
		expectedProjectionDigest:  opened.expectedProjectionDigest,
		observedProjectionDigest:  observedDigest.String(),
		recordedAt:                recordedAt.UTC().Format(time.RFC3339Nano),
	}
}

func deterministicDebtIdentifier(
	domain string,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	path string,
	expectedDigest projectprofile.ContentDigest,
	cycle uint64,
) string {
	return deterministicIdentifier(
		domain,
		admission.ProjectRoot().String(),
		admission.AdmissionRecordRef().String(),
		admission.AdmissionRecordDigest().String(),
		strconv.FormatUint(admission.LedgerRevision().Value(), 10),
		path,
		expectedDigest.String(),
		strconv.FormatUint(cycle, 10),
	)
}

func deterministicEventIdentifier(debtID string, eventKind string) string {
	return deterministicIdentifier(
		"project-profile-projection-event",
		debtID,
		eventKind,
	)
}

func deterministicIdentifier(domain string, values ...string) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(domain))
	writeDeterministicIdentifierValues(digest, values, 0)
	encoded := hex.EncodeToString(digest.Sum(nil))
	return domain + "-" + encoded
}

func writeDeterministicIdentifierValues(
	digest hash.Hash,
	values []string,
	index int,
) {
	if index >= len(values) {
		return
	}
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(values[index]))
	writeDeterministicIdentifierValues(digest, values, index+1)
}

func debtOpenArguments(record debtRecord) []any {
	return []any{
		record.eventID,
		record.debtID,
		string(record.profileRevisionGeneration),
		record.admissionID,
		record.admissionDigest,
		record.projectRoot,
		record.ledgerRevision,
		record.profilePayloadDigest,
		record.projectionPath,
		"opened",
		record.reasonCode,
		record.detail,
		record.expectedProjectionDigest,
		record.observedProjectionDigest,
		nil,
		nil,
		record.recordedAt,
	}
}

func debtResolvedArguments(record debtRecord, opened debtRecord) []any {
	return []any{
		record.eventID,
		record.debtID,
		string(record.profileRevisionGeneration),
		record.admissionID,
		record.admissionDigest,
		record.projectRoot,
		record.ledgerRevision,
		record.profilePayloadDigest,
		record.projectionPath,
		"resolved",
		record.reasonCode,
		record.detail,
		record.expectedProjectionDigest,
		record.observedProjectionDigest,
		string(opened.storageGeneration),
		opened.eventID,
		record.recordedAt,
	}
}
