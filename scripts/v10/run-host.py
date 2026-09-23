#!/usr/bin/env python3
"""Run one real Codex session in an explicitly owned B1 sandbox.

This is qualification tooling, not an agent runtime shipped by haft10. It keeps
the supplied prompt, exact invocation and raw output; it never labels an attempt
as successful merely because the process exits zero.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import time
import tomllib


def digest(path):
    return "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest()


def prepare_project_trust(sandbox, project):
    """Create only the isolated trust table; never inherit a live config layer.

    Codex 0.156.1 decides whether to load project config before applying CLI
    trust overrides. The persisted sandbox trust table must therefore be read.
    """
    directory = sandbox / "codex"
    if directory.is_symlink() or not directory.resolve().is_relative_to(sandbox):
        raise SystemExit("Sandbox Codex directory must not escape through a symlink")
    directory.mkdir(parents=True, exist_ok=True)
    config_path = directory / "config.toml"
    expected = {"projects": {str(project): {"trust_level": "trusted"}}}
    if config_path.is_symlink():
        raise SystemExit("Refusing a symlinked sandbox Codex config")
    if config_path.exists():
        if not config_path.is_file():
            raise SystemExit("Sandbox Codex config must be a regular file")
        try:
            actual = tomllib.loads(config_path.read_text())
        except (ValueError, OSError) as error:
            raise SystemExit("Invalid existing sandbox Codex config") from error
        if actual != expected:
            raise SystemExit("Existing sandbox Codex config differs from the exact harness trust table")
    else:
        body = '[projects.' + json.dumps(str(project)) + ']\ntrust_level = "trusted"\n'
        descriptor = os.open(config_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "w") as target:
            target.write(body)
    return config_path


def configured_haft_tool(project):
    """An unrelated baseline must not inherit a phantom Haft server or policy."""
    path = project / ".codex/config.toml"
    if not path.exists():
        return False
    if not path.is_file() or not path.resolve().is_relative_to(project):
        raise SystemExit("Project Codex config must be a file inside the project")
    try:
        config = tomllib.loads(path.read_text())
    except (ValueError, OSError) as error:
        raise SystemExit("Invalid project Codex config") from error
    servers = config.get("mcp_servers", {})
    server = servers.get("haft10", {}) if isinstance(servers, dict) else {}
    if not isinstance(server, dict):
        return False
    command, args = server.get("command"), server.get("args")
    return (isinstance(command, str) and bool(command.strip())
            and isinstance(args, list) and all(isinstance(arg, str) for arg in args)
            and server.get("enabled", True) is not False)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--sandbox", type=Path, required=True)
    parser.add_argument("--project", type=Path, required=True)
    parser.add_argument("--prompt", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--label", required=True)
    parser.add_argument("--timeout", type=int, default=1200)
    args = parser.parse_args()
    sandbox, project = args.sandbox.resolve(), args.project.resolve()
    if not (sandbox / ".haft-v10-owned").is_file():
        raise SystemExit("Sandbox must have an explicit .haft-v10-owned marker")
    if not project.is_relative_to(sandbox) or not project.is_dir():
        raise SystemExit("Project must be a directory inside the owned sandbox")
    if not 1 <= args.timeout <= 1200:
        raise SystemExit("Host attempt timeout must be within 1..1200 seconds")
    if args.output.exists():
        raise SystemExit("Refusing to overwrite a previous attempt output")
    args.output.mkdir(parents=True)
    for name in ["home", "codex", "haft", "tmp", "xdg/config", "xdg/cache",
                 "xdg/data", "xdg/state", "go-build", "go-mod", "gopath", "bin"]:
        (sandbox / name).mkdir(parents=True, exist_ok=True)
    sandbox_config = prepare_project_trust(sandbox, project)
    live = Path(os.environ.get("CODEX_HOME", str(Path.home() / ".codex")))
    config_path = live / "config.toml"
    config = tomllib.loads(config_path.read_text()) if config_path.exists() else {}
    model = config.get("model", "gpt-6-astra")
    effort = config.get("model_reasoning_effort", "max")
    credential = sandbox / "codex/auth.json"
    if credential.exists():
        raise SystemExit("Refusing to replace an existing sandbox credential")
    inherited = ("PATH", "LANG", "LC_ALL", "LC_CTYPE", "TERM", "TZ",
                 "SSL_CERT_FILE", "SSL_CERT_DIR", "HTTPS_PROXY", "HTTP_PROXY",
                 "NO_PROXY", "REQUESTS_CA_BUNDLE")
    env = {key: os.environ[key] for key in inherited if key in os.environ}
    env.update(HOME=str(sandbox / "home"), CODEX_HOME=str(sandbox / "codex"),
               HAFT_HOME=str(sandbox / "haft"), TMPDIR=str(sandbox / "tmp"),
               XDG_CONFIG_HOME=str(sandbox / "xdg/config"),
               XDG_CACHE_HOME=str(sandbox / "xdg/cache"),
               XDG_DATA_HOME=str(sandbox / "xdg/data"),
               XDG_STATE_HOME=str(sandbox / "xdg/state"),
               GOCACHE=str(sandbox / "go-build"), GOMODCACHE=str(sandbox / "go-mod"),
               GOPATH=str(sandbox / "gopath"), GOWORK="off", GOTOOLCHAIN="local",
               OPENSPEC_TELEMETRY="0", DO_NOT_TRACK="1",
               PATH=str(sandbox / "bin") + os.pathsep + env.get("PATH", ""))
    executable = shutil.which("codex")
    if not executable:
        raise SystemExit("Codex executable unavailable")
    prompt = args.prompt.read_text()
    (args.output / "prompt.txt").write_text(prompt)
    approve_haft = configured_haft_tool(project)
    command = [executable, "--no-daemon", "-a", "never", "exec",
               "--ignore-rules", "--sandbox", "workspace-write",
               "--json", "-m", model, "-c", "model_reasoning_effort=" + json.dumps(effort)]
    if approve_haft:
        command += ["-c", 'mcp_servers.haft10.tools.haft.approval_mode="approve"']
    command += ["--skip-git-repo-check", "-C", str(project), prompt]
    meta = {"format": "haft.host-attempt/1", "label": args.label,
            "kind": "actual_codex_session", "command": command,
            "sandbox": str(sandbox), "project": str(project),
            "model": model, "reasoning_effort": effort, "timeout_seconds": args.timeout,
            "prompt_digest": digest(args.output / "prompt.txt"),
            "harness_digest": digest(Path(__file__)),
            "sandbox_config_digest": digest(sandbox_config),
            "sandbox_config_policy": "Only generated exact project trust; no live config layer",
            "mcp_approval_policy": "Explicit harness-only approval for mcp_servers.haft10.tools.haft; application semantics remain unchanged" if approve_haft else "none; no configured haft10 stdio tool",
            "codex_digest": digest(Path(executable).resolve()),
            "started": datetime.datetime.now(datetime.timezone.utc).isoformat()}
    metadata = args.output / "attempt.json"
    started = time.monotonic()
    process = None
    def interrupted(signum, frame):
        raise KeyboardInterrupt("Host harness interrupted")
    signal.signal(signal.SIGTERM, interrupted)
    try:
        # Open with private permissions before copying credential bytes.
        with credential.open("xb") as target:
            os.chmod(credential, 0o600)
            with (live / "auth.json").open("rb") as original:
                shutil.copyfileobj(original, target)
        with (args.output / "stdout.jsonl").open("wb") as stdout, (args.output / "stderr.log").open("wb") as stderr:
            process = subprocess.Popen(command, cwd=project, env=env, stdin=subprocess.DEVNULL,
                                       stdout=stdout, stderr=stderr, start_new_session=True)
            meta["pid"] = process.pid
            metadata.write_text(json.dumps(meta, indent=2) + "\n")
            try:
                meta["exit_code"] = process.wait(timeout=args.timeout)
                meta["process_outcome"] = "exited"
            except subprocess.TimeoutExpired:
                meta["process_outcome"] = "timeout"
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()
                meta["exit_code"] = process.returncode
    finally:
        if process is not None and process.poll() is None:
            meta["process_outcome"] = "interrupted"
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
            meta["exit_code"] = process.returncode
        credential.unlink(missing_ok=True)
        meta["ended"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        meta["elapsed_seconds"] = time.monotonic() - started
        meta["credential_copy_removed"] = not credential.exists()
        metadata.write_text(json.dumps(meta, indent=2) + "\n")
    print(json.dumps({key: value for key, value in meta.items() if key != "command"}))


if __name__ == "__main__":
    main()
