package fpf

import (
	"embed"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	FPFEngineCoverageSchemaVersion = 1
	FPFEngineCoverageC0RowCount    = 22
)

const (
	RoutingClassNormativeSourceOnly         = "normative_source_only"
	RoutingClassRetrievalCandidateOnly      = "retrieval_candidate_only"
	RoutingClassAffordanceCandidate         = "routing_affordance_candidate"
	RoutingClassCompiledPatternUseRoute     = "compiled_patternuse_route"
	RoutingClassDedicatedSkillOrWorkflow    = "dedicated_skill_or_workflow"
	RoutingClassKernelSubstrate             = "kernel_substrate"
	RoutingClassMethodPackBridgeOnly        = "methodpack_bridge_only"
	RoutingClassNeverRouteDirectly          = "never_route_directly"
	RoutingClassSourcePackMetadata          = "source_pack_metadata"
	UserFacingFalse                         = "false"
	UserFacingTrue                          = "true"
	UserFacingTrueAsCandidateOnly           = "true_as_candidate_only"
	UserFacingTrueAsSuggestedSurface        = "true_as_suggested_surface"
	UserFacingTrueOnlyAsSuggestedMethodRefs = "true_only_as_suggested_method_refs"
)

//go:embed fpf_engine_coverage_matrix.yaml
var fpfEngineCoverageFS embed.FS

var fpfEngineClusterIDRE = regexp.MustCompile(`^PU\.[A-Z0-9_]+$`)

type FPFEngineCoverageMatrix struct {
	SchemaVersion int                        `json:"schema_version" yaml:"schema_version"`
	MatrixKind    string                     `json:"matrix_kind" yaml:"matrix_kind"`
	Authority     string                     `json:"authority" yaml:"authority"`
	Rows          []FPFEngineCoverageCluster `json:"rows" yaml:"rows"`
}

type FPFEngineCoverageCluster struct {
	ClusterID               string   `json:"cluster_id" yaml:"cluster_id"`
	HumanName               string   `json:"human_name" yaml:"human_name"`
	Description             string   `json:"description" yaml:"description"`
	PrimaryRefs             []string `json:"primary_refs" yaml:"primary_refs"`
	OwnerSurface            []string `json:"owner_surface" yaml:"owner_surface"`
	RoutingClass            string   `json:"routing_class" yaml:"routing_class"`
	UserFacingAllowed       string   `json:"user_facing_allowed" yaml:"user_facing_allowed"`
	CurrentRuntimeStatus    string   `json:"current_runtime_status" yaml:"current_runtime_status"`
	OutputCarrierKind       string   `json:"output_carrier_kind,omitempty" yaml:"output_carrier_kind,omitempty"`
	NoneReason              string   `json:"none_reason,omitempty" yaml:"none_reason,omitempty"`
	AuthorityBoundary       string   `json:"authority_boundary" yaml:"authority_boundary"`
	WrongBoundaryExamples   []string `json:"wrong_boundary_examples" yaml:"wrong_boundary_examples"`
	PositiveTriggerExamples []string `json:"positive_trigger_examples" yaml:"positive_trigger_examples"`
	NegativeTriggerExamples []string `json:"negative_trigger_examples" yaml:"negative_trigger_examples"`
	PromotionCriteria       []string `json:"promotion_criteria" yaml:"promotion_criteria"`
	DemotionCriteria        []string `json:"demotion_criteria" yaml:"demotion_criteria"`
	RequiredTests           []string `json:"required_tests" yaml:"required_tests"`
	MethodPackRelation      string   `json:"methodpack_relation" yaml:"methodpack_relation"`
	Notes                   string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

func DefaultFPFEngineCoverageMatrix() (FPFEngineCoverageMatrix, error) {
	data, err := fpfEngineCoverageFS.ReadFile("fpf_engine_coverage_matrix.yaml")
	if err != nil {
		return FPFEngineCoverageMatrix{}, fmt.Errorf("read embedded FPF engine coverage matrix: %w", err)
	}
	return ParseFPFEngineCoverageMatrix(data)
}

func ParseFPFEngineCoverageMatrix(data []byte) (FPFEngineCoverageMatrix, error) {
	var matrix FPFEngineCoverageMatrix
	if err := yaml.Unmarshal(data, &matrix); err != nil {
		return FPFEngineCoverageMatrix{}, fmt.Errorf("parse FPF engine coverage matrix: %w", err)
	}
	return matrix, nil
}

func ValidateFPFEngineCoverageMatrix(matrix FPFEngineCoverageMatrix) error {
	var errs []string
	if matrix.SchemaVersion != FPFEngineCoverageSchemaVersion {
		errs = append(errs, fmt.Sprintf("schema_version = %d, want %d", matrix.SchemaVersion, FPFEngineCoverageSchemaVersion))
	}
	if strings.TrimSpace(matrix.MatrixKind) == "" {
		errs = append(errs, "matrix_kind is required")
	}
	if !strings.Contains(strings.ToLower(matrix.Authority), "inventory") {
		errs = append(errs, "authority must declare inventory posture")
	}
	if len(matrix.Rows) != FPFEngineCoverageC0RowCount {
		errs = append(errs, fmt.Sprintf("rows = %d, want %d", len(matrix.Rows), FPFEngineCoverageC0RowCount))
	}

	seen := map[string]bool{}
	for index, row := range matrix.Rows {
		rowRef := fmt.Sprintf("rows[%d]", index)
		if strings.TrimSpace(row.ClusterID) == "" {
			errs = append(errs, rowRef+" cluster_id is required")
		}
		if row.ClusterID != "" && !fpfEngineClusterIDRE.MatchString(row.ClusterID) {
			errs = append(errs, row.ClusterID+" cluster_id must match ^PU\\.[A-Z0-9_]+$")
		}
		if seen[row.ClusterID] {
			errs = append(errs, row.ClusterID+" duplicate cluster_id")
		}
		seen[row.ClusterID] = true

		errs = append(errs, validateFPFEngineCoverageRequiredText(rowRef, row)...)
		errs = append(errs, validateFPFEngineCoverageEnums(row)...)
		errs = append(errs, validateFPFEngineCoverageBoundary(row)...)
	}
	if len(errs) > 0 {
		return fmt.Errorf("FPF engine coverage matrix invalid: %s", strings.Join(errs, "; "))
	}
	return nil
}

func validateFPFEngineCoverageRequiredText(rowRef string, row FPFEngineCoverageCluster) []string {
	var errs []string
	required := map[string]string{
		"human_name":             row.HumanName,
		"description":            row.Description,
		"routing_class":          row.RoutingClass,
		"user_facing_allowed":    row.UserFacingAllowed,
		"current_runtime_status": row.CurrentRuntimeStatus,
		"authority_boundary":     row.AuthorityBoundary,
		"methodpack_relation":    row.MethodPackRelation,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, rowRef+" "+field+" is required")
		}
	}
	if len(row.PrimaryRefs) == 0 {
		errs = append(errs, rowRef+" primary_refs is required")
	}
	if len(row.OwnerSurface) == 0 {
		errs = append(errs, rowRef+" owner_surface is required")
	}
	if len(row.WrongBoundaryExamples) < 2 {
		errs = append(errs, rowRef+" needs at least two wrong_boundary_examples")
	}
	if len(row.PositiveTriggerExamples) == 0 {
		errs = append(errs, rowRef+" positive_trigger_examples is required")
	}
	if len(row.NegativeTriggerExamples) == 0 {
		errs = append(errs, rowRef+" negative_trigger_examples is required")
	}
	if len(row.PromotionCriteria) == 0 {
		errs = append(errs, rowRef+" promotion_criteria is required")
	}
	if len(row.DemotionCriteria) == 0 {
		errs = append(errs, rowRef+" demotion_criteria is required")
	}
	if len(row.RequiredTests) == 0 {
		errs = append(errs, rowRef+" required_tests is required")
	}
	return errs
}

func validateFPFEngineCoverageEnums(row FPFEngineCoverageCluster) []string {
	var errs []string
	if !validFPFEngineRoutingClass(row.RoutingClass) {
		errs = append(errs, row.ClusterID+" unsupported routing_class "+row.RoutingClass)
	}
	if !validFPFEngineUserFacing(row.UserFacingAllowed) {
		errs = append(errs, row.ClusterID+" unsupported user_facing_allowed "+row.UserFacingAllowed)
	}
	if row.RoutingClass == RoutingClassAffordanceCandidate && row.UserFacingAllowed != UserFacingFalse {
		errs = append(errs, row.ClusterID+" routing_affordance_candidate must not be user-facing")
	}
	if row.RoutingClass == RoutingClassMethodPackBridgeOnly && !strings.Contains(strings.ToLower(row.MethodPackRelation), "methodpack") {
		errs = append(errs, row.ClusterID+" methodpack_bridge_only needs MethodPack relation")
	}
	return errs
}

func validateFPFEngineCoverageBoundary(row FPFEngineCoverageCluster) []string {
	var errs []string
	if row.UserFacingAllowed != UserFacingFalse &&
		strings.TrimSpace(row.OutputCarrierKind) == "" &&
		len(row.OwnerSurface) == 0 {
		errs = append(errs, row.ClusterID+" user-facing row needs output carrier or suggested owner surface")
	}
	if row.UserFacingAllowed == UserFacingFalse &&
		strings.TrimSpace(row.OutputCarrierKind) == "" &&
		strings.TrimSpace(row.NoneReason) == "" {
		errs = append(errs, row.ClusterID+" non-user-facing row needs none_reason when no output carrier exists")
	}
	switch row.RoutingClass {
	case RoutingClassKernelSubstrate, RoutingClassSourcePackMetadata, RoutingClassNeverRouteDirectly:
		if strings.TrimSpace(row.OutputCarrierKind) != "" {
			errs = append(errs, row.ClusterID+" substrate/non-route rows cannot emit output_carrier_kind")
		}
		if row.UserFacingAllowed != UserFacingFalse {
			errs = append(errs, row.ClusterID+" substrate/non-route rows must not be user-facing")
		}
	}
	return errs
}

func validFPFEngineRoutingClass(value string) bool {
	switch value {
	case RoutingClassNormativeSourceOnly,
		RoutingClassRetrievalCandidateOnly,
		RoutingClassAffordanceCandidate,
		RoutingClassCompiledPatternUseRoute,
		RoutingClassDedicatedSkillOrWorkflow,
		RoutingClassKernelSubstrate,
		RoutingClassMethodPackBridgeOnly,
		RoutingClassNeverRouteDirectly,
		RoutingClassSourcePackMetadata:
		return true
	default:
		return false
	}
}

func validFPFEngineUserFacing(value string) bool {
	switch value {
	case UserFacingFalse,
		UserFacingTrue,
		UserFacingTrueAsCandidateOnly,
		UserFacingTrueAsSuggestedSurface,
		UserFacingTrueOnlyAsSuggestedMethodRefs:
		return true
	default:
		return false
	}
}
