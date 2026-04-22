# Tracker Ticket Templates

Use these as starting points for real canary tickets. Replace placeholders
before creating the ticket.

## Low-Risk Intake Canary

Title:

```text
Open-Sleigh canary: inspect and report repository health
```

Description:

```text
Goal:
Run Open-Sleigh on a low-risk repository task and produce evidence without
publishing to a protected branch.

Scope:
- Inspect the repository.
- Make no production-impacting change unless the ProblemCard explicitly asks
  for one.
- Produce a short evidence note.

problem_card_ref: <haft-problem-card-ref>

target_branch: feature/open-sleigh-canary
```

Expected behavior:

- Ticket is active according to `tracker.active_states`.
- ProblemCard resolves through Haft.
- Branch does not match `external_publication.branch_regex`.
- Runtime should not require publication approval.

## Publication-Gated Canary

Title:

```text
Open-Sleigh canary: publication gate smoke test
```

Description:

```text
Goal:
Verify that Open-Sleigh requests human approval before configured external
publication.

Scope:
- Use a harmless documentation-only change.
- Stop at the human gate unless an authorized approver explicitly approves.

problem_card_ref: <haft-problem-card-ref>

target_branch: main
```

Expected behavior:

- Ticket reaches Execute.
- `commission_approved` human gate is requested because `target_branch`
  matches `external_publication.branch_regex`.
- Authorized approval is required before publication proceeds.

## Required Fields

- `problem_card_ref`: must point to a live upstream Haft ProblemCard.
- `target_branch`: required for publication policy behavior.
- Active tracker state: must match `tracker.active_states`.

Do not use a self-authored ProblemCard. Open-Sleigh rejects upstream framing
artifacts whose `authoring_source` is `open_sleigh_self`.

