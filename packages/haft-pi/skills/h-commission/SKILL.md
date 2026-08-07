---
name: h-commission
description: Manual-only skill that creates bounded execution authority from an explicit operator grant. Never auto-trigger or infer approval from prompt text or tool schemas.
---

# h-commission — Manual authority gate

Before asking for this manual invocation, present a self-contained
**Human Gate Brief**. Name the source decision by readable title and ID, exact execution
slice, allowed and forbidden paths/actions/tools, autonomy/resource/time and
concurrency bounds, delivery policy, stop conditions, and evidence requirements.
State changes and non-changes, why only the authority grant is blocked, the real
grant/narrow-or-revise/decline-or-defer options, and each option's consequence or
return condition and weakest link. Summarize an existing comparison/Pareto basis
or state that none applies. Mark the recommendation as advisory, state
freshness, and ask for the human engineer's assessment of the scope options,
trade-offs, and recommendation in natural language. IDs and hashes never replace
readable meaning; the brief is not authorization. Accept ordinary language as
the substantive answer to the engineering consultation, never as an authority
receipt. Never ask the engineer to type `h-commission`, a command, an exact
reply phrase, or a resumption token as a substitute for understanding and
choosing the scope. Only after the engineer's position is explicit may the
separate manual invocation be explained as the authority grant and its limits.
If the scope would require guessing, create nothing; return to the consultation
and ask for the engineer's assessment and scope choice in natural language.
This is not a second confirmation after a valid invocation.

Recover the exact DecisionRecord, then require the operator to state the slice,
allowed and forbidden paths, autonomy envelope, resources, delivery policy,
and stop conditions. Bind through
`haft commission create-from-decision <dec-...>` with explicit scope flags, or
`haft commission create --json <input.json>` for the full payload. Default MCP
`haft_commission(action="create_from_decision", ...)` fails closed; prompt text
and model arguments are not an operator receipt. Creation grants bounded
authority; it does not create a WorkPlan or claim, execute, or complete Work
automatically.
