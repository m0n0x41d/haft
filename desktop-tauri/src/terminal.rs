use std::collections::HashMap;
use std::io::{Read, Write};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{SystemTime, UNIX_EPOCH};

use portable_pty::{CommandBuilder, MasterPty, NativePtySystem, PtySize, PtySystem};
use serde::Serialize;
use tauri::{AppHandle, Emitter, State};

use crate::shell_env::ShellEnvState;

// ─── Constants ───

const DEFAULT_ROWS: u16 = 24;
const DEFAULT_COLS: u16 = 80;

// ─── Event payloads ───
//
// Field names here are the JSON keys the frontend listeners filter on
// (TerminalPanel.tsx subscribes to `terminal.output` and reads `payload.id`).
// Keep `id` — not `session_id` — or the filter silently drops every chunk.

#[derive(Clone, Serialize)]
struct TerminalOutputEvent {
    id: String,
    data: String,
}

#[derive(Clone, Serialize)]
struct TerminalExitEvent {
    id: String,
    exit_code: u32,
}

// ─── Session state ───

struct TerminalSession {
    id: String,
    title: String,
    project_path: String,
    cwd: String,
    shell: String,
    created_at: String,
    updated_at: Mutex<String>,
    master: Mutex<Box<dyn MasterPty + Send>>,
    writer: Mutex<Box<dyn Write + Send>>,
    child: Mutex<Option<Box<dyn portable_pty::Child + Send>>>,
    closed: AtomicBool,
}

/// Frontend-visible view of a terminal session (matches `TerminalSession`
/// in `desktop/frontend/src/lib/api.ts`). Emitted on create and list.
#[derive(Clone, Serialize)]
pub struct TerminalSessionView {
    pub id: String,
    pub title: String,
    pub project_path: String,
    pub cwd: String,
    pub shell: String,
    pub status: String,
    pub created_at: String,
    pub updated_at: String,
}

// ─── Managed state ───

pub struct TerminalManagerState(pub Mutex<TerminalManager>);

pub struct TerminalManager {
    sessions: HashMap<String, Arc<TerminalSession>>,
    seq: u64,
}

impl TerminalManager {
    pub fn new() -> Self {
        Self {
            sessions: HashMap::new(),
            seq: 0,
        }
    }

    fn next_id(&mut self) -> String {
        self.seq += 1;
        let ts = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs();
        format!("term-{ts}-{}", self.seq)
    }
}

// ─── Tauri commands ───

/// Flat argument shape to match the frontend's
/// `invoke("create_terminal_session", { cwd })` call. Tauri binds by
/// parameter name, so wrapping these in a `request: CreateTerminalRequest`
/// struct would force the frontend to send `{ request: { ... } }` — which
/// it doesn't. All three fields are optional; 0/"" fall back to defaults.
#[tauri::command]
pub fn create_terminal_session(
    app: AppHandle,
    manager: State<'_, TerminalManagerState>,
    env_state: State<'_, ShellEnvState>,
    cwd: Option<String>,
    cols: Option<u16>,
    rows: Option<u16>,
) -> Result<TerminalSessionView, String> {
    let cwd = cwd.unwrap_or_default();
    let cols = cols.unwrap_or(DEFAULT_COLS);
    let rows = rows.unwrap_or(DEFAULT_ROWS);

    let shell_env = env_state
        .0
        .lock()
        .map_err(|e| e.to_string())?
        .clone();

    let cols = if cols == 0 { DEFAULT_COLS } else { cols };
    let rows = if rows == 0 { DEFAULT_ROWS } else { rows };

    let pty_system = NativePtySystem::default();
    let pair = pty_system
        .openpty(PtySize {
            rows,
            cols,
            pixel_width: 0,
            pixel_height: 0,
        })
        .map_err(|e| format!("open pty: {e}"))?;

    // Spawn user's login shell.
    let shell = detect_shell_from_env(&shell_env);
    let mut cmd = CommandBuilder::new(&shell);
    cmd.args(["-l"]);

    if !cwd.is_empty() {
        cmd.cwd(&cwd);
    }

    let env_map = crate::shell_env::build_agent_env(
        &shell_env,
        &[("TERM", "xterm-256color")],
    );
    for (k, v) in &env_map {
        cmd.env(k, v);
    }

    let child = pair
        .slave
        .spawn_command(cmd)
        .map_err(|e| format!("spawn shell: {e}"))?;

    // Drop slave — master owns the PTY now.
    drop(pair.slave);

    let reader = pair
        .master
        .try_clone_reader()
        .map_err(|e| format!("clone pty reader: {e}"))?;

    let writer = pair
        .master
        .take_writer()
        .map_err(|e| format!("take pty writer: {e}"))?;

    let mut mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let session_id = mgr.next_id();
    let created_at = now_rfc3339();
    let title = if cwd.is_empty() {
        shell.clone()
    } else {
        std::path::Path::new(&cwd)
            .file_name()
            .and_then(|s| s.to_str())
            .unwrap_or(&shell)
            .to_string()
    };

    let session = Arc::new(TerminalSession {
        id: session_id.clone(),
        title: title.clone(),
        project_path: cwd.clone(),
        cwd: cwd.clone(),
        shell: shell.clone(),
        created_at: created_at.clone(),
        updated_at: Mutex::new(created_at.clone()),
        master: Mutex::new(pair.master),
        writer: Mutex::new(writer),
        child: Mutex::new(Some(child)),
        closed: AtomicBool::new(false),
    });

    mgr.sessions.insert(session_id.clone(), Arc::clone(&session));
    drop(mgr);

    // Reader thread — streams raw PTY output as events.
    let app_reader = app.clone();
    let session_reader = Arc::clone(&session);
    thread::spawn(move || terminal_reader_loop(app_reader, session_reader, reader));

    // Wait thread — detects process exit.
    let app_wait = app;
    let session_wait = Arc::clone(&session);
    thread::spawn(move || terminal_wait_loop(app_wait, session_wait));

    Ok(session_view(&session))
}

#[tauri::command]
pub fn list_terminal_sessions(
    manager: State<'_, TerminalManagerState>,
) -> Result<Vec<TerminalSessionView>, String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    Ok(mgr.sessions.values().map(|s| session_view(s)).collect())
}

/// Frontend sends `id` (the session identifier) — not `session_id`. Matching
/// the Rust parameter name to the JSON key is what Tauri v2 binds against.
#[tauri::command]
pub fn write_terminal_input(
    manager: State<'_, TerminalManagerState>,
    id: String,
    data: String,
) -> Result<(), String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let session = mgr
        .sessions
        .get(&id)
        .cloned()
        .ok_or_else(|| format!("terminal session not found: {id}"))?;
    drop(mgr);

    if session.closed.load(Ordering::SeqCst) {
        return Err(format!("terminal session closed: {id}"));
    }

    let mut writer = session.writer.lock().map_err(|e| e.to_string())?;
    writer
        .write_all(data.as_bytes())
        .map_err(|e| format!("write to pty: {e}"))?;

    Ok(())
}

#[tauri::command]
pub fn resize_terminal_session(
    manager: State<'_, TerminalManagerState>,
    id: String,
    cols: u16,
    rows: u16,
) -> Result<(), String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let session = mgr
        .sessions
        .get(&id)
        .cloned()
        .ok_or_else(|| format!("terminal session not found: {id}"))?;
    drop(mgr);

    if session.closed.load(Ordering::SeqCst) {
        return Err(format!("terminal session closed: {id}"));
    }

    let master = session.master.lock().map_err(|e| e.to_string())?;
    master
        .resize(PtySize {
            rows,
            cols,
            pixel_width: 0,
            pixel_height: 0,
        })
        .map_err(|e| format!("resize pty: {e}"))?;

    Ok(())
}

#[tauri::command]
pub fn close_terminal_session(
    app: AppHandle,
    manager: State<'_, TerminalManagerState>,
    id: String,
) -> Result<(), String> {
    let mgr = manager.0.lock().map_err(|e| e.to_string())?;
    let session = mgr
        .sessions
        .get(&id)
        .cloned()
        .ok_or_else(|| format!("terminal session not found: {id}"))?;
    drop(mgr);

    kill_session(&session);

    let _ = app.emit(
        "terminal.exit",
        TerminalExitEvent {
            id: id.clone(),
            exit_code: 0,
        },
    );

    // Remove from manager.
    let mut mgr = manager.0.lock().map_err(|e| e.to_string())?;
    mgr.sessions.remove(&id);

    Ok(())
}

fn session_view(session: &TerminalSession) -> TerminalSessionView {
    let status = if session.closed.load(Ordering::SeqCst) {
        "closed".to_string()
    } else {
        "running".to_string()
    };
    let updated_at = session
        .updated_at
        .lock()
        .map(|g| g.clone())
        .unwrap_or_else(|_| session.created_at.clone());
    TerminalSessionView {
        id: session.id.clone(),
        title: session.title.clone(),
        project_path: session.project_path.clone(),
        cwd: session.cwd.clone(),
        shell: session.shell.clone(),
        status,
        created_at: session.created_at.clone(),
        updated_at,
    }
}

fn now_rfc3339() -> String {
    let ts = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    // Approximate RFC3339: seconds-since-epoch encoded as UTC. Frontend only
    // needs a lexicographically stable string, not a parseable date.
    format!("t{ts}")
}

// ─── Internal ───

fn kill_session(session: &TerminalSession) {
    session.closed.store(true, Ordering::SeqCst);

    let mut child_guard = match session.child.lock() {
        Ok(g) => g,
        Err(_) => return,
    };

    if let Some(ref mut child) = *child_guard {
        child.kill().ok();
        // Wait briefly for clean exit.
        let start = std::time::Instant::now();
        while start.elapsed() < std::time::Duration::from_secs(1) {
            match child.try_wait() {
                Ok(Some(_)) => break,
                Ok(None) => thread::sleep(std::time::Duration::from_millis(25)),
                Err(_) => break,
            }
        }
    }

    *child_guard = None;
}

fn terminal_reader_loop(
    app: AppHandle,
    session: Arc<TerminalSession>,
    mut reader: Box<dyn Read + Send>,
) {
    let mut buf = [0u8; 4096];

    loop {
        if session.closed.load(Ordering::SeqCst) {
            break;
        }

        match reader.read(&mut buf) {
            Ok(0) => break,
            Ok(n) => {
                // Send raw bytes — frontend terminal emulator handles ANSI.
                let data = String::from_utf8_lossy(&buf[..n]).into_owned();
                let _ = app.emit(
                    "terminal.output",
                    TerminalOutputEvent {
                        id: session.id.clone(),
                        data,
                    },
                );
            }
            Err(_) => break,
        }
    }
}

fn terminal_wait_loop(
    app: AppHandle,
    session: Arc<TerminalSession>,
) {
    loop {
        if session.closed.load(Ordering::SeqCst) {
            return;
        }

        let mut child_guard = match session.child.lock() {
            Ok(g) => g,
            Err(_) => return,
        };

        if let Some(ref mut child) = *child_guard {
            match child.try_wait() {
                Ok(Some(status)) => {
                    let code = status.exit_code();
                    drop(child_guard);

                    session.closed.store(true, Ordering::SeqCst);

                    let _ = app.emit(
                        "terminal.exit",
                        TerminalExitEvent {
                            id: session.id.clone(),
                            exit_code: code,
                        },
                    );
                    return;
                }
                Ok(None) => {
                    drop(child_guard);
                    thread::sleep(std::time::Duration::from_millis(100));
                }
                Err(_) => {
                    drop(child_guard);
                    return;
                }
            }
        } else {
            drop(child_guard);
            return;
        }
    }
}

fn detect_shell_from_env(env: &[(String, String)]) -> String {
    // Prefer SHELL from resolved env.
    for (k, v) in env {
        if k == "SHELL" && !v.is_empty() {
            return v.clone();
        }
    }
    // Fallback.
    std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".into())
}
