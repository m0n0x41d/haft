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

func TestGetAllowedPhases(t *testing.T) {
	tests := []struct {
		tool    string
		wantNil bool
		want    []Phase
	}{
		// Unified entry point - allowed in any phase
		{"quint_internalize", true, nil},
		{"quint_search", true, nil},
		// Phase-gated tools
		{"quint_propose", false, []Phase{PhaseIdle, PhaseAbduction, PhaseDeduction, PhaseInduction}},
		{"quint_verify", false, []Phase{PhaseAbduction, PhaseDeduction}},
		{"quint_test", false, []Phase{PhaseDeduction, PhaseInduction}},
		{"quint_audit", false, []Phase{PhaseInduction, PhaseAudit}},
		{"quint_decide", false, []Phase{PhaseAudit, PhaseDecision}},
		// No phase gate (nil)
		{"quint_calculate_r", true, nil},
		{"quint_reset", true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := GetAllowedPhases(tt.tool)
			if tt.wantNil {
				if got != nil {
					t.Errorf("GetAllowedPhases(%q) = %v, want nil", tt.tool, got)
				}
			} else {
				if got == nil {
					t.Errorf("GetAllowedPhases(%q) = nil, want %v", tt.tool, tt.want)
				} else if len(got) != len(tt.want) {
					t.Errorf("GetAllowedPhases(%q) = %v, want %v", tt.tool, got, tt.want)
				}
			}
		})
	}
}

func TestIsPhaseAllowed(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		phase   Phase
		allowed bool
	}{
		// quint_internalize - allowed in any phase
		{"internalize_in_idle", "quint_internalize", PhaseIdle, true},
		{"internalize_in_abduction", "quint_internalize", PhaseAbduction, true},
		{"internalize_in_decision", "quint_internalize", PhaseDecision, true},

		// quint_search - allowed in any phase
		{"search_in_idle", "quint_search", PhaseIdle, true},
		{"search_in_audit", "quint_search", PhaseAudit, true},

		// quint_propose - IDLE, ABD, DED, IND (regression allowed)
		{"propose_in_idle", "quint_propose", PhaseIdle, true},
		{"propose_in_abduction", "quint_propose", PhaseAbduction, true},
		{"propose_in_deduction", "quint_propose", PhaseDeduction, true},
		{"propose_in_induction", "quint_propose", PhaseInduction, true},
		{"propose_in_audit", "quint_propose", PhaseAudit, false},
		{"propose_in_decision", "quint_propose", PhaseDecision, false},

		// quint_verify - ABD, DED
		{"verify_in_idle", "quint_verify", PhaseIdle, false},
		{"verify_in_abduction", "quint_verify", PhaseAbduction, true},
		{"verify_in_deduction", "quint_verify", PhaseDeduction, true},
		{"verify_in_induction", "quint_verify", PhaseInduction, false},

		// quint_test - DED, IND
		{"test_in_deduction", "quint_test", PhaseDeduction, true},
		{"test_in_induction", "quint_test", PhaseInduction, true},
		{"test_in_idle", "quint_test", PhaseIdle, false},
		{"test_in_audit", "quint_test", PhaseAudit, false},

		// quint_audit - IND, AUDIT
		{"audit_in_induction", "quint_audit", PhaseInduction, true},
		{"audit_in_audit", "quint_audit", PhaseAudit, true},
		{"audit_in_idle", "quint_audit", PhaseIdle, false},

		// quint_decide - AUDIT, DECISION
		{"decide_in_audit", "quint_decide", PhaseAudit, true},
		{"decide_in_decision", "quint_decide", PhaseDecision, true},
		{"decide_in_idle", "quint_decide", PhaseIdle, false},

		// No phase gate - allowed anywhere
		{"calculate_r_in_any", "quint_calculate_r", PhaseDecision, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPhaseAllowed(tt.tool, tt.phase)
			if got != tt.allowed {
				t.Errorf("IsPhaseAllowed(%q, %v) = %v, want %v", tt.tool, tt.phase, got, tt.allowed)
			}
		})
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
