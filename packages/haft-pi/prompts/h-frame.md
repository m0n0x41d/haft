Frame the problem before any solution work.

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
