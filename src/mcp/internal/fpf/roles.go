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
//
// DESIGN DECISION: All phase gates removed.
// Semantic preconditions in preconditions.go provide sufficient validation:
// - quint_verify checks "hypothesis must be in L0"
// - quint_test checks "hypothesis must be in L1 or L2"
// - quint_audit checks "hypothesis must be in L2"
// - quint_decide checks "at least one L2 must exist"
//
// Phase gates were redundant and caused batch operation failures.
// DerivePhase remains for informational purposes (quint_internalize status).
// See: git history for 0690a2c, 443be87, 4a84ce0 for the whack-a-mole pattern.
var ToolPhaseGate = map[string][]Phase{
	"quint_internalize": nil,
	"quint_search":      nil,
	"quint_resolve":     nil,
	"quint_propose":     nil,
	"quint_verify":      nil,
	"quint_test":        nil,
	"quint_audit":       nil,
	"quint_decide":      nil,
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
