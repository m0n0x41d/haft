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

func TestEvaluateReceiptAcceptsRegisteredHostVerifier(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	receipt := structurallyCompleteHostReceipt(now)
	action := BindingAction{
		Tool:        "haft_decision",
		Action:      "decide",
		PayloadHash: "sha256:abc",
	}
	registry := HostReceiptVerifierRegistry{
		"codex_desktop": HostReceiptVerifierFunc(func(
			gotNow time.Time,
			gotReceipt Receipt,
			gotAction BindingAction,
		) HostReceiptVerification {
			if !gotNow.Equal(now) {
				t.Fatalf("now = %s, want %s", gotNow, now)
			}
			if gotReceipt.Source != receipt.Source {
				t.Fatalf("receipt source = %q", gotReceipt.Source)
			}
			if gotAction.PayloadHash != action.PayloadHash {
				t.Fatalf("payload hash = %q", gotAction.PayloadHash)
			}
			return HostReceiptVerification{
				Valid:  true,
				Reason: "codex_desktop verifier confirmed principal, session, action, payload hash, expiry, and source",
			}
		}),
	}

	evaluation := EvaluateReceiptWithHostVerifiers(now, receipt, action, registry)

	if evaluation.Status != ReceiptStatusValid {
		t.Fatalf("status = %q, want %q: %+v", evaluation.Status, ReceiptStatusValid, evaluation)
	}
	if evaluation.RequiredKind != ReceiptKindHost {
		t.Fatalf("required_kind = %q, want host receipt", evaluation.RequiredKind)
	}
	if evaluation.Reason == "" {
		t.Fatal("reason is empty")
	}
}

func TestEvaluateReceiptRejectsRegisteredHostVerifierDenial(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	receipt := structurallyCompleteHostReceipt(now)
	registry := HostReceiptVerifierRegistry{
		"codex_desktop": HostReceiptVerifierFunc(func(
			time.Time,
			Receipt,
			BindingAction,
		) HostReceiptVerification {
			return HostReceiptVerification{
				Valid:  false,
				Reason: "host session no longer matches operator confirmation",
			}
		}),
	}

	evaluation := EvaluateReceiptWithHostVerifiers(now, receipt, BindingAction{
		Tool:        "haft_decision",
		Action:      "decide",
		PayloadHash: "sha256:abc",
	}, registry)

	if evaluation.Status != ReceiptStatusInvalid {
		t.Fatalf("status = %q, want %q", evaluation.Status, ReceiptStatusInvalid)
	}
	if evaluation.Reason != "host session no longer matches operator confirmation" {
		t.Fatalf("reason = %q", evaluation.Reason)
	}
}

func TestEvaluateReceiptRejectsRegisteredHostVerifierWithoutReason(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	receipt := structurallyCompleteHostReceipt(now)
	registry := HostReceiptVerifierRegistry{
		"codex_desktop": HostReceiptVerifierFunc(func(
			time.Time,
			Receipt,
			BindingAction,
		) HostReceiptVerification {
			return HostReceiptVerification{Valid: true}
		}),
	}

	evaluation := EvaluateReceiptWithHostVerifiers(now, receipt, BindingAction{
		Tool:        "haft_decision",
		Action:      "decide",
		PayloadHash: "sha256:abc",
	}, registry)

	if evaluation.Status != ReceiptStatusInvalid {
		t.Fatalf("status = %q, want %q", evaluation.Status, ReceiptStatusInvalid)
	}
	if evaluation.Reason != "host receipt verifier returned no reason" {
		t.Fatalf("reason = %q", evaluation.Reason)
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

func structurallyCompleteHostReceipt(now time.Time) Receipt {
	return Receipt{
		Kind:                    ReceiptKindHost,
		Source:                  "codex_desktop",
		PrincipalIdentitySource: "codex:user",
		HostSessionSource:       "codex:session",
		Tool:                    "haft_decision",
		Action:                  "decide",
		PayloadHash:             "sha256:abc",
		ExpiresAt:               now.Add(time.Hour).Format(time.RFC3339),
		SingleUse:               true,
	}
}
