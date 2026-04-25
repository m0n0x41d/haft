use std::collections::HashMap;
use std::io::{Read, Write};
use std::path::Path;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use portable_pty::{CommandBuilder, NativePtySystem, PtySize, PtySystem};
use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter, State};

use crate::models::ChatBlock;
use crate::project_readiness::{READY, inspect_project_readiness, project_not_ready_message};
use crate::rpc;
use crate::shell_env::ShellEnvState;

// ─── Constants ───

const PTY_ROWS: u16 = 32;
const PTY_COLS: u16 = 120;
const OUTPUT_MAX_LINES: usize = 500;
const OUTPUT_MAX_CHARS: usize = 64 * 1024;
const FLUSH_INTERVAL: Duration = Duration::from_millis(350);
const CANCEL_GRACE: Duration = Duration::from_secs(2);
const CONTINUATION_TRANSCRIPT_MAX_CHARS: usize = 12_000;
const CONTROL_PROMPT_PREFIX: &str = "Continue the existing desktop task.";
const CONTROL_PROMPT_FOLLOW_UP: &str = "Operator follow-up:";
const CONTROL_PROMPT_SUFFIX: &str =
    "Continue from the prior context. Do not repeat completed setup unless it is necessary.";

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ChatBlockVisibility {
    Visible,
    AuditOnly,
}

// ─── Agent kinds ───

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AgentKind {
    Claude,
    Codex,
}

impl AgentKind {
    fn from_str(s: &str) -> Option<Self> {
        match s.trim().to_lowercase().as_str() {
            "claude" => Some(Self::Claude),
            "codex" => Some(Self::Codex),
            _ => None,
        }
    }

    fn as_str(&self) -> &'static str {
        match self {
            Self::Claude => "claude",
            Self::Codex => "codex",
        }
    }
}

// ─── Event payloads ───

#[derive(Clone, Serialize)]
pub struct TaskOutputEvent {
    pub id: String,
    pub chunk: String,
    pub output: String,
}

#[derive(Clone, Serialize)]
pub struct TaskStatusEvent {
    pub id: String,
    pub status: String,
    pub error_message: String,
}

// ─── Task state (in-memory) ───

struct RunningTask {
    id: String,
    agent: AgentKind,
    project_name: String,
    project_path: String,
    title: String,
    prompt: String,
    status: Mutex<String>,
    error_message: Mutex<String>,
    output: Mutex<OutputBuffer>,
    chat_blocks: Mutex<Vec<ChatBlock>>,
    block_seq: AtomicU64,
    started_at: String,
    cancelled: AtomicBool,
    child: Mutex<Option<Box<dyn portable_pty::Child + Send>>>,
    /// PTY master writer, retained so `write_task_input` can feed keystrokes
    /// into an interactive agent. `None` when the writer was never taken
    /// (should not happen for normal spawns) or the task has been finalized.
    writer: Mutex<Option<Box<dyn Write + Send>>>,
}

struct OutputBuffer {
    lines: Vec<String>,
    total_chars: usize,
}

struct TaskContinuationContext {
    title: String,
    prompt: String,
    agent: String,
    project_name: String,
    project_path: String,
    transcript: String,
    chat_blocks: Vec<ChatBlock>,
}

impl OutputBuffer {
    fn new() -> Self {
        Self {
            lines: Vec::new(),
            total_chars: 0,
        }
    }

    fn append(&mut self, text: &str) {
        for line in text.split('\n') {
            self.lines.push(line.to_string());
            self.total_chars += line.len() + 1;
        }

        while self.lines.len() > OUTPUT_MAX_LINES || self.total_chars > OUTPUT_MAX_CHARS {
            if let Some(removed) = self.lines.first() {
                self.total_chars = self.total_chars.saturating_sub(removed.len() + 1);
                self.lines.remove(0);
            } else {
                break;
            }
        }
    }

    fn snapshot(&self) -> String {
        self.lines.join("\n")
    }
}

// ─── Managed state ───

pub struct AgentManagerState(pub Mutex<AgentManager>);

pub struct AgentManager {
    tasks: HashMap<String, Arc<RunningTask>>,
    seq: u64,
}

impl AgentManager {
    pub fn new() -> Self {
        Self {
            tasks: HashMap::new(),
            seq: 0,
        }
    }

    fn next_id(&mut self) -> String {
        self.seq += 1;
        let ts = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs();
        format!("task-{ts}-{}", self.seq)
    }
}

// ─── Tauri commands ───

#[derive(Deserialize)]
pub struct SpawnAgentRequest {
    pub agent: String,
    pub prompt: String,
    pub project_name: String,
    pub project_path: String,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub initial_chat_blocks: Vec<ChatBlock>,
}

#[tauri::command]
pub fn spawn_agent(
    app: AppHandle,
    manager: State<'_, AgentManagerState>,
    env_state: State<'_, ShellEnvState>,
    request: SpawnAgentRequest,
) -> Result<serde_json::Value, String> {
    spawn_pty_task(&app, &manager, &env_state, request)
}

/// Frontend-side command — accepts the flat shape `{ agent, prompt, worktree,
/// branch }` that `api.ts` sends and resolves `project_name` / `project_path`
/// from the active-project registry. `worktree` / `branch` are accepted for
/// forward compatibility but not yet honored (tracked as follow-up —
/// spawning into a worktree requires cloning the project root on the fly).
#[tauri::command]
pub fn spawn_task(
    app: AppHandle,
    manager: State<'_, AgentManagerState>,
    env_state: State<'_, ShellEnvState>,
    agent: String,
    prompt: String,
    #[allow(unused_variables)] worktree: bool,
    #[allow(unused_variables)] branch: String,
) -> Result<serde_json::Value, String> {
    let (project_path, project_name) = resolve_active_project_context()?;
    spawn_pty_task(
        &app,
        &manager,
        &env_state,
        SpawnAgentRequest {
            agent,
            prompt,
            project_name,
            project_path,
            title: String::new(),
            initial_chat_blocks: Vec::new(),
        },
    )
}

/// Resolve `(project_path, project_name)` for the active desktop project.
/// A registry entry is only runnable when the path still exists and contains
/// the minimum project specification set; stale or under-onboarded carriers
/// must fail before PTY spawn.
fn resolve_active_project_context() -> Result<(String, String), String> {
    let home = std::env::var("HOME").map_err(|e| format!("resolve HOME: {e}"))?;
    let registry_path = format!("{home}/.haft/desktop-projects.json");
    let content = std::fs::read_to_string(&registry_path)
        .map_err(|e| format!("read project registry: {e}"))?;
    let val: serde_json::Value =
        serde_json::from_str(&content).map_err(|e| format!("parse project registry: {e}"))?;
    let active = val
        .get("active_path")
        .and_then(|v| v.as_str())
        .unwrap_or("")
        .to_string();
    if active.is_empty() {
        return Err("no active ready project; add or switch to a project first".into());
    }
    let readiness = inspect_project_readiness(&active);
    if readiness.status != READY {
        return Err(project_not_ready_message(&active, &readiness));
    }
    let name = val
        .get("projects")
        .and_then(|v| v.as_array())
        .and_then(|arr| {
            arr.iter()
                .find(|p| p.get("path").and_then(|s| s.as_str()) == Some(active.as_str()))
        })
        .and_then(|p| p.get("name").and_then(|s| s.as_str()))
        .unwrap_or_else(|| {
            std::path::Path::new(&active)
                .file_name()
                .and_then(|s| s.to_str())
                .unwrap_or("")
        })
        .to_string();
    Ok((active, name))
}

fn spawn_pty_task(
    app: &AppHandle,
    manager: &State<'_, AgentManagerState>,
    env_state: &State<'_, ShellEnvState>,
    request: SpawnAgentRequest,
) -> Result<serde_json::Value, String> {
    let kind = AgentKind::from_str(&request.agent)
        .ok_or_else(|| format!("unsupported agent: {}", request.agent))?;
    validate_project_root(&request.project_path)?;

    let args = build_agent_args(kind, &request.prompt, &request.project_path);
    if args.is_empty() {
        return Err(format!("cannot build args for agent: {}", request.agent));
    }

    let shell_env = env_state.0.lock().map_err(|e| e.to_string())?.clone();

    let mut mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let task_id = mgr.next_id();
    let title = if request.title.trim().is_empty() {
        truncate(&request.prompt, 60)
    } else {
        request.title.trim().to_string()
    };
    let started_at = now_rfc3339();

    let task = Arc::new(RunningTask {
        id: task_id.clone(),
        agent: kind,
        project_name: request.project_name.clone(),
        project_path: request.project_path.clone(),
        title: title.clone(),
        prompt: request.prompt.clone(),
        status: Mutex::new("running".into()),
        error_message: Mutex::new(String::new()),
        output: Mutex::new(OutputBuffer::new()),
        chat_blocks: Mutex::new(request.initial_chat_blocks.clone()),
        block_seq: AtomicU64::new(0),
        started_at: started_at.clone(),
        cancelled: AtomicBool::new(false),
        child: Mutex::new(None),
        writer: Mutex::new(None),
    });

    // Spawn PTY.
    let pty_system = NativePtySystem::default();
    let pair = pty_system
        .openpty(PtySize {
            rows: PTY_ROWS,
            cols: PTY_COLS,
            pixel_width: 0,
            pixel_height: 0,
        })
        .map_err(|e| format!("open pty: {e}"))?;

    let mut cmd = CommandBuilder::new(&args[0]);
    cmd.args(&args[1..]);
    cmd.cwd(&request.project_path);

    let env_map = crate::shell_env::build_agent_env(
        &shell_env,
        &[
            ("TERM", "xterm-256color"),
            ("HAFT_PROJECT_ROOT", &request.project_path),
            ("HAFT_TASK_ID", &task_id),
        ],
    );
    for (k, v) in &env_map {
        cmd.env(k, v);
    }

    // Retain a writer handle so `write_task_input` can feed keystrokes to
    // interactive agents. If `take_writer` fails, the task still runs —
    // input just silently drops.
    if let Ok(writer) = pair.master.take_writer() {
        let mut guard = task.writer.lock().map_err(|e| e.to_string())?;
        *guard = Some(writer);
    }

    let child = pair
        .slave
        .spawn_command(cmd)
        .map_err(|e| format!("spawn {}: {e}", kind.as_str()))?;

    // Store child handle for cancellation.
    {
        let mut guard = task.child.lock().map_err(|e| e.to_string())?;
        *guard = Some(child);
    }

    mgr.tasks.insert(task_id.clone(), Arc::clone(&task));
    drop(mgr);

    // Persist initial state via RPC.
    persist_task_state(&task, None);

    // Start PTY reader thread.
    let reader = pair
        .master
        .try_clone_reader()
        .map_err(|e| format!("clone pty reader: {e}"))?;
    let app_reader = app.clone();
    let task_reader = Arc::clone(&task);
    thread::spawn(move || pty_reader_loop(app_reader, task_reader, reader));

    // Start flush + wait thread.
    let app_wait = app.clone();
    let task_wait = Arc::clone(&task);
    thread::spawn(move || wait_and_finalize(app_wait, task_wait));

    // Emit initial status.
    let _ = app.emit(
        "task.status",
        TaskStatusEvent {
            id: task_id.clone(),
            status: "running".into(),
            error_message: String::new(),
        },
    );

    Ok(running_task_json(&task, Some("running")))
}

fn validate_project_root(project_path: &str) -> Result<(), String> {
    if project_path.trim().is_empty() {
        return Err("project path is required before starting an agent".into());
    }

    if !Path::new(project_path).is_dir() {
        return Err(format!("project path does not exist: {project_path}"));
    }

    let readiness = inspect_project_readiness(project_path);
    if readiness.status != READY {
        return Err(project_not_ready_message(project_path, &readiness));
    }

    Ok(())
}

fn running_task_json(task: &RunningTask, status_override: Option<&str>) -> serde_json::Value {
    let status = status_override
        .map(String::from)
        .or_else(|| task.status.lock().ok().map(|s| s.clone()))
        .unwrap_or_else(|| "running".into());

    let error_message = task
        .error_message
        .lock()
        .map(|e| e.clone())
        .unwrap_or_default();

    let output = task.output.lock().map(|b| b.snapshot()).unwrap_or_default();

    let chat_blocks = task
        .chat_blocks
        .lock()
        .map(|b| b.clone())
        .unwrap_or_default();

    let completed_at = match status.as_str() {
        "completed" | "failed" | "cancelled" => now_rfc3339(),
        _ => String::new(),
    };

    serde_json::json!({
        "id": task.id,
        "title": task.title,
        "agent": task.agent.as_str(),
        "project": task.project_name,
        "project_path": task.project_path,
        "status": status,
        "prompt": task.prompt,
        "branch": "",
        "worktree": false,
        "worktree_path": "",
        "reused_worktree": false,
        "started_at": task.started_at,
        "completed_at": completed_at,
        "error_message": error_message,
        "output": output,
        "chat_blocks": chat_blocks,
        "raw_output": output,
        "auto_run": false,
    })
}

#[tauri::command]
pub fn cancel_agent(
    app: AppHandle,
    manager: State<'_, AgentManagerState>,
    task_id: String,
) -> Result<(), String> {
    cancel_running_task(&app, &manager, &task_id)
}

/// Frontend-side alias — sends `{ id }` to match the Tasks page convention.
/// Delegates to the same cancellation path used by `cancel_agent`.
#[tauri::command]
pub fn cancel_task(
    app: AppHandle,
    manager: State<'_, AgentManagerState>,
    id: String,
) -> Result<(), String> {
    cancel_running_task(&app, &manager, &id)
}

/// Feed `data` into a running task's PTY — the same channel the user would
/// see on their own terminal. Used for interactive agents that prompt for
/// input mid-run.
#[tauri::command]
pub fn write_task_input(
    manager: State<'_, AgentManagerState>,
    id: String,
    data: String,
) -> Result<(), String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let task = mgr
        .tasks
        .get(&id)
        .cloned()
        .ok_or_else(|| format!("task not found: {id}"))?;
    drop(mgr);

    if task.cancelled.load(Ordering::SeqCst) {
        return Err(format!("task already cancelled: {id}"));
    }

    let status = task.status.lock().map_err(|e| e.to_string())?.clone();
    if !task_accepts_input(&status) {
        return Err(task_input_rejection_message(&id, &status));
    }

    let mut guard = task.writer.lock().map_err(|e| e.to_string())?;
    let writer = guard
        .as_mut()
        .ok_or_else(|| task_input_rejection_message(&id, "closed"))?;
    writer.write_all(data.as_bytes()).map_err(|e| {
        format!("task {id} input stream is closed: {e}. Start a handoff or new task.")
    })?;
    writer.flush().ok();
    Ok(())
}

/// Run a flow on demand — loads the flow record, bumps `last_run_at`, and
/// launches a real PTY task with the flow's agent/prompt/project. The Go
/// side's `run-flow-now` RPC only marked the timestamp and never spawned,
/// so clicking "Run now" from the Jobs page was a silent no-op.
#[tauri::command]
pub fn run_flow_now(
    app: AppHandle,
    manager: State<'_, AgentManagerState>,
    env_state: State<'_, ShellEnvState>,
    db_state: State<'_, crate::commands_read::DbState>,
    id: String,
) -> Result<serde_json::Value, String> {
    let (agent, prompt, project_name, project_path) = {
        let db = db_state.0.lock().map_err(|e| e.to_string())?;
        let flow = db
            .get_flow(&id)
            .map_err(|e| format!("load flow {id}: {e}"))?;
        db.mark_flow_run(&id).map_err(|e| e.to_string())?;
        (
            flow.agent,
            flow.prompt,
            flow.project_name,
            flow.project_path,
        )
    };
    spawn_pty_task(
        &app,
        &manager,
        &env_state,
        SpawnAgentRequest {
            agent,
            prompt,
            project_name,
            project_path,
            title: format!("flow: {id}"),
            initial_chat_blocks: Vec::new(),
        },
    )
}

/// Spawn a sibling task with the same project context but a different agent
/// and a handoff-framed prompt. Used when the user wants a second agent to
/// continue an in-flight or finished task.
///
/// Looks up the source task in the in-memory AgentManager first (fast path
/// for live tasks), then falls back to the persisted `desktop_tasks` row
/// via HaftDb — completed or restart-restored tasks aren't in the manager,
/// and the Tasks UI surfaces a handoff action for them too.
#[tauri::command]
pub fn handoff_task(
    app: AppHandle,
    manager: State<'_, AgentManagerState>,
    env_state: State<'_, ShellEnvState>,
    db_state: State<'_, crate::commands_read::DbState>,
    id: String,
    agent: String,
) -> Result<serde_json::Value, String> {
    let (title, prompt, project_name, project_path) = {
        let mgr = manager.0.lock().map_err(|e| e.to_string())?;
        if let Some(task) = mgr.tasks.get(&id).cloned() {
            (
                task.title.clone(),
                task.prompt.clone(),
                task.project_name.clone(),
                task.project_path.clone(),
            )
        } else {
            drop(mgr);
            let db = db_state.0.lock().map_err(|e| e.to_string())?;
            let row = db
                .get_task(&id)
                .map_err(|e| format!("task not found (in-memory or persisted) for {id}: {e}"))?;
            (row.title, row.prompt, row.project, row.project_path)
        }
    };
    let handoff_prompt = format!("Handoff for {title}\n\n{prompt}");
    spawn_pty_task(
        &app,
        &manager,
        &env_state,
        SpawnAgentRequest {
            agent,
            prompt: handoff_prompt,
            project_name,
            project_path,
            title: format!("handoff: {title}"),
            initial_chat_blocks: Vec::new(),
        },
    )
}

#[tauri::command]
pub fn continue_task(
    app: AppHandle,
    manager: State<'_, AgentManagerState>,
    env_state: State<'_, ShellEnvState>,
    db_state: State<'_, crate::commands_read::DbState>,
    id: String,
    message: String,
) -> Result<serde_json::Value, String> {
    let trimmed_message = message.trim();
    if trimmed_message.is_empty() {
        return Err("continuation message is required".into());
    }

    let context = task_continuation_context(&manager, &db_state, &id)?;
    let visible_prompt = visible_original_prompt(&context.prompt, &context.chat_blocks);
    let transcript = strip_control_prompt_sections(&context.transcript);
    let prompt = continuation_prompt(
        &context.title,
        &visible_prompt,
        &transcript,
        trimmed_message,
    );
    let initial_chat_blocks =
        continuation_seed_blocks(&visible_prompt, &context.chat_blocks, trimmed_message);

    let next_task = spawn_pty_task(
        &app,
        &manager,
        &env_state,
        SpawnAgentRequest {
            agent: context.agent,
            prompt,
            project_name: context.project_name,
            project_path: context.project_path,
            title: context.title,
            initial_chat_blocks,
        },
    )?;

    // Continuation is a new runtime turn for the same operator conversation.
    // Hide the old terminal turn from the task list so the sidebar remains a
    // conversation list rather than a process-history list.
    if let Ok(db) = db_state.0.lock() {
        let _ = db.archive_task(&id);
    }

    Ok(next_task)
}

fn task_continuation_context(
    manager: &State<'_, AgentManagerState>,
    db_state: &State<'_, crate::commands_read::DbState>,
    id: &str,
) -> Result<TaskContinuationContext, String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    if let Some(task) = mgr.tasks.get(id).cloned() {
        let transcript = task.output.lock().map(|b| b.snapshot()).unwrap_or_default();

        return Ok(TaskContinuationContext {
            title: task.title.clone(),
            prompt: task.prompt.clone(),
            agent: task.agent.as_str().into(),
            project_name: task.project_name.clone(),
            project_path: task.project_path.clone(),
            transcript,
            chat_blocks: task
                .chat_blocks
                .lock()
                .map(|blocks| blocks.clone())
                .unwrap_or_default(),
        });
    }
    drop(mgr);

    let db = db_state.0.lock().map_err(|e| e.to_string())?;
    let row = db
        .get_task(id)
        .map_err(|e| format!("task not found (in-memory or persisted) for {id}: {e}"))?;
    let transcript = first_nonempty(&[&row.raw_output, &row.output]);

    Ok(TaskContinuationContext {
        title: row.title,
        prompt: row.prompt,
        agent: row.agent,
        project_name: row.project,
        project_path: row.project_path,
        transcript,
        chat_blocks: row.chat_blocks,
    })
}

fn cancel_running_task(
    app: &AppHandle,
    manager: &State<'_, AgentManagerState>,
    task_id: &str,
) -> Result<(), String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let task = mgr
        .tasks
        .get(task_id)
        .cloned()
        .ok_or_else(|| format!("task not found: {task_id}"))?;
    drop(mgr);

    task.cancelled.store(true, Ordering::SeqCst);

    // SIGTERM via kill — portable-pty Child trait exposes kill().
    let mut child_guard = task.child.lock().map_err(|e| e.to_string())?;
    if let Some(ref mut child) = *child_guard {
        // First attempt: kill (sends appropriate signal on the platform).
        child.kill().ok();

        // Give the process CANCEL_GRACE to exit, then force-wait.
        let start = Instant::now();
        loop {
            match child.try_wait() {
                Ok(Some(_)) => break,
                Ok(None) => {
                    if start.elapsed() >= CANCEL_GRACE {
                        // Force kill again — on Unix this is the same as kill().
                        child.kill().ok();
                        break;
                    }
                    thread::sleep(Duration::from_millis(50));
                }
                Err(_) => break,
            }
        }
    }
    drop(child_guard);

    // Update status.
    if let Ok(mut status) = task.status.lock() {
        *status = "cancelled".into();
    }
    close_task_writer(&task);

    let _ = app.emit(
        "task.status",
        TaskStatusEvent {
            id: task_id.to_string(),
            status: "cancelled".into(),
            error_message: String::new(),
        },
    );

    persist_task_state(&task, Some("cancelled"));

    Ok(())
}

#[tauri::command]
pub fn list_running_agents(
    manager: State<'_, AgentManagerState>,
) -> Result<Vec<serde_json::Value>, String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let result: Vec<serde_json::Value> = mgr
        .tasks
        .values()
        .map(|t| {
            let status = t.status.lock().map(|s| s.clone()).unwrap_or_default();
            serde_json::json!({
                "id": t.id,
                "agent": t.agent.as_str(),
                "title": t.title,
                "project_name": t.project_name,
                "project_path": t.project_path,
                "status": status,
                "started_at": t.started_at,
            })
        })
        .collect();
    Ok(result)
}

#[tauri::command]
pub fn get_agent_output(
    manager: State<'_, AgentManagerState>,
    task_id: String,
) -> Result<serde_json::Value, String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let task = mgr
        .tasks
        .get(&task_id)
        .ok_or_else(|| format!("task not found: {task_id}"))?;

    let output = task.output.lock().map(|b| b.snapshot()).unwrap_or_default();
    let blocks = task
        .chat_blocks
        .lock()
        .map(|b| b.clone())
        .unwrap_or_default();
    let status = task.status.lock().map(|s| s.clone()).unwrap_or_default();

    Ok(serde_json::json!({
        "id": task_id,
        "status": status,
        "output": output,
        "chat_blocks": blocks,
    }))
}

// ─── PTY reader ───

fn pty_reader_loop(app: AppHandle, task: Arc<RunningTask>, mut reader: Box<dyn Read + Send>) {
    let mut buf = [0u8; 4096];
    let mut line_buf = String::new();
    let mut last_emit = Instant::now();

    loop {
        match reader.read(&mut buf) {
            Ok(0) => break, // EOF
            Ok(n) => {
                let raw = String::from_utf8_lossy(&buf[..n]);
                let cleaned = strip_ansi(&raw);

                // Append to output buffer.
                if let Ok(mut output) = task.output.lock() {
                    output.append(&cleaned);
                }

                // Parse lines for chat blocks.
                line_buf.push_str(&cleaned);
                while let Some(newline_pos) = line_buf.find('\n') {
                    let line: String = line_buf.drain(..=newline_pos).collect();
                    let line = line.trim_end_matches('\n').trim_end_matches('\r');
                    if !line.is_empty() {
                        parse_and_append_blocks(&task, line);
                    }
                }

                // Emit output event (debounced to avoid flooding).
                if last_emit.elapsed() >= Duration::from_millis(50) {
                    let snapshot = task.output.lock().map(|b| b.snapshot()).unwrap_or_default();
                    let _ = app.emit(
                        "task.output",
                        TaskOutputEvent {
                            id: task.id.clone(),
                            chunk: cleaned.to_string(),
                            output: snapshot,
                        },
                    );
                    last_emit = Instant::now();
                }
            }
            Err(_) => break,
        }
    }

    // Flush any remaining partial line.
    let remaining = line_buf.trim().to_string();
    if !remaining.is_empty() {
        parse_and_append_blocks(&task, &remaining);
    }

    // Final output emit.
    let snapshot = task.output.lock().map(|b| b.snapshot()).unwrap_or_default();
    let _ = app.emit(
        "task.output",
        TaskOutputEvent {
            id: task.id.clone(),
            chunk: String::new(),
            output: snapshot,
        },
    );
}

// ─── Wait + finalize ───

fn wait_and_finalize(app: AppHandle, task: Arc<RunningTask>) {
    // Periodic flush to RPC while running.
    let flush_task = Arc::clone(&task);
    let flush_stop = Arc::new(AtomicBool::new(false));
    let flush_stop_clone = Arc::clone(&flush_stop);

    let flush_thread = thread::spawn(move || {
        loop {
            thread::sleep(FLUSH_INTERVAL);
            if flush_stop_clone.load(Ordering::SeqCst) {
                break;
            }
            let status = flush_task
                .status
                .lock()
                .map(|s| s.clone())
                .unwrap_or_default();
            if status != "running" {
                break;
            }
            persist_task_state(&flush_task, None);
        }
    });

    // Wait for child process to exit.
    loop {
        let mut child_guard = match task.child.lock() {
            Ok(g) => g,
            Err(_) => break,
        };

        if let Some(ref mut child) = *child_guard {
            match child.try_wait() {
                Ok(Some(status)) => {
                    let was_cancelled = task.cancelled.load(Ordering::SeqCst);
                    let final_status = if was_cancelled {
                        "cancelled"
                    } else if status.exit_code() == 0 {
                        "completed"
                    } else {
                        "failed"
                    };

                    if let Ok(mut s) = task.status.lock() {
                        *s = final_status.into();
                    }
                    close_task_writer(&task);

                    if final_status == "failed" {
                        if let Ok(mut em) = task.error_message.lock() {
                            *em = format!("agent exited with code {}", status.exit_code());
                        }
                    }

                    drop(child_guard);

                    // Stop flusher.
                    flush_stop.store(true, Ordering::SeqCst);
                    flush_thread.join().ok();

                    // Final persist.
                    persist_task_state(&task, Some(final_status));

                    // Emit final status.
                    let error_msg = task
                        .error_message
                        .lock()
                        .map(|e| e.clone())
                        .unwrap_or_default();
                    let _ = app.emit(
                        "task.status",
                        TaskStatusEvent {
                            id: task.id.clone(),
                            status: final_status.into(),
                            error_message: error_msg,
                        },
                    );

                    return;
                }
                Ok(None) => {
                    drop(child_guard);
                    thread::sleep(Duration::from_millis(100));
                }
                Err(_) => {
                    drop(child_guard);
                    break;
                }
            }
        } else {
            drop(child_guard);
            break;
        }
    }

    // If we get here without a clean exit, mark as failed.
    flush_stop.store(true, Ordering::SeqCst);
    flush_thread.join().ok();

    if let Ok(mut s) = task.status.lock() {
        if *s == "running" {
            *s = "failed".into();
        }
    }
    close_task_writer(&task);

    persist_task_state(&task, Some("failed"));

    let _ = app.emit(
        "task.status",
        TaskStatusEvent {
            id: task.id.clone(),
            status: "failed".into(),
            error_message: "agent process lost".into(),
        },
    );
}

// ─── Chat block parsing ───

fn parse_and_append_blocks(task: &RunningTask, line: &str) {
    let blocks = match task.agent {
        AgentKind::Claude => parse_claude_line(line),
        AgentKind::Codex => parse_codex_line(line),
    };

    if blocks.is_empty() {
        return;
    }

    if let Ok(mut cb) = task.chat_blocks.lock() {
        for mut block in blocks {
            let seq = task.block_seq.fetch_add(1, Ordering::SeqCst);
            block.id = format!("block-{seq}");
            cb.push(block);
        }
    }
}

// ── Claude stream-json parsing ──

#[derive(Deserialize, Default)]
struct ClaudeEnvelope {
    #[serde(default, rename = "type")]
    msg_type: String,
    #[serde(default)]
    message: ClaudeMessage,
    #[serde(default)]
    parent_tool_use_id: String,
    #[serde(default)]
    error: Option<ClaudeError>,
}

#[derive(Deserialize, Default)]
struct ClaudeMessage {
    #[serde(default)]
    role: String,
    #[serde(default)]
    content: Option<serde_json::Value>,
}

#[derive(Deserialize, Default)]
struct ClaudeError {
    #[serde(default)]
    message: String,
}

#[derive(Deserialize, Default)]
struct ClaudeContentBlock {
    #[serde(default, rename = "type")]
    block_type: String,
    #[serde(default)]
    id: String,
    #[serde(default)]
    tool_use_id: String,
    #[serde(default)]
    name: String,
    #[serde(default)]
    text: String,
    #[serde(default)]
    thinking: String,
    #[serde(default)]
    input: Option<serde_json::Value>,
    #[serde(default)]
    content: Option<serde_json::Value>,
    #[serde(default)]
    is_error: bool,
}

fn parse_claude_line(line: &str) -> Vec<ChatBlock> {
    let envelope: ClaudeEnvelope = match serde_json::from_str(line) {
        Ok(e) => e,
        Err(_) => return Vec::new(),
    };

    match envelope.msg_type.as_str() {
        "system" | "rate_limit_event" => Vec::new(),

        "result" => Vec::new(),

        "error" => {
            let text = envelope
                .error
                .as_ref()
                .map(|e| e.message.clone())
                .filter(|m| !m.is_empty())
                .unwrap_or_else(|| line.to_string());
            vec![ChatBlock {
                block_type: "text".into(),
                role: "system".into(),
                text,
                ..Default::default()
            }]
        }

        "assistant" | "user" | "message" => {
            let role = first_nonempty(&[
                &envelope.message.role,
                if envelope.msg_type == "user" {
                    "user"
                } else {
                    "assistant"
                },
            ]);

            let content_blocks = parse_claude_content(envelope.message.content.as_ref());
            let mut blocks = Vec::new();

            for cb in content_blocks {
                let block_type = normalize_claude_block_type(&cb);
                match block_type.as_str() {
                    "text" => {
                        let text =
                            first_nonempty(&[&cb.text, &format_json_value(cb.content.as_ref())]);
                        if !text.is_empty() {
                            blocks.push(ChatBlock {
                                block_type: "text".into(),
                                role: role.clone(),
                                text,
                                ..Default::default()
                            });
                        }
                    }
                    "thinking" | "redacted_thinking" => {
                        let mut text = first_nonempty(&[
                            &cb.thinking,
                            &cb.text,
                            &format_json_value(cb.content.as_ref()),
                        ]);
                        if text.is_empty() && block_type == "redacted_thinking" {
                            text = "[redacted thinking]".into();
                        }
                        blocks.push(ChatBlock {
                            block_type: "thinking".into(),
                            role: role.clone(),
                            text,
                            ..Default::default()
                        });
                    }
                    "tool_use" => {
                        blocks.push(ChatBlock {
                            block_type: "tool_use".into(),
                            role: role.clone(),
                            name: cb.name.clone(),
                            call_id: first_nonempty(&[&cb.id, &cb.tool_use_id]),
                            input: format_json_value(cb.input.as_ref()),
                            ..Default::default()
                        });
                    }
                    "tool_result" => {
                        blocks.push(ChatBlock {
                            block_type: "tool_result".into(),
                            role: role.clone(),
                            call_id: first_nonempty(&[
                                &cb.tool_use_id,
                                &cb.id,
                                &envelope.parent_tool_use_id,
                            ]),
                            output: format_json_value(cb.content.as_ref()),
                            is_error: cb.is_error,
                            ..Default::default()
                        });
                    }
                    _ => {
                        let text = format_json_value(Some(&serde_json::json!(cb.text)));
                        if !text.is_empty() {
                            blocks.push(ChatBlock {
                                block_type: "text".into(),
                                role: role.clone(),
                                text,
                                ..Default::default()
                            });
                        }
                    }
                }
            }

            blocks
        }

        _ => Vec::new(),
    }
}

fn parse_claude_content(raw: Option<&serde_json::Value>) -> Vec<ClaudeContentBlock> {
    let value = match raw {
        Some(v) if !v.is_null() => v,
        _ => return Vec::new(),
    };

    // String → single text block.
    if let Some(s) = value.as_str() {
        return vec![ClaudeContentBlock {
            block_type: "text".into(),
            text: s.to_string(),
            ..Default::default()
        }];
    }

    // Array → parse each element.
    if let Some(arr) = value.as_array() {
        return arr
            .iter()
            .filter_map(|item| serde_json::from_value::<ClaudeContentBlock>(item.clone()).ok())
            .map(|cb| {
                let mut cb = cb;
                cb.block_type = normalize_claude_block_type(&cb);
                cb
            })
            .collect();
    }

    // Single object.
    serde_json::from_value::<ClaudeContentBlock>(value.clone())
        .ok()
        .into_iter()
        .collect()
}

fn normalize_claude_block_type(block: &ClaudeContentBlock) -> String {
    if !block.block_type.trim().is_empty() {
        return block.block_type.clone();
    }

    if !block.thinking.trim().is_empty() {
        return "thinking".into();
    }
    if !block.text.trim().is_empty() {
        return "text".into();
    }
    if !block.name.trim().is_empty() || has_json_value(block.input.as_ref()) {
        return "tool_use".into();
    }
    if !block.tool_use_id.trim().is_empty() || has_json_value(block.content.as_ref()) {
        return "tool_result".into();
    }

    String::new()
}

// ── Codex stream parsing ──

#[derive(Deserialize, Default)]
struct CodexEnvelope {
    #[serde(default, rename = "type")]
    msg_type: String,
    #[serde(default)]
    text: String,
    #[serde(default)]
    delta: String,
    #[serde(default)]
    message: String,
    #[serde(default)]
    output_text: String,
    #[serde(default)]
    error: String,
    #[serde(default)]
    item: CodexItem,
}

#[derive(Deserialize, Default)]
struct CodexItem {
    #[serde(default)]
    id: String,
    #[serde(default, rename = "type")]
    item_type: String,
    #[serde(default)]
    name: String,
    #[serde(default)]
    server: String,
    #[serde(default)]
    tool: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    text: String,
    #[serde(default)]
    command: String,
    #[serde(default)]
    query: String,
    #[serde(default)]
    description: String,
    #[serde(default)]
    aggregated_output: String,
    #[serde(default)]
    exit_code: Option<i64>,
    #[serde(default)]
    input: Option<serde_json::Value>,
    #[serde(default)]
    arguments: Option<serde_json::Value>,
    #[serde(default)]
    output: Option<serde_json::Value>,
    #[serde(default)]
    result: Option<serde_json::Value>,
}

fn parse_codex_line(line: &str) -> Vec<ChatBlock> {
    let envelope: CodexEnvelope = match serde_json::from_str(line) {
        Ok(e) => e,
        Err(_) => return Vec::new(),
    };

    match envelope.msg_type.as_str() {
        "thread.started" | "turn.started" | "turn.completed" => Vec::new(),

        "turn.failed" | "error" => {
            let text = first_nonempty(&[
                &envelope.error,
                &envelope.message,
                &envelope.text,
                &envelope.output_text,
            ]);
            let text = if text.is_empty() {
                line.to_string()
            } else {
                text
            };
            vec![ChatBlock {
                block_type: "text".into(),
                role: "system".into(),
                text,
                ..Default::default()
            }]
        }

        "item.started" => {
            if let Some(block) = codex_tool_use_block(&envelope.item) {
                vec![block]
            } else {
                Vec::new()
            }
        }

        "item.completed" => {
            if let Some(block) = codex_narrative_block(&envelope.item) {
                return vec![block];
            }
            if let Some(block) = codex_tool_result_block(&envelope.item) {
                return vec![block];
            }
            Vec::new()
        }

        "agent_message"
        | "assistant_message"
        | "assistant_message_delta"
        | "assistant_response"
        | "assistant"
        | "agent_message_delta"
        | "message"
        | "message_delta" => {
            let text = first_nonempty(&[
                &envelope.text,
                &envelope.delta,
                &envelope.output_text,
                &envelope.message,
            ]);
            if text.trim().is_empty() {
                return Vec::new();
            }
            vec![ChatBlock {
                block_type: "text".into(),
                role: "assistant".into(),
                text,
                ..Default::default()
            }]
        }

        "reasoning" | "reasoning_delta" | "reasoning_summary" | "reasoning_summary_delta" => {
            let text = first_nonempty(&[
                &envelope.text,
                &envelope.delta,
                &envelope.output_text,
                &envelope.message,
            ]);
            if text.trim().is_empty() {
                return Vec::new();
            }
            vec![ChatBlock {
                block_type: "thinking".into(),
                role: "assistant".into(),
                text,
                ..Default::default()
            }]
        }

        _ => Vec::new(),
    }
}

fn is_codex_tool_item(item: &CodexItem) -> bool {
    let t = item.item_type.trim();
    if t.is_empty() || is_codex_narrative_type(t) {
        return false;
    }

    matches!(
        t,
        "command_execution"
            | "mcp_tool_call"
            | "file_change"
            | "web_search"
            | "web_search_call"
            | "todo_list"
            | "file_search"
            | "tool_call"
    ) || !item.command.trim().is_empty()
        || !item.query.trim().is_empty()
        || !item.description.trim().is_empty()
        || !item.server.trim().is_empty()
        || !item.tool.trim().is_empty()
        || !item.aggregated_output.trim().is_empty()
        || has_json_value(item.arguments.as_ref())
        || has_json_value(item.input.as_ref())
        || has_json_value(item.output.as_ref())
        || has_json_value(item.result.as_ref())
        || item.exit_code.is_some()
}

fn is_codex_narrative_type(t: &str) -> bool {
    matches!(
        t.trim(),
        "" | "agent_message"
            | "assistant_message"
            | "assistant_response"
            | "assistant"
            | "message"
            | "reasoning"
            | "reasoning_summary"
            | "error"
    )
}

fn codex_tool_name(item: &CodexItem) -> String {
    if item.item_type == "mcp_tool_call" {
        let server = item.server.trim();
        let name = first_nonempty(&[&item.name, &item.tool, &item.item_type]);
        if server.is_empty() {
            return name;
        }
        return format!("{server}:{name}");
    }

    first_nonempty(&[&item.name, &item.tool, &item.item_type])
}

fn codex_tool_input(item: &CodexItem) -> String {
    if !item.command.trim().is_empty() {
        return item.command.clone();
    }
    if !item.query.trim().is_empty() {
        return item.query.clone();
    }
    if !item.description.trim().is_empty() {
        return item.description.clone();
    }

    let input = format_json_value(item.arguments.as_ref());
    if !input.is_empty() {
        return input;
    }

    format_json_value(item.input.as_ref())
}

fn codex_tool_output(item: &CodexItem) -> String {
    if !item.aggregated_output.trim().is_empty() {
        return item.aggregated_output.clone();
    }
    if !item.text.trim().is_empty() {
        return item.text.clone();
    }
    let output = format_json_value(item.output.as_ref());
    if !output.is_empty() {
        return output;
    }
    format_json_value(item.result.as_ref())
}

fn codex_tool_failed(item: &CodexItem) -> bool {
    let status = item.status.trim().to_lowercase();
    if status == "failed" || status == "error" {
        return true;
    }
    item.exit_code.is_some_and(|c| c != 0)
}

fn codex_tool_use_block(item: &CodexItem) -> Option<ChatBlock> {
    if !is_codex_tool_item(item) {
        return None;
    }

    Some(ChatBlock {
        block_type: "tool_use".into(),
        role: "assistant".into(),
        name: codex_tool_name(item),
        call_id: item.id.clone(),
        input: codex_tool_input(item),
        ..Default::default()
    })
}

fn codex_narrative_block(item: &CodexItem) -> Option<ChatBlock> {
    let text = first_nonempty(&[
        &item.text,
        &format_json_value(item.output.as_ref()),
        &format_json_value(item.result.as_ref()),
    ]);
    if text.trim().is_empty() {
        return None;
    }

    match item.item_type.trim() {
        "agent_message" | "assistant_message" | "assistant_response" | "assistant" | "message" => {
            Some(ChatBlock {
                block_type: "text".into(),
                role: "assistant".into(),
                text,
                ..Default::default()
            })
        }
        "reasoning" | "reasoning_summary" => Some(ChatBlock {
            block_type: "thinking".into(),
            role: "assistant".into(),
            text,
            ..Default::default()
        }),
        "error" => Some(ChatBlock {
            block_type: "text".into(),
            role: "system".into(),
            text,
            is_error: true,
            ..Default::default()
        }),
        _ => None,
    }
}

fn codex_tool_result_block(item: &CodexItem) -> Option<ChatBlock> {
    if !is_codex_tool_item(item) {
        return None;
    }

    Some(ChatBlock {
        block_type: "tool_result".into(),
        role: "assistant".into(),
        name: codex_tool_name(item),
        call_id: item.id.clone(),
        output: codex_tool_output(item),
        is_error: codex_tool_failed(item),
        ..Default::default()
    })
}

// ─── Agent CLI construction ───

fn build_agent_args(kind: AgentKind, prompt: &str, work_dir: &str) -> Vec<String> {
    match kind {
        AgentKind::Claude => vec![
            "claude".into(),
            "-p".into(),
            "--verbose".into(),
            "--output-format".into(),
            "stream-json".into(),
            "--permission-mode".into(),
            "default".into(),
            prompt.into(),
        ],
        AgentKind::Codex => vec![
            "codex".into(),
            "exec".into(),
            "--cd".into(),
            work_dir.into(),
            "--ask-for-approval".into(),
            "untrusted".into(),
            "--json".into(),
            "-c".into(),
            "mcp_servers={}".into(),
            prompt.into(),
        ],
    }
}

// ─── Persistence via haft desktop-rpc ───

fn persist_task_state(task: &RunningTask, status_override: Option<&str>) {
    let status = status_override
        .map(String::from)
        .or_else(|| task.status.lock().ok().map(|s| s.clone()))
        .unwrap_or_else(|| "running".into());

    let output = task.output.lock().map(|b| b.snapshot()).unwrap_or_default();

    let blocks = task
        .chat_blocks
        .lock()
        .map(|b| b.clone())
        .unwrap_or_default();

    let blocks_json = serde_json::to_string(&blocks).unwrap_or_else(|_| "[]".into());

    let error_msg = task
        .error_message
        .lock()
        .map(|e| e.clone())
        .unwrap_or_default();

    let completed_at = if status == "completed" || status == "failed" || status == "cancelled" {
        now_rfc3339()
    } else {
        String::new()
    };

    let payload = serde_json::json!({
        "id": task.id,
        "project_name": task.project_name,
        "project_path": task.project_path,
        "title": task.title,
        "agent": task.agent.as_str(),
        "status": status,
        "prompt": task.prompt,
        "error_message": error_msg,
        "output_tail": truncate(&output, 4096),
        "chat_blocks_json": blocks_json,
        "raw_output": truncate(&output, OUTPUT_MAX_CHARS),
        "started_at": task.started_at,
        "completed_at": completed_at,
        "updated_at": now_rfc3339(),
    });

    // Fire-and-forget: RPC persistence failure is non-fatal.
    let _ = rpc::call_rpc("persist-task", Some(payload), Some(&task.project_path));
}

// ─── Helpers ───

fn task_accepts_input(status: &str) -> bool {
    matches!(status.trim().to_lowercase().as_str(), "running" | "idle")
}

fn task_input_rejection_message(task_id: &str, status: &str) -> String {
    format!(
        "task {task_id} is not accepting input (status: {status}). Start a handoff or new task."
    )
}

fn close_task_writer(task: &RunningTask) {
    if let Ok(mut writer) = task.writer.lock() {
        *writer = None;
    }
}

fn strip_ansi(s: &str) -> String {
    let bytes = s.as_bytes();
    let mut result = Vec::with_capacity(bytes.len());
    let mut i = 0;

    while i < bytes.len() {
        if bytes[i] == 0x1b {
            i += 1;
            if i >= bytes.len() {
                break;
            }
            if bytes[i] == b'[' {
                // CSI sequence: skip until final byte (0x40..=0x7E).
                i += 1;
                while i < bytes.len() && bytes[i] < 0x40 {
                    i += 1;
                }
                if i < bytes.len() {
                    i += 1; // skip final byte
                }
            } else if bytes[i] == b']' {
                // OSC sequence: skip until ST (ESC \ or BEL).
                i += 1;
                while i < bytes.len() {
                    if bytes[i] == 0x07 {
                        i += 1;
                        break;
                    }
                    if bytes[i] == 0x1b && i + 1 < bytes.len() && bytes[i + 1] == b'\\' {
                        i += 2;
                        break;
                    }
                    i += 1;
                }
            } else {
                // Simple escape: skip one char.
                i += 1;
            }
        } else {
            result.push(bytes[i]);
            i += 1;
        }
    }

    String::from_utf8_lossy(&result).into_owned()
}

fn format_json_value(value: Option<&serde_json::Value>) -> String {
    match value {
        None => String::new(),
        Some(serde_json::Value::Null) => String::new(),
        Some(serde_json::Value::String(s)) => s.clone(),
        Some(serde_json::Value::Array(arr)) => {
            let parts: Vec<String> = arr
                .iter()
                .map(|v| format_json_value(Some(v)))
                .filter(|s| !s.trim().is_empty())
                .collect();
            parts.join("\n")
        }
        Some(other) => serde_json::to_string(other).unwrap_or_default(),
    }
}

fn has_json_value(value: Option<&serde_json::Value>) -> bool {
    matches!(value, Some(v) if !v.is_null())
}

fn first_nonempty(candidates: &[&str]) -> String {
    for s in candidates {
        if !s.trim().is_empty() {
            return s.to_string();
        }
    }
    String::new()
}

fn continuation_prompt(title: &str, prompt: &str, transcript: &str, message: &str) -> String {
    let transcript_tail = tail_text(transcript, CONTINUATION_TRANSCRIPT_MAX_CHARS);

    format!(
        "{CONTROL_PROMPT_PREFIX}\n\nTask title:\n{title}\n\nOriginal prompt:\n{prompt}\n\nPrior transcript tail:\n{transcript_tail}\n\n{CONTROL_PROMPT_FOLLOW_UP}\n{message}\n\n{CONTROL_PROMPT_SUFFIX}"
    )
}

fn continuation_seed_blocks(
    original_prompt: &str,
    previous_blocks: &[ChatBlock],
    message: &str,
) -> Vec<ChatBlock> {
    let mut blocks: Vec<ChatBlock> = previous_blocks
        .iter()
        .filter_map(visible_conversation_block)
        .collect();
    let prompt = original_prompt.trim();

    if !prompt.is_empty() && !is_control_prompt(prompt) && !has_user_text(&blocks, prompt) {
        blocks.insert(0, user_text_block("initial-user", prompt));
    }

    let continuation_id = format!("continuation-user-{}", blocks.len() + 1);
    blocks.push(user_text_block(&continuation_id, message.trim()));

    blocks
}

fn has_user_text(blocks: &[ChatBlock], text: &str) -> bool {
    blocks
        .iter()
        .any(|block| block.role == "user" && block.text.trim() == text)
}

fn visible_original_prompt(prompt: &str, blocks: &[ChatBlock]) -> String {
    let trimmed_prompt = prompt.trim();

    if !trimmed_prompt.is_empty() && !is_control_prompt(trimmed_prompt) {
        return trimmed_prompt.into();
    }

    blocks
        .iter()
        .filter_map(visible_conversation_block)
        .find(|block| {
            block.role == "user" && !block.text.trim().is_empty() && !is_control_prompt(&block.text)
        })
        .map(|block| block.text.trim().to_string())
        .unwrap_or_default()
}

fn visible_conversation_block(block: &ChatBlock) -> Option<ChatBlock> {
    let without_control = strip_control_prompt_sections(&block.text);
    let visible_text = strip_audit_only_provider_envelope_lines(&without_control);
    let visible_block = if visible_text == block.text {
        block.clone()
    } else {
        let mut next = block.clone();
        next.text = visible_text;
        next
    };

    if chat_block_has_renderable_value(&visible_block) {
        return Some(visible_block);
    }

    None
}

fn chat_block_has_renderable_value(block: &ChatBlock) -> bool {
    [&block.text, &block.output, &block.input, &block.name]
        .iter()
        .any(|value| !value.trim().is_empty())
}

fn is_control_prompt(text: &str) -> bool {
    let trimmed = text.trim_start();

    trimmed.starts_with(CONTROL_PROMPT_PREFIX)
        || trimmed.contains(CONTROL_PROMPT_PREFIX)
        || (trimmed.contains(CONTROL_PROMPT_FOLLOW_UP) && trimmed.contains(CONTROL_PROMPT_SUFFIX))
}

fn strip_control_prompt_sections(text: &str) -> String {
    let prefixed = strip_prefixed_control_prompt_sections(text);

    strip_orphaned_control_prompt_tail(&prefixed)
}

fn strip_prefixed_control_prompt_sections(text: &str) -> String {
    if !text.contains(CONTROL_PROMPT_PREFIX) {
        return text.into();
    }

    let mut sections = text.split(CONTROL_PROMPT_PREFIX);
    let head = sections.next().unwrap_or_default();
    let visible_tails = sections
        .map(strip_control_prompt_tail)
        .collect::<Vec<_>>()
        .join("");

    format!("{head}{visible_tails}")
}

fn strip_control_prompt_tail(section: &str) -> String {
    section
        .split_once(CONTROL_PROMPT_SUFFIX)
        .map(|(_, tail)| tail.to_string())
        .unwrap_or_default()
}

fn strip_orphaned_control_prompt_tail(text: &str) -> String {
    if !text.contains(CONTROL_PROMPT_FOLLOW_UP) || !text.contains(CONTROL_PROMPT_SUFFIX) {
        return text.into();
    }

    let mut output = String::new();
    let mut remaining = text;

    loop {
        let Some(start) = remaining.find(CONTROL_PROMPT_FOLLOW_UP) else {
            output.push_str(remaining);
            return output;
        };
        let tail = &remaining[start..];
        let Some(end_rel) = tail.find(CONTROL_PROMPT_SUFFIX) else {
            output.push_str(remaining);
            return output;
        };
        let end = start + end_rel + CONTROL_PROMPT_SUFFIX.len();

        output.push_str(&remaining[..start]);
        remaining = &remaining[end..];
    }
}

#[derive(Deserialize, Default)]
struct ProviderEnvelope {
    #[serde(default, rename = "type")]
    envelope_type: String,
}

fn strip_audit_only_provider_envelope_lines(text: &str) -> String {
    if is_audit_only_provider_envelope(text) {
        return String::new();
    }

    text.lines()
        .filter(|line| !is_audit_only_provider_envelope_line(line.trim()))
        .collect::<Vec<_>>()
        .join("\n")
}

fn is_audit_only_provider_envelope(text: &str) -> bool {
    let trimmed = text.trim();

    if provider_envelope_visibility(trimmed) == ChatBlockVisibility::AuditOnly {
        return true;
    }

    let lines = trimmed
        .lines()
        .map(str::trim)
        .filter(|line| !line.is_empty())
        .collect::<Vec<_>>();

    if lines.len() <= 1 {
        return false;
    }

    lines
        .iter()
        .all(|line| is_audit_only_provider_envelope_line(line))
}

fn is_audit_only_provider_envelope_line(text: &str) -> bool {
    provider_envelope_visibility(text) == ChatBlockVisibility::AuditOnly
}

fn provider_envelope_visibility(text: &str) -> ChatBlockVisibility {
    if !looks_like_json_container(text) {
        return ChatBlockVisibility::Visible;
    }

    let envelope = match serde_json::from_str::<ProviderEnvelope>(text) {
        Ok(envelope) => envelope,
        Err(_) => return ChatBlockVisibility::Visible,
    };

    if audit_only_provider_envelope_type(&envelope.envelope_type) {
        return ChatBlockVisibility::AuditOnly;
    }

    ChatBlockVisibility::Visible
}

fn audit_only_provider_envelope_type(envelope_type: &str) -> bool {
    matches!(
        envelope_type,
        "result"
            | "system"
            | "rate_limit_event"
            | "thread.started"
            | "turn.started"
            | "turn.completed"
    )
}

fn looks_like_json_container(value: &str) -> bool {
    let trimmed = value.trim();

    (trimmed.starts_with('{') && trimmed.ends_with('}'))
        || (trimmed.starts_with('[') && trimmed.ends_with(']'))
}

fn user_text_block(id: &str, text: &str) -> ChatBlock {
    ChatBlock {
        id: id.into(),
        block_type: "text".into(),
        role: "user".into(),
        text: text.into(),
        ..ChatBlock::default()
    }
}

fn tail_text(s: &str, max_chars: usize) -> String {
    let chars: Vec<char> = s.chars().collect();

    if chars.len() <= max_chars {
        return s.to_string();
    }

    let tail: String = chars[chars.len() - max_chars..].iter().collect();
    format!("[truncated]\n{tail}")
}

fn truncate(s: &str, max: usize) -> String {
    if s.len() <= max {
        s.to_string()
    } else {
        s[..max].to_string()
    }
}

fn now_rfc3339() -> String {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default();
    // Simple RFC3339 without pulling in chrono.
    let secs = now.as_secs();
    let days_since_epoch = secs / 86400;
    let time_of_day = secs % 86400;
    let hours = time_of_day / 3600;
    let minutes = (time_of_day % 3600) / 60;
    let seconds = time_of_day % 60;

    // Compute year/month/day from days since 1970-01-01.
    let (year, month, day) = days_to_ymd(days_since_epoch);

    format!("{year:04}-{month:02}-{day:02}T{hours:02}:{minutes:02}:{seconds:02}Z")
}

fn days_to_ymd(mut days: u64) -> (u64, u64, u64) {
    // Algorithm from http://howardhinnant.github.io/date_algorithms.html
    days += 719468;
    let era = days / 146097;
    let doe = days - era * 146097;
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = doy - (153 * mp + 2) / 5 + 1;
    let m = if mp < 10 { mp + 3 } else { mp - 9 };
    let y = if m <= 2 { y + 1 } else { y };
    (y, m, d)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn agent_kind_accepts_only_v7_desktop_hosts() {
        assert_eq!(AgentKind::from_str("claude"), Some(AgentKind::Claude));
        assert_eq!(AgentKind::from_str("codex"), Some(AgentKind::Codex));
        assert_eq!(AgentKind::from_str("haft"), None);
    }

    #[test]
    fn strip_ansi_removes_csi() {
        assert_eq!(strip_ansi("\x1b[32mhello\x1b[0m"), "hello");
    }

    #[test]
    fn strip_ansi_removes_osc() {
        assert_eq!(strip_ansi("\x1b]0;title\x07text"), "text");
    }

    #[test]
    fn strip_ansi_passthrough() {
        assert_eq!(strip_ansi("plain text"), "plain text");
    }

    #[test]
    fn parse_claude_text_block() {
        let line = r#"{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello world"}]}}"#;
        let blocks = parse_claude_line(line);
        assert_eq!(blocks.len(), 1);
        assert_eq!(blocks[0].block_type, "text");
        assert_eq!(blocks[0].role, "assistant");
        assert_eq!(blocks[0].text, "Hello world");
    }

    #[test]
    fn parse_claude_tool_use() {
        let line = r#"{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"call-1","name":"Read","input":{"path":"/tmp/test"}}]}}"#;
        let blocks = parse_claude_line(line);
        assert_eq!(blocks.len(), 1);
        assert_eq!(blocks[0].block_type, "tool_use");
        assert_eq!(blocks[0].name, "Read");
        assert_eq!(blocks[0].call_id, "call-1");
    }

    #[test]
    fn parse_claude_error() {
        let line = r#"{"type":"error","error":{"message":"rate limited"}}"#;
        let blocks = parse_claude_line(line);
        assert_eq!(blocks.len(), 1);
        assert_eq!(blocks[0].block_type, "text");
        assert_eq!(blocks[0].role, "system");
        assert_eq!(blocks[0].text, "rate limited");
    }

    #[test]
    fn parse_claude_result_envelope_is_audit_only() {
        let line = r#"{"type":"result","usage":{"input_tokens":6},"result":{"duration_ms":10}}"#;
        let blocks = parse_claude_line(line);

        assert!(blocks.is_empty());
    }

    #[test]
    fn parse_codex_agent_message() {
        let line = r#"{"type":"agent_message","text":"I'll help you"}"#;
        let blocks = parse_codex_line(line);
        assert_eq!(blocks.len(), 1);
        assert_eq!(blocks[0].block_type, "text");
        assert_eq!(blocks[0].text, "I'll help you");
    }

    #[test]
    fn parse_codex_tool_item() {
        let line = r#"{"type":"item.started","item":{"id":"item-1","type":"command_execution","command":"ls -la"}}"#;
        let blocks = parse_codex_line(line);
        assert_eq!(blocks.len(), 1);
        assert_eq!(blocks[0].block_type, "tool_use");
        assert_eq!(blocks[0].input, "ls -la");
    }

    #[test]
    fn parse_codex_reasoning() {
        let line = r#"{"type":"reasoning","text":"Let me think..."}"#;
        let blocks = parse_codex_line(line);
        assert_eq!(blocks.len(), 1);
        assert_eq!(blocks[0].block_type, "thinking");
    }

    #[test]
    fn output_buffer_bounds() {
        let mut buf = OutputBuffer::new();
        for i in 0..600 {
            buf.append(&format!("line {i}"));
        }
        assert!(buf.lines.len() <= OUTPUT_MAX_LINES);
    }

    #[test]
    fn task_input_acceptance_is_terminal_state_aware() {
        assert!(task_accepts_input("running"));
        assert!(task_accepts_input("idle"));
        assert!(task_accepts_input(" RUNNING "));

        assert!(!task_accepts_input("completed"));
        assert!(!task_accepts_input("failed"));
        assert!(!task_accepts_input("cancelled"));
        assert!(!task_accepts_input("checkpointed"));
        assert!(!task_accepts_input("blocked"));
        assert!(!task_accepts_input("Ready for PR"));
        assert!(!task_accepts_input(""));
    }

    fn assistant_text_block(id: &str, text: &str) -> ChatBlock {
        ChatBlock {
            id: id.into(),
            block_type: "text".into(),
            role: "assistant".into(),
            text: text.into(),
            ..ChatBlock::default()
        }
    }

    #[test]
    fn continuation_prompt_preserves_follow_up_and_tail() {
        let prompt = continuation_prompt("Task", "Original", "line one\nline two", "Next step");

        assert!(prompt.contains("Task title:\nTask"));
        assert!(prompt.contains("Original prompt:\nOriginal"));
        assert!(prompt.contains("Prior transcript tail:\nline one\nline two"));
        assert!(prompt.contains("Operator follow-up:\nNext step"));
    }

    #[test]
    fn tail_text_keeps_recent_context() {
        let tail = tail_text("abcdef", 3);

        assert_eq!(tail, "[truncated]\ndef");
    }

    #[test]
    fn continuation_seed_blocks_adds_visible_user_turns() {
        let previous = vec![ChatBlock {
            id: "assistant-1".into(),
            block_type: "text".into(),
            role: "assistant".into(),
            text: "Hello again.".into(),
            ..ChatBlock::default()
        }];

        let blocks = continuation_seed_blocks("Hello", &previous, "What next?");

        assert_eq!(blocks.len(), 3);
        assert_eq!(blocks[0].role, "user");
        assert_eq!(blocks[0].text, "Hello");
        assert_eq!(blocks[1].role, "assistant");
        assert_eq!(blocks[2].role, "user");
        assert_eq!(blocks[2].text, "What next?");
    }

    #[test]
    fn continuation_seed_blocks_drops_control_prompt_blocks() {
        let previous = vec![user_text_block(
            "control",
            "Continue the existing desktop task.\n\nOperator follow-up:\nhello",
        )];

        let blocks = continuation_seed_blocks("", &previous, "actual follow-up");

        assert_eq!(blocks.len(), 1);
        assert_eq!(blocks[0].text, "actual follow-up");
    }

    #[test]
    fn continuation_seed_blocks_keep_durable_turns_through_fourth_follow_up() {
        let original_prompt = "Original request";
        let first_turn_blocks = vec![
            user_text_block("initial-user", original_prompt),
            assistant_text_block("assistant-1", "Completed first pass."),
        ];

        let second_turn_blocks =
            continuation_seed_blocks(original_prompt, &first_turn_blocks, "Second follow-up");

        let mut checkpoint_blocks = second_turn_blocks.clone();
        checkpoint_blocks.push(user_text_block(
            "control-2",
            &continuation_prompt(
                "Task",
                original_prompt,
                "Completed first pass.",
                "Second follow-up",
            ),
        ));
        checkpoint_blocks.push(assistant_text_block(
            "provider-result",
            r#"{"type":"result","usage":{"input_tokens":6}}"#,
        ));
        checkpoint_blocks.push(assistant_text_block("assistant-2", "Checkpoint saved."));

        let third_prompt = visible_original_prompt(
            &continuation_prompt(
                "Task",
                original_prompt,
                "Checkpoint saved.",
                "Third follow-up",
            ),
            &checkpoint_blocks,
        );
        let third_turn_blocks =
            continuation_seed_blocks(&third_prompt, &checkpoint_blocks, "Third follow-up");

        let mut blocked_blocks = third_turn_blocks.clone();
        blocked_blocks.push(user_text_block(
            "control-3",
            &[
                "Operator follow-up:",
                "Third follow-up",
                "",
                "Continue from the prior context. Do not repeat completed setup unless it is necessary.",
            ]
            .join("\n"),
        ));
        blocked_blocks.push(assistant_text_block(
            "provider-turn",
            r#"{"type":"turn.completed","usage":{"input_tokens":9}}"#,
        ));
        blocked_blocks.push(assistant_text_block("assistant-3", "Blocked on approval."));

        let fourth_prompt = visible_original_prompt(
            &continuation_prompt(
                "Task",
                original_prompt,
                "Blocked on approval.",
                "Fourth follow-up",
            ),
            &blocked_blocks,
        );
        let fourth_turn_blocks =
            continuation_seed_blocks(&fourth_prompt, &blocked_blocks, "Fourth follow-up");
        let visible_texts = fourth_turn_blocks
            .iter()
            .map(|block| block.text.as_str())
            .collect::<Vec<_>>();
        let rendered = visible_texts.join("\n");

        assert_eq!(
            visible_texts,
            vec![
                "Original request",
                "Completed first pass.",
                "Second follow-up",
                "Checkpoint saved.",
                "Third follow-up",
                "Blocked on approval.",
                "Fourth follow-up",
            ],
        );
        assert!(!rendered.contains(r#""type":"result""#));
        assert!(!rendered.contains("Operator follow-up:"));
        assert!(!rendered.contains("Continue the existing desktop task."));
    }

    #[test]
    fn continuation_seed_blocks_drops_partially_parsed_control_prompt_blocks() {
        let previous = vec![user_text_block(
            "control",
            &[
                r#"{"type":"result","usage":{"input_tokens":6}}"#,
                "",
                "Operator follow-up:",
                "how are you?",
                "",
                "Continue from the prior context. Do not repeat completed setup unless it is necessary.",
            ]
            .join("\n"),
        )];

        let blocks = continuation_seed_blocks("", &previous, "actual follow-up");

        assert_eq!(blocks.len(), 1);
        assert_eq!(blocks[0].text, "actual follow-up");
    }

    #[test]
    fn visible_original_prompt_uses_first_real_user_block_when_prompt_is_control() {
        let blocks = vec![user_text_block("user", "Hello")];
        let prompt = visible_original_prompt(
            "Continue the existing desktop task.\n\nOperator follow-up:\nhello",
            &blocks,
        );

        assert_eq!(prompt, "Hello");
    }

    #[test]
    fn visible_original_prompt_skips_audit_only_user_envelopes() {
        let blocks = vec![
            user_text_block(
                "provider",
                r#"{"type":"result","usage":{"input_tokens":6}}"#,
            ),
            user_text_block("user", "Hello"),
        ];
        let prompt = visible_original_prompt(
            "Continue the existing desktop task.\n\nOperator follow-up:\nhello",
            &blocks,
        );

        assert_eq!(prompt, "Hello");
    }

    #[test]
    fn strip_control_prompt_sections_removes_envelope() {
        let text = [
            "before",
            "Continue the existing desktop task.",
            "Task title:",
            "Hello",
            "Continue from the prior context. Do not repeat completed setup unless it is necessary.",
            "after",
        ]
        .join("\n");

        let stripped = strip_control_prompt_sections(&text);

        assert!(stripped.contains("before"));
        assert!(stripped.contains("after"));
        assert!(!stripped.contains("Operator follow-up"));
        assert!(!stripped.contains("Task title:"));
    }

    #[test]
    fn strip_control_prompt_sections_removes_orphaned_control_tail() {
        let text = [
            "before",
            r#"{"type":"result","usage":{"input_tokens":6}}"#,
            "Operator follow-up:",
            "how are you?",
            "Continue from the prior context. Do not repeat completed setup unless it is necessary.",
            "after",
        ]
        .join("\n");

        let stripped = strip_control_prompt_sections(&text);

        assert!(stripped.contains("before"));
        assert!(stripped.contains(r#"{"type":"result""#));
        assert!(stripped.contains("after"));
        assert!(!stripped.contains("Operator follow-up"));
        assert!(!stripped.contains("how are you?"));
    }

    #[test]
    fn running_task_json_contains_full_frontend_contract() {
        let task = RunningTask {
            id: "task-1".into(),
            agent: AgentKind::Codex,
            project_name: "haft".into(),
            project_path: "/repo/haft".into(),
            title: "Test task".into(),
            prompt: "Do work".into(),
            status: Mutex::new("running".into()),
            error_message: Mutex::new(String::new()),
            output: Mutex::new(OutputBuffer::new()),
            chat_blocks: Mutex::new(Vec::new()),
            block_seq: AtomicU64::new(0),
            started_at: "2026-04-24T00:00:00Z".into(),
            cancelled: AtomicBool::new(false),
            child: Mutex::new(None),
            writer: Mutex::new(None),
        };

        let value = running_task_json(&task, Some("running"));

        assert_eq!(value["id"], "task-1");
        assert_eq!(value["project"], "haft");
        assert_eq!(value["project_path"], "/repo/haft");
        assert_eq!(value["prompt"], "Do work");
        assert!(value["chat_blocks"].is_array());
        assert_eq!(value["raw_output"], "");
        assert_eq!(value["auto_run"], false);
    }

    #[test]
    fn first_nonempty_picks_first() {
        assert_eq!(first_nonempty(&["", "  ", "hello"]), "hello");
        assert_eq!(first_nonempty(&["first", "second"]), "first");
        assert_eq!(first_nonempty(&[""]), "");
    }

    #[test]
    fn now_rfc3339_format() {
        let ts = now_rfc3339();
        assert!(ts.contains('T'));
        assert!(ts.ends_with('Z'));
    }
}
