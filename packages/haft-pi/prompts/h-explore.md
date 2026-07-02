Generate a solution portfolio for a framed problem.

Before substantive exploration, call `haft_query`:

```json
{
  "action": "pattern_use",
  "mode": "compact",
  "query": "<operator concern>"
}
```

Skip only mechanical/status/exact-lookup requests where no FPF pattern choice is
material. If `should_use_pattern=true` and exploration needs output-shape
detail, ask for `mode="full"` before applying the returned pattern. PatternUse
is advisory/read-only: not approval, not evidence, not a DecisionRecord, not a
WorkCommission, not MethodPack, and not a gate. Do not inline the FPF catalog
or route list in this prompt.

1. Confirm a ProblemCard exists (frame first via /h-frame if not).
2. Generate 3-5 variants that differ in KIND, not degree. Directions to
   force diversity: data-flow restructure, algorithmic alternative,
   infrastructure swap, caching/batching, architectural extraction, workflow
   restructure, stepping-stone.
3. Each variant carries: title, description, novelty_marker, weakest_link
   (what bounds quality — not the title repeated), stepping_stone flag with
   basis, risks, strengths.
4. Keep 1-2 stepping stones (weak on quality now, unlock future search
   space) or state no_stepping_stone_rationale.
5. Persist via the native `haft_solution` tool:

```json
{
  "action": "explore",
  "problem_ref": "<prob-...>",
  "variants": [{ "title": "...", "weakest_link": "...", "novelty_marker": "...", "description": "...", "risks": ["..."], "strengths": ["..."], "stepping_stone": false }]
}
```

6. Read kernel warnings (disguised duplicates, weak weakest-links) and
   self-correct. Recommend /h-compare next if 2+ variants stand.
