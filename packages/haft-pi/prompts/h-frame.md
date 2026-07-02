Frame the problem before any solution work.

Before substantive framing, call `haft_query`:

```json
{
  "action": "pattern_use",
  "mode": "compact",
  "query": "<operator concern>"
}
```

Skip only mechanical/status/exact-lookup requests where no FPF pattern choice is
material. If `should_use_pattern=true` and framing needs output-shape detail,
ask for `mode="full"` before applying the returned pattern. PatternUse is
advisory/read-only: not approval, not evidence, not a DecisionRecord, not a
WorkCommission, not MethodPack, and not a gate. Do not inline the FPF catalog
or route list in this prompt.

1. Stabilize the signal: what is actually broken or wanted, in one sentence?
   What observable condition would make the operator say "solved"?
2. Type it: diagnosis | optimization | search | synthesis.
3. Repair umbrella words ("quality", "done", "scalable") — refuse to frame on
   them; unpack to concrete observables first.
4. Persist via the native `haft_problem` tool:

```json
{
  "action": "frame",
  "problem_type": "<diagnosis|optimization|search|synthesis>",
  "title": "<short title>",
  "signal": "<what is happening / what is needed>",
  "acceptance": "<observable condition>",
  "constraints": ["<hard limit>"],
  "blast_radius": "<what gets affected>",
  "reversibility": "<low|medium|high>"
}
```

5. Surface the ProblemCard ID with its title to the operator.

Do not skip to variants: without a framed problem, exploration floats.
