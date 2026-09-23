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
    command = [executable, "--no-daemon", "-a", "never", "exec",
               "--ignore-user-config", "--ignore-rules", "--sandbox", "workspace-write",
               "--json", "-m", model, "-c", "model_reasoning_effort=" + json.dumps(effort),
               "-c", "projects." + json.dumps(str(project)) + '.trust_level="trusted"',
               "--skip-git-repo-check", "-C", str(project), prompt]
    meta = {"format": "haft.host-attempt/1", "label": args.label,
            "kind": "actual_codex_session", "command": command,
            "sandbox": str(sandbox), "project": str(project),
            "model": model, "reasoning_effort": effort, "timeout_seconds": args.timeout,
            "prompt_digest": digest(args.output / "prompt.txt"),
            "harness_digest": digest(Path(__file__)),
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
