package fpf

import (
	"testing"
)

func TestGetRoleForTool(t *testing.T) {
	tests := []struct {
		tool string
		want Role
	}{
		// Initialization
		{"quint_init", RoleInitializer},
		{"quint_record_context", RoleInitializer},
		// ADI Cycle
		{"quint_propose", RoleAbductor},
		{"quint_verify", RoleDeductor},
		{"quint_test", RoleInductor},
		{"quint_audit", RoleAuditor},
		{"quint_decide", RoleDecider},
		// Maintenance
		{"quint_reset", RoleMaintainer},
		{"quint_check_decay", RoleMaintainer},
		{"quint_actualize", RoleMaintainer},
		// Read-only
		{"quint_status", RoleObserver},
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
		// Phase-gated tools
		{"quint_init", false, []Phase{PhaseIdle}},
		{"quint_record_context", false, []Phase{PhaseIdle}},
		{"quint_propose", false, []Phase{PhaseIdle, PhaseAbduction, PhaseDeduction, PhaseInduction}},
		{"quint_verify", false, []Phase{PhaseAbduction, PhaseDeduction}},
		{"quint_test", false, []Phase{PhaseDeduction, PhaseInduction}},
		{"quint_audit", false, []Phase{PhaseInduction, PhaseAudit}},
		{"quint_decide", false, []Phase{PhaseAudit, PhaseDecision}},
		// No phase gate (nil)
		{"quint_status", true, nil},
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
		// quint_init - only IDLE
		{"init_in_idle", "quint_init", PhaseIdle, true},
		{"init_in_abduction", "quint_init", PhaseAbduction, false},
		{"init_in_deduction", "quint_init", PhaseDeduction, false},

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
		{"status_in_idle", "quint_status", PhaseIdle, true},
		{"status_in_audit", "quint_status", PhaseAudit, true},
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
