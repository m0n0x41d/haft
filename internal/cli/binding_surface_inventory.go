package cli

const (
	bindingSurfaceReadOnly                   = "read_only"
	bindingSurfaceDraftOnly                  = "draft_only"
	bindingSurfaceEvidenceRecording          = "evidence_recording"
	bindingSurfaceBindingAuthorityMutation   = "binding_authority_mutation"
	bindingSurfaceLifecycleAuthorityMutation = "lifecycle_authority_mutation"
	bindingSurfaceExecutionAuthorityMutation = "execution_authority_mutation"

	bindingEnforcementMCPAllowed                       = "mcp_allowed"
	bindingEnforcementMCPAllowedExistingArtifact       = "mcp_allowed_existing_artifact"
	bindingEnforcementMCPAllowedMaintenancePolicyGated = "mcp_allowed_maintenance_policy_gated"
	bindingEnforcementMCPOperatorConfirmationRequired  = "mcp_operator_confirmation_required"
)

type bindingSurfaceInventoryEntry struct {
	Tool        string
	Action      string
	Class       string
	Enforcement string
	AllowedPath string
}

func bindingSurfaceInventory() []bindingSurfaceInventoryEntry {
	return []bindingSurfaceInventoryEntry{
		{Tool: "haft_note", Class: bindingSurfaceEvidenceRecording, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_problem", Action: "frame", Class: bindingSurfaceDraftOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_problem", Action: "characterize", Class: bindingSurfaceDraftOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_problem", Action: "select", Class: bindingSurfaceDraftOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_problem", Action: "close", Class: bindingSurfaceLifecycleAuthorityMutation, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_solution", Action: "explore", Class: bindingSurfaceDraftOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_solution", Action: "compare", Class: bindingSurfaceDraftOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_solution", Action: "similar", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{
			Tool:        "haft_decision",
			Action:      "decide",
			Class:       bindingSurfaceBindingAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use the manual h-decide/CLI decision workflow after explicit operator authorization.",
		},
		{Tool: "haft_decision", Action: "apply", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_decision", Action: "measure", Class: bindingSurfaceEvidenceRecording, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_decision", Action: "evidence", Class: bindingSurfaceEvidenceRecording, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_decision", Action: "baseline", Class: bindingSurfaceEvidenceRecording, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_query", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_method", Action: "pull", Class: bindingSurfaceDraftOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_method", Action: "close", Class: bindingSurfaceEvidenceRecording, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_method", Action: "show", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_method", Action: "detail", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_method", Action: "status", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{
			Tool:        "haft_commission",
			Action:      "create",
			Class:       bindingSurfaceExecutionAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use the manual h-commission/CLI commission workflow after explicit operator authorization.",
		},
		{
			Tool:        "haft_commission",
			Action:      "create_from_decision",
			Class:       bindingSurfaceExecutionAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use the manual h-commission/CLI commission workflow after explicit operator authorization.",
		},
		{
			Tool:        "haft_commission",
			Action:      "create_batch_from_decisions",
			Class:       bindingSurfaceExecutionAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use the manual h-commission/CLI commission workflow after explicit operator authorization.",
		},
		{
			Tool:        "haft_commission",
			Action:      "create_from_plan",
			Class:       bindingSurfaceExecutionAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use the manual h-commission/CLI commission workflow after explicit operator authorization.",
		},
		{Tool: "haft_commission", Action: "list", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_commission", Action: "list_runnable", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_commission", Action: "show", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_commission", Action: "claim_for_preflight", Class: bindingSurfaceLifecycleAuthorityMutation, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_commission", Action: "record_preflight", Class: bindingSurfaceLifecycleAuthorityMutation, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_commission", Action: "start_after_preflight", Class: bindingSurfaceLifecycleAuthorityMutation, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_commission", Action: "record_run_event", Class: bindingSurfaceEvidenceRecording, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_commission", Action: "complete_or_block", Class: bindingSurfaceLifecycleAuthorityMutation, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_commission", Action: "requeue", Class: bindingSurfaceLifecycleAuthorityMutation, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_commission", Action: "cancel", Class: bindingSurfaceLifecycleAuthorityMutation, Enforcement: bindingEnforcementMCPAllowedExistingArtifact},
		{Tool: "haft_spec_section", Action: "lifecycle", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_spec_section", Action: "next_step", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{
			Tool:        "haft_spec_section",
			Action:      "approve",
			Class:       bindingSurfaceLifecycleAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use `haft spec approve` or an equivalent host workflow that records explicit operator authorization.",
		},
		{
			Tool:        "haft_spec_section",
			Action:      "rebaseline",
			Class:       bindingSurfaceLifecycleAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use `haft spec rebaseline` or an equivalent host workflow that records explicit operator authorization.",
		},
		{
			Tool:        "haft_spec_section",
			Action:      "reopen",
			Class:       bindingSurfaceLifecycleAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use `haft spec reopen` or an equivalent host workflow that records explicit operator authorization.",
		},
		{Tool: "haft_refresh", Action: "scan", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_refresh", Action: "plan", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_refresh", Action: "review", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
		{Tool: "haft_refresh", Action: "drain", Class: bindingSurfaceLifecycleAuthorityMutation, Enforcement: bindingEnforcementMCPAllowedMaintenancePolicyGated},
		{
			Tool:        "haft_refresh",
			Action:      "waive",
			Class:       bindingSurfaceLifecycleAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use the manual CLI refresh/lifecycle workflow after explicit operator authorization.",
		},
		{
			Tool:        "haft_refresh",
			Action:      "reopen",
			Class:       bindingSurfaceLifecycleAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use the manual CLI refresh/lifecycle workflow after explicit operator authorization.",
		},
		{
			Tool:        "haft_refresh",
			Action:      "supersede",
			Class:       bindingSurfaceLifecycleAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use the manual CLI refresh/lifecycle workflow after explicit operator authorization.",
		},
		{
			Tool:        "haft_refresh",
			Action:      "deprecate",
			Class:       bindingSurfaceLifecycleAuthorityMutation,
			Enforcement: bindingEnforcementMCPOperatorConfirmationRequired,
			AllowedPath: "Use the manual CLI refresh/lifecycle workflow after explicit operator authorization.",
		},
		{Tool: "haft_refresh", Action: "reconcile", Class: bindingSurfaceReadOnly, Enforcement: bindingEnforcementMCPAllowed},
	}
}

func lookupBindingSurface(tool string, action string) (bindingSurfaceInventoryEntry, bool) {
	for _, entry := range bindingSurfaceInventory() {
		if entry.Tool != tool {
			continue
		}
		if entry.Action != "" && entry.Action != action {
			continue
		}
		return entry, true
	}
	return bindingSurfaceInventoryEntry{}, false
}
