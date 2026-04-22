# Term Map — Canonical Vocabulary

> Reading order: 2 of N. Read after SYSTEM_CONTEXT. 15 minutes.
>
> One meaning per term. When in doubt, check here.
> If a term isn't here, it doesn't have a canonical meaning yet — add it before using it in specs.

## Core Domain

| Term | Definition | NOT this | Aliases allowed |
|------|-----------|----------|-----------------|
| **Artifact** | Any persisted reasoning object in the artifact graph: ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, Note, RefreshReport | Not a code artifact. Not a build artifact. Not a generic "thing." | — |
| **ProblemCard** | Artifact that frames what's broken: signal, constraints, optimization targets, observation indicators, acceptance criteria, blast radius | Not an issue. Not a ticket. Not a bug report. | "framed problem" in casual speech |
| **SolutionPortfolio** | Artifact containing 2+ genuinely distinct variants for a framed problem, with optional characterization (comparison dimensions) and comparison results | Not a plan. Not a spec. | "portfolio", "options" in casual speech |
| **DecisionRecord** | Artifact that records what was chosen and why: selected variant, rationale, invariants, claims, rollback, valid_until, affected files | Not an ADR (ADRs are static docs; DecisionRecords have live evidence). Not a commit message. | "decision", "DRR" in internal/FPF context |
| **EvidencePack** | Artifact containing measurement data attached to a decision: type, verdict, congruence level, valid_until | Not a test report. Not a PR review. | "evidence" |
| **Note** | Micro-decision artifact: what was decided and why, with rationale validation. Auto-expires in 90 days. | Not a comment. Not a log entry. Not a TODO. | — |
| **RefreshReport** | Artifact documenting a lifecycle action (waive, reopen, supersede, deprecate) | Not an audit log entry (though audit log exists separately). | — |

## Engineering Modes (User-Facing)

| Term | Definition | NOT this | FPF mapping (internal only) |
|------|-----------|----------|---------------------------|
| **Understand** | Activity of framing the problem before solving it | Not "requirements gathering." Not "reading code." | Problem framing, characterization |
| **Explore** | Activity of generating genuinely distinct solution variants | Not "brainstorming." Not "asking the agent for ideas." | Solution portfolio construction |
| **Choose** | Activity of comparing variants honestly and deciding readiness | Not "picking the first one." Not "asking the agent to recommend." | Pareto comparison, probe-or-commit |
| **Execute** | Activity of recording the decision as a contract and implementing | Not "just coding it." The decision record comes first. | DRR creation, baseline |
| **Verify** | Activity of checking whether decisions still hold | Not "running tests." Broader: drift, staleness, evidence decay, pending claims. | Refresh, measurement, evidence attachment |
| **Note** | Fast-path: capture a micro-decision without full ceremony | Not a sticky note. Has rationale validation and conflict check. | Note artifact |

## Evidence & Trust

| Term | Definition | NOT this |
|------|-----------|----------|
| **R_eff** | Effective reliability of a decision. Computed as `min(evidence_scores)` with CL penalties. Weakest-link principle: one bad evidence item bounds the whole score. | Not an average. Not a confidence percentage. |
| **WLNK** | Weakest Link — the single mechanism that bounds a decision's quality. Identified per variant during Explore, preserved in DecisionRecord. | Not "cons." Not a generic risk. The specific thing that fails first. |
| **CL (Congruence Level)** | How well evidence transfers across contexts. CL3 = same context (0 penalty). CL2 = similar (0.1). CL1 = different (0.4). CL0 = opposed (0.9). | Not a confidence score. Not a quality rating. A context-transfer penalty. |
| **Evidence decay** | Evidence has `valid_until`. After expiry, scores 0.1 (weak, not absent). Graduated severity by days overdue. | Not "evidence deleted." Decayed evidence stays for audit. |
| **Claim** | A falsifiable statement attached to a DecisionRecord with `observable`, `threshold`, and optional `verify_after` date | Not an assertion. Not a hope. Must be measurable. |
| **Verdict** | Assessment of evidence against a claim: `supports`, `weakens`, `refutes`, `superseded` | Not pass/fail. Supports/weakens is a spectrum. |
| **Baseline** | SHA-256 snapshot of affected files after implementation. Used for drift detection. | Not a git tag. Not a test suite. A hash snapshot at a point in time. |
| **Drift** | File content changed since baseline. Signal, not automatic invalidation — some drift is expected. | Not "bug." Not "regression." Drift is a fact; interpretation follows. |

## Comparison & Choice

| Term | Definition | NOT this |
|------|-----------|----------|
| **Pareto front** | The non-dominated set of variants: no variant in this set is strictly worse than another on all dimensions. Human chooses from this set. | Not "the best option." Not a ranked list. A set of trade-offs. |
| **Parity** | Same inputs, same scope, same measurement procedure for all variants being compared. Without parity, comparison is invalid. | Not "fairness" in a social sense. Technical measurement fairness. |
| **Probe-or-commit** | Assessment gate before comparison: should we gather more data (probe), expand options (widen), or proceed to compare (commit)? | Not "should we decide?" It's "is the comparison basis ready?" |
| **Indicator role** | Each comparison dimension is tagged: `constraint` (hard limit, must satisfy), `target` (optimize), `observation` (watch but don't optimize — Anti-Goodhart). | Not all dimensions are equal. Constraints eliminate; targets rank; observations prevent gaming. |
| **Constraint elimination** | Variants violating a constraint dimension are removed before Pareto computation, not merely scored lower. | Not "penalty." Elimination. |

## Architecture

| Term | Definition | NOT this |
|------|-----------|----------|
| **Core** | Architecture module: artifact graph, knowledge graph, FPF search, evidence engine, codebase analysis. Pure domain logic + persistence. No UI dependencies. | Not "core team." Not "core features." The domain kernel. |
| **Flow** | Architecture module: task runner, worktree lifecycle, agent spawning, invariant injection, post-execution verification. | Not "workflow tool." The execution orchestration layer. |
| **Governor** | Architecture module: background scanner, drift detection, stale refresh, invariant verification, problem factory. | Not "governance framework." The automated post-merge integrity system. |
| **Surface** | A user-facing entry point: Desktop App, MCP Plugin, or CLI. Surfaces depend on Core/Flow/Governor. | Not "interface" in the Go sense. A product delivery channel. |
| **Knowledge graph** | Query interface over artifact→code→module→dependency relationships. Maps decisions to files, invariants to modules, drift to impact. | Not a Neo4j graph. SQLite queries over existing tables with cycle-safe traversal. |

## Work Execution

| Term | Definition | NOT this |
|------|-----------|----------|
| **ImplementationPlan** | A graph of intended execution work derived from one or more active DecisionRecords. Contains WorkCommissions, dependencies, locksets, evidence requirements, and scheduling policy. | Not a DecisionRecord. Not a flat TODO list. Not a tracker epic. |
| **WorkCommission** | Human-authorized, bounded permission to execute a selected DecisionRecord in a declared Scope. It records repo, branch/base SHA, scope hash, gates, evidence requirements, projection policy, freshness snapshot, and allowed runner policy. | Not the decision itself. Not a Linear/Jira issue. Not a RuntimeRun. Not a prompt hint. |
| **Scope** | Closed authorization object for what a WorkCommission may touch: repo/ref, base SHA, target branch policy, allowed paths, forbidden paths, affected files/modules, allowed actions, optional allowed modules, and associated lockset. Its canonical serialized form is hashed into the commission snapshot. | Not a fuzzy task description. Not merely affected_files. Not a workspace safety check. |
| **CommissionSnapshot** | Deterministic equality set frozen when a WorkCommission is queued: DecisionRecord revision/hash, ProblemCard ref/revision, Scope hash, base SHA, ImplementationPlan revision, AutonomyEnvelope revision, projection policy, and lease state. Preflight compares this set before Execute. | Not runtime evidence. Not semantic freshness judgement. |
| **RuntimeRun** | One concrete execution attempt against a WorkCommission by a runner such as Open-Sleigh. Carries runner id, lease, phase outcomes, logs/evidence refs, and terminal result. | Not the WorkCommission. Not proof that the decision is correct. |
| **Preflight** | Mandatory readiness check before a WorkCommission can enter RuntimeRun execution. Checks commission state, linked DecisionRecord freshness, scope drift, lease ownership, policies, and runner eligibility. | Not implementation. Not a best-effort agent summary. |
| **AutonomyEnvelope** | Explicit human-approved bounds for batch/YOLO execution: max commissions, concurrency, paths/repos allowed, forbidden actions, risk ceiling, failure strategy, and one-way-door exclusions. | Not unlimited permission. Not a way to skip freshness, evidence, lock, or policy gates. |
| **Lease** | Short-lived exclusive claim on a WorkCommission or RuntimeRun phase held by one runner. Prevents two agents from executing the same work or overlapping locksets concurrently. | Not ownership of the decision. Not long-term assignment. |
| **Lockset** | Concurrency-control projection of Scope: files/modules/resources used to prevent overlapping running commissions. | Not authorization by itself. Not affected_files evidence. Not a git lock. |

## External Coordination

| Term | Definition | NOT this |
|------|-----------|----------|
| **ExternalProjection** | Idempotent binding from Haft work state to an external coordination carrier such as Linear/Jira/GitHub Issues. Stores external id, desired state, observed state, sync hash, drift, and last sync time. | Not semantic authority. Not `.haft/*.md` artifact Projection. |
| **ProjectionPolicy** | WorkCommission/ImplementationPlan setting that determines external publishing: `local_only`, `external_optional`, or `external_required`, plus targets and audience. | Not tracker configuration alone. Not execution permission by itself. |
| **ProjectionIntent** | Deterministic fact packet saying what should be communicated externally: state, reason, blockers, next actions, required links, redactions, and forbidden claims. | Not prose. Not an LLM decision. |
| **ProjectionWriterAgent** | Bounded LLM writer that turns ProjectionIntent into plain external text for managers/analysts/leads. It may choose wording only. | Not the authority for status, severity, completion, scope, or promises. |
| **ProjectionDraft** | Candidate title/body/comment/field update produced from ProjectionIntent by the writer. Must pass validation before publication. | Not published truth until connector writes it. |
| **ProjectionValidation** | Deterministic check that a ProjectionDraft preserves the closed ProjectionIntent field-by-field, includes required links, follows omission rules, omits forbidden claims, and does not invent status, owner, date, severity, completion, scope, or promises. | Not a semantic source of work state. |
| **ProjectionDebt** | Explicit state created when a WorkCommission with `external_required` has valid execution evidence but required external publication has not successfully synced. Local execution may be complete; external coordination is not closed. | Not execution failure. Not proof the carrier is authoritative. |

## Persistence

| Term | Definition | NOT this |
|------|-----------|----------|
| **Projection** | Markdown file in `.haft/` generated from the database. Human-readable, git-tracked, reviewable in PRs. Unqualified "Projection" means this artifact projection. | Not semantic authority by itself. Not editable (overwritten on next artifact change). Not ExternalProjection. |
| **Artifact graph** | The DAG of artifacts: problems link to portfolios link to decisions link to evidence. Stored in SQLite. | Not a file tree. Not a git graph. A semantic relationship graph. |
| **.haft/ directory** | Project-local directory containing projections, project.yaml, and subdirectories for each artifact kind. Git-tracked. | Not the database. Not config. The shared-with-team surface. |
| **~/.haft/** | User-local directory: project databases, cross-project index, config, registry. NOT git-tracked. | Not project-specific. Global to the user. |

## FPF (Internal Only — Not User-Facing)

| Term | Definition | When to use |
|------|-----------|-------------|
| **FPF** | First Principles Framework by Anatoly Levenchuk. The theoretical engine powering Haft's reasoning discipline. | Internal docs, architecture discussions. Never in user-facing text. |
| **L1 / L2 / L3** | Delivery layers: L1 = Detect+Ask (skill instructions), L2 = Persist+Enforce (tools/validators), L3 = Watchdog (agent loop checks). | Architecture discussions about where to enforce a pattern. |
| **Pattern** | A named FPF concept (214 total). Mapped to delivery layers. Most stay as L0 RAG, ~80 become L1, ~25 become L2, ~15 become L3. | Mapping FPF to features. Never in user-facing text (say "mode" or "capability" instead). |
| **Transformer Mandate** | Agent generates options; human decides. No self-validation of one's own work. | Architecture principle. In user-facing text: "you choose, the agent recommends." |

## Comparison & Evidence (Runtime Vocabulary)

| Term | Definition | NOT this |
|------|-----------|----------|
| **Characterization** | The set of comparison dimensions defined on a ProblemCard BEFORE generating variants. Each dimension has name, scale, unit, polarity, and indicator role. | Not "requirements." Not "acceptance criteria." Criteria for comparing options, not for accepting the final result. |
| **ComparisonDimension** | A single axis in the characteristic space: name + scale + unit + polarity (higher-is-better / lower-is-better) + role (constraint/target/observation) | Not a "metric." A dimension can be qualitative. |
| **ParityPlan** | Statement of what must be equal across all variants for fair comparison. Currently stored as rules text. | Not a formal report (v6). A structured parity report with evidence trace is planned (v7+). |
| **Advisory recommendation** | The `selected_ref` + `recommendation_rationale` persisted by the compare action. Advisory only — does NOT cross the human choice boundary. | Not "the decision." The decision happens at `/h-decide` after human confirmation. |
| **Coverage gap** | A claim on a DecisionRecord that has no evidence attached. Surfaced in UI and `/h-verify`. | Not "missing test." Any claim without evidence, regardless of claim type. |
| **Derived health** | Computed view-state of a DecisionRecord: Maturity (Unassessed / Pending / Shipped) + Freshness (Healthy / Stale / AT RISK). Never stored. | Not "status." Status is stored (`active`, `superseded`, etc.). Phase is derived from status + evidence. |
| **F_eff (Formality)** | How structured is the evidence? F0 (anecdote) → F3 (formal proof). View concern for evidence decomposition. | Not a separate trust score. Decomposes R_eff inputs. |
| **G_eff (Groundedness)** | How close is the evidence to the thing it verifies? Derived from CL. View concern. | Not a separate trust score. Decomposes R_eff inputs. |

## Desktop Surfaces

| Term | Definition | NOT this |
|------|-----------|----------|
| **Dashboard** | The single unified operator page in the desktop app. Shows active decisions, governance findings, automations, and recent activity. | Not "Problem Board." Problems appear as cards within the dashboard. |
| **Implement** | Dashboard action on a DecisionRecord: spawns agent in worktree with invariants + rationale + workflow.md. | Not "run task." Implement is decision-anchored — the agent gets full reasoning context. |
| **Adopt** | Dashboard action on a governance finding (stale/drifted): creates agent task/thread with decision context + drift report for interactive resolution. | Not auto-fix. Human resolves with agent assistance, then re-baselines, reopens, or waives. |
| **Automation** | A configured trigger that auto-creates ProblemCards: CI fail, dependency update, scheduled, manual. The "problem factory." | Not a workflow engine. Creates problems, doesn't execute solutions. |

## Terms That Cause Confusion

| Confusing term | Clarification |
|---------------|--------------|
| "service" | Which? The OAuth provider? The API endpoint? The background job? The team? Always unpack. |
| "process" | OS process? Business process? Engineering process? State machine? Always qualify. |
| "component" | UI component? Architecture component? System component? Module? Use the specific term. |
| "quality" | Which dimension? Latency? Correctness? Maintainability? Use the indicator role (constraint/target/observation). |
| "simple" | Fewer lines? Fewer dependencies? Familiar to the team? Lower cyclomatic complexity? Operationalize or don't use. |
| "scalable" | Along which axis? Users? Data volume? Team size? Regions? Define the scaling dimension. |
| "refresh" | Legacy term for `/h-verify`. Use "verify" for the mode, "refresh" only when referring to the `haft_refresh` MCP tool. |
| "decision" | The act of choosing? Or the DecisionRecord artifact? Default to "DecisionRecord" when referring to the artifact. |
