use std::collections::HashMap;
use std::io::Read;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use portable_pty::{CommandBuilder, NativePtySystem, PtySize, PtySystem};
use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter, State};

use crate::models::ChatBlock;
use crate::rpc;
use crate::shell_env::ShellEnvState;

// ─── Constants ───

const PTY_ROWS: u16 = 32;
const PTY_COLS: u16 = 120;
const OUTPUT_MAX_LINES: usize = 500;
const OUTPUT_MAX_CHARS: usize = 64 * 1024;
const FLUSH_INTERVAL: Duration = Duration::from_millis(350);
const CANCEL_GRACE: Duration = Duration::from_secs(2);

// ─── Agent kinds ───

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AgentKind {
    Claude,
    Codex,
    Haft,
}

impl AgentKind {
    fn from_str(s: &str) -> Option<Self> {
        match s.trim().to_lowercase().as_str() {
            "claude" => Some(Self::Claude),
            "codex" => Some(Self::Codex),
            "haft" => Some(Self::Haft),
            _ => None,
        }
    }

    fn as_str(&self) -> &'static str {
        match self {
            Self::Claude => "claude",
            Self::Codex => "codex",
            Self::Haft => "haft",
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
}

struct OutputBuffer {
    lines: Vec<String>,
    total_chars: usize,
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
}

#[tauri::command]
pub fn spawn_agent(
    app: AppHandle,
    manager: State<'_, AgentManagerState>,
    env_state: State<'_, ShellEnvState>,
    request: SpawnAgentRequest,
) -> Result<serde_json::Value, String> {
    let kind = AgentKind::from_str(&request.agent)
        .ok_or_else(|| format!("unsupported agent: {}", request.agent))?;

    let args = build_agent_args(kind, &request.prompt, &request.project_path);
    if args.is_empty() {
        return Err(format!("cannot build args for agent: {}", request.agent));
    }

    let shell_env = env_state
        .0
        .lock()
        .map_err(|e| e.to_string())?
        .clone();

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
        chat_blocks: Mutex::new(Vec::new()),
        block_seq: AtomicU64::new(0),
        started_at: started_at.clone(),
        cancelled: AtomicBool::new(false),
        child: Mutex::new(None),
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

    Ok(serde_json::json!({
        "id": task_id,
        "title": title,
        "agent": kind.as_str(),
        "status": "running",
        "started_at": started_at,
    }))
}

#[tauri::command]
pub fn cancel_agent(
    app: AppHandle,
    manager: State<'_, AgentManagerState>,
    task_id: String,
) -> Result<(), String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let task = mgr
        .tasks
        .get(&task_id)
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

    let _ = app.emit(
        "task.status",
        TaskStatusEvent {
            id: task_id,
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

    let output = task
        .output
        .lock()
        .map(|b| b.snapshot())
        .unwrap_or_default();
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

fn pty_reader_loop(
    app: AppHandle,
    task: Arc<RunningTask>,
    mut reader: Box<dyn Read + Send>,
) {
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
                    let snapshot = task
                        .output
                        .lock()
                        .map(|b| b.snapshot())
                        .unwrap_or_default();
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
    let snapshot = task
        .output
        .lock()
        .map(|b| b.snapshot())
        .unwrap_or_default();
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

fn wait_and_finalize(
    app: AppHandle,
    task: Arc<RunningTask>,
) {
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
        AgentKind::Haft => Vec::new(), // haft agent has no structured streaming yet
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
    session_id: String,
    #[serde(default)]
    message: ClaudeMessage,
    #[serde(default)]
    parent_tool_use_id: String,
    #[serde(default)]
    result: Option<serde_json::Value>,
    #[serde(default)]
    is_error: bool,
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

        "result" => {
            let text = format_json_value(envelope.result.as_ref());
            if text.trim().is_empty() {
                return Vec::new();
            }
            let role = if envelope.is_error {
                "system"
            } else {
                "assistant"
            };
            vec![ChatBlock {
                block_type: "text".into(),
                role: role.into(),
                text,
                is_error: envelope.is_error,
                ..Default::default()
            }]
        }

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
                        let text = first_nonempty(&[
                            &cb.text,
                            &format_json_value(cb.content.as_ref()),
                        ]);
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
    thread_id: String,
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

        "agent_message" | "assistant_message" | "assistant_message_delta"
        | "assistant_response" | "assistant" | "agent_message_delta" | "message"
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
        AgentKind::Haft => vec!["haft".into(), "agent".into(), prompt.into()],
    }
}

// ─── Persistence via haft desktop-rpc ───

fn persist_task_state(task: &RunningTask, status_override: Option<&str>) {
    let status = status_override
        .map(String::from)
        .or_else(|| task.status.lock().ok().map(|s| s.clone()))
        .unwrap_or_else(|| "running".into());

    let output = task
        .output
        .lock()
        .map(|b| b.snapshot())
        .unwrap_or_default();

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
