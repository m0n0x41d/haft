use serde_json::{json, Value};

use crate::rpc::call_rpc;

// ─────────────────────────────────────────────────────────────────────────────
// IPC command forwarders for mutations + queries that shell out to
// `haft desktop-rpc <cmd>`.
//
// Each Tauri command below mirrors the exact field shape the frontend sends
// via `tauriInvoke(cmd, { ...fields })`. The IPC argument list must match
// what the frontend passes; if it doesn't, Tauri rejects the call with
// "invalid args ... missing required key ..." before we ever reach call_rpc.
//
// `projectRoot` is optional everywhere: when omitted, the spawned
// `haft desktop-rpc` subprocess inherits `HAFT_PROJECT_ROOT` from the Tauri
// host's environment (set by the `haft desktop` launcher). Explicit passing
// is reserved for multi-project routing.
//
// The macro `rpc_fwd!` generates the boilerplate. Use it for commands whose
// shape is `{ field1, field2, ... }` → forward as stdin JSON to the CLI.
// ─────────────────────────────────────────────────────────────────────────────

/// Generate a Tauri command that forwards named fields as a JSON payload to
/// `haft desktop-rpc <cmd>`. Accepts an optional `projectRoot` which is used
/// to set `HAFT_PROJECT_ROOT` on the spawned subprocess.
macro_rules! rpc_fwd {
    ($fn_name:ident, $rpc_cmd:expr, { $($field:ident : $ty:ty),* $(,)? }) => {
        #[tauri::command]
        pub fn $fn_name(
            $($field: $ty,)*
            project_root: Option<String>,
        ) -> Result<Value, String> {
            let payload = json!({
                $(stringify!($field): $field,)*
            });
            call_rpc($rpc_cmd, Some(payload), project_root.as_deref())
        }
    };
}

/// Generate a Tauri command that takes no input fields and forwards an empty
/// payload to `haft desktop-rpc <cmd>`. Used for pure queries.
macro_rules! rpc_query {
    ($fn_name:ident, $rpc_cmd:expr) => {
        #[tauri::command]
        pub fn $fn_name(project_root: Option<String>) -> Result<Value, String> {
            call_rpc($rpc_cmd, None, project_root.as_deref())
        }
    };
}

// ── Project management ──

rpc_fwd!(switch_project, "switch-project", { path: String });
rpc_fwd!(add_project, "add-project", { path: String });
rpc_fwd!(init_project, "init-project", { path: String });

// Frontend calls add_project_smart when the path may not yet be a haft
// project — the CLI detects-or-init's. Reuses the add-project RPC for now;
// CLI-side heuristic is still TODO (see desktop_rpc_handlers.go).
rpc_fwd!(add_project_smart, "add-project", { path: String });

// ── Artifact authoring ──

rpc_fwd!(create_problem, "create-problem", { input: Value });
rpc_fwd!(create_decision, "create-decision", { input: Value });
rpc_fwd!(create_portfolio, "create-portfolio", { input: Value });
rpc_fwd!(compare_portfolio, "compare-portfolio", { input: Value });

// Frontend uses "characterize_problem" command name; CLI dispatches via
// "characterize" RPC verb.
rpc_fwd!(characterize_problem, "characterize", { input: Value });

// ── Decision lifecycle ──

rpc_fwd!(
    implement_decision,
    "implement-decision",
    {
        decision_id: String,
        agent: String,
        worktree: bool,
        branch: String,
    }
);
rpc_fwd!(verify_decision, "verify-decision", { decision_id: String, agent: String });
rpc_fwd!(baseline_decision, "baseline", { decision_id: String });
rpc_fwd!(
    measure_decision,
    "measure",
    {
        decision_id: String,
        findings: String,
        verdict: String,
    }
);

// ── Artifact lifecycle ──

rpc_fwd!(waive_decision, "waive", { decision_id: String, reason: String });
rpc_fwd!(deprecate_decision, "deprecate", { decision_id: String, reason: String });
rpc_fwd!(reopen_decision, "reopen", { decision_id: String, reason: String });

// ── Problem candidates ──

// Frontend uses adopt_problem_candidate / dismiss_problem_candidate; CLI
// RPC verbs are adopt-candidate / dismiss-candidate.
rpc_fwd!(adopt_problem_candidate, "adopt-candidate", { id: String });
rpc_fwd!(dismiss_problem_candidate, "dismiss-candidate", { id: String });

// ── Comparison readiness ──

// Frontend sends portfolio_id; CLI expects portfolioId via the assess RPC.
rpc_fwd!(
    assess_comparison_readiness,
    "assess-readiness",
    { portfolio_id: String }
);

// ── Flow management ──

rpc_fwd!(create_flow, "create-flow", { input: Value });
rpc_fwd!(update_flow, "update-flow", { input: Value });
rpc_fwd!(toggle_flow, "toggle-flow", { id: String, enabled: bool });
rpc_fwd!(delete_flow, "delete-flow", { id: String });
rpc_fwd!(run_flow_now, "run-flow-now", { id: String });

// ── Governance & analysis ──

rpc_query!(refresh_governance, "refresh-governance");
rpc_query!(get_governance_overview, "get-governance-overview");
rpc_query!(get_coverage, "get-coverage");
rpc_query!(detect_agents, "detect-agents");

// ── PR pipeline ──

rpc_fwd!(create_pull_request, "create-pull-request", { task_id: String });
