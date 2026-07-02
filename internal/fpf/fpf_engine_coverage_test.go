package fpf

import (
	"strings"
	"testing"
)

func TestDefaultFPFEngineCoverageMatrixValidates(t *testing.T) {
	matrix, err := DefaultFPFEngineCoverageMatrix()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateFPFEngineCoverageMatrix(matrix); err != nil {
		t.Fatalf("ValidateFPFEngineCoverageMatrix returned error: %v", err)
	}
	if len(matrix.Rows) != FPFEngineCoverageC0RowCount {
		t.Fatalf("rows = %d, want %d", len(matrix.Rows), FPFEngineCoverageC0RowCount)
	}
}

func TestFPFEngineCoverageMatrixHasReviewSeedClusters(t *testing.T) {
	matrix := mustDefaultFPFEngineCoverageMatrix(t)
	got := map[string]bool{}
	for _, row := range matrix.Rows {
		got[row.ClusterID] = true
	}

	for _, want := range []string{
		"PU.NAMING",
		"PU.ARCH",
		"PU.NEXT_MOVE",
		"PU.SOTA",
		"PU.DIAG",
		"PU.COMPARE",
		"PU.EVIDENCE",
		"PU.SEMIO",
		"PU.API_BOUNDARY",
		"PU.DECISION_AUTH",
		"PU.WORK_PLAN",
		"PU.AGENT_ACTION",
		"PU.SPEC_LIFECYCLE",
		"PU.LAYERS",
		"PU.MEASURE",
		"PU.PUBLICATION",
		"PU.LANGSTATE",
		"PU.CAUSAL_EVAL",
		"PU.TIME_FRESH",
		"PU.METHODPACK_BRIDGE",
		"PU.SOURCEPACK",
		"PU.PATTERN_QUALITY",
	} {
		if !got[want] {
			t.Fatalf("matrix missing %s", want)
		}
	}
}

func TestFPFEngineCoverageMatrixRejectsMissingBoundary(t *testing.T) {
	matrix := mustDefaultFPFEngineCoverageMatrix(t)
	matrix.Rows[0].AuthorityBoundary = ""

	err := ValidateFPFEngineCoverageMatrix(matrix)
	if err == nil {
		t.Fatal("ValidateFPFEngineCoverageMatrix accepted missing authority boundary")
	}
	if !strings.Contains(err.Error(), "authority_boundary") {
		t.Fatalf("error = %v, want authority_boundary", err)
	}
}

func TestFPFEngineCoverageMatrixRejectsUserFacingAffordanceCandidate(t *testing.T) {
	matrix := mustDefaultFPFEngineCoverageMatrix(t)
	for index := range matrix.Rows {
		if matrix.Rows[index].RoutingClass == RoutingClassAffordanceCandidate {
			matrix.Rows[index].UserFacingAllowed = UserFacingTrue
			break
		}
	}

	err := ValidateFPFEngineCoverageMatrix(matrix)
	if err == nil {
		t.Fatal("ValidateFPFEngineCoverageMatrix accepted user-facing routing_affordance_candidate")
	}
	if !strings.Contains(err.Error(), "routing_affordance_candidate must not be user-facing") {
		t.Fatalf("error = %v, want affordance user-facing failure", err)
	}
}

func TestFPFEngineCoverageMatrixKeepsSourcePackSubstrateNonRoute(t *testing.T) {
	matrix := mustDefaultFPFEngineCoverageMatrix(t)
	var sourcePack FPFEngineCoverageCluster
	for _, row := range matrix.Rows {
		if row.ClusterID == "PU.SOURCEPACK" {
			sourcePack = row
		}
	}
	if sourcePack.RoutingClass != RoutingClassKernelSubstrate {
		t.Fatalf("PU.SOURCEPACK routing_class = %q", sourcePack.RoutingClass)
	}
	if sourcePack.UserFacingAllowed != UserFacingFalse {
		t.Fatalf("PU.SOURCEPACK user_facing_allowed = %q", sourcePack.UserFacingAllowed)
	}
	if sourcePack.OutputCarrierKind != "" {
		t.Fatalf("PU.SOURCEPACK output_carrier_kind = %q, want empty", sourcePack.OutputCarrierKind)
	}
}

func TestFPFEngineCoverageMatrixReflectsC1RuntimeRoutes(t *testing.T) {
	if got := len(DefaultPatternUseRouteCards()); got != 15 {
		t.Fatalf("runtime compiled route cards = %d, want C1 route count 15", got)
	}
	matrix := mustDefaultFPFEngineCoverageMatrix(t)
	compiled := map[string]bool{}
	for _, row := range matrix.Rows {
		if row.RoutingClass != RoutingClassCompiledPatternUseRoute {
			continue
		}
		compiled[row.ClusterID] = true
	}
	for _, want := range []string{
		"PU.WORK_PLAN",
		"PU.AGENT_ACTION",
		"PU.SPEC_LIFECYCLE",
		"PU.LAYERS",
	} {
		if !compiled[want] {
			t.Fatalf("matrix row %s must reflect C1 compiled PatternUse support", want)
		}
	}
}

func mustDefaultFPFEngineCoverageMatrix(t *testing.T) FPFEngineCoverageMatrix {
	t.Helper()
	matrix, err := DefaultFPFEngineCoverageMatrix()
	if err != nil {
		t.Fatal(err)
	}
	return matrix
}
