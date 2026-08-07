package projecttypeenvstagerevalidation

import "sort"

// StageRevalidationIssueKind is the closed location of a structural failure.
type StageRevalidationIssueKind uint8

const (
	IssueKindInput StageRevalidationIssueKind = iota + 1
	IssueKindTargetClosure
	IssueKindTrustedEditions
	IssueKindTargetRuntime
	IssueKindCurrentGraph
	IssueKindPredecessor
	IssueKindAssertionRevalidation
	IssueKindProjectProfile
	IssueKindTypeEnvCompatibility
	IssueKindProjectionProfileCompatibility
)

func (kind StageRevalidationIssueKind) String() string {
	switch kind {
	case IssueKindInput:
		return "input"
	case IssueKindTargetClosure:
		return "target_closure"
	case IssueKindTrustedEditions:
		return "trusted_editions"
	case IssueKindTargetRuntime:
		return "target_runtime"
	case IssueKindCurrentGraph:
		return "current_graph"
	case IssueKindPredecessor:
		return "predecessor"
	case IssueKindAssertionRevalidation:
		return "assertion_revalidation"
	case IssueKindProjectProfile:
		return "project_profile"
	case IssueKindTypeEnvCompatibility:
		return "typeenv_compatibility"
	case IssueKindProjectionProfileCompatibility:
		return "projection_profile_compatibility"
	default:
		return ""
	}
}

// StageRevalidationIssueCode is a stable machine-facing reason. Its metadata
// defines both the issue kind and canonical order.
type StageRevalidationIssueCode string

const (
	IssueStageInvalid                           StageRevalidationIssueCode = "stage_invalid"
	IssueFinalVerificationInvalid               StageRevalidationIssueCode = "final_verification_invalid"
	IssueExecutableSnapshotInvalid              StageRevalidationIssueCode = "executable_snapshot_invalid"
	IssueGraphObservationInvalid                StageRevalidationIssueCode = "graph_observation_invalid"
	IssueProjectProfileBasisInvalid             StageRevalidationIssueCode = "project_profile_basis_invalid"
	IssueHeadObservationInvalid                 StageRevalidationIssueCode = "head_observation_invalid"
	IssueCurrentSelectionStageInvalid           StageRevalidationIssueCode = "current_selection_stage_invalid"
	IssueBaseMismatch                           StageRevalidationIssueCode = "base_mismatch"
	IssueOrderedExtensionsMismatch              StageRevalidationIssueCode = "ordered_extensions_mismatch"
	IssueRuntimeBasisMismatch                   StageRevalidationIssueCode = "runtime_basis_mismatch"
	IssueCompositeMismatch                      StageRevalidationIssueCode = "composite_mismatch"
	IssueFinalVerificationMismatch              StageRevalidationIssueCode = "final_verification_mismatch"
	IssueExecutableSnapshotMismatch             StageRevalidationIssueCode = "executable_snapshot_mismatch"
	IssueLoweredEnvironmentMismatch             StageRevalidationIssueCode = "lowered_environment_mismatch"
	IssueLowererEditionMismatch                 StageRevalidationIssueCode = "lowerer_edition_mismatch"
	IssueTrustedEditionInputInvalid             StageRevalidationIssueCode = "trusted_edition_input_invalid"
	IssueStageSchemaUnsupported                 StageRevalidationIssueCode = "stage_schema_unsupported"
	IssueStageCompilerUnsupported               StageRevalidationIssueCode = "stage_compiler_unsupported"
	IssueBaseCompilerUnsupported                StageRevalidationIssueCode = "base_compiler_schema_unsupported"
	IssueStageProducerUnsupported               StageRevalidationIssueCode = "stage_producer_unsupported"
	IssueStageRevalidatorUnsupported            StageRevalidationIssueCode = "stage_revalidator_unsupported"
	IssueCompositeLowererUnsupported            StageRevalidationIssueCode = "composite_lowerer_unsupported"
	IssueTargetRuntimeBasisMismatch             StageRevalidationIssueCode = "target_runtime_registry_basis_mismatch"
	IssueGraphProjectMismatch                   StageRevalidationIssueCode = "current_graph_project_mismatch"
	IssueGraphSnapshotMismatch                  StageRevalidationIssueCode = "current_graph_snapshot_mismatch"
	IssueGraphRevisionMismatch                  StageRevalidationIssueCode = "current_graph_revision_mismatch"
	IssueHeadProjectMismatch                    StageRevalidationIssueCode = "current_head_project_mismatch"
	IssueHeadPresenceMismatch                   StageRevalidationIssueCode = "current_head_presence_mismatch"
	IssuePriorHeadRefMismatch                   StageRevalidationIssueCode = "prior_head_ref_mismatch"
	IssuePriorHeadRevisionMismatch              StageRevalidationIssueCode = "prior_head_revision_mismatch"
	IssuePriorSelectedCompositeDrift            StageRevalidationIssueCode = "prior_selected_composite_mismatch"
	IssuePriorExecutableSnapshotInvalid         StageRevalidationIssueCode = "prior_executable_snapshot_invalid"
	IssuePriorExecutableSnapshotMismatch        StageRevalidationIssueCode = "prior_executable_snapshot_mismatch"
	IssueTypeEnvCompatibilityInvalid            StageRevalidationIssueCode = "typeenv_compatibility_invalid"
	IssueTypeEnvCompatibilityMismatch           StageRevalidationIssueCode = "typeenv_compatibility_mismatch"
	IssueProjectionProfileCompatibilityInvalid  StageRevalidationIssueCode = "projection_profile_compatibility_invalid"
	IssueProjectionProfileCompatibilityMismatch StageRevalidationIssueCode = "projection_profile_compatibility_mismatch"
	IssueProjectionProfileBlocked               StageRevalidationIssueCode = "projection_profile_blocked"
	IssueAssertionRevalidationFailed            StageRevalidationIssueCode = "assertion_revalidation_failed"
	IssueAssertionRevalidationMismatch          StageRevalidationIssueCode = "assertion_revalidation_mismatch"
	IssueAssertionRevalidationConflict          StageRevalidationIssueCode = "assertion_revalidation_conflict"
	IssueAssertionRevalidationUnderdetermined   StageRevalidationIssueCode = "assertion_revalidation_underdetermined"
	IssueProfileFitAssessmentFailed             StageRevalidationIssueCode = "profile_fit_assessment_failed"
	IssueProfileLedgerRevisionMismatch          StageRevalidationIssueCode = "profile_ledger_revision_mismatch"
	IssueProfileLedgerDigestMismatch            StageRevalidationIssueCode = "profile_ledger_digest_mismatch"
	IssueProfileFitMismatch                     StageRevalidationIssueCode = "profile_fit_mismatch"
	IssueProfileIncompatible                    StageRevalidationIssueCode = "profile_incompatible"
	IssueProfileUnderdetermined                 StageRevalidationIssueCode = "profile_underdetermined"
	IssueProfileUnavailable                     StageRevalidationIssueCode = "profile_unavailable"
)

type stageRevalidationIssueMetadata struct {
	kind StageRevalidationIssueKind
	rank int
}

func issueMetadata(
	code StageRevalidationIssueCode,
) stageRevalidationIssueMetadata {
	switch code {
	case IssueStageInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 10}
	case IssueFinalVerificationInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 20}
	case IssueExecutableSnapshotInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 30}
	case IssueGraphObservationInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 40}
	case IssueProjectProfileBasisInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 45}
	case IssueHeadObservationInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 50}
	case IssueCurrentSelectionStageInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 55}
	case IssueBaseMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTargetClosure, rank: 100}
	case IssueOrderedExtensionsMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTargetClosure, rank: 110}
	case IssueRuntimeBasisMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTargetClosure, rank: 120}
	case IssueCompositeMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTargetClosure, rank: 130}
	case IssueFinalVerificationMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTargetClosure, rank: 140}
	case IssueExecutableSnapshotMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTargetClosure, rank: 150}
	case IssueLoweredEnvironmentMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTargetClosure, rank: 160}
	case IssueLowererEditionMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTargetClosure, rank: 170}
	case IssueTrustedEditionInputInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 60}
	case IssueStageSchemaUnsupported:
		return stageRevalidationIssueMetadata{kind: IssueKindTrustedEditions, rank: 180}
	case IssueStageCompilerUnsupported:
		return stageRevalidationIssueMetadata{kind: IssueKindTrustedEditions, rank: 190}
	case IssueBaseCompilerUnsupported:
		return stageRevalidationIssueMetadata{kind: IssueKindTrustedEditions, rank: 200}
	case IssueStageProducerUnsupported:
		return stageRevalidationIssueMetadata{kind: IssueKindTrustedEditions, rank: 210}
	case IssueStageRevalidatorUnsupported:
		return stageRevalidationIssueMetadata{kind: IssueKindTrustedEditions, rank: 220}
	case IssueCompositeLowererUnsupported:
		return stageRevalidationIssueMetadata{kind: IssueKindTrustedEditions, rank: 230}
	case IssueTargetRuntimeBasisMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTargetRuntime, rank: 240}
	case IssueGraphProjectMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindCurrentGraph, rank: 250}
	case IssueGraphSnapshotMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindCurrentGraph, rank: 260}
	case IssueGraphRevisionMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindCurrentGraph, rank: 270}
	case IssueHeadProjectMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindPredecessor, rank: 280}
	case IssueHeadPresenceMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindPredecessor, rank: 290}
	case IssuePriorHeadRefMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindPredecessor, rank: 300}
	case IssuePriorHeadRevisionMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindPredecessor, rank: 310}
	case IssuePriorSelectedCompositeDrift:
		return stageRevalidationIssueMetadata{kind: IssueKindPredecessor, rank: 320}
	case IssuePriorExecutableSnapshotInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 321}
	case IssuePriorExecutableSnapshotMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTypeEnvCompatibility, rank: 322}
	case IssueTypeEnvCompatibilityInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 323}
	case IssueTypeEnvCompatibilityMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindTypeEnvCompatibility, rank: 324}
	case IssueProjectionProfileCompatibilityInvalid:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 325}
	case IssueProjectionProfileCompatibilityMismatch:
		return stageRevalidationIssueMetadata{
			kind: IssueKindProjectionProfileCompatibility,
			rank: 326,
		}
	case IssueProjectionProfileBlocked:
		return stageRevalidationIssueMetadata{
			kind: IssueKindProjectionProfileCompatibility,
			rank: 327,
		}
	case IssueAssertionRevalidationFailed:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 330}
	case IssueAssertionRevalidationMismatch:
		return stageRevalidationIssueMetadata{
			kind: IssueKindAssertionRevalidation,
			rank: 340,
		}
	case IssueAssertionRevalidationConflict:
		return stageRevalidationIssueMetadata{
			kind: IssueKindAssertionRevalidation,
			rank: 350,
		}
	case IssueAssertionRevalidationUnderdetermined:
		return stageRevalidationIssueMetadata{
			kind: IssueKindAssertionRevalidation,
			rank: 360,
		}
	case IssueProfileFitAssessmentFailed:
		return stageRevalidationIssueMetadata{kind: IssueKindInput, rank: 370}
	case IssueProfileLedgerRevisionMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindProjectProfile, rank: 375}
	case IssueProfileLedgerDigestMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindProjectProfile, rank: 376}
	case IssueProfileFitMismatch:
		return stageRevalidationIssueMetadata{kind: IssueKindProjectProfile, rank: 380}
	case IssueProfileIncompatible:
		return stageRevalidationIssueMetadata{kind: IssueKindProjectProfile, rank: 390}
	case IssueProfileUnderdetermined:
		return stageRevalidationIssueMetadata{kind: IssueKindProjectProfile, rank: 400}
	case IssueProfileUnavailable:
		return stageRevalidationIssueMetadata{kind: IssueKindProjectProfile, rank: 410}
	default:
		return stageRevalidationIssueMetadata{}
	}
}

// StageRevalidationIssue is immutable. Expected and actual are canonical
// display coordinates only; they do not become proof or authority.
type StageRevalidationIssue struct {
	kind     StageRevalidationIssueKind
	code     StageRevalidationIssueCode
	subject  string
	expected string
	actual   string
	repair   string
}

func (issue StageRevalidationIssue) Kind() StageRevalidationIssueKind {
	return issue.kind
}

func (issue StageRevalidationIssue) Code() StageRevalidationIssueCode {
	return issue.code
}

func (issue StageRevalidationIssue) Subject() string { return issue.subject }

func (issue StageRevalidationIssue) Expected() string { return issue.expected }

func (issue StageRevalidationIssue) Actual() string { return issue.actual }

func (issue StageRevalidationIssue) Repair() string { return issue.repair }

func newIssue(
	code StageRevalidationIssueCode,
	subject string,
	expected string,
	actual string,
	repair string,
) StageRevalidationIssue {
	metadata := issueMetadata(code)
	return StageRevalidationIssue{
		kind:     metadata.kind,
		code:     code,
		subject:  subject,
		expected: expected,
		actual:   actual,
		repair:   repair,
	}
}

func normalizeIssues(
	values []StageRevalidationIssue,
) []StageRevalidationIssue {
	owned := append([]StageRevalidationIssue(nil), values...)
	sort.Slice(owned, func(left int, right int) bool {
		leftMetadata := issueMetadata(owned[left].code)
		rightMetadata := issueMetadata(owned[right].code)
		if leftMetadata.rank != rightMetadata.rank {
			return leftMetadata.rank < rightMetadata.rank
		}
		if owned[left].kind != owned[right].kind {
			return owned[left].kind < owned[right].kind
		}
		if owned[left].code != owned[right].code {
			return owned[left].code < owned[right].code
		}
		if owned[left].subject != owned[right].subject {
			return owned[left].subject < owned[right].subject
		}
		if owned[left].expected != owned[right].expected {
			return owned[left].expected < owned[right].expected
		}
		if owned[left].actual != owned[right].actual {
			return owned[left].actual < owned[right].actual
		}
		return owned[left].repair < owned[right].repair
	})
	result := make([]StageRevalidationIssue, 0, len(owned))
	for _, issue := range owned {
		if len(result) > 0 && issuesEqual(result[len(result)-1], issue) {
			continue
		}
		result = append(result, issue)
	}
	return result
}

func issuesEqual(left StageRevalidationIssue, right StageRevalidationIssue) bool {
	return left.kind == right.kind &&
		left.code == right.code &&
		left.subject == right.subject &&
		left.expected == right.expected &&
		left.actual == right.actual &&
		left.repair == right.repair
}
