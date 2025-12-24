package fpf

import (
	"testing"
)

func TestGetRoleForTool(t *testing.T) {
	tests := []struct {
		tool string
		want Role
	}{
		// Unified entry point
		{"quint_internalize", RoleObserver},
		{"quint_search", RoleObserver},
		// ADI Cycle
		{"quint_propose", RoleAbductor},
		{"quint_verify", RoleDeductor},
		{"quint_test", RoleInductor},
		{"quint_audit", RoleAuditor},
		{"quint_decide", RoleDecider},
		// Maintenance
		{"quint_reset", RoleMaintainer},
		// Read-only
		{"quint_calculate_r", RoleObserver},
		{"quint_audit_tree", RoleObserver},
		// Unknown tool defaults to Observer
		{"unknown_tool", RoleObserver},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := GetRoleForTool(tt.tool)
			if got != tt.want {
				t.Errorf("GetRoleForTool(%q) = %v, want %v", tt.tool, got, tt.want)
			}
		})
	}
}

// TestGetAllowedPhases verifies all tools have no phase gates (nil).
// Phase gates were removed - semantic preconditions are sufficient.
// See roles.go for design decision.
func TestGetAllowedPhases(t *testing.T) {
	tools := []string{
		"quint_internalize",
		"quint_search",
		"quint_resolve",
		"quint_propose",
		"quint_verify",
		"quint_test",
		"quint_audit",
		"quint_decide",
		"quint_reset",
		"quint_calculate_r",
		"quint_audit_tree",
	}

	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			got := GetAllowedPhases(tool)
			if got != nil {
				t.Errorf("GetAllowedPhases(%q) = %v, want nil (no phase gates)", tool, got)
			}
		})
	}
}

// TestIsPhaseAllowed verifies all tools are allowed in any phase.
// Phase gates were removed - semantic preconditions are sufficient.
func TestIsPhaseAllowed(t *testing.T) {
	tools := []string{
		"quint_internalize",
		"quint_search",
		"quint_propose",
		"quint_verify",
		"quint_test",
		"quint_audit",
		"quint_decide",
		"quint_calculate_r",
	}

	phases := []Phase{
		PhaseIdle,
		PhaseAbduction,
		PhaseDeduction,
		PhaseInduction,
		PhaseAudit,
		PhaseDecision,
	}

	for _, tool := range tools {
		for _, phase := range phases {
			name := tool + "_in_" + string(phase)
			t.Run(name, func(t *testing.T) {
				got := IsPhaseAllowed(tool, phase)
				if !got {
					t.Errorf("IsPhaseAllowed(%q, %v) = false, want true (no phase gates)", tool, phase)
				}
			})
		}
	}
}

func TestGetExpectedRole(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseIdle, "Initializer or Abductor"},
		{PhaseAbduction, "Abductor or Deductor"},
		{PhaseDeduction, "Deductor or Inductor"},
		{PhaseInduction, "Inductor or Auditor"},
		{PhaseAudit, "Auditor or Decider"},
		{PhaseDecision, "Decider"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			got := GetExpectedRole(tt.phase)
			if got != tt.want {
				t.Errorf("GetExpectedRole(%v) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}
