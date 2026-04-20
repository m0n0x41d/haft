pub mod agents;
pub mod commands_mutate;
pub mod commands_read;
pub mod db;
pub mod models;
pub mod rpc;
pub mod shell_env;
pub mod terminal;
pub mod watcher;

use agents::AgentManagerState;
use commands_read::DbState;
use db::HaftDb;
use shell_env::ShellEnvState;
use std::sync::Mutex;
use terminal::TerminalManagerState;
use watcher::WatcherState;

pub fn run() {
    let shell_env = shell_env::resolve_user_shell_env();

    // Apply resolved shell env to current process so all child processes
    // (RPC subprocess, agents) inherit full PATH even when launched from Spotlight.
    for (key, value) in &shell_env {
        // SAFETY: single-threaded at this point (before Tauri runtime starts).
        unsafe { std::env::set_var(key, value) };
    }

    // Resolve initial project and open DB. App starts even if no project found.
    let db = match resolve_project_db() {
        Some(db) => db,
        None => {
            eprintln!("haft desktop: no project found, starting in empty state");
            // Open an in-memory DB so the app can start without a project.
            // User will add a project from the UI.
            HaftDb::open(":memory:").expect("failed to open in-memory db")
        }
    };

    tauri::Builder::default()
        .manage(DbState(Mutex::new(db)))
        .manage(WatcherState(Mutex::new(None)))
        .manage(ShellEnvState(Mutex::new(shell_env)))
        .manage(AgentManagerState(Mutex::new(agents::AgentManager::new())))
        .manage(TerminalManagerState(Mutex::new(terminal::TerminalManager::new())))
        .invoke_handler(tauri::generate_handler![
            // ── Read (direct SQLite) ──
            commands_read::get_dashboard,
            commands_read::list_problems,
            commands_read::get_problem,
            commands_read::list_decisions,
            commands_read::get_decision,
            commands_read::list_portfolios,
            commands_read::get_portfolio,
            commands_read::list_tasks,
            commands_read::list_all_tasks,
            commands_read::get_task_output,
            commands_read::archive_task,
            commands_read::set_task_auto_run,
            commands_read::list_projects,
            commands_read::search_artifacts,
            commands_read::get_config,
            commands_read::save_config,
            commands_read::list_flows,
            commands_read::list_flow_templates,
            // ── Mutate (haft desktop-rpc subprocess) ──
            commands_mutate::create_problem,
            commands_mutate::create_decision,
            commands_mutate::create_portfolio,
            commands_mutate::characterize_problem,
            commands_mutate::compare_portfolio,
            commands_mutate::implement_decision,
            commands_mutate::verify_decision,
            commands_mutate::baseline_decision,
            commands_mutate::measure_decision,
            commands_mutate::waive_decision,
            commands_mutate::deprecate_decision,
            commands_mutate::reopen_decision,
            commands_mutate::adopt_problem_candidate,
            commands_mutate::dismiss_problem_candidate,
            commands_mutate::create_flow,
            commands_mutate::update_flow,
            commands_mutate::toggle_flow,
            commands_mutate::delete_flow,
            commands_mutate::run_flow_now,
            commands_mutate::switch_project,
            commands_mutate::add_project,
            commands_mutate::add_project_smart,
            commands_mutate::init_project,
            commands_mutate::refresh_governance,
            commands_mutate::get_governance_overview,
            commands_mutate::get_coverage,
            commands_mutate::assess_comparison_readiness,
            commands_mutate::detect_agents,
            commands_mutate::create_pull_request,
            // ── Agents (PTY) ──
            agents::spawn_agent,
            agents::spawn_task,
            agents::cancel_agent,
            agents::cancel_task,
            agents::write_task_input,
            agents::handoff_task,
            agents::list_running_agents,
            agents::get_agent_output,
            // ── Terminal (interactive PTY) ──
            terminal::create_terminal_session,
            terminal::list_terminal_sessions,
            terminal::write_terminal_input,
            terminal::resize_terminal_session,
            terminal::close_terminal_session,
            // ── Watcher ──
            watcher::start_watcher,
            watcher::stop_watcher,
        ])
        .run(tauri::generate_context!())
        .expect("failed to run haft desktop");
}

/// Resolve the project DB by finding .haft/project.yaml and computing the DB path.
/// Priority: HAFT_PROJECT_ROOT env > cwd walk > first project in registry.
fn resolve_project_db() -> Option<HaftDb> {
    let db_path = resolve_db_path()?;
    HaftDb::open(&db_path).ok()
}

/// Resolve haft.db path from the project root.
/// Returns ~/.haft/projects/{id}/haft.db
pub(crate) fn resolve_db_path() -> Option<String> {
    // Try explicit env var
    if let Ok(p) = std::env::var("HAFT_DB") {
        return Some(p);
    }

    let project_root = resolve_project_root()?;
    let project_yaml = std::path::Path::new(&project_root).join(".haft/project.yaml");
    let content = std::fs::read_to_string(&project_yaml).ok()?;

    // Parse project ID from yaml (simple: look for "id: qnt_...")
    let id = content.lines()
        .find(|line| line.starts_with("id:"))
        .and_then(|line| line.strip_prefix("id:"))
        .map(|s| s.trim().to_string())?;

    let home = std::env::var("HOME").unwrap_or_else(|_| ".".into());
    let db_path = format!("{home}/.haft/projects/{id}/haft.db");

    if std::path::Path::new(&db_path).exists() {
        Some(db_path)
    } else {
        eprintln!("haft desktop: DB not found at {db_path}");
        None
    }
}

/// Find project root: HAFT_PROJECT_ROOT > cwd walk > registry fallback.
fn resolve_project_root() -> Option<String> {
    // 1. Env var (set by `haft desktop` launcher)
    if let Ok(root) = std::env::var("HAFT_PROJECT_ROOT") {
        let root = root.trim().to_string();
        if !root.is_empty() && std::path::Path::new(&root).join(".haft/project.yaml").exists() {
            return Some(root);
        }
    }

    // 2. Walk up from cwd
    if let Ok(cwd) = std::env::current_dir() {
        let mut dir = cwd.as_path();
        let home = dirs::home_dir();
        loop {
            // Skip ~/.haft/ (global config, not a project)
            if let Some(ref h) = home {
                if dir == h.as_path() {
                    if let Some(parent) = dir.parent() {
                        dir = parent;
                        continue;
                    }
                    break;
                }
            }
            let candidate = dir.join(".haft/project.yaml");
            if candidate.exists() {
                return Some(dir.to_string_lossy().to_string());
            }
            match dir.parent() {
                Some(parent) if parent != dir => dir = parent,
                _ => break,
            }
        }
    }

    // 3. Registry fallback — read ~/.haft/desktop-projects.json
    let home = std::env::var("HOME").unwrap_or_else(|_| ".".into());
    let registry_path = format!("{home}/.haft/desktop-projects.json");
    if let Ok(content) = std::fs::read_to_string(&registry_path) {
        // Simple JSON parse: find first "path": "..." value
        if let Ok(val) = serde_json::from_str::<serde_json::Value>(&content) {
            if let Some(projects) = val.get("projects").and_then(|p| p.as_array()) {
                for project in projects {
                    if let Some(path) = project.get("path").and_then(|p| p.as_str()) {
                        if std::path::Path::new(path).join(".haft/project.yaml").exists() {
                            return Some(path.to_string());
                        }
                    }
                }
            }
        }
    }

    None
}
