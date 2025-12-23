package fpf

// Role definitions - expanded from fsm.go
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
	// Unified Entry Point (replaces quint_init, quint_status, quint_actualize, quint_check_decay)
	"quint_internalize": RoleObserver,

	// Search
	"quint_search": RoleObserver,

	// Decision Resolution (reconciliation, same category as internalize)
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
}

// ToolPhaseGate maps tool name → allowed phases.
// nil = no restriction (any phase allowed).
var ToolPhaseGate = map[string][]Phase{
	// Unified entry point - allowed in any phase
	"quint_internalize": nil,

	// Search - allowed in any phase (read-only)
	"quint_search": nil,

	// Decision resolution - allowed in any phase (reconciliation)
	"quint_resolve": nil,

	// Abduction - allows regression from later phases (DED, IND)
	// Blocked in AUDIT and DECISION to prevent disruption during finalization
	"quint_propose": {PhaseIdle, PhaseAbduction, PhaseDeduction, PhaseInduction},

	// Deduction
	"quint_verify": {PhaseAbduction, PhaseDeduction},

	// Induction - L1 promotion checked here; L2 refresh bypasses in preconditions
	"quint_test": {PhaseDeduction, PhaseInduction},

	// Audit
	"quint_audit": {PhaseInduction, PhaseAudit},

	// Decision
	"quint_decide": {PhaseAudit, PhaseDecision},

	// No phase gate (nil = allowed anytime)
	"quint_reset":       nil,
	"quint_calculate_r": nil,
	"quint_audit_tree":  nil,
}

// GetRoleForTool returns the role associated with a tool.
// Returns RoleObserver for unknown tools (safe default).
func GetRoleForTool(toolName string) Role {
	if role, ok := ToolRole[toolName]; ok {
		return role
	}
	return RoleObserver
}

// GetAllowedPhases returns the phases in which a tool can be called.
// Returns nil if no restriction (tool allowed in any phase).
func GetAllowedPhases(toolName string) []Phase {
	return ToolPhaseGate[toolName]
}

// IsPhaseAllowed checks if a tool can be called in the current phase.
func IsPhaseAllowed(toolName string, currentPhase Phase) bool {
	allowed := GetAllowedPhases(toolName)
	if allowed == nil {
		return true // no restriction
	}
	for _, p := range allowed {
		if p == currentPhase {
			return true
		}
	}
	return false
}

// GetExpectedRole returns a human-readable description of expected roles for a phase.
func GetExpectedRole(phase Phase) string {
	switch phase {
	case PhaseIdle:
		return "Initializer or Abductor"
	case PhaseAbduction:
		return "Abductor or Deductor"
	case PhaseDeduction:
		return "Deductor or Inductor"
	case PhaseInduction:
		return "Inductor or Auditor"
	case PhaseAudit:
		return "Auditor or Decider"
	case PhaseDecision:
		return "Decider"
	case PhaseOperation:
		return "Decider"
	default:
		return "Unknown"
	}
}
