use serde_json::Value;

/// Mutation command: project_root + JSON payload → `haft desktop-rpc <cmd>`.
macro_rules! rpc_mutation {
    ($fn_name:ident, $rpc_cmd:expr) => {
        #[tauri::command]
        pub fn $fn_name(project_root: String, payload: Value) -> Result<Value, String> {
            crate::rpc::call_rpc($rpc_cmd, Some(payload), Some(&project_root))
        }
    };
}

/// Query command: project_root only, no payload.
macro_rules! rpc_query {
    ($fn_name:ident, $rpc_cmd:expr) => {
        #[tauri::command]
        pub fn $fn_name(project_root: String) -> Result<Value, String> {
            crate::rpc::call_rpc($rpc_cmd, None, Some(&project_root))
        }
    };
}

// ── Artifact authoring ──

rpc_mutation!(create_problem, "create-problem");
rpc_mutation!(create_decision, "create-decision");
rpc_mutation!(create_portfolio, "create-portfolio");
rpc_mutation!(characterize, "characterize");
rpc_mutation!(compare_portfolio, "compare-portfolio");

// ── Decision lifecycle ──

rpc_mutation!(implement_decision, "implement-decision");
rpc_mutation!(verify_decision, "verify-decision");
rpc_mutation!(baseline_decision, "baseline");
rpc_mutation!(measure_decision, "measure");

// ── Artifact lifecycle ──

rpc_mutation!(waive_decision, "waive");
rpc_mutation!(deprecate_decision, "deprecate");
rpc_mutation!(reopen_decision, "reopen");

// ── Problem candidates ──

rpc_mutation!(adopt_candidate, "adopt-candidate");
rpc_mutation!(dismiss_candidate, "dismiss-candidate");

// ── Flow management ──

rpc_mutation!(create_flow, "create-flow");
rpc_mutation!(update_flow, "update-flow");
rpc_mutation!(toggle_flow, "toggle-flow");
rpc_mutation!(delete_flow, "delete-flow");
rpc_mutation!(run_flow_now, "run-flow-now");

// ── Project management ──

rpc_mutation!(switch_project, "switch-project");
rpc_mutation!(add_project, "add-project");
rpc_mutation!(init_project, "init-project");

// ── Governance & analysis ──

rpc_query!(refresh_governance, "refresh-governance");
rpc_query!(get_governance_overview, "get-governance-overview");
rpc_query!(get_coverage, "get-coverage");
rpc_mutation!(assess_readiness, "assess-readiness");
rpc_query!(detect_agents, "detect-agents");
rpc_mutation!(create_pull_request, "create-pull-request");
