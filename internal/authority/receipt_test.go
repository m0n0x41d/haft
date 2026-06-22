package authority

import (
	"testing"
	"time"
)

func TestEvaluateReceiptAcceptsMatchingManualCLIReceipt(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	evaluation := EvaluateReceipt(now, Receipt{
		Kind:                    ReceiptKindManualCLI,
		PrincipalIdentitySource: "local_user",
		HostSessionSource:       "local_cli",
		Tool:                    "haft_decision",
		Action:                  "decide",
		PayloadHash:             "sha256:abc",
		ExpiresAt:               now.Add(time.Hour).Format(time.RFC3339),
		SingleUse:               true,
	}, BindingAction{
		Tool:        "haft_decision",
		Action:      "decide",
		PayloadHash: "sha256:abc",
	})

	if evaluation.Status != ReceiptStatusValid {
		t.Fatalf("status = %q, want %q: %+v", evaluation.Status, ReceiptStatusValid, evaluation)
	}
}

func TestEvaluateReceiptRejectsModelSuppliedArguments(t *testing.T) {
	evaluation := EvaluateReceipt(time.Now(), Receipt{
		Kind:   ReceiptKindModel,
		Tool:   "haft_decision",
		Action: "decide",
	}, BindingAction{
		Tool:   "haft_decision",
		Action: "decide",
	})

	if evaluation.Status != ReceiptStatusUnsupportedFuture {
		t.Fatalf("status = %q, want %q", evaluation.Status, ReceiptStatusUnsupportedFuture)
	}
	if evaluation.RequiredKind != ReceiptKindManualCLI {
		t.Fatalf("required_kind = %q", evaluation.RequiredKind)
	}
	if evaluation.Reason == "" || evaluation.Reason == "only manual CLI receipts are accepted in v1; host receipts need a future verifier" {
		t.Fatalf("reason should name model arguments boundary, got %q", evaluation.Reason)
	}
}

func TestEvaluateReceiptRejectsIncompleteHostReceipt(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	evaluation := EvaluateReceipt(now, Receipt{
		Kind:                    ReceiptKindHost,
		PrincipalIdentitySource: "codex:user",
		HostSessionSource:       "codex:session",
		Tool:                    "haft_decision",
		Action:                  "decide",
		PayloadHash:             "sha256:abc",
		ExpiresAt:               now.Add(time.Hour).Format(time.RFC3339),
	}, BindingAction{
		Tool:        "haft_decision",
		Action:      "decide",
		PayloadHash: "sha256:abc",
	})

	if evaluation.Status != ReceiptStatusInvalid {
		t.Fatalf("status = %q, want %q", evaluation.Status, ReceiptStatusInvalid)
	}
	if evaluation.RequiredKind != ReceiptKindHost {
		t.Fatalf("required_kind = %q, want host receipt", evaluation.RequiredKind)
	}
	if evaluation.Reason != "host receipt is missing verifier source" {
		t.Fatalf("reason = %q", evaluation.Reason)
	}
}

func TestEvaluateReceiptFailsClosedForStructurallyCompleteHostReceipt(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	evaluation := EvaluateReceipt(now, Receipt{
		Kind:                    ReceiptKindHost,
		Source:                  "codex_desktop",
		PrincipalIdentitySource: "codex:user",
		HostSessionSource:       "codex:session",
		Tool:                    "haft_decision",
		Action:                  "decide",
		PayloadHash:             "sha256:abc",
		ExpiresAt:               now.Add(time.Hour).Format(time.RFC3339),
		SingleUse:               true,
	}, BindingAction{
		Tool:        "haft_decision",
		Action:      "decide",
		PayloadHash: "sha256:abc",
	})

	if evaluation.Status != ReceiptStatusUnsupportedFuture {
		t.Fatalf("status = %q, want %q", evaluation.Status, ReceiptStatusUnsupportedFuture)
	}
	if evaluation.RequiredKind != ReceiptKindHost {
		t.Fatalf("required_kind = %q, want host receipt", evaluation.RequiredKind)
	}
	if evaluation.Reason == "" {
		t.Fatal("reason must name missing verifier boundary")
	}
}

func TestEvaluateReceiptRejectsMismatchedPayloadHash(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	evaluation := EvaluateReceipt(now, Receipt{
		Kind:                    ReceiptKindManualCLI,
		PrincipalIdentitySource: "local_user",
		HostSessionSource:       "local_cli",
		Tool:                    "haft_commission",
		Action:                  "create",
		PayloadHash:             "sha256:old",
	}, BindingAction{
		Tool:        "haft_commission",
		Action:      "create",
		PayloadHash: "sha256:new",
	})

	if evaluation.Status != ReceiptStatusInvalid {
		t.Fatalf("status = %q, want %q", evaluation.Status, ReceiptStatusInvalid)
	}
}

func TestMCPCLIOnlyEvaluationNamesReceiptBoundary(t *testing.T) {
	evaluation := MCPCLIOnlyEvaluation()

	if evaluation.Status != ReceiptStatusUnsupportedInMCP {
		t.Fatalf("status = %q", evaluation.Status)
	}
	if evaluation.RequiredKind != ReceiptKindManualCLI {
		t.Fatalf("required_kind = %q", evaluation.RequiredKind)
	}
}
