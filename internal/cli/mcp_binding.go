package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/authority"
)

const mcpBindingModeCLIOnly = "cli-only"

type operatorConfirmationRequired struct {
	Code            string `json:"code"`
	Tool            string `json:"tool"`
	Action          string `json:"action"`
	BindingMode     string `json:"binding_mode"`
	Message         string `json:"message"`
	AllowedPath     string `json:"allowed_path"`
	CreatesArtifact bool   `json:"creates_artifact"`
	ReceiptStatus   string `json:"authorization_receipt_status"`
	RequiredReceipt string `json:"required_authorization_receipt"`
	ReceiptReason   string `json:"authorization_receipt_reason"`
}

type operatorConfirmationRequiredError struct {
	payload operatorConfirmationRequired
}

func (e operatorConfirmationRequiredError) Error() string {
	encoded, err := json.Marshal(e.payload)
	if err != nil {
		return fmt.Sprintf(
			`{"code":"operator_confirmation_required","tool":%q,"action":%q,"binding_mode":"cli-only"}`,
			e.payload.Tool,
			e.payload.Action,
		)
	}
	return string(encoded)
}

func rejectMCPBindingAction(name string, args map[string]any) error {
	action := strings.TrimSpace(stringArg(args, "action"))
	surface, requiresConfirmation := mcpBindingSurfaceRequiresOperatorConfirmation(name, action)
	if !requiresConfirmation {
		return nil
	}

	evaluation := authority.MCPCLIOnlyEvaluation()
	return operatorConfirmationRequiredError{
		payload: operatorConfirmationRequired{
			Code:            "operator_confirmation_required",
			Tool:            name,
			Action:          action,
			BindingMode:     mcpBindingModeCLIOnly,
			Message:         "This action is a binding governance act. The MCP server is in cli-only binding mode and cannot treat model-supplied arguments as proof of operator authorization.",
			AllowedPath:     mcpBindingAllowedPath(surface, name, action),
			CreatesArtifact: false,
			ReceiptStatus:   evaluation.Status,
			RequiredReceipt: evaluation.RequiredKind,
			ReceiptReason:   evaluation.Reason,
		},
	}
}

func mcpActionRequiresOperatorConfirmation(name, action string) bool {
	_, required := mcpBindingSurfaceRequiresOperatorConfirmation(name, action)
	return required
}

func mcpBindingSurfaceRequiresOperatorConfirmation(
	name string,
	action string,
) (bindingSurfaceInventoryEntry, bool) {
	surface, ok := lookupBindingSurface(name, action)
	if !ok {
		return bindingSurfaceInventoryEntry{}, false
	}
	return surface, surface.Enforcement == bindingEnforcementMCPOperatorConfirmationRequired
}

func mcpBindingAllowedPath(surface bindingSurfaceInventoryEntry, name, action string) string {
	if strings.TrimSpace(surface.AllowedPath) != "" {
		return surface.AllowedPath
	}
	return "Use a manual CLI or host workflow after explicit operator authorization."
}
