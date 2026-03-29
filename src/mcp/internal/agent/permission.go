package agent

import "strings"

// destructivePatterns are shell commands that require user approval.
var destructivePatterns = []string{
	"rm -rf",
	"rm -r",
	"git push --force",
	"git push -f",
	"git reset --hard",
	"git checkout -- .",
	"git clean -f",
	"git branch -D",
	"drop table",
	"drop database",
	"truncate",
	"> /dev/",
	"chmod 777",
	"kill -9",
	"pkill",
	"killall",
	"shutdown",
	"reboot",
	"mkfs",
	"dd if=",
}

// EvaluatePermission determines whether a tool call needs user approval.
// Pure function — no I/O, fully testable.
func EvaluatePermission(toolName string, args string) PermissionLevel {
	switch toolName {
	// Read-only tools — always safe
	case "read", "glob", "grep":
		return PermissionAllowed
	// Core agent infrastructure — internal, no user data mutation
	case "spawn_agent", "wait_agent":
		return PermissionAllowed
	// Quint kernel tools — internal artifact operations
	case "quint_problem", "quint_solution", "quint_decision",
		"quint_query", "quint_refresh", "quint_note":
		return PermissionAllowed
	// File mutations — need approval
	case "write", "edit":
		return PermissionNeedsApproval
	case "bash":
		return evaluateBashPermission(args)
	default:
		return PermissionNeedsApproval
	}
}

func evaluateBashPermission(args string) PermissionLevel {
	lower := strings.ToLower(args)
	for _, pattern := range destructivePatterns {
		if strings.Contains(lower, pattern) {
			return PermissionNeedsApproval
		}
	}
	return PermissionAllowed
}
