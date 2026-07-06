# Term Map

```yaml term-map
status: active
entries:
  - term: Target system
    category: fpf-boundary
    definition: The project capability or repository state that Haft is meant to change and govern.
    aliases:
      - target-system
    not:
      - The host coding agent.
      - The implementation plan.
    owners:
      - human
  - term: Enabling system
    category: fpf-boundary
    definition: The people, workflow, architecture, tools, and runtime paths that make the target system possible.
    aliases:
      - enabling-system
    not:
      - The governed repository state itself.
    owners:
      - human
  - term: Governance substrate
    category: haft-role
    definition: Haft's role as the typed artifact graph, validation layer, and query surface that make work admissible.
    aliases:
      - substrate
    not:
      - A coding agent.
      - A CI runner.
      - A product manager.
    owners:
      - human
  - term: Carrier
    category: authority
    definition: A file or surface representation that stores a description until the kernel parses and validates it.
    aliases:
      - markdown carrier
    not:
      - Authority by itself.
    owners:
      - human
  - term: SpecSection
    category: specification
    definition: A typed target-system or enabling-system claim parsed from a spec carrier and governed by lifecycle checks.
    aliases:
      - spec section
    not:
      - Freeform markdown prose.
    owners:
      - human
  - term: Baseline
    category: specification
    definition: A recorded approved hash of an active SpecSection that lets future drift be detected.
    aliases:
      - SpecSectionBaseline
    not:
      - A casual review comment.
    owners:
      - human
  - term: DecisionRecord
    category: reasoning
    definition: A human-gated binding artifact that records the chosen contract, rationale, consequences, evidence, and refresh triggers.
    aliases:
      - DRR
      - decision record
    not:
      - An agent recommendation.
    owners:
      - human
  - term: WorkCommission
    category: execution
    definition: A bounded authorization to attempt work under an explicit scope, delivery policy, evidence requirement, and autonomy envelope.
    aliases:
      - commission
    not:
      - A runtime process.
      - Permission to publish externally.
    owners:
      - human
  - term: RuntimeRun
    category: execution
    definition: A concrete harness execution of an admissible WorkCommission in an isolated workspace or worktree.
    aliases:
      - runtime run
    not:
      - The commission itself.
    owners:
      - human
  - term: Evidence
    category: verification
    definition: An inspectable observation attached to an artifact with verdict, congruence level, and freshness boundary.
    aliases:
      - evidence record
    not:
      - Documentation alone.
    owners:
      - human
  - term: R_eff
    category: verification
    definition: Effective reliability of a claim computed by the weakest supported evidence link, not by averaging.
    aliases:
      - effective reliability
    not:
      - Confidence by prose volume.
    owners:
      - human
```

This term map grounds load-bearing Haft vocabulary used by the active target
and enabling specs. It is a carrier; typed Haft checks decide whether entries
are parseable and non-conflicting.
