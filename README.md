<img src="assets/banner.svg" alt="Haft" width="600">

*formerly [quint-code](https://github.com/m0n0x41d/quint-code)*

**Source-native FPF delivery and reliance-bearing project memory for
AI-assisted software engineering.**

Haft brings a versioned FPF source into an agent's working context and keeps a
local project graph when later work must rely on the result. A small,
reversible exchange can remain in conversation. A decision that authorizes
work, a handoff that another session must replay, or a claim that needs fresh
evidence becomes a typed project record.

Under the hood Haft is built on [FPF](https://github.com/ailev/FPF) by
[Anatoly Levenchuk](https://www.linkedin.com/in/ailev/). FPF is a rigorous
architecture for thinking about systems, and it is not small. Haft delivers
the pinned source, project-local application surfaces, and durable records
without replacing FPF with a second pattern catalog.

## Runtime truth labels

<!-- haft:truth-labels:start -->

- **CURRENT PRODUCT** — the published
  [v8.1.0 release](https://github.com/m0n0x41d/haft/releases/tag/v8.1.0).
  No v8.2 release was published.
- **V9 CONTRACT** — source-native FPF Query, fused code-graph orientation,
  project-profile onboarding, task-level EntityOfConcern establishment, typed
  project memory, neighborhood, and recall are included in the v9 contract.
  Contract inclusion specifies behavior; it is not delivery evidence.
- **EXACT-CANDIDATE EVIDENCE** — installed-runtime readiness claims require
  current P14 evidence tied to one exact candidate. Source, schema, skill, or
  local-test presence is not installed-runtime proof. That evidence does not
  grant RC or release status; either additionally requires release authority.
- **DEFERRED RESEARCH** — dense/hybrid retrieval and any claim that Haft
  retrieves or helps an agent better than a pinned FPF file, `rg`, or built-in
  RAG. No such superiority claim is made before the separate benchmark.

<!-- haft:truth-labels:end -->

These distinctions separate contract inclusion, exact-candidate evidence,
published-product status, and release authority. No source file, local test, or
matrix result silently promotes a contract to CURRENT PRODUCT, an RC, or a
release.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/haft/main/install.sh | bash
```

Then initialize Haft in your project. Bare `haft init` opens an interactive
multi-select when stdin and stdout are terminals; no host is preselected.
Scripts and CI must name their intent explicitly:

```bash
haft init                    # Interactive host multi-select (TTY only)
haft init --core-only        # Project core/ledger, no host carriers
haft init --claude           # Claude MCP + skills + CLAUDE.md section
haft init --claude --local   # Claude integration with repo-local skills
haft init --codex            # Codex MCP + skills + AGENTS.md section
haft init --codex --local    # Codex integration with repo-local skills
haft init --codex --mcp-only # Compatibility: Codex MCP config only
haft init --agents           # Global .agents/skills only
haft init --agents --local   # Repo-local .agents/skills only
haft init --codex --agents   # Codex integration; shared skill target coalesces
haft init --grok             # Grok CLI project MCP + skills
haft init --hermes           # Hermes MCP + external skills directory
haft init --zed              # Zed global MCP context server
haft init --agy              # Google Antigravity MCP + shared skills
haft init --all              # Full Claude + Codex integrations
haft init --all --mcp-only   # Claude + Codex MCP configs only
```

Any explicit init flag skips the menu and executes its declared behavior.
Bare non-interactive invocation fails before writing files instead of guessing
a host or waiting for terminal input. `--mcp-only` is a compatibility modifier:
it requires an explicit host flag or `--all` and suppresses that host's skills
and managed instruction section.

Codex publishes its transformed skills to `~/.agents/skills`, or project
`.agents/skills` with `--local`, as part of the full Codex integration.
`--agents` addresses the same location as an independent skills-only target
without MCP or instruction publication. It may compose with host flags; with
`--codex`, the identical `.agents/skills` projection is coalesced into one
write rather than treated as two competing targets.
`--all` means exactly the full Claude and Codex integrations; it does not add a
second independent `--agents` target.

For an already initialized project that only needs an additive database
upgrade, use the explicit core-only recovery command:

```bash
haft project migrate --project-root /absolute/project/root --project-id qnt_........
```

It verifies the exact project binding and changes no agent-host config, skill,
instruction, hook, or package carrier.

Claude Code and Codex are the stable supported hosts. Grok, Pi, Hermes, Zed,
Antigravity, Cursor, Gemini CLI, and OpenCode remain experimental or legacy
adapters with additional config flags
(`--grok`, `--pi`, `--hermes`, `--zed`, `--agy`, `--cursor`, `--gemini`,
`--opencode`). Please report host-specific issues with a PR or issue.

**Grok:** use `haft init --grok` (optionally `--local`). Native project
`.grok/config.toml` takes precedence over Claude/Cursor compat MCP sources, so
a stale global `haft` entry in `~/.claude.json` no longer shadows the project
server. Reload MCP in Grok (`/mcps` → `r`) or start a new session after init.

**Cursor:** after init, open Settings -> MCP -> find `haft` -> enable the
toggle. Cursor adds MCP servers disabled by default.

### What init does per tool

The binary is the same; host adapters differ in MCP config, transformed skill
location, and instruction carrier. Re-init replaces recognized legacy Haft
skills and updates only the content between `<!-- haft:start -->` and
`<!-- haft:end -->` in project instruction files. Content outside those
markers remains project-owned. A foreign file colliding with a desired
Haft-owned skill path fails before writes instead of being overwritten.

| Tool | MCP config | Skills | Project instructions |
|------|-----------|--------|----------------------|
| Claude Code (stable) | `.mcp.json` | `~/.claude/skills/` or `.claude/skills/` with `--local` | managed section in `CLAUDE.md` |
| Codex CLI / App (stable) | `.codex/config.toml` | `~/.agents/skills/` or `.agents/skills/` with `--local` | managed section in `AGENTS.md` |
| Agent skill bundle (`--agents`) | n/a | `~/.agents/skills/` or `.agents/skills/` with `--local` | none |
| Grok CLI (experimental) | `.grok/config.toml` (`mcp_servers.haft`) | `~/.grok/skills/` or `.grok/skills/` with `--local` | host adapter |
| Hermes (experimental) | `~/.hermes/config.yaml` (or `$HERMES_HOME/config.yaml`; profile via `--profile`) | generated Hermes-adapted skills through `skills.external_dirs` | host adapter |
| Zed (experimental) | `~/.config/zed/settings.json` (`context_servers.Haft`) | n/a | none |
| Antigravity (experimental) | `~/.gemini/config/mcp_config.json` (`mcpServers.haft`) | `~/.gemini/skills/` or `.gemini/skills/` with `--local` | host adapter |

Project-scoped configs (`.mcp.json`, `.codex/config.toml`, `.grok/config.toml`)
use portable project-root paths, so they are safe to commit for shared
repositories.
Zed and Antigravity settings are global and may start MCP/context servers
outside the workspace cwd. `haft init --zed` writes `HAFT_PROJECT_ROOT` and
`HAFT_EXPECTED_PROJECT_ID` for the project where you ran init. `haft init
--agy` writes `serve --project-root <root> --expected-project-id <id>` args so
the Antigravity entry does not depend on cwd or env propagation. Re-run the
host-specific init command in another project to point the global host entry
there.

### Local footprint

Haft is local-first, but it is not a zero-footprint prompt pack. `haft init`
creates markdown carriers in `.haft/` and a project SQLite database under
`~/.haft/projects/<id>/`. Those databases are where Haft keeps the structured
artifact graph, baselines, indexes, and runtime state that agents query through
CLI/MCP.

The Rust `haft-embed` sidecar is retained as an optional compatibility
component for older semantic-recall paths. It is outside the v9 contract and
P14 acceptance basis; its presence carries no v9 retrieval-quality claim. If
one of those compatibility paths uses it, Haft may start a shared local
EmbeddingGemma process and cache models under `~/.haft/`; a warm sidecar can
reasonably take around 1-2 GB of RAM depending on platform, model, and
workload. If the sidecar is absent or disabled, core governance still works
and the older compatibility path falls back to keyword/graph recall. Set
`embedding.provider: none` in `~/.haft/config.yaml` to keep that path off.

---

## How to Start

FPF is <ins>**powerful**</ins> and *genuinely complex*.
You do not need to learn it all before Haft becomes useful.

Install Haft, run `haft init` for your agent, and keep working normally. When a
concern benefits from FPF, the agent can retrieve the relevant source and use
the smallest applicable method. Call `/h-reason` when you want a deliberate
source-supported reasoning pass.

The narrower skills are independent application surfaces, not stages in a
mandatory workflow:

- `/h-frame` — clarify the real problem and acceptance criteria
- `/h-explore` — generate genuinely different solution variants
- `/h-compare` — compare options under explicit parity rules
- `/h-decide` — route a direct operator request for a binding decision
- `/h-verify` — check whether a past decision still holds
- `/h-status` — read the compact project cockpit

Haft makes actual gates explicit: when a human decision is needed, when
evidence has gone stale, when spec drift needs review, and when execution needs
an authorized plan or commission. It does not infer a universal next step from
the order of skills or artifacts.

Existing project? Run `haft init --core-only`. A complete, non-truncated,
supported singleton detector result is admitted as `origin=detector_default`;
Haft then installs only the specification and MethodPack carriers applicable
to that scope. Mixed, multiple-scope, insufficient, truncated, or manually
reviewed bases remain profile-review work, and init never changes an existing
canonical profile. A later direct, unambiguous operator request may supersede
only a current `detector_default` profile; onboarding status reports
`profile_override_eligible`, and successful application appends an
`host_routed_operator_request` admission. Further operator-mediated and legacy
profile changes remain a separate contract.

Check spec carriers locally:

```bash
haft spec check
haft spec check --json
haft spec migrate
haft spec migrate --json # read-only state for agents and automation
```

`SoftwareSystemSpec` describes the software that realizes the target system:
its role, responsibility allocation, behavior, interfaces, constraints, and
selected structure. Agent, commission, harness-runtime, and delivery policies
are deliberately outside this spec. Development-version `enabling-system.md`
carriers are migrated with one state-driven `haft spec migrate` command. Haft
resolves the internal exact candidate itself, records semantic review on one
invocation, performs the already reviewed migration on a later invocation, and
continues a sealed recovery journal when needed. Humans never pass packet
paths, hashes, refs, targets, or recovery modes.

`haft spec check` is deterministic L0/L1/L1.5 only: it parses fenced
`yaml spec-section` blocks, checks required structural fields, validates known
carrier shapes, and confirms the term-map carrier parses. It makes no L2
semantic judgment, no LLM review, and no L3 runtime claim.

---

## What is Haft?

The v9 target contract is a **source-native FPF delivery and project-memory
substrate** for principal-led engineering work. It includes source-native FPF
Query, project-profile onboarding, and typed project memory. Source presence is
not installed-runtime proof: readiness claims require current P14 evidence tied
to one exact candidate, while RC or release status additionally requires
release authority. The target contract exposes the versioned FPF source to the
host agent, then materializes problem frames, comparisons, decisions, notes,
commissions, specs, methods, and evidence only when a receiving use depends on
them.

Haft is not a coding agent, and it is not an AI documentation generator. It is
the handle between the human principal, the agent, the repository, and the
evolution lifecycle of your project.

## What Makes It Different

- **Source-native FPF delivery** — agents can retrieve the versioned FPF source
  with provenance instead of relying on a Haft-owned substitute catalog.
- **Reliance-bearing memory** — ordinary local reasoning may stay in chat;
  records enter the project graph for handoff, replay, authority, automation,
  evidence, or another explicit downstream reliance.
- **Kernel gates, not prompt-only discipline** — skills carry the procedure; the MCP
  kernel validates required fields, parity gaps, missing evidence, and authority
  boundaries server-side.
- **Reasoning fused with code** — `haft_query` can show the decisions and
  invariants governing a file or symbol while the agent reads or changes it.
- **Evidence decays** — old proof is not treated as forever current; Haft
  surfaces stale claims and drift for review.
- **Human authority stays explicit** — agents can frame, compare, verify, and
  prepare records, but binding decisions and commissions require the human
  principal.
- **Agent guidance is built in** — status, method cards, spec checks, and
  structured errors tell the agent what is missing.

### Three surfaces, one artifact graph

Haft is consumed through three surfaces over one `.haft/` artifact graph:

- **Skills** in your agent. Capability skills may trigger from the current
  operator request; `h-decide` may route a direct unambiguous binding request,
  while `h-commission` still requires manual invocation.
- **CLI** (`haft problem`, `haft solution`, `haft decision`, ...) — manual access. Those might be used manually, but usually it is just another surface for your agent to use.
- **MCP server** (`haft serve`) — programmatic access for any LLM agent over the Model Context Protocol

The kernel MCP server is the cross-host enforcement surface: it validates
arguments server-side and returns structured errors for FPF violations (missing
required fields, parity gaps, weakest-link omissions, predictions without
`verify_after`, and so on).

Skills carry the procedure; the kernel carries the gates. The same graph also
gives agents retrieval over project-local reasoning: notes for small facts,
decision records for authority, problem cards for open work, spec sections for
target/software-system shape, MethodRuns for execution discipline, and evidence
for verification.

### FPF source and project-local application

The FPF source defines the available concepts and methods. Haft pins and
indexes that source, carries it into the host agent, and supplies project-local
records and gates where later work needs them. Retrieval is not application,
evidence, approval, or performed work.

FPF is relation-first: the order of text, graph edges, cards, skills, or a
demonstrative walkthrough does not by itself prescribe causal, temporal,
method, or performed-work order. Such order exists only when an explicit,
separately governed causal claim, `U.MethodDescription`, `WorkPlan`, or work
relation states it. This does not make FPF acausal. Causal reasoning remains
available, but causality must be carried as a claim rather than inferred from
layout.

Planning remains a separate current task. A `U.WorkPlan` may state dependencies
and execution order whenever planning is the concern; an FPF source lookup,
application record, or reasoning capability does not prescribe that order. A
`WorkCommission` carries bounded execution authority and must not stand in for
the plan. Decision binding requires a direct unambiguous operator request;
execution-authority grants through `h-commission` remain manual-only.

`haft fpf query|lookup|inspect` and
`haft_query(action="fpf", mode="concern|lookup|inspect", ...)` expose the
versioned source as addressable publication units. Concern retrieval reports
observable authored-phrase, heading/keyword, and role-local FTS grounds;
lookup tries exact identity before returning compact candidates; inspect is
exact-only. Source roles (`practical_use_card`, `toc_row`, `preface`,
`pattern_body`, `pattern_section`, `pattern_scope`) control progressive disclosure. Candidates
are not selected patterns, and their order is not a causal or work order.

#### Maintaining the pinned FPF publication

Start with `task fpf-refresh-check`. It fetches and resolves one exact upstream
candidate, builds private temporary artifacts, and writes
`.context/fpf-refresh/latest-report.json` without changing the checked-out
source, embedded database, integration lock, typed-memory candidate, or specs.
The report has one closed result:

| Result | Meaning | Next action |
|---|---|---|
| `no_change` | Candidate and current publication are the same. | Nothing to apply. |
| `apply_ready` | The technical compatibility checks admit the candidate. | Run `task fpf-refresh`. |
| `review_ready` | Technical adaptation succeeded and semantic deltas are material enough to preserve explicitly. | `task fpf-refresh` applies the fresh source baseline and retains the deltas for downstream review. |
| `candidate_rejected` | Source, grammar, derivation, or compatibility checks failed closed. | Resolve the reported diagnostic and check again. |

`task fpf-refresh` repeats the check, pins the resolved candidate SHA, and
recoverably applies the coherent source, database, and generated
integration-lock publication for both `apply_ready` and `review_ready`.
`review_ready` is an auditable semantic-delta classification, not a veto on
adopting fresh FPF as Haft's source baseline. Applying it records no semantic
approval and grants no release authority. The apply also rebases only the
repo-owned Local-Practice compiled memory basis and `FPF-Spec.md` source pins,
then proves that the carrier parses, compiles, seals, verifies, and links
against the fresh basis. `task fpf-refresh` finishes with the exact read-only
integration verification. After a low-level recovery, run
`task fpf-verify`. If an apply is interrupted, keep the receipt unchanged and
use `task fpf-refresh-resume` to continue or `task fpf-refresh-restore` to
restore its exact predecessor; never delete or hand-edit the receipt.

Generated hashes, counts, and compatibility results remain distinct from
human-reviewed semantic and changelog claims. None of these commands commits,
binds a decision, changes a SpecSection lifecycle, changes the active
project-memory model, installs or restarts Haft, runs P13/P14, pushes, tags, or
releases.

### Fused code-graph Explore (v9 contract)

Explore accepts exactly one input shape: an exact symbol, or a bounded concern
query. The concern route returns advisory candidates and never selects a code
identity from rank alone.

```bash
haft graph explore --symbol PublishIndexEpoch --view working --json
haft graph explore --query "where is the index epoch published?" --view working --json
```

The equivalent MCP calls are
`haft_query(action="explore", symbol="PublishIndexEpoch")` and
`haft_query(action="explore", query="where is the index epoch published?")`.
Both surfaces execute one canonical `ExploreEnvelope` and use the same JSON
encoder. `working` is bounded, score-free, and omits retrieval and per-edge
provenance. `trace` adds bounded provenance plus an opaque replay basis;
`diagnostic` adds retrieval and traversal internals. A replay after the index,
request, or canonical result changes returns `replay_mismatch`.

Use Explore when area or flow orientation is current. Before a non-mechanical
edit to governed code, use `code_context` or `impact` on the actual target.
Purely mechanical work may abstain explicitly. Code-graph orientation and
typed-memory orientation are separate; neither substitutes for the other.

### What changed in v8

v8 dropped the standalone interactive agent (`haft agent`), its coding-agent
TUI, and the desktop wrappers. The terminal presentation used by current
`haft board` and `haft run` commands remains supported. Haft no longer competes
with general coding agents on the runtime surface — it adds governance
discipline on top of whichever agent you already use.

Upgrading from the published v8.1.0 release, or still migrating a v7 project?
See [MIGRATION-v8.md](MIGRATION-v8.md) for backup, forward-upgrade, host
restart, and rollback boundaries.

## How It Works

### Twelve MCP tools

| Tool | What it does |
|------|-------------|
| `haft_note` | Micro-decisions — atomic facts with typed anchors, validation, auto-expiry |
| `haft_problem` | Frame problems, declare comparison dimensions with indicator roles |
| `haft_solution` | Explore variants with diversity check, compare under parity |
| `haft_decision` | Decision contracts: invariants, claims, evidence, baseline lifecycle |
| `haft_refresh` | Lifecycle management for every artifact kind |
| `haft_query` | Project search/status, code graph, source-native FPF query/lookup/inspect, and typed-memory resolve/neighborhood/recall |
| `haft_method` | Task-local SWE MethodRun cards: pull gates before non-trivial work, close with evidence or waivers |
| `haft_commission` | WorkCommission lifecycle for execution harnesses |
| `haft_spec_section` | Typed SpecSection lifecycle projection over project SQL editions; FPF source compatibility is assessed separately; manual CLI gates approve, rebaseline, or reopen baselines |
| `haft_onboard` | Read setup status or prepare a non-binding project-profile review; `haft init` automatically admits only a complete supported singleton as `origin=detector_default` and installs default project memory |
| `haft_entity` | Proactively establish one minimum non-binding EntityOfConcern when current Work supplies a concrete operator-named or agent-inferred durable receiving use; known absence alone is insufficient |
| `haft_memory` | Expert raw MemoryChangeSet validation/admission through a nested `request` envelope; ordinary agents use `haft_entity`, and validation never admits automatically |

### Twelve skills installed by `haft init`

Haft installs 12 skills with independent trigger conditions:

| Skill | Mode | What it does |
|---|---|---|
| **h-reason** | auto (umbrella) | Minimum FPF distillate and source-query entry point. It helps choose one current capability without imposing a project sequence. |
| **h-frame** | auto | Shape the current problem conversationally; persist a ProblemCard only on explicit save intent or a named durable receiving use. |
| **h-diagnose** | auto | Diagnose a failure with parallel hypothesis testing (one Agent subagent per hypothesis to prevent anchoring) |
| **h-explore** | auto | Generate genuinely distinct candidate variants when exploration is the current task; record only when durability is warranted. |
| **h-compare** | auto | Compare an existing candidate set under declared parity and return a Pareto front, not a scalar winner. |
| **h-decide** | auto router | Bind a DecisionRecord only from a direct, unambiguous operator request; the skill token is not a receipt. |
| **h-verify** | auto | Baseline → measure → evidence loop with drift detection |
| **h-status** | auto | Read-only project FPF cockpit with explicit drill-down calls |
| **h-onboard** | auto | Bootstrap Haft and typed spec carriers when project state is absent or incomplete. |
| **h-spec** | auto | Spec lifecycle and source-currentness repair: inspect readiness, draft/clarify carriers, repair FPF semantic fanout, and route approve/rebaseline/reopen gates |
| **h-note** | auto | Persist an explicitly requested non-binding fact, observation, caveat, or rationale in project memory. |
| **h-commission** | **manual** | WorkCommission lifecycle — manual per Transformer Mandate (`disable-model-invocation`) |

Auto-triggering skills fire when their description matches operator context.
`h-decide` is a host-side router: a manual invocation remains a compatible
shortcut, but invocation itself creates no communicative act and adds no
authority. `h-commission` remains manual-only because it grants bounded
execution authority. Boundary unpacking, abductive checks, and semantic fanout
review are internal routines, not extra public stages.

For a DecisionRecord, one direct, unambiguous operator request must identify the
effect, readable subject, selected option, and scope. The host then runs the
CLI/input-file effect sink without another confirmation and records
`host_routed_operator_request`; it does not claim independent proof of
`U.SpeechAct`. If anything is ambiguous, the agent presents one Human Gate
Brief and accepts the natural-language answer. Bare `yes` or `да` works only for
one current brief with one unambiguous effect and selection. Quotations, pasted
text, agent recommendations, hypotheticals, and tool output do not bind.

MCP `haft_decision(action="decide", ...)` remains fail-closed with
`operator_confirmation_required` until a verifiable host receipt exists.
Project-profile application and later non-default project-memory model changes
use the same direct-request rule through their own effect sinks. The automatic singleton
profile bootstrap is a separate system policy and stays `detector_default`.

Project-local `.haft/config.yaml` is no longer an authority-policy carrier.
Fresh `haft init` does not create it. Init removes only the exact historical
Haft-generated authority-only file after digest revalidation; changed or
unrecognized bytes are preserved, ignored, and reported as legacy. Global
`~/.haft/config.yaml` and project `.haft/project.yaml` are unchanged.

### Evidence workflow

Attach evidence with `haft_decision(action="evidence", ...)`. Evidence carries
formality levels (F0–F9), congruence levels (CL0–CL3), and expiry dates. Trust
scores (R_eff) degrade as evidence ages; stale evidence triggers refresh. Use
`haft_decision(action="measure", ...)` for post-implementation verification.

### Harness execution engine (beta, Codex only)

> 🚨 It is likely to be suspended in future versions because the work commission is highly commoditized already by subagents of most agents. Those agents usually have access to the same skills and MCP, meaning... Subagents work quite well on half-created problems and decisions, so I find myself using Claude code and Codex subagents quite well without the need for any external harness circuit.

The harness implements code from `DecisionRecord` artifacts under a real Codex
agent in an isolated workspace. It is **beta**, and the execution agent is
**Codex only** — there is no Claude execution path. Single-commission
`haft harness run` is the trustworthy operator path; drain mode and auto-apply
are validated on docs-class commissions, so treat them as beta on
production-code commissions.

Two entry points spawn the engine. `haft run` implements one decision directly:

```bash
haft run dec-20260414-001
```

It reads the decision's invariants, claims, and affected files from the graph,
builds a prompt with full reasoning context, spawns a Codex agent with the
invariants as guardrails, and takes a baseline snapshot on completion.

`haft harness` runs commissioned work through Open-Sleigh, with scope guards
(`allowed_paths` / `forbidden_paths`), per-commission locks, and discrete
revertable apply commits:

```bash
haft harness run --prepare-only      # create/reuse commissions, do not start runtime
haft harness run                     # create/reuse commissions and start Open-Sleigh
haft harness run --drain --concurrency 4   # drain the queue (apply still manual by default)
haft harness status                  # inspect active/recent runs
haft harness result wc-...           # inspect one completed run and its workspace diff
haft harness apply wc-...            # apply a completed workspace patch to this checkout
```

Commissions carry a `delivery_policy`. The default `workspace_patch_manual`
keeps changes in the isolated workspace until you run `haft harness apply`.
`workspace_patch_auto_on_pass` applies a passing run as a discrete commit;
`blocked_policy` / failed runs wait for an operator decision.

Broad harness execution is blocked for `needs_onboard` projects by default. For
intentional tactical out-of-spec work, pass `--force-skip-specs "<reason>"`;
haft records the reason on the selected commissions.

Release archives bundle the Open-Sleigh BEAM runtime, so normal harness use
needs no Elixir/Mix install:

```text
~/.haft/runtimes/open-sleigh/current
```

The lower-level surface is the `haft commission` CLI for binding creation
(`create-from-decision`, `create-batch`, `create-from-plan`, ...) plus the
`haft_commission` MCP tool for non-creation lifecycle/read actions. MCP creation
actions fail closed by default with `operator_confirmation_required`; every
authorized commission action must come through a kernel-accepted `manual_cli`
authorization receipt and becomes a typed artifact transition. Model-supplied
MCP arguments or prompt text are not proof of operator authorization:

```text
SpecSection(s) --may-govern--> DecisionRecord
DecisionRecord --may-authorize--> WorkCommission
WorkCommission --may-start--> RuntimeRun
Evidence --supports-or-weakens--> claim / SpecSection
current relations --derive--> SpecCoverage
```

This is the commissioned execution path after explicit authority, not a
universal FPF or planning sequence.

---

## Cookbook — common workflows

### Record an architectural choice

```text
operator (to agent): "we need to pick a queue for the new ingestion path"
↓ agent retrieves the relevant FPF source and checks whether a durable record is needed
↓ agent uses only the applicable methods; framing, exploration, and comparison are available but not a fixed sequence
↓ when downstream work depends on a binding choice, the operator directly says which exact option to bind
↓ h-decide routes that request; a skill token is optional and is not authority
↓ kernel validates required DRR fields; missing fields → structured error
↓ on pass: DRR written to .haft/decisions/, ready for `haft run`
```

### Diagnose a failure with rival hypotheses

```text
operator: "tests are failing on the schema migration after the deploy"
↓ h-diagnose auto-triggers, spawns 3+ parallel Agent subagents, one per hypothesis
↓ each subagent reads only what its hypothesis needs (no anchoring)
↓ results merged, ranked by the FPF B.5.2 filter chain
↓ the diagnosis stays in chat unless the operator explicitly asks to save it
  or names a reliance-bearing receiving use that needs a durable record
↓ when that persistence condition holds: /h-note records the diagnosis
↓ /h-frame is used only if the evidence shows the problem itself needs reframing
```

### Verify a decision still holds

```text
operator: "did dec-20260420-cache-redesign actually work"
↓ h-verify auto-triggers
↓ reads decision predictions + valid_until + baseline file hashes
↓ measures observable claims (test output, metric query, ...)
↓ writes evidence with CL/freshness; updates R_eff
↓ if R_eff < 0.5 → marks stale; if predictions failed → reopens the problem
```

### Quick operator status

```bash
haft check          # CI-friendly governance verification (exit 0 clean / 1 findings)
```

From the host agent: `/h-status` for the compact cockpit; use explicit
drill-down calls for full status, coverage, drift/stale detail, and maintenance
plans.

Shipped history lives in [CHANGELOG.md](CHANGELOG.md).

---

## License

MIT
