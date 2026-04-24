# Execution Contract — v6.2 + Commissioned Execution Draft

> What Implement, Adopt, and verification are allowed to do.
> BDD scenarios define the authority boundaries.

## Commissioned Execution Model

Direct `DecisionRecord -> agent` execution is the v6.2 local loop. The
Haft target model inserts a deliberate work authorization layer:

```
DecisionRecord -> WorkCommission -> Preflight -> RuntimeRun -> Evidence
```

The distinction is load-bearing:

- DecisionRecord = what was chosen and why.
- WorkCommission = permission to execute that choice in a declared scope.
- RuntimeRun = one actual attempt by a runner.
- Evidence = what was verified after execution.

`Open-Sleigh` is the current runtime implementation of `haft harness`. It is
the execution subsystem of Haft, not a peer source of truth.

A DecisionRecord may have zero WorkCommissions. Creating a decision does not
mean the work is scheduled. A WorkCommission may be queued for later, and must
be revalidated before execution starts.

## WorkCommission Lifecycle

```gherkin
Scenario: Queue work without executing it
  Given a DecisionRecord with status "active"
  When the user creates a WorkCommission from that decision
  Then the WorkCommission stores:
    | field                  | source                                      |
    | decision_ref           | selected DecisionRecord                     |
    | decision_revision_hash | current DecisionRecord content/revision     |
    | problem_ref            | linked ProblemCard                          |
    | scope                  | closed Scope object: repo, branch, base SHA, paths, actions |
    | scope_hash             | canonical hash of the Scope object          |
    | base_sha               | repository commit pinned at queue time      |
    | implementation_plan_revision | parent plan revision/hash, if any      |
    | autonomy_envelope_revision | approved envelope revision/hash, if any |
    | gates                  | decision invariants + workflow policy       |
    | evidence_requirements  | decision claims + commission-specific checks |
    | projection_policy      | local_only / external_optional / external_required |
    | valid_until            | explicit execution freshness deadline       |
  And the WorkCommission is "queued"
  And no RuntimeRun starts
```

```gherkin
Scenario: Scope is authorization, not prompt context
  Given a WorkCommission with Scope S
  Then S is serialized into a canonical form and stored as scope_hash
  And the Scope contains:
    | field             | meaning                                   |
    | repo_ref          | repository identity                       |
    | base_sha          | repository commit pinned for comparison   |
    | target_branch     | branch or branch policy                   |
    | allowed_paths     | paths the runner may read/write as declared |
    | forbidden_paths   | paths the runner must not mutate          |
    | allowed_actions   | edit_files, run_tests, commit, or similar |
    | allowed_modules   | optional module-level slice               |
    | affected_files    | expected mutation/evidence surface        |
    | lockset           | concurrency-control projection            |
  And Open-Sleigh must carry the Scope in Session and AdapterSession
  And every file-mutating adapter call must check the target path/action
      against the Scope before executing
  And terminal diff validation must prove every mutation stayed inside Scope
```

```gherkin
Scenario: Start a fresh commission
  Given a WorkCommission in "queued" or "ready"
  And its linked DecisionRecord is still active
  And the decision_revision_hash still matches
  And the problem_ref/revision still matches
  And the scope_hash still matches the current commission Scope
  And the base_sha still matches the admitted repository context
  And the implementation_plan_revision still matches, if present
  And the autonomy_envelope_revision still matches, if present
  And the commission valid_until is in the future
  And no linked ProblemCard or governing DecisionRecord was superseded
  When the user starts the WorkCommission
  Then Haft moves it to "preflighting"
  And grants exactly one runner lease
  And Open-Sleigh may run the Preflight phase
```

```gherkin
Scenario: Block a stale commission before execution
  Given a WorkCommission created from DecisionRecord revision R1
  And the DecisionRecord was superseded to revision R2 before execution
  When the user or YOLO scheduler attempts to start the WorkCommission
  Then Haft marks the WorkCommission "blocked_stale"
  And no RuntimeRun enters Execute
  And the block reason names the invalidating artifact
```

```gherkin
Scenario: Block a commission after snapshot drift
  Given a WorkCommission was queued with CommissionSnapshot C1
  And a human approval or YOLO lease was recorded for C1
  When the DecisionRecord revision, ProblemCard revision, Scope hash, base SHA,
      ImplementationPlan revision, AutonomyEnvelope revision, or lease state
      changes before Execute
  Then the previous approval is no longer reusable
  And Haft requires deterministic re-preflight before any RuntimeRun may Execute
  And unresolved drift blocks as "blocked_stale" or "needs_human_review"
```

```gherkin
Scenario: Runtime mutation outside Scope is terminal
  Given a RuntimeRun is executing WorkCommission W with Scope S
  When the runner attempts to edit a path outside S.allowed_paths
  Or attempts an action not present in S.allowed_actions
  Or the terminal diff contains a mutation outside S
  Then the RuntimeRun fails terminally with reason "mutation_outside_commission_scope"
  And Haft marks the WorkCommission "blocked_policy" or "failed"
  And no evidence from the out-of-scope mutation can complete W
```

## Preflight

Preflight is mandatory before execution, including YOLO/batch runs. It has two
parts:

| Layer | May do | May NOT do |
|-------|--------|------------|
| Deterministic gate | Check existence, status, revisions, expiry, leases, policies, required approvals, runner eligibility, CommissionSnapshot equality, scope hash, base SHA, plan revision, envelope revision, lockset availability | Infer semantic freshness from prose |
| Preflight agent | Read linked artifacts, inspect repo context, summarize material changes, recommend pass/block/review | Decide final authority state or skip deterministic gates |

The deterministic equality set is closed for MVP-1R:

| Field | Owner | Drift outcome |
|-------|-------|---------------|
| DecisionRecord ref/revision/hash | Haft | block stale |
| ProblemCard ref/revision/hash | Haft | block stale or human review |
| Scope hash | Haft | block policy |
| base SHA / admitted repo context | Haft + repo adapter | re-preflight or block stale |
| ImplementationPlan revision | Haft | release/recompute queue node |
| AutonomyEnvelope revision | Haft | block policy until re-approved |
| lease id/state | Haft | deny start_after_preflight |

```gherkin
Scenario: Runner cannot bypass Haft authority
  Given Open-Sleigh has a commission_id
  When it starts work
  Then it first calls Haft to claim a preflight lease
  And it receives a signed/structured preflight context
  And it may only continue to Execute after Haft records preflight as passed
```

```gherkin
Scenario: Uncertain preflight needs human review
  Given deterministic checks pass
  But the preflight agent reports material context change it cannot classify
  When Haft validates the PreflightReport
  Then the WorkCommission becomes "needs_human_review"
  And Open-Sleigh stops before Execute
```

## ImplementationPlan and YOLO Mode

YOLO mode is batch continuation inside a human-approved AutonomyEnvelope. It
does not skip freshness, evidence, lease, lockset, or one-way-door gates.

```gherkin
Scenario: Run an approved implementation plan in YOLO mode
  Given an ImplementationPlan with 20 WorkCommissions
  And an AutonomyEnvelope approved by the human principal:
    | property        | example                         |
    | max_concurrency | 4                               |
    | allowed_repos   | current project                 |
    | allowed_paths   | internal/**, desktop/**         |
    | forbidden_paths | release/**, migrations/**       |
    | allowed_actions | edit_files, run_tests, commit   |
    | forbidden_actions | merge_pr, tag_release, delete_data |
    | on_failure      | continue_independent            |
    | on_stale        | block_node                      |
  When Open-Sleigh starts the plan
  Then it schedules only dependency-ready WorkCommissions
  And it never runs two commissions with overlapping locksets
  And it preflights every commission immediately before Execute
  And it blocks stale or uncertain nodes without blocking independent nodes
  And it records RuntimeRun and Evidence for every attempted commission
```

```gherkin
Scenario: YOLO cannot expand its own authority
  Given an AutonomyEnvelope forbids schema changes and release tagging
  When an agent discovers the chosen implementation requires a schema change
  Then the current WorkCommission becomes "needs_human_review"
  And no schema migration or release tag is created automatically
```

```gherkin
Scenario: ImplementationPlan changes after a commission is leased
  Given a WorkCommission was leased under ImplementationPlan revision P1
  And the plan is revised to P2 before the commission reaches Execute
  When Open-Sleigh calls start_after_preflight
  Then Haft rejects the start with "plan_revision_changed"
  And the scheduler must release or re-preflight the node under P2
```

## External Projection

Haft works without Linear/Jira/GitHub Issues. External projection is optional
per workspace and per WorkCommission.

```gherkin
Scenario: Local-only commission
  Given a WorkCommission with projection_policy "local_only"
  When it runs and completes
  Then Desktop/CLI/.haft status are updated
  And no external tracker call is required
```

```gherkin
Scenario: External projection uses bounded LLM writing
  Given a WorkCommission with projection_policy "external_optional"
  And Linear is configured as a projection target
  When Haft computes a ProjectionIntent
  Then a ProjectionWriterAgent may draft manager-facing text
  And ProjectionValidation must pass before publication
  And the LLM may not decide lifecycle status, severity, evidence verdict, or completion
```

First live canary rule: use deterministic projection templates only. Enable
ProjectionWriterAgent after the closed ProjectionIntent schema and
ProjectionValidation field-by-field checks have their own evidence.

```gherkin
Scenario: External required creates projection debt, not execution failure
  Given a WorkCommission with projection_policy "external_required"
  And RuntimeRun evidence satisfies the commission evidence requirements
  But the required external carrier publish fails or is unavailable
  When Haft completes local execution adjudication
  Then the RuntimeRun evidence remains valid
  And the WorkCommission enters "completed_with_projection_debt"
  And Haft records ProjectionDebt naming the carrier, target, last error,
      and retry policy
  And the commission is not shown as externally closed until the debt is resolved
```

```gherkin
Scenario: Manual external Done does not complete Haft work
  Given a Linear issue linked by ExternalProjection
  And a human manually moves the issue to Done
  But the WorkCommission has no accepted evidence
  When Haft observes the external state
  Then Haft records projection drift/conflict
  And the WorkCommission remains not completed
```

## Implement (Decision → Agent → Verify → Baseline)

The v6.2 direct Implement flow remains the local single-run surface. In the
commissioned execution model, "Implement" becomes a convenience action that
creates a WorkCommission and, if the user chooses "start now", immediately
runs the same preflight path described above.

### Happy path

```gherkin
Scenario: Implement a decision from dashboard
  Given a DecisionRecord with status "active" and at least one invariant
  And the decision has affected_files defined
  When the user clicks "Implement"
  Then a new worktree is created on a feature branch
  And an agent session starts with:
    | context              | source                          |
    | invariants           | from DecisionRecord             |
    | affected_files       | from DecisionRecord             |
    | rationale            | from linked SolutionPortfolio   |
    | workflow policy      | from .haft/workflow.md          |
    | governing invariants | from knowledge graph (all decisions governing affected files) |
  And the agent executes in checkpointed mode (pauses for review)
  And the user sees real-time output in the dashboard task view
```

### Post-execution verification

```gherkin
Scenario: Successful verification after implementation
  Given an agent task completed without errors
  When post-execution verification runs automatically
  Then each invariant from the DecisionRecord is checked against the worktree
  And if all invariants hold:
    | action               | detail                                    |
    | baseline refresh     | SHA-256 snapshot of affected_files         |
    | verification record  | linked to DecisionRecord as evidence CL3  |
    | task status           | "Ready for PR"                            |
  And the dashboard shows "Create PR" button

Scenario: Verification failure
  Given an agent task completed
  When post-execution verification finds a broken invariant
  Then the task status becomes "Needs attention"
  And the broken invariant is shown with diff context
  And the user can:
    | action         | what happens                                    |
    | Fix and retry  | agent continues in same worktree                |
    | Reopen         | creates new ProblemCard linked to original decision |
    | Dismiss        | waive the invariant with justification          |
  And NO baseline is taken (verification did not pass)
  And NO PR is suggested
```

### Implement guards (what blocks Implement)

```gherkin
Scenario: Decision without invariants
  Given a DecisionRecord with zero invariants
  When the user clicks "Implement"
  Then implementation proceeds with a warning:
    "No invariants defined — post-execution verification will be skipped"

Scenario: Decision with unresolved subjective dimensions (G4 warning)
  Given a DecisionRecord whose comparison used subjective dimensions
  And the dimensions were not decomposed or tagged as observation-only
  When the user clicks "Implement"
  Then a warning is shown:
    "Comparison basis includes unresolved subjective dimensions — proceed?"
  And the user must confirm to continue

Scenario: Decision without parity plan (G2 warning)
  Given a DecisionRecord in standard/deep mode without structured parity plan
  When the user clicks "Implement"
  Then a warning is shown:
    "No parity plan recorded — comparison may not be fair — proceed?"
  And the user must confirm to continue

Scenario: Multiple active decisions for same problem (G1 — blocked)
  Given two active DecisionRecords for the same ProblemCard
  When the user clicks "Implement" on either
  Then implementation is blocked:
    "Multiple active decisions for this problem — supersede one first"
```

## Adopt (Governance Finding → Agent Thread → Resolution)

### Happy path

```gherkin
Scenario: Adopt a drifted decision
  Given a governance finding: "dec-001 drifted — 3 files modified"
  When the user clicks "Adopt"
  Then a new agent task/thread is created with context:
    | context          | source                          |
    | decision         | full DecisionRecord body        |
    | drift report     | modified/missing files + diffs  |
    | invariants       | from DecisionRecord             |
    | affected modules | from knowledge graph            |
  And the agent and user investigate interactively
  And the user resolves by choosing one of:
    | resolution      | what happens                                |
    | Re-baseline     | new SHA-256 snapshot, decision stays active  |
    | Reopen          | new ProblemCard, decision superseded        |
    | Waive           | extend valid_until with justification       |

Scenario: Adopt a stale decision
  Given a governance finding: "dec-002 stale — R_eff 0.3, evidence expired"
  When the user clicks "Adopt"
  Then an agent thread opens with the decision context + evidence history
  And the user can:
    | resolution      | what happens                                |
    | Measure         | run verification, attach new evidence       |
    | Waive           | extend with justification                   |
    | Deprecate       | archive as no longer relevant               |
    | Reopen          | new problem cycle from this decision        |
```

### Adopt guards

```gherkin
Scenario: Adopt does not auto-resolve
  Given any governance finding
  When the user clicks "Adopt"
  Then the agent NEVER automatically re-baselines, waives, or deprecates
  And the agent presents options and waits for the user to choose
  And the chosen action is recorded as a RefreshReport

Scenario: Adopt preserves decision history
  Given a governance finding being adopted
  When the user chooses any resolution
  Then the original DecisionRecord is never deleted
  And superseded/deprecated decisions retain full body and evidence for audit
```

## Human Confirmation Points

```gherkin
Scenario: Human must confirm before every irreversible action
  Given any of: Implement, Create PR, Reopen, Supersede, Deprecate
  When the action is triggered from the dashboard
  Then the user sees a confirmation with:
    - what will happen
    - what cannot be undone
    - affected artifacts
  And the action executes only after explicit confirmation
  And auto-advance mode does NOT skip confirmation for one-way-door actions
```

## What Implement/Adopt May Change

| Action | May create | May modify | May NOT modify |
|--------|-----------|-----------|----------------|
| **Create WorkCommission** | WorkCommission draft/queued record | commission status/scope before approval | DecisionRecord body, evidence, baseline |
| **Start WorkCommission** | preflight lease, RuntimeRun shell | WorkCommission status → preflighting/running/blocked | DecisionRecord body; execution may not start if freshness gate fails |
| **Implement** | WorkCommission, worktree, branch, files in worktree | task/commission status | DecisionRecord, evidence, baseline (until verification passes) |
| **Post-verify (pass)** | evidence item (CL3), baseline snapshot | task status → "Ready for PR" | DecisionRecord body |
| **Post-verify (fail)** | — | task status → "Needs attention" | nothing else until user decides |
| **Adopt** | RefreshReport, optionally new ProblemCard | decision status (only via explicit waive/supersede/deprecate) | decision body, evidence content |
| **Create PR** | git branch push, PR body | — | artifacts, evidence, baselines |
| **ExternalProjection publish** | external issue/comment/update | ExternalProjection observed/sync metadata | WorkCommission semantic state, DecisionRecord, evidence |

## What Is NOT in v6.2

These are deferred per 5.4 review:

| Feature | Why deferred |
|---------|-------------|
| Automation triggers (CI fail, dep update, scheduled) | Mixing problem factory + execution in one release = scope sprawl |
| DecisionRecord→WorkCommission→RuntimeRun Pipeline with auto-advance | Build single Implement first; commissioned/batch execution is the Open-Sleigh integration path |
| Deep onboard as automation input | Onboard prompt already deep in v6.1, automation wrapper is v7 |
| Autonomous verification agent | Detect-only first (v8 Phase A), actuation later (Phase B) |
