# Execution Contract — v6.2

> What Implement, Adopt, and verification are allowed to do.
> BDD scenarios define the authority boundaries.

## Implement (Decision → Agent → Verify → Baseline)

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
| **Implement** | worktree, branch, files in worktree | task status | DecisionRecord, evidence, baseline (until verification passes) |
| **Post-verify (pass)** | evidence item (CL3), baseline snapshot | task status → "Ready for PR" | DecisionRecord body |
| **Post-verify (fail)** | — | task status → "Needs attention" | nothing else until user decides |
| **Adopt** | RefreshReport, optionally new ProblemCard | decision status (only via explicit waive/supersede/deprecate) | decision body, evidence content |
| **Create PR** | git branch push, PR body | — | artifacts, evidence, baselines |

## What Is NOT in v6.2

These are deferred per 5.4 review:

| Feature | Why deferred |
|---------|-------------|
| Automation triggers (CI fail, dep update, scheduled) | Mixing problem factory + execution in one release = scope sprawl |
| DecisionRecord→Task Pipeline with auto-advance | Build single Implement first, pipeline is v7 |
| Deep onboard as automation input | Onboard prompt already deep in v6.1, automation wrapper is v7 |
| Autonomous verification agent | Detect-only first (v8 Phase A), actuation later (Phase B) |
