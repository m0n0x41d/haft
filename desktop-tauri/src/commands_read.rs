use std::sync::Mutex;

use tauri::State;
use tauri_plugin_dialog::DialogExt;

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
pub fn get_decision(state: State<'_, DbState>, id: String) -> Result<DecisionDetailView, String> {
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
pub fn get_portfolio(state: State<'_, DbState>, id: String) -> Result<PortfolioDetailView, String> {
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
    db.set_task_auto_run(&id, auto_run)
        .map_err(|e| e.to_string())
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

    let val: serde_json::Value =
        serde_json::from_str(&content).map_err(|e| format!("parse registry: {e}"))?;
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
        let path = p
            .get("path")
            .and_then(|s| s.as_str())
            .unwrap_or("")
            .to_string();
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
        let id = p
            .get("id")
            .and_then(|s| s.as_str())
            .unwrap_or("")
            .to_string();
        let exists = std::path::Path::new(&path).is_dir();
        let has_haft = std::path::Path::new(&path)
            .join(".haft/project.yaml")
            .is_file();
        let status = match (exists, has_haft) {
            (false, _) => "missing",
            (true, false) => "needs_init",
            (true, true) => "ready",
        }
        .to_string();
        let is_active = !active_path.is_empty() && path == active_path && status == "ready";

        out.push(ProjectInfo {
            path,
            name,
            id,
            status,
            exists,
            has_haft,
            is_active,
            problem_count: 0,
            decision_count: 0,
            stale_count: 0,
        });
    }

    out.sort_by(|left, right| {
        let left_rank = project_status_rank(left);
        let right_rank = project_status_rank(right);

        left_rank
            .cmp(&right_rank)
            .then_with(|| left.name.cmp(&right.name))
            .then_with(|| left.path.cmp(&right.path))
    });

    Ok(out)
}

fn project_status_rank(project: &ProjectInfo) -> i32 {
    if project.is_active {
        return 0;
    }

    match project.status.as_str() {
        "ready" => 1,
        "needs_init" => 2,
        _ => 3,
    }
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
/// render.
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

/// Persist the Settings page's edits to `~/.haft/config.json`. Creates the
/// parent directory if missing. Writes the full config snapshot on every
/// save — we don't patch fields individually, matching the frontend contract
/// (`saveConfig(config: DesktopConfig)`).
#[tauri::command]
pub fn save_config(
    _state: State<'_, DbState>,
    config: DesktopConfig,
) -> Result<DesktopConfig, String> {
    let home = std::env::var("HOME").unwrap_or_else(|_| ".".into());
    let haft_dir = format!("{home}/.haft");
    std::fs::create_dir_all(&haft_dir).map_err(|e| format!("create config dir: {e}"))?;

    let config_path = format!("{haft_dir}/config.json");
    let serialized =
        serde_json::to_string_pretty(&config).map_err(|e| format!("serialize config: {e}"))?;
    std::fs::write(&config_path, serialized).map_err(|e| format!("write config: {e}"))?;

    Ok(config)
}

/// Open a native directory picker. Resolves to the chosen path, or an empty
/// string if the user cancels. Used by the Add-Project modal.
#[tauri::command]
pub async fn open_directory_picker(app: tauri::AppHandle) -> Result<String, String> {
    let (tx, rx) = std::sync::mpsc::channel();
    app.dialog().file().pick_folder(move |folder| {
        let _ = tx.send(folder.map(|f| f.to_string()).unwrap_or_default());
    });
    rx.recv().map_err(|e| format!("dialog channel: {e}"))
}

/// Launch the user's configured IDE at the given path. Falls back through a
/// small list of known launchers until one succeeds (`cursor`, `code`,
/// `idea`, `subl`). Errors when no launcher is on PATH so the UI can report
/// back to the user; a silent `Ok(())` would hide the miss.
#[tauri::command]
pub fn open_path_in_ide(path: String) -> Result<(), String> {
    if path.trim().is_empty() {
        return Err("path is empty".into());
    }
    let candidates = ["cursor", "code", "idea", "subl"];
    let mut last_err = String::from("no IDE launcher found on PATH");
    for cmd in candidates {
        match std::process::Command::new(cmd).arg(&path).spawn() {
            Ok(_) => return Ok(()),
            Err(e) => {
                last_err = format!("{cmd}: {e}");
            }
        }
    }
    Err(last_err)
}

/// Walk common project-holding directories (`~/Projects`, `~/Repos`,
/// `~/Documents`, `~/Developer`) and return any folder that contains a
/// `.haft/project.yaml`. Depth is capped at 3 so deep `node_modules`-style
/// trees don't blow the walk time. Called from the New Project modal and
/// Settings → Project scanner during mount.
#[tauri::command]
pub fn scan_for_projects(_state: State<'_, DbState>) -> Result<Vec<ProjectInfo>, String> {
    let home = std::env::var("HOME").unwrap_or_else(|_| ".".into());
    let candidates = [
        format!("{home}/Projects"),
        format!("{home}/Repos"),
        format!("{home}/Documents"),
        format!("{home}/Developer"),
    ];

    let mut out = Vec::new();
    let mut seen = std::collections::HashSet::<String>::new();
    for root in &candidates {
        if !std::path::Path::new(root).is_dir() {
            continue;
        }
        walk_for_haft_projects(std::path::Path::new(root), 0, 3, &mut out, &mut seen);
    }
    Ok(out)
}

fn walk_for_haft_projects(
    dir: &std::path::Path,
    depth: usize,
    max_depth: usize,
    out: &mut Vec<ProjectInfo>,
    seen: &mut std::collections::HashSet<String>,
) {
    if depth > max_depth {
        return;
    }
    let project_yaml = dir.join(".haft/project.yaml");
    if project_yaml.exists() {
        let path = dir.to_string_lossy().to_string();
        if !seen.contains(&path) {
            seen.insert(path.clone());
            let name = dir
                .file_name()
                .and_then(|s| s.to_str())
                .unwrap_or("")
                .to_string();
            out.push(ProjectInfo {
                path,
                name,
                id: String::new(),
                status: "ready".into(),
                exists: true,
                has_haft: true,
                is_active: false,
                problem_count: 0,
                decision_count: 0,
                stale_count: 0,
            });
        }
        return;
    }
    let entries = match std::fs::read_dir(dir) {
        Ok(e) => e,
        Err(_) => return,
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if !path.is_dir() {
            continue;
        }
        // Skip common churn dirs that can't contain a haft project.
        if let Some(name) = path.file_name().and_then(|s| s.to_str()) {
            if matches!(
                name,
                "node_modules" | ".git" | "target" | "dist" | "build" | ".next" | "vendor"
            ) {
                continue;
            }
            if name.starts_with('.') && name != ".haft" {
                continue;
            }
        }
        walk_for_haft_projects(&path, depth + 1, max_depth, out, seen);
    }
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
