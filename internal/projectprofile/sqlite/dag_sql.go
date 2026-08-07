package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	sqlitedriver "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
)

const insertMethodDescriptionSQL = `INSERT INTO profile_onboarding_method_descriptions (
	method_description_ref, described_method_ref, bounded_context_ref, source_revision, edition,
	required_role_ref, required_system_kind, state_plane_ref, affected_ref_kind, effect_witness_rule_ref,
	method_description_json, method_description_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectMethodDescriptionSQL = `SELECT
	method_description_ref, described_method_ref, bounded_context_ref, source_revision, edition,
	required_role_ref, required_system_kind, state_plane_ref, affected_ref_kind, effect_witness_rule_ref,
	method_description_json, method_description_digest, recorded_at
FROM profile_onboarding_method_descriptions WHERE method_description_ref = ?`

const insertMethodContractSQL = `INSERT INTO profile_onboarding_method_contracts (
	method_contract_ref, edition, method_description_ref, method_description_digest, bounded_context_ref,
	role_admission_policy_ref, system_admission_policy_ref, parameter_spec_set_digest,
	accepted_result_kinds_json, required_occurrence_slots_json, occurrence_coverage_rule_refs_json,
	effect_state_witness_rule_ref, acceptance_standard_ref, acceptance_standard_edition,
	holder_equals_executed_within_rule_ref, method_contract_json, method_contract_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectMethodContractSQL = `SELECT
	method_contract_ref, edition, method_description_ref, method_description_digest, bounded_context_ref,
	role_admission_policy_ref, system_admission_policy_ref, parameter_spec_set_digest,
	accepted_result_kinds_json, required_occurrence_slots_json, occurrence_coverage_rule_refs_json,
	effect_state_witness_rule_ref, acceptance_standard_ref, acceptance_standard_edition,
	holder_equals_executed_within_rule_ref, method_contract_json, method_contract_digest, recorded_at
FROM profile_onboarding_method_contracts WHERE method_contract_ref = ?`

const insertSystemAdmissionSQL = `INSERT INTO profile_onboarding_executor_system_admissions (
	system_admission_ref, system_ref, admitted_system_kind, bounded_context_ref, governing_pattern_ref,
	identity_basis_kind, identity_basis_system_ref, identity_basis_kernel_identity,
	identity_basis_kernel_version, identity_basis_designation_ref, identity_basis_designation_digest,
	acting_eligibility_basis_ref, acting_eligibility_basis_digest, session_ref, valid_from, valid_until,
	method_description_ref, method_description_digest, method_contract_ref, method_contract_digest,
	system_admission_policy_ref, system_admission_json, system_admission_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectSystemAdmissionSQL = `SELECT
	system_admission_ref, system_ref, admitted_system_kind, bounded_context_ref, governing_pattern_ref,
	identity_basis_kind, identity_basis_system_ref, identity_basis_kernel_identity,
	identity_basis_kernel_version, identity_basis_designation_ref, identity_basis_designation_digest,
	acting_eligibility_basis_ref, acting_eligibility_basis_digest, session_ref, valid_from, valid_until,
	method_description_ref, method_description_digest, method_contract_ref, method_contract_digest,
	system_admission_policy_ref, system_admission_json, system_admission_digest, recorded_at
FROM profile_onboarding_executor_system_admissions WHERE system_admission_ref = ?`

const insertRoleAdmissionSQL = `INSERT INTO profile_author_role_admissions (
	role_admission_ref, role_ref, bounded_context_ref, governing_pattern_ref,
	method_description_ref, method_description_digest, method_contract_ref, method_contract_digest,
	role_admission_policy_ref, role_admission_json, role_admission_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectRoleAdmissionSQL = `SELECT
	role_admission_ref, role_ref, bounded_context_ref, governing_pattern_ref,
	method_description_ref, method_description_digest, method_contract_ref, method_contract_digest,
	role_admission_policy_ref, role_admission_json, role_admission_digest, recorded_at
FROM profile_author_role_admissions WHERE role_admission_ref = ?`

const insertAssignmentSupportSQL = `INSERT INTO profile_author_assignment_support_carriers (
	assignment_justification_ref, assignment_rule_ref, assignment_rule_statement, bounded_context_ref,
	system_admission_ref, system_admission_digest, role_admission_ref, role_admission_digest,
	assignment_from, assignment_until, method_contract_ref, method_contract_digest,
	assignment_justification_json, assignment_justification_digest,
	assignment_provenance_ref, provenance_justification_ref, provenance_justification_digest,
	session_ref, kernel_identity, kernel_version, runtime_identity, runtime_version,
	provenance_recorded_at, assignment_provenance_json, assignment_provenance_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectAssignmentSupportSQL = `SELECT
	assignment_justification_ref, assignment_rule_ref, assignment_rule_statement, bounded_context_ref,
	system_admission_ref, system_admission_digest, role_admission_ref, role_admission_digest,
	assignment_from, assignment_until, method_contract_ref, method_contract_digest,
	assignment_justification_json, assignment_justification_digest,
	assignment_provenance_ref, provenance_justification_ref, provenance_justification_digest,
	session_ref, kernel_identity, kernel_version, runtime_identity, runtime_version,
	provenance_recorded_at, assignment_provenance_json, assignment_provenance_digest, recorded_at
FROM profile_author_assignment_support_carriers WHERE assignment_justification_ref = ?`

const insertRoleAssignmentSQL = `INSERT INTO profile_author_role_assignments (
	role_assignment_ref, holder_system_ref, admitted_role_ref, bounded_context_ref, valid_from, valid_until,
	system_admission_ref, system_admission_digest, role_admission_ref, role_admission_digest,
	assignment_justification_ref, assignment_justification_digest,
	assignment_provenance_ref, assignment_provenance_digest,
	role_assignment_json, role_assignment_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectRoleAssignmentSQL = `SELECT
	role_assignment_ref, holder_system_ref, admitted_role_ref, bounded_context_ref, valid_from, valid_until,
	system_admission_ref, system_admission_digest, role_admission_ref, role_admission_digest,
	assignment_justification_ref, assignment_justification_digest,
	assignment_provenance_ref, assignment_provenance_digest,
	role_assignment_json, role_assignment_digest, recorded_at
FROM profile_author_role_assignments WHERE role_assignment_ref = ?`

const insertObservedBasisSQL = `INSERT INTO observed_project_bases (
	observed_project_basis_ref, project_root, observation_from, observation_until,
	detector_version, classifier_version, observed_project_basis_json,
	observed_project_basis_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectObservedBasisSQL = `SELECT
	observed_project_basis_ref, project_root, observation_from, observation_until,
	detector_version, classifier_version, observed_project_basis_json,
	observed_project_basis_digest, recorded_at
FROM observed_project_bases WHERE observed_project_basis_ref = ?`

const insertWorkSQL = `INSERT INTO profile_onboarding_work_records (
	work_record_ref, work_ref, project_root, enacts_method_ref,
	method_description_ref, method_description_digest, method_contract_ref, method_contract_digest,
	parameter_bindings_json, performed_by_role_assignment_ref,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest, executed_within_ref,
	work_from, work_until, bounded_context_ref, basis_observation_from, basis_observation_until,
	observed_project_basis_ref, observed_project_basis_digest,
	inputs_json, outputs_json, resources_json, affected_ref_kind, affected_refs_json,
	state_plane_ref, pre_state_ref, post_state_ref, delta_predicate_ref,
	outcome_kind, profile_payload_digest, observed_basis_digest, missing_basis_digest,
	work_record_json, work_record_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectWorkSQL = `SELECT
	work_record_ref, work_ref, project_root, enacts_method_ref,
	method_description_ref, method_description_digest, method_contract_ref, method_contract_digest,
	parameter_bindings_json, performed_by_role_assignment_ref,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest, executed_within_ref,
	work_from, work_until, bounded_context_ref, basis_observation_from, basis_observation_until,
	observed_project_basis_ref, observed_project_basis_digest,
	inputs_json, outputs_json, resources_json, affected_ref_kind, affected_refs_json,
	state_plane_ref, pre_state_ref, post_state_ref, delta_predicate_ref,
	outcome_kind, profile_payload_digest, observed_basis_digest, missing_basis_digest,
	work_record_json, work_record_digest, recorded_at
FROM profile_onboarding_work_records WHERE work_record_ref = ? AND project_root = ?`

const insertEffectSQL = `INSERT INTO profile_onboarding_effects (
	effect_ref, work_record_ref, work_ref, work_record_digest, result_kind, output_ref,
	profile_payload_digest, observed_project_basis_ref, observed_project_basis_digest, missing_basis_digest,
	affected_entity_refs_json, state_plane_ref, pre_state_ref, post_state_ref, delta_predicate_ref,
	evidence_provenance_path_refs_json, effect_json, effect_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectEffectSQL = `SELECT
	effect_ref, work_record_ref, work_ref, work_record_digest, result_kind, output_ref,
	profile_payload_digest, observed_project_basis_ref, observed_project_basis_digest, missing_basis_digest,
	affected_entity_refs_json, state_plane_ref, pre_state_ref, post_state_ref, delta_predicate_ref,
	evidence_provenance_path_refs_json, effect_json, effect_digest, recorded_at
FROM profile_onboarding_effects WHERE effect_ref = ?`

const insertAssessmentSQL = `INSERT INTO profile_onboarding_outcome_assessments (
	outcome_assessment_ref, effect_ref, effect_digest, work_record_ref, work_ref, work_record_digest,
	acceptance_standard_ref, acceptance_standard_edition, comparator_ref, comparator_edition,
	verdict_kind, verdict_reason_ref, missing_basis_digest, evidence_provenance_path_refs_json,
	outcome_assessment_json, outcome_assessment_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const selectAssessmentSQL = `SELECT
	outcome_assessment_ref, effect_ref, effect_digest, work_record_ref, work_ref, work_record_digest,
	acceptance_standard_ref, acceptance_standard_edition, comparator_ref, comparator_edition,
	verdict_kind, verdict_reason_ref, missing_basis_digest, evidence_provenance_path_refs_json,
	outcome_assessment_json, outcome_assessment_digest, recorded_at
FROM profile_onboarding_outcome_assessments WHERE outcome_assessment_ref = ?`

type exactRowSpec[T comparable] struct {
	label             string
	table             string
	insertSQL         string
	arguments         []any
	key               string
	requested         T
	load              func(context.Context, *sqlitetransaction.Transaction, string) (T, error)
	withoutRecordedAt func(T) T
}

func persistExactRow[T comparable](
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	spec exactRowSpec[T],
) error {
	_, insertErr := transaction.Execute(ctx, spec.insertSQL, spec.arguments)
	inserted := insertErr == nil
	if insertErr != nil && !isAppendOnlyIdentityConflict(insertErr, spec.table) {
		return fmt.Errorf("insert %s: %w", spec.label, insertErr)
	}
	actual, err := spec.load(ctx, transaction, spec.key)
	if err != nil {
		return errors.Join(insertErr, err)
	}
	if inserted && actual != spec.requested {
		return fmt.Errorf("new %s changed during persistence", spec.label)
	}
	actualSemantics := spec.withoutRecordedAt(actual)
	requestedSemantics := spec.withoutRecordedAt(spec.requested)
	if actualSemantics != requestedSemantics {
		return fmt.Errorf("%s identity collides with different canonical content", spec.label)
	}
	return nil
}

func isAppendOnlyIdentityConflict(err error, table string) bool {
	var sqliteErr *sqlitedriver.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	if sqliteErr.Code() != sqlitelib.SQLITE_CONSTRAINT_TRIGGER {
		return false
	}
	want := table + " is append-only"
	message := sqliteErr.Error()
	return strings.Contains(message, want)
}

func persistMethodDescription(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row methodDescriptionRow,
) error {
	spec := exactRowSpec[methodDescriptionRow]{
		label:     "profile-onboarding MethodDescription",
		table:     "profile_onboarding_method_descriptions",
		insertSQL: insertMethodDescriptionSQL,
		arguments: methodDescriptionArguments(row),
		key:       row.methodDescriptionRef,
		requested: row,
		load:      loadMethodDescription,
		withoutRecordedAt: func(value methodDescriptionRow) methodDescriptionRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func methodDescriptionArguments(row methodDescriptionRow) []any {
	return []any{
		row.methodDescriptionRef,
		row.describedMethodRef,
		row.boundedContextRef,
		row.sourceRevision,
		row.edition,
		row.requiredRoleRef,
		row.requiredSystemKind,
		row.statePlaneRef,
		row.affectedRefKind,
		row.effectWitnessRuleRef,
		row.canonicalJSON,
		row.digest,
		row.recordedAt,
	}
}

func loadMethodDescription(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
) (methodDescriptionRow, error) {
	var value methodDescriptionRow
	err := transaction.ScanOne(
		ctx,
		selectMethodDescriptionSQL,
		[]any{key},
		[]any{
			&value.methodDescriptionRef,
			&value.describedMethodRef,
			&value.boundedContextRef,
			&value.sourceRevision,
			&value.edition,
			&value.requiredRoleRef,
			&value.requiredSystemKind,
			&value.statePlaneRef,
			&value.affectedRefKind,
			&value.effectWitnessRuleRef,
			&value.canonicalJSON,
			&value.digest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "MethodDescription", key)
}

func persistMethodContract(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row methodContractRow,
) error {
	spec := exactRowSpec[methodContractRow]{
		label:     "profile-onboarding MethodContract",
		table:     "profile_onboarding_method_contracts",
		insertSQL: insertMethodContractSQL,
		arguments: methodContractArguments(row),
		key:       row.methodContractRef,
		requested: row,
		load:      loadMethodContract,
		withoutRecordedAt: func(value methodContractRow) methodContractRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func methodContractArguments(row methodContractRow) []any {
	return []any{
		row.methodContractRef,
		row.edition,
		row.methodDescriptionRef,
		row.methodDescriptionDigest,
		row.boundedContextRef,
		row.roleAdmissionPolicyRef,
		row.systemAdmissionPolicyRef,
		row.parameterSpecSetDigest,
		row.acceptedResultKindsJSON,
		row.requiredOccurrenceSlotsJSON,
		row.occurrenceCoverageRuleRefsJSON,
		row.effectStateWitnessRuleRef,
		row.acceptanceStandardRef,
		row.acceptanceStandardEdition,
		row.holderEqualsExecutedWithinRuleRef,
		row.canonicalJSON,
		row.digest,
		row.recordedAt,
	}
}

func loadMethodContract(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
) (methodContractRow, error) {
	var value methodContractRow
	err := transaction.ScanOne(
		ctx,
		selectMethodContractSQL,
		[]any{key},
		[]any{
			&value.methodContractRef,
			&value.edition,
			&value.methodDescriptionRef,
			&value.methodDescriptionDigest,
			&value.boundedContextRef,
			&value.roleAdmissionPolicyRef,
			&value.systemAdmissionPolicyRef,
			&value.parameterSpecSetDigest,
			&value.acceptedResultKindsJSON,
			&value.requiredOccurrenceSlotsJSON,
			&value.occurrenceCoverageRuleRefsJSON,
			&value.effectStateWitnessRuleRef,
			&value.acceptanceStandardRef,
			&value.acceptanceStandardEdition,
			&value.holderEqualsExecutedWithinRuleRef,
			&value.canonicalJSON,
			&value.digest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "MethodContract", key)
}

func persistSystemAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row systemAdmissionRow,
) error {
	spec := exactRowSpec[systemAdmissionRow]{
		label:     "profile-onboarding executor-system admission",
		table:     "profile_onboarding_executor_system_admissions",
		insertSQL: insertSystemAdmissionSQL,
		arguments: systemAdmissionArguments(row),
		key:       row.ref,
		requested: row,
		load:      loadSystemAdmission,
		withoutRecordedAt: func(value systemAdmissionRow) systemAdmissionRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func systemAdmissionArguments(row systemAdmissionRow) []any {
	return []any{
		row.ref,
		row.systemRef,
		row.admittedSystemKind,
		row.boundedContextRef,
		row.governingPatternRef,
		row.identityBasisKind,
		row.identityBasisSystemRef,
		row.identityBasisKernelIdentity,
		row.identityBasisKernelVersion,
		row.identityBasisDesignationRef,
		row.identityBasisDesignationDigest,
		row.actingEligibilityBasisRef,
		row.actingEligibilityBasisDigest,
		row.sessionRef,
		row.validFrom,
		row.validUntil,
		row.methodDescriptionRef,
		row.methodDescriptionDigest,
		row.methodContractRef,
		row.methodContractDigest,
		row.admissionPolicyRef,
		row.canonicalJSON,
		row.digest,
		row.recordedAt,
	}
}

func loadSystemAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
) (systemAdmissionRow, error) {
	var value systemAdmissionRow
	err := transaction.ScanOne(
		ctx,
		selectSystemAdmissionSQL,
		[]any{key},
		[]any{
			&value.ref,
			&value.systemRef,
			&value.admittedSystemKind,
			&value.boundedContextRef,
			&value.governingPatternRef,
			&value.identityBasisKind,
			&value.identityBasisSystemRef,
			&value.identityBasisKernelIdentity,
			&value.identityBasisKernelVersion,
			&value.identityBasisDesignationRef,
			&value.identityBasisDesignationDigest,
			&value.actingEligibilityBasisRef,
			&value.actingEligibilityBasisDigest,
			&value.sessionRef,
			&value.validFrom,
			&value.validUntil,
			&value.methodDescriptionRef,
			&value.methodDescriptionDigest,
			&value.methodContractRef,
			&value.methodContractDigest,
			&value.admissionPolicyRef,
			&value.canonicalJSON,
			&value.digest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "executor-system admission", key)
}

func persistRoleAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row roleAdmissionRow,
) error {
	spec := exactRowSpec[roleAdmissionRow]{
		label:     "ProfileAuthor role admission",
		table:     "profile_author_role_admissions",
		insertSQL: insertRoleAdmissionSQL,
		arguments: roleAdmissionArguments(row),
		key:       row.ref,
		requested: row,
		load:      loadRoleAdmission,
		withoutRecordedAt: func(value roleAdmissionRow) roleAdmissionRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func roleAdmissionArguments(row roleAdmissionRow) []any {
	return []any{
		row.ref,
		row.roleRef,
		row.boundedContextRef,
		row.governingPatternRef,
		row.methodDescriptionRef,
		row.methodDescriptionDigest,
		row.methodContractRef,
		row.methodContractDigest,
		row.admissionPolicyRef,
		row.canonicalJSON,
		row.digest,
		row.recordedAt,
	}
}

func loadRoleAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
) (roleAdmissionRow, error) {
	var value roleAdmissionRow
	err := transaction.ScanOne(
		ctx,
		selectRoleAdmissionSQL,
		[]any{key},
		[]any{
			&value.ref,
			&value.roleRef,
			&value.boundedContextRef,
			&value.governingPatternRef,
			&value.methodDescriptionRef,
			&value.methodDescriptionDigest,
			&value.methodContractRef,
			&value.methodContractDigest,
			&value.admissionPolicyRef,
			&value.canonicalJSON,
			&value.digest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "ProfileAuthor role admission", key)
}

func persistAssignmentSupport(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row assignmentSupportRow,
) error {
	spec := exactRowSpec[assignmentSupportRow]{
		label:     "ProfileAuthor assignment support carrier",
		table:     "profile_author_assignment_support_carriers",
		insertSQL: insertAssignmentSupportSQL,
		arguments: assignmentSupportArguments(row),
		key:       row.justificationRef,
		requested: row,
		load:      loadAssignmentSupport,
		withoutRecordedAt: func(value assignmentSupportRow) assignmentSupportRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func assignmentSupportArguments(row assignmentSupportRow) []any {
	return []any{
		row.justificationRef,
		row.ruleRef,
		row.ruleStatement,
		row.boundedContextRef,
		row.systemAdmissionRef,
		row.systemAdmissionDigest,
		row.roleAdmissionRef,
		row.roleAdmissionDigest,
		row.assignmentFrom,
		row.assignmentUntil,
		row.methodContractRef,
		row.methodContractDigest,
		row.justificationJSON,
		row.justificationDigest,
		row.provenanceRef,
		row.provenanceJustificationRef,
		row.provenanceJustificationDigest,
		row.sessionRef,
		row.kernelIdentity,
		row.kernelVersion,
		row.runtimeIdentity,
		row.runtimeVersion,
		row.provenanceRecordedAt,
		row.provenanceJSON,
		row.provenanceDigest,
		row.recordedAt,
	}
}

func loadAssignmentSupport(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
) (assignmentSupportRow, error) {
	var value assignmentSupportRow
	err := transaction.ScanOne(
		ctx,
		selectAssignmentSupportSQL,
		[]any{key},
		[]any{
			&value.justificationRef,
			&value.ruleRef,
			&value.ruleStatement,
			&value.boundedContextRef,
			&value.systemAdmissionRef,
			&value.systemAdmissionDigest,
			&value.roleAdmissionRef,
			&value.roleAdmissionDigest,
			&value.assignmentFrom,
			&value.assignmentUntil,
			&value.methodContractRef,
			&value.methodContractDigest,
			&value.justificationJSON,
			&value.justificationDigest,
			&value.provenanceRef,
			&value.provenanceJustificationRef,
			&value.provenanceJustificationDigest,
			&value.sessionRef,
			&value.kernelIdentity,
			&value.kernelVersion,
			&value.runtimeIdentity,
			&value.runtimeVersion,
			&value.provenanceRecordedAt,
			&value.provenanceJSON,
			&value.provenanceDigest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "ProfileAuthor assignment support", key)
}

func exactRowLoadError(err error, label string, key string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s %q is not durably readable", label, key)
	}
	if err != nil {
		return fmt.Errorf("reread %s %q: %w", label, key, err)
	}
	return nil
}

func persistRoleAssignment(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row roleAssignmentRow,
) error {
	spec := exactRowSpec[roleAssignmentRow]{
		label:     "ProfileAuthorRoleAssignment",
		table:     "profile_author_role_assignments",
		insertSQL: insertRoleAssignmentSQL,
		arguments: roleAssignmentArguments(row),
		key:       row.ref,
		requested: row,
		load:      loadRoleAssignment,
		withoutRecordedAt: func(value roleAssignmentRow) roleAssignmentRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func roleAssignmentArguments(row roleAssignmentRow) []any {
	return []any{
		row.ref,
		row.holderSystemRef,
		row.admittedRoleRef,
		row.boundedContextRef,
		row.validFrom,
		row.validUntil,
		row.systemAdmissionRef,
		row.systemAdmissionDigest,
		row.roleAdmissionRef,
		row.roleAdmissionDigest,
		row.justificationRef,
		row.justificationDigest,
		row.provenanceRef,
		row.provenanceDigest,
		row.canonicalJSON,
		row.digest,
		row.recordedAt,
	}
}

func loadRoleAssignment(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
) (roleAssignmentRow, error) {
	var value roleAssignmentRow
	err := transaction.ScanOne(
		ctx,
		selectRoleAssignmentSQL,
		[]any{key},
		[]any{
			&value.ref,
			&value.holderSystemRef,
			&value.admittedRoleRef,
			&value.boundedContextRef,
			&value.validFrom,
			&value.validUntil,
			&value.systemAdmissionRef,
			&value.systemAdmissionDigest,
			&value.roleAdmissionRef,
			&value.roleAdmissionDigest,
			&value.justificationRef,
			&value.justificationDigest,
			&value.provenanceRef,
			&value.provenanceDigest,
			&value.canonicalJSON,
			&value.digest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "ProfileAuthorRoleAssignment", key)
}

func persistObservedBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row observedBasisRow,
) error {
	spec := exactRowSpec[observedBasisRow]{
		label:     "ObservedProjectBasis",
		table:     "observed_project_bases",
		insertSQL: insertObservedBasisSQL,
		arguments: observedBasisArguments(row),
		key:       row.ref,
		requested: row,
		load:      loadObservedBasis,
		withoutRecordedAt: func(value observedBasisRow) observedBasisRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func observedBasisArguments(row observedBasisRow) []any {
	return []any{
		row.ref,
		row.projectRoot,
		row.observationFrom,
		row.observationUntil,
		row.detectorVersion,
		row.classifierVersion,
		row.canonicalJSON,
		row.digest,
		row.recordedAt,
	}
}

func loadObservedBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
) (observedBasisRow, error) {
	var value observedBasisRow
	err := transaction.ScanOne(
		ctx,
		selectObservedBasisSQL,
		[]any{key},
		[]any{
			&value.ref,
			&value.projectRoot,
			&value.observationFrom,
			&value.observationUntil,
			&value.detectorVersion,
			&value.classifierVersion,
			&value.canonicalJSON,
			&value.digest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "ObservedProjectBasis", key)
}

func persistWork(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row workRow,
) error {
	spec := exactRowSpec[workRow]{
		label:     "profile-onboarding Work",
		table:     "profile_onboarding_work_records",
		insertSQL: insertWorkSQL,
		arguments: workArguments(row),
		key:       row.workRecordRef,
		requested: row,
		load: func(ctx context.Context, transaction *sqlitetransaction.Transaction, key string) (workRow, error) {
			return loadWork(ctx, transaction, key, row.projectRoot)
		},
		withoutRecordedAt: func(value workRow) workRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func workArguments(row workRow) []any {
	return []any{
		row.workRecordRef,
		row.workRef,
		row.projectRoot,
		row.enactsMethodRef,
		row.methodDescriptionRef,
		row.methodDescriptionDigest,
		row.methodContractRef,
		row.methodContractDigest,
		row.parameterBindingsJSON,
		row.performedByRoleAssignmentRef,
		row.profileAuthorRoleAssignmentRef,
		row.profileAuthorRoleAssignmentDigest,
		row.executedWithinRef,
		row.workFrom,
		row.workUntil,
		row.boundedContextRef,
		row.basisObservationFrom,
		row.basisObservationUntil,
		row.observedProjectBasisRef,
		row.observedProjectBasisDigest,
		row.inputsJSON,
		row.outputsJSON,
		row.resourcesJSON,
		row.affectedRefKind,
		row.affectedRefsJSON,
		row.statePlaneRef,
		row.preStateRef,
		row.postStateRef,
		row.deltaPredicateRef,
		row.outcomeKind,
		row.profilePayloadDigest,
		row.observedBasisDigest,
		row.missingBasisDigest,
		row.canonicalJSON,
		row.digest,
		row.recordedAt,
	}
}

func loadWork(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
	projectRoot string,
) (workRow, error) {
	var value workRow
	err := transaction.ScanOne(
		ctx,
		selectWorkSQL,
		[]any{key, projectRoot},
		[]any{
			&value.workRecordRef,
			&value.workRef,
			&value.projectRoot,
			&value.enactsMethodRef,
			&value.methodDescriptionRef,
			&value.methodDescriptionDigest,
			&value.methodContractRef,
			&value.methodContractDigest,
			&value.parameterBindingsJSON,
			&value.performedByRoleAssignmentRef,
			&value.profileAuthorRoleAssignmentRef,
			&value.profileAuthorRoleAssignmentDigest,
			&value.executedWithinRef,
			&value.workFrom,
			&value.workUntil,
			&value.boundedContextRef,
			&value.basisObservationFrom,
			&value.basisObservationUntil,
			&value.observedProjectBasisRef,
			&value.observedProjectBasisDigest,
			&value.inputsJSON,
			&value.outputsJSON,
			&value.resourcesJSON,
			&value.affectedRefKind,
			&value.affectedRefsJSON,
			&value.statePlaneRef,
			&value.preStateRef,
			&value.postStateRef,
			&value.deltaPredicateRef,
			&value.outcomeKind,
			&value.profilePayloadDigest,
			&value.observedBasisDigest,
			&value.missingBasisDigest,
			&value.canonicalJSON,
			&value.digest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "profile-onboarding Work", key)
}

func persistEffect(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row effectRow,
) error {
	spec := exactRowSpec[effectRow]{
		label:     "profile-onboarding effect",
		table:     "profile_onboarding_effects",
		insertSQL: insertEffectSQL,
		arguments: effectArguments(row),
		key:       row.ref,
		requested: row,
		load:      loadEffect,
		withoutRecordedAt: func(value effectRow) effectRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func effectArguments(row effectRow) []any {
	return []any{
		row.ref,
		row.workRecordRef,
		row.workRef,
		row.workRecordDigest,
		row.resultKind,
		row.outputRef,
		row.profilePayloadDigest,
		row.observedProjectBasisRef,
		row.observedProjectBasisDigest,
		row.missingBasisDigest,
		row.affectedEntityRefsJSON,
		row.statePlaneRef,
		row.preStateRef,
		row.postStateRef,
		row.deltaPredicateRef,
		row.evidencePathRefsJSON,
		row.canonicalJSON,
		row.digest,
		row.recordedAt,
	}
}

func loadEffect(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
) (effectRow, error) {
	var value effectRow
	err := transaction.ScanOne(
		ctx,
		selectEffectSQL,
		[]any{key},
		[]any{
			&value.ref,
			&value.workRecordRef,
			&value.workRef,
			&value.workRecordDigest,
			&value.resultKind,
			&value.outputRef,
			&value.profilePayloadDigest,
			&value.observedProjectBasisRef,
			&value.observedProjectBasisDigest,
			&value.missingBasisDigest,
			&value.affectedEntityRefsJSON,
			&value.statePlaneRef,
			&value.preStateRef,
			&value.postStateRef,
			&value.deltaPredicateRef,
			&value.evidencePathRefsJSON,
			&value.canonicalJSON,
			&value.digest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "profile-onboarding effect", key)
}

func persistAssessment(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row assessmentRow,
) error {
	spec := exactRowSpec[assessmentRow]{
		label:     "profile-onboarding outcome assessment",
		table:     "profile_onboarding_outcome_assessments",
		insertSQL: insertAssessmentSQL,
		arguments: assessmentArguments(row),
		key:       row.ref,
		requested: row,
		load:      loadAssessment,
		withoutRecordedAt: func(value assessmentRow) assessmentRow {
			value.recordedAt = ""
			return value
		},
	}
	return persistExactRow(ctx, transaction, spec)
}

func assessmentArguments(row assessmentRow) []any {
	return []any{
		row.ref,
		row.effectRef,
		row.effectDigest,
		row.workRecordRef,
		row.workRef,
		row.workRecordDigest,
		row.acceptanceStandardRef,
		row.acceptanceStandardEdition,
		row.comparatorRef,
		row.comparatorEdition,
		row.verdictKind,
		row.verdictReasonRef,
		row.missingBasisDigest,
		row.evidencePathRefsJSON,
		row.canonicalJSON,
		row.digest,
		row.recordedAt,
	}
}

func loadAssessment(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	key string,
) (assessmentRow, error) {
	var value assessmentRow
	err := transaction.ScanOne(
		ctx,
		selectAssessmentSQL,
		[]any{key},
		[]any{
			&value.ref,
			&value.effectRef,
			&value.effectDigest,
			&value.workRecordRef,
			&value.workRef,
			&value.workRecordDigest,
			&value.acceptanceStandardRef,
			&value.acceptanceStandardEdition,
			&value.comparatorRef,
			&value.comparatorEdition,
			&value.verdictKind,
			&value.verdictReasonRef,
			&value.missingBasisDigest,
			&value.evidencePathRefsJSON,
			&value.canonicalJSON,
			&value.digest,
			&value.recordedAt,
		},
	)
	return value, exactRowLoadError(err, "profile-onboarding assessment", key)
}
