# Migrating existing Haft installations

## Published release lineage

The published predecessor to v9 is
[v8.1.0](https://github.com/m0n0x41d/haft/releases/tag/v8.1.0).
There is no published v8.2 release. Work that was previously described as an
8.2 candidate remains unreleased v9 lineage.

This document states the supported migration and rollback boundary. It is not
evidence that a particular project has been upgraded successfully. Release
publication remains separate from the real v8.1-to-v9 rehearsal on the exact
candidate binary.

## Upgrading v8.1.0 → v9.0.0

### 1. Stop writers and take a complete backup

Quit every host agent that can run `haft serve`, then confirm no Haft MCP
server is still using the project. Back up both state locations before the
first v9 start:

- the project-local `.haft/` directory;
- the project ledger at
  `~/.haft/projects/<project-id>/haft.db`, where `<project-id>` is the `id`
  stored in `.haft/project.yaml`.

Use SQLite's online backup API or `.backup` command if the database can still
be open. If every writer is stopped, copy `haft.db` together with any
`haft.db-wal` and `haft.db-shm` sidecars as one unit. Copying only the main
database file while a WAL writer is active is not a valid backup.

Keep the backup outside both original directories and record its checksum.
Do not test the upgrade on the only copy of a project ledger.

### 2. Install and identify the candidate

Install the intended v9 binary only after it has a published artifact, or use
the exact local release-candidate binary during controlled acceptance. Record:

```bash
haft version
```

The version, commit, and binary checksum must identify the same candidate that
will be tested. Restarting the host is part of the upgrade: an already running
MCP process continues executing the old loaded binary even after the file on
disk is replaced.

### 3. Re-run init for the selected hosts

Run `haft init` with the exact host targets you use. For example:

```bash
haft init --claude
haft init --codex
```

Haft-owned skill projections and marked instruction sections are refreshed to
the installed version. Project text outside Haft-owned markers remains project
owned. Recognized legacy Haft skills are replaced; foreign path collisions
fail before writes. Re-run only the host flags you actually want. Use
`--mcp-only` only when you deliberately want host MCP config without skills or
instructions; `--core-only` performs project-core migration without publishing
host files.

### 4. Restart and verify

Completely restart each host agent after init. Check that its MCP server starts
the same binary recorded in step 2, then exercise project status, an existing
record read, and one explicitly authorized v9 write on a copied project before
upgrading the working project.

Database startup migrations are forward migrations. Successful command exit is
not sufficient upgrade evidence: the release rehearsal must also verify legacy
record counts and content, project binding, typed-memory reads, one new write,
reopen/idempotent retry, and restoration from the backup.

### Downgrade boundary

Opening a v9-migrated project ledger with v8.1 is unsupported. Do not use the
v8.1 binary to write to a database after v9 has migrated it.

To roll back:

1. stop all v9 host agents and MCP servers;
2. move the migrated project `.haft/` and global project-ledger directory out
   of the way without deleting them;
3. restore both locations from the pre-upgrade backup;
4. reinstall v8.1.0 and re-run its host initialization;
5. verify the restored project before resuming work.

Rollback means restoring the pre-upgrade state. Installing an older binary over
the v9 binary without restoring the database is not a supported rollback.

## Historical v7 → v8 surface migration

v8 is a governance-substrate pivot. The reasoning kernel, the artifact graph,
the FPF spec retrieval, the WorkCommission lifecycle — **all unchanged**.
What changed is the surface: haft is now consumed through host-AI skills
plus an MCP server, not through a standalone interactive agent.

Full rationale, parity-compared variants, rollback plan, and falsifiable
predictions live in
`.haft/decisions/dec-20260525-v8-architecture-pivot-from-standalone-agent-to-g-bbe45cb7.md`.

## What was dropped

- `haft agent` — the standalone interactive REPL
- TUI surface (`internal/tui`, `tui/` package)
- Desktop wrappers (Tauri / Wails apps in `desktop/`)
- v7 helper commands: `haft login`, `haft models`, `haft setup`
- the v7 `/h-reason` implementation — replaced by the v8-era skill catalog;
  v9 restores `h-reason` as a source-first umbrella over independent skills

If you depended on any of those, **do not upgrade** until you've migrated
to one of the three remaining surfaces (skills/CLI/MCP).

## What replaced what

| v7 you used | v8 equivalent |
|------|------|
| `haft agent` (interactive) | Talk to Claude Code / Codex / OpenCode / Cursor; the v8 skills auto-trigger |
| `/h-reason "..."` (one-shot) | v9 `/h-reason` for source-first reasoning, or one exact specialized skill |
| `/h-reason` for explicit reasoning | Invoke `/h-reason`; specialized skills remain independent capabilities, not mandatory stages |
| `haft setup`, `haft login`, `haft models` | Not needed — host AI provides the LLM; haft only manages the artifact graph |
| Desktop dashboard | `haft check`, `/h-status`, `/h-verify` from the host AI; PR creation via `haft run` |
| TUI session view | `/h-status` in the host AI, or `haft_query(action="status")` from MCP |

## Upgrade steps for an existing project

1. **Re-run `haft init`.** Select the host config and skill publication targets
   explicitly. This installs the current 12-skill catalog,
   refreshes `h-reason` and the independent specialized skills, and registers
   the MCP server under the selected host config.

   ```bash
   cd /your/project
   haft init --claude          # Claude config and Claude skills
   haft init --codex           # Codex config, .agents/skills, and AGENTS.md
   ```

2. **Audit references to dropped commands.** Search your project notes
   and CI for `haft agent`, `haft login`, `haft models`, `haft setup`,
   and replace them per the table above. Current `/h-reason` references are
   valid v9 usage and should not be removed mechanically.

   ```bash
   grep -rn "haft agent\|haft login\|haft models\|haft setup" .
   ```

3. **Restart your host AI.** Claude Code, Codex, etc. cache skill
   manifests at startup. After `haft init`, restart the host to pick
   up the new catalog.

4. **Your `.haft/` artifact graph is unchanged.** Decisions, problems,
   evidence, baselines, WorkCommissions — all still load, still verify,
   still surface in `/h-status`. The v7→v8 pivot is a surface change,
   not a schema change.

## Behavioral changes worth knowing

- **h-decide is now manual-only** (`disable-model-invocation: true`).
  The host AI agent will not fire it automatically even on matching
  prompts. You must type `/h-decide` (or its host-specific equivalent).
  Same for `/h-commission`. This enforces the Transformer Mandate:
  binding artifacts come from the human principal, not the agent.

- **Tactical-mode validation has explicit skip.** When recording a
  decision in `tactical` mode, you can pass `_skips` (list of field
  names) plus `_skip_reason` to bypass validation on a specific field
  with reason recorded. The allowlist excludes load-bearing fields
  like `selected_title`. Standard and deep modes cannot skip.

- **MCP returns structured errors as enforcement gates.** Missing
  required DRR fields produce a plain-text error with field hints +
  FPF spec references (CMP-02, DEC-08, X-WLNK, etc.) + a "how to
  proceed" section listing skip semantics. Read it; the message
  contains everything needed to either fill the missing data or
  acknowledge with a skip.

- **Diagnose runs parallel hypotheses.** The new `h-diagnose` skill
  spawns one Agent subagent per hypothesis in the same message,
  preventing the LLM's natural anchoring bias toward the first
  plausible cause. Forces 3+ rivals per FPF CC-B.5.2-2.

- **Compare runs dim-wise parallel scoring.** The new `h-compare`
  skill spawns one Agent subagent per comparison dimension scoring
  all variants — again to prevent anchoring. Parity plan and
  selection policy declared BEFORE scoring (Anti-Goodhart).

## Historical v7 rollback

This section applies only when reproducing the old v7-to-v8 surface migration.
It is not the v9-to-v8.1 downgrade procedure; that boundary requires the
complete pre-v9 restore described above.

If v8 produces regressions in a still-v7 migration rehearsal:

1. Pin to the last v7 release in your install command.
2. Re-run `haft init` on that pinned version to restore the v7 skill
   layout.
3. File a regression issue at https://github.com/m0n0x41d/haft/issues
   with the host AI + version + skill that failed to fire as expected.

The v8 pivot used a deterministic keyword-overlap routing check. v9 removed
that check with the compiled router: the 12 skills are independent
capabilities, while `h-reason` queries source-native FPF material for the
current concern. Historical routing scores are not v9 release evidence.

## Reference

- Full pivot DRR: `.haft/decisions/dec-20260525-v8-architecture-pivot-from-standalone-agent-to-g-bbe45cb7.md`
- Execution plan: `.context/v8_haft_governance_substrate_plan.md`
- Skill catalog: README.md "Twelve skills installed by `haft init`"
