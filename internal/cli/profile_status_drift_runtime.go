package cli

import (
	"context"
	"fmt"
	"strings"
)

const (
	projectProfileDriftRecordKind    = "haft_project_profile_drift"
	projectProfileDriftSchemaVersion = 1
)

// projectProfileDriftProjection is a read-only comparison record. The
// declared basis and detector observation remain separate so repository
// signals cannot become profile authority merely by appearing in status.
type projectProfileDriftProjection struct {
	Kind              string
	SchemaVersion     int
	SemanticRole      string
	AuthorityBoundary string
	DeclaredBasis     canonicalProfileView
	DetectorBasis     projectProfileDetectorBasis
	Relation          profileInspectionRelation
}

type projectProfileDetectorBasis struct {
	SuggestionRef     string
	DetectorVersion   string
	PolicyVersion     string
	ObservationBasis  string
	ObservationDigest string
	Classification    string
	ConfidencePosture string
	ScannedFileCount  int
	ScanTruncated     bool
}

type statusProjectProfileInspection interface {
	statusProjectProfileInspectionVariant()
}

type statusProjectProfileNotCompared struct{}

func (statusProjectProfileNotCompared) statusProjectProfileInspectionVariant() {}

type statusProjectProfileCompared struct {
	inspection profileInspectionResponse
}

func (statusProjectProfileCompared) statusProjectProfileInspectionVariant() {}

func inspectStatusProjectProfile(
	ctx context.Context,
	projectRoot string,
	readiness canonicalProjectReadiness,
) (statusProjectProfileInspection, error) {
	if !readiness.hasDeclaredProfileBasis() {
		return statusProjectProfileNotCompared{}, nil
	}
	inspection, err := executeProfileInspection(ctx, projectRoot, false)
	if err != nil {
		return statusProjectProfileNotCompared{}, err
	}
	return statusProjectProfileCompared{inspection: inspection}, nil
}

func (readiness canonicalProjectReadiness) hasDeclaredProfileBasis() bool {
	if !readiness.profileEvaluated || readiness.profileUnavailable {
		return false
	}
	return readiness.resolution.basis.valid()
}

func projectProfileDriftFromInspection(
	inspection profileInspectionResponse,
) (projectProfileDriftProjection, bool) {
	if inspection.Relation.Kind != "conflicts_with_declared" {
		return projectProfileDriftProjection{}, false
	}
	detector := inspection.Suggestion
	projection := projectProfileDriftProjection{
		Kind:              projectProfileDriftRecordKind,
		SchemaVersion:     projectProfileDriftSchemaVersion,
		SemanticRole:      "read_only_profile_attention",
		AuthorityBoundary: "detector_observation_cannot_mutate_or_displace_declared_profile",
		DeclaredBasis:     inspection.CanonicalProfile,
		DetectorBasis: projectProfileDetectorBasis{
			SuggestionRef:     detector.SuggestionRef,
			DetectorVersion:   detector.DetectorVersion,
			PolicyVersion:     detector.PolicyVersion,
			ObservationBasis:  detector.ObservationBasis,
			ObservationDigest: detector.ObservationDigest,
			Classification:    detector.Classification,
			ConfidencePosture: detector.ConfidencePosture,
			ScannedFileCount:  detector.Scan.ScannedFileCount,
			ScanTruncated:     detector.Scan.ScanTruncated,
		},
		Relation: inspection.Relation,
	}
	if !projection.valid() {
		return projectProfileDriftProjection{}, false
	}
	return projection, true
}

func (projection projectProfileDriftProjection) valid() bool {
	declared := projection.DeclaredBasis
	detector := projection.DetectorBasis
	return projection.Kind == projectProfileDriftRecordKind &&
		projection.SchemaVersion == projectProfileDriftSchemaVersion &&
		projection.SemanticRole == "read_only_profile_attention" &&
		projection.AuthorityBoundary != "" &&
		declared.Kind == "declared" &&
		declared.AdmissionRecordRef != "" &&
		declared.AdmissionRecordDigest != "" &&
		declared.PayloadDigest != "" &&
		declared.LedgerRevision > 0 &&
		declared.RecordedAt != "" &&
		len(declared.Scopes) > 0 &&
		detector.SuggestionRef != "" &&
		detector.DetectorVersion != "" &&
		detector.PolicyVersion != "" &&
		detector.ObservationBasis != "" &&
		detector.ObservationDigest != "" &&
		detector.Classification != "" &&
		detector.ConfidencePosture != "" &&
		projection.Relation.Kind == "conflicts_with_declared" &&
		projection.Relation.BindingSource == "sqlite_profile_admission_ledger" &&
		!projection.Relation.DetectorMayMutate &&
		!projection.Relation.DetectorMaySatisfyGate
}

func statusProjectProfilePrefix(
	readiness canonicalProjectReadiness,
	comparison statusProjectProfileInspection,
	full bool,
) string {
	compared, ok := comparison.(statusProjectProfileCompared)
	if !ok {
		return statusProfilePrefix(readiness, full)
	}
	inspection := compared.inspection
	drift, drifted := projectProfileDriftFromInspection(inspection)
	if !drifted {
		return statusProfilePrefix(readiness, full)
	}
	return renderProjectProfileDrift(readiness, inspection, drift, full)
}

func renderProjectProfileDrift(
	readiness canonicalProjectReadiness,
	inspection profileInspectionResponse,
	drift projectProfileDriftProjection,
	full bool,
) string {
	declared := drift.DeclaredBasis
	detector := drift.DetectorBasis
	lines := []string{
		"## Project profile",
		"",
		fmt.Sprintf(
			"Profile drift: kind=%s; schema_version=%d; semantic_role=%s.",
			drift.Kind,
			drift.SchemaVersion,
			drift.SemanticRole,
		),
		fmt.Sprintf(
			"Declared basis: scope_ids=[%s]; realization_kinds=[%s]; admission_record_ref=%s; admission_record_digest=%s; profile_payload_digest=%s; ledger_revision=%d; recorded_at=%s.",
			strings.Join(canonicalProfileScopeIDs(declared.Scopes), ","),
			strings.Join(canonicalRealizationKinds(declared.Scopes), ","),
			declared.AdmissionRecordRef,
			declared.AdmissionRecordDigest,
			declared.PayloadDigest,
			declared.LedgerRevision,
			declared.RecordedAt,
		),
		fmt.Sprintf(
			"Detector observation: suggestion_ref=%s; detector_version=%s; policy_version=%s; observation_basis=%s; observation_digest=%s; classification=%s; confidence=%s; scanned_files=%d; scan_truncated=%t.",
			detector.SuggestionRef,
			detector.DetectorVersion,
			detector.PolicyVersion,
			detector.ObservationBasis,
			detector.ObservationDigest,
			detector.Classification,
			detector.ConfidencePosture,
			detector.ScannedFileCount,
			detector.ScanTruncated,
		),
		fmt.Sprintf(
			"Comparison: relation=%s; binding_source=%s; detail=%s.",
			drift.Relation.Kind,
			drift.Relation.BindingSource,
			drift.Relation.Detail,
		),
		"Authority: " + drift.AuthorityBoundary + "; the declared profile remains binding and no mutation was performed.",
	}
	if !full {
		lines = append(
			lines,
			"Full detail: `haft_query(action=\"status\", full=true)`; detector evidence: `haft profile inspect --full-evidence --json`.",
		)
		return strings.Join(lines, "\n") + "\n\n"
	}
	lines = append(lines, "", "Declared scopes:")
	for _, scope := range declared.Scopes {
		lines = append(lines, fmt.Sprintf(
			"- scope_id=%s; realization_kind=%s; entity_ref=%s; admitted_kind_ref=%s; governing_pattern_refs=[%s]; contract_refs=[%s].",
			scope.ScopeID,
			scope.RealizationKind,
			emptyProfileStatusValue(scope.EntityRef),
			emptyProfileStatusValue(scope.AdmittedKindRef),
			strings.Join(scope.GoverningPatternRefs, ","),
			strings.Join(scope.ContractRefs, ","),
		))
	}
	lines = append(
		lines,
		"",
		"Capability applicability (authority=canonical_profile_capability_matrix.v1):",
	)
	lines = append(lines, readiness.fullProfileCapabilityLines()...)
	lines = append(lines, "", "Detector candidates (non-binding):")
	for _, scope := range profileSuggestionScopes(inspection.Suggestion) {
		lines = append(lines, fmt.Sprintf(
			"- component_candidate_ref=%s; realization_kind=%s; orientation=%s; positive_signals=%d; negative_signals=%d.",
			scope.ComponentCandidateRef,
			scope.RealizationKind,
			scope.Orientation,
			scope.PositiveSignalCount,
			scope.NegativeSignalCount,
		))
	}
	lines = append(
		lines,
		"Detector evidence remains orientation only; use `haft profile inspect --full-evidence --json` for the exact observed-file and signal lists.",
	)
	return strings.Join(lines, "\n") + "\n\n"
}

func canonicalProfileScopeIDs(scopes []canonicalProfileScopeView) []string {
	result := make([]string, len(scopes))
	for index, scope := range scopes {
		result[index] = scope.ScopeID
	}
	return result
}

func emptyProfileStatusValue(value string) string {
	if value == "" {
		return "absent"
	}
	return value
}
