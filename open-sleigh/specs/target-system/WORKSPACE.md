---
title: "11. Workspace Management and Hooks"
description: Per-commission workspace lifecycle, hook kinds, trust posture, startup terminal cleanup. Inherited from Symphony §5.3.4 / §9 with FPF constraints.
reading_order: 11
---

# Open-Sleigh: Workspace Management and Hooks

> **FPF note.** Hooks are `Carrier` for bootstrap work (clone repo,
> install deps). They are NOT a `Carrier` for governance logic — that
> attempt is a Thai-disaster attractor and the first symptom of
> harness-about-harness drift. The workspace itself is the scope within
> which the agent's `Description` of its work becomes `Reality`.

---

## 1. Per-commission workspace lifecycle

- **Path:** `<workspace.root>/<sanitized_identifier>`. Sanitization
  replaces any char not in `[A-Za-z0-9._-]` with `_`.
- **Reuse across runs for the same WorkCommission** (matches Symphony):
  successful runs do NOT delete workspaces. This makes continuation
  turns and retry attempts cheap.
- **Workspace is the `cwd` for the agent subprocess.** Enforced by
  `PathGuard.canonical/1` (see `../enabling-system/FUNCTIONAL_ARCHITECTURE.md L4`).
  Workspace must never resolve (after symlink/hardlink/canonical
  expansion) into `open_sleigh/` or `~/.open-sleigh/` — hard-fail at
  `Session.new/1`. Bypass matrix covered by `ILLEGAL_STATES.md`
  CL5–CL12.

## 2. Hooks

Hooks are shell scripts declared in `sleigh.md` under `hooks:`. They are
**trusted configuration** (same trust posture as the operator DSL
itself) but their writes are still subject to PathGuard.

| Hook | Fires when | Failure semantics |
|---|---|---|
| `after_create` | New workspace directory created (not on reuse) | Fatal to workspace creation. If it fails, the run attempt errors; partially-created directory is cleaned. |
| `before_run` | Before each agent attempt (every turn loop start) | Fatal to current attempt. |
| `after_run` | After each agent attempt (any terminal state) | Logged, ignored. |
| `before_remove` | Before workspace deletion (only on commission-terminal cleanup) | Logged, ignored. |

- **`hooks.timeout_ms`:** default 60_000 ms. Non-positive falls back.
- **Execution:** `bash -lc <script>` with `cwd = workspace_path`.

## 3. Bootstrap-only guidance (v0.6.1)

**Operator discipline.** Hooks exist for **workspace bootstrap**: repo
clone, dependency install, tool fetch. They are explicitly NOT for:

- orchestration logic
- agent pre-prompts
- result post-processing
- governance routines
- any behaviour that belongs inside the phase-gated engine

If a hook is growing past a handful of lines of bootstrap shell, that
is governance-drift — a Thai-attractor pattern documented in
`../enabling-system/FUNCTIONAL_ARCHITECTURE.md §Thai-disaster
architectural guardrails`. The hook should be moved into a proper
adapter / gate / phase mechanism instead.

**Enforcement:** review norm today; no static linter yet. `ILLEGAL_STATES.md`
WH1 documents the honest limitation — hook writes are partially
enforceable (PathGuard dereferences post-hook) but the hook content
itself is trusted.

## 4. Startup terminal workspace cleanup

On engine startup (per Symphony §8.6):

1. Query Haft for WorkCommissions currently terminal/cancelled/expired.
2. For each returned identifier, sanitise to workspace key, check if
   `<workspace.root>/<key>` exists, run `before_remove` hook if
   present, `rm -rf` the directory.
3. On Haft/projection failure: log warning and continue startup (don't block
   on cleanup).

Prevents workspace accumulation after restarts — especially important
when the canary suite churns commissions T1/T1'/T2/T3 across gate-
regression runs.

---

## See also

- [ILLEGAL_STATES.md](ILLEGAL_STATES.md) — CL5–CL12 (PathGuard bypass matrix), WH1–WH5 (hook invariants), SE3 (workspace path canonicalisation)
- [AGENT_PROTOCOL.md](AGENT_PROTOCOL.md) — §1 transport (workspace is the subprocess `cwd`)
- [SLEIGH_CONFIG.md](SLEIGH_CONFIG.md) — `hooks:` + `workspace.root` configuration surface
- [../enabling-system/FUNCTIONAL_ARCHITECTURE.md](../enabling-system/FUNCTIONAL_ARCHITECTURE.md) — L4 `PathGuard.canonical/1` algorithm
- [RISKS.md](RISKS.md) — Haft-wins reconciliation (why workspaces may need terminal cleanup)
