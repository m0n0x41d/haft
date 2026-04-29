use std::collections::HashMap;
use std::process::Command;
use std::sync::Mutex;
use std::time::Duration;

/// Managed Tauri state holding the resolved user shell environment.
/// Resolved once at app startup so that PTY-spawned agents get the full PATH
/// even when the app is launched from Spotlight/Finder (which gives a minimal env).
pub struct ShellEnvState(pub Mutex<Vec<(String, String)>>);

/// Resolve the user's login shell environment.
///
/// Runs `<shell> -l -c env` with a 3s timeout, parses `KEY=VALUE` lines.
/// Falls back to `std::env::vars()` if shell detection or capture fails.
pub fn resolve_user_shell_env() -> Vec<(String, String)> {
    let shell = match detect_shell() {
        Some(s) => s,
        None => return std::env::vars().collect(),
    };

    let output = Command::new(&shell)
        .args(["-l", "-c", "env"])
        .current_dir(std::env::temp_dir())
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::null())
        .spawn()
        .and_then(|child| wait_with_timeout(child, Duration::from_secs(3)));

    let stdout = match output {
        Ok(out) => out,
        Err(_) => return std::env::vars().collect(),
    };

    let text = String::from_utf8_lossy(&stdout);
    let env: Vec<(String, String)> = text.lines().filter_map(parse_env_line).collect();

    if env.is_empty() {
        return std::env::vars().collect();
    }

    env
}

/// Build a flat `Vec<(String, String)>` suitable for `CommandBuilder::env`.
/// Merges the resolved shell env with extra per-process overrides.
pub fn build_agent_env(
    base: &[(String, String)],
    extras: &[(&str, &str)],
) -> HashMap<String, String> {
    let mut map: HashMap<String, String> =
        base.iter().map(|(k, v)| (k.clone(), v.clone())).collect();

    for (k, v) in extras {
        map.insert(k.to_string(), v.to_string());
    }

    map
}

/// Detect the user's login shell: $SHELL → zsh → bash → sh.
fn detect_shell() -> Option<String> {
    if let Ok(shell) = std::env::var("SHELL") {
        let shell = shell.trim().to_string();
        if !shell.is_empty() && std::path::Path::new(&shell).exists() {
            return Some(shell);
        }
    }

    for candidate in ["zsh", "bash", "sh"] {
        if let Ok(path) = which(candidate) {
            return Some(path);
        }
    }

    None
}

/// Minimal `which` — resolve a binary name via PATH.
fn which(name: &str) -> Result<String, ()> {
    let path_var = std::env::var("PATH").unwrap_or_default();
    for dir in std::env::split_paths(&path_var) {
        let candidate = dir.join(name);
        if candidate.is_file() {
            return Ok(candidate.to_string_lossy().into_owned());
        }
    }
    Err(())
}

/// Wait for a child process with a timeout, returning stdout bytes.
fn wait_with_timeout(
    mut child: std::process::Child,
    timeout: Duration,
) -> std::io::Result<Vec<u8>> {
    let start = std::time::Instant::now();

    loop {
        match child.try_wait()? {
            Some(status) => {
                if !status.success() {
                    return Err(std::io::Error::new(
                        std::io::ErrorKind::Other,
                        format!("shell exited with {status}"),
                    ));
                }

                let mut stdout = Vec::new();
                if let Some(mut pipe) = child.stdout.take() {
                    std::io::Read::read_to_end(&mut pipe, &mut stdout)?;
                }

                return Ok(stdout);
            }
            None => {
                if start.elapsed() >= timeout {
                    child.kill().ok();
                    return Err(std::io::Error::new(
                        std::io::ErrorKind::TimedOut,
                        "shell env capture timed out",
                    ));
                }

                std::thread::sleep(Duration::from_millis(25));
            }
        }
    }
}

/// Parse a single `KEY=VALUE` line from env output.
/// Handles values containing `=` correctly.
fn parse_env_line(line: &str) -> Option<(String, String)> {
    let eq = line.find('=')?;
    let key = &line[..eq];
    let value = &line[eq + 1..];

    if key.is_empty() {
        return None;
    }

    Some((key.to_string(), value.to_string()))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_env_line_basic() {
        assert_eq!(
            parse_env_line("HOME=/Users/test"),
            Some(("HOME".into(), "/Users/test".into()))
        );
    }

    #[test]
    fn parse_env_line_value_with_equals() {
        assert_eq!(
            parse_env_line("FOO=bar=baz"),
            Some(("FOO".into(), "bar=baz".into()))
        );
    }

    #[test]
    fn parse_env_line_empty_key() {
        assert_eq!(parse_env_line("=value"), None);
    }

    #[test]
    fn parse_env_line_no_equals() {
        assert_eq!(parse_env_line("no_equals_here"), None);
    }

    #[test]
    fn resolve_returns_nonempty() {
        let env = resolve_user_shell_env();
        assert!(!env.is_empty(), "resolved env should not be empty");
        assert!(
            env.iter().any(|(k, _)| k == "PATH"),
            "resolved env should contain PATH"
        );
    }

    #[test]
    fn build_agent_env_merges() {
        let base = vec![
            ("PATH".into(), "/usr/bin".into()),
            ("HOME".into(), "/home/test".into()),
        ];
        let extras = [
            ("TERM", "xterm-256color"),
            ("HAFT_PROJECT_ROOT", "/tmp/proj"),
        ];
        let merged = build_agent_env(&base, &extras);

        assert_eq!(merged.get("PATH").map(String::as_str), Some("/usr/bin"));
        assert_eq!(
            merged.get("TERM").map(String::as_str),
            Some("xterm-256color")
        );
        assert_eq!(
            merged.get("HAFT_PROJECT_ROOT").map(String::as_str),
            Some("/tmp/proj")
        );
    }
}
