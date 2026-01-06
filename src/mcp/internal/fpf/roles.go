package fpf

type Role string

const (
	RoleInitializer Role = "Initializer"
	RoleAbductor    Role = "Abductor"
	RoleDeductor    Role = "Deductor"
	RoleInductor    Role = "Inductor"
	RoleAuditor     Role = "Auditor"
	RoleDecider     Role = "Decider"
	RoleObserver    Role = "Observer"
	RoleMaintainer  Role = "Maintainer"
)

// ToolRole maps tool name → role (static, deterministic).
// Role is implicit - derived from tool name, not passed by agent.
var ToolRole = map[string]Role{
	// Unified Entry Point
	"quint_internalize": RoleObserver,

	// Search
	"quint_search": RoleObserver,

	// Decision Resolution
	"quint_resolve": RoleObserver,

	// ADI Cycle
	"quint_propose": RoleAbductor,
	"quint_verify":  RoleDeductor,
	"quint_test":    RoleInductor,
	"quint_audit":   RoleAuditor,
	"quint_decide":  RoleDecider,

	// Maintenance
	"quint_reset": RoleMaintainer,

	// Read-only
	"quint_calculate_r": RoleObserver,
	"quint_audit_tree":  RoleObserver,

	// Linking
	"quint_link":      RoleObserver,
	"quint_implement": RoleObserver,
}

// GetRoleForTool returns the role associated with a tool.
// Returns RoleObserver for unknown tools (safe default).
func GetRoleForTool(toolName string) Role {
	if role, ok := ToolRole[toolName]; ok {
		return role
	}
	return RoleObserver
}
