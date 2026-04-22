# Open-Sleigh

Open-Sleigh is a local engineering agent runner for project work.

Current implementation status:

- `open-sleigh/` currently contains a legacy tracker-first CLI runtime used
  for the bootstrap canary.
- The Haft monorepo target is commission-first: Haft owns WorkCommissions,
  RuntimeRuns, Evidence, and optional ExternalProjection; Open-Sleigh executes
  leased commissions.
- Until `CommissionSource.Adapter` and `haft_commission.*` are implemented,
  commission-first wording in specs describes the target path, not current
  production behavior.

Target runtime loop:

1. claim runnable Haft WorkCommissions,
2. preflight each commission against its linked DecisionRecord and scope,
3. create a per-commission workspace,
4. run Codex through `codex app-server`,
5. write RuntimeRun and phase outcomes to Haft,
6. evaluate calibrated semantic gates,
7. request human approval before configured one-way-door transitions,
8. let Haft publish optional Linear/Jira/GitHub projections.

The current product surface is CLI-first and runs on the operator's machine.

## Requirements

- Elixir 1.18
- `codex` with `codex app-server`
- `haft` with `haft serve`
- `git`
- Linear credentials for the current legacy tracker-first canary
- Linear/Jira/GitHub credentials only when external projections are enabled
  in the commission-first target
- A Git repository URL that `git clone` can read

## Configure

Start from the example:

```sh
cp sleigh.md.example sleigh.md
```

`sleigh.md.example` is the current legacy tracker-first fixture. The
commission-first target shape for the Haft monorepo integration is documented
in `sleigh.commission.md.example` and is not yet the default runtime spine.

Set the real environment:

```sh
export REPO_URL=git@github.com:org/repo.git
# Optional, only when projection targets are enabled in Haft:
export LINEAR_API_KEY=...
```

`REPO_URL` is not GitHub-specific. It is passed to `git clone`, so SSH/HTTPS
URLs for GitHub, GitLab, self-hosted Git, and local Git remotes work if your
machine can authenticate.

In the commission-first target, Open-Sleigh does not require a tracker to run.
Work intake is through Haft WorkCommissions. A commission carries the
ProblemCard/DecisionRecord links, scope, evidence requirements, projection
policy, and freshness snapshot.

The current legacy tracker-first canary still uses tracker tickets and a
ProblemCard reference in the description by default:

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

- Current implementation: legacy `TrackerPoller` reads active tracker tickets
  and constructs sessions from `Ticket` snapshots.
- Commission-first target: Haft selects runnable WorkCommissions and grants
  short-lived leases.
- Open-Sleigh runs a Preflight phase before Execute. Preflight may inspect
  Haft artifacts and repository context, but Haft decides whether execution is
  admissible.
- `hooks.after_create` prepares a fresh workspace, typically with `git clone`.
- `hooks.before_run` refreshes the workspace before each phase.
- `after_create` and `before_run` hook failures block the session and surface
  through retry/status/projection failure reporting.
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
- Codex receives rendered prompts with WorkCommission, DecisionRecord, and
  ProblemCard values.
- Semantic gates use calibrated golden sets; real runtime calls the configured
  judge provider through the same agent adapter boundary.
- Dispatch and session failures are written to Haft. If external projection is
  enabled, Haft may publish manager-facing comments to Linear/Jira/GitHub.
- `external_publication.branch_regex` automatically triggers a HumanGate during
  Execute when the target branch matches.
- On terminal pass, Open-Sleigh writes RuntimeRun/Evidence to Haft. Haft then
  updates local status and any configured ExternalProjection.

## Current Limits

- First commission-first real-run evidence requires the new Haft
  `haft_commission.*` surface, a WorkCommission, repository URL, and one canary
  plan. External tracker credentials are optional in that path.
- The production path is Haft + Codex first. Linear/Jira/GitHub are optional
  projection targets; the Claude adapter is a parity skeleton until the Codex
  canary has live evidence.
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
