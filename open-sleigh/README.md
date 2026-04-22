# Open-Sleigh

Open-Sleigh is a local engineering agent runner for project work:

1. poll active Linear tickets,
2. create a per-ticket workspace,
3. run Codex through `codex app-server`,
4. write phase outcomes to Haft,
5. evaluate calibrated semantic gates,
6. request human approval before configured external publication,
7. transition completed tickets in the tracker.

The current product surface is CLI-first and runs on the operator's machine.

## Requirements

- Elixir 1.18
- `codex` with `codex app-server`
- `haft` with `haft serve`
- `git`
- Linear API key
- A Git repository URL that `git clone` can read

## Configure

Start from the example:

```sh
cp sleigh.md.example sleigh.md
```

Set the real environment:

```sh
export LINEAR_API_KEY=...
export REPO_URL=git@github.com:org/repo.git
```

`REPO_URL` is not GitHub-specific. It is passed to `git clone`, so SSH/HTTPS
URLs for GitHub, GitLab, self-hosted Git, and local Git remotes work if your
machine can authenticate.

In Linear, active tickets must include a ProblemCard reference in the
description by default:

```text
problem_card_ref: haft-pc-123
```

The marker can be changed in `sleigh.md`:

```yaml
tracker:
  problem_card_ref_marker: problem_card_ref
```

## Preflight

Before starting the engine, run:

```sh
mix deps.get
mix open_sleigh.doctor --path sleigh.md
```

The doctor checks `sleigh.md`, required environment variables, external
commands, repository URL shape, workspace root write access, publication
branch regex, hooks, and known MVP limitations.

Machine-readable output is available for scripts:

```sh
mix open_sleigh.doctor --path sleigh.md --json
```

Semantic-gate calibration can be checked without live services:

```sh
mix open_sleigh.gate_report
mix open_sleigh.gate_report --json
```

For the first real canary, use [docs/FIRST_RUN.md](docs/FIRST_RUN.md) and
[docs/TICKET_TEMPLATES.md](docs/TICKET_TEMPLATES.md). For failure diagnosis,
use [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md).

## Run

Run one polling pass:

```sh
mix open_sleigh.start --path sleigh.md --once
```

Run continuously:

```sh
mix open_sleigh.start --path sleigh.md
```

Read the latest status snapshot from another shell:

```sh
mix open_sleigh.status
mix open_sleigh.status --json
```

Text status includes snapshot age/staleness, claimed/running counts, pending
human gate details, retries, failure count, and recent failure summaries.
`--json` returns the full stored snapshot for scripts.

To build an operator CLI wrapper:

```sh
mix escript.build
./open_sleigh doctor --path sleigh.md
./open_sleigh gate_report
```

To expose the read-only local status API and dashboard, set
`engine.status_http.enabled: true` in `sleigh.md`, then open:

```text
http://127.0.0.1:4767/dashboard
http://127.0.0.1:4767/api/v1/state
```

For a local smoke test without external services:

```sh
mix open_sleigh.start --path sleigh.md.example --mock --once
mix open_sleigh.canary --duration 0s
```

## Runtime Flow

- `tracker.active_states` selects Linear tickets to claim.
- `hooks.after_create` prepares a fresh workspace, typically with `git clone`.
- `hooks.before_run` refreshes the workspace before each phase.
- `after_create` and `before_run` hook failures block the session and surface
  through retry/status/tracker failure reporting.
- `workspace.cleanup_policy: keep` is the only supported cleanup policy for
  now; Open-Sleigh does not delete local workspaces without an explicit future
  policy change.
- `engine.status_path` receives the latest runtime snapshot for
  `mix open_sleigh.status`, including pending human gates and recent
  dispatch/session failures.
- `engine.log_path` receives JSONL lifecycle events with event ids for
  troubleshooting and run evidence.
- `engine.status_http` can expose the same redacted status snapshot over a
  loopback-only HTTP endpoint and minimal browser dashboard.
- Codex receives rendered prompts with ticket and ProblemCard values.
- Semantic gates use calibrated golden sets; real runtime calls the configured
  judge provider through the same agent adapter boundary.
- Dispatch and session failures post actionable comments back to the tracker.
- `external_publication.branch_regex` automatically triggers
  `commission_approved` during Execute when the ticket target branch matches.
- On terminal pass, `external_publication.tracker_transition_to` transitions
  the ticket, for example to `Done`.

## Current Limits

- First real-run evidence still requires operator-provided Linear credentials,
  a repository URL, and one live canary ticket.
- The production path is Linear + Codex + Haft first. The Claude adapter is a
  parity skeleton only until the Codex canary has live evidence.
- Dynamic client tools from Codex app-server are rejected in this MVP runtime;
  built-in Codex file and command tools are the expected path.

## Verification

Project gate:

```sh
mix format --check-formatted
mix compile --warnings-as-errors
mix test
mix credo --strict
```
