# Open-Sleigh Troubleshooting

Use `mix open_sleigh.doctor` before runtime, then `mix open_sleigh.status`
and the JSONL runtime log while runtime is active or after `--once`.

## Doctor Fails

Command:

```sh
mix open_sleigh.doctor --path sleigh.md
```

Common failures:

- `linear.api_key`: set `LINEAR_API_KEY`.
- `linear.project`: set `tracker.project_slug` or `tracker.team`.
- `linear.active_states`: configure at least one active state.
- `hooks.repo_url`: set `REPO_URL` when hooks use `$REPO_URL`.
- `hooks.repo_url_format`: use an SSH, HTTPS, file, or local git remote path.
- `hooks.git`: install `git` or fix `PATH`.
- `workspace.root`: choose a creatable and writable directory.
- `workspace.cleanup_policy`: use `keep`.
- `hooks.failure_policy`: use `blocking`, `warning`, or `ignore`.
- `external_publication.branch_regex`: fix the regex syntax.
- `config.schema`: read `field_path`, `expected`, `actual`, and `hint` from
  `mix open_sleigh.doctor --json`.

## Gate Report Fails

Command:

```sh
mix open_sleigh.gate_report --json
```

If a row fails, inspect `gate`, `expected`, `actual`, `cl`, and `rationale`.
The deterministic baseline should pass before a live judge comparison is
useful.

## Runtime Starts But Does Nothing

Check:

```sh
mix open_sleigh.status --path <status-path>
```

Look at:

- `stale`: if true, the runtime is probably stopped or not writing status.
- `claimed`: if zero, no active ticket was accepted by the poller.
- `failures`: if non-zero, inspect `recent_failures`.
- `pending_human`: if non-zero, approval is required in the tracker.

If `engine.status_http.enabled: true`, the same redacted snapshot is available
from `/api/v1/state` and the browser dashboard at `/dashboard`.

## Dispatch Failed

Tracker comments contain a marker like:

```text
open-sleigh:dispatch-failed:<reason>
```

Common reasons:

- `no_upstream_frame`: create or link a valid upstream Haft ProblemCard.
- `upstream_self_authored`: replace the ProblemCard with one authored outside
  Open-Sleigh.
- `unknown_phase` or `unknown_prompt`: fix `sleigh.md` workflow/prompt config.

After fixing the ticket or config, leave the ticket active and let the next
poll retry.

## Session Failed

Tracker comments contain a marker like:

```text
open-sleigh:session-failed:<phase>:<reason>
```

Common reasons:

- `thread_start_failed`: check app-server startup and local login state.
- `agent_command_not_found`: fix the configured agent command or `PATH`.
- `agent_launch_failed`: check the local runtime can launch from the same shell.
- `handshake_timeout`: check the app-server is not waiting for input.
- `stalled` or `timed_out`: inspect workspace state and retry timing.
- `haft_unavailable`: start or fix `haft serve`.
- `hook_failed` or `hook_timeout`: inspect `hooks.after_create` or
  `hooks.before_run`.

Retry state is visible in:

```sh
mix open_sleigh.status --path <status-path> --json
```

## Hook Failures

Hook policies:

- `blocking`: fail the session and retry while the ticket remains active.
- `warning`: record the failure and continue.
- `ignore`: continue without recording an operator warning.

Recommended production defaults:

```yaml
hooks:
  failure_policy:
    after_create: blocking
    before_run: blocking
    after_run: warning
```

## Human Gate Pending

If `pending_human` is non-zero, inspect the tracker comments for the approval
request. The configured approver must comment with the expected approval or
rejection command. Unauthorized approvers are rejected with a tracker comment.

## Runtime Log

The runtime log is JSONL at `engine.log_path`.

```sh
tail -n 50 ~/.open-sleigh/runtime.jsonl
```

Each line contains:

- `event_id`
- `event`
- `at`
- `metadata`
- `data`

Use `event_id` values when recording first-run evidence.
