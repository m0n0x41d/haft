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
    let db_path = resolve_db_path();

    let db = HaftDb::open(&db_path).expect("failed to open haft.db");
    let shell_env = shell_env::resolve_user_shell_env();

    // Apply resolved shell env to current process so all child processes
    // (RPC subprocess, agents) inherit full PATH even when launched from Spotlight.
    for (key, value) in &shell_env {
        // SAFETY: single-threaded at this point (before Tauri runtime starts).
        unsafe { std::env::set_var(key, value) };
    }

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
            commands_read::get_task_output,
            commands_read::list_projects,
            commands_read::search_artifacts,
            commands_read::get_config,
            commands_read::list_flows,
            commands_read::list_flow_templates,
            // ── Mutate (haft desktop-rpc subprocess) ──
            commands_mutate::create_problem,
            commands_mutate::create_decision,
            commands_mutate::create_portfolio,
            commands_mutate::characterize,
            commands_mutate::compare_portfolio,
            commands_mutate::implement_decision,
            commands_mutate::verify_decision,
            commands_mutate::baseline_decision,
            commands_mutate::measure_decision,
            commands_mutate::waive_decision,
            commands_mutate::deprecate_decision,
            commands_mutate::reopen_decision,
            commands_mutate::adopt_candidate,
            commands_mutate::dismiss_candidate,
            commands_mutate::create_flow,
            commands_mutate::update_flow,
            commands_mutate::toggle_flow,
            commands_mutate::delete_flow,
            commands_mutate::run_flow_now,
            commands_mutate::switch_project,
            commands_mutate::add_project,
            commands_mutate::init_project,
            commands_mutate::refresh_governance,
            commands_mutate::get_governance_overview,
            commands_mutate::get_coverage,
            commands_mutate::assess_readiness,
            commands_mutate::detect_agents,
            commands_mutate::create_pull_request,
            // ── Agents (PTY) ──
            agents::spawn_agent,
            agents::cancel_agent,
            agents::list_running_agents,
            agents::get_agent_output,
            // ── Terminal (interactive PTY) ──
            terminal::create_terminal_session,
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

/// Resolve haft.db path: $HAFT_DB > ~/.haft/haft.db
pub(crate) fn resolve_db_path() -> String {
    if let Ok(p) = std::env::var("HAFT_DB") {
        return p;
    }
    let home = std::env::var("HOME").unwrap_or_else(|_| ".".into());
    format!("{home}/.haft/haft.db")
}
