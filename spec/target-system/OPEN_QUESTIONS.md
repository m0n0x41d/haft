# Open Questions

> Non-blocking for v6 release. Classified by v7 impact after GPT-5.4 spec review.

## Decided (formerly blocking v7)

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| Q1 | Workflow DSL shape | **`.haft/workflow.md` as hybrid markdown + structured YAML block.** Human-readable prose + parseable defaults + path policies. Not a workflow engine. | Pure markdown = too hard to validate. Pure YAML = config fatigue. FPF flow card = too heavy for solo dev. Hybrid gives reviewable prose + parseable structure + low implementation burden. Ships in v6.1. |
| Q2 | Fate of `haft agent` | **Transitional. Retained for CI/headless/power-user. Not co-equal strategic surface.** Desktop = primary interactive. MCP = primary embedded. Agent = secondary headless utility. No feature-parity promise with desktop. | Desktop subsumes interactive use. But CI stakeholder needs headless mode. SSH/remote use cases real. Removing entirely loses too much utility. |
| Q10 | L2 enforcement scope | **Both JSON Schema + Go validators, but narrow: parity minimums + subjective dimension operationalization only.** No broad A.6 universalization before v7. | Schema alone can't catch semantic hollowness. Go validators without schema = brittle text heuristics. Narrow scope = feasible for solo dev. Ships in v6.1. |

## Resolved (No Longer Open)

| # | Question | Resolution |
|---|----------|-----------|
| Q7 | Should the GitHub repo rename from quint-code to haft? | Rename completed in v6.0.0. GitHub redirects old URLs. Migration task, not design question. |
| Q8 | How to handle install.sh URL after rename? | install.sh updated to install `haft` binary. `quint.codes` domain stays. Operational task. |

## Deferred (Not Blocking v7)

| # | Question | Context | When relevant |
|---|----------|---------|--------------|
| Q3 | Multi-project governance views in desktop? | Per-project tabs exist. Cross-project portfolio view doesn't. | When multi-project users request it |
| Q4 | When to include research-before-code lane? | Some tasks need external knowledge. | Only if deep mode or compliance tasks enter v7 default execution loop |
| Q5 | Autonomy budget granularity? | Currently binary (on/off). FPF E.16 describes typed budgets. | When host agent permissions prove insufficient |
| Q6 | How to measure semio-quality? | No benchmark for fanout, authority confusion, gate/evidence mixing. | When L2 language precision ships |
| Q9 | Is "keeps the coder honest" the right tagline? | Validated in research. Slightly adversarial. Doesn't reflect verification/memory. | Next marketing pass |
| Q11 | Role/context-scoped skills vs global skill library? | FPF suggests role+context binding. Hermes uses global. | When demand signal exists |
