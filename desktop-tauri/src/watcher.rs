use std::path::{Path, PathBuf};
use std::sync::mpsc;
use std::sync::Mutex;
use std::thread;
use std::time::{Duration, Instant};

use notify::{Config, RecommendedWatcher, RecursiveMode, Watcher};
use serde::Serialize;
use tauri::{AppHandle, Emitter, State};

const DEBOUNCE: Duration = Duration::from_millis(500);
const POLL_INTERVAL: Duration = Duration::from_millis(100);

// ─── Event payloads ───

#[derive(Clone, Serialize)]
pub struct StateChangedPayload {
    pub paths: Vec<String>,
}

// ─── Managed state ───

/// Wraps the active file watcher. `None` when no project is loaded.
pub struct WatcherState(pub Mutex<Option<HaftWatcher>>);

// ─── Watcher ───

pub struct HaftWatcher {
    _watcher: RecommendedWatcher,
    stop_tx: mpsc::Sender<()>,
}

impl HaftWatcher {
    /// Start watching `.haft/` in `project_root` and the DB directory for WAL changes.
    /// Spawns a background thread that debounces events and emits to Tauri.
    pub fn start(app: AppHandle, project_root: &str, db_path: &str) -> Result<Self, String> {
        let (event_tx, event_rx) = mpsc::channel();
        let (stop_tx, stop_rx) = mpsc::channel();

        let mut watcher =
            RecommendedWatcher::new(event_tx, Config::default()).map_err(|e| format!("watcher init: {e}"))?;

        // Watch .haft/ directory recursively for governance artifact changes.
        let haft_dir = PathBuf::from(project_root).join(".haft");
        if haft_dir.is_dir() {
            watcher
                .watch(&haft_dir, RecursiveMode::Recursive)
                .map_err(|e| format!("watch .haft/: {e}"))?;
        }

        // Watch DB parent directory (non-recursive) for WAL file changes.
        if let Some(db_dir) = Path::new(db_path).parent() {
            if db_dir.is_dir() {
                watcher
                    .watch(db_dir, RecursiveMode::NonRecursive)
                    .map_err(|e| format!("watch db dir: {e}"))?;
            }
        }

        thread::spawn(move || event_loop(app, event_rx, stop_rx));

        Ok(Self {
            _watcher: watcher,
            stop_tx,
        })
    }
}

impl Drop for HaftWatcher {
    fn drop(&mut self) {
        let _ = self.stop_tx.send(());
    }
}

// ─── Event loop (runs in background thread) ───

fn event_loop(
    app: AppHandle,
    event_rx: mpsc::Receiver<Result<notify::Event, notify::Error>>,
    stop_rx: mpsc::Receiver<()>,
) {
    let mut last_state_emit = Instant::now() - DEBOUNCE;
    let mut last_tasks_emit = Instant::now() - DEBOUNCE;
    let mut pending_paths: Vec<String> = Vec::new();
    let mut tasks_pending = false;

    loop {
        if stop_rx.try_recv().is_ok() {
            break;
        }

        match event_rx.recv_timeout(POLL_INTERVAL) {
            Ok(Ok(event)) => {
                for path in &event.paths {
                    if is_wal_path(path) {
                        tasks_pending = true;
                    } else if is_governance_path(path) {
                        pending_paths.push(path.to_string_lossy().into_owned());
                    }
                }
            }
            Ok(Err(_)) => {}                                    // watcher error — skip
            Err(mpsc::RecvTimeoutError::Timeout) => {}          // no events — check timers
            Err(mpsc::RecvTimeoutError::Disconnected) => break, // watcher dropped
        }

        let now = Instant::now();

        if !pending_paths.is_empty() && now.duration_since(last_state_emit) >= DEBOUNCE {
            let payload = StateChangedPayload {
                paths: std::mem::take(&mut pending_paths),
            };
            let _ = app.emit("state-changed", &payload);
            let _ = app.emit("governance-updated", &payload);
            last_state_emit = now;
        }

        if tasks_pending && now.duration_since(last_tasks_emit) >= DEBOUNCE {
            let _ = app.emit("tasks-updated", ());
            tasks_pending = false;
            last_tasks_emit = now;
        }
    }
}

// ─── Path classification ───

fn is_governance_path(path: &Path) -> bool {
    let s = path.to_string_lossy();
    s.contains(".haft/decisions/") || s.contains(".haft/problems/") || s.contains(".haft/notes/")
}

fn is_wal_path(path: &Path) -> bool {
    path.file_name()
        .is_some_and(|n| n == "haft.db-wal")
}

// ─── Tauri commands ───

/// (Re)start the file watcher for a project. Stops any existing watcher first.
/// Frontend calls this on project load and after project switch.
#[tauri::command]
pub fn start_watcher(
    app: AppHandle,
    watcher_state: State<'_, WatcherState>,
    project_root: String,
) -> Result<(), String> {
    let db_path = crate::resolve_db_path()
        .ok_or_else(|| "no project DB found".to_string())?;
    let mut guard = watcher_state.0.lock().map_err(|e| e.to_string())?;
    // Drop existing watcher — stops thread, releases file handles.
    *guard = None;
    *guard = Some(HaftWatcher::start(app, &project_root, &db_path)?);
    Ok(())
}

/// Stop the file watcher. Frontend calls this on app shutdown or before project unload.
#[tauri::command]
pub fn stop_watcher(watcher_state: State<'_, WatcherState>) -> Result<(), String> {
    let mut guard = watcher_state.0.lock().map_err(|e| e.to_string())?;
    *guard = None;
    Ok(())
}

// ─── Tests ───

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;

    #[test]
    fn classify_governance_paths() {
        assert!(is_governance_path(Path::new("/home/user/project/.haft/decisions/dec-001.md")));
        assert!(is_governance_path(Path::new("/home/user/project/.haft/problems/prob-001.md")));
        assert!(is_governance_path(Path::new("/home/user/project/.haft/notes/note-001.md")));
        assert!(!is_governance_path(Path::new("/home/user/project/.haft/workflow.md")));
        assert!(!is_governance_path(Path::new("/home/user/project/src/main.rs")));
    }

    #[test]
    fn classify_wal_paths() {
        assert!(is_wal_path(Path::new("/home/user/.haft/haft.db-wal")));
        assert!(!is_wal_path(Path::new("/home/user/.haft/haft.db")));
        assert!(!is_wal_path(Path::new("/home/user/.haft/haft.db-shm")));
    }
}
