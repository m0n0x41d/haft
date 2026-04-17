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
    project_name: String,
) -> Result<DashboardView, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.get_dashboard(&project_name).map_err(|e| e.to_string())
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
    project_path: String,
) -> Result<Vec<TaskState>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.list_tasks(&project_path).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn get_task_output(state: State<'_, DbState>, id: String) -> Result<String, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    let task = db.get_task(&id).map_err(|e| e.to_string())?;
    Ok(task.raw_output)
}

// ─── Projects ───

#[tauri::command]
pub fn list_projects(state: State<'_, DbState>) -> Result<Vec<String>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.list_projects().map_err(|e| e.to_string())
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

#[tauri::command]
pub fn get_config(state: State<'_, DbState>, key: String) -> Result<String, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.get_config(&key).map_err(|e| e.to_string())
}

// ─── Flows ───

#[tauri::command]
pub fn list_flows(
    state: State<'_, DbState>,
    project_path: String,
) -> Result<Vec<FlowView>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    db.list_flows(&project_path).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn list_flow_templates(state: State<'_, DbState>) -> Result<Vec<FlowTemplateView>, String> {
    let db = state.0.lock().map_err(|e| e.to_string())?;
    Ok(db.list_flow_templates())
}
