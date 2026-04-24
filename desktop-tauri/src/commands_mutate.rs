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

/// Generate a Tauri command whose IPC shape is `{ input: <opaque object> }`
/// — the frontend convention for artifact-authoring commands (create_problem,
/// create_decision, etc) — and forwards the *inner* input object as the RPC
/// stdin payload. The Go-side handlers expect flat records
/// (`artifact.ProblemFrameInput`, `artifact.DecideInput`, …), not a nested
/// `{"input": …}` wrapper, so `rpc_fwd!` with its auto-wrapping behavior
/// cannot be used here. Matches what the frontend sends via
/// `tauriInvoke("create_problem", { input })`.
macro_rules! rpc_fwd_input {
    ($fn_name:ident, $rpc_cmd:expr) => {
        #[tauri::command]
        pub fn $fn_name(
            input: Value,
            project_root: Option<String>,
        ) -> Result<Value, String> {
            call_rpc($rpc_cmd, Some(input), project_root.as_deref())
        }
    };
}

/// Like `rpc_fwd!`, but each field specifies an explicit JSON key used in the
/// payload sent to the Go handler. Needed whenever the frontend-side field
/// name diverges from the Go-side JSON tag — e.g. historical frontend code
/// sends `decision_id` but `internal/cli/desktop_rpc_handlers.go` reads
/// `decision_ref` (and `artifact_ref` for waive/deprecate). Every field must
/// declare its rpc key, even if it happens to match the Rust identifier —
/// explicit is easier to audit than mixed.
macro_rules! rpc_fwd_renamed {
    ($fn_name:ident, $rpc_cmd:expr, { $($field:ident as $key:literal : $ty:ty),* $(,)? }) => {
        #[tauri::command]
        pub fn $fn_name(
            $($field: $ty,)*
            project_root: Option<String>,
        ) -> Result<Value, String> {
            let payload = json!({
                $($key: $field,)*
            });
            call_rpc($rpc_cmd, Some(payload), project_root.as_deref())
        }
    };
}

// ── Project management ──

rpc_fwd!(switch_project, "switch-project", { path: String });
rpc_fwd!(add_project, "add-project", { path: String });
rpc_fwd!(init_project, "init-project", { path: String });

// Frontend calls add_project_smart when the path may not yet be a haft
// project — the CLI detects-or-init's.
rpc_fwd!(add_project_smart, "add-project-smart", { path: String });

// ── Artifact authoring ──
//
// These commands take `{ input: ... }` on the IPC boundary but forward the
// inner value as the RPC stdin payload — Go handlers expect flat records.

rpc_fwd_input!(create_problem, "create-problem");
rpc_fwd_input!(create_decision, "create-decision");
rpc_fwd_input!(create_portfolio, "create-portfolio");
rpc_fwd_input!(compare_portfolio, "compare-portfolio");

// Frontend uses "characterize_problem" command name; CLI dispatches via
// "characterize" RPC verb.
rpc_fwd_input!(characterize_problem, "characterize");

// ── Decision lifecycle ──
//
// Frontend sends `decision_id` (historical name), but Go handlers read
// `decision_ref` — translate via `rpc_fwd_renamed!`.

rpc_fwd_renamed!(
    implement_decision,
    "implement-decision",
    {
        decision_id as "decision_ref": String,
        agent as "agent": String,
        worktree as "worktree": bool,
        branch as "branch": String,
    }
);
rpc_fwd_renamed!(
    verify_decision,
    "verify-decision",
    {
        decision_id as "decision_ref": String,
        agent as "agent": String,
    }
);
rpc_fwd_renamed!(
    baseline_decision,
    "baseline",
    { decision_id as "decision_ref": String }
);
rpc_fwd_renamed!(
    measure_decision,
    "measure",
    {
        decision_id as "decision_ref": String,
        findings as "findings": String,
        verdict as "verdict": String,
    }
);

// ── Artifact lifecycle ──
//
// waive/deprecate operate on any artifact (not just decisions) and the Go
// handler reads `artifact_ref`. reopen is decision-specific and reads
// `decision_ref`. Frontend uniformly sends `decision_id` — translate.

rpc_fwd_renamed!(
    waive_decision,
    "waive",
    {
        decision_id as "artifact_ref": String,
        reason as "reason": String,
    }
);
rpc_fwd_renamed!(
    deprecate_decision,
    "deprecate",
    {
        decision_id as "artifact_ref": String,
        reason as "reason": String,
    }
);
rpc_fwd_renamed!(
    reopen_decision,
    "reopen",
    {
        decision_id as "decision_ref": String,
        reason as "reason": String,
    }
);

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

rpc_fwd_input!(create_flow, "create-flow");
rpc_fwd_input!(update_flow, "update-flow");
rpc_fwd!(toggle_flow, "toggle-flow", { id: String, enabled: bool });
rpc_fwd!(delete_flow, "delete-flow", { id: String });
// run_flow_now lives in agents.rs because it needs the shared PTY spawn
// helper + AgentManagerState + ShellEnvState to actually launch the task.

// ── Governance & analysis ──

rpc_query!(refresh_governance, "refresh-governance");
rpc_query!(get_governance_overview, "get-governance-overview");
rpc_query!(get_coverage, "get-coverage");
rpc_query!(detect_agents, "detect-agents");

// ── PR pipeline ──
//
// Go's `create-pull-request` handler reads `decision_ref` + `branch` (see
// internal/cli/desktop_rpc_handlers.go handleCreatePullRequest). The
// frontend derives both values from the selected task before calling.

rpc_fwd!(
    create_pull_request,
    "create-pull-request",
    {
        decision_ref: String,
        branch: String,
    }
);
