package authority

import (
	"strings"
	"time"
)

const (
	ReceiptKindManualCLI = "manual_cli"
	ReceiptKindHost      = "host_authorization_receipt"
	ReceiptKindModel     = "model_supplied_arguments"

	ReceiptStatusValid             = "valid"
	ReceiptStatusMissing           = "missing"
	ReceiptStatusInvalid           = "invalid"
	ReceiptStatusUnsupportedInMCP  = "unsupported_in_mcp_cli_only"
	ReceiptStatusUnsupportedFuture = "unsupported_future_host_receipt"
)

type BindingAction struct {
	Tool        string `json:"tool"`
	Action      string `json:"action"`
	PayloadHash string `json:"payload_hash,omitempty"`
}

type Receipt struct {
	Kind                    string `json:"kind"`
	PrincipalIdentitySource string `json:"principal_identity_source,omitempty"`
	HostSessionSource       string `json:"host_session_source,omitempty"`
	Tool                    string `json:"tool,omitempty"`
	Action                  string `json:"action,omitempty"`
	PayloadHash             string `json:"payload_hash,omitempty"`
	ExpiresAt               string `json:"expires_at,omitempty"`
	SingleUse               bool   `json:"single_use,omitempty"`
	VerificationResult      string `json:"verification_result,omitempty"`
}

type Evaluation struct {
	Status       string `json:"status"`
	RequiredKind string `json:"required_kind"`
	Reason       string `json:"reason"`
}

func EvaluateReceipt(now time.Time, receipt Receipt, action BindingAction) Evaluation {
	required := ReceiptKindManualCLI
	if strings.TrimSpace(receipt.Kind) == "" {
		return Evaluation{
			Status:       ReceiptStatusMissing,
			RequiredKind: required,
			Reason:       "no kernel-verifiable authorization receipt was provided",
		}
	}

	if receipt.Kind != ReceiptKindManualCLI {
		return Evaluation{
			Status:       ReceiptStatusUnsupportedFuture,
			RequiredKind: required,
			Reason:       "only manual CLI receipts are accepted in v1; host receipts need a future verifier",
		}
	}

	if strings.TrimSpace(receipt.PrincipalIdentitySource) == "" {
		return invalid(required, "manual CLI receipt is missing principal identity source")
	}
	if strings.TrimSpace(receipt.HostSessionSource) == "" {
		return invalid(required, "manual CLI receipt is missing host/session source")
	}
	if !sameToken(receipt.Tool, action.Tool) || !sameToken(receipt.Action, action.Action) {
		return invalid(required, "manual CLI receipt does not match requested binding action")
	}
	if receipt.PayloadHash != "" && action.PayloadHash != "" && receipt.PayloadHash != action.PayloadHash {
		return invalid(required, "manual CLI receipt payload hash does not match requested binding payload")
	}
	if receipt.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, receipt.ExpiresAt)
		if err != nil {
			return invalid(required, "manual CLI receipt expiry is not RFC3339")
		}
		if !now.Before(expiresAt) {
			return invalid(required, "manual CLI receipt is expired")
		}
	}

	return Evaluation{
		Status:       ReceiptStatusValid,
		RequiredKind: required,
		Reason:       "manual CLI receipt matches the requested binding action",
	}
}

func MCPCLIOnlyEvaluation() Evaluation {
	return Evaluation{
		Status:       ReceiptStatusUnsupportedInMCP,
		RequiredKind: ReceiptKindManualCLI,
		Reason:       "MCP cli-only mode cannot verify operator binding authorization; model-supplied arguments are not receipts",
	}
}

func invalid(required, reason string) Evaluation {
	return Evaluation{
		Status:       ReceiptStatusInvalid,
		RequiredKind: required,
		Reason:       reason,
	}
}

func sameToken(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}
