CREATE TABLE authority_presentations (
				presentation_id TEXT PRIMARY KEY,
				speech_act_ref TEXT NOT NULL,
				speech_act_digest TEXT NOT NULL,
				authorization_content_ref TEXT NOT NULL,
				authorization_content_digest TEXT NOT NULL,
				permission_ref TEXT NOT NULL UNIQUE,
				permission_digest TEXT NOT NULL,
				permission_modality TEXT NOT NULL CHECK(permission_modality = 'MAY'),
				permission_source_speech_act_ref TEXT NOT NULL,
				permission_subject_role_assignment_ref TEXT NOT NULL,
				permission_authorization_content_ref TEXT NOT NULL,
				permission_action_kind TEXT NOT NULL,
				permission_project_root TEXT NOT NULL,
				permission_method_description_ref TEXT NOT NULL,
				permission_valid_from TEXT NOT NULL,
				permission_valid_until TEXT NOT NULL,
				permission_single_use_key TEXT NOT NULL,
				permission_profile_admission_predicate_ref TEXT NOT NULL,
				permission_context_policy_ref TEXT NOT NULL,
				context_policy_ref TEXT NOT NULL,
				context_policy_digest TEXT NOT NULL,
				action_kind TEXT NOT NULL,
				project_root TEXT NOT NULL,
				profile_author_role_assignment_ref TEXT NOT NULL,
				method_description_ref TEXT NOT NULL,
				classifier_version TEXT NOT NULL,
				policy_version TEXT NOT NULL,
				session_ref TEXT NOT NULL,
				allowed_work_from TEXT NOT NULL,
				allowed_work_until TEXT NOT NULL,
				basis_observation_from TEXT NOT NULL,
				basis_observation_until TEXT NOT NULL,
				valid_from TEXT NOT NULL,
				valid_until TEXT NOT NULL,
				single_use_key TEXT NOT NULL UNIQUE,
				project_binding_digest TEXT NOT NULL,
				envelope_digest TEXT NOT NULL,
				presentation_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				UNIQUE(speech_act_ref, authorization_content_ref, permission_ref, context_policy_ref)
			) WITHOUT ROWID;
CREATE INDEX idx_authority_presentations_action_project ON authority_presentations(action_kind, project_root);
CREATE TRIGGER authority_presentations_no_replace
			BEFORE INSERT ON authority_presentations
			WHEN EXISTS (
				SELECT 1 FROM authority_presentations existing
				WHERE existing.presentation_id = NEW.presentation_id
				OR existing.permission_ref = NEW.permission_ref
				OR existing.single_use_key = NEW.single_use_key
				OR existing.presentation_digest = NEW.presentation_digest
			) BEGIN
				SELECT RAISE(ABORT, 'authority_presentations is append-only');
			END;
CREATE TRIGGER authority_presentations_no_update
			BEFORE UPDATE ON authority_presentations BEGIN
				SELECT RAISE(ABORT, 'authority_presentations is append-only');
			END;
CREATE TRIGGER authority_presentations_no_delete
			BEFORE DELETE ON authority_presentations BEGIN
				SELECT RAISE(ABORT, 'authority_presentations is append-only');
			END;
CREATE TABLE authority_resolution_records (
				authority_resolution_id TEXT PRIMARY KEY,
				presentation_id TEXT NOT NULL UNIQUE,
				presentation_digest TEXT NOT NULL UNIQUE,
				verifier_identity TEXT NOT NULL,
				verifier_version TEXT NOT NULL,
				verification_policy_ref TEXT NOT NULL,
				verification_policy_digest TEXT NOT NULL,
				resolved_at TEXT NOT NULL,
				valid_until TEXT NOT NULL,
				authority_resolution_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL
			) WITHOUT ROWID;
CREATE INDEX idx_authority_resolution_presentation ON authority_resolution_records(presentation_id);
CREATE TRIGGER authority_resolution_records_no_replace
			BEFORE INSERT ON authority_resolution_records
			WHEN EXISTS (
				SELECT 1 FROM authority_resolution_records existing
				WHERE existing.authority_resolution_id = NEW.authority_resolution_id
				OR existing.presentation_id = NEW.presentation_id
				OR existing.presentation_digest = NEW.presentation_digest
				OR existing.authority_resolution_digest = NEW.authority_resolution_digest
			) BEGIN
				SELECT RAISE(ABORT, 'authority_resolution_records is append-only');
			END;
CREATE TRIGGER authority_resolution_records_no_update
			BEFORE UPDATE ON authority_resolution_records BEGIN
				SELECT RAISE(ABORT, 'authority_resolution_records is append-only');
			END;
CREATE TRIGGER authority_resolution_records_no_delete
			BEFORE DELETE ON authority_resolution_records BEGIN
				SELECT RAISE(ABORT, 'authority_resolution_records is append-only');
			END;
CREATE TABLE authority_uses (
				use_id TEXT PRIMARY KEY,
				authority_resolution_ref TEXT NOT NULL UNIQUE,
				authority_resolution_digest TEXT NOT NULL,
				single_use_key TEXT NOT NULL UNIQUE,
				action_kind TEXT NOT NULL,
				project_root TEXT NOT NULL,
				project_binding_digest TEXT NOT NULL,
				envelope_digest TEXT NOT NULL,
				authority_record_ref TEXT NOT NULL,
				authority_record_digest TEXT NOT NULL,
				admission_request_digest TEXT NOT NULL,
				verifier_identity TEXT NOT NULL,
				verifier_version TEXT NOT NULL,
				committed_result_ref TEXT NOT NULL UNIQUE,
				committed_result_digest TEXT NOT NULL,
				consumed_at TEXT NOT NULL
			) WITHOUT ROWID;
CREATE INDEX idx_authority_uses_record ON authority_uses(authority_record_ref);
CREATE TRIGGER authority_uses_no_replace
			BEFORE INSERT ON authority_uses
			WHEN EXISTS (
				SELECT 1 FROM authority_uses existing
				WHERE existing.use_id = NEW.use_id
				OR existing.authority_resolution_ref = NEW.authority_resolution_ref
				OR existing.single_use_key = NEW.single_use_key
				OR existing.committed_result_ref = NEW.committed_result_ref
			) BEGIN
				SELECT RAISE(ABORT, 'authority_uses is append-only');
			END;
CREATE TRIGGER authority_uses_exact_tuple
			BEFORE INSERT ON authority_uses
			WHEN NOT EXISTS (
				SELECT 1
				FROM authority_resolution_records resolution
				JOIN authority_presentations presentation
					ON presentation.presentation_id = resolution.presentation_id
				JOIN project_profile_admissions admission
					ON admission.admission_id = NEW.committed_result_ref
				WHERE resolution.authority_resolution_id = NEW.authority_resolution_ref
				AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
				AND resolution.verifier_identity = NEW.verifier_identity
				AND resolution.verifier_version = NEW.verifier_version
				AND presentation.single_use_key = NEW.single_use_key
				AND presentation.action_kind = NEW.action_kind
				AND presentation.project_root = NEW.project_root
				AND presentation.project_binding_digest = NEW.project_binding_digest
				AND presentation.envelope_digest = NEW.envelope_digest
				AND presentation.presentation_id = NEW.authority_record_ref
				AND presentation.presentation_digest = NEW.authority_record_digest
				AND admission.admission_digest = NEW.committed_result_digest
				AND admission.admission_request_digest = NEW.admission_request_digest
				AND admission.authority_resolution_ref = NEW.authority_resolution_ref
				AND admission.authority_resolution_digest = NEW.authority_resolution_digest
				AND admission.single_use_key = NEW.single_use_key
				AND admission.action_kind = NEW.action_kind
			) BEGIN
				SELECT RAISE(ABORT, 'authority_uses tuple does not match canonical resolution and admission');
			END;
CREATE TRIGGER authority_uses_no_update
			BEFORE UPDATE ON authority_uses BEGIN
				SELECT RAISE(ABORT, 'authority_uses is append-only');
			END;
CREATE TRIGGER authority_uses_no_delete
			BEFORE DELETE ON authority_uses BEGIN
				SELECT RAISE(ABORT, 'authority_uses is append-only');
			END;
CREATE TABLE profile_onboarding_work_records (
				work_record_ref TEXT PRIMARY KEY,
				work_ref TEXT NOT NULL UNIQUE,
				project_root TEXT NOT NULL,
				enacts_method_ref TEXT NOT NULL,
				method_description_ref TEXT NOT NULL,
				parameter_bindings_json TEXT NOT NULL,
				parameter_bindings_digest TEXT NOT NULL,
				performed_by_role_assignment_ref TEXT NOT NULL,
				executed_within_ref TEXT NOT NULL,
				work_from TEXT NOT NULL,
				work_until TEXT NOT NULL,
				role_assignment_from TEXT NOT NULL,
				role_assignment_until TEXT NOT NULL,
				bounded_context_ref TEXT NOT NULL,
				inputs_json TEXT NOT NULL,
				inputs_digest TEXT NOT NULL,
				outputs_json TEXT NOT NULL,
				outputs_digest TEXT NOT NULL,
				resources_json TEXT NOT NULL,
				resources_digest TEXT NOT NULL,
				affected_episteme_ref TEXT NOT NULL,
				state_plane_ref TEXT NOT NULL,
				pre_state_ref TEXT NOT NULL DEFAULT '',
				post_state_ref TEXT NOT NULL DEFAULT '',
				delta_predicate_ref TEXT NOT NULL DEFAULT '',
				basis_observation_from TEXT NOT NULL,
				basis_observation_until TEXT NOT NULL,
				outcome_kind TEXT NOT NULL CHECK(outcome_kind IN ('candidate_payload_produced', 'classification_underdetermined')),
				profile_payload_digest TEXT NOT NULL DEFAULT '',
				observed_basis_digest TEXT NOT NULL DEFAULT '',
				missing_basis_digest TEXT NOT NULL DEFAULT '',
				work_record_json TEXT NOT NULL,
				work_record_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				CHECK(
					(pre_state_ref != '' AND post_state_ref != '' AND delta_predicate_ref = '')
					OR (pre_state_ref = '' AND post_state_ref = '' AND delta_predicate_ref != '')
				),
				CHECK(
					(outcome_kind = 'candidate_payload_produced' AND profile_payload_digest != '' AND observed_basis_digest != '' AND missing_basis_digest = '')
					OR (outcome_kind = 'classification_underdetermined' AND profile_payload_digest = '' AND observed_basis_digest = '' AND missing_basis_digest != '')
				)
			) WITHOUT ROWID;
CREATE INDEX idx_profile_onboarding_work_project ON profile_onboarding_work_records(project_root, work_from);
CREATE TRIGGER profile_onboarding_work_records_no_replace
			BEFORE INSERT ON profile_onboarding_work_records
			WHEN EXISTS (
				SELECT 1 FROM profile_onboarding_work_records existing
				WHERE existing.work_record_ref = NEW.work_record_ref
					OR existing.work_ref = NEW.work_ref
					OR existing.work_record_digest = NEW.work_record_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_work_records is append-only');
			END;
CREATE TRIGGER profile_onboarding_work_records_no_update
			BEFORE UPDATE ON profile_onboarding_work_records BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_work_records is append-only');
			END;
CREATE TRIGGER profile_onboarding_work_records_no_delete
			BEFORE DELETE ON profile_onboarding_work_records BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_work_records is append-only');
			END;
CREATE TABLE project_profile_admissions (
				admission_id TEXT PRIMARY KEY,
				action_kind TEXT NOT NULL CHECK(action_kind = 'classify_and_declare'),
				project_root TEXT NOT NULL,
				project_binding_digest TEXT NOT NULL,
				profile_payload_json TEXT NOT NULL,
				candidate_provenance_ref TEXT NOT NULL,
				candidate_provenance_digest TEXT NOT NULL,
				profile_payload_digest TEXT NOT NULL,
				observed_basis_digest TEXT NOT NULL,
				work_record_ref TEXT NOT NULL,
				work_record_digest TEXT NOT NULL,
				authority_basis_ref TEXT NOT NULL,
				authority_basis_digest TEXT NOT NULL,
				authority_resolution_ref TEXT NOT NULL UNIQUE,
				authority_resolution_digest TEXT NOT NULL,
				receipt_ref TEXT NOT NULL UNIQUE,
				receipt_json TEXT NOT NULL,
				receipt_digest TEXT NOT NULL UNIQUE,
				expected_ledger_revision INTEGER NOT NULL CHECK(expected_ledger_revision >= 0),
				ledger_revision INTEGER NOT NULL CHECK(ledger_revision = expected_ledger_revision + 1),
				single_use_key TEXT NOT NULL UNIQUE,
				admission_request_digest TEXT NOT NULL,
				admission_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				UNIQUE(project_root, ledger_revision)
			) WITHOUT ROWID;
CREATE INDEX idx_project_profile_admissions_project ON project_profile_admissions(project_root, ledger_revision);
CREATE TRIGGER project_profile_admissions_no_replace
			BEFORE INSERT ON project_profile_admissions
			WHEN EXISTS (
				SELECT 1 FROM project_profile_admissions existing
				WHERE existing.admission_id = NEW.admission_id
				OR existing.authority_resolution_ref = NEW.authority_resolution_ref
				OR existing.receipt_ref = NEW.receipt_ref
				OR existing.receipt_digest = NEW.receipt_digest
				OR existing.single_use_key = NEW.single_use_key
				OR existing.admission_digest = NEW.admission_digest
				OR (existing.project_root = NEW.project_root AND existing.ledger_revision = NEW.ledger_revision)
			) BEGIN
				SELECT RAISE(ABORT, 'project_profile_admissions is append-only');
			END;
CREATE TRIGGER project_profile_admissions_revision_cas
			BEFORE INSERT ON project_profile_admissions
			WHEN COALESCE((
				SELECT MAX(existing.ledger_revision)
				FROM project_profile_revisions existing
				WHERE existing.project_root = NEW.project_root
			), 0) != NEW.expected_ledger_revision BEGIN
				SELECT RAISE(ABORT, 'project profile ledger revision conflict');
			END;
CREATE TRIGGER project_profile_admissions_exact_authority
			BEFORE INSERT ON project_profile_admissions
			WHEN NOT EXISTS (
				SELECT 1
				FROM authority_resolution_records resolution
				JOIN authority_presentations presentation
					ON presentation.presentation_id = resolution.presentation_id
				JOIN profile_onboarding_work_records work_record
					ON work_record.work_record_ref = NEW.work_record_ref
				WHERE presentation.presentation_id = NEW.authority_basis_ref
				AND presentation.presentation_digest = NEW.authority_basis_digest
				AND presentation.action_kind = NEW.action_kind
				AND NEW.action_kind = 'classify_and_declare'
				AND presentation.project_root = NEW.project_root
				AND presentation.project_binding_digest = NEW.project_binding_digest
				AND presentation.single_use_key = NEW.single_use_key
				AND resolution.authority_resolution_id = NEW.authority_resolution_ref
				AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
				AND work_record.work_record_digest = NEW.work_record_digest
				AND work_record.project_root = NEW.project_root
				AND work_record.outcome_kind = 'candidate_payload_produced'
				AND work_record.profile_payload_digest = NEW.profile_payload_digest
				AND work_record.observed_basis_digest = NEW.observed_basis_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile admission does not match canonical authority resolution');
			END;
CREATE TRIGGER project_profile_admissions_no_update
			BEFORE UPDATE ON project_profile_admissions BEGIN
				SELECT RAISE(ABORT, 'project_profile_admissions is append-only');
			END;
CREATE TRIGGER project_profile_admissions_no_delete
			BEFORE DELETE ON project_profile_admissions BEGIN
				SELECT RAISE(ABORT, 'project_profile_admissions is append-only');
			END;
CREATE TABLE project_profile_projection_debt (
				event_id TEXT PRIMARY KEY,
				debt_id TEXT NOT NULL,
				admission_id TEXT NOT NULL,
				admission_digest TEXT NOT NULL,
				project_root TEXT NOT NULL,
				ledger_revision INTEGER NOT NULL CHECK(ledger_revision > 0),
				profile_payload_digest TEXT NOT NULL,
				projection_path TEXT NOT NULL,
				event_kind TEXT NOT NULL CHECK(event_kind IN ('opened', 'resolved')),
				reason_code TEXT NOT NULL,
				detail TEXT NOT NULL,
				expected_projection_digest TEXT NOT NULL,
				observed_projection_digest TEXT NOT NULL DEFAULT '',
				supersedes_event_id TEXT,
				recorded_at TEXT NOT NULL
			) WITHOUT ROWID;
CREATE INDEX idx_project_profile_projection_debt_open ON project_profile_projection_debt(debt_id, event_kind, recorded_at);
CREATE TRIGGER project_profile_projection_debt_no_replace
			BEFORE INSERT ON project_profile_projection_debt
			WHEN EXISTS (
				SELECT 1 FROM project_profile_projection_debt existing
				WHERE existing.event_id = NEW.event_id
			) BEGIN
				SELECT RAISE(ABORT, 'project_profile_projection_debt is append-only');
			END;
CREATE TRIGGER project_profile_projection_debt_no_update
			BEFORE UPDATE ON project_profile_projection_debt BEGIN
				SELECT RAISE(ABORT, 'project_profile_projection_debt is append-only');
			END;
CREATE TRIGGER project_profile_projection_debt_no_delete
			BEFORE DELETE ON project_profile_projection_debt BEGIN
				SELECT RAISE(ABORT, 'project_profile_projection_debt is append-only');
			END;
CREATE TABLE project_profile_revisions (
				project_root TEXT NOT NULL,
				ledger_revision INTEGER NOT NULL CHECK(ledger_revision > 0),
				configured_profile_kind TEXT NOT NULL CHECK(configured_profile_kind = 'Declared'),
				profile_payload_json TEXT NOT NULL,
				profile_payload_digest TEXT NOT NULL,
				receipt_ref TEXT NOT NULL,
				receipt_json TEXT NOT NULL,
				receipt_digest TEXT NOT NULL,
				admission_id TEXT NOT NULL UNIQUE,
				admission_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				PRIMARY KEY(project_root, ledger_revision)
			) WITHOUT ROWID;
CREATE INDEX idx_project_profile_revisions_current ON project_profile_revisions(project_root, ledger_revision DESC);
CREATE TRIGGER project_profile_revisions_no_replace
			BEFORE INSERT ON project_profile_revisions
			WHEN EXISTS (
				SELECT 1 FROM project_profile_revisions existing
				WHERE (existing.project_root = NEW.project_root AND existing.ledger_revision = NEW.ledger_revision)
				OR existing.admission_id = NEW.admission_id
				OR existing.admission_digest = NEW.admission_digest
			) BEGIN
				SELECT RAISE(ABORT, 'project_profile_revisions is append-only');
			END;
CREATE TRIGGER project_profile_revisions_exact_admission
			BEFORE INSERT ON project_profile_revisions
			WHEN NOT EXISTS (
				SELECT 1
				FROM project_profile_admissions admission
				JOIN authority_uses authority_use
					ON authority_use.committed_result_ref = admission.admission_id
				WHERE admission.admission_id = NEW.admission_id
				AND admission.admission_digest = NEW.admission_digest
				AND admission.project_root = NEW.project_root
				AND admission.ledger_revision = NEW.ledger_revision
				AND admission.profile_payload_json = NEW.profile_payload_json
				AND admission.profile_payload_digest = NEW.profile_payload_digest
				AND admission.receipt_ref = NEW.receipt_ref
				AND admission.receipt_json = NEW.receipt_json
				AND admission.receipt_digest = NEW.receipt_digest
				AND authority_use.committed_result_digest = admission.admission_digest
				AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
				AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
				AND authority_use.single_use_key = admission.single_use_key
				AND authority_use.action_kind = admission.action_kind
				AND authority_use.project_root = admission.project_root
				AND authority_use.project_binding_digest = admission.project_binding_digest
				AND authority_use.admission_request_digest = admission.admission_request_digest
			) BEGIN
				SELECT RAISE(ABORT, 'project profile revision does not match canonical admission');
			END;
CREATE TRIGGER project_profile_revisions_no_update
			BEFORE UPDATE ON project_profile_revisions BEGIN
				SELECT RAISE(ABORT, 'project_profile_revisions is append-only');
			END;
CREATE TRIGGER project_profile_revisions_no_delete
			BEFORE DELETE ON project_profile_revisions BEGIN
				SELECT RAISE(ABORT, 'project_profile_revisions is append-only');
			END;
CREATE VIEW current_project_profiles AS
			SELECT revision.*
			FROM project_profile_revisions revision
			WHERE NOT EXISTS (
				SELECT 1 FROM project_profile_revisions newer
				WHERE newer.project_root = revision.project_root
				AND newer.ledger_revision > revision.ledger_revision
			);
