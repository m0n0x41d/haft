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
		// Linking
		{"quint_link", RoleObserver},
		{"quint_implement", RoleObserver},
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
