# Open Questions

> Non-blocking for v6 release. Classified by v7 impact after GPT-5.4 spec review.

## Blocking Before v7

| # | Question | Context | Status |
|---|----------|---------|--------|
| Q1 | What workflow DSL for repo-level execution policies? | v7 deliverable: `.haft/workflow.md`. Can't ship execution loop without choosing representation. | Candidates: `.haft/workflow.md` (simple) vs structured YAML vs FPF-native flow card |
| Q2 | Is `haft agent` strategic, transitional, or deprecated? | Desktop app subsumes most standalone agent use. TUI adds maintenance burden. Affects architecture, docs, testing. | Candidates: Keep both / Desktop-only / Agent for CI-only |
| Q10 | Which FPF patterns get L2 enforcement before v7? | Live gaps: parity enforcement (G2) and subjective dimension operationalization (G4). Cannot expand execution loop while these are L1-only. | Candidates: A.6.P/Q/A triggers in Go / JSON Schema structured outputs / Both |

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
