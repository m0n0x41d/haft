use serde::Deserialize;
use std::io::Write;
use std::path::PathBuf;
use std::process::{Command, Stdio};
use std::time::{Duration, Instant};

const RPC_TIMEOUT: Duration = Duration::from_secs(30);

/// Envelope returned by `haft desktop-rpc` on stdout.
#[derive(Deserialize)]
struct RpcEnvelope {
    ok: bool,
    data: Option<serde_json::Value>,
    error: Option<String>,
}

/// Resolve the `haft` binary: (1) same directory as the Tauri executable, (2) PATH.
fn resolve_haft_binary() -> Result<PathBuf, String> {
    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            let candidate = dir.join("haft");
            if candidate.is_file() {
                return Ok(candidate);
            }
        }
    }
    // Fall back to PATH — Command::new resolves it at spawn time.
    Ok(PathBuf::from("haft"))
}

/// Spawn `haft desktop-rpc <cmd>`, pipe JSON stdin, read JSON stdout.
///
/// - 30 s timeout — kills subprocess on exceed.
/// - Non-zero exit → error with stderr content.
/// - RPC envelope `{"ok": false, "error": "..."}` → error.
pub fn call_rpc(
    cmd: &str,
    input: Option<serde_json::Value>,
    project_root: Option<&str>,
) -> Result<serde_json::Value, String> {
    let binary = resolve_haft_binary()?;

    let mut builder = Command::new(&binary);
    builder
        .args(["desktop-rpc", cmd])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    if let Some(root) = project_root {
        builder.env("HAFT_PROJECT_ROOT", root);
    }

    let mut child = builder
        .spawn()
        .map_err(|e| format!("spawn haft desktop-rpc {cmd}: {e}"))?;

    // Pipe input to stdin, then close it.
    if let Some(ref payload) = input {
        if let Some(mut stdin) = child.stdin.take() {
            let data =
                serde_json::to_vec(payload).map_err(|e| format!("serialize input: {e}"))?;
            stdin
                .write_all(&data)
                .map_err(|e| format!("write stdin: {e}"))?;
        }
    }
    drop(child.stdin.take());

    // Read stdout/stderr in background threads to avoid pipe deadlock.
    let stdout_pipe = child.stdout.take();
    let stderr_pipe = child.stderr.take();

    let stdout_thread = std::thread::spawn(move || {
        let mut buf = Vec::new();
        if let Some(mut r) = stdout_pipe {
            std::io::Read::read_to_end(&mut r, &mut buf).ok();
        }
        buf
    });

    let stderr_thread = std::thread::spawn(move || {
        let mut buf = Vec::new();
        if let Some(mut r) = stderr_pipe {
            std::io::Read::read_to_end(&mut r, &mut buf).ok();
        }
        buf
    });

    // Poll for exit with timeout.
    let start = Instant::now();
    let status = loop {
        match child.try_wait() {
            Ok(Some(s)) => break s,
            Ok(None) => {
                if start.elapsed() >= RPC_TIMEOUT {
                    child.kill().ok();
                    return Err(format!(
                        "haft desktop-rpc {cmd} timed out after {}s",
                        RPC_TIMEOUT.as_secs()
                    ));
                }
                std::thread::sleep(Duration::from_millis(25));
            }
            Err(e) => return Err(format!("wait for haft desktop-rpc {cmd}: {e}")),
        }
    };

    let stdout = stdout_thread.join().unwrap_or_default();
    let stderr = stderr_thread.join().unwrap_or_default();

    if !status.success() {
        let msg = String::from_utf8_lossy(&stderr);
        return Err(format!(
            "haft desktop-rpc {cmd} failed (exit {}): {msg}",
            status.code().unwrap_or(-1)
        ));
    }

    let envelope: RpcEnvelope = serde_json::from_slice(&stdout)
        .map_err(|e| format!("parse rpc response for {cmd}: {e}"))?;

    if !envelope.ok {
        return Err(envelope.error.unwrap_or_else(|| "unknown rpc error".into()));
    }

    Ok(envelope.data.unwrap_or(serde_json::Value::Null))
}
