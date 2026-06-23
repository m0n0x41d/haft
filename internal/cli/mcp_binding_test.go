package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestDispatchToolRejectsMCPBindingDecisionWithoutCreatingArtifact(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	result, createdRef, err := dispatchTool(ctx, store, nil, haftDir, "haft_decision", map[string]any{
		"action":           "decide",
		"selected_title":   "Unauthorized MCP decision",
		"why_selected":     "A model-supplied argument is not operator authorization.",
		"selection_policy": "Reject binding acts at MCP ingress.",
	})

	if err == nil {
		t.Fatalf("dispatchTool returned nil error with result %q", result)
	}
	if createdRef != "" {
		t.Fatalf("createdRef = %q, want empty", createdRef)
	}
	assertOperatorConfirmationRequired(t, err, "haft_decision", "decide")

	decisions, listErr := store.ListByKind(ctx, artifact.KindDecisionRecord, 10)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(decisions) != 0 {
		t.Fatalf("unauthorized MCP decide created %d decision(s)", len(decisions))
	}
}

func TestDispatchToolRejectsMCPBindingActionsBeforeHandlers(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	cases := []struct {
		name   string
		tool   string
		action string
		args   map[string]any
	}{
		{
			name:   "commission create",
			tool:   "haft_commission",
			action: "create",
			args: map[string]any{
				"action": "create",
			},
		},
		{
			name:   "spec approve",
			tool:   "haft_spec_section",
			action: "approve",
			args: map[string]any{
				"action":     "approve",
				"section_id": "TS.example.001",
			},
		},
		{
			name:   "refresh supersede",
			tool:   "haft_refresh",
			action: "supersede",
			args: map[string]any{
				"action":       "supersede",
				"artifact_ref": "dec-example",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, createdRef, err := dispatchTool(ctx, store, nil, haftDir, tc.tool, tc.args)
			if err == nil {
				t.Fatalf("dispatchTool returned nil error with result %q", result)
			}
			if createdRef != "" {
				t.Fatalf("createdRef = %q, want empty", createdRef)
			}
			assertOperatorConfirmationRequired(t, err, tc.tool, tc.action)
		})
	}
}

func TestDispatchToolAllowsMCPReadAndEvidenceDiscoveryActions(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, _, err := dispatchTool(ctx, store, nil, haftDir, "haft_query", map[string]any{
		"action": "status",
	})
	if err != nil {
		t.Fatalf("read-only query rejected: %v", err)
	}

	_, _, err = dispatchTool(ctx, store, nil, haftDir, "haft_spec_section", map[string]any{
		"action":       "lifecycle",
		"project_root": haftDir,
	})
	if err != nil && strings.Contains(err.Error(), "operator_confirmation_required") {
		t.Fatalf("spec lifecycle was treated as binding: %v", err)
	}
}

func assertOperatorConfirmationRequired(t *testing.T, err error, tool, action string) {
	t.Helper()

	var payload operatorConfirmationRequired
	if unmarshalErr := json.Unmarshal([]byte(err.Error()), &payload); unmarshalErr != nil {
		t.Fatalf("error is not structured JSON: %v\n%v", unmarshalErr, err)
	}
	if payload.Code != "operator_confirmation_required" {
		t.Fatalf("code = %q", payload.Code)
	}
	if payload.Tool != tool {
		t.Fatalf("tool = %q, want %q", payload.Tool, tool)
	}
	if payload.Action != action {
		t.Fatalf("action = %q, want %q", payload.Action, action)
	}
	if payload.BindingMode != mcpBindingModeCLIOnly {
		t.Fatalf("binding_mode = %q", payload.BindingMode)
	}
	if payload.CreatesArtifact {
		t.Fatal("denial must not report artifact creation")
	}
	if payload.ReceiptStatus != "unsupported_in_mcp_cli_only" {
		t.Fatalf("authorization_receipt_status = %q", payload.ReceiptStatus)
	}
	if payload.RequiredReceipt != "manual_cli" {
		t.Fatalf("required_authorization_receipt = %q", payload.RequiredReceipt)
	}
	if payload.ReceiptReason == "" {
		t.Fatal("authorization_receipt_reason is empty")
	}
	assertAuthorizationReceiptCandidate(t, payload, "manual_cli", "accepted_by_manual_cli_binding_path", []string{
		"principal_identity_source",
		"host_session_source",
		"tool",
		"action",
	})
	assertAuthorizationReceiptCandidate(t, payload, "host_authorization_receipt", "requires_registered_kernel_verifier_not_enabled_in_default_mcp_cli_only", []string{
		"source",
		"principal_identity_source",
		"host_session_source",
		"tool",
		"action",
		"payload_hash",
		"expires_at",
		"registered_kernel_verifier",
	})
	if !strings.Contains(payload.Message, "model-supplied arguments") {
		t.Fatalf("message does not explain model-supplied args boundary: %q", payload.Message)
	}
	if payload.AllowedPath == "" {
		t.Fatal("allowed_path is empty")
	}
}

func assertAuthorizationReceiptCandidate(
	t *testing.T,
	payload operatorConfirmationRequired,
	kind string,
	status string,
	requirements []string,
) {
	t.Helper()

	for _, candidate := range payload.ReceiptCandidates {
		if candidate.Kind != kind {
			continue
		}
		if candidate.Status != status {
			t.Fatalf("%s candidate status = %q, want %q", kind, candidate.Status, status)
		}
		for _, requirement := range requirements {
			if !stringSliceContains(candidate.Requirements, requirement) {
				t.Fatalf("%s candidate requirements = %#v, missing %q", kind, candidate.Requirements, requirement)
			}
		}
		return
	}

	t.Fatalf("authorization_receipt_candidates missing %q: %#v", kind, payload.ReceiptCandidates)
}
