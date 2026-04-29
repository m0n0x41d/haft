package workcommission

import "testing"

func TestLifecycleSemanticsForTerminalAndRecoverableStates(t *testing.T) {
	cases := []struct {
		State               string
		Terminal            bool
		Recoverable         bool
		Runnable            bool
		Executing           bool
		SatisfiesDependency bool
	}{
		{
			State:       "failed",
			Recoverable: true,
		},
		{
			State:    "cancelled",
			Terminal: true,
		},
		{
			State:    "expired",
			Terminal: true,
		},
		{
			State:               "completed",
			Terminal:            true,
			SatisfiesDependency: true,
		},
		{
			State:               "completed_with_projection_debt",
			Terminal:            true,
			SatisfiesDependency: true,
		},
		{
			State:       "queued",
			Recoverable: true,
			Runnable:    true,
		},
		{
			State:       "ready",
			Recoverable: true,
			Runnable:    true,
		},
		{
			State:       "preflighting",
			Recoverable: true,
			Executing:   true,
		},
		{
			State:       "running",
			Recoverable: true,
			Executing:   true,
		},
	}

	for _, tc := range cases {
		if got := IsTerminalState(tc.State); got != tc.Terminal {
			t.Fatalf("%s terminal = %v, want %v", tc.State, got, tc.Terminal)
		}
		if got := IsRecoverableState(tc.State); got != tc.Recoverable {
			t.Fatalf("%s recoverable = %v, want %v", tc.State, got, tc.Recoverable)
		}
		if got := IsRunnableState(tc.State); got != tc.Runnable {
			t.Fatalf("%s runnable = %v, want %v", tc.State, got, tc.Runnable)
		}
		if got := IsExecutingState(tc.State); got != tc.Executing {
			t.Fatalf("%s executing = %v, want %v", tc.State, got, tc.Executing)
		}
		if got := SatisfiesDependencyState(tc.State); got != tc.SatisfiesDependency {
			t.Fatalf("%s dependency = %v, want %v", tc.State, got, tc.SatisfiesDependency)
		}
	}
}

func TestDeliveryAfterLocalEvidenceRequiresPassPolicyAndEnvelopeAllowed(t *testing.T) {
	cases := []struct {
		Name      string
		Policy    DeliveryPolicy
		Verdict   DeliveryVerdict
		Gate      DeliveryGate
		AutoApply bool
		Reason    string
	}{
		{
			Name:      "auto on pass allowed",
			Policy:    DeliveryPolicyWorkspacePatchAutoOnPass,
			Verdict:   DeliveryVerdictPass,
			Gate:      DeliveryGateAllowed,
			AutoApply: true,
			Reason:    "policy_auto_on_pass_and_envelope_allowed",
		},
		{
			Name:    "manual policy",
			Policy:  DeliveryPolicyWorkspacePatchManual,
			Verdict: DeliveryVerdictPass,
			Gate:    DeliveryGateAllowed,
			Reason:  "delivery_policy_manual",
		},
		{
			Name:    "blocked envelope",
			Policy:  DeliveryPolicyWorkspacePatchAutoOnPass,
			Verdict: DeliveryVerdictPass,
			Gate:    DeliveryGateBlocked,
			Reason:  "autonomy_envelope_blocked",
		},
		{
			Name:    "non pass verdict",
			Policy:  DeliveryPolicyWorkspacePatchAutoOnPass,
			Verdict: DeliveryVerdictNonPass,
			Gate:    DeliveryGateAllowed,
			Reason:  "verdict_not_pass",
		},
	}

	for _, tc := range cases {
		decision := DeliveryAfterLocalEvidence(tc.Policy, tc.Verdict, tc.Gate)
		if decision.AutoApply != tc.AutoApply {
			t.Fatalf("%s auto apply = %v, want %v", tc.Name, decision.AutoApply, tc.AutoApply)
		}
		if decision.Reason != tc.Reason {
			t.Fatalf("%s reason = %s, want %s", tc.Name, decision.Reason, tc.Reason)
		}
	}
}
