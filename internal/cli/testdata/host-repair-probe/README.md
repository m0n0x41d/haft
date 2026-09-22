# Shared Codex/Air initialization fixture

Run from the repository root, with its pinned `data/FPF` submodule available:

```sh
python3 internal/cli/testdata/host-repair-probe/build.py --baseline
python3 .context/host-probe/baseline.py
python3 .context/host-probe/run.py
```

`build.py` builds a CLI and Go-overlay private-access probes into `.context/host-probe`.
Omit `--baseline` when commit `17d5bbe5` is unavailable. `baseline.py` requires that
optional build and reproduces the original sequential failure and stale-receipt failure.

The probe compiles the ordinary explicit single-host request and invokes the real
host planner and publisher. The core project plan is inert: no database is opened,
no `init` core is executed, and no profile or project binding is established.
Explicit disposable project/home roots are passed as arguments. `HOME` and
`CODEX_HOME` are not changed. The fixed CLI supplies the same publication identity
to all probes, so generated adapter/config differences are isolated.

The previous-generation overlay emits exact shared 10-second server/env tables
and suppresses the redundant legacy registry entry. It models prior generated
host receipts; it is not an exact historical full CLI build. The optional base
probe uses the three actual host CLI source files from `17d5bbe5`.

`run.py` covers both host orders on fresh and exact legacy configurations, two
prior host manifests, repeat byte/mtime stability, unselected receipt preservation,
nested approvals, custom startup/tool timeouts, required/approval fields, commands,
environment entries, and a carrier edit between planning and publication. Each
run gets a new fixture directory and saves all probe responses in `calls.jsonl`.

These are bounded standalone fixture probes, not `go test`, suite, installed-host,
MCP startup, migration, or release-readiness evidence. The focused Go regression
cases live beside the CLI and init-planning code and remain separately runnable.
