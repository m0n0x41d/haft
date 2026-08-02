package profileprojection

import "fmt"

const insertDebtEventV3SQL = `
INSERT INTO project_profile_projection_debt_v3 (
    event_id, debt_id, profile_revision_generation,
    admission_id, admission_digest,
    project_root, ledger_revision, profile_payload_digest,
    projection_path, event_kind, reason_code, detail,
    expected_projection_digest, observed_projection_digest,
    supersedes_event_generation, supersedes_event_id, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertDebtEventV4SQL = `
INSERT INTO project_profile_projection_debt_v4 (
    event_id, debt_id, profile_revision_generation,
    admission_id, admission_digest,
    project_root, ledger_revision, profile_payload_digest,
    projection_path, event_kind, reason_code, detail,
    expected_projection_digest, observed_projection_digest,
    supersedes_event_generation, supersedes_event_id, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertDebtEventV5SQL = `
INSERT INTO project_profile_projection_debt_v5 (
    event_id, debt_id, profile_revision_generation,
    admission_id, admission_digest,
    project_root, ledger_revision, profile_payload_digest,
    projection_path, event_kind, reason_code, detail,
    expected_projection_digest, observed_projection_digest,
    supersedes_event_generation, supersedes_event_id, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const legacyV1DebtEventsCTE = `WITH scoped_debt_events AS (
    SELECT 'v1' AS storage_generation,
           'v1' AS profile_revision_generation,
           event_id, debt_id, admission_id, admission_digest,
           project_root, ledger_revision, profile_payload_digest,
           projection_path, event_kind, reason_code, detail,
           expected_projection_digest, observed_projection_digest,
           CASE
               WHEN COALESCE(supersedes_event_id, '') = '' THEN NULL
               ELSE 'v1'
           END AS supersedes_event_generation,
           NULLIF(supersedes_event_id, '') AS supersedes_event_id,
           recorded_at
    FROM project_profile_projection_debt
    UNION ALL
    SELECT 'v2' AS storage_generation,
           profile_revision_generation,
           event_id, debt_id, admission_id, admission_digest,
           project_root, ledger_revision, profile_payload_digest,
           projection_path, event_kind, reason_code, detail,
           expected_projection_digest, observed_projection_digest,
           supersedes_event_generation, supersedes_event_id, recorded_at
    FROM project_profile_projection_debt_v2
    WHERE profile_revision_generation = 'v1'
    UNION ALL
    SELECT 'v3' AS storage_generation,
           profile_revision_generation,
           event_id, debt_id, admission_id, admission_digest,
           project_root, ledger_revision, profile_payload_digest,
           projection_path, event_kind, reason_code, detail,
           expected_projection_digest, observed_projection_digest,
           supersedes_event_generation, supersedes_event_id, recorded_at
    FROM project_profile_projection_debt_v3
    WHERE profile_revision_generation = 'v1'
)`

const v2DebtEventsCTE = `WITH scoped_debt_events AS (
    SELECT 'v2' AS storage_generation,
           profile_revision_generation,
           event_id, debt_id, admission_id, admission_digest,
           project_root, ledger_revision, profile_payload_digest,
           projection_path, event_kind, reason_code, detail,
           expected_projection_digest, observed_projection_digest,
           supersedes_event_generation, supersedes_event_id, recorded_at
    FROM project_profile_projection_debt_v2
    WHERE profile_revision_generation = 'v2'
    UNION ALL
    SELECT 'v3' AS storage_generation,
           profile_revision_generation,
           event_id, debt_id, admission_id, admission_digest,
           project_root, ledger_revision, profile_payload_digest,
           projection_path, event_kind, reason_code, detail,
           expected_projection_digest, observed_projection_digest,
           supersedes_event_generation, supersedes_event_id, recorded_at
    FROM project_profile_projection_debt_v3
    WHERE profile_revision_generation = 'v2'
)`

const v3DebtEventsCTE = `WITH scoped_debt_events AS (
    SELECT 'v3' AS storage_generation,
           profile_revision_generation,
           event_id, debt_id, admission_id, admission_digest,
           project_root, ledger_revision, profile_payload_digest,
           projection_path, event_kind, reason_code, detail,
           expected_projection_digest, observed_projection_digest,
           supersedes_event_generation, supersedes_event_id, recorded_at
    FROM project_profile_projection_debt_v3
    WHERE profile_revision_generation = 'v3'
)`

const v4DebtEventsCTE = `WITH scoped_debt_events AS (
    SELECT 'v4' AS storage_generation,
           profile_revision_generation,
           event_id, debt_id, admission_id, admission_digest,
           project_root, ledger_revision, profile_payload_digest,
           projection_path, event_kind, reason_code, detail,
           expected_projection_digest, observed_projection_digest,
           supersedes_event_generation, supersedes_event_id, recorded_at
    FROM project_profile_projection_debt_v4
    WHERE profile_revision_generation = 'v4'
)`

const v5DebtEventsCTE = `WITH scoped_debt_events AS (
    SELECT 'v5' AS storage_generation,
           profile_revision_generation,
           event_id, debt_id, admission_id, admission_digest,
           project_root, ledger_revision, profile_payload_digest,
           projection_path, event_kind, reason_code, detail,
           expected_projection_digest, observed_projection_digest,
           supersedes_event_generation, supersedes_event_id, recorded_at
    FROM project_profile_projection_debt_v5
    WHERE profile_revision_generation = 'v5'
)`

type debtEventStorageGeneration string

const (
	debtEventStorageV1 debtEventStorageGeneration = "v1"
	debtEventStorageV2 debtEventStorageGeneration = "v2"
	debtEventStorageV3 debtEventStorageGeneration = "v3"
	debtEventStorageV4 debtEventStorageGeneration = "v4"
	debtEventStorageV5 debtEventStorageGeneration = "v5"
)

type debtEventScope struct {
	admissionGeneration admissionStorageGeneration
}

func newDebtEventScope(source exactAdmissionSource) (debtEventScope, error) {
	if !source.valid() {
		return debtEventScope{}, fmt.Errorf(
			"projection-debt source has unknown admission generation %q",
			source.generation,
		)
	}
	return debtEventScope{admissionGeneration: source.generation}, nil
}

func (scope debtEventScope) statement(body string) (string, error) {
	prefix, err := scope.commonTableExpression()
	if err != nil {
		return "", err
	}
	return prefix + "\n" + body, nil
}

func (scope debtEventScope) commonTableExpression() (string, error) {
	switch scope.admissionGeneration {
	case admissionStorageV1:
		return legacyV1DebtEventsCTE, nil
	case admissionStorageV2:
		return v2DebtEventsCTE, nil
	case admissionStorageV3:
		return v3DebtEventsCTE, nil
	case admissionStorageV4:
		return v4DebtEventsCTE, nil
	case admissionStorageV5:
		return v5DebtEventsCTE, nil
	default:
		return "", fmt.Errorf(
			"projection-debt scope has unknown admission generation %q",
			scope.admissionGeneration,
		)
	}
}

func (scope debtEventScope) acceptsOpenedStorage(
	storage debtEventStorageGeneration,
) bool {
	if scope.admissionGeneration == admissionStorageV1 {
		return storage == debtEventStorageV1 ||
			storage == debtEventStorageV2 ||
			storage == debtEventStorageV3
	}
	if scope.admissionGeneration == admissionStorageV2 {
		return storage == debtEventStorageV2 || storage == debtEventStorageV3
	}
	if scope.admissionGeneration == admissionStorageV3 {
		return storage == debtEventStorageV3
	}
	if scope.admissionGeneration == admissionStorageV4 {
		return storage == debtEventStorageV4
	}
	return scope.admissionGeneration == admissionStorageV5 &&
		storage == debtEventStorageV5
}

func (scope debtEventScope) invalidOwnershipStatement() (
	string,
	debtOwnershipArgumentMode,
	error,
) {
	wrongV2 := `SELECT COUNT(*)
FROM project_profile_projection_debt_v2
WHERE project_root = ?
  AND ledger_revision = ?
  AND admission_id = ?
  AND admission_digest = ?
  AND profile_payload_digest = ?
  AND projection_path = ?
	  AND expected_projection_digest = ?
	  AND profile_revision_generation != '` + string(scope.admissionGeneration) + `'`
	wrongV3 := `SELECT COUNT(*)
FROM project_profile_projection_debt_v3
WHERE project_root = ?
  AND ledger_revision = ?
  AND admission_id = ?
  AND admission_digest = ?
  AND profile_payload_digest = ?
  AND projection_path = ?
	  AND expected_projection_digest = ?
	  AND profile_revision_generation != '` + string(scope.admissionGeneration) + `'`
	wrongV4 := `SELECT COUNT(*)
FROM project_profile_projection_debt_v4
WHERE project_root = ?
  AND ledger_revision = ?
  AND admission_id = ?
  AND admission_digest = ?
  AND profile_payload_digest = ?
  AND projection_path = ?
	  AND expected_projection_digest = ?
	  AND profile_revision_generation != '` + string(scope.admissionGeneration) + `'`
	wrongV5 := `SELECT COUNT(*)
FROM project_profile_projection_debt_v5
WHERE project_root = ?
  AND ledger_revision = ?
  AND admission_id = ?
  AND admission_digest = ?
  AND profile_payload_digest = ?
  AND projection_path = ?
	  AND expected_projection_digest = ?
	  AND profile_revision_generation != '` + string(scope.admissionGeneration) + `'`
	if scope.admissionGeneration == admissionStorageV1 {
		statement := "SELECT (" + wrongV2 + ") + (" + wrongV3 + ")"
		return statement, debtOwnershipArgumentsTwice, nil
	}
	if scope.admissionGeneration != admissionStorageV2 &&
		scope.admissionGeneration != admissionStorageV3 &&
		scope.admissionGeneration != admissionStorageV4 &&
		scope.admissionGeneration != admissionStorageV5 {
		return "", 0, fmt.Errorf(
			"projection-debt ownership has unknown admission generation %q",
			scope.admissionGeneration,
		)
	}
	legacy := `SELECT COUNT(*)
FROM project_profile_projection_debt
WHERE project_root = ?
  AND ledger_revision = ?
  AND admission_id = ?
  AND admission_digest = ?
  AND profile_payload_digest = ?
  AND projection_path = ?
	  AND expected_projection_digest = ?`
	if scope.admissionGeneration == admissionStorageV4 {
		statement := "SELECT (" + legacy + ") + (" + wrongV2 + ") + (" + wrongV3 + ") + (" + wrongV4 + ")"
		return statement, debtOwnershipArgumentsFourTimes, nil
	}
	if scope.admissionGeneration == admissionStorageV5 {
		statement := "SELECT (" + legacy + ") + (" + wrongV2 + ") + (" + wrongV3 + ") + (" + wrongV4 + ") + (" + wrongV5 + ")"
		return statement, debtOwnershipArgumentsFiveTimes, nil
	}
	statement := "SELECT (" + legacy + ") + (" + wrongV2 + ") + (" + wrongV3 + ")"
	return statement, debtOwnershipArgumentsThrice, nil
}

type debtOwnershipArgumentMode uint8

const (
	debtOwnershipArgumentsTwice debtOwnershipArgumentMode = iota + 1
	debtOwnershipArgumentsThrice
	debtOwnershipArgumentsFourTimes
	debtOwnershipArgumentsFiveTimes
)

func debtOwnershipArguments(
	exact []any,
	mode debtOwnershipArgumentMode,
) []any {
	repetitions := 2
	if mode == debtOwnershipArgumentsThrice {
		repetitions = 3
	}
	if mode == debtOwnershipArgumentsFourTimes {
		repetitions = 4
	}
	if mode == debtOwnershipArgumentsFiveTimes {
		repetitions = 5
	}
	arguments := make([]any, 0, len(exact)*repetitions)
	arguments = append(arguments, exact...)
	arguments = append(arguments, exact...)
	if repetitions >= 3 {
		arguments = append(arguments, exact...)
	}
	if repetitions >= 4 {
		arguments = append(arguments, exact...)
	}
	if repetitions == 5 {
		arguments = append(arguments, exact...)
	}
	return arguments
}
