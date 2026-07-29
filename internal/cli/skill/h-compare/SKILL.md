---
name: h-compare
description: |
  Compare two or more existing candidates under an explicit characteristic space, parity basis, and predeclared selection policy. Return constraints, trade-offs, and a non-dominated set rather than hiding the choice in one score. This skill is independent of h-frame and h-explore. Default to a conversational comparison; persist comparison artifacts only on explicit save intent or when a named receiving use needs replay. A binding choice remains manual h-decide.
when_to_use: |
  Two or more live candidates already exist and the current question is their fair comparison.
argument-hint: "[candidate set, portfolio reference, or comparison question]"
allowed-tools: Bash Read Grep Glob Agent mcp__haft__haft_note mcp__haft__haft_problem mcp__haft__haft_solution mcp__haft__haft_query
---

# h-compare — Compare without hidden order or scalarization

Retrieve the current source before applying the comparison distillate. For a
known SourceID or UnitID, call
`mcp__haft__haft_query(action="fpf", mode="inspect", identifier="...")`.
Otherwise call `mcp__haft__haft_query(action="fpf", mode="concern",
query="<comparison question>")`, then inspect the direct pattern body. Query
returns source candidates, not a selected governing pattern or a comparison
verdict. A retrieval score or candidate-list order is presentation metadata,
not comparison evidence or precedence.

## Conditional project-memory orientation

When this comparison is context-heavy, multi-session, or reliance-bearing and
the exact EntityOfConcern is not already current, resolve its identity with
`haft_query(action="memory", memory_request={"mode":"resolve",
"contract_version":"haft.memory.v1","basis":{"kind":"project_current"},
"query":"...","max_candidates":5})`. Select the exact candidate by the current
use rather than rank, then use the closed `memory_request` neighborhood branch
advertised by the tool schema with
`projection_profile_ref="agent_orientation.v2"`.

Inspect `result_kind` before relying on content. `project_basis_unavailable`,
known absence, or explicit abstention is visible but non-blocking: continue the
comparison without inventing a profile, entity, artifact, or human gate. This
read does not replace code-graph preflight before a later code edit. Never
persist typed memory merely because a read failed; persistence requires an
explicit operator save request or a named receiving use with provenance.

## Procedure

1. Name the candidates and the current comparison question. An inline set is
   enough for ordinary use; no prior SolutionPortfolio is required.
2. Draft the characteristic space before scoring. Separate:
   - constraints: hard admissibility limits;
   - targets: at most 1-3 values under optimization pressure;
   - observations: values to watch without optimizing.
3. Name what each target is a proxy for. Keep incompatible scales separate.
4. Declare parity before results: comparator set, evidence window, equalized
   budget or conditions, missing-data policy, and material assumptions.
5. Declare the selection policy before scoring. Do not change it after seeing
   a preferred result without making the policy change explicit.
6. Evaluate one dimension across all variants before moving to another. Use
   independent evaluators when that reduces anchoring.
7. Eliminate constraint violations, then report the non-dominated set and the
   concrete trade-off among survivors. Abstain where parity is insufficient.

## Persistence boundary

Ordinary comparison stays conversational. Persist characterization or a
comparison only when the operator asks to save it or a named receiving use
needs addressable replay. Use existing problem/portfolio refs when available;
do not manufacture a frame or exploration history to simulate a universal
workflow.

For a durable current portfolio, preserve the exact typed-memory coordinates.
Each option must already be an independently addressable
`Haft.ProjectRecordRef`. If the receiving use needs a new durable portfolio,
first persist only the candidate descriptions it will rely on with
`haft_note`, retain each returned `record_reference`, and pass those exact
references as each variant's `project_record_ref` to `haft_solution(action="explore")`.
Never derive a record ID from a filename or artifact ID.

Then discover the exact interfaces and call:

```text
mcp__haft__haft_problem(action="characterize", problem_ref="<ref>", dimensions=[...])
mcp__haft__haft_solution(
  action="compare",
  portfolio_ref="<ref>",
  entity_ref={
    "ref_kind_id": "U.EntityRef",
    "reference_id": "<exact resolved EntityOfConcern>"
  },
  bounded_context_ref="<exact bounded context>",
  dimensions=[...],
  scores={...},
  parity_plan={...},
  policy_applied="<declared before scoring>"
)
```

On a committed typed projection, preserve the returned comparison
`record_reference`; it is the exact address of this comparison edition.
Missing portfolio or option references leave the legacy carrier durable but
the typed projection `underdetermined`. Repair the missing reference instead
of inventing one.

Do not send `selected_ref` for an ordinary typed comparison. The kernel keeps
that legacy field only for compatibility and excludes it from
`Haft.PortfolioComparison`. A non-dominated set is not a winner,
recommendation, ChoiceResult, operator selection, gate, or execution
authority.

## Result

Present evidence limitations first, then the options still worth considering,
what each gives up, and the exact value choice that remains with the operator.
Do not prescribe `h-decide`; mention it only when the operator says the
binding choice is now current.
