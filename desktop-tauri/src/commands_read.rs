use std::sync::Mutex;

use tauri::State;

use crate::db::HaftDb;
use crate::models::*;

/// Managed state: a Mutex-wrapped HaftDb for thread-safe Tauri command access.
pub struct DbState(pub Mutex<HaftDb>);

// ─── Dashboard ───

#[tauri::command]
pub fn get_dashboard(
    state: State<'_, DbState>,
    project_name: Option<String>,
) -> Result<DashboardView, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    let name = project_name.unwrap_or_default();
    db.get_dashboard(&name).map_err(|e| e.to_string())
}

// ─── Problems ───

#[tauri::command]
pub fn list_problems(state: State<'_, DbState>) -> Result<Vec<ProblemView>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.list_problems().map_err(|e| e.to_string())
}

#[tauri::command]
pub fn get_problem(state: State<'_, DbState>, id: String) -> Result<ProblemDetailView, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.get_problem(&id).map_err(|e| e.to_string())
}

// ─── Decisions ───

#[tauri::command]
pub fn list_decisions(state: State<'_, DbState>) -> Result<Vec<DecisionView>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.list_decisions().map_err(|e| e.to_string())
}

#[tauri::command]
pub fn get_decision(
    state: State<'_, DbState>,
    id: String,
) -> Result<DecisionDetailView, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.get_decision(&id).map_err(|e| e.to_string())
}

// ─── Portfolios ───

#[tauri::command]
pub fn list_portfolios(state: State<'_, DbState>) -> Result<Vec<PortfolioSummaryView>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.list_portfolios().map_err(|e| e.to_string())
}

#[tauri::command]
pub fn get_portfolio(
    state: State<'_, DbState>,
    id: String,
) -> Result<PortfolioDetailView, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.get_portfolio(&id).map_err(|e| e.to_string())
}

// ─── Tasks ───

#[tauri::command]
pub fn list_tasks(
    state: State<'_, DbState>,
    project_path: Option<String>,
) -> Result<Vec<TaskState>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    // If no project_path provided, list all tasks across projects
    match project_path {
        Some(path) if !path.is_empty() => db.list_tasks(&path).map_err(|e| e.to_string()),
        _ => db.list_all_tasks().map_err(|e| e.to_string()),
    }
}

/// Frontend-side alias for `list_tasks` with no project_path — lists every
/// task across all projects. Registered separately because `api.ts` invokes
/// `list_all_tasks` by name (see Jobs page); unifying that into one command
/// on both sides is a follow-up.
#[tauri::command]
pub fn list_all_tasks(state: State<'_, DbState>) -> Result<Vec<TaskState>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.list_all_tasks().map_err(|e| e.to_string())
}

/// Soft-delete a task row. Frontend's Jobs page calls this when the user
/// removes a finished task from the list.
#[tauri::command]
pub fn archive_task(state: State<'_, DbState>, id: String) -> Result<(), String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.archive_task(&id).map_err(|e| e.to_string())
}

/// Flip the `auto_run` flag on a task — used by the UI to toggle whether a
/// finished task should auto-resume on restart.
#[tauri::command]
pub fn set_task_auto_run(
    state: State<'_, DbState>,
    id: String,
    auto_run: bool,
) -> Result<(), String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.set_task_auto_run(&id, auto_run).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn get_task_output(state: State<'_, DbState>, id: String) -> Result<String, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    let task = db.get_task(&id).map_err(|e| e.to_string())?;
    Ok(task.raw_output)
}

// ─── Projects ───

/// Returns `Vec<ProjectInfo>` rather than `Vec<String>` to match the
/// TypeScript contract in `desktop/frontend/src/lib/api.ts`. Source of truth
/// is the registry at `~/.haft/desktop-projects.json`; the SQLite
/// `desktop_tasks` table only knows about projects that already ran a task,
/// so it misses freshly-added projects. We join:
///
/// - registry (path, name, id, is_active)
/// - counts fall back to 0 here; real counts would require opening each
///   project's own DB, which is deferred until the sidebar actually needs
///   live numbers (see `get_dashboard` — it is per-project).
#[tauri::command]
pub fn list_projects(_state: State<'_, DbState>) -> Result<Vec<ProjectInfo>, String> {
    let home = std::env::var("HOME").unwrap_or_else(|_| ".".into());
    let registry_path = format!("{home}/.haft/desktop-projects.json");
    let content = match std::fs::read_to_string(&registry_path) {
        Ok(s) => s,
        Err(_) => return Ok(Vec::new()),
    };

    let val: serde_json::Value = serde_json::from_str(&content)
        .map_err(|e| format!("parse registry: {e}"))?;
    let active_path = val
        .get("active_path")
        .and_then(|v| v.as_str())
        .unwrap_or("")
        .to_string();
    let projects = val
        .get("projects")
        .and_then(|v| v.as_array())
        .cloned()
        .unwrap_or_default();

    let mut out = Vec::with_capacity(projects.len());
    for p in projects {
        let path = p.get("path").and_then(|s| s.as_str()).unwrap_or("").to_string();
        if path.is_empty() {
            continue;
        }
        let name = p
            .get("name")
            .and_then(|s| s.as_str())
            .unwrap_or_else(|| {
                std::path::Path::new(&path)
                    .file_name()
                    .and_then(|s| s.to_str())
                    .unwrap_or("")
            })
            .to_string();
        let id = p.get("id").and_then(|s| s.as_str()).unwrap_or("").to_string();
        let is_active = !active_path.is_empty() && path == active_path;
        out.push(ProjectInfo {
            path,
            name,
            id,
            is_active,
            problem_count: 0,
            decision_count: 0,
            stale_count: 0,
        });
    }
    Ok(out)
}

// ─── Search ───

#[tauri::command]
pub fn search_artifacts(
    state: State<'_, DbState>,
    query: String,
) -> Result<Vec<ArtifactView>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.search_artifacts(&query).map_err(|e| e.to_string())
}

// ─── Config ───

/// Returns a typed `DesktopConfig` rather than a JSON string to match the
/// TypeScript contract. Reads `~/.haft/config.json`; falls back to defaults
/// when the file is absent or unparseable so the Settings page can still
/// render (user edits will persist via `save_config` once wired).
#[tauri::command]
pub fn get_config(_state: State<'_, DbState>) -> Result<DesktopConfig, String> {
    let home = std::env::var("HOME").unwrap_or_else(|_| ".".into());
    let config_path = format!("{home}/.haft/config.json");
    let content = match std::fs::read_to_string(&config_path) {
        Ok(s) => s,
        Err(_) => return Ok(DesktopConfig::default()),
    };
    serde_json::from_str::<DesktopConfig>(&content).or_else(|_| Ok(DesktopConfig::default()))
}

// ─── Flows ───

#[tauri::command]
pub fn list_flows(
    state: State<'_, DbState>,
    project_path: Option<String>,
) -> Result<Vec<FlowView>, String> {
    let project_path = project_path.unwrap_or_default();
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.list_flows(&project_path).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn list_flow_templates(state: State<'_, DbState>) -> Result<Vec<FlowTemplateView>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    Ok(db.list_flow_templates())
}
