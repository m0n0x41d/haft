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
	Source                  string `json:"source,omitempty"`
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

type HostReceiptVerifier interface {
	VerifyHostReceipt(now time.Time, receipt Receipt, action BindingAction) HostReceiptVerification
}

type HostReceiptVerifierFunc func(time.Time, Receipt, BindingAction) HostReceiptVerification

func (f HostReceiptVerifierFunc) VerifyHostReceipt(
	now time.Time,
	receipt Receipt,
	action BindingAction,
) HostReceiptVerification {
	return f(now, receipt, action)
}

type HostReceiptVerification struct {
	Valid  bool
	Reason string
}

type HostReceiptVerifierRegistry map[string]HostReceiptVerifier

type receiptEvaluator func(
	time.Time,
	Receipt,
	BindingAction,
	HostReceiptVerifierRegistry,
) Evaluation

var receiptEvaluators = map[string]receiptEvaluator{
	ReceiptKindManualCLI: evaluateManualCLIReceipt,
	ReceiptKindHost:      evaluateHostReceipt,
	ReceiptKindModel:     evaluateModelSuppliedReceipt,
}

func EvaluateReceipt(now time.Time, receipt Receipt, action BindingAction) Evaluation {
	return EvaluateReceiptWithHostVerifiers(now, receipt, action, nil)
}

// EvaluateReceiptWithHostVerifiers is a legacy diagnostic helper. Even a
// ReceiptStatusValid Evaluation is not a canonical permission and has no
// conversion path to KernelGate's package-private admitted-use snapshot.
func EvaluateReceiptWithHostVerifiers(
	now time.Time,
	receipt Receipt,
	action BindingAction,
	verifiers HostReceiptVerifierRegistry,
) Evaluation {
	if strings.TrimSpace(receipt.Kind) == "" {
		return Evaluation{
			Status:       ReceiptStatusMissing,
			RequiredKind: ReceiptKindManualCLI,
			Reason:       "no kernel-verifiable authorization receipt was provided",
		}
	}

	evaluator, ok := receiptEvaluators[receipt.Kind]
	if !ok {
		return unsupportedReceiptKind()
	}
	return evaluator(now, receipt, action, verifiers)
}

func evaluateManualCLIReceipt(
	now time.Time,
	receipt Receipt,
	action BindingAction,
	_ HostReceiptVerifierRegistry,
) Evaluation {
	required := ReceiptKindManualCLI

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

func evaluateModelSuppliedReceipt(
	_ time.Time,
	_ Receipt,
	_ BindingAction,
	_ HostReceiptVerifierRegistry,
) Evaluation {
	return Evaluation{
		Status:       ReceiptStatusUnsupportedFuture,
		RequiredKind: ReceiptKindManualCLI,
		Reason:       "model-supplied arguments are not authorization receipts",
	}
}

func unsupportedReceiptKind() Evaluation {
	return Evaluation{
		Status:       ReceiptStatusUnsupportedFuture,
		RequiredKind: ReceiptKindManualCLI,
		Reason:       "only manual CLI receipts are accepted in v1; host receipts need a future verifier",
	}
}

func evaluateHostReceipt(
	now time.Time,
	receipt Receipt,
	action BindingAction,
	verifiers HostReceiptVerifierRegistry,
) Evaluation {
	required := ReceiptKindHost
	if strings.TrimSpace(receipt.Source) == "" {
		return invalid(required, "host receipt is missing verifier source")
	}
	if strings.TrimSpace(receipt.PrincipalIdentitySource) == "" {
		return invalid(required, "host receipt is missing principal identity source")
	}
	if strings.TrimSpace(receipt.HostSessionSource) == "" {
		return invalid(required, "host receipt is missing host/session source")
	}
	if !sameToken(receipt.Tool, action.Tool) || !sameToken(receipt.Action, action.Action) {
		return invalid(required, "host receipt does not match requested binding action")
	}
	if strings.TrimSpace(receipt.PayloadHash) == "" {
		return invalid(required, "host receipt is missing payload hash")
	}
	if strings.TrimSpace(action.PayloadHash) == "" {
		return invalid(required, "binding action is missing payload hash for host receipt verification")
	}
	if receipt.PayloadHash != action.PayloadHash {
		return invalid(required, "host receipt payload hash does not match requested binding payload")
	}
	if strings.TrimSpace(receipt.ExpiresAt) == "" {
		return invalid(required, "host receipt is missing expiry")
	}
	expiresAt, err := time.Parse(time.RFC3339, receipt.ExpiresAt)
	if err != nil {
		return invalid(required, "host receipt expiry is not RFC3339")
	}
	if !now.Before(expiresAt) {
		return invalid(required, "host receipt is expired")
	}

	source := strings.TrimSpace(receipt.Source)
	verifier, ok := verifiers[source]
	if !ok || verifier == nil {
		return Evaluation{
			Status:       ReceiptStatusUnsupportedFuture,
			RequiredKind: required,
			Reason:       "host receipt is structurally complete, but no kernel verifier is registered for source " + source,
		}
	}

	verification := verifier.VerifyHostReceipt(now, receipt, action)
	reason := strings.TrimSpace(verification.Reason)
	if reason == "" {
		return invalid(required, "host receipt verifier returned no reason")
	}
	if !verification.Valid {
		return invalid(required, reason)
	}
	return Evaluation{
		Status:       ReceiptStatusValid,
		RequiredKind: required,
		Reason:       reason,
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
